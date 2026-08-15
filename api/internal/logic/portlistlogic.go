package logic

import "regexp"

import "cscan/internal/model"

import (
	"context"
	"sort"

	"cscan/api/internal/logic/common"
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
		matchObj["host"] = bson.M{"$regex": regexp.QuoteMeta(req.Host), "$options": "i"}
	}
	if req.OrgId != "" {
		matchObj["org_id"] = req.OrgId
	}

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)

	assetModel := l.svcCtx.GetAssetModel()
	results, _, err := assetModel.AggregatePortList(l.ctx, matchObj, 0, 100000)
	if err != nil {
		l.Logger.Errorf("查询端口聚合失败: %v", err)
	}

	type wsPortItem struct {
		Port       int
		AssetCount int
		Hosts      []string
		Services   []string
		CreateTime string
		UpdateTime string
	}

	var allPorts []wsPortItem
	portItemMap := make(map[int]*wsPortItem)

	for _, r := range results {
		services := []string{}
		for _, s := range r.Services {
			if s != "" {
				services = append(services, s)
			}
		}

		if existing, ok := portItemMap[r.Port]; ok {
			existing.AssetCount += r.AssetCount
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
