package desktoporg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DesktopManager 桌面图标管理器
type DesktopManager struct {
	mu          sync.RWMutex
	icons       map[string]*DesktopIcon
	groups      map[string]*IconGroup
	layouts     map[string]*DesktopLayout
	currentLayout string
	storagePath string
	gridSize    GridSize
}

// DesktopConfig 桌面配置
type DesktopConfig struct {
	StoragePath string   `json:"storage_path"`
	GridSize    GridSize `json:"grid_size"`
}

// NewDesktopManager 创建桌面管理器
func NewDesktopManager(cfg *DesktopConfig) *DesktopManager {
	if cfg == nil {
		cfg = &DesktopConfig{
			StoragePath: "/var/lib/nas-os/desktop",
			GridSize: GridSize{
				Columns: 12,
				Rows:    8,
				CellW:   80,
				CellH:   80,
				Gap:     10,
			},
		}
	}

	m := &DesktopManager{
		icons:       make(map[string]*DesktopIcon),
		groups:      make(map[string]*IconGroup),
		layouts:     make(map[string]*DesktopLayout),
		storagePath: cfg.StoragePath,
		gridSize:    cfg.GridSize,
	}

	return m
}

// Init 初始化管理器，加载持久化数据
func (m *DesktopManager) Init() error {
	if err := os.MkdirAll(m.storagePath, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 尝试加载已有数据
	if err := m.loadFromDisk(); err != nil {
		// 首次运行，创建默认布局
		m.createDefaultLayout()
	}

	return nil
}

// createDefaultLayout 创建默认布局
func (m *DesktopManager) createDefaultLayout() {
	defaultLayout := &DesktopLayout{
		ID:        "default",
		Name:      "默认布局",
		IsDefault: true,
		Screens: []ScreenLayout{
			{
				ScreenID: "screen-0",
				Name:     "主屏幕",
				Resolution: Size{Width: 1920, Height: 1080},
				Primary:  true,
			},
		},
		GridSize: m.gridSize,
		Theme: LayoutTheme{
			IconStyle:     "flat",
			LabelPosition: "bottom",
			LabelColor:    "#ffffff",
			ShowGrid:      false,
			AnimateIcons:  true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.layouts[defaultLayout.ID] = defaultLayout
	m.currentLayout = defaultLayout.ID
}

// Icon CRUD 操作

// CreateIcon 创建图标
func (m *DesktopManager) CreateIcon(req *CreateIconRequest) (*DesktopIcon, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证位置有效性
	if err := m.validatePosition(req.Position, req.ScreenID); err != nil {
		return nil, err
	}

	// 检查位置冲突
	if err := m.checkPositionConflict(req.Position, req.ScreenID, ""); err != nil {
		return nil, err
	}

	icon := &DesktopIcon{
		ID:        generateID("icon"),
		Name:      req.Name,
		IconURL:   req.IconURL,
		AppID:     req.AppID,
		Type:      req.Type,
		Position:  req.Position,
		GroupID:   req.GroupID,
		ScreenID:  req.ScreenID,
		Size:      req.Size,
		Visible:   true,
		Locked:    false,
		Command:   req.Command,
		Tooltip:   req.Tooltip,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 设置默认值
	if icon.Type == "" {
		icon.Type = IconTypeApp
	}
	if icon.Size == "" {
		icon.Size = SizeMedium
	}
	if icon.ScreenID == "" {
		icon.ScreenID = "screen-0"
	}

	m.icons[icon.ID] = icon

	// 如果指定了分组，添加到分组
	if icon.GroupID != "" {
		if group, exists := m.groups[icon.GroupID]; exists {
			group.IconIDs = append(group.IconIDs, icon.ID)
			group.UpdatedAt = time.Now()
		}
	}

	// 更新布局
	m.updateLayoutIcons()

	return icon, nil
}

// GetIcon 获取图标
func (m *DesktopManager) GetIcon(id string) (*DesktopIcon, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	icon, exists := m.icons[id]
	if !exists {
		return nil, fmt.Errorf("图标 %s 不存在", id)
	}

	return icon, nil
}

// UpdateIcon 更新图标
func (m *DesktopManager) UpdateIcon(id string, req *UpdateIconRequest) (*DesktopIcon, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	icon, exists := m.icons[id]
	if !exists {
		return nil, fmt.Errorf("图标 %s 不存在", id)
	}

	if icon.Locked {
		return nil, fmt.Errorf("图标 %s 已锁定，无法修改", id)
	}

	// 更新字段
	if req.Name != nil {
		icon.Name = *req.Name
	}
	if req.IconURL != nil {
		icon.IconURL = *req.IconURL
	}
	if req.Position != nil {
		if err := m.checkPositionConflict(*req.Position, icon.ScreenID, id); err != nil {
			return nil, err
		}
		icon.Position = *req.Position
	}
	if req.GroupID != nil {
		// 从旧分组移除
		m.removeIconFromGroup(id, icon.GroupID)
		// 添加到新分组
		if *req.GroupID != "" {
			if group, exists := m.groups[*req.GroupID]; exists {
				group.IconIDs = append(group.IconIDs, id)
				group.UpdatedAt = time.Now()
			}
		}
		icon.GroupID = *req.GroupID
	}
	if req.ScreenID != nil {
		icon.ScreenID = *req.ScreenID
	}
	if req.Size != nil {
		icon.Size = *req.Size
	}
	if req.Visible != nil {
		icon.Visible = *req.Visible
	}
	if req.Locked != nil {
		icon.Locked = *req.Locked
	}
	if req.Command != nil {
		icon.Command = *req.Command
	}
	if req.Tooltip != nil {
		icon.Tooltip = *req.Tooltip
	}

	icon.UpdatedAt = time.Now()
	m.updateLayoutIcons()

	return icon, nil
}

// DeleteIcon 删除图标
func (m *DesktopManager) DeleteIcon(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	icon, exists := m.icons[id]
	if !exists {
		return fmt.Errorf("图标 %s 不存在", id)
	}

	// 从分组中移除
	m.removeIconFromGroup(id, icon.GroupID)

	delete(m.icons, id)
	m.updateLayoutIcons()

	return nil
}

// MoveIcon 移动图标
func (m *DesktopManager) MoveIcon(id string, req *MoveIconRequest) (*DesktopIcon, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	icon, exists := m.icons[id]
	if !exists {
		return nil, fmt.Errorf("图标 %s 不存在", id)
	}

	if icon.Locked {
		return nil, fmt.Errorf("图标 %s 已锁定，无法移动", id)
	}

	if err := m.checkPositionConflict(req.Position, req.ScreenID, id); err != nil {
		return nil, err
	}

	icon.Position = req.Position
	if req.ScreenID != "" {
		icon.ScreenID = req.ScreenID
	}
	icon.UpdatedAt = time.Now()

	m.updateLayoutIcons()

	return icon, nil
}

// ListIcons 列出图标
func (m *DesktopManager) ListIcons(screenID string) []*DesktopIcon {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DesktopIcon, 0)
	for _, icon := range m.icons {
		if screenID == "" || icon.ScreenID == screenID {
			result = append(result, icon)
		}
	}

	// 按位置排序
	sort.Slice(result, func(i, j int) bool {
		if result[i].ScreenID != result[j].ScreenID {
			return result[i].ScreenID < result[j].ScreenID
		}
		if result[i].Position.Y != result[j].Position.Y {
			return result[i].Position.Y < result[j].Position.Y
		}
		return result[i].Position.X < result[j].Position.X
	})

	return result
}

// Group CRUD 操作

// CreateGroup 创建分组
func (m *DesktopManager) CreateGroup(req *CreateGroupRequest) (*IconGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validatePosition(req.Position, req.ScreenID); err != nil {
		return nil, err
	}

	group := &IconGroup{
		ID:          generateID("group"),
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Color:       req.Color,
		Position:    req.Position,
		ScreenID:    req.ScreenID,
		Collapsed:   false,
		IconIDs:     make([]string, 0),
		Layout:      req.Layout,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if group.Layout == "" {
		group.Layout = LayoutGrid
	}
	if group.ScreenID == "" {
		group.ScreenID = "screen-0"
	}

	m.groups[group.ID] = group
	m.updateLayoutGroups()

	return group, nil
}

// GetGroup 获取分组
func (m *DesktopManager) GetGroup(id string) (*IconGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("分组 %s 不存在", id)
	}

	return group, nil
}

// UpdateGroup 更新分组
func (m *DesktopManager) UpdateGroup(id string, req *UpdateGroupRequest) (*IconGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("分组 %s 不存在", id)
	}

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = *req.Description
	}
	if req.Icon != nil {
		group.Icon = *req.Icon
	}
	if req.Color != nil {
		group.Color = *req.Color
	}
	if req.Position != nil {
		group.Position = *req.Position
	}
	if req.Collapsed != nil {
		group.Collapsed = *req.Collapsed
	}
	if req.Layout != nil {
		group.Layout = *req.Layout
	}

	group.UpdatedAt = time.Now()

	return group, nil
}

// DeleteGroup 删除分组
func (m *DesktopManager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[id]
	if !exists {
		return fmt.Errorf("分组 %s 不存在", id)
	}

	// 将分组内的图标移出分组
	for _, iconID := range group.IconIDs {
		if icon, exists := m.icons[iconID]; exists {
			icon.GroupID = ""
			icon.UpdatedAt = time.Now()
		}
	}

	delete(m.groups, id)
	m.updateLayoutGroups()

	return nil
}

// ListGroups 列出分组
func (m *DesktopManager) ListGroups(screenID string) []*IconGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IconGroup, 0)
	for _, group := range m.groups {
		if screenID == "" || group.ScreenID == screenID {
			result = append(result, group)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ScreenID != result[j].ScreenID {
			return result[i].ScreenID < result[j].ScreenID
		}
		if result[i].Position.Y != result[j].Position.Y {
			return result[i].Position.Y < result[j].Position.Y
		}
		return result[i].Position.X < result[j].Position.X
	})

	return result
}

// AddIconToGroup 添加图标到分组
func (m *DesktopManager) AddIconToGroup(groupID, iconID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}

	icon, exists := m.icons[iconID]
	if !exists {
		return fmt.Errorf("图标 %s 不存在", iconID)
	}

	// 检查是否已在分组中
	for _, id := range group.IconIDs {
		if id == iconID {
			return nil // 已在分组中
		}
	}

	// 从旧分组移除
	m.removeIconFromGroup(iconID, icon.GroupID)

	// 添加到新分组
	group.IconIDs = append(group.IconIDs, iconID)
	icon.GroupID = groupID
	icon.UpdatedAt = time.Now()
	group.UpdatedAt = time.Now()

	return nil
}

// RemoveIconFromGroup 从分组移除图标
func (m *DesktopManager) RemoveIconFromGroup(groupID, iconID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	icon, exists := m.icons[iconID]
	if !exists {
		return fmt.Errorf("图标 %s 不存在", iconID)
	}

	if icon.GroupID != groupID {
		return fmt.Errorf("图标 %s 不在分组 %s 中", iconID, groupID)
	}

	m.removeIconFromGroup(iconID, groupID)
	icon.GroupID = ""
	icon.UpdatedAt = time.Now()

	return nil
}

// Layout 操作

// GetLayout 获取当前布局
func (m *DesktopManager) GetLayout() (*DesktopLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layout, exists := m.layouts[m.currentLayout]
	if !exists {
		return nil, fmt.Errorf("当前布局不存在")
	}

	return layout, nil
}

// SaveLayout 保存布局
func (m *DesktopManager) SaveLayout(layout *DesktopLayout) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout.UpdatedAt = time.Now()
	m.layouts[layout.ID] = layout

	return m.persistToDisk()
}

// SetDefaultLayout 设置默认布局
func (m *DesktopManager) SetDefaultLayout(layoutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, exists := m.layouts[layoutID]
	if !exists {
		return fmt.Errorf("布局 %s 不存在", layoutID)
	}

	// 取消其他布局的默认状态
	for _, l := range m.layouts {
		l.IsDefault = false
	}

	layout.IsDefault = true
	m.currentLayout = layoutID

	return nil
}

// AddScreen 添加屏幕
func (m *DesktopManager) AddScreen(name string, resolution Size, primary bool) (*ScreenLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, exists := m.layouts[m.currentLayout]
	if !exists {
		return nil, fmt.Errorf("当前布局不存在")
	}

	// 如果设为主屏幕，取消其他主屏幕
	if primary {
		for i := range layout.Screens {
			layout.Screens[i].Primary = false
		}
	}

	screen := ScreenLayout{
		ScreenID:   fmt.Sprintf("screen-%d", len(layout.Screens)),
		Name:       name,
		Resolution: resolution,
		Primary:    primary,
	}

	layout.Screens = append(layout.Screens, screen)
	layout.UpdatedAt = time.Now()

	return &screen, nil
}

// RemoveScreen 移除屏幕
func (m *DesktopManager) RemoveScreen(screenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, exists := m.layouts[m.currentLayout]
	if !exists {
		return fmt.Errorf("当前布局不存在")
	}

	// 不允许移除主屏幕
	for _, s := range layout.Screens {
		if s.ScreenID == screenID && s.Primary {
			return fmt.Errorf("不能移除主屏幕")
		}
	}

	// 移除屏幕
	for i, s := range layout.Screens {
		if s.ScreenID == screenID {
			layout.Screens = append(layout.Screens[:i], layout.Screens[i+1:]...)
			break
		}
	}

	// 将该屏幕的图标移到主屏幕
	for _, icon := range m.icons {
		if icon.ScreenID == screenID {
			icon.ScreenID = "screen-0"
			icon.UpdatedAt = time.Now()
		}
	}

	layout.UpdatedAt = time.Now()

	return nil
}

// 内部辅助方法

func (m *DesktopManager) validatePosition(pos Position, screenID string) error {
	if pos.X < 0 || pos.Y < 0 {
		return fmt.Errorf("位置坐标不能为负数")
	}
	if pos.X >= m.gridSize.Columns || pos.Y >= m.gridSize.Rows {
		return fmt.Errorf("位置超出网格范围 (%d,%d)", m.gridSize.Columns, m.gridSize.Rows)
	}
	return nil
}

func (m *DesktopManager) checkPositionConflict(pos Position, screenID, excludeID string) error {
	for _, icon := range m.icons {
		if icon.ID == excludeID {
			continue
		}
		if icon.ScreenID == screenID &&
			icon.Position.X == pos.X &&
			icon.Position.Y == pos.Y {
			return fmt.Errorf("位置 (%d,%d) 已被图标 %s 占用", pos.X, pos.Y, icon.Name)
		}
	}
	return nil
}

func (m *DesktopManager) removeIconFromGroup(iconID, groupID string) {
	if groupID == "" {
		return
	}
	if group, exists := m.groups[groupID]; exists {
		for i, id := range group.IconIDs {
			if id == iconID {
				group.IconIDs = append(group.IconIDs[:i], group.IconIDs[i+1:]...)
				group.UpdatedAt = time.Now()
				break
			}
		}
	}
}

func (m *DesktopManager) updateLayoutIcons() {
	layout, exists := m.layouts[m.currentLayout]
	if !exists {
		return
	}

	// 按屏幕收集图标
	screenIcons := make(map[string][]string)
	for _, icon := range m.icons {
		screenIcons[icon.ScreenID] = append(screenIcons[icon.ScreenID], icon.ID)
	}

	for i, screen := range layout.Screens {
		layout.Screens[i].IconIDs = screenIcons[screen.ScreenID]
	}
}

func (m *DesktopManager) updateLayoutGroups() {
	layout, exists := m.layouts[m.currentLayout]
	if !exists {
		return
	}

	screenGroups := make(map[string][]string)
	for _, group := range m.groups {
		screenGroups[group.ScreenID] = append(screenGroups[group.ScreenID], group.ID)
	}

	for i, screen := range layout.Screens {
		layout.Screens[i].GroupIDs = screenGroups[screen.ScreenID]
	}
}

// 持久化

func (m *DesktopManager) persistToDisk() error {
	data := map[string]interface{}{
		"icons":          m.icons,
		"groups":         m.groups,
		"layouts":        m.layouts,
		"current_layout": m.currentLayout,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	filePath := filepath.Join(m.storagePath, "desktop.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func (m *DesktopManager) loadFromDisk() error {
	filePath := filepath.Join(m.storagePath, "desktop.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在")
		}
		return err
	}

	var persisted struct {
		Icons       map[string]*DesktopIcon   `json:"icons"`
		Groups      map[string]*IconGroup     `json:"groups"`
		Layouts     map[string]*DesktopLayout `json:"layouts"`
		CurrentLayout string                  `json:"current_layout"`
	}

	if err := json.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.icons = persisted.Icons
	m.groups = persisted.Groups
	m.layouts = persisted.Layouts
	m.currentLayout = persisted.CurrentLayout

	if m.icons == nil {
		m.icons = make(map[string]*DesktopIcon)
	}
	if m.groups == nil {
		m.groups = make(map[string]*IconGroup)
	}
	if m.layouts == nil {
		m.layouts = make(map[string]*DesktopLayout)
	}

	return nil
}

// Save 持久化当前状态
func (m *DesktopManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.persistToDisk()
}

// generateID 生成唯一ID
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
