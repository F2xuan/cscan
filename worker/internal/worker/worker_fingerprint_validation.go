package worker

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"cscan/internal/model"
	"cscan/internal/scanner"
	"cscan/internal/scheduler"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== 指纹验证任务处理 ====================

// FingerprintValidationResult 指纹验证结果
type FingerprintValidationResult struct {
	Matched      bool            `json:"matched"`
	Details      string          `json:"details"`
	MatchedList  []string        `json:"matchedList,omitempty"`
	MatchedInfos []MatchedFpInfo `json:"matchedInfos,omitempty"` // 批量验证匹配详情
	TotalScanned int             `json:"totalScanned,omitempty"` // 批量验证扫描总数
	StatusCode   int             `json:"statusCode,omitempty"`
	Error        string          `json:"error,omitempty"`
	Path         string          `json:"path,omitempty"`        // 主动指纹探测路径
	MatchedRule  string          `json:"matchedRule,omitempty"` // 主动指纹匹配的规则名
	PathResults  []PathResult    `json:"pathResults,omitempty"` // 主动指纹各路径结果
}

// MatchedFpInfo 批量验证中匹配的指纹信息
type MatchedFpInfo struct {
	Id                string `json:"id"`
	Name              string `json:"name"`
	IsBuiltin         bool   `json:"isBuiltin"`
	IsActive          bool   `json:"isActive"`
	MatchedConditions string `json:"matchedConditions"`
}

// PathResult 主动指纹单个路径的验证结果
type PathResult struct {
	Path           string `json:"path"`
	StatusCode     int    `json:"statusCode"`
	Matched        bool   `json:"matched"`
	MatchedRule    string `json:"matchedRule,omitempty"`
	MatchedDetails string `json:"matchedDetails,omitempty"`
	Error          string `json:"error,omitempty"`
}

// executeFingerprintValidateTask 执行被动指纹验证任务
func (w *Worker) executeFingerprintValidateTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusStarted, "正在验证指纹...")

	url, _ := taskConfig["url"].(string)
	fpId, _ := taskConfig["fingerprintId"].(string)

	if url == "" || fpId == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "URL或指纹ID为空"})
		return
	}

	// 1. 获取目标数据（HTTP请求 + 指纹数据）
	data, err := w.fetchFingerprintDataForValidate(url)
	if err != nil {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "请求目标失败: " + err.Error()})
		return
	}

	// 2. 从服务端获取指纹列表（包含目标指纹）
	fpResp, err := w.httpClient.GetFingerprints(ctx, &FingerprintsReq{EnabledOnly: false})
	if err != nil || !fpResp.Success {
		errMsg := "获取指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	// 3. 找到目标指纹并匹配
	var result FingerprintValidationResult
	for _, doc := range fpResp.Fingerprints {
		if doc.Id == fpId {
			// 将文档转为 model.Fingerprint 格式
			fp := &model.Fingerprint{
				Name:    doc.Name,
				Rule:    doc.Rule,
				Source:  doc.Source,
				Enabled: true,
			}
			fp.Id, _ = primitive.ObjectIDFromHex(doc.Id)

			// 使用 wappalyzer 库检测（内置/wappalyzer来源）
			if doc.Source == "wappalyzer" || doc.IsBuiltin {
				wappalyzerClient := w.getWappalyzerClient()
				if wappalyzerClient != nil {
					apps := wappalyzerClient.Fingerprint(data.Headers, data.BodyBytes)
					fpNameLower := strings.ToLower(doc.Name)
					for app := range apps {
						if strings.ToLower(app) == fpNameLower {
							result = FingerprintValidationResult{
								Matched: true,
								Details: fmt.Sprintf("wappalyzergo 库检测匹配: %s", doc.Name),
							}
							break
						}
					}
				}
			}

			// 如果wappalyzer没匹配，使用自定义引擎（MatchWithId返回匹配的指纹列表）
			if !result.Matched {
				engine := scanner.NewCustomFingerprintEngine([]*model.Fingerprint{fp})
				matchedFps := engine.MatchWithId(data)
				matched := len(matchedFps) > 0
				details := "未匹配"
				if matched {
					var matchedNames []string
					for _, m := range matchedFps {
						matchedNames = append(matchedNames, m.Name)
					}
					details = fmt.Sprintf("自定义引擎匹配: %s", strings.Join(matchedNames, ", "))
				}
				result = FingerprintValidationResult{
					Matched: matched,
					Details: details,
				}
			}
			break
		}
	}

	duration := time.Since(startTime).Seconds()
	w.saveFingerprintValidationResult(ctx, task.TaskId, fmt.Sprintf("验证完成, 耗时%.2fs", duration), result)
}

// executeFingerprintBatchValidateTask 执行批量指纹验证任务
func (w *Worker) executeFingerprintBatchValidateTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	url, _ := taskConfig["url"].(string)
	scope, _ := taskConfig["scope"].(string)

	if url == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "URL不能为空"})
		return
	}

	w.taskLog(task.TaskId, LevelInfo, "Batch fingerprint validate: target=%s, scope=%s", url, scope)

	// 1. 获取目标数据（HTTP请求 + 指纹数据）
	data, err := w.fetchFingerprintDataForValidate(url)
	if err != nil {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "请求目标失败: " + err.Error()})
		return
	}

	// 2. 从服务端获取启用的指纹列表
	fpResp, err := w.httpClient.GetFingerprints(ctx, &FingerprintsReq{EnabledOnly: true})
	if err != nil || !fpResp.Success {
		errMsg := "获取指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	// 3. 根据 scope 过滤指纹
	var filteredFps []FingerprintDocument
	for _, doc := range fpResp.Fingerprints {
		switch scope {
		case "builtin":
			if doc.IsBuiltin {
				filteredFps = append(filteredFps, doc)
			}
		case "custom":
			if !doc.IsBuiltin {
				filteredFps = append(filteredFps, doc)
			}
		default: // "all" 或空
			filteredFps = append(filteredFps, doc)
		}
	}

	// 4. 批量匹配
	var matchedInfos []MatchedFpInfo
	var fpsToEngine []*model.Fingerprint

	for _, doc := range filteredFps {
		fp := &model.Fingerprint{
			Name:      doc.Name,
			Rule:      doc.Rule,
			Source:    doc.Source,
			IsBuiltin: doc.IsBuiltin,
			Enabled:   true,
		}
		fp.Id, _ = primitive.ObjectIDFromHex(doc.Id)
		fpsToEngine = append(fpsToEngine, fp)
	}

	// 使用 wappalyzer 库检测（复用单例客户端）
	wappalyzerApps := make(map[string]struct{})
	if wappalyzerClient := w.getWappalyzerClient(); wappalyzerClient != nil {
		apps := wappalyzerClient.Fingerprint(data.Headers, data.BodyBytes)
		for app := range apps {
			wappalyzerApps[strings.ToLower(app)] = struct{}{}
		}
	}

	// 逐个匹配
	for i, doc := range filteredFps {
		matched := false
		var matchedConditions []string

		// wappalyzer 检测
		if (doc.Source == "wappalyzer" || doc.IsBuiltin) && w.wappalyzerClient != nil {
			if _, ok := wappalyzerApps[strings.ToLower(doc.Name)]; ok {
				matched = true
				matchedConditions = append(matchedConditions, fmt.Sprintf("wappalyzergo 库检测匹配: %s", doc.Name))
			}
		}

		// 自定义引擎匹配
		if !matched && fpsToEngine[i].Rule != "" {
			engine := scanner.NewCustomFingerprintEngine([]*model.Fingerprint{fpsToEngine[i]})
			matchedFps := engine.MatchWithId(data)
			if len(matchedFps) > 0 {
				matched = true
				for _, m := range matchedFps {
					matchedConditions = append(matchedConditions, fmt.Sprintf("自定义规则匹配: %s", m.Name))
				}
			}
		}

		if matched {
			matchedInfos = append(matchedInfos, MatchedFpInfo{
				Id:                doc.Id,
				Name:              doc.Name,
				IsBuiltin:         doc.IsBuiltin,
				MatchedConditions: strings.Join(matchedConditions, "\n"),
			})
		}
	}

	duration := time.Since(startTime).Seconds()
	result := FingerprintValidationResult{
		Matched:      len(matchedInfos) > 0,
		Details:      fmt.Sprintf("验证完成，共检测 %d 个指纹，匹配 %d 个", len(filteredFps), len(matchedInfos)),
		MatchedInfos: matchedInfos,
		TotalScanned: len(filteredFps),
	}

	w.saveFingerprintValidationResult(ctx, task.TaskId, fmt.Sprintf("批量验证完成, 耗时%.2fs, 匹配%d/%d", duration, len(matchedInfos), len(filteredFps)), result)
}

// executeActiveFingerprintValidateTask 执行主动指纹验证任务
func (w *Worker) executeActiveFingerprintValidateTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusStarted, "正在验证主动指纹...")

	url, _ := taskConfig["url"].(string)
	activeFpId, _ := taskConfig["activeFpId"].(string)

	if url == "" || activeFpId == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "URL或主动指纹ID为空"})
		return
	}

	// 1. 获取主动指纹配置
	afpResp, err := w.httpClient.GetActiveFingerprints(ctx, false)
	if err != nil || !afpResp.Success {
		errMsg := "获取主动指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	var activeFp *ActiveFingerprintDocument
	for _, doc := range afpResp.Fingerprints {
		if doc.Id == activeFpId {
			activeFp = &doc
			break
		}
	}
	if activeFp == nil {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "主动指纹不存在"})
		return
	}

	// 2. 获取同名被动指纹（用于匹配规则）
	fpResp, err := w.httpClient.GetFingerprints(ctx, &FingerprintsReq{EnabledOnly: false})
	if err != nil || !fpResp.Success {
		errMsg := "获取被动指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	var passiveFps []*model.Fingerprint
	for _, doc := range fpResp.Fingerprints {
		if doc.Name == activeFp.Name {
			fp := &model.Fingerprint{
				Name:    doc.Name,
				Rule:    doc.Rule,
				Source:  doc.Source,
				Enabled: true,
			}
			fp.Id, _ = primitive.ObjectIDFromHex(doc.Id)
			passiveFps = append(passiveFps, fp)
		}
	}
	if len(passiveFps) == 0 {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{
			Error: fmt.Sprintf("未找到同名被动指纹 '%s'", activeFp.Name),
		})
		return
	}

	// 3. 解析基础URL
	baseUrl, scheme := extractBaseUrlWithSchemeForWorker(url)
	if baseUrl == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "无效的URL格式"})
		return
	}

	// 4. 遍历每个探测路径
	anyMatched := false
	var pathResults []PathResult
	client := w.createValidateHttpClientForWorker()

	for _, path := range activeFp.Paths {
		pr := PathResult{Path: path}

		resp, body, finalUrl, err := w.smartHttpRequestForWorker(client, baseUrl, path, scheme)
		if err != nil {
			pr.Error = err.Error()
			pathResults = append(pathResults, pr)
			continue
		}

		pr.StatusCode = resp.StatusCode

		// 提取标题
		title := ""
		titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
		if matches := titleRe.FindStringSubmatch(body); len(matches) > 1 {
			title = strings.TrimSpace(matches[1])
		}

		// 构建header字符串
		var headerStr strings.Builder
		for key, values := range resp.Header {
			for _, v := range values {
				headerStr.WriteString(key)
				headerStr.WriteString(": ")
				headerStr.WriteString(v)
				headerStr.WriteString("\n")
			}
		}

		data := &scanner.FingerprintData{
			Title:        title,
			Body:         body,
			BodyBytes:    []byte(body),
			Headers:      resp.Header,
			HeaderString: headerStr.String(),
			Server:       resp.Header.Get("Server"),
			URL:          finalUrl,
			Cookies:      resp.Header.Get("Set-Cookie"),
		}

		// 使用被动指纹规则匹配
		for _, fp := range passiveFps {
			engine := scanner.NewCustomFingerprintEngine([]*model.Fingerprint{fp})
			matchedFps := engine.MatchWithId(data)
			if len(matchedFps) > 0 {
				pr.Matched = true
				pr.MatchedRule = fp.Name
				pr.MatchedDetails = fmt.Sprintf("自定义引擎匹配: %s", fp.Name)
				anyMatched = true
				break
			}
		}

		if !pr.Matched {
			pr.MatchedDetails = "未匹配任何规则"
		}
		pathResults = append(pathResults, pr)
	}

	duration := time.Since(startTime).Seconds()

	// 构建匹配详情（用于API层填充MatchedConditions）
	details := ""
	if anyMatched {
		var matchedPaths []string
		for _, pr := range pathResults {
			if pr.Matched {
				matchedPaths = append(matchedPaths, fmt.Sprintf("路径[%s]匹配规则: %s", pr.Path, pr.MatchedRule))
			}
		}
		details = strings.Join(matchedPaths, "\n")
	}

	result := FingerprintValidationResult{
		Matched:     anyMatched,
		Details:     details,
		PathResults: pathResults,
	}

	w.saveFingerprintValidationResult(ctx, task.TaskId, fmt.Sprintf("主动指纹验证完成, 耗时%.2fs", duration), result)
}

// executeActiveFingerprintBatchValidateTask 执行批量主动指纹验证任务
func (w *Worker) executeActiveFingerprintBatchValidateTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusStarted, "正在批量验证主动指纹...")

	url, _ := taskConfig["url"].(string)
	if url == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "URL不能为空"})
		return
	}

	// 1. 获取启用的主动指纹列表
	afpResp, err := w.httpClient.GetActiveFingerprints(ctx, true)
	if err != nil || !afpResp.Success {
		errMsg := "获取主动指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	if len(afpResp.Fingerprints) == 0 {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "没有启用的主动指纹"})
		return
	}

	// 2. 获取被动指纹列表（用于匹配规则）
	fpResp, err := w.httpClient.GetFingerprints(ctx, &FingerprintsReq{EnabledOnly: false})
	if err != nil || !fpResp.Success {
		errMsg := "获取被动指纹列表失败"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: errMsg})
		return
	}

	// 3. 解析基础URL
	baseUrl, scheme := extractBaseUrlWithSchemeForWorker(url)
	if baseUrl == "" {
		w.saveFingerprintValidationResult(ctx, task.TaskId, "", FingerprintValidationResult{Error: "无效的URL格式"})
		return
	}

	// 4. 构建被动指纹名称映射
	passiveFpByName := make(map[string][]*model.Fingerprint)
	for _, doc := range fpResp.Fingerprints {
		fp := &model.Fingerprint{
			Name:    doc.Name,
			Rule:    doc.Rule,
			Source:  doc.Source,
			Enabled: true,
		}
		fp.Id, _ = primitive.ObjectIDFromHex(doc.Id)
		passiveFpByName[doc.Name] = append(passiveFpByName[doc.Name], fp)
	}

	client := w.createValidateHttpClientForWorker()
	var matchedInfos []MatchedFpInfo
	totalScanned := 0

	// 5. 遍历每个主动指纹
	for _, afp := range afpResp.Fingerprints {
		passiveFps, ok := passiveFpByName[afp.Name]
		if !ok || len(passiveFps) == 0 {
			continue
		}
		totalScanned++

		// 遍历每个探测路径
		fpMatched := false
		var matchedConds []string
		for _, path := range afp.Paths {
			resp, body, finalUrl, err := w.smartHttpRequestForWorker(client, baseUrl, path, scheme)
			if err != nil {
				continue
			}

			// 提取标题
			title := ""
			titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
			if matches := titleRe.FindStringSubmatch(body); len(matches) > 1 {
				title = strings.TrimSpace(matches[1])
			}

			// 构建header字符串
			var headerStr strings.Builder
			for key, values := range resp.Header {
				for _, v := range values {
					headerStr.WriteString(key)
					headerStr.WriteString(": ")
					headerStr.WriteString(v)
					headerStr.WriteString("\n")
				}
			}

			data := &scanner.FingerprintData{
				Title:        title,
				Body:         body,
				BodyBytes:    []byte(body),
				Headers:      resp.Header,
				HeaderString: headerStr.String(),
				Server:       resp.Header.Get("Server"),
				URL:          finalUrl,
				Cookies:      resp.Header.Get("Set-Cookie"),
			}

			// 匹配规则
			for _, fp := range passiveFps {
				engine := scanner.NewCustomFingerprintEngine([]*model.Fingerprint{fp})
				matchedFps := engine.MatchWithId(data)
				if len(matchedFps) > 0 {
					fpMatched = true
					matchedConds = append(matchedConds, fmt.Sprintf("路径 [%s] 匹配规则: %s", path, fp.Name))
					break
				}
			}
			if fpMatched {
				break
			}
		}

		if fpMatched {
			matchedInfos = append(matchedInfos, MatchedFpInfo{
				Id:                afp.Id,
				Name:              afp.Name,
				IsActive:          true,
				MatchedConditions: strings.Join(matchedConds, "\n"),
			})
		}
	}

	duration := time.Since(startTime).Seconds()
	result := FingerprintValidationResult{
		Matched:      len(matchedInfos) > 0,
		Details:      fmt.Sprintf("验证完成，共检测 %d 个主动指纹，匹配 %d 个", totalScanned, len(matchedInfos)),
		MatchedInfos: matchedInfos,
		TotalScanned: totalScanned,
	}

	w.saveFingerprintValidationResult(ctx, task.TaskId, fmt.Sprintf("主动指纹批量验证完成, 耗时%.2fs, 匹配%d/%d", duration, len(matchedInfos), totalScanned), result)
}

// saveFingerprintValidationResult 保存指纹验证结果（终态更新，包含worker字段，不应再调用updateTaskStatus覆盖）
func (w *Worker) saveFingerprintValidationResult(ctx context.Context, taskId, msg string, result FingerprintValidationResult) {
	resultData := map[string]interface{}{
		"taskId":     taskId,
		"status":     "SUCCESS",
		"result":     result,
		"updateTime": time.Now().Local().Format("2006-01-02 15:04:05"),
	}
	if result.Error != "" {
		resultData["status"] = "FAILURE"
		resultData["error"] = result.Error
	}

	resultJson, err := json.Marshal(resultData)
	if err != nil {
		w.taskLog(taskId, LevelError, "Failed to marshal fingerprint validation result: %v", err)
		return
	}

	status := scheduler.TaskStatusSuccess
	if result.Error != "" {
		status = scheduler.TaskStatusFailure
	}
	// 终态更新：包含 state、worker、result（JSON），不再由后续 updateTaskStatus 覆盖
	_, err = w.httpClient.UpdateTask(ctx, &TaskUpdateReq{
		TaskId: taskId,
		State:  status,
		Worker: w.config.Name,
		Result: string(resultJson),
	})
	if err != nil {
		w.taskLog(taskId, LevelError, "Failed to save fingerprint validation result: %v", err)
	}
}

// fetchFingerprintDataForValidate 从目标URL获取指纹数据
func (w *Worker) fetchFingerprintDataForValidate(targetUrl string) (*scanner.FingerprintData, error) {
	targetUrl = extractBaseUrlForWorker(targetUrl)

	w.logger.Info("[Fingerprint] HTTP GET %s", targetUrl)

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Get(targetUrl)
	if err != nil {
		w.logger.Warn("[Fingerprint] HTTP GET %s failed: %v", targetUrl, err)
		return nil, err
	}
	defer resp.Body.Close()
	w.logger.Info("[Fingerprint] HTTP %s -> %d %s (%dms)", targetUrl, resp.StatusCode, resp.Status, time.Since(start).Milliseconds())

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	title := ""
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
	if matches := titleRe.FindStringSubmatch(body); len(matches) > 1 {
		title = strings.TrimSpace(matches[1])
	}

	var headerStr strings.Builder
	for key, values := range resp.Header {
		for _, v := range values {
			headerStr.WriteString(key)
			headerStr.WriteString(": ")
			headerStr.WriteString(v)
			headerStr.WriteString("\n")
		}
	}

	faviconHash := w.fetchFaviconHashForWorker(targetUrl, body, client)

	return &scanner.FingerprintData{
		Title:        title,
		Body:         body,
		BodyBytes:    bodyBytes,
		Headers:      resp.Header,
		HeaderString: headerStr.String(),
		Server:       resp.Header.Get("Server"),
		URL:          targetUrl,
		FaviconHash:  faviconHash,
		Cookies:      resp.Header.Get("Set-Cookie"),
	}, nil
}

// fetchFaviconHashForWorker 获取favicon并计算MMH3 hash
func (w *Worker) fetchFaviconHashForWorker(baseUrl, body string, client *http.Client) string {
	faviconUrl := ""
	linkRe := regexp.MustCompile(`(?i)<link[^>]*rel=["'](?:shortcut )?icon["'][^>]*href=["']([^"']+)["']`)
	if matches := linkRe.FindStringSubmatch(body); len(matches) > 1 {
		faviconUrl = matches[1]
	}
	if faviconUrl == "" {
		linkRe2 := regexp.MustCompile(`(?i)<link[^>]*href=["']([^"']+)["'][^>]*rel=["'](?:shortcut )?icon["']`)
		if matches := linkRe2.FindStringSubmatch(body); len(matches) > 1 {
			faviconUrl = matches[1]
		}
	}
	if faviconUrl == "" {
		faviconUrl = "/favicon.ico"
	}
	if !strings.HasPrefix(faviconUrl, "http") {
		if strings.HasPrefix(faviconUrl, "//") {
			faviconUrl = "https:" + faviconUrl
		} else if strings.HasPrefix(faviconUrl, "/") {
			u := extractBaseUrlForWorker(baseUrl)
			if u != "" {
				faviconUrl = strings.TrimRight(u, "/") + faviconUrl
			}
		} else {
			u := extractBaseUrlForWorker(baseUrl)
			if u != "" {
				faviconUrl = strings.TrimRight(u, "/") + "/" + faviconUrl
			}
		}
	}

	resp, err := client.Get(faviconUrl)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	iconBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil || len(iconBytes) == 0 {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(iconBytes)
	hash := mmh3Hash32ForWorker([]byte(encoded))
	return fmt.Sprintf("%d", int32(hash))
}

// mmh3Hash32ForWorker MurmurHash3 32位实现（与API端一致）
func mmh3Hash32ForWorker(data []byte) uint32 {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
		r1 = 15
		r2 = 13
		m  = 5
		n  = 0xe6546b64
	)
	length := len(data)
	h1 := uint32(0)
	pos := 0
	for pos+4 <= length {
		k1 := uint32(data[pos]) | uint32(data[pos+1])<<8 | uint32(data[pos+2])<<16 | uint32(data[pos+3])<<24
		pos += 4
		k1 *= c1
		k1 = (k1 << r1) | (k1 >> (32 - r1))
		k1 *= c2
		h1 ^= k1
		h1 = (h1 << r2) | (h1 >> (32 - r2))
		h1 = h1*m + n
	}
	var tail uint32
	switch length - pos {
	case 3:
		tail ^= uint32(data[pos+2]) << 16
		fallthrough
	case 2:
		tail ^= uint32(data[pos+1]) << 8
		fallthrough
	case 1:
		tail ^= uint32(data[pos])
		tail *= c1
		tail = (tail << r1) | (tail >> (32 - r1))
		tail *= c2
		h1 ^= tail
	}
	h1 ^= uint32(length)
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h1 *= 0xc2b2ae35
	h1 ^= h1 >> 16
	return h1
}

// createValidateHttpClientForWorker 创建HTTP客户端
func (w *Worker) createValidateHttpClientForWorker() *http.Client {
	dialer := &net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 0,
	}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10},
		DisableKeepAlives:   true,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 8 * time.Second,
		ForceAttemptHTTP2:   false,
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// smartHttpRequestForWorker 智能HTTP请求，自动处理协议切换
func (w *Worker) smartHttpRequestForWorker(client *http.Client, baseUrl, path, originalScheme string) (*http.Response, string, string, error) {
	fullUrl := baseUrl + path
	var urls []string

	switch originalScheme {
	case "https":
		urls = append(urls, fullUrl)
		urls = append(urls, strings.Replace(fullUrl, "https://", "http://", 1))
	case "http":
		urls = append(urls, fullUrl)
	default:
		urls = append(urls, fullUrl)
		if strings.HasPrefix(fullUrl, "http://") {
			urls = append(urls, strings.Replace(fullUrl, "http://", "https://", 1))
		}
	}

	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Connection", "close")

		resp, err := client.Do(req)
		if err == nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
			resp.Body.Close()
			return resp, string(bodyBytes), url, nil
		}
		lastErr = err
	}
	return nil, "", "", fmt.Errorf("请求失败: %v", lastErr)
}

// extractBaseUrlForWorker 从URL提取基础部分
func extractBaseUrlForWorker(rawUrl string) string {
	rawUrl = strings.TrimSpace(rawUrl)
	if rawUrl == "" {
		return ""
	}
	if !strings.Contains(rawUrl, "://") {
		rawUrl = "http://" + rawUrl
	}
	schemeEnd := strings.Index(rawUrl, "://")
	rest := rawUrl[schemeEnd+3:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return rawUrl
	}
	return rawUrl[:schemeEnd+3+slashIdx]
}

// extractBaseUrlWithSchemeForWorker 从URL提取基础部分和协议
func extractBaseUrlWithSchemeForWorker(rawUrl string) (string, string) {
	rawUrl = strings.TrimSpace(rawUrl)
	if rawUrl == "" {
		return "", ""
	}
	var scheme string
	schemeEnd := strings.Index(rawUrl, "://")
	if schemeEnd == -1 {
		rawUrl = "http://" + rawUrl
		scheme = "http"
		schemeEnd = 4
	} else {
		scheme = rawUrl[:schemeEnd]
	}
	rest := rawUrl[schemeEnd+3:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return rawUrl, scheme
	}
	return rawUrl[:schemeEnd+3+slashIdx], scheme
}

// WorkerHttpServiceChecker Worker端的HTTP服务检查器实现
type WorkerHttpServiceChecker struct {
	serviceCache map[string]bool // serviceName -> isHttp
	httpPorts    map[int]bool    // HTTP端口
	httpsPorts   map[int]bool    // HTTPS端口
	nonHttpPorts map[int]bool    // 非HTTP端口（明确排除）
	mu           sync.RWMutex
}

// NewWorkerHttpServiceChecker 创建HTTP服务检查器
func NewWorkerHttpServiceChecker() *WorkerHttpServiceChecker {
	return &WorkerHttpServiceChecker{
		serviceCache: make(map[string]bool),
		httpPorts:    make(map[int]bool),
		httpsPorts:   make(map[int]bool),
		nonHttpPorts: make(map[int]bool),
	}
}

// IsHttpService 判断服务名称是否为HTTP服务
func (c *WorkerHttpServiceChecker) IsHttpService(serviceName string) (isHttp bool, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	isHttp, found = c.serviceCache[serviceName]
	return
}

// IsHttpPort 判断端口是否为HTTP端口
func (c *WorkerHttpServiceChecker) IsHttpPort(port int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.httpPorts[port] || c.httpsPorts[port]
}

// IsNonHttpPort 判断端口是否为非HTTP端口（明确排除）
func (c *WorkerHttpServiceChecker) IsNonHttpPort(port int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nonHttpPorts[port]
}

// CheckIsHttp 综合判断是否为HTTP服务（服务名称+端口）
func (c *WorkerHttpServiceChecker) CheckIsHttp(serviceName string, port int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. 先检查是否在非HTTP端口列表中（明确排除）
	if c.nonHttpPorts[port] {
		return false
	}

	// 2. 检查服务名称映射
	if isHttp, found := c.serviceCache[serviceName]; found {
		return isHttp
	}

	// 3. 检查HTTP/HTTPS端口
	return c.httpPorts[port] || c.httpsPorts[port]
}

// SetMapping 设置服务映射
func (c *WorkerHttpServiceChecker) SetMapping(serviceName string, isHttp bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceCache[serviceName] = isHttp
}

// SetHttpPorts 设置HTTP端口列表
func (c *WorkerHttpServiceChecker) SetHttpPorts(ports []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpPorts = make(map[int]bool)
	for _, port := range ports {
		c.httpPorts[port] = true
	}
}

// SetHttpsPorts 设置HTTPS端口列表
func (c *WorkerHttpServiceChecker) SetHttpsPorts(ports []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpsPorts = make(map[int]bool)
	for _, port := range ports {
		c.httpsPorts[port] = true
	}
}

// SetNonHttpPorts 设置非HTTP端口列表
func (c *WorkerHttpServiceChecker) SetNonHttpPorts(ports []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nonHttpPorts = make(map[int]bool)
	for _, port := range ports {
		c.nonHttpPorts[port] = true
	}
}

// executePortIdentify 执行端口识别阶段（Nmap/Fingerprintx 服务识别）
