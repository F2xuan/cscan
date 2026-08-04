package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// WeakpassDictListLogic 弱口令字典列表逻辑
type WeakpassDictListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictListLogic {
	return &WeakpassDictListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WeakpassDictListLogic) WeakpassDictList(req *types.WeakpassDictListReq) (*types.WeakpassDictListResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 获取列表
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	dicts, err := dictModel.FindAll(l.ctx, req.Page, req.PageSize, req.Service, req.Name)
	if err != nil {
		return nil, err
	}

	// 获取总数
	total, err := dictModel.Count(l.ctx, req.Service)
	if err != nil {
		return nil, err
	}

	// 转换为响应类型
	list := make([]types.WeakpassDict, 0, len(dicts))
	for _, d := range dicts {
		list = append(list, types.WeakpassDict{
			Id:          d.Id.Hex(),
			Name:        d.Name,
			Description: d.Description,
			Service:     d.Service,
			Content:     d.Content,
			WordCount:   d.WordCount,
			Enabled:     d.Enabled,
			IsBuiltin:   d.IsBuiltin,
			CreateTime:  d.CreateTime.Format("2006-01-02 15:04:05"),
			UpdateTime:  d.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.WeakpassDictListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}

// WeakpassDictSaveLogic 保存弱口令字典逻辑
type WeakpassDictSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictSaveLogic {
	return &WeakpassDictSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WeakpassDictSaveLogic) WeakpassDictSave(req *types.WeakpassDictSaveReq) (*types.BaseRespWithId, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 计算词条数量（用户名:密码组合数）
	wordCount := countWeakpassCombinations(req.Content)

	if req.Id != "" {
		// 更新
		dict := &model.WeakpassDict{
			Name:        req.Name,
			Description: req.Description,
			Service:     req.Service,
			Content:     req.Content,
			WordCount:   wordCount,
			Enabled:     req.Enabled,
		}
		if err := dictModel.Update(l.ctx, req.Id, dict); err != nil {
			return nil, err
		}
		return &types.BaseRespWithId{Code: 0, Msg: "success", Id: req.Id}, nil
	}

	// 新增
	dict := &model.WeakpassDict{
		Name:        req.Name,
		Description: req.Description,
		Service:     req.Service,
		Content:     req.Content,
		WordCount:   wordCount,
		Enabled:     req.Enabled,
		IsBuiltin:   false,
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	}
	if err := dictModel.Insert(l.ctx, dict); err != nil {
		return nil, err
	}

	return &types.BaseRespWithId{Code: 0, Msg: "success", Id: dict.Id.Hex()}, nil
}

// WeakpassDictDeleteLogic 删除弱口令字典逻辑
type WeakpassDictDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictDeleteLogic {
	return &WeakpassDictDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WeakpassDictDeleteLogic) WeakpassDictDelete(req *types.WeakpassDictDeleteReq) (*types.BaseResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 先检查是否为内置字典
	dict, err := dictModel.FindById(l.ctx, req.Id)
	if err != nil {
		l.Logger.Errorf("WeakpassDictDelete: find dict failed, id=%s, error=%v", req.Id, err)
		return &types.BaseResp{Code: 500, Msg: "查询字典失败"}, nil
	}
	if dict == nil {
		return &types.BaseResp{Code: 404, Msg: "字典不存在"}, nil
	}
	if dict.IsBuiltin {
		return &types.BaseResp{Code: 1, Msg: "无法删除内置字典"}, nil
	}

	if err := dictModel.Delete(l.ctx, req.Id); err != nil {
		return nil, err
	}

	return &types.BaseResp{Code: 0, Msg: "success"}, nil
}

// WeakpassDictClearLogic 清空弱口令字典逻辑
type WeakpassDictClearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictClearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictClearLogic {
	return &WeakpassDictClearLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WeakpassDictClearLogic) WeakpassDictClear() (*types.WeakpassDictClearResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 只删除非内置字典
	deleted, err := dictModel.DeleteNonBuiltin(l.ctx)
	if err != nil {
		return nil, err
	}

	return &types.WeakpassDictClearResp{
		Code:    0,
		Msg:     "success",
		Deleted: int(deleted),
	}, nil
}

// WeakpassDictEnabledListLogic 获取启用的弱口令字典列表逻辑
type WeakpassDictEnabledListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictEnabledListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictEnabledListLogic {
	return &WeakpassDictEnabledListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WeakpassDictEnabledListLogic) WeakpassDictEnabledList() (*types.WeakpassDictEnabledListResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 获取所有启用的字典（不限制服务类型）
	dicts, err := dictModel.FindEnabled(l.ctx, "")
	if err != nil {
		return nil, err
	}

	list := make([]types.WeakpassDictSimple, 0, len(dicts))
	for _, d := range dicts {
		list = append(list, types.WeakpassDictSimple{
			Id:        d.Id.Hex(),
			Name:      d.Name,
			Service:   d.Service,
			WordCount: d.WordCount,
			IsBuiltin: d.IsBuiltin,
		})
	}

	return &types.WeakpassDictEnabledListResp{
		Code: 0,
		Msg:  "success",
		List: list,
	}, nil
}

// WeakpassDictImportLogic 导入弱口令字典逻辑
type WeakpassDictImportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictImportLogic {
	return &WeakpassDictImportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WeakpassDictImport 导入弱口令字典
func (l *WeakpassDictImportLogic) WeakpassDictImport(req *types.WeakpassDictImportReq) (*types.WeakpassDictImportResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	result := &types.WeakpassDictImportResp{
		Errors: []string{},
	}

	// 自动检测格式
	format := strings.ToLower(req.Format)
	if format == "" || format == "auto" {
		format = detectWeakpassFormat(req.Content)
	}

	var groups map[string][]model.WeakpassEntry

	switch format {
	case "grouped":
		// 分组格式：[service]\nuser:pass\n...
		groups = model.ParseGroupedWeakpassDict(req.Content)
	case "simple":
		// 简单格式：每行 user:pass
		entries := model.ParseWeakpassDict(req.Content)
		groups = map[string][]model.WeakpassEntry{
			"common": entries,
		}
	default:
		// 默认按简单格式处理
		entries := model.ParseWeakpassDict(req.Content)
		groups = map[string][]model.WeakpassEntry{
			"common": entries,
		}
	}

	// 如果指定了单个服务名称且格式为simple，合并到指定服务
	if req.Service != "" && format == "simple" && len(groups) == 1 {
		// 已经合并了
	} else if req.Service != "" {
		// 合并所有到指定服务
		var allEntries []model.WeakpassEntry
		for _, entries := range groups {
			allEntries = append(allEntries, entries...)
		}
		groups = map[string][]model.WeakpassEntry{
			req.Service: allEntries,
		}
	}

	// 导入每个分组
	for service, entries := range groups {
		if len(entries) == 0 {
			continue
		}

		// 生成字典名称
		dictName := req.Name
		if dictName == "" {
			if service != "common" {
				dictName = fmt.Sprintf("%s-导入", strings.ToUpper(service))
			} else {
				dictName = fmt.Sprintf("通用-导入-%d", time.Now().Unix())
			}
		}

		// 检查是否已存在
		existing, _ := dictModel.FindByName(l.ctx, dictName)

		// 生成内容
		content := generateWeakpassContent(entries)

		// 计算词条数
		wordCount := countWeakpassCombinations(content)

		if existing != nil {
			// 合并更新
			if req.MergeSame {
				// 合并现有内容和导入内容
				existingEntries := model.ParseWeakpassDict(existing.Content)
				merged := mergeEntries(existingEntries, entries)
				content = generateWeakpassContent(merged)
				wordCount = countWeakpassCombinations(content)

				dict := &model.WeakpassDict{
					Description: existing.Description,
					Service:     existing.Service,
					Content:     content,
					WordCount:   wordCount,
					Enabled:     existing.Enabled,
				}
				if err := dictModel.Update(l.ctx, existing.Id.Hex(), dict); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("更新字典'%s'失败: %v", dictName, err))
					continue
				}
				result.Updated++
			} else {
				// 跳过
				result.Skipped++
				continue
			}
		} else {
			// 新增
			dict := &model.WeakpassDict{
				Name:        dictName,
				Description: fmt.Sprintf("导入的弱口令字典，包含 %d 个词条", len(entries)),
				Service:     service,
				Content:     content,
				WordCount:   wordCount,
				Enabled:     true,
				IsBuiltin:   false,
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := dictModel.Insert(l.ctx, dict); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("插入字典'%s'失败: %v", dictName, err))
				continue
			}
			result.Imported++
		}
	}

	result.Msg = fmt.Sprintf("导入完成: 新增%d, 更新%d, 跳过%d", result.Imported, result.Updated, result.Skipped)

	return result, nil
}

// WeakpassDictExportLogic 导出弱口令字典逻辑
type WeakpassDictExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictExportLogic {
	return &WeakpassDictExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WeakpassDictExport 导出弱口令字典
func (l *WeakpassDictExportLogic) WeakpassDictExport(req *types.WeakpassDictExportReq) (*types.WeakpassDictExportResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	var dicts []model.WeakpassDict
	var err error

	if len(req.Ids) > 0 {
		// 按ID导出
		dicts, err = dictModel.FindByIds(l.ctx, req.Ids)
	} else {
		// 导出全部
		dicts, err = dictModel.FindEnabled(l.ctx, "")
	}
	if err != nil {
		return nil, err
	}

	format := strings.ToLower(req.Format)
	if format == "" {
		format = "simple"
	}

	var content string
	switch format {
	case "grouped":
		// 分组格式
		content = generateGroupedContent(dicts)
	case "merged":
		// 合并为一个字典
		content = mergeAllContent(dicts, req.Name)
	default:
		// 简单格式
		content = mergeAllContent(dicts, "")
	}

	return &types.WeakpassDictExportResp{
		Code:    0,
		Msg:     "success",
		Content: content,
	}, nil
}

// WeakpassDictParseLogic 解析弱口令字典逻辑
type WeakpassDictParseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictParseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictParseLogic {
	return &WeakpassDictParseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WeakpassDictParse 解析弱口令字典内容（预览用）
func (l *WeakpassDictParseLogic) WeakpassDictParse(req *types.WeakpassDictParseReq) (*types.WeakpassDictParseResp, error) {
	format := strings.ToLower(req.Format)
	if format == "" || format == "auto" {
		format = detectWeakpassFormat(req.Content)
	}

	resp := &types.WeakpassDictParseResp{
		Groups: []types.WeakpassDictGroup{},
	}

	lines := strings.Split(req.Content, "\n")
	resp.TotalLines = len(lines)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			resp.EmptyLines++
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			resp.CommentLines++
			continue
		}
		resp.ValidLines++
	}

	var groups map[string][]model.WeakpassEntry
	switch format {
	case "grouped":
		groups = model.ParseGroupedWeakpassDict(req.Content)
	default:
		entries := model.ParseWeakpassDict(req.Content)
		groups = map[string][]model.WeakpassEntry{
			"common": entries,
		}
	}

	for service, entries := range groups {
		group := types.WeakpassDictGroup{
			Service:   service,
			LineCount: len(entries),
			Preview:   []string{},
		}
		// 预览前5条
		maxPreview := 5
		if len(entries) < maxPreview {
			maxPreview = len(entries)
		}
		for i := 0; i < maxPreview; i++ {
			entry := entries[i]
			if entry.Password == "" {
				group.Preview = append(group.Preview, fmt.Sprintf("%s:", entry.Username))
			} else {
				group.Preview = append(group.Preview, fmt.Sprintf("%s:%s", entry.Username, entry.Password))
			}
		}
		resp.Groups = append(resp.Groups, group)
	}

	resp.Code = 0
	resp.Msg = "success"

	return resp, nil
}

// WeakpassDictServiceStatsLogic 服务类型统计逻辑
type WeakpassDictServiceStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeakpassDictServiceStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeakpassDictServiceStatsLogic {
	return &WeakpassDictServiceStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WeakpassDictServiceStats 获取服务类型统计
func (l *WeakpassDictServiceStatsLogic) WeakpassDictServiceStats() (*types.WeakpassDictServiceStatsResp, error) {
	dictModel := model.NewWeakpassDictModel(l.svcCtx.MongoDB)

	// 获取所有启用的字典
	dicts, err := dictModel.FindEnabled(l.ctx, "")
	if err != nil {
		return nil, err
	}

	// 按服务分组统计
	statsMap := make(map[string]struct {
		DictCount int
		WordCount int
	})

	for _, d := range dicts {
		if _, ok := statsMap[d.Service]; !ok {
			statsMap[d.Service] = struct {
				DictCount int
				WordCount int
			}{}
		}
		stats := statsMap[d.Service]
		stats.DictCount++
		stats.WordCount += d.WordCount
		statsMap[d.Service] = stats
	}

	// 转换为响应
	stats := make([]types.WeakpassDictServiceStat, 0, len(statsMap))
	for service, stat := range statsMap {
		stats = append(stats, types.WeakpassDictServiceStat{
			Service:   service,
			DictCount: stat.DictCount,
			WordCount: stat.WordCount,
		})
	}

	return &types.WeakpassDictServiceStatsResp{
		Code:  0,
		Msg:   "success",
		Stats: stats,
	}, nil
}

// Helper functions

// detectWeakpassFormat 检测弱口令字典格式
func detectWeakpassFormat(content string) string {
	lines := strings.Split(content, "\n")
	hasGroupMarker := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			hasGroupMarker = true
			break
		}
	}

	if hasGroupMarker {
		return "grouped"
	}
	return "simple"
}

// generateWeakpassContent 生成弱口令字典内容
func generateWeakpassContent(entries []model.WeakpassEntry) string {
	var lines []string
	for _, entry := range entries {
		if entry.Password == "" {
			lines = append(lines, entry.Username+":")
		} else {
			lines = append(lines, entry.Username+":"+entry.Password)
		}
	}
	return strings.Join(lines, "\n")
}

// generateGroupedContent 生成分组格式的字典内容
func generateGroupedContent(dicts []model.WeakpassDict) string {
	var lines []string

	// 按服务分组
	groups := make(map[string][]string)
	for _, dict := range dicts {
		entries := model.ParseWeakpassDict(dict.Content)
		for _, entry := range entries {
			var line string
			if entry.Password == "" {
				line = entry.Username + ":"
			} else {
				line = entry.Username + ":" + entry.Password
			}
			groups[dict.Service] = append(groups[dict.Service], line)
		}
	}

	// 生成输出
	for service, serviceLines := range groups {
		if service != "common" {
			lines = append(lines, "["+service+"]")
		}
		lines = append(lines, serviceLines...)
		lines = append(lines, "") // 空行分隔
	}

	return strings.Join(lines, "\n")
}

// mergeAllContent 合并所有字典内容
func mergeAllContent(dicts []model.WeakpassDict, name string) string {
	var lines []string
	seen := make(map[string]bool)

	for _, dict := range dicts {
		entries := model.ParseWeakpassDict(dict.Content)
		for _, entry := range entries {
			var line string
			if entry.Password == "" {
				line = entry.Username + ":"
			} else {
				line = entry.Username + ":" + entry.Password
			}
			if !seen[line] {
				seen[line] = true
				lines = append(lines, line)
			}
		}
	}

	if name != "" {
		header := fmt.Sprintf("# 字典名称: %s\n# 词条数量: %d\n\n", name, len(lines))
		return header + strings.Join(lines, "\n")
	}

	return strings.Join(lines, "\n")
}

// mergeEntries 合并弱口令条目
func mergeEntries(existing, new []model.WeakpassEntry) []model.WeakpassEntry {
	seen := make(map[string]bool)
	var result []model.WeakpassEntry

	for _, entry := range existing {
		key := entry.Username + ":" + entry.Password
		seen[key] = true
		result = append(result, entry)
	}

	for _, entry := range new {
		key := entry.Username + ":" + entry.Password
		if !seen[key] {
			seen[key] = true
			result = append(result, entry)
		}
	}

	return result
}

// countWeakpassCombinations 计算弱口令字典中的"用户名:密码"组合数量
func countWeakpassCombinations(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释行
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}
