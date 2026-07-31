package logic

import (
	"context"
	"time"

	"cscan/model"
	"cscan/pkg/utils"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SaveTaskResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveTaskResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveTaskResultLogic {
	return &SaveTaskResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SaveTaskResult 保存任务结果
func (l *SaveTaskResultLogic) SaveTaskResult(in *pb.SaveTaskResultReq) (*pb.SaveTaskResultResp, error) {
	if len(in.Assets) == 0 {
		return &pb.SaveTaskResultResp{
			Success: true,
			Message: "No assets to save",
		}, nil
	}

	workspaceId := in.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	assetModel := l.svcCtx.GetAssetModel(workspaceId)
	targetMetaModel := l.svcCtx.GetAssetTargetMetaModel(workspaceId)
	// T1.1: 变化基线快照模型，本次保存产出的新增/更新变化在此聚合后批量写入。
	diffModel := model.NewScanDiffModel(l.svcCtx.MongoDB, workspaceId)
	var diffs []model.ScanDiff
	// 登记顶层资产 meta 的 host 去重集合：同一次 SaveTaskResult 内同一目标只登记一次。
	seenTargets := make(map[string]struct{})

	var totalAsset, newAsset, updateAsset int32
	now := time.Now()

	for _, pbAsset := range in.Assets {
		// 转换为model.Asset
		asset := &model.Asset{
			Authority:     pbAsset.Authority,
			Host:          pbAsset.Host,
			Port:          int(pbAsset.Port),
			Category:      pbAsset.Category,
			Service:       pbAsset.Service,
			Title:         pbAsset.Title,
			App:           pbAsset.App,
			HttpStatus:    pbAsset.HttpStatus,
			HttpHeader:    pbAsset.HttpHeader,
			HttpBody:      pbAsset.HttpBody,
			IconHash:      pbAsset.IconHash,
			IconHashBytes: pbAsset.IconData,
			Screenshot:    pbAsset.Screenshot,
			Server:        pbAsset.Server,
			Banner:        pbAsset.Banner,
			IsHTTP:        pbAsset.IsHttp,
			TaskId:        in.MainTaskId,
			Source:        pbAsset.Source,
			OrgId:         in.OrgId,
		}

		// 如果Source为空，设置默认值
		if asset.Source == "" {
			asset.Source = "scan"
		}

		// 处理IP信息
		if len(pbAsset.Ipv4) > 0 {
			for _, ip := range pbAsset.Ipv4 {
				asset.Ip.IpV4 = append(asset.Ip.IpV4, model.IPV4{
					IPName:   ip.Ip,
					Location: ip.Location,
				})
			}
		} else if utils.IsIPv4(asset.Host) {
			asset.Ip.IpV4 = append(asset.Ip.IpV4, model.IPV4{
				IPName: asset.Host,
			})
		}

		if len(pbAsset.Ipv6) > 0 {
			for _, ip := range pbAsset.Ipv6 {
				asset.Ip.IpV6 = append(asset.Ip.IpV6, model.IPV6{
					IPName:   ip.Ip,
					Location: ip.Location,
				})
			}
		} else if utils.IsIPv6(asset.Host) {
			asset.Ip.IpV6 = append(asset.Ip.IpV6, model.IPV6{
				IPName: asset.Host,
			})
		}

		// 处理CName
		if pbAsset.Cname != "" {
			asset.CName = pbAsset.Cname
		}

		// 设置Domain字段 - 如果Host不是IP地址，则设置为Domain
		if asset.Category == "domain" || !utils.IsIPAddress(asset.Host) {
			asset.Domain = asset.Host
		}

		// ========= 尝试继承基础域名的IP和CName =========
		if asset.Port > 0 && len(asset.Ip.IpV4) == 0 && len(asset.Ip.IpV6) == 0 && !utils.IsIPAddress(asset.Host) {
			baseAsset, _ := assetModel.FindByAuthorityOnly(l.ctx, asset.Host)
			if baseAsset != nil {
				asset.Ip = baseAsset.Ip // 继承 IP 数组
				if asset.CName == "" {
					asset.CName = baseAsset.CName // 继承 CName
				}
				if asset.Domain == "" {
					asset.Domain = baseAsset.Domain
				}
				if asset.OrgId == "" {
					asset.OrgId = baseAsset.OrgId
				}
			}
		}

		// ========= 将新资产的 IP/Location 回填到基础域名资产 =========
		// 当端口扫描（如 Naabu）发现了域名对应的 IP 和 Location，
		// 而基础域名资产（由子域名扫描创建）的 Location 为空时，
		// 将 Location 回填到基础域名资产，使前端能立即显示地理位置
		if asset.Port > 0 && (len(asset.Ip.IpV4) > 0 || len(asset.Ip.IpV6) > 0) && !utils.IsIPAddress(asset.Host) {
			baseAsset, _ := assetModel.FindByAuthorityOnly(l.ctx, asset.Host)
			if baseAsset != nil {
				needsUpdate := false
				// 检查是否有 Location 需要回填（基础域名资产的 IP 缺少 Location）
				for i, ipv4 := range baseAsset.Ip.IpV4 {
					if ipv4.Location == "" {
						// 在新资产的 IP 列表中查找匹配的 IP
						for _, newIpv4 := range asset.Ip.IpV4 {
							if newIpv4.IPName == ipv4.IPName && newIpv4.Location != "" {
								baseAsset.Ip.IpV4[i].Location = newIpv4.Location
								needsUpdate = true
								break
							}
						}
					}
				}
				for i, ipv6 := range baseAsset.Ip.IpV6 {
					if ipv6.Location == "" {
						for _, newIpv6 := range asset.Ip.IpV6 {
							if newIpv6.IPName == ipv6.IPName && newIpv6.Location != "" {
								baseAsset.Ip.IpV6[i].Location = newIpv6.Location
								needsUpdate = true
								break
							}
						}
					}
				}
				if needsUpdate {
					if err := assetModel.UpdateWithRaw(l.ctx, baseAsset.Id.Hex(), bson.M{
						"$set": bson.M{"ip": baseAsset.Ip},
					}); err == nil {
						l.Logger.Infof("已回填Location到基础域名资产: %s", asset.Host)
					}
				}
			}
		}
		// ===============================================

		// ========= 当保存带端口的域名资产时，删除同名的无端口资产 =========
		// 这样可以避免同一个域名出现"www.example.com"和"www.example.com:80"两条记录
		// 删除前先合并基础资产的特有字段（CName/Domain/OrgId/IP），防止数据丢失
		if asset.Port > 0 && !utils.IsIPAddress(asset.Host) {
			// 查找同名的无端口资产
			noPortAsset, err := assetModel.FindByAuthorityOnly(l.ctx, asset.Host)
			if err == nil && noPortAsset != nil {
				// 合并基础资产的特有字段到新资产（仅当新资产没有这些字段时）
				if asset.CName == "" && noPortAsset.CName != "" {
					asset.CName = noPortAsset.CName
				}
				if asset.Domain == "" && noPortAsset.Domain != "" {
					asset.Domain = noPortAsset.Domain
				}
				if asset.OrgId == "" && noPortAsset.OrgId != "" {
					asset.OrgId = noPortAsset.OrgId
				}
				// 如果新资产没有 IP 信息，从基础资产继承
				if len(asset.Ip.IpV4) == 0 && len(asset.Ip.IpV6) == 0 && (len(noPortAsset.Ip.IpV4) > 0 || len(noPortAsset.Ip.IpV6) > 0) {
					asset.Ip = noPortAsset.Ip
				}
				// 删除无端口的同名资产
				if deleteErr := assetModel.Delete(l.ctx, noPortAsset.Id.Hex()); deleteErr == nil {
					l.Logger.Infof("已删除同名无端口资产: %s (被 %s:%d 替代)", asset.Host, asset.Host, asset.Port)
				}
			}
		}
		// ===============================================

		// 检查是否已存在
		var existing *model.Asset
		var err error

		if asset.Port > 0 {
			// 有端口的资产，按host:port查找
			existing, err = assetModel.FindByHostPort(l.ctx, asset.Host, asset.Port)
		} else {
			// 无端口的资产（如域名），按authority查找（不限制taskId）
			existing, err = assetModel.FindByAuthorityOnly(l.ctx, asset.Authority)
		}

		if err != nil || existing == nil {
			// 新资产
			asset.Id = primitive.NewObjectID()
			asset.CreateTime = now
			asset.UpdateTime = now
			asset.IsNewAsset = true
			asset.IsUpdated = false
			asset.FirstSeenTime = now
			asset.LastTaskId = ""                 // 新资产没有上一个任务
			asset.FirstSeenTaskId = in.MainTaskId // 记录首次发现的任务ID
			asset.LastStatusChangeTime = now      // 记录状态变化时间

		if err := assetModel.Insert(l.ctx, asset); err != nil {
			l.Logger.Errorf("Insert asset failed: %v", err)
			continue
		}
		newAsset++
		// T1.1: 记录本次新增资产（唯一真源为落库时刻）
		diffs = append(diffs, model.ScanDiff{
			TaskId:      in.MainTaskId,
			WorkspaceId: workspaceId,
			DiffType:    model.ScanDiffTypeAsset,
			ChangeType:  model.ScanDiffChangeAdded,
			TargetKey:   asset.Authority,
			Summary:     asset.Host,
		})
		} else {
			// 更新已存在的资产
			// 判断是否是不同任务的更新
			// 只要任务ID不同（或者之前没有任务ID），就认为是新一轮扫描
			isDifferentTask := existing.TaskId != in.MainTaskId

			// 使用共享 helper 构造更新文档，统一空值守护 / 状态字段 / update_time 门控
			// 同时返回字段级 diff（changes），供历史记录与 T1.1 变化快照复用，避免重复计算
			opts := model.AssetWriteOptions{
				TaskId:          in.MainTaskId,
				IsDifferentTask: isDifferentTask,
			}
			update, changes := model.BuildAssetUpdateDoc(asset, existing, opts)

			// 只有当任务ID不同时才保存历史记录（表示是新一轮扫描，需要记录上一次的状态）
			// 这样可以确保在任务的第一次保存时就记录历史，避免后续更新覆盖旧状态
			if isDifferentTask {
				historyModel := l.svcCtx.GetAssetHistoryModel(workspaceId)

				// 检查是否已存在同一任务的历史记录（避免重复）
				exists, _ := historyModel.ExistsByAssetIdAndTaskId(l.ctx, existing.Id.Hex(), existing.TaskId)
				if !exists {
					// 只有当有实际变更时才保存历史记录
					if len(changes) > 0 {
						history := model.SnapshotFromAsset(existing, existing.TaskId, existing.UpdateTime, changes)
						if err := historyModel.Insert(l.ctx, history); err != nil {
							l.Logger.Errorf("Insert asset history failed: %v", err)
							// 继续更新资产，不中断
						} else {
							l.Logger.Infof("保存资产变更历史: assetId=%s, oldTaskId=%s, newTaskId=%s, changes=%d",
								existing.Id.Hex(), existing.TaskId, in.MainTaskId, len(changes))
						}
					}
				}
			}

			if err := assetModel.UpdateWithRaw(l.ctx, existing.Id.Hex(), update); err != nil {
				l.Logger.Errorf("Update asset failed: %v", err)
				continue
			}
			if isDifferentTask {
				updateAsset++
				// T1.1: 仅当存在字段级变化时记录 updated 快照，避免无变化也写入噪声
				if len(changes) > 0 {
					diffs = append(diffs, model.ScanDiff{
						TaskId:      in.MainTaskId,
						WorkspaceId: workspaceId,
						DiffType:    model.ScanDiffTypeAsset,
						ChangeType:  model.ScanDiffChangeUpdated,
						TargetKey:   existing.Authority,
						Summary:     existing.Host,
						Changes:     changes,
					})
				}
			}
		}
		totalAsset++

		// 登记/刷新顶层资产 meta，使扫描产出的资产出现在资产页顶层资产列表。
		// 同一批内同 (host,domain) 去重；labels 传 nil 保留既有标签。
		targetKey := asset.Host + "\x00" + asset.Domain
		if _, ok := seenTargets[targetKey]; ok {
			continue
		}
		seenTargets[targetKey] = struct{}{}
		if err := targetMetaModel.EnsureForAsset(l.ctx, workspaceId, asset.Host, asset.Domain, nil); err != nil {
			l.Logger.Errorf("[SaveTaskResult] upsert target meta host=%s fail: %v", asset.Host, err)
			continue
		}
		// 推进 last_scan_time：EnsureForAsset 仅在新建时写入，此处确保每次扫描也刷新时间戳
		tType, tValue := model.ResolveAssetTarget(asset.Host, asset.Domain)
		if tType != "" && tValue != "" {
			targetId := model.EncodeTargetID(tType, tValue)
			if err := targetMetaModel.UpdateLastScanTime(l.ctx, targetId, now); err != nil {
				l.Logger.Errorf("[SaveTaskResult] update last_scan_time id=%s fail: %v", targetId, err)
			}
		}
	}

	l.Logger.Infof("SaveTaskResult: total=%d, new=%d, update=%d", totalAsset, newAsset, updateAsset)

	// T1.1: 批量写入本次扫描的变化快照（新增/更新）。失败仅记录日志，不阻断主流程。
	if len(diffs) > 0 {
		if err := diffModel.BatchInsert(l.ctx, diffs); err != nil {
			l.Logger.Errorf("[ScanDiff] batch insert failed (task=%s): %v", in.MainTaskId, err)
		} else {
			l.Logger.Infof("[ScanDiff] wrote %d diff records for task=%s", len(diffs), in.MainTaskId)
		}
	}

	return &pb.SaveTaskResultResp{
		Success:     true,
		Message:     "Assets saved successfully",
		TotalAsset:  totalAsset,
		NewAsset:    newAsset,
		UpdateAsset: updateAsset,
	}, nil
}
