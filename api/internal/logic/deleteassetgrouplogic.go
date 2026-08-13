package logic

import (
	"context"
	"strings"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type DeleteAssetGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAssetGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAssetGroupLogic {
	return &DeleteAssetGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteAssetGroup 删除资产分组及其下的所有资产和相关任务
func (l *DeleteAssetGroupLogic) DeleteAssetGroup(req *types.DeleteAssetGroupReq) (resp *types.DeleteAssetGroupResp, err error) {
	l.Logger.Infof("DeleteAssetGroup: domain=%s", req.Domain)

	if req.Domain == "" {
		return &types.DeleteAssetGroupResp{
			Code: 1,
			Msg:  "域名不能为空",
		}, nil
	}

	totalDeleted := int64(0)
	totalTasksDeleted := int64(0)

	// 1. 删除资产
	assetModel := l.svcCtx.GetAssetModel()

	// 先查询所有资产，然后筛选出属于该分组的资产
	// 这样可以确保使用和分组查询相同的逻辑
	allAssets, err := assetModel.Find(l.ctx, bson.M{}, 0, 0)
	if err != nil {
		l.Logger.Errorf("查询资产失败: %v", err)
	}

	// 收集需要删除的资产ID
	var idsToDelete []string
	// 记录被删除资产的host，用于清理历史记录
	var hostsToDelete []string
	for _, asset := range allAssets {
		// 使用和分组查询相同的逻辑提取主域名
		mainDomain := extractMainDomainForDelete(asset.Host)
		if mainDomain == req.Domain {
			idsToDelete = append(idsToDelete, asset.Id.Hex())
			hostsToDelete = append(hostsToDelete, asset.Host)
		}
	}

	l.Logger.Infof("找到 %d 个匹配的资产", len(idsToDelete))

	// 批量删除资产
	if len(idsToDelete) > 0 {
		deletedCount, err := assetModel.BatchDelete(l.ctx, idsToDelete)
		if err != nil {
			l.Logger.Errorf("删除资产失败: %v", err)
		} else {
			l.Logger.Infof("删除了 %d 个资产", deletedCount)
			totalDeleted += deletedCount
		}

		// 删除资产历史记录（ScanResultHistoryModel）
		historyModel := model.NewScanResultHistoryModel(l.svcCtx.MongoDB)
		historyDeleted := int64(0)
		for _, host := range hostsToDelete {
			// 按 host 删除历史记录
			filter := bson.M{"host": host}
			result, err := historyModel.DeleteByFilter(l.ctx, filter)
			if err != nil {
				l.Logger.Errorf("删除资产历史记录失败: host=%s, err=%v", host, err)
			} else {
				historyDeleted += result
			}
		}
		l.Logger.Infof("已清理 %d 条资产历史记录", historyDeleted)

		// 删除 JSFinder 结果
		jsfinderModel := l.svcCtx.GetJSFinderResultModel()
		jsfinderDeleted := int64(0)
		for _, host := range hostsToDelete {
			filter := bson.M{"host": host}
			result, err := jsfinderModel.DeleteMany(l.ctx, filter)
			if err != nil {
				l.Logger.Errorf("删除 JSFinder 结果失败: host=%s, err=%v", host, err)
			} else {
				jsfinderDeleted += result
			}
		}
		l.Logger.Infof("已清理 %d 条 JSFinder 结果", jsfinderDeleted)
	}

	// 2. 删除相关任务
	taskModel := l.svcCtx.GetMainTaskModel()

	// 查询所有任务
	allTasks, err := taskModel.Find(l.ctx, bson.M{}, 0, 0)
	if err != nil {
		l.Logger.Errorf("查询任务失败: %v", err)
	}

	// 收集需要删除的任务ID
	var taskIdsToDelete []string
	for _, task := range allTasks {
		// 从任务目标中提取域名
		targets := strings.Split(task.Target, "\n")
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}

			// 提取主域名
			domain := extractMainDomainFromTargetForDelete(target)
			if domain == req.Domain {
				taskIdsToDelete = append(taskIdsToDelete, task.Id.Hex())
				break // 找到匹配的目标就跳出，避免重复添加
			}
		}
	}

	l.Logger.Infof("找到 %d 个匹配的任务", len(taskIdsToDelete))

	// 批量删除任务
	if len(taskIdsToDelete) > 0 {
		deletedTaskCount, err := taskModel.BatchDelete(l.ctx, taskIdsToDelete)
		if err != nil {
			l.Logger.Errorf("删除任务失败: %v", err)
		} else {
			l.Logger.Infof("删除了 %d 个任务", deletedTaskCount)
			totalTasksDeleted += deletedTaskCount
		}
	}

	return &types.DeleteAssetGroupResp{
		Code:         0,
		Msg:          "删除成功",
		DeletedCount: totalDeleted,
	}, nil
}

// extractMainDomainForDelete 从主机名中提取主域名（与assetgroupslogic.go中的逻辑保持一致）
func extractMainDomainForDelete(host string) string {
	// 如果是IP地址，返回IP
	if isIPAddressForDelete(host) {
		return host
	}

	// 分割域名
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}

	// 返回主域名（最后两部分）
	return strings.Join(parts[len(parts)-2:], ".")
}

// extractMainDomainFromTargetForDelete 从任务目标中提取主域名
func extractMainDomainFromTargetForDelete(target string) string {
	// 移除协议前缀
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")

	// 移除端口
	if idx := strings.Index(target, ":"); idx > 0 {
		target = target[:idx]
	}

	// 移除路径
	if idx := strings.Index(target, "/"); idx > 0 {
		target = target[:idx]
	}

	// 移除通配符
	target = strings.TrimPrefix(target, "*.")

	// 移除CIDR
	if strings.Contains(target, "/") {
		return "" // CIDR不作为域名分组
	}

	// 如果是IP地址，返回IP
	if isIPAddressForDelete(target) {
		return target
	}

	// 提取主域名
	parts := strings.Split(target, ".")
	if len(parts) < 2 {
		return target
	}

	// 返回主域名（最后两部分）
	return strings.Join(parts[len(parts)-2:], ".")
}

// isIPAddressForDelete 判断是否为IP地址
func isIPAddressForDelete(host string) bool {
	// 简单判断：包含数字和点
	for _, c := range host {
		if (c >= '0' && c <= '9') || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return strings.Contains(host, ".")
}
