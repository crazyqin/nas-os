// Package snapshotaudit - 合规快照审计管理器
package snapshotaudit

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Manager 快照审计管理器
type Manager struct {
	mu        sync.RWMutex
	snapshots map[string]*SnapshotRecord
	policies  map[string]*AuditPolicy
	reports   []*AuditReport
	logs      []*AuditLog
	stats     *AuditStats
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		snapshots: make(map[string]*SnapshotRecord),
		policies:  make(map[string]*AuditPolicy),
		reports:   make([]*AuditReport, 0),
		logs:      make([]*AuditLog, 0),
		stats:     &AuditStats{StatusBreakdown: make(map[string]int)},
	}
}

// RegisterSnapshot 注册快照
func (m *Manager) RegisterSnapshot(snap *SnapshotRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if snap.ID == "" {
		snap.ID = fmt.Sprintf("snap-%d", time.Now().UnixNano())
	}
	// 计算哈希
	if snap.Hash == "" {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%d", snap.Name, snap.Volume, snap.CreatedAt.Unix())))
		snap.Hash = fmt.Sprintf("%x", h)
	}
	snap.CreatedAt = time.Now()
	m.snapshots[snap.ID] = snap
	m.stats.TotalSnapshots++
	m.addLog("register", snap.ID, "", "快照注册: "+snap.Name)
	return nil
}

// CreatePolicy 创建审计策略
func (m *Manager) CreatePolicy(policy *AuditPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("audit-policy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// RunAudit 运行审计
func (m *Manager) RunAudit(policyID string) (*AuditReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	start := time.Now()
	report := &AuditReport{
		ID:         fmt.Sprintf("audit-%d", start.UnixNano()),
		PolicyID:   policyID,
		PolicyName: policy.Name,
		Status:     StatusRunning,
		StartTime:  start,
	}

	// 检查所有快照
	validCount := 0
	failedCount := 0
	warningCount := 0
	issues := make([]*AuditIssue, 0)

	for _, snap := range m.snapshots {
		// 完整性检查
		if policy.CheckIntegrity {
			if snap.Status == SnapshotCorrupted || snap.Status == SnapshotTampered {
				issues = append(issues, &AuditIssue{
					SnapshotID:   snap.ID,
					SnapshotName: snap.Name,
					Severity:     "critical",
					Code:         "INTEGRITY_FAIL",
					Message:      fmt.Sprintf("快照 %s 完整性校验失败", snap.Name),
					Suggestion:   "建议重新创建快照或从备份恢复",
				})
				failedCount++
				continue
			}
		}

		// 保留期检查
		if policy.CheckRetention && snap.RetentionDays > 0 {
			age := time.Since(snap.CreatedAt).Hours() / 24
			if int(age) > snap.RetentionDays {
				issues = append(issues, &AuditIssue{
					SnapshotID:   snap.ID,
					SnapshotName: snap.Name,
					Severity:     "warning",
					Code:         "RETENTION_EXPIRED",
					Message:      fmt.Sprintf("快照 %s 已超过保留期 (%d天)", snap.Name, snap.RetentionDays),
					Suggestion:   "建议删除过期快照或延长保留期",
				})
				warningCount++
				continue
			}
		}

		validCount++
		snap.Verified = true
		now := time.Now()
		snap.VerifiedAt = &now
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)
	report.TotalSnapshots = len(m.snapshots)
	report.ValidCount = validCount
	report.FailedCount = failedCount
	report.WarningCount = warningCount
	report.Issues = issues

	if failedCount > 0 {
		report.Status = StatusFailed
		report.Result = ResultNonCompliant
	} else if warningCount > 0 {
		report.Status = StatusWarning
		report.Result = ResultCompliant
	} else {
		report.Status = StatusPassed
		report.Result = ResultCompliant
	}

	m.reports = append(m.reports, report)
	m.stats.TotalAudits++
	if report.Status == StatusPassed {
		m.stats.PassedAudits++
	} else {
		m.stats.FailedAudits++
	}
	m.stats.TotalIssues += len(issues)
	for _, issue := range issues {
		if issue.Severity == "critical" {
			m.stats.CriticalIssues++
		}
	}
	now := time.Now()
	m.stats.LastAuditTime = &now
	m.addLog("audit", policyID, "", fmt.Sprintf("审计完成: %d 通过, %d 失败, %d 警告", validCount, failedCount, warningCount))

	return report, nil
}

// ListSnapshots 列出快照
func (m *Manager) ListSnapshots() []*SnapshotRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snaps := make([]*SnapshotRecord, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		snaps = append(snaps, s)
	}
	return snaps
}

// GetReports 获取审计报告
func (m *Manager) GetReports() []*AuditReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reports
}

// GetLogs 获取审计日志
func (m *Manager) GetLogs() []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.logs
}

// GetStats 获取统计
func (m *Manager) GetStats() *AuditStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Manager) addLog(action, resource, user, details string) {
	m.logs = append(m.logs, &AuditLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Action:    action,
		Resource:  resource,
		User:      user,
		Details:   details,
		Timestamp: time.Now(),
	})
}
