// Package armadapter 提供 ARM 架构适配功能
// 硬件检测、架构识别、兼容性验证、优化配置建议
package armadapter

import (
	"fmt"
	"time"
)

// ========== 架构定义 ==========

// ArchType ARM 架构类型
type ArchType string

const (
	ArchARM64  ArchType = "arm64"  // AArch64 / ARMv8-A (64-bit)
	ArchARMv7  ArchType = "armv7"  // ARMv7-A (32-bit, Cortex-A 系列)
	ArchARMv6  ArchType = "armv6"  // ARMv6 (旧设备)
	ArchARMv5  ArchType = "armv5"  // ARMv5 (极旧设备)
	ArchUnknown ArchType = "unknown"
)

// SoCFamily SoC 厂商家族
type SoCFamily string

const (
	SoCRockchip  SoCFamily = "rockchip"  // 瑞芯微
	SoCAllwinner SoCFamily = "allwinner" // 全志
	SoCQualcomm  SoCFamily = "qualcomm"  // 高通
	SoCBroadcom  SoCFamily = "broadcom"  // 博通 (树莓派)
	SoCAmlogic   SoCFamily = "amlogic"   // 晶晨
	SoCSamsung   SoCFamily = "samsung"   // 三星 Exynos
	SoCMediaTek  SoCFamily = "mediatek"  // 联发科
	SoCHiSilicon SoCFamily = "hisilicon" // 海思
	SoCUnknown   SoCFamily = "unknown"
)

// ========== 硬件信息 ==========

// CPUFeature CPU 特性标志
type CPUFeature string

const (
	FeatureNEON     CPUFeature = "neon"      // SIMD 指令集
	FeatureVFPv4    CPUFeature = "vfpv4"     // 浮点运算
	FeatureAES      CPUFeature = "aes"       // AES 硬件加速
	FeatureSHA1     CPUFeature = "sha1"      // SHA1 硬件加速
	FeatureSHA2     CPUFeature = "sha2"      // SHA2 硬件加速
	FeatureCRC32    CPUFeature = "crc32"     // CRC32 硬件指令
	FeatureLSE      CPUFeature = "lse"       // 大型系统扩展 (原子操作)
	FeatureSVE      CPUFeature = "sve"       // 可伸缩向量扩展
	FeatureDotProd  CPUFeature = "dotprod"   // 点积指令
	FeatureFP16     CPUFeature = "fp16"      // 半精度浮点
	FeatureI8MM     CPUFeature = "i8mm"      // Int8 矩阵乘法
)

// ARMHardwareInfo ARM 硬件信息
type ARMHardwareInfo struct {
	// 基本信息
	ArchType    ArchType  `json:"archType"`    // 架构类型
	ArchVersion string    `json:"archVersion"` // 架构版本 (e.g., "ARMv8.2-A")
	Bits        int       `json:"bits"`        // 位宽 (32/64)
	Endianness  string    `json:"endianness"`  // 字节序 (little/big)

	// CPU 信息
	SoC         SoCFamily `json:"soc"`         // SoC 厂商
	SoCModel    string    `json:"socModel"`    // SoC 型号 (e.g., "RK3588")
	CPUCores    int       `json:"cpuCores"`    // CPU 核心数
	BigCores    int       `json:"bigCores"`    // 大核数 (big.LITTLE)
	LittleCores int       `json:"littleCores"` // 小核数
	MaxFreqMHz  int       `json:"maxFreqMhz"` // 最大频率 (MHz)
	Features    []CPUFeature `json:"features"` // CPU 特性

	// 内存信息
	MemoryMB    int    `json:"memoryMb"`    // 内存大小 (MB)
	LPDDRType   string `json:"lpddrType"`   // LPDDR 类型 (LPDDR4/LPDDR4X/LPDDR5)

	// 存储信息
	StorageType string `json:"storageType"` // 存储类型 (eMMC/NVMe/SATA/SD)
	HasUSB3     bool   `json:"hasUsb3"`     // USB 3.0 支持
	HasPCIe     bool   `json:"hasPcie"`     // PCIe 支持
	HasSATA     bool   `json:"hasSata"`     // SATA 支持

	// 网络信息
	HasGbE      bool   `json:"hasGbE"`      // 千兆以太网
	Has2_5GbE   bool   `json:"has2_5GbE"`   // 2.5G 以太网
	HasWiFi     bool   `json:"hasWiFi"`     // WiFi 支持
	WiFiVersion string `json:"wifiVersion"` // WiFi 版本

	// 检测时间
	DetectedAt  time.Time `json:"detectedAt"`
}

// ========== 兼容性 ==========

// CompatLevel 兼容性等级
type CompatLevel int

const (
	CompatFull      CompatLevel = iota // 完全兼容
	CompatPartial                      // 部分兼容
	CompatLimited                      // 有限兼容
	CompatUnsupported                  // 不支持
)

func (c CompatLevel) String() string {
	switch c {
	case CompatFull:
		return "full"
	case CompatPartial:
		return "partial"
	case CompatLimited:
		return "limited"
	case CompatUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// CompatIssue 兼容性问题
type CompatIssue struct {
	Component   string      `json:"component"`   // 组件名
	Level       CompatLevel `json:"level"`        // 兼容等级
	Description string      `json:"description"`  // 问题描述
	Workaround  string      `json:"workaround"`   // 解决方案
}

// CompatReport 兼容性报告
type CompatReport struct {
	Overall     CompatLevel    `json:"overall"`     // 整体兼容等级
	Score       int            `json:"score"`       // 兼容性评分 (0-100)
	DeviceName  string         `json:"deviceName"`  // 设备名称
	ArchType    ArchType       `json:"archType"`    // 架构类型
	SoC         SoCFamily      `json:"soc"`         // SoC 厂商
	Issues      []CompatIssue  `json:"issues"`      // 兼容性问题列表
	CheckedAt   time.Time      `json:"checkedAt"`   // 检测时间
}

// ========== 优化建议 ==========

// OptCategory 优化类别
type OptCategory string

const (
	OptCPU     OptCategory = "cpu"     // CPU 调度优化
	OptMemory  OptCategory = "memory"  // 内存优化
	OptStorage OptCategory = "storage" // 存储优化
	OptNetwork OptCategory = "network" // 网络优化
	OptPower   OptCategory = "power"   // 功耗优化
	OptKernel  OptCategory = "kernel"  // 内核参数优化
)

// OptPriority 优化建议优先级
type OptPriority int

const (
	OptPriorityHigh   OptPriority = 3 // 高优先级
	OptPriorityMedium OptPriority = 2 // 中优先级
	OptPriorityLow    OptPriority = 1 // 低优先级
)

// Optimization 优化建议
type Optimization struct {
	Category    OptCategory `json:"category"`    // 类别
	Priority    OptPriority `json:"priority"`    // 优先级
	Title       string      `json:"title"`       // 标题
	Description string      `json:"description"` // 描述
	Parameter   string      `json:"parameter"`   // 配置参数名
	Value       string      `json:"value"`       // 建议值
	Reason      string      `json:"reason"`      // 原因
}

// OptProfile 优化配置档案
type OptProfile struct {
	DeviceName    string         `json:"deviceName"`    // 设备名称
	ArchType      ArchType       `json:"archType"`      // 架构类型
	Optimizations []Optimization `json:"optimizations"` // 优化建议列表
	GeneratedAt   time.Time      `json:"generatedAt"`   // 生成时间
}

// ========== 支持的设备 ==========

// SupportedDevice 支持的 ARM 设备
type SupportedDevice struct {
	Name        string    `json:"name"`        // 设备名称
	SoC         SoCFamily `json:"soc"`         // SoC 厂商
	SoCModel    string    `json:"socModel"`    // SoC 型号
	ArchType    ArchType  `json:"archType"`    // 架构类型
	MinMemoryMB int       `json:"minMemoryMb"` // 最低内存要求
	CompatLevel CompatLevel `json:"compatLevel"` // 兼容等级
	Notes       string    `json:"notes"`       // 备注
}

// ErrUnsupportedArch 不支持的架构错误
type ErrUnsupportedArch struct {
	Arch ArchType
}

func (e *ErrUnsupportedArch) Error() string {
	return fmt.Sprintf("不支持的 ARM 架构: %s", e.Arch)
}
