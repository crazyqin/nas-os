// Package threat - 威胁优先级系统管理器
// 整合 KEV/EPSS/LEV 的统一管理接口
package threat

import (
	"context"
	"sync"
	"time"
)

// ========== 威胁优先级系统管理器 ==========

// ThreatManager 威胁优先级管理器.
type ThreatManager struct {
	kevDB   *KEVDatabase
	epssDB  *EPSSDatabase
	levCalc *LEVCalculator
	config  ThreatConfig
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// ThreatConfig 威胁系统配置.
type ThreatConfig struct {
	KEV      KEVConfig  `json:"kev"`
	EPSS     EPSSConfig `json:"epss"`
	LEV      LEVConfig  `json:"lev"`
	Enabled  bool       `json:"enabled"`
	AutoSync bool       `json:"auto_sync"`
}

// DefaultThreatConfig 默认威胁系统配置.
func DefaultThreatConfig() ThreatConfig {
	return ThreatConfig{
		KEV:      DefaultKEVConfig(),
		EPSS:     DefaultEPSSConfig(),
		LEV:      DefaultLEVConfig(),
		Enabled:  true,
		AutoSync: true,
	}
}

// NewThreatManager 创建威胁管理器.
func NewThreatManager(config ThreatConfig) *ThreatManager {
	ctx, cancel := context.WithCancel(context.Background())

	tm := &ThreatManager{
		kevDB:   NewKEVDatabase(config.KEV),
		epssDB:  NewEPSSDatabase(config.EPSS),
		levCalc: NewLEVCalculator(config.LEV),
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}

	// 启动自动同步
	if config.AutoSync && config.Enabled {
		go tm.autoSyncLoop()
	}

	return tm
}

// autoSyncLoop 自动同步循环.
func (tm *ThreatManager) autoSyncLoop() {
	// 初始同步
	_ = tm.SyncKEV(tm.ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			_ = tm.SyncKEV(tm.ctx)
		}
	}
}

// Stop 停止管理器.
func (tm *ThreatManager) Stop() {
	tm.cancel()
}

// ========== KEV 操作 ==========

// SyncKEV 同步 KEV 目录.
func (tm *ThreatManager) SyncKEV(ctx context.Context) error {
	return tm.kevDB.Sync(ctx)
}

// IsInKEV 检查是否在 KEV 目录中.
func (tm *ThreatManager) IsInKEV(cveID string) bool {
	return tm.kevDB.IsInKEV(cveID)
}

// GetKEVEntry 获取 KEV 条目.
func (tm *ThreatManager) GetKEVEntry(cveID string) *KEVEntry {
	return tm.kevDB.GetKEVEntry(cveID)
}

// GetAllKEVEntries 获取所有 KEV 条目.
func (tm *ThreatManager) GetAllKEVEntries() []KEVEntry {
	return tm.kevDB.GetAllEntries()
}

// GetRansomwareKEV 获取勒索软件相关 KEV.
func (tm *ThreatManager) GetRansomwareKEV() []KEVEntry {
	return tm.kevDB.GetRansomwareRelated()
}

// GetOverdueKEV 获取过期 KEV.
func (tm *ThreatManager) GetOverdueKEV() []KEVEntry {
	return tm.kevDB.GetOverdue()
}

// SearchKEV 搜索 KEV.
func (tm *ThreatManager) SearchKEV(query string) []KEVEntry {
	return tm.kevDB.Search(query)
}

// ========== EPSS 操作 ==========

// FetchEPSS 获取 EPSS 数据.
func (tm *ThreatManager) FetchEPSS(ctx context.Context, cveIDs []string) (map[string]*EPSSData, error) {
	return tm.epssDB.FetchEPSS(ctx, cveIDs)
}

// GetEPSS 获取单个 EPSS 数据.
func (tm *ThreatManager) GetEPSS(cveID string) *EPSSData {
	return tm.epssDB.GetEPSS(cveID)
}

// GetHighEPSS 获取高 EPSS 评分漏洞.
func (tm *ThreatManager) GetHighEPSS(threshold float64) []*EPSSData {
	return tm.epssDB.GetHighEPSS(threshold)
}

// ========== LEV 计算 ==========

// CalculateLEV 计算 LEV 评分.
func (tm *ThreatManager) CalculateLEV(input LEVInput) *LEVScore {
	// 补充 KEV 和 EPSS 数据
	if input.IsInKEV {
		// 已提供
	} else {
		input.IsInKEV = tm.IsInKEV(input.CVEID)
	}

	if input.EPSSScore == 0 {
		epssData := tm.GetEPSS(input.CVEID)
		if epssData != nil {
			input.EPSSScore = epssData.Score
			input.EPSSPercentile = epssData.Percentile
		}
	}

	// 补充 KEV 详情
	if input.IsInKEV {
		kevEntry := tm.GetKEVEntry(input.CVEID)
		if kevEntry != nil {
			if kevEntry.DueDate != "" {
				dueDate, err := time.Parse("2006-01-02", kevEntry.DueDate)
				if err == nil {
					input.KEVDueDate = &dueDate
				}
			}
			input.IsRansomware = kevEntry.KnownRansomwareCampaignUse == "Known"
		}
	}

	return tm.levCalc.Calculate(input)
}

// CalculateLEVBatch 批量计算 LEV 评分.
func (tm *ThreatManager) CalculateLEVBatch(inputs []LEVInput) []*LEVScore {
	// 收集需要获取 EPSS 的 CVE
	cveIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if input.EPSSScore == 0 && !tm.config.EPSS.OfflineMode {
			cveIDs = append(cveIDs, input.CVEID)
		}
	}

	// 批量获取 EPSS 数据
	if len(cveIDs) > 0 && !tm.config.EPSS.OfflineMode {
		_, _ = tm.FetchEPSS(tm.ctx, cveIDs)
	}

	// 计算每个评分
	scores := make([]*LEVScore, len(inputs))
	for i, input := range inputs {
		scores[i] = tm.CalculateLEV(input)
	}

	return scores
}

// ========== 漏洞评估 ==========

// VulnerabilityData 漏洞数据.
type VulnerabilityData struct {
	CVEID             string    `json:"cve_id"`
	CVSSScore         float64   `json:"cvss_score"`
	PublishedDate     time.Time `json:"published_date"`
	AssetCriticality  float64   `json:"asset_criticality"`
	ExposureLevel     float64   `json:"exposure_level"`
	NetworkAccessible bool      `json:"network_accessible"`
}

// AssessVulnerability 评估单个漏洞.
func (tm *ThreatManager) AssessVulnerability(data VulnerabilityData) *LEVScore {
	input := LEVInput{
		CVEID:             data.CVEID,
		CVSSScore:         data.CVSSScore,
		VulnerabilityAge:  int(time.Since(data.PublishedDate).Hours() / 24),
		AssetCriticality:  data.AssetCriticality,
		ExposureLevel:     data.ExposureLevel,
		NetworkAccessible: data.NetworkAccessible,
	}

	return tm.CalculateLEV(input)
}

// AssessVulnerabilities 批量评估漏洞.
func (tm *ThreatManager) AssessVulnerabilities(data []VulnerabilityData) []*LEVScore {
	inputs := make([]LEVInput, len(data))
	for i, d := range data {
		inputs[i] = LEVInput{
			CVEID:             d.CVEID,
			CVSSScore:         d.CVSSScore,
			VulnerabilityAge:  int(time.Since(d.PublishedDate).Hours() / 24),
			AssetCriticality:  d.AssetCriticality,
			ExposureLevel:     d.ExposureLevel,
			NetworkAccessible: d.NetworkAccessible,
		}
	}

	return tm.CalculateLEVBatch(inputs)
}

// GenerateLEVReport 生成 LEV 报告.
func (tm *ThreatManager) GenerateLEVReport(data []VulnerabilityData) *LEVReport {
	scores := tm.AssessVulnerabilities(data)
	return tm.levCalc.GenerateReport(scores)
}

// ========== 状态和统计 ==========

// GetStatus 获取系统状态.
func (tm *ThreatManager) GetStatus() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return map[string]interface{}{
		"enabled":    tm.config.Enabled,
		"kev_info":   tm.kevDB.GetCatalogInfo(),
		"epss_stats": tm.epssDB.GetStatistics(),
	}
}

// GetStatistics 获取统计信息.
func (tm *ThreatManager) GetStatistics() *ThreatStatistics {
	return &ThreatStatistics{
		KEVTotal:      len(tm.kevDB.GetAllEntries()),
		KEVRansomware: len(tm.kevDB.GetRansomwareRelated()),
		KEVOverdue:    len(tm.kevDB.GetOverdue()),
		EPSSCached:    len(tm.epssDB.cache),
	}
}

// ThreatStatistics 威胁统计.
type ThreatStatistics struct {
	KEVTotal      int `json:"kev_total"`
	KEVRansomware int `json:"kev_ransomware"`
	KEVOverdue    int `json:"kev_overdue"`
	EPSSCached    int `json:"epss_cached"`
}

// ========== 预定义的漏洞输入构建器 ==========

// NewLEVInput 创建 LEV 输入.
func NewLEVInput(cveID string, cvssScore float64) LEVInput {
	return LEVInput{
		CVEID:             cveID,
		CVSSScore:         cvssScore,
		AssetCriticality:  1.0, // 默认高关键性
		ExposureLevel:     0.8, // 默认高暴露
		NetworkAccessible: true,
	}
}

// WithKEV 设置 KEV 信息.
func (input LEVInput) WithKEV(isInKEV bool, dueDate *time.Time, isRansomware bool) LEVInput {
	input.IsInKEV = isInKEV
	input.KEVDueDate = dueDate
	input.IsRansomware = isRansomware
	return input
}

// WithEPSS 设置 EPSS 信息.
func (input LEVInput) WithEPSS(score, percentile float64) LEVInput {
	input.EPSSScore = score
	input.EPSSPercentile = percentile
	return input
}

// WithAsset 设置资产信息.
func (input LEVInput) WithAsset(criticality, exposure float64, networkAccessible bool) LEVInput {
	input.AssetCriticality = criticality
	input.ExposureLevel = exposure
	input.NetworkAccessible = networkAccessible
	return input
}

// WithAge 设置漏洞年龄.
func (input LEVInput) WithAge(ageDays int) LEVInput {
	input.VulnerabilityAge = ageDays
	return input
}
