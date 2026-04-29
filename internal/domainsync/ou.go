package domainsync

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// OUDiscoverer OU 发现器，用于从域控制器获取 OU 列表.
type OUDiscoverer struct {
	config DCConfig
}

// NewOUDiscoverer 创建 OU 发现器.
func NewOUDiscoverer(config DCConfig) *OUDiscoverer {
	return &OUDiscoverer{config: config}
}

// Discover 发现所有 OU.
func (d *OUDiscoverer) Discover() ([]*OU, error) {
	conn, err := d.connect()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer conn.Close()

	baseDN := d.config.BaseDN
	if baseDN == "" {
		baseDN = domainToDN(d.config.Domain)
	}

	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=organizationalUnit)",
		[]string{"dn", "ou", "description", "distinguishedName"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOUFetchFailed, err)
	}

	ous := make([]*OU, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		ou := &OU{
			DN:          entry.DN,
			Name:        entry.GetAttributeValue("ou"),
			Description: entry.GetAttributeValue("description"),
			Enabled:     true,
		}
		if ou.Name == "" {
			ou.Name = extractCNFromDN(entry.DN)
		}
		ou.ParentDN = extractParentDN(entry.DN)
		ou.Level = calculateLevel(entry.DN, baseDN)
		ous = append(ous, ou)
	}

	return ous, nil
}

// connect 连接到域控制器.
func (d *OUDiscoverer) connect() (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", d.config.Host, d.config.Port)
	if d.config.Port == 0 {
		if d.config.UseTLS {
			addr = fmt.Sprintf("%s:636", d.config.Host)
		} else {
			addr = fmt.Sprintf("%s:389", d.config.Host)
		}
	}

	timeout := d.config.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var conn *ldap.Conn
	var err error

	if d.config.UseTLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: d.config.SkipTLSVerify,
		}
		conn, err = ldap.DialTLS("tcp", addr, tlsCfg)
	} else {
		conn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}

	conn.SetTimeout(timeout)

	// 绑定
	if d.config.BindDN != "" {
		if err := conn.Bind(d.config.BindDN, d.config.BindPassword); err != nil {
			conn.Close()
			return nil, fmt.Errorf("绑定失败: %w", err)
		}
	}

	return conn, nil
}

// domainToDN 将域名转换为 DN. 例如 example.com -> dc=example,dc=com.
func domainToDN(domain string) string {
	if domain == "" {
		return ""
	}
	parts := strings.Split(domain, ".")
	dnParts := make([]string, 0, len(parts))
	for _, p := range parts {
		dnParts = append(dnParts, "dc="+p)
	}
	return strings.Join(dnParts, ",")
}

// extractCNFromDN 从 DN 中提取第一个 CN 或 OU 值.
func extractCNFromDN(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return dn
	}
	if len(parsed.RDNs) > 0 {
		for _, attr := range parsed.RDNs[0].Attributes {
			if strings.EqualFold(attr.Type, "ou") || strings.EqualFold(attr.Type, "cn") {
				return attr.Value
			}
		}
	}
	return dn
}

// extractParentDN 提取父 DN（去掉第一级 RDN）.
func extractParentDN(dn string) string {
	idx := strings.Index(dn, ",")
	if idx < 0 {
		return ""
	}
	return dn[idx+1:]
}

// calculateLevel 计算 OU 层级深度.
func calculateLevel(dn, baseDN string) int {
	dnParts := len(strings.Split(dn, ","))
	baseParts := 0
	if baseDN != "" {
		baseParts = len(strings.Split(baseDN, ","))
	}
	level := dnParts - baseParts
	if level < 0 {
		level = 0
	}
	return level
}
