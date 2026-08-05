package brute

import (
	"context"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHPlugin SSH爆破插件
type SSHPlugin struct{}

func (p *SSHPlugin) Name() string     { return "ssh" }
func (p *SSHPlugin) DefaultPort() int { return 22 }

func (p *SSHPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *SSHPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "ssh", Success: false, Message: "canceled"}
			default:
			}

			ok := testSSH(host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "ssh",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "ssh", Success: false, Message: "Authentication failed"}
}

// testSSH 测试SSH连接
// 注意：OpenSSH 10.2 等新版SSH服务器可能需要更长的握手时间
// 因此 TCP 连接超时和 SSH 握手超时需要分别设置，且 TCP 超时应该更短
func testSSH(host string, port int, username, password string, timeout int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// TCP 连接超时 - 使用传入的 timeout 参数，但至少 3 秒
	tcpTimeout := timeout
	if tcpTimeout < 3 {
		tcpTimeout = 3
	}

	// SSH 握手超时 - 使用传入的 timeout 参数，但至少 5 秒
	handshakeTimeout := timeout
	if handshakeTimeout < 5 {
		handshakeTimeout = 5
	}

	// 建立 TCP 连接
	netConn, err := net.DialTimeout("tcp", addr, time.Duration(tcpTimeout)*time.Second)
	if err != nil {
		return false
	}

	// 设置 SSH 客户端配置
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(handshakeTimeout) * time.Second,
	}

	// 执行 SSH 握手
	client, _, _, err := ssh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		// 认证失败或握手失败，关闭底层连接
		netConn.Close()
		// 检查是否是超时错误
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// 连接超时，不算作有效凭证
			return false
		}
		return false
	}

	// 认证成功，关闭连接并返回 true
	client.Close()
	return true
}
