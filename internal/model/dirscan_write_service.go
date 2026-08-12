package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// ScannerDirScanResult 目录扫描结果数据传输对象（避免循环依赖）
type ScannerDirScanResult struct {
	Authority     string
	Host          string
	Port          int
	URL           string
	Path          string
	StatusCode    int
	ContentLength int64
	ContentType   string
	Title         string
	RedirectURL   string
	ContentWords  int64
	ContentLines  int64
	Duration      int64
	Request       string
	Response      string
}

// DirScanWriteService 目录扫描结果写入服务，封装完整的保存业务逻辑
type DirScanWriteService struct {
	db          *mongo.Database
	workspaceId string
	dirModel    *DirScanResultModel
}

// NewDirScanWriteService 创建目录扫描结果写入服务
func NewDirScanWriteService(db *mongo.Database, workspaceId string) *DirScanWriteService {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return &DirScanWriteService{
		db:          db,
		workspaceId: workspaceId,
		dirModel:    NewDirScanResultModel(db, workspaceId),
	}
}

// SaveResults 保存目录扫描结果列表（完整业务逻辑从 API handler 层迁移）
func (s *DirScanWriteService) SaveResults(ctx context.Context, mainTaskID string, results []*ScannerDirScanResult) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if len(results) == 0 {
		return nil
	}

	docs := make([]*DirScanResult, 0, len(results))
	now := time.Now()

	for _, r := range results {
		docs = append(docs, &DirScanResult{
			WorkspaceId:   s.workspaceId,
			MainTaskId:    mainTaskID,
			Authority:     r.Authority,
			Host:          r.Host,
			Port:          r.Port,
			URL:           r.URL,
			Path:          r.Path,
			StatusCode:    r.StatusCode,
			ContentLength: r.ContentLength,
			ContentType:   r.ContentType,
			Title:         r.Title,
			RedirectURL:   r.RedirectURL,
			ContentWords:  r.ContentWords,
			ContentLines:  r.ContentLines,
			Duration:      r.Duration,
			Request:       r.Request,
			Response:      r.Response,
			CreateTime:    now,
			UpdateTime:    now,
			ScanTime:      now,
			Version:       1,
		})
	}

	if err := s.dirModel.InsertMany(ctx, docs); err != nil {
		logx.Errorf("[DirScanWriteService] InsertMany failed: %v", err)
		return err
	}

	logx.Infof("[DirScanWriteService] SaveResults: saved %d directory scan results for task=%s", len(results), mainTaskID)
	return nil
}
