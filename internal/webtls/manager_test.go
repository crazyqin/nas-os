// manager_test.go - webtls 证书管理测试（不触网：自签生成/导入/状态/持久化）.
package webtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerLifecycle(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// 初始状态：无证书、跳转关
	st := m.Status()
	if st.HasCert || st.Enabled || st.Source != "" {
		t.Fatalf("初始状态异常: %+v", st)
	}
	if m.HasValidCert() || m.RedirectActive() {
		t.Fatal("无证书时不应报告可用")
	}

	// 无证书时开跳转应失败
	if _, err := m.SetEnabled(true); err == nil {
		t.Fatal("无证书开启跳转应报错")
	}

	// 生成自签证书
	st, err := m.Generate([]string{"nas.local", "192.168.1.10", "  ", "nas.local"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !st.HasCert || st.Source != SourceSelfSigned {
		t.Fatalf("生成后状态异常: %+v", st)
	}
	if len(st.SANs) != 2 { // 去重去空后应恰为 2 条
		t.Fatalf("SAN 去重失败: %v", st.SANs)
	}
	if st.Subject != "nas.local" {
		t.Fatalf("CN 应为首个主机名: %q", st.Subject)
	}
	if st.DaysRemaining < 800 || st.DaysRemaining > 830 {
		t.Fatalf("自签有效期应约 825 天: %d", st.DaysRemaining)
	}
	if !m.HasValidCert() {
		t.Fatal("生成后应有有效证书")
	}

	// 开启跳转
	if st, err = m.SetEnabled(true); err != nil || !st.Enabled {
		t.Fatalf("SetEnabled(true): %v %+v", err, st)
	}
	if !m.RedirectActive() {
		t.Fatal("证书+开关齐备时跳转应生效")
	}

	// 重新 Load：状态应从磁盘恢复
	m2 := NewManager(dir)
	if err := m2.Load(); err != nil {
		t.Fatalf("二次 Load: %v", err)
	}
	st = m2.Status()
	if !st.HasCert || !st.Enabled || st.Source != SourceSelfSigned {
		t.Fatalf("持久化状态丢失: %+v", st)
	}
	if !m2.RedirectActive() {
		t.Fatal("重启后跳转应保持")
	}

	// 删除证书：跳转联动关闭
	if st, err = m2.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if st.HasCert || st.Enabled {
		t.Fatalf("删除后状态异常: %+v", st)
	}
	if m2.HasValidCert() || m2.RedirectActive() {
		t.Fatal("删除后不应残留可用状态")
	}
	if _, err := os.Stat(filepath.Join(dir, keyFile)); !os.IsNotExist(err) {
		t.Fatal("私钥文件应已删除")
	}
}

func TestManagerGenerateValidation(t *testing.T) {
	m := NewManager(t.TempDir())
	if _, err := m.Generate([]string{"bad host!"}); err == nil {
		t.Fatal("非法主机名应报错")
	}
	// 空列表 → 走默认收集（至少含 localhost）
	st, err := m.Generate(nil)
	if err != nil {
		t.Fatalf("默认主机收集: %v", err)
	}
	found := false
	for _, s := range st.SANs {
		if s == "localhost" {
			found = true
		}
	}
	if !found {
		t.Fatalf("默认 SAN 应含 localhost: %v", st.SANs)
	}
}

func TestManagerImport(t *testing.T) {
	m := NewManager(t.TempDir())
	certPEM, keyPEM := makeUserCert(t, time.Now().Add(90*24*time.Hour))

	st, err := m.Import(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if st.Source != SourceUser || st.Subject != "import.example.com" {
		t.Fatalf("导入后状态异常: %+v", st)
	}

	// 错配的私钥应失败
	_, otherKey := makeUserCert(t, time.Now().Add(90*24*time.Hour))
	if _, err := m.Import(certPEM, otherKey); err == nil {
		t.Fatal("证书私钥不匹配应报错")
	}

	// 过期证书应被拒绝
	expiredPEM, expiredKey := makeUserCert(t, time.Now().Add(-24*time.Hour))
	if _, err := m.Import(expiredPEM, expiredKey); err == nil {
		t.Fatal("过期证书应报错")
	}

	// 仅客户端用途（缺 ServerAuth）的证书应被拒绝
	clientPEM, clientKey := makeClientOnlyCert(t, time.Now().Add(90*24*time.Hour))
	if _, err := m.Import(clientPEM, clientKey); err == nil {
		t.Fatal("缺少 ServerAuth 用途的证书应报错")
	}

	// 空 body
	if _, err := m.Import(nil, keyPEM); err == nil {
		t.Fatal("空证书应报错")
	}
}

func TestRedirectDisablesWhenCertExpires(t *testing.T) {
	// 跳转开关开着但证书已过期 → RedirectActive 应为 false（防跳到死证书）.
	// 用短有效期证书：导入时仍有效，轮询等待其过期.
	m := NewManager(t.TempDir())
	notAfter := time.Now().Add(1 * time.Second)
	pemC, pemK := makeUserCert(t, notAfter)
	if _, err := m.Import(pemC, pemK); err != nil {
		t.Fatalf("Import 短效期证书: %v", err)
	}
	if _, err := m.SetEnabled(true); err != nil {
		t.Fatalf("有效期内开启跳转: %v", err)
	}
	if !m.RedirectActive() {
		t.Fatal("有效期内跳转应生效")
	}
	deadline := time.Now().Add(5 * time.Second) // 等待过期（含慢机器余量）
	for time.Now().Before(notAfter) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if m.HasValidCert() {
		t.Fatal("过期后不应再报告有效证书")
	}
	if m.RedirectActive() {
		t.Fatal("证书过期后跳转应联动失效")
	}
}

func TestTLSConfigServesCertificate(t *testing.T) {
	// 端到端：Generate 后 TLSConfig 的 GetCertificate 应能返回可用证书.
	m := NewManager(t.TempDir())
	if _, err := m.Generate([]string{"127.0.0.1"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	cfg := m.TLSConfig()
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion 应为 TLS1.2: %x", cfg.MinVersion)
	}
	if cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "127.0.0.1"}); err != nil || cert == nil {
		t.Fatalf("GetCertificate: %v %v", cert, err)
	}
	// 删除后 GetCertificate 返回错误（热卸载）
	if _, err := m.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := cfg.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("删除后 GetCertificate 应报错")
	}
}

// makeUserCert 构造测试用用户证书（ServerAuth 用途，模拟用户上传场景）.
func makeUserCert(t *testing.T, notAfter time.Time) (certPEM, keyPEM []byte) {
	return makeCert(t, notAfter, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
}

// makeClientOnlyCert 构造仅含 ClientAuth 用途的证书（应被 Import 拒绝）.
func makeClientOnlyCert(t *testing.T, notAfter time.Time) (certPEM, keyPEM []byte) {
	return makeCert(t, notAfter, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
}

// makeCert 构造测试用证书（eku 为扩展用途）.
func makeCert(t *testing.T, notAfter time.Time, eku []x509.ExtKeyUsage) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成测试私钥: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "import.example.com"},
		NotBefore:    notAfter.Add(-365 * 24 * time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"import.example.com"},
		IPAddresses:  []net.IP{net.ParseIP("10.0.0.5")},
		ExtKeyUsage:  eku,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("签发测试证书: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("序列化测试私钥: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}
