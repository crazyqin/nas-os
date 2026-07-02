package dsmagent

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// GuidedWizard 引导式向导，帮助管理员完成复杂任务
// 提供步骤引导、用户交互、进度跟踪和错误恢复能力.
type GuidedWizard struct {
	mu        sync.RWMutex
	templates map[string]*WizardTemplate // 向导模板库
	sessions  map[string]*WizardSession  // 活跃会话
}

// WizardTemplate 向导模板.
type WizardTemplate struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Category      string          `json:"category"`
	Steps         []WizardStepDef `json:"steps"`
	RequiredRole  AgentRole       `json:"required_role"`
	EstimatedTime time.Duration   `json:"estimated_time"` // 预估完成时间
}

// WizardStepDef 向导步骤定义.
type WizardStepDef struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        WizardStepType `json:"type"`
	Fields      []WizardField  `json:"fields,omitempty"`     // 需要用户输入的字段
	Validation  string         `json:"validation,omitempty"` // 验证规则
	Optional    bool           `json:"optional,omitempty"`   // 是否可选步骤
}

// WizardStepType 向导步骤类型.
type WizardStepType string

const (
	StepTypeInput   WizardStepType = "input"   // 用户输入
	StepTypeSelect  WizardStepType = "select"  // 单选
	StepTypeMulti   WizardStepType = "multi"   // 多选
	StepTypeConfirm WizardStepType = "confirm" // 确认
	StepTypeInfo    WizardStepType = "info"    // 信息展示
	StepTypeExecute WizardStepType = "execute" // 执行操作
)

// WizardField 向导字段定义.
type WizardField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text, number, password, select
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"` // 用于select类型
	Placeholder string   `json:"placeholder,omitempty"`
	HelpText    string   `json:"help_text,omitempty"`
}

// WizardSession 向导会话实例.
type WizardSession struct {
	ID          string                 `json:"id"`
	TemplateID  string                 `json:"template_id"`
	UserID      string                 `json:"user_id"`
	Status      WizardStatus           `json:"status"`
	CurrentStep int                    `json:"current_step"` // 当前步骤索引
	Responses   map[string]interface{} `json:"responses"`    // 用户响应数据
	Results     map[string]interface{} `json:"results"`      // 执行结果
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// WizardStatus 向导状态.
type WizardStatus string

const (
	WizardStatusActive    WizardStatus = "active"
	WizardStatusPaused    WizardStatus = "paused"
	WizardStatusCompleted WizardStatus = "completed"
	WizardStatusFailed    WizardStatus = "failed"
	WizardStatusCancelled WizardStatus = "cancelled"
)

// WizardStepInfo 向导步骤信息（返回给客户端）.
type WizardStepInfo struct {
	StepIndex  int                    `json:"step_index"`
	TotalSteps int                    `json:"total_steps"`
	Step       WizardStepDef          `json:"step"`
	Progress   float64                `json:"progress"`            // 进度百分比
	PrevData   map[string]interface{} `json:"prev_data,omitempty"` // 之前步骤的数据
}

// NewGuidedWizard 创建引导式向导实例.
func NewGuidedWizard() *GuidedWizard {
	wizard := &GuidedWizard{
		templates: make(map[string]*WizardTemplate),
		sessions:  make(map[string]*WizardSession),
	}

	// 注册默认向导模板
	wizard.registerDefaultTemplates()

	return wizard
}

// registerDefaultTemplates 注册默认向导模板.
func (w *GuidedWizard) registerDefaultTemplates() {
	// 存储池创建向导
	w.RegisterTemplate(&WizardTemplate{
		ID:            "wizard_create_storage_pool",
		Name:          "创建存储池",
		Description:   "引导您创建新的存储池，配置RAID类型和磁盘选择",
		Category:      "storage",
		RequiredRole:  RoleStorageAdmin,
		EstimatedTime: 5 * time.Minute,
		Steps: []WizardStepDef{
			{
				ID:          "step_pool_name",
				Name:        "存储池名称",
				Description: "为新存储池设置一个名称",
				Type:        StepTypeInput,
				Fields: []WizardField{
					{Name: "pool_name", Label: "存储池名称", Type: "text", Required: true, Placeholder: "例如: Volume1", HelpText: "使用英文和数字，避免特殊字符"},
				},
			},
			{
				ID:          "step_raid_type",
				Name:        "RAID类型选择",
				Description: "选择适合您需求的RAID类型",
				Type:        StepTypeSelect,
				Fields: []WizardField{
					{Name: "raid_type", Label: "RAID类型", Type: "select", Required: true, Options: []string{"Basic", "RAID 0", "RAID 1", "RAID 5", "RAID 6", "RAID 10"}, HelpText: "RAID 1/5/6提供数据冗余保护"},
				},
			},
			{
				ID:          "step_disk_select",
				Name:        "磁盘选择",
				Description: "选择要加入存储池的磁盘",
				Type:        StepTypeMulti,
				Fields: []WizardField{
					{Name: "disks", Label: "选择磁盘", Type: "multi_select", Required: true, HelpText: "根据RAID类型选择合适数量的磁盘"},
				},
			},
			{
				ID:          "step_filesystem",
				Name:        "文件系统",
				Description: "选择文件系统类型",
				Type:        StepTypeSelect,
				Fields: []WizardField{
					{Name: "filesystem", Label: "文件系统", Type: "select", Required: true, Options: []string{"Btrfs", "ext4"}, Default: "Btrfs", HelpText: "Btrfs支持快照和数据校验"},
				},
			},
			{
				ID:          "step_confirm",
				Name:        "确认创建",
				Description: "确认存储池配置",
				Type:        StepTypeConfirm,
			},
			{
				ID:          "step_execute",
				Name:        "执行创建",
				Description: "正在创建存储池...",
				Type:        StepTypeExecute,
			},
		},
	})

	// 网络配置向导
	w.RegisterTemplate(&WizardTemplate{
		ID:            "wizard_network_config",
		Name:          "网络配置",
		Description:   "引导您配置网络接口、IP地址和DNS设置",
		Category:      "network",
		RequiredRole:  RoleNetworkAdmin,
		EstimatedTime: 3 * time.Minute,
		Steps: []WizardStepDef{
			{
				ID:          "step_interface",
				Name:        "选择网卡",
				Description: "选择要配置的网络接口",
				Type:        StepTypeSelect,
				Fields: []WizardField{
					{Name: "interface", Label: "网络接口", Type: "select", Required: true, HelpText: "选择物理网卡或虚拟接口"},
				},
			},
			{
				ID:          "step_ip_mode",
				Name:        "IP配置方式",
				Description: "选择IP地址获取方式",
				Type:        StepTypeSelect,
				Fields: []WizardField{
					{Name: "ip_mode", Label: "IP模式", Type: "select", Required: true, Options: []string{"DHCP", "静态IP"}, Default: "DHCP"},
				},
			},
			{
				ID:          "step_static_ip",
				Name:        "静态IP配置",
				Description: "配置静态IP地址（仅静态IP模式需要）",
				Type:        StepTypeInput,
				Optional:    true,
				Fields: []WizardField{
					{Name: "ip_address", Label: "IP地址", Type: "text", Required: true, Placeholder: "192.168.1.100"},
					{Name: "subnet_mask", Label: "子网掩码", Type: "text", Required: true, Default: "255.255.255.0"},
					{Name: "gateway", Label: "默认网关", Type: "text", Required: true, Placeholder: "192.168.1.1"},
				},
			},
			{
				ID:          "step_dns",
				Name:        "DNS配置",
				Description: "配置DNS服务器",
				Type:        StepTypeInput,
				Fields: []WizardField{
					{Name: "dns_primary", Label: "主DNS", Type: "text", Required: true, Default: "8.8.8.8"},
					{Name: "dns_secondary", Label: "备用DNS", Type: "text", Default: "8.8.4.4"},
				},
			},
			{
				ID:          "step_confirm",
				Name:        "确认配置",
				Description: "确认网络配置",
				Type:        StepTypeConfirm,
			},
			{
				ID:          "step_execute",
				Name:        "应用配置",
				Description: "正在应用网络配置...",
				Type:        StepTypeExecute,
			},
		},
	})

	// 用户创建向导
	w.RegisterTemplate(&WizardTemplate{
		ID:            "wizard_create_user",
		Name:          "创建用户",
		Description:   "引导您创建新用户账户并配置权限",
		Category:      "user",
		RequiredRole:  RoleSystemAdmin,
		EstimatedTime: 2 * time.Minute,
		Steps: []WizardStepDef{
			{
				ID:          "step_user_info",
				Name:        "用户信息",
				Description: "设置用户基本信息",
				Type:        StepTypeInput,
				Fields: []WizardField{
					{Name: "username", Label: "用户名", Type: "text", Required: true, Placeholder: "john_doe"},
					{Name: "full_name", Label: "全名", Type: "text", Required: true},
					{Name: "email", Label: "邮箱", Type: "text", Required: true},
					{Name: "password", Label: "密码", Type: "password", Required: true},
				},
			},
			{
				ID:          "step_user_group",
				Name:        "用户组",
				Description: "选择用户所属的组",
				Type:        StepTypeMulti,
				Fields: []WizardField{
					{Name: "groups", Label: "用户组", Type: "multi_select", Required: true, HelpText: "选择用户需要加入的组"},
				},
			},
			{
				ID:          "step_quota",
				Name:        "配额设置",
				Description: "设置用户存储配额（可选）",
				Type:        StepTypeInput,
				Optional:    true,
				Fields: []WizardField{
					{Name: "quota_enabled", Label: "启用配额", Type: "select", Options: []string{"是", "否"}, Default: "否"},
					{Name: "quota_size", Label: "配额大小(GB)", Type: "number", Placeholder: "100"},
				},
			},
			{
				ID:          "step_confirm",
				Name:        "确认创建",
				Description: "确认用户配置",
				Type:        StepTypeConfirm,
			},
			{
				ID:          "step_execute",
				Name:        "创建用户",
				Description: "正在创建用户...",
				Type:        StepTypeExecute,
			},
		},
	})

	log.Printf("[GuidedWizard] 注册了 %d 个默认向导模板", len(w.templates))
}

// RegisterTemplate 注册向导模板.
func (w *GuidedWizard) RegisterTemplate(tmpl *WizardTemplate) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if tmpl.ID == "" {
		return fmt.Errorf("向导模板ID不能为空")
	}
	if _, exists := w.templates[tmpl.ID]; exists {
		return fmt.Errorf("向导模板已存在: %s", tmpl.ID)
	}

	w.templates[tmpl.ID] = tmpl
	log.Printf("[GuidedWizard] 注册向导模板: %s (%s)", tmpl.Name, tmpl.ID)
	return nil
}

// ListTemplates 列出所有向导模板.
func (w *GuidedWizard) ListTemplates(category *string) []*WizardTemplate {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var templates []*WizardTemplate
	for _, tmpl := range w.templates {
		if category == nil || tmpl.Category == *category {
			templates = append(templates, tmpl)
		}
	}
	return templates
}

// StartSession 启动新的向导会话.
func (w *GuidedWizard) StartSession(templateID, userID string) (*WizardSession, error) {
	w.mu.RLock()
	_, exists := w.templates[templateID]
	w.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("向导模板不存在: %s", templateID)
	}

	session := &WizardSession{
		ID:          fmt.Sprintf("wizard_%s_%d", templateID, time.Now().UnixNano()),
		TemplateID:  templateID,
		UserID:      userID,
		Status:      WizardStatusActive,
		CurrentStep: 0,
		Responses:   make(map[string]interface{}),
		Results:     make(map[string]interface{}),
		StartedAt:   time.Now(),
	}

	w.mu.Lock()
	w.sessions[session.ID] = session
	w.mu.Unlock()

	log.Printf("[GuidedWizard] 启动向导会话: %s (模板: %s, 用户: %s)", session.ID, templateID, userID)

	// 返回第一步信息
	return session, nil
}

// GetCurrentStep 获取当前步骤信息.
func (w *GuidedWizard) GetCurrentStep(sessionID string) (*WizardStepInfo, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	session, exists := w.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("向导会话不存在: %s", sessionID)
	}

	if session.Status != WizardStatusActive {
		return nil, fmt.Errorf("向导会话不在活跃状态: %s", session.Status)
	}

	tmpl, exists := w.templates[session.TemplateID]
	if !exists {
		return nil, fmt.Errorf("向导模板不存在: %s", session.TemplateID)
	}

	if session.CurrentStep >= len(tmpl.Steps) {
		return nil, fmt.Errorf("向导已完成所有步骤")
	}

	step := tmpl.Steps[session.CurrentStep]
	progress := float64(session.CurrentStep) / float64(len(tmpl.Steps)) * 100

	return &WizardStepInfo{
		StepIndex:  session.CurrentStep,
		TotalSteps: len(tmpl.Steps),
		Step:       step,
		Progress:   progress,
		PrevData:   session.Responses,
	}, nil
}

// SubmitStepResponse 提交当前步骤的响应.
func (w *GuidedWizard) SubmitStepResponse(sessionID string, response map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	session, exists := w.sessions[sessionID]
	if !exists {
		return fmt.Errorf("向导会话不存在: %s", sessionID)
	}

	if session.Status != WizardStatusActive {
		return fmt.Errorf("向导会话不在活跃状态: %s", session.Status)
	}

	tmpl, exists := w.templates[session.TemplateID]
	if !exists {
		return fmt.Errorf("向导模板不存在: %s", session.TemplateID)
	}

	if session.CurrentStep >= len(tmpl.Steps) {
		return fmt.Errorf("向导已完成所有步骤")
	}

	step := tmpl.Steps[session.CurrentStep]

	// 验证必填字段
	for _, field := range step.Fields {
		if field.Required {
			val, ok := response[field.Name]
			if !ok || val == nil || val == "" {
				return fmt.Errorf("必填字段缺失: %s", field.Label)
			}
		}
	}

	// 保存响应
	for key, val := range response {
		session.Responses[fmt.Sprintf("%s.%s", step.ID, key)] = val
	}

	// 如果是执行步骤，模拟执行
	if step.Type == StepTypeExecute {
		session.Results[step.ID] = map[string]interface{}{
			"status":  "success",
			"message": "操作已完成",
		}
	}

	// 前进到下一步
	session.CurrentStep++

	// 检查是否完成所有步骤
	if session.CurrentStep >= len(tmpl.Steps) {
		now := time.Now()
		session.Status = WizardStatusCompleted
		session.CompletedAt = &now
		log.Printf("[GuidedWizard] 向导会话完成: %s", sessionID)
	}

	return nil
}

// GoBack 返回上一步.
func (w *GuidedWizard) GoBack(sessionID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	session, exists := w.sessions[sessionID]
	if !exists {
		return fmt.Errorf("向导会话不存在: %s", sessionID)
	}

	if session.Status != WizardStatusActive {
		return fmt.Errorf("向导会话不在活跃状态: %s", session.Status)
	}

	if session.CurrentStep <= 0 {
		return fmt.Errorf("已经在第一步，无法后退")
	}

	session.CurrentStep--
	log.Printf("[GuidedWizard] 向导后退到步骤: %d", session.CurrentStep)
	return nil
}

// CancelSession 取消向导会话.
func (w *GuidedWizard) CancelSession(sessionID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	session, exists := w.sessions[sessionID]
	if !exists {
		return fmt.Errorf("向导会话不存在: %s", sessionID)
	}

	session.Status = WizardStatusCancelled
	now := time.Now()
	session.CompletedAt = &now

	log.Printf("[GuidedWizard] 向导会话已取消: %s", sessionID)
	return nil
}

// GetSession 获取向导会话详情.
func (w *GuidedWizard) GetSession(sessionID string) (*WizardSession, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	session, exists := w.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("向导会话不存在: %s", sessionID)
	}
	return session, nil
}

// ListSessions 列出用户的向导会话.
func (w *GuidedWizard) ListSessions(userID string) []*WizardSession {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var sessions []*WizardSession
	for _, session := range w.sessions {
		if userID == "" || session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}
