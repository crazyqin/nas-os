package armadapter

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== 硬件检测器 ==========

// Detector ARM 硬件检测器
type Detector struct {
	mu       sync.RWMutex
	info     *ARMHardwareInfo
	detected bool
}

// NewDetector 创建硬件检测器
func NewDetector() *Detector {
	return &Detector{}
}

// Detect 检测 ARM 硬件信息
func (d *Detector) Detect() (*ARMHardwareInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.detected {
		return d.info, nil
	}

	info := &ARMHardwareInfo{
		DetectedAt:  nowFunc(),
		Endianness:  "little",
		StorageType: "eMMC", // 默认
	}

	// 检测架构类型
	info.ArchType = d.detectArchType()
	info.ArchVersion = d.detectArchVersion()
	info.Bits = d.detectBits()

	// 检测 CPU 信息
	info.SoC, info.SoCModel = d.detectSoC()
	info.CPUCores = d.detectCPUCores()
	info.MaxFreqMHz = d.detectMaxFreq()
	info.Features = d.detectFeatures()

	// 检测 big.LITTLE 配置
	info.BigCores, info.LittleCores = d.detectBigLittle(info.CPUCores)

	// 检测内存
	info.MemoryMB = d.detectMemory()
	info.LPDDRType = d.detectLPDDRType()

	// 检测存储能力
	info.HasUSB3 = d.detectUSB3()
	info.HasPCIe = d.detectPCIe()
	info.HasSATA = d.detectSATA()

	// 检测网络能力
	info.HasGbE, info.Has2_5GbE = d.detectEthernet()
	info.HasWiFi, info.WiFiVersion = d.detectWiFi()

	d.info = info
	d.detected = true

	log.Printf("[ARM适配] 检测完成: arch=%s soc=%s cores=%d memory=%dMB",
		info.ArchType, info.SoCModel, info.CPUCores, info.MemoryMB)

	return info, nil
}

// GetInfo 获取已检测的硬件信息（需先调用 Detect）
func (d *Detector) GetInfo() (*ARMHardwareInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.detected {
		return nil, fmt.Errorf("hardware not detected, call Detect() first")
	}
	return d.info, nil
}

// Reset 重置检测器，强制重新检测
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.detected = false
	d.info = nil
}

// ========== 架构检测 ==========

// detectArchType 检测 ARM 架构类型
func (d *Detector) detectArchType() ArchType {
	goarch := runtime.GOARCH
	switch goarch {
	case "arm64":
		return ArchARM64
	case "arm":
		// 需要进一步检查 ARM 版本
		return d.detectARMVersion()
	default:
		return ArchUnknown
	}
}

// detectARMVersion 检测 32 位 ARM 版本
func (d *Detector) detectARMVersion() ArchType {
	cpuinfo, err := readCPUInfo()
	if err != nil {
		return ArchARMv7 // 默认假设 v7
	}

	// 检查 CPU 架构行
	if arch, ok := cpuinfo["CPU architecture"]; ok {
		ver, err := strconv.Atoi(arch)
		if err == nil {
			switch {
			case ver >= 8:
				return ArchARM64
			case ver == 7:
				return ArchARMv7
			case ver == 6:
				return ArchARMv6
			default:
				return ArchARMv5
			}
		}
	}

	// 检查 CPU part 推断版本
	if part, ok := cpuinfo["CPU part"]; ok {
		return d.inferArchFromPart(part)
	}

	return ArchARMv7
}

// inferArchFromPart 从 CPU part 推断架构版本
func (d *Detector) inferArchFromPart(part string) ArchType {
	part = strings.ToLower(part)
	switch {
	case strings.HasPrefix(part, "0xd0") || strings.HasPrefix(part, "0xd4"):
		return ArchARM64 // Cortex-A7x 系列
	case strings.HasPrefix(part, "0xc0"):
		return ArchARM64 // Cortex-A5x 系列
	case strings.HasPrefix(part, "0xd") || strings.HasPrefix(part, "0xc"):
		return ArchARM64
	default:
		return ArchARMv7
	}
}

// detectArchVersion 检测架构版本字符串
func (d *Detector) detectArchVersion() string {
	cpuinfo, err := readCPUInfo()
	if err != nil {
		return "unknown"
	}

	if arch, ok := cpuinfo["CPU architecture"]; ok {
		ver, _ := strconv.Atoi(arch)
		switch {
		case ver >= 9:
			return "ARMv9-A"
		case ver == 8:
			return "ARMv8-A"
		case ver == 7:
			return "ARMv7-A"
		case ver == 6:
			return "ARMv6"
		default:
			return fmt.Sprintf("ARMv%d", ver)
		}
	}

	goarch := runtime.GOARCH
	if goarch == "arm64" {
		return "ARMv8-A"
	}
	return "ARMv7-A"
}

// detectBits 检测位宽
func (d *Detector) detectBits() int {
	if runtime.GOARCH == "arm64" {
		return 64
	}
	return 32
}

// ========== SoC 检测 ==========

// detectSoC 检测 SoC 厂商和型号
func (d *Detector) detectSoC() (SoCFamily, string) {
	// 1. 从 /proc/device-tree/compatible 读取
	if compat := readDeviceTreeCompatible(); compat != "" {
		if soc, model := parseSoCFromCompatible(compat); soc != SoCUnknown {
			return soc, model
		}
	}

	// 2. 从 /proc/cpuinfo 的 Hardware 字段推断
	cpuinfo, err := readCPUInfo()
	if err == nil {
		if hw, ok := cpuinfo["Hardware"]; ok {
			return d.inferSoCFromHardware(hw)
		}
		if model, ok := cpuinfo["model name"]; ok {
			return d.inferSoCFromModel(model)
		}
	}

	// 3. 从 /sys/class/dmi 推断 (部分 ARM 设备有 DMI 信息)
	if dmi := readDMISysVendor(); dmi != "" {
		return d.inferSoCFromDMIVendor(dmi)
	}

	return SoCUnknown, "unknown"
}

// inferSoCFromHardware 从 Hardware 字段推断 SoC
func (d *Detector) inferSoCFromHardware(hw string) (SoCFamily, string) {
	hw = strings.ToLower(hw)
	switch {
	case strings.Contains(hw, "rockchip"):
		model := extractRockchipModel(hw)
		return SoCRockchip, model
	case strings.Contains(hw, "allwinner") || strings.Contains(hw, "sun"):
		return SoCAllwinner, "Allwinner SoC"
	case strings.Contains(hw, "qualcomm") || strings.Contains(hw, "qcom") || strings.Contains(hw, "apq") || strings.Contains(hw, "msm"):
		return SoCQualcomm, "Qualcomm SoC"
	case strings.Contains(hw, "bcm") || strings.Contains(hw, "broadcom"):
		return SoCBroadcom, "Broadcom SoC"
	case strings.Contains(hw, "amlogic"):
		return SoCAmlogic, "Amlogic SoC"
	case strings.Contains(hw, "samsung") || strings.Contains(hw, "exynos"):
		return SoCSamsung, "Samsung Exynos"
	case strings.Contains(hw, "mediatek") || strings.Contains(hw, "mt"):
		return SoCMediaTek, "MediaTek SoC"
	case strings.Contains(hw, "hisilicon") || strings.Contains(hw, "hi") || strings.Contains(hw, "kirin"):
		return SoCHiSilicon, "HiSilicon SoC"
	default:
		return SoCUnknown, hw
	}
}

// inferSoCFromModel 从 model name 推断 SoC
func (d *Detector) inferSoCFromModel(model string) (SoCFamily, string) {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "rockchip") || strings.Contains(model, "rk"):
		return SoCRockchip, extractRockchipModel(model)
	case strings.Contains(model, "cortex-a") || strings.Contains(model, "arm"):
		return SoCUnknown, model
	default:
		return SoCUnknown, model
	}
}

// inferSoCFromDMIVendor 从 DMI 厂商推断 SoC
func (d *Detector) inferSoCFromDMIVendor(vendor string) (SoCFamily, string) {
	vendor = strings.ToLower(vendor)
	switch {
	case strings.Contains(vendor, "raspberry"):
		return SoCBroadcom, "Broadcom BCM2711/BCM2712"
	case strings.Contains(vendor, "pine64"):
		return SoCAllwinner, "Allwinner A64/H6"
	case strings.Contains(vendor, "hardkernel") || strings.Contains(vendor, "odroid"):
		return SoCAmlogic, "Amlogic S905/S922"
	default:
		return SoCUnknown, vendor
	}
}

// extractRockchipModel 从字符串中提取 Rockchip 型号
func extractRockchipModel(s string) string {
	re := regexp.MustCompile(`(?i)(rk\d{4}[a-z]*)`)
	if match := re.FindString(s); match != "" {
		return strings.ToUpper(match)
	}
	return "Rockchip SoC"
}

// ========== CPU 检测 ==========

// detectCPUCores 检测 CPU 核心数
func (d *Detector) detectCPUCores() int {
	// /sys/devices/system/cpu/online 比 runtime 更准确
	if data, err := os.ReadFile("/sys/devices/system/cpu/online"); err == nil {
		cores := parseCPUOnline(strings.TrimSpace(string(data)))
		if cores > 0 {
			return cores
		}
	}
	return runtime.NumCPU()
}

// parseCPUOnline 解析 CPU online 字符串 (e.g., "0-3", "0-3,4-7")
func parseCPUOnline(s string) int {
	total := 0
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) == 2 {
				start, err1 := strconv.Atoi(bounds[0])
				end, err2 := strconv.Atoi(bounds[1])
				if err1 == nil && err2 == nil {
					total += end - start + 1
				}
			}
		} else {
			if _, err := strconv.Atoi(part); err == nil {
				total++
			}
		}
	}
	return total
}

// detectMaxFreq 检测最大 CPU 频率
func (d *Detector) detectMaxFreq() int {
	// 尝试读取 CPU0 的最大频率
	paths := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq",
	}
	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			freq, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && freq > 0 {
				return freq / 1000 // kHz -> MHz
			}
		}
	}
	return 0
}

// detectFeatures 检测 CPU 特性
func (d *Detector) detectFeatures() []CPUFeature {
	var features []CPUFeature
	featureSet := make(map[CPUFeature]bool)

	// 从 /proc/cpuinfo 的 Features 行读取
	cpuinfo, err := readCPUInfo()
	if err == nil {
		if flags, ok := cpuinfo["Features"]; ok {
			features = append(features, parseFeatures(flags, featureSet)...)
		}
		if flags, ok := cpuinfo["flags"]; ok {
			features = append(features, parseFeatures(flags, featureSet)...)
		}
	}

	return features
}

// parseFeatures 解析 CPU 特性字符串
func parseFeatures(flags string, seen map[CPUFeature]bool) []CPUFeature {
	var features []CPUFeature
	featureMap := map[string]CPUFeature{
		"neon":     FeatureNEON,
		"asimd":    FeatureNEON, // ARM64 的 NEON 叫 ASIMD
		"vfpv4":    FeatureVFPv4,
		"vfp":      FeatureVFPv4,
		"aes":      FeatureAES,
		"sha1":     FeatureSHA1,
		"sha2":     FeatureSHA2,
		"crc32":    FeatureCRC32,
		"lse":      FeatureLSE,
		"atomics":  FeatureLSE,
		"sve":      FeatureSVE,
		"asimddp":  FeatureDotProd,
		"dotprod":  FeatureDotProd,
		"fp16":     FeatureFP16,
		"fphp":     FeatureFP16,
		"i8mm":     FeatureI8MM,
	}

	for _, token := range strings.Fields(flags) {
		if feat, ok := featureMap[token]; ok && !seen[feat] {
			features = append(features, feat)
			seen[feat] = true
		}
	}
	return features
}

// detectBigLittle 检测 big.LITTLE 配置
func (d *Detector) detectBigLittle(totalCores int) (big, little int) {
	// 尝试从 sysfs 推断不同集群的最大频率
	freqs := make(map[int]bool)
	for i := 0; i < totalCores; i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		freq, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		freqs[freq] = true
	}

	if len(freqs) >= 2 {
		// 有不同频率的 CPU 核心，说明是 big.LITTLE
		maxFreq := 0
		minFreq := int(^uint(0) >> 1)
		for f := range freqs {
			if f > maxFreq {
				maxFreq = f
			}
			if f < minFreq {
				minFreq = f
			}
		}

		// 计算每个集群的核心数
		for i := 0; i < totalCores; i++ {
			path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			freq, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				continue
			}
			// 频率更高的是大核
			if freq >= (maxFreq+minFreq)/2 {
				big++
			} else {
				little++
			}
		}

		if big > 0 && little > 0 {
			return
		}
	}

	// 无法确定，假设全部是同一类型
	if totalCores >= 4 {
		// 常见 ARM NAS 设备配置
		big = totalCores / 2
		little = totalCores - big
	}
	return
}

// ========== 内存检测 ==========

// detectMemory 检测内存大小
func (d *Detector) detectMemory() int {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.Atoi(fields[1])
				if err == nil {
					return kb / 1024 // kB -> MB
				}
			}
		}
	}
	return 0
}

// detectLPDDRType 检测 LPDDR 类型
func (d *Detector) detectLPDDRType() string {
	// 尝试从 DMI 或设备树推断
	if data, err := os.ReadFile("/proc/device-tree/memory/ddr_type"); err == nil {
		t := strings.TrimSpace(string(data))
		if t != "" {
			return t
		}
	}
	// 从频率推断
	if data, err := os.ReadFile("/sys/class/devfreq/ddrfreq/cur_freq"); err == nil {
		freq, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			freqMHz := freq / 1000000
			switch {
			case freqMHz >= 3200:
				return "LPDDR5"
			case freqMHz >= 2133:
				return "LPDDR4X"
			case freqMHz >= 1600:
				return "LPDDR4"
			default:
				return "LPDDR3"
			}
		}
	}
	return "unknown"
}

// ========== 存储检测 ==========

// detectUSB3 检测 USB 3.0 支持
func (d *Detector) detectUSB3() bool {
	// 检查 /sys/bus/usb/devices/ 下是否有 USB 3.0 设备
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		speedPath := fmt.Sprintf("/sys/bus/usb/devices/%s/speed", entry.Name())
		data, err := os.ReadFile(speedPath)
		if err != nil {
			continue
		}
		speed, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if speed >= 5000 { // USB 3.0 = 5000 Mbps
			return true
		}
	}
	return false
}

// detectPCIe 检测 PCIe 支持
func (d *Detector) detectPCIe() bool {
	entries, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// detectSATA 检测 SATA 支持
func (d *Detector) detectSATA() bool {
	entries, err := os.ReadDir("/sys/class/ata_port")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// ========== 网络检测 ==========

// detectEthernet 检测以太网能力
func (d *Detector) detectEthernet() (gbe, gbe25 bool) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return false, false
	}

	for _, entry := range entries {
		if entry.Name() == "lo" {
			continue
		}
		// 检查是否是物理网络接口
		driverPath := fmt.Sprintf("/sys/class/net/%s/device/driver", entry.Name())
		if _, err := os.Stat(driverPath); err != nil {
			continue
		}

		// 检查速度
		speedPath := fmt.Sprintf("/sys/class/net/%s/speed", entry.Name())
		data, err := os.ReadFile(speedPath)
		if err != nil {
			continue
		}
		speed, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if speed >= 2500 {
			gbe25 = true
			gbe = true
		} else if speed >= 1000 {
			gbe = true
		}
	}
	return
}

// detectWiFi 检测 WiFi 支持
func (d *Detector) detectWiFi() (bool, string) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return false, ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "wl") {
			continue
		}

		// 读取设备信息尝试判断 WiFi 版本
		ueventPath := fmt.Sprintf("/sys/class/net/%s/device/uevent", name)
		data, err := os.ReadFile(ueventPath)
		if err != nil {
			return true, "unknown"
		}
		content := string(data)
		switch {
		case strings.Contains(content, "wifi6") || strings.Contains(content, "ax"):
			return true, "WiFi 6"
		case strings.Contains(content, "wifi5") || strings.Contains(content, "ac"):
			return true, "WiFi 5"
		case strings.Contains(content, "wifi4") || strings.Contains(content, "n"):
			return true, "WiFi 4"
		default:
			return true, "WiFi"
		}
	}
	return false, ""
}

// ========== 辅助函数 ==========

// readCPUInfo 读取 /proc/cpuinfo 并解析为 map
func readCPUInfo() (map[string]string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if _, exists := result[key]; !exists {
				result[key] = value
			}
		}
	}
	return result, scanner.Err()
}

// readDeviceTreeCompatible 读取设备树 compatible 属性
func readDeviceTreeCompatible() string {
	data, err := os.ReadFile("/proc/device-tree/compatible")
	if err != nil {
		return ""
	}
	// 设备树 compatible 用 null 分隔
	return strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", ",")
}

// parseSoCFromCompatible 从 compatible 字符串解析 SoC 信息
func parseSoCFromCompatible(compat string) (SoCFamily, string) {
	compat = strings.ToLower(compat)
	switch {
	case strings.Contains(compat, "rockchip"):
		return SoCRockchip, extractRockchipModel(compat)
	case strings.Contains(compat, "allwinner") || strings.Contains(compat, "sun"):
		return SoCAllwinner, "Allwinner SoC"
	case strings.Contains(compat, "qcom") || strings.Contains(compat, "qualcomm"):
		return SoCQualcomm, "Qualcomm SoC"
	case strings.Contains(compat, "brcm") || strings.Contains(compat, "broadcom"):
		return SoCBroadcom, "Broadcom SoC"
	case strings.Contains(compat, "amlogic"):
		return SoCAmlogic, "Amlogic SoC"
	case strings.Contains(compat, "samsung") || strings.Contains(compat, "exynos"):
		return SoCSamsung, "Samsung Exynos"
	case strings.Contains(compat, "mediatek") || strings.Contains(compat, "mt"):
		return SoCMediaTek, "MediaTek SoC"
	case strings.Contains(compat, "hisilicon") || strings.Contains(compat, "hi"):
		return SoCHiSilicon, "HiSilicon SoC"
	default:
		return SoCUnknown, ""
	}
}

// readDMISysVendor 读取 DMI 系统厂商
func readDMISysVendor() string {
	data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// 用于测试的时间函数
var nowFunc = func() time.Time { return time.Now() }
