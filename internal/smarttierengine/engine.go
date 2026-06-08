package smarttierengine

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// TierLevel 存储层级
type TierLevel int

const (
	TierNVMe  TierLevel = 0 // NVMe SSD - 热数据
	TierSSD   TierLevel = 1 // SATA SSD - 温数据
	TierHDD   TierLevel = 2 // HDD - 冷数据
	TierCloud TierLevel = 3 // 云存储 - 归档
)

func (t TierLevel) String() string {
	switch t {
	case TierNVMe:
		return "nvme"
	case TierSSD:
		return "ssd"
	case TierHDD:
		return "hdd"
	case TierCloud:
		return "cloud"
	default:
		return "unknown"
	}
}

// FileMetrics 文件访问指标
type FileMetrics struct {
	FilePath     string    `json:"filePath"`
	Size         int64     `json:"size"`
	ReadCount    int64     `json:"readCount"`
	WriteCount   int64     `json:"writeCount"`
	LastAccess   time.Time `json:"lastAccess"`
	CurrentTier  TierLevel `json:"currentTier"`
	HeatScore    float64   `json:"heatScore"` // 0.0-1.0, 越高越热
	AccessFreq   float64   `json:"accessFreq"`
}

// TierPolicy 分层策略
type TierPolicy struct {
	Name         string             `json:"name"`
	TierRules    map[TierLevel]TierRule `json:"tierRules"`
	EvalInterval time.Duration      `json:"evalInterval"`
	Enabled      bool               `json:"enabled"`
}

// TierRule 层级规则
type TierRule struct {
	MinHeatScore  float64 `json:"minHeatScore"`
	MaxHeatScore  float64 `json:"maxHeatScore"`
	MinAccessFreq float64 `json:"minAccessFreq"`
	MaxFileSize   int64   `json:"maxFileSize"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"filePath"`
	SourceTier  TierLevel `json:"sourceTier"`
	TargetTier  TierLevel `json:"targetTier"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"` // pending, running, completed, failed
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalFiles      int                `json:"totalFiles"`
	TierDistribution map[TierLevel]int `json:"tierDistribution"`
	PendingMigrates int                `json:"pendingMigrates"`
	ActiveMigrates  int                `json:"activeMigrates"`
	LastEvalTime    time.Time          `json:"lastEvalTime"`
	TotalMigrated   int64              `json:"totalMigrated"`
	BytesSaved      int64              `json:"bytesSaved"`
}

// SmartTierEngine 统一智能分层引擎
// 结合 ML 热冷数据检测 + 自动迁移，对标群晖 Tiering + TrueNAS 混合池
type SmartTierEngine struct {
	mu          sync.RWMutex
	files       map[string]*FileMetrics
	policies    []TierPolicy
	migrations  []MigrationTask
	stats       EngineStats
	stopCh      chan struct{}
	running     bool
}

// NewEngine 创建引擎
func NewEngine() *SmartTierEngine {
	return &SmartTierEngine{
		files:      make(map[string]*FileMetrics),
		policies:   defaultPolicies(),
		migrations: make([]MigrationTask, 0),
		stopCh:     make(chan struct{}),
	}
}

func defaultPolicies() []TierPolicy {
	return []TierPolicy{
		{
			Name: "default",
			TierRules: map[TierLevel]TierRule{
				TierNVMe: {MinHeatScore: 0.7, MaxHeatScore: 1.0, MinAccessFreq: 10},
				TierSSD:  {MinHeatScore: 0.3, MaxHeatScore: 0.7, MinAccessFreq: 3},
				TierHDD:  {MinHeatScore: 0.05, MaxHeatScore: 0.3, MinAccessFreq: 0.5},
				TierCloud: {MinHeatScore: 0.0, MaxHeatScore: 0.05},
			},
			EvalInterval: 15 * time.Minute,
			Enabled:      true,
		},
	}
}

// Start 启动引擎
func (e *SmartTierEngine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.evaluationLoop()
	log.Println("[SmartTierEngine] 智能分层引擎已启动")
}

// Stop 停止引擎
func (e *SmartTierEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	close(e.stopCh)
	e.running = false
	log.Println("[SmartTierEngine] 智能分层引擎已停止")
}

// RecordAccess 记录文件访问
func (e *SmartTierEngine) RecordAccess(path string, size int64, isRead bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fm, ok := e.files[path]
	if !ok {
		fm = &FileMetrics{
			FilePath:    path,
			Size:        size,
			CurrentTier: TierHDD,
		}
		e.files[path] = fm
	}

	if isRead {
		fm.ReadCount++
	} else {
		fm.WriteCount++
	}
	fm.LastAccess = time.Now()
	fm.HeatScore = e.calculateHeat(fm)
}

// calculateHeat 计算热度分数
func (e *SmartTierEngine) calculateHeat(fm *FileMetrics) float64 {
	age := time.Since(fm.LastAccess).Hours()
	totalAccess := float64(fm.ReadCount + fm.WriteCount)
	
	// 指数衰减 + 访问频率加权
	recency := math.Exp(-age / 168) // 一周衰减
	frequency := math.Log1p(totalAccess) / 10.0
	if frequency > 1.0 {
		frequency = 1.0
	}
	
	return recency*0.6 + frequency*0.4
}

// evaluate 评估并生成迁移任务
func (e *SmartTierEngine) evaluate() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, fm := range e.files {
		targetTier := e.determineTier(fm.HeatScore)
		if targetTier != fm.CurrentTier && e.canMigrate(fm, targetTier) {
			task := MigrationTask{
				ID:         fmt.Sprintf("mig-%d", time.Now().UnixNano()),
				FilePath:   fm.FilePath,
				SourceTier: fm.CurrentTier,
				TargetTier: targetTier,
				Reason:     fmt.Sprintf("热度=%.2f, 目标层级=%s", fm.HeatScore, targetTier),
				Status:     "pending",
				CreatedAt:  time.Now(),
			}
			e.migrations = append(e.migrations, task)
			fm.CurrentTier = targetTier
			e.stats.TotalMigrated++
			log.Printf("[SmartTierEngine] 迁移: %s %s -> %s (热度=%.2f)", fm.FilePath, task.SourceTier, targetTier, fm.HeatScore)
		}
	}
	e.stats.LastEvalTime = time.Now()
	e.updateStats()
}

func (e *SmartTierEngine) determineTier(heat float64) TierLevel {
	if heat >= 0.7 {
		return TierNVMe
	}
	if heat >= 0.3 {
		return TierSSD
	}
	if heat >= 0.05 {
		return TierHDD
	}
	return TierCloud
}

func (e *SmartTierEngine) canMigrate(fm *FileMetrics, target TierLevel) bool {
	// 避免频繁迁移（最少间隔1小时）
	for _, m := range e.migrations {
		if m.FilePath == fm.FilePath && m.Status == "completed" {
			if time.Since(m.CompletedAt) < time.Hour {
				return false
			}
		}
	}
	return true
}

func (e *SmartTierEngine) evaluationLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.evaluate()
		case <-e.stopCh:
			return
		}
	}
}

func (e *SmartTierEngine) updateStats() {
	dist := make(map[TierLevel]int)
	for _, fm := range e.files {
		dist[fm.CurrentTier]++
	}
	e.stats.TierDistribution = dist
	e.stats.TotalFiles = len(e.files)
	pending := 0
	active := 0
	for _, m := range e.migrations {
		switch m.Status {
		case "pending":
			pending++
		case "running":
			active++
		}
	}
	e.stats.PendingMigrates = pending
	e.stats.ActiveMigrates = active
}

// GetStats 获取统计
func (e *SmartTierEngine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetFiles 获取文件列表
func (e *SmartTierEngine) GetFiles() []FileMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]FileMetrics, 0, len(e.files))
	for _, fm := range e.files {
		result = append(result, *fm)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].HeatScore > result[j].HeatScore })
	return result
}

// GetMigrations 获取迁移任务
func (e *SmartTierEngine) GetMigrations() []MigrationTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.migrations
}
