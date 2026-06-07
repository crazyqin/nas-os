// Package datagovernance - 核心管理器
package datagovernance

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Manager 数据治理管理器.
type Manager struct {
	mu       sync.RWMutex
	config   Config
	assets   map[string]*DataAsset
	policies map[string]*RetentionPolicy
	auditLog []AuditRecord
	lineage  map[string][]*LineageRecord // assetID -> lineage records
	reports  map[string]*ComplianceReport
	running  bool
}

// NewManager 创建数据治理管理器.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:   cfg,
		assets:   make(map[string]*DataAsset),
		policies: make(map[string]*RetentionPolicy),
		lineage:  make(map[string][]*LineageRecord),
		reports:  make(map[string]*ComplianceReport),
	}
}

// Start 启动数据治理引擎.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return ErrAlreadyRunning
	}
	m.running = true
	m.initDefaultPolicies()
	return nil
}

// Stop 停止数据治理引擎.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 返回引擎运行状态.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Manager) initDefaultPolicies() {
	defaults := []RetentionPolicy{
		{
			ID:                "policy-public",
			Name:              "公开数据保留策略",
			Description:       "公开数据保留365天后归档",
			SensitivityLevels: []SensitivityLevel{LevelPublic},
			RetentionDays:     365,
			ExpirationAction:  ActionArchive,
			Enabled:           true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
		{
			ID:                "policy-internal",
			Name:              "内部数据保留策略",
			Description:       "内部数据保留730天后归档",
			SensitivityLevels: []SensitivityLevel{LevelInternal},
			RetentionDays:     730,
			ExpirationAction:  ActionArchive,
			Enabled:           true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
		{
			ID:                "policy-confidential",
			Name:              "机密数据保留策略",
			Description:       "机密数据保留1095天后匿名化",
			SensitivityLevels: []SensitivityLevel{LevelConfidential},
			RetentionDays:     1095,
			ExpirationAction:  ActionAnonymize,
			Enabled:           true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
		{
			ID:                "policy-top-secret",
			Name:              "绝密数据保留策略",
			Description:       "绝密数据保留1825天后销毁",
			SensitivityLevels: []SensitivityLevel{LevelTopSecret},
			RetentionDays:     1825,
			ExpirationAction:  RetentionActionDestroy,
			Enabled:           true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
	}
	for i := range defaults {
		m.policies[defaults[i].ID] = &defaults[i]
	}
}

// ========== 数据分类标签 ==========

// ClassifyAsset 对数据资产进行敏感度分级.
func (m *Manager) ClassifyAsset(assetID string, level SensitivityLevel, classifiedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return ErrRecordNotFound
	}

	oldLevel := asset.Sensitivity
	asset.Sensitivity = level
	asset.ClassifiedBy = classifiedBy
	asset.UpdatedAt = time.Now()

	// 自动匹配保留策略
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}
		for _, sl := range policy.SensitivityLevels {
			if sl == level {
				asset.PolicyID = policy.ID
				deadline := time.Now().AddDate(0, 0, policy.RetentionDays)
				asset.RetentionDeadline = &deadline
				break
			}
		}
	}

	m.auditLog = append(m.auditLog, AuditRecord{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Action:    ActionClassify,
		AssetID:   assetID,
		AssetName: asset.Name,
		Details:   fmt.Sprintf("sensitivity changed from %s to %s by %s", oldLevel, level, classifiedBy),
		Result:    "success",
		RiskLevel: "low",
	})

	return nil
}

// AutoClassify 自动分类扫描（基于文件路径和类型规则）.
func (m *Manager) AutoClassify() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	classified := 0
	for _, asset := range m.assets {
		if asset.Sensitivity != "" && asset.ClassifiedBy != "" {
			continue // 已分类
		}

		level := m.inferSensitivity(asset)
		asset.Sensitivity = level
		asset.ClassifiedBy = "ai"
		asset.UpdatedAt = time.Now()

		// 匹配策略
		for _, policy := range m.policies {
			if !policy.Enabled {
				continue
			}
			for _, sl := range policy.SensitivityLevels {
				if sl == level {
					asset.PolicyID = policy.ID
					deadline := time.Now().AddDate(0, 0, policy.RetentionDays)
					asset.RetentionDeadline = &deadline
					break
				}
			}
		}

		classified++
	}
	return classified
}

func (m *Manager) inferSensitivity(asset *DataAsset) SensitivityLevel {
	path := strings.ToLower(asset.FilePath)
	name := strings.ToLower(asset.Name)
	ftype := strings.ToLower(asset.FileType)

	// 绝密：密钥、证书、密码文件
	if strings.Contains(path, "secret") || strings.Contains(path, "private") ||
		strings.Contains(name, "key") || strings.Contains(name, "credential") ||
		ftype == "pem" || ftype == "key" || ftype == "p12" {
		return LevelTopSecret
	}

	// 机密：财务、人事、合同
	if strings.Contains(path, "finance") || strings.Contains(path, "hr") ||
		strings.Contains(path, "contract") || strings.Contains(path, "payroll") ||
		strings.Contains(name, "confidential") {
		return LevelConfidential
	}

	// 内部：代码、文档
	if strings.Contains(path, "internal") || strings.Contains(path, "docs") ||
		ftype == "go" || ftype == "py" || ftype == "js" || ftype == "docx" || ftype == "xlsx" {
		return LevelInternal
	}

	// 默认公开
	return LevelPublic
}

// ========== 数据驻留合规 ==========

// RegisterAsset 注册数据资产.
func (m *Manager) RegisterAsset(asset DataAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if asset.ID == "" {
		asset.ID = fmt.Sprintf("asset-%d", time.Now().UnixNano())
	}
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = time.Now()
	}
	asset.UpdatedAt = time.Now()

	m.assets[asset.ID] = &asset

	m.auditLog = append(m.auditLog, AuditRecord{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		UserID:    asset.OwnerID,
		UserName:  asset.OwnerName,
		Action:    ActionCreate,
		AssetID:   asset.ID,
		AssetName: asset.Name,
		Details:   fmt.Sprintf("registered in region %s", asset.Region),
		Region:    asset.Region,
		Result:    "success",
		RiskLevel: "low",
	})

	return nil
}

// CheckResidency 检查数据驻留合规性.
func (m *Manager) CheckResidency(assetID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return false, ErrRecordNotFound
	}

	if len(m.config.AllowedRegions) == 0 {
		return true, nil
	}

	for _, region := range m.config.AllowedRegions {
		if asset.Region == region {
			return true, nil
		}
	}
	return false, nil
}

// CheckAllResidency 检查所有资产的驻留合规性.
func (m *Manager) CheckAllResidency() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var violations []string
	for _, asset := range m.assets {
		if len(m.config.AllowedRegions) == 0 {
			continue
		}
		compliant := false
		for _, region := range m.config.AllowedRegions {
			if asset.Region == region {
				compliant = true
				break
			}
		}
		if !compliant {
			violations = append(violations, asset.ID)
		}
	}
	return violations
}

// RelocateAsset 迁移资产到指定区域.
func (m *Manager) RelocateAsset(assetID string, newRegion GeoRegion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return ErrRecordNotFound
	}

	oldRegion := asset.Region
	asset.Region = newRegion
	asset.UpdatedAt = time.Now()

	m.auditLog = append(m.auditLog, AuditRecord{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Action:    ActionUpdate,
		AssetID:   assetID,
		AssetName: asset.Name,
		Details:   fmt.Sprintf("relocated from %s to %s", oldRegion, newRegion),
		Region:    newRegion,
		Result:    "success",
		RiskLevel: "medium",
	})

	return nil
}

// ========== 保留策略执行 ==========

// CreatePolicy 创建保留策略.
func (m *Manager) CreatePolicy(policy RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = &policy
	return nil
}

// GetPolicy 获取保留策略.
func (m *Manager) GetPolicy(id string) (*RetentionPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有保留策略.
func (m *Manager) ListPolicies() []RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RetentionPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, *p)
	}
	return result
}

// UpdatePolicy 更新保留策略.
func (m *Manager) UpdatePolicy(id string, update RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.policies[id]
	if !ok {
		return ErrPolicyNotFound
	}

	if update.Name != "" {
		p.Name = update.Name
	}
	if update.Description != "" {
		p.Description = update.Description
	}
	if update.RetentionDays > 0 {
		p.RetentionDays = update.RetentionDays
	}
	if update.ExpirationAction != "" {
		p.ExpirationAction = update.ExpirationAction
	}
	if len(update.SensitivityLevels) > 0 {
		p.SensitivityLevels = update.SensitivityLevels
	}
	p.Enabled = update.Enabled
	p.UpdatedAt = time.Now()

	return nil
}

// DeletePolicy 删除保留策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return ErrPolicyNotFound
	}
	delete(m.policies, id)
	return nil
}

// EnforceRetention 执行保留策略，返回到期资产列表.
func (m *Manager) EnforceRetention() []DataAsset {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []DataAsset
	now := time.Now()

	for _, asset := range m.assets {
		if asset.RetentionDeadline == nil || asset.PolicyID == "" {
			continue
		}
		if now.Before(*asset.RetentionDeadline) {
			continue
		}

		policy, ok := m.policies[asset.PolicyID]
		if !ok || !policy.Enabled {
			continue
		}

		expired = append(expired, *asset)

		m.auditLog = append(m.auditLog, AuditRecord{
			ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
			Timestamp: now,
			Action:    ActionDestroy,
			AssetID:   asset.ID,
			AssetName: asset.Name,
			Details:   fmt.Sprintf("retention policy %s triggered, action: %s", policy.ID, policy.ExpirationAction),
			Result:    "success",
			RiskLevel: "high",
		})
	}

	return expired
}

// ========== 审计追踪 ==========

// LogAudit 手动记录审计事件.
func (m *Manager) LogAudit(record AuditRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	record.Timestamp = time.Now()
	m.auditLog = append(m.auditLog, record)

	// 限制审计日志大小
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-5000:]
	}
}

// GetAuditLog 获取审计日志（支持过滤和分页）.
func (m *Manager) GetAuditLog(userID, action, assetID string, page, pageSize int) ([]AuditRecord, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AuditRecord
	for _, r := range m.auditLog {
		if userID != "" && r.UserID != userID {
			continue
		}
		if action != "" && string(r.Action) != action {
			continue
		}
		if assetID != "" && r.AssetID != assetID {
			continue
		}
		result = append(result, r)
	}

	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// ========== 合规报告生成 ==========

// GenerateReport 生成合规报告.
func (m *Manager) GenerateReport(framework ComplianceFramework) *ComplianceReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("rpt-%d", time.Now().UnixNano()),
		Framework:   framework,
		MaxScore:    100,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(30 * 24 * time.Hour),
		GeneratedBy: "system",
	}

	// 检查各合规项
	score := 0.0
	checks := m.evaluateCompliance(framework)
	report.TotalChecks = len(checks)

	for _, c := range checks {
		if c.passed {
			report.PassedChecks++
			score += c.score
		} else {
			report.FailedChecks++
			report.Findings = append(report.Findings, ReportFinding{
				ID:             fmt.Sprintf("find-%d", time.Now().UnixNano()),
				Category:       c.category,
				Title:          c.title,
				Description:    c.description,
				Severity:       c.severity,
				AffectedAssets: c.affectedAssets,
				Remediation:    c.remediation,
				Status:         "open",
			})
		}
	}

	if report.TotalChecks > 0 {
		report.OverallScore = score / float64(report.TotalChecks)
	}

	// 区域合规
	regionStats := make(map[GeoRegion]*RegionComplianceStatus)
	for _, asset := range m.assets {
		rs, ok := regionStats[asset.Region]
		if !ok {
			rs = &RegionComplianceStatus{Region: asset.Region}
			regionStats[asset.Region] = rs
		}
		rs.TotalAssets++
		compliant := len(m.config.AllowedRegions) == 0
		for _, r := range m.config.AllowedRegions {
			if asset.Region == r {
				compliant = true
				break
			}
		}
		if compliant {
			rs.CompliantAssets++
		} else {
			rs.Violations++
		}
	}
	for _, rs := range regionStats {
		if rs.TotalAssets > 0 {
			rs.Score = float64(rs.CompliantAssets) / float64(rs.TotalAssets) * 100
		}
		report.RegionCompliance = append(report.RegionCompliance, *rs)
	}

	// 状态判定
	switch {
	case report.OverallScore >= 90:
		report.Status = "compliant"
	case report.OverallScore >= 60:
		report.Status = "partial"
	default:
		report.Status = "non_compliant"
	}

	m.reports[report.ID] = report
	return report
}

type complianceCheck struct {
	passed         bool
	score          float64
	category       string
	title          string
	description    string
	severity       string
	affectedAssets int
	remediation    string
}

func (m *Manager) evaluateCompliance(framework ComplianceFramework) []complianceCheck {
	var checks []complianceCheck

	// 数据分类覆盖检查
	totalAssets := len(m.assets)
	classifiedAssets := 0
	for _, a := range m.assets {
		if a.Sensitivity != "" {
			classifiedAssets++
		}
	}
	classifyScore := 0.0
	classifyPassed := false
	if totalAssets > 0 {
		classifyScore = float64(classifiedAssets) / float64(totalAssets) * 100
		classifyPassed = classifyScore >= 80
	} else {
		classifyPassed = true
		classifyScore = 100
	}
	checks = append(checks, complianceCheck{
		passed:         classifyPassed,
		score:          classifyScore,
		category:       "数据分类",
		title:          "数据分类覆盖率",
		description:    fmt.Sprintf("已分类 %d/%d 资产", classifiedAssets, totalAssets),
		severity:       "high",
		affectedAssets: totalAssets - classifiedAssets,
		remediation:    "启用自动分类或手动标记未分类数据",
	})

	// 数据驻留合规检查
	violations := 0
	for _, a := range m.assets {
		compliant := len(m.config.AllowedRegions) == 0
		for _, r := range m.config.AllowedRegions {
			if a.Region == r {
				compliant = true
				break
			}
		}
		if !compliant {
			violations++
		}
	}
	residencyScore := 100.0
	if totalAssets > 0 {
		residencyScore = float64(totalAssets-violations) / float64(totalAssets) * 100
	}
	checks = append(checks, complianceCheck{
		passed:         violations == 0,
		score:          residencyScore,
		category:       "数据驻留",
		title:          "数据驻留合规",
		description:    fmt.Sprintf("%d 个资产存在驻留违规", violations),
		severity:       "critical",
		affectedAssets: violations,
		remediation:    "将违规数据迁移到允许的地理区域",
	})

	// 保留策略覆盖检查
	withPolicy := 0
	for _, a := range m.assets {
		if a.PolicyID != "" {
			withPolicy++
		}
	}
	policyScore := 0.0
	policyPassed := false
	if totalAssets > 0 {
		policyScore = float64(withPolicy) / float64(totalAssets) * 100
		policyPassed = policyScore >= 90
	} else {
		policyPassed = true
		policyScore = 100
	}
	checks = append(checks, complianceCheck{
		passed:         policyPassed,
		score:          policyScore,
		category:       "保留策略",
		title:          "保留策略覆盖",
		description:    fmt.Sprintf("%d/%d 资产关联了保留策略", withPolicy, totalAssets),
		severity:       "medium",
		affectedAssets: totalAssets - withPolicy,
		remediation:    "为未关联策略的资产分配适当的保留策略",
	})

	// 审计追踪完整性
	auditScore := 100.0
	auditPassed := true
	if len(m.auditLog) == 0 && totalAssets > 0 {
		auditScore = 50
		auditPassed = false
	}
	checks = append(checks, complianceCheck{
		passed:      auditPassed,
		score:       auditScore,
		category:    "审计追踪",
		title:       "审计日志完整性",
		description: fmt.Sprintf("审计记录 %d 条", len(m.auditLog)),
		severity:    "high",
		remediation: "确保所有数据操作都被记录到审计日志中",
	})

	// 框架特定检查
	switch framework {
	case FrameworkGDPR, FrameworkPIPL:
		// GDPR/PIPL: 检查是否有绝密数据缺乏额外保护
		topSecretNoPolicy := 0
		for _, a := range m.assets {
			if a.Sensitivity == LevelTopSecret && a.PolicyID == "" {
				topSecretNoPolicy++
			}
		}
		checks = append(checks, complianceCheck{
			passed:         topSecretNoPolicy == 0,
			score:          100 - float64(topSecretNoPolicy)*10,
			category:       "数据保护",
			title:          "敏感数据策略覆盖",
			description:    fmt.Sprintf("%d 个绝密资产无保留策略", topSecretNoPolicy),
			severity:       "critical",
			affectedAssets: topSecretNoPolicy,
			remediation:    "为绝密数据配置保留策略和加密保护",
		})
	case FrameworkCCPA:
		// CCPA: 检查消费者数据删除能力
		checks = append(checks, complianceCheck{
			passed:      true,
			score:       100,
			category:    "消费者权利",
			title:       "数据删除能力",
			description: "系统支持数据删除请求",
			severity:    "high",
			remediation: "定期验证数据删除流程的有效性",
		})
	}

	return checks
}

// GetReport 获取合规报告.
func (m *Manager) GetReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rpt, ok := m.reports[id]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return rpt, nil
}

// ListReports 列出合规报告.
func (m *Manager) ListReports(framework ComplianceFramework, page, pageSize int) ([]ComplianceReport, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ComplianceReport
	for _, rpt := range m.reports {
		if framework == "" || rpt.Framework == framework {
			result = append(result, *rpt)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// ========== 数据血缘追踪 ==========

// AddLineage 添加血缘记录.
func (m *Manager) AddLineage(record LineageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.assets[record.AssetID]; !ok {
		return ErrRecordNotFound
	}

	record.ID = fmt.Sprintf("lineage-%d", time.Now().UnixNano())
	record.CreatedAt = time.Now()

	m.lineage[record.AssetID] = append(m.lineage[record.AssetID], &record)

	// 也记录到审计日志
	m.auditLog = append(m.auditLog, AuditRecord{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		UserID:    record.OperatorID,
		UserName:  record.OperatorName,
		Action:    ActionCreate,
		AssetID:   record.AssetID,
		AssetName: record.AssetName,
		Details:   fmt.Sprintf("lineage: %s %s from %s", record.AssetName, record.Relation, record.SourceAssetName),
		Result:    "success",
		RiskLevel: "low",
	})

	return nil
}

// GetLineage 获取资产的血缘记录.
func (m *Manager) GetLineage(assetID string) ([]LineageRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.assets[assetID]; !ok {
		return nil, ErrRecordNotFound
	}

	records := m.lineage[assetID]
	result := make([]LineageRecord, 0, len(records))
	for _, r := range records {
		result = append(result, *r)
	}
	return result, nil
}

// GetLineageUpstream 获取资产的完整上游血缘链.
func (m *Manager) GetLineageUpstream(assetID string) []LineageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var chain []LineageRecord
	visited := make(map[string]bool)
	m.walkUpstream(assetID, &chain, visited)
	return chain
}

func (m *Manager) walkUpstream(assetID string, chain *[]LineageRecord, visited map[string]bool) {
	if visited[assetID] {
		return
	}
	visited[assetID] = true

	records := m.lineage[assetID]
	for _, r := range records {
		*chain = append(*chain, *r)
		m.walkUpstream(r.SourceAssetID, chain, visited)
	}
}

// ========== 资产管理 ==========

// GetAsset 获取数据资产.
func (m *Manager) GetAsset(id string) (*DataAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[id]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return asset, nil
}

// ListAssets 列出数据资产.
func (m *Manager) ListAssets(sensitivity SensitivityLevel, region GeoRegion, page, pageSize int) ([]DataAsset, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []DataAsset
	for _, a := range m.assets {
		if sensitivity != "" && a.Sensitivity != sensitivity {
			continue
		}
		if region != "" && a.Region != region {
			continue
		}
		result = append(result, *a)
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// DeleteAsset 删除数据资产.
func (m *Manager) DeleteAsset(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[id]
	if !ok {
		return ErrRecordNotFound
	}

	m.auditLog = append(m.auditLog, AuditRecord{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Action:    ActionDelete,
		AssetID:   id,
		AssetName: asset.Name,
		Details:   "asset deleted",
		Result:    "success",
		RiskLevel: "medium",
	})

	delete(m.assets, id)
	delete(m.lineage, id)
	return nil
}

// ========== 统计概览 ==========

// GetStats 获取数据治理统计.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		Classifications:    make(map[SensitivityLevel]int),
		RegionDistribution: make(map[GeoRegion]int),
	}

	for _, a := range m.assets {
		stats.TotalAssets++
		stats.Classifications[a.Sensitivity]++
		stats.RegionDistribution[a.Region]++

		// 检查驻留违规
		compliant := len(m.config.AllowedRegions) == 0
		for _, r := range m.config.AllowedRegions {
			if a.Region == r {
				compliant = true
				break
			}
		}
		if !compliant {
			stats.ResidencyViolations++
		}

		// 检查待销毁
		if a.RetentionDeadline != nil && time.Now().After(*a.RetentionDeadline) {
			stats.PendingDestructions++
		}
	}

	stats.TotalPolicies = len(m.policies)
	for _, p := range m.policies {
		if p.Enabled {
			stats.ActivePolicies++
		}
	}

	stats.TotalAuditRecords = len(m.auditLog)
	stats.TotalLineageRecords = 0
	for _, records := range m.lineage {
		stats.TotalLineageRecords += len(records)
	}

	// 最近审计记录
	if len(m.auditLog) > 0 {
		start := len(m.auditLog) - 10
		if start < 0 {
			start = 0
		}
		stats.RecentAuditRecords = make([]AuditRecord, len(m.auditLog[start:]))
		copy(stats.RecentAuditRecords, m.auditLog[start:])
	}

	return stats
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}
