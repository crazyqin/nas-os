// Package updatecoord 提供系统更新协调器功能
// 编排 NAS 系统更新的完整流程：检查→预检→下载→备份→安装→验证→切换
package updatecoord

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler 系统更新协调器 API 处理器.
// 注册到 /api/v1/updatecoord/ 路由.
type Handler struct {
	service *Service
}

// NewHandler 创建更新协调器处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由到 /api/v1/updatecoord/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/updatecoord")
	{
		g.GET("/check", h.check)             // 检查可用更新
		g.POST("/precheck", h.precheck)      // 更新前预检
		g.POST("/apply", h.apply)            // 应用更新
		g.GET("/history", h.history)         // 更新历史
		g.POST("/rollback", h.rollback)      // 回滚更新
	}
}

// check 检查可用更新.
func (h *Handler) check(c *gin.Context) {
	// 可选渠道过滤
	var channel *UpdateChannel
	channelStr := c.Query("channel")
	if channelStr != "" {
		ch := UpdateChannel(channelStr)
		channel = &ch
	}

	updates, err := h.service.Check(c.Request.Context(), channel)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, updates)
}

// precheck 更新前预检.
func (h *Handler) precheck(c *gin.Context) {
	var req PreCheckRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.PreCheck(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "更新预检失败")
		return
	}

	api.OK(c, result)
}

// apply 应用更新.
func (h *Handler) apply(c *gin.Context) {
	var req ApplyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Apply(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "更新应用失败")
		return
	}

	api.OK(c, result)
}

// history 查询更新历史.
func (h *Handler) history(c *gin.Context) {
	entries, err := h.service.GetHistory()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, entries)
}

// rollback 回滚更新.
func (h *Handler) rollback(c *gin.Context) {
	var req RollbackRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Rollback(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "更新回滚失败")
		return
	}

	api.OK(c, result)
}
