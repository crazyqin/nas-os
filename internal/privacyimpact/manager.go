// Package privacyimpact 实现隐私影响评估模块
package privacyimpact

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PII 正则表达式模式.
var piiPatterns = map[PIIType]*regexp.Regexp{
	PIIIDCard:   regexp.MustCompile(`\b[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`),
	PIIPhone:    regexp.MustCompile(`\b1[3-9]\d{9}\b`),
	PIIEmail:    regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
	PIIBankCard: regexp.MustCompile(`\b(?:6[0-9]{15,18}|4[0-9]{12,15}|5[1-5][0-9]{14}|3[47][0-9]{13})\b`),
	PIIPassport: regexp.MustCompile(`\b[A-Z][A-Z0-9]{5,8}\b`),
}

// Manager 隐私影响评估管理器.
type Manager struct {
	config      *Config
	mu          sync.RWMutex
	assessments map[string]*PrivacyAssessment
	dataFlows   []*DataFlowRecord
	auditLog    []*AuditEvent
	running     bool
	stopCh      chan struct{}
}

// NewManager 创建新的隐私影响评估管理器.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:                 true,
			AutoAssess:              false,
			RiskThreshold:           60.0,
			EnabledFrameworks:       []ComplianceFramework{FrameworkPIPL, FrameworkGDPR},
			RetentionDays:           90,
			MaxAuditLogSize:         10000,
			DataFlowTrackingEnabled: true,
		}
	}
	return &Manager{
		config:      config,
		assessments: make(map[string]*PrivacyAssessment),
		dataFlows:   make([]*DataFlowRecord, 0),
		auditLog:    make([]*AuditEvent, 0),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return ErrAlreadyRunning
	}
	if !m.config.Enabled {
		return ErrInvalidConfig
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.cleanupLoop()
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return ErrNotRunning
	}
	m.running = false
	close(m.stopCh)
	return nil
}

// IsRunning 返回管理器是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// cleanupLoop 后台清理过期评估.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupOldAssessments()
		}
	}
}

func (m *Manager) cleanupOldAssessments() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config.RetentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	for id, a := range m.assessments {
		if a.AssessedAt.Before(cutoff) {
			delete(m.assessments, id)
		}
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// addAuditEvent 添加审计事件（调用时需持有写锁）.
func (m *Manager) addAuditEvent(action, resource, resourceID, details string, riskLevel RiskLevel) {
	if m.config.MaxAuditLogSize > 0 && len(m.auditLog) >= m.config.MaxAuditLogSize {
		m.auditLog = m.auditLog[1:]
	}
	event := &AuditEvent{
		ID:         generateID(),
		Timestamp:  time.Now(),
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		Result:     "success",
		RiskLevel:  riskLevel,
	}
	m.auditLog = append(m.auditLog, event)
}

// AssessOperation 评估数据操作的隐私风险.
func (m *Manager) AssessOperation(op DataOperation, dataType string, dataSize int64) (*PrivacyAssessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil, ErrNotRunning
	}

	validOps := map[DataOperation]bool{
		OpUpload: true, OpShare: true, OpExport: true,
		OpDownload: true, OpDelete: true, OpProcess: true, OpTransfer: true,
	}
	if !validOps[op] {
		return nil, ErrInvalidOperation
	}

	score := m.calculateRiskScore(op, dataType, dataSize)

	assessment := &PrivacyAssessment{
		ID:         generateID(),
		Title:      fmt.Sprintf("评估: %s - %s", string(op), dataType),
		Operation:  op,
		DataType:   dataType,
		Status:     AssessmentCompleted,
		RiskScore:  score,
		RiskLevel:  m.scoreToLevel(score),
		AssessedAt: time.Now(),
		AssessedBy: "system",
		Metadata:   make(map[string]string),
	}

	assessment.Recommendations = m.generateRecommendations(assessment)
	m.assessments[assessment.ID] = assessment

	m.addAuditEvent("assess", "operation", assessment.ID,
		fmt.Sprintf("assessed operation %s on %s", op, dataType), assessment.RiskLevel)

	return assessment, nil
}

func (m *Manager) calculateRiskScore(op DataOperation, dataType string, dataSize int64) float64 {
	base := 20.0

	opRisk := map[DataOperation]float64{
		OpUpload: 1.0, OpShare: 1.5, OpExport: 1.3,
		OpDownload: 0.8, OpDelete: 0.5, OpProcess: 1.2, OpTransfer: 1.4,
	}
	if mult, ok := opRisk[op]; ok {
		base *= mult
	}

	dt := strings.ToLower(dataType)
	switch {
	case strings.Contains(dt, "身份证") || strings.Contains(dt, "id_card"):
		base += 40
	case strings.Contains(dt, "银行") || strings.Contains(dt, "bank"):
		base += 40
	case strings.Contains(dt, "健康") || strings.Contains(dt, "health") || strings.Contains(dt, "医疗"):
		base += 30
	case strings.Contains(dt, "生物") || strings.Contains(dt, "biometric"):
		base += 35
	case strings.Contains(dt, "位置") || strings.Contains(dt, "location"):
		base += 20
	case strings.Contains(dt, "邮箱") || strings.Contains(dt, "email"):
		base += 15
	case strings.Contains(dt, "手机") || strings.Contains(dt, "phone"):
		base += 20
	}

	if dataSize > 100*1024*1024 {
		base += 15
	} else if dataSize > 10*1024*1024 {
		base += 10
	} else if dataSize > 1024*1024 {
		base += 5
	}

	if base > 100 {
		base = 100
	}
	return base
}

func (m *Manager) scoreToLevel(score float64) RiskLevel {
	switch {
	case score >= 80:
		return RiskCritical
	case score >= 60:
		return RiskHigh
	case score >= 40:
		return RiskMedium
	case score >= 20:
		return RiskLow
	default:
		return RiskNone
	}
}

func (m *Manager) generateRecommendations(a *PrivacyAssessment) []Recommendation {
	recs := make([]Recommendation, 0)

	if a.RiskScore >= 60 {
		recs = append(recs, Recommendation{
			ID:          generateID(),
			Category:    "encryption",
			Title:       "启用端到端加密",
			Description: "高风险数据操作应使用端到端加密保护",
			Priority:    1,
			EffortLevel: "medium",
			ImpactLevel: "high",
			Steps:       []string{"评估加密方案", "实施传输加密", "实施存储加密"},
		})
	}

	if a.Operation == OpShare || a.Operation == OpTransfer {
		recs = append(recs, Recommendation{
			ID:          generateID(),
			Category:    "access_control",
			Title:       "加强访问控制",
			Description: "数据共享/传输应实施严格的访问控制",
			Priority:    2,
			EffortLevel: "low",
			ImpactLevel: "medium",
			Steps:       []string{"定义访问策略", "实施最小权限原则", "定期审计访问日志"},
		})
	}

	if a.RiskScore >= 40 {
		recs = append(recs, Recommendation{
			ID:          generateID(),
			Category:    "monitoring",
			Title:       "增强监控告警",
			Description: "对敏感数据操作实施实时监控和告警",
			Priority:    2,
			EffortLevel: "medium",
			ImpactLevel: "high",
			Steps:       []string{"配置监控规则", "设置告警阈值", "建立响应流程"},
		})
	}

	return recs
}

// GetAssessment 获取指定ID的评估.
func (m *Manager) GetAssessment(id string) (*PrivacyAssessment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running {
		return nil, ErrNotRunning
	}
	a, ok := m.assessments[id]
	if !ok {
		return nil, ErrAssessmentNotFound
	}
	return a, nil
}

// ListAssessments 列出所有评估.
func (m *Manager) ListAssessments() []*PrivacyAssessment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*PrivacyAssessment, 0, len(m.assessments))
	for _, a := range m.assessments {
		result = append(result, a)
	}
	return result
}

// DetectSensitiveData 检测路径中的敏感数据（PII类型）.
func (m *Manager) DetectSensitiveData(path string) ([]PIIType, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil, ErrNotRunning
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataNotFound, path)
	}

	detected := make(map[PIIType]bool)

	if info.IsDir() {
		err = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() || fi.Size() > 10*1024*1024 {
				return nil
			}
			for _, t := range m.scanFile(p) {
				detected[t] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		if info.Size() <= 10*1024*1024 {
			for _, t := range m.scanFile(path) {
				detected[t] = true
			}
		}
	}

	result := make([]PIIType, 0, len(detected))
	for t := range detected {
		result = append(result, t)
	}

	if len(result) > 0 {
		m.addAuditEvent("scan", "file", path,
			fmt.Sprintf("detected %d PII types", len(result)), RiskMedium)
	}

	return result, nil
}

func (m *Manager) scanFile(path string) []PIIType {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	found := make([]PIIType, 0)
	for piiType, pattern := range piiPatterns {
		if pattern.MatchString(content) {
			found = append(found, piiType)
		}
	}
	return found
}

// TrackDataFlow 追踪数据流向.
func (m *Manager) TrackDataFlow(source, destination string, operation DataOperation) (*DataFlowRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil, ErrNotRunning
	}
	if !m.config.DataFlowTrackingEnabled {
		return nil, fmt.Errorf("data flow tracking is disabled")
	}

	record := &DataFlowRecord{
		ID:        generateID(),
		Operation: operation,
		Source: DataEndpoint{
			Type:     "local",
			Location: source,
		},
		Destination: DataEndpoint{
			Type:     "remote",
			Location: destination,
		},
		Timestamp: time.Now(),
		Status:    "recorded",
	}

	m.dataFlows = append(m.dataFlows, record)
	m.addAuditEvent("dataflow", "transfer", record.ID,
		fmt.Sprintf("tracked %s from %s to %s", operation, source, destination), RiskLow)

	return record, nil
}

// RunComplianceCheck 执行合规检查.
func (m *Manager) RunComplianceCheck() (*ComplianceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil, ErrNotRunning
	}

	totalScore := 0.0
	count := 0
	for _, a := range m.assessments {
		if a.Status == AssessmentCompleted {
			totalScore += a.RiskScore
			count++
		}
	}

	avgRisk := 0.0
	if count > 0 {
		avgRisk = totalScore / float64(count)
	}

	complianceScore := 100.0 - avgRisk
	if complianceScore < 0 {
		complianceScore = 0
	}

	status := StatusCompliant
	if complianceScore < 60 {
		status = StatusNonCompliant
	} else if complianceScore < 80 {
		status = StatusPartial
	}

	checks := []ComplianceCheck{
		{
			ID:          generateID(),
			Name:        "数据加密检查",
			Description: "检查敏感数据是否加密存储和传输",
			Status:      status,
			Score:       complianceScore,
			MaxScore:    100,
		},
		{
			ID:          generateID(),
			Name:        "访问控制检查",
			Description: "检查数据访问权限是否合理",
			Status:      status,
			Score:       complianceScore,
			MaxScore:    100,
		},
		{
			ID:          generateID(),
			Name:        "数据保留策略检查",
			Description: "检查数据保留期限是否合规",
			Status:      status,
			Score:       complianceScore,
			MaxScore:    100,
		},
	}

	result := &ComplianceResult{
		Framework: FrameworkPIPL,
		Status:    status,
		Score:     complianceScore,
		Checks:    checks,
		CheckedAt: time.Now(),
	}

	m.addAuditEvent("compliance", "check", "system",
		fmt.Sprintf("compliance check: score=%.1f, status=%s", complianceScore, status),
		m.scoreToLevel(avgRisk))

	return result, nil
}

// GetDashboard 获取仪表盘统计数据.
func (m *Manager) GetDashboard() *PrivacyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PrivacyStats{
		PIIByType:        make(map[PIIType]int),
		ComplianceScores: make(map[ComplianceFramework]float64),
	}

	totalScore := 0.0
	for _, a := range m.assessments {
		stats.TotalAssessments++
		switch a.Status {
		case AssessmentCompleted:
			stats.CompletedAssessments++
			totalScore += a.RiskScore
		case AssessmentPending:
			stats.PendingAssessments++
		}
		if a.RiskLevel == RiskHigh {
			stats.HighRiskCount++
		}
		if a.RiskLevel == RiskCritical {
			stats.CriticalRiskCount++
		}
		if a.AssessedAt.After(stats.LastAssessmentTime) {
			stats.LastAssessmentTime = a.AssessedAt
		}
	}

	if stats.CompletedAssessments > 0 {
		stats.AverageRiskScore = totalScore / float64(stats.CompletedAssessments)
	}

	stats.TotalAuditEvents = len(m.auditLog)
	stats.DataFlowRecords = len(m.dataFlows)

	stats.ComplianceScores[FrameworkPIPL] = 100 - stats.AverageRiskScore
	stats.ComplianceScores[FrameworkGDPR] = 100 - stats.AverageRiskScore

	return stats
}

// GetRecommendations 获取全局建议列表（按分类去重，保留最高优先级）.
func (m *Manager) GetRecommendations() []*Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recMap := make(map[string]*Recommendation)

	for _, a := range m.assessments {
		for i := range a.Recommendations {
			rec := &a.Recommendations[i]
			if existing, ok := recMap[rec.Category]; !ok || rec.Priority < existing.Priority {
				// 复制一份避免指针指向 slice 内部
				copy := *rec
				recMap[rec.Category] = &copy
			}
		}
	}

	result := make([]*Recommendation, 0, len(recMap))
	for _, rec := range recMap {
		result = append(result, rec)
	}
	return result
}
