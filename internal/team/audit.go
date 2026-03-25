// Package team 操作审计日志
package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu         sync.RWMutex
	logs       []*TeamAuditLog
	configPath string
	maxLogs    int
	stopChan   chan struct{}
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(configPath string) *AuditLogger {
	al := &AuditLogger{
		logs:       make([]*TeamAuditLog, 0),
		configPath: configPath,
		maxLogs:    100000, // 默认保留10万条日志
		stopChan:   make(chan struct{}),
	}
	
	// 加载配置
	if configPath != "" {
		al.loadConfig()
	}
	
	return al
}

// loadConfig 加载配置
func (al *AuditLogger) loadConfig() error {
	logPath := filepath.Join(al.configPath, "team_audit.json")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return nil
	}
	
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, &al.logs)
}

// saveConfig 保存配置
func (al *AuditLogger) saveConfig() error {
	if al.configPath == "" {
		return nil
	}
	
	if err := os.MkdirAll(al.configPath, 0750); err != nil {
		return err
	}
	
	logPath := filepath.Join(al.configPath, "team_audit.json")
	data, err := json.MarshalIndent(al.logs, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(logPath, data, 0600)
}

// Log 记录审计日志
func (al *AuditLogger) Log(entry *TeamAuditLog) {
	if entry == nil {
		return
	}
	
	al.mu.Lock()
	defer al.mu.Unlock()
	
	// 设置ID和时间戳
	if entry.ID == "" {
		entry.ID = generateID()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	
	// 添加日志
	al.logs = append(al.logs, entry)
	
	// 检查是否需要清理
	if len(al.logs) > al.maxLogs {
		// 保留后90%
		keepCount := int(float64(al.maxLogs) * 0.9)
		al.logs = al.logs[len(al.logs)-keepCount:]
	}
	
	// 异步保存
	go al.saveConfig()
}

// Query 查询审计日志
func (al *AuditLogger) Query(options AuditQueryOptions) []*TeamAuditLog {
	al.mu.RLock()
	defer al.mu.RUnlock()
	
	results := make([]*TeamAuditLog, 0)
	
	for _, log := range al.logs {
		// 应用过滤条件
		if options.TeamID != "" && log.TeamID != options.TeamID {
			continue
		}
		if options.UserID != "" && log.UserID != options.UserID {
			continue
		}
		if options.Action != "" && log.Action != options.Action {
			continue
		}
		if options.ResourceType != "" && log.ResourceType != options.ResourceType {
			continue
		}
		if options.ResourceID != "" && log.ResourceID != options.ResourceID {
			continue
		}
		if options.StartTime != nil && log.Timestamp.Before(*options.StartTime) {
			continue
		}
		if options.EndTime != nil && log.Timestamp.After(*options.EndTime) {
			continue
		}
		
		results = append(results, log)
	}
	
	// 应用分页
	if options.Offset > 0 && options.Offset < len(results) {
		results = results[options.Offset:]
	}
	if options.Limit > 0 && len(results) > options.Limit {
		results = results[:options.Limit]
	}
	
	return results
}

// GetTeamLogs 获取团队审计日志
func (al *AuditLogger) GetTeamLogs(teamID string, limit int) []*TeamAuditLog {
	return al.Query(AuditQueryOptions{
		TeamID: teamID,
		Limit:  limit,
	})
}

// GetUserLogs 获取用户审计日志
func (al *AuditLogger) GetUserLogs(userID string, limit int) []*TeamAuditLog {
	return al.Query(AuditQueryOptions{
		UserID: userID,
		Limit:  limit,
	})
}

// GetResourceLogs 获取资源审计日志
func (al *AuditLogger) GetResourceLogs(resourceType, resourceID string, limit int) []*TeamAuditLog {
	return al.Query(AuditQueryOptions{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Limit:        limit,
	})
}

// GetStats 获取审计统计
func (al *AuditLogger) GetStats() map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()
	
	// 按操作类型统计
	actionCounts := make(map[TeamAuditAction]int)
	// 按用户统计
	userCounts := make(map[string]int)
	// 今日统计
	today := time.Now().Truncate(24 * time.Hour)
	todayCount := 0
	
	for _, log := range al.logs {
		actionCounts[log.Action]++
		if log.UserID != "" {
			userCounts[log.UserID]++
		}
		if log.Timestamp.After(today) {
			todayCount++
		}
	}
	
	// 最近活跃用户
	topUsers := make([]map[string]interface{}, 0)
	for userID, count := range userCounts {
		if len(topUsers) < 10 {
			topUsers = append(topUsers, map[string]interface{}{
				"user_id": userID,
				"count":   count,
			})
		}
	}
	
	return map[string]interface{}{
		"total_logs":    len(al.logs),
		"today_logs":    todayCount,
		"action_counts": actionCounts,
		"top_users":     topUsers,
	}
}

// Export 导出审计日志
func (al *AuditLogger) Export(options AuditQueryOptions, format string) ([]byte, error) {
	logs := al.Query(options)
	
	switch format {
	case "json":
		return json.MarshalIndent(logs, "", "  ")
	case "jsonl":
		var result []byte
		for _, log := range logs {
			data, err := json.Marshal(log)
			if err != nil {
				return nil, err
			}
			result = append(result, data...)
			result = append(result, '\n')
		}
		return result, nil
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// CleanOldLogs 清理旧日志
func (al *AuditLogger) CleanOldLogs(retentionDays int) int {
	al.mu.Lock()
	defer al.mu.Unlock()
	
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	newLogs := make([]*TeamAuditLog, 0)
	removed := 0
	
	for _, log := range al.logs {
		if log.Timestamp.After(cutoff) {
			newLogs = append(newLogs, log)
		} else {
			removed++
		}
	}
	
	al.logs = newLogs
	al.saveConfig()
	
	return removed
}

// GetActionGroups 按操作分组统计
func (al *AuditLogger) GetActionGroups(teamID string, startTime, endTime time.Time) map[string]int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	
	groups := make(map[string]int)
	
	for _, log := range al.logs {
		if teamID != "" && log.TeamID != teamID {
			continue
		}
		if !startTime.IsZero() && log.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && log.Timestamp.After(endTime) {
			continue
		}
		
		// 分组
		var group string
		switch log.Action {
		case AuditTeamCreate, AuditTeamUpdate, AuditTeamDelete,
			AuditTeamMemberAdd, AuditTeamMemberRemove, AuditTeamMemberRole:
			group = "团队管理"
		case AuditFolderCreate, AuditFolderUpdate, AuditFolderDelete, AuditFolderMove:
			group = "文件夹操作"
		case AuditFileUpload, AuditFileDownload, AuditFileDelete, AuditFileMove, AuditFileCopy, AuditFileRename:
			group = "文件操作"
		case AuditShareCreate, AuditShareAccess, AuditShareRevoke:
			group = "分享操作"
		case AuditEditStart, AuditEditEnd, AuditEditSave, AuditEditConflict:
			group = "协同编辑"
		case AuditCommentCreate, AuditCommentUpdate, AuditCommentDelete:
			group = "评论操作"
		default:
			group = "其他"
		}
		
		groups[group]++
	}
	
	return groups
}

// GetDailyStats 获取每日统计
func (al *AuditLogger) GetDailyStats(days int) []map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()
	
	// 初始化每日统计
	now := time.Now()
	dailyStats := make([]map[string]interface{}, days)
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		dailyStats[i] = map[string]interface{}{
			"date":  day.Format("2006-01-02"),
			"count": 0,
		}
	}
	
	// 统计每日日志数
	for _, log := range al.logs {
		for i, stat := range dailyStats {
			day, _ := time.Parse("2006-01-02", stat["date"].(string))
			nextDay := day.Add(24 * time.Hour)
			if log.Timestamp.After(day) && log.Timestamp.Before(nextDay) {
				dailyStats[i]["count"] = dailyStats[i]["count"].(int) + 1
				break
			}
		}
	}
	
	return dailyStats
}