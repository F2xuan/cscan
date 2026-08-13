package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// ScannerJSFinderResult JS扫描结果数据传输对象（避免循环依赖）
type ScannerJSFinderResult struct {
	Authority        string
	Host             string
	Port             int
	URL              string
	Severity         string
	VulName          string
	Result           string
	Tags             []string
	MatcherName      string
	ExtractedResults []string
	CurlCommand      string
	Request          string
	Response         string
}

// JSFinderWriteService JS扫描结果写入服务，封装完整的保存业务逻辑
type JSFinderWriteService struct {
	db      *mongo.Database
	jsModel *JSFinderResultModel
}

// NewJSFinderWriteService 创建JS扫描结果写入服务
func NewJSFinderWriteService(db *mongo.Database) *JSFinderWriteService {
	return &JSFinderWriteService{
		db:      db,
		jsModel: NewJSFinderResultModel(db),
	}
}

// SaveResults 保存JS扫描结果列表（完整业务逻辑从 API handler 层迁移）
func (s *JSFinderWriteService) SaveResults(ctx context.Context, mainTaskID string, results []*ScannerJSFinderResult) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if len(results) == 0 {
		return nil
	}

	modelResults := make([]*JSFinderResult, 0, len(results))
	now := time.Now()

	for _, r := range results {
		modelResults = append(modelResults, &JSFinderResult{
			MainTaskId:       mainTaskID,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
			CreateTime:       now,
			UpdateTime:       now,
		})
	}

	if err := s.jsModel.EnsureIndexes(ctx); err != nil {
		logx.Errorf("[JSFinderWriteService] EnsureIndexes failed: %v", err)
	}

	if err := s.jsModel.UpsertMany(ctx, modelResults); err != nil {
		logx.Errorf("[JSFinderWriteService] UpsertMany failed: %v", err)
		return err
	}

	logx.Infof("[JSFinderWriteService] SaveResults: saved %d JS findings for task=%s", len(results), mainTaskID)
	return nil
}
