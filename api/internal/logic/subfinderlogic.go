package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"go.mongodb.org/mongo-driver/bson"
)

type SubfinderLogic struct {
	ctx context.Context
	svc *svc.ServiceContext
}

func NewSubfinderLogic(ctx context.Context, svc *svc.ServiceContext) *SubfinderLogic {
	return &SubfinderLogic{ctx: ctx, svc: svc}
}

// ProviderList 获取所有Subfinder数据源配置
func (l *SubfinderLogic) ProviderList() (*types.SubfinderProviderListResp, error) {
	providerModel := model.NewSubfinderProviderModel(l.svc.MongoDB)
	docs, err := providerModel.FindAll(l.ctx)
	if err != nil {
		return &types.SubfinderProviderListResp{Code: 500, Msg: "查询失败"}, nil
	}

	list := make([]types.SubfinderProvider, 0, len(docs))
	for _, doc := range docs {
		// 对密钥进行脱敏处理
		maskedKeys := make([]string, len(doc.Keys))
		for i, key := range doc.Keys {
			maskedKeys[i] = maskKey(key)
		}
		list = append(list, types.SubfinderProvider{
			Id:          doc.Id.Hex(),
			Provider:    doc.Provider,
			Keys:        maskedKeys,
			Status:      doc.Status,
			Description: doc.Description,
			CreateTime:  doc.CreateTime.Local().Format("2006-01-02 15:04:05"),
			UpdateTime:  doc.UpdateTime.Local().Format("2006-01-02 15:04:05"),
		})
	}

	return &types.SubfinderProviderListResp{Code: 0, Msg: "success", List: list}, nil
}

// ProviderSave 保存Subfinder数据源配置
func (l *SubfinderLogic) ProviderSave(req *types.SubfinderProviderSaveReq) (*types.BaseResp, error) {
	providerModel := model.NewSubfinderProviderModel(l.svc.MongoDB)

	// 如果keys为空，只更新状态，不更新密钥
	if len(req.Keys) == 0 {
		// 查找现有配置
		existing, err := providerModel.FindByProvider(l.ctx, req.Provider)
		if err == nil && existing != nil {
			// 只更新状态
			update := bson.M{
				"status":      req.Status,
				"description": req.Description,
			}
			if err := providerModel.Update(l.ctx, existing.Id.Hex(), update); err != nil {
				return &types.BaseResp{Code: 500, Msg: "更新失败: " + err.Error()}, nil
			}
			return &types.BaseResp{Code: 0, Msg: "保存成功"}, nil
		}
		// 如果不存在且没有密钥，返回错误
		return &types.BaseResp{Code: 400, Msg: "请输入API密钥"}, nil
	}

	doc := &model.SubfinderProvider{
		Provider:    req.Provider,
		Keys:        req.Keys,
		Status:      req.Status,
		Description: req.Description,
	}

	if err := providerModel.Upsert(l.ctx, doc); err != nil {
		return &types.BaseResp{Code: 500, Msg: "保存失败: " + err.Error()}, nil
	}

	return &types.BaseResp{Code: 0, Msg: "保存成功"}, nil
}

// ProviderInfo 获取所有支持的数据源信息
func (l *SubfinderLogic) ProviderInfo() (*types.SubfinderProviderInfoResp, error) {
	// 从model中获取预定义的数据源信息
	list := make([]types.SubfinderProviderMeta, 0, len(model.SubfinderProviderInfo))
	for _, info := range model.SubfinderProviderInfo {
		list = append(list, types.SubfinderProviderMeta{
			Provider:    info.Provider,
			Name:        info.Name,
			Description: info.Description,
			KeyFormat:   info.KeyFormat,
			URL:         info.URL,
		})
	}

	return &types.SubfinderProviderInfoResp{Code: 0, Msg: "success", List: list}, nil
}

// maskKey 对密钥进行脱敏处理
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
