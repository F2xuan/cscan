package brute

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	go_ora "github.com/sijms/go-ora/v2"
)

// 常见 Oracle 服务名列表
var oracleServiceNames = []string{
	"orcl",
	"xe",
	"oracle",
}

// OraclePlugin Oracle爆破插件
type OraclePlugin struct{}

func (p *OraclePlugin) Name() string     { return "oracle" }
func (p *OraclePlugin) DefaultPort() int { return 1521 }

func (p *OraclePlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := sql.Open("oracle", go_ora.BuildUrl(host, port, "", "", "", nil))
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *OraclePlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "oracle", Success: false, Message: "canceled"}
			default:
			}

			ok, sn := testOracle(ctx, host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:      host,
					Port:      port,
					Service:   "oracle",
					Username:  username,
					Password:  password,
					Success:   true,
					Message:   fmt.Sprintf("Authentication successful (service_name=%s)", sn),
					ExtraInfo: sn,
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "oracle", Success: false, Message: "Authentication failed"}
}

// testOracle 测试Oracle连接，自动遍历常见服务名
func testOracle(ctx context.Context, host string, port int, username, password string, timeout int) (bool, string) {
	for _, sn := range oracleServiceNames {
		ok := oracleConnect(ctx, host, port, username, password, sn, timeout)
		if ok {
			return true, sn
		}
	}
	return false, ""
}

// oracleConnect 使用 go-ora 驱动尝试连接 Oracle
func oracleConnect(ctx context.Context, host string, port int, username, password, serviceName string, timeout int) bool {
	urlOptions := map[string]string{
		"CONNECTION TIMEOUT": strconv.Itoa(timeout),
	}
	connectionString := go_ora.BuildUrl(host, port, serviceName, username, password, urlOptions)

	conn, err := sql.Open("oracle", connectionString)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetConnMaxLifetime(time.Duration(timeout) * time.Second)
	conn.SetConnMaxIdleTime(time.Duration(timeout) * time.Second)
	conn.SetMaxIdleConns(0)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	err = conn.PingContext(ctx)
	if err != nil {
		return false
	}

	return true
}
