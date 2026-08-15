package worker

import (
	"context"
	"fmt"

	"cscan/internal/model"
	"cscan/internal/scanner"

	"github.com/zeromicro/go-zero/core/logx"
)

// saveAssetResultWithFallback 先直写 MongoDB，失败后将完整请求持久化到本地队列。
func (w *Worker) saveAssetResultWithFallback(ctx context.Context, mainTaskID, orgID string, assets []*scanner.Asset) error {
	if len(assets) == 0 {
		return nil
	}
	if err := w.saveAssetResultDirect(ctx, mainTaskID, orgID, assets); err == nil {
		return nil
	} else {
		if w.resultQueue == nil {
			return fmt.Errorf("save assets to MongoDB failed: %v; result queue is unavailable", err)
		}
		queueErr := w.resultQueue.Enqueue(&TaskResultReq{
			MainTaskId:  mainTaskID,
			OrgId:       orgID,
			Assets:      scannerAssetsToDocuments(assets),
			IsFinalSave: false,
		})
		if queueErr != nil {
			return fmt.Errorf("save assets to MongoDB failed: %v; queue fallback failed: %w", err, queueErr)
		}
		w.taskLog(mainTaskID, LevelWarn, "MongoDB asset save failed; queued %d assets for replay: %v", len(assets), err)
		return nil
	}
}

// saveVulResultWithFallback 先直写 MongoDB，失败后将漏洞请求持久化到本地队列。
func (w *Worker) saveVulResultWithFallback(ctx context.Context, mainTaskID string, vuls []*scanner.Vulnerability) error {
	if len(vuls) == 0 {
		return nil
	}
	if err := w.saveVulResultDirect(ctx, mainTaskID, vuls); err == nil {
		return nil
	} else {
		req := &VulResultReq{MainTaskId: mainTaskID, Vuls: make([]VulDocument, 0, len(vuls))}
		for _, vul := range vuls {
			req.Vuls = append(req.Vuls, ToVulDocument(vul, mainTaskID))
		}
		if w.resultQueue == nil {
			return fmt.Errorf("save vulnerabilities to MongoDB failed: %w", err)
		}
		if queueErr := w.resultQueue.EnqueueVul(&TaskResultReq{MainTaskId: mainTaskID}, req.Vuls); queueErr != nil {
			return fmt.Errorf("save vulnerabilities to MongoDB failed: %v; queue fallback failed: %w", err, queueErr)
		}
		w.taskLog(mainTaskID, LevelWarn, "MongoDB vulnerability save failed; queued %d vulnerabilities for replay: %v", len(vuls), err)
		return nil
	}
}

// saveCertResultsWithFallback 先直写 MongoDB，失败后将证书请求持久化到本地队列。
func (w *Worker) saveCertResultsWithFallback(ctx context.Context, mainTaskID string, certs []*scanner.CertResult) error {
	if len(certs) == 0 {
		return nil
	}
	if err := w.saveCertResultsDirect(ctx, mainTaskID, certs); err == nil {
		return nil
	} else {
		req := &SaveCertResultReq{MainTaskId: mainTaskID, Results: make([]*CertResultItem, 0, len(certs))}
		for _, cert := range certs {
			req.Results = append(req.Results, certResultToItem(cert))
		}
		if w.resultQueue == nil {
			return fmt.Errorf("save certificates to MongoDB failed: %w", err)
		}
		if queueErr := w.resultQueue.EnqueueCert(req); queueErr != nil {
			return fmt.Errorf("save certificates to MongoDB failed: %v; queue fallback failed: %w", err, queueErr)
		}
		w.taskLog(mainTaskID, LevelWarn, "MongoDB certificate save failed; queued %d certificates for replay: %v", len(certs), err)
		return nil
	}
}

// saveJSFinderResultWithFallback 先直写 MongoDB，失败后将 JS 结果持久化到本地队列。
func (w *Worker) saveJSFinderResultWithFallback(ctx context.Context, mainTaskID string, results []*JSFinderResultItem) error {
	if len(results) == 0 {
		return nil
	}
	if err := w.saveJSFinderResultDirect(ctx, mainTaskID, results); err == nil {
		return nil
	} else {
		req := &SaveJSFinderResultReq{MainTaskId: mainTaskID, Results: results}
		if w.resultQueue == nil {
			return fmt.Errorf("save JSFinder results to MongoDB failed: %w", err)
		}
		if queueErr := w.resultQueue.EnqueueJS(req); queueErr != nil {
			return fmt.Errorf("save JSFinder results to MongoDB failed: %v; queue fallback failed: %w", err, queueErr)
		}
		w.taskLog(mainTaskID, LevelWarn, "MongoDB JSFinder save failed; queued %d findings for replay: %v", len(results), err)
		return nil
	}
}

// replayAssetResult 从本地队列重放资产结果到 MongoDB（回放路径不带 fallback，避免二次入队）。
func (w *Worker) replayAssetResult(ctx context.Context, req *TaskResultReq) error {
	if w.mongoDB == nil {
		return fmt.Errorf("mongoDB unavailable; asset replay requires direct MongoDB connection")
	}
	if len(req.Assets) == 0 {
		return nil
	}
	assets := make([]*model.ScannerAsset, len(req.Assets))
	for i := range req.Assets {
		assets[i] = assetDocumentToScannerAsset(&req.Assets[i])
	}
	svc := model.NewAssetWriteService(w.mongoDB)
	result, err := svc.SaveAssets(ctx, req.MainTaskId, req.OrgId, assets)
	if err != nil {
		return err
	}
	w.taskLog(req.MainTaskId, LevelInfo, "[Replay] assets saved: total=%d, new=%d, update=%d",
		result.TotalAsset, result.NewAsset, result.UpdateAsset)
	return nil
}

func assetDocumentToScannerAsset(doc *AssetDocument) *model.ScannerAsset {
	ipv4 := make([]model.ScannerIPInfo, len(doc.Ipv4))
	for i, ip := range doc.Ipv4 {
		ipv4[i] = model.ScannerIPInfo{IP: ip.IP, Location: ip.Location}
	}
	ipv6 := make([]model.ScannerIPInfo, len(doc.Ipv6))
	for i, ip := range doc.Ipv6 {
		ipv6[i] = model.ScannerIPInfo{IP: ip.IP, Location: ip.Location}
	}
	return &model.ScannerAsset{
		Authority:  doc.Authority,
		Host:       doc.Host,
		Port:       int(doc.Port),
		Category:   doc.Category,
		Service:    doc.Service,
		Server:     doc.Server,
		Banner:     doc.Banner,
		Title:      doc.Title,
		App:        doc.App,
		HttpStatus: doc.HttpStatus,
		HttpHeader: doc.HttpHeader,
		HttpBody:   doc.HttpBody,
		Cert:       doc.Cert,
		IconHash:   doc.IconHash,
		IsCDN:      doc.IsCdn,
		CName:      doc.Cname,
		IsCloud:    doc.IsCloud,
		IsHTTP:     doc.IsHttp,
		IPV4:       ipv4,
		IPV6:       ipv6,
		Screenshot: doc.Screenshot,
		Source:     doc.Source,
		IconData:   doc.IconData,
	}
}

func certResultToItem(cert *scanner.CertResult) *CertResultItem {
	return &CertResultItem{
		Host: cert.Host, Port: cert.Port, Authority: cert.Authority, Subject: cert.Subject,
		SubjectDN: cert.SubjectDN, Issuer: cert.Issuer, IssuerDN: cert.IssuerDN,
		SerialNumber: cert.SerialNumber, SigAlg: cert.SigAlg, NotBefore: cert.NotBefore,
		NotAfter: cert.NotAfter, Version: cert.Version, SANs: cert.SANs,
		Fingerprints: cert.Fingerprints, IsSelfSigned: cert.IsSelfSigned,
	}
}

func scannerAssetsToDocuments(assets []*scanner.Asset) []AssetDocument {
	documents := make([]AssetDocument, 0, len(assets))
	for _, asset := range assets {
		documents = append(documents, AssetDocument{
			Authority: asset.Authority, Host: asset.Host, Port: int32(asset.Port), Category: asset.Category,
			Service: asset.Service, Server: asset.Server, Banner: asset.Banner, Title: asset.Title,
			App: asset.App, HttpStatus: asset.HttpStatus, HttpHeader: asset.HttpHeader, HttpBody: asset.HttpBody,
			IconHash: asset.IconHash, IsCdn: asset.IsCDN, Cname: asset.CName, IsCloud: asset.IsCloud,
			Screenshot: asset.Screenshot, IsHttp: asset.IsHTTP, Source: asset.Source, IconData: asset.IconData,
		})
	}
	return documents
}

func vulDocumentToScanner(doc *VulDocument) (*scanner.Vulnerability, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil vulnerability document")
	}
	vul := &scanner.Vulnerability{
		Authority: doc.Authority, Host: doc.Host, Port: int(doc.Port), Url: doc.Url,
		PocFile: doc.PocFile, Source: doc.Source, Severity: doc.Severity,
		Extra: doc.Extra, Result: doc.Result, Tags: doc.Tags,
		ExtractedResults: doc.ExtractedResults, ResponseTruncated: valueOrFalse(doc.ResponseTruncated),
	}
	if doc.VulName != nil {
		vul.VulName = *doc.VulName
	}
	if doc.CvssScore != nil {
		vul.CvssScore = *doc.CvssScore
	}
	if doc.CveId != nil {
		vul.CveId = *doc.CveId
	}
	if doc.CweId != nil {
		vul.CweId = *doc.CweId
	}
	if doc.Remediation != nil {
		vul.Remediation = *doc.Remediation
	}
	if doc.References != nil {
		vul.References = doc.References
	}
	if doc.MatcherName != nil {
		vul.MatcherName = *doc.MatcherName
	}
	if doc.CurlCommand != nil {
		vul.CurlCommand = *doc.CurlCommand
	}
	if doc.Request != nil {
		vul.Request = *doc.Request
	}
	if doc.Response != nil {
		vul.Response = *doc.Response
	}
	return vul, nil
}

func valueOrFalse(value *bool) bool {
	return value != nil && *value
}

// saveAssetResultDirect 将扫描资产直接写入 MongoDB
func (w *Worker) saveAssetResultDirect(ctx context.Context, mainTaskID, orgID string, assets []*scanner.Asset) error {
	if w.mongoDB == nil || len(assets) == 0 {
		return nil
	}

	scannerAssets := make([]*model.ScannerAsset, len(assets))
	for i, asset := range assets {
		scannerAssets[i] = scannerAssetToDTO(asset)
	}

	svc := model.NewAssetWriteService(w.mongoDB)
	result, err := svc.SaveAssets(ctx, mainTaskID, orgID, scannerAssets)
	if err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] SaveAssets failed: %v", err)
		return err
	}

	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] assets saved: total=%d, new=%d, update=%d",
		result.TotalAsset, result.NewAsset, result.UpdateAsset)
	return nil
}

// saveVulResultDirect 将漏洞结果直接写入 MongoDB
func (w *Worker) saveVulResultDirect(ctx context.Context, mainTaskID string, vuls []*scanner.Vulnerability) error {
	if w.mongoDB == nil || len(vuls) == 0 {
		return nil
	}

	scannerVuls := make([]*model.ScannerVulnerability, len(vuls))
	for i, vul := range vuls {
		scannerVuls[i] = scannerVulToDTO(vul)
	}

	svc := model.NewVulWriteService(w.mongoDB)
	result, err := svc.SaveVuls(ctx, mainTaskID, scannerVuls)
	if err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] SaveVuls failed: %v", err)
		return err
	}

	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] vuls saved: total=%d, new=%d",
		result.SavedCount, result.NewVulCount)
	return nil
}

// saveCertResultsDirect 将证书结果直接写入 MongoDB
func (w *Worker) saveCertResultsDirect(ctx context.Context, mainTaskID string, certs []*scanner.CertResult) error {
	if w.mongoDB == nil || len(certs) == 0 {
		return nil
	}

	scannerCerts := make([]*model.ScannerCert, len(certs))
	for i, c := range certs {
		scannerCerts[i] = &model.ScannerCert{
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
		}
	}

	svc := model.NewCertWriteService(w.mongoDB)
	if err := svc.SaveCerts(ctx, mainTaskID, scannerCerts); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] SaveCerts failed: %v", err)
		return err
	}

	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] certs saved: %d certificates", len(certs))
	return nil
}

// saveJSFinderResultDirect 将 JSFinder 结果直接写入 MongoDB
func (w *Worker) saveJSFinderResultDirect(ctx context.Context, mainTaskID string, results []*JSFinderResultItem) error {
	if w.mongoDB == nil || len(results) == 0 {
		return nil
	}

	scannerResults := make([]*model.ScannerJSFinderResult, len(results))
	for i, r := range results {
		scannerResults[i] = &model.ScannerJSFinderResult{
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
		}
	}

	svc := model.NewJSFinderWriteService(w.mongoDB)
	if err := svc.SaveResults(ctx, mainTaskID, scannerResults); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] SaveJSFinderResult failed: %v", err)
		return err
	}

	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] JS results saved: %d findings", len(results))
	return nil
}

// saveDirScanResultsDirect 将目录扫描结果直接写入 MongoDB
func (w *Worker) saveDirScanResultsDirect(ctx context.Context, mainTaskID string, results []DirScanResultDocument) error {
	if w.mongoDB == nil || len(results) == 0 {
		return nil
	}

	scannerResults := make([]*model.ScannerDirScanResult, len(results))
	for i, r := range results {
		scannerResults[i] = &model.ScannerDirScanResult{
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
		}
	}

	svc := model.NewDirScanWriteService(w.mongoDB)
	if err := svc.SaveResults(ctx, mainTaskID, scannerResults); err != nil {
		w.taskLog(mainTaskID, LevelError, "[MongoDirect] SaveDirScanResults failed: %v", err)
		return err
	}

	w.taskLog(mainTaskID, LevelInfo, "[MongoDirect] dir scan results saved: %d paths", len(results))
	return nil
}

// updateExecutorTaskDirect 直接更新 MongoDB 中的 executor_task 状态/结果
func (w *Worker) updateExecutorTaskDirect(ctx context.Context, taskID, state, result string) {
	if w.mongoDB == nil {
		return
	}

	executorTaskModel := model.NewExecutorTaskModel(w.mongoDB)
	update := map[string]interface{}{
		"status": state,
		"result": result,
	}
	if err := executorTaskModel.UpdateByTaskId(ctx, taskID, update); err != nil {
		logx.Errorf("[MongoDirect] updateExecutorTaskDirect failed: taskId=%s err=%v", taskID, err)
	}
}

// scannerAssetToDTO 将 scanner.Asset 转换为 model.ScannerAsset DTO
func scannerAssetToDTO(asset *scanner.Asset) *model.ScannerAsset {
	dto := &model.ScannerAsset{
		Authority:  asset.Authority,
		Host:       asset.Host,
		Port:       asset.Port,
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
		IsCDN:      asset.IsCDN,
		CName:      asset.CName,
		IsCloud:    asset.IsCloud,
		IsHTTP:     asset.IsHTTP,
		Source:     asset.Source,
	}

	for _, ip := range asset.IPV4 {
		dto.IPV4 = append(dto.IPV4, model.ScannerIPInfo{
			IP:       ip.IP,
			Location: ip.Location,
		})
	}

	for _, ip := range asset.IPV6 {
		dto.IPV6 = append(dto.IPV6, model.ScannerIPInfo{
			IP:       ip.IP,
			Location: ip.Location,
		})
	}

	return dto
}

// scannerVulToDTO 将 scanner.Vulnerability 转换为 model.ScannerVulnerability DTO
func scannerVulToDTO(vul *scanner.Vulnerability) *model.ScannerVulnerability {
	return &model.ScannerVulnerability{
		Authority:         vul.Authority,
		Host:              vul.Host,
		Port:              vul.Port,
		Url:               vul.Url,
		PocFile:           vul.PocFile,
		Source:            vul.Source,
		Severity:          vul.Severity,
		Result:            vul.Result,
		Extra:             vul.Extra,
		VulName:           vul.VulName,
		Tags:              vul.Tags,
		CvssScore:         vul.CvssScore,
		CveId:             vul.CveId,
		CweId:             vul.CweId,
		Remediation:       vul.Remediation,
		References:        vul.References,
		MatcherName:       vul.MatcherName,
		ExtractedResults:  vul.ExtractedResults,
		CurlCommand:       vul.CurlCommand,
		Request:           vul.Request,
		Response:          vul.Response,
		ResponseTruncated: vul.ResponseTruncated,
	}
}
