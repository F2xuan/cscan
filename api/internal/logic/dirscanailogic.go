package logic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// ==================== DirScan AI研判 ====================

// dirscanAIBatchTasks 全局批量任务状态表
var dirscanAIBatchTasks sync.Map // taskId -> *dirscanBatchTaskState

type dirscanBatchTaskState struct {
	mu        sync.Mutex
	TaskId    string
	Total     int64
	Completed int64
	Status    string // running/completed/failed/stopped/stopping
	StopCh    chan struct{}
}

// DirScanLogic 目录扫描AI研判逻辑
type DirScanLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanLogic {
	return &DirScanLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// getDirScanModel 获取目录扫描结果model（全局集合 dirscan_result）
func (l *DirScanLogic) getModel() *model.DirScanResultModel {
	return model.NewDirScanResultModel(l.svcCtx.MongoDB)
}

// AnalyzeSingle 对单条目录扫描结果进行AI研判
func (l *DirScanLogic) AnalyzeSingle(req *types.DirScanAIAnalyzeReq) (*types.DirScanAIAnalyzeResp, error) {
	// 1. 加载AI配置
	aiCfg, err := l.loadDirScanAIConfig(req.WorkspaceId)
	if err != nil {
		return &types.DirScanAIAnalyzeResp{Code: 500, Msg: err.Error()}, nil
	}

	// 2. 查找目标记录
	m := l.getModel()
	doc, err := m.FindByID(l.ctx, req.Id)
	if err != nil {
		return &types.DirScanAIAnalyzeResp{Code: 404, Msg: "未找到该记录"}, nil
	}

	// 3. 构造Prompt并调用大模型
	result, reason, err := l.callDirScanAIAnalysis(aiCfg, doc)
	if err != nil {
		return &types.DirScanAIAnalyzeResp{Code: 500, Msg: "AI研判失败: " + err.Error()}, nil
	}

	// 4. 回写数据库
	now := time.Now()
	aiResult := "no_risk"
	if result == "risk" {
		aiResult = "risk"
	}
	if err := m.UpdateAIResult(l.ctx, req.Id, "completed", aiResult, reason, now); err != nil {
		return &types.DirScanAIAnalyzeResp{Code: 500, Msg: "结果保存失败: " + err.Error()}, nil
	}

	return &types.DirScanAIAnalyzeResp{
		Code: 0, Msg: "success",
		Data: &types.DirScanAIAnalyzeData{
			Id: req.Id, AIStatus: "completed", AIResult: aiResult,
			AIReason: reason, AIAnalyzedAt: now.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// BatchAnalyzeAsync 启动批量研判异步任务
func (l *DirScanLogic) BatchAnalyzeAsync(req *types.DirScanAIBatchAnalyzeReq) (*types.DirScanAIBatchAnalyzeResp, error) {
	workspaceId := req.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	m := l.getModel()

	// 构造过滤条件（使用 $and 组合，避免 $or 冲突）
	var andConditions []bson.M
	if req.Query != "" {
		andConditions = append(andConditions, bson.M{"$or": []bson.M{
			{"url": bson.M{"$regex": req.Query, "$options": "i"}},
			{"path": bson.M{"$regex": req.Query, "$options": "i"}},
			{"title": bson.M{"$regex": req.Query, "$options": "i"}},
			{"authority": bson.M{"$regex": req.Query, "$options": "i"}},
		}})
	}
	if req.StatusCode > 0 {
		andConditions = append(andConditions, bson.M{"status_code": req.StatusCode})
	}
	if req.Path != "" {
		andConditions = append(andConditions, bson.M{"path": bson.M{"$regex": req.Path, "$options": "i"}})
	}
	if req.Authority != "" {
		andConditions = append(andConditions, bson.M{"authority": bson.M{"$regex": req.Authority, "$options": "i"}})
	}
	if workspaceId != "" && workspaceId != "all" {
		andConditions = append(andConditions, bson.M{"workspace_id": workspaceId})
	}
	// 当指定了 aiResult 筛选时，按筛选条件匹配；未指定时默认只处理未研判数据
	if req.AIResult != "" {
		andConditions = append(andConditions, bson.M{"ai_result": req.AIResult})
	} else {
		// 强制未研判条件
		andConditions = append(andConditions, bson.M{"ai_status": bson.M{"$ne": "completed"}})
	}

	filter := bson.M{}
	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	var pendingDocs []*model.DirScanResult

	if len(req.Ids) > 0 {
		// 模式1：按选中ID列表
		pendingDocs, _ = m.FindPendingByIds(l.ctx, req.Ids)
	} else {
		// 模式2/3：按过滤条件
		pendingDocs, _ = m.FindPendingByFilter(l.ctx, filter, 0)
	}

	if len(pendingDocs) == 0 {
		return &types.DirScanAIBatchAnalyzeResp{Code: 0, Msg: "无待研判数据", Total: 0}, nil
	}

	// 提前校验AI配置，避免启动goroutine后才发现配置缺失
	if _, err := l.loadDirScanAIConfig(workspaceId); err != nil {
		return &types.DirScanAIBatchAnalyzeResp{Code: 500, Msg: err.Error()}, nil
	}

	taskId := primitive.NewObjectID().Hex()
	state := &dirscanBatchTaskState{
		TaskId: taskId,
		Total:  int64(len(pendingDocs)),
		Status: "running",
		StopCh: make(chan struct{}),
	}
	dirscanAIBatchTasks.Store(taskId, state)

	go l.runDirScanBatchAnalysis(taskId, state, pendingDocs)

	return &types.DirScanAIBatchAnalyzeResp{
		Code: 0, Msg: "批量研判任务已启动", TaskId: taskId, Total: int64(len(pendingDocs)),
	}, nil
}

// runDirScanBatchAnalysis 批量研判实际执行逻辑
func (l *DirScanLogic) runDirScanBatchAnalysis(taskId string, state *dirscanBatchTaskState, pendingDocs []*model.DirScanResult) {
	bgCtx := context.Background()

	aiCfg, err := l.loadDirScanAIConfigWithCtx(bgCtx, "")
	if err != nil {
		state.mu.Lock()
		state.Status = "failed"
		state.mu.Unlock()
		logx.Errorf("[DirScan-AI] batch task %s config error: %v", taskId, err)
		return
	}

	concurrency := 5
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	stopped := int32(0)
	m := l.getModel()

	for _, doc := range pendingDocs {
		select {
		case <-state.StopCh:
			atomic.StoreInt32(&stopped, 1)
		default:
		}
		if atomic.LoadInt32(&stopped) == 1 {
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(d *model.DirScanResult) {
			defer wg.Done()
			defer func() { <-sem }()

			if atomic.LoadInt32(&stopped) == 1 {
				state.mu.Lock()
				state.Completed++
				state.mu.Unlock()
				return
			}

			result, reason, err := l.callDirScanAIAnalysis(aiCfg, d)
			now := time.Now()
			aiResult := "no_risk"
			if err == nil && result == "risk" {
				aiResult = "risk"
			}
			reason2use := reason
			if err != nil {
				reason2use = "研判失败: " + err.Error()
				logx.Errorf("[DirScan-AI] doc %s analyze error: %v", d.Id.Hex(), err)
			}
			_ = m.UpdateAIResult(context.Background(), d.Id.Hex(), "completed", aiResult, reason2use, now)

			state.mu.Lock()
			state.Completed++
			state.mu.Unlock()
		}(doc)
	}
	wg.Wait()

	state.mu.Lock()
	if atomic.LoadInt32(&stopped) == 1 {
		state.Status = "stopped"
	} else {
		state.Status = "completed"
	}
	state.mu.Unlock()
	logx.Infof("[DirScan-AI] batch task %s finished: status=%s, completed=%d/%d", taskId, state.Status, state.Completed, state.Total)
}

// StopBatchTask 停止批量研判
func (l *DirScanLogic) StopBatchTask(taskId string) error {
	v, ok := dirscanAIBatchTasks.Load(taskId)
	if !ok {
		return fmt.Errorf("任务不存在或已结束")
	}
	state := v.(*dirscanBatchTaskState)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.Status != "running" {
		return fmt.Errorf("任务当前状态不允许停止: %s", state.Status)
	}
	close(state.StopCh)
	state.Status = "stopping"
	logx.Infof("[DirScan-AI] batch task %s stop signal sent", taskId)
	return nil
}

// GetBatchProgress 查询批量研判进度
func (l *DirScanLogic) GetBatchProgress(req *types.DirScanAIBatchProgressReq) (*types.DirScanAIBatchProgressResp, error) {
	v, ok := dirscanAIBatchTasks.Load(req.TaskId)
	if !ok {
		return &types.DirScanAIBatchProgressResp{Code: 404, Msg: "任务不存在"}, nil
	}
	state := v.(*dirscanBatchTaskState)
	state.mu.Lock()
	defer state.mu.Unlock()
	return &types.DirScanAIBatchProgressResp{
		Code: 0, Msg: "success",
		Total: state.Total, Completed: state.Completed, Status: state.Status,
	}, nil
}

// GetDirScanDetail 按id取单条详情（含request/response大字段）
func (l *DirScanLogic) GetDirScanDetail(req *types.DirScanDetailReq) (*types.DirScanDetailResp, error) {
	id := strings.TrimSpace(req.Id)
	if id == "" {
		return &types.DirScanDetailResp{Code: 400, Msg: "id不能为空"}, nil
	}

	m := l.getModel()
	doc, err := m.FindByID(l.ctx, id)
	if err != nil {
		return &types.DirScanDetailResp{Code: 404, Msg: "未找到该目录扫描结果"}, nil
	}

	aiAnalyzedAt := ""
	if !doc.AIAnalyzedAt.IsZero() {
		aiAnalyzedAt = doc.AIAnalyzedAt.Format("2006-01-02 15:04:05")
	}
	createTime := ""
	if !doc.CreateTime.IsZero() {
		createTime = doc.CreateTime.Format("2006-01-02 15:04:05")
	}
	updateTime := ""
	if !doc.UpdateTime.IsZero() {
		updateTime = doc.UpdateTime.Format("2006-01-02 15:04:05")
	}
	scanTime := ""
	if !doc.ScanTime.IsZero() {
		scanTime = doc.ScanTime.Format("2006-01-02 15:04:05")
	}

	return &types.DirScanDetailResp{
		Code: 0, Msg: "success",
		Data: &types.DirScanResult{
			Id:            doc.Id.Hex(),
			WorkspaceId:   doc.WorkspaceId,
			MainTaskId:    doc.MainTaskId,
			Authority:     doc.Authority,
			Host:          doc.Host,
			Port:          doc.Port,
			URL:           doc.URL,
			Path:          doc.Path,
			StatusCode:    doc.StatusCode,
			ContentLength: doc.ContentLength,
			ContentType:   doc.ContentType,
			Title:         doc.Title,
			RedirectURL:   doc.RedirectURL,
			ContentWords:  doc.ContentWords,
			ContentLines:  doc.ContentLines,
			Duration:      doc.Duration,
			Request:       doc.Request,
			Response:      doc.Response,
			CreateTime:    createTime,
			UpdateTime:    updateTime,
			ScanTime:      scanTime,
			AIStatus:      doc.AIStatus,
			AIResult:      doc.AIResult,
			AIAnalyzedAt:  aiAnalyzedAt,
			AIReason:      doc.AIReason,
		},
	}, nil
}

// GetDirScanList 目录扫描列表（带投影排除大字段 + AI状态过滤 + 缓存）
func (l *DirScanLogic) GetDirScanList(req *types.DirScanResultListReq) (*types.DirScanResultListResp, error) {
	workspaceId := req.WorkspaceId
	if workspaceId == "" {
		workspaceId = "all"
	}
	wsKey := workspaceId

	// 列表投影：排除request/response大字段
	projection := bson.M{
		"request":  0,
		"response": 0,
	}

	// 构造 $and 条件
	var andConditions []bson.M

	if workspaceId != "" && workspaceId != "all" {
		andConditions = append(andConditions, bson.M{"workspace_id": workspaceId})
	}
	if req.TaskId != "" {
		andConditions = append(andConditions, bson.M{"main_task_id": req.TaskId})
	}
	if req.Authority != "" {
		andConditions = append(andConditions, bson.M{"authority": bson.M{"$regex": req.Authority, "$options": "i"}})
	}
	if req.Url != "" {
		andConditions = append(andConditions, bson.M{"url": bson.M{"$regex": req.Url, "$options": "i"}})
	}
	if req.Path != "" {
		andConditions = append(andConditions, bson.M{"path": bson.M{"$regex": req.Path, "$options": "i"}})
	}
	if req.StatusCode > 0 {
		andConditions = append(andConditions, bson.M{"status_code": req.StatusCode})
	}
	if req.SizeMin != nil || req.SizeMax != nil {
		sizeFilter := bson.M{}
		if req.SizeMin != nil {
			sizeFilter["$gte"] = *req.SizeMin
		}
		if req.SizeMax != nil {
			sizeFilter["$lte"] = *req.SizeMax
		}
		andConditions = append(andConditions, bson.M{"content_length": sizeFilter})
	}
	if req.Query != "" && req.Url == "" && req.Path == "" && req.Authority == "" {
		andConditions = append(andConditions, bson.M{"$or": []bson.M{
			{"url": bson.M{"$regex": req.Query, "$options": "i"}},
			{"path": bson.M{"$regex": req.Query, "$options": "i"}},
			{"title": bson.M{"$regex": req.Query, "$options": "i"}},
		}})
	}
	if req.AIStatus != "" {
		if req.AIStatus == "pending" {
			andConditions = append(andConditions, bson.M{"$or": []bson.M{
				{"ai_status": bson.M{"$exists": false}},
				{"ai_status": ""},
				{"ai_status": "pending"},
			}})
		} else {
			andConditions = append(andConditions, bson.M{"ai_status": req.AIStatus})
		}
	}
	if req.AIResult != "" {
		andConditions = append(andConditions, bson.M{"ai_result": req.AIResult})
	}

	filter := bson.M{}
	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	m := l.getModel()

	var total int64
	var err error
	if len(filter) == 0 {
		total, err = m.EstimatedCount(l.ctx)
	} else {
		total, err = m.CountByFilter(l.ctx, filter)
	}
	if err != nil {
		return nil, fmt.Errorf("统计失败: %w", err)
	}

	sortField := req.SortField
	sortOrder := req.SortOrder

	opts := options.Find().
		SetSkip(int64((req.Page - 1) * req.PageSize)).
		SetLimit(int64(req.PageSize)).
		SetProjection(projection)

	sortValue := -1
	if sortOrder == "asc" {
		sortValue = 1
	}
	switch sortField {
	case "statusCode":
		opts.SetSort(bson.D{{Key: "status_code", Value: sortValue}, {Key: "create_time", Value: -1}})
	case "contentLength":
		opts.SetSort(bson.D{{Key: "content_length", Value: sortValue}, {Key: "create_time", Value: -1}})
	case "contentWords":
		opts.SetSort(bson.D{{Key: "content_words", Value: sortValue}, {Key: "create_time", Value: -1}})
	case "contentLines":
		opts.SetSort(bson.D{{Key: "content_lines", Value: sortValue}, {Key: "create_time", Value: -1}})
	case "duration":
		opts.SetSort(bson.D{{Key: "duration", Value: sortValue}, {Key: "create_time", Value: -1}})
	default:
		opts.SetSort(bson.D{{Key: "create_time", Value: -1}})
	}

	cursor, err := m.Collection().Find(l.ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer cursor.Close(l.ctx)

	var docs []model.DirScanResult
	if err = cursor.All(l.ctx, &docs); err != nil {
		return nil, fmt.Errorf("解码失败: %w", err)
	}
	if docs == nil {
		docs = []model.DirScanResult{}
	}

	_ = wsKey
	respList := make([]*types.DirScanResult, 0, len(docs))
	for _, d := range docs {
		respList = append(respList, dirScanToResp(&d))
	}

	return &types.DirScanResultListResp{
		Code: 0, Msg: "success", Total: total, List: respList,
	}, nil
}

func dirScanToResp(d *model.DirScanResult) *types.DirScanResult {
	createTime := ""
	if !d.CreateTime.IsZero() {
		createTime = d.CreateTime.Format("2006-01-02 15:04:05")
	}
	updateTime := ""
	if !d.UpdateTime.IsZero() {
		updateTime = d.UpdateTime.Format("2006-01-02 15:04:05")
	}
	scanTime := ""
	if !d.ScanTime.IsZero() {
		scanTime = d.ScanTime.Format("2006-01-02 15:04:05")
	}
	aiAnalyzedAt := ""
	if !d.AIAnalyzedAt.IsZero() {
		aiAnalyzedAt = d.AIAnalyzedAt.Format("2006-01-02 15:04:05")
	}
	return &types.DirScanResult{
		Id:            d.Id.Hex(),
		WorkspaceId:   d.WorkspaceId,
		MainTaskId:    d.MainTaskId,
		Authority:     d.Authority,
		Host:          d.Host,
		Port:          d.Port,
		URL:           d.URL,
		Path:          d.Path,
		StatusCode:    d.StatusCode,
		ContentLength: d.ContentLength,
		ContentType:   d.ContentType,
		Title:         d.Title,
		RedirectURL:   d.RedirectURL,
		ContentWords:  d.ContentWords,
		ContentLines:  d.ContentLines,
		Duration:      d.Duration,
		CreateTime:    createTime,
		UpdateTime:    updateTime,
		ScanTime:      scanTime,
		AIStatus:      d.AIStatus,
		AIResult:      d.AIResult,
		AIAnalyzedAt:  aiAnalyzedAt,
		AIReason:      d.AIReason,
	}
}

// ClearDirScanResults 清空目录扫描结果
func (l *DirScanLogic) ClearDirScanResults(workspaceId string) (int64, error) {
	m := l.getModel()
	filter := bson.M{}
	if workspaceId != "" && workspaceId != "all" {
		filter["workspace_id"] = workspaceId
	}
	return m.DeleteMany(l.ctx, filter)
}

// ==================== AI研判辅助方法 ====================

func (l *DirScanLogic) loadDirScanAIConfig(workspaceId string) (*model.APIConfig, error) {
	return l.loadDirScanAIConfigWithCtx(l.ctx, workspaceId)
}

func (l *DirScanLogic) loadDirScanAIConfigWithCtx(ctx context.Context, workspaceId string) (*model.APIConfig, error) {
	tryWorkspaces := []string{}
	if workspaceId != "" {
		tryWorkspaces = append(tryWorkspaces, workspaceId)
	}
	tryWorkspaces = append(tryWorkspaces, "all", "default")

	for _, wsId := range tryWorkspaces {
		cfgModel := model.NewAPIConfigModel(l.svcCtx.MongoDB, wsId)
		doc, err := cfgModel.FindByPlatform(ctx, "ai")
		if err == nil && doc != nil {
			return doc, nil
		}
	}

	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db := l.svcCtx.MongoDB
	collections, err := db.ListCollectionNames(listCtx, bson.M{"name": bson.M{"$regex": "_api_config$"}})
	if err == nil {
		tried := map[string]bool{}
		for _, ws := range tryWorkspaces {
			tried[ws] = true
		}
		for _, collName := range collections {
			wsId := collName[:len(collName)-len("_api_config")]
			if tried[wsId] {
				continue
			}
			tempModel := model.NewAPIConfigModel(db, wsId)
			doc, err := tempModel.FindByPlatform(ctx, "ai")
			if err == nil && doc != nil {
				return doc, nil
			}
		}
	}
	return nil, fmt.Errorf("未配置AI服务，请先在系统设置中配置AI")
}

// callDirScanAIAnalysis 调用大模型研判单条目录扫描结果
func (l *DirScanLogic) callDirScanAIAnalysis(cfg *model.APIConfig, doc *model.DirScanResult) (string, string, error) {
	client := NewAIClientFromConfig(cfg)
	prompt := buildDirScanAnalysisPrompt(doc)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	content, err := client.Chat(ctx, prompt, 1024)
	if err != nil {
		return "", "", err
	}
	return parseAIAnalysisResult(content) // 复用JSFinder的JSON解析逻辑
}

// buildDirScanAnalysisPrompt 构造目录扫描结果的风险研判Prompt
func buildDirScanAnalysisPrompt(doc *model.DirScanResult) string {
	var sb strings.Builder
	sb.WriteString("你是一个Web安全专家，请判断以下目录扫描发现的路径是否存在安全风险。\n\n")
	sb.WriteString(fmt.Sprintf("目标URL: %s\n", doc.URL))
	sb.WriteString(fmt.Sprintf("路径: %s\n", doc.Path))
	sb.WriteString(fmt.Sprintf("HTTP状态码: %d\n", doc.StatusCode))
	sb.WriteString(fmt.Sprintf("页面标题: %s\n", doc.Title))
	sb.WriteString(fmt.Sprintf("内容类型: %s\n", doc.ContentType))
	sb.WriteString(fmt.Sprintf("响应大小: %d bytes\n", doc.ContentLength))
	if doc.RedirectURL != "" {
		sb.WriteString(fmt.Sprintf("重定向到: %s\n", doc.RedirectURL))
	}
	if doc.Response != "" {
		// 截取响应内容前2000字符，避免token过多
		resp := doc.Response
		if len(resp) > 2000 {
			resp = resp[:2000] + "...(截断)"
		}
		sb.WriteString(fmt.Sprintf("响应内容(前2000字符):\n%s\n", resp))
	}
	sb.WriteString("\n请判断该路径是否存在安全风险。重点关注以下情况：\n")
	sb.WriteString("1. 敏感目录/文件暴露（如备份文件、配置文件、.git目录、数据库文件、日志文件、phpinfo等）\n")
	sb.WriteString("2. 管理后台入口、未授权访问页面\n")
	sb.WriteString("3. 目录遍历漏洞、文件列表暴露\n")
	sb.WriteString("4. 错误页面泄露敏感信息（路径、堆栈、版本号等）\n")
	sb.WriteString("5. 200状态码下的默认页面、空页面、普通静态资源不算风险\n")
	sb.WriteString(`请严格按以下JSON格式回复（不要有其他内容）：
{"result": "risk" 或 "no_risk", "reason": "简短说明判断理由，不超过100字"}`)
	return sb.String()
}

// 确保引用 common 包以避免 unused 报错（workspace 列表可能用到）
var _ = common.GetWorkspaceIds
