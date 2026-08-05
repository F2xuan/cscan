package scanner

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"
)

// genTestCert 生成用于测试的证书。
// signWithCA=false → 自签（IsSelfSigned=true）；signWithCA=true → 由独立 CA 签发（IsSelfSigned=false）。
func genTestCert(t *testing.T, notAfter time.Time, dnsNames []string, signWithCA bool) tls.Certificate {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test.example.com", Organization: []string{"Test"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames:              dnsNames,
		BasicConstraintsValid: true,
	}
	parent := tmpl
	signer := priv
	if signWithCA {
		caPriv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		caTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(99),
			Subject:               pkix.Name{CommonName: "TestCA"},
			NotBefore:              time.Now().Add(-time.Hour),
			NotAfter:               time.Now().Add(24 * time.Hour),
			IsCA:                   true,
			BasicConstraintsValid:  true,
			KeyUsage:               x509.KeyUsageCertSign,
		}
		parent = caTmpl
		signer = caPriv
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &priv.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
}

// startTestTLSServer 用给定证书启动一个 TLS 服务端（仅完成握手即关闭），返回可连接的 host:port。
func startTestTLSServer(t *testing.T, cert tls.Certificate) (string, int) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return "127.0.0.1", port
}

// certcheck 不再作为独立扫描阶段注册（证书抓取是指纹识别的附加功能），此处不再校验注册。
func TestBuildCertResult(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name         string
		notAfter     time.Time
		dnsNames     []string
		signWithCA   bool
		wantSelfSign bool
		wantSANs     int
	}{
		{"valid-selfsigned", now.Add(365 * 24 * time.Hour), []string{"a.example.com", "b.example.com"}, false, true, 2},
		{"expired-ca-signed", now.Add(-48 * time.Hour), []string{"x.example.com"}, true, false, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cert := genTestCert(t, c.notAfter, c.dnsNames, c.signWithCA)
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				t.Fatal(err)
			}
			cr := buildCertResult(certTarget{Host: "h", Port: 443, Authority: "h:443"}, leaf)
			if cr.IsSelfSigned != c.wantSelfSign {
				t.Errorf("IsSelfSigned=%v want %v", cr.IsSelfSigned, c.wantSelfSign)
			}
			if len(cr.SANs) != c.wantSANs {
				t.Errorf("SANs=%v want %d", cr.SANs, c.wantSANs)
			}
			if cr.Fingerprints["sha1"] == "" || cr.Fingerprints["sha256"] == "" {
				t.Errorf("fingerprints missing: %v", cr.Fingerprints)
			}
			if len(cr.Fingerprints["sha256"]) != 64 {
				t.Errorf("sha256 fingerprint length=%d want 64", len(cr.Fingerprints["sha256"]))
			}
			if len(cr.Fingerprints["sha1"]) != 40 {
				t.Errorf("sha1 fingerprint length=%d want 40", len(cr.Fingerprints["sha1"]))
			}
			if len(cr.Fingerprints["md5"]) != 32 {
				t.Errorf("md5 fingerprint length=%d want 32", len(cr.Fingerprints["md5"]))
			}
			if cr.SerialNumber == "" || cr.SigAlg == "" {
				t.Errorf("serial/sig algo empty")
			}
			if cr.SubjectDN == "" || cr.IssuerDN == "" {
				t.Errorf("subject/issuer DN empty")
			}
			if cr.Subject.CommonName == "" || cr.Issuer.CommonName == "" {
				t.Errorf("structured subject/issuer commonName empty")
			}
			if cr.NotBefore.IsZero() || cr.NotAfter.IsZero() {
				t.Errorf("validity zero")
			}
		})
	}
}

func TestFetchCertExpiredServer(t *testing.T) {
	cert := genTestCert(t, time.Now().Add(-48*time.Hour), []string{"expired.example.com"}, false)
	host, port := startTestTLSServer(t, cert)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cr := FetchCert(ctx, host, port, 5*time.Second)
	if cr == nil {
		t.Fatalf("expected cert result, got nil")
	}
	daysLeft := int(cr.NotAfter.Sub(time.Now()).Hours() / 24)
	if daysLeft >= 0 {
		t.Errorf("expected negative daysLeft for expired cert, got %d", daysLeft)
	}
}

func TestFetchCertSelfSigned(t *testing.T) {
	cert := genTestCert(t, time.Now().Add(365*24*time.Hour), []string{"ok.example.com"}, false)
	host, port := startTestTLSServer(t, cert)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cr := FetchCert(ctx, host, port, 5*time.Second)
	if cr == nil {
		t.Fatalf("expected cert result, got nil")
	}
	if !cr.IsSelfSigned {
		t.Errorf("expected IsSelfSigned=true")
	}
}

func TestFetchCertUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cr := FetchCert(ctx, "127.0.0.1", closedPort, 2*time.Second)
	if cr != nil {
		t.Errorf("expected nil for unreachable target, got %+v", cr)
	}
}

func TestIsCertFetchTarget(t *testing.T) {
	cases := []struct {
		name string
		port int
		svc  string
		want bool
	}{
		{"https-443", 443, "", true},
		{"https-8443", 8443, "", true},
		{"https-service", 8080, "https", true},
		{"smtps-465", 465, "", true},
		{"imaps-993", 993, "", true},
		{"plain-http-80", 80, "http", false},
		{"random-non-tls-8080", 8080, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &Asset{Host: "h", Port: c.port, Service: c.svc}
			if got := isCertFetchTarget(a); got != c.want {
				t.Errorf("isCertFetchTarget(port=%d, svc=%q)=%v want %v", c.port, c.svc, got, c.want)
			}
		})
	}
}
