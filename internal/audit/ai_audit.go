package audit

import (
	"context"
	"errors"
	"sync"
	"time"
)

// AIAuditLogger logs AI service operations
type AIAuditLogger struct {
	logs     map[string]*AuditLog
	storage  AuditStorage
	mu       sync.RWMutex
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Service     ServiceType            `json:"service"`
	Action      string                 `json:"action"`
	Model       string                 `json:"model,omitempty"`
	Request     map[string]interface{} `json:"request,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Tokens      int                    `json:"tokens"`
	Duration    int64                  `json:"duration_ms"`
	Timestamp   time.Time              `json:"timestamp"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	PrivacyFlag bool                   `json:"privacy_flag"`
}

// ServiceType defines AI service type
type ServiceType string

const (
	ServiceChat      ServiceType = "chat"
	ServiceEmbedding ServiceType = "embedding"
	ServiceFace      ServiceType = "face_recognition"
	ServiceOCR       ServiceType = "ocr"
	ServiceLLM       ServiceType = "llm"
)

// AuditStorage interface for audit persistence
type AuditStorage interface {
	Save(log *AuditLog) error
	Query(filter AuditFilter) ([]AuditLog, error)
	Delete(id string) error
}

// AuditFilter for querying logs
type AuditFilter struct {
	UserID    string
	Service   ServiceType
	StartTime time.Time
	EndTime   time.Time
	Success   bool
}

// NewAIAuditLogger creates a new audit logger
func NewAIAuditLogger(storage AuditStorage) *AIAuditLogger {
	return &AIAuditLogger{
		logs:    make(map[string]*AuditLog),
		storage: storage,
	}
}

// Log logs an AI service operation
func (l *AIAuditLogger) Log(ctx context.Context, entry *AuditLog) error {
	if entry.ID == "" {
		entry.ID = generateAuditID()
	}
	entry.Timestamp = time.Now()
	
	l.mu.Lock()
	l.logs[entry.ID] = entry
	l.mu.Unlock()
	
	// Persist to storage
	return l.storage.Save(entry)
}

// LogChat logs a chat/completion request
func (l *AIAuditLogger) LogChat(ctx context.Context, userID, model string, 
		tokens int, duration int64, success bool, err string) error {
	return l.Log(ctx, &AuditLog{
		UserID:   userID,
		Service:  ServiceChat,
		Action:   "generate",
		Model:    model,
		Tokens:   tokens,
		Duration: duration,
		Success:  success,
		Error:    err,
	})
}

// LogFaceRecognition logs face recognition operation
func (l *AIAuditLogger) LogFaceRecognition(ctx context.Context, userID, action string,
		imagePath string, personID string, success bool, err string) error {
	// 人脸识别涉及隐私，标记privacy_flag
	return l.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      action,
		Request:     map[string]interface{}{"image_path": imagePath},
		Response:    map[string]interface{}{"person_id": personID},
		PrivacyFlag: true, // 隐私敏感操作
		Success:     success,
		Error:       err,
	})
}

// Query retrieves audit logs
func (l *AIAuditLogger) Query(ctx context.Context, filter AuditFilter) ([]AuditLog, error) {
	// 从storage查询
	return l.storage.Query(filter)
}

// GetUserAuditHistory retrieves user's audit history
func (l *AIAuditLogger) GetUserAuditHistory(ctx context.Context, userID string, 
		limit int) ([]AuditLog, error) {
	filter := AuditFilter{
		UserID: userID,
	}
	
	logs, err := l.storage.Query(filter)
	if err != nil {
		return nil, err
	}
	
	// 按时间排序，取最近的记录
	if len(logs) > limit {
		logs = logs[:limit]
	}
	
	return logs, nil
}

// GetServiceStats retrieves service statistics
func (l *AIAuditLogger) GetServiceStats(ctx context.Context, service ServiceType,
		start, end time.Time) ServiceStats {
	filter := AuditFilter{
		Service:   service,
		StartTime: start,
		EndTime:   end,
	}
	
	logs, err := l.storage.Query(filter)
	if err != nil {
		return ServiceStats{}
	}
	
	stats := ServiceStats{
		Service:    service,
		StartTime:  start,
		EndTime:    end,
	}
	
	for _, log := range logs {
		stats.TotalRequests++
		stats.TotalTokens += log.Tokens
		stats.TotalDuration += log.Duration
		
		if log.Success {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		
		if log.PrivacyFlag {
			stats.PrivacyOperations++
		}
	}
	
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRequests) * 100
	}
	
	return stats
}

// ServiceStats holds service statistics
type ServiceStats struct {
	Service          ServiceType `json:"service"`
	StartTime        time.Time   `json:"start_time"`
	EndTime          time.Time   `json:"end_time"`
	TotalRequests    int         `json:"total_requests"`
	SuccessCount     int         `json:"success_count"`
	ErrorCount       int         `json:"error_count"`
	TotalTokens      int         `json:"total_tokens"`
	TotalDuration    int64       `json:"total_duration_ms"`
	PrivacyOperations int        `json:"privacy_operations"`
	SuccessRate      float64     `json:"success_rate"`
}

// FacePrivacyAudit specifically for face recognition privacy compliance
type FacePrivacyAudit struct {
	logger *AIAuditLogger
}

// NewFacePrivacyAudit creates face privacy auditor
func NewFacePrivacyAudit(logger *AIAuditLogger) *FacePrivacyAudit {
	return &FacePrivacyAudit{logger: logger}
}

// LogFaceDetection logs face detection
func (f *FacePrivacyAudit) LogFaceDetection(ctx context.Context, userID, imagePath string,
		numFaces int, success bool, err string) error {
	return f.logger.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      "detect",
		Request:     map[string]interface{}{"image_path": imagePath},
		Response:    map[string]interface{}{"num_faces": numFaces},
		PrivacyFlag: true,
		Success:     success,
		Error:       err,
	})
}

// LogPersonIdentification logs person identification
func (f *FacePrivacyAudit) LogPersonIdentification(ctx context.Context, userID, faceID string,
		personID string, confidence float64, success bool, err string) error {
	return f.logger.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      "identify",
		Request:     map[string]interface{}{"face_id": faceID},
		Response:    map[string]interface{}{"person_id": personID, "confidence": confidence},
		PrivacyFlag: true,
		Success:     success,
		Error:       err,
	})
}

// LogPersonCreation logs new person creation
func (f *FacePrivacyAudit) LogPersonCreation(ctx context.Context, userID, personID, name string,
		success bool, err string) error {
	return f.logger.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      "create_person",
		Response:    map[string]interface{}{"person_id": personID, "name": name},
		PrivacyFlag: true,
		Success:     success,
		Error:       err,
	})
}

// LogPersonMerge logs person merge operation
func (f *FacePrivacyAudit) LogPersonMerge(ctx context.Context, userID, sourceID, targetID string,
		success bool, err string) error {
	return f.logger.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      "merge_person",
		Request:     map[string]interface{}{"source_id": sourceID},
		Response:    map[string]interface{}{"target_id": targetID},
		PrivacyFlag: true,
		Success:     success,
		Error:       err,
	})
}

// LogPersonDeletion logs person deletion
func (f *FacePrivacyAudit) LogPersonDeletion(ctx context.Context, userID, personID string,
		success bool, err string) error {
	return f.logger.Log(ctx, &AuditLog{
		UserID:      userID,
		Service:     ServiceFace,
		Action:      "delete_person",
		Request:     map[string]interface{}{"person_id": personID},
		PrivacyFlag: true,
		Success:     success,
		Error:       err,
	})
}

// CheckCompliance checks privacy compliance
func (f *FacePrivacyAudit) CheckCompliance(ctx context.Context, userID string) ComplianceReport {
	// 检查用户的人脸识别操作合规性
	filter := AuditFilter{
		UserID: userID,
		Service: ServiceFace,
	}
	
	logs, err := f.logger.storage.Query(filter)
	if err != nil {
		return ComplianceReport{Compliant: true}
	}
	
	report := ComplianceReport{
		UserID:         userID,
		TotalOperations: len(logs),
		Compliant:       true,
	}
	
	for _, log := range logs {
		if !log.PrivacyFlag {
			// 人脸操作未标记隐私，不合规
			report.Compliant = false
			report.Issues = append(report.Issues, 
				"face operation not marked as privacy-sensitive: " + log.ID)
		}
	}
	
	return report
}

// ComplianceReport holds compliance check results
type ComplianceReport struct {
	UserID          string   `json:"user_id"`
	TotalOperations int      `json:"total_operations"`
	Compliant        bool     `json:"compliant"`
	Issues          []string `json:"issues,omitempty"`
}

// Helper
func generateAuditID() string {
	return "audit_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	return "00000000" // placeholder
}

// Errors
var (
	ErrAuditNotFound = errors.New("audit log not found")
)