// Package quotaalert 提供存储配额智能预警核心管理逻辑
package quotaalert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrQuotaNotFound 配额规则不存在.
	ErrQuotaNotFound = errors.New("配额规则不存在")
	// ErrAlertNotFound 告警不存在.
	ErrAlertNotFound = errors.New("告警不存在")
	// ErrInvalidThreshold 阈值无效.
	ErrInvalidThreshold = errors.New("阈值必须在 0 到 1 之间")
	// ErrQuotaExceeded 配额已超限.
	ErrQuotaExceeded = errors.New("配额已超限")
)

// Manager 配额预警管理器.
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	storagePath string
	rules       map[string]*QuotaRule     // key: userID:path
	usages      map[string]*QuotaUsage    // key: userID:path
	alerts      map[string]*QuotaAlert    // key: alertID
	history     map[string][]UsageHistory // key: userID:path, 历史记录
}

// UsageHistory 使用量历史记录.
type UsageHistory struct {
	UsedBytes int64
	UsedFiles int64
	Timestamp time.Time
}

// NewManager 创建配额预警管理器.
func NewManager(storagePath string) *Manager {
	m := &Manager{
		logger:      zap.NewNop(),
		storagePath: storagePath,
		rules:       make(map[string]*QuotaRule),
		usages:      make(map[string]*QuotaUsage),
		alerts:      make(map[string]*QuotaAlert),
		history:     make(map[string][]UsageHistory),
	}

	// 确保存储目录存在
	if storagePath != "" {
		os.MkdirAll(storagePath, 0755)
	}

	return m
}

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ruleKey 生成规则存储 key.
func ruleKey(userID, path string) string {
	return userID + ":" + path
}

// SetQuota 设置配额规则.
func (m *Manager) SetQuota(ctx context.Context, rule QuotaRule) error {
	// 验证阈值
	if rule.WarnThreshold <= 0 || rule.WarnThreshold >= 1 {
		return ErrInvalidThreshold
	}
	if rule.CriticalThreshold <= 0 || rule.CriticalThreshold >= 1 {
		return ErrInvalidThreshold
	}
	if rule.WarnThreshold >= rule.CriticalThreshold {
		return errors.New("告警阈值必须小于严重告警阈值")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成 ID
	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()

	key := ruleKey(rule.UserID, rule.Path)
	m.rules[key] = &rule

	m.logger.Info("设置配额规则",
		zap.String("user_id", rule.UserID),
		zap.String("path", rule.Path),
		zap.Int64("max_bytes", rule.MaxBytes))

	return nil
}

// GetQuota 获取配额规则.
func (m *Manager) GetQuota(ctx context.Context, userID, path string) (*QuotaRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := ruleKey(userID, path)
	rule, ok := m.rules[key]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	return rule, nil
}

// UpdateUsage 更新使用量.
func (m *Manager) UpdateUsage(ctx context.Context, userID, path string, usedBytes, usedFiles int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ruleKey(userID, path)

	// 获取配额规则
	rule, ok := m.rules[key]
	if !ok {
		return ErrQuotaNotFound
	}

	// 计算使用百分比
	usagePercent := float64(usedBytes) / float64(rule.MaxBytes) * 100

	// 更新使用量
	usage := &QuotaUsage{
		UserID:       userID,
		Path:         path,
		UsedBytes:    usedBytes,
		UsedFiles:    usedFiles,
		TotalBytes:   rule.MaxBytes,
		TotalFiles:   rule.MaxFiles,
		UsagePercent: usagePercent,
		LastUpdated:  time.Now(),
	}
	m.usages[key] = usage

	// 添加历史记录
	history := UsageHistory{
		UsedBytes: usedBytes,
		UsedFiles: usedFiles,
		Timestamp: time.Now(),
	}
	m.history[key] = append(m.history[key], history)

	// 限制历史记录数量
	if len(m.history[key]) > 30 {
		m.history[key] = m.history[key][len(m.history[key])-30:]
	}

	m.logger.Debug("更新使用量",
		zap.String("user_id", userID),
		zap.String("path", path),
		zap.Int64("used_bytes", usedBytes),
		zap.Float64("usage_percent", usagePercent))

	return nil
}

// CheckQuota 检查配额，返回告警.
func (m *Manager) CheckQuota(ctx context.Context, userID, path string) (*QuotaAlert, error) {
	m.mu.RLock()
	key := ruleKey(userID, path)
	rule, ok := m.rules[key]
	usage, usageOk := m.usages[key]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrQuotaNotFound
	}
	if !usageOk {
		return nil, nil
	}

	// 计算使用百分比
	usagePercent := float64(usage.UsedBytes) / float64(rule.MaxBytes)

	// 检查是否需要生成告警
	var level AlertLevel
	var message string

	switch {
	case usagePercent >= 1.0:
		level = AlertExceeded
		message = fmt.Sprintf("用户 %s 路径 %s 已超限，当前使用 %.1f%%", userID, path, usagePercent*100)
	case usagePercent >= rule.CriticalThreshold:
		level = AlertCritical
		message = fmt.Sprintf("用户 %s 路径 %s 即将超限，当前使用 %.1f%%", userID, path, usagePercent*100)
	case usagePercent >= rule.WarnThreshold:
		level = AlertWarning
		message = fmt.Sprintf("用户 %s 路径 %s 使用量较高，当前使用 %.1f%%", userID, path, usagePercent*100)
	default:
		return nil, nil
	}

	// 创建告警
	m.mu.Lock()
	alert := &QuotaAlert{
		ID:           generateID(),
		UserID:       userID,
		Path:         path,
		Level:        level,
		Message:      message,
		CurrentUsage: usage.UsedBytes,
		Threshold:    usagePercent,
		Acknowledged: false,
		CreatedAt:    time.Now(),
	}
	m.alerts[alert.ID] = alert
	m.mu.Unlock()

	m.logger.Warn("配额告警",
		zap.String("alert_id", alert.ID),
		zap.String("level", string(level)),
		zap.String("message", message))

	return alert, nil
}

// PredictFullDate 预测磁盘用满日期（基于历史增长）.
func (m *Manager) PredictFullDate(ctx context.Context, userID, path string) (*UsageTrend, error) {
	m.mu.RLock()
	key := ruleKey(userID, path)
	rule, ok := m.rules[key]
	usage, usageOk := m.usages[key]
	history := m.history[key]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrQuotaNotFound
	}
	if !usageOk {
		return nil, errors.New("暂无使用量数据")
	}

	trend := &UsageTrend{
		UserID: userID,
		Path:   path,
	}

	// 计算历史增长
	if len(history) >= 2 {
		// 计算日增长（最近一条与前一天对比）
		lastIdx := len(history) - 1
		if lastIdx >= 1 {
			last := history[lastIdx]
			prev := history[lastIdx-1]
			hoursDiff := last.Timestamp.Sub(prev.Timestamp).Hours()

			if hoursDiff > 0 {
				bytesPerHour := float64(last.UsedBytes-prev.UsedBytes) / hoursDiff
				trend.DailyGrowth = int64(bytesPerHour * 24)
				trend.WeeklyGrowth = trend.DailyGrowth * 7
				trend.MonthlyGrowth = trend.DailyGrowth * 30
			}
		}

		// 预测用满日期
		if trend.DailyGrowth > 0 {
			remainingBytes := rule.MaxBytes - usage.UsedBytes
			if remainingBytes > 0 {
				daysUntilFull := float64(remainingBytes) / float64(trend.DailyGrowth)
				predictedDate := time.Now().AddDate(0, 0, int(daysUntilFull))
				trend.PredictedFullDate = &predictedDate
			}
		}

		// 判断趋势方向
		if trend.DailyGrowth > 1024*1024 { // 日增长超过 1MB
			trend.TrendDirection = TrendGrowing
		} else if trend.DailyGrowth < -1024*1024 { // 日减少超过 1MB
			trend.TrendDirection = TrendShrinking
		} else {
			trend.TrendDirection = TrendStable
		}
	} else {
		trend.TrendDirection = TrendStable
	}

	return trend, nil
}

// GenerateCleanupSuggestions 生成清理建议.
func (m *Manager) GenerateCleanupSuggestions(ctx context.Context, userID, path string) ([]CleanupSuggestion, error) {
	m.mu.RLock()
	key := ruleKey(userID, path)
	rule, ok := m.rules[key]
	usage, usageOk := m.usages[key]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrQuotaNotFound
	}
	if !usageOk {
		return nil, nil
	}

	suggestions := make([]CleanupSuggestion, 0)
	priority := 1

	// 检查临时文件
	if usage.UsagePercent > float64(rule.WarnThreshold)*100 {
		suggestions = append(suggestions, CleanupSuggestion{
			ID:                    generateID(),
			UserID:                userID,
			Path:                  path,
			SuggestionType:        SuggestionTemp,
			EstimatedReclaimBytes: int64(float64(usage.UsedBytes) * 0.1), // 估算 10%
			Priority:              priority,
			Files: []string{
				filepath.Join(path, "*.tmp"),
				filepath.Join(path, "*.log"),
			},
		})
		priority++
	}

	// 检查重复文件
	if usage.UsagePercent > float64(rule.CriticalThreshold)*100 {
		suggestions = append(suggestions, CleanupSuggestion{
			ID:                    generateID(),
			UserID:                userID,
			Path:                  path,
			SuggestionType:        SuggestionDuplicates,
			EstimatedReclaimBytes: int64(float64(usage.UsedBytes) * 0.05), // 估算 5%
			Priority:              priority,
			Files:                 []string{},
		})
		priority++
	}

	// 检查旧备份
	suggestions = append(suggestions, CleanupSuggestion{
		ID:                    generateID(),
		UserID:                userID,
		Path:                  path,
		SuggestionType:        SuggestionOldBackups,
		EstimatedReclaimBytes: int64(float64(usage.UsedBytes) * 0.15), // 估算 15%
		Priority:              priority,
		Files: []string{
			filepath.Join(path, "backups", "*.bak"),
			filepath.Join(path, "backups", "*.tar.gz"),
		},
	})
	priority++

	// 检查大文件
	if usage.UsedBytes > rule.MaxBytes/2 {
		suggestions = append(suggestions, CleanupSuggestion{
			ID:                    generateID(),
			UserID:                userID,
			Path:                  path,
			SuggestionType:        SuggestionLargeFiles,
			EstimatedReclaimBytes: int64(float64(usage.UsedBytes) * 0.2), // 估算 20%
			Priority:              priority,
			Files: []string{
				filepath.Join(path, "**", "*.iso"),
				filepath.Join(path, "**", "*.zip"),
				filepath.Join(path, "**", "*.tar"),
			},
		})
	}

	return suggestions, nil
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(ctx context.Context, userID string, unacknowledgedOnly bool) []QuotaAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]QuotaAlert, 0)
	for _, alert := range m.alerts {
		// 按用户过滤
		if userID != "" && alert.UserID != userID {
			continue
		}
		// 按确认状态过滤
		if unacknowledgedOnly && alert.Acknowledged {
			continue
		}
		alerts = append(alerts, *alert)
	}

	return alerts
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(ctx context.Context, alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return ErrAlertNotFound
	}

	alert.Acknowledged = true
	m.logger.Info("告警已确认", zap.String("alert_id", alertID))

	return nil
}

// GenerateReport 生成全局配额报告.
func (m *Manager) GenerateReport(ctx context.Context) (*QuotaReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &QuotaReport{
		GeneratedAt: time.Now(),
		Users:       make([]UserQuotaSummary, 0),
	}

	// 收集所有用户
	userMap := make(map[string]bool)
	for key := range m.rules {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) > 0 {
			userMap[parts[0]] = true
		}
	}

	// 为每个用户生成摘要
	for userID := range userMap {
		summary := UserQuotaSummary{
			UserID:      userID,
			UserName:    userID, // 简化：使用 userID 作为用户名
			Alerts:      make([]QuotaAlert, 0),
			Suggestions: make([]CleanupSuggestion, 0),
		}

		// 计算总配额和使用量
		for key, rule := range m.rules {
			if !strings.HasPrefix(key, userID+":") {
				continue
			}

			summary.TotalQuota += rule.MaxBytes

			if usage, ok := m.usages[key]; ok {
				summary.UsedQuota += usage.UsedBytes
			}
		}

		// 计算使用百分比
		if summary.TotalQuota > 0 {
			summary.UsagePercent = float64(summary.UsedQuota) / float64(summary.TotalQuota) * 100
		}

		// 收集告警
		for _, alert := range m.alerts {
			if alert.UserID == userID {
				summary.Alerts = append(summary.Alerts, *alert)
			}
		}

		report.Users = append(report.Users, summary)
	}

	return report, nil
}

// AutoCleanup 自动清理临时文件.
func (m *Manager) AutoCleanup(ctx context.Context, userID string) (int64, error) {
	m.mu.RLock()
	// 收集用户的所有规则
	userRules := make([]*QuotaRule, 0)
	for key, rule := range m.rules {
		if strings.HasPrefix(key, userID+":") {
			userRules = append(userRules, rule)
		}
	}
	m.mu.RUnlock()

	var totalCleaned int64

	for _, rule := range userRules {
		if !rule.Enabled {
			continue
		}

		// 检查是否需要清理
		m.mu.RLock()
		key := ruleKey(rule.UserID, rule.Path)
		usage, ok := m.usages[key]
		m.mu.RUnlock()

		if !ok {
			continue
		}

		usagePercent := float64(usage.UsedBytes) / float64(rule.MaxBytes)

		// 只有超过警告阈值才清理
		if usagePercent < rule.WarnThreshold {
			continue
		}

		// 扫描并清理临时文件
		cleaned, err := m.cleanupTempFiles(rule.Path)
		if err != nil {
			m.logger.Error("清理临时文件失败",
				zap.String("path", rule.Path),
				zap.Error(err))
			continue
		}

		totalCleaned += cleaned
	}

	m.logger.Info("自动清理完成",
		zap.String("user_id", userID),
		zap.Int64("cleaned_bytes", totalCleaned))

	return totalCleaned, nil
}

// cleanupTempFiles 清理临时文件.
func (m *Manager) cleanupTempFiles(path string) (int64, error) {
	var cleaned int64

	// 临时文件扩展名
	tempExts := map[string]bool{
		".tmp": true,
		".log": true,
		".bak": true,
	}

	// 遍历目录
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 检查是否是临时文件
		ext := strings.ToLower(filepath.Ext(filePath))
		if tempExts[ext] {
			// 检查文件修改时间（超过 7 天的临时文件）
			if time.Since(info.ModTime()) > 7*24*time.Hour {
				size := info.Size()
				if err := os.Remove(filePath); err == nil {
					cleaned += size
					m.logger.Debug("删除临时文件",
						zap.String("file", filePath),
						zap.Int64("size", size))
				}
			}
		}

		return nil
	})

	return cleaned, err
}
