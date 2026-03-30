// Package dashboard 提供监控仪表板功能
// widget_manager.go - Widget管理器（注册、获取、布局）
package dashboard

import (
	"sync"
	"time"
)

// WidgetManager Widget管理器，负责widget的注册、获取和布局管理
type WidgetManager struct {
	mu          sync.RWMutex
	registry    *WidgetRegistry
	layoutMgr   *WidgetLayoutManager
	definitions map[WidgetType]*WidgetDefinition
}

// WidgetDefinition Widget定义，包含元数据、默认配置和创建函数
type WidgetDefinition struct {
	Type          WidgetType
	Name          string
	Description   string
	Category      WidgetCategory
	DefaultSize   WidgetSize
	DefaultConfig WidgetConfig
	CreateFunc    func() *Widget
	Icon          string
}

// WidgetCategory Widget分类
type WidgetCategory string

const (
	// CategorySystem 系统监控类
	CategorySystem WidgetCategory = "system"
	// CategoryStorage 存储监控类
	CategoryStorage WidgetCategory = "storage"
	// CategoryNetwork 网络监控类
	CategoryNetwork WidgetCategory = "network"
	// CategoryAlert 告警类
	CategoryAlert WidgetCategory = "alert"
	// CategoryCustom 自定义类
	CategoryCustom WidgetCategory = "custom"
)

// NewWidgetManager 创建Widget管理器
func NewWidgetManager() *WidgetManager {
	mgr := &WidgetManager{
		registry:    NewWidgetRegistry(),
		layoutMgr:   NewWidgetLayoutManager(),
		definitions: make(map[WidgetType]*WidgetDefinition),
	}

	// 注册内置widget定义
	mgr.registerBuiltinDefinitions()

	return mgr
}

// registerBuiltinDefinitions 注册内置widget定义
func (m *WidgetManager) registerBuiltinDefinitions() {
	// CPU监控
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeCPU,
		Name:          "CPU监控",
		Description:   "实时显示CPU使用率、负载和核心状态",
		Category:      CategorySystem,
		DefaultSize:   WidgetSizeMedium,
		DefaultConfig: WidgetConfig{ShowPerCore: true, WarningThreshold: 70, CriticalThreshold: 90},
		Icon:          "⚡",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeCPU,
				Title:       "CPU监控",
				Size:        WidgetSizeMedium,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 内存监控
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeMemory,
		Name:          "内存监控",
		Description:   "实时显示内存使用率和交换分区状态",
		Category:      CategorySystem,
		DefaultSize:   WidgetSizeMedium,
		DefaultConfig: WidgetConfig{ShowSwap: true, ShowBuffers: true, WarningThreshold: 80, CriticalThreshold: 95},
		Icon:          "💾",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeMemory,
				Title:       "内存监控",
				Size:        WidgetSizeMedium,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 磁盘监控
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeDisk,
		Name:          "磁盘监控",
		Description:   "显示磁盘使用情况和IO性能",
		Category:      CategoryStorage,
		DefaultSize:   WidgetSizeLarge,
		DefaultConfig: WidgetConfig{ShowIOStats: true},
		Icon:          "💿",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeDisk,
				Title:       "磁盘监控",
				Size:        WidgetSizeLarge,
				Enabled:     true,
				RefreshRate: 30 * time.Second,
			}
		},
	})

	// 网络监控
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeNetwork,
		Name:          "网络监控",
		Description:   "显示网络流量和接口状态",
		Category:      CategoryNetwork,
		DefaultSize:   WidgetSizeMedium,
		DefaultConfig: WidgetConfig{ShowPackets: true, ShowErrors: true},
		Icon:          "🌐",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeNetwork,
				Title:       "网络监控",
				Size:        WidgetSizeMedium,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 系统负载
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeSystemLoad,
		Name:          "系统负载",
		Description:   "显示系统负载和进程统计",
		Category:      CategorySystem,
		DefaultSize:   WidgetSizeSmall,
		DefaultConfig: WidgetConfig{},
		Icon:          "📊",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeSystemLoad,
				Title:       "系统负载",
				Size:        WidgetSizeSmall,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 存储IO
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeStorageIO,
		Name:          "存储IO",
		Description:   "实时显示磁盘读写速率",
		Category:      CategoryStorage,
		DefaultSize:   WidgetSizeMedium,
		DefaultConfig: WidgetConfig{},
		Icon:          "📈",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeStorageIO,
				Title:       "存储IO",
				Size:        WidgetSizeMedium,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 网络流量
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeNetworkTraffic,
		Name:          "网络流量",
		Description:   "实时显示网络吞吐量趋势",
		Category:      CategoryNetwork,
		DefaultSize:   WidgetSizeMedium,
		DefaultConfig: WidgetConfig{},
		Icon:          "🚀",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeNetworkTraffic,
				Title:       "网络流量",
				Size:        WidgetSizeMedium,
				Enabled:     true,
				RefreshRate: 5 * time.Second,
			}
		},
	})

	// 告警汇总
	m.RegisterDefinition(&WidgetDefinition{
		Type:          WidgetTypeAlertSummary,
		Name:          "告警汇总",
		Description:   "显示当前告警状态和数量",
		Category:      CategoryAlert,
		DefaultSize:   WidgetSizeSmall,
		DefaultConfig: WidgetConfig{},
		Icon:          "🚨",
		CreateFunc: func() *Widget {
			return &Widget{
				Type:        WidgetTypeAlertSummary,
				Title:       "告警汇总",
				Size:        WidgetSizeSmall,
				Enabled:     true,
				RefreshRate: 10 * time.Second,
			}
		},
	})
}

// RegisterDefinition 注册Widget定义
func (m *WidgetManager) RegisterDefinition(def *WidgetDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.definitions[def.Type] = def

	// 同时注册到registry（如果提供了CreateFunc）
	if def.CreateFunc != nil {
		m.registry.Register(&definitionProvider{def: def})
	}
}

// GetDefinition 获取Widget定义
func (m *WidgetManager) GetDefinition(widgetType WidgetType) (*WidgetDefinition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	def, ok := m.definitions[widgetType]
	return def, ok
}

// GetAllDefinitions 获取所有Widget定义
func (m *WidgetManager) GetAllDefinitions() []*WidgetDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	defs := make([]*WidgetDefinition, 0, len(m.definitions))
	for _, def := range m.definitions {
		defs = append(defs, def)
	}
	return defs
}

// GetDefinitionsByCategory 按分类获取Widget定义
func (m *WidgetManager) GetDefinitionsByCategory(category WidgetCategory) []*WidgetDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	defs := make([]*WidgetDefinition, 0)
	for _, def := range m.definitions {
		if def.Category == category {
			defs = append(defs, def)
		}
	}
	return defs
}

// CreateWidget 根据定义创建Widget实例
func (m *WidgetManager) CreateWidget(widgetType WidgetType) (*Widget, error) {
	def, ok := m.GetDefinition(widgetType)
	if !ok {
		return nil, ErrWidgetTypeNotFound
	}

	widget := def.CreateFunc()
	widget.ID = generateID()
	widget.CreatedAt = time.Now()
	widget.UpdatedAt = time.Now()
	widget.Config = def.DefaultConfig

	return widget, nil
}

// GetLayout 获取用户布局
func (m *WidgetManager) GetLayout(userID string) (*UserWidgetLayout, error) {
	return m.layoutMgr.GetLayout(userID)
}

// SaveLayout 保存用户布局
func (m *WidgetManager) SaveLayout(userID string, layout *UserWidgetLayout) error {
	return m.layoutMgr.SaveLayout(userID, layout)
}

// UpdateWidgetPosition 更新Widget位置
func (m *WidgetManager) UpdateWidgetPosition(userID, dashboardID, widgetID string, pos WidgetPosition) error {
	return m.layoutMgr.UpdateWidgetPosition(userID, dashboardID, widgetID, pos)
}

// ReorderWidgets 重排序Widgets
func (m *WidgetManager) ReorderWidgets(userID, dashboardID string, widgetOrder []string) error {
	return m.layoutMgr.ReorderWidgets(userID, dashboardID, widgetOrder)
}

// GetRegistry 获取Widget注册表
func (m *WidgetManager) GetRegistry() *WidgetRegistry {
	return m.registry
}

// GetLayoutManager 获取布局管理器
func (m *WidgetManager) GetLayoutManager() *WidgetLayoutManager {
	return m.layoutMgr
}

// definitionProvider 基于定义的Widget提供者
type definitionProvider struct {
	def *WidgetDefinition
}

func (p *definitionProvider) GetType() WidgetType {
	return p.def.Type
}

func (p *definitionProvider) GetData(widget *Widget) (*WidgetData, error) {
	// 基础实现，具体数据获取由专门的Provider处理
	return &WidgetData{
		WidgetID:  widget.ID,
		Type:      widget.Type,
		Timestamp: time.Now(),
		Data:      widget.Config,
	}, nil
}

// Widget错误定义
var (
	ErrWidgetTypeNotFound = NewWidgetError("widget_type_not_found", "Widget类型不存在")
	ErrWidgetNotFound     = NewWidgetError("widget_not_found", "Widget不存在")
	ErrLayoutNotFound     = NewWidgetError("layout_not_found", "布局不存在")
	ErrInvalidPosition    = NewWidgetError("invalid_position", "无效的位置参数")
)

// WidgetError Widget错误类型
type WidgetError struct {
	Code    string
	Message string
}

func NewWidgetError(code, message string) *WidgetError {
	return &WidgetError{Code: code, Message: message}
}

func (e *WidgetError) Error() string {
	return e.Message
}
