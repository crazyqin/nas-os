package driveinsight

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Collector 数据采集器。
// 采集磁盘统计信息，计算每 TB 成本，分析温度趋势。
type Collector struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	drives      map[string]*DriveStats   // serial -> stats
	tempHistory map[string][]TempReading // serial -> temperature readings
	tiers       []StorageTier
	tierCosts   map[TierType]float64 // 每层每 TB 月成本
}

// NewCollector 创建数据采集器。
func NewCollector(logger *zap.Logger) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Collector{
		logger:      logger,
		drives:      make(map[string]*DriveStats),
		tempHistory: make(map[string][]TempReading),
		tiers:       make([]StorageTier, 0),
		tierCosts:   defaultTierCosts(),
	}
}

// defaultTierCosts 默认各类型每 TB 月成本（元）。
func defaultTierCosts() map[TierType]float64 {
	return map[TierType]float64{
		TierTypeNVMe:    800,
		TierTypeSSD:     400,
		TierTypeHDD:     80,
		TierTypeHybrid:  200,
		TierTypeCloud:   150,
		TierTypeArchive: 20,
	}
}

// SetTierCosts 设置自定义层级成本。
func (c *Collector) SetTierCosts(costs map[TierType]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tierCosts = costs
}

// CollectDrive 采集单个磁盘统计信息。
// 在实际系统中会读取 /sys/class/block/、smartctl 等数据源；
// 本实现提供可注入的接口，方便测试与扩展。
func (c *Collector) CollectDrive(stats DriveStats) error {
	if stats.SerialNumber == "" {
		return fmt.Errorf("磁盘序列号不能为空")
	}
	if stats.CapacityBytes <= 0 {
		return fmt.Errorf("磁盘容量必须大于0: %s", stats.SerialNumber)
	}

	// 计算已用/可用
	if stats.UsedBytes > stats.CapacityBytes {
		return fmt.Errorf("已用容量超过总容量: %s", stats.SerialNumber)
	}
	stats.FreeBytes = stats.CapacityBytes - stats.UsedBytes
	stats.LastUpdated = time.Now()

	// 根据温度推断健康状态
	if stats.HealthStatus == "" {
		stats.HealthStatus = inferHealthFromTemp(stats.TemperatureC)
	}

	c.mu.Lock()
	c.drives[stats.SerialNumber] = &stats
	c.mu.Unlock()

	// 记录温度历史
	c.recordTemperature(stats.SerialNumber, stats.TemperatureC)

	c.logger.Info("采集磁盘统计",
		zap.String("serial", stats.SerialNumber),
		zap.String("model", stats.Model),
		zap.Int64("capacity_gb", stats.CapacityBytes/(1024*1024*1024)),
		zap.Float64("temp_c", stats.TemperatureC),
	)

	return nil
}

// recordTemperature 记录温度读数。
func (c *Collector) recordTemperature(serial string, tempC float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reading := TempReading{
		Timestamp:    time.Now(),
		TemperatureC: tempC,
	}

	// 最多保留 288 个读数（24小时，每5分钟一次）
	const maxReadings = 288
	history := c.tempHistory[serial]
	history = append(history, reading)
	if len(history) > maxReadings {
		history = history[len(history)-maxReadings:]
	}
	c.tempHistory[serial] = history
}

// GetDrive 获取磁盘统计。
func (c *Collector) GetDrive(serial string) (*DriveStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	drive, ok := c.drives[serial]
	if !ok {
		return nil, fmt.Errorf("磁盘不存在: %s", serial)
	}
	return drive, nil
}

// GetAllDrives 获取所有磁盘统计。
func (c *Collector) GetAllDrives() []DriveStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]DriveStats, 0, len(c.drives))
	for _, d := range c.drives {
		result = append(result, *d)
	}
	return result
}

// GetTemperatureTrend 获取磁盘温度趋势。
func (c *Collector) GetTemperatureTrend(serial string) (*TemperatureTrend, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	readings, ok := c.tempHistory[serial]
	if !ok || len(readings) == 0 {
		return nil, fmt.Errorf("无温度数据: %s", serial)
	}

	trend := &TemperatureTrend{
		SerialNumber: serial,
		Readings:     make([]TempReading, len(readings)),
	}
	copy(trend.Readings, readings)

	// 计算最小/最大/平均
	trend.MinTemp = readings[0].TemperatureC
	trend.MaxTemp = readings[0].TemperatureC
	sum := 0.0
	for _, r := range readings {
		if r.TemperatureC < trend.MinTemp {
			trend.MinTemp = r.TemperatureC
		}
		if r.TemperatureC > trend.MaxTemp {
			trend.MaxTemp = r.TemperatureC
		}
		sum += r.TemperatureC
	}
	trend.AvgTemp = sum / float64(len(readings))

	// 判断趋势方向：比较后半段与前半段平均值
	trend.Trend = calculateTempTrend(readings)

	return trend, nil
}

// calculateTempTrend 计算温度趋势方向。
func calculateTempTrend(readings []TempReading) TempTrendDirection {
	n := len(readings)
	if n < 4 {
		return TempTrendStable
	}

	mid := n / 2
	firstHalfAvg := 0.0
	for _, r := range readings[:mid] {
		firstHalfAvg += r.TemperatureC
	}
	firstHalfAvg /= float64(mid)

	secondHalfAvg := 0.0
	for _, r := range readings[mid:] {
		secondHalfAvg += r.TemperatureC
	}
	secondHalfAvg /= float64(n - mid)

	diff := secondHalfAvg - firstHalfAvg
	// 温差超过 2°C 才认为有趋势
	if diff > 2 {
		return TempTrendRising
	}
	if diff < -2 {
		return TempTrendFalling
	}
	return TempTrendStable
}

// inferHealthFromTemp 根据温度推断健康状态。
func inferHealthFromTemp(tempC float64) HealthStatus {
	switch {
	case tempC == 0:
		return HealthUnknown
	case tempC < 50:
		return HealthGood
	case tempC < 65:
		return HealthWarning
	default:
		return HealthCritical
	}
}

// CollectFromSysfs 从 /sys 文件系统采集磁盘信息。
// 在 Linux 系统上读取 /sys/class/block/ 下的磁盘信息。
func (c *Collector) CollectFromSysfs() error {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return fmt.Errorf("读取 /sys/class/block 失败: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过分区和 loop 设备
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") {
			continue
		}
		if strings.Contains(name, "p") && !strings.HasPrefix(name, "nvme") {
			continue // 跳过分区 (sda1, sdb2 等)
		}

		devicePath := filepath.Join("/dev", name)
		stats, err := readSysfsDrive(name, devicePath)
		if err != nil {
			c.logger.Debug("跳过设备，无法读取信息", zap.String("device", name), zap.Error(err))
			continue
		}
		if err := c.CollectDrive(*stats); err != nil {
			c.logger.Debug("跳过设备，数据无效", zap.String("device", name), zap.Error(err))
		}
	}

	return nil
}

// readSysfsDrive 从 sysfs 读取单个磁盘信息。
func readSysfsDrive(blockName, devicePath string) (*DriveStats, error) {
	base := filepath.Join("/sys/class/block", blockName)

	// 读取容量
	sizeBytes, err := readSysfsSize(base)
	if err != nil {
		return nil, err
	}
	if sizeBytes == 0 {
		return nil, fmt.Errorf("设备容量为0: %s", blockName)
	}

	// 读取型号
	model, _ := readSysfsFile(base, "device/model")
	model = strings.TrimSpace(model)

	// 读取序列号
	serial, _ := readSysfsFile(base, "device/wwn")
	serial = strings.TrimSpace(serial)
	if serial == "" {
		serial = fmt.Sprintf("sysfs-%s", blockName)
	}

	// 读取温度（如果有）
	tempC := 0.0
	tempStr, err := readSysfsFile(base, "device/hwmon/hwmon*/temp1_input")
	if err == nil && tempStr != "" {
		var tempMilli float64
		if _, err := fmt.Sscanf(tempStr, "%f", &tempMilli); err == nil {
			tempC = tempMilli / 1000.0
		}
	}

	// 判断磁盘类型
	driveType := DriveTypeHDD
	if strings.HasPrefix(blockName, "nvme") {
		driveType = DriveTypeNVMe
	} else if strings.Contains(strings.ToLower(model), "ssd") {
		driveType = DriveTypeSSD
	}

	stats := &DriveStats{
		SerialNumber:  serial,
		Model:         model,
		DevicePath:    devicePath,
		Interface:     inferInterface(blockName),
		Type:          driveType,
		CapacityBytes: sizeBytes,
		UsedBytes:     0, // 需要从文件系统层面获取
		FreeBytes:     sizeBytes,
		HealthStatus:  inferHealthFromTemp(tempC),
		TemperatureC:  tempC,
		LastUpdated:   time.Now(),
	}

	return stats, nil
}

// readSysfsFile 读取 sysfs 文件内容。
func readSysfsFile(base, relPath string) (string, error) {
	// 支持 glob 模式
	matches, err := filepath.Glob(filepath.Join(base, relPath))
	if err != nil || len(matches) == 0 {
		// 尝试直接读取
		data, err := os.ReadFile(filepath.Join(base, relPath))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readSysfsSize 读取磁盘大小（字节）。
func readSysfsSize(base string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(base, "size"))
	if err != nil {
		return 0, err
	}
	var sectors int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &sectors); err != nil {
		return 0, err
	}
	// 扇区大小为 512 字节
	return sectors * 512, nil
}

// inferInterface 从设备名推断接口类型。
func inferInterface(blockName string) string {
	switch {
	case strings.HasPrefix(blockName, "nvme"):
		return "NVMe"
	case strings.HasPrefix(blockName, "sd"):
		return "SATA/SAS"
	case strings.HasPrefix(blockName, "mmcblk"):
		return "MMC"
	case strings.HasPrefix(blockName, "usb"):
		return "USB"
	default:
		return "Unknown"
	}
}

// RegisterTier 注册存储层。
func (c *Collector) RegisterTier(tier StorageTier) error {
	if tier.Name == "" {
		return fmt.Errorf("存储层名称不能为空")
	}
	if tier.CapacityBytes <= 0 {
		return fmt.Errorf("存储层容量必须大于0: %s", tier.Name)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 更新已存在的层或追加
	for i, t := range c.tiers {
		if t.Name == tier.Name {
			c.tiers[i] = tier
			return nil
		}
	}
	c.tiers = append(c.tiers, tier)
	return nil
}

// GetTiers 获取所有存储层。
func (c *Collector) GetTiers() []StorageTier {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]StorageTier, len(c.tiers))
	copy(result, c.tiers)
	return result
}

// CalculateCostReport 计算成本报告。
// 基于 NVMe/SSD/HDD 等各层的容量、使用量和每 TB 成本生成完整成本分析。
func (c *Collector) CalculateCostReport() *CostReport {
	c.mu.RLock()
	defer c.mu.RUnlock()

	report := &CostReport{
		GeneratedAt: time.Now(),
	}

	var totalCapacityTB, totalUsedTB, totalMonthlyCost float64
	var weightedSum float64

	tierItems := make([]TierCostItem, 0, len(c.tiers))

	for _, tier := range c.tiers {
		capacityTB := bytesToTB(tier.CapacityBytes)
		usedTB := bytesToTB(tier.UsedBytes)
		costPerTB := tier.CostPerTB
		if costPerTB == 0 {
			costPerTB = c.tierCosts[tier.Type]
		}

		monthlyCost := capacityTB * costPerTB
		yearlyCost := monthlyCost * 12

		tierItems = append(tierItems, TierCostItem{
			TierName:    tier.Name,
			TierType:    tier.Type,
			CapacityTB:  capacityTB,
			UsedTB:      usedTB,
			CostPerTB:   costPerTB,
			MonthlyCost: monthlyCost,
			YearlyCost:  yearlyCost,
		})

		totalCapacityTB += capacityTB
		totalUsedTB += usedTB
		totalMonthlyCost += monthlyCost
		weightedSum += capacityTB * costPerTB
	}

	report.TierCosts = tierItems
	report.TotalCapacityTB = totalCapacityTB
	report.TotalUsedTB = totalUsedTB
	report.TotalMonthlyCost = totalMonthlyCost
	report.TotalYearlyCost = totalMonthlyCost * 12

	if totalCapacityTB > 0 {
		report.AvgCostPerTB = weightedSum / totalCapacityTB
	}

	// 计算分层优化可节省的成本
	// 假设将冷数据从高性能层迁移到 HDD/Archive 层
	report.PotentialSavings = c.estimateSavings()
	if totalMonthlyCost > 0 {
		report.SavingsPercent = report.PotentialSavings / totalMonthlyCost * 100
	}

	return report
}

// estimateSavings 估算通过分层可节省的月成本。
func (c *Collector) estimateSavings() float64 {
	var savings float64
	for _, tier := range c.tiers {
		// 高性能层（NVMe/SSD）中未使用的部分可以释放
		if tier.Type == TierTypeNVMe || tier.Type == TierTypeSSD {
			unusedTB := bytesToTB(tier.FreeBytes)
			hddCost := c.tierCosts[TierTypeHDD]
			tierCost := tier.CostPerTB
			if tierCost == 0 {
				tierCost = c.tierCosts[tier.Type]
			}
			// 将空闲的高性能层容量迁移到 HDD 可节省差价
			savings += unusedTB * (tierCost - hddCost)
		}
	}
	// 只保留正的节省值
	return math.Max(0, savings)
}

// bytesToTB 将字节转换为 TB。
func bytesToTB(bytes int64) float64 {
	return float64(bytes) / float64(1024*1024*1024*1024)
}

// bytesToGB 将字节转换为 GB。
func bytesToGB(bytes int64) float64 {
	return float64(bytes) / float64(1024*1024*1024)
}

// CollectFileAccessPatterns 采集指定路径下的文件访问模式。
// 用于冷热数据识别，遍历目录收集文件的修改时间、访问时间等信息。
func (c *Collector) CollectFileAccessPatterns(rootPath string, maxDepth int) ([]FileAccessPattern, error) {
	if rootPath == "" {
		return nil, fmt.Errorf("路径不能为空")
	}
	if _, err := os.Stat(rootPath); err != nil {
		return nil, fmt.Errorf("路径不可访问: %w", err)
	}

	var patterns []FileAccessPattern
	var mu sync.Mutex
	depth := 0
	if maxDepth <= 0 {
		maxDepth = 10
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Debug("跳过无法访问的文件", zap.String("path", path), zap.Error(err))
			return nil
		}

		// 深度控制
		rel, _ := filepath.Rel(rootPath, path)
		currentDepth := strings.Count(rel, string(os.PathSeparator))
		if currentDepth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 只处理文件
		if info.IsDir() {
			return nil
		}

		pattern := FileAccessPattern{
			Path:       path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			AccessTime: getAccessTime(path, info),
		}
		pattern.AccessFreq = classifyAccessFreq(pattern.AccessTime, pattern.ModTime)
		pattern.DataTier = recommendTier(pattern.AccessFreq, pattern.ModTime)

		mu.Lock()
		patterns = append(patterns, pattern)
		mu.Unlock()

		return nil
	})

	_ = depth
	if err != nil {
		return nil, fmt.Errorf("遍历路径失败: %w", err)
	}

	c.logger.Info("采集文件访问模式完成",
		zap.String("path", rootPath),
		zap.Int("files", len(patterns)),
	)

	return patterns, nil
}

// getAccessTime 获取文件最后访问时间。
func getAccessTime(path string, info os.FileInfo) time.Time {
	// 尝试使用 os.Stat 获取 atime（Linux 上有效）
	stat, ok := info.Sys().(*syscall.Stat_t)
	if ok {
		return time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	}
	// 回退到 ModTime
	return info.ModTime()
}

// classifyAccessFreq 根据访问时间和修改时间分类访问频率。
func classifyAccessFreq(accessTime, modTime time.Time) AccessFreq {
	now := time.Now()
	daysSinceAccess := now.Sub(accessTime).Hours() / 24

	switch {
	case daysSinceAccess <= 7:
		return AccessFreqHot
	case daysSinceAccess <= 30:
		return AccessFreqWarm
	case daysSinceAccess <= 90:
		return AccessFreqCold
	default:
		return AccessFreqFrozen
	}
}

// recommendTier 根据访问频率推荐存储层。
func recommendTier(freq AccessFreq, modTime time.Time) TierID {
	switch freq {
	case AccessFreqHot:
		return TierIDHot
	case AccessFreqWarm:
		return TierIDWarm
	case AccessFreqCold:
		return TierIDCold
	case AccessFreqFrozen:
		return TierIDArchive
	default:
		return TierIDWarm
	}
}
