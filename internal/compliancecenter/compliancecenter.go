// Package compliancecenter 实现数据合规检查器
// 学习 TrueNAS 高级合规功能，提供数据分类、隐私保护、合规审计
package compliancecenter

import (
	"fmt"
	"sync"
	"time"
)

// ComplianceStandard 合规标准
type ComplianceStandard string

const (
	// StandardGDPR GDPR通用数据保护条例
	StandardGDPR ComplianceStandard = "GDPR"
	// StandardCCPA CCPA加州消费者隐私法案
	StandardCCPA ComplianceStandard = "CCPA"
	// StandardHIPAA HIPAA健康保险可携性和责任法案
	StandardHIPAA ComplianceStandard = "HIPAA"
	// StandardSOX SOX萨班斯-奥克斯利法案
	StandardSOX ComplianceStandard = "SOX"
	// StandardPCIDSS PCI-DSS支付卡行业数据安全标准
	StandardPCIDSS ComplianceStandard = "PCI-DSS"
	// StandardISO27001 ISO27001信息安全管理体系
	StandardISO27001 ComplianceStandard = "ISO27001"
	// StandardMLPS2 等保2.0
	StandardMLPS2 ComplianceStandard = "MLPS2.0"
)

// DataClassification 数据分类
type DataClassification string

const (
	// DataPublic 公开数据
	DataPublic DataClassification = "public"
	// DataInternal 内部数据
	DataInternal DataClassification = "internal"
	// DataConfidential 机密数据
	DataConfidential DataClassification = "confidential"
	// DataRestricted 受限数据
	DataRestricted DataClassification = "restricted"
	// DataTopSecret 绝密数据
	DataTopSecret DataClassification = "top_secret"
)

// SensitiveDataType 敏感数据类型
type SensitiveDataType string

const (
	// SensitivePII 个人身份信息
	SensitivePII SensitiveDataType = "PII"
	// SensitivePHI 受保护健康信息
	SensitivePHI SensitiveDataType = "PHI"
	// SensitivePCI 支付卡信息
	SensitivePCI SensitiveDataType = "PCI"
	// SensitiveFinancial 财务信息
	SensitiveFinancial SensitiveDataType = "financial"
	// SensitiveCredential 凭据信息
	SensitiveCredential SensitiveDataType = "credential"
	// SensitiveBiometric 生物识别信息
	SensitiveBiometric SensitiveDataType = "biometric"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	// ScanStatusPending 待扫描
	ScanStatusPending ScanStatus = "pending"
	// ScanStatusScanning 扫描中
	ScanStatusScanning ScanStatus = "scanning"
	// ScanStatusCompleted 完成
	ScanStatusCompleted ScanStatus = "completed"
	// ScanStatusFailed 失败
	ScanStatusFailed ScanStatus = "failed"
)

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	// ID 检查项ID
	ID string `json:"id"`
	// Standard 合规标准
	Standard ComplianceStandard `json:"standard"`
	// Category 分类
	Category string `json:"category"`
	// Name 名称
	Name string `json:"name"`
	// Description 描述
	Description string `json:"description"`
	// Requirement 要求
	Requirement string `json:"requirement"`
	// Status 状态
	Status string `json:"status"`
	// Score 分数 (0-100)
	Score float64 `json:"score"`
	// MaxScore 满分
	MaxScore float64 `json:"maxScore"`
	// Severity 严重程度
	Severity string `json:"severity"`
	// Evidence 证据
	Evidence []string `json:"evidence"`
	// Remediation 整改建议
	Remediation string `json:"remediation"`
	// LastChecked 上次检查时间
	LastChecked time.Time `json:"lastChecked"`
}

// DataScanResult 数据扫描结果
type DataScanResult struct {
	// ID 扫描ID
	ID string `json:"id"`
	// ScanTime 扫描时间
	ScanTime time.Time `json:"scanTime"`
	// TotalFiles 扫描文件总数
	TotalFiles int `json:"totalFiles"`
	// SensitiveFiles 敏感文件数
	SensitiveFiles int `json:"sensitiveFiles"`
	// TotalSize 总大小
	TotalSize int64 `json:"totalSize"`
	// SensitiveSize 敏感数据大小
	SensitiveSize int64 `json:"sensitiveSize"`
	// Findings 发现列表
	Findings []SensitiveFinding `json:"findings"`
	// Classification 分类统计
	Classification map[DataClassification]int `json:"classification"`
	// DataType 数据类型统计
	DataType map[SensitiveDataType]int `json:"dataType"`
	// Status 状态
	Status ScanStatus `json:"status"`
	// Duration 持续时间
	Duration time.Duration `json:"duration"`
}

// SensitiveFinding 敏感数据发现
type SensitiveFinding struct {
	// ID 发现ID
	ID string `json:"id"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// DataType 敏感数据类型
	DataType SensitiveDataType `json:"dataType"`
	// Classification 数据分类
	Classification DataClassification `json:"classification"`
	// MatchCount 匹配数量
	MatchCount int `json:"matchCount"`
	// Sample 样本（脱敏）
	Sample string `json:"sample"`
	// LineNumber 行号
	LineNumber int `json:"lineNumber"`
	// Context 上下文
	Context string `json:"context"`
	// Confidence 置信度 (0-100)
	Confidence float64 `json:"confidence"`
	// DetectedAt 检测时间
	DetectedAt time.Time `json:"detectedAt"`
	// Remediated 是否已整改
	Remediated bool `json:"remediated"`
	// RemediatedAt 整改时间
	RemediatedAt time.Time `json:"remediatedAt,omitempty"`
}

// PrivacyRequest 隐私请求
type PrivacyRequest struct {
	// ID 请求ID
	ID string `json:"id"`
	// Type 请求类型
	Type string `json:"type"` // access, deletion, portability, correction
	// UserID 用户ID
	UserID string `json:"userId"`
	// UserName 用户名
	UserName string `json:"userName"`
	// Email 邮箱
	Email string `json:"email"`
	// Description 描述
	Description string `json:"description"`
	// Status 状态
	Status string `json:"status"` // pending, processing, completed, rejected
	// SubmittedAt 提交时间
	SubmittedAt time.Time `json:"submittedAt"`
	// ProcessedAt 处理时间
	ProcessedAt time.Time `json:"processedAt,omitempty"`
	// CompletedAt 完成时间
	CompletedAt time.Time `json:"completedAt,omitempty"`
	// AssignedTo 指派人
	AssignedTo string `json:"assignedTo"`
	// Notes 备注
	Notes string `json:"notes"`
}

// AuditEvent 审计事件
type AuditEvent struct {
	// ID 事件ID
	ID string `json:"id"`
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// UserID 用户ID
	UserID string `json:"userId"`
	// UserName 用户名
	UserName string `json:"userName"`
	// Action 操作
	Action string `json:"action"`
	// Resource 资源
	Resource string `json:"resource"`
	// ResourceID 资源ID
	ResourceID string `json:"resourceId"`
	// Details 详情
	Details string `json:"details"`
	// IPAddress IP地址
	IPAddress string `json:"ipAddress"`
	// UserAgent 用户代理
	UserAgent string `json:"userAgent"`
	// Result 结果
	Result string `json:"result"`
	// RiskLevel 风险等级
	RiskLevel string `json:"riskLevel"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	// ID 报告ID
	ID string `json:"id"`
	// Standard 合规标准
	Standard ComplianceStandard `json:"standard"`
	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generatedAt"`
	// ValidUntil 有效期至
	ValidUntil time.Time `json:"validUntil"`
	// OverallScore 总分
	OverallScore float64 `json:"overallScore"`
	// MaxScore 满分
	MaxScore float64 `json:"maxScore"`
	// Status 状态
	Status string `json:"status"`
	// TotalChecks 总检查数
	TotalChecks int `json:"totalChecks"`
	// PassedChecks 通过数
	PassedChecks int `json:"passedChecks"`
	// FailedChecks 失败数
	FailedChecks int `json:"failedChecks"`
	// Findings 发现项
	Findings []ComplianceCheck `json:"findings"`
	// Recommendations 建议
	Recommendations []string `json:"recommendations"`
}

// ComplianceCenter 合规中心
type ComplianceCenter struct {
	mu       sync.RWMutex
	checks   map[string]*ComplianceCheck
	scans    map[string]*DataScanResult
	requests map[string]*PrivacyRequest
	auditLog []AuditEvent
	reports  map[string]*ComplianceReport
}

// NewComplianceCenter 创建合规中心
func NewComplianceCenter() *ComplianceCenter {
	return &ComplianceCenter{
		checks:   make(map[string]*ComplianceCheck),
		scans:    make(map[string]*DataScanResult),
		requests: make(map[string]*PrivacyRequest),
		reports:  make(map[string]*ComplianceReport),
	}
}

// AddCheck 添加检查项
func (cc *ComplianceCenter) AddCheck(check ComplianceCheck) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.checks[check.ID] = &check
	return nil
}

// UpdateCheck 更新检查项
func (cc *ComplianceCenter) UpdateCheck(check ComplianceCheck) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.checks[check.ID] = &check
	return nil
}

// GetCheck 获取检查项
func (cc *ComplianceCenter) GetCheck(checkID string) (*ComplianceCheck, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	check, ok := cc.checks[checkID]
	if !ok {
		return nil, fmt.Errorf("check not found: %s", checkID)
	}

	return check, nil
}

// ListChecks 列出检查项
func (cc *ComplianceCenter) ListChecks(standard ComplianceStandard) []*ComplianceCheck {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	checks := make([]*ComplianceCheck, 0)
	for _, check := range cc.checks {
		if standard == "" || check.Standard == standard {
			checks = append(checks, check)
		}
	}
	return checks
}

// StartScan 开始数据扫描
func (cc *ComplianceCenter) StartScan(scanID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	scan, ok := cc.scans[scanID]
	if !ok {
		return fmt.Errorf("scan not found: %s", scanID)
	}

	scan.Status = ScanStatusScanning
	return nil
}

// CompleteScan 完成数据扫描
func (cc *ComplianceCenter) CompleteScan(scanID string, result DataScanResult) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	scan, ok := cc.scans[scanID]
	if !ok {
		return fmt.Errorf("scan not found: %s", scanID)
	}

	scan.Status = ScanStatusCompleted
	scan.TotalFiles = result.TotalFiles
	scan.SensitiveFiles = result.SensitiveFiles
	scan.TotalSize = result.TotalSize
	scan.SensitiveSize = result.SensitiveSize
	scan.Findings = result.Findings
	scan.Classification = result.Classification
	scan.DataType = result.DataType
	scan.Duration = result.Duration
	return nil
}

// GetScan 获取扫描结果
func (cc *ComplianceCenter) GetScan(scanID string) (*DataScanResult, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	scan, ok := cc.scans[scanID]
	if !ok {
		return nil, fmt.Errorf("scan not found: %s", scanID)
	}

	return scan, nil
}

// ListScans 列出扫描结果
func (cc *ComplianceCenter) ListScans(status ScanStatus) []*DataScanResult {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	scans := make([]*DataScanResult, 0)
	for _, scan := range cc.scans {
		if status == "" || scan.Status == status {
			scans = append(scans, scan)
		}
	}
	return scans
}

// SubmitRequest 提交隐私请求
func (cc *ComplianceCenter) SubmitRequest(request PrivacyRequest) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	request.Status = "pending"
	request.SubmittedAt = time.Now()
	cc.requests[request.ID] = &request
	return nil
}

// ProcessRequest 处理隐私请求
func (cc *ComplianceCenter) ProcessRequest(requestID string, assignedTo string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	request, ok := cc.requests[requestID]
	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	request.Status = "processing"
	request.AssignedTo = assignedTo
	request.ProcessedAt = time.Now()
	return nil
}

// CompleteRequest 完成隐私请求
func (cc *ComplianceCenter) CompleteRequest(requestID string, notes string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	request, ok := cc.requests[requestID]
	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	request.Status = "completed"
	request.CompletedAt = time.Now()
	request.Notes = notes
	return nil
}

// GetRequest 获取隐私请求
func (cc *ComplianceCenter) GetRequest(requestID string) (*PrivacyRequest, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	request, ok := cc.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", requestID)
	}

	return request, nil
}

// ListRequests 列出隐私请求
func (cc *ComplianceCenter) ListRequests(status string) []*PrivacyRequest {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	requests := make([]*PrivacyRequest, 0)
	for _, request := range cc.requests {
		if status == "" || request.Status == status {
			requests = append(requests, request)
		}
	}
	return requests
}

// LogAuditEvent 记录审计事件
func (cc *ComplianceCenter) LogAuditEvent(event AuditEvent) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	event.Timestamp = time.Now()
	cc.auditLog = append(cc.auditLog, event)

	// 限制日志数量
	if len(cc.auditLog) > 10000 {
		cc.auditLog = cc.auditLog[len(cc.auditLog)-5000:]
	}
}

// GetAuditLog 获取审计日志
func (cc *ComplianceCenter) GetAuditLog(userID string, limit int) []AuditEvent {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	logs := make([]AuditEvent, 0)
	for _, log := range cc.auditLog {
		if userID == "" || log.UserID == userID {
			logs = append(logs, log)
		}
	}

	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	return logs
}

// GenerateReport 生成合规报告
func (cc *ComplianceCenter) GenerateReport(standard ComplianceStandard) (*ComplianceReport, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("rpt-%d", time.Now().UnixNano()),
		Standard:    standard,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(30 * 24 * time.Hour),
	}

	totalScore := 0.0
	maxScore := 0.0
	for _, check := range cc.checks {
		if standard != "" && check.Standard != standard {
			continue
		}
		report.TotalChecks++
		totalScore += check.Score
		maxScore += check.MaxScore

		if check.Score >= check.MaxScore*0.8 {
			report.PassedChecks++
		} else {
			report.FailedChecks++
			report.Findings = append(report.Findings, *check)
		}
	}

	report.OverallScore = totalScore
	report.MaxScore = maxScore

	if maxScore > 0 {
		percentage := totalScore / maxScore * 100
		if percentage >= 90 {
			report.Status = "compliant"
		} else if percentage >= 70 {
			report.Status = "partially_compliant"
		} else {
			report.Status = "non_compliant"
		}
	}

	cc.reports[report.ID] = report
	return report, nil
}

// GetReport 获取合规报告
func (cc *ComplianceCenter) GetReport(reportID string) (*ComplianceReport, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	report, ok := cc.reports[reportID]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}

	return report, nil
}

// ListReports 列出合规报告
func (cc *ComplianceCenter) ListReports(standard ComplianceStandard) []*ComplianceReport {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	reports := make([]*ComplianceReport, 0)
	for _, report := range cc.reports {
		if standard == "" || report.Standard == standard {
			reports = append(reports, report)
		}
	}
	return reports
}
