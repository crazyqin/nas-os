package smartbackupadvisor

import (
	"fmt"
	"sync"
	"time"
)

// AdvisorBackupStrategy 备份策略
type AdvisorBackupStrategy string

const (
	AdvisorStrategyFull        AdvisorBackupStrategy = "full"
	AdvisorStrategyIncremental AdvisorBackupStrategy = "incremental"
	AdvisorStrategyDifferential AdvisorBackupStrategy = "differential"
	AdvisorStrategyMirror      AdvisorBackupStrategy = "mirror"
)

// AdvisorRiskLevel 风险等级
type AdvisorRiskLevel string

const (
	AdvisorRiskLow    AdvisorRiskLevel = "low"
	AdvisorRiskMedium AdvisorRiskLevel = "medium"
	AdvisorRiskHigh   AdvisorRiskLevel = "high"
	AdvisorRiskCritical AdvisorRiskLevel = "critical"
)

// DataSource 数据源
type DataSource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // file, database, vm, container, app
	SizeGB      float64   `json:"size_gb"`
	ChangeRate  float64   `json:"change_rate"` // 每日变更率 0-1
	Importance  int       `json:"importance"`  // 1-10
	LastBackup  *time.Time `json:"last_backup,omitempty"`
	Retention   int       `json:"retention_days"`
	Encrypted   bool      `json:"encrypted"`
	Compressed  bool      `json:"compressed"`
}

// BackupPlan 备份计划
type BackupPlan struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Source      DataSource      `json:"source"`
	Strategy    AdvisorBackupStrategy  `json:"strategy"`
	Schedule    string          `json:"schedule"` // cron expression
	Destination string          `json:"destination"`
	RetentionDays int           `json:"retention_days"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	SourceID    string    `json:"source_id"`
	SourceName  string    `json:"source_name"`
	AdvisorRiskLevel   AdvisorRiskLevel `json:"risk_level"`
	RiskScore   float64   `json:"risk_score"` // 0-100
	Factors     []string  `json:"factors"`
	LastBackup  *time.Time `json:"last_backup,omitempty"`
	DaysSinceBackup int   `json:"days_since_backup"`
	Recommendation  string `json:"recommendation"`
}

// BackupReport 备份报告
type BackupReport struct {
	ID          string           `json:"id"`
	GeneratedAt time.Time        `json:"generated_at"`
	TotalSources int             `json:"total_sources"`
	BackedUp    int              `json:"backed_up"`
	AtRisk      int              `json:"at_risk"`
	Assessments []RiskAssessment `json:"assessments"`
	Recommendations []string     `json:"recommendations"`
	StorageUsedGB   float64      `json:"storage_used_gb"`
	StorageSavedGB  float64      `json:"storage_saved_gb"`
}

// Advisor 智能备份顾问
type Advisor struct {
	sources map[string]*DataSource
	plans   map[string]*BackupPlan
	mu      sync.RWMutex
}

// NewAdvisor 创建智能备份顾问
func NewAdvisor() *Advisor {
	return &Advisor{
		sources: make(map[string]*DataSource),
		plans:   make(map[string]*BackupPlan),
	}
}

// RegisterSource 注册数据源
func (a *Advisor) RegisterSource(source *DataSource) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if source.ID == "" {
		source.ID = fmt.Sprintf("src_%d", time.Now().UnixNano())
	}

	a.sources[source.ID] = source
	return nil
}

// CreatePlan 创建备份计划
func (a *Advisor) CreatePlan(plan *BackupPlan) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if plan.ID == "" {
		plan.ID = fmt.Sprintf("plan_%d", time.Now().UnixNano())
	}

	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	a.plans[plan.ID] = plan
	return nil
}

// AssessRisk 评估风险
func (a *Advisor) AssessRisk(sourceID string) (*RiskAssessment, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	source, ok := a.sources[sourceID]
	if !ok {
		return nil, fmt.Errorf("source not found: %s", sourceID)
	}

	assessment := &RiskAssessment{
		SourceID:   source.ID,
		SourceName: source.Name,
	}

	// 计算风险分数
	score := 0.0
	factors := []string{}

	// 未备份天数
	if source.LastBackup != nil {
		days := int(time.Since(*source.LastBackup).Hours() / 24)
		assessment.DaysSinceBackup = days
		assessment.LastBackup = source.LastBackup
		if days > 7 {
			score += 30
			factors = append(factors, fmt.Sprintf("超过%d天未备份", days))
		} else if days > 3 {
			score += 15
			factors = append(factors, fmt.Sprintf("超过%d天未备份", days))
		}
	} else {
		score += 50
		factors = append(factors, "从未备份")
		assessment.DaysSinceBackup = -1
	}

	// 数据重要性
	if source.Importance >= 8 {
		score += 20
		factors = append(factors, "高重要性数据")
	}

	// 变更率
	if source.ChangeRate > 0.3 {
		score += 15
		factors = append(factors, "高变更率")
	}

	// 未加密
	if !source.Encrypted {
		score += 10
		factors = append(factors, "未加密")
	}

	// 未压缩
	if !source.Compressed {
		score += 5
		factors = append(factors, "未压缩")
	}

	assessment.RiskScore = score
	assessment.Factors = factors

	// 确定风险等级
	switch {
	case score >= 70:
		assessment.AdvisorRiskLevel = AdvisorRiskCritical
		assessment.Recommendation = "立即执行全量备份，启用加密和压缩"
	case score >= 50:
		assessment.AdvisorRiskLevel = AdvisorRiskHigh
		assessment.Recommendation = "建议24小时内执行备份"
	case score >= 30:
		assessment.AdvisorRiskLevel = AdvisorRiskMedium
		assessment.Recommendation = "建议执行增量备份"
	default:
		assessment.AdvisorRiskLevel = AdvisorRiskLow
		assessment.Recommendation = "当前备份状态良好"
	}

	return assessment, nil
}

// RecommendStrategy 推荐备份策略
func (a *Advisor) RecommendStrategy(sourceID string) (AdvisorBackupStrategy, string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	source, ok := a.sources[sourceID]
	if !ok {
		return "", "", fmt.Errorf("source not found: %s", sourceID)
	}

	// 根据数据特征推荐策略
	if source.ChangeRate < 0.05 {
		return AdvisorStrategyMirror, "低变更率数据推荐镜像备份，保证快速恢复", nil
	}
	if source.ChangeRate < 0.2 {
		return AdvisorStrategyIncremental, "中等变更率推荐增量备份，节省存储空间", nil
	}
	if source.SizeGB > 100 {
		return AdvisorStrategyDifferential, "大容量高变更率推荐差异备份，平衡速度与空间", nil
	}
	return AdvisorStrategyFull, "小容量数据推荐全量备份，确保完整恢复", nil
}

// GenerateReport 生成备份报告
func (a *Advisor) GenerateReport() *BackupReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &BackupReport{
		ID:          fmt.Sprintf("rpt_%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
		TotalSources: len(a.sources),
	}

	recommendations := []string{}
	backedUp := 0
	atRisk := 0

	for _, source := range a.sources {
		if source.LastBackup != nil && time.Since(*source.LastBackup).Hours() < 72 {
			backedUp++
		} else {
			atRisk++
		}
		report.StorageUsedGB += source.SizeGB
	}

	report.BackedUp = backedUp
	report.AtRisk = atRisk

	if atRisk > 0 {
		recommendations = append(recommendations, fmt.Sprintf("有%d个数据源需要立即备份", atRisk))
	}
	if backedUp == 0 {
		recommendations = append(recommendations, "建议配置自动备份计划")
	}

	report.Recommendations = recommendations
	return report
}

// GetSources 获取所有数据源
func (a *Advisor) GetSources() []*DataSource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	sources := make([]*DataSource, 0, len(a.sources))
	for _, s := range a.sources {
		sources = append(sources, s)
	}
	return sources
}

// GetPlans 获取所有备份计划
func (a *Advisor) GetPlans() []*BackupPlan {
	a.mu.RLock()
	defer a.mu.RUnlock()

	plans := make([]*BackupPlan, 0, len(a.plans))
	for _, p := range a.plans {
		plans = append(plans, p)
	}
	return plans
}
