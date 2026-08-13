package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	mongoLogChannelSize = 2000  // 日志通道缓冲
	mongoLogBatchSize   = 100   // 批量写入大小
	mongoLogFlushInterval = 2 * time.Second
)

// mongoLogDoc Worker 日志 MongoDB 文档（与 model.WorkerLog 字段一致，但 bson tag 对齐）
type mongoLogDoc struct {
	Worker     string    `bson:"worker"`
	TaskId     string    `bson:"task_id,omitempty"`
	Level      string    `bson:"level"`
	Msg        string    `bson:"msg"`
	CreateTime time.Time `bson:"create_time"`
	Seq        int64     `bson:"seq"`
}

// MongoLogger 将 Worker 日志批量直写 MongoDB
type MongoLogger struct {
	coll       *mongo.Collection
	workerName string
	logCh      chan mongoLogDoc
	closeChan  chan struct{}
	closeOnce  sync.Once
	wg         sync.WaitGroup
	seq        atomic.Int64
}

// NewMongoLogger 创建 MongoDB 日志写入器
func NewMongoLogger(db *mongo.Database, workerName string) *MongoLogger {
	m := &MongoLogger{
		coll:       db.Collection("worker_log"),
		workerName: workerName,
		logCh:      make(chan mongoLogDoc, mongoLogChannelSize),
		closeChan:  make(chan struct{}),
	}
	m.wg.Add(1)
	go m.flushLoop()
	return m
}

// Write 写入一条日志（非阻塞，channel 满时丢弃并告警）
func (m *MongoLogger) Write(level, taskId, msg string) {
	if m == nil {
		return
	}
	doc := mongoLogDoc{
		Worker:     m.workerName,
		TaskId:     taskId,
		Level:      level,
		Msg:        msg,
		CreateTime: time.Now().Local(),
		Seq:        m.seq.Add(1),
	}
	select {
	case m.logCh <- doc:
	default:
		// channel 满，丢弃日志（避免阻塞扫描任务）
		logx.Errorf("[MongoLogger] log channel full, dropping log: level=%s worker=%s", level, m.workerName)
	}
}

// SetWorkerName 更新 worker 名称（rename 后调用）
func (m *MongoLogger) SetWorkerName(name string) {
	if m != nil {
		m.workerName = name
	}
}

// Close 关闭写入器，flush 剩余日志
func (m *MongoLogger) Close() {
	m.closeOnce.Do(func() {
		close(m.closeChan)
	})
	m.wg.Wait()
}

// flushLoop 批量写入 goroutine
func (m *MongoLogger) flushLoop() {
	defer m.wg.Done()

	batch := make([]mongoLogDoc, 0, mongoLogBatchSize)
	ticker := time.NewTicker(mongoLogFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		docs := make([]interface{}, len(batch))
		for i, d := range batch {
			docs[i] = d
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := m.coll.InsertMany(ctx, docs)
		cancel()
		if err != nil {
			logx.Errorf("[MongoLogger] InsertMany failed: %v, retaining %d logs for retry", err, len(batch))
			// 修复 #17：保留 batch 待下次重试；积压超过 3 倍批次大小时丢弃防止 OOM
			if len(batch) >= mongoLogBatchSize*3 {
				logx.Errorf("[MongoLogger] batch backlog too large (%d), dropping to prevent OOM", len(batch))
				batch = batch[:0]
			}
			return
		}
		batch = batch[:0]
	}

	for {
		select {
		case doc := <-m.logCh:
			batch = append(batch, doc)
			if len(batch) >= mongoLogBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-m.closeChan:
			// 排空 channel
			for {
				select {
				case doc := <-m.logCh:
					batch = append(batch, doc)
				default:
					flush()
					return
				}
			}
		}
	}
}
