package worker

import (
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// newRedisClient 从环境变量初始化 Redis 客户端
func newRedisClient() *redis.Client {
	addr := os.Getenv("CSCAN_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	password := os.Getenv("CSCAN_REDIS_PASSWORD")
	db := 0
	if dbStr := os.Getenv("CSCAN_REDIS_DB"); dbStr != "" {
		if v, err := strconv.Atoi(dbStr); err == nil {
			db = v
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     20,
		MinIdleConns: 2,
		MaxRetries:   3,
	})

	logx.Infof("[RedisClient] initialized, addr=%s, db=%d", addr, db)
	return client
}
