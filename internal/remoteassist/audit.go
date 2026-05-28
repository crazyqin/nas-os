// audit.go - 操作审计日志
package remoteassist

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditService 审计服务.
type AuditService struct {
	events    []*AuditEvent
	storagePath string
	mu        sync.RWMutex
}

// NewAuditService 创建审计服务.
func NewAuditService() *AuditService {
	return &AuditService{
		events:      make([]*AuditEvent, 0),
		storagePath: "/var/nas-os/remoteassist/audit",
	}
}

// LogEvent 记录事件.
func (s *AuditService) LogEvent(event *AuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	s.events = append(s.events, event)

	// 异步持久化
	go s.persistEvent(event)

	log.Printf("📋 审计事件: %s, 操作: %s, 用户: %s, 风险: %s",
		event.ID, event.Action, event.Username, event.RiskLevel)
}

// persistEvent 持久化事件.
func (s *AuditService) persistEvent(event *AuditEvent) {
	// 创建存储目录
	if err := os.MkdirAll(s.storagePath, 0755); err != nil {
		log.Printf("⚠️ 创建审计目录失败: %v", err)
		return
	}

	// 按日期组织文件
	dateStr := event.Timestamp.Format("2006-01-02")
	filePath := filepath.Join(s.storagePath, dateStr+".jsonl")

	// 追加写入
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("⚠️ 打开审计文件失败: %v", err)
		return
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("⚠️ 序列化审计事件失败: %v", err)
		return
	}

	data = append(data, '\n')
	if _, err := file.Write(data); err != nil {
		log.Printf("⚠️ 写入审计事件失败: %v", err)
	}
}

// LogSessionCreated 记录会话创建.
func (s *AuditService) LogSessionCreated(session *Session, userID string) {
	s.LogEvent(&AuditEvent{
		SessionID: session.ID,
		UserID:    userID,
		Action:    "session_created",
		Resource:  session.ID,
		Details: map[string]interface{}{
			"type":     session.Type,
			"host_id":  session.HostID,
			"guest_id": session.GuestID,
		},
		Status:    "success",
		RiskLevel: "low",
	})
}

// LogSessionActivated 记录会话激活.
func (s *AuditService) LogSessionActivated(session *Session, userID string) {
	s.LogEvent(&AuditEvent{
		SessionID: session.ID,
		UserID:    userID,
		Action:    "session_activated",
		Resource:  session.ID,
		Status:    "success",
		RiskLevel: "low",
	})
}

// LogSessionEnded 记录会话结束.
func (s *AuditService) LogSessionEnded(session *Session, userID string, reason string) {
	s.LogEvent(&AuditEvent{
		SessionID: session.ID,
		UserID:    userID,
		Action:    "session_ended",
		Resource:  session.ID,
		Details: map[string]interface{}{
			"reason":   reason,
			"duration": session.Duration,
		},
		Status:    "success",
		RiskLevel: "low",
	})
}

// LogScreenShareStarted 记录屏幕共享开始.
func (s *AuditService) LogScreenShareStarted(sessionID string, userID string) {
	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    "screen_share_started",
		Resource:  sessionID,
		Status:    "success",
		RiskLevel: "medium",
	})
}

// LogScreenShareStopped 记录屏幕共享停止.
func (s *AuditService) LogScreenShareStopped(sessionID string, userID string) {
	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    "screen_share_stopped",
		Resource:  sessionID,
		Status:    "success",
		RiskLevel: "low",
	})
}

// LogTerminalCommand 记录终端命令.
func (s *AuditService) LogTerminalCommand(sessionID string, userID string, command string, exitCode int) {
	riskLevel := "low"
	if exitCode != 0 {
		riskLevel = "medium"
	}

	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    "terminal_command",
		Resource:  sessionID,
		Details: map[string]interface{}{
			"command":   command,
			"exit_code": exitCode,
		},
		Status:    "success",
		RiskLevel: riskLevel,
	})
}

// LogFileTransfer 记录文件传输.
func (s *AuditService) LogFileTransfer(sessionID string, userID string, transfer *FileTransfer) {
	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    "file_transfer",
		Resource:  transfer.ID,
		Details: map[string]interface{}{
			"direction":  transfer.Direction,
			"file_name":  transfer.FileName,
			"file_size":  transfer.FileSize,
			"status":     transfer.Status,
		},
		Status:    "success",
		RiskLevel: "medium",
	})
}

// LogAuth 记录认证事件.
func (s *AuditService) LogAuth(username string, ip string, success bool, reason string) {
	status := "success"
	riskLevel := "low"
	if !success {
		status = "failure"
		riskLevel = "high"
	}

	s.LogEvent(&AuditEvent{
		UserID:    username,
		Username:  username,
		Action:    "authentication",
		Resource:  "auth",
		IPAddress: ip,
		Details: map[string]interface{}{
			"reason": reason,
		},
		Status:    status,
		RiskLevel: riskLevel,
	})
}

// LogPermissionChange 记录权限变更.
func (s *AuditService) LogPermissionChange(sessionID string, userID string, targetUser string, permission string) {
	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    "permission_change",
		Resource:  sessionID,
		Details: map[string]interface{}{
			"target_user": targetUser,
			"permission":  permission,
		},
		Status:    "success",
		RiskLevel: "high",
	})
}

// LogRecording 记录录制事件.
func (s *AuditService) LogRecording(sessionID string, userID string, action string) {
	s.LogEvent(&AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Action:    fmt.Sprintf("recording_%s", action),
		Resource:  sessionID,
		Status:    "success",
		RiskLevel: "medium",
	})
}

// QueryEvents 查询事件.
func (s *AuditService) QueryEvents(query *AuditQuery) ([]*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AuditEvent, 0)

	for _, event := range s.events {
		if !s.matchEvent(event, query) {
			continue
		}
		result = append(result, event)
	}

	// 应用分页
	if query.Offset > 0 && query.Offset < len(result) {
		result = result[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result, nil
}

// AuditQuery 审计查询.
type AuditQuery struct {
	SessionID string     `json:"session_id"` // 会话ID
	UserID    string     `json:"user_id"`    // 用户ID
	Action    string     `json:"action"`     // 操作类型
	RiskLevel string     `json:"risk_level"` // 风险级别
	StartTime *time.Time `json:"start_time"` // 开始时间
	EndTime   *time.Time `json:"end_time"`   // 结束时间
	Limit     int        `json:"limit"`      // 限制数量
	Offset    int        `json:"offset"`     // 偏移量
}

// matchEvent 匹配事件.
func (s *AuditService) matchEvent(event *AuditEvent, query *AuditQuery) bool {
	if query.SessionID != "" && event.SessionID != query.SessionID {
		return false
	}
	if query.UserID != "" && event.UserID != query.UserID {
		return false
	}
	if query.Action != "" && event.Action != query.Action {
		return false
	}
	if query.RiskLevel != "" && event.RiskLevel != query.RiskLevel {
		return false
	}
	if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
		return false
	}
	return true
}

// GetEventsBySession 获取会话事件.
func (s *AuditService) GetEventsBySession(sessionID string) []*AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AuditEvent, 0)
	for _, event := range s.events {
		if event.SessionID == sessionID {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByUser 获取用户事件.
func (s *AuditService) GetEventsByUser(userID string) []*AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AuditEvent, 0)
	for _, event := range s.events {
		if event.UserID == userID {
			result = append(result, event)
		}
	}
	return result
}

// GetHighRiskEvents 获取高风险事件.
func (s *AuditService) GetHighRiskEvents(limit int) []*AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AuditEvent, 0)
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].RiskLevel == "high" {
			result = append(result, s.events[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetAuditStats 获取审计统计.
func (s *AuditService) GetAuditStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_events":  len(s.events),
		"by_action":     make(map[string]int),
		"by_risk_level": make(map[string]int),
		"by_user":       make(map[string]int),
	}

	byAction := stats["by_action"].(map[string]int)
	byRisk := stats["by_risk_level"].(map[string]int)
	byUser := stats["by_user"].(map[string]int)

	for _, event := range s.events {
		byAction[event.Action]++
		byRisk[event.RiskLevel]++
		byUser[event.UserID]++
	}

	return stats
}

// LoadEventsFromFile 从文件加载事件.
func (s *AuditService) LoadEventsFromFile(filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开审计文件失败: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for decoder.More() {
		event := &AuditEvent{}
		if err := decoder.Decode(event); err != nil {
			log.Printf("⚠️ 解析审计事件失败: %v", err)
			continue
		}
		s.events = append(s.events, event)
	}

	log.Printf("✅ 从文件加载审计事件: %s", filePath)
	return nil
}

// LoadEventsFromDirectory 从目录加载事件.
func (s *AuditService) LoadEventsFromDirectory(dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("读取审计目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".jsonl" {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		if err := s.LoadEventsFromFile(filePath); err != nil {
			log.Printf("⚠️ 加载审计文件失败: %s, %v", filePath, err)
		}
	}

	return nil
}

// CleanupOldEvents 清理旧事件.
func (s *AuditService) CleanupOldEvents(retentionDays int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	cleaned := 0

	// 清理内存中的事件
	newEvents := make([]*AuditEvent, 0)
	for _, event := range s.events {
		if event.Timestamp.Before(cutoff) {
			cleaned++
		} else {
			newEvents = append(newEvents, event)
		}
	}
	s.events = newEvents

	// 清理文件
	files, err := os.ReadDir(s.storagePath)
	if err != nil {
		return cleaned, nil
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".jsonl" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(s.storagePath, file.Name())
			if err := os.Remove(filePath); err == nil {
				cleaned++
			}
		}
	}

	log.Printf("🧹 清理旧审计事件: %d 条", cleaned)
	return cleaned, nil
}
