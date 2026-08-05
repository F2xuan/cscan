package brute

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zeromicro/go-zero/core/logx"
)

// RedisPlugin Redis爆破插件
type RedisPlugin struct{}

func (p *RedisPlugin) Name() string     { return "redis" }
func (p *RedisPlugin) DefaultPort() int { return 6379 }

func (p *RedisPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *RedisPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	logx.Infof("[Redis Brute] Starting brute force for %s:%d, usernames=%d, passwords=%d, timeout=%d", host, port, len(usernames), len(passwords), timeout)

	// Redis: 先尝试无密码检测
	ok, debugMsg := testRedisNoAuth(ctx, host, port, timeout)
	logx.Infof("[Redis Brute] Testing no-auth for %s:%d - Result: %v, Debug: %s", host, port, ok, debugMsg)
	if ok {
		return &BruteResult{
			Host:     host,
			Port:     port,
			Service:  "redis",
			Username: "",
			Password: "",
			Success:  true,
			Message:  "No password required (anonymous access allowed)",
		}
	}

	// 无密码失败，尝试有密码
	seen := make(map[string]bool)
	for _, password := range passwords {
		if password == "" || seen[password] {
			continue
		}
		seen[password] = true

		select {
		case <-ctx.Done():
			return &BruteResult{Host: host, Port: port, Service: "redis", Success: false, Message: "canceled"}
		default:
		}

		ok := testRedisWithPassword(ctx, host, port, password, timeout)
		logx.Infof("[Redis Brute] Testing password(len=%d) for %s:%d - Result: %v", len(password), host, port, ok)
		if ok {
			return &BruteResult{
				Host:     host,
				Port:     port,
				Service:  "redis",
				Username: "",
				Password: password,
				Success:  true,
				Message:  "Authentication successful",
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "redis", Success: false, Message: "Authentication failed"}
}

// testRedisNoAuth 测试Redis无密码访问
// 返回: 是否成功, 调试信息
func testRedisNoAuth(ctx context.Context, host string, port int, timeout int) (bool, string) {
	addr := fmt.Sprintf("%s:%d", host, port)

	// 创建带超时的 context，避免永久阻塞
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 使用 go-redis 库检测无密码访问
	opt := redis.Options{
		Addr:         addr,
		Password:     "", // 无密码
		DB:           0,
		DialTimeout:  time.Duration(timeout) * time.Second,
		ReadTimeout:  time.Duration(timeout) * time.Second,
		WriteTimeout: time.Duration(timeout) * time.Second,
	}

	client := redis.NewClient(&opt)
	defer client.Close()

	// 使用 PING 命令检测
	_, err := client.Ping(ctx).Result()
	if err != nil {
		errStr := err.Error()
		logx.Infof("[Redis Brute] No-auth failed: %s", errStr)

		// 如果错误包含 NOAUTH，说明需要密码
		if strings.Contains(errStr, "NOAUTH") || strings.Contains(errStr, "invalid password") {
			return false, "Requires password"
		}
		// 如果是连接超时
		if strings.Contains(errStr, "timeout") {
			return false, "Connection timeout"
		}
		return false, errStr
	}

	return true, "Connection successful"
}

// testRedisWithPassword 测试Redis带密码访问
func testRedisWithPassword(ctx context.Context, host string, port int, password string, timeout int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)

	// 创建带超时的 context，避免永久阻塞
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	opt := redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  time.Duration(timeout) * time.Second,
		ReadTimeout:  time.Duration(timeout) * time.Second,
		WriteTimeout: time.Duration(timeout) * time.Second,
	}

	client := redis.NewClient(&opt)
	defer client.Close()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return false
	}
	return true
}
