package brute

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

// PostgreSQLPlugin PostgreSQL爆破插件
type PostgreSQLPlugin struct{}

func (p *PostgreSQLPlugin) Name() string     { return "postgresql" }
func (p *PostgreSQLPlugin) DefaultPort() int { return 5432 }

func (p *PostgreSQLPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *PostgreSQLPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "postgresql", Success: false, Message: "canceled"}
			default:
			}

			ok := testPostgreSQL(ctx, host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "postgresql",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "postgresql", Success: false, Message: "Authentication failed"}
}

// testPostgreSQL 测试PostgreSQL连接
func testPostgreSQL(ctx context.Context, host string, port int, username, password string, timeout int) bool {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable connect_timeout=%d",
		host, port, username, password, timeout)

	db, err := sql.Open("postgres", dsn)
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
