package logic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/internal/onlineapi"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// onlineImportTaskState 在线导入任务状态
type onlineImportTaskState struct {
	mu           sync.RWMutex
	TaskId       string    `json:"taskId"`
	Status       string    `json:"status"` // running/completed/failed
	Total        int       `json:"total"`
	Completed    int       `json:"completed"`
	Imported     int       `json:"imported"`
	Skipped      int       `json:"skipped"`
	ErrorMsg     string    `json:"errorMsg,omitempty"`
	Platform     string    `json:"platform"`
	ImportType   string    `json:"importType"` // current/all
	StartTime    time.Time `json:"startTime"`
	EndTime      time.Time `json:"endTime,omitempty"`
	TotalFetched int       `json:"totalFetched"` // ImportAll专用
	TotalPages   int       `json:"totalPages"`   // ImportAll专用
}

// onlineImportTasks 全局任务存储（taskId -> *onlineImportTaskState）
var onlineImportTasks sync.Map

func init() {
	// 定期清理已完成/失败超过 1 小时的导入任务，防止内存泄漏
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-1 * time.Hour)
			onlineImportTasks.Range(func(key, value any) bool {
				state := value.(*onlineImportTaskState)
				state.mu.RLock()
				shouldDelete := (state.Status == "completed" || state.Status == "failed") && state.EndTime.Before(cutoff)
				state.mu.RUnlock()
				if shouldDelete {
					onlineImportTasks.Delete(key)
				}
				return true
			})
		}
	}()
}

// OnlineAPILogic 在线API逻辑
type OnlineAPILogic struct {
	ctx context.Context
	svc *svc.ServiceContext
}

func NewOnlineAPILogic(ctx context.Context, svc *svc.ServiceContext) *OnlineAPILogic {
	return &OnlineAPILogic{ctx: ctx, svc: svc}
}

func (l *OnlineAPILogic) Search(req *types.OnlineSearchReq, workspaceId string) (*types.OnlineSearchResp, error) {
	// 获取API配置
	configModel := model.NewAPIConfigModel(l.svc.MongoDB, workspaceId)
	config, err := configModel.FindByPlatform(l.ctx, req.Platform)
	if err != nil {
		logx.Errorf("OnlineAPI Search: find config failed, platform=%s, error=%v", req.Platform, err)
		return &types.OnlineSearchResp{Code: 500, Msg: "查询API配置失败"}, nil
	}
	if config == nil {
		return &types.OnlineSearchResp{Code: 404, Msg: "未配置" + req.Platform + "的API密钥"}, nil
	}

	var results []types.OnlineSearchResult
	var total int

	switch req.Platform {
	case "fofa":
		client := onlineapi.NewFofaClient(config.Key, config.Version)
		req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
		result, err := client.Search(l.ctx, req.Query, req.Page, req.PageSize)
		if err != nil {
			return &types.OnlineSearchResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
		}
		total = result.Size
		assets := client.ParseResults(result)
		for _, a := range assets {
			results = append(results, types.OnlineSearchResult{
				Host: a.Host, IP: a.IP, Port: a.Port, Protocol: a.Protocol,
				Domain: a.Domain, Title: a.Title, Server: a.Server,
				Country: a.Country, City: a.City, Banner: a.Banner,
				ICP: a.ICP, Product: a.Product, OS: a.OS,
			})
		}
	case "hunter":
		client := onlineapi.NewHunterClient(config.Key)
		// Hunter API page_size 最大为100
		hunterPageSize := req.PageSize
		if hunterPageSize > 100 {
			hunterPageSize = 100
		}
		result, err := client.Search(l.ctx, req.Query, req.Page, hunterPageSize, "", "")
		if err != nil {
			return &types.OnlineSearchResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
		}
		total = result.Data.Total
		for _, a := range result.Data.Arr {
			component := ""
			if len(a.Component) > 0 {
				component = a.Component[0].Name
			}
			results = append(results, types.OnlineSearchResult{
				Host: a.URL, IP: a.IP, Port: a.Port, Protocol: a.Protocol,
				Domain: a.Domain, Title: a.WebTitle, Server: component,
				Country: a.Country, City: a.City, Banner: a.Banner,
				ICP: a.Number, Product: component, OS: a.OS,
			})
		}
	case "quake":
		client := onlineapi.NewQuakeClient(config.Key)
		result, err := client.Search(l.ctx, req.Query, req.Page, req.PageSize)
		if err != nil {
			return &types.OnlineSearchResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
		}
		// 检查是否配额用尽
		if result.Data.IsExhausted {
			return &types.OnlineSearchResp{Code: 403, Msg: "Quake API 配额已用尽，无法获取更多数据"}, nil
		}
		total = result.Meta.Pagination.Total
		for _, a := range result.Data.Items {
			results = append(results, types.OnlineSearchResult{
				Host: a.Service.HTTP.Host, IP: a.IP, Port: a.Port, Protocol: a.Service.Name,
				Title: a.Service.HTTP.Title, Server: a.Service.HTTP.Server,
				Country: a.Location.CountryCN, City: a.Location.CityCN,
			})
		}
	default:
		return &types.OnlineSearchResp{Code: 400, Msg: "不支持的平台"}, nil
	}

	return &types.OnlineSearchResp{Code: 0, Msg: "success", Total: total, List: results}, nil
}

// Import 导入当前页资产（同步执行，由handler异步调用并上报进度）
func (l *OnlineAPILogic) Import(req *types.OnlineImportReq, workspaceId string, state *onlineImportTaskState) (*types.BaseResp, error) {
	// 将 "all" 解析为真实的默认工作空间，避免写入 all_asset 集合
	workspaceId = common.GetDefaultWorkspaceId(l.ctx, l.svc, workspaceId)
	assetModel := l.svc.GetAssetModel(workspaceId)
	targetMetaModel := l.svc.GetAssetTargetMetaModel(workspaceId)

	imported := 0 // 新增数
	skipped := 0  // 跳过（空主机 + 已存在）
	total := len(req.Assets)

	for i, a := range req.Assets {
		// 复用 onlineapi.BuildAsset 公共构造（与定时拉取一致），保持手动导入行为不变
		asset := onlineapi.BuildAsset(a.Host, a.IP, a.Domain, a.Protocol, a.Title, a.Server, a.Country, a.City, a.Banner, a.Product, a.Port, req.Platform)
		// 手动导入来源固定为 "onlineapi"（批量/拉取路径用 "onlineapi-<platform>"）
		asset.Source = "onlineapi"
		// Skip if host is empty（BuildAsset 内部已解析，空 host 视为无效）
		if asset.Host == "" {
			skipped++
			state.mu.Lock()
			state.Completed = i + 1
			state.Skipped = skipped
			state.mu.Unlock()
			continue
		}

		// 使用 UpsertWithResult 区分新增和已存在
		res, err := assetModel.UpsertWithResult(l.ctx, asset)
		if err != nil {
			logx.Errorf("Import: upsert asset host=%s failed: %v", asset.Host, err)
		} else if res.IsNew {
			imported++
		} else {
			// 资产已存在（重复导入），计入跳过
			skipped++
		}
		// 同步创建/更新顶层资产（AssetTargetMeta），确保资产出现在资产概览中
		if err := targetMetaModel.EnsureForAsset(l.ctx, workspaceId, asset.Host, asset.Domain, nil); err != nil {
			logx.Errorf("Import: failed to ensure target meta for host=%s: %v", asset.Host, err)
		}

		state.mu.Lock()
		state.Completed = i + 1
		state.Imported = imported
		state.Skipped = skipped
		state.mu.Unlock()
	}

	_ = total // total used for state initialization
	return &types.BaseResp{Code: 0, Msg: fmt.Sprintf("成功新增%d条资产，跳过%d条（空主机/已存在）", imported, skipped)}, nil
}

// ImportAll 导入全部资产（自动遍历所有页面，同步执行，由handler异步调用并上报进度）
func (l *OnlineAPILogic) ImportAll(req *types.OnlineImportAllReq, workspaceId string, state *onlineImportTaskState) (*types.OnlineImportAllResp, error) {
	// 先用原始 workspaceId 获取API配置（配置存储在原始集合中）
	configModel := model.NewAPIConfigModel(l.svc.MongoDB, workspaceId)
	config, err := configModel.FindByPlatform(l.ctx, req.Platform)
	if err != nil {
		logx.Errorf("OnlineAPI ImportAll: find config failed, platform=%s, error=%v", req.Platform, err)
		return &types.OnlineImportAllResp{Code: 500, Msg: "查询API配置失败"}, nil
	}
	if config == nil {
		return &types.OnlineImportAllResp{Code: 404, Msg: "未配置" + req.Platform + "的API密钥"}, nil
	}

	// 将 "all" 解析为真实的默认工作空间，避免资产写入 all_asset 集合
	workspaceId = common.GetDefaultWorkspaceId(l.ctx, l.svc, workspaceId)
	assetModel := l.svc.GetAssetModel(workspaceId)
	targetMetaModel := l.svc.GetAssetTargetMetaModel(workspaceId)
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 500
	}

	// 各平台单次最大条数限制：
	// - FOFA: 单次最大500（高级会员上限），API QPS限制1次/秒，需要足够间隔
	// - Hunter: 官方限制单次最大100
	// - Quake: 官方限制单次最大100
	switch req.Platform {
	case "fofa":
		if pageSize > 500 {
			pageSize = 500
		}
	case "hunter", "quake":
		if pageSize > 100 {
			pageSize = 100
		}
	}

	// maxPages <= 0 表示不限制页数
	maxPages := req.MaxPages
	hasMaxPages := maxPages > 0

	totalFetched := 0
	totalImport := 0
	totalSkipped := 0
	currentPage := 1
	emptyPageCount := 0 // 连续空页计数（处理API不稳定返回空的情况）
	const maxEmptyPages = 2
	// FOFA请求重试参数（QPS限制1次/秒，错误码45012=请求过快）
	const maxRateLimitRetries = 3
	const rateLimitWait = 2 * time.Second

PageLoop:
	for {
		// 如果设置了最大页数限制，检查是否超过
		if hasMaxPages && currentPage > maxPages {
			break
		}

		var results []types.OnlineSearchResult
		var rawResultCount int // API 原始返回条数（解析前）

		switch req.Platform {
		case "fofa":
			client := onlineapi.NewFofaClient(config.Key, config.Version)
			var result *onlineapi.FofaResult
			var err error
			// FOFA限流重试：遇到45012错误等待后重试
			for retry := 0; retry <= maxRateLimitRetries; retry++ {
				result, err = client.Search(l.ctx, req.Query, currentPage, pageSize)
				if err != nil {
					errStr := err.Error()
					// 45012 = 请求速度过快，等待后重试
					if strings.Contains(errStr, "45012") && retry < maxRateLimitRetries {
						logx.Infof("ImportAll fofa page=%d rate limited (45012), waiting %v before retry %d", currentPage, rateLimitWait, retry+1)
						time.Sleep(rateLimitWait)
						continue
					}
					if currentPage == 1 {
						return &types.OnlineImportAllResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
					}
					// 非首页错误（非限流），记录日志后终止
					logx.Errorf("ImportAll fofa page=%d error: %v", currentPage, err)
					break PageLoop
				}
				break
			}
			// FOFA 的 results 数组长度即为本页返回条数
			// FOFA API 限制：普通会员最多返回前100条，高级会员前500条，企业版更多
			// FOFA 的 Size 字段含义：v1版是查询匹配总数，v5版是当前页条数，不依赖此字段
			rawResultCount = len(result.Results)
			assets := client.ParseResults(result)
			for _, a := range assets {
				results = append(results, types.OnlineSearchResult{
					Host: a.Host, IP: a.IP, Port: a.Port, Protocol: a.Protocol,
					Domain: a.Domain, Title: a.Title, Server: a.Server,
					Country: a.Country, City: a.City, Banner: a.Banner,
					ICP: a.ICP, Product: a.Product, OS: a.OS,
				})
			}
		case "hunter":
			client := onlineapi.NewHunterClient(config.Key)
			hunterPageSize := pageSize
			if hunterPageSize > 100 {
				hunterPageSize = 100
			}
			result, err := client.Search(l.ctx, req.Query, currentPage, hunterPageSize, "", "")
			if err != nil {
				if currentPage == 1 {
					return &types.OnlineImportAllResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
				}
				logx.Errorf("ImportAll hunter page=%d error: %v", currentPage, err)
				break PageLoop
			}
			if currentPage == 1 && result.Data.Total > 0 {
				state.mu.Lock()
				state.Total = result.Data.Total
				state.mu.Unlock()
			}
			rawResultCount = len(result.Data.Arr)
			for _, a := range result.Data.Arr {
				var components []string
				for _, c := range a.Component {
					components = append(components, c.Name)
				}
				component := strings.Join(components, ",")
				results = append(results, types.OnlineSearchResult{
					Host: a.URL, IP: a.IP, Port: a.Port, Protocol: a.Protocol,
					Domain: a.Domain, Title: a.WebTitle, Server: component,
					Country: a.Country, City: a.City, Banner: a.Banner,
					ICP: a.Number, Product: component, OS: a.OS,
				})
			}
		case "quake":
			client := onlineapi.NewQuakeClient(config.Key)
			result, err := client.Search(l.ctx, req.Query, currentPage, pageSize)
			if err != nil {
				if currentPage == 1 {
					return &types.OnlineImportAllResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
				}
				logx.Errorf("ImportAll quake page=%d error: %v", currentPage, err)
				break PageLoop
			}
			if result.Data.IsExhausted {
				break PageLoop
			}
			if currentPage == 1 && result.Meta.Pagination.Total > 0 {
				state.mu.Lock()
				state.Total = result.Meta.Pagination.Total
				state.mu.Unlock()
			}
			rawResultCount = len(result.Data.Items)
			for _, a := range result.Data.Items {
				results = append(results, types.OnlineSearchResult{
					Host: a.Service.HTTP.Host, IP: a.IP, Port: a.Port, Protocol: a.Service.Name,
					Title: a.Service.HTTP.Title, Server: a.Service.HTTP.Server,
					Country: a.Location.CountryCN, City: a.Location.CityCN,
				})
			}
		default:
			return &types.OnlineImportAllResp{Code: 400, Msg: "不支持的平台"}, nil
		}

		// 没有更多数据了
		if rawResultCount == 0 {
			emptyPageCount++
			if emptyPageCount >= maxEmptyPages {
				// 连续空页，确认没有更多数据
				break
			}
			// 偶尔空页可能是API不稳定，等待后重试一次
			if currentPage > 1 {
				time.Sleep(500 * time.Millisecond)
				currentPage++
				continue
			}
			break
		}
		emptyPageCount = 0 // 重置空页计数

		// 用 API 原始返回条数累加，而非 ParseResults 过滤后的条数
		totalFetched += rawResultCount

		// 导入当前页的资产
		for _, a := range results {
			// 复用 onlineapi.BuildAsset 公共构造（与定时拉取一致），保持批量导入行为不变
			asset := onlineapi.BuildAsset(a.Host, a.IP, a.Domain, a.Protocol, a.Title, a.Server, a.Country, a.City, a.Banner, a.Product, a.Port, req.Platform)
			asset.Source = "onlineapi-" + req.Platform
			// Skip if host is empty（BuildAsset 内部已解析，空 host 视为无效）
			if asset.Host == "" {
				totalSkipped++
				state.mu.Lock()
				state.Completed++
				state.Skipped = totalSkipped
				state.TotalFetched = totalFetched
				state.mu.Unlock()
				continue
			}

			// 使用 UpsertWithResult 区分新增和已存在
			res, err := assetModel.UpsertWithResult(l.ctx, asset)
			if err != nil {
				logx.Errorf("ImportAll: upsert asset host=%s failed: %v", asset.Host, err)
			} else if res.IsNew {
				totalImport++
			} else {
				// 资产已存在（重复导入），计入跳过
				totalSkipped++
			}
			// 同步创建/更新顶层资产（AssetTargetMeta），确保资产出现在资产概览中
			if err := targetMetaModel.EnsureForAsset(l.ctx, workspaceId, asset.Host, asset.Domain, nil); err != nil {
				logx.Errorf("ImportAll: failed to ensure target meta for host=%s: %v", asset.Host, err)
			}

			state.mu.Lock()
			state.Completed++
			state.Imported = totalImport
			state.Skipped = totalSkipped
			state.TotalFetched = totalFetched
			state.mu.Unlock()
		}

		// 判断是否还有更多数据
		// 如果本页返回条数小于请求的pageSize，说明已经到最后一页
		if rawResultCount < pageSize {
			break
		}

		currentPage++

		// 分页请求间添加足够延迟，避免触发API限流
		// - FOFA: QPS限制1次/秒，至少间隔1秒
		// - Hunter/Quake: 间隔1秒保险
		time.Sleep(1500 * time.Millisecond)
	}

	totalPages := currentPage
	msg := fmt.Sprintf("成功新增 %d 条资产", totalImport)
	if totalSkipped > 0 {
		msg += fmt.Sprintf("，跳过 %d 条（空主机/已存在）", totalSkipped)
	}
	return &types.OnlineImportAllResp{
		Code:         0,
		Msg:          msg,
		TotalFetched: totalFetched,
		TotalImport:  totalImport,
		TotalPages:   totalPages,
	}, nil
}

// ===== 导入任务进度/结果查询 =====

// SubmitImportTask 提交导入任务（返回taskId）
// platform/importType/total 在创建时原子写入，避免竞态读取
func SubmitImportTask(platform, importType string, total int) string {
	taskId := uuid.New().String()
	state := &onlineImportTaskState{
		TaskId:     taskId,
		Status:     "running",
		Platform:   platform,
		ImportType: importType,
		Total:      total,
		StartTime:  time.Now(),
	}
	onlineImportTasks.Store(taskId, state)
	return taskId
}

// GetOnlineImportTaskState 获取任务状态（供handler使用，直接操作state指针）
func GetOnlineImportTaskState(taskId string) (*onlineImportTaskState, bool) {
	v, ok := onlineImportTasks.Load(taskId)
	if !ok {
		return nil, false
	}
	return v.(*onlineImportTaskState), true
}

// GetImportTaskProgress 获取导入任务进度
func (l *OnlineAPILogic) GetImportTaskProgress(req *types.OnlineImportTaskProgressReq) (*types.OnlineImportTaskProgressResp, error) {
	if req.TaskId == "" {
		return &types.OnlineImportTaskProgressResp{Code: 400, Msg: "任务ID不能为空"}, nil
	}

	if v, ok := onlineImportTasks.Load(req.TaskId); ok {
		state := v.(*onlineImportTaskState)
		state.mu.RLock()
		defer state.mu.RUnlock()
		return &types.OnlineImportTaskProgressResp{
			Code:       0,
			TaskId:     state.TaskId,
			Status:     state.Status,
			Total:      state.Total,
			Completed:  state.Completed,
			Imported:   state.Imported,
			Skipped:    state.Skipped,
			ErrorMsg:   state.ErrorMsg,
			Platform:   state.Platform,
			ImportType: state.ImportType,
			StartTime:  state.StartTime.Local().Format("2006-01-02 15:04:05.000"),
			EndTime:    formatOptTime(state.EndTime),
		}, nil
	}

	return &types.OnlineImportTaskProgressResp{Code: 404, Msg: "任务不存在或已过期"}, nil
}

// GetImportTaskResult 获取导入任务结果
func (l *OnlineAPILogic) GetImportTaskResult(req *types.OnlineImportTaskResultReq) (*types.OnlineImportTaskResultResp, error) {
	if req.TaskId == "" {
		return &types.OnlineImportTaskResultResp{Code: 400, Msg: "任务ID不能为空"}, nil
	}

	if v, ok := onlineImportTasks.Load(req.TaskId); ok {
		state := v.(*onlineImportTaskState)
		state.mu.RLock()
		defer state.mu.RUnlock()
		return &types.OnlineImportTaskResultResp{
			Code:         0,
			TaskId:       state.TaskId,
			Status:       state.Status,
			Total:        state.Total,
			Completed:    state.Completed,
			Imported:     state.Imported,
			Skipped:      state.Skipped,
			ErrorMsg:     state.ErrorMsg,
			Platform:     state.Platform,
			ImportType:   state.ImportType,
			StartTime:    state.StartTime.Local().Format("2006-01-02 15:04:05.000"),
			EndTime:      formatOptTime(state.EndTime),
			TotalFetched: state.TotalFetched,
			TotalPages:   state.TotalPages,
		}, nil
	}

	return &types.OnlineImportTaskResultResp{Code: 404, Msg: "任务不存在或已过期"}, nil
}

func (l *OnlineAPILogic) ConfigList(workspaceId string) (*types.APIConfigListResp, error) {
	configModel := model.NewAPIConfigModel(l.svc.MongoDB, workspaceId)
	docs, err := configModel.FindAll(l.ctx)
	if err != nil {
		return &types.APIConfigListResp{Code: 500, Msg: "查询失败"}, nil
	}

	list := make([]types.APIConfig, 0, len(docs))
	for _, doc := range docs {
		list = append(list, types.APIConfig{
			Id:         doc.Id.Hex(),
			Platform:   doc.Platform,
			Key:        maskSecret(doc.Key),
			Secret:     maskSecret(doc.Secret),
			Version:    doc.Version,
			Status:     doc.Status,
			CreateTime: doc.CreateTime.Local().Format("2006-01-02 15:04:05"),
		})
	}

	return &types.APIConfigListResp{Code: 0, Msg: "success", List: list}, nil
}

func (l *OnlineAPILogic) ConfigSave(req *types.APIConfigSaveReq, workspaceId string) (*types.BaseResp, error) {
	configModel := model.NewAPIConfigModel(l.svc.MongoDB, workspaceId)

	if req.Id != "" {
		update := bson.M{"update_time": time.Now()}
		// 仅在有值时覆盖凭证，避免配置更新时误清空 Key/Secret
		if req.Key != "" {
			update["key"] = req.Key
		}
		if req.Secret != "" {
			update["secret"] = req.Secret
		}
		if req.Version != "" {
			update["version"] = req.Version
		}
		if req.Status != "" {
			update["status"] = req.Status
		}
		if err := configModel.Update(l.ctx, req.Id, update); err != nil {
			return &types.BaseResp{Code: 500, Msg: "更新失败"}, nil
		}
	} else {
		status := req.Status
		if status == "" {
			status = "enable"
		}
		doc := &model.APIConfig{
			Id:       primitive.NewObjectID(),
			Platform: req.Platform,
			Key:      req.Key,
			Secret:   req.Secret,
			Version:  req.Version,
			Status:   status,
		}
		if err := configModel.Insert(l.ctx, doc); err != nil {
			return &types.BaseResp{Code: 500, Msg: "保存失败"}, nil
		}
	}

	return &types.BaseResp{Code: 0, Msg: "保存成功"}, nil
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// formatOptTime 格式化可选时间字段；为零值返回空字符串（避免展示 0001-01-01）
func formatOptTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05.000")
}
