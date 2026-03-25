// Package threat - EPSS (Exploit Prediction Scoring System) 评分系统
package threat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== EPSS 评分系统 ==========

// EPSSData EPSS 数据结构.
type EPSSData struct {
	CVEID      string    `json:"cve"`
	Score      float64   `json:"epss"`       // 0-1 概率评分
	Percentile float64   `json:"percentile"` // 0-1 百分位
	Date       time.Time `json:"date"`
}

// EPSSResponse EPSS API 响应结构.
type EPSSResponse struct {
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

// EPSSDatabase EPSS 数据库管理器.
type EPSSDatabase struct {
	cache      map[string]*EPSSData
	cachePath  string
	sourceURL  string
	httpClient *http.Client
	lastSync   time.Time
	mu         sync.RWMutex
}

// EPSSConfig EPSS 配置.
type EPSSConfig struct {
	SourceURL   string `json:"source_url"`
	CachePath   string `json:"cache_path"`
	OfflineMode bool   `json:"offline_mode"`
}

// DefaultEPSSConfig 默认 EPSS 配置.
func DefaultEPSSConfig() EPSSConfig {
	return EPSSConfig{
		SourceURL:   "https://api.first.org/data/v1/epss",
		CachePath:   "/var/lib/nas-os/threat/epss_cache.json",
		OfflineMode: false,
	}
}

// NewEPSSDatabase 创建 EPSS 数据库.
func NewEPSSDatabase(config EPSSConfig) *EPSSDatabase {
	// 确保缓存目录存在
	cacheDir := filepath.Dir(config.CachePath)
	_ = os.MkdirAll(cacheDir, 0750)

	edb := &EPSSDatabase{
		cache:     make(map[string]*EPSSData),
		cachePath: config.CachePath,
		sourceURL: config.SourceURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 加载本地缓存
	_ = edb.loadCache()

	return edb
}

// loadCache 加载本地缓存.
func (edb *EPSSDatabase) loadCache() error {
	data, err := os.ReadFile(edb.cachePath)
	if err != nil {
		return err
	}

	var cache map[string]*EPSSData
	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	edb.mu.Lock()
	edb.cache = cache
	edb.mu.Unlock()

	return nil
}

// saveCache 保存缓存.
func (edb *EPSSDatabase) saveCache() error {
	edb.mu.RLock()
	data, err := json.MarshalIndent(edb.cache, "", "  ")
	edb.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(edb.cachePath, data, 0600)
}

// FetchEPSS 批量获取 EPSS 数据.
func (edb *EPSSDatabase) FetchEPSS(ctx context.Context, cveIDs []string) (map[string]*EPSSData, error) {
	if len(cveIDs) == 0 {
		return make(map[string]*EPSSData), nil
	}

	results := make(map[string]*EPSSData)
	batchSize := 100 // EPSS API 每次最多 100 个 CVE

	for i := 0; i < len(cveIDs); i += batchSize {
		end := i + batchSize
		if end > len(cveIDs) {
			end = len(cveIDs)
		}

		batch := cveIDs[i:end]
		cveList := strings.Join(batch, ",")

		url := fmt.Sprintf("%s?cve=%s", edb.sourceURL, cveList)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "NAS-OS-ThreatSystem/1.0")

		resp, err := edb.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}

		var epssResp EPSSResponse
		if err := json.NewDecoder(resp.Body).Decode(&epssResp); err != nil {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()

		for _, item := range epssResp.Data {
			date, _ := time.Parse("2006-01-02", item.Date)
			results[item.CVE] = &EPSSData{
				CVEID:      item.CVE,
				Score:      item.EPSS,
				Percentile: item.Percentile,
				Date:       date,
			}
		}

		// 更新缓存
		edb.mu.Lock()
		for cve, data := range results {
			edb.cache[cve] = data
		}
		edb.mu.Unlock()
	}

	edb.mu.Lock()
	edb.lastSync = time.Now()
	edb.mu.Unlock()

	// 保存缓存
	_ = edb.saveCache()

	return results, nil
}

// GetEPSS 获取单个 CVE 的 EPSS 数据.
func (edb *EPSSDatabase) GetEPSS(cveID string) *EPSSData {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	return edb.cache[cveID]
}

// GetEPSSBatch 批量获取缓存中的 EPSS 数据.
func (edb *EPSSDatabase) GetEPSSBatch(cveIDs []string) map[string]*EPSSData {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	results := make(map[string]*EPSSData)
	for _, cveID := range cveIDs {
		if data, ok := edb.cache[cveID]; ok {
			results[cveID] = data
		}
	}
	return results
}

// GetHighEPSS 获取高 EPSS 评分的漏洞.
func (edb *EPSSDatabase) GetHighEPSS(threshold float64) []*EPSSData {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	var results []*EPSSData
	for _, data := range edb.cache {
		if data.Score >= threshold {
			results = append(results, data)
		}
	}
	return results
}

// GetHighPercentile 获取高百分位的漏洞.
func (edb *EPSSDatabase) GetHighPercentile(threshold float64) []*EPSSData {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	var results []*EPSSData
	for _, data := range edb.cache {
		if data.Percentile >= threshold {
			results = append(results, data)
		}
	}
	return results
}

// ========== EPSS 风险等级计算 ==========

// EPSSRiskLevel EPSS 风险等级.
type EPSSRiskLevel string

const (
	// EPSSRiskLow indicates low EPSS risk (score < 0.01).
	EPSSRiskLow EPSSRiskLevel = "low" // < 0.01
	// EPSSRiskMedium indicates medium EPSS risk (score 0.01 - 0.1).
	EPSSRiskMedium EPSSRiskLevel = "medium" // 0.01 - 0.1
	// EPSSRiskHigh indicates high EPSS risk (score 0.1 - 0.5).
	EPSSRiskHigh EPSSRiskLevel = "high" // 0.1 - 0.5
	// EPSSRiskCritical indicates critical EPSS risk (score > 0.5).
	EPSSRiskCritical EPSSRiskLevel = "critical" // > 0.5
)

// GetEPSSRiskLevel 获取 EPSS 风险等级.
func (edb *EPSSDatabase) GetEPSSRiskLevel(score float64) EPSSRiskLevel {
	switch {
	case score >= 0.5:
		return EPSSRiskCritical
	case score >= 0.1:
		return EPSSRiskHigh
	case score >= 0.01:
		return EPSSRiskMedium
	default:
		return EPSSRiskLow
	}
}

// ClassifyByRiskLevel 按风险等级分类.
func (edb *EPSSDatabase) ClassifyByRiskLevel() map[EPSSRiskLevel][]*EPSSData {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	result := map[EPSSRiskLevel][]*EPSSData{
		EPSSRiskCritical: {},
		EPSSRiskHigh:     {},
		EPSSRiskMedium:   {},
		EPSSRiskLow:      {},
	}

	for _, data := range edb.cache {
		level := edb.GetEPSSRiskLevel(data.Score)
		result[level] = append(result[level], data)
	}

	return result
}

// GetStatistics 获取 EPSS 统计信息.
func (edb *EPSSDatabase) GetStatistics() map[string]interface{} {
	edb.mu.RLock()
	defer edb.mu.RUnlock()

	if len(edb.cache) == 0 {
		return map[string]interface{}{
			"total":     0,
			"last_sync": edb.lastSync,
		}
	}

	var totalScore, totalPercentile float64
	levelCounts := map[EPSSRiskLevel]int{
		EPSSRiskCritical: 0,
		EPSSRiskHigh:     0,
		EPSSRiskMedium:   0,
		EPSSRiskLow:      0,
	}

	for _, data := range edb.cache {
		totalScore += data.Score
		totalPercentile += data.Percentile
		level := edb.GetEPSSRiskLevel(data.Score)
		levelCounts[level]++
	}

	count := len(edb.cache)

	return map[string]interface{}{
		"total":              count,
		"average_score":      totalScore / float64(count),
		"average_percentile": totalPercentile / float64(count),
		"by_risk_level":      levelCounts,
		"last_sync":          edb.lastSync,
	}
}
