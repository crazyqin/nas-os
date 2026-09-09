// Package webtls 提供 Web UI 的 HTTPS 证书管理（自签生成 / 用户上传）
// 与同端口双协议（TLS + 明文回退）服务能力。
//
// 设计要点：
//   - 证书与跳转开关持久化在 DataDir/webtls/（config 目录在容器部署常为只读挂载）。
//   - 证书热加载：TLSConfig 使用 GetCertificate，生成/上传后无需重启即生效。
//   - Enabled 只控制明文 → HTTPS 的 301 跳转；证书存在时 TLS 端口始终可用。
package webtls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// stateFile 启用状态持久化文件名.
	stateFile = "state.json"
	// certFile / keyFile PEM 证书与私钥文件名.
	certFile = "cert.pem"
	keyFile  = "key.pem"

	// SourceSelfSigned 自签生成的证书来源标记.
	SourceSelfSigned = "self-signed"
	// SourceUser 用户上传的证书来源标记.
	SourceUser = "user"

	// selfSignedValidity 自签证书有效期（约 2.25 年，与主流浏览器自签策略一致）.
	selfSignedValidity = 825 * 24 * time.Hour

	// maxPEMSize 单个 PEM 文件的大小上限（含证书链的正常证书远小于此值）.
	maxPEMSize = 64 << 10

	// maxHosts 自签证书 SAN 条目上限.
	maxHosts = 20
)

// Status 是 /system/tls 返回的证书与跳转状态.
type Status struct {
	Enabled       bool      `json:"enabled"`           // 明文 → HTTPS 跳转开关
	HasCert       bool      `json:"hasCert"`           // 是否已有证书
	Source        string    `json:"source"`            // "" | self-signed | user
	Subject       string    `json:"subject,omitempty"` // 证书 CN
	NotAfter      time.Time `json:"notAfter"`          // 证书到期时间（无证书时为零值）
	DaysRemaining int       `json:"daysRemaining"`     // 距到期剩余天数（可负）
	SANs          []string  `json:"sans,omitempty"`    // 证书覆盖的域名/IP
}

// stateFileJSON 是 state.json 的磁盘结构.
type stateFileJSON struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}

// Manager 管理证书生命周期与跳转开关（并发安全）.
type Manager struct {
	dir string

	mu      sync.Mutex
	enabled bool
	source  string
	cert    *tls.Certificate
	leaf    *x509.Certificate
}

// NewManager 创建管理器，dir 为状态目录（DataDir/webtls）.
// 后续需调用 Load 才会读取磁盘状态.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

// Load 读取磁盘状态与证书对；证书缺失不算错误（默认无证书）.
// 证书损坏返回错误，调用方按"无证书 + 跳转关闭"降级处理.
func (m *Manager) Load() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("创建目录 %s 失败：%w", m.dir, err)
	}
	if data, err := os.ReadFile(filepath.Join(m.dir, stateFile)); err == nil {
		var st stateFileJSON
		if json.Unmarshal(data, &st) == nil {
			m.mu.Lock()
			m.enabled = st.Enabled
			m.source = st.Source
			m.mu.Unlock()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取状态文件失败：%w", err)
	}

	certPEM, errC := os.ReadFile(filepath.Join(m.dir, certFile))
	keyPEM, errK := os.ReadFile(filepath.Join(m.dir, keyFile))
	if errors.Is(errC, os.ErrNotExist) && errors.Is(errK, os.ErrNotExist) {
		return nil
	}
	if errC != nil {
		return fmt.Errorf("读取证书失败：%w", errC)
	}
	if errK != nil {
		return fmt.Errorf("读取私钥失败：%w", errK)
	}
	pair, leaf, err := parsePair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cert = pair
	m.leaf = leaf
	m.mu.Unlock()
	return nil
}

// Status 返回当前证书与跳转状态快照.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

// SetEnabled 开/关明文 → HTTPS 跳转；开启时要求已有证书.
func (m *Manager) SetEnabled(enabled bool) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled && m.cert == nil {
		return Status{}, errors.New("尚未生成或上传证书")
	}
	m.enabled = enabled
	if err := m.persistStateLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

// Generate 生成自签证书；hosts 为空时自动收集本机主机名/出站 IP/loopback.
// 生成后跳转开关保持原状（需显式开启）.
func (m *Manager) Generate(hosts []string) (Status, error) {
	hosts = normalizeHosts(hosts)
	if len(hosts) == 0 {
		return Status{}, errors.New("未提供有效的主机名或 IP")
	}
	for _, h := range hosts {
		if !validHost(h) {
			return Status{}, fmt.Errorf("无效的主机名或 IP：%q", h)
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Status{}, fmt.Errorf("生成私钥失败：%w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return Status{}, fmt.Errorf("生成序列号失败：%w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: hosts[0], Organization: []string{"NAS-OS"}},
		NotBefore:             time.Now().Add(-time.Hour), // 容忍时钟偏差
		NotAfter:              time.Now().Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return Status{}, fmt.Errorf("签发证书失败：%w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return Status{}, fmt.Errorf("解析证书失败：%w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return Status{}, fmt.Errorf("序列化私钥失败：%w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.savePairLocked(certPEM, keyPEM); err != nil {
		return Status{}, err
	}
	m.cert = &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
	m.leaf = leaf
	m.source = SourceSelfSigned
	if err := m.persistStateLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

// Import 导入用户证书（PEM 文本，可含证书链）；要求未过期且具备 ServerAuth 用途.
func (m *Manager) Import(certPEM, keyPEM []byte) (Status, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return Status{}, errors.New("证书与私钥均不能为空")
	}
	if len(certPEM) > maxPEMSize || len(keyPEM) > maxPEMSize {
		return Status{}, errors.New("证书或私钥文件过大（上限 64KB）")
	}
	pair, leaf, err := parsePair(certPEM, keyPEM)
	if err != nil {
		return Status{}, err
	}
	if !leaf.NotAfter.After(time.Now()) {
		return Status{}, errors.New("证书已过期")
	}
	if len(leaf.ExtKeyUsage) > 0 && !hasServerAuth(leaf.ExtKeyUsage) {
		return Status{}, errors.New("证书缺少服务器认证（ServerAuth）用途")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.savePairLocked(certPEM, keyPEM); err != nil {
		return Status{}, err
	}
	m.cert = pair
	m.leaf = leaf
	m.source = SourceUser
	if err := m.persistStateLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

// Remove 删除证书与私钥并关闭跳转（状态文件保留 enabled=false）.
func (m *Manager) Remove() (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range []string{certFile, keyFile} {
		if err := os.Remove(filepath.Join(m.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Status{}, fmt.Errorf("删除 %s 失败：%w", name, err)
		}
	}
	m.cert = nil
	m.leaf = nil
	m.source = ""
	m.enabled = false
	if err := m.persistStateLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

// RedirectActive 报告是否应对非本机明文请求做 HTTPS 跳转
// （开关开启 + 证书存在且未过期）.
func (m *Manager) RedirectActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled && m.hasValidCertLocked()
}

// HasValidCert 报告当前是否有未过期证书（TLS 可达）.
func (m *Manager) HasValidCert() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hasValidCertLocked()
}

// TLSConfig 返回用于双协议监听的 TLS 配置（GetCertificate 热加载）.
func (m *Manager) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.cert == nil {
				return nil, errors.New("webtls: 证书不可用")
			}
			return m.cert, nil
		},
	}
}

// ---- 内部实现 ----

func (m *Manager) hasValidCertLocked() bool {
	return m.cert != nil && m.leaf != nil && m.leaf.NotAfter.After(time.Now())
}

func (m *Manager) statusLocked() Status {
	st := Status{Enabled: m.enabled, HasCert: m.cert != nil, Source: m.source}
	if m.leaf != nil {
		st.Subject = m.leaf.Subject.CommonName
		if st.Subject == "" {
			st.Subject = m.leaf.Subject.String()
		}
		st.NotAfter = m.leaf.NotAfter
		st.DaysRemaining = int(time.Until(m.leaf.NotAfter).Hours() / 24)
		st.SANs = append(st.SANs, m.leaf.DNSNames...)
		for _, ip := range m.leaf.IPAddresses {
			st.SANs = append(st.SANs, ip.String())
		}
	}
	return st
}

// savePairLocked 原子写入证书（0644）与私钥（0600）；调用方持有锁.
func (m *Manager) savePairLocked(certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("创建目录 %s 失败：%w", m.dir, err)
	}
	if err := writeAtomic(filepath.Join(m.dir, keyFile), keyPEM, 0o600); err != nil {
		return fmt.Errorf("写入私钥失败：%w", err)
	}
	if err := writeAtomic(filepath.Join(m.dir, certFile), certPEM, 0o644); err != nil {
		return fmt.Errorf("写入证书失败：%w", err)
	}
	return nil
}

func (m *Manager) persistStateLocked() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("创建目录 %s 失败：%w", m.dir, err)
	}
	data, err := json.MarshalIndent(stateFileJSON{Enabled: m.enabled, Source: m.source}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(m.dir, stateFile), data, 0o644)
}

// parsePair 解析并匹配 PEM 证书/私钥，返回带 leaf 的 tls.Certificate.
func parsePair(certPEM, keyPEM []byte) (*tls.Certificate, *x509.Certificate, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("证书/私钥不匹配或格式错误：%w", err)
	}
	leaf := pair.Leaf
	if leaf == nil {
		block, _ := pem.Decode(certPEM)
		if block == nil {
			return nil, nil, errors.New("证书 PEM 解析失败")
		}
		if leaf, err = x509.ParseCertificate(block.Bytes); err != nil {
			return nil, nil, fmt.Errorf("解析证书失败：%w", err)
		}
		pair.Leaf = leaf
	}
	return &pair, leaf, nil
}

func hasServerAuth(usages []x509.ExtKeyUsage) bool {
	for _, u := range usages {
		if u == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}

// writeAtomic 临时文件 + 原子改名（与 sysupdate 持久化策略一致）.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// normalizeHosts 规整主机列表：去空白、去重、限制数量；
// 输入全为空时返回本机默认值（主机名 + 出站 IP + loopback）.
func normalizeHosts(in []string) []string {
	trimmed := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, h := range in {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		trimmed = append(trimmed, h)
	}
	if len(trimmed) == 0 {
		return defaultHosts()
	}
	if len(trimmed) > maxHosts {
		trimmed = trimmed[:maxHosts]
	}
	return trimmed
}

// defaultHosts 自动收集本机标识作为默认 SAN.
func defaultHosts() []string {
	hosts := []string{"localhost", "127.0.0.1"}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		hosts = append(hosts, strings.ToLower(hn))
	}
	if ip := outboundIP(); ip != "" {
		hosts = append(hosts, ip)
	}
	return hosts
}

// outboundIP 通过 UDP 拨号探测默认路由出站 IP（不实际发包）.
func outboundIP() string {
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	if udp, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return udp.IP.String()
	}
	return ""
}

// validHost 校验 DNS 名称（含通配符首标签）或 IP 字面量.
func validHost(h string) bool {
	if net.ParseIP(h) != nil {
		return true
	}
	if h == "" || len(h) > 253 {
		return false
	}
	h = strings.TrimPrefix(h, "*.")
	if h == "" {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, r := range label {
			ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
			if !ok {
				return false
			}
		}
	}
	return true
}
