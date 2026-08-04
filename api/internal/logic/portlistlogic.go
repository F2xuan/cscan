package logic
import "cscan/model"

import (
	"context"
	"sort"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type PortListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPortListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PortListLogic {
	return &PortListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PortListLogic) PortList(req *types.PortListReq) (*types.PortListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workspaceId := middleware.GetWorkspaceId(l.ctx)

	// Prepare match object
	matchObj := bson.M{"port": bson.M{"$gt": 0}}

	// Parsing general text grouping
	if req.Query != "" {
		parseQuerySyntax(req.Query, matchObj)
	}

	if req.Port > 0 {
		matchObj["port"] = req.Port
	}
	if req.Host != "" {
		matchObj["host"] = bson.M{"$regex": req.Host, "$options": "i"}
	}
	if req.OrgId != "" {
		matchObj["org_id"] = req.OrgId
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)

	// 修复跨 ws 分页 bug：原实现每个 ws 各自按 skip/limit 分页再 append，导致第 2 页变成各 ws 第 2 页的并集
	// 现改为：每个 ws 拉所有端口聚合结果（skip=0, limit=很大），内存全局合并 + 排序 + 分页
	// 多 ws 时正确返回全局第 N 页；单 ws 时行为不变
	type wsPortItem struct {
		Port       int
		AssetCount int
		Hosts      []string
		Services   []string
		CreateTime string
		UpdateTime string
	}

	var allPorts []wsPortItem
	portItemMap := make(map[int]*wsPortItem) // 按 port 合并多 ws 的统计

	for _, wsId := range wsIds {
		assetModel := l.svcCtx.GetAssetModel(wsId)
		// 拉所有端口聚合结果（不分页），用于跨 ws 全局合并
		results, _, err := assetModel.AggregatePortList(l.ctx, matchObj, 0, 100000)
		if err != nil {
			l.Logger.Errorf("查询工作空间 %s 端口聚合失败: %v", wsId, err)
			continue
		}

		for _, r := range results {
			services := []string{}
			for _, s := range r.Services {
				if s != "" {
					services = append(services, s)
				}
			}

			if existing, ok := portItemMap[r.Port]; ok {
				// 跨 ws 合并同一 port 的统计
				existing.AssetCount += r.AssetCount
				// hosts 去重合并
				hostSet := make(map[string]struct{}, len(existing.Hosts))
				for _, h := range existing.Hosts {
					hostSet[h] = struct{}{}
				}
				for _, h := range r.Hosts {
					if h == "" {
						continue
					}
					if _, exists := hostSet[h]; !exists {
						existing.Hosts = append(existing.Hosts, h)
						hostSet[h] = struct{}{}
					}
				}
				// services 去重合并
				svcSet := make(map[string]struct{}, len(existing.Services))
				for _, s := range existing.Services {
					svcSet[s] = struct{}{}
				}
				for _, s := range services {
					if _, exists := svcSet[s]; !exists {
						existing.Services = append(existing.Services, s)
						svcSet[s] = struct{}{}
					}
				}
				// createTime 取最早，updateTime 取最晚
				if ct := r.CreateTime.Local().Format("2006-01-02 15:04:05"); ct < existing.CreateTime {
					existing.CreateTime = ct
				}
				if ut := r.UpdateTime.Local().Format("2006-01-02 15:04:05"); ut > existing.UpdateTime {
					existing.UpdateTime = ut
				}
			} else {
				item := &wsPortItem{
					Port:       r.Port,
					AssetCount: r.AssetCount,
					Hosts:      r.Hosts,
					Services:   services,
					CreateTime: r.CreateTime.Local().Format("2006-01-02 15:04:05"),
					UpdateTime: r.UpdateTime.Local().Format("2006-01-02 15:04:05"),
				}
				portItemMap[r.Port] = item
				allPorts = append(allPorts, *item)
			}
		}
	}

	// 全局排序（按 port 升序，与 AggregatePortList 内部排序一致）
	sort.Slice(allPorts, func(i, j int) bool {
		return allPorts[i].Port < allPorts[j].Port
	})

	total := int64(len(allPorts))
	skip := (req.Page - 1) * req.PageSize
	limit := req.PageSize
	if skip > len(allPorts) {
		skip = len(allPorts)
	}
	end := skip + limit
	if end > len(allPorts) {
		end = len(allPorts)
	}

	pageItems := allPorts[skip:end]
	list := make([]types.PortListItem, 0, len(pageItems))
	for _, r := range pageItems {
		orgName := ""
		if req.OrgId != "" {
			orgName = orgMap[req.OrgId]
		}
		list = append(list, types.PortListItem{
			Port:       r.Port,
			AssetCount: r.AssetCount,
			Hosts:      r.Hosts,
			Services:   r.Services,
			OrgName:    orgName,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
		})
	}

	return &types.PortListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}
