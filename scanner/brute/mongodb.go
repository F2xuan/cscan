package brute

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/zeromicro/go-zero/core/logx"
)

// MongoDBPlugin MongoDB爆破插件
type MongoDBPlugin struct{}

func (p *MongoDBPlugin) Name() string     { return "mongodb" }
func (p *MongoDBPlugin) DefaultPort() int { return 27017 }

func (p *MongoDBPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *MongoDBPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	logx.Infof("[MongoDB Brute] Starting brute force for %s:%d, usernames=%d, passwords=%d, timeout=%d", host, port, len(usernames), len(passwords), timeout)

	// MongoDB: 先尝试无认证检测
	ok, debugMsg := testMongoDBNoAuth(ctx, host, port, timeout)
	logx.Infof("[MongoDB Brute] Testing no-auth for %s:%d - Result: %v, Debug: %s", host, port, ok, debugMsg)
	if ok {
		return &BruteResult{
			Host:     host,
			Port:     port,
			Service:  "mongodb",
			Username: "",
			Password: "",
			Success:  true,
			Message:  "No authentication required (anonymous access allowed)",
		}
	}

	// 无认证失败，尝试带用户名/密码的认证
	seen := make(map[string]bool)
	for _, username := range usernames {
		for _, password := range passwords {
			if password == "" {
				continue
			}
			key := username + ":" + password
			if seen[key] {
				continue
			}
			seen[key] = true

			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "mongodb", Success: false, Message: "canceled"}
			default:
			}

			ok := testMongoDBWithAuth(ctx, host, port, username, password, timeout)
			logx.Infof("[MongoDB Brute] Testing '%s' with password(len=%d) for %s:%d - Result: %v", username, len(password), host, port, ok)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "mongodb",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "mongodb", Success: false, Message: "Authentication failed"}
}

// testMongoDBNoAuth 测试MongoDB无认证访问
// 返回: 是否成功, 调试信息
func testMongoDBNoAuth(ctx context.Context, host string, port int, timeout int) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 使用 MongoDB 官方驱动检测无认证访问
	uri := fmt.Sprintf("mongodb://%s:%d", host, port)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetConnectTimeout(time.Duration(timeout)*time.Second))
	if err != nil {
		logx.Infof("[MongoDB Brute] Connection failed: %v", err)
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			// ignore
		}
	}()

	// Ping 数据库检测连接
	err = client.Ping(ctx, nil)
	if err != nil {
		errStr := err.Error()
		logx.Infof("[MongoDB Brute] Ping failed: %v", err)

		// 检查是否是认证错误
		if strings.Contains(errStr, "auth") || strings.Contains(errStr, "Authentication") || strings.Contains(errStr, "Unauthorized") {
			return false, "Authentication required"
		}
		return false, errStr
	}

	return true, "Connection successful"
}

// testMongoDBWithAuth 测试MongoDB带认证访问
func testMongoDBWithAuth(ctx context.Context, host string, port int, username, password string, timeout int) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 使用 MongoDB 官方驱动检测认证访问
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/?authSource=admin", username, password, host, port)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetConnectTimeout(time.Duration(timeout)*time.Second))
	if err != nil {
		return false
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			// ignore
		}
	}()

	// Ping 数据库检测连接
	err = client.Ping(ctx, nil)
	if err != nil {
		return false
	}

	return true
}
