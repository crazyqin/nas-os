package warrantytracker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Manager 保修追踪管理器.
type Manager struct {
	mu          sync.RWMutex
	devices     map[string]*Device
	warranties  map[string]*Warranty
	repairs     map[string]*RepairRecord
	attachments map[string]*Attachment
	extendedW   map[string]*ExtendedWarranty
	config      *DepreciationConfig
	logger      Logger
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// Logger 日志接口.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建保修追踪管理器.
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		devices:     make(map[string]*Device),
		warranties:  make(map[string]*Warranty),
		repairs:     make(map[string]*RepairRecord),
		attachments: make(map[string]*Attachment),
		extendedW:   make(map[string]*ExtendedWarranty),
		config: &DepreciationConfig{
			Method:          "straight_line",
			UsefulLifeYears: 5,
			ResidualRate:    0.1,
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动到期提醒检查
	m.wg.Add(1)
	go m.reminderLoop()

	return m
}

// CreateDevice 创建设备.
func (m *Manager) CreateDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = generateID("dev")
	}
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	if device.Status == "" {
		device.Status = StatusActive
	}

	m.devices[device.ID] = device
	m.logger.Info("设备创建成功: %s %s (%s)", device.Brand, device.Model, device.ID)
	return nil
}

// UpdateDevice 更新设备.
func (m *Manager) UpdateDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.devices[device.ID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", device.ID)
	}

	device.CreatedAt = existing.CreatedAt
	device.UpdatedAt = time.Now()
	m.devices[device.ID] = device
	m.logger.Info("设备更新成功: %s", device.ID)
	return nil
}

// DeleteDevice 删除设备.
func (m *Manager) DeleteDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[deviceID]; !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 删除关联的保修、维修记录、附件
	delete(m.devices, deviceID)
	for id, w := range m.warranties {
		if w.DeviceID == deviceID {
			delete(m.warranties, id)
		}
	}
	for id, r := range m.repairs {
		if r.DeviceID == deviceID {
			delete(m.repairs, id)
		}
	}
	for id, a := range m.attachments {
		if a.DeviceID == deviceID {
			delete(m.attachments, id)
		}
	}

	m.logger.Info("设备删除成功: %s", deviceID)
	return nil
}

// GetDevice 获取设备.
func (m *Manager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices(category string) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		if category == "" || string(d.Category) == category {
			devices = append(devices, d)
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].PurchaseDate.After(devices[j].PurchaseDate)
	})
	return devices
}

// CreateWarranty 创建保修.
func (m *Manager) CreateWarranty(warranty *Warranty) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证设备存在
	if _, ok := m.devices[warranty.DeviceID]; !ok {
		return fmt.Errorf("设备不存在: %s", warranty.DeviceID)
	}

	if warranty.ID == "" {
		warranty.ID = generateID("war")
	}
	warranty.CreatedAt = time.Now()
	warranty.UpdatedAt = time.Now()
	warranty.Status = m.calculateWarrantyStatus(warranty)
	warranty.ReminderDays = 30 // 默认提前30天提醒

	m.warranties[warranty.ID] = warranty
	m.logger.Info("保修创建成功: 设备 %s, 保修至 %s", warranty.DeviceID, warranty.EndDate.Format("2006-01-02"))
	return nil
}

// UpdateWarranty 更新保修.
func (m *Manager) UpdateWarranty(warranty *Warranty) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.warranties[warranty.ID]
	if !ok {
		return fmt.Errorf("保修不存在: %s", warranty.ID)
	}

	warranty.CreatedAt = existing.CreatedAt
	warranty.UpdatedAt = time.Now()
	warranty.Status = m.calculateWarrantyStatus(warranty)
	m.warranties[warranty.ID] = warranty
	return nil
}

// GetDeviceWarranties 获取设备的所有保修.
func (m *Manager) GetDeviceWarranties(deviceID string) []*Warranty {
	m.mu.RLock()
	defer m.mu.RUnlock()

	warranties := make([]*Warranty, 0)
	for _, w := range m.warranties {
		if w.DeviceID == deviceID {
			warranties = append(warranties, w)
		}
	}
	return warranties
}

// AddExtendedWarranty 添加延保.
func (m *Manager) AddExtendedWarranty(ew *ExtendedWarranty) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	warranty, ok := m.warranties[ew.WarrantyID]
	if !ok {
		return fmt.Errorf("保修不存在: %s", ew.WarrantyID)
	}

	if ew.ID == "" {
		ew.ID = generateID("ext")
	}
	ew.CreatedAt = time.Now()

	// 更新原保修状态
	warranty.EndDate = ew.EndDate
	warranty.Status = WarrantyExtended
	warranty.UpdatedAt = time.Now()

	m.extendedW[ew.ID] = ew
	m.logger.Info("延保添加成功: 保修 %s, 延至 %s", ew.WarrantyID, ew.EndDate.Format("2006-01-02"))
	return nil
}

// CreateRepairRecord 创建维修记录.
func (m *Manager) CreateRepairRecord(repair *RepairRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[repair.DeviceID]; !ok {
		return fmt.Errorf("设备不存在: %s", repair.DeviceID)
	}

	if repair.ID == "" {
		repair.ID = generateID("rep")
	}
	repair.CreatedAt = time.Now()
	repair.UpdatedAt = time.Now()
	if repair.Status == "" {
		repair.Status = "pending"
	}

	m.repairs[repair.ID] = repair
	m.logger.Info("维修记录创建成功: 设备 %s, 故障: %s", repair.DeviceID, repair.FaultDesc)
	return nil
}

// UpdateRepairRecord 更新维修记录.
func (m *Manager) UpdateRepairRecord(repair *RepairRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.repairs[repair.ID]
	if !ok {
		return fmt.Errorf("维修记录不存在: %s", repair.ID)
	}

	repair.CreatedAt = existing.CreatedAt
	repair.UpdatedAt = time.Now()
	m.repairs[repair.ID] = repair
	return nil
}

// GetDeviceRepairRecords 获取设备的维修记录.
func (m *Manager) GetDeviceRepairRecords(deviceID string) []*RepairRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*RepairRecord, 0)
	for _, r := range m.repairs {
		if r.DeviceID == deviceID {
			records = append(records, r)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].RepairDate.After(records[j].RepairDate)
	})
	return records
}

// AddAttachment 添加附件.
func (m *Manager) AddAttachment(att *Attachment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[att.DeviceID]; !ok {
		return fmt.Errorf("设备不存在: %s", att.DeviceID)
	}

	if att.ID == "" {
		att.ID = generateID("att")
	}
	att.CreatedAt = time.Now()

	m.attachments[att.ID] = att
	m.logger.Info("附件添加成功: %s (%s)", att.Name, att.Type)
	return nil
}

// DeleteAttachment 删除附件.
func (m *Manager) DeleteAttachment(attID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.attachments[attID]; !ok {
		return fmt.Errorf("附件不存在: %s", attID)
	}

	delete(m.attachments, attID)
	return nil
}

// GetDeviceAttachments 获取设备附件.
func (m *Manager) GetDeviceAttachments(deviceID string) []*Attachment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	attachments := make([]*Attachment, 0)
	for _, a := range m.attachments {
		if a.DeviceID == deviceID {
			attachments = append(attachments, a)
		}
	}
	return attachments
}

// GetAssetValuation 获取资产估值.
func (m *Manager) GetAssetValuation(deviceID string) (*AssetValuation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 计算折旧
	years := time.Since(device.PurchaseDate).Hours() / 8760
	depreciationRate := 1 - math.Pow(1-m.config.ResidualRate, years/float64(m.config.UsefulLifeYears))
	if depreciationRate > 1 {
		depreciationRate = 1
	}

	depreciationTotal := device.PurchasePrice * depreciationRate
	currentValue := device.PurchasePrice - depreciationTotal
	if currentValue < device.PurchasePrice*m.config.ResidualRate {
		currentValue = device.PurchasePrice * m.config.ResidualRate
	}

	// 计算维修总成本
	repairCostTotal := 0.0
	for _, r := range m.repairs {
		if r.DeviceID == deviceID && r.Status == "completed" {
			repairCostTotal += r.Cost
		}
	}

	return &AssetValuation{
		DeviceID:          deviceID,
		PurchasePrice:     device.PurchasePrice,
		CurrentValue:      currentValue,
		DepreciationTotal: depreciationTotal,
		DepreciationRate:  depreciationRate * 100,
		RepairCostTotal:   repairCostTotal,
		EvaluatedAt:       time.Now(),
	}, nil
}

// GetWarrantyStats 获取保修统计.
func (m *Manager) GetWarrantyStats() *WarrantyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WarrantyStats{
		TotalDevices: len(m.devices),
	}

	for _, d := range m.devices {
		if d.Status == StatusActive {
			stats.ActiveDevices++
		}
		stats.TotalValue += d.PurchasePrice
	}

	// 计算当前总价值
	for _, d := range m.devices {
		val, _ := m.calculateCurrentValue(d)
		stats.CurrentValue += val
	}

	// 统计保修状态
	for _, w := range m.warranties {
		switch w.Status {
		case WarrantyActive:
			stats.WarrantyActive++
		case WarrantyExpiring:
			stats.WarrantyExpiring++
		case WarrantyExpired:
			stats.WarrantyExpired++
		case WarrantyExtended:
			stats.WarrantyActive++ // 延保也算保修中
		}
	}

	// 统计维修
	for _, r := range m.repairs {
		stats.RepairsTotal++
		stats.RepairCostTotal += r.Cost
		if r.WarrantyClaim {
			stats.RepairsWarranty++
		}
	}

	return stats
}

// GetExpiringWarranties 获取即将到期的保修.
func (m *Manager) GetExpiringWarranties(days int) []*Reminder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reminders := make([]*Reminder, 0)
	now := time.Now()

	for _, w := range m.warranties {
		if w.Status == WarrantyExpired {
			continue
		}

		daysLeft := int(w.EndDate.Sub(now).Hours() / 24)
		if daysLeft <= days && daysLeft > 0 {
			device := m.devices[w.DeviceID]
			deviceName := ""
			if device != nil {
				deviceName = device.Brand + " " + device.Model
			}

			reminders = append(reminders, &Reminder{
				DeviceID:   w.DeviceID,
				DeviceName: deviceName,
				WarrantyID: w.ID,
				EndDate:    w.EndDate,
				DaysLeft:   daysLeft,
				Type:       "expiring",
			})
		}
	}

	sort.Slice(reminders, func(i, j int) bool {
		return reminders[i].DaysLeft < reminders[j].DaysLeft
	})
	return reminders
}

// GetExpiredWarranties 获取已过期的保修.
func (m *Manager) GetExpiredWarranties() []*Reminder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reminders := make([]*Reminder, 0)
	now := time.Now()

	for _, w := range m.warranties {
		if w.EndDate.Before(now) {
			device := m.devices[w.DeviceID]
			deviceName := ""
			if device != nil {
				deviceName = device.Brand + " " + device.Model
			}

			daysExpired := int(now.Sub(w.EndDate).Hours() / 24)
			reminders = append(reminders, &Reminder{
				DeviceID:   w.DeviceID,
				DeviceName: deviceName,
				WarrantyID: w.ID,
				EndDate:    w.EndDate,
				DaysLeft:   -daysExpired,
				Type:       "expired",
			})
		}
	}
	return reminders
}

// UpdateDepreciationConfig 更新折旧配置.
func (m *Manager) UpdateDepreciationConfig(config *DepreciationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// calculateWarrantyStatus 计算保修状态.
func (m *Manager) calculateWarrantyStatus(warranty *Warranty) WarrantyStatus {
	now := time.Now()
	if warranty.EndDate.Before(now) {
		return WarrantyExpired
	}

	daysLeft := int(warranty.EndDate.Sub(now).Hours() / 24)
	if daysLeft <= 30 {
		return WarrantyExpiring
	}
	return WarrantyActive
}

// calculateCurrentValue 计算设备当前价值.
func (m *Manager) calculateCurrentValue(device *Device) (float64, error) {
	years := time.Since(device.PurchaseDate).Hours() / 8760
	depreciationRate := 1 - math.Pow(1-m.config.ResidualRate, years/float64(m.config.UsefulLifeYears))
	if depreciationRate > 1 {
		depreciationRate = 1
	}

	currentValue := device.PurchasePrice * (1 - depreciationRate)
	minValue := device.PurchasePrice * m.config.ResidualRate
	if currentValue < minValue {
		currentValue = minValue
	}
	return currentValue, nil
}

// reminderLoop 定期检查到期提醒.
func (m *Manager) reminderLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Hour * 24) // 每天检查一次
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkReminders()
		}
	}
}

// checkReminders 检查并更新提醒状态.
func (m *Manager) checkReminders() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, w := range m.warranties {
		if w.Status == WarrantyExpired {
			continue
		}

		daysLeft := int(w.EndDate.Sub(now).Hours() / 24)
		if daysLeft <= 0 {
			w.Status = WarrantyExpired
			w.UpdatedAt = now
			m.logger.Info("保修已过期: %s", w.ID)
		} else if daysLeft <= w.ReminderDays && !w.Notified {
			w.Status = WarrantyExpiring
			w.Notified = true
			w.UpdatedAt = now
			m.logger.Info("保修即将到期: %s, 剩余 %d 天", w.ID, daysLeft)
		}
	}
}

// generateID 生成唯一ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// RegisterRoutes 注册 HTTP 路由.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 设备管理
	mux.HandleFunc("/api/warranty/devices", m.handleDevices)
	mux.HandleFunc("/api/warranty/devices/", m.handleDeviceDetail)

	// 保修管理
	mux.HandleFunc("/api/warranty/warranties", m.handleWarranties)
	mux.HandleFunc("/api/warranty/extended", m.handleExtendedWarranty)

	// 维修记录
	mux.HandleFunc("/api/warranty/repairs", m.handleRepairs)

	// 附件管理
	mux.HandleFunc("/api/warranty/attachments", m.handleAttachments)

	// 资产估值
	mux.HandleFunc("/api/warranty/valuation", m.handleValuation)

	// 统计报表
	mux.HandleFunc("/api/warranty/stats", m.handleStats)

	// 到期提醒
	mux.HandleFunc("/api/warranty/reminders", m.handleReminders)

	// 折旧配置
	mux.HandleFunc("/api/warranty/depreciation-config", m.handleDepreciationConfig)
}

func (m *Manager) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		devices := m.ListDevices(category)
		writeJSON(w, devices)
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleDeviceDetail(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Path[len("/api/warranty/devices/"):]
	if deviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		device, err := m.GetDevice(deviceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, device)
	case http.MethodPut:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		device.ID = deviceID
		if err := m.UpdateDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	case http.MethodDelete:
		if err := m.DeleteDevice(deviceID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleWarranties(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deviceID := r.URL.Query().Get("device_id")
		if deviceID == "" {
			http.Error(w, "device_id required", http.StatusBadRequest)
			return
		}
		warranties := m.GetDeviceWarranties(deviceID)
		writeJSON(w, warranties)
	case http.MethodPost:
		var warranty Warranty
		if err := json.NewDecoder(r.Body).Decode(&warranty); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateWarranty(&warranty); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, warranty)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleExtendedWarranty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ew ExtendedWarranty
	if err := json.NewDecoder(r.Body).Decode(&ew); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := m.AddExtendedWarranty(&ew); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, ew)
}

func (m *Manager) handleRepairs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deviceID := r.URL.Query().Get("device_id")
		if deviceID == "" {
			http.Error(w, "device_id required", http.StatusBadRequest)
			return
		}
		records := m.GetDeviceRepairRecords(deviceID)
		writeJSON(w, records)
	case http.MethodPost:
		var repair RepairRecord
		if err := json.NewDecoder(r.Body).Decode(&repair); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateRepairRecord(&repair); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, repair)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleAttachments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deviceID := r.URL.Query().Get("device_id")
		if deviceID == "" {
			http.Error(w, "device_id required", http.StatusBadRequest)
			return
		}
		attachments := m.GetDeviceAttachments(deviceID)
		writeJSON(w, attachments)
	case http.MethodPost:
		var att Attachment
		if err := json.NewDecoder(r.Body).Decode(&att); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddAttachment(&att); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, att)
	case http.MethodDelete:
		attID := r.URL.Query().Get("id")
		if attID == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if err := m.DeleteAttachment(attID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleValuation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}

	valuation, err := m.GetAssetValuation(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, valuation)
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetWarrantyStats()
	writeJSON(w, stats)
}

func (m *Manager) handleReminders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	days := 30 // 默认30天
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	type_ := r.URL.Query().Get("type")
	if type_ == "expired" {
		reminders := m.GetExpiredWarranties()
		writeJSON(w, reminders)
	} else {
		reminders := m.GetExpiringWarranties(days)
		writeJSON(w, reminders)
	}
}

func (m *Manager) handleDepreciationConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, m.config)
	case http.MethodPut:
		var config DepreciationConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		m.UpdateDepreciationConfig(&config)
		writeJSON(w, m.config)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}
