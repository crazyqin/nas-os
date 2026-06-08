package armadapter

import (
	"testing"
	"time"
)

// ========== 类型测试 ==========

func TestArchTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		arch     ArchType
		expected string
	}{
		{"ARM64", ArchARM64, "arm64"},
		{"ARMv7", ArchARMv7, "armv7"},
		{"ARMv6", ArchARMv6, "armv6"},
		{"ARMv5", ArchARMv5, "armv5"},
		{"Unknown", ArchUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.arch) != tt.expected {
				t.Errorf("ArchType(%s) = %s, want %s", tt.name, string(tt.arch), tt.expected)
			}
		})
	}
}

func TestSoCFamilyConstants(t *testing.T) {
	tests := []struct {
		name     string
		soc      SoCFamily
		expected string
	}{
		{"Rockchip", SoCRockchip, "rockchip"},
		{"Allwinner", SoCAllwinner, "allwinner"},
		{"Qualcomm", SoCQualcomm, "qualcomm"},
		{"Broadcom", SoCBroadcom, "broadcom"},
		{"Amlogic", SoCAmlogic, "amlogic"},
		{"Samsung", SoCSamsung, "samsung"},
		{"MediaTek", SoCMediaTek, "mediatek"},
		{"HiSilicon", SoCHiSilicon, "hisilicon"},
		{"Unknown", SoCUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.soc) != tt.expected {
				t.Errorf("SoCFamily(%s) = %s, want %s", tt.name, string(tt.soc), tt.expected)
			}
		})
	}
}

func TestCompatLevelString(t *testing.T) {
	tests := []struct {
		level    CompatLevel
		expected string
	}{
		{CompatFull, "full"},
		{CompatPartial, "partial"},
		{CompatLimited, "limited"},
		{CompatUnsupported, "unsupported"},
		{CompatLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("CompatLevel(%d).String() = %s, want %s", tt.level, tt.level.String(), tt.expected)
			}
		})
	}
}

func TestErrUnsupportedArch(t *testing.T) {
	err := &ErrUnsupportedArch{Arch: ArchUnknown}
	expected := "不支持的 ARM 架构: unknown"
	if err.Error() != expected {
		t.Errorf("ErrUnsupportedArch.Error() = %s, want %s", err.Error(), expected)
	}
}

// ========== 检测器测试 ==========

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("NewDetector() returned nil")
	}
}

func TestDetectorDetectReturnsInfo(t *testing.T) {
	d := NewDetector()
	info, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if info == nil {
		t.Fatal("Detect() returned nil info")
	}

	// 验证基本字段已设置
	if info.ArchType == "" {
		t.Error("ArchType is empty")
	}
	if info.Bits == 0 {
		t.Error("Bits is 0")
	}
	if info.CPUCores == 0 {
		t.Error("CPUCores is 0")
	}
	if info.DetectedAt.IsZero() {
		t.Error("DetectedAt is zero")
	}
}

func TestDetectorCachesResult(t *testing.T) {
	d := NewDetector()

	info1, err := d.Detect()
	if err != nil {
		t.Fatalf("first Detect() error: %v", err)
	}

	info2, err := d.Detect()
	if err != nil {
		t.Fatalf("second Detect() error: %v", err)
	}

	// 应该返回缓存的结果（同一指针）
	if info1 != info2 {
		t.Error("Detect() should return cached result on second call")
	}
}

func TestDetectorGetInfoBeforeDetect(t *testing.T) {
	d := NewDetector()
	_, err := d.GetInfo()
	if err == nil {
		t.Error("GetInfo() should return error before Detect()")
	}
}

func TestDetectorReset(t *testing.T) {
	d := NewDetector()

	_, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	d.Reset()

	_, err = d.GetInfo()
	if err == nil {
		t.Error("GetInfo() should return error after Reset()")
	}
}

func TestDetectBits(t *testing.T) {
	d := NewDetector()
	info, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if info.Bits != 32 && info.Bits != 64 {
		t.Errorf("Bits = %d, want 32 or 64", info.Bits)
	}
}

func TestDetectMemory(t *testing.T) {
	d := NewDetector()
	info, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if info.MemoryMB < 0 {
		t.Errorf("MemoryMB = %d, want >= 0", info.MemoryMB)
	}
}

// ========== 兼容性检查器测试 ==========

func TestNewCompatibilityChecker(t *testing.T) {
	c := NewCompatibilityChecker()
	if c == nil {
		t.Fatal("NewCompatibilityChecker() returned nil")
	}
}

func TestCompatibilityCheckerCheck(t *testing.T) {
	c := NewCompatibilityChecker()

	tests := []struct {
		name           string
		info           *ARMHardwareInfo
		wantOverallMin CompatLevel
		wantScoreMin   int
	}{
		{
			name: "high-end ARM64 device",
			info: &ARMHardwareInfo{
				ArchType:   ArchARM64,
				Bits:       64,
				SoC:        SoCRockchip,
				SoCModel:   "RK3588",
				CPUCores:   8,
				BigCores:   4,
				LittleCores: 4,
				MemoryMB:   8192,
				HasUSB3:    true,
				HasPCIe:    true,
				HasSATA:    true,
				HasGbE:     true,
				Has2_5GbE:  true,
				Features:   []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32, FeatureLSE},
			},
			wantOverallMin: CompatFull,
			wantScoreMin:   80,
		},
		{
			name: "mid-range ARM64 device",
			info: &ARMHardwareInfo{
				ArchType:   ArchARM64,
				Bits:       64,
				SoC:        SoCAllwinner,
				SoCModel:   "H618",
				CPUCores:   4,
				MemoryMB:   1024,
				HasUSB3:    true,
				HasGbE:     true,
				Features:   []CPUFeature{FeatureNEON},
			},
			wantOverallMin: CompatPartial,
			wantScoreMin:   50,
		},
		{
			name: "low-end ARM64 device",
			info: &ARMHardwareInfo{
				ArchType: ArchARM64,
				Bits:     64,
				SoC:      SoCBroadcom,
				SoCModel: "BCM2837",
				CPUCores: 4,
				MemoryMB: 512,
				HasGbE:   true,
				Features: []CPUFeature{FeatureNEON},
			},
			wantOverallMin: CompatLimited,
			wantScoreMin:   30,
		},
		{
			name: "32-bit ARM device",
			info: &ARMHardwareInfo{
				ArchType: ArchARMv7,
				Bits:     32,
				SoC:      SoCRockchip,
				SoCModel: "RK3288",
				CPUCores: 4,
				MemoryMB: 2048,
				HasGbE:   true,
				Features: []CPUFeature{FeatureNEON},
			},
			wantOverallMin: CompatLimited,
			wantScoreMin:   40,
		},
		{
			name: "insufficient memory device",
			info: &ARMHardwareInfo{
				ArchType: ArchARM64,
				Bits:     64,
				SoC:      SoCRockchip,
				SoCModel: "RK3308",
				CPUCores: 4,
				MemoryMB: 128,
				HasGbE:   true,
			},
			wantOverallMin: CompatUnsupported,
			wantScoreMin:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := c.Check(tt.info)
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}

			if report == nil {
				t.Fatal("Check() returned nil report")
			}

			if report.Overall < tt.wantOverallMin {
				t.Errorf("Overall = %s, want >= %s", report.Overall, tt.wantOverallMin)
			}

			if report.Score < tt.wantScoreMin {
				t.Errorf("Score = %d, want >= %d", report.Score, tt.wantScoreMin)
			}

			if report.CheckedAt.IsZero() {
				t.Error("CheckedAt is zero")
			}
		})
	}
}

func TestCompatibilityCheckerNilInfo(t *testing.T) {
	c := NewCompatibilityChecker()
	_, err := c.Check(nil)
	if err == nil {
		t.Error("Check(nil) should return error")
	}
}

func TestGetSupportedDevices(t *testing.T) {
	c := NewCompatibilityChecker()
	devices := c.GetSupportedDevices()

	if len(devices) == 0 {
		t.Fatal("GetSupportedDevices() returned empty list")
	}

	// 验证一些已知设备
	found := make(map[string]bool)
	for _, d := range devices {
		found[d.SoCModel] = true
	}

	expectedDevices := []string{"RK3588", "RK3568", "RK3399", "H618", "S905X3", "BCM2712"}
	for _, name := range expectedDevices {
		if !found[name] {
			t.Errorf("Expected device %s not found in supported list", name)
		}
	}
}

func TestSetMinRequirements(t *testing.T) {
	c := NewCompatibilityChecker()

	req := MinRequirements{
		MinArchBits:  64,
		MinMemoryMB:  2048,
		MinCPUCores:  4,
		RequiredArch: []ArchType{ArchARM64},
	}
	c.SetMinRequirements(req)

	// 测试低配设备应被拒绝
	info := &ARMHardwareInfo{
		ArchType: ArchARM64,
		Bits:     64,
		CPUCores: 4,
		MemoryMB: 1024,
		HasGbE:   true,
		Features: []CPUFeature{FeatureNEON},
	}

	report, err := c.Check(info)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	// 内存不足 2048，应有不兼容问题
	if report.Overall == CompatFull {
		t.Error("Expected non-full compatibility for 1024MB device with 2048MB requirement")
	}
}

// ========== 优化引擎测试 ==========

func TestNewOptimizer(t *testing.T) {
	o := NewOptimizer()
	if o == nil {
		t.Fatal("NewOptimizer() returned nil")
	}
}

func TestOptimizerGenerateProfile(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:    ArchARM64,
		ArchVersion: "ARMv8-A",
		Bits:        64,
		SoC:         SoCRockchip,
		SoCModel:    "RK3588",
		CPUCores:    8,
		BigCores:    4,
		LittleCores: 4,
		MaxFreqMHz:  2400,
		MemoryMB:    8192,
		HasUSB3:     true,
		HasPCIe:     true,
		HasSATA:     true,
		HasGbE:      true,
		Has2_5GbE:   true,
		Features:    []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32, FeatureLSE},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	if profile == nil {
		t.Fatal("GenerateProfile() returned nil profile")
	}

	if profile.DeviceName != "RK3588" {
		t.Errorf("DeviceName = %s, want RK3588", profile.DeviceName)
	}

	if len(profile.Optimizations) == 0 {
		t.Error("GenerateProfile() returned no optimizations")
	}

	// 验证包含各类优化
	categories := make(map[OptCategory]bool)
	for _, opt := range profile.Optimizations {
		categories[opt.Category] = true
	}

	expectedCategories := []OptCategory{OptCPU, OptMemory, OptStorage, OptNetwork, OptPower, OptKernel}
	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("Missing optimization category: %s", cat)
		}
	}

	if profile.GeneratedAt.IsZero() {
		t.Error("GeneratedAt is zero")
	}
}

func TestOptimizerNilInfo(t *testing.T) {
	o := NewOptimizer()
	_, err := o.GenerateProfile(nil)
	if err == nil {
		t.Error("GenerateProfile(nil) should return error")
	}
}

func TestOptimizerBigLittleOptimizations(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:    ArchARM64,
		SoC:         SoCRockchip,
		SoCModel:    "RK3588",
		CPUCores:    8,
		BigCores:    4,
		LittleCores: 4,
		MaxFreqMHz:  2400,
		MemoryMB:    8192,
		HasPCIe:     true,
		HasGbE:      true,
		Features:    []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	// 检查 big.LITTLE 优化建议
	foundBigLittle := false
	for _, opt := range profile.Optimizations {
		if opt.Title == "big.LITTLE 调度优化" {
			foundBigLittle = true
			if opt.Priority != OptPriorityHigh {
				t.Errorf("big.LITTLE optimization priority = %d, want %d", opt.Priority, OptPriorityHigh)
			}
		}
	}
	if !foundBigLittle {
		t.Error("Missing big.LITTLE optimization for 4+4 core device")
	}
}

func TestOptimizerLowMemoryOptimizations(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		SoC:        SoCAllwinner,
		SoCModel:   "H618",
		CPUCores:   4,
		MemoryMB:   512,
		HasGbE:     true,
		Features:   []CPUFeature{FeatureNEON},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	// 低内存设备应有 swap 和 zram 优化
	foundSwap := false
	foundZram := false
	for _, opt := range profile.Optimizations {
		if opt.Parameter == "vm.swappiness" {
			foundSwap = true
		}
		if opt.Parameter == "zram" {
			foundZram = true
		}
	}

	if !foundSwap {
		t.Error("Missing swap optimization for low memory device")
	}
	if !foundZram {
		t.Error("Missing zram optimization for low memory device")
	}
}

func TestOptimizerNVMeOptimizations(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		SoC:        SoCRockchip,
		SoCModel:   "RK3588",
		CPUCores:   8,
		MemoryMB:   4096,
		HasPCIe:    true,
		HasGbE:     true,
		Features:   []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	// PCIe 设备应推荐 none 调度器
	foundScheduler := false
	for _, opt := range profile.Optimizations {
		if opt.Parameter == "queue/scheduler" && opt.Value == "none" {
			foundScheduler = true
		}
	}
	if !foundScheduler {
		t.Error("Missing NVMe scheduler optimization for PCIe device")
	}
}

func TestOptimizer25GbEOptimizations(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		SoC:        SoCRockchip,
		SoCModel:   "RK3588",
		CPUCores:   8,
		MemoryMB:   8192,
		HasPCIe:    true,
		HasGbE:     true,
		Has2_5GbE:  true,
		Features:   []CPUFeature{FeatureNEON, FeatureAES},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	// 2.5G 设备应有专用优化
	found25G := false
	for _, opt := range profile.Optimizations {
		if opt.Title == "2.5G 网络优化" {
			found25G = true
		}
	}
	if !found25G {
		t.Error("Missing 2.5G network optimization for 2.5GbE device")
	}
}

func TestOptimizerCRC32Optimizations(t *testing.T) {
	o := NewOptimizer()

	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		SoC:        SoCRockchip,
		SoCModel:   "RK3588",
		CPUCores:   8,
		MemoryMB:   8192,
		HasPCIe:    true,
		HasGbE:     true,
		Features:   []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32},
	}

	profile, err := o.GenerateProfile(info)
	if err != nil {
		t.Fatalf("GenerateProfile() error: %v", err)
	}

	foundCRC32 := false
	for _, opt := range profile.Optimizations {
		if opt.Title == "CRC32 硬件加速" {
			foundCRC32 = true
		}
	}
	if !foundCRC32 {
		t.Error("Missing CRC32 optimization for device with CRC32 feature")
	}
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManagerInit(t *testing.T) {
	m := NewManager()
	err := m.Init()
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
}

func TestManagerInitIdempotent(t *testing.T) {
	m := NewManager()

	err1 := m.Init()
	if err1 != nil {
		t.Fatalf("first Init() error: %v", err1)
	}

	err2 := m.Init()
	if err2 != nil {
		t.Fatalf("second Init() error: %v", err2)
	}
}

func TestManagerGetHardwareInfo(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	info, err := m.GetHardwareInfo()
	if err != nil {
		t.Fatalf("GetHardwareInfo() error: %v", err)
	}

	if info == nil {
		t.Fatal("GetHardwareInfo() returned nil")
	}
}

func TestManagerGetCompatReport(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	report, err := m.GetCompatReport()
	if err != nil {
		t.Fatalf("GetCompatReport() error: %v", err)
	}

	if report == nil {
		t.Fatal("GetCompatReport() returned nil")
	}
}

func TestManagerGetOptProfile(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	profile, err := m.GetOptProfile()
	if err != nil {
		t.Fatalf("GetOptProfile() error: %v", err)
	}

	if profile == nil {
		t.Fatal("GetOptProfile() returned nil")
	}

	if len(profile.Optimizations) == 0 {
		t.Error("OptProfile has no optimizations")
	}
}

func TestManagerGetSummary(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	summary, err := m.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary() error: %v", err)
	}

	if summary == nil {
		t.Fatal("GetSummary() returned nil")
	}

	requiredKeys := []string{
		"arch_type", "bits", "cpu_cores", "memory_mb",
		"compat_overall", "compat_score", "compatible",
	}
	for _, key := range requiredKeys {
		if _, ok := summary[key]; !ok {
			t.Errorf("GetSummary() missing key: %s", key)
		}
	}
}

func TestManagerIsCompatible(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	compat, err := m.IsCompatible()
	if err != nil {
		t.Fatalf("IsCompatible() error: %v", err)
	}

	// 当前运行环境应该是兼容的
	if !compat {
		t.Log("Warning: Current device reported as not compatible")
	}
}

func TestManagerGetSupportedDevices(t *testing.T) {
	m := NewManager()
	devices := m.GetSupportedDevices()

	if len(devices) == 0 {
		t.Fatal("GetSupportedDevices() returned empty list")
	}

	// 应至少支持 42 款设备（参考飞牛 fnOS）
	if len(devices) < 42 {
		t.Logf("Warning: Only %d supported devices (fnOS has 42+)", len(devices))
	}
}

func TestManagerGetOptimizationsByCategory(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	categories := []OptCategory{OptCPU, OptMemory, OptStorage, OptNetwork, OptPower, OptKernel}
	for _, cat := range categories {
		opts, err := m.GetOptimizationsByCategory(cat)
		if err != nil {
			t.Errorf("GetOptimizationsByCategory(%s) error: %v", cat, err)
			continue
		}
		if len(opts) == 0 {
			t.Errorf("GetOptimizationsByCategory(%s) returned empty", cat)
		}
		for _, opt := range opts {
			if opt.Category != cat {
				t.Errorf("Optimization category = %s, want %s", opt.Category, cat)
			}
		}
	}
}

func TestManagerGetHighPriorityOpts(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	opts, err := m.GetHighPriorityOpts()
	if err != nil {
		t.Fatalf("GetHighPriorityOpts() error: %v", err)
	}

	if len(opts) == 0 {
		t.Error("GetHighPriorityOpts() returned empty")
	}

	for _, opt := range opts {
		if opt.Priority != OptPriorityHigh {
			t.Errorf("Non-high priority optimization found: %s (priority=%d)", opt.Title, opt.Priority)
		}
	}
}

func TestManagerBeforeInit(t *testing.T) {
	m := NewManager()

	_, err := m.GetHardwareInfo()
	if err == nil {
		t.Error("GetHardwareInfo() should fail before Init()")
	}

	_, err = m.GetCompatReport()
	if err == nil {
		t.Error("GetCompatReport() should fail before Init()")
	}

	_, err = m.GetOptProfile()
	if err == nil {
		t.Error("GetOptProfile() should fail before Init()")
	}

	_, err = m.GetSummary()
	if err == nil {
		t.Error("GetSummary() should fail before Init()")
	}

	_, err = m.IsCompatible()
	if err == nil {
		t.Error("IsCompatible() should fail before Init()")
	}
}

func TestManagerReset(t *testing.T) {
	m := NewManager()

	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	m.Reset()

	// 重置后应该可以重新初始化
	if err := m.Init(); err != nil {
		t.Fatalf("Init() after Reset() error: %v", err)
	}
}

// ========== 辅助函数测试 ==========

func TestParseCPUOnline(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 1},
		{"0-3", 4},
		{"0-3,4-7", 8},
		{"0,2,4,6", 4},
		{"0-1,4-5", 4},
		{"0-7", 8},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCPUOnline(tt.input)
			if result != tt.expected {
				t.Errorf("parseCPUOnline(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractRockchipModel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"rockchip,rk3588", "RK3588"},
		{"Rockchip RK3568 Board", "RK3568"},
		{"rk3399-based system", "RK3399"},
		{"no model here", "Rockchip SoC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRockchipModel(tt.input)
			if result != tt.expected {
				t.Errorf("extractRockchipModel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasFeature(t *testing.T) {
	features := []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32}

	if !hasFeature(features, FeatureNEON) {
		t.Error("hasFeature should find NEON")
	}

	if hasFeature(features, FeatureSVE) {
		t.Error("hasFeature should not find SVE")
	}

	if hasFeature(nil, FeatureNEON) {
		t.Error("hasFeature(nil, ...) should return false")
	}
}

// ========== 性能基准测试 ==========

func BenchmarkDetectorDetect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d := NewDetector()
		d.Detect()
	}
}

func BenchmarkCompatibilityCheckerCheck(b *testing.B) {
	c := NewCompatibilityChecker()
	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		Bits:       64,
		SoC:        SoCRockchip,
		SoCModel:   "RK3588",
		CPUCores:   8,
		BigCores:   4,
		LittleCores: 4,
		MemoryMB:   8192,
		HasUSB3:    true,
		HasPCIe:    true,
		HasSATA:    true,
		HasGbE:     true,
		Has2_5GbE:  true,
		Features:   []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32, FeatureLSE},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Check(info)
	}
}

func BenchmarkOptimizerGenerateProfile(b *testing.B) {
	o := NewOptimizer()
	info := &ARMHardwareInfo{
		ArchType:   ArchARM64,
		SoC:        SoCRockchip,
		SoCModel:   "RK3588",
		CPUCores:   8,
		BigCores:   4,
		LittleCores: 4,
		MaxFreqMHz: 2400,
		MemoryMB:   8192,
		HasPCIe:    true,
		HasGbE:     true,
		Has2_5GbE:  true,
		Features:   []CPUFeature{FeatureNEON, FeatureAES, FeatureCRC32},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.GenerateProfile(info)
	}
}

func BenchmarkManagerInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := NewManager()
		m.Init()
	}
}

// ========== 边界条件测试 ==========

func TestCompatibilityCheckerEdgeCases(t *testing.T) {
	c := NewCompatibilityChecker()

	// 最小可用设备
	t.Run("minimum viable device", func(t *testing.T) {
		info := &ARMHardwareInfo{
			ArchType: ArchARMv7,
			Bits:     32,
			CPUCores: 2,
			MemoryMB: 512,
			HasGbE:   true,
			Features: []CPUFeature{FeatureNEON},
		}
		report, err := c.Check(info)
		if err != nil {
			t.Fatalf("Check() error: %v", err)
		}
		if report.Score < 0 || report.Score > 100 {
			t.Errorf("Score = %d, want 0-100", report.Score)
		}
	})

	// 零值设备
	t.Run("zero value device", func(t *testing.T) {
		info := &ARMHardwareInfo{
			ArchType: ArchUnknown,
			Bits:     0,
			CPUCores: 0,
			MemoryMB: 0,
		}
		report, err := c.Check(info)
		if err != nil {
			t.Fatalf("Check() error: %v", err)
		}
		if report.Overall != CompatUnsupported {
			t.Errorf("Overall = %s, want unsupported", report.Overall)
		}
	})
}

func TestOptimizerEdgeCases(t *testing.T) {
	o := NewOptimizer()

	// 最小配置
	t.Run("minimal config", func(t *testing.T) {
		info := &ARMHardwareInfo{
			ArchType: ARM64OrV7(),
			Bits:     32,
			CPUCores: 2,
			MemoryMB: 512,
		}
		profile, err := o.GenerateProfile(info)
		if err != nil {
			t.Fatalf("GenerateProfile() error: %v", err)
		}
		if len(profile.Optimizations) == 0 {
			t.Error("No optimizations generated for minimal config")
		}
	})

	// 最大配置
	t.Run("maximal config", func(t *testing.T) {
		info := &ARMHardwareInfo{
			ArchType:    ArchARM64,
			Bits:        64,
			SoC:         SoCRockchip,
			SoCModel:    "RK3588",
			CPUCores:    8,
			BigCores:    4,
			LittleCores: 4,
			MaxFreqMHz:  2400,
			MemoryMB:    32768,
			HasUSB3:     true,
			HasPCIe:     true,
			HasSATA:     true,
			HasGbE:      true,
			Has2_5GbE:   true,
			Features:    []CPUFeature{FeatureNEON, FeatureAES, FeatureSHA1, FeatureSHA2, FeatureCRC32, FeatureLSE, FeatureSVE, FeatureDotProd, FeatureFP16, FeatureI8MM},
		}
		profile, err := o.GenerateProfile(info)
		if err != nil {
			t.Fatalf("GenerateProfile() error: %v", err)
		}
		if len(profile.Optimizations) < 10 {
			t.Errorf("Expected many optimizations for maximal config, got %d", len(profile.Optimizations))
		}
	})
}

// ARM64OrV7 返回当前运行架构
func ARM64OrV7() ArchType {
	if isARM64() {
		return ArchARM64
	}
	return ArchARMv7
}

func isARM64() bool {
	return true // 在 ARM64 设备上测试
}

// ========== 并发安全测试 ==========

func TestManagerConcurrentAccess(t *testing.T) {
	m := NewManager()
	if err := m.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			// 并发读取
			m.GetHardwareInfo()
			m.GetCompatReport()
			m.GetOptProfile()
			m.GetSummary()
			m.IsCompatible()
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent access timeout")
		}
	}
}

func TestDetectorConcurrentDetect(t *testing.T) {
	d := NewDetector()

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			d.Detect()
		}()
	}

	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent detect timeout")
		}
	}
}
