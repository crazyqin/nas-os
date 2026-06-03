package adaptivetier

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Tier 存储层级
type Tier string

const (
	TierHot    Tier = "HOT"    // SSD/NVMe
	TierWarm   Tier = "WARM"   // HDD
	TierCold   Tier = "COLD"   // 归档/云
	TierFrozen Tier = "FROZEN" // 离线/磁带
)

// AccessPattern 访问模式
type AccessPattern struct {
	LastAccess    time.Time `json:"last_access"`
	AccessCount   int       `json:"access_count"`
	AvgInterval   float64   `json:"avg_interval_hours"`
	RecentAccess  int       `json:"recent_access_7d"`
	AccessFreq    float64   `json:"access_freq_per_day"`
}

// TierRule 分层规则
type TierRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	FromTier     Tier          `json:"from_tier"`
	ToTier       Tier          `json:"to_tier"`
	IdleDays     int           `json:"idle_days"`
	MinFileSize  int64         `json:"min_file_size"`
	MaxFileSize  int64         `json:"max_file_size"`
	Extensions   []string      `json:"extensions"`
	Enabled      bool          `json:"enabled"`
	Priority     int           `json:"priority"`
}

// FileTierInfo 文件分层信息
type FileTierInfo struct {
	Path      string        `json:"path"`
	Current   Tier          `json:"current_tier"`
	Size      int64         `json:"size"`
	Pattern   AccessPattern `json:"access_pattern"`
	MigratedAt *time.Time   `json:"migrated_at,omitempty"`
	NextCheck  time.Time    `json:"next_check"`
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID        string    `json:"id"`
	Files     []string  `json:"files"`
	FromTier  Tier      `json:"from_tier"`
	ToTier    Tier      `json:"to_tier"`
	Status    string    `json:"status"`
	Progress  float64   `json:"progress"`
	StartedAt time.Time `json:"started_at"`
	Error     string    `json:"error,omitempty"`
}

// TierStats 分层统计
type TierStats struct {
	HotSize     int64   `json:"hot_size_bytes"`
	WarmSize    int64   `json:"warm_size_bytes"`
	ColdSize    int64   `json:"cold_size_bytes"`
	FrozenSize  int64   `json:"frozen_size_bytes"`
	TotalFiles  int     `json:"total_files"`
	SavingsRate float64 `json:"savings_rate"`
}

// AdaptiveTierEngine 自适应分层引擎
type AdaptiveTierEngine struct {
	rules    []*TierRule
	files    map[string]*FileTierInfo
	jobs     []*MigrationJob
	dataPath string
	mu       sync.RWMutex
}

// NewAdaptiveTierEngine 创建分层引擎
func NewAdaptiveTierEngine(dataPath string) *AdaptiveTierEngine {
	os.MkdirAll(dataPath, 0755)
	e := &AdaptiveTierEngine{
		files:    make(map[string]*FileTierInfo),
		dataPath: dataPath,
	}
	e.loadState()
	e.initDefaultRules()
	return e
}

// AddRule 添加规则
func (e *AdaptiveTierEngine) AddRule(rule *TierRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
	e.saveState()
}

// RemoveRule 移除规则
func (e *AdaptiveTierEngine) RemoveRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			break
		}
	}
	e.saveState()
}

// RegisterFile 注册文件
func (e *AdaptiveTierEngine) RegisterFile(path string, tier Tier, size int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.files[path] = &FileTierInfo{
		Path:      path,
		Current:   tier,
		Size:      size,
		NextCheck: time.Now().Add(24 * time.Hour),
	}
}

// RecordAccess 记录访问
func (e *AdaptiveTierEngine) RecordAccess(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := e.files[path]
	if !ok {
		return
	}
	now := time.Now()
	if !f.Pattern.LastAccess.IsZero() {
		hours := now.Sub(f.Pattern.LastAccess).Hours()
		f.Pattern.AvgInterval = (f.Pattern.AvgInterval*float64(f.Pattern.AccessCount) + hours) / float64(f.Pattern.AccessCount+1)
	}
	f.Pattern.LastAccess = now
	f.Pattern.AccessCount++
	f.Pattern.RecentAccess++
	f.Pattern.AccessFreq = float64(f.Pattern.AccessCount) / max(1, now.Sub(f.Pattern.LastAccess).Hours()/24)
}

// Evaluate 执行分层评估
func (e *AdaptiveTierEngine) Evaluate() []*MigrationJob {
	e.mu.Lock()
	defer e.mu.Unlock()
	var jobs []*MigrationJob
	for _, file := range e.files {
		if time.Now().Before(file.NextCheck) {
			continue
		}
		for _, rule := range e.rules {
			if !rule.Enabled {
				continue
			}
			if file.Current != rule.FromTier {
				continue
			}
			if !e.matchesRule(file, rule) {
				continue
			}
			job := &MigrationJob{
				ID:        fmt.Sprintf("mig-%d", time.Now().UnixNano()),
				Files:     []string{file.Path},
				FromTier:  rule.FromTier,
				ToTier:    rule.ToTier,
				Status:    "PENDING",
				StartedAt: time.Now(),
			}
			jobs = append(jobs, job)
			e.jobs = append(e.jobs, job)
			file.Current = rule.ToTier
			now := time.Now()
			file.MigratedAt = &now
		}
		file.NextCheck = time.Now().Add(24 * time.Hour)
	}
	e.saveState()
	return jobs
}

// GetStats 获取统计
func (e *AdaptiveTierEngine) GetStats() TierStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats := TierStats{}
	for _, f := range e.files {
		stats.TotalFiles++
		switch f.Current {
		case TierHot:
			stats.HotSize += f.Size
		case TierWarm:
			stats.WarmSize += f.Size
		case TierCold:
			stats.ColdSize += f.Size
		case TierFrozen:
			stats.FrozenSize += f.Size
		}
	}
	total := stats.HotSize + stats.WarmSize + stats.ColdSize + stats.FrozenSize
	if total > 0 {
		stats.SavingsRate = float64(stats.ColdSize+stats.FrozenSize) / float64(total) * 100
	}
	return stats
}

// GetFiles 获取文件列表
func (e *AdaptiveTierEngine) GetFiles(tier *Tier) []*FileTierInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*FileTierInfo
	for _, f := range e.files {
		if tier != nil && f.Current != *tier {
			continue
		}
		result = append(result, f)
	}
	return result
}

// GetRules 获取规则列表
func (e *AdaptiveTierEngine) GetRules() []*TierRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rules
}

func (e *AdaptiveTierEngine) matchesRule(file *FileTierInfo, rule *TierRule) bool {
	idleDays := int(time.Since(file.Pattern.LastAccess).Hours() / 24)
	if idleDays < rule.IdleDays {
		return false
	}
	if rule.MinFileSize > 0 && file.Size < rule.MinFileSize {
		return false
	}
	if rule.MaxFileSize > 0 && file.Size > rule.MaxFileSize {
		return false
	}
	return true
}

func (e *AdaptiveTierEngine) initDefaultRules() {
	e.rules = []*TierRule{
		{ID: "hot-to-warm", Name: "热→温(30天未访问)", FromTier: TierHot, ToTier: TierWarm, IdleDays: 30, Enabled: true, Priority: 1},
		{ID: "warm-to-cold", Name: "温→冷(90天未访问)", FromTier: TierWarm, ToTier: TierCold, IdleDays: 90, Enabled: true, Priority: 2},
		{ID: "cold-to-frozen", Name: "冷→冻(365天未访问)", FromTier: TierCold, ToTier: TierFrozen, IdleDays: 365, Enabled: true, Priority: 3},
	}
}

func (e *AdaptiveTierEngine) saveState() {
	state := struct {
		Rules []*TierRule              `json:"rules"`
		Files map[string]*FileTierInfo `json:"files"`
	}{e.rules, e.files}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(e.dataPath+"/state.json", data, 0644)
}

func (e *AdaptiveTierEngine) loadState() {
	data, err := os.ReadFile(e.dataPath + "/state.json")
	if err != nil {
		return
	}
	var state struct {
		Rules []*TierRule              `json:"rules"`
		Files map[string]*FileTierInfo `json:"files"`
	}
	json.Unmarshal(data, &state)
	if state.Rules != nil {
		e.rules = state.Rules
	}
	if state.Files != nil {
		e.files = state.Files
	}
}
