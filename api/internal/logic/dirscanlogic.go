package logic

import (
	"context"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// DirScanDictListLogic 目录扫描字典列表逻辑
type DirScanDictListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanDictListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanDictListLogic {
	return &DirScanDictListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DirScanDictListLogic) DirScanDictList(req *types.DirScanDictListReq) (*types.DirScanDictListResp, error) {
	dictModel := model.NewDirScanDictModel(l.svcCtx.MongoDB)

	// 获取列表
	dicts, err := dictModel.FindAll(l.ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	// 获取总数（空 filter 场景使用 EstimatedDocumentCount，O(1) 元数据查询）
	total, err := dictModel.EstimatedCount(l.ctx)
	if err != nil {
		return nil, err
	}

	// 转换为响应类型
	list := make([]types.DirScanDict, 0, len(dicts))
	for _, d := range dicts {
		list = append(list, types.DirScanDict{
			Id:          d.Id.Hex(),
			Name:        d.Name,
			Description: d.Description,
			Content:     d.Content,
			PathCount:   d.PathCount,
			Enabled:     d.Enabled,
			IsBuiltin:   d.IsBuiltin,
			CreateTime:  d.CreateTime.Format("2006-01-02 15:04:05"),
			UpdateTime:  d.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.DirScanDictListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}

// DirScanDictSaveLogic 保存目录扫描字典逻辑
type DirScanDictSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanDictSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanDictSaveLogic {
	return &DirScanDictSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DirScanDictSaveLogic) DirScanDictSave(req *types.DirScanDictSaveReq) (*types.BaseRespWithId, error) {
	dictModel := model.NewDirScanDictModel(l.svcCtx.MongoDB)

	// 计算路径数量
	pathCount := countPaths(req.Content)

	if req.Id != "" {
		// 更新
		dict := &model.DirScanDict{
			Name:        req.Name,
			Description: req.Description,
			Content:     req.Content,
			PathCount:   pathCount,
			Enabled:     req.Enabled,
		}
		if err := dictModel.Update(l.ctx, req.Id, dict); err != nil {
			return nil, err
		}
		return &types.BaseRespWithId{Code: 0, Msg: "success", Id: req.Id}, nil
	}

	// 新增
	dict := &model.DirScanDict{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		PathCount:   pathCount,
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

// DirScanDictDeleteLogic 删除目录扫描字典逻辑
type DirScanDictDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanDictDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanDictDeleteLogic {
	return &DirScanDictDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DirScanDictDeleteLogic) DirScanDictDelete(req *types.DirScanDictDeleteReq) (*types.BaseResp, error) {
	dictModel := model.NewDirScanDictModel(l.svcCtx.MongoDB)

	if err := dictModel.Delete(l.ctx, req.Id); err != nil {
		return nil, err
	}

	return &types.BaseResp{Code: 0, Msg: "success"}, nil
}

// DirScanDictClearLogic 清空目录扫描字典逻辑
type DirScanDictClearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanDictClearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanDictClearLogic {
	return &DirScanDictClearLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DirScanDictClearLogic) DirScanDictClear() (*types.DirScanDictClearResp, error) {
	dictModel := model.NewDirScanDictModel(l.svcCtx.MongoDB)

	// 只删除非内置字典
	deleted, err := dictModel.DeleteNonBuiltin(l.ctx)
	if err != nil {
		return nil, err
	}

	return &types.DirScanDictClearResp{
		Code:    0,
		Msg:     "success",
		Deleted: int(deleted),
	}, nil
}

// DirScanDictEnabledListLogic 获取启用的目录扫描字典列表逻辑
type DirScanDictEnabledListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirScanDictEnabledListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirScanDictEnabledListLogic {
	return &DirScanDictEnabledListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DirScanDictEnabledListLogic) DirScanDictEnabledList() (*types.DirScanDictEnabledListResp, error) {
	dictModel := model.NewDirScanDictModel(l.svcCtx.MongoDB)

	dicts, err := dictModel.FindEnabled(l.ctx)
	if err != nil {
		return nil, err
	}

	list := make([]types.DirScanDictSimple, 0, len(dicts))
	for _, d := range dicts {
		list = append(list, types.DirScanDictSimple{
			Id:        d.Id.Hex(),
			Name:      d.Name,
			PathCount: d.PathCount,
			IsBuiltin: d.IsBuiltin,
		})
	}

	return &types.DirScanDictEnabledListResp{
		Code: 0,
		Msg:  "success",
		List: list,
	}, nil
}

// countPaths 计算字典中的路径数量
func countPaths(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}
