// Package personalportal 提供个人门户仪表盘功能。
// engine.go 实现门户引擎核心，包括布局管理和组件渲染。
package personalportal

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 错误定义。
var (
	ErrPortalNotFound      = errors.New("门户不存在")
	ErrWidgetNotFound      = errors.New("小组件不存在")
	ErrWidgetOverlap       = errors.New("小组件位置重叠")
	ErrInvalidPosition     = errors.New("无效的位置")
	ErrFeedConfigNotFound  = errors.New("信息流配置不存在")
	ErrNotificationNotFound = errors.New("通知不存在")
	ErrPreferencesNotFound = errors.New("偏好设置不存在")
)

// PortalEngine 门户引擎。
type PortalEngine struct {
	mu            sync.RWMutex
	portals       map[string]*Portal
	feedConfigs   map[string]*FeedConfig
	feedItems     map[string][]*FeedItem
	notifications map[string][]*Notification
	preferences   map[string]*UserPreferences
}

// NewPortalEngine 创建门户引擎。
func NewPortalEngine() *PortalEngine {
	return &PortalEngine{
		portals:       make(map[string]*Portal),
		feedConfigs:   make(map[string]*FeedConfig),
		feedItems:     make(map[string][]*FeedItem),
		notifications: make(map[string][]*Notification),
		preferences:   make(map[string]*UserPreferences),
	}
}

// ========== 门户管理 ==========

// CreatePortal 创建门户。
func (pe *PortalEngine) CreatePortal(userID, name, description string, theme Theme) (*Portal, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	now := time.Now()
	portal := &Portal{
		ID:          generateID(),
		UserID:      userID,
		Name:        name,
		Description: description,
		Theme:       theme,
		Layout: Layout{
			Columns:   4,
			RowHeight: 120,
			Gap:       16,
			Compact:   false,
			Margin:    []int{16, 16, 16, 16},
		},
		Widgets:   make([]*Widget, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}

	pe.portals[portal.ID] = portal
	return portal, nil
}

// GetPortal 获取门户。
func (pe *PortalEngine) GetPortal(id string) (*Portal, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	portal, exists := pe.portals[id]
	if !exists {
		return nil, ErrPortalNotFound
	}
	return portal, nil
}

// GetUserPortals 获取用户的所有门户。
func (pe *PortalEngine) GetUserPortals(userID string) []*Portal {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	result := make([]*Portal, 0)
	for _, portal := range pe.portals {
		if portal.UserID == userID {
			result = append(result, portal)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// UpdatePortal 更新门户。
func (pe *PortalEngine) UpdatePortal(id string, updates map[string]interface{}) (*Portal, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[id]
	if !exists {
		return nil, ErrPortalNotFound
	}

	if name, ok := updates["name"].(string); ok {
		portal.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		portal.Description = desc
	}
	if theme, ok := updates["theme"].(Theme); ok {
		portal.Theme = theme
	}
	if layout, ok := updates["layout"].(Layout); ok {
		portal.Layout = layout
	}

	portal.UpdatedAt = time.Now()
	return portal, nil
}

// DeletePortal 删除门户。
func (pe *PortalEngine) DeletePortal(id string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, exists := pe.portals[id]; !exists {
		return ErrPortalNotFound
	}

	delete(pe.portals, id)
	return nil
}

// ClonePortal 克隆门户。
func (pe *PortalEngine) ClonePortal(id, newName string) (*Portal, error) {
	pe.mu.RLock()
	original, exists := pe.portals[id]
	if !exists {
		pe.mu.RUnlock()
		return nil, ErrPortalNotFound
	}

	// 深拷贝
	clone := &Portal{
		ID:          generateID(),
		UserID:      original.UserID,
		Name:        newName,
		Description: original.Description,
		Theme:       original.Theme,
		Layout:      original.Layout,
		Widgets:     make([]*Widget, 0, len(original.Widgets)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for _, w := range original.Widgets {
		widgetCopy := *w
		widgetCopy.ID = generateID()
		widgetCopy.CreatedAt = clone.CreatedAt
		widgetCopy.UpdatedAt = clone.UpdatedAt
		clone.Widgets = append(clone.Widgets, &widgetCopy)
	}
	pe.mu.RUnlock()

	pe.mu.Lock()
	pe.portals[clone.ID] = clone
	pe.mu.Unlock()

	return clone, nil
}

// SetDefaultPortal 设置默认门户。
func (pe *PortalEngine) SetDefaultPortal(portalID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	// 取消该用户的其他默认门户
	for _, p := range pe.portals {
		if p.UserID == portal.UserID && p.IsDefault {
			p.IsDefault = false
		}
	}

	portal.IsDefault = true
	portal.UpdatedAt = time.Now()
	return nil
}

// ========== 小组件管理 ==========

// AddWidget 添加小组件。
func (pe *PortalEngine) AddWidget(portalID string, widget *Widget) (*Widget, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return nil, ErrPortalNotFound
	}

	// 检查位置冲突
	if widget.Position.W > 0 && widget.Position.H > 0 {
		if pe.hasOverlap(portal, widget) {
			return nil, ErrWidgetOverlap
		}
	}

	now := time.Now()
	widget.ID = generateID()
	widget.CreatedAt = now
	widget.UpdatedAt = now
	widget.Enabled = true

	portal.Widgets = append(portal.Widgets, widget)
	portal.UpdatedAt = now

	return widget, nil
}

// GetWidget 获取小组件。
func (pe *PortalEngine) GetWidget(portalID, widgetID string) (*Widget, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return nil, ErrPortalNotFound
	}

	for _, widget := range portal.Widgets {
		if widget.ID == widgetID {
			return widget, nil
		}
	}

	return nil, ErrWidgetNotFound
}

// UpdateWidget 更新小组件。
func (pe *PortalEngine) UpdateWidget(portalID string, widget *Widget) (*Widget, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return nil, ErrPortalNotFound
	}

	for i, w := range portal.Widgets {
		if w.ID == widget.ID {
			widget.UpdatedAt = time.Now()
			portal.Widgets[i] = widget
			portal.UpdatedAt = time.Now()
			return widget, nil
		}
	}

	return nil, ErrWidgetNotFound
}

// RemoveWidget 移除小组件。
func (pe *PortalEngine) RemoveWidget(portalID, widgetID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	for i, w := range portal.Widgets {
		if w.ID == widgetID {
			portal.Widgets = append(portal.Widgets[:i], portal.Widgets[i+1:]...)
			portal.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWidgetNotFound
}

// MoveWidget 移动小组件位置。
func (pe *PortalEngine) MoveWidget(portalID, widgetID string, pos WidgetPosition) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	for _, w := range portal.Widgets {
		if w.ID == widgetID {
			// 临时移除当前组件检查重叠
			oldPos := w.Position
			w.Position = pos

			if pe.hasOverlapExcluding(portal, widgetID, pos) {
				w.Position = oldPos
				return ErrWidgetOverlap
			}

			w.UpdatedAt = time.Now()
			portal.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWidgetNotFound
}

// hasOverlap 检查位置是否重叠。
func (pe *PortalEngine) hasOverlap(portal *Portal, widget *Widget) bool {
	return pe.hasOverlapExcluding(portal, widget.ID, widget.Position)
}

// hasOverlapExcluding 检查位置是否重叠（排除指定组件）。
func (pe *PortalEngine) hasOverlapExcluding(portal *Portal, excludeID string, pos WidgetPosition) bool {
	for _, w := range portal.Widgets {
		if w.ID == excludeID || !w.Enabled {
			continue
		}

		if rectanglesOverlap(pos, w.Position) {
			return true
		}
	}
	return false
}

// rectanglesOverlap 检查两个矩形是否重叠。
func rectanglesOverlap(a, b WidgetPosition) bool {
	return a.X < b.X+b.W &&
		a.X+a.W > b.X &&
		a.Y < b.Y+b.H &&
		a.Y+a.H > b.Y
}

// ListWidgets 列出小组件。
func (pe *PortalEngine) ListWidgets(portalID string) ([]*Widget, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return nil, ErrPortalNotFound
	}

	return portal.Widgets, nil
}

// ToggleWidget 切换小组件启用状态。
func (pe *PortalEngine) ToggleWidget(portalID, widgetID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	for _, w := range portal.Widgets {
		if w.ID == widgetID {
			w.Enabled = !w.Enabled
			w.UpdatedAt = time.Now()
			portal.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWidgetNotFound
}

// ========== 布局管理 ==========

// UpdateLayout 更新布局。
func (pe *PortalEngine) UpdateLayout(portalID string, layout Layout) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	portal.Layout = layout
	portal.UpdatedAt = time.Now()
	return nil
}

// AutoLayout 自动布局。
func (pe *PortalEngine) AutoLayout(portalID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	columns := portal.Layout.Columns
	if columns <= 0 {
		columns = 4
	}

	x, y := 0, 0
	maxH := 0

	for _, widget := range portal.Widgets {
		if !widget.Enabled {
			continue
		}

		w := widget.Position.W
		h := widget.Position.H
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}

		// 换行
		if x+w > columns {
			x = 0
			y += maxH
			maxH = 0
		}

		widget.Position.X = x
		widget.Position.Y = y
		widget.UpdatedAt = time.Now()

		x += w
		if h > maxH {
			maxH = h
		}
	}

	portal.UpdatedAt = time.Now()
	return nil
}

// ========== 主题定制 ==========

// SetTheme 设置主题。
func (pe *PortalEngine) SetTheme(portalID string, theme Theme) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return ErrPortalNotFound
	}

	portal.Theme = theme
	portal.UpdatedAt = time.Now()
	return nil
}

// ========== 用户偏好 ==========

// GetPreferences 获取用户偏好。
func (pe *PortalEngine) GetPreferences(userID string) (*UserPreferences, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	prefs, exists := pe.preferences[userID]
	if !exists {
		// 返回默认偏好
		return &UserPreferences{
			UserID:         userID,
			Theme:          ThemeAuto,
			Language:       "zh-CN",
			TimeZone:       "Asia/Shanghai",
			DateFormat:     "YYYY-MM-DD",
			NotificationOn: true,
			UpdatedAt:      time.Now(),
		}, nil
	}

	return prefs, nil
}

// UpdatePreferences 更新用户偏好。
func (pe *PortalEngine) UpdatePreferences(userID string, prefs *UserPreferences) (*UserPreferences, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	prefs.UserID = userID
	prefs.UpdatedAt = time.Now()
	pe.preferences[userID] = prefs

	return prefs, nil
}

// ========== 通知管理 ==========

// AddNotification 添加通知。
func (pe *PortalEngine) AddNotification(userID string, notification *Notification) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	notification.ID = generateID()
	notification.CreatedAt = time.Now()
	notification.Read = false

	pe.notifications[userID] = append(pe.notifications[userID], notification)
	return nil
}

// ListNotifications 列出通知。
func (pe *PortalEngine) ListNotifications(userID string, unreadOnly bool) []*Notification {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	result := make([]*Notification, 0)
	for _, n := range pe.notifications[userID] {
		if unreadOnly && n.Read {
			continue
		}
		result = append(result, n)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// MarkNotificationRead 标记通知已读。
func (pe *PortalEngine) MarkNotificationRead(userID, notificationID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, n := range pe.notifications[userID] {
		if n.ID == notificationID {
			n.Read = true
			return nil
		}
	}

	return ErrNotificationNotFound
}

// MarkAllNotificationsRead 标记所有通知已读。
func (pe *PortalEngine) MarkAllNotificationsRead(userID string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, n := range pe.notifications[userID] {
		n.Read = true
	}
}

// GetUnreadNotificationCount 获取未读通知数量。
func (pe *PortalEngine) GetUnreadNotificationCount(userID string) int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	count := 0
	for _, n := range pe.notifications[userID] {
		if !n.Read {
			count++
		}
	}

	return count
}

// ========== 统计 ==========

// GetPortalStats 获取门户统计。
func (pe *PortalEngine) GetPortalStats(portalID string) (*PortalStats, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	portal, exists := pe.portals[portalID]
	if !exists {
		return nil, ErrPortalNotFound
	}

	feedCount := 0
	unreadCount := 0

	for _, items := range pe.feedItems {
		feedCount += len(items)
		for _, item := range items {
			if !item.Read {
				unreadCount++
			}
		}
	}

	return &PortalStats{
		WidgetCount:   len(portal.Widgets),
		FeedItemCount: feedCount,
		UnreadCount:   unreadCount,
		LastActive:    portal.UpdatedAt,
	}, nil
}

// ========== 辅助函数 ==========

func generateID() string {
	return uuid.New().String()
}
