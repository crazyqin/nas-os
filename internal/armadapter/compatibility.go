package armadapter

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 兼容性验证器 ==========

// CompatibilityChecker 兼容性检查器
type CompatibilityChecker struct {
	mu              sync.RWMutex
	supportedDevices []SupportedDevice
	minRequirements MinRequirements
}

// MinRequirements 最低运行要求
type MinRequirements struct {
	MinArchBits  int // 最低位宽 (32/64)
	MinMemoryMB  int // 最低内存 (MB)
	MinCPUCores  int // 最低 CPU 核心数
	RequiredArch []ArchType // 支持的架构列表
}

// NewCompatibilityChecker 创建兼容性检查器
func NewCompatibilityChecker() *CompatibilityChecker {
	checker := &CompatibilityChecker{
		minRequirements: MinRequirements{
			MinArchBits: 32,
			MinMemoryMB: 512,
			MinCPUCores: 2,
			RequiredArch: []ArchType{ArchARM64, ArchARMv7},
		},
	}
	checker.initSupportedDevices()
	return checker
}

// initSupportedDevices 初始化支持的设备列表
// 参考飞牛 fnOS 2026年1月 ARM 公测版首批适配设备
func (c *CompatibilityChecker) initSupportedDevices() {
	c.supportedDevices = []SupportedDevice{
		// ===== Rockchip 系列 =====
		{Name: "Rockchip RK3588", SoC: SoCRockchip, SoCModel: "RK3588", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "4x A76 + 4x A55, 8nm, 支持 NVMe/PCIe 3.0"},
		{Name: "Rockchip RK3588S", SoC: SoCRockchip, SoCModel: "RK3588S", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "RK3588 精简版, 适合 NAS"},
		{Name: "Rockchip RK3568", SoC: SoCRockchip, SoCModel: "RK3568", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatFull, Notes: "4x A55, 支持 SATA/PCIe 2.1"},
		{Name: "Rockchip RK3566", SoC: SoCRockchip, SoCModel: "RK3566", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatFull, Notes: "4x A55, 精简版"},
		{Name: "Rockchip RK3528", SoC: SoCRockchip, SoCModel: "RK3528", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatFull, Notes: "4x A53, 入门级 NAS"},
		{Name: "Rockchip RK3399", SoC: SoCRockchip, SoCModel: "RK3399", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "2x A72 + 4x A53, 经典 NAS 芯片"},
		{Name: "Rockchip RK3328", SoC: SoCRockchip, SoCModel: "RK3328", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatFull, Notes: "4x A53, 低功耗 NAS"},
		{Name: "Rockchip RK3326", SoC: SoCRockchip, SoCModel: "RK3326", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "4x A35, 低功耗"},
		{Name: "Rockchip RK3308", SoC: SoCRockchip, SoCModel: "RK3308", ArchType: ArchARM64, MinMemoryMB: 256, CompatLevel: CompatLimited, Notes: "4x A35, 无显示输出"},
		{Name: "Rockchip RK3288", SoC: SoCRockchip, SoCModel: "RK3288", ArchType: ArchARMv7, MinMemoryMB: 2048, CompatLevel: CompatPartial, Notes: "4x A17, 32 位"},
		{Name: "Rockchip RK3188", SoC: SoCRockchip, SoCModel: "RK3188", ArchType: ArchARMv7, MinMemoryMB: 1024, CompatLevel: CompatLimited, Notes: "4x A9, 老旧设备"},
		{Name: "Rockchip RK3066", SoC: SoCRockchip, SoCModel: "RK3066", ArchType: ArchARMv7, MinMemoryMB: 512, CompatLevel: CompatLimited, Notes: "2x A9, 古早设备"},

		// ===== Allwinner 系列 =====
		{Name: "Allwinner A523", SoC: SoCAllwinner, SoCModel: "A523", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatFull, Notes: "8x A55, 新一代"},
		{Name: "Allwinner H618", SoC: SoCAllwinner, SoCModel: "H618", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatFull, Notes: "4x A53, 常见 SBC"},
		{Name: "Allwinner H616", SoC: SoCAllwinner, SoCModel: "H616", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatFull, Notes: "4x A53"},
		{Name: "Allwinner H6", SoC: SoCAllwinner, SoCModel: "H6", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatPartial, Notes: "4x A53, 无 SATA"},
		{Name: "Allwinner H5", SoC: SoCAllwinner, SoCModel: "H5", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "4x A53, Pine64 系列"},
		{Name: "Allwinner H3", SoC: SoCAllwinner, SoCModel: "H3", ArchType: ArchARMv7, MinMemoryMB: 256, CompatLevel: CompatLimited, Notes: "4x A7, NanoPi/OpiZero"},
		{Name: "Allwinner A64", SoC: SoCAllwinner, SoCModel: "A64", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "4x A53, Pine A64"},

		// ===== Amlogic 系列 =====
		{Name: "Amlogic S928X", SoC: SoCAmlogic, SoCModel: "S928X", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "1x A76 + 4x A55"},
		{Name: "Amlogic S905X4", SoC: SoCAmlogic, SoCModel: "S905X4", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatFull, Notes: "4x A55"},
		{Name: "Amlogic S905X3", SoC: SoCAmlogic, SoCModel: "S905X3", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatFull, Notes: "4x A55, 常见 TV Box"},
		{Name: "Amlogic S922X", SoC: SoCAmlogic, SoCModel: "S922X", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "4x A73 + 2x A53, ODROID-N2"},
		{Name: "Amlogic S905X2", SoC: SoCAmlogic, SoCModel: "S905X2", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatPartial, Notes: "4x A53"},
		{Name: "Amlogic S905X", SoC: SoCAmlogic, SoCModel: "S905X", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "4x A53, 经典"},
		{Name: "Amlogic S905", SoC: SoCAmlogic, SoCModel: "S905", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "4x A53"},
		{Name: "Amlogic S805X", SoC: SoCAmlogic, SoCModel: "S805X", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatLimited, Notes: "4x A53, 入门级"},

		// ===== Broadcom (树莓派系列) =====
		{Name: "Broadcom BCM2712", SoC: SoCBroadcom, SoCModel: "BCM2712", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatFull, Notes: "Raspberry Pi 5, 4x A76"},
		{Name: "Broadcom BCM2711", SoC: SoCBroadcom, SoCModel: "BCM2711", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatPartial, Notes: "Raspberry Pi 4, 4x A72"},
		{Name: "Broadcom BCM2837", SoC: SoCBroadcom, SoCModel: "BCM2837", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatLimited, Notes: "Raspberry Pi 3, 4x A53, USB 2.0"},
		{Name: "Broadcom BCM2836", SoC: SoCBroadcom, SoCModel: "BCM2836", ArchType: ArchARMv7, MinMemoryMB: 512, CompatLevel: CompatLimited, Notes: "Raspberry Pi 2, 4x A7"},

		// ===== Qualcomm 系列 =====
		{Name: "Qualcomm IPQ807x", SoC: SoCQualcomm, SoCModel: "IPQ807x", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatPartial, Notes: "路由器 SoC, 可做 NAS"},
		{Name: "Qualcomm SDM845", SoC: SoCQualcomm, SoCModel: "SDM845", ArchType: ArchARM64, MinMemoryMB: 4096, CompatLevel: CompatPartial, Notes: "骁龙845, 手机改 NAS"},
		{Name: "Qualcomm SDM660", SoC: SoCQualcomm, SoCModel: "SDM660", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatLimited, Notes: "骁龙660"},

		// ===== Samsung Exynos 系列 =====
		{Name: "Samsung Exynos 5422", SoC: SoCSamsung, SoCModel: "Exynos 5422", ArchType: ArchARM64, MinMemoryMB: 2048, CompatLevel: CompatPartial, Notes: "ODROID-XU4, 4x A15 + 4x A7"},
		{Name: "Samsung Exynos 8890", SoC: SoCSamsung, SoCModel: "Exynos 8890", ArchType: ArchARM64, MinMemoryMB: 4096, CompatLevel: CompatPartial, Notes: "Galaxy S7 改 NAS"},

		// ===== MediaTek 系列 =====
		{Name: "MediaTek MT7981", SoC: SoCMediaTek, SoCModel: "MT7981", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatPartial, Notes: "Filogic 830, 路由器 SoC"},
		{Name: "MediaTek MT7622", SoC: SoCMediaTek, SoCModel: "MT7622", ArchType: ArchARM64, MinMemoryMB: 512, CompatLevel: CompatLimited, Notes: "2x A53, 路由器芯片"},

		// ===== HiSilicon 系列 =====
		{Name: "HiSilicon Hi3536DV100", SoC: SoCHiSilicon, SoCModel: "Hi3536DV100", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatPartial, Notes: "安防 NVR 芯片"},
		{Name: "HiSilicon Hi3798MV310", SoC: SoCHiSilicon, SoCModel: "Hi3798MV310", ArchType: ArchARM64, MinMemoryMB: 1024, CompatLevel: CompatLimited, Notes: "机顶盒芯片"},

		// ===== 海思 Kirin 系列 (手机改 NAS) =====
		{Name: "HiSilicon Kirin 990", SoC: SoCHiSilicon, SoCModel: "Kirin 990", ArchType: ArchARM64, MinMemoryMB: 4096, CompatLevel: CompatPartial, Notes: "2x A76 + 2x A76 + 4x A55, 手机改 NAS"},
		{Name: "HiSilicon Kirin 980", SoC: SoCHiSilicon, SoCModel: "Kirin 980", ArchType: ArchARM64, MinMemoryMB: 4096, CompatLevel: CompatPartial, Notes: "2x A76 + 2x A76 + 4x A55, 手机改 NAS"},
	}
}

// Check 兼容性检查
func (c *CompatibilityChecker) Check(info *ARMHardwareInfo) (*CompatReport, error) {
	if info == nil {
		return nil, fmt.Errorf("hardware info is nil")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	report := &CompatReport{
		DeviceName: info.SoCModel,
		ArchType:   info.ArchType,
		SoC:        info.SoC,
		CheckedAt:  time.Now(),
	}

	var issues []CompatIssue
	score := 100

	// 1. 架构兼容性检查
	archIssue, archPenalty := c.checkArchCompat(info)
	if archIssue != nil {
		issues = append(issues, *archIssue)
		score -= archPenalty
	}

	// 2. 内存兼容性检查
	memIssue, memPenalty := c.checkMemoryCompat(info)
	if memIssue != nil {
		issues = append(issues, *memIssue)
		score -= memPenalty
	}

	// 3. CPU 核心数检查
	cpuIssue, cpuPenalty := c.checkCPUCompat(info)
	if cpuIssue != nil {
		issues = append(issues, *cpuIssue)
		score -= cpuPenalty
	}

	// 4. 存储能力检查
	storageIssue, storagePenalty := c.checkStorageCompat(info)
	if storageIssue != nil {
		issues = append(issues, *storageIssue)
		score -= storagePenalty
	}

	// 5. 网络能力检查
	netIssue, netPenalty := c.checkNetworkCompat(info)
	if netIssue != nil {
		issues = append(issues, *netIssue)
		score -= netPenalty
	}

	// 6. CPU 特性检查
	featureIssues, featurePenalty := c.checkFeatureCompat(info)
	issues = append(issues, featureIssues...)
	score -= featurePenalty

	// 7. 检查是否在已知支持设备列表中
	if devCompat := c.checkKnownDevice(info); devCompat != nil {
		if devCompat.CompatLevel > CompatPartial {
			issues = append(issues, CompatIssue{
				Component:   "设备",
				Level:       devCompat.CompatLevel,
				Description: fmt.Sprintf("已知设备 %s 兼容性等级: %s", devCompat.Name, devCompat.CompatLevel),
				Workaround:  devCompat.Notes,
			})
		}
	}

	// 确保评分在 0-100 范围内
	if score < 0 {
		score = 0
	}
	report.Score = score
	report.Issues = issues

	// 确定整体兼容等级
	report.Overall = c.determineOverallLevel(score, issues)

	log.Printf("[ARM适配] 兼容性检查: device=%s score=%d overall=%s issues=%d",
		report.DeviceName, report.Score, report.Overall, len(report.Issues))

	return report, nil
}

// checkArchCompat 架构兼容性检查
func (c *CompatibilityChecker) checkArchCompat(info *ARMHardwareInfo) (*CompatIssue, int) {
	supported := false
	for _, arch := range c.minRequirements.RequiredArch {
		if info.ArchType == arch {
			supported = true
			break
		}
	}

	if !supported {
		return &CompatIssue{
			Component:   "CPU 架构",
			Level:       CompatUnsupported,
			Description: fmt.Sprintf("不支持的架构: %s", info.ArchType),
			Workaround:  "需要 ARMv7 或更高版本的处理器",
		}, 100
	}

	if info.Bits == 32 {
		return &CompatIssue{
			Component:   "CPU 位宽",
			Level:       CompatPartial,
			Description: "32 位 ARM 架构，部分功能受限",
			Workaround:  "建议使用 64 位 ARM 设备以获得完整功能",
		}, 20
	}

	return nil, 0
}

// checkMemoryCompat 内存兼容性检查
func (c *CompatibilityChecker) checkMemoryCompat(info *ARMHardwareInfo) (*CompatIssue, int) {
	if info.MemoryMB < c.minRequirements.MinMemoryMB {
		return &CompatIssue{
			Component:   "内存",
			Level:       CompatUnsupported,
			Description: fmt.Sprintf("内存不足: %dMB (最低要求: %dMB)", info.MemoryMB, c.minRequirements.MinMemoryMB),
			Workaround:  "需要增加内存或使用更高端设备",
		}, 50
	}

	if info.MemoryMB < 1024 {
		return &CompatIssue{
			Component:   "内存",
			Level:       CompatLimited,
			Description: fmt.Sprintf("内存较低: %dMB，可能影响性能", info.MemoryMB),
			Workaround:  "建议至少 1GB 内存以获得流畅体验",
		}, 15
	}

	if info.MemoryMB < 2048 {
		return &CompatIssue{
			Component:   "内存",
			Level:       CompatPartial,
			Description: fmt.Sprintf("内存中等: %dMB，建议 2GB 以上", info.MemoryMB),
			Workaround:  "部分高负载功能可能受限",
		}, 5
	}

	return nil, 0
}

// checkCPUCompat CPU 兼容性检查
func (c *CompatibilityChecker) checkCPUCompat(info *ARMHardwareInfo) (*CompatIssue, int) {
	if info.CPUCores < c.minRequirements.MinCPUCores {
		return &CompatIssue{
			Component:   "CPU 核心",
			Level:       CompatUnsupported,
			Description: fmt.Sprintf("CPU 核心数不足: %d (最低要求: %d)", info.CPUCores, c.minRequirements.MinCPUCores),
			Workaround:  "需要多核处理器",
		}, 40
	}

	if info.CPUCores < 4 {
		return &CompatIssue{
			Component:   "CPU 核心",
			Level:       CompatPartial,
			Description: fmt.Sprintf("CPU 核心数较少: %d，多任务性能可能受限", info.CPUCores),
			Workaround:  "建议使用四核或更多核心的处理器",
		}, 10
	}

	return nil, 0
}

// checkStorageCompat 存储兼容性检查
func (c *CompatibilityChecker) checkStorageCompat(info *ARMHardwareInfo) (*CompatIssue, int) {
	if !info.HasUSB3 && !info.HasPCIe && !info.HasSATA {
		return &CompatIssue{
			Component:   "存储接口",
			Level:       CompatLimited,
			Description: "未检测到高速存储接口 (USB3/PCIe/SATA)",
			Workaround:  "存储性能将受到限制，建议使用支持 USB3 或 SATA 的设备",
		}, 25
	}

	if !info.HasPCIe && !info.HasSATA {
		return &CompatIssue{
			Component:   "存储接口",
			Level:       CompatPartial,
			Description: "仅支持 USB 存储，无原生 SATA/PCIe",
			Workaround:  "RAID 功能受限，建议通过 USB3 外接存储",
		}, 10
	}

	return nil, 0
}

// checkNetworkCompat 网络兼容性检查
func (c *CompatibilityChecker) checkNetworkCompat(info *ARMHardwareInfo) (*CompatIssue, int) {
	if !info.HasGbE {
		return &CompatIssue{
			Component:   "网络",
			Level:       CompatLimited,
			Description: "未检测到千兆以太网",
			Workaround:  "网络传输速度受限，建议使用支持千兆以太网的设备",
		}, 15
	}

	return nil, 0
}

// checkFeatureCompat CPU 特性兼容性检查
func (c *CompatibilityChecker) checkFeatureCompat(info *ARMHardwareInfo) ([]CompatIssue, int) {
	var issues []CompatIssue
	penalty := 0

	featureSet := make(map[CPUFeature]bool)
	for _, f := range info.Features {
		featureSet[f] = true
	}

	// NEON 是关键特性
	if !featureSet[FeatureNEON] {
		issues = append(issues, CompatIssue{
			Component:   "CPU 特性",
			Level:       CompatPartial,
			Description: "缺少 NEON SIMD 指令集，多媒体处理性能将受影响",
			Workaround:  "软件编解码将替代硬件加速",
		})
		penalty += 10
	}

	// AES 加速
	if !featureSet[FeatureAES] {
		issues = append(issues, CompatIssue{
			Component:   "CPU 特性",
			Level:       CompatPartial,
			Description: "缺少 AES 硬件加速，加密性能将受影响",
			Workaround:  "VPN 和加密存储性能降低",
		})
		penalty += 5
	}

	// CRC32 硬件指令
	if !featureSet[FeatureCRC32] {
		issues = append(issues, CompatIssue{
			Component:   "CPU 特性",
			Level:       CompatLimited,
			Description: "缺少 CRC32 硬件指令，ZFS/Btrfs 校验性能降低",
			Workaround:  "建议使用支持 CRC32 指令的 ARMv8 处理器",
		})
		penalty += 5
	}

	return issues, penalty
}

// checkKnownDevice 检查已知支持设备列表
func (c *CompatibilityChecker) checkKnownDevice(info *ARMHardwareInfo) *SupportedDevice {
	for _, dev := range c.supportedDevices {
		if dev.SoC == info.SoC && (dev.SoCModel == info.SoCModel || info.SoCModel == "") {
			return &dev
		}
	}
	return nil
}

// determineOverallLevel 确定整体兼容等级
func (c *CompatibilityChecker) determineOverallLevel(score int, issues []CompatIssue) CompatLevel {
	// 检查是否有不支持的问题
	for _, issue := range issues {
		if issue.Level == CompatUnsupported {
			return CompatUnsupported
		}
	}

	switch {
	case score >= 80:
		return CompatFull
	case score >= 60:
		return CompatPartial
	case score >= 40:
		return CompatLimited
	default:
		return CompatUnsupported
	}
}

// GetSupportedDevices 获取支持的设备列表
func (c *CompatibilityChecker) GetSupportedDevices() []SupportedDevice {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]SupportedDevice, len(c.supportedDevices))
	copy(result, c.supportedDevices)
	return result
}

// SetMinRequirements 设置最低要求
func (c *CompatibilityChecker) SetMinRequirements(req MinRequirements) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.minRequirements = req
}
