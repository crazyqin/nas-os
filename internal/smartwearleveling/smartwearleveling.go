package smartwearleveling

import (
	"fmt"
	"sync"
	"time"
)

// SSDInfo SSD磁盘信息
type SSDInfo struct {
	ID              string         `json:"id"`
	Device          string         `json:"device"`
	Model           string         `json:"model"`
	Serial          string         `json:"serial"`
	Capacity        int64          `json:"capacity_bytes"`
	TBWWritten      int64          `json:"tbw_written_bytes"`
	TBWMax          int64          `json:"tbw_max_bytes"`
	LifePercent     float64        `json:"life_percent"`
	Temperature     int            `json:"temperature_celsius"`
	PowerOnHours    int64          `json:"power_on_hours"`
	TotalWritten    int64          `json:"total_written_bytes"`
	TotalRead       int64          `json:"total_read_bytes"`
	WearLevel       int            `json:"wear_level"`
	PowerCycles     int64          `json:"power_cycles"`
	UnsafeShutdowns int64          `json:"unsafe_shutdowns"`
	MediaErrors     int64          `json:"media_errors"`
	SMARTPassed     bool           `json:"smart_passed"`
	Attributes      []*SMARTAttr   `json:"attributes"`
	LastCheck       time.Time      `json:"last_check"`
	Status          WearStatus     `json:"status"`
}

// SMARTAttr SMART属性
type SMARTAttr struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  int64  `json:"raw_value"`
	Status    string `json:"status"`
}

// WearStatus 磨损状态
type WearStatus string

const (
	WearHealthy  WearStatus = "healthy"
	WearModerate WearStatus = "moderate"
	WearHigh     WearStatus = "high"
	WearCritical WearStatus = "critical"
)

// MigrationPolicy 迁移策略
type MigrationPolicy struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	SourceThreshold float64 `json:"source_threshold_percent"`
	TargetThreshold float64 `json:"target_threshold_percent"`
	Enabled         bool    `json:"enabled"`
	AutoMigrate     bool    `json:"auto_migrate"`
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	TargetID    string    `json:"target_id"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// WearPrediction 磨损预测
type WearPrediction struct {
	SSDID             string    `json:"ssd_id"`
	CurrentLife       float64   `json:"current_life_percent"`
	PredictedLifeDays int       `json:"predicted_life_days"`
	DailyWearRate     float64   `json:"daily_wear_rate"`
	EstimatedEOL      time.Time `json:"estimated_eol"`
	RiskLevel         string    `json:"risk_level"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	LifeWarningThreshold  float64 `json:"life_warning_threshold"`
	LifeCriticalThreshold float64 `json:"life_critical_threshold"`
	TempWarningThreshold  int     `json:"temp_warning_threshold"`
	TempCriticalThreshold int     `json:"temp_critical_threshold"`
	Enabled               bool    `json:"enabled"`
}

// WearAlert 磨损告警
type WearAlert struct {
	ID        string    `json:"id"`
	SSDID     string    `json:"ssd_id"`
	Device    string    `json:"device"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// WearStats 磨损统计
type WearStats struct {
	TotalSSDs      int     `json:"total_ssds"`
	HealthySSDs    int     `json:"healthy_ssds"`
	ModerateSSDs   int     `json:"moderate_ssds"`
	HighSSDs       int     `json:"high_ssds"`
	CriticalSSDs   int     `json:"critical_ssds"`
	AvgLifePercent float64 `json:"avg_life_percent"`
	MinLifePercent float64 `json:"min_life_percent"`
	TotalTBW       int64   `json:"total_tbw_bytes"`
	ActiveAlerts   int     `json:"active_alerts"`
	ActiveJobs     int     `json:"active_jobs"`
}

// SmartWearLevelingManager SSD磨损均衡管理器
type SmartWearLevelingManager struct {
	mu        sync.RWMutex
	ssds      map[string]*SSDInfo
	policies  []*MigrationPolicy
	jobs      []*MigrationJob
	alerts    []*WearAlert
	alertCfg  *AlertConfig
	dataPath  string
}

// NewManager 创建磨损均衡管理器
func NewManager(dataPath string) *SmartWearLevelingManager {
	m := &SmartWearLevelingManager{
		ssds:     make(map[string]*SSDInfo),
		policies: make([]*MigrationPolicy, 0),
		jobs:     make([]*MigrationJob, 0),
		alerts:   make([]*WearAlert, 0),
		alertCfg: &AlertConfig{
			LifeWarningThreshold:  30.0,
			LifeCriticalThreshold: 10.0,
			TempWarningThreshold:  60,
			TempCriticalThreshold: 70,
			Enabled:               true,
		},
		dataPath: dataPath,
	}
	m.initDefaultPolicies()
	return m
}

// RegisterSSD 注册SSD
func (m *SmartWearLevelingManager) RegisterSSD(ssd *SSDInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ssd.Device == "" {
		return fmt.Errorf("设备路径不能为空")
	}
	if ssd.ID == "" {
		ssd.ID = fmt.Sprintf("ssd_%d", time.Now().UnixNano())
	}
	ssd.LastCheck = time.Now()
	m.ssds[ssd.ID] = ssd
	m.checkWearStatus(ssd)
	return nil
}

// UnregisterSSD 注销SSD
func (m *SmartWearLevelingManager) UnregisterSSD(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.ssds[id]; !exists {
		return fmt.Errorf("SSD不存在: %s", id)
	}
	delete(m.ssds, id)
	return nil
}

// GetSSD 获取SSD信息
func (m *SmartWearLevelingManager) GetSSD(id string) (*SSDInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ssd, exists := m.ssds[id]
	if !exists {
		return nil, fmt.Errorf("SSD不存在: %s", id)
	}
	return ssd, nil
}

// ListSSDs 列出所有SSD
func (m *SmartWearLevelingManager) ListSSDs() []*SSDInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ssds := make([]*SSDInfo, 0, len(m.ssds))
	for _, ssd := range m.ssds {
		ssds = append(ssds, ssd)
	}
	return ssds
}

// UpdateSSDStats 更新SSD状态
func (m *SmartWearLevelingManager) UpdateSSDStats(id string, stats *SSDInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ssd, exists := m.ssds[id]
	if !exists {
		return fmt.Errorf("SSD不存在: %s", id)
	}
	if stats.TBWWritten > 0 {
		ssd.TBWWritten = stats.TBWWritten
	}
	if stats.Temperature > 0 {
		ssd.Temperature = stats.Temperature
	}
	if stats.TotalWritten > 0 {
		ssd.TotalWritten = stats.TotalWritten
	}
	if stats.TotalRead > 0 {
		ssd.TotalRead = stats.TotalRead
	}
	if stats.WearLevel > 0 {
		ssd.WearLevel = stats.WearLevel
	}
	if stats.TBWMax > 0 {
		ssd.TBWMax = stats.TBWMax
	}
	ssd.LastCheck = time.Now()
	m.checkWearStatus(ssd)
	return nil
}

// PredictWear 磨损预测
func (m *SmartWearLevelingManager) PredictWear(ssdID string) (*WearPrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ssd, exists := m.ssds[ssdID]
	if !exists {
		return nil, fmt.Errorf("SSD不存在: %s", ssdID)
	}

	prediction := &WearPrediction{
		SSDID:       ssdID,
		CurrentLife: ssd.LifePercent,
	}

	// 基于TBW计算每日磨损率
	if ssd.TBWWritten > 0 && ssd.PowerOnHours > 0 {
		daysUsed := float64(ssd.PowerOnHours) / 24.0
		if daysUsed > 0 {
			tbWUsedPercent := float64(ssd.TBWWritten) / float64(ssd.TBWMax) * 100
			prediction.DailyWearRate = tbWUsedPercent / daysUsed
			if prediction.DailyWearRate > 0 {
				prediction.PredictedLifeDays = int(ssd.LifePercent / prediction.DailyWearRate)
			}
		}
	}

	// 如果没有足够数据，基于当前寿命估算
	if prediction.PredictedLifeDays == 0 {
		if ssd.LifePercent > 80 {
			prediction.PredictedLifeDays = 365 * 5
		} else if ssd.LifePercent > 50 {
			prediction.PredictedLifeDays = 365 * 2
		} else if ssd.LifePercent > 20 {
			prediction.PredictedLifeDays = 365
		} else {
			prediction.PredictedLifeDays = 180
		}
		prediction.DailyWearRate = ssd.LifePercent / float64(prediction.PredictedLifeDays)
	}

	prediction.EstimatedEOL = time.Now().AddDate(0, 0, prediction.PredictedLifeDays)
	prediction.RiskLevel = m.calculateRiskLevel(ssd.LifePercent)
	return prediction, nil
}

// EvaluateMigrations 评估迁移需求
func (m *SmartWearLevelingManager) EvaluateMigrations() []*MigrationJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newJobs []*MigrationJob
	for _, policy := range m.policies {
		if !policy.Enabled || !policy.AutoMigrate {
			continue
		}
		// 查找高磨损源盘
		for _, src := range m.ssds {
			if src.LifePercent > policy.SourceThreshold {
				continue
			}
			// 查找低磨损目标盘
			for _, tgt := range m.ssds {
				if tgt.ID == src.ID {
					continue
				}
				if tgt.LifePercent < policy.TargetThreshold {
					continue
				}
				// 检查是否已有迁移任务
				if m.hasActiveJob(src.ID, tgt.ID) {
					continue
				}
				job := &MigrationJob{
					ID:        fmt.Sprintf("wear-mig-%d", time.Now().UnixNano()),
					SourceID:  src.ID,
					TargetID:  tgt.ID,
					Status:    "pending",
					StartedAt: time.Now(),
				}
				newJobs = append(newJobs, job)
				m.jobs = append(m.jobs, job)
			}
		}
	}
	return newJobs
}

// GetStats 获取统计
func (m *SmartWearLevelingManager) GetStats() *WearStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WearStats{
		TotalSSDs:      len(m.ssds),
		MinLifePercent: 100,
	}

	totalLife := 0.0
	for _, ssd := range m.ssds {
		totalLife += ssd.LifePercent
		stats.TotalTBW += ssd.TBWWritten
		if ssd.LifePercent < stats.MinLifePercent {
			stats.MinLifePercent = ssd.LifePercent
		}
		switch ssd.Status {
		case WearHealthy:
			stats.HealthySSDs++
		case WearModerate:
			stats.ModerateSSDs++
		case WearHigh:
			stats.HighSSDs++
		case WearCritical:
			stats.CriticalSSDs++
		}
	}
	if stats.TotalSSDs > 0 {
		stats.AvgLifePercent = totalLife / float64(stats.TotalSSDs)
	}

	for _, alert := range m.alerts {
		if !alert.Resolved {
			stats.ActiveAlerts++
		}
	}
	for _, job := range m.jobs {
		if job.Status == "pending" || job.Status == "running" {
			stats.ActiveJobs++
		}
	}
	return stats
}

// GetAlerts 获取告警
func (m *SmartWearLevelingManager) GetAlerts(resolved bool) []*WearAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alerts := make([]*WearAlert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// ResolveAlert 解决告警
func (m *SmartWearLevelingManager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id && !a.Resolved {
			a.Resolved = true
			now := time.Now()
			a.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("告警不存在或已解决: %s", id)
}

// GetJobs 获取迁移任务
func (m *SmartWearLevelingManager) GetJobs() []*MigrationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs
}

// UpdateAlertConfig 更新告警配置
func (m *SmartWearLevelingManager) UpdateAlertConfig(cfg *AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCfg = cfg
}

// GetAlertConfig 获取告警配置
func (m *SmartWearLevelingManager) GetAlertConfig() *AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertCfg
}

// AddPolicy 添加迁移策略
func (m *SmartWearLevelingManager) AddPolicy(policy *MigrationPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies = append(m.policies, policy)
}

// GetPolicies 获取迁移策略
func (m *SmartWearLevelingManager) GetPolicies() []*MigrationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policies
}

func (m *SmartWearLevelingManager) checkWearStatus(ssd *SSDInfo) {
	if ssd.LifePercent <= 0 {
		ssd.Status = WearCritical
	} else if ssd.LifePercent <= m.alertCfg.LifeCriticalThreshold {
		ssd.Status = WearCritical
		m.addAlert(ssd, "critical", fmt.Sprintf("SSD寿命严重不足: %.1f%% (阈值: %.1f%%)", ssd.LifePercent, m.alertCfg.LifeCriticalThreshold))
	} else if ssd.LifePercent <= m.alertCfg.LifeWarningThreshold {
		ssd.Status = WearHigh
		m.addAlert(ssd, "warning", fmt.Sprintf("SSD寿命较低: %.1f%% (阈值: %.1f%%)", ssd.LifePercent, m.alertCfg.LifeWarningThreshold))
	} else if ssd.LifePercent <= 50 {
		ssd.Status = WearModerate
	} else {
		ssd.Status = WearHealthy
	}

	if ssd.Temperature >= m.alertCfg.TempCriticalThreshold {
		m.addAlert(ssd, "critical", fmt.Sprintf("SSD温度过高: %d°C (阈值: %d°C)", ssd.Temperature, m.alertCfg.TempCriticalThreshold))
	} else if ssd.Temperature >= m.alertCfg.TempWarningThreshold {
		m.addAlert(ssd, "warning", fmt.Sprintf("SSD温度较高: %d°C (阈值: %d°C)", ssd.Temperature, m.alertCfg.TempWarningThreshold))
	}
}

func (m *SmartWearLevelingManager) addAlert(ssd *SSDInfo, level, message string) {
	if !m.alertCfg.Enabled {
		return
	}
	for _, a := range m.alerts {
		if a.SSDID == ssd.ID && a.Message == message && !a.Resolved {
			return
		}
	}
	m.alerts = append(m.alerts, &WearAlert{
		ID:        fmt.Sprintf("wear-alert-%d", time.Now().UnixNano()),
		SSDID:     ssd.ID,
		Device:    ssd.Device,
		Level:     level,
		Message:   message,
		CreatedAt: time.Now(),
	})
}

func (m *SmartWearLevelingManager) calculateRiskLevel(lifePercent float64) string {
	if lifePercent >= 70 {
		return "low"
	} else if lifePercent >= 40 {
		return "medium"
	} else if lifePercent >= 15 {
		return "high"
	}
	return "critical"
}

func (m *SmartWearLevelingManager) hasActiveJob(sourceID, targetID string) bool {
	for _, j := range m.jobs {
		if j.SourceID == sourceID && j.TargetID == targetID && (j.Status == "pending" || j.Status == "running") {
			return true
		}
	}
	return false
}

func (m *SmartWearLevelingManager) initDefaultPolicies() {
	m.policies = []*MigrationPolicy{
		{
			ID:              "default-wear-migration",
			Name:            "默认磨损迁移策略",
			SourceThreshold: 20.0,
			TargetThreshold: 60.0,
			Enabled:         true,
			AutoMigrate:     false,
		},
	}
}
