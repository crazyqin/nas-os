package smartsleep

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// Handler HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smartsleep/config", h.handleConfig)
	mux.HandleFunc("/api/v1/smartsleep/disks", h.handleDisks)
	mux.HandleFunc("/api/v1/smartsleep/disks/", h.handleDiskByID)
	mux.HandleFunc("/api/v1/smartsleep/predictions", h.handlePredictions)
	mux.HandleFunc("/api/v1/smartsleep/predictions/", h.handlePredictionByDisk)
	mux.HandleFunc("/api/v1/smartsleep/stats", h.handleStats)
	mux.HandleFunc("/api/v1/smartsleep/policy", h.handlePolicy)
	mux.HandleFunc("/api/v1/smartsleep/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/smartsleep/tasks/", h.handleTaskByID)
	mux.HandleFunc("/api/v1/smartsleep/wake", h.handleWake)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.manager.GetConfig())
	case http.MethodPut:
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := h.manager.UpdateConfig(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	disks := h.manager.ListDisks()
	writeJSON(w, http.StatusOK, disks)
}

func (h *Handler) handleDiskByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/smartsleep/disks/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing disk id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		disk, err := h.manager.GetDisk(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, disk)
	case http.MethodPost:
		var req struct {
			Action string `json:"action"` // sleep, wake
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		switch req.Action {
		case "sleep":
			if err := h.manager.PutDiskToSleep(id); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		case "wake":
			resp, err := h.manager.EmergencyWake(id, "api")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handlePredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	predictions := h.manager.GetAllPredictions()
	writeJSON(w, http.StatusOK, predictions)
}

func (h *Handler) handlePredictionByDisk(w http.ResponseWriter, r *http.Request) {
	diskID := strings.TrimPrefix(r.URL.Path, "/api/v1/smartsleep/predictions/")
	if diskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing disk id"})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	pred, err := h.manager.PredictSleep(diskID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pred)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	stats := h.manager.GetEnergyStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.manager.GetPolicy())
	case http.MethodPut:
		var policy WeekendPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.manager.UpdatePolicy(&policy)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks := h.manager.ListTasks()
		writeJSON(w, http.StatusOK, tasks)
	case http.MethodPost:
		var task ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := h.manager.AddTask(&task); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, task)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/smartsleep/tasks/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing task id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := h.manager.GetTask(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, task)
	case http.MethodDelete:
		if err := h.manager.RemoveTask(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleWake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req WakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	resp, err := h.manager.EmergencyWake(req.DiskID, req.RequestedBy)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ========== Manager 方法 ==========

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		config:      config,
		disks:       make(map[string]*DiskInfo),
		patterns:    make(map[string]*AccessPattern),
		predictions: make(map[string]*SleepPrediction),
		policy: &WeekendPolicy{
			WeekendMode:      config.WeekendMode,
			WeekendIdleSec:   config.DefaultIdleSec * 2,
			WeekdayIdleSec:   config.DefaultIdleSec,
			WakeHours:        []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			WorkdayWakeHours: []int{9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
		},
		tasks:     make(map[string]*ScheduledTask),
		stats:     &EnergyStats{},
		accessLog: make([]AccessRecord, 0, 1000),
		wakeChan:  make(chan WakeRequest, 100),
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	if config == nil {
		return ErrInvalidConfig
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

// RegisterDisk 注册磁盘
func (m *Manager) RegisterDisk(id, device, model, serial string, wattsActive, wattsSleep float64) *DiskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk := &DiskInfo{
		ID:              id,
		Device:          device,
		Model:           model,
		Serial:          serial,
		State:           StateActive,
		LastAccess:      time.Now(),
		WattsWhenActive: wattsActive,
		WattsWhenSleep:  wattsSleep,
	}
	m.disks[id] = disk
	return disk
}

// GetDisk 获取磁盘信息
func (m *Manager) GetDisk(id string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[id]
	if !ok {
		return nil, ErrDiskNotFound
	}
	return disk, nil
}

// ListDisks 列出所有磁盘
func (m *Manager) ListDisks() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}
	return disks
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(diskID string, ioBytes int64, opType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return ErrDiskNotFound
	}

	now := time.Now()
	disk.LastAccess = now

	m.accessLog = append(m.accessLog, AccessRecord{
		DiskID:    diskID,
		Timestamp: now,
		IOBytes:   ioBytes,
		OpType:    opType,
	})

	// 更新访问模式
	pattern, ok := m.patterns[diskID]
	if !ok {
		pattern = &AccessPattern{
			DiskID: diskID,
		}
		m.patterns[diskID] = pattern
	}

	hour := now.Hour()
	weekday := int(now.Weekday())
	pattern.HourlyAccess[hour]++
	pattern.WeeklyAccess[weekday]++
	pattern.TotalRecords++
	pattern.LastUpdated = now

	// 计算平均空闲时间和高峰/安静时段
	m.recalculatePattern(pattern)

	return nil
}

func (m *Manager) recalculatePattern(pattern *AccessPattern) {
	// 计算高峰时段（访问量 > 平均值*1.5）
	totalAccess := 0
	for _, v := range pattern.HourlyAccess {
		totalAccess += v
	}
	avgAccess := float64(totalAccess) / 24.0

	var peakHours, quietHours []int
	for h, v := range pattern.HourlyAccess {
		if float64(v) > avgAccess*1.5 {
			peakHours = append(peakHours, h)
		} else if float64(v) < avgAccess*0.3 && avgAccess > 0 {
			quietHours = append(quietHours, h)
		}
	}
	pattern.PeakHours = peakHours
	pattern.QuietHours = quietHours

	// 估算平均空闲时间
	if pattern.TotalRecords > 1 && len(m.accessLog) > 1 {
		var totalIdle time.Duration
		count := 0
		for i := 1; i < len(m.accessLog); i++ {
			if m.accessLog[i].DiskID == pattern.DiskID {
				totalIdle += m.accessLog[i].Timestamp.Sub(m.accessLog[i-1].Timestamp)
				count++
			}
		}
		if count > 0 {
			pattern.AvgIdleSeconds = totalIdle.Seconds() / float64(count)
		}
	}
}

// PredictSleep 预测休眠
func (m *Manager) PredictSleep(diskID string) (*SleepPrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return nil, ErrDiskNotFound
	}

	pattern, hasPattern := m.patterns[diskID]
	now := time.Now()

	prediction := &SleepPrediction{
		DiskID: diskID,
	}

	// 基于当前时间和访问模式判断
	currentHour := now.Hour()
	isWeekend := now.Weekday() == time.Saturday || now.Weekday() == time.Sunday

	// 检查是否在安静时段
	isQuietHour := true
	if hasPattern {
		for _, h := range pattern.QuietHours {
			if h == currentHour {
				isQuietHour = true
				break
			}
		}
		for _, h := range pattern.PeakHours {
			if h == currentHour {
				isQuietHour = false
				break
			}
		}
	}

	// 空闲时长计算
	idleDuration := now.Sub(disk.LastAccess)
	idleMinutes := idleDuration.Minutes()

	// 决策逻辑
	threshold := float64(m.config.DefaultIdleSec) / 60.0
	if isWeekend {
		threshold = float64(m.policy.WeekendIdleSec) / 60.0
	} else {
		threshold = float64(m.policy.WeekdayIdleSec) / 60.0
	}

	if idleMinutes >= threshold {
		prediction.ShouldSleep = true
		prediction.Confidence = math.Min(0.95, 0.6+idleMinutes/threshold*0.1)
		prediction.Reason = fmt.Sprintf("已空闲 %.0f 分钟，超过阈值 %.0f 分钟", idleMinutes, threshold)
	} else if isQuietHour && idleMinutes >= threshold*0.5 {
		prediction.ShouldSleep = true
		prediction.Confidence = 0.5 + idleMinutes/threshold*0.2
		prediction.Reason = "安静时段且持续空闲"
	} else {
		prediction.ShouldSleep = false
		prediction.Confidence = math.Min(0.9, 0.5+threshold/idleMinutes*0.1)
		prediction.Reason = "活跃时段或空闲时间不足"
	}

	prediction.PredictedIdleMinutes = threshold - idleMinutes
	if prediction.PredictedIdleMinutes < 0 {
		prediction.PredictedIdleMinutes = 0
	}

	// 预测下次唤醒时间
	if hasPattern && len(pattern.PeakHours) > 0 {
		nextPeak := pattern.PeakHours[0]
		for _, h := range pattern.PeakHours {
			if h > currentHour {
				nextPeak = h
				break
			}
		}
		wakeTime := time.Date(now.Year(), now.Month(), now.Day(), nextPeak, 0, 0, 0, now.Location())
		if wakeTime.Before(now) {
			wakeTime = wakeTime.Add(24 * time.Hour)
		}
		prediction.NextWakeTime = wakeTime
	} else {
		prediction.NextWakeTime = now.Add(time.Duration(m.config.DefaultIdleSec) * time.Second)
	}

	// 缓存预测结果
	m.predictions[diskID] = prediction

	return prediction, nil
}

// GetAllPredictions 获取所有磁盘的休眠预测
func (m *Manager) GetAllPredictions() []*SleepPrediction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	predictions := make([]*SleepPrediction, 0, len(m.predictions))
	for _, p := range m.predictions {
		predictions = append(predictions, p)
	}
	return predictions
}

// PutDiskToSleep 将磁盘置入休眠
func (m *Manager) PutDiskToSleep(diskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return ErrDiskNotFound
	}

	if disk.State == StateSleep || disk.State == StateStandby || disk.State == StateSpindown {
		return ErrDiskAlreadySleeping
	}

	disk.State = StateSleep
	disk.LastSleepTime = time.Now()
	return nil
}

// EmergencyWake 紧急唤醒磁盘
func (m *Manager) EmergencyWake(diskID, requestedBy string) (*WakeResponse, error) {
	start := time.Now()

	m.mu.Lock()
	disk, ok := m.disks[diskID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrDiskNotFound
	}

	wasSleeping := disk.State != StateActive
	disk.State = StateActive
	disk.WakeCount++

	if wasSleeping && !disk.LastSleepTime.IsZero() {
		disk.TotalSleepSeconds += int64(time.Since(disk.LastSleepTime).Seconds())
	}
	m.mu.Unlock()

	elapsed := time.Since(start).Milliseconds()

	return &WakeResponse{
		DiskID:     diskID,
		Success:    true,
		WakeTimeMs: elapsed,
		Message:    fmt.Sprintf("磁盘已唤醒，由 %s 请求", requestedBy),
	}, nil
}

// GetEnergyStats 获取节能统计
func (m *Manager) GetEnergyStats() *EnergyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &EnergyStats{}
	var totalWattsSaved, totalKWh, totalCost, totalCO2 float64
	var totalSleepHours float64

	dailyMap := make(map[string]*DailyEnergyStat)

	for _, disk := range m.disks {
		sleepHours := float64(disk.TotalSleepSeconds) / 3600.0
		totalSleepHours += sleepHours

		wattsDiff := disk.WattsWhenActive - disk.WattsWhenSleep
		kwh := wattsDiff * sleepHours / 1000.0
		cost := kwh * m.config.ElectricityRate
		co2 := kwh * m.config.CarbonFactor

		totalWattsSaved += wattsDiff
		totalKWh += kwh
		totalCost += cost
		totalCO2 += co2

		// 按日期聚合
		if !disk.LastSleepTime.IsZero() {
			date := disk.LastSleepTime.Format("2006-01-02")
			if _, ok := dailyMap[date]; !ok {
				dailyMap[date] = &DailyEnergyStat{Date: date}
			}
			dailyMap[date].SleepHours += sleepHours
			dailyMap[date].KWhSaved += kwh
			dailyMap[date].CostSaved += cost
		}
	}

	stats.TotalSleepHours = totalSleepHours
	stats.WattsSaved = totalWattsSaved
	stats.KWhSaved = totalKWh
	stats.CostSaved = totalCost
	stats.CO2Reduced = totalCO2
	// 1棵树每年吸收约22kg CO2
	stats.TreesEquivalent = totalCO2 / 22.0

	dailyStats := make([]DailyEnergyStat, 0, len(dailyMap))
	for _, ds := range dailyMap {
		dailyStats = append(dailyStats, *ds)
	}
	stats.DailyStats = dailyStats

	return stats
}

// GetPolicy 获取周末策略
func (m *Manager) GetPolicy() *WeekendPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// UpdatePolicy 更新周末策略
func (m *Manager) UpdatePolicy(policy *WeekendPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
}

// GetPattern 获取访问模式
func (m *Manager) GetPattern(diskID string) (*AccessPattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pattern, ok := m.patterns[diskID]
	if !ok {
		return nil, ErrDiskNotFound
	}
	return pattern, nil
}

// AddTask 添加定时任务
func (m *Manager) AddTask(task *ScheduledTask) error {
	if task == nil || task.ID == "" {
		return ErrInvalidConfig
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查磁盘是否存在
	for _, diskID := range task.DiskIDs {
		if _, ok := m.disks[diskID]; !ok {
			return fmt.Errorf("%w: %s", ErrDiskNotFound, diskID)
		}
	}

	m.tasks[task.ID] = task
	return nil
}

// GetTask 获取定时任务
func (m *Manager) GetTask(id string) (*ScheduledTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// ListTasks 列出所有定时任务
func (m *Manager) ListTasks() []*ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// RemoveTask 删除定时任务
func (m *Manager) RemoveTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	delete(m.tasks, id)
	return nil
}

// CheckTaskConflicts 检查定时任务与休眠计划的冲突
func (m *Manager) CheckTaskConflicts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var conflicts []string
	for _, task := range m.tasks {
		if !task.Enabled {
			continue
		}
		for _, diskID := range task.DiskIDs {
			if disk, ok := m.disks[diskID]; ok {
				if disk.State == StateSleep || disk.State == StateStandby {
					conflicts = append(conflicts, fmt.Sprintf("任务 %s 需要磁盘 %s，但磁盘处于休眠状态", task.Name, diskID))
				}
			}
		}
	}
	return conflicts
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
