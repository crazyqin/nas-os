package zfsenhanced

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CreatePool 创建ZFS池
func (pm *PoolManager) CreatePool(ctx context.Context, config PoolConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.pools[config.Name]; exists {
		return fmt.Errorf("pool %s already exists", config.Name)
	}

	args := []string{"create"}

	// 设置选项
	if config.Ashift > 0 {
		args = append(args, "-o", fmt.Sprintf("ashift=%d", config.Ashift))
	}
	if config.Compression != "" {
		args = append(args, "-o", fmt.Sprintf("compression=%s", config.Compression))
	}
	if config.Dedup != "" {
		args = append(args, "-o", fmt.Sprintf("dedup=%s", config.Dedup))
	}
	if config.Sync != "" {
		args = append(args, "-o", fmt.Sprintf("sync=%s", config.Sync))
	}
	if config.BlockSize > 0 {
		args = append(args, "-o", fmt.Sprintf("recordsize=%d", config.BlockSize))
	}
	if config.Comment != "" {
		args = append(args, "-o", fmt.Sprintf("comment=%s", config.Comment))
	}

	args = append(args, config.Name)

	// 添加RAID类型和磁盘
	switch config.RaidType {
	case RaidTypeStripe:
		args = append(args, config.Disks...)
	case RaidTypeMirror:
		args = append(args, "mirror")
		args = append(args, config.Disks...)
	case RaidTypeRaidz:
		args = append(args, "raidz")
		args = append(args, config.Disks...)
	case RaidTypeRaidz2:
		args = append(args, "raidz2")
		args = append(args, config.Disks...)
	case RaidTypeRaidz3:
		args = append(args, "raidz3")
		args = append(args, config.Disks...)
	default:
		return fmt.Errorf("unsupported raid type: %s", config.RaidType)
	}

	// 添加备用盘
	for _, spare := range config.Spares {
		args = append(args, "spare", spare)
	}

	cmd := exec.CommandContext(ctx, "zpool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create pool: %w, output: %s", err, string(output))
	}

	pm.pools[config.Name] = &PoolInfo{
		Name:      config.Name,
		Status:    PoolStatusOnline,
		RaidType:  config.RaidType,
		Disks:     make([]DiskInfo, 0),
		Spares:    make([]DiskInfo, 0),
		Timestamp: time.Now(),
	}

	return nil
}

// DeletePool 删除ZFS池
func (pm *PoolManager) DeletePool(ctx context.Context, name string, force bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.pools[name]; !exists {
		return fmt.Errorf("pool %s not found", name)
	}

	args := []string{"destroy"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete pool: %w, output: %s", err, string(output))
	}

	delete(pm.pools, name)
	return nil
}

// GetPoolStatus 获取池状态
func (pm *PoolManager) GetPoolStatus(ctx context.Context, name string) (*PoolInfo, error) {
	pm.mu.RLock()
	if pool, exists := pm.pools[name]; exists {
		pm.mu.RUnlock()
		return pool, nil
	}
	pm.mu.RUnlock()

	// 从系统获取
	return pm.fetchPoolStatus(ctx, name)
}

// ListPools 列出所有池
func (pm *PoolManager) ListPools(ctx context.Context) ([]PoolInfo, error) {
	pm.mu.RLock()
	if len(pm.pools) > 0 {
		pools := make([]PoolInfo, 0, len(pm.pools))
		for _, p := range pm.pools {
			pools = append(pools, *p)
		}
		pm.mu.RUnlock()
		return pools, nil
	}
	pm.mu.RUnlock()

	return pm.fetchAllPools(ctx)
}

// GetPoolIOStats 获取池IO统计
func (pm *PoolManager) GetPoolIOStats(ctx context.Context, name string) (map[string]int64, error) {
	cmd := exec.CommandContext(ctx, "zpool", "iostat", "-vy", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get iostat: %w", err)
	}

	stats := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			if strings.Contains(line, "alloc") || strings.Contains(line, "free") {
				continue
			}
			// 尝试解析数值
			for i, f := range fields[1:] {
				val, err := strconv.ParseInt(f, 10, 64)
				if err == nil {
					switch i {
					case 0:
						stats["alloc"] = val
					case 1:
						stats["free"] = val
					}
				}
			}
		}
	}

	return stats, nil
}

// ExpandPool 扩展池
func (pm *PoolManager) ExpandPool(ctx context.Context, name string) error {
	pm.mu.RLock()
	_, exists := pm.pools[name]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "zpool", "online", "-e", name, "auto")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to expand pool: %w, output: %s", err, string(output))
	}

	return nil
}

// AddDisk 添加磁盘到池
func (pm *PoolManager) AddDisk(ctx context.Context, poolName, diskPath, vdevType string) error {
	pm.mu.RLock()
	_, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", poolName)
	}

	args := []string{"add"}
	if vdevType != "" {
		args = append(args, vdevType)
	}
	args = append(args, poolName, diskPath)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add disk: %w, output: %s", err, string(output))
	}

	return nil
}

// RemoveDisk 从池中移除磁盘
func (pm *PoolManager) RemoveDisk(ctx context.Context, poolName, diskPath string) error {
	pm.mu.RLock()
	_, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", poolName)
	}

	cmd := exec.CommandContext(ctx, "zpool", "remove", poolName, diskPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove disk: %w, output: %s", err, string(output))
	}

	return nil
}

// AttachDisk 附加磁盘（镜像）
func (pm *PoolManager) AttachDisk(ctx context.Context, poolName, existingDisk, newDisk string) error {
	pm.mu.RLock()
	_, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", poolName)
	}

	cmd := exec.CommandContext(ctx, "zpool", "attach", poolName, existingDisk, newDisk)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach disk: %w, output: %s", err, string(output))
	}

	return nil
}

// DetachDisk 分离磁盘
func (pm *PoolManager) DetachDisk(ctx context.Context, poolName, diskPath string) error {
	pm.mu.RLock()
	_, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", poolName)
	}

	cmd := exec.CommandContext(ctx, "zpool", "detach", poolName, diskPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to detach disk: %w, output: %s", err, string(output))
	}

	return nil
}

// ReplaceDisk 替换磁盘
func (pm *PoolManager) ReplaceDisk(ctx context.Context, poolName, oldDisk, newDisk string) error {
	pm.mu.RLock()
	_, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", poolName)
	}

	cmd := exec.CommandContext(ctx, "zpool", "replace", poolName, oldDisk, newDisk)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to replace disk: %w, output: %s", err, string(output))
	}

	return nil
}

// ScrubPool 执行scrub
func (pm *PoolManager) ScrubPool(ctx context.Context, name string) error {
	pm.mu.RLock()
	_, exists := pm.pools[name]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "zpool", "scrub", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to scrub pool: %w, output: %s", err, string(output))
	}

	return nil
}

// CancelScrub 取消scrub
func (pm *PoolManager) CancelScrub(ctx context.Context, name string) error {
	pm.mu.RLock()
	_, exists := pm.pools[name]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "zpool", "scrub", "-s", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to cancel scrub: %w, output: %s", err, string(output))
	}

	return nil
}

// ExportPool 导出池
func (pm *PoolManager) ExportPool(ctx context.Context, name string, force bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	args := []string{"export"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to export pool: %w, output: %s", err, string(output))
	}

	delete(pm.pools, name)
	return nil
}

// ImportPool 导入池
func (pm *PoolManager) ImportPool(ctx context.Context, name string, force bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	args := []string{"import"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import pool: %w, output: %s", err, string(output))
	}

	pm.pools[name] = &PoolInfo{
		Name:      name,
		Status:    PoolStatusOnline,
		Timestamp: time.Now(),
	}

	return nil
}

// GetPoolProperties 获取池属性
func (pm *PoolManager) GetPoolProperties(ctx context.Context, name string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "zpool", "get", "all", name, "-H", "-p")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get properties: %w", err)
	}

	props := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			props[fields[1]] = fields[2]
		}
	}

	return props, nil
}

// SetPoolProperty 设置池属性
func (pm *PoolManager) SetPoolProperty(ctx context.Context, name, key, value string) error {
	pm.mu.RLock()
	_, exists := pm.pools[name]
	pm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("pool %s not found", name)
	}

	cmd := exec.CommandContext(ctx, "zpool", "set", fmt.Sprintf("%s=%s", key, value), name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set property: %w, output: %s", err, string(output))
	}

	return nil
}

// GetAlerts 获取告警列表
func (pm *PoolManager) GetAlerts() []Alert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	alerts := make([]Alert, len(pm.alerts))
	copy(alerts, pm.alerts)
	return alerts
}

// AcknowledgeAlert 确认告警
func (pm *PoolManager) AcknowledgeAlert(alertID, ackedBy string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i := range pm.alerts {
		if pm.alerts[i].ID == alertID {
			pm.alerts[i].Acked = true
			pm.alerts[i].AckedAt = time.Now()
			pm.alerts[i].AckedBy = ackedBy
			return nil
		}
	}

	return fmt.Errorf("alert %s not found", alertID)
}

// ResolveAlert 解决告警
func (pm *PoolManager) ResolveAlert(alertID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i := range pm.alerts {
		if pm.alerts[i].ID == alertID {
			pm.alerts[i].Resolved = true
			pm.alerts[i].ResolvedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("alert %s not found", alertID)
}

// CheckPoolAlerts 检查池告警
func (pm *PoolManager) CheckPoolAlerts(ctx context.Context, poolName string) []Alert {
	pm.mu.RLock()
	pool, exists := pm.pools[poolName]
	pm.mu.RUnlock()
	if !exists {
		return nil
	}

	var newAlerts []Alert

	// 检查容量告警
	if pool.UsedPercent >= pm.alertConfig.CapacityEmergencyPercent {
		newAlerts = append(newAlerts, Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Type:      AlertTypeCapacityThreshold,
			Severity:  AlertSeverityEmergency,
			PoolName:  poolName,
			Message:   fmt.Sprintf("Pool %s capacity at %.1f%% (emergency threshold)", poolName, pool.UsedPercent),
			Timestamp: time.Now(),
		})
	} else if pool.UsedPercent >= pm.alertConfig.CapacityCriticalPercent {
		newAlerts = append(newAlerts, Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Type:      AlertTypeCapacityThreshold,
			Severity:  AlertSeverityCritical,
			PoolName:  poolName,
			Message:   fmt.Sprintf("Pool %s capacity at %.1f%% (critical threshold)", poolName, pool.UsedPercent),
			Timestamp: time.Now(),
		})
	} else if pool.UsedPercent >= pm.alertConfig.CapacityWarningPercent {
		newAlerts = append(newAlerts, Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Type:      AlertTypeCapacityThreshold,
			Severity:  AlertSeverityWarning,
			PoolName:  poolName,
			Message:   fmt.Sprintf("Pool %s capacity at %.1f%% (warning threshold)", poolName, pool.UsedPercent),
			Timestamp: time.Now(),
		})
	}

	// 检查池状态
	if pool.Status == PoolStatusDegraded {
		newAlerts = append(newAlerts, Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Type:      AlertTypePoolDegraded,
			Severity:  AlertSeverityCritical,
			PoolName:  poolName,
			Message:   fmt.Sprintf("Pool %s is degraded", poolName),
			Timestamp: time.Now(),
		})
	}

	// 检查校验和错误
	if pool.ChecksumErrors > pm.alertConfig.ChecksumErrorThreshold {
		newAlerts = append(newAlerts, Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Type:      AlertTypeChecksumError,
			Severity:  AlertSeverityWarning,
			PoolName:  poolName,
			Message:   fmt.Sprintf("Pool %s has %d checksum errors", poolName, pool.ChecksumErrors),
			Timestamp: time.Now(),
		})
	}

	pm.mu.Lock()
	pm.alerts = append(pm.alerts, newAlerts...)
	pm.mu.Unlock()

	return newAlerts
}

// UpdateAlertConfig 更新告警配置
func (pm *PoolManager) UpdateAlertConfig(config AlertConfig) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alertConfig = config
}

// GetAlertConfig 获取告警配置
func (pm *PoolManager) GetAlertConfig() AlertConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.alertConfig
}

// --- 内部方法 ---

func (pm *PoolManager) fetchPoolStatus(ctx context.Context, name string) (*PoolInfo, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", name, "-P", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get pool status: %w", err)
	}

	pool := parseZpoolStatus(string(output), name)

	pm.mu.Lock()
	pm.pools[name] = pool
	pm.mu.Unlock()

	return pool, nil
}

func (pm *PoolManager) fetchAllPools(ctx context.Context) ([]PoolInfo, error) {
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "name,size,allocated,free,cap,health,fragmentation")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list pools: %w", err)
	}

	var pools []PoolInfo
	pm.mu.Lock()
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 7 {
			pool := PoolInfo{
				Name:      fields[0],
				Timestamp: time.Now(),
			}

			// 解析大小
			pool.SizeBytes = parseSize(fields[1])
			pool.UsedBytes = parseSize(fields[2])
			pool.FreeBytes = parseSize(fields[3])

			// 解析容量百分比
			capStr := strings.TrimSuffix(fields[4], "%")
			pool.UsedPercent, _ = strconv.ParseFloat(capStr, 64)

			// 解析健康状态
			pool.Health = fields[5]
			pool.Status = PoolStatus(fields[5])

			// 解析碎片化
			fragStr := strings.TrimSuffix(fields[6], "%")
			pool.Fragmentation, _ = strconv.ParseFloat(fragStr, 64)

			pool.Disks = make([]DiskInfo, 0)
			pool.Spares = make([]DiskInfo, 0)
			pool.Properties = make(map[string]string)

			pools = append(pools, pool)
			pm.pools[pool.Name] = &pool
		}
	}
	pm.mu.Unlock()

	return pools, nil
}

func parseZpoolStatus(output, poolName string) *PoolInfo {
	pool := &PoolInfo{
		Name:       poolName,
		Disks:      make([]DiskInfo, 0),
		Spares:     make([]DiskInfo, 0),
		Properties: make(map[string]string),
		Timestamp:  time.Now(),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	inConfig := false
	inScan := false

	// 正则表达式
	statusRe := regexp.MustCompile(`^\s+state:\s+(\S+)`)
	scanRe := regexp.MustCompile(`^\s+scan:\s+(.+)`)
	configRe := regexp.MustCompile(`^\s+config:`)
	diskRe := regexp.MustCompile(`^\s+(\S+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// 解析状态
		if matches := statusRe.FindStringSubmatch(line); len(matches) > 1 {
			pool.Status = PoolStatus(matches[1])
			pool.Health = matches[1]
			continue
		}

		// 解析扫描状态
		if matches := scanRe.FindStringSubmatch(line); len(matches) > 1 {
			pool.ScanStatus = matches[1]
			inScan = true
			continue
		}

		if inScan {
			// 解析扫描进度
			progressRe := regexp.MustCompile(`(\d+(?:\.\d+)?)% done`)
			if matches := progressRe.FindStringSubmatch(line); len(matches) > 1 {
				pool.ScanProgress, _ = strconv.ParseFloat(matches[1], 64)
			}
			if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
				inScan = false
			}
		}

		// 检测配置段
		if configRe.MatchString(line) {
			inConfig = true
			continue
		}

		// 解析磁盘信息
		if inConfig {
			if matches := diskRe.FindStringSubmatch(line); len(matches) > 4 {
				disk := DiskInfo{
					Name:  matches[1],
					State: matches[2],
				}
				disk.ReadErrors, _ = strconv.ParseInt(matches[3], 10, 64)
				disk.WriteErrors, _ = strconv.ParseInt(matches[4], 10, 64)
				disk.ChecksumErrors, _ = strconv.ParseInt(matches[5], 10, 64)

				switch disk.State {
				case "ONLINE":
					disk.Status = PoolStatusOnline
				case "DEGRADED":
					disk.Status = PoolStatusDegraded
				case "FAULTED":
					disk.Status = PoolStatusFaulted
				case "OFFLINE":
					disk.Status = PoolStatusOffline
				case "UNAVAIL":
					disk.Status = PoolStatusUnavail
				case "REMOVED":
					disk.Status = PoolStatusRemoved
				}

				// 判断是否是spare
				if strings.Contains(strings.ToLower(line), "spare") {
					disk.IsSpare = true
					pool.Spares = append(pool.Spares, disk)
				} else {
					pool.Disks = append(pool.Disks, disk)
				}

				// 累加错误数
				pool.ReadErrors += disk.ReadErrors
				pool.WriteErrors += disk.WriteErrors
				pool.ChecksumErrors += disk.ChecksumErrors
			}
		}
	}

	return pool
}

func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}

	// 移除逗号
	s = strings.ReplaceAll(s, ",", "")

	// 提取数字和单位
	re := regexp.MustCompile(`^([\d.]+)([KMGTP]?)([B]?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(s))
	if len(matches) < 2 {
		val, _ := strconv.ParseInt(s, 10, 64)
		return val
	}

	val, _ := strconv.ParseFloat(matches[1], 64)
	unit := matches[2]

	switch unit {
	case "K":
		return int64(val * 1024)
	case "M":
		return int64(val * 1024 * 1024)
	case "G":
		return int64(val * 1024 * 1024 * 1024)
	case "T":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "P":
		return int64(val * 1024 * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(val)
	}
}
