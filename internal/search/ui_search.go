// Package search 提供UI搜索功能
// 用于前端全局搜索，支持用户、共享、应用、设置等范围
package search

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UISearchRequest UI搜索请求.
type UISearchRequest struct {
	Query string   `json:"query"`         // 搜索关键词
	Scope []string `json:"scope"`         // 搜索范围: users, shares, apps, settings
	Limit int      `json:"limit"`         // 每类结果数量限制
}

// UISearchResult UI搜索结果（按类别分组）.
type UISearchResult struct {
	Category string       `json:"category"` // 类别名称
	Items    []SearchItem `json:"items"`    // 搜索结果项
}

// SearchItem 搜索结果项.
type SearchItem struct {
	ID          string `json:"id"`                    // 唯一标识
	Name        string `json:"name"`                  // 显示名称
	Path        string `json:"path"`                  // 跳转路径
	Description string `json:"description,omitempty"` // 描述信息
	Icon        string `json:"icon,omitempty"`        // 图标
	Type        string `json:"type,omitempty"`        // 子类型
}

// UISearchHandler UI搜索处理器.
type UISearchHandler struct {
	settingsRegistry *SettingsRegistry
	appRegistry      *AppRegistry
	apiRegistry      *APIRegistry
	userSearcher     UserSearcher
	shareSearcher    ShareSearcher
}

// UserSearcher 用户搜索接口.
type UserSearcher interface {
	SearchUsers(query string, limit int) []UserSearchResult
}

// UserSearchResult 用户搜索结果.
type UserSearchResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

// ShareSearcher 共享搜索接口.
type ShareSearcher interface {
	SearchShares(query string, limit int) []ShareSearchResult
}

// ShareSearchResult 共享搜索结果.
type ShareSearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // smb, nfs, webdav
	Description string `json:"description,omitempty"`
}

// NewUISearchHandler 创建UI搜索处理器.
func NewUISearchHandler(
	settingsRegistry *SettingsRegistry,
	appRegistry *AppRegistry,
	apiRegistry *APIRegistry,
) *UISearchHandler {
	return &UISearchHandler{
		settingsRegistry: settingsRegistry,
		appRegistry:      appRegistry,
		apiRegistry:      apiRegistry,
	}
}

// SetUserSearcher 设置用户搜索器.
func (h *UISearchHandler) SetUserSearcher(searcher UserSearcher) {
	h.userSearcher = searcher
}

// SetShareSearcher 设置共享搜索器.
func (h *UISearchHandler) SetShareSearcher(searcher ShareSearcher) {
	h.shareSearcher = searcher
}

// RegisterRoutes 注册路由.
func (h *UISearchHandler) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/search")
	{
		search.POST("/ui", h.searchUI)
	}
}

// searchUI UI搜索
// @Summary UI全局搜索
// @Description 执行UI全局搜索，支持用户、共享、应用、设置等范围，返回分组结果
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body UISearchRequest true "搜索请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/search/ui [post].
func (h *UISearchHandler) searchUI(c *gin.Context) {
	var req UISearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 默认搜索范围
	if len(req.Scope) == 0 {
		req.Scope = []string{"users", "shares", "apps", "settings"}
	}

	// 默认限制
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// 执行搜索
	results := h.doSearch(req)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  h.countTotal(results),
			"result": results,
		},
	})
}

// doSearch 执行搜索.
func (h *UISearchHandler) doSearch(req UISearchRequest) []UISearchResult {
	results := make([]UISearchResult, 0)

	for _, scope := range req.Scope {
		switch scope {
		case "users":
			items := h.searchUsers(req.Query, req.Limit)
			if len(items) > 0 {
				results = append(results, UISearchResult{
					Category: "users",
					Items:    items,
				})
			}
		case "shares":
			items := h.searchShares(req.Query, req.Limit)
			if len(items) > 0 {
				results = append(results, UISearchResult{
					Category: "shares",
					Items:    items,
				})
			}
		case "apps":
			items := h.searchApps(req.Query, req.Limit)
			if len(items) > 0 {
				results = append(results, UISearchResult{
					Category: "apps",
					Items:    items,
				})
			}
		case "settings":
			items := h.searchSettings(req.Query, req.Limit)
			if len(items) > 0 {
				results = append(results, UISearchResult{
					Category: "settings",
					Items:    items,
				})
			}
		}
	}

	return results
}

// searchUsers 搜索用户.
func (h *UISearchHandler) searchUsers(query string, limit int) []SearchItem {
	if h.userSearcher == nil {
		return nil
	}

	userResults := h.userSearcher.SearchUsers(query, limit)
	items := make([]SearchItem, 0, len(userResults))

	for _, u := range userResults {
		items = append(items, SearchItem{
			ID:          u.ID,
			Name:        u.Username,
			Path:        "/system/users/" + u.ID,
			Description: h.buildUserDescription(u),
			Icon:        "user",
			Type:        u.Role,
		})
	}

	return items
}

// buildUserDescription 构建用户描述.
func (h *UISearchHandler) buildUserDescription(u UserSearchResult) string {
	desc := u.Role
	if u.Email != "" {
		desc += " | " + u.Email
	}
	if u.Disabled {
		desc += " | 已禁用"
	}
	return desc
}

// searchShares 搜索共享.
func (h *UISearchHandler) searchShares(query string, limit int) []SearchItem {
	if h.shareSearcher == nil {
		return nil
	}

	shareResults := h.shareSearcher.SearchShares(query, limit)
	items := make([]SearchItem, 0, len(shareResults))

	for _, s := range shareResults {
		items = append(items, SearchItem{
			ID:          s.ID,
			Name:        s.Name,
			Path:        "/storage/shares/" + s.ID,
			Description: s.Description + " | " + s.Type,
			Icon:        "share-alt",
			Type:        s.Type,
		})
	}

	return items
}

// searchApps 搜索应用.
func (h *UISearchHandler) searchApps(query string, limit int) []SearchItem {
	if h.appRegistry == nil {
		return nil
	}

	appResults := h.appRegistry.SearchApps(query, limit)
	items := make([]SearchItem, 0, len(appResults))

	for _, r := range appResults {
		var item SearchItem
		if r.Type == "app" {
			if app, ok := r.Item.(AppItem); ok {
				item = SearchItem{
					ID:          app.ID,
					Name:        app.DisplayName,
					Path:        app.Path,
					Description: app.Description + " | " + app.Status,
					Icon:        app.Icon,
					Type:        app.Category,
				}
			}
		} else if r.Type == "container" {
			if container, ok := r.Item.(ContainerItem); ok {
				item = SearchItem{
					ID:          container.ID,
					Name:        container.Name,
					Path:        "/containers/" + container.ID,
					Description: container.Image + " | " + container.Status,
					Icon:        "docker",
					Type:        "container",
				}
			}
		}
		if item.ID != "" {
			items = append(items, item)
		}
	}

	return items
}

// searchSettings 搜索设置.
func (h *UISearchHandler) searchSettings(query string, limit int) []SearchItem {
	if h.settingsRegistry == nil {
		return nil
	}

	settingsResults := h.settingsRegistry.SearchSettings(query, limit)
	items := make([]SearchItem, 0, len(settingsResults))

	for _, r := range settingsResults {
		items = append(items, SearchItem{
			ID:          r.Setting.ID,
			Name:        r.Setting.Name,
			Path:        r.Setting.Path,
			Description: r.Setting.Description,
			Icon:        r.Setting.Icon,
			Type:        r.Setting.Category,
		})
	}

	return items
}

// countTotal 计算总结果数.
func (h *UISearchHandler) countTotal(results []UISearchResult) int {
	total := 0
	for _, r := range results {
		total += len(r.Items)
	}
	return total
}

// --- 默认用户搜索器实现 ---

// DefaultUserSearcher 默认用户搜索器.
type DefaultUserSearcher struct {
	users []UserSearchResult
}

// NewDefaultUserSearcher 创建默认用户搜索器.
func NewDefaultUserSearcher() *DefaultUserSearcher {
	return &DefaultUserSearcher{
		users: make([]UserSearchResult, 0),
	}
}

// SetUsers 设置用户列表.
func (s *DefaultUserSearcher) SetUsers(users []UserSearchResult) {
	s.users = users
}

// SearchUsers 搜索用户.
func (s *DefaultUserSearcher) SearchUsers(query string, limit int) []UserSearchResult {
	if query == "" {
		return nil
	}

	query = strings.ToLower(query)
	results := make([]UserSearchResult, 0)

	for _, u := range s.users {
		// 搜索用户名
		if strings.Contains(strings.ToLower(u.Username), query) {
			results = append(results, u)
			continue
		}
		// 搜索邮箱
		if u.Email != "" && strings.Contains(strings.ToLower(u.Email), query) {
			results = append(results, u)
			continue
		}
		// 搜索ID
		if strings.Contains(strings.ToLower(u.ID), query) {
			results = append(results, u)
			continue
		}
	}

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// --- 默认共享搜索器实现 ---

// DefaultShareSearcher 默认共享搜索器.
type DefaultShareSearcher struct {
	shares []ShareSearchResult
}

// NewDefaultShareSearcher 创建默认共享搜索器.
func NewDefaultShareSearcher() *DefaultShareSearcher {
	return &DefaultShareSearcher{
		shares: make([]ShareSearchResult, 0),
	}
}

// SetShares 设置共享列表.
func (s *DefaultShareSearcher) SetShares(shares []ShareSearchResult) {
	s.shares = shares
}

// SearchShares 搜索共享.
func (s *DefaultShareSearcher) SearchShares(query string, limit int) []ShareSearchResult {
	if query == "" {
		return nil
	}

	query = strings.ToLower(query)
	results := make([]ShareSearchResult, 0)

	for _, share := range s.shares {
		// 搜索名称
		if strings.Contains(strings.ToLower(share.Name), query) {
			results = append(results, share)
			continue
		}
		// 搜索路径
		if strings.Contains(strings.ToLower(share.Path), query) {
			results = append(results, share)
			continue
		}
		// 搜索类型
		if strings.Contains(strings.ToLower(share.Type), query) {
			results = append(results, share)
			continue
		}
		// 搜索描述
		if share.Description != "" && strings.Contains(strings.ToLower(share.Description), query) {
			results = append(results, share)
			continue
		}
	}

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}