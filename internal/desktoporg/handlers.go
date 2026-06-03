package desktoporg

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DesktopHandler 桌面HTTP处理器
type DesktopHandler struct {
	manager *DesktopManager
}

// NewDesktopHandler 创建桌面处理器
func NewDesktopHandler(manager *DesktopManager) *DesktopHandler {
	return &DesktopHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *DesktopHandler) RegisterRoutes(r *gin.RouterGroup) {
	desktop := r.Group("/desktop")
	{
		// 图标管理
		icons := desktop.Group("/icons")
		{
			icons.GET("", h.ListIcons)
			icons.POST("", h.CreateIcon)
			icons.GET("/:id", h.GetIcon)
			icons.PUT("/:id", h.UpdateIcon)
			icons.DELETE("/:id", h.DeleteIcon)
			icons.PUT("/:id/move", h.MoveIcon)
		}

		// 分组管理
		groups := desktop.Group("/groups")
		{
			groups.GET("", h.ListGroups)
			groups.POST("", h.CreateGroup)
			groups.GET("/:id", h.GetGroup)
			groups.PUT("/:id", h.UpdateGroup)
			groups.DELETE("/:id", h.DeleteGroup)
			groups.POST("/:id/icons", h.AddIconToGroup)
			groups.DELETE("/:id/icons/:iconId", h.RemoveIconFromGroup)
		}

		// 布局管理
		layout := desktop.Group("/layout")
		{
			layout.GET("", h.GetLayout)
			layout.PUT("", h.SaveLayout)
			layout.POST("/default/:id", h.SetDefaultLayout)
			layout.POST("/screens", h.AddScreen)
			layout.DELETE("/screens/:id", h.RemoveScreen)
		}
	}
}

// ListIcons 获取图标列表
func (h *DesktopHandler) ListIcons(c *gin.Context) {
	screenID := c.Query("screen_id")
	icons := h.manager.ListIcons(screenID)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: ListResponse{
			Items: icons,
			Total: len(icons),
		},
	})
}

// CreateIcon 创建图标
func (h *DesktopHandler) CreateIcon(c *gin.Context) {
	var req CreateIconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	icon, err := h.manager.CreateIcon(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    icon,
		Message: "图标创建成功",
	})
}

// GetIcon 获取图标详情
func (h *DesktopHandler) GetIcon(c *gin.Context) {
	id := c.Param("id")

	icon, err := h.manager.GetIcon(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    icon,
	})
}

// UpdateIcon 更新图标
func (h *DesktopHandler) UpdateIcon(c *gin.Context) {
	id := c.Param("id")

	var req UpdateIconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	icon, err := h.manager.UpdateIcon(id, &req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    icon,
		Message: "图标更新成功",
	})
}

// DeleteIcon 删除图标
func (h *DesktopHandler) DeleteIcon(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteIcon(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "图标删除成功",
	})
}

// MoveIcon 移动图标
func (h *DesktopHandler) MoveIcon(c *gin.Context) {
	id := c.Param("id")

	var req MoveIconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	icon, err := h.manager.MoveIcon(id, &req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    icon,
		Message: "图标移动成功",
	})
}

// ListGroups 获取分组列表
func (h *DesktopHandler) ListGroups(c *gin.Context) {
	screenID := c.Query("screen_id")
	groups := h.manager.ListGroups(screenID)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: ListResponse{
			Items: groups,
			Total: len(groups),
		},
	})
}

// CreateGroup 创建分组
func (h *DesktopHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	group, err := h.manager.CreateGroup(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    group,
		Message: "分组创建成功",
	})
}

// GetGroup 获取分组详情
func (h *DesktopHandler) GetGroup(c *gin.Context) {
	id := c.Param("id")

	group, err := h.manager.GetGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    group,
	})
}

// UpdateGroup 更新分组
func (h *DesktopHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	group, err := h.manager.UpdateGroup(id, &req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    group,
		Message: "分组更新成功",
	})
}

// DeleteGroup 删除分组
func (h *DesktopHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "分组删除成功",
	})
}

// AddIconToGroup 添加图标到分组
func (h *DesktopHandler) AddIconToGroup(c *gin.Context) {
	groupID := c.Param("id")

	var req GroupAddIconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddIconToGroup(groupID, req.IconID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "图标已添加到分组",
	})
}

// RemoveIconFromGroup 从分组移除图标
func (h *DesktopHandler) RemoveIconFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	iconID := c.Param("iconId")

	if err := h.manager.RemoveIconFromGroup(groupID, iconID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "图标已从分组移除",
	})
}

// GetLayout 获取当前布局
func (h *DesktopHandler) GetLayout(c *gin.Context) {
	layout, err := h.manager.GetLayout()
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    layout,
	})
}

// SaveLayout 保存布局
func (h *DesktopHandler) SaveLayout(c *gin.Context) {
	var layout DesktopLayout
	if err := c.ShouldBindJSON(&layout); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.SaveLayout(&layout); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "布局保存成功",
	})
}

// SetDefaultLayout 设置默认布局
func (h *DesktopHandler) SetDefaultLayout(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.SetDefaultLayout(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "默认布局设置成功",
	})
}

// AddScreen 添加屏幕
func (h *DesktopHandler) AddScreen(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		Width      int    `json:"width" binding:"required"`
		Height     int    `json:"height" binding:"required"`
		Primary    bool   `json:"primary"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	screen, err := h.manager.AddScreen(req.Name, Size{Width: req.Width, Height: req.Height}, req.Primary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    screen,
		Message: "屏幕添加成功",
	})
}

// RemoveScreen 移除屏幕
func (h *DesktopHandler) RemoveScreen(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RemoveScreen(id); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "屏幕移除成功",
	})
}

// ParsePageParams 解析分页参数
func ParsePageParams(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}
