package brute

import (
	"context"
	"crypto/des"
	"encoding/binary"
	"net"
	"strconv"
	"time"
	"unicode/utf16"

	"golang.org/x/crypto/md4"
)

// SMBPlugin SMB爆破插件
type SMBPlugin struct{}

func (p *SMBPlugin) Name() string     { return "smb" }
func (p *SMBPlugin) DefaultPort() int { return 445 }

func (p *SMBPlugin) Probe(ctx context.Context, host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (p *SMBPlugin) Brute(ctx context.Context, host string, port int, usernames, passwords []string, timeout int) *BruteResult {
	for _, username := range usernames {
		for _, password := range passwords {
			select {
			case <-ctx.Done():
				return &BruteResult{Host: host, Port: port, Service: "smb", Success: false, Message: "canceled"}
			default:
			}

			ok := testSMB(host, port, username, password, timeout)
			if ok {
				return &BruteResult{
					Host:     host,
					Port:     port,
					Service:  "smb",
					Username: username,
					Password: password,
					Success:  true,
					Message:  "Authentication successful",
				}
			}
		}
	}
	return &BruteResult{Host: host, Port: port, Service: "smb", Success: false, Message: "Authentication failed"}
}

// testSMB 测试SMB连接 (SMB2 Session Setup with NTLM)
func testSMB(host string, port int, username, password string, timeout int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	conn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second))

	// Step 1: SMB2 NEGOTIATE request
	negotiate := buildSMB2Packet(1, 0, 0, 0, 0, 0, []byte{})

	structSize := []byte{0x24, 0x00}
	dialectCount := []byte{0x05, 0x00}
	securityMode := []byte{0x03, 0x00}
	capabilities := make([]byte, 8)
	clientGuid := make([]byte, 16)
	for i := range clientGuid {
		clientGuid[i] = byte(i)
	}
	negotiateOffset := []byte{0x80, 0x00, 0x00, 0x00}
	negotiateCount := []byte{0x05, 0x00}
	dialects := []byte{0x02, 0x02, 0x10, 0x02, 0x00, 0x03, 0x02, 0x03, 0x11, 0x03}

	body := make([]byte, 0, 100)
	body = append(body, structSize...)
	body = append(body, dialectCount...)
	body = append(body, securityMode...)
	body = append(body, capabilities...)
	body = append(body, clientGuid...)
	body = append(body, negotiateOffset...)
	body = append(body, negotiateCount...)
	body = append(body, dialects...)

	negotiate = buildSMB2Packet(1, 0, 0, 0, 0, 0, body)
	if _, err := conn.Write(negotiate); err != nil {
		return false
	}

	// Read NEGOTIATE response
	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil || n < 68 {
		return false
	}

	if resp[0] != 0xFE || resp[1] != 'S' || resp[2] != 'M' || resp[3] != 'B' {
		return false
	}
	status := binary.LittleEndian.Uint32(resp[8:12])
	if status != 0 {
		return false
	}

	// Step 2: SMB2 SESSION_SETUP
	domain := ""
	workstation := ""
	ntlmMsg := buildNTLMNegotiate(username, domain, workstation)
	securityBlob := ntlmMsg

	body2 := make([]byte, 0)
	body2 = append(body2, []byte{0x19, 0x00}...)
	body2 = append(body2, []byte{0x00}...)
	body2 = append(body2, []byte{0x00, 0x00, 0x00, 0x00}...)
	body2 = append(body2, []byte{0x00, 0x00, 0x00, 0x00}...)
	body2 = append(body2, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)
	secBlobOffset := make([]byte, 4)
	secBlobLen := uint32(len(securityBlob))
	offset := uint32(64 + len(body2))
	binary.LittleEndian.PutUint32(secBlobOffset, offset)
	body2 = append(body2, secBlobOffset...)
	body2 = appendUint32(body2, secBlobLen)
	body2 = appendUint32(body2, secBlobLen)
	body2 = append(body2, securityBlob...)

	sessionSetup := buildSMB2Packet(1, 0, 5, 0, 0, 0, body2)
	if _, err := conn.Write(sessionSetup); err != nil {
		return false
	}

	resp2 := make([]byte, 4096)
	n2, err := conn.Read(resp2)
	if err != nil || n2 < 64 {
		return false
	}
	if resp2[0] != 0xFE || resp2[1] != 'S' || resp2[2] != 'M' || resp2[3] != 'B' {
		return false
	}
	status2 := binary.LittleEndian.Uint32(resp2[8:12])

	if status2 == 0 {
		return true
	}

	if status2 != 0xC0000016 && status2 != 0xC000006D && status2 != 0 {
		return false
	}

	sessionID := binary.LittleEndian.Uint64(resp2[56:64])

	// Step 3: SESSION_SETUP with NTLM AUTHENTICATE
	secBlobLenResp := binary.LittleEndian.Uint32(resp2[40:44])
	if secBlobLenResp == 0 || int(secBlobLenResp) > n2-64 {
		return false
	}
	secBlobResp := resp2[64 : 64+secBlobLenResp]

	ntlmAuth := buildNTLMAuthenticate(username, password, domain, workstation, secBlobResp)
	securityBlob2 := ntlmAuth

	body3 := make([]byte, 0)
	body3 = append(body3, []byte{0x19, 0x00}...)
	body3 = append(body3, []byte{0x01}...)
	body3 = append(body3, []byte{0x00, 0x00, 0x00, 0x00}...)
	body3 = append(body3, []byte{0x00, 0x00, 0x00, 0x00}...)
	body3 = append(body3, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)
	offset2 := uint32(64 + len(body3))
	body3 = appendUint32(body3, offset2)
	secBlobLen2 := uint32(len(securityBlob2))
	body3 = appendUint32(body3, secBlobLen2)
	body3 = appendUint32(body3, secBlobLen2)
	body3 = append(body3, securityBlob2...)

	sessionSetup2 := buildSMB2PacketWithSession(1, 0, 5, 0, 0, sessionID, body3)
	if _, err := conn.Write(sessionSetup2); err != nil {
		return false
	}

	resp3 := make([]byte, 4096)
	n3, err := conn.Read(resp3)
	if err != nil || n3 < 64 {
		return false
	}
	if resp3[0] != 0xFE || resp3[1] != 'S' || resp3[2] != 'M' || resp3[3] != 'B' {
		return false
	}
	status3 := binary.LittleEndian.Uint32(resp3[8:12])
	return status3 == 0
}

// buildSMB2Packet builds an SMB2 packet header + body
func buildSMB2Packet(creditCharge uint16, creditReq uint16, command uint16, flags uint32, asyncId uint64, sessionId uint64, body []byte) []byte {
	packetLen := 64 + len(body)
	buf := make([]byte, packetLen)

	buf[0], buf[1], buf[2], buf[3] = 0xFE, 'S', 'M', 'B'
	binary.LittleEndian.PutUint16(buf[4:6], 64)
	binary.LittleEndian.PutUint16(buf[6:8], creditCharge)
	binary.LittleEndian.PutUint32(buf[8:12], 0)
	binary.LittleEndian.PutUint16(buf[12:14], command)
	binary.LittleEndian.PutUint16(buf[14:16], creditReq)
	binary.LittleEndian.PutUint32(buf[16:20], flags)
	binary.LittleEndian.PutUint32(buf[20:24], 0)

	msgId := time.Now().UnixNano() % 0x7FFFFFFFFFFFFFFF
	binary.LittleEndian.PutUint64(buf[24:32], uint64(msgId))
	binary.LittleEndian.PutUint64(buf[32:40], asyncId)
	binary.LittleEndian.PutUint64(buf[48:56], sessionId)

	copy(buf[64:], body)
	return buf
}

// buildSMB2PacketWithSession is like buildSMB2Packet but allows setting session ID
func buildSMB2PacketWithSession(creditCharge uint16, creditReq uint16, command uint16, flags uint32, asyncId uint64, sessionId uint64, body []byte) []byte {
	buf := buildSMB2Packet(creditCharge, creditReq, command, flags, asyncId, 0, body)
	binary.LittleEndian.PutUint64(buf[48:56], sessionId)
	return buf
}

// buildNTLMNegotiate creates NTLM NEGOTIATE message
func buildNTLMNegotiate(user, domain, workstation string) []byte {
	sig := []byte("NTLMSSP\x00")
	msgType := []byte{0x01, 0x00, 0x00, 0x00}
	flags := []byte{0xe2, 0x82, 0x08, 0x80}
	return buildNTLMBlob(sig, msgType, flags, domain, user, workstation)
}

// buildNTLMAuthenticate creates NTLM AUTHENTICATE message
func buildNTLMAuthenticate(user, password, domain, workstation string, challengeBlob []byte) []byte {
	if len(challengeBlob) < 60 {
		return nil
	}

	targetInfoLen := binary.LittleEndian.Uint16(challengeBlob[44:46])
	targetInfoOffset := binary.LittleEndian.Uint32(challengeBlob[48:52])
	var serverChallenge []byte
	if int(targetInfoOffset)+int(targetInfoLen) <= len(challengeBlob) && targetInfoLen > 0 {
		ti := challengeBlob[targetInfoOffset : targetInfoOffset+uint32(targetInfoLen)]
		for len(ti) >= 4 {
			avType := binary.LittleEndian.Uint16(ti[0:2])
			avLen := binary.LittleEndian.Uint16(ti[2:4])
			if int(avLen) > len(ti)-4 {
				break
			}
			if avType == 2 && avLen == 8 {
				serverChallenge = ti[4 : 4+avLen]
			}
			ti = ti[4+uint32(avLen):]
		}
	}
	if len(serverChallenge) != 8 {
		if len(challengeBlob) >= 32 {
			serverChallenge = challengeBlob[24:32]
		} else {
			serverChallenge = make([]byte, 8)
		}
	}

	ntlmHash := computeNTLMHash(password)
	lmResp := desECB(ntlmHash[:8], serverChallenge[:8])
	ntResp := desECB(ntlmHash[:7], serverChallenge[:8])

	response := append(lmResp, ntResp...)

	sig := []byte("NTLMSSP\x00")
	msgType := []byte{0x03, 0x00, 0x00, 0x00}
	return buildNTLMBlobWithResponse(sig, msgType, user, domain, workstation, response, nil)
}

func buildNTLMBlob(sig, msgType, flags []byte, domain, user, workstation string) []byte {
	var domainPtr, userPtr, wsPtr uint16 = 0, 0, 0
	var domainLen, userLen, wsLen uint16 = 0, 0, 0
	offset := uint32(56)

	if domain != "" {
		domainLen = uint16(len(domain))
		domainPtr = uint16(offset)
		offset += uint32(domainLen)
	}
	if user != "" {
		userLen = uint16(len(user))
		userPtr = uint16(offset)
		offset += uint32(userLen)
	}
	if workstation != "" {
		wsLen = uint16(len(workstation))
		wsPtr = uint16(offset)
		offset += uint32(wsLen)
	}

	total := offset
	buf := make([]byte, 0, total)
	buf = append(buf, sig...)
	buf = append(buf, msgType...)
	buf = append(buf, flags...)
	buf = appendUint16(buf, domainLen)
	buf = appendUint16(buf, domainLen)
	buf = appendUint32(buf, uint32(domainPtr))
	buf = appendUint16(buf, userLen)
	buf = appendUint16(buf, userLen)
	buf = appendUint32(buf, uint32(userPtr))
	buf = appendUint16(buf, wsLen)
	buf = appendUint16(buf, wsLen)
	buf = appendUint32(buf, uint32(wsPtr))
	buf = append(buf, make([]byte, 8)...)
	buf = append(buf, []byte(domain)...)
	buf = append(buf, []byte(user)...)
	buf = append(buf, []byte(workstation)...)

	return wrapNTLMToken(buf)
}

func buildNTLMBlobWithResponse(sig, msgType []byte, user, domain, workstation string, lmResp, ntResp []byte) []byte {
	userBytes := []byte(user)
	domainBytes := []byte(domain)
	wsBytes := []byte(workstation)
	if lmResp == nil {
		lmResp = make([]byte, 24)
	}
	if ntResp == nil {
		ntResp = make([]byte, 24)
	}

	offset := 64 + uint32(len(domainBytes)+len(userBytes)+len(wsBytes))
	flags := []byte{0xe2, 0x82, 0x08, 0x80}

	buf := make([]byte, 0, 200)
	buf = append(buf, sig...)
	buf = append(buf, msgType...)
	buf = append(buf, make([]byte, 8)...)
	buf = appendUint16(buf, uint16(len(lmResp)))
	buf = appendUint16(buf, uint16(len(lmResp)))
	buf = appendUint32(buf, uint32(64))

	buf = append(buf, make([]byte, 8)...)
	buf = appendUint16(buf, uint16(len(ntResp)))
	buf = appendUint16(buf, uint16(len(ntResp)))
	buf = appendUint32(buf, uint32(64+len(lmResp)))

	buf = appendUint16(buf, uint16(len(domainBytes)))
	buf = appendUint16(buf, uint16(len(domainBytes)))
	buf = appendUint32(buf, offset)
	offset += uint32(len(domainBytes))

	buf = appendUint16(buf, uint16(len(userBytes)))
	buf = appendUint16(buf, uint16(len(userBytes)))
	buf = appendUint32(buf, offset)
	offset += uint32(len(userBytes))

	buf = appendUint16(buf, uint16(len(wsBytes)))
	buf = appendUint16(buf, uint16(len(wsBytes)))
	buf = appendUint32(buf, offset)
	offset += uint32(len(wsBytes))

	buf = appendUint16(buf, 0)
	buf = appendUint16(buf, 0)
	buf = append(buf, flags...)
	buf = append(buf, make([]byte, 8)...)
	buf = append(buf, make([]byte, 16)...)

	buf = append(buf, lmResp...)
	buf = append(buf, ntResp...)
	buf = append(buf, domainBytes...)
	buf = append(buf, userBytes...)
	buf = append(buf, wsBytes...)

	return wrapNTLMToken(buf)
}

func wrapNTLMToken(data []byte) []byte {
	oid := []byte{0x60, 0x37, 0x06, 0x06, 0x2b, 0x06, 0x01, 0x05, 0x05, 0x02}
	total := len(oid) + 2 + len(data)
	blob := make([]byte, 0, total)
	blob = append(blob, 0x60)
	blob = append(blob, encodeLength(total-1)...)
	blob = append(blob, oid...)
	blob = append(blob, encodeLength(len(data))...)
	blob = append(blob, data...)
	return blob
}

func encodeLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	return []byte{0x80 | byte(n>>7), byte(n & 0x7F)}
}

// computeNTLMHash computes the NTLM hash (MD4 of UTF-16LE password)
// 使用 unicode/utf16 包正确处理非 BMP 字符（代理对，4 字节），
// 避免 rune > 0xFFFF 时缓冲区不足导致越界 panic（修复 P0-32）
func computeNTLMHash(password string) []byte {
	// utf16.Encode 正确处理代理对，返回 []uint16
	encoded := utf16.Encode([]rune(password))
	// 转为小端序 []byte
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	h := md4.New()
	h.Write(buf)
	return h.Sum(nil)
}

// desECB performs single-block DES encryption in ECB mode
func desECB(key, plaintext []byte) []byte {
	if len(key) < 8 {
		padded := make([]byte, 8)
		copy(padded, key)
		key = padded
	}
	plain := make([]byte, 8)
	copy(plain, plaintext)

	block, err := des.NewCipher(key)
	if err != nil {
		return plain
	}
	out := make([]byte, 8)
	block.Encrypt(out, plain)
	return out
}
