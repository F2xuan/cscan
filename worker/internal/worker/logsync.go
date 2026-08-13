package worker

import (
	"go.mongodb.org/mongo-driver/mongo"
	"github.com/zeromicro/go-zero/core/logx"
)

// InitMongoLogger 初始化 MongoDB 直写日志器
// Worker 日志直接写入 MongoDB worker_log 集合（TTL 7 天自动过期），
// 不再经过本地文件 + WebSocket 游标同步
func InitMongoLogger(db *mongo.Database, workerName string) {
	InitGlobalMongoLogger(db, workerName)
	logx.Infof("[MongoLogger] Initialized: worker=%s, collection=worker_log", workerName)
}
