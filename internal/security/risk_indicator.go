// Package security 提供安全风险指标功能
// 实现基于 KEV 和 EPSS 的漏洞风险评分系统
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 风险指标类型定义 ==========

// RiskIndicator 风险指标
type RiskIndicator struct {
	ID             string     `json:"id"`
	CVEID          string     `json:"cve_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	RiskScore      float64    `json:"risk_score"`      // 综合风险评分 0-100
	RiskLevel      string     `json:"risk_level"`      // low, medium, high, critical
	CVSSScore      float64    `json:"cvss_score"`      // CVSS 基础评分
	EPSSScore      float64    `json:"epss_score"`      // EPSS 概率评分 0-1
	EPSSPercentile float64    `json:"epss_percentile"` // EPSS 百分位
	IsInKEV        bool       `json:"is_in_kev"`       // 是否在 KEV 目录中
	KEVInfo        *KEVInfo   `json:"kev_info,omitempty"`
	Exploitability string     `json:"exploitability"` // none, potential, likely, known
	Priority       int        `json:"priority"`       // 1-4, 1最高
	Remediation    string     `json:"remediation"`
	DueDate        *time.Time `json:"due_date,omitempty"` // CISA 修复期限
	LastUpdated    time.Time  `json:"last_updated"`
}

// KEVInfo KEV 漏洞详情
type KEVInfo struct {
	CVEID                      string     `json:"cve_id"`
	VendorProject              string     `json:"vendor_project"`
	Product                    string     `json:"product"`
	VulnerabilityName          string     `json:"vulnerability_name"`
	DateAdded                  time.Time  `json:"date_added"`
	ShortDescription           string     `json:"short_description"`
	RequiredAction             string     `json:"required_action"`
	DueDate                    *time.Time `json:"due_date,omitempty"`
	KnownRansomwareCampaignUse string     `json:"known_ransomware_campaign_use"` // Known, Unknown
	Notes                      string     `json:"notes,omitempty"`
	ActionsRequired            []string   `json:"actions_required"`
	ExploitReferences          []string   `json:"exploit_references"`
	IsHistoricallyExploited    bool       `json:"is_historically_exploited"`
	ActiveExploitation         bool       `json:"active_exploitation"`
}

// EPSSData EPSS 数据
type EPSSData struct {
	CVEID      string    `json:"cve"`
	EPSSScore  float64   `json:"epss"`
	Percentile float64   `json:"percentile"`
	Date       time.Time `json:"date"`
}

// RiskScoreFactors 风险评分因子
type RiskScoreFactors struct {
	CVSSWeight       float64 `json:"cvss_weight"`       // CVSS 权重
	EPSSWeight       float64 `json:"epss_weight"`       // EPSS 权重
	KEVWeight        float64 `json:"kev_weight"`        // KEV 权重
	AgeWeight        float64 `json:"age_weight"`        // 漏洞年龄权重
	AssetWeight      float64 `json:"asset_weight"`      // 资产重要性权重
	ExploitWeight    float64 `json:"exploit_weight"`    // 已知利用权重
	RansomwareWeight float64 `json:"ransomware_weight"` // 勒索软件权重
}

// DefaultRiskScoreFactors 默认风险评分因子
func DefaultRiskScoreFactors() RiskScoreFactors {
	return RiskScoreFactors{
		CVSSWeight:       0.25,
		EPSSWeight:       0.20,
		KEVWeight:        0.25,
		AgeWeight:        0.10,
		AssetWeight:      0.10,
		ExploitWeight:    0.05,
		RansomwareWeight: 0.05,
	}
}

// RiskIndicatorConfig 风险指标配置
type RiskIndicatorConfig struct {
	Enabled                 bool             `json:"enabled"`
	KEVSourceURL            string           `json:"kev_source_url"`
	EPSSSourceURL           string           `json:"epss_source_url"`
	KEVUpdateInterval       time.Duration    `json:"kev_update_interval"`
	EPSSUpdateInterval      time.Duration    `json:"epss_update_interval"`
	CacheDir                string           `json:"cache_dir"`
	RequestTimeout          time.Duration    `json:"request_timeout"`
	EPSSThreshold           float64          `json:"epss_threshold"`            // EPSS 高风险阈值
	EPSSPercentileThreshold float64          `json:"epss_percentile_threshold"` // EPSS 百分位阈值
	ScoreFactors            RiskScoreFactors `json:"score_factors"`
	AutoUpdate              bool             `json:"auto_update"`
	OfflineMode             bool             `json:"offline_mode"`
}

// DefaultRiskIndicatorConfig 默认配置
func DefaultRiskIndicatorConfig() RiskIndicatorConfig {
	return RiskIndicatorConfig{
		Enabled:                 true,
		KEVSourceURL:            "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json",
		EPSSSourceURL:           "https://api.first.org/data/v1/epss",
		KEVUpdateInterval:       24 * time.Hour,
		EPSSUpdateInterval:      24 * time.Hour,
		CacheDir:                "/var/lib/nas-os/risk-indicators",
		RequestTimeout:          30 * time.Second,
		EPSSThreshold:           0.1, // EPSS > 0.1 视为高风险
		EPSSPercentileThreshold: 0.9, // 百分位 > 90% 视为高风险
		ScoreFactors:            DefaultRiskScoreFactors(),
		AutoUpdate:              true,
		OfflineMode:             false,
	}
}

// RiskIndicatorManager 风险指标管理器
type RiskIndicatorManager struct {
	config       RiskIndicatorConfig
	kevCatalog   *KEVCatalog
	epssCache    map[string]*EPSSData
	riskCache    map[string]*RiskIndicator
	httpClient   *http.Client
	storageDir   string
	lastKEVSync  time.Time
	lastEPSSSync time.Time
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// KEVCatalog KEV 目录
type KEVCatalog struct {
	Title           string     `json:"title"`
	CatalogVersion  string     `json:"catalogVersion"`
	DateReleased    time.Time  `json:"dateReleased"`
	Count           int        `json:"count"`
	Vulnerabilities []KEVEntry `json:"vulnerabilities"`
}

// KEVEntry KEV 条目
type KEVEntry struct {
	CVEID                      string `json:"cveID"`
	VendorProject              string `json:"vendorProject"`
	Product                    string `json:"product"`
	VulnerabilityName          string `json:"vulnerabilityName"`
	DateAdded                  string `json:"dateAdded"`
	ShortDescription           string `json:"shortDescription"`
	RequiredAction             string `json:"requiredAction"`
	DueDate                    string `json:"dueDate"`
	KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
	Notes                      string `json:"notes"`
}

// ========== KEV 数据库集成 ==========

// NewRiskIndicatorManager 创建风险指标管理器
func NewRiskIndicatorManager(config RiskIndicatorConfig) *RiskIndicatorManager {
	ctx, cancel := context.WithCancel(context.Background())

	storageDir := config.CacheDir
	_ = os.MkdirAll(storageDir, 0750)

	rim := &RiskIndicatorManager{
		config:     config,
		kevCatalog: &KEVCatalog{},
		epssCache:  make(map[string]*EPSSData),
		riskCache:  make(map[string]*RiskIndicator),
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		storageDir: storageDir,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 加载本地缓存
	rim.loadCache()

	// 启动自动更新
	if config.AutoUpdate && !config.OfflineMode {
		go rim.autoUpdateLoop()
	}

	return rim
}

// loadCache 加载本地缓存
func (rim *RiskIndicatorManager) loadCache() {
	// 加载 KEV 缓存
	kevCachePath := filepath.Join(rim.storageDir, "kev_catalog.json")
	if data, err := os.ReadFile(kevCachePath); err == nil {
		var catalog KEVCatalog
		if err := json.Unmarshal(data, &catalog); err == nil {
			rim.kevCatalog = &catalog
		}
	}

	// 加载 EPSS 缓存
	epssCachePath := filepath.Join(rim.storageDir, "epss_cache.json")
	if data, err := os.ReadFile(epssCachePath); err == nil {
		var epssData map[string]*EPSSData
		if err := json.Unmarshal(data, &epssData); err == nil {
			rim.epssCache = epssData
		}
	}
}

// saveCache 保存缓存
func (rim *RiskIndicatorManager) saveCache() error {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	// 保存 KEV 缓存
	kevCachePath := filepath.Join(rim.storageDir, "kev_catalog.json")
	if data, err := json.MarshalIndent(rim.kevCatalog, "", "  "); err == nil {
		if err := os.WriteFile(kevCachePath, data, 0600); err != nil {
			return fmt.Errorf("保存 KEV 缓存失败: %w", err)
		}
	}

	// 保存 EPSS 缓存
	epssCachePath := filepath.Join(rim.storageDir, "epss_cache.json")
	if data, err := json.MarshalIndent(rim.epssCache, "", "  "); err == nil {
		if err := os.WriteFile(epssCachePath, data, 0600); err != nil {
			return fmt.Errorf("保存 EPSS 缓存失败: %w", err)
		}
	}

	return nil
}

// FetchKEVCatalog 获取 KEV 目录
func (rim *RiskIndicatorManager) FetchKEVCatalog(ctx context.Context) error {
	if rim.config.OfflineMode {
		return fmt.Errorf("离线模式下无法获取 KEV 目录")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rim.config.KEVSourceURL, nil)
	if err != nil {
		return fmt.Errorf("创建 KEV 请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NAS-OS-RiskIndicator/1.0")

	resp, err := rim.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("获取 KEV 目录失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("KEV API 返回错误状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取 KEV 响应失败: %w", err)
	}

	var catalog KEVCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return fmt.Errorf("解析 KEV 目录失败: %w", err)
	}

	rim.mu.Lock()
	rim.kevCatalog = &catalog
	rim.lastKEVSync = time.Now()
	rim.mu.Unlock()

	// 保存缓存
	return rim.saveCache()
}

// FetchEPSSData 获取 EPSS 数据
func (rim *RiskIndicatorManager) FetchEPSSData(ctx context.Context, cveIDs []string) (map[string]*EPSSData, error) {
	if rim.config.OfflineMode {
		return nil, fmt.Errorf("离线模式下无法获取 EPSS 数据")
	}

	if len(cveIDs) == 0 {
		return make(map[string]*EPSSData), nil
	}

	// 批量查询 EPSS（每次最多 100 个 CVE）
	results := make(map[string]*EPSSData)
	batchSize := 100

	for i := 0; i < len(cveIDs); i += batchSize {
		end := i + batchSize
		if end > len(cveIDs) {
			end = len(cveIDs)
		}

		batch := cveIDs[i:end]
		cveList := strings.Join(batch, ",")

		url := fmt.Sprintf("%s?cve=%s", rim.config.EPSSSourceURL, cveList)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "NAS-OS-RiskIndicator/1.0")

		resp, err := rim.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}

		var epssResp struct {
			Status     string `json:"status"`
			StatusCode int    `json:"status-code"`
			Version    string `json:"version"`
			Access     string `json:"access"`
			Total      int    `json:"total"`
			Offset     int    `json:"offset"`
			Limit      int    `json:"limit"`
			Data       []struct {
				CVE        string  `json:"cve"`
				EPSS       float64 `json:"epss"`
				Percentile float64 `json:"percentile"`
				Date       string  `json:"date"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&epssResp); err != nil {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()

		for _, item := range epssResp.Data {
			date, _ := time.Parse("2006-01-02", item.Date)
			results[item.CVE] = &EPSSData{
				CVEID:      item.CVE,
				EPSSScore:  item.EPSS,
				Percentile: item.Percentile,
				Date:       date,
			}
		}

		// 更新缓存
		rim.mu.Lock()
		for cve, data := range results {
			rim.epssCache[cve] = data
		}
		rim.mu.Unlock()
	}

	rim.mu.Lock()
	rim.lastEPSSSync = time.Now()
	rim.mu.Unlock()

	return results, nil
}

// GetKEVInfo 获取 KEV 信息
func (rim *RiskIndicatorManager) GetKEVInfo(cveID string) *KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if entry.CVEID == cveID {
			return rim.parseKEVEntry(entry)
		}
	}

	return nil
}

// parseKEVEntry 解析 KEV 条目
func (rim *RiskIndicatorManager) parseKEVEntry(entry KEVEntry) *KEVInfo {
	dateAdded, _ := time.Parse("2006-01-02", entry.DateAdded)

	var dueDate *time.Time
	if entry.DueDate != "" {
		if d, err := time.Parse("2006-01-02", entry.DueDate); err == nil {
			dueDate = &d
		}
	}

	isRansomware := entry.KnownRansomwareCampaignUse == "Known"

	return &KEVInfo{
		CVEID:                      entry.CVEID,
		VendorProject:              entry.VendorProject,
		Product:                    entry.Product,
		VulnerabilityName:          entry.VulnerabilityName,
		DateAdded:                  dateAdded,
		ShortDescription:           entry.ShortDescription,
		RequiredAction:             entry.RequiredAction,
		DueDate:                    dueDate,
		KnownRansomwareCampaignUse: entry.KnownRansomwareCampaignUse,
		Notes:                      entry.Notes,
		ActionsRequired:            rim.parseActionsRequired(entry.RequiredAction),
		ExploitReferences:          rim.parseExploitReferences(entry.Notes),
		IsHistoricallyExploited:    true,
		ActiveExploitation:         isRansomware,
	}
}

// parseActionsRequired 解析所需行动
func (rim *RiskIndicatorManager) parseActionsRequired(requiredAction string) []string {
	actions := []string{}

	// 常见行动关键词
	keywords := []string{
		"Apply updates",
		"Upgrade",
		"Patch",
		"Disable",
		"Remove",
		"Isolate",
		"Monitor",
		"Review",
		"Implement",
	}

	actionLower := strings.ToLower(requiredAction)
	for _, kw := range keywords {
		if strings.Contains(actionLower, strings.ToLower(kw)) {
			actions = append(actions, kw)
		}
	}

	if len(actions) == 0 {
		actions = append(actions, requiredAction)
	}

	return actions
}

// parseExploitReferences 解析利用参考
func (rim *RiskIndicatorManager) parseExploitReferences(notes string) []string {
	refs := []string{}

	// 提取 URL
	words := strings.Fields(notes)
	for _, word := range words {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			refs = append(refs, word)
		}
	}

	return refs
}

// GetEPSSData 获取 EPSS 数据
func (rim *RiskIndicatorManager) GetEPSSData(cveID string) *EPSSData {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	return rim.epssCache[cveID]
}

// IsInKEV 检查 CVE 是否在 KEV 目录中
func (rim *RiskIndicatorManager) IsInKEV(cveID string) bool {
	return rim.GetKEVInfo(cveID) != nil
}

// ========== 风险评分计算 ==========

// CalculateRiskScore 计算综合风险评分
func (rim *RiskIndicatorManager) CalculateRiskScore(
	cveID string,
	cvssScore float64,
	assetImportance float64,
	publishedDate time.Time,
) *RiskIndicator {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	indicator := &RiskIndicator{
		CVEID:       cveID,
		CVSSScore:   cvssScore,
		LastUpdated: time.Now(),
	}

	// 获取 KEV 信息
	kevInfo := rim.getKEVInfoUnlocked(cveID)
	if kevInfo != nil {
		indicator.IsInKEV = true
		indicator.KEVInfo = kevInfo
		indicator.Name = kevInfo.VulnerabilityName
		indicator.Description = kevInfo.ShortDescription
		indicator.Remediation = kevInfo.RequiredAction
		indicator.DueDate = kevInfo.DueDate
	}

	// 获取 EPSS 数据
	epssData := rim.epssCache[cveID]
	if epssData != nil {
		indicator.EPSSScore = epssData.EPSSScore
		indicator.EPSSPercentile = epssData.Percentile
	}

	// 计算风险评分
	indicator.RiskScore = rim.computeRiskScore(
		cvssScore,
		indicator.EPSSScore,
		indicator.IsInKEV,
		assetImportance,
		publishedDate,
		kevInfo,
	)

	// 确定风险等级
	indicator.RiskLevel = rim.scoreToRiskLevel(indicator.RiskScore)

	// 确定可利用性
	indicator.Exploitability = rim.determineExploitability(indicator)

	// 确定优先级
	indicator.Priority = rim.determinePriority(indicator)

	// 缓存结果
	rim.riskCache[cveID] = indicator

	return indicator
}

// computeRiskScore 计算风险评分核心逻辑
func (rim *RiskIndicatorManager) computeRiskScore(
	cvssScore float64,
	epssScore float64,
	isInKEV bool,
	assetImportance float64,
	publishedDate time.Time,
	kevInfo *KEVInfo,
) float64 {
	factors := rim.config.ScoreFactors

	// 1. CVSS 贡献 (0-100)
	cvssContribution := cvssScore * 10 * factors.CVSSWeight

	// 2. EPSS 贡献 (0-100)
	epssContribution := epssScore * 100 * factors.EPSSWeight

	// 3. KEV 贡献 (0-100)
	kevContribution := 0.0
	if isInKEV {
		kevContribution = 100 * factors.KEVWeight
	}

	// 4. 漏洞年龄贡献 (较新的漏洞风险更高)
	ageDays := time.Since(publishedDate).Hours() / 24
	var ageFactor float64
	switch {
	case ageDays < 30:
		ageFactor = 1.0
	case ageDays < 90:
		ageFactor = 0.8
	case ageDays < 365:
		ageFactor = 0.6
	default:
		ageFactor = 0.4
	}
	ageContribution := ageFactor * 100 * factors.AgeWeight

	// 5. 资产重要性贡献
	assetContribution := assetImportance * 100 * factors.AssetWeight

	// 6. 已知利用贡献
	exploitContribution := 0.0
	if isInKEV {
		exploitContribution = 100 * factors.ExploitWeight
	}

	// 7. 勒索软件关联贡献
	ransomwareContribution := 0.0
	if kevInfo != nil && kevInfo.ActiveExploitation {
		ransomwareContribution = 100 * factors.RansomwareWeight
	}

	// 综合评分
	totalScore := cvssContribution + epssContribution + kevContribution +
		ageContribution + assetContribution + exploitContribution +
		ransomwareContribution

	// 限制在 0-100 范围内
	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	return totalScore
}

// scoreToRiskLevel 分数转风险等级
func (rim *RiskIndicatorManager) scoreToRiskLevel(score float64) string {
	switch {
	case score >= 80:
		return "critical"
	case score >= 60:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}

// determineExploitability 确定可利用性
func (rim *RiskIndicatorManager) determineExploitability(indicator *RiskIndicator) string {
	if indicator.IsInKEV {
		if indicator.KEVInfo != nil && indicator.KEVInfo.ActiveExploitation {
			return "known"
		}
		return "known"
	}

	if indicator.EPSSScore >= rim.config.EPSSThreshold ||
		indicator.EPSSPercentile >= rim.config.EPSSPercentileThreshold {
		return "likely"
	}

	if indicator.EPSSScore > 0.01 {
		return "potential"
	}

	return "none"
}

// determinePriority 确定优先级
func (rim *RiskIndicatorManager) determinePriority(indicator *RiskIndicator) int {
	switch {
	case indicator.RiskScore >= 80:
		return 1 // 最高优先级
	case indicator.RiskScore >= 60:
		return 2
	case indicator.RiskScore >= 40:
		return 3
	default:
		return 4
	}
}

// getKEVInfoUnlocked 无锁获取 KEV 信息
func (rim *RiskIndicatorManager) getKEVInfoUnlocked(cveID string) *KEVInfo {
	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if entry.CVEID == cveID {
			return rim.parseKEVEntry(entry)
		}
	}
	return nil
}

// ========== 批量操作 ==========

// GetRiskIndicators 批量获取风险指标
func (rim *RiskIndicatorManager) GetRiskIndicators(vulnerabilities []VulnerabilityItem) []*RiskIndicator {
	indicators := make([]*RiskIndicator, 0, len(vulnerabilities))

	// 收集需要查询 EPSS 的 CVE
	cveIDs := make([]string, 0, len(vulnerabilities))
	for _, vuln := range vulnerabilities {
		cveIDs = append(cveIDs, vuln.CVEID)
	}

	// 批量获取 EPSS 数据（如果缓存中没有）
	if !rim.config.OfflineMode {
		missingEPSS := make([]string, 0)
		rim.mu.RLock()
		for _, cve := range cveIDs {
			if _, ok := rim.epssCache[cve]; !ok {
				missingEPSS = append(missingEPSS, cve)
			}
		}
		rim.mu.RUnlock()

		if len(missingEPSS) > 0 {
			_, _ = rim.FetchEPSSData(rim.ctx, missingEPSS)
		}
	}

	// 计算风险指标
	for _, vuln := range vulnerabilities {
		indicator := rim.CalculateRiskScore(
			vuln.CVEID,
			vuln.CVSSScore,
			1.0, // 默认资产重要性
			vuln.FirstDetected,
		)
		indicators = append(indicators, indicator)
	}

	// 保存缓存
	_ = rim.saveCache()

	return indicators
}

// ========== 报告生成 ==========

// RiskIndicatorReport 风险指标报告
type RiskIndicatorReport struct {
	GeneratedAt             time.Time             `json:"generated_at"`
	TotalVulnerabilities    int                   `json:"total_vulnerabilities"`
	CriticalCount           int                   `json:"critical_count"`
	HighCount               int                   `json:"high_count"`
	MediumCount             int                   `json:"medium_count"`
	LowCount                int                   `json:"low_count"`
	KEVCount                int                   `json:"kev_count"`
	HighEPSSCount           int                   `json:"high_epss_count"`
	ExploitableCount        int                   `json:"exploitable_count"`
	RansomwareRelated       int                   `json:"ransomware_related"`
	TopRisks                []*RiskIndicator      `json:"top_risks"`
	KEVVulnerabilities      []*RiskIndicator      `json:"kev_vulnerabilities"`
	HighEPSSVulnerabilities []*RiskIndicator      `json:"high_epss_vulnerabilities"`
	RemediationPriority     []RiskRemediationItem `json:"remediation_priority"`
	KEVCatalogInfo          *KEVCatalogInfo       `json:"kev_catalog_info"`
}

// RiskRemediationItem 风险修复优先级项
type RiskRemediationItem struct {
	CVEID       string     `json:"cve_id"`
	RiskScore   float64    `json:"risk_score"`
	Priority    int        `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Remediation string     `json:"remediation"`
	IsOverdue   bool       `json:"is_overdue,omitempty"`
}

// KEVCatalogInfo KEV 目录信息
type KEVCatalogInfo struct {
	Version      string    `json:"version"`
	LastUpdated  time.Time `json:"last_updated"`
	TotalEntries int       `json:"total_entries"`
}

// GenerateRiskReport 生成风险报告
func (rim *RiskIndicatorManager) GenerateRiskReport(vulnerabilities []VulnerabilityItem) *RiskIndicatorReport {
	indicators := rim.GetRiskIndicators(vulnerabilities)

	report := &RiskIndicatorReport{
		GeneratedAt:             time.Now(),
		TotalVulnerabilities:    len(indicators),
		TopRisks:                make([]*RiskIndicator, 0),
		KEVVulnerabilities:      make([]*RiskIndicator, 0),
		HighEPSSVulnerabilities: make([]*RiskIndicator, 0),
		RemediationPriority:     make([]RiskRemediationItem, 0),
	}

	// 统计并分类
	for _, ind := range indicators {
		switch ind.RiskLevel {
		case "critical":
			report.CriticalCount++
		case "high":
			report.HighCount++
		case "medium":
			report.MediumCount++
		default:
			report.LowCount++
		}

		if ind.IsInKEV {
			report.KEVCount++
			report.KEVVulnerabilities = append(report.KEVVulnerabilities, ind)

			if ind.KEVInfo != nil && ind.KEVInfo.ActiveExploitation {
				report.RansomwareRelated++
			}
		}

		if ind.EPSSScore >= rim.config.EPSSThreshold {
			report.HighEPSSCount++
			report.HighEPSSVulnerabilities = append(report.HighEPSSVulnerabilities, ind)
		}

		if ind.Exploitability == "known" || ind.Exploitability == "likely" {
			report.ExploitableCount++
		}
	}

	// 获取前 10 个最高风险
	sortedIndicators := make([]*RiskIndicator, len(indicators))
	copy(sortedIndicators, indicators)

	// 简单排序（冒泡排序，适用于小规模数据）
	for i := 0; i < len(sortedIndicators)-1; i++ {
		for j := i + 1; j < len(sortedIndicators); j++ {
			if sortedIndicators[j].RiskScore > sortedIndicators[i].RiskScore {
				sortedIndicators[i], sortedIndicators[j] = sortedIndicators[j], sortedIndicators[i]
			}
		}
	}

	topCount := 10
	if len(sortedIndicators) < topCount {
		topCount = len(sortedIndicators)
	}
	report.TopRisks = sortedIndicators[:topCount]

	// 生成修复优先级列表
	for _, ind := range report.KEVVulnerabilities {
		if ind.KEVInfo != nil {
			item := RiskRemediationItem{
				CVEID:       ind.CVEID,
				RiskScore:   ind.RiskScore,
				Priority:    ind.Priority,
				DueDate:     ind.DueDate,
				Remediation: ind.Remediation,
			}

			// 检查是否过期
			if ind.DueDate != nil && ind.DueDate.Before(time.Now()) {
				item.IsOverdue = true
			}

			report.RemediationPriority = append(report.RemediationPriority, item)
		}
	}

	// KEV 目录信息
	rim.mu.RLock()
	report.KEVCatalogInfo = &KEVCatalogInfo{
		Version:      rim.kevCatalog.CatalogVersion,
		LastUpdated:  rim.kevCatalog.DateReleased,
		TotalEntries: rim.kevCatalog.Count,
	}
	rim.mu.RUnlock()

	return report
}

// ========== 自动更新 ==========

// autoUpdateLoop 自动更新循环
func (rim *RiskIndicatorManager) autoUpdateLoop() {
	// 初始更新
	ctx := context.Background()
	_ = rim.FetchKEVCatalog(ctx)

	kevTicker := time.NewTicker(rim.config.KEVUpdateInterval)
	epssTicker := time.NewTicker(rim.config.EPSSUpdateInterval)

	defer func() {
		kevTicker.Stop()
		epssTicker.Stop()
	}()

	for {
		select {
		case <-rim.ctx.Done():
			return
		case <-kevTicker.C:
			_ = rim.FetchKEVCatalog(ctx)
		case <-epssTicker.C:
			// EPSS 数据按需获取，这里只做标记
			rim.mu.Lock()
			rim.lastEPSSSync = time.Now()
			rim.mu.Unlock()
		}
	}
}

// Stop 停止管理器
func (rim *RiskIndicatorManager) Stop() {
	rim.cancel()
	_ = rim.saveCache()
}

// GetStatus 获取状态
func (rim *RiskIndicatorManager) GetStatus() map[string]interface{} {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	return map[string]interface{}{
		"enabled":             rim.config.Enabled,
		"offline_mode":        rim.config.OfflineMode,
		"kev_catalog_version": rim.kevCatalog.CatalogVersion,
		"kev_entries":         len(rim.kevCatalog.Vulnerabilities),
		"kev_last_sync":       rim.lastKEVSync,
		"epss_cache_size":     len(rim.epssCache),
		"epss_last_sync":      rim.lastEPSSSync,
		"risk_cache_size":     len(rim.riskCache),
	}
}

// GetKEVCatalog 获取 KEV 目录
func (rim *RiskIndicatorManager) GetKEVCatalog() *KEVCatalog {
	rim.mu.RLock()
	defer rim.mu.RUnlock()
	return rim.kevCatalog
}

// GetAllKEVVulnerabilities 获取所有 KEV 漏洞
func (rim *RiskIndicatorManager) GetAllKEVVulnerabilities() []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0, len(rim.kevCatalog.Vulnerabilities))
	for _, entry := range rim.kevCatalog.Vulnerabilities {
		result = append(result, rim.parseKEVEntry(entry))
	}

	return result
}

// SearchKEV 搜索 KEV 目录
func (rim *RiskIndicatorManager) SearchKEV(query string) []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0)
	queryLower := strings.ToLower(query)

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		// 在多个字段中搜索
		if strings.Contains(strings.ToLower(entry.CVEID), queryLower) ||
			strings.Contains(strings.ToLower(entry.VendorProject), queryLower) ||
			strings.Contains(strings.ToLower(entry.Product), queryLower) ||
			strings.Contains(strings.ToLower(entry.VulnerabilityName), queryLower) ||
			strings.Contains(strings.ToLower(entry.ShortDescription), queryLower) {
			result = append(result, rim.parseKEVEntry(entry))
		}
	}

	return result
}

// FilterKEVByVendor 按厂商过滤 KEV
func (rim *RiskIndicatorManager) FilterKEVByVendor(vendor string) []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0)
	vendorLower := strings.ToLower(vendor)

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if strings.Contains(strings.ToLower(entry.VendorProject), vendorLower) {
			result = append(result, rim.parseKEVEntry(entry))
		}
	}

	return result
}

// FilterKEVByProduct 按产品过滤 KEV
func (rim *RiskIndicatorManager) FilterKEVByProduct(product string) []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0)
	productLower := strings.ToLower(product)

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if strings.Contains(strings.ToLower(entry.Product), productLower) {
			result = append(result, rim.parseKEVEntry(entry))
		}
	}

	return result
}

// FilterKEVByRansomware 过滤与勒索软件相关的 KEV
func (rim *RiskIndicatorManager) FilterKEVByRansomware() []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0)

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if entry.KnownRansomwareCampaignUse == "Known" {
			result = append(result, rim.parseKEVEntry(entry))
		}
	}

	return result
}

// GetOverdueKEVVulnerabilities 获取过期的 KEV 漏洞
func (rim *RiskIndicatorManager) GetOverdueKEVVulnerabilities() []*KEVInfo {
	rim.mu.RLock()
	defer rim.mu.RUnlock()

	result := make([]*KEVInfo, 0)
	now := time.Now()

	for _, entry := range rim.kevCatalog.Vulnerabilities {
		if entry.DueDate != "" {
			if dueDate, err := time.Parse("2006-01-02", entry.DueDate); err == nil {
				if dueDate.Before(now) {
					result = append(result, rim.parseKEVEntry(entry))
				}
			}
		}
	}

	return result
}

// ========== 风险指标统计 ==========

// RiskStatistics 风险统计
type RiskStatistics struct {
	TotalIndicators    int               `json:"total_indicators"`
	ByRiskLevel        map[string]int    `json:"by_risk_level"`
	ByExploitability   map[string]int    `json:"by_exploitability"`
	ByPriority         map[int]int       `json:"by_priority"`
	KEVPercentage      float64           `json:"kev_percentage"`
	HighEPSSPercentage float64           `json:"high_epss_percentage"`
	AverageRiskScore   float64           `json:"average_risk_score"`
	AverageEPSSScore   float64           `json:"average_epss_score"`
	TopVendors         []VendorRiskStats `json:"top_vendors"`
}

// VendorRiskStats 厂商风险统计
type VendorRiskStats struct {
	Vendor       string  `json:"vendor"`
	VulnCount    int     `json:"vuln_count"`
	AvgRiskScore float64 `json:"avg_risk_score"`
	KEVCount     int     `json:"kev_count"`
}

// GetStatistics 获取风险统计
func (rim *RiskIndicatorManager) GetStatistics(indicators []*RiskIndicator) *RiskStatistics {
	if len(indicators) == 0 {
		return &RiskStatistics{
			ByRiskLevel:      make(map[string]int),
			ByExploitability: make(map[string]int),
			ByPriority:       make(map[int]int),
			TopVendors:       make([]VendorRiskStats, 0),
		}
	}

	stats := &RiskStatistics{
		TotalIndicators:  len(indicators),
		ByRiskLevel:      make(map[string]int),
		ByExploitability: make(map[string]int),
		ByPriority:       make(map[int]int),
		TopVendors:       make([]VendorRiskStats, 0),
	}

	var totalScore, totalEPSS float64
	var kevCount, highEPSSCount int

	vendorStats := make(map[string]*VendorRiskStats)

	for _, ind := range indicators {
		// 风险等级统计
		stats.ByRiskLevel[ind.RiskLevel]++

		// 可利用性统计
		stats.ByExploitability[ind.Exploitability]++

		// 优先级统计
		stats.ByPriority[ind.Priority]++

		// 总分累计
		totalScore += ind.RiskScore
		totalEPSS += ind.EPSSScore

		// KEV 统计
		if ind.IsInKEV {
			kevCount++
		}

		// 高 EPSS 统计
		if ind.EPSSScore >= rim.config.EPSSThreshold {
			highEPSSCount++
		}

		// 厂商统计
		if ind.KEVInfo != nil {
			vendor := ind.KEVInfo.VendorProject
			if _, ok := vendorStats[vendor]; !ok {
				vendorStats[vendor] = &VendorRiskStats{
					Vendor: vendor,
				}
			}
			vendorStats[vendor].VulnCount++
			vendorStats[vendor].AvgRiskScore += ind.RiskScore
			if ind.IsInKEV {
				vendorStats[vendor].KEVCount++
			}
		}
	}

	// 计算平均值
	stats.AverageRiskScore = totalScore / float64(len(indicators))
	stats.AverageEPSSScore = totalEPSS / float64(len(indicators))
	stats.KEVPercentage = float64(kevCount) / float64(len(indicators)) * 100
	stats.HighEPSSPercentage = float64(highEPSSCount) / float64(len(indicators)) * 100

	// 计算厂商平均分
	for _, vs := range vendorStats {
		vs.AvgRiskScore = vs.AvgRiskScore / float64(vs.VulnCount)
		stats.TopVendors = append(stats.TopVendors, *vs)
	}

	return stats
}
