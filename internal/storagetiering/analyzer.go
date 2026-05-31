package storagetiering

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Analyzer 访问频率分析器
// 基于访问频率、文件大小、文件类型智能判定数据温度
type Analyzer struct {
	mu     sync.RWMutex
	config AnalyzerConfig
	policy PolicyConfig
	logger *zap.Logger

	// 访问记录
	accessHistory map[string][]AccessRecord // path -> records
	files         map[string]*FileEntry     // path -> metadata

	// 统计
	lastAnalysis *time.Time
	hitCount     int64
	missCount    int64
}

// NewAnalyzer 创建访问频率分析器
func NewAnalyzer(config AnalyzerConfig, policy PolicyConfig, logger *zap.Logger) *Analyzer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Analyzer{
		config:        config,
		policy:        policy,
		logger:        logger,
		accessHistory: make(map[string][]AccessRecord),
		files:         make(map[string]*FileEntry),
	}
}

// RegisterFile 注册文件
func (a *Analyzer) RegisterFile(entry FileEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[entry.Path] = &entry
}

// RecordAccess 记录访问事件
func (a *Analyzer) RecordAccess(record AccessRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.accessHistory[record.Path] = append(a.accessHistory[record.Path], record)

	// 更新文件元数据
	entry, exists := a.files[record.Path]
	if !exists {
		entry = &FileEntry{
			Path:        record.Path,
			Size:        record.Size,
			ContentType: inferContentType(record.Path),
			CurrentTier: TierHDD,
		}
		a.files[record.Path] = entry
	}
	entry.AccessedAt = record.Timestamp
	entry.AccessCount++
	switch record.OpType {
	case "read":
		entry.ReadCount++
	case "write":
		entry.WriteCount++
		entry.ModifiedAt = record.Timestamp
	}

	// 裁剪历史
	a.trimHistory(record.Path)
}

// trimHistory 裁剪过期访问记录
func (a *Analyzer) trimHistory(path string) {
	cutoff := time.Now().AddDate(0, 0, -a.config.HistoryWindowDays)
	records := a.accessHistory[path]
	start := 0
	for i, r := range records {
		if r.Timestamp.After(cutoff) {
			start = i
			break
		}
		if i == len(records)-1 {
			start = len(records)
		}
	}
	if start > 0 {
		a.accessHistory[path] = records[start:]
	}
}

// Analyze 执行全量分析，返回需要迁移的任务列表
func (a *Analyzer) Analyze(ctx context.Context) ([]*MigrationTask, error) {
	// 先检查 context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	a.lastAnalysis = &now

	var tasks []*MigrationTask
	for path, entry := range a.files {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 计算热度评分
		heatScore := a.calculateHeatScore(entry)
		entry.HeatScore = heatScore

		// 判定温度
		temperature := a.classifyTemperature(heatScore)
		entry.Temperature = temperature

		// 确定目标层级
		targetTier := a.temperatureToTier(temperature)

		// 如果当前层级与目标不同，生成迁移任务
		if entry.CurrentTier != targetTier {
			// 大文件 (>1GB) 额外惩罚热度
			if entry.Size > 1024*1024*1024 {
				// 大文件额外惩罚热度
				adjustedScore := heatScore * (1.0 - a.policy.LargeFilePenalty)
				adjustedTemp := a.classifyTemperature(adjustedScore)
				targetTier = a.temperatureToTier(adjustedTemp)
				if entry.CurrentTier == targetTier {
					continue
				}
			}

			task := &MigrationTask{
				ID:       generateTaskID(path, entry.CurrentTier, targetTier),
				FilePath: path,
				FromTier: entry.CurrentTier,
				ToTier:   targetTier,
				FileSize: entry.Size,
				State:    StatePending,
				Reason:   a.buildReason(entry, heatScore, temperature),
			}
			tasks = append(tasks, task)

			a.logger.Info("migration candidate",
				zap.String("path", path),
				zap.Float64("heat", heatScore),
				zap.String("temperature", temperature.String()),
				zap.String("from", entry.CurrentTier.String()),
				zap.String("to", targetTier.String()))
		}
	}

	a.logger.Info("analysis completed",
		zap.Int("files", len(a.files)),
		zap.Int("migration_candidates", len(tasks)))

	return tasks, nil
}

// calculateHeatScore 计算热度评分 (0-100)
// 综合因素：访问频率、最近访问时间、读写比、文件类型
func (a *Analyzer) calculateHeatScore(entry *FileEntry) float64 {
	if entry.AccessCount == 0 {
		return 0
	}

	now := time.Now()

	// 1. 访问频率分数 (0-40)
	// 30天内访问次数 → 归一化
	freqScore := float64(entry.AccessCount) / float64(a.config.HistoryWindowDays*24) * 40
	if freqScore > 40 {
		freqScore = 40
	}

	// 2. 最近访问时间分数 (0-30)
	// 越近越高，指数衰减
	hoursSinceAccess := now.Sub(entry.AccessedAt).Hours()
	recencyScore := 30.0
	if hoursSinceAccess > 0 {
		recencyScore = 30.0 * (1.0 / (1.0 + hoursSinceAccess/24.0))
	}

	// 3. 读写活跃度 (0-15)
	var rwScore float64
	if entry.ReadCount+entry.WriteCount > 0 {
		// 读写混合操作加分
		ratio := float64(entry.ReadCount) / float64(entry.ReadCount+entry.WriteCount)
		rwScore = 15.0 * (1.0 - 2.0*abs(ratio-0.5))
	}

	// 4. 文件类型加成 (0-15)
	ext := strings.ToLower(filepath.Ext(entry.Path))
	boost := a.policy.FileTypeBoosts[ext]
	typeScore := 7.5 + boost // 中心值 7.5，加成可 ±15
	if typeScore < 0 {
		typeScore = 0
	}
	if typeScore > 15 {
		typeScore = 15
	}

	total := freqScore + recencyScore + rwScore + typeScore
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}

	return total
}

// classifyTemperature 根据热度评分判定温度
func (a *Analyzer) classifyTemperature(score float64) DataTemperature {
	if score >= a.policy.Thresholds.HotMinScore {
		return TempHot
	}
	if score >= a.policy.Thresholds.WarmMinScore {
		return TempWarm
	}
	return TempCold
}

// temperatureToTier 温度映射到存储层级
func (a *Analyzer) temperatureToTier(dt DataTemperature) Tier {
	switch dt {
	case TempHot:
		return TierSSD
	case TempWarm:
		return TierHDD
	case TempCold:
		return TierCold
	default:
		return TierHDD
	}
}

// buildReason 构建迁移原因描述
func (a *Analyzer) buildReason(entry *FileEntry, heatScore float64, temp DataTemperature) string {
	return "auto-tier: heat=" + formatFloat(heatScore, 1) +
		", temp=" + temp.String() +
		", access_count=" + formatInt(entry.AccessCount)
}

// GetFileHeat 获取文件热度
func (a *Analyzer) GetFileHeat(path string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	entry, ok := a.files[path]
	if !ok {
		return 0, false
	}
	return entry.HeatScore, true
}

// GetFileEntry 获取文件条目
func (a *Analyzer) GetFileEntry(path string) (*FileEntry, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	entry, ok := a.files[path]
	if !ok {
		return nil, false
	}
	// 返回副本
	cp := *entry
	return &cp, true
}

// RecordHit 记录命中
func (a *Analyzer) RecordHit() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hitCount++
}

// RecordMiss 记录未命中
func (a *Analyzer) RecordMiss() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.missCount++
}

// HitRate 返回命中率
func (a *Analyzer) HitRate() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	total := a.hitCount + a.missCount
	if total == 0 {
		return 0
	}
	return float64(a.hitCount) / float64(total)
}

// FileCount 返回文件数量
func (a *Analyzer) FileCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.files)
}

// LastAnalysis 返回最后分析时间
func (a *Analyzer) LastAnalysis() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastAnalysis
}

// ============================================================
// 辅助函数
// ============================================================

// inferContentType 根据扩展名推断 MIME 类型
func inferContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".log":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".zip", ".tar", ".gz", ".bz2", ".xz":
		return "application/octet-stream"
	case ".mp4", ".mkv", ".avi":
		return "video/mp4"
	case ".mp3", ".flac", ".wav":
		return "audio/mpeg"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".db", ".sqlite":
		return "application/x-sqlite3"
	default:
		return "application/octet-stream"
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func formatFloat(f float64, prec int) string {
	// 简单格式化，避免引入 fmt
	s := ""
	val := int(f * float64(pow10(prec)))
	if val < 0 {
		s = "-"
		val = -val
	}
	intPart := val / pow10(prec)
	fracPart := val % pow10(prec)
	s += formatInt(int64(intPart))
	if prec > 0 {
		s += "."
		fStr := formatInt(int64(fracPart))
		for len(fStr) < prec {
			fStr = "0" + fStr
		}
		s += fStr
	}
	return s
}

func pow10(n int) int {
	r := 1
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// generateTaskID 生成任务 ID
func generateTaskID(path string, from, to Tier) string {
	return filepath.Base(path) + ":" + from.String() + "->" + to.String() + ":" + formatInt(time.Now().UnixNano())
}
