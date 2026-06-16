// Package smartdiskai - 数据迁移建议与维护建议引擎
package smartdiskai

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// MigrationAdvisor - 数据迁移建议引擎
// ============================================================

// MigrationAdvisor 数据迁移建议引擎
type MigrationAdvisor struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	collector *SMARTCollector
	scorer    *HealthScorer
	predictor *FailurePredictor

	// 可用目标设备池
	targetDevices []TargetDevice
}

// TargetDevice 目标设备信息
type TargetDevice struct {
	Device       string `json:"device"`
	Model        string `json:"model"`
	Capacity     uint64 `json:"capacity_bytes"`
	IsSSD        bool   `json:"is_ssd"`
	HealthScore  float64 `json:"health_score"`
	Available    bool   `json:"available"`
}

// NewMigrationAdvisor 创建迁移建议引擎
func NewMigrationAdvisor(logger *zap.Logger, collector *SMARTCollector, scorer *HealthScorer, predictor *FailurePredictor) *MigrationAdvisor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MigrationAdvisor{
		logger:        logger,
		collector:     collector,
		scorer:        scorer,
		predictor:     predictor,
		targetDevices: make([]TargetDevice, 0),
	}
}

// RegisterTargetDevice 注册可用目标设备
func (m *MigrationAdvisor) RegisterTargetDevice(device TargetDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targetDevices = append(m.targetDevices, device)
}

// GetRecommendations 获取所有设备的迁移建议
func (m *MigrationAdvisor) GetRecommendations() ([]*MigrationRecommendation, error) {
	devices := m.collector.GetDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("无磁盘数据")
	}

	var recommendations []*MigrationRecommendation
	for _, device := range devices {
		rec, err := m.GetRecommendation(device)
		if err != nil {
			continue
		}
		if rec != nil {
			recommendations = append(recommendations, rec)
		}
	}

	// 按紧急程度排序
	sort.Slice(recommendations, func(i, j int) bool {
		urgencyOrder := map[MigrationUrgency]int{
			MigrationImmediate: 0,
			MigrationSoon:      1,
			MigrationPlanned:   2,
			MigrationOptional:  3,
		}
		return urgencyOrder[recommendations[i].Urgency] < urgencyOrder[recommendations[j].Urgency]
	})

	return recommendations, nil
}

// GetRecommendation 获取单个设备的迁移建议
func (m *MigrationAdvisor) GetRecommendation(device string) (*MigrationRecommendation, error) {
	// 获取故障预测
	prediction, err := m.predictor.Predict(device)
	if err != nil {
		return nil, err
	}

	// 只有中高风险才建议迁移
	if prediction.RiskLevel == RiskLow {
		return nil, nil
	}

	data, err := m.collector.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	rec := &MigrationRecommendation{
		ID:            fmt.Sprintf("MIG-%s-%d", device, time.Now().Unix()),
		SourceDevice:  device,
		RiskLevel:     prediction.RiskLevel,
		EstimatedSize: data.CapacityBytes,
		CreatedAt:     time.Now(),
	}

	// 确定紧急程度
	switch prediction.RiskLevel {
	case RiskCritical:
		rec.Urgency = MigrationImmediate
		rec.Reason = fmt.Sprintf("设备故障概率 %.1f%%，预计剩余寿命 %d 天，建议立即迁移数据",
			prediction.FailureProbability*100, prediction.EstimatedLifeDays)
	case RiskHigh:
		rec.Urgency = MigrationSoon
		rec.Reason = fmt.Sprintf("设备风险较高，故障概率 %.1f%%，建议尽快迁移",
			prediction.FailureProbability*100)
	case RiskMedium:
		rec.Urgency = MigrationPlanned
		rec.Reason = "设备存在风险因素，建议计划迁移"
	default:
		rec.Urgency = MigrationOptional
		rec.Reason = "设备状态尚可，可选择性迁移"
	}

	// 估算迁移时间（假设 100MB/s 平均速度）
	transferSpeed := 100 * 1024 * 1024.0 // 100 MB/s
	estimatedSeconds := float64(rec.EstimatedSize) / transferSpeed
	if estimatedSeconds < 3600 {
		rec.EstimatedTime = fmt.Sprintf("%.0f 分钟", estimatedSeconds/60)
	} else {
		rec.EstimatedTime = fmt.Sprintf("%.1f 小时", estimatedSeconds/3600)
	}

	// 推荐文件系统
	if data.IsSSD {
		rec.RecommendedFS = "ext4"
	} else {
		rec.RecommendedFS = "ext4"
	}

	// 查找最佳目标设备
	target := m.findBestTarget(data)
	if target != nil {
		rec.TargetDevice = target.Device
	}

	// 生成迁移步骤
	rec.Steps = m.generateMigrationSteps(rec)

	m.logger.Info("生成迁移建议",
		zap.String("source", device),
		zap.String("target", rec.TargetDevice),
		zap.String("urgency", string(rec.Urgency)),
	)

	return rec, nil
}

// findBestTarget 查找最佳目标设备
func (m *MigrationAdvisor) findBestTarget(source *SMARTData) *TargetDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *TargetDevice
	bestScore := -1.0

	for i, target := range m.targetDevices {
		if !target.Available {
			continue
		}
		// 目标容量必须大于源设备
		if target.Capacity < source.CapacityBytes {
			continue
		}
		// 优先选择健康评分高的设备
		if target.HealthScore > bestScore {
			bestScore = target.HealthScore
			best = &m.targetDevices[i]
		}
	}

	return best
}

// generateMigrationSteps 生成迁移步骤
func (m *MigrationAdvisor) generateMigrationSteps(rec *MigrationRecommendation) []string {
	steps := []string{
		fmt.Sprintf("1. 备份 %s 上的关键数据到安全位置", rec.SourceDevice),
		"2. 检查目标设备健康状态，确保无错误",
		"3. 在目标设备上创建文件系统",
		"4. 使用 rsync 或 dd 进行数据迁移",
		"5. 验证迁移数据的完整性（校验和对比）",
		"6. 更新 fstab 或挂载配置",
		"7. 测试新设备读写功能",
		"8. 确认无误后，格式化源设备（可选）",
	}

	if rec.Urgency == MigrationImmediate {
		steps = append([]string{"⚠️ 紧急：建议立即停止对源设备的写入操作"}, steps...)
	}

	return steps
}

// ============================================================
// MaintenanceAdvisor - 维护建议引擎
// ============================================================

// MaintenanceAdvisor 维护建议引擎
type MaintenanceAdvisor struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	collector *SMARTCollector
	scorer    *HealthScorer
	lifecycle *LifecycleManager

	// 维护规则
	rules []MaintenanceRule
}

// MaintenanceRule 维护规则
type MaintenanceRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Check       func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice
}

// NewMaintenanceAdvisor 创建维护建议引擎
func NewMaintenanceAdvisor(logger *zap.Logger, collector *SMARTCollector, scorer *HealthScorer, lifecycle *LifecycleManager) *MaintenanceAdvisor {
	if logger == nil {
		logger = zap.NewNop()
	}
	advisor := &MaintenanceAdvisor{
		logger:    logger,
		collector: collector,
		scorer:    scorer,
		lifecycle: lifecycle,
	}
	advisor.initRules()
	return advisor
}

// initRules 初始化维护规则
func (m *MaintenanceAdvisor) initRules() {
	m.rules = []MaintenanceRule{
		{
			ID:       "MR-001",
			Name:     "温度过高检查",
			Category: "thermal",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				temp := getAttributeValue(data, SMARTIDTemperatureCelsius)
				if temp >= 60 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-001", device),
						Device:      device,
						Title:       "磁盘温度严重过高",
						Description: fmt.Sprintf("当前温度 %d℃，已超过安全阈值 60℃。高温会加速硬盘老化，增加故障风险。", temp),
						Priority:    PriorityUrgent,
						Category:    "thermal",
						Urgency:     "立即处理",
					}
				}
				if temp >= 50 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-002", device),
						Device:      device,
						Title:       "磁盘温度偏高",
						Description: fmt.Sprintf("当前温度 %d℃，接近安全阈值。建议改善散热条件。", temp),
						Priority:    PriorityHigh,
						Category:    "thermal",
						Urgency:     "尽快处理",
					}
				}
				return nil
			},
		},
		{
			ID:       "MR-002",
			Name:     "坏扇区检查",
			Category: "storage",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
				pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
				total := reallocated + pending
				if total > 50 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-003", device),
						Device:      device,
						Title:       "坏扇区数量过多",
						Description: fmt.Sprintf("重映射扇区 %d + 待映射扇区 %d = %d，已超过安全阈值。磁盘物理介质可能损坏。", reallocated, pending, total),
						Priority:    PriorityUrgent,
						Category:    "storage",
						Urgency:     "立即备份数据并更换磁盘",
					}
				}
				if total > 10 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-004", device),
						Device:      device,
						Title:       "存在坏扇区",
						Description: fmt.Sprintf("检测到 %d 个坏扇区，建议持续监控。", total),
						Priority:    PriorityHigh,
						Category:    "storage",
						Urgency:     "持续监控",
					}
				}
				return nil
			},
		},
		{
			ID:       "MR-003",
			Name:     "通电时间检查",
			Category: "lifecycle",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				poh := data.PowerOnHours
				if poh > 40000 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-005", device),
						Device:      device,
						Title:       "通电时间过长",
						Description: fmt.Sprintf("已通电 %d 小时（约 %.1f 年），超过典型使用寿命。建议考虑更换。", poh, float64(poh)/8760),
						Priority:    PriorityHigh,
						Category:    "lifecycle",
						Urgency:     "计划更换",
					}
				}
				if poh > 25000 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-006", device),
						Device:      device,
						Title:       "通电时间较长",
						Description: fmt.Sprintf("已通电 %d 小时（约 %.1f 年），建议加强监控频率。", poh, float64(poh)/8760),
						Priority:    PriorityMedium,
						Category:    "lifecycle",
						Urgency:     "定期检查",
					}
				}
				return nil
			},
		},
		{
			ID:       "MR-004",
			Name:     "SSD 磨损检查",
			Category: "wear",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				if !data.IsSSD {
					return nil
				}
				lifeLeft := getAttributeValue(data, SMARTIDSSDLifeLeft)
				if lifeLeft > 0 && lifeLeft < 10 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-007", device),
						Device:      device,
						Title:       "SSD 寿命即将耗尽",
						Description: fmt.Sprintf("SSD 剩余寿命仅 %d%%，写入量接近 TBW 上限。建议立即更换。", lifeLeft),
						Priority:    PriorityUrgent,
						Category:    "wear",
						Urgency:     "立即更换",
					}
				}
				if lifeLeft > 0 && lifeLeft < 20 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-008", device),
						Device:      device,
						Title:       "SSD 磨损严重",
						Description: fmt.Sprintf("SSD 剩余寿命 %d%%，建议计划更换。", lifeLeft),
						Priority:    PriorityHigh,
						Category:    "wear",
						Urgency:     "计划更换",
					}
				}
				return nil
			},
		},
		{
			ID:       "MR-005",
			Name:     "健康评分检查",
			Category: "health",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				if score == nil {
					return nil
				}
				if score.Score < 30 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-009", device),
						Device:      device,
						Title:       "健康评分过低",
						Description: fmt.Sprintf("综合健康评分 %.1f/100（等级：%s），磁盘状态严重。", score.Score, string(score.Grade)),
						Priority:    PriorityUrgent,
						Category:    "health",
						Urgency:     "立即检查",
					}
				}
				if score.Score < 50 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-010", device),
						Device:      device,
						Title:       "健康评分偏低",
						Description: fmt.Sprintf("综合健康评分 %.1f/100（等级：%s），建议加强监控。", score.Score, string(score.Grade)),
						Priority:    PriorityHigh,
						Category:    "health",
						Urgency:     "定期检查",
					}
				}
				return nil
			},
		},
		{
			ID:       "MR-006",
			Name:     "不安全关机检查",
			Category: "reliability",
			Check: func(device string, data *SMARTData, score *HealthScore) *MaintenanceAdvice {
				unsafeShutdowns := getAttributeValue(data, SMARTIDUnsafeShutdownCount)
				if unsafeShutdowns > 10 {
					return &MaintenanceAdvice{
						ID:          fmt.Sprintf("MA-%s-011", device),
						Device:      device,
						Title:       "不安全关机次数过多",
						Description: fmt.Sprintf("检测到 %d 次不安全关机，可能导致文件系统损坏。建议检查 UPS 和电源。", unsafeShutdowns),
						Priority:    PriorityMedium,
						Category:    "reliability",
						Urgency:     "建议配置 UPS",
					}
				}
				return nil
			},
		},
	}
}

// GetAdvices 获取所有设备的维护建议
func (m *MaintenanceAdvisor) GetAdvices() ([]*MaintenanceAdvice, error) {
	devices := m.collector.GetDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("无磁盘数据")
	}

	var advices []*MaintenanceAdvice
	for _, device := range devices {
		deviceAdvices, err := m.GetAdvicesForDevice(device)
		if err != nil {
			continue
		}
		advices = append(advices, deviceAdvices...)
	}

	// 按优先级排序
	sort.Slice(advices, func(i, j int) bool {
		priorityOrder := map[AdvicePriority]int{
			PriorityUrgent: 0,
			PriorityHigh:   1,
			PriorityMedium: 2,
			PriorityLow:    3,
			PriorityInfo:   4,
		}
		return priorityOrder[advices[i].Priority] < priorityOrder[advices[j].Priority]
	})

	return advices, nil
}

// GetAdvicesForDevice 获取单个设备的维护建议
func (m *MaintenanceAdvisor) GetAdvicesForDevice(device string) ([]*MaintenanceAdvice, error) {
	data, err := m.collector.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	score, err := m.scorer.Calculate(device)
	if err != nil {
		// 无评分时继续
		score = nil
	}

	var advices []*MaintenanceAdvice
	for _, rule := range m.rules {
		if advice := rule.Check(device, data, score); advice != nil {
			advices = append(advices, advice)
		}
	}

	m.logger.Debug("维护建议检查完成",
		zap.String("device", device),
		zap.Int("advices", len(advices)),
	)

	return advices, nil
}

// ============================================================
// DashboardBuilder - 仪表板数据构建器
// ============================================================

// DashboardBuilder 仪表板数据构建器
type DashboardBuilder struct {
	logger    *zap.Logger
	collector *SMARTCollector
	scorer    *HealthScorer
	lifecycle *LifecycleManager
	advisor   *MaintenanceAdvisor
	migrator  *MigrationAdvisor
}

// NewDashboardBuilder 创建仪表板构建器
func NewDashboardBuilder(logger *zap.Logger, collector *SMARTCollector, scorer *HealthScorer, lifecycle *LifecycleManager, advisor *MaintenanceAdvisor, migrator *MigrationAdvisor) *DashboardBuilder {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DashboardBuilder{
		logger:    logger,
		collector: collector,
		scorer:    scorer,
		lifecycle: lifecycle,
		advisor:   advisor,
		migrator:  migrator,
	}
}

// BuildDashboard 构建仪表板数据
func (d *DashboardBuilder) BuildDashboard() (*DashboardData, error) {
	devices := d.collector.GetDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("无磁盘数据")
	}

	dashboard := &DashboardData{
		TotalDisks: len(devices),
		GeneratedAt: time.Now(),
	}

	var totalScore float64
	worstScore := 100.0

	for _, device := range devices {
		score, err := d.scorer.Calculate(device)
		if err != nil {
			continue
		}

		totalScore += score.Score
		switch score.Status {
		case StatusHealthy:
			dashboard.HealthyDisks++
		case StatusWarning:
			dashboard.WarningDisks++
		case StatusCritical:
			dashboard.CriticalDisks++
		case StatusFailed:
			dashboard.FailedDisks++
		}

		if score.Score < worstScore {
			worstScore = score.Score
			dashboard.WorstDisk = device
			dashboard.WorstScore = score.Score
		}
	}

	if dashboard.TotalDisks > 0 {
		dashboard.AverageScore = totalScore / float64(dashboard.TotalDisks)
	}

	// 统计建议数
	if d.advisor != nil {
		advices, _ := d.advisor.GetAdvices()
		dashboard.AdvicesCount = len(advices)
	}

	// 统计迁移建议数
	if d.migrator != nil {
		migrations, _ := d.migrator.GetRecommendations()
		dashboard.MigrationsCount = len(migrations)
	}

	// 统计温度告警数
	for _, device := range devices {
		tempTrend, err := d.collector.AnalyzeTemperature(device)
		if err != nil {
			continue
		}
		dashboard.TempAlertsCount += len(tempTrend.Alerts)
	}

	return dashboard, nil
}

// ============================================================
// 辅助函数
// ============================================================

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// minFloat64 返回两个浮点数中的较小值
func minFloat64(a, b float64) float64 {
	return math.Min(a, b)
}
