// Package onboarding 提供用户引导与快速入门系统
// wizard.go: 新手向导引擎 - 步骤式引导、断点续做、智能推荐
package onboarding

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================
// 硬件信息与RAID推荐
// ============================================================

// DiskInfo 磁盘信息（由系统扫描提供）
type DiskInfo struct {
	Device     string `json:"device"`     // 设备名，如 /dev/sda
	SizeBytes  int64  `json:"sizeBytes"`  // 磁盘大小
	Type       string `json:"type"`       // disk_type: hdd|ssd|nvme
	RPM        int    `json:"rpm"`        // 转速（SSD/NVMe 为 0）
	IsUSB      bool   `json:"isUsb"`      // 是否 USB 设备
	IsSystem   bool   `json:"isSystem"`   // 是否系统盘
	HealthOK   bool   `json:"healthOk"`   // SMART 健康状态
	Removable  bool   `json:"removable"`  // 是否可移除设备
}

// HardwareConfig 硬件配置描述
type HardwareConfig struct {
	TotalRAMGB    int        `json:"totalRamGB"`    // 内存(GB)
	CPUCores      int        `json:"cpuCores"`      // CPU 核心数
	Disks         []DiskInfo `json:"disks"`         // 可用磁盘列表
	HasUPS        bool       `json:"hasUps"`        // 是否有 UPS
	NetworkSpeed  int        `json:"networkSpeed"`  // 网络速度 (Mbps)
}

// RAIDRecommendation RAID 推荐结果
type RAIDRecommendation struct {
	Level        string  `json:"level"`        // 推荐级别: single|mirror|raidz1|raidz2|raidz3|stripe
	LevelName    string  `json:"levelName"`    // 中文名称
	Reason       string  `json:"reason"`       // 推荐理由
	MinDisks     int     `json:"minDisks"`     // 该级别最少磁盘数
	UsableRatio  float64 `json:"usableRatio"`  // 可用容量比例
	SafetyScore  int     `json:"safetyScore"`  // 安全评分 1-10
	Performance  string  `json:"performance"`  // 性能等级: high|medium|low
	Warning      string  `json:"warning,omitempty"` // 额外警告
}

// RecommendRAID 根据硬件配置推荐 RAID 级别
// 逻辑：
//   - 1 块盘 → single（单盘，提示无冗余）
//   - 2 块盘 → mirror（RAID1 镜像）
//   - 3 块盘 → raidz1（RAIDZ1，允许坏1块）
//   - 4 块盘 → raidz1（推荐）或可选 raidz2
//   - 5~8 块盘 → raidz2（RAIDZ2，允许坏2块）
//   - 9+ 块盘 → raidz3（RAIDZ3，允许坏3块）
//   - 全 SSD/NVMe → 可考虑 stripe 或 mirror（高速场景）
//   - USB 盘 → 排除不计入
func RecommendRAID(hw HardwareConfig) RAIDRecommendation {
	// 过滤掉系统盘、USB盘、不健康磁盘
	var eligible []DiskInfo
	for _, d := range hw.Disks {
		if d.IsSystem || d.IsUSB || d.Removable || !d.HealthOK {
			continue
		}
		eligible = append(eligible, d)
	}

	n := len(eligible)

	// 检查是否全部为 SSD/NVMe
	allFlash := true
	for _, d := range eligible {
		if d.Type == "hdd" {
			allFlash = false
			break
		}
	}

	switch {
	case n == 0:
		return RAIDRecommendation{
			Level:       "none",
			LevelName:   "无可用磁盘",
			Reason:      "未检测到可用磁盘，请插入非系统盘后重试",
			MinDisks:    1,
			SafetyScore: 0,
		}

	case n == 1:
		rec := RAIDRecommendation{
			Level:       "single",
			LevelName:   "单盘（无冗余）",
			Reason:      "仅检测到1块可用磁盘，无法组建冗余阵列",
			MinDisks:    1,
			UsableRatio: 1.0,
			SafetyScore: 2,
			Performance: "high",
			Warning:     "⚠️ 单盘无数据保护，磁盘故障将导致数据丢失，建议添加更多磁盘",
		}
		if eligible[0].Type == "nvme" {
			rec.Performance = "high"
		}
		return rec

	case n == 2:
		rec := RAIDRecommendation{
			Level:       "mirror",
			LevelName:   "镜像（RAID1）",
			Reason:      "2块磁盘推荐镜像模式，数据同时写入两块盘，安全可靠",
			MinDisks:    2,
			UsableRatio: 0.5,
			SafetyScore: 9,
			Performance: "medium",
		}
		if allFlash {
			rec.Performance = "high"
			rec.Reason += "；全闪存配置，读取性能优秀"
		}
		return rec

	case n == 3:
		return RAIDRecommendation{
			Level:       "raidz1",
			LevelName:   "RAIDZ1（单校验）",
			Reason:      "3块磁盘推荐RAIDZ1，允许1块盘故障，容量利用率较优",
			MinDisks:    3,
			UsableRatio: float64(n-1) / float64(n),
			SafetyScore: 7,
			Performance: "medium",
		}

	case n == 4:
		rec := RAIDRecommendation{
			Level:       "raidz1",
			LevelName:   "RAIDZ1（单校验）",
			Reason:      "4块磁盘可选RAIDZ1（容量优先）或RAIDZ2（安全优先），默认推荐RAIDZ1",
			MinDisks:    3,
			UsableRatio: float64(n-1) / float64(n),
			SafetyScore: 7,
			Performance: "medium",
		}
		if !hw.HasUPS {
			rec.Warning = "⚡ 未检测到UPS，建议考虑RAIDZ2提升数据安全性"
		}
		return rec

	case n >= 5 && n <= 8:
		rec := RAIDRecommendation{
			Level:       "raidz2",
			LevelName:   "RAIDZ2（双校验）",
			Reason:      fmt.Sprintf("%d块磁盘推荐RAIDZ2，允许同时坏2块盘，安全性与容量平衡", n),
			MinDisks:    4,
			UsableRatio: float64(n-2) / float64(n),
			SafetyScore: 9,
			Performance: "medium",
		}
		if allFlash {
			rec.Performance = "high"
			rec.Reason += "；全闪存配置，性能表现优异"
		}
		return rec

	default: // 9+
		return RAIDRecommendation{
			Level:       "raidz3",
			LevelName:   "RAIDZ3（三校验）",
			Reason:      fmt.Sprintf("%d块磁盘推荐RAIDZ3，大容量阵列需要更强的数据保护", n),
			MinDisks:    5,
			UsableRatio: float64(n-3) / float64(n),
			SafetyScore: 10,
			Performance: "medium",
		}
	}
}

// ============================================================
// 向导引擎
// ============================================================

// WizardStepStatus 向导步骤状态
type WizardStepStatus string

const (
	WizardStepPending    WizardStepStatus = "pending"
	WizardStepActive     WizardStepStatus = "active"
	WizardStepDone       WizardStepStatus = "done"
	WizardStepSkipped    WizardStepStatus = "skipped"
	WizardStepAutoDetect WizardStepStatus = "auto_detected"
)

// WizardStep 向导步骤
type WizardStep struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Order         int              `json:"order"`
	Status        WizardStepStatus `json:"status"`
	OperationHint string           `json:"operationHint"` // 操作提示
	CanSkip       bool             `json:"canSkip"`       // 是否可跳过
	AutoSkip      bool             `json:"autoSkip"`      // 自动跳过（条件满足时）
	SkipReason    string           `json:"skipReason,omitempty"` // 跳过原因
	StartedAt     *time.Time       `json:"startedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
}

// SystemDetector 系统状态检测器接口
// 生产环境中由 storagepool、users、shares 等模块实现
type SystemDetector interface {
	HasStoragePool() bool
	HasShares() bool
	HasNonAdminUsers() bool
	HasInstalledApps() bool
}

// noopDetector 默认空检测器（测试用或未接入时返回全部 false）
type noopDetector struct{}

func (n *noopDetector) HasStoragePool() bool    { return false }
func (n *noopDetector) HasShares() bool         { return false }
func (n *noopDetector) HasNonAdminUsers() bool  { return false }
func (n *noopDetector) HasInstalledApps() bool  { return false }

// WizardState 向导全局状态
type WizardState struct {
	Steps          []*WizardStep    `json:"steps"`
	CurrentStep    int              `json:"currentStep"` // 当前步骤索引
	Hardware       *HardwareConfig  `json:"hardware,omitempty"`
	Recommendation *RAIDRecommendation `json:"recommendation,omitempty"`
	StartedAt      *time.Time       `json:"startedAt,omitempty"`
	CompletedAt    *time.Time       `json:"completedAt,omitempty"`
	IsCompleted    bool             `json:"isCompleted"`
	IsSkipped      bool             `json:"isSkipped"`
}

// WizardEngine 新手向导引擎
type WizardEngine struct {
	steps     []*WizardStep
	detector  SystemDetector
	hw        *HardwareConfig
	rec       *RAIDRecommendation
	startedAt *time.Time
	completed *time.Time
	mu        sync.RWMutex
}

// NewWizardEngine 创建向导引擎
func NewWizardEngine(detector SystemDetector) *WizardEngine {
	if detector == nil {
		detector = &noopDetector{}
	}
	w := &WizardEngine{detector: detector}
	w.initSteps()
	return w
}

// initSteps 初始化向导步骤定义
func (w *WizardEngine) initSteps() {
	w.steps = []*WizardStep{
		{
			ID:            "storage_pool",
			Name:          "创建存储池",
			Description:   "选择磁盘并配置RAID级别，这是NAS数据存储的基础",
			Order:         1,
			Status:        WizardStepPending,
			OperationHint: "进入「存储管理」→ 点击「创建存储池」→ 选择磁盘 → 设置RAID级别 → 确认创建",
			CanSkip:       false,
		},
		{
			ID:            "share_config",
			Name:          "配置共享文件夹",
			Description:   "创建SMB/NFS共享，让局域网设备可以访问NAS文件",
			Order:         2,
			Status:        WizardStepPending,
			OperationHint: "进入「共享管理」→ 点击「新建共享」→ 选择数据集 → 设置共享名和权限 → 开启SMB协议",
			CanSkip:       true,
		},
		{
			ID:            "user_creation",
			Name:          "创建用户",
			Description:   "为家庭成员或同事创建账户，设置权限和配额",
			Order:         3,
			Status:        WizardStepPending,
			OperationHint: "进入「用户管理」→ 点击「快速创建」→ 选择角色模板 → 填写用户名和密码 → 确认",
			CanSkip:       true,
		},
		{
			ID:            "app_install",
			Name:          "安装应用",
			Description:   "从应用商店安装常用应用，如文件管理、照片管理、Docker等",
			Order:         4,
			Status:        WizardStepPending,
			OperationHint: "进入「应用商店」→ 浏览或搜索应用 → 点击「安装」→ 等待部署完成",
			CanSkip:       true,
		},
	}
}

// SetHardwareConfig 设置硬件配置并生成 RAID 推荐
func (w *WizardEngine) SetHardwareConfig(hw HardwareConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hw = &hw
	rec := RecommendRAID(hw)
	w.rec = &rec
}

// DetectAndAutoSkip 检测系统状态，自动跳过已完成的步骤
func (w *WizardEngine) DetectAndAutoSkip() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, step := range w.steps {
		if step.Status == WizardStepDone || step.Status == WizardStepSkipped {
			continue // 已完成或手动跳过的不处理
		}
		switch step.ID {
		case "storage_pool":
			if w.detector.HasStoragePool() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有存储池，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
				log.Printf("[Wizard] 步骤 %s 已自动跳过（存储池已存在）", step.ID)
			}
		case "share_config":
			if w.detector.HasShares() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有共享配置，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
				log.Printf("[Wizard] 步骤 %s 已自动跳过（共享已存在）", step.ID)
			}
		case "user_creation":
			if w.detector.HasNonAdminUsers() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有非管理员用户，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
				log.Printf("[Wizard] 步骤 %s 已自动跳过（用户已存在）", step.ID)
			}
		case "app_install":
			if w.detector.HasInstalledApps() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已安装应用，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
				log.Printf("[Wizard] 步骤 %s 已自动跳过（应用已安装）", step.ID)
			}
		}
	}
}

// Start 启动向导
func (w *WizardEngine) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	w.startedAt = &now

	// 找到第一个非自动跳过的步骤作为当前步骤
	for _, s := range w.steps {
		if s.Status == WizardStepPending {
			s.Status = WizardStepActive
			s.StartedAt = &now
			break
		}
	}

	log.Printf("[Wizard] 向导已启动")
	return nil
}

// CompleteStep 完成指定步骤
func (w *WizardEngine) CompleteStep(stepID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	step := w.findStep(stepID)
	if step == nil {
		return fmt.Errorf("步骤 %s 不存在", stepID)
	}
	if step.Status == WizardStepDone || step.Status == WizardStepSkipped || step.Status == WizardStepAutoDetect {
		return fmt.Errorf("步骤 %s 已完成或已跳过", stepID)
	}

	now := time.Now()
	step.Status = WizardStepDone
	step.CompletedAt = &now

	// 激活下一个待处理步骤
	w.activateNext()

	// 检查是否全部完成
	if w.allDone() {
		w.completed = &now
		log.Printf("[Wizard] 所有步骤已完成")
	}

	log.Printf("[Wizard] 步骤 %s 已完成", stepID)
	return nil
}

// SkipStep 跳过指定步骤
func (w *WizardEngine) SkipStep(stepID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	step := w.findStep(stepID)
	if step == nil {
		return fmt.Errorf("步骤 %s 不存在", stepID)
	}
	if !step.CanSkip {
		return fmt.Errorf("步骤 %s 不允许跳过", stepID)
	}
	if step.Status == WizardStepDone || step.Status == WizardStepSkipped || step.Status == WizardStepAutoDetect {
		return fmt.Errorf("步骤 %s 已完成或已跳过", stepID)
	}

	now := time.Now()
	step.Status = WizardStepSkipped
	step.CompletedAt = &now
	step.SkipReason = "用户手动跳过"

	// 激活下一个
	w.activateNext()

	log.Printf("[Wizard] 步骤 %s 已跳过", stepID)
	return nil
}

// GetState 获取向导当前状态（快照）
func (w *WizardEngine) GetState() WizardState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	steps := make([]*WizardStep, len(w.steps))
	for i, s := range w.steps {
		cp := *s
		steps[i] = &cp
	}

	state := WizardState{
		Steps:       steps,
		CurrentStep: w.currentStepIndex(),
		IsCompleted: w.completed != nil,
		IsSkipped:   false,
	}
	if w.hw != nil {
		hw := *w.hw
		state.Hardware = &hw
	}
	if w.rec != nil {
		rec := *w.rec
		state.Recommendation = &rec
	}
	if w.startedAt != nil {
		t := *w.startedAt
		state.StartedAt = &t
	}
	if w.completed != nil {
		t := *w.completed
		state.CompletedAt = &t
	}
	return state
}

// GetRecommendation 获取 RAID 推荐
func (w *WizardEngine) GetRecommendation() *RAIDRecommendation {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.rec == nil {
		return nil
	}
	cp := *w.rec
	return &cp
}

// Reset 重置向导状态
func (w *WizardEngine) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.startedAt = nil
	w.completed = nil
	for _, s := range w.steps {
		s.Status = WizardStepPending
		s.AutoSkip = false
		s.SkipReason = ""
		s.StartedAt = nil
		s.CompletedAt = nil
	}
	// 重新执行自动检测
	if w.hw != nil {
		w.detectInternal()
	}
	log.Printf("[Wizard] 向导已重置")
}

// findStep 按ID查找步骤
func (w *WizardEngine) findStep(id string) *WizardStep {
	for _, s := range w.steps {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// activateNext 找到下一个待处理步骤并激活
func (w *WizardEngine) activateNext() {
	now := time.Now()
	for _, s := range w.steps {
		if s.Status == WizardStepPending {
			s.Status = WizardStepActive
			s.StartedAt = &now
			return
		}
	}
}

// currentStepIndex 获取当前活跃步骤的索引
func (w *WizardEngine) currentStepIndex() int {
	for i, s := range w.steps {
		if s.Status == WizardStepActive {
			return i
		}
	}
	// 全部完成时返回最后一步索引
	if len(w.steps) > 0 {
		return len(w.steps) - 1
	}
	return 0
}

// allDone 检查是否所有步骤都已完成
func (w *WizardEngine) allDone() bool {
	for _, s := range w.steps {
		if s.Status == WizardStepPending || s.Status == WizardStepActive {
			return false
		}
	}
	return true
}

// detectInternal 内部自动检测（不加锁，调用者需持有锁）
func (w *WizardEngine) detectInternal() {
	for _, step := range w.steps {
		if step.Status != WizardStepPending {
			continue
		}
		switch step.ID {
		case "storage_pool":
			if w.detector.HasStoragePool() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有存储池，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
			}
		case "share_config":
			if w.detector.HasShares() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有共享配置，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
			}
		case "user_creation":
			if w.detector.HasNonAdminUsers() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已有非管理员用户，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
			}
		case "app_install":
			if w.detector.HasInstalledApps() {
				step.Status = WizardStepAutoDetect
				step.AutoSkip = true
				step.SkipReason = "检测到已安装应用，自动跳过"
				now := time.Now()
				step.CompletedAt = &now
			}
		}
	}
}
