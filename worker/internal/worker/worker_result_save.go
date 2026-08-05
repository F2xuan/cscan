package worker

import (
	"context"
	"fmt"
	"time"

	"cscan/internal/scanner"
)

func (w *Worker) saveAssetResult(ctx context.Context, workspaceId, mainTaskId, orgId string, assets []*scanner.Asset) {
	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("Save asset result panic recovered: %v, stack: %s", r, string(getStackTrace()))
		}
	}()

	if len(assets) == 0 {
		return
	}

	// OOM 约束：maxBatchSize=50 为硬上限，避免大批次在低内存场景下加剧内存压力；
	// 同时按字节切分（累计 ≤ 1MB，单条 ≤ 20KB），取较小者，防止单批过大。
	totalAssets := len(assets)
	boundaries := calculateAssetBatchBoundaries(assets)
	totalBatches := len(boundaries)

	w.taskLog(mainTaskId, LevelInfo, "Saving %d assets in %d batches (maxBatchSize=%d, maxBatchBytes=%d)",
		totalAssets, totalBatches, maxBatchSize, maxBatchBytes)

	var totalNew, totalUpdate int32

	for batchIdx := 0; batchIdx < totalBatches; batchIdx++ {
		start := boundaries[batchIdx].start
		end := boundaries[batchIdx].end

		batchAssets := assets[start:end]
		httpAssets := make([]AssetDocument, 0, len(batchAssets))

		for _, asset := range batchAssets {
			httpAsset := AssetDocument{
				Authority:  asset.Authority,
				Host:       asset.Host,
				Port:       int32(asset.Port),
				Category:   asset.Category,
				Service:    asset.Service,
				Title:      asset.Title,
				App:        asset.App,
				HttpStatus: asset.HttpStatus,
				HttpHeader: asset.HttpHeader,
				HttpBody:   asset.HttpBody,
				IconHash:   asset.IconHash,
				IconData:   asset.IconData,
				Screenshot: asset.Screenshot,
				Server:     asset.Server,
				Banner:     asset.Banner,
				IsHttp:     asset.IsHTTP,
				Cname:      asset.CName,
				IsCdn:      asset.IsCDN,
				IsCloud:    asset.IsCloud,
				Source:     asset.Source,
			}

			// 添加IPv4信息
			for _, ip := range asset.IPV4 {
				httpAsset.Ipv4 = append(httpAsset.Ipv4, IPV4Info{
					IP:       ip.IP,
					Location: ip.Location,
				})
			}

			// 添加IPv6信息
			for _, ip := range asset.IPV6 {
				httpAsset.Ipv6 = append(httpAsset.Ipv6, IPV6Info{
					IP:       ip.IP,
					Location: ip.Location,
				})
			}

			httpAssets = append(httpAssets, httpAsset)
		}

		// 失败时使用新 context 重试，避免共享父 ctx 已 cancel 导致重试无效
		const maxBatchRetry = 3
		var resp *TaskResultResp
		var err error
		for attempt := 1; attempt <= maxBatchRetry; attempt++ {
			batchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			resp, err = w.httpClient.SaveTaskResult(batchCtx, &TaskResultReq{
				WorkspaceId: workspaceId,
				MainTaskId:  mainTaskId,
				Assets:      httpAssets,
				OrgId:       orgId,
			})
			cancel()
			// 仅当传输成功且业务码为 0 才视为成功（API 业务失败仍返回 HTTP 200 + code!=0）
			if err == nil && resp != nil && resp.Code == 0 {
				break
			}
			if attempt < maxBatchRetry {
				respCode := 0
				if resp != nil {
					respCode = resp.Code
				}
				w.taskLog(mainTaskId, LevelWarn, "Batch %d/%d save attempt %d/%d failed: err=%v code=%d",
					batchIdx+1, totalBatches, attempt, maxBatchRetry, err, respCode)
				time.Sleep(time.Duration(attempt*2) * time.Second)
			}
		}

		respCode := 0
		respMsg := ""
		if resp != nil {
			respCode = resp.Code
			respMsg = resp.Msg
		}
		// 成功：累加计数
		if err == nil && respCode == 0 {
			totalNew += resp.NewAsset
			totalUpdate += resp.UpdateAsset
			w.taskLog(mainTaskId, LevelDebug, "Batch %d/%d saved: new=%d, update=%d", batchIdx+1, totalBatches, resp.NewAsset, resp.UpdateAsset)
			continue
		}
		// 业务拒绝（参数/校验错误，code=400）为非瞬时错误：不重试、不入队，避免永久积压
		if respCode == 400 {
			w.taskLog(mainTaskId, LevelError, "Batch %d/%d save rejected by API (code=400), dropped: err=%v msg=%s",
				batchIdx+1, totalBatches, err, respMsg)
			continue
		}
		// 瞬时失败（传输错误或服务端 5xx）：入队，由 replayLoop 在 API 恢复后重放
		if w.resultQueue != nil {
			queueReq := &TaskResultReq{
				WorkspaceId: workspaceId,
				MainTaskId:  mainTaskId,
				Assets:      httpAssets,
				OrgId:       orgId,
			}
			if queueErr := w.resultQueue.Enqueue(queueReq); queueErr != nil {
				w.taskLog(mainTaskId, LevelError, "Batch %d/%d save failed and queue failed: %v (queue error: %v)",
					batchIdx+1, totalBatches, maxBatchRetry, err, queueErr)
			} else {
				w.taskLog(mainTaskId, LevelWarn, "Batch %d/%d save failed after %d attempts, queued for retry: %v",
					batchIdx+1, totalBatches, maxBatchRetry, err)
			}
		} else {
			w.taskLog(mainTaskId, LevelError, "Batch %d/%d save failed after %d attempts: %v",
				batchIdx+1, totalBatches, maxBatchRetry, err)
		}
	}

	w.taskLog(mainTaskId, LevelInfo, "Save completed: total=%d, new=%d, update=%d", totalAssets, totalNew, totalUpdate)
}

// respMsgOf 安全提取 BaseResp 的 Msg，避免 nil 解引用
func respMsgOf(resp *BaseResp) string {
	if resp == nil {
		return ""
	}
	return resp.Msg
}

// saveJSFinderResult 保存 JS 扫描结果（修复 P0：增加重试 + 本地队列回退）
// 原实现仅在调用处记一条错误日志即返回，API 抖动时结果永久丢失。
// 现在与资产/漏洞一致：失败重试 3 次，仍失败则落本地队列，API 恢复后由 replayLoop 重放。
func (w *Worker) saveJSFinderResult(ctx context.Context, workspaceId, mainTaskId string, results []*JSFinderResultItem) {
	if len(results) == 0 {
		return
	}

	req := &SaveJSFinderResultReq{
		WorkspaceId: workspaceId,
		MainTaskId:  mainTaskId,
		Results:     results,
	}

	const maxRetry = 3
	var err error
	var resp *BaseResp
	var saveErr error
	for attempt := 1; attempt <= maxRetry; attempt++ {
		batchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, saveErr = w.httpClient.SaveJSFinderResult(batchCtx, req)
		cancel()
		// 业务码 0 或传输错误为可重试；code=400 为参数/校验拒绝，属非瞬时错误
		respCode := 0
		if resp != nil {
			respCode = resp.Code
		}
		if saveErr == nil && respCode == 0 {
			break
		}
		err = saveErr
		if respCode == 400 {
			w.taskLog(mainTaskId, LevelWarn, "[JSFinder] save rejected by API (code=400): %s", respMsgOf(resp))
			return
		}
		if attempt < maxRetry {
			w.taskLog(mainTaskId, LevelWarn, "[JSFinder] save attempt %d/%d failed: err=%v code=%d", attempt, maxRetry, err, respCode)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	respCode := 0
	if resp != nil {
		respCode = resp.Code
	}
	if err == nil && respCode == 0 {
		w.taskLog(mainTaskId, LevelInfo, "JSFinder completed: saved %d findings", len(results))
		return
	}
	// 非瞬时拒绝
	if respCode == 400 {
		w.taskLog(mainTaskId, LevelError, "[JSFinder] save rejected by API (code=400), dropped: %s", respMsgOf(resp))
		return
	}
	// 瞬时失败：入队重放
	if w.resultQueue != nil {
		if qErr := w.resultQueue.EnqueueJS(req); qErr != nil {
			w.taskLog(mainTaskId, LevelError, "[JSFinder] save failed after %d attempts and queue failed: %v (queue error: %v)", maxRetry, err, qErr)
		} else {
			w.taskLog(mainTaskId, LevelWarn, "[JSFinder] save failed after %d attempts, queued for retry: %v", maxRetry, err)
		}
	} else {
		w.taskLog(mainTaskId, LevelError, "[JSFinder] save failed after %d attempts: %v", maxRetry, err)
	}
}

// saveVulResult 保存漏洞结果（支持去重与聚合）
func (w *Worker) saveVulResult(ctx context.Context, workspaceId, mainTaskId string, vuls []*scanner.Vulnerability) {
	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("Save vulnerability result panic recovered: %v, stack: %s", r, string(getStackTrace()))
		}
	}()

	if len(vuls) == 0 {
		return
	}

	w.taskLog(mainTaskId, LevelInfo, "[SaveVul] Starting to save %d vulnerabilities via HTTP API", len(vuls))

	httpVuls := make([]VulDocument, 0, len(vuls))
	for _, vul := range vuls {
		// Debug: 打印扫描层与发送层的关键字段
		w.taskLog(mainTaskId, LevelDebug, "[SaveVul] scanner.PocFile=%s, scanner.VulName=%q, scanner.Tags=%v, CurlCommand len=%d, Request len=%d, Response len=%d",
			vul.PocFile, vul.VulName, vul.Tags, len(vul.CurlCommand), len(vul.Request), len(vul.Response))

		httpVul := ToVulDocument(vul, mainTaskId)
		httpVulName := ""
		if httpVul.VulName != nil {
			httpVulName = *httpVul.VulName
		}

		// 输出httpVul中的关键字段
		w.taskLog(mainTaskId, LevelDebug, "[SaveVul] httpVul.VulName.nil=%v, httpVul.VulName=%q, httpVul.Tags=%v, httpVul.CurlCommand=%v, httpVul.Request=%v, httpVul.Response=%v",
			httpVul.VulName == nil, httpVulName, httpVul.Tags, httpVul.CurlCommand != nil, httpVul.Request != nil, httpVul.Response != nil)

		httpVuls = append(httpVuls, httpVul)
	}

	w.taskLog(mainTaskId, LevelInfo, "[SaveVul] Calling HTTP API to save %d vulnerabilities, workspaceId=%s", len(httpVuls), workspaceId)

	// 修复 C-02：与 saveAssetResult 对称，增加重试 + 本地队列回退，避免 API 不可用时漏洞永久丢失
	const maxVulRetry = 3
	var resp *VulResultResp
	var err error
	for attempt := 1; attempt <= maxVulRetry; attempt++ {
		batchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, err = w.httpClient.SaveVulResult(batchCtx, &VulResultReq{
			WorkspaceId: workspaceId,
			MainTaskId:  mainTaskId,
			Vuls:        httpVuls,
		})
		cancel()
		if err == nil {
			break
		}
		if attempt < maxVulRetry {
			w.taskLog(mainTaskId, LevelWarn, "[SaveVul] save attempt %d/%d failed: %v", attempt, maxVulRetry, err)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	if err != nil {
		// API 不可用时，将漏洞结果入队到本地队列（复用 resultQueue，封装为 TaskResultReq 兼容格式）
		if w.resultQueue != nil {
			// 将漏洞序列化为 JSON 存入 TaskResultReq 的扩展字段不现实，
			// 改为直接落盘到独立的漏洞队列文件
			queueErr := w.enqueueVulResult(workspaceId, mainTaskId, httpVuls)
			if queueErr != nil {
				w.taskLog(mainTaskId, LevelError, "[SaveVul] save failed after %d attempts and queue failed: %v (queue error: %v)",
					maxVulRetry, err, queueErr)
			} else {
				w.taskLog(mainTaskId, LevelWarn, "[SaveVul] save failed after %d attempts, queued for retry: %v",
					maxVulRetry, err)
			}
		} else {
			w.taskLog(mainTaskId, LevelError, "[SaveVul] save failed after %d attempts: %v", maxVulRetry, err)
		}
		return
	}

	if resp != nil {
		w.taskLog(mainTaskId, LevelInfo, "[SaveVul] HTTP API response: success=%v, message=%s, total=%d", resp.Success, resp.Msg, resp.Total)
	} else {
		w.taskLog(mainTaskId, LevelWarn, "[SaveVul] HTTP API response is nil")
	}
}

// enqueueVulResult 将漏洞结果落盘到本地队列文件，供 API 恢复后重放
// 修复 C-02：避免 API 不可用时漏洞结果永久丢失
func (w *Worker) enqueueVulResult(workspaceId, mainTaskId string, vuls []VulDocument) error {
	if w.resultQueue == nil {
		return fmt.Errorf("result queue not initialized")
	}
	// 复用 resultQueue 的目录结构，漏洞结果用 "vul" 后缀区分
	queueReq := &TaskResultReq{
		WorkspaceId: workspaceId,
		MainTaskId:  mainTaskId,
	}
	// 将漏洞数据序列化后作为资产队列条目存入（resultQueue 内部按 JSON 文件持久化）
	// 这里用 mainTaskId 标记，重放时会调用 replayFn，由 replayFn 内部判断是否为漏洞
	// 简化方案：直接写入 resultQueue 的目录，文件名带 vul 标记
	return w.resultQueue.EnqueueVul(queueReq, vuls)
}

// reportResultLoop 上报结果循环（内部方法）
func (w *Worker) reportResultLoop() {
	for {
		select {
		case <-w.stopChan:
			return
		case result := <-w.resultChan:
			w.handleResult(result)
		}
	}
}

// handleResult 处理结果
func (w *Worker) handleResult(result *scanner.ScanResult) {
	ctx := context.Background()
	w.saveAssetResult(ctx, result.WorkspaceId, result.MainTaskId, "", result.Assets)
	w.saveVulResult(ctx, result.WorkspaceId, result.MainTaskId, result.Vulnerabilities)
	// 证书采集结果（指纹识别附加产出）经 HTTP API 批量 upsert
	if len(result.CertResults) > 0 {
		w.saveCertResults(ctx, result.WorkspaceId, result.MainTaskId, result.CertResults)
	}
}

// saveCertResults 将指纹识别阶段附加产出的证书结果经 HTTP API 落库。
func (w *Worker) saveCertResults(ctx context.Context, workspaceId, mainTaskId string, certs []*scanner.CertResult) {
	if len(certs) == 0 {
		return
	}
	items := make([]*CertResultItem, 0, len(certs))
	for _, c := range certs {
		items = append(items, &CertResultItem{
			Host:         c.Host,
			Port:         c.Port,
			Authority:    c.Authority,
			Subject:      c.Subject,
			SubjectDN:    c.SubjectDN,
			Issuer:       c.Issuer,
			IssuerDN:     c.IssuerDN,
			SerialNumber: c.SerialNumber,
			SigAlg:       c.SigAlg,
			NotBefore:    c.NotBefore,
			NotAfter:     c.NotAfter,
			Version:      c.Version,
			SANs:         c.SANs,
			Fingerprints: c.Fingerprints,
			IsSelfSigned: c.IsSelfSigned,
		})
	}

	req := &SaveCertResultReq{
		WorkspaceId: workspaceId,
		MainTaskId:  mainTaskId,
		Results:     items,
	}

	// 修复 P0：增加重试 + 本地队列回退，与资产/漏洞/JS 对称
	const maxRetry = 3
	var err error
	var resp *BaseResp
	var saveErr error
	for attempt := 1; attempt <= maxRetry; attempt++ {
		batchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, saveErr = w.httpClient.SaveCertResult(batchCtx, req)
		cancel()
		respCode := 0
		if resp != nil {
			respCode = resp.Code
		}
		if saveErr == nil && respCode == 0 {
			break
		}
		err = saveErr
		if respCode == 400 {
			w.taskLog(mainTaskId, LevelWarn, "[CertResult] save rejected by API (code=400): %s", respMsgOf(resp))
			return
		}
		if attempt < maxRetry {
			w.taskLog(mainTaskId, LevelWarn, "[CertResult] save attempt %d/%d failed: err=%v code=%d", attempt, maxRetry, err, respCode)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	respCode := 0
	if resp != nil {
		respCode = resp.Code
	}
	if err == nil && respCode == 0 {
		w.taskLog(mainTaskId, LevelInfo, "CertResult: saved %d certificates", len(certs))
		return
	}
	if respCode == 400 {
		w.taskLog(mainTaskId, LevelError, "[CertResult] save rejected by API (code=400), dropped: %s", respMsgOf(resp))
		return
	}
	// 瞬时失败：入队重放
	if w.resultQueue != nil {
		if qErr := w.resultQueue.EnqueueCert(req); qErr != nil {
			w.taskLog(mainTaskId, LevelError, "[CertResult] save failed after %d attempts and queue failed: %v (queue error: %v)", maxRetry, err, qErr)
		} else {
			w.taskLog(mainTaskId, LevelWarn, "[CertResult] save failed after %d attempts, queued for retry: %v", maxRetry, err)
		}
	} else {
		w.taskLog(mainTaskId, LevelError, "[CertResult] save failed after %d attempts: %v", maxRetry, err)
	}
}

// keepAliveLoop 心跳循环（内部方法）
// 心跳使用独立协程，不受扫描任务阻塞影响
