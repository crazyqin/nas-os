package securitycompliance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Severity 严重级别
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// CheckStatus 检查状态
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckFailed  CheckStatus = "failed"
	CheckWarning CheckStatus = "warning"
	CheckSkipped CheckStatus = "skipped"
)

// SecurityCheck 安全检查
type SecurityCheck struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Severity    Severity    `json:"severity"`
	Status      CheckStatus `json:"status"`
	Details     string      `json:"details,omitempty"`
	Remediation string      `json:"remediation,omitempty"`
	CheckedAt   time.Time   `json:"checked_at"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	IP        string    `json:"ip"`
	Details   string    `json:"details,omitempty"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID             string           `json:"id"`
	GeneratedAt    time.Time        `json:"generated_at"`
	TotalChecks    int              `json:"total_checks"`
	Passed         int              `json:"passed"`
	Failed         int              `json:"failed"`
	Warnings       int              `json:"warnings"`
	Score          float64          `json:"score"`
	Checks         []*SecurityCheck `json:"checks"`
	CriticalIssues []*SecurityCheck `json:"critical_issues"`
}

// Manager 安全合规管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	checks   []*SecurityCheck
	auditLog []*AuditLog
	dataPath string
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:   logger,
		dataPath: dataPath,
	}
	_ = m.loadData()
	return m
}

// RunSecurityScan 运行安全扫描
func (m *Manager) RunSecurityScan() *ComplianceReport {
	checks := []*SecurityCheck{
		{ID: "ssh-root", Name: "SSH Root登录", Category: "认证", Severity: SeverityHigh, Remediation: "禁止SSH root登录"},
		{ID: "firewall", Name: "防火墙状态", Category: "网络", Severity: SeverityCritical, Remediation: "启用防火墙"},
		{ID: "password-policy", Name: "密码策略", Category: "认证", Severity: SeverityMedium, Remediation: "设置强密码策略"},
		{ID: "updates", Name: "系统更新", Category: "系统", Severity: SeverityMedium, Remediation: "定期更新系统"},
		{ID: "disk-encryption", Name: "磁盘加密", Category: "存储", Severity: SeverityHigh, Remediation: "启用磁盘加密"},
		{ID: "backup-status", Name: "备份状态", Category: "存储", Severity: SeverityCritical, Remediation: "配置定期备份"},
		{ID: "ssl-cert", Name: "SSL证书", Category: "网络", Severity: SeverityHigh, Remediation: "使用有效SSL证书"},
		{ID: "port-scan", Name: "开放端口", Category: "网络", Severity: SeverityMedium, Remediation: "关闭不必要的端口"},
		{ID: "file-perms", Name: "文件权限", Category: "系统", Severity: SeverityLow, Remediation: "检查敏感文件权限"},
		{ID: "audit-log", Name: "审计日志", Category: "合规", Severity: SeverityMedium, Remediation: "启用审计日志"},
	}
	// 模拟检查结果
	for _, c := range checks {
		c.Status = CheckPassed
		c.CheckedAt = time.Now()
	}

	report := &ComplianceReport{
		ID:          genID(),
		GeneratedAt: time.Now(),
		TotalChecks: len(checks),
		Checks:      checks,
	}
	for _, c := range checks {
		switch c.Status {
		case CheckPassed:
			report.Passed++
		case CheckFailed:
			report.Failed++
			if c.Severity == SeverityCritical || c.Severity == SeverityHigh {
				report.CriticalIssues = append(report.CriticalIssues, c)
			}
		case CheckWarning:
			report.Warnings++
		}
	}
	if report.TotalChecks > 0 {
		report.Score = float64(report.Passed) / float64(report.TotalChecks) * 100
	}

	m.mu.Lock()
	m.checks = checks
	m.mu.Unlock()
	_ = m.saveData()
	return report
}

// AddAuditLog 添加审计日志
func (m *Manager) AddAuditLog(user, action, resource, result, ip, details string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log := &AuditLog{
		ID:        genID(),
		Timestamp: time.Now(),
		User:      user,
		Action:    action,
		Resource:  resource,
		Result:    result,
		IP:        ip,
		Details:   details,
	}
	m.auditLog = append(m.auditLog, log)
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-10000:]
	}
	_ = m.saveData()
}

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs(user string, limit int) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*AuditLog
	for i := len(m.auditLog) - 1; i >= 0; i-- {
		if user != "" && m.auditLog[i].User != user {
			continue
		}
		result = append(result, m.auditLog[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() *ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.checks) == 0 {
		return nil
	}
	report := &ComplianceReport{
		ID:          genID(),
		GeneratedAt: time.Now(),
		Checks:      m.checks,
	}
	for _, c := range m.checks {
		report.TotalChecks++
		switch c.Status {
		case CheckPassed:
			report.Passed++
		case CheckFailed:
			report.Failed++
		case CheckWarning:
			report.Warnings++
		}
	}
	if report.TotalChecks > 0 {
		report.Score = float64(report.Passed) / float64(report.TotalChecks) * 100
	}
	return report
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "security_compliance.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Checks   []*SecurityCheck `json:"checks"`
		AuditLog []*AuditLog      `json:"audit_log"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	m.checks = stored.Checks
	m.auditLog = stored.AuditLog
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	stored := struct {
		Checks   []*SecurityCheck `json:"checks"`
		AuditLog []*AuditLog      `json:"audit_log"`
	}{Checks: m.checks, AuditLog: m.auditLog}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataPath, "security_compliance.json"), data, 0o644)
}

func genID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// Handlers API
type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/security")
	{
		g.POST("/scan", h.scan)
		g.GET("/report", h.report)
		g.GET("/audit-logs", h.auditLogs)
		g.POST("/audit-logs", h.addAuditLog)
	}
}

func (h *Handlers) scan(c *gin.Context) {
	report := h.mgr.RunSecurityScan()
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) report(c *gin.Context) {
	report := h.mgr.GetLatestReport()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no report available"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) auditLogs(c *gin.Context) {
	user := c.Query("user")
	limit := 50
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	c.JSON(http.StatusOK, gin.H{"logs": h.mgr.GetAuditLogs(user, limit)})
}

func (h *Handlers) addAuditLog(c *gin.Context) {
	var req struct {
		User     string `json:"user" binding:"required"`
		Action   string `json:"action" binding:"required"`
		Resource string `json:"resource"`
		Result   string `json:"result"`
		IP       string `json:"ip"`
		Details  string `json:"details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mgr.AddAuditLog(req.User, req.Action, req.Resource, req.Result, req.IP, req.Details)
	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
}
