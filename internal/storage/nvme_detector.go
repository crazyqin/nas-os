// Package storage 提供 NVMe/SSD 智能检测和识别功能
// 通过检测设备类型，为 Fusion Pool 智能分层提供决策依据
package storage

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DeviceType 设备类型枚举.
type DeviceType int

const (
	DeviceTypeUnknown DeviceType = iota
	DeviceTypeNVMe               // NVMe SSD
	DeviceTypeSSD                // SATA/SAS SSD
	DeviceTypeHDD                // 机械硬盘
)

func (d DeviceType) String() string {
	switch d {
	case DeviceTypeNVMe:
		return "NVMe"
	case DeviceTypeSSD:
		return "SSD"
	case DeviceTypeHDD:
		return "HDD"
	default:
		return "Unknown"
	}
}

// DeviceInfo 设备信息.
type DeviceInfo struct {
	// 基本信息
	Name   string     `json:"name"`   // 设备名称，如 /dev/nvme0n1
	Path   string     `json:"path"`   // 设备路径
	Type   DeviceType `json:"type"`   // 设备类型
	Model  string     `json:"model"`  // 型号
	Serial string     `json:"serial"` // 序列号
	Vendor string     `json:"vendor"` // 厂商

	// 性能参数
	SizeBytes uint64  `json:"sizeBytes"` // 容量（字节）
	SizeGB    float64 `json:"sizeGB"`    // 容量（GB）
	BlockSize uint64  `json:"blockSize"` // 块大小

	// 性能指标（估算或实测）
	ReadSpeedMB  uint64 `json:"readSpeedMB"`  // 读取速度 MB/s
	WriteSpeedMB uint64 `json:"writeSpeedMB"` // 写入速度 MB/s
	IOPS         uint64 `json:"iops"`         // IOPS（估算）

	// SMART 信息
	SmartAvailable bool   `json:"smartAvailable"` // SMART 是否可用
	Temperature    int    `json:"temperature"`    // 温度（摄氏度）
	PowerOnHours   uint64 `json:"powerOnHours"`   // 已开机小时数
	HealthPercent  int    `json:"healthPercent"`  // 健康百分比

	// NVMe 特有属性
	NVMeController string `json:"nvmeController,omitempty"` // NVMe 控制器名称
	NVMeNamespace  string `json:"nvmeNamespace,omitempty"`  // NVMe 命名空间

	// 分层推荐
	RecommendedRole DeviceRole `json:"recommendedRole"` // 推荐角色
}

// DeviceRole 设备角色.
type DeviceRole int

const (
	RoleUnknown  DeviceRole = iota
	RoleMetadata            // 元数据专用（NVMe首选）
	RoleCache               // 缓存层（SSD/NVMe）
	RoleHotData             // 热数据存储
	RoleBulkData            // 大容量数据存储（HDD）
	RoleSpare               // 热备盘
)

func (r DeviceRole) String() string {
	switch r {
	case RoleMetadata:
		return "metadata"
	case RoleCache:
		return "cache"
	case RoleHotData:
		return "hot_data"
	case RoleBulkData:
		return "bulk_data"
	case RoleSpare:
		return "spare"
	default:
		return "unknown"
	}
}

// NVMeDetector NVMe/SSD 检测器.
type NVMeDetector struct {
	devices      map[string]*DeviceInfo
	mu           sync.RWMutex
	lastScan     time.Time
	scanInterval time.Duration
}

// NewNVMeDetector 创建检测器.
func NewNVMeDetector() *NVMeDetector {
	return &NVMeDetector{
		devices:      make(map[string]*DeviceInfo),
		scanInterval: 5 * time.Minute,
	}
}

// ScanDevices 扫描所有块设备.
func (d *NVMeDetector) ScanDevices() ([]*DeviceInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 检查是否需要重新扫描
	if time.Since(d.lastScan) < d.scanInterval && len(d.devices) > 0 {
		result := make([]*DeviceInfo, 0, len(d.devices))
		for _, dev := range d.devices {
			result = append(result, dev)
		}
		return result, nil
	}

	// 清空旧数据
	d.devices = make(map[string]*DeviceInfo)

	// 扫描 NVMe 设备
	nvmeDevices, err := d.scanNVMeDevices()
	if err == nil {
		for _, dev := range nvmeDevices {
			d.devices[dev.Name] = dev
		}
	}

	// 扫描 SATA/SAS 设备
	sataDevices, err := d.scanSATADevices()
	if err == nil {
		for _, dev := range sataDevices {
			d.devices[dev.Name] = dev
		}
	}

	d.lastScan = time.Now()

	result := make([]*DeviceInfo, 0, len(d.devices))
	for _, dev := range d.devices {
		result = append(result, dev)
	}
	return result, nil
}

// scanNVMeDevices 扫描 NVMe 设备.
func (d *NVMeDetector) scanNVMeDevices() ([]*DeviceInfo, error) {
	// 列出所有 NVMe 控制器
	controllers, err := filepath.Glob("/sys/class/nvme/nvme*")
	if err != nil {
		return nil, err
	}

	var devices []*DeviceInfo

	for _, ctrlPath := range controllers {
		ctrlName := filepath.Base(ctrlPath)

		// 获取控制器信息
		info := &DeviceInfo{
			Path:           ctrlPath,
			NVMeController: ctrlName,
			Type:           DeviceTypeNVMe,
		}

		// 读取设备名称
		devicePath := filepath.Join(ctrlPath, "device")
		if deviceFiles, err := filepath.Glob(filepath.Join(devicePath, "nvme*n*")); err == nil && len(deviceFiles) > 0 {
			info.Name = "/dev/" + filepath.Base(deviceFiles[0])
			info.NVMeNamespace = filepath.Base(deviceFiles[0])
		} else {
			// 尝试其他路径
			nsPaths, _ := filepath.Glob(filepath.Join(ctrlPath, "nvme*n*"))
			if len(nsPaths) > 0 {
				info.Name = "/dev/" + filepath.Base(nsPaths[0])
				info.NVMeNamespace = filepath.Base(nsPaths[0])
			} else {
				info.Name = "/dev/" + ctrlName + "n1"
			}
		}

		// 读取型号
		modelPath := filepath.Join(ctrlPath, "model")
		if data, err := os.ReadFile(modelPath); err == nil {
			info.Model = strings.TrimSpace(string(data))
		}

		// 读取序列号
		serialPath := filepath.Join(ctrlPath, "serial")
		if data, err := os.ReadFile(serialPath); err == nil {
			info.Serial = strings.TrimSpace(string(data))
		}

		// 读取厂商
		firmwarePath := filepath.Join(ctrlPath, "firmware_rev")
		if data, err := os.ReadFile(firmwarePath); err == nil {
			info.Vendor = strings.TrimSpace(string(data))
		}

		// 获取容量
		nsSizePath := filepath.Join(ctrlPath, "namespace", info.NVMeNamespace, "size")
		if _, err := os.Stat(nsSizePath); err == nil {
			if data, err := os.ReadFile(nsSizePath); err == nil {
				size, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
				// NVMe 块大小通常是 512 或 4096
				info.SizeBytes = size * 512 // 假设 LBA 512
				info.SizeGB = float64(info.SizeBytes) / (1024 * 1024 * 1024)
			}
		} else {
			// 从 sysfs 获取
			sizePath := filepath.Join("/sys/block", filepath.Base(info.Name), "size")
			if data, err := os.ReadFile(sizePath); err == nil {
				size, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
				info.SizeBytes = size * 512
				info.SizeGB = float64(info.SizeBytes) / (1024 * 1024 * 1024)
			}
		}

		// NVMe 性能估算（基于型号特征）
		info.ReadSpeedMB = d.estimateNVMeReadSpeed(info.Model)
		info.WriteSpeedMB = d.estimateNVMeWriteSpeed(info.Model)
		info.IOPS = d.estimateNVMeIOPS(info.Model)

		// 推荐角色：NVMe 最适合元数据存储
		info.RecommendedRole = RoleMetadata

		// 获取 SMART 信息
		d.getNVMeSmartInfo(info)

		devices = append(devices, info)
	}

	return devices, nil
}

// scanSATADevices 扫描 SATA/SAS 设备.
func (d *NVMeDetector) scanSATADevices() ([]*DeviceInfo, error) {
	// 列出所有块设备
	blockDevices, err := filepath.Glob("/sys/block/sd*")
	if err != nil {
		return nil, err
	}

	var devices []*DeviceInfo

	for _, devPath := range blockDevices {
		devName := filepath.Base(devPath)
		info := &DeviceInfo{
			Name: "/dev/" + devName,
			Path: devPath,
		}

		// 获取容量
		sizePath := filepath.Join(devPath, "size")
		if data, err := os.ReadFile(sizePath); err == nil {
			size, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			info.SizeBytes = size * 512
			info.SizeGB = float64(info.SizeBytes) / (1024 * 1024 * 1024)
		}

		// 读取型号
		modelPath := filepath.Join(devPath, "device/model")
		if data, err := os.ReadFile(modelPath); err == nil {
			info.Model = strings.TrimSpace(string(data))
		}

		// 读取序列号
		serialPath := filepath.Join(devPath, "device/serial")
		if data, err := os.ReadFile(serialPath); err == nil {
			info.Serial = strings.TrimSpace(string(data))
		}

		// 读取厂商
		vendorPath := filepath.Join(devPath, "device/vendor")
		if data, err := os.ReadFile(vendorPath); err == nil {
			info.Vendor = strings.TrimSpace(string(data))
		}

		// 判断是 SSD 还是 HDD
		info.Type = d.detectSSDOrHDD(devName, info.Model)
		info.BlockSize = 512 // SATA 默认

		// 设置性能参数和推荐角色
		if info.Type == DeviceTypeSSD {
			info.ReadSpeedMB = d.estimateSSDReadSpeed(info.Model)
			info.WriteSpeedMB = d.estimateSSDWriteSpeed(info.Model)
			info.IOPS = d.estimateSSDIOPS(info.Model)
			info.RecommendedRole = RoleCache
		} else {
			info.ReadSpeedMB = d.estimateHDDReadSpeed(info.Model)
			info.WriteSpeedMB = d.estimateHDDWriteSpeed(info.Model)
			info.IOPS = d.estimateHDDIOPS(info.Model)
			info.RecommendedRole = RoleBulkData
		}

		// 获取 SMART 信息
		d.getSmartInfo(info)

		devices = append(devices, info)
	}

	return devices, nil
}

// detectSSDOrHDD 检测是 SSD 还是 HDD.
func (d *NVMeDetector) detectSSDOrHDD(devName, model string) DeviceType {
	// 方法 1：检查 rotational 属性（最可靠）
	rotPath := filepath.Join("/sys/block", devName, "queue/rotational")
	if data, err := os.ReadFile(rotPath); err == nil {
		rotational := strings.TrimSpace(string(data))
		switch rotational {
		case "0":
			return DeviceTypeSSD
		case "1":
			return DeviceTypeHDD
		}
	}

	// 方法 2：型号特征检测
	modelLower := strings.ToLower(model)
	ssdKeywords := []string{"ssd", "solid", "flash", "nvme", "sata ssd", "sas ssd"}
	for _, kw := range ssdKeywords {
		if strings.Contains(modelLower, kw) {
			return DeviceTypeSSD
		}
	}

	// 方法 3：容量推断（大于 2TB 且非 NVMe，大概率是 HDD）
	// 但这需要先知道容量，所以作为补充

	// 默认假设是 HDD（保守）
	return DeviceTypeHDD
}

// estimateNVMeReadSpeed 估算 NVMe 读取速度.
func (d *NVMeDetector) estimateNVMeReadSpeed(model string) uint64 {
	modelLower := strings.ToLower(model)

	// Gen4 NVMe (7000+ MB/s)
	gen4Keywords := []string{"980", "990", "wd black", "rocket 4", "ex920", "s50", "firecuda 530"}
	for _, kw := range gen4Keywords {
		if strings.Contains(modelLower, strings.ToLower(kw)) {
			return 7000
		}
	}

	// Gen3 NVMe (3500 MB/s)
	gen3Keywords := []string{"970", "960", "860", "ex900", "s40", "mx500"}
	for _, kw := range gen3Keywords {
		if strings.Contains(modelLower, strings.ToLower(kw)) {
			return 3500
		}
	}

	// 保守估计：Gen3 速度
	return 2000
}

// estimateNVMeWriteSpeed 估算 NVMe 写入速度.
func (d *NVMeDetector) estimateNVMeWriteSpeed(model string) uint64 {
	// 通常写入速度略低于读取速度
	readSpeed := d.estimateNVMeReadSpeed(model)
	return readSpeed * 80 / 100 // 约 80% 的读取速度
}

// estimateNVMeIOPS 估算 NVMe IOPS.
func (d *NVMeDetector) estimateNVMeIOPS(model string) uint64 {
	modelLower := strings.ToLower(model)

	// 高端 NVMe：500K+ IOPS
	highKeywords := []string{"980", "990", "wd black", "rocket 4"}
	for _, kw := range highKeywords {
		if strings.Contains(modelLower, strings.ToLower(kw)) {
			return 500000
		}
	}

	// 中端 NVMe：300K IOPS
	return 300000
}

// estimateSSDReadSpeed 估算 SSD 读取速度.
func (d *NVMeDetector) estimateSSDReadSpeed(model string) uint64 {
	modelLower := strings.ToLower(model)

	// 高端 SATA SSD (550 MB/s)
	highKeywords := []string{"mx500", "860 evo", "860 pro", "wd blue 3d"}
	for _, kw := range highKeywords {
		if strings.Contains(modelLower, strings.ToLower(kw)) {
			return 550
		}
	}

	// 标准 SATA SSD (500 MB/s)
	return 500
}

// estimateSSDWriteSpeed 估算 SSD 写入速度.
func (d *NVMeDetector) estimateSSDWriteSpeed(model string) uint64 {
	readSpeed := d.estimateSSDReadSpeed(model)
	return readSpeed * 90 / 100
}

// estimateSSDIOPS 估算 SSD IOPS.
func (d *NVMeDetector) estimateSSDIOPS(model string) uint64 {
	modelLower := strings.ToLower(model)

	// 高端 SSD：90K IOPS
	if strings.Contains(modelLower, "pro") || strings.Contains(modelLower, "mx500") {
		return 90000
	}

	// 标准 SSD：50K IOPS
	return 50000
}

// estimateHDDReadSpeed 估算 HDD 读取速度.
func (d *NVMeDetector) estimateHDDReadSpeed(model string) uint64 {
	modelLower := strings.ToLower(model)

	// 7200 RPM HDD：150-200 MB/s
	if strings.Contains(modelLower, "7200") || strings.Contains(modelLower, "black") ||
		strings.Contains(modelLower, "gold") || strings.Contains(modelLower, "re") {
		return 200
	}

	// 5400 RPM HDD：100-150 MB/s
	return 150
}

// estimateHDDWriteSpeed 估算 HDD 写入速度.
func (d *NVMeDetector) estimateHDDWriteSpeed(model string) uint64 {
	return d.estimateHDDReadSpeed(model)
}

// estimateHDDIOPS 估算 HDD IOPS.
func (d *NVMeDetector) estimateHDDIOPS(model string) uint64 {
	modelLower := strings.ToLower(model)

	// 7200 RPM：80-100 IOPS
	if strings.Contains(modelLower, "7200") || strings.Contains(modelLower, "black") {
		return 100
	}

	// 5400 RPM：50-80 IOPS
	return 80
}

// getNVMeSmartInfo 获取 NVMe SMART 信息.
func (d *NVMeDetector) getNVMeSmartInfo(info *DeviceInfo) {
	// 使用 nvme-cli 工具
	cmd := exec.Command("nvme", "smart-log", info.Name)
	output, err := cmd.Output()
	if err != nil {
		info.SmartAvailable = false
		return
	}

	info.SmartAvailable = true

	// 解析 SMART 输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 温度
		if strings.Contains(line, "temperature") {
			re := regexp.MustCompile(`(\d+)\s*celsius`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				temp, _ := strconv.Atoi(matches[1])
				info.Temperature = temp
			}
		}

		// 开机时间
		if strings.Contains(line, "power_on_hours") {
			re := regexp.MustCompile(`(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				hours, _ := strconv.ParseUint(matches[1], 10, 64)
				info.PowerOnHours = hours
			}
		}

		// 健康度
		if strings.Contains(line, "percentage_used") {
			re := regexp.MustCompile(`(\d+)%`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				used, _ := strconv.Atoi(matches[1])
				info.HealthPercent = 100 - used
			}
		}
	}
}

// getSmartInfo 获取 SATA SMART 信息.
func (d *NVMeDetector) getSmartInfo(info *DeviceInfo) {
	// 使用 smartctl 工具
	cmd := exec.Command("smartctl", "-A", "-i", info.Name)
	output, err := cmd.Output()
	if err != nil {
		info.SmartAvailable = false
		return
	}

	info.SmartAvailable = true

	// 解析 SMART 输出
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		// 温度
		if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "194") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				temp, _ := strconv.Atoi(fields[9])
				info.Temperature = temp
			}
		}

		// 开机时间
		if strings.Contains(line, "Power_On_Hours") || strings.Contains(line, "9") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				hours, _ := strconv.ParseUint(fields[9], 10, 64)
				info.PowerOnHours = hours
			}
		}
	}
}

// GetNVMeDevices 获取所有 NVMe 设备.
func (d *NVMeDetector) GetNVMeDevices() ([]*DeviceInfo, error) {
	devices, err := d.ScanDevices()
	if err != nil {
		return nil, err
	}

	var nvmeDevices []*DeviceInfo
	for _, dev := range devices {
		if dev.Type == DeviceTypeNVMe {
			nvmeDevices = append(nvmeDevices, dev)
		}
	}
	return nvmeDevices, nil
}

// GetSSDDevices 获取所有 SSD 设备.
func (d *NVMeDetector) GetSSDDevices() ([]*DeviceInfo, error) {
	devices, err := d.ScanDevices()
	if err != nil {
		return nil, err
	}

	var ssdDevices []*DeviceInfo
	for _, dev := range devices {
		if dev.Type == DeviceTypeSSD || dev.Type == DeviceTypeNVMe {
			ssdDevices = append(ssdDevices, dev)
		}
	}
	return ssdDevices, nil
}

// GetHDDDevices 获取所有 HDD 设备.
func (d *NVMeDetector) GetHDDDevices() ([]*DeviceInfo, error) {
	devices, err := d.ScanDevices()
	if err != nil {
		return nil, err
	}

	var hddDevices []*DeviceInfo
	for _, dev := range devices {
		if dev.Type == DeviceTypeHDD {
			hddDevices = append(hddDevices, dev)
		}
	}
	return hddDevices, nil
}

// GetDevice 获取指定设备信息.
func (d *NVMeDetector) GetDevice(name string) (*DeviceInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if dev, ok := d.devices[name]; ok {
		return dev, nil
	}

	// 扫描获取
	devices, err := d.ScanDevices()
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		if dev.Name == name {
			return dev, nil
		}
	}

	return nil, fmt.Errorf("设备 %s 不存在", name)
}

// RecommendFusionPoolConfig 推荐融合池配置.
func (d *NVMeDetector) RecommendFusionPoolConfig() (*FusionPoolRecommendation, error) {
	devices, err := d.ScanDevices()
	if err != nil {
		return nil, err
	}

	rec := &FusionPoolRecommendation{
		MetadataCandidates:  []string{},
		CacheCandidates:     []string{},
		BulkDataCandidates:  []string{},
		HasNVMeForMetadata:  false,
		HasSSDForCache:      false,
		HasHDDForBulk:       false,
		RecommendedStrategy: "none",
	}

	for _, dev := range devices {
		switch dev.RecommendedRole {
		case RoleMetadata:
			rec.MetadataCandidates = append(rec.MetadataCandidates, dev.Name)
			rec.HasNVMeForMetadata = true
		case RoleCache:
			rec.CacheCandidates = append(rec.CacheCandidates, dev.Name)
			rec.HasSSDForCache = true
		case RoleBulkData:
			rec.BulkDataCandidates = append(rec.BulkDataCandidates, dev.Name)
			rec.HasHDDForBulk = true
		}
	}

	// 推荐策略
	if rec.HasNVMeForMetadata && rec.HasHDDForBulk {
		if len(rec.MetadataCandidates) >= 2 {
			rec.RecommendedStrategy = "nvme_raid1_metadata"
		} else {
			rec.RecommendedStrategy = "nvme_single_metadata"
		}
	} else if rec.HasSSDForCache && rec.HasHDDForBulk {
		rec.RecommendedStrategy = "ssd_cache"
	} else if len(rec.BulkDataCandidates) > 0 {
		rec.RecommendedStrategy = "hdd_only"
	}

	return rec, nil
}

// FusionPoolRecommendation 融合池推荐配置.
type FusionPoolRecommendation struct {
	MetadataCandidates  []string `json:"metadataCandidates"`  // 适合元数据存储的设备
	CacheCandidates     []string `json:"cacheCandidates"`     // 适合缓存的设备
	BulkDataCandidates  []string `json:"bulkDataCandidates"`  // 适合大容量存储的设备
	HasNVMeForMetadata  bool     `json:"hasNVMeForMetadata"`  // 是否有 NVMe 可用于元数据
	HasSSDForCache      bool     `json:"hasSSDForCache"`      // 是否有 SSD 可用于缓存
	HasHDDForBulk       bool     `json:"hasHDDForBulk"`       // 是否有 HDD 用于大容量
	RecommendedStrategy string   `json:"recommendedStrategy"` // 推荐策略
	Suggestions         []string `json:"suggestions"`         // 详细建议
}

// IsNVMeDevice 检查设备是否为 NVMe.
func (d *NVMeDetector) IsNVMeDevice(name string) bool {
	return strings.HasPrefix(name, "/dev/nvme")
}

// IsSSDDevice 检查设备是否为 SSD（包括 NVMe）.
func (d *NVMeDetector) IsSSDDevice(name string) bool {
	dev, err := d.GetDevice(name)
	if err != nil {
		return false
	}
	return dev.Type == DeviceTypeNVMe || dev.Type == DeviceTypeSSD
}

// GetBestMetadataDevice 获取最佳元数据设备.
func (d *NVMeDetector) GetBestMetadataDevice() (*DeviceInfo, error) {
	nvmeDevices, err := d.GetNVMeDevices()
	if err != nil {
		return nil, err
	}

	if len(nvmeDevices) == 0 {
		// 没有 NVMe，尝试 SSD
		ssdDevices, _ := d.GetSSDDevices()
		if len(ssdDevices) > 0 {
			return ssdDevices[0], nil
		}
		return nil, fmt.Errorf("没有可用于元数据存储的 SSD/NVMe 设备")
	}

	// 选择健康度最高、IOPS 最高的 NVMe
	best := nvmeDevices[0]
	for _, dev := range nvmeDevices {
		if dev.HealthPercent > best.HealthPercent || dev.IOPS > best.IOPS {
			best = dev
		}
	}
	return best, nil
}
