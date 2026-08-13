package logic

import "cscan/internal/model"

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// assetGroupsRecentTaskLimit 推断域名状态时查询的最近任务条数。
	// 域名数量通常远小于任务数量，最近 50 条足以覆盖绝大多数域名的最新状态。
	assetGroupsRecentTaskLimit = 50
	// assetGroupsCacheTTL 分组结果缓存时长，平衡刷新延迟与重复扫描开销。
	assetGroupsCacheTTL = 30 * time.Second
)

type AssetGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetGroupsLogic {
	return &AssetGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetGroups 获取按域名分组的资产统计
func (l *AssetGroupsLogic) AssetGroups(req *types.AssetGroupsReq) (resp *types.AssetGroupsResp, err error) {
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	cacheKey := fmt.Sprintf("asset_groups:%d:%d:%s", req.Page, req.PageSize, req.Query)
	cached, cerr := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, assetGroupsCacheTTL, func() (interface{}, error) {
		return l.buildAssetGroups(req)
	})
	if cerr != nil {
		l.Logger.Errorf("AssetGroups 缓存读取失败: %v", cerr)
		return l.buildAssetGroups(req)
	}
	if r, ok := cached.(*types.AssetGroupsResp); ok && r != nil {
		return r, nil
	}
	return l.buildAssetGroups(req)
}

func (l *AssetGroupsLogic) buildAssetGroups(req *types.AssetGroupsReq) (*types.AssetGroupsResp, error) {
	// domain -> group
	domainGroups := make(map[string]*types.AssetGroup)
	// domain -> 最新任务的执行时长
	domainDuration := make(map[string]time.Duration)

	// 1. 资产聚合：只投影 host/domain/create_time/update_time，避免大字段加载
	assetModel := l.svcCtx.GetAssetModel()
	rows, err := assetModel.AggregateGroupByDomain(l.ctx)
	if err != nil {
		l.Logger.Errorf("查询资产聚合失败: %v", err)
	} else {
		for _, row := range rows {
			domain := resolveRootDomain(row.Host, row.Domain)
			if domain == "" {
				continue
			}
			group, exists := domainGroups[domain]
			if !exists {
				group = &types.AssetGroup{
					Domain:       domain,
					Source:       "Auto Discovery",
					Status:       "finished", // 仅有资产无任务时默认已完成
					FirstSeen:    row.CreateTime,
					LatestUpdate: row.UpdateTime,
				}
				domainGroups[domain] = group
			}
			group.TotalServices++
			if row.CreateTime.Before(group.FirstSeen) {
				group.FirstSeen = row.CreateTime
			}
			if row.UpdateTime.After(group.LatestUpdate) {
				group.LatestUpdate = row.UpdateTime
			}
		}
	}

	// 2. 任务状态推断：仅查最近 N 条任务，按 update_time 降序覆盖域名状态
	taskModel := l.svcCtx.GetMainTaskModel()
	tasks, err := taskModel.FindRecent(l.ctx, assetGroupsRecentTaskLimit)
	if err != nil {
		l.Logger.Errorf("查询最近任务失败: %v", err)
	} else {
		domainStatusSet := make(map[string]struct{})
		for _, task := range tasks {
			for _, target := range strings.Split(task.Target, "\n") {
				target = strings.TrimSpace(target)
				if target == "" {
					continue
				}
				domain := extractMainDomainFromTarget(target)
				if domain == "" {
					continue
				}
				if _, seen := domainStatusSet[domain]; seen {
					continue
				}
				domainStatusSet[domain] = struct{}{}

				status := getTaskStatusForGroup(task.Status)
				duration := time.Duration(0)
				if task.StartTime != nil && task.EndTime != nil {
					duration = task.EndTime.Sub(*task.StartTime)
				} else if task.StartTime != nil && task.Status == "STARTED" {
					duration = time.Since(*task.StartTime)
				}
				domainDuration[domain] = duration

				group, exists := domainGroups[domain]
				if !exists {
					domainGroups[domain] = &types.AssetGroup{
						Domain:       domain,
						Source:       "Auto Discovery",
						Status:       status,
						FirstSeen:    task.CreateTime,
						LatestUpdate: task.UpdateTime,
					}
				} else {
					group.Status = status
					if task.UpdateTime.After(group.LatestUpdate) {
						group.LatestUpdate = task.UpdateTime
					}
				}
			}
		}
	}

	// 3. 计算 Duration / LastUpdated
	for domain, group := range domainGroups {
		if d, ok := domainDuration[domain]; ok && d > 0 {
			group.Duration = formatDuration(d)
		} else {
			group.Duration = "-"
		}
		group.LastUpdated = formatRelativeTime(group.LatestUpdate)
	}

	// 4. 转列表 + 按服务数排序
	list := make([]types.AssetGroup, 0, len(domainGroups))
	for _, group := range domainGroups {
		list = append(list, *group)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TotalServices > list[j].TotalServices
	})

	// 5. Query 子串过滤（后端过滤）
	if q := strings.TrimSpace(req.Query); q != "" {
		filtered := make([]types.AssetGroup, 0, len(list))
		for _, g := range list {
			if strings.Contains(strings.ToLower(g.Domain), strings.ToLower(q)) {
				filtered = append(filtered, g)
			}
		}
		list = filtered
	}

	// 6. 分页
	total := len(list)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start >= total {
		list = []types.AssetGroup{}
	} else {
		if end > total {
			end = total
		}
		list = list[start:end]
	}

	return &types.AssetGroupsResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  list,
	}, nil
}

// resolveRootDomain 资产按域名分组：IP host 优先用 Domain 字段，否则用 host 提取根域名。
func resolveRootDomain(host, domain string) string {
	if isIPAddress(host) && domain != "" {
		return extractMainDomain(domain)
	}
	return extractMainDomain(host)
}

// formatDuration 格式化时间持续时长
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	} else if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm%ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}

// formatRelativeTime 将时间戳格式化为相对当前时间的字符串
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// extractMainDomainFromTarget 从任务目标中提取主域名
func extractMainDomainFromTarget(target string) string {
	info := utils.ParseTarget(target)
	if info.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(info.Host, "*.")
	if rootDomain := utils.GetRootDomain(host); rootDomain != "" {
		return rootDomain
	}
	return host
}

// getTaskStatusForGroup 将任务状态转换为分组状态
func getTaskStatusForGroup(taskStatus string) string {
	switch taskStatus {
	case "CREATED", "PENDING":
		return "starting"
	case "STARTED":
		return "running"
	case "SUCCESS":
		return "finished"
	case "FAILURE":
		return "failed"
	case "STOPPED", "REVOKED", "PAUSED":
		return "stopped"
	default:
		return "finished"
	}
}

// extractMainDomain 从主机名中提取主域名
func extractMainDomain(host string) string {
	if isIPAddress(host) {
		return host
	}
	if rootDomain := utils.GetRootDomain(host); rootDomain != "" {
		return rootDomain
	}
	return host
}

// isIPAddress 判断是否为IP地址
func isIPAddress(host string) bool {
	for _, c := range host {
		if (c >= '0' && c <= '9') || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return strings.Contains(host, ".")
}
