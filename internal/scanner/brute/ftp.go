package brute

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/zeromicro/go-zero/core/logx"
)

// FTPPlugin FTP爆破插件
type FTPPlugin struct{}

func (p *FTPPlugin) Name() string     { return "ftp" }
func (p *FTPPlugin) DefaultPort() int { return 21 }

func (p *FTPPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *FTPPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "ftp", Success: false, Message: "canceled"}
			default:
			}

			ok := testFTP(host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "ftp",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "ftp", Success: false, Message: "Authentication failed"}
}

// testFTP 使用 ftp 库测试 FTP 登录
func testFTP(host string, port int, username, password string, timeout int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(time.Duration(timeout)*time.Second))
	if err != nil {
		logx.Infof("[FTP Brute] Connection failed to %s: %v", addr, err)
		return false
	}
	defer conn.Quit()

	err = conn.Login(username, password)
	if err != nil {
		logx.Infof("[FTP Brute] Login failed for %s with password(len=%d)@%s: %v", username, len(password), addr, err)
		return false
	}

	logx.Infof("[FTP Brute] Login successful for %s with password(len=%d)@%s", username, len(password), addr)
	return true
}
