package massdeploy

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Logger 日志接口.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// Manager 批量部署管理器.
type Manager struct {
	mu                sync.RWMutex
	assets            map[string]*Asset
	templates         map[string]*ConfigTemplate
	deployJobs        map[string]*DeployJob
	firmwareInfo      map[string]*FirmwareInfo
	firmwareJobs      map[string]*FirmwareUpgradeJob
	costRecords       []*CostRecord
	reports           []*Report
	discoveredDevices map[string]*DiscoveredDevice
	events            []Event
	config            *Config
	logger            Logger
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

// Event 事件.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	AssetID   string    `json:"asset_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Config 配置.
type Config struct {
	ScanSubnet       string        `json:"scan_subnet"`
	ScanPort         int           `json:"scan_port"`
	ScanTimeout      time.Duration `json:"scan_timeout"`
	MaxConcurrentDep int           `json:"max_concurrent_deploy"`
	DeployTimeout    time.Duration `json:"deploy_timeout"`
	MaxRetries       int           `json:"max_retries"`
	DepreciationRate float64       `json:"depreciation_rate"` // 年折旧率
	UsefulLifeYears  int           `json:"useful_life_years"`
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		ScanSubnet:       "192.168.1.0/24",
		ScanPort:         5000,
		ScanTimeout:      5 * time.Second,
		MaxConcurrentDep: 10,
		DeployTimeout:    5 * time.Minute,
		MaxRetries:       3,
		DepreciationRate: 0.20,
		UsefulLifeYears:  5,
	}
}

// NewManager 创建管理器.
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		assets:            make(map[string]*Asset),
		templates:         make(map[string]*ConfigTemplate),
		deployJobs:        make(map[string]*DeployJob),
		firmwareInfo:      make(map[string]*FirmwareInfo),
		firmwareJobs:      make(map[string]*FirmwareUpgradeJob),
		costRecords:       make([]*CostRecord, 0),
		reports:           make([]*Report, 0),
		discoveredDevices: make(map[string]*DiscoveredDevice),
		events:            make([]Event, 0),
		config:            DefaultConfig(),
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// ==================== 设备发现 ====================

// ScanNetwork 扫描网络发现 NAS 设备.
func (m *Manager) ScanNetwork(req *ScanRequest) (*ScanResult, error) {
	if req == nil {
		req = &ScanRequest{
			Subnet:  m.config.ScanSubnet,
			Timeout: m.config.ScanTimeout,
		}
	}

	start := time.Now()
	_, ipNet, err := net.ParseCIDR(req.Subnet)
	if err != nil {
		return nil, fmt.Errorf("无效的子网: %w", err)
	}

	port := m.config.ScanPort
	if req.PortRange != "" {
		// 解析端口范围，取第一个端口
		fmt.Sscanf(req.PortRange, "%d", &port)
	}

	var devices []*DiscoveredDevice
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)

	for ip := nextIP(ip); ipNet.Contains(ip); ip = nextIP(ip) {
		addr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, req.Timeout)
		if err != nil {
			continue
		}
		conn.Close()

		device := &DiscoveredDevice{
			ID:           generateID("disc"),
			IP:           ip.String(),
			Status:       DeviceStatusOnline,
			DiscoveredAt: time.Now(),
		}
		devices = append(devices, device)

		m.mu.Lock()
		m.discoveredDevices[device.ID] = device
		m.mu.Unlock()
	}

	result := &ScanResult{
		Subnet:    req.Subnet,
		Devices:   devices,
		Total:     len(devices),
		ScanTime:  time.Since(start),
		ScannedAt: time.Now(),
	}

	m.addEvent("scan_complete", fmt.Sprintf("网络扫描完成: 发现 %d 台设备", len(devices)), "")
	return result, nil
}

// GetDiscoveredDevice 获取已发现设备.
func (m *Manager) GetDiscoveredDevice(deviceID string) (*DiscoveredDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.discoveredDevices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// ListDiscoveredDevices 列出已发现设备.
func (m *Manager) ListDiscoveredDevices() []*DiscoveredDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*DiscoveredDevice, 0, len(m.discoveredDevices))
	for _, d := range m.discoveredDevices {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})
	return devices
}

// ==================== 资产管理 ====================

// AddAsset 添加资产.
func (m *Manager) AddAsset(asset *Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if asset.ID == "" {
		asset.ID = generateID("asset")
	}
	asset.CreatedAt = time.Now()
	asset.UpdatedAt = time.Now()

	m.assets[asset.ID] = asset
	m.addEvent("asset_added", fmt.Sprintf("资产添加: %s (%s)", asset.Name, asset.SerialNumber), asset.ID)
	m.logger.Info("资产添加成功: %s (%s)", asset.Name, asset.ID)
	return nil
}

// UpdateAsset 更新资产.
func (m *Manager) UpdateAsset(asset *Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.assets[asset.ID]
	if !ok {
		return fmt.Errorf("资产不存在: %s", asset.ID)
	}

	asset.CreatedAt = existing.CreatedAt
	asset.UpdatedAt = time.Now()
	m.assets[asset.ID] = asset
	return nil
}

// RemoveAsset 删除资产.
func (m *Manager) RemoveAsset(assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return fmt.Errorf("资产不存在: %s", assetID)
	}

	delete(m.assets, assetID)
	m.addEvent("asset_removed", fmt.Sprintf("资产移除: %s", asset.Name), assetID)
	return nil
}

// GetAsset 获取资产.
func (m *Manager) GetAsset(assetID string) (*Asset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return nil, fmt.Errorf("资产不存在: %s", assetID)
	}
	return asset, nil
}

// ListAssets 列出资产.
func (m *Manager) ListAssets(assetType AssetType, status AssetStatus) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]*Asset, 0, len(m.assets))
	for _, a := range m.assets {
		if assetType != "" && a.Type != assetType {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		assets = append(assets, a)
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Name < assets[j].Name
	})
	return assets
}

// GetHardwareInfo 获取资产硬件信息.
func (m *Manager) GetHardwareInfo(assetID string) (*HardwareInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return nil, fmt.Errorf("资产不存在: %s", assetID)
	}

	info := &HardwareInfo{
		AssetID:       asset.ID,
		CPUCores:      asset.CPUCores,
		MemoryTotalGB: float64(asset.MemoryGB),
		DiskSlots:     asset.DiskSlots,
		UpdatedAt:     time.Now(),
	}
	return info, nil
}

// ==================== 部署模板 ====================

// CreateTemplate 创建配置模板.
func (m *Manager) CreateTemplate(tmpl *ConfigTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tmpl.ID == "" {
		tmpl.ID = generateID("tmpl")
	}
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()

	m.templates[tmpl.ID] = tmpl
	m.logger.Info("模板创建: %s (%s)", tmpl.Name, tmpl.ID)
	return nil
}

// UpdateTemplate 更新模板.
func (m *Manager) UpdateTemplate(tmpl *ConfigTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.templates[tmpl.ID]
	if !ok {
		return fmt.Errorf("模板不存在: %s", tmpl.ID)
	}

	tmpl.CreatedAt = existing.CreatedAt
	tmpl.UpdatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.templates[templateID]; !ok {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	delete(m.templates, templateID)
	return nil
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(templateID string) (*ConfigTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpl, ok := m.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}
	return tmpl, nil
}

// ListTemplates 列出模板.
func (m *Manager) ListTemplates() []*ConfigTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*ConfigTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates
}

// ==================== 部署任务 ====================

// CreateDeployJob 创建部署任务.
func (m *Manager) CreateDeployJob(job *DeployJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证模板存在
	if _, ok := m.templates[job.TemplateID]; !ok {
		return fmt.Errorf("模板不存在: %s", job.TemplateID)
	}

	if job.ID == "" {
		job.ID = generateID("job")
	}
	job.Status = JobStatusPending
	job.TotalDevices = len(job.TargetDevices)
	job.MaxRetries = m.config.MaxRetries
	job.Results = make(map[string]*DeployResult)
	job.CreatedAt = time.Now()

	m.deployJobs[job.ID] = job
	m.addEvent("job_created", fmt.Sprintf("部署任务创建: %s → %d 台设备", job.Name, job.TotalDevices), "")
	m.logger.Info("部署任务创建: %s (%s)", job.Name, job.ID)

	// 异步执行部署
	go m.executeDeployJob(job.ID)

	return nil
}

// executeDeployJob 执行部署任务.
func (m *Manager) executeDeployJob(jobID string) {
	m.mu.Lock()
	job, ok := m.deployJobs[jobID]
	if !ok {
		m.mu.Unlock()
		return
	}
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()
	m.mu.Unlock()

	// 逐台设备部署
	for i, deviceID := range job.TargetDevices {
		result := &DeployResult{
			DeviceID:  deviceID,
			StartedAt: time.Now(),
		}

		// 模拟部署
		err := m.deployToDevice(deviceID, job.TemplateID, job.Config)
		if err != nil {
			result.Success = false
			result.Error = err.Error()

			// 重试逻辑
			for retry := 0; retry < job.MaxRetries; retry++ {
				m.mu.Lock()
				job.RetryCount++
				job.Status = JobStatusRetrying
				m.mu.Unlock()

				result.Retries = retry + 1
				err = m.deployToDevice(deviceID, job.TemplateID, job.Config)
				if err == nil {
					result.Success = true
					result.Message = "部署成功（重试）"
					break
				}
			}
		} else {
			result.Success = true
			result.Message = "部署成功"
		}

		result.Duration = time.Since(result.StartedAt)

		m.mu.Lock()
		job.Results[deviceID] = result
		if result.Success {
			job.SuccessCount++
		} else {
			job.FailCount++
		}
		job.Progress = int(float64(i+1) / float64(job.TotalDevices) * 100)
		m.mu.Unlock()
	}

	m.mu.Lock()
	job.CompletedAt = time.Now()
	if job.FailCount == 0 {
		job.Status = JobStatusCompleted
	} else if job.SuccessCount == 0 {
		job.Status = JobStatusFailed
	} else {
		job.Status = JobStatusCompleted // 部分成功
	}
	m.addEvent("job_completed", fmt.Sprintf("部署任务完成: %s (成功:%d 失败:%d)", job.Name, job.SuccessCount, job.FailCount), "")
	m.mu.Unlock()
}

// deployToDevice 部署配置到单台设备.
func (m *Manager) deployToDevice(deviceID, templateID string, overrides map[string]string) error {
	// 模拟部署逻辑：验证设备和模板存在
	m.mu.RLock()
	_, assetOK := m.assets[deviceID]
	_, tmplOK := m.templates[templateID]
	m.mu.RUnlock()

	if !assetOK {
		return fmt.Errorf("资产不存在: %s", deviceID)
	}
	if !tmplOK {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	// 实际部署会通过 SSH/HTTP 推送配置
	return nil
}

// GetDeployJob 获取部署任务.
func (m *Manager) GetDeployJob(jobID string) (*DeployJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.deployJobs[jobID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}
	return job, nil
}

// ListDeployJobs 列出部署任务.
func (m *Manager) ListDeployJobs(status JobStatus) []*DeployJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*DeployJob, 0, len(m.deployJobs))
	for _, j := range m.deployJobs {
		if status != "" && j.Status != status {
			continue
		}
		jobs = append(jobs, j)
	}

	sort.Slice(jobs, func(i, k int) bool {
		return jobs[i].CreatedAt.After(jobs[k].CreatedAt)
	})
	return jobs
}

// CancelDeployJob 取消部署任务.
func (m *Manager) CancelDeployJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.deployJobs[jobID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
		return fmt.Errorf("任务已完成，无法取消")
	}

	job.Status = JobStatusCancelled
	job.CompletedAt = time.Now()
	return nil
}

// RetryDeployJob 重试失败的部署任务.
func (m *Manager) RetryDeployJob(jobID string) error {
	m.mu.Lock()
	job, ok := m.deployJobs[jobID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status != JobStatusFailed {
		m.mu.Unlock()
		return fmt.Errorf("只有失败的任务才能重试")
	}

	// 收集失败的设备
	var failedDevices []string
	for deviceID, result := range job.Results {
		if !result.Success {
			failedDevices = append(failedDevices, deviceID)
		}
	}

	job.TargetDevices = failedDevices
	job.TotalDevices = len(failedDevices)
	job.FailCount = 0
	job.SuccessCount = 0
	job.Progress = 0
	job.Status = JobStatusPending
	job.Results = make(map[string]*DeployResult)
	m.mu.Unlock()

	go m.executeDeployJob(jobID)
	return nil
}

// ==================== 固件管理 ====================

// AddFirmwareInfo 添加固件信息.
func (m *Manager) AddFirmwareInfo(info *FirmwareInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info.ID == "" {
		info.ID = generateID("fw")
	}
	info.CreatedAt = time.Now()

	m.firmwareInfo[info.ID] = info
	m.logger.Info("固件信息添加: %s v%s", info.Model, info.Version)
	return nil
}

// ListFirmwareInfo 列出固件信息.
func (m *Manager) ListFirmwareInfo(model string) []*FirmwareInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]*FirmwareInfo, 0, len(m.firmwareInfo))
	for _, info := range m.firmwareInfo {
		if model != "" && info.Model != model {
			continue
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ReleaseDate.After(infos[j].ReleaseDate)
	})
	return infos
}

// CreateFirmwareUpgradeJob 创建固件升级任务.
func (m *Manager) CreateFirmwareUpgradeJob(job *FirmwareUpgradeJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job.ID == "" {
		job.ID = generateID("fwjob")
	}
	job.Status = JobStatusPending
	job.Results = make(map[string]string)
	job.CreatedAt = time.Now()

	m.firmwareJobs[job.ID] = job
	m.addEvent("firmware_job_created", fmt.Sprintf("固件升级任务创建: v%s → %d 台设备", job.Version, len(job.TargetDevices)), "")
	return nil
}

// ExecuteFirmwareUpgrade 执行固件升级.
func (m *Manager) ExecuteFirmwareUpgrade(jobID string) error {
	m.mu.Lock()
	job, ok := m.firmwareJobs[jobID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("固件升级任务不存在: %s", jobID)
	}
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()
	total := len(job.TargetDevices)
	m.mu.Unlock()

	for i, deviceID := range job.TargetDevices {
		// 模拟升级
		m.mu.Lock()
		job.Results[deviceID] = "success"
		job.Progress = int(float64(i+1) / float64(total) * 100)
		m.mu.Unlock()
	}

	m.mu.Lock()
	job.Status = JobStatusCompleted
	job.CompletedAt = time.Now()
	m.addEvent("firmware_upgrade_done", fmt.Sprintf("固件升级完成: v%s", job.Version), "")
	m.mu.Unlock()

	return nil
}

// RollbackFirmware 回滚固件.
func (m *Manager) RollbackFirmware(deviceID, targetVersion string) error {
	m.mu.RLock()
	_, ok := m.assets[deviceID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("资产不存在: %s", deviceID)
	}

	m.addEvent("firmware_rollback", fmt.Sprintf("固件回滚: %s → v%s", deviceID, targetVersion), deviceID)
	m.logger.Info("固件回滚: %s → v%s", deviceID, targetVersion)
	return nil
}

// CheckFirmwareUpdates 检查固件更新.
func (m *Manager) CheckFirmwareUpdates() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	updates := make(map[string]string)
	for _, asset := range m.assets {
		latestVersion := m.findLatestFirmware(asset.Model)
		if latestVersion != "" && latestVersion != asset.FirmwareVersion {
			updates[asset.ID] = latestVersion
		}
	}
	return updates
}

func (m *Manager) findLatestFirmware(model string) string {
	var latest *FirmwareInfo
	for _, info := range m.firmwareInfo {
		if info.Model != model {
			continue
		}
		if latest == nil || info.ReleaseDate.After(latest.ReleaseDate) {
			latest = info
		}
	}
	if latest != nil {
		return latest.Version
	}
	return ""
}

// ==================== 费用统计 ====================

// AddCostRecord 添加费用记录.
func (m *Manager) AddCostRecord(record *CostRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = generateID("cost")
	}
	record.CreatedAt = time.Now()

	m.costRecords = append(m.costRecords, record)
	m.logger.Info("费用记录添加: %s %.2f %s", record.Type, record.Amount, record.Currency)
	return nil
}

// GetCostSummary 获取费用汇总.
func (m *Manager) GetCostSummary(period string) *CostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{
		ByAsset:      make(map[string]float64),
		ByType:       make(map[string]float64),
		Period:       period,
		CalculatedAt: time.Now(),
	}

	for _, record := range m.costRecords {
		switch record.Type {
		case CostTypePurchase:
			summary.TotalPurchase += record.Amount
		case CostTypeMaintenance:
			summary.TotalMaintenance += record.Amount
		case CostTypePower:
			summary.TotalPower += record.Amount
		case CostTypeNetwork:
			summary.TotalNetwork += record.Amount
		case CostTypeLicense:
			summary.TotalLicense += record.Amount
		}

		summary.ByAsset[record.AssetID] += record.Amount
		summary.ByType[string(record.Type)] += record.Amount
	}

	summary.GrandTotal = summary.TotalPurchase + summary.TotalMaintenance +
		summary.TotalPower + summary.TotalNetwork + summary.TotalLicense

	return summary
}

// CalculateDepreciation 计算资产折旧.
func (m *Manager) CalculateDepreciation(assetID string) (*DepreciationInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return nil, fmt.Errorf("资产不存在: %s", assetID)
	}

	elapsedYears := time.Since(asset.PurchaseDate).Hours() / (24 * 365.25)
	rate := m.config.DepreciationRate
	if rate <= 0 {
		rate = 0.20
	}

	// 直线折旧法
	currentValue := asset.PurchaseCost * math.Pow(1-rate, elapsedYears)
	if currentValue < 0 {
		currentValue = 0
	}

	return &DepreciationInfo{
		AssetID:          assetID,
		PurchaseCost:     asset.PurchaseCost,
		CurrentValue:     math.Round(currentValue*100) / 100,
		DepreciationRate: rate,
		DepreciationDate: time.Now(),
		UsefulLifeYears:  m.config.UsefulLifeYears,
		ElapsedYears:     math.Round(elapsedYears*100) / 100,
	}, nil
}

// GetCostRecords 获取费用记录.
func (m *Manager) GetCostRecords(assetID string, costType CostType) []*CostRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*CostRecord, 0, len(m.costRecords))
	for _, r := range m.costRecords {
		if assetID != "" && r.AssetID != assetID {
			continue
		}
		if costType != "" && r.Type != costType {
			continue
		}
		records = append(records, r)
	}
	return records
}

// ==================== 报告生成 ====================

// GenerateDeployReport 生成部署报告.
func (m *Manager) GenerateDeployReport(period string) *Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalJobs := len(m.deployJobs)
	completedJobs := 0
	failedJobs := 0
	totalSuccess := 0
	totalFail := 0

	for _, job := range m.deployJobs {
		switch job.Status {
		case JobStatusCompleted:
			completedJobs++
		case JobStatusFailed:
			failedJobs++
		}
		totalSuccess += job.SuccessCount
		totalFail += job.FailCount
	}

	report := &Report{
		ID:          generateID("rpt"),
		Type:        ReportTypeDeploy,
		Title:       "部署报告",
		Period:      period,
		GeneratedAt: time.Now(),
		Summary:     fmt.Sprintf("共 %d 个部署任务，成功 %d，失败 %d，设备成功 %d，失败 %d", totalJobs, completedJobs, failedJobs, totalSuccess, totalFail),
	}

	m.reports = append(m.reports, report)
	return report
}

// GenerateAssetReport 生成资产报告.
func (m *Manager) GenerateAssetReport(period string) *Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalAssets := len(m.assets)
	activeAssets := 0
	byType := make(map[AssetType]int)

	for _, asset := range m.assets {
		if asset.Status == AssetStatusActive {
			activeAssets++
		}
		byType[asset.Type]++
	}

	var typeInfo []string
	for t, count := range byType {
		typeInfo = append(typeInfo, fmt.Sprintf("%s: %d", t, count))
	}

	report := &Report{
		ID:          generateID("rpt"),
		Type:        ReportTypeAsset,
		Title:       "资产报告",
		Period:      period,
		GeneratedAt: time.Now(),
		Summary:     fmt.Sprintf("共 %d 台资产，活跃 %d 台。类型分布: %s", totalAssets, activeAssets, strings.Join(typeInfo, ", ")),
	}

	m.reports = append(m.reports, report)
	return report
}

// GenerateCostReport 生成费用报告.
func (m *Manager) GenerateCostReport(period string) *Report {
	summary := m.GetCostSummary(period)

	report := &Report{
		ID:          generateID("rpt"),
		Type:        ReportTypeCost,
		Title:       "费用报告",
		Period:      period,
		GeneratedAt: time.Now(),
		Summary:     fmt.Sprintf("总费用: %.2f (采购: %.2f, 维护: %.2f, 电费: %.2f)", summary.GrandTotal, summary.TotalPurchase, summary.TotalMaintenance, summary.TotalPower),
	}

	m.mu.Lock()
	m.reports = append(m.reports, report)
	m.mu.Unlock()

	return report
}

// ListReports 列出报告.
func (m *Manager) ListReports(reportType ReportType) []*Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*Report, 0, len(m.reports))
	for _, r := range m.reports {
		if reportType != "" && r.Type != reportType {
			continue
		}
		reports = append(reports, r)
	}
	return reports
}

// ==================== 统计信息 ====================

// GetStats 获取统计信息.
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalAssets:      len(m.assets),
		TotalDeployJobs:  len(m.deployJobs),
		TotalFirmwareOps: len(m.firmwareJobs),
	}

	for _, a := range m.assets {
		switch a.Status {
		case AssetStatusActive:
			stats.ActiveAssets++
		case AssetStatusInactive, AssetStatusRetired:
			stats.InactiveAssets++
		}
	}

	for _, j := range m.deployJobs {
		switch j.Status {
		case JobStatusRunning:
			stats.RunningJobs++
		case JobStatusCompleted:
			stats.CompletedJobs++
		case JobStatusFailed:
			stats.FailedJobs++
		}
	}

	summary := m.getCostSummaryInternal()
	stats.TotalCost = summary.GrandTotal

	return stats
}

// getCostSummaryInternal 内部费用汇总（调用者需持锁）.
func (m *Manager) getCostSummaryInternal() *CostSummary {
	summary := &CostSummary{
		ByAsset: make(map[string]float64),
		ByType:  make(map[string]float64),
	}

	for _, record := range m.costRecords {
		switch record.Type {
		case CostTypePurchase:
			summary.TotalPurchase += record.Amount
		case CostTypeMaintenance:
			summary.TotalMaintenance += record.Amount
		case CostTypePower:
			summary.TotalPower += record.Amount
		case CostTypeNetwork:
			summary.TotalNetwork += record.Amount
		case CostTypeLicense:
			summary.TotalLicense += record.Amount
		}
	}

	summary.GrandTotal = summary.TotalPurchase + summary.TotalMaintenance +
		summary.TotalPower + summary.TotalNetwork + summary.TotalLicense

	return summary
}

// ==================== 事件管理 ====================

// GetEvents 获取事件.
func (m *Manager) GetEvents(limit int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}
	return m.events[start:]
}

func (m *Manager) addEvent(eventType, message, assetID string) {
	event := Event{
		ID:        generateID("evt"),
		Type:      eventType,
		Message:   message,
		AssetID:   assetID,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)

	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}
}

// ==================== HTTP 路由 ====================

// RegisterRoutes 注册 HTTP 路由.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 设备发现
	mux.HandleFunc("/api/massdeploy/discover", m.handleDiscover)
	mux.HandleFunc("/api/massdeploy/discovered", m.handleDiscovered)

	// 资产管理
	mux.HandleFunc("/api/massdeploy/assets", m.handleAssets)
	mux.HandleFunc("/api/massdeploy/assets/", m.handleAssetDetail)
	mux.HandleFunc("/api/massdeploy/assets/hardware/", m.handleHardwareInfo)

	// 部署模板
	mux.HandleFunc("/api/massdeploy/templates", m.handleTemplates)
	mux.HandleFunc("/api/massdeploy/templates/", m.handleTemplateDetail)

	// 部署任务
	mux.HandleFunc("/api/massdeploy/deploy", m.handleDeploy)
	mux.HandleFunc("/api/massdeploy/deploy/", m.handleDeployDetail)
	mux.HandleFunc("/api/massdeploy/deploy/cancel/", m.handleCancelDeploy)
	mux.HandleFunc("/api/massdeploy/deploy/retry/", m.handleRetryDeploy)

	// 固件管理
	mux.HandleFunc("/api/massdeploy/firmware", m.handleFirmware)
	mux.HandleFunc("/api/massdeploy/firmware/upgrade", m.handleFirmwareUpgrade)
	mux.HandleFunc("/api/massdeploy/firmware/rollback", m.handleFirmwareRollback)
	mux.HandleFunc("/api/massdeploy/firmware/check", m.handleFirmwareCheck)

	// 费用统计
	mux.HandleFunc("/api/massdeploy/costs", m.handleCosts)
	mux.HandleFunc("/api/massdeploy/costs/summary", m.handleCostSummary)
	mux.HandleFunc("/api/massdeploy/costs/depreciation/", m.handleDepreciation)

	// 报告
	mux.HandleFunc("/api/massdeploy/reports", m.handleReports)
	mux.HandleFunc("/api/massdeploy/reports/deploy", m.handleDeployReport)
	mux.HandleFunc("/api/massdeploy/reports/asset", m.handleAssetReport)
	mux.HandleFunc("/api/massdeploy/reports/cost", m.handleCostReport)

	// 统计信息
	mux.HandleFunc("/api/massdeploy/stats", m.handleStats)

	// 事件日志
	mux.HandleFunc("/api/massdeploy/events", m.handleEvents)
}

// ==================== Stop ====================

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// ==================== 辅助函数 ====================

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}
