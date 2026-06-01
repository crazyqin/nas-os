package ztaccess

import (
	"sync"
	"time"
)

// AuditManager manages audit logging
type AuditManager struct {
	zt        *ZTAccess
	mu        sync.RWMutex
	maxEntries int
}

// NewAuditManager creates a new audit manager
func NewAuditManager(zt *ZTAccess) *AuditManager {
	return &AuditManager{
		zt:         zt,
		maxEntries: 10000,
	}
}

// LogActivity logs an activity
func (am *AuditManager) LogActivity(userID, action, resource, result, ipAddress, userAgent, details string) {
	am.zt.mu.Lock()
	defer am.zt.mu.Unlock()

	entry := AuditEntry{
		EntryID:   generateSessionID(),
		Timestamp: time.Now(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   details,
	}

	am.zt.auditLog = append(am.zt.auditLog, entry)

	// Trim if exceeds max
	if len(am.zt.auditLog) > am.maxEntries {
		am.zt.auditLog = am.zt.auditLog[len(am.zt.auditLog)-am.maxEntries:]
	}
}

// GetAuditLog returns audit log entries
func (am *AuditManager) GetAuditLog(limit int, filters map[string]string) []AuditEntry {
	am.zt.mu.RLock()
	defer am.zt.mu.RUnlock()

	var filtered []AuditEntry

	for i := len(am.zt.auditLog) - 1; i >= 0; i-- {
		entry := am.zt.auditLog[i]

		// Apply filters
		if userID, ok := filters["user_id"]; ok && entry.UserID != userID {
			continue
		}
		if action, ok := filters["action"]; ok && entry.Action != action {
			continue
		}
		if result, ok := filters["result"]; ok && entry.Result != result {
			continue
		}

		filtered = append(filtered, entry)

		if len(filtered) >= limit {
			break
		}
	}

	return filtered
}

// GetAuditStats returns audit statistics
func (am *AuditManager) GetAuditStats() map[string]interface{} {
	am.zt.mu.RLock()
	defer am.zt.mu.RUnlock()

	total := len(am.zt.auditLog)
	successful := 0
	failed := 0

	for _, entry := range am.zt.auditLog {
		if entry.Result == "success" || entry.Result == "allowed" {
			successful++
		} else {
			failed++
		}
	}

	return map[string]interface{}{
		"total":      total,
		"successful": successful,
		"failed":     failed,
	}
}

// GetUserActivity returns activity for a specific user
func (am *AuditManager) GetUserActivity(userID string, limit int) []AuditEntry {
	am.zt.mu.RLock()
	defer am.zt.mu.RUnlock()

	var userEntries []AuditEntry

	for i := len(am.zt.auditLog) - 1; i >= 0; i-- {
		if am.zt.auditLog[i].UserID == userID {
			userEntries = append(userEntries, am.zt.auditLog[i])
			if len(userEntries) >= limit {
				break
			}
		}
	}

	return userEntries
}

// GetAnomalies returns detected anomalies
func (am *AuditManager) GetAnomalies(limit int) []AnomalyDetection {
	am.zt.mu.RLock()
	defer am.zt.mu.RUnlock()

	if limit > len(am.zt.anomalies) {
		limit = len(am.zt.anomalies)
	}

	result := make([]AnomalyDetection, limit)
	copy(result, am.zt.anomalies[len(am.zt.anomalies)-limit:])
	return result
}

// DetectAnomaly detects anomalies based on behavior
func (am *AuditManager) DetectAnomaly(session *Session) []AnomalyDetection {
	am.zt.mu.Lock()
	defer am.zt.mu.Unlock()

	var anomalies []AnomalyDetection

	// Check for rapid requests
	if len(session.ActivityLog) > 100 {
		recentActivities := session.ActivityLog[len(session.ActivityLog)-100:]
		timeWindow := recentActivities[len(recentActivities)-1].Timestamp.Sub(recentActivities[0].Timestamp)
		if timeWindow < 1*time.Minute {
			anomalies = append(anomalies, AnomalyDetection{
				UserID:      session.UserID,
				SessionID:   session.SessionID,
				AnomalyType: "rapid_requests",
				Severity:    "high",
				Description: "检测到异常快速请求",
				Timestamp:   time.Now(),
				DeviceInfo:  session.Device,
			})
		}
	}

	// Check for unusual access patterns
	accessedResources := make(map[string]int)
	for _, activity := range session.ActivityLog {
		accessedResources[activity.Resource]++
	}

	// If accessing many different resources, might be suspicious
	if len(accessedResources) > 50 {
		anomalies = append(anomalies, AnomalyDetection{
			UserID:      session.UserID,
			SessionID:   session.SessionID,
			AnomalyType: "resource_scanning",
			Severity:    "medium",
			Description: "检测到大量不同资源访问",
			Timestamp:   time.Now(),
			DeviceInfo:  session.Device,
		})
	}

	// Store anomalies
	am.zt.anomalies = append(am.zt.anomalies, anomalies...)

	return anomalies
}
