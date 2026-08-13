package logic

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type SiteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSiteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteLogic {
	return &SiteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// resolveAssetIP 解析资产的展示 IP 与归属地
// host 字段对 Web 资产可能保存的是域名，真实 IP 存于 ip.ipv4/ip.ipv6；仅当 host 本身是 IP 字面量时才回退到 host
func resolveAssetIP(ip model.IP, host string) (string, string) {
	for _, v4 := range ip.IpV4 {
		if v4.IPName != "" {
			return v4.IPName, v4.Location
		}
	}
	for _, v6 := range ip.IpV6 {
		if v6.IPName != "" {
			return v6.IPName, v6.Location
		}
	}
	if common.IsIPAddress(host) {
		return host, ""
	}
	return "", ""
}

// SiteList 站点列表 - 只返回Web资产（HTTP/HTTPS服务）
// 判断条件：is_http=true 或 service=http/https 或 有title 或 有screenshot
func (l *SiteLogic) SiteList(req *types.SiteListReq) (*types.SiteListResp, error) {
	resp := &types.SiteListResp{Code: 0, List: []types.Site{}}

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)

	var allSites []types.Site
	totalCount := 0

	assetModel := model.NewAssetModel(l.svcCtx.MongoDB)

		// 构建Web资产查询条件
		// Web资产判断：确保是有效的站点资产，有实际的Web特征(is_http, title, status, screenshot 等)
		// 去除了纯端口或推测服务判断，过滤掉无实际HTTP响应的无效资产
		webFilter := bson.M{
			"$or": []bson.M{
				{"is_http": true},
				{"title": bson.M{"$exists": true, "$ne": ""}},
				{"status": bson.M{"$exists": true, "$nin": []string{"", "0"}}},
				{"screenshot": bson.M{"$exists": true, "$ne": ""}},
			},
		}

		// 额外搜索条件
		filter := bson.M{}
		conditions := []bson.M{webFilter}

		// 通用 Query 模糊搜索（当未指定具体字段时）
		if req.Query != "" && req.Site == "" && req.Title == "" && req.App == "" {
			q := regexp.QuoteMeta(req.Query)
			conditions = append(conditions, bson.M{
				"$or": []bson.M{
					{"authority": bson.M{"$regex": q, "$options": "i"}},
					{"host": bson.M{"$regex": q, "$options": "i"}},
					{"title": bson.M{"$regex": q, "$options": "i"}},
				},
			})
		}

		if req.Site != "" {
			siteQuery := req.Site
			if strings.HasPrefix(siteQuery, "http://") {
				siteQuery = strings.TrimPrefix(siteQuery, "http://")
			} else if strings.HasPrefix(siteQuery, "https://") {
				siteQuery = strings.TrimPrefix(siteQuery, "https://")
			}
			siteQuery = regexp.QuoteMeta(siteQuery)
			conditions = append(conditions, bson.M{
				"$or": []bson.M{
					{"authority": bson.M{"$regex": siteQuery, "$options": "i"}},
					{"host": bson.M{"$regex": siteQuery, "$options": "i"}},
				},
			})
		}
		if req.Title != "" {
			titleQuery := regexp.QuoteMeta(req.Title)
			conditions = append(conditions, bson.M{"title": bson.M{"$regex": titleQuery, "$options": "i"}})
		}
		if req.App != "" {
			appQuery := regexp.QuoteMeta(req.App)
			conditions = append(conditions, bson.M{"app": bson.M{"$regex": appQuery, "$options": "i"}})
		}
		if req.HttpStatus != "" {
			conditions = append(conditions, bson.M{"status": req.HttpStatus})
		}
		if req.OrgId != "" {
			conditions = append(conditions, bson.M{"org_id": req.OrgId})
		}

		if len(conditions) > 1 {
			filter["$and"] = conditions
		} else {
			filter = webFilter
		}

		// 统计总数
		count, _ := assetModel.Count(l.ctx, filter)
		totalCount += int(count)

		// 查询数据（保留 icon_hash_bytes 用于展示 favicon）
		req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
		assets, err := assetModel.FindForSite(l.ctx, filter, req.Page, req.PageSize)
		if err != nil {
			l.Logger.Errorf("SiteList 查询资产失败: %v", err)
		}

		for _, asset := range assets {
			site := types.Site{
				Id:         asset.Id.Hex(),
				Title:      asset.Title,
				Port:       asset.Port,
				Service:    asset.Service,
				HttpStatus: asset.HttpStatus,
				App:        asset.App,
				Labels:     asset.Labels,
				Screenshot: asset.Screenshot,
				OrgId:      asset.OrgId,
				HttpHeader: asset.HttpHeader,
				IconHash:   asset.IconHash,
				ColorTag:   asset.ColorTag,
				Memo:       asset.Memo,
			}

			// IP 与归属地：host 对 Web 资产可能是域名，真实 IP 存于 ip.ipv4/ip.ipv6
			site.IP, site.Location = resolveAssetIP(asset.Ip, asset.Host)

			// favicon 图片数据
			if len(asset.IconHashBytes) > 0 && isValidImageBytes(asset.IconHashBytes) {
				site.IconHashBytes = base64.StdEncoding.EncodeToString(asset.IconHashBytes)
			}

			// 构建站点URL
			scheme := "http"
			if asset.Service == "https" || asset.Port == 443 || asset.Port == 8443 {
				scheme = "https"
			}
			if asset.Authority != "" {
				site.Site = fmt.Sprintf("%s://%s", scheme, asset.Authority)
			} else {
				site.Site = fmt.Sprintf("%s://%s:%d", scheme, asset.Host, asset.Port)
			}

			// 组织名称
			if asset.OrgId != "" {
				site.OrgName = orgMap[asset.OrgId]
			}

			site.UpdateTime = asset.UpdateTime.Local().Format("2006-01-02 15:04:05")
			site.CreateTime = asset.CreateTime.Local().Format("2006-01-02 15:04:05")
			allSites = append(allSites, site)
	}

	resp.Total = totalCount
	resp.List = allSites
	return resp, nil
}

// SiteDelete 删除站点（实际删除对应的资产）
func (l *SiteLogic) SiteDelete(req *types.SiteDeleteReq) (*types.BaseResp, error) {
	assetModel := model.NewAssetModel(l.svcCtx.MongoDB)
	asset, err := assetModel.FindById(l.ctx, req.Id)
	if err == nil && asset != nil {
		if err = assetModel.Delete(l.ctx, req.Id); err == nil {
			return &types.BaseResp{Code: 0, Msg: "删除成功"}, nil
		}
	}

	return &types.BaseResp{Code: 500, Msg: "删除失败，资产不存在"}, nil
}

// SiteBatchDelete 批量删除站点
func (l *SiteLogic) SiteBatchDelete(req *types.SiteBatchDeleteReq) (*types.BaseResp, error) {
	if len(req.Ids) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的站点"}, nil
	}

	assetModel := model.NewAssetModel(l.svcCtx.MongoDB)
	totalDeleted, _ := assetModel.BatchDelete(l.ctx, req.Ids)

	if totalDeleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败，未找到匹配的站点"}, nil
	}

	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.Itoa(int(totalDeleted)) + " 个站点"}, nil
}

// SiteStat 站点统计
// 优化点：
//  1. 原实现每个 ws 跑 4 次 Count（4×N 次 collection scan），现改用 $facet 一次聚合（N 次）
//  2. 整体结果走 60s 缓存
func (l *SiteLogic) SiteStat() (*types.SiteStatResp, error) {
	cacheKey := "site_stat"
	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		resp := &types.SiteStatResp{Code: 0}

		// Web资产过滤条件
		webFilter := bson.M{
			"$or": []bson.M{
				{"is_http": true},
				{"service": bson.M{"$in": []string{"http", "https"}}},
				{"title": bson.M{"$exists": true, "$ne": ""}},
				{"screenshot": bson.M{"$exists": true, "$ne": ""}},
			},
		}
		httpFilter := bson.M{
			"$or": []bson.M{
				{"service": "http"},
				{"port": 80},
			},
		}
		httpsFilter := bson.M{
			"$or": []bson.M{
				{"service": "https"},
				{"port": 443},
			},
		}
		newFilter := bson.M{
			"$expr": bson.M{"$gte": bson.A{
				bson.M{"$ifNull": bson.A{"$first_seen_time", "$create_time"}},
				time.Now().AddDate(0, 0, -1),
			}},
		}

		assetModel := l.svcCtx.GetAssetModel()
		stats, statErr := assetModel.AggregateSiteStats(l.ctx, webFilter, httpFilter, httpsFilter, newFilter)
		if statErr != nil {
			l.Logger.Errorf("SiteStat 聚合失败: %v", statErr)
		} else {
			resp.Total = int(stats.Total)
			resp.HttpCount = int(stats.Http)
			resp.HttpsCount = int(stats.Https)
			resp.NewCount = int(stats.NewCount)
		}

		return resp, nil
	})
	if err != nil {
		return &types.SiteStatResp{Code: 0}, nil
	}
	if resp, ok := cached.(*types.SiteStatResp); ok {
		return resp, nil
	}
	return &types.SiteStatResp{Code: 0}, nil
}
