package brute

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLPlugin MySQL爆破插件
type MySQLPlugin struct{}

func (p *MySQLPlugin) Name() string     { return "mysql" }
func (p *MySQLPlugin) DefaultPort() int { return 3306 }

func (p *MySQLPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *MySQLPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "mysql", Success: false, Message: "canceled"}
			default:
			}

			ok := testMySQL(ctx, host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "mysql",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "mysql", Success: false, Message: "Authentication failed"}
}

// testMySQL 测试MySQL连接
func testMySQL(ctx context.Context, host string, port int, username, password string, timeout int) bool {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=%ds&interpolateParams=false",
		username, password, host, port, timeout)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return false
	}
	defer db.Close()
	db.SetConnMaxLifetime(time.Duration(timeout) * time.Second)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return false
	}
	return true
}
