// Package powerbudget 提供用电预算告警功能
package powerbudget

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AlertManager 告警管理器.
type AlertManager struct {
	engine          *Engine
	logger          *zap.Logger
	cooldownMinutes int
	mu              sync.RWMutex
	lastAlertTime   map[string]time.Time
}

// NewAlertManager 创建告警管理器.
func NewAlertManager(engine *Engine, logger *zap.Logger) *AlertManager {
	return &AlertManager{
		engine:          engine,
		logger:          logger,
		cooldownMinutes: 30,
		lastAlertTime:   make(map[string]time.Time),
	}
}

// ========== 预算告警 ==========

// CheckBudgetAlerts 检查预算告警.
func (am *AlertManager) CheckBudgetAlerts() {
	am.engine.mu.RLock()
	budget := am.engine.budget
	am.engine.mu.RUnlock()

	if budget == nil || !budget.Enabled {
		return
	}

	// 计算本月使用情况
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	am.engine.mu.RLock()
	var usedCost int64
	for _, r := range am.engine.records {
		if r.Timestamp.After(startOfMonth) || r.Timestamp.Equal(startOfMonth) {
			usedCost += r.CostCents
		}
	}
	am.engine.mu.RUnlock()

	usedPercent := float64(usedCost) / budget.MonthlyAmount * 100.0

	// 检查紧急阈值
	if usedPercent >= budget.CriticalThreshold {
		am.triggerBudgetAlert(AlertLevelEmergency, "用电预算即将耗尽",
			"本月用电已达预算的"+formatPercent(usedPercent)+"，剩余预算"+formatCost(budget.MonthlyAmount-float64(usedCost))+"元",
			usedPercent, budget.CriticalThreshold)
		return
	}

	// 检查严重阈值
	if usedPercent >= budget.CriticalThreshold*0.9 {
		am.triggerBudgetAlert(AlertLevelCritical, "用电预算严重不足",
			"本月用电已达预算的"+formatPercent(usedPercent)+"，请注意控制用电",
			usedPercent, budget.CriticalThreshold)
		return
	}

	// 检查预警阈值
	if usedPercent >= budget.WarningThreshold {
		am.triggerBudgetAlert(AlertLevelWarning, "用电预算预警",
			"本月用电已达预算的"+formatPercent(usedPercent)+"",
			usedPercent, budget.WarningThreshold)
		return
	}

	// 检查预测超支
	prediction := am.engine.analyzer.PredictMonthly()
	if prediction != nil && prediction.WillExceed {
		am.triggerBudgetAlert(AlertLevelWarning, "预计本月用电将超支",
			"按当前趋势，预计月用电"+formatEnergy(prediction.PredictedKWh)+"，超出预算",
			usedPercent, 100.0)
	}
}

// ========== 异常功耗告警 ==========

// CheckAnomalyPower 检查异常功耗.
func (am *AlertManager) CheckAnomalyPower() {
	anomalies := am.engine.analyzer.DetectAnomalies(7)

	for _, anomaly := range anomalies {
		if anomaly.Severity == "critical" {
			am.triggerAnomalyAlert(anomaly)
		}
	}
}

// ========== 告警触发 ==========

func (am *AlertManager) triggerBudgetAlert(level AlertLevel, title, message string, value, threshold float64) {
	am.mu.RLock()
	key := "budget_" + string(level)
	lastTime, exists := am.lastAlertTime[key]
	am.mu.RUnlock()

	if exists && time.Since(lastTime) < time.Duration(am.cooldownMinutes)*time.Minute {
		return
	}

	alert := &Alert{
		ID:          uuid.New().String(),
		Type:        AlertTypeBudgetWarning,
		Level:       level,
		Title:       title,
		Message:     message,
		Value:       value,
		Threshold:   threshold,
		TriggeredAt: time.Now(),
		Active:      true,
	}

	if level == AlertLevelEmergency {
		alert.Type = AlertTypeBudgetExceeded
	}

	am.engine.mu.Lock()
	am.engine.alerts = append(am.engine.alerts, alert)
	am.engine.mu.Unlock()

	am.mu.Lock()
	am.lastAlertTime[key] = time.Now()
	am.mu.Unlock()

	am.logger.Warn("用电告警触发",
		zap.String("type", string(alert.Type)),
		zap.String("level", string(level)),
		zap.String("title", title),
		zap.Float64("value", value),
	)
}

func (am *AlertManager) triggerAnomalyAlert(anomaly AnomalyResult) {
	am.mu.RLock()
	key := "anomaly_" + anomaly.DeviceID
	lastTime, exists := am.lastAlertTime[key]
	am.mu.RUnlock()

	if exists && time.Since(lastTime) < time.Duration(am.cooldownMinutes)*time.Minute {
		return
	}

	alert := &Alert{
		ID:          uuid.New().String(),
		Type:        AlertTypeAnomalyPower,
		Level:       AlertLevelWarning,
		Title:       anomaly.DeviceName + " 功耗异常",
		Message:       anomaly.DeviceName + "功率" + formatPower(anomaly.PowerWatts) + "W超过正常范围（" + formatPower(anomaly.ExpectedMax) + "W）",
		DeviceID:    anomaly.DeviceID,
		Value:       anomaly.PowerWatts,
		Threshold:   anomaly.ExpectedMax,
		TriggeredAt: time.Now(),
		Active:      true,
	}

	if anomaly.Severity == "critical" {
		alert.Level = AlertLevelCritical
	}

	am.engine.mu.Lock()
	am.engine.alerts = append(am.engine.alerts, alert)
	am.engine.mu.Unlock()

	am.mu.Lock()
	am.lastAlertTime[key] = time.Now()
	am.mu.Unlock()

	am.logger.Warn("异常功耗告警",
		zap.String("device", anomaly.DeviceName),
		zap.Float64("power", anomaly.PowerWatts),
		zap.Float64("expected_max", anomaly.ExpectedMax),
		zap.Float64("deviation", anomaly.Deviation),
	)
}

// ========== 告警查询 ==========

// GetAlertsByLevel 按级别获取告警.
func (am *AlertManager) GetAlertsByLevel(level AlertLevel) []*Alert {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()

	var result []*Alert
	for _, alert := range am.engine.alerts {
		if alert.Level == level {
			result = append(result, alert)
		}
	}

	return result
}

// GetAlertsByType 按类型获取告警.
func (am *AlertManager) GetAlertsByType(alertType AlertType) []*Alert {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()

	var result []*Alert
	for _, alert := range am.engine.alerts {
		if alert.Type == alertType {
			result = append(result, alert)
		}
	}

	return result
}

// GetAlertStats 获取告警统计.
func (am *AlertManager) GetAlertStats() map[string]int {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()

	stats := map[string]int{
		"total":            len(am.engine.alerts),
		"active":           0,
		"resolved":         0,
		"info":             0,
		"warning":          0,
		"critical":         0,
		"emergency":        0,
		"budget_warning":   0,
		"budget_exceeded":  0,
		"anomaly_power":    0,
		"device_overload":  0,
	}

	for _, alert := range am.engine.alerts {
		if alert.Active {
			stats["active"]++
		} else {
			stats["resolved"]++
		}
		stats[string(alert.Level)]++
		stats[string(alert.Type)]++
	}

	return stats
}

// SetCooldownMinutes 设置告警冷却时间.
func (am *AlertManager) SetCooldownMinutes(minutes int) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.cooldownMinutes = minutes
}
