package containerscanpro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CVEDatabase CVE 漏洞数据库
type CVEDatabase struct {
	mu          sync.RWMutex
	vulns       map[string]*CVEInfo
	packageVulns map[string][]string // package -> CVE IDs
	indexFile   string
	dataDir     string
	lastUpdated time.Time
}

// NewCVEDatabase 创建 CVE 数据库实例
func NewCVEDatabase(dataDir string) (*CVEDatabase, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db := &CVEDatabase{
		vulns:       make(map[string]*CVEInfo),
		packageVulns: make(map[string][]string),
		dataDir:     dataDir,
		indexFile:   filepath.Join(dataDir, "cve-index.json"),
	}

	// 加载现有数据
	if err := db.load(); err != nil {
		// 初始化内置漏洞库
		db.initBuiltinVulnerabilities()
	}
	
	// 如果没有加载到数据，初始化内置漏洞库
	if len(db.vulns) == 0 {
		db.initBuiltinVulnerabilities()
	}

	return db, nil
}

// load 从文件加载漏洞数据
func (db *CVEDatabase) load() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	data, err := os.ReadFile(db.indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var vulns []*CVEInfo
	if err := json.Unmarshal(data, &vulns); err != nil {
		return err
	}

	for _, v := range vulns {
		db.vulns[v.ID] = v
	}

	return nil
}

// save 保存漏洞数据到文件
func (db *CVEDatabase) save() error {
	db.mu.RLock()
	vulns := make([]*CVEInfo, 0, len(db.vulns))
	for _, v := range db.vulns {
		vulns = append(vulns, v)
	}
	db.mu.RUnlock()

	// 按 ID 排序
	sort.Slice(vulns, func(i, j int) bool {
		return vulns[i].ID < vulns[j].ID
	})

	data, err := json.MarshalIndent(vulns, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(db.indexFile, data, 0644)
}

// initBuiltinVulnerabilities 初始化内置常见漏洞库
func (db *CVEDatabase) initBuiltinVulnerabilities() {
	builtinVulns := []*CVEInfo{
		{
			ID:          "CVE-2024-3094",
			Title:       "XZ Utils Backdoor",
			Description: "xz/liblzma 后门，可导致远程代码执行",
			Severity:    SeverityCritical,
			Score:       10.0,
			PublishedAt: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-3094"},
			FixVersions: []string{"5.6.1+"},
		},
		{
			ID:          "CVE-2024-21626",
			Title:       "runc容器逃逸漏洞",
			Description: "runc中文件描述符泄漏导致容器逃逸",
			Severity:    SeverityCritical,
			Score:       8.6,
			PublishedAt: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-21626"},
			FixVersions: []string{"1.1.12+"},
		},
		{
			ID:          "CVE-2024-24557",
			Title:       "Docker Classic漏洞",
			Description: "classic builder中的cache poisoning",
			Severity:    SeverityHigh,
			Score:       7.5,
			PublishedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-24557"},
			FixVersions: []string{"25.0.2+"},
		},
		{
			ID:          "CVE-2023-44487",
			Title:       "HTTP/2 Rapid Reset Attack",
			Description: "HTTP/2协议拒绝服务攻击",
			Severity:    SeverityHigh,
			Score:       7.5,
			PublishedAt: time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2023-44487"},
			FixVersions: []string{"Go 1.21.3+", "Go 1.20.10+"},
		},
		{
			ID:          "CVE-2023-39325",
			Title:       "Go HTTP/2 DoS",
			Description: "Go net/http中HTTP/2流处理漏洞",
			Severity:    SeverityHigh,
			Score:       7.5,
			PublishedAt: time.Date(2023, 10, 5, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2023-39325"},
			FixVersions: []string{"Go 1.21.3+", "Go 1.20.10+"},
		},
		{
			ID:          "CVE-2024-6387",
			Title:       "OpenSSH regreSSHion",
			Description: "OpenSSH信号处理竞态条件漏洞",
			Severity:    SeverityHigh,
			Score:       8.1,
			PublishedAt: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-6387"},
			FixVersions: []string{"9.8p1+"},
		},
		{
			ID:          "CVE-2024-3596",
			Title:       "RADIUS协议漏洞",
			Description: "RADIUS协议中MD5碰撞攻击",
			Severity:    SeverityHigh,
			Score:       7.4,
			PublishedAt: time.Date(2024, 7, 9, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-3596"},
		},
		{
			ID:          "CVE-2024-28849",
			Title:       "gotenv凭证泄露",
			Description: "跟随重定向时泄露凭证",
			Severity:    SeverityMedium,
			Score:       5.9,
			PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-28849"},
			FixVersions: []string{"1.10.0+"},
		},
		{
			ID:          "CVE-2024-24790",
			Title:       "Go net/netip地址解析漏洞",
			Description: "net/netip中IPv4-mapped IPv6地址处理漏洞",
			Severity:    SeverityCritical,
			Score:       9.8,
			PublishedAt: time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-24790"},
			FixVersions: []string{"Go 1.22.4+", "Go 1.21.11+"},
		},
		{
			ID:          "CVE-2023-45288",
			Title:       "Go x/net/http2 CONTINUATION Flood",
			Description: "HTTP/2 CONTINUATION帧拒绝服务",
			Severity:    SeverityHigh,
			Score:       7.5,
			PublishedAt: time.Date(2024, 4, 3, 0, 0, 0, 0, time.UTC),
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2023-45288"},
			FixVersions: []string{"Go 1.22.2+", "Go 1.21.9+"},
		},
	}

	for _, v := range builtinVulns {
		db.vulns[v.ID] = v
	}
}

// Lookup 查询 CVE 信息
func (db *CVEDatabase) Lookup(cveID string) (*CVEInfo, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	vuln, ok := db.vulns[cveID]
	return vuln, ok
}

// SearchByKeyword 按关键词搜索
func (db *CVEDatabase) SearchByKeyword(keyword string) []*CVEInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var results []*CVEInfo
	keyword = strings.ToLower(keyword)

	for _, v := range db.vulns {
		if strings.Contains(strings.ToLower(v.ID), keyword) ||
			strings.Contains(strings.ToLower(v.Title), keyword) ||
			strings.Contains(strings.ToLower(v.Description), keyword) {
			results = append(results, v)
		}
	}

	// 按严重程度排序
	sort.Slice(results, func(i, j int) bool {
		return severityWeight(results[i].Severity) > severityWeight(results[j].Severity)
	})

	return results
}

// SearchBySeverity 按严重程度搜索
func (db *CVEDatabase) SearchBySeverity(severity Severity) []*CVEInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var results []*CVEInfo
	for _, v := range db.vulns {
		if v.Severity == severity {
			results = append(results, v)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// AddVulnerability 添加漏洞
func (db *CVEDatabase) AddVulnerability(vuln *CVEInfo) error {
	if vuln.ID == "" {
		return fmt.Errorf("CVE ID is required")
	}

	db.mu.Lock()
	db.vulns[vuln.ID] = vuln
	db.mu.Unlock()

	return db.save()
}

// GetStats 获取数据库统计
func (db *CVEDatabase) GetStats() map[string]int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := map[string]int{
		"total":    len(db.vulns),
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
		"info":     0,
	}

	for _, v := range db.vulns {
		switch v.Severity {
		case SeverityCritical:
			stats["critical"]++
		case SeverityHigh:
			stats["high"]++
		case SeverityMedium:
			stats["medium"]++
		case SeverityLow:
			stats["low"]++
		case SeverityInfo:
			stats["info"]++
		}
	}

	return stats
}

// UpdateFromRemote 从远程更新漏洞库（模拟）
func (db *CVEDatabase) UpdateFromRemote() error {
	// 实际实现中这里会调用 NVD API 或其他漏洞源
	// 这里模拟更新过程
	db.mu.Lock()
	db.lastUpdated = time.Now()
	db.mu.Unlock()

	return db.save()
}

// severityWeight 严重程度权重
func severityWeight(s Severity) int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// MatchPackages 匹配软件包已知漏洞
func (db *CVEDatabase) MatchPackages(packages []PackageInfo) []VulnerabilityCVE {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var vulns []VulnerabilityCVE

	for _, pkg := range packages {
		for _, v := range db.vulns {
			if db.isPackageAffected(pkg, v) {
				vulns = append(vulns, VulnerabilityCVE{
					CVEID:       v.ID,
					Severity:    v.Severity,
					Title:       v.Title,
					Description: v.Description,
					Package:     pkg.Name,
					Version:     pkg.InstalledVersion,
					CVSS:        v.Score,
					PublishedAt: v.PublishedAt,
				})
			}
		}
	}

	return vulns
}

// isPackageAffected 检查包是否受漏洞影响
func (db *CVEDatabase) isPackageAffected(pkg PackageInfo, vuln *CVEInfo) bool {
	// 简化版本：检查包名是否在漏洞描述中
	pkgName := strings.ToLower(pkg.Name)
	title := strings.ToLower(vuln.Title)
	desc := strings.ToLower(vuln.Description)

	return strings.Contains(title, pkgName) || strings.Contains(desc, pkgName)
}
