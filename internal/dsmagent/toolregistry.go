package dsmagent

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ToolRegistry 工具注册中心，管理系统可用的工具和技能
// 提供工具注册、发现、执行和生命周期管理能力.
type ToolRegistry struct {
	mu         sync.RWMutex
	tools      map[string]*RegisteredTool // 已注册工具
	categories map[string]*ToolCategory   // 工具分类
}

// RegisteredTool 已注册的工具.
type RegisteredTool struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	Version      string          `json:"version"`
	Author       string          `json:"author"`
	Enabled      bool            `json:"enabled"`
	Parameters   []ToolParameter `json:"parameters,omitempty"`
	Actions      []ToolAction    `json:"actions"`
	Permissions  []string        `json:"permissions,omitempty"` // 所需权限
	Timeout      time.Duration   `json:"timeout"`
	RegisteredAt time.Time       `json:"registered_at"`
	LastUsed     *time.Time      `json:"last_used,omitempty"`
	ExecCount    int64           `json:"exec_count"`
	ErrorCount   int64           `json:"error_count"`
	handler      ToolHandler     `json:"-"` // 工具处理函数
}

// ToolParameter 工具参数定义.
type ToolParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, int, bool, float
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// ToolAction 工具支持的操作.
type ToolAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolHandler 工具执行处理函数类型.
type ToolHandler func(action string, params map[string]interface{}) error

// ToolCategory 工具分类.
type ToolCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
}

// NewToolRegistry 创建工具注册中心实例.
func NewToolRegistry() *ToolRegistry {
	registry := &ToolRegistry{
		tools:      make(map[string]*RegisteredTool),
		categories: make(map[string]*ToolCategory),
	}

	// 注册默认分类
	registry.registerDefaultCategories()
	// 注册默认工具
	registry.registerDefaultTools()

	return registry
}

// registerDefaultCategories 注册默认工具分类.
func (r *ToolRegistry) registerDefaultCategories() {
	defaultCategories := []*ToolCategory{
		{ID: "cat_system", Name: "系统管理", Description: "系统级别管理工具"},
		{ID: "cat_storage", Name: "存储管理", Description: "磁盘和存储管理工具"},
		{ID: "cat_network", Name: "网络管理", Description: "网络配置和诊断工具"},
		{ID: "cat_security", Name: "安全管理", Description: "安全扫描和加固工具"},
		{ID: "cat_backup", Name: "备份恢复", Description: "数据备份和恢复工具"},
		{ID: "cat_monitor", Name: "监控告警", Description: "系统监控和告警工具"},
		{ID: "cat_service", Name: "服务管理", Description: "服务和应用管理工具"},
	}

	for _, cat := range defaultCategories {
		r.categories[cat.ID] = cat
	}
}

// registerDefaultTools 注册默认内置工具.
func (r *ToolRegistry) registerDefaultTools() {
	// CPU 检查工具
	r.Register(&RegisteredTool{
		ID:          "tool_cpu_check",
		Name:        "CPU检查",
		Description: "检查CPU使用率和负载情况",
		Category:    "cat_system",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     10 * time.Second,
		Actions: []ToolAction{
			{Name: "check_cpu", Description: "检查CPU使用率"},
			{Name: "list_processes", Description: "列出高CPU进程"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行CPU检查: %s", action)
			return nil
		},
	})

	// 内存检查工具
	r.Register(&RegisteredTool{
		ID:          "tool_memory_check",
		Name:        "内存检查",
		Description: "检查内存使用情况和交换分区",
		Category:    "cat_system",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     10 * time.Second,
		Actions: []ToolAction{
			{Name: "check_memory", Description: "检查内存使用率"},
			{Name: "list_memory_hogs", Description: "列出内存占用大户"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行内存检查: %s", action)
			return nil
		},
	})

	// 磁盘检查工具
	r.Register(&RegisteredTool{
		ID:          "tool_disk_check",
		Name:        "磁盘检查",
		Description: "检查磁盘使用率和健康状态",
		Category:    "cat_storage",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     30 * time.Second,
		Actions: []ToolAction{
			{Name: "check_disk", Description: "检查磁盘使用率"},
			{Name: "check_disk_health", Description: "检查磁盘健康状态（SMART）"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行磁盘检查: %s", action)
			return nil
		},
	})

	// 网络检查工具
	r.Register(&RegisteredTool{
		ID:          "tool_network_check",
		Name:        "网络检查",
		Description: "检查网络连接和流量状态",
		Category:    "cat_network",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     15 * time.Second,
		Actions: []ToolAction{
			{Name: "check_network", Description: "检查网络连接状态"},
			{Name: "check_bandwidth", Description: "检查带宽使用率"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行网络检查: %s", action)
			return nil
		},
	})

	// 温度检查工具
	r.Register(&RegisteredTool{
		ID:          "tool_temperature_check",
		Name:        "温度检查",
		Description: "检查系统温度传感器",
		Category:    "cat_monitor",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     10 * time.Second,
		Actions: []ToolAction{
			{Name: "check_temperature", Description: "检查系统温度"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行温度检查: %s", action)
			return nil
		},
	})

	// 安全扫描工具
	r.Register(&RegisteredTool{
		ID:          "tool_security_scan",
		Name:        "安全扫描",
		Description: "执行端口扫描和漏洞检测",
		Category:    "cat_security",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     300 * time.Second,
		Actions: []ToolAction{
			{Name: "port_scan", Description: "端口扫描"},
			{Name: "vulnerability_scan", Description: "漏洞扫描"},
			{Name: "check_permissions", Description: "权限检查"},
			{Name: "analyze_logs", Description: "安全日志分析"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行安全扫描: %s", action)
			return nil
		},
	})

	// 备份工具
	r.Register(&RegisteredTool{
		ID:          "tool_backup",
		Name:        "备份工具",
		Description: "备份管理、验证和恢复",
		Category:    "cat_backup",
		Version:     "1.0",
		Author:      "系统内置",
		Enabled:     true,
		Timeout:     600 * time.Second,
		Actions: []ToolAction{
			{Name: "list_backups", Description: "列出所有备份"},
			{Name: "verify_checksums", Description: "验证备份校验和"},
			{Name: "test_restore", Description: "测试备份恢复"},
		},
		handler: func(action string, params map[string]interface{}) error {
			log.Printf("[ToolRegistry] 执行备份操作: %s", action)
			return nil
		},
	})

	log.Printf("[ToolRegistry] 注册了 %d 个默认工具", len(r.tools))
}

// Register 注册新工具.
func (r *ToolRegistry) Register(tool *RegisteredTool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.ID == "" {
		return fmt.Errorf("工具ID不能为空")
	}
	if _, exists := r.tools[tool.ID]; exists {
		return fmt.Errorf("工具已存在: %s", tool.ID)
	}

	tool.RegisteredAt = time.Now()
	r.tools[tool.ID] = tool
	log.Printf("[ToolRegistry] 注册工具: %s (%s)", tool.Name, tool.ID)
	return nil
}

// Unregister 注销工具.
func (r *ToolRegistry) Unregister(toolID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[toolID]; !exists {
		return fmt.Errorf("工具不存在: %s", toolID)
	}

	delete(r.tools, toolID)
	log.Printf("[ToolRegistry] 注销工具: %s", toolID)
	return nil
}

// GetTool 获取已注册的工具.
func (r *ToolRegistry) GetTool(toolID string) (*RegisteredTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return nil, fmt.Errorf("工具不存在: %s", toolID)
	}
	return tool, nil
}

// ListTools 列出所有已注册工具.
func (r *ToolRegistry) ListTools(category *string, enabledOnly bool) []*RegisteredTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []*RegisteredTool
	for _, tool := range r.tools {
		if enabledOnly && !tool.Enabled {
			continue
		}
		if category == nil || tool.Category == *category {
			tools = append(tools, tool)
		}
	}
	return tools
}

// FindToolByAction 根据动作名查找工具.
func (r *ToolRegistry) FindToolByAction(action string) (*RegisteredTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		if !tool.Enabled {
			continue
		}
		for _, a := range tool.Actions {
			if a.Name == action {
				return tool, nil
			}
		}
	}
	return nil, fmt.Errorf("未找到支持动作 '%s' 的工具", action)
}

// ExecuteAction 执行指定动作.
func (r *ToolRegistry) ExecuteAction(action string, params map[string]interface{}) error {
	r.mu.RLock()
	tool, err := r.findToolForAction(action)
	r.mu.RUnlock()

	if err != nil {
		return err
	}

	if !tool.Enabled {
		return fmt.Errorf("工具已禁用: %s", tool.Name)
	}

	if tool.handler == nil {
		return fmt.Errorf("工具未配置处理函数: %s", tool.Name)
	}

	// 执行工具
	startTime := time.Now()
	execErr := tool.handler(action, params)
	duration := time.Since(startTime)

	// 更新统计
	r.mu.Lock()
	tool.ExecCount++
	now := time.Now()
	tool.LastUsed = &now
	if execErr != nil {
		tool.ErrorCount++
	}
	r.mu.Unlock()

	if execErr != nil {
		return fmt.Errorf("工具执行失败 (%s): %w", tool.Name, execErr)
	}

	log.Printf("[ToolRegistry] 动作执行成功: %s (耗时: %v)", action, duration)
	return nil
}

// findToolForAction 内部查找支持指定动作的工具.
func (r *ToolRegistry) findToolForAction(action string) (*RegisteredTool, error) {
	for _, tool := range r.tools {
		if !tool.Enabled {
			continue
		}
		for _, a := range tool.Actions {
			if a.Name == action {
				return tool, nil
			}
		}
	}
	return nil, fmt.Errorf("未找到支持动作 '%s' 的工具", action)
}

// EnableTool 启用工具.
func (r *ToolRegistry) EnableTool(toolID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return fmt.Errorf("工具不存在: %s", toolID)
	}

	tool.Enabled = true
	log.Printf("[ToolRegistry] 启用工具: %s", toolID)
	return nil
}

// DisableTool 禁用工具.
func (r *ToolRegistry) DisableTool(toolID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return fmt.Errorf("工具不存在: %s", toolID)
	}

	tool.Enabled = false
	log.Printf("[ToolRegistry] 禁用工具: %s", toolID)
	return nil
}

// GetStats 获取工具注册中心统计信息.
func (r *ToolRegistry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalTools := len(r.tools)
	enabledTools := 0
	var totalExec int64
	var totalErrors int64

	for _, tool := range r.tools {
		if tool.Enabled {
			enabledTools++
		}
		totalExec += tool.ExecCount
		totalErrors += tool.ErrorCount
	}

	return map[string]interface{}{
		"total_tools":   totalTools,
		"enabled_tools": enabledTools,
		"total_execs":   totalExec,
		"total_errors":  totalErrors,
		"categories":    len(r.categories),
	}
}

// RegisterCategory 注册工具分类.
func (r *ToolRegistry) RegisterCategory(cat *ToolCategory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.categories[cat.ID] = cat
}

// ListCategories 列出所有工具分类.
func (r *ToolRegistry) ListCategories() []*ToolCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var categories []*ToolCategory
	for _, cat := range r.categories {
		categories = append(categories, cat)
	}
	return categories
}
