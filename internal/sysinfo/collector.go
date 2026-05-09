package sysinfo

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Collector 系统信息采集器。
type Collector struct {
	logger *zap.Logger
}

// NewCollector 创建系统信息采集器。
func NewCollector(logger *zap.Logger) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Collector{logger: logger}
}

// Collect 采集完整系统信息。
func (c *Collector) Collect() SystemInfo {
	info := SystemInfo{
		CollectedAt: time.Now(),
		Hostname:    c.collectHostname(),
		OS:          runtime.GOOS,
		Kernel:      c.collectKernel(),
		Arch:        runtime.GOARCH,
		Uptime:      c.collectUptime(),
		CPU:         c.CollectCPU(),
		Memory:      c.CollectMemory(),
		Disks:       c.CollectDisks(),
		Network:     c.CollectNetwork(),
		LoadAvg:     c.CollectLoadAvg(),
	}
	return info
}

// CollectCPU 采集 CPU 信息。
func (c *Collector) CollectCPU() CPUInfo {
	info := CPUInfo{
		Cores: runtime.NumCPU(),
		Model: c.collectCPUModel(),
	}

	// 读取 CPU 温度
	info.TempCelsius = c.collectCPUTemp()

	// 读取 CPU 频率
	info.Frequencies = c.collectCPUFrequencies()

	// 读取 CPU 使用率（简化：读 /proc/stat 两次取差值）
	info.Usage = c.collectCPUUsage()

	return info
}

// CollectMemory 采集内存信息。
func (c *Collector) CollectMemory() MemInfo {
	info := MemInfo{}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		c.logger.Debug("failed to read /proc/meminfo", zap.Error(err))
		return info
	}

	fields := parseMemInfo(string(data))
	info.TotalBytes = getInt64(fields, "MemTotal")
	info.FreeBytes = getInt64(fields, "MemFree")
	info.CachedBytes = getInt64(fields, "Cached") + getInt64(fields, "Buffers")
	info.SwapTotalBytes = getInt64(fields, "SwapTotal")
	swapFree := getInt64(fields, "SwapFree")
	info.SwapUsedBytes = info.SwapTotalBytes - swapFree

	info.UsedBytes = info.TotalBytes - info.FreeBytes - info.CachedBytes
	if info.UsedBytes < 0 {
		info.UsedBytes = info.TotalBytes - info.FreeBytes
	}

	info.UsagePercent = CalcMemUsagePercent(info.TotalBytes, info.UsedBytes)
	return info
}

// CollectDisks 采集磁盘信息。
func (c *Collector) CollectDisks() []DiskInfo {
	mounts := c.parseMounts()
	var disks []DiskInfo

	for _, m := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.mountPoint, &stat); err != nil {
			c.logger.Debug("statfs failed", zap.String("path", m.mountPoint), zap.Error(err))
			continue
		}

		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)
		used := total - free

		var usagePct float64
		if total > 0 {
			usagePct = float64(used) / float64(total) * 100
		}

		disk := DiskInfo{
			Device:       m.device,
			MountPoint:   m.mountPoint,
			FileSystem:   m.fsType,
			TotalBytes:   total,
			UsedBytes:    used,
			FreeBytes:    free,
			UsagePercent: usagePct,
			Health:       CalcDiskHealth(usagePct),
		}

		// 尝试读取磁盘型号和序列号
		disk.Model, disk.Serial = c.collectDiskInfo(m.device)

		disks = append(disks, disk)
	}

	return disks
}

// CollectNetwork 采集网络接口信息。
func (c *Collector) CollectNetwork() []NetInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		c.logger.Debug("failed to get network interfaces", zap.Error(err))
		return nil
	}

	var nets []NetInfo
	for _, iface := range ifaces {
		// 跳过 lo
		if iface.Name == "lo" {
			continue
		}

		info := NetInfo{
			Name: iface.Name,
			MAC:  iface.HardwareAddr.String(),
			IsUp: iface.Flags&net.FlagUp != 0,
		}

		// 获取 IP 地址
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
					info.IP = ipNet.IP.String()
					break
				}
			}
		}

		// 读取网络统计
		info.RxBytes, info.TxBytes, info.RxPackets, info.TxPackets = c.collectNetStats(iface.Name)

		// 读取链路速度
		info.Speed = c.collectNetSpeed(iface.Name)

		nets = append(nets, info)
	}

	return nets
}

// CollectLoadAvg 采集系统负载。
func (c *Collector) CollectLoadAvg() LoadAvg {
	info := LoadAvg{}

	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		c.logger.Debug("failed to read /proc/loadavg", zap.Error(err))
		return info
	}

	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		info.Load1, _ = strconv.ParseFloat(fields[0], 64)
		info.Load5, _ = strconv.ParseFloat(fields[1], 64)
		info.Load15, _ = strconv.ParseFloat(fields[2], 64)
	}

	return info
}

// ========== 内部辅助方法 ==========

func (c *Collector) collectHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

func (c *Collector) collectKernel() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "unknown"
	}
	// 版本格式: "Linux version x.y.z ..."
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		return parts[2]
	}
	return strings.TrimSpace(string(data))
}

func (c *Collector) collectUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		uptime, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return 0
		}
		return int64(uptime)
	}
	return 0
}

func (c *Collector) collectCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "unknown"
}

func (c *Collector) collectCPUTemp() float64 {
	// 尝试读取 thermal zone
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp", i)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}
		// 温度通常是毫摄氏度
		if val > 1000 {
			return val / 1000
		}
		return val
	}
	return 0
}

func (c *Collector) collectCPUFrequencies() []float64 {
	var freqs []float64
	for i := 0; i < runtime.NumCPU(); i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}
		freqs = append(freqs, val/1000) // kHz -> MHz
	}
	return freqs
}

func (c *Collector) collectCPUUsage() float64 {
	// 简化实现：读取两次 /proc/stat 计算差值
	usage1 := readCPUSample()
	if usage1 == nil {
		return 0
	}

	time.Sleep(100 * time.Millisecond)

	usage2 := readCPUSample()
	if usage2 == nil {
		return 0
	}

	totalDelta := usage2.total - usage1.total
	idleDelta := usage2.idle - usage1.idle

	if totalDelta <= 0 {
		return 0
	}

	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

type cpuSample struct {
	total int64
	idle  int64
}

func readCPUSample() *cpuSample {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return nil
		}

		var total int64
		for i := 1; i < len(fields); i++ {
			val, _ := strconv.ParseInt(fields[i], 10, 64)
			total += val
		}

		idle, _ := strconv.ParseInt(fields[4], 10, 64)
		return &cpuSample{total: total, idle: idle}
	}
	return nil
}

type mountInfo struct {
	device     string
	mountPoint string
	fsType     string
}

func (c *Collector) parseMounts() []mountInfo {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		c.logger.Debug("failed to open /proc/mounts", zap.Error(err))
		return nil
	}
	defer file.Close()

	// 只关心这些文件系统类型
	validFSTypes := map[string]bool{
		"ext4": true, "ext3": true, "ext2": true,
		"xfs": true, "btrfs": true, "zfs": true,
		"ntfs": true, "vfat": true, "fuseblk": true,
		"nfs": true, "nfs4": true, "cifs": true,
	}

	// 忽略的挂载点前缀
	ignorePrefixes := []string{"/proc", "/sys", "/dev", "/run", "/snap"}

	var mounts []mountInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// 跳过虚拟文件系统
		if !validFSTypes[fsType] {
			continue
		}

		// 跳过特殊路径
		skip := false
		for _, prefix := range ignorePrefixes {
			if strings.HasPrefix(mountPoint, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		mounts = append(mounts, mountInfo{
			device:     device,
			mountPoint: mountPoint,
			fsType:     fsType,
		})
	}

	return mounts
}

func (c *Collector) collectDiskInfo(device string) (model, serial string) {
	// 从设备路径提取磁盘名（如 /dev/sda -> sda）
	baseName := filepath.Base(device)
	// 去掉分区号，如 sda1 -> sda
	diskName := strings.TrimRight(baseName, "0123456789")

	// 读取型号
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", diskName)
	if data, err := os.ReadFile(modelPath); err == nil {
		model = strings.TrimSpace(string(data))
	}

	// 读取序列号
	serialPath := fmt.Sprintf("/sys/block/%s/device/serial", diskName)
	if data, err := os.ReadFile(serialPath); err == nil {
		serial = strings.TrimSpace(string(data))
	}

	return model, serial
}

func (c *Collector) collectNetStats(name string) (rxBytes, txBytes, rxPackets, txPackets int64) {
	basePath := fmt.Sprintf("/sys/class/net/%s/statistics", name)

	rxBytes = readSysInt64(filepath.Join(basePath, "rx_bytes"))
	txBytes = readSysInt64(filepath.Join(basePath, "tx_bytes"))
	rxPackets = readSysInt64(filepath.Join(basePath, "rx_packets"))
	txPackets = readSysInt64(filepath.Join(basePath, "tx_packets"))

	return
}

func (c *Collector) collectNetSpeed(name string) int64 {
	path := fmt.Sprintf("/sys/class/net/%s/speed", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	speed, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return speed
}

// ========== 工具函数 ==========

func parseMemInfo(content string) map[string]int64 {
	result := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		val, err := strconv.ParseInt(strings.TrimSpace(valStr), 10, 64)
		if err != nil {
			continue
		}
		result[key] = val * 1024 // kB -> bytes
	}
	return result
}

func getInt64(fields map[string]int64, key string) int64 {
	if v, ok := fields[key]; ok {
		return v
	}
	return 0
}

func readSysInt64(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return val
}
