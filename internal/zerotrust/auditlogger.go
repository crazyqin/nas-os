// Package zerotrust - 审计日志器
package zerotrust

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// AuditLogger 零信任审计日志器.
type AuditLogger struct {
	mu       sync.RWMutex
	file     *os.File
	path     string
	buffer   []AuditEntry
	flushInt time.Duration
}

// AuditEntry 审计日志条目.
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"eventType"`
	SubjectID   string                 `json:"subjectId"`
	ResourceID  string                 `json:"resourceId,omitempty"`
	ResourceType string                `json:"resourceType,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Result      string                 `json:"result"`
	Details     map[string]interface{} `json:"details,omitempty"`
	SourceIP    string                 `json:"sourceIp,omitempty"`
	DeviceID    string                 `json:"deviceId,omitempty"`
	RuleID      string                 `json:"ruleId,omitempty"`
	SessionID   string                 `json:"sessionId,omitempty"`
	PolicyID    string                 `json:"policyId,omitempty"`
	PolicyName  string                 `json:"policyName,omitempty"`
}

// NewAuditLogger 创建审计日志器.
func NewAuditLogger(path string) (*AuditLogger, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	al := &AuditLogger{
		file:     file,
		path:     path,
		buffer:   make([]AuditEntry, 0, 100),
		flushInt: 5 * time.Second,
	}

	// 启动后台刷新
	go al.flushLoop()

	return al, nil
}

// flushLoop 后台刷新循环.
func (al *AuditLogger) flushLoop() {
	ticker := time.NewTicker(al.flushInt)
	defer ticker.Stop()

	for range ticker.C {
		al.flush()
	}
}

// flush 刷新缓冲区到文件.
func (al *AuditLogger) flush() {
	al.mu.Lock()
	if len(al.buffer) == 0 {
		al.mu.Unlock()
		return
	}

	entries := al.buffer
	al.buffer = make([]AuditEntry, 0, 100)
	al.mu.Unlock()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		al.file.Write(append(data, '\n'))
	}
}

// log 记录审计条目.
func (al *AuditLogger) log(entry AuditEntry) {
	entry.Timestamp = time.Now()
	al.mu.Lock()
	al.buffer = append(al.buffer, entry)
	al.mu.Unlock()
}

// LogAccessGranted 记录访问授权.
func (al *AuditLogger) LogAccessGranted(req *AccessRequest, ruleID, sessionID string) {
	al.log(AuditEntry{
		EventType:    "access_granted",
		SubjectID:    req.SubjectID,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Action:       req.Action,
		Result:       "allowed",
		RuleID:       ruleID,
		SessionID:    sessionID,
		SourceIP:     req.SourceIP,
		DeviceID:     req.DeviceID,
	})
}

// LogAccessDenied 记录访问拒绝.
func (al *AuditLogger) LogAccessDenied(req *AccessRequest, reason string) {
	al.log(AuditEntry{
		EventType:    "access_denied",
		SubjectID:    req.SubjectID,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Action:       req.Action,
		Result:       "denied",
		SourceIP:     req.SourceIP,
		DeviceID:     req.DeviceID,
		Details:      map[string]interface{}{"reason": reason},
	})
}

// LogSessionCreated 记录会话创建.
func (al *AuditLogger) LogSessionCreated(sessionID, subjectID string) {
	al.log(AuditEntry{
		EventType: "session_created",
		SubjectID: subjectID,
		SessionID: sessionID,
		Result:    "success",
	})
}

// LogSessionTerminated 记录会话终止.
func (al *AuditLogger) LogSessionTerminated(sessionID, reason string) {
	al.log(AuditEntry{
		EventType: "session_terminated",
		SessionID: sessionID,
		Result:    "terminated",
		Details:   map[string]interface{}{"reason": reason},
	})
}

// LogSessionRevoked 记录会话撤销.
func (al *AuditLogger) LogSessionRevoked(sessionID, subjectID string) {
	al.log(AuditEntry{
		EventType: "session_revoked",
		SubjectID: subjectID,
		SessionID: sessionID,
		Result:    "revoked",
	})
}

// LogPolicyViolation 记录策略违规.
func (al *AuditLogger) LogPolicyViolation(req *AccessRequest, policyID, reason string) {
	al.log(AuditEntry{
		EventType:    "policy_violation",
		SubjectID:    req.SubjectID,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Result:       "violation",
		PolicyID:     policyID,
		Details:      map[string]interface{}{"reason": reason},
	})
}

// LogPolicyChange 记录策略变更.
func (al *AuditLogger) LogPolicyChange(action, policyID, policyName string) {
	al.log(AuditEntry{
		EventType:  "policy_change",
		Result:     "success",
		Action:     action,
		PolicyID:   policyID,
		PolicyName: policyName,
	})
}

// LogAuditEvent 记录审计事件.
func (al *AuditLogger) LogAuditEvent(req *AccessRequest, policyID, reason string) {
	al.log(AuditEntry{
		EventType:    "audit_event",
		SubjectID:    req.SubjectID,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Action:       req.Action,
		Result:       "triggered",
		PolicyID:     policyID,
		Details:      map[string]interface{}{"reason": reason},
	})
}

// Close 关闭审计日志器.
func (al *AuditLogger) Close() error {
	al.flush()
	return al.file.Close()
}

// GetRecentEntries 获取最近的审计条目.
func (al *AuditLogger) GetRecentEntries(limit int) []AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if limit <= 0 || limit > len(al.buffer) {
		limit = len(al.buffer)
	}

	start := len(al.buffer) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AuditEntry, limit)
	copy(result, al.buffer[start:])
	return result
}

// GetLogs 获取审计日志（支持分页和过滤）.
func (al *AuditLogger) GetLogs(page, pageSize int, eventType, severity, subjectID, allowed string) map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()

	// 过滤条目
	var filtered []AuditEntry
	for _, entry := range al.buffer {
		if eventType != "" && entry.EventType != eventType {
			continue
		}
		if subjectID != "" && entry.SubjectID != subjectID {
			continue
		}
		if allowed != "" {
			if allowed == "true" && entry.Result != "allowed" {
				continue
			}
			if allowed == "false" && entry.Result == "allowed" {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	// 分页
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		start = 0
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	logs := filtered[start:end]
	if logs == nil {
		logs = []AuditEntry{}
	}

	return map[string]interface{}{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"totalPages": (total + pageSize - 1) / pageSize,
	}
}

// GetStats 获取审计统计.
func (al *AuditLogger) GetStats() map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := map[string]interface{}{
		"bufferSize": len(al.buffer),
		"path":       al.path,
	}

	// 统计各类型事件数量
	eventCounts := make(map[string]int)
	for _, entry := range al.buffer {
		eventCounts[entry.EventType]++
	}
	stats["eventCounts"] = eventCounts

	return stats
}
