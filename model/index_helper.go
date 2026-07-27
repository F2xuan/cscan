package model

import (
	"context"
	"errors"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

var indexOnce sync.Map

// ensureIndexes 创建索引。
// 行为约定：
//   - 索引冲突（IndexOptionsConflict / already exists / duplicate key）视为可恢复，记录 warning 后返回 nil，允许服务继续启动；
//   - 其他基础设施错误返回 error，调用方可决定是否重试或降级启动。
func ensureIndexes(coll *mongo.Collection, indexes []mongo.IndexModel) error {
	key := coll.Name()
	if _, loaded := indexOnce.LoadOrStore(key, true); !loaded {
		_, err := coll.Indexes().CreateMany(context.Background(), indexes)
		if err != nil {
			if isIndexConflictError(err) {
				logx.Infof("[index_helper] index conflict for %s, skip: %v", coll.Name(), err)
				return nil
			}
			return err
		}
	}
	return nil
}

// isIndexConflictError 判断是否为索引定义冲突类错误（已存在或选项冲突）。
// 使用 mongo.CommandError 错误码判断，避免字符串匹配误判真实数据冲突。
// 注意: 11000 (DuplicateKey) 是数据层重复值错误，不是索引定义冲突 ——
// 新增唯一索引含旧重复数据时应失败而非静默跳过，否则将失去唯一性保证。
// 参考 MongoDB 错误码：IndexAlreadyExists=68, IndexOptionsConflict=85,
// IndexKeySpecsConflict=86, DuplicateKey=11000
func isIndexConflictError(err error) bool {
	if err == nil {
		return false
	}
	// MongoDB 驱动 CommandError
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		switch cmdErr.Code {
		case 68, 85, 86: // IndexAlreadyExists / IndexOptionsConflict / IndexKeySpecsConflict
			return true
		}
	}
	// WriteException 中 11000 (DuplicateKey) 表示数据层冲突,不应跳过
	return false
}
