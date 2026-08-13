package worker

import (
	"context"

	"cscan/internal/model"
	"cscan/internal/scanner"

	"github.com/zeromicro/go-zero/core/logx"
)

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
