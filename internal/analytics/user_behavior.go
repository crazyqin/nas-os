package analytics

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// UserBehaviorAnalyzer 用户行为分析器.
type UserBehaviorAnalyzer struct {
	mu           sync.RWMutex
	accessLogs   []AccessLog
	hotFiles     map[string]*HotFile
	userActivity map[string]*UserActivity
	maxLogs      int
}

// AccessLog 访问日志.
type AccessLog struct {
	Timestamp    time.Time `json:"timestamp"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	FilePath     string    `json:"filePath"`
	Action       string    `json:"action"` // read, write, delete, create
	BytesRead    uint64    `json:"bytesRead,omitempty"`
	BytesWritten uint64    `json:"bytesWritten,omitempty"`
}

// NewUserBehaviorAnalyzer 创建用户行为分析器.
func NewUserBehaviorAnalyzer(maxLogs int) *UserBehaviorAnalyzer {
	if maxLogs <= 0 {
		maxLogs = 10000
	}
	return &UserBehaviorAnalyzer{
		accessLogs:   make([]AccessLog, 0, maxLogs),
		hotFiles:     make(map[string]*HotFile),
		userActivity: make(map[string]*UserActivity),
		maxLogs:      maxLogs,
	}
}

// RecordAccess 记录访问.
func (uba *UserBehaviorAnalyzer) RecordAccess(log AccessLog) {
	uba.mu.Lock()
	defer uba.mu.Unlock()

	// 维护日志大小
	if len(uba.accessLogs) >= uba.maxLogs {
		uba.accessLogs = uba.accessLogs[1:]
	}
	uba.accessLogs = append(uba.accessLogs, log)

	// 更新热门文件
	if file, ok := uba.hotFiles[log.FilePath]; ok {
		file.AccessCount++
		file.LastAccessed = log.Timestamp
		if log.BytesRead > 0 {
			file.TotalBytes += log.BytesRead
		}
		// 添加用户（如果不存在）
		found := false
		for _, u := range file.Users {
			if u == log.Username {
				found = true
				break
			}
		}
		if !found {
			file.Users = append(file.Users, log.Username)
		}
	} else {
		uba.hotFiles[log.FilePath] = &HotFile{
			Path:         log.FilePath,
			AccessCount:  1,
			LastAccessed: log.Timestamp,
			TotalBytes:   log.BytesRead,
			Users:        []string{log.Username},
		}
	}

	// 更新用户活动
	if activity, ok := uba.userActivity[log.UserID]; ok {
		activity.AccessCount++
		activity.BytesRead += log.BytesRead
		activity.BytesWritten += log.BytesWritten
		activity.LastActive = log.Timestamp
	} else {
		uba.userActivity[log.UserID] = &UserActivity{
			UserID:       log.UserID,
			Username:     log.Username,
			AccessCount:  1,
			BytesRead:    log.BytesRead,
			BytesWritten: log.BytesWritten,
			LastActive:   log.Timestamp,
		}
	}
}

// Analyze 执行用户行为分析.
func (uba *UserBehaviorAnalyzer) Analyze() *UserBehavior {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	return &UserBehavior{
		Timestamp:      time.Now(),
		AccessPatterns: uba.calculateAccessPatterns(),
		HotFiles:       uba.getTopHotFiles(20),
		UsageTrend:     uba.calculateUsageTrend(),
		UserActivity:   uba.getUserActivityList(),
	}
}

// calculateAccessPatterns 计算访问模式.
func (uba *UserBehaviorAnalyzer) calculateAccessPatterns() []AccessPattern {
	patterns := make(map[string]*AccessPattern)

	for _, log := range uba.accessLogs {
		key := time.Now().Weekday().String() + "_" + time.Now().Format("15")

		if p, ok := patterns[key]; ok {
			p.AccessCount++
			p.BytesRead += log.BytesRead
			p.BytesWritten += log.BytesWritten
		} else {
			patterns[key] = &AccessPattern{
				Hour:         log.Timestamp.Hour(),
				DayOfWeek:    int(log.Timestamp.Weekday()),
				AccessCount:  1,
				BytesRead:    log.BytesRead,
				BytesWritten: log.BytesWritten,
			}
		}
	}

	result := make([]AccessPattern, 0, len(patterns))
	for _, p := range patterns {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].DayOfWeek != result[j].DayOfWeek {
			return result[i].DayOfWeek < result[j].DayOfWeek
		}
		return result[i].Hour < result[j].Hour
	})

	return result
}

// getTopHotFiles 获取热门文件.
func (uba *UserBehaviorAnalyzer) getTopHotFiles(limit int) []HotFile {
	files := make([]HotFile, 0, len(uba.hotFiles))
	for _, f := range uba.hotFiles {
		files = append(files, *f)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].AccessCount > files[j].AccessCount
	})

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	return files
}

// calculateUsageTrend 计算使用趋势.
func (uba *UserBehaviorAnalyzer) calculateUsageTrend() []UsageTrendPoint {
	// 按小时分组
	hourlyData := make(map[string]*UsageTrendPoint)
	hourlyUsers := make(map[string]map[string]bool)

	for _, log := range uba.accessLogs {
		hourKey := log.Timestamp.Format("2006-01-02 15")

		if point, ok := hourlyData[hourKey]; ok {
			point.AccessCount++
			point.DataTransfer += log.BytesRead + log.BytesWritten
		} else {
			hourlyData[hourKey] = &UsageTrendPoint{
				Timestamp:    log.Timestamp.Truncate(time.Hour),
				AccessCount:  1,
				DataTransfer: log.BytesRead + log.BytesWritten,
			}
			hourlyUsers[hourKey] = make(map[string]bool)
		}

		if _, ok := hourlyUsers[hourKey]; !ok {
			hourlyUsers[hourKey] = make(map[string]bool)
		}
		hourlyUsers[hourKey][log.UserID] = true
	}

	// 计算活跃用户数
	for key, users := range hourlyUsers {
		if point, ok := hourlyData[key]; ok {
			point.ActiveUsers = len(users)
		}
	}

	// 转换为切片并排序
	result := make([]UsageTrendPoint, 0, len(hourlyData))
	for _, point := range hourlyData {
		result = append(result, *point)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// getUserActivityList 获取用户活动列表.
func (uba *UserBehaviorAnalyzer) getUserActivityList() []UserActivity {
	result := make([]UserActivity, 0, len(uba.userActivity))
	for _, activity := range uba.userActivity {
		result = append(result, *activity)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].AccessCount > result[j].AccessCount
	})

	return result
}

// GetUserAccessHistory 获取用户访问历史.
func (uba *UserBehaviorAnalyzer) GetUserAccessHistory(userID string, limit int) []AccessLog {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	result := make([]AccessLog, 0)
	for i := len(uba.accessLogs) - 1; i >= 0; i-- {
		if uba.accessLogs[i].UserID == userID {
			result = append(result, uba.accessLogs[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// GetFileAccessHistory 获取文件访问历史.
func (uba *UserBehaviorAnalyzer) GetFileAccessHistory(filePath string, limit int) []AccessLog {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	result := make([]AccessLog, 0)
	for i := len(uba.accessLogs) - 1; i >= 0; i-- {
		if uba.accessLogs[i].FilePath == filePath {
			result = append(result, uba.accessLogs[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// GetAccessCountByTimeRange 获取指定时间范围内的访问次数.
func (uba *UserBehaviorAnalyzer) GetAccessCountByTimeRange(start, end time.Time) int64 {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	var count int64
	for _, log := range uba.accessLogs {
		if log.Timestamp.After(start) && log.Timestamp.Before(end) {
			count++
		}
	}
	return count
}

// GetMostActiveHours 获取最活跃时段.
func (uba *UserBehaviorAnalyzer) GetMostActiveHours(topN int) []HourActivity {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	hourlyCount := make(map[int]int64)
	for _, log := range uba.accessLogs {
		hourlyCount[log.Timestamp.Hour()]++
	}

	result := make([]HourActivity, 0, 24)
	for hour := 0; hour < 24; hour++ {
		result = append(result, HourActivity{
			Hour:   hour,
			Count:  hourlyCount[hour],
			Format: formatHour(hour),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if topN > 0 && len(result) > topN {
		result = result[:topN]
	}

	return result
}

// HourActivity 小时活动.
type HourActivity struct {
	Hour   int    `json:"hour"`
	Count  int64  `json:"count"`
	Format string `json:"format"`
}

// formatHour 格式化小时.
func formatHour(hour int) string {
	if hour == 0 {
		return "00:00-01:00"
	}
	return fmt.Sprintf("%02d:00-%02d:00", hour, hour+1)
}

// ClearHistory 清空历史记录.
func (uba *UserBehaviorAnalyzer) ClearHistory() {
	uba.mu.Lock()
	defer uba.mu.Unlock()

	uba.accessLogs = make([]AccessLog, 0, uba.maxLogs)
	uba.hotFiles = make(map[string]*HotFile)
	uba.userActivity = make(map[string]*UserActivity)
}

// GetStats 获取统计信息.
func (uba *UserBehaviorAnalyzer) GetStats() map[string]interface{} {
	uba.mu.RLock()
	defer uba.mu.RUnlock()

	return map[string]interface{}{
		"totalLogs":      len(uba.accessLogs),
		"uniqueFiles":    len(uba.hotFiles),
		"uniqueUsers":    len(uba.userActivity),
		"maxLogCapacity": uba.maxLogs,
	}
}
