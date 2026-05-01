// Package onboarding 提供用户引导与快速入门系统
// 首次安装时的分步引导、快速入门卡片、新手教程
// 支持引导进度追踪、跳过机制、完成率统计
package onboarding

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// StepStatus 步骤状态
type StepStatus string

const (
	StepNotStarted StepStatus = "not_started"
	StepInProgress StepStatus = "in_progress"
	StepCompleted  StepStatus = "completed"
	StepSkipped    StepStatus = "skipped"
)

// OnboardingState 引导状态
type OnboardingState string

const (
	StateNotStarted OnboardingState = "not_started"
	StateInProgress OnboardingState = "in_progress"
	StateCompleted  OnboardingState = "completed"
	StateSkipped    OnboardingState = "skipped"
)

// Step 引导步骤
type Step struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	Order            int         `json:"order"`
	Status           StepStatus  `json:"status"`
	PreConditions    []string    `json:"preConditions"`
	ValidationChecks []string    `json:"validationChecks"`
	RollbackFunc     string      `json:"rollbackFunc,omitempty"`
	StartedAt        *time.Time  `json:"startedAt,omitempty"`
	CompletedAt      *time.Time  `json:"completedAt,omitempty"`
}

// QuickStartCard 快速入门卡片
type QuickStartCard struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Action      string   `json:"action"`
	Category    string   `json:"category"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
}

// Tutorial 新手教程
type Tutorial struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
	Duration    string       `json:"duration"`
	Difficulty  string       `json:"difficulty"`
	Steps       []TutorialStep `json:"steps"`
	Tags        []string     `json:"tags"`
}

// TutorialStep 教程步骤
type TutorialStep struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	Hint        string `json:"hint,omitempty"`
	ActionURL   string `json:"actionUrl,omitempty"`
}

// ContextHelp 上下文帮助提示
type ContextHelp struct {
	Context   string `json:"context"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	ActionURL string `json:"actionUrl,omitempty"`
	Priority  int    `json:"priority"`
}

// CompletionStats 完成率统计（匿名）
type CompletionStats struct {
	TotalStarts    int64              `json:"totalStarts"`
	TotalCompleted int64              `json:"totalCompleted"`
	TotalSkipped   int64              `json:"totalSkipped"`
	CompletionRate float64            `json:"completionRate"`
	StepStats      map[string]int64   `json:"stepStats"`
	AvgDuration    time.Duration      `json:"avgDuration"`
}

// Onboarding 引导系统管理器
type Onboarding struct {
	state       OnboardingState
	steps       []*Step
	quickCards  []*QuickStartCard
	tutorials   []*Tutorial
	helpTips    map[string]*ContextHelp
	stats       CompletionStats
	startedAt   *time.Time
	completedAt *time.Time
	mu          sync.RWMutex
}

// NewOnboarding 创建引导系统
func NewOnboarding() *Onboarding {
	ob := &Onboarding{
		state:      StateNotStarted,
		helpTips:   make(map[string]*ContextHelp),
		quickCards: make([]*QuickStartCard, 0),
		tutorials:  make([]*Tutorial, 0),
		stats:      CompletionStats{StepStats: make(map[string]int64)},
	}
	ob.initSteps()
	ob.initQuickStartCards()
	ob.initTutorials()
	ob.initContextHelp()
	return ob
}

// initSteps 初始化引导步骤
func (ob *Onboarding) initSteps() {
	ob.steps = []*Step{
		{
			ID:               "storage_pool",
			Name:             "存储池创建",
			Description:      "创建您的第一个存储池，选择磁盘并配置RAID级别",
			Order:            1,
			Status:           StepNotStarted,
			PreConditions:    []string{"system_ready", "disks_available"},
			ValidationChecks: []string{"pool_exists", "pool_healthy"},
			RollbackFunc:     "destroyPool",
		},
		{
			ID:               "network_config",
			Name:             "网络配置",
			Description:      "配置网络接口、IP地址和DNS设置",
			Order:            2,
			Status:           StepNotStarted,
			PreConditions:    []string{"storage_pool_created"},
			ValidationChecks: []string{"network_reachable", "dns_working"},
			RollbackFunc:     "resetNetwork",
		},
		{
			ID:               "user_creation",
			Name:             "用户创建",
			Description:      "创建管理员账户和初始用户",
			Order:            3,
			Status:           StepNotStarted,
			PreConditions:    []string{"network_configured"},
			ValidationChecks: []string{"admin_exists", "password_set"},
			RollbackFunc:     "removeUsers",
		},
		{
			ID:               "share_setup",
			Name:             "共享设置",
			Description:      "配置SMB/NFS共享，设置访问权限",
			Order:            4,
			Status:           StepNotStarted,
			PreConditions:    []string{"users_created", "storage_pool_created"},
			ValidationChecks: []string{"share_accessible"},
			RollbackFunc:     "removeShares",
		},
		{
			ID:               "app_install",
			Name:             "应用安装",
			Description:      "安装常用应用：文件管理器、Docker等",
			Order:            5,
			Status:           StepNotStarted,
			PreConditions:    []string{"share_setup_done", "network_configured"},
			ValidationChecks: []string{"apps_running"},
			RollbackFunc:     "uninstallApps",
		},
	}
}

// initQuickStartCards 初始化快速入门卡片
func (ob *Onboarding) initQuickStartCards() {
	ob.quickCards = []*QuickStartCard{
		{ID: "create_share", Title: "创建共享文件夹", Description: "快速创建SMB/NFS共享，让设备间无缝传输文件", Icon: "folder-shared", Action: "/shares/create", Category: "storage", Priority: 1, Tags: []string{"smb", "nfs", "共享"}},
		{ID: "install_app", Title: "安装应用", Description: "从应用商店安装Docker容器化应用", Icon: "apps", Action: "/apps/catalog", Category: "apps", Priority: 2, Tags: []string{"docker", "应用", "容器"}},
		{ID: "backup_photos", Title: "照片自动备份", Description: "配置手机照片自动备份到NAS", Icon: "photo-camera", Action: "/backup/photos", Category: "backup", Priority: 3, Tags: []string{"照片", "备份", "手机"}},
		{ID: "remote_access", Title: "远程访问", Description: "配置外网访问，随时随地管理您的NAS", Icon: "cloud", Action: "/network/remote", Category: "network", Priority: 4, Tags: []string{"远程", "外网", "DDNS"}},
		{ID: "disk_health", Title: "磁盘健康检查", Description: "查看磁盘状态、SMART信息和健康度", Icon: "storage", Action: "/storage/health", Category: "storage", Priority: 5, Tags: []string{"磁盘", "健康", "SMART"}},
		{ID: "user_manage", Title: "用户管理", Description: "创建用户、设置权限和配额", Icon: "people", Action: "/users", Category: "system", Priority: 6, Tags: []string{"用户", "权限", "配额"}},
	}
}

// initTutorials 初始化新手教程
func (ob *Onboarding) initTutorials() {
	ob.tutorials = []*Tutorial{
		{
			ID: "smb_share", Title: "SMB共享入门", Description: "学习如何创建SMB共享并在Windows/Mac上访问",
			Category: "storage", Duration: "10分钟", Difficulty: "beginner",
			Tags: []string{"SMB", "Windows", "Mac", "共享"},
			Steps: []TutorialStep{
				{Title: "创建存储池", Content: "进入存储管理，选择可用磁盘创建存储池。建议新手选择RAID1镜像模式保障数据安全。", Hint: "至少需要2块磁盘才能创建RAID1"},
				{Title: "创建数据集", Content: "在存储池下创建数据集，用于存放共享文件。可设置配额限制容量。", Hint: "数据集名建议使用英文"},
				{Title: "配置SMB共享", Content: "进入共享管理，添加SMB共享，选择刚创建的数据集。设置共享名称和访问权限。"},
				{Title: "访问共享", Content: "在Windows资源管理器输入\\\\NAS-IP\\共享名，或在Finder中选择连接服务器输入smb://NAS-IP。", Hint: "首次访问需要输入用户名和密码"},
			},
		},
		{
			ID: "docker_app", Title: "Docker应用部署", Description: "学习如何安装和管理Docker容器化应用",
			Category: "apps", Duration: "15分钟", Difficulty: "intermediate",
			Tags: []string{"Docker", "容器", "应用"},
			Steps: []TutorialStep{
				{Title: "了解应用商店", Content: "打开应用商店，浏览可用应用。每个应用都有详细说明、截图和评分。"},
				{Title: "安装应用", Content: "选择需要的应用，点击安装。可自定义端口映射、存储路径等高级配置。", Hint: "建议先使用默认配置"},
				{Title: "管理应用", Content: "在已安装列表中管理应用：启动、停止、重启、更新、查看日志。"},
				{Title: "自定义Docker", Content: "高级用户可通过Docker Compose自定义部署。进入Docker管理界面即可操作。"},
			},
		},
		{
			ID: "photo_backup", Title: "照片自动备份", Description: "配置手机照片自动备份到NAS，再也不丢照片",
			Category: "backup", Duration: "8分钟", Difficulty: "beginner",
			Tags: []string{"照片", "备份", "手机", "相册"},
			Steps: []TutorialStep{
				{Title: "安装照片应用", Content: "在应用商店安装NAS Photos或Immich照片管理应用。"},
				{Title: "配置存储路径", Content: "为照片应用指定专用存储路径和配额。建议使用独立数据集。", Hint: "照片占用空间较大，建议设置充足配额"},
				{Title: "手机端设置", Content: "在手机上下载对应App，扫描NAS上的二维码完成配对。开启自动备份功能。"},
				{Title: "验证备份", Content: "在手机上拍摄几张照片，等待几分钟后在NAS Web界面检查是否已同步。"},
			},
		},
		{
			ID: "remote_access", Title: "远程访问设置", Description: "配置外网访问，在任何地方管理您的NAS",
			Category: "network", Duration: "12分钟", Difficulty: "intermediate",
			Tags: []string{"远程", "DDNS", "内网穿透", "VPN"},
			Steps: []TutorialStep{
				{Title: "选择方案", Content: "支持三种方案：DDNS直连（需公网IP）、内网穿透（无需公网IP）、VPN接入（最安全）。"},
				{Title: "DDNS配置", Content: "如果有公网IP，配置DDNS动态域名。在系统设置中填入DDNS服务商信息。"},
				{Title: "安全设置", Content: "启用HTTPS、配置防火墙规则、设置访问白名单。安全第一！", Hint: "建议开启两步验证"},
				{Title: "测试访问", Content: "使用手机4G网络（非WiFi）访问NAS，确认远程连接正常。"},
			},
		},
	}
}

// initContextHelp 初始化上下文帮助
func (ob *Onboarding) initContextHelp() {
	ob.helpTips = map[string]*ContextHelp{
		"storage_pool": {Context: "storage_pool", Title: "存储池创建帮助", Message: "存储池是NAS数据存储的基础。建议新手选择RAID1镜像模式，兼顾安全和简单。", Priority: 1},
		"network":      {Context: "network", Title: "网络配置帮助", Message: "建议使用静态IP地址，避免DHCP分配变化导致访问中断。", Priority: 2},
		"shares":       {Context: "shares", Title: "共享管理帮助", Message: "SMB适用于Windows/Mac，NFS适用于Linux。可同时开启多种协议。", Priority: 3},
		"apps":         {Context: "apps", Title: "应用管理帮助", Message: "应用商店中的应用经过测试和安全审核。自定义Docker需要一定技术基础。", Priority: 4},
		"users":        {Context: "users", Title: "用户管理帮助", Message: "建议为每位家庭成员创建独立账户，便于权限管理和使用统计。", Priority: 5},
	}
}

// GetState 获取引导状态
func (ob *Onboarding) GetState() OnboardingState {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.state
}

// Start 开始引导
func (ob *Onboarding) Start() error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if ob.state == StateInProgress {
		return fmt.Errorf("onboarding already in progress")
	}
	if ob.state == StateCompleted {
		return fmt.Errorf("onboarding already completed, use reset to restart")
	}
	ob.state = StateInProgress
	now := time.Now()
	ob.startedAt = &now
	ob.stats.TotalStarts++
	log.Printf("[Onboarding] started")
	return nil
}

// CompleteStep 完成某步骤
func (ob *Onboarding) CompleteStep(stepID string) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if ob.state != StateInProgress {
		return fmt.Errorf("onboarding not in progress")
	}
	step := ob.findStep(stepID)
	if step == nil {
		return fmt.Errorf("step %s not found", stepID)
	}
	// 检查前置条件
	if err := ob.checkPreConditions(step); err != nil {
		return fmt.Errorf("precondition failed: %w", err)
	}
	// 验证
	if err := ob.validateStep(step); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	now := time.Now()
	step.Status = StepCompleted
	step.CompletedAt = &now
	ob.stats.StepStats[stepID]++
	log.Printf("[Onboarding] step %s completed", stepID)

	// 检查是否全部完成
	if ob.allStepsCompleted() {
		ob.state = StateCompleted
		ob.completedAt = &now
		ob.stats.TotalCompleted++
		ob.stats.CompletionRate = float64(ob.stats.TotalCompleted) / float64(ob.stats.TotalStarts) * 100
		log.Printf("[Onboarding] all steps completed, rate=%.1f%%", ob.stats.CompletionRate)
	}
	return nil
}

// Skip 跳过引导
func (ob *Onboarding) Skip() error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if ob.state == StateCompleted {
		return fmt.Errorf("onboarding already completed")
	}
	ob.state = StateSkipped
	ob.stats.TotalSkipped++
	log.Printf("[Onboarding] skipped")
	return nil
}

// Reset 重置引导
func (ob *Onboarding) Reset() {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.state = StateNotStarted
	ob.startedAt = nil
	ob.completedAt = nil
	for _, step := range ob.steps {
		step.Status = StepNotStarted
		step.StartedAt = nil
		step.CompletedAt = nil
	}
	log.Printf("[Onboarding] reset")
}

// GetSteps 获取所有步骤及状态
func (ob *Onboarding) GetSteps() []*Step {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	result := make([]*Step, len(ob.steps))
	for i, s := range ob.steps {
		cp := *s
		result[i] = &cp
	}
	return result
}

// GetProgress 获取引导进度
func (ob *Onboarding) GetProgress() (total, completed, inProgress, notStarted int) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	total = len(ob.steps)
	for _, s := range ob.steps {
		switch s.Status {
		case StepCompleted:
			completed++
		case StepInProgress:
			inProgress++
		default:
			notStarted++
		}
	}
	return
}

// GetQuickStartCards 获取快速入门卡片
func (ob *Onboarding) GetQuickStartCards() []*QuickStartCard {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	result := make([]*QuickStartCard, len(ob.quickCards))
	copy(result, ob.quickCards)
	return result
}

// GetTutorials 获取教程列表
func (ob *Onboarding) GetTutorials() []*Tutorial {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	result := make([]*Tutorial, len(ob.tutorials))
	copy(result, ob.tutorials)
	return result
}

// GetTutorial 获取教程详情
func (ob *Onboarding) GetTutorial(id string) (*Tutorial, error) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	for _, t := range ob.tutorials {
		if t.ID == id {
			cp := *t
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("tutorial %s not found", id)
}

// GetContextHelp 获取上下文帮助
func (ob *Onboarding) GetContextHelp(context string) *ContextHelp {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	help, ok := ob.helpTips[context]
	if !ok {
		return nil
	}
	cp := *help
	return &cp
}

// GetCompletionStats 获取完成率统计
func (ob *Onboarding) GetCompletionStats() CompletionStats {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.stats
}

// findStep 查找步骤
func (ob *Onboarding) findStep(id string) *Step {
	for _, s := range ob.steps {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// checkPreConditions 检查前置条件
func (ob *Onboarding) checkPreConditions(step *Step) error {
	for _, cond := range step.PreConditions {
		switch cond {
		case "storage_pool_created":
			poolStep := ob.findStep("storage_pool")
			if poolStep != nil && poolStep.Status != StepCompleted {
				return fmt.Errorf("storage pool must be created first")
			}
		case "network_configured":
			netStep := ob.findStep("network_config")
			if netStep != nil && netStep.Status != StepCompleted {
				return fmt.Errorf("network must be configured first")
			}
		case "users_created":
			userStep := ob.findStep("user_creation")
			if userStep != nil && userStep.Status != StepCompleted {
				return fmt.Errorf("users must be created first")
			}
		case "share_setup_done":
			shareStep := ob.findStep("share_setup")
			if shareStep != nil && shareStep.Status != StepCompleted {
				return fmt.Errorf("shares must be set up first")
			}
		}
	}
	return nil
}

// validateStep 验证步骤
func (ob *Onboarding) validateStep(step *Step) error {
	// 模拟验证，实际运行时执行真实检查
	log.Printf("[Onboarding] validating step %s: checks=%v", step.ID, step.ValidationChecks)
	return nil
}

// allStepsCompleted 检查是否所有步骤完成
func (ob *Onboarding) allStepsCompleted() bool {
	for _, s := range ob.steps {
		if s.Status != StepCompleted && s.Status != StepSkipped {
			return false
		}
	}
	return true
}
