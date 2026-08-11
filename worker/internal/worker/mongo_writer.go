package worker

import (
	"context"
	"time"

	"cscan/internal/model"
	"cscan/internal/scanner"
	"cscan/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

// saveAssetResultDirect 将扫描资产直接写入 MongoDB
func (w *Worker) saveAssetResultDirect(ctx context.Context, workspaceID, mainTaskID, orgID string, assets []*scanner.Asset) {
	if w.mongoDB == nil || len(assets) == 0 {
		return
	}

	assetModel := model.NewAssetModel(w.mongoDB, workspaceID)
	var newCount, updateCount int32
	for _, asset := range assets {
		modelAsset := scannerAssetToModel(asset, mainTaskID, orgID)
		res, err := assetModel.UpsertWithResult(ctx, modelAsset)
		if err != nil {
			w.taskLog(mainTaskID, LevelError, "[MongoDirect] upsert asset authority=%s failed: %v", modelAsset.Authority, err)
			continue
		}
		if res != nil && res.IsNew {
			newCount++
		} else {
			updateCount++
		}
	}
	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] assets saved: total=%d, new=%d, update=%d", len(assets), newCount, updateCount)
}

// saveVulResultDirect 将漏洞结果直接写入 MongoDB
func (w *Worker) saveVulResultDirect(ctx context.Context, workspaceID, mainTaskID string, vuls []*scanner.Vulnerability) {
	if w.mongoDB == nil || len(vuls) == 0 {
		return
	}

	vulModel := model.NewVulModel(w.mongoDB, workspaceID)
	for _, vul := range vuls {
		doc := scannerVulToModel(vul, mainTaskID)
		if err := vulModel.Insert(ctx, doc); err != nil {
			w.taskLog(mainTaskID, LevelError, "[MongoDirect] saveVulResult insert failed: %v", err)
		}
	}
}

// saveCertResultsDirect 将证书结果直接写入 MongoDB
func (w *Worker) saveCertResultsDirect(ctx context.Context, workspaceID, mainTaskID string, certs []*scanner.CertResult) {
	if w.mongoDB == nil || len(certs) == 0 {
		return
	}

	certModel := model.NewCertModel(w.mongoDB, workspaceID)
	items := make([]*model.Cert, 0, len(certs))
	now := time.Now()
	for _, c := range certs {
		items = append(items, &model.Cert{
			WorkspaceId:  workspaceID,
			TaskId:       mainTaskID,
			Host:         c.Host,
			Port:         c.Port,
			Authority:    c.Authority,
			Subject:      model.CertNameInfo(c.Subject),
			SubjectDN:    c.SubjectDN,
			Issuer:       model.CertNameInfo(c.Issuer),
			IssuerDN:     c.IssuerDN,
			SerialNumber: c.SerialNumber,
			SigAlg:       c.SigAlg,
			NotBefore:    c.NotBefore,
			NotAfter:     c.NotAfter,
			Version:      c.Version,
			SANs:         c.SANs,
			Fingerprints: c.Fingerprints,
			IsSelfSigned: c.IsSelfSigned,
			CreateTime:   now,
			UpdateTime:   now,
		})
	}
	if err := certModel.UpsertMany(ctx, items); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] saveCertResults failed: %v", err)
	}
}

// saveJSFinderResultDirect 将 JSFinder 结果直接写入 MongoDB
func (w *Worker) saveJSFinderResultDirect(ctx context.Context, workspaceID, mainTaskID string, results []*JSFinderResultItem) {
	if w.mongoDB == nil || len(results) == 0 {
		return
	}

	jsModel := model.NewJSFinderResultModel(w.mongoDB, workspaceID)
	items := make([]*model.JSFinderResult, 0, len(results))
	now := time.Now()
	for _, r := range results {
		items = append(items, &model.JSFinderResult{
			WorkspaceId:      workspaceID,
			MainTaskId:       mainTaskID,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
			CreateTime:       now,
			UpdateTime:       now,
		})
	}
	if err := jsModel.UpsertMany(ctx, items); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] saveJSFinderResult failed: %v", err)
	}
}

// saveDirScanResultsDirect 将目录扫描结果直接写入 MongoDB
func (w *Worker) saveDirScanResultsDirect(ctx context.Context, workspaceID, mainTaskID string, results []DirScanResultDocument) {
	if w.mongoDB == nil || len(results) == 0 {
		return
	}

	dirModel := model.NewDirScanResultModel(w.mongoDB, workspaceID)
	docs := make([]*model.DirScanResult, 0, len(results))
	now := time.Now()
	for _, r := range results {
		docs = append(docs, &model.DirScanResult{
			WorkspaceId:   workspaceID,
			MainTaskId:    mainTaskID,
			Authority:     r.Authority,
			Host:          r.Host,
			Port:          r.Port,
			URL:           r.URL,
			Path:          r.Path,
			StatusCode:    r.StatusCode,
			ContentLength: r.ContentLength,
			ContentType:   r.ContentType,
			Title:         r.Title,
			RedirectURL:   r.RedirectURL,
			ContentWords:  r.ContentWords,
			ContentLines:  r.ContentLines,
			Duration:      r.Duration,
			Request:       r.Request,
			Response:      r.Response,
			CreateTime:    now,
			UpdateTime:    now,
			ScanTime:      now,
			Version:       1,
		})
	}
	if err := dirModel.InsertMany(ctx, docs); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] saveDirScanResults failed: %v", err)
	}
}

// updateExecutorTaskDirect 直接更新 MongoDB 中的 executor_task 状态/结果
func (w *Worker) updateExecutorTaskDirect(ctx context.Context, workspaceID, taskID, state, result string) {
	if w.mongoDB == nil {
		return
	}

	executorTaskModel := model.NewExecutorTaskModel(w.mongoDB, workspaceID)
	update := map[string]interface{}{
		"status": state,
		"result": result,
	}
	if err := executorTaskModel.UpdateByTaskId(ctx, taskID, update); err != nil {
		logx.Errorf("[MongoDirect] updateExecutorTaskDirect failed: taskId=%s err=%v", taskID, err)
	}
}

// scannerAssetToModel 将 scanner.Asset 转换为 model.Asset
// 字段映射与原 RPC 侧 SaveTaskResult 保持一致（rpc/task/internal/logic/savetaskresultlogic.go）
func scannerAssetToModel(asset *scanner.Asset, mainTaskID, orgID string) *model.Asset {
	doc := &model.Asset{
		Authority:     asset.Authority,
		Host:          asset.Host,
		Port:          asset.Port,
		Category:      asset.Category,
		Service:       asset.Service,
		Title:         asset.Title,
		App:           asset.App,
		HttpStatus:    asset.HttpStatus,
		HttpHeader:    asset.HttpHeader,
		HttpBody:      asset.HttpBody,
		Cert:          asset.Cert,
		IconHash:      asset.IconHash,
		IconHashBytes: asset.IconData,
		Screenshot:    asset.Screenshot,
		Server:        asset.Server,
		Banner:        asset.Banner,
		IsCDN:         asset.IsCDN,
		CName:         asset.CName,
		IsCloud:       asset.IsCloud,
		IsHTTP:        asset.IsHTTP,
		TaskId:        mainTaskID,
		Source:        asset.Source,
		OrgId:         orgID,
	}

	if doc.Source == "" {
		doc.Source = "scan"
	}

	// IP 信息：扫描器未提供时，若 Host 本身是 IP 则回填
	if len(asset.IPV4) > 0 {
		for _, ip := range asset.IPV4 {
			doc.Ip.IpV4 = append(doc.Ip.IpV4, model.IPV4{
				IPName:   ip.IP,
				Location: ip.Location,
			})
		}
	} else if utils.IsIPv4(asset.Host) {
		doc.Ip.IpV4 = append(doc.Ip.IpV4, model.IPV4{IPName: asset.Host})
	}

	if len(asset.IPV6) > 0 {
		for _, ip := range asset.IPV6 {
			doc.Ip.IpV6 = append(doc.Ip.IpV6, model.IPV6{
				IPName:   ip.IP,
				Location: ip.Location,
			})
		}
	} else if utils.IsIPv6(asset.Host) {
		doc.Ip.IpV6 = append(doc.Ip.IpV6, model.IPV6{IPName: asset.Host})
	}

	// Host 非 IP 时视为域名
	if asset.Category == "domain" || !utils.IsIPAddress(asset.Host) {
		doc.Domain = asset.Host
	}

	return doc
}

func scannerVulToModel(vul *scanner.Vulnerability, mainTaskID string) *model.Vul {
	doc := &model.Vul{
		Authority: vul.Authority,
		Host:      vul.Host,
		Port:      vul.Port,
		Url:       vul.Url,
		PocFile:   vul.PocFile,
		Source:    vul.Source,
		Severity:  vul.Severity,
		Result:    vul.Result,
		Extra:     vul.Extra,
		TaskId:    mainTaskID,
	}
	if vul.VulName != "" {
		doc.VulName = vul.VulName
	}
	if len(vul.Tags) > 0 {
		doc.Tags = vul.Tags
	}
	if vul.CvssScore > 0 {
		doc.CvssScore = vul.CvssScore
	}
	if vul.CveId != "" {
		doc.CveId = vul.CveId
	}
	if vul.CweId != "" {
		doc.CweId = vul.CweId
	}
	if vul.Remediation != "" {
		doc.Remediation = vul.Remediation
	}
	if len(vul.References) > 0 {
		doc.References = vul.References
	}
	if vul.MatcherName != "" {
		doc.MatcherName = vul.MatcherName
	}
	if len(vul.ExtractedResults) > 0 {
		doc.ExtractedResults = vul.ExtractedResults
	}
	if vul.CurlCommand != "" {
		doc.CurlCommand = vul.CurlCommand
	}
	if vul.Request != "" {
		doc.Request = vul.Request
	}
	if vul.Response != "" {
		doc.Response = vul.Response
	}
	doc.ResponseTruncated = vul.ResponseTruncated
	return doc
}
