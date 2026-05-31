// Package appmarket 应用市场模块
package appmarket

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RegisterRoutes 注册HTTP路由
func RegisterRoutes(r *gin.RouterGroup, mgr *Manager) {
	h := &handler{mgr: mgr}

	// 应用发布（开发者）
	r.POST("/apps", h.PublishApp)
	r.PUT("/apps/:id", h.UpdateApp)

	// 应用审核（管理员）
	r.GET("/apps/pending", h.ListPendingApps)
	r.POST("/apps/:id/review", h.ReviewApp)
	r.GET("/apps/:id/reviews", h.GetReviewHistory)

	// 应用浏览
	r.GET("/apps", h.SearchApps)
	r.GET("/apps/:id", h.GetApp)
	r.GET("/apps/category/:category", h.ListAppsByCategory)
	r.GET("/categories", h.ListCategories)

	// 应用安装/卸载/更新（用户）
	r.POST("/install", h.InstallApp)
	r.DELETE("/install/:id", h.UninstallApp)
	r.PUT("/install/:id", h.UpdateInstalledApp)
	r.GET("/installed", h.ListInstalledApps)
	r.GET("/installed/:id", h.GetInstalledApp)

	// 评分评论
	r.POST("/apps/:id/rate", h.RateApp)
	r.GET("/apps/:id/ratings", h.GetAppRatings)
}

type handler struct {
	mgr *Manager
}

// ========== 应用发布 ==========

// PublishApp 发布新应用
func (h *handler) PublishApp(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 从上下文获取开发者ID（实际项目中从JWT解析）
	developerID := c.GetString("user_id")
	if developerID == "" {
		developerID = "anonymous"
	}

	app, err := h.mgr.PublishApp(&req, developerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "应用已提交审核",
		Data:    app,
	})
}

// UpdateApp 更新应用
func (h *handler) UpdateApp(c *gin.Context) {
	id := c.Param("id")

	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	developerID := c.GetString("user_id")
	if developerID == "" {
		developerID = "anonymous"
	}

	app, err := h.mgr.UpdateApp(id, &req, developerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "应用已更新并重新提交审核",
		Data:    app,
	})
}

// ========== 应用审核 ==========

// ListPendingApps 列出待审核应用
func (h *handler) ListPendingApps(c *gin.Context) {
	apps := h.mgr.ListApps(StatusPendingReview)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    apps,
	})
}

// ReviewApp 审核应用
func (h *handler) ReviewApp(c *gin.Context) {
	appID := c.Param("id")

	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	reviewer := c.GetString("user_id")
	if reviewer == "" {
		reviewer = "admin"
	}

	record, err := h.mgr.ReviewApp(appID, &req, reviewer)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "审核完成",
		Data:    record,
	})
}

// GetReviewHistory 获取审核历史
func (h *handler) GetReviewHistory(c *gin.Context) {
	appID := c.Param("id")

	reviews := h.mgr.GetReviewHistory(appID)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    reviews,
	})
}

// ========== 应用浏览 ==========

// SearchApps 搜索应用
func (h *handler) SearchApps(c *gin.Context) {
	var req SearchRequest
	// 支持GET参数和JSON Body
	if c.Request.Method == "GET" {
		req.Query = c.Query("q")
		req.Category = AppCategory(c.Query("category"))
		req.Sort = SortOption(c.Query("sort"))
		// 解析 tags
		if tags := c.Query("tags"); tags != "" {
			req.Tags = splitTags(tags)
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    -1,
				Message: "请求参数错误: " + err.Error(),
			})
			return
		}
	}

	// 分页参数
	if p := c.Query("page"); p != "" {
		parseIntDefault(p, &req.Page, 1)
	}
	if ps := c.Query("page_size"); ps != "" {
		parseIntDefault(ps, &req.PageSize, 20)
	}

	result := h.mgr.SearchApps(&req)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    result,
	})
}

// GetApp 获取单个应用
func (h *handler) GetApp(c *gin.Context) {
	id := c.Param("id")

	app, err := h.mgr.GetApp(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    app,
	})
}

// ListAppsByCategory 按分类列出应用
func (h *handler) ListAppsByCategory(c *gin.Context) {
	category := AppCategory(c.Param("category"))

	apps := h.mgr.ListAppsByCategory(category)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    apps,
	})
}

// ListCategories 列出所有分类
func (h *handler) ListCategories(c *gin.Context) {
	categories := h.mgr.ListCategories()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    categories,
	})
}

// ========== 应用安装/卸载/更新 ==========

// InstallApp 安装应用
func (h *handler) InstallApp(c *gin.Context) {
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "default"
	}

	installed, err := h.mgr.InstallApp(&req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "应用安装成功",
		Data:    installed,
	})
}

// UninstallApp 卸载应用
func (h *handler) UninstallApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.mgr.UninstallApp(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "应用卸载成功",
	})
}

// UpdateInstalledApp 更新已安装应用
func (h *handler) UpdateInstalledApp(c *gin.Context) {
	id := c.Param("id")

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}
	req.AppID = id

	installed, err := h.mgr.UpdateInstalledApp(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "应用更新成功",
		Data:    installed,
	})
}

// ListInstalledApps 列出已安装应用
func (h *handler) ListInstalledApps(c *gin.Context) {
	apps := h.mgr.GetInstalledApps()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    apps,
	})
}

// GetInstalledApp 获取单个已安装应用
func (h *handler) GetInstalledApp(c *gin.Context) {
	id := c.Param("id")

	app, err := h.mgr.GetInstalledApp(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    app,
	})
}

// ========== 评分评论 ==========

// RateApp 评分应用
func (h *handler) RateApp(c *gin.Context) {
	appID := c.Param("id")

	var req RatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	rating, err := h.mgr.RateApp(appID, &req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "评分成功",
		Data:    rating,
	})
}

// GetAppRatings 获取应用评分列表
func (h *handler) GetAppRatings(c *gin.Context) {
	appID := c.Param("id")

	ratings := h.mgr.GetAppRatings(appID)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    ratings,
	})
}

// ========== 工具函数 ==========

func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	result := make([]string, 0)
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func parseIntDefault(s string, v *int, defaultVal int) {
	if s == "" {
		*v = defaultVal
		return
	}
	// 简单的数字解析
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			*v = defaultVal
			return
		}
	}
	if n == 0 {
		*v = defaultVal
	} else {
		*v = n
	}
}
