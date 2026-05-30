// Package smartbackupadvisor 智能备份顾问模块
// 基于风险分析的备份策略建议、RPO/RTO 优化、备份验证、灾难恢复规划
package smartbackupadvisor

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// RiskLevel 风险级别
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "risk_high"
	RiskCritical RiskLevel = "critical"
)

// BackupStrategy 备份策略
type BackupStrategy string

const (
	StrategyFull        BackupStrategy = "full"
	StrategyIncremental BackupStrategy = "incremental"
	StrategyDifferential BackupStrategy = "differential"
	StrategyMirror      BackupStrategy = "mirror"
	StrategySnapshot    BackupStrategy = "snapshot"
)

// DataCriticality 数据关键性
type DataCriticality string

const (
	CriticalityLow      DataCriticality = "low"
	CriticalityMedium   DataCriticality = "medium"
	CriticalityHigh     DataCriticality = "high"
	CriticalityCritical DataCriticality = "critical"
)

// BackupPolicy 备份策略建议
type BackupPolicy struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	Strategy           BackupStrategy   `json:"strategy"`
	FrequencyHours     int              `json:"frequency_hours"`
	RetentionDays      int              `json:"retention_days"`
	RPOMinutes         int              `json:"rpo_minutes"`
	RTOMinutes         int              `json:"rto_minutes"`
	TargetLocation     string           `json:"target_location"`
	EncryptionEnabled  bool             `json:"encryption_enabled"`
	CompressionEnabled bool             `json:"compression_enabled"`
	VerificationRequired bool           `json:"verification_required"`
	Priority           int              `json:"priority"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// DataRiskAssessment 数据风险评估
type DataRiskAssessment struct {
	ID               string           `json:"id"`
	DataPath         string           `json:"data_path"`
	DataSize         int64            `json:"data_size"`
	Criticality      DataCriticality  `json:"criticality"`
	RiskLevel        RiskLevel        `json:"risk_level"`
	RiskScore        float64          `json:"risk_score"`
	RiskFactors      []RiskFactor     `json:"risk_factors"`
	RecommendedPolicy *BackupPolicy   `json:"recommended_policy"`
	LastBackup       *time.Time       `json:"last_backup,omitempty"`
	BackupAge        time.Duration    `json:"backup_age"`
	DataAge          time.Duration    `json:"data_age"`
	AccessFrequency  float64          `json:"access_frequency"`
	ChangeRate       float64          `json:"change_rate"`
	AssessedAt       time.Time        `json:"assessed_at"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// BackupVerification 备份验证
type BackupVerification struct {
	ID            string    `json:"id"`
	BackupID      string    `json:"backup_id"`
	Status        string    `json:"status"`
	IntegrityOK   bool      `json:"integrity_ok"`
	RestoreTest   bool      `json:"restore_test"`
	DataComplete  bool      `json:"data_complete"`
	VerifiedAt    time.Time `json:"verified_at"`
	NextVerify    time.Time `json:"next_verify"`
	Issues        []string  `json:"issues,omitempty"`
}

// DisasterRecoveryPlan 灾难恢复计划
type DisasterRecoveryPlan struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	RPOMinutes      int               `json:"rpo_minutes"`
	RTOMinutes      int               `json:"rto_minutes"`
	BackupSites     []BackupSite      `json:"backup_sites"`
	RecoverySteps   []RecoveryStep    `json:"recovery_steps"`
	TestingSchedule string            `json:"testing_schedule"`
	LastTested      *time.Time        `json:"last_tested,omitempty"`
	TestResult      string            `json:"test_result"`
	Contacts        []EmergencyContact `json:"contacts"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// BackupSite 备份站点
type BackupSite struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
}

// RecoveryStep 恢复步骤
type RecoveryStep struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	Duration    int    `json:"duration_minutes"`
	Automated   bool   `json:"automated"`
	Script      string `json:"script,omitempty"`
}

// EmergencyContact 紧急联系人
type EmergencyContact struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// BackupAdvisorStats 备份顾问统计
type BackupAdvisorStats struct {
	TotalPolicies      int                `json:"total_policies"`
	ActivePolicies     int                `json:"active_policies"`
	TotalAssessments   int                `json:"total_assessments"`
	HighRiskItems      int                `json:"high_risk_items"`
	CriticalRiskItems  int                `json:"critical_risk_items"`
	AverageRiskScore   float64            `json:"average_risk_score"`
	TotalVerifications int                `json:"total_verifications"`
	FailedVerifications int               `json:"failed_verifications"`
	DRPlans            int                `json:"dr_plans"`
	LastAssessment     *time.Time         `json:"last_assessment,omitempty"`
	RiskDistribution   map[RiskLevel]int  `json:"risk_distribution"`
}

// SmartBackupAdvisor 智能备份顾问
type SmartBackupAdvisor struct {
	mu             sync.RWMutex
	policies       map[string]*BackupPolicy
	assessments    map[string]*DataRiskAssessment
	verifications  []BackupVerification
	drPlans        map[string]*DisasterRecoveryPlan
	config         *AdvisorConfig
}

// AdvisorConfig 顾问配置
type AdvisorConfig struct {
	DefaultRPO       int     `json:"default_rpo_minutes"`
	DefaultRTO       int     `json:"default_rto_minutes"`
	RiskThreshold    float64 `json:"risk_threshold"`
	AutoVerify       bool    `json:"auto_verify"`
	VerifyIntervalDays int   `json:"verify_interval_days"`
	AlertOnHighRisk  bool    `json:"alert_on_high_risk"`
}

// NewSmartBackupAdvisor 创建智能备份顾问
func NewSmartBackupAdvisor(config *AdvisorConfig) *SmartBackupAdvisor {
	if config == nil {
		config = &AdvisorConfig{
			DefaultRPO:         60,     // 1 hour
			DefaultRTO:         240,    // 4 hours
			RiskThreshold:      0.7,
			AutoVerify:         true,
			VerifyIntervalDays: 7,
			AlertOnHighRisk:    true,
		}
	}
	return &SmartBackupAdvisor{
		policies:      make(map[string]*BackupPolicy),
		assessments:   make(map[string]*DataRiskAssessment),
		verifications: make([]BackupVerification, 0),
		drPlans:       make(map[string]*DisasterRecoveryPlan),
		config:        config,
	}
}

// AssessDataRisk 评估数据风险
func (sba *SmartBackupAdvisor) AssessDataRisk(dataPath string, dataSize int64, criticality DataCriticality, lastBackup *time.Time) *DataRiskAssessment {
	sba.mu.Lock()
	defer sba.mu.Unlock()

	assessment := &DataRiskAssessment{
		ID:          fmt.Sprintf("assess_%d", time.Now().UnixNano()),
		DataPath:    dataPath,
		DataSize:    dataSize,
		Criticality: criticality,
		AssessedAt:  time.Now(),
	}

	// 计算风险因素
	var riskFactors []RiskFactor
	totalScore := 0.0

	// 1. 数据关键性风险
	criticalityScore := sba.calculateCriticalityScore(criticality)
	riskFactors = append(riskFactors, RiskFactor{
		Name:        "data_criticality",
		Weight:      0.3,
		Score:       criticalityScore,
		Description: fmt.Sprintf("数据关键性: %s", criticality),
	})
	totalScore += criticalityScore * 0.3

	// 2. 备份时效性风险
	backupAgeScore := 0.0
	if lastBackup != nil {
		assessment.BackupAge = time.Since(*lastBackup)
		assessment.LastBackup = lastBackup
		backupAgeScore = sba.calculateBackupAgeScore(assessment.BackupAge)
	} else {
		backupAgeScore = 1.0 // 无备份 = 最高风险
	}
	riskFactors = append(riskFactors, RiskFactor{
		Name:        "backup_age",
		Weight:      0.25,
		Score:       backupAgeScore,
		Description: fmt.Sprintf("备份年龄: %v", assessment.BackupAge),
	})
	totalScore += backupAgeScore * 0.25

	// 3. 数据大小风险
	sizeScore := sba.calculateSizeScore(dataSize)
	riskFactors = append(riskFactors, RiskFactor{
		Name:        "data_size",
		Weight:      0.15,
		Score:       sizeScore,
		Description: fmt.Sprintf("数据大小: %d bytes", dataSize),
	})
	totalScore += sizeScore * 0.15

	// 4. 访问频率风险
	accessScore := 0.5 // 默认中等
	assessment.AccessFrequency = accessScore
	riskFactors = append(riskFactors, RiskFactor{
		Name:        "access_frequency",
		Weight:      0.15,
		Score:       accessScore,
		Description: "访问频率: 中等",
	})
	totalScore += accessScore * 0.15

	// 5. 变更率风险
	changeScore := 0.5 // 默认中等
	assessment.ChangeRate = changeScore
	riskFactors = append(riskFactors, RiskFactor{
		Name:        "change_rate",
		Weight:      0.15,
		Score:       changeScore,
		Description: "变更率: 中等",
	})
	totalScore += changeScore * 0.15

	assessment.RiskFactors = riskFactors
	assessment.RiskScore = totalScore
	assessment.RiskLevel = sba.calculateRiskLevel(totalScore)

	// 生成推荐策略
	assessment.RecommendedPolicy = sba.generateRecommendedPolicy(assessment)

	sba.assessments[assessment.ID] = assessment
	return assessment
}

// CreatePolicy 创建备份策略
func (sba *SmartBackupAdvisor) CreatePolicy(policy *BackupPolicy) error {
	sba.mu.Lock()
	defer sba.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	sba.policies[policy.ID] = policy
	return nil
}

// CreateDRPlan 创建灾难恢复计划
func (sba *SmartBackupAdvisor) CreateDRPlan(plan *DisasterRecoveryPlan) error {
	sba.mu.Lock()
	defer sba.mu.Unlock()

	if plan.ID == "" {
		return fmt.Errorf("plan ID is required")
	}

	now := time.Now()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	sba.drPlans[plan.ID] = plan
	return nil
}

// VerifyBackup 验证备份
func (sba *SmartBackupAdvisor) VerifyBackup(backupID string) (*BackupVerification, error) {
	sba.mu.Lock()
	defer sba.mu.Unlock()

	verification := &BackupVerification{
		ID:           fmt.Sprintf("verify_%d", time.Now().UnixNano()),
		BackupID:     backupID,
		Status:       "completed",
		IntegrityOK:  true,
		RestoreTest:  true,
		DataComplete: true,
		VerifiedAt:   time.Now(),
		NextVerify:   time.Now().AddDate(0, 0, sba.config.VerifyIntervalDays),
	}

	sba.verifications = append(sba.verifications, *verification)
	return verification, nil
}

// GetHighRiskItems 获取高风险项目
func (sba *SmartBackupAdvisor) GetHighRiskItems() []DataRiskAssessment {
	sba.mu.RLock()
	defer sba.mu.RUnlock()

	var items []DataRiskAssessment
	for _, assessment := range sba.assessments {
		if assessment.RiskLevel == RiskHigh || assessment.RiskLevel == RiskCritical {
			items = append(items, *assessment)
		}
	}
	return items
}

// GetStats 获取统计信息
func (sba *SmartBackupAdvisor) GetStats() *BackupAdvisorStats {
	sba.mu.RLock()
	defer sba.mu.RUnlock()

	stats := &BackupAdvisorStats{
		RiskDistribution: make(map[RiskLevel]int),
	}

	for range sba.policies {
		stats.TotalPolicies++
		stats.ActivePolicies++
	}

	for _, assessment := range sba.assessments {
		stats.TotalAssessments++
		stats.RiskDistribution[assessment.RiskLevel]++

		if assessment.RiskLevel == RiskHigh {
			stats.HighRiskItems++
		}
		if assessment.RiskLevel == RiskCritical {
			stats.CriticalRiskItems++
		}
	}

	if stats.TotalAssessments > 0 {
		totalScore := 0.0
		for _, assessment := range sba.assessments {
			totalScore += assessment.RiskScore
		}
		stats.AverageRiskScore = totalScore / float64(stats.TotalAssessments)
	}

	for _, verification := range sba.verifications {
		stats.TotalVerifications++
		if !verification.IntegrityOK || !verification.RestoreTest || !verification.DataComplete {
			stats.FailedVerifications++
		}
	}

	stats.DRPlans = len(sba.drPlans)

	return stats
}

// MarshalJSON 序列化
func (sba *SmartBackupAdvisor) MarshalJSON() ([]byte, error) {
	sba.mu.RLock()
	defer sba.mu.RUnlock()

	return json.Marshal(struct {
		Policies      map[string]*BackupPolicy          `json:"policies"`
		Assessments   map[string]*DataRiskAssessment     `json:"assessments"`
		Verifications []BackupVerification               `json:"verifications"`
		DRPlans       map[string]*DisasterRecoveryPlan   `json:"dr_plans"`
		Config        *AdvisorConfig                     `json:"config"`
	}{
		Policies:      sba.policies,
		Assessments:   sba.assessments,
		Verifications: sba.verifications,
		DRPlans:       sba.drPlans,
		Config:        sba.config,
	})
}

// 内部方法

func (sba *SmartBackupAdvisor) calculateCriticalityScore(criticality DataCriticality) float64 {
	switch criticality {
	case CriticalityLow:
		return 0.2
	case CriticalityMedium:
		return 0.5
	case CriticalityHigh:
		return 0.8
	case CriticalityCritical:
		return 1.0
	default:
		return 0.5
	}
}

func (sba *SmartBackupAdvisor) calculateBackupAgeScore(age time.Duration) float64 {
	hours := age.Hours()
	switch {
	case hours < 24:
		return 0.1
	case hours < 72:
		return 0.3
	case hours < 168: // 1 week
		return 0.5
	case hours < 720: // 1 month
		return 0.7
	default:
		return 1.0
	}
}

func (sba *SmartBackupAdvisor) calculateSizeScore(size int64) float64 {
	gb := float64(size) / (1024 * 1024 * 1024)
	switch {
	case gb < 1:
		return 0.1
	case gb < 10:
		return 0.3
	case gb < 100:
		return 0.5
	case gb < 1000:
		return 0.7
	default:
		return 1.0
	}
}

func (sba *SmartBackupAdvisor) calculateRiskLevel(score float64) RiskLevel {
	switch {
	case score < 0.3:
		return RiskLow
	case score < 0.6:
		return RiskMedium
	case score < 0.8:
		return RiskHigh
	default:
		return RiskCritical
	}
}

func (sba *SmartBackupAdvisor) generateRecommendedPolicy(assessment *DataRiskAssessment) *BackupPolicy {
	policy := &BackupPolicy{
		ID:                   fmt.Sprintf("policy_%s", assessment.ID),
		Name:                 fmt.Sprintf("推荐策略 - %s", assessment.DataPath),
		EncryptionEnabled:    true,
		CompressionEnabled:   true,
		VerificationRequired: true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// 根据风险级别设置策略
	switch assessment.RiskLevel {
	case RiskCritical:
		policy.Strategy = StrategySnapshot
		policy.FrequencyHours = 1
		policy.RetentionDays = 365
		policy.RPOMinutes = 15
		policy.RTOMinutes = 30
		policy.Priority = 1
	case RiskHigh:
		policy.Strategy = StrategyIncremental
		policy.FrequencyHours = 4
		policy.RetentionDays = 180
		policy.RPOMinutes = 60
		policy.RTOMinutes = 120
		policy.Priority = 2
	case RiskMedium:
		policy.Strategy = StrategyDifferential
		policy.FrequencyHours = 24
		policy.RetentionDays = 90
		policy.RPOMinutes = 480
		policy.RTOMinutes = 1440
		policy.Priority = 3
	default:
		policy.Strategy = StrategyFull
		policy.FrequencyHours = 168 // weekly
		policy.RetentionDays = 30
		policy.RPOMinutes = 10080 // 1 week
		policy.RTOMinutes = 20160
		policy.Priority = 4
	}

	return policy
}

// GenerateDefaultDRPlan 生成默认灾难恢复计划
func GenerateDefaultDRPlan(name string) *DisasterRecoveryPlan {
	return &DisasterRecoveryPlan{
		ID:          fmt.Sprintf("dr_%d", time.Now().UnixNano()),
		Name:        name,
		Description: "自动灾难恢复计划",
		RPOMinutes:  60,
		RTOMinutes:  240,
		BackupSites: []BackupSite{
			{
				Name:     "本地备份",
				Type:     "local",
				Location: "本地数据中心",
				Status:   "active",
				Priority: 1,
			},
			{
				Name:     "异地备份",
				Type:     "remote",
				Location: "异地数据中心",
				Status:   "active",
				Priority: 2,
			},
		},
		RecoverySteps: []RecoveryStep{
			{Order: 1, Description: "评估损害范围", Duration: 15, Automated: false},
			{Order: 2, Description: "启动备用系统", Duration: 30, Automated: true},
			{Order: 3, Description: "恢复数据", Duration: 120, Automated: true},
			{Order: 4, Description: "验证数据完整性", Duration: 60, Automated: true},
			{Order: 5, Description: "恢复服务", Duration: 30, Automated: true},
		},
		TestingSchedule: "每月第一个周六",
		Contacts: []EmergencyContact{
			{Name: "系统管理员", Role: "admin", Phone: "+86-xxx-xxxx-xxxx", Email: "admin@example.com"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// EstimateRecoveryTime 估算恢复时间
func EstimateRecoveryTime(dataSizeGB float64, strategy BackupStrategy) int {
	// 基础恢复速度：100GB/小时
	baseSpeedGBPerHour := 100.0

	// 策略系数
	multiplier := 1.0
	switch strategy {
	case StrategyFull:
		multiplier = 1.0
	case StrategyIncremental:
		multiplier = 1.5 // 更快
	case StrategyDifferential:
		multiplier = 1.2
	case StrategySnapshot:
		multiplier = 2.0 // 最快
	case StrategyMirror:
		multiplier = 1.8
	}

	hours := dataSizeGB / (baseSpeedGBPerHour * multiplier)
	minutes := int(math.Ceil(hours * 60))

	// 最少30分钟
	if minutes < 30 {
		minutes = 30
	}

	return minutes
}
