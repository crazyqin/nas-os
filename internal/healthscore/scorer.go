package healthscore

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scorer 健康评分引擎.
type Scorer struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	threshold   float64          // 告警阈值
	lastScore   *OverallScore    // 最近一次评分
	lastDetails *ScoreDetails    // 最近一次详情
	history     []ScoreRecord    // 历史记录
	alerts      []Alert          // 告警记录
	maxHistory  int              // 最大历史条数
}

// NewScorer 创建评分引擎.
func NewScorer(logger *zap.Logger) *Scorer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scorer{
		logger:     logger,
		threshold:  50,
		maxHistory: 1000,
	}
}

// SetThreshold 设置告警阈值.
func (s *Scorer) SetThreshold(threshold float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threshold = threshold
}

// Calculate 执行健康评分.
func (s *Scorer) Calculate(req CalculateRequest) (*OverallScore, error) {
	if req.Threshold > 0 {
		s.SetThreshold(req.Threshold)
	}

	s.mu.RLock()
	threshold := s.threshold
	s.mu.RUnlock()

	// 计算各分项评分
	categories := s.evaluateCategories()

	// 加权计算综合评分
	overall := s.calcOverall(categories)

	level := ClassifyLevel(overall)

	score := &OverallScore{
		Score:       roundTo2(overall),
		Level:       level,
		Categories:  categories,
		EvaluatedAt: time.Now(),
	}

	// 检查是否需要告警
	if overall < threshold {
		alert := Alert{
			Timestamp: time.Now(),
			Score:     overall,
			Threshold: threshold,
			Message:   fmt.Sprintf("存储健康评分 %.1f 低于阈值 %.1f，等级: %s", overall, threshold, level),
		}
		score.Alerts = []Alert{alert}

		s.mu.Lock()
		s.alerts = append(s.alerts, alert)
		s.mu.Unlock()

		s.logger.Warn("健康评分告警",
			zap.Float64("score", overall),
			zap.Float64("threshold", threshold),
			zap.String("level", string(level)),
		)
	}

	// 记录历史
	record := ScoreRecord{
		Timestamp:  time.Now(),
		Overall:    score.Score,
		Level:      level,
		Categories: categories,
	}

	s.mu.Lock()
	s.lastScore = score
	s.history = append(s.history, record)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.mu.Unlock()

	s.logger.Info("健康评分完成",
		zap.Float64("overall", overall),
		zap.String("level", string(level)),
	)

	return score, nil
}

// GetLastScore 获取最近一次评分.
func (s *Scorer) GetLastScore() (*OverallScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastScore == nil {
		return nil, ErrNoScoreData
	}
	return s.lastScore, nil
}

// GetDetails 获取评分详情.
func (s *Scorer) GetDetails() (*ScoreDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastScore == nil {
		return nil, ErrNoScoreData
	}

	details := &ScoreDetails{
		Overall:       *s.lastScore,
		SmartStatus:   s.buildSmartStatus(),
		RaidStatus:    s.buildRaidStatus(),
		Fragmentation: s.buildFragmentationInfo(),
		BackupInfo:    s.buildBackupInfo(),
		DataAgeInfo:   s.buildDataAgeInfo(),
	}
	return details, nil
}

// GetHistory 获取历史评分.
func (s *Scorer) GetHistory(query HistoryQuery) *HistoryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]ScoreRecord, 0)
	cutoff := time.Now().AddDate(0, 0, -query.Days)
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	for i := len(s.history) - 1; i >= 0; i-- {
		r := s.history[i]
		if query.Days > 0 && r.Timestamp.Before(cutoff) {
			break
		}
		records = append(records, r)
		if len(records) >= limit {
			break
		}
	}

	// 反转为时间正序
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	resp := &HistoryResponse{
		Records:    records,
		TotalCount: len(records),
	}

	if len(records) == 0 {
		return resp
	}

	// 统计
	var sum, minS, maxS float64
	minS = math.MaxFloat64
	for _, r := range records {
		sum += r.Overall
		if r.Overall < minS {
			minS = r.Overall
		}
		if r.Overall > maxS {
			maxS = r.Overall
		}
	}
	resp.AvgScore = roundTo2(sum / float64(len(records)))
	resp.MinScore = minS
	resp.MaxScore = maxS

	// 趋势判断
	if len(records) >= 2 {
		first := records[0].Overall
		last := records[len(records)-1].Overall
		diff := last - first
		if diff > 5 {
			resp.Trend = "rising"
		} else if diff < -5 {
			resp.Trend = "falling"
		} else {
			resp.Trend = "stable"
		}
	} else {
		resp.Trend = "stable"
	}

	return resp
}

// GetRecommendations 获取健康建议.
func (s *Scorer) GetRecommendations() (*RecommendationsResponse, error) {
	s.mu.RLock()
	score := s.lastScore
	s.mu.RUnlock()

	if score == nil {
		return nil, ErrNoScoreData
	}

	recs := s.generateRecommendations(score.Categories)

	return &RecommendationsResponse{
		OverallScore:    score.Score,
		Level:           score.Level,
		Recommendations: recs,
		GeneratedAt:     time.Now(),
	}, nil
}

// ========== 内部评估方法 ==========

// evaluateCategories 评估所有分项.
func (s *Scorer) evaluateCategories() []CategoryScore {
	return []CategoryScore{
		s.evalHardwareHealth(),
		s.evalDataSafety(),
		s.evalPerformanceHealth(),
		s.evalBackupHealth(),
	}
}

// evalHardwareHealth 硬件健康（SMART、温度、RAID）.
func (s *Scorer) evalHardwareHealth() CategoryScore {
	// 模拟SMART数据：基于典型NAS场景
	smartScore := s.calcSmartScore()
	raidScore := s.calcRaidScore()

	// 硬件健康 = SMART 60% + RAID 40%
	score := smartScore*0.6 + raidScore*0.4

	return CategoryScore{
		Name:   "硬件健康",
		Score:  roundTo2(score),
		Weight: 0.3,
		Level:  string(ClassifyLevel(score)),
		Detail: fmt.Sprintf("SMART评分: %.0f, RAID评分: %.0f", smartScore, raidScore),
	}
}

// evalDataSafety 数据安全（冗余、一致性）.
func (s *Scorer) evalDataSafety() CategoryScore {
	redundancyScore := s.calcRedundancyScore()
	integrityScore := s.calcIntegrityScore()

	score := redundancyScore*0.6 + integrityScore*0.4

	return CategoryScore{
		Name:   "数据安全",
		Score:  roundTo2(score),
		Weight: 0.25,
		Level:  string(ClassifyLevel(score)),
		Detail: fmt.Sprintf("冗余评分: %.0f, 一致性评分: %.0f", redundancyScore, integrityScore),
	}
}

// evalPerformanceHealth 性能健康（碎片化、IO延迟）.
func (s *Scorer) evalPerformanceHealth() CategoryScore {
	fragScore := s.calcFragmentationScore()
	ioScore := s.calcIOScore()

	score := fragScore*0.5 + ioScore*0.5

	return CategoryScore{
		Name:   "性能健康",
		Score:  roundTo2(score),
		Weight: 0.25,
		Level:  string(ClassifyLevel(score)),
		Detail: fmt.Sprintf("碎片化评分: %.0f, IO评分: %.0f", fragScore, ioScore),
	}
}

// evalBackupHealth 备份健康.
func (s *Scorer) evalBackupHealth() CategoryScore {
	backupScore := s.calcBackupScore()
	dataAgeScore := s.calcDataAgeScore()

	score := backupScore*0.6 + dataAgeScore*0.4

	return CategoryScore{
		Name:   "备份健康",
		Score:  roundTo2(score),
		Weight: 0.2,
		Level:  string(ClassifyLevel(score)),
		Detail: fmt.Sprintf("备份评分: %.0f, 数据新鲜度: %.0f", backupScore, dataAgeScore),
	}
}

// calcOverall 加权计算综合评分.
func (s *Scorer) calcOverall(categories []CategoryScore) float64 {
	var total, weightSum float64
	for _, c := range categories {
		total += c.Score * c.Weight
		weightSum += c.Weight
	}
	if weightSum == 0 {
		return 0
	}
	return total / weightSum
}

// ========== 评分计算（模拟数据源） ==========

// calcSmartScore 计算SMART评分.
func (s *Scorer) calcSmartScore() float64 {
	// 模拟：3块磁盘，状态各异
	disks := []DiskSmartInfo{
		{Device: "/dev/sda", Status: "passed", TempC: 38, PowerOnH: 8760, Realloct: 0, Score: 95},
		{Device: "/dev/sdb", Status: "passed", TempC: 42, PowerOnH: 12000, Realloct: 2, Score: 88},
		{Device: "/dev/sdc", Status: "degraded", TempC: 51, PowerOnH: 18000, Realloct: 128, Score: 55},
	}

	var total float64
	for _, d := range disks {
		total += d.Score
	}
	return total / float64(len(disks))
}

// calcRaidScore 计算RAID评分.
func (s *Scorer) calcRaidScore() float64 {
	// 模拟RAID5，3盘中1盘degraded
	state := "degraded"
	switch state {
	case "clean":
		return 95
	case "degraded":
		return 60
	case "failed":
		return 10
	default:
		return 50
	}
}

// calcRedundancyScore 计算冗余评分.
func (s *Scorer) calcRedundancyScore() float64 {
	// 基于RAID冗余度模拟
	redundancy := 0.67 // RAID5 3盘冗余度
	return redundancy * 100
}

// calcIntegrityScore 计算数据一致性评分.
func (s *Scorer) calcIntegrityScore() float64 {
	// 模拟：基于scrub状态
	return 85.0
}

// calcFragmentationScore 计算碎片化评分.
func (s *Scorer) calcFragmentationScore() float64 {
	// 模拟碎片率
	fragmentRatio := 0.12
	return math.Max(0, 100-fragmentRatio*500)
}

// calcIOScore 计算IO性能评分.
func (s *Scorer) calcIOScore() float64 {
	// 模拟IO延迟评分
	return 78.0
}

// calcBackupScore 计算备份评分.
func (s *Scorer) calcBackupScore() float64 {
	// 模拟：3天前有备份，覆盖率75%
	daysSince := 3
	coverage := 0.75

	ageScore := math.Max(0, 100-float64(daysSince)*5)
	coverageScore := coverage * 100
	return (ageScore + coverageScore) / 2
}

// calcDataAgeScore 计算数据新鲜度评分.
func (s *Scorer) calcDataAgeScore() float64 {
	// 模拟：15%数据超过1年未访问
	staleRatio := 0.15
	return math.Max(0, 100-staleRatio*200)
}

// ========== 详情构建 ==========

// buildSmartStatus 构建SMART状态信息.
func (s *Scorer) buildSmartStatus() SmartStatusInfo {
	disks := []DiskSmartInfo{
		{Device: "/dev/sda", Status: "passed", TempC: 38, PowerOnH: 8760, Realloct: 0, Score: 95},
		{Device: "/dev/sdb", Status: "passed", TempC: 42, PowerOnH: 12000, Realloct: 2, Score: 88},
		{Device: "/dev/sdc", Status: "degraded", TempC: 51, PowerOnH: 18000, Realloct: 128, Score: 55},
	}

	healthy, degraded, failed := 0, 0, 0
	for _, d := range disks {
		switch d.Status {
		case "passed":
			healthy++
		case "degraded":
			degraded++
		case "failed":
			failed++
		}
	}

	return SmartStatusInfo{
		TotalDisks:    len(disks),
		HealthyDisks:  healthy,
		DegradedDisks: degraded,
		FailedDisks:   failed,
		Disks:         disks,
	}
}

// buildRaidStatus 构建RAID状态.
func (s *Scorer) buildRaidStatus() RaidStatusInfo {
	return RaidStatusInfo{
		Level:       "RAID5",
		State:       "degraded",
		ActiveDisks: 2,
		TotalDisks:  3,
		Redundancy:  0.67,
		Score:       60,
	}
}

// buildFragmentationInfo 构建碎片化信息.
func (s *Scorer) buildFragmentationInfo() FragmentationInfo {
	return FragmentationInfo{
		TotalFragments: 1247,
		FragmentRatio:  0.12,
		Score:          s.calcFragmentationScore(),
	}
}

// buildBackupInfo 构建备份信息.
func (s *Scorer) buildBackupInfo() BackupInfo {
	lastBackup := time.Now().Add(-3 * 24 * time.Hour)
	return BackupInfo{
		LastBackupTime:  &lastBackup,
		BackupCount:     5,
		Coverage:        0.75,
		DaysSinceBackup: 3,
		Score:           s.calcBackupScore(),
	}
}

// buildDataAgeInfo 构建数据老化信息.
func (s *Scorer) buildDataAgeInfo() DataAgeInfo {
	return DataAgeInfo{
		OldestFileAge:  "3年2月",
		StaleDataRatio: 0.15,
		TotalFiles:     128500,
		StaleFiles:     19275,
		Score:          s.calcDataAgeScore(),
	}
}

// ========== 建议生成 ==========

// generateRecommendations 根据分项评分生成建议.
func (s *Scorer) generateRecommendations(categories []CategoryScore) []Recommendation {
	var recs []Recommendation

	for _, cat := range categories {
		catRecs := s.recsForCategory(cat)
		recs = append(recs, catRecs...)
	}

	// 按严重程度排序: high > medium > low
	sort.Slice(recs, func(i, j int) bool {
		return severityWeight(recs[i].Severity) > severityWeight(recs[j].Severity)
	})

	return recs
}

// recsForCategory 为单个类别生成建议.
func (s *Scorer) recsForCategory(cat CategoryScore) []Recommendation {
	var recs []Recommendation

	switch cat.Name {
	case "硬件健康":
		if cat.Score < 70 {
			recs = append(recs, Recommendation{
				Category:    "硬件健康",
				Severity:    "high",
				Title:       "RAID阵列降级",
				Description: "RAID阵列当前处于degraded状态，存在数据丢失风险",
				Action:      "立即检查并更换故障磁盘，重建RAID阵列",
			})
		}
		if cat.Score < 85 {
			recs = append(recs, Recommendation{
				Category:    "硬件健康",
				Severity:    "medium",
				Title:       "磁盘SMART预警",
				Description: "部分磁盘SMART指标异常，重分配扇区数量偏高",
				Action:      "密切监控磁盘状态，准备替换老化磁盘",
			})
		}

	case "数据安全":
		if cat.Score < 60 {
			recs = append(recs, Recommendation{
				Category:    "数据安全",
				Severity:    "high",
				Title:       "数据冗余不足",
				Description: "当前RAID配置冗余度较低，无法承受多盘同时故障",
				Action:      "考虑升级RAID级别或增加热备盘",
			})
		}

	case "性能健康":
		if cat.Score < 70 {
			recs = append(recs, Recommendation{
				Category:    "性能健康",
				Severity:    "medium",
				Title:       "存储碎片化严重",
				Description: "文件碎片化率较高，影响读写性能",
				Action:      "执行存储碎片整理，或迁移数据到新卷",
			})
		}

	case "备份健康":
		if cat.Score < 60 {
			recs = append(recs, Recommendation{
				Category:    "备份健康",
				Severity:    "high",
				Title:       "备份策略不完善",
				Description: "备份覆盖率偏低，部分数据缺乏保护",
				Action:      "完善自动备份策略，确保关键数据有多份副本",
			})
		}
		if cat.Score < 80 {
			recs = append(recs, Recommendation{
				Category:    "备份健康",
				Severity:    "low",
				Title:       "备份频率可优化",
				Description: "最近一次备份距今较久，建议增加备份频率",
				Action:      "设置每日增量备份，每周全量备份",
			})
		}
	}

	return recs
}

// severityWeight 严重程度权重.
func severityWeight(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// roundTo2 保留两位小数.
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
