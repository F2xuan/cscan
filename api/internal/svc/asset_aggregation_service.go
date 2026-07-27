package svc

import (
	"context"
	"fmt"
	"sync"

	"cscan/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AssetAggregationService 资产聚合服务 - 解决 N+1 查询问题
type AssetAggregationService struct {
	db *mongo.Database
}

// NewAssetAggregationService 创建资产聚合服务
func NewAssetAggregationService(db *mongo.Database) *AssetAggregationService {
	return &AssetAggregationService{db: db}
}

// AssetSummary 资产摘要信息
type AssetSummary struct {
	AssetId       primitive.ObjectID `bson:"_id" json:"assetId"`
	Host          string             `bson:"host" json:"host"`
	Port          int                `bson:"port" json:"port"`
	Authority     string             `bson:"authority" json:"authority"`
	DirScanCount  int                `bson:"dir_scan_count" json:"dirScanCount"`
	VulnCount     int                `bson:"vuln_count" json:"vulnCount"`
	HighRiskCount int                `bson:"high_risk_count" json:"highRiskCount"`
	RiskScore     float64            `bson:"risk_score" json:"riskScore"`
}

// GetAssetSummaries 批量获取资产摘要（使用聚合管道，一次查询获取所有关联数据）
func (s *AssetAggregationService) GetAssetSummaries(
	ctx context.Context,
	workspaceId string,
	assetIds []primitive.ObjectID,
) ([]AssetSummary, error) {
	if len(assetIds) == 0 {
		return []AssetSummary{}, nil
	}

	assetColl := s.db.Collection(fmt.Sprintf("%s_asset", workspaceId))

	pipeline := mongo.Pipeline{
		// 1. 匹配指定资产
		{{Key: "$match", Value: bson.M{"_id": bson.M{"$in": assetIds}}}},

		// 2. 关联目录扫描结果
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: fmt.Sprintf("%s_dirscan_result", workspaceId)},
			{Key: "let", Value: bson.D{
				{Key: "host", Value: "$host"},
				{Key: "port", Value: "$port"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$host", "$$host"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$port", "$$port"}}},
						}},
					}},
				}}},
				{{Key: "$count", Value: "count"}},
			}},
			{Key: "as", Value: "dir_results"},
		}}},

		// 3. 关联漏洞结果
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: fmt.Sprintf("%s_vul", workspaceId)},
			{Key: "let", Value: bson.D{
				{Key: "host", Value: "$host"},
				{Key: "port", Value: "$port"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$host", "$$host"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$port", "$$port"}}},
						}},
					}},
				}}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
					{Key: "high_risk", Value: bson.D{
						{Key: "$sum", Value: bson.D{
							{Key: "$cond", Value: bson.A{
								bson.D{{Key: "$in", Value: bson.A{"$severity", bson.A{"critical", "high"}}}},
								1, 0,
							}},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "vul_stats"},
		}}},

		// 4. 整理输出字段
		{{Key: "$project", Value: bson.D{
			{Key: "host", Value: 1},
			{Key: "port", Value: 1},
			{Key: "authority", Value: 1},
			{Key: "risk_score", Value: 1},
			{Key: "dir_scan_count", Value: bson.D{
				{Key: "$ifNull", Value: bson.A{
					bson.D{{Key: "$arrayElemAt", Value: bson.A{"$dir_results.count", 0}}},
					0,
				}},
			}},
			{Key: "vuln_count", Value: bson.D{
				{Key: "$ifNull", Value: bson.A{
					bson.D{{Key: "$arrayElemAt", Value: bson.A{"$vul_stats.total", 0}}},
					0,
				}},
			}},
			{Key: "high_risk_count", Value: bson.D{
				{Key: "$ifNull", Value: bson.A{
					bson.D{{Key: "$arrayElemAt", Value: bson.A{"$vul_stats.high_risk", 0}}},
					0,
				}},
			}},
		}}},
	}

	cursor, err := assetColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate assets: %w", err)
	}
	defer cursor.Close(ctx)

	var results []AssetSummary
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode results: %w", err)
	}

	return results, nil
}

// GetAssetSummariesConcurrent 并发批量查询（当聚合管道复杂度过高时使用）
func (s *AssetAggregationService) GetAssetSummariesConcurrent(
	ctx context.Context,
	workspaceId string,
	assetIds []string,
	concurrency int,
) ([]AssetSummary, error) {
	if len(assetIds) == 0 {
		return []AssetSummary{}, nil
	}

	if concurrency <= 0 {
		concurrency = 10
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]AssetSummary, 0, len(assetIds))
		errChan = make(chan error, 1)
		sem     = make(chan struct{}, concurrency) // 限制并发数
	)

	for _, assetId := range assetIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			summary, err := s.getSingleAssetSummary(ctx, workspaceId, id)
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}

			mu.Lock()
			results = append(results, *summary)
			mu.Unlock()
		}(assetId)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	return results, nil
}

func (s *AssetAggregationService) getSingleAssetSummary(ctx context.Context, workspaceId, assetId string) (*AssetSummary, error) {
	oid, err := primitive.ObjectIDFromHex(assetId)
	if err != nil {
		return nil, err
	}

	assetModel := model.NewAssetModel(s.db, workspaceId)
	asset, err := assetModel.FindById(ctx, assetId)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("asset not found: %s", assetId)
	}

	// 并发查询关联数据
	var (
		wg          sync.WaitGroup
		dirCount    int64
		vulnCount   int64
		highRiskCnt int64
		dirErr      error
		vulnErr     error
		highRiskErr error
	)

	wg.Add(3)

	// 目录扫描计数
	go func() {
		defer wg.Done()
		dirScanModel := model.NewDirScanResultModel(s.db)
		dirCount, dirErr = dirScanModel.CountByFilter(ctx, bson.M{
			"host": asset.Host,
			"port": asset.Port,
		})
	}()

	// 漏洞计数
	go func() {
		defer wg.Done()
		vulModel := model.NewVulModel(s.db, workspaceId)
		vulnCount, vulnErr = vulModel.Count(ctx, bson.M{
			"host": asset.Host,
			"port": asset.Port,
		})
	}()

	// 高危漏洞计数
	go func() {
		defer wg.Done()
		vulModel := model.NewVulModel(s.db, workspaceId)
		highRiskCnt, highRiskErr = vulModel.Count(ctx, bson.M{
			"host":     asset.Host,
			"port":     asset.Port,
			"severity": bson.M{"$in": []string{"critical", "high"}},
		})
	}()

	wg.Wait()

	// 检查错误
	if dirErr != nil {
		return nil, dirErr
	}
	if vulnErr != nil {
		return nil, vulnErr
	}
	if highRiskErr != nil {
		return nil, highRiskErr
	}

	return &AssetSummary{
		AssetId:       oid,
		Host:          asset.Host,
		Port:          asset.Port,
		Authority:     asset.Authority,
		DirScanCount:  int(dirCount),
		VulnCount:     int(vulnCount),
		HighRiskCount: int(highRiskCnt),
		RiskScore:     asset.RiskScore,
	}, nil
}

// GetAssetStatsByWorkspace 获取工作空间资产统计
func (s *AssetAggregationService) GetAssetStatsByWorkspace(ctx context.Context, workspaceId string) (map[string]interface{}, error) {
	assetColl := s.db.Collection(fmt.Sprintf("%s_asset", workspaceId))

	pipeline := mongo.Pipeline{
		{{Key: "$facet", Value: bson.D{
			// 总数
			{Key: "total", Value: mongo.Pipeline{
				{{Key: "$count", Value: "count"}},
			}},
			// 按风险等级分组
			{Key: "by_risk_level", Value: mongo.Pipeline{
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$risk_level"},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
			}},
			// 按服务分组（Top 10）
			{Key: "by_service", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{{Key: "service", Value: bson.D{{Key: "$ne", Value: ""}}}}}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$service"},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
				{{Key: "$limit", Value: 10}},
			}},
			// 新资产数量
			{Key: "new_assets", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{{Key: "new", Value: true}}}},
				{{Key: "$count", Value: "count"}},
			}},
		}}},
	}

	cursor, err := assetColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return map[string]interface{}{}, nil
	}

	return results[0], nil
}
