// Package remotebackup 远程备份引擎模块
package remotebackup

import (
	"net/http"

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

	// 目标管理
	r.GET("/targets", h.ListTargets)
	r.POST("/targets", h.CreateTarget)
	r.PUT("/targets/:id", h.UpdateTarget)
	r.DELETE("/targets/:id", h.DeleteTarget)
	r.POST("/targets/:id/test", h.TestConnection)

	// 任务管理
	r.GET("/jobs", h.ListJobs)
	r.POST("/jobs", h.CreateJob)
	r.POST("/jobs/:id/run", h.RunJob)
	r.POST("/jobs/:id/cancel", h.CancelJob)
	r.GET("/jobs/:id/versions", h.ListVersions)

	// 恢复
	r.POST("/restore", h.Restore)
}

type handler struct {
	mgr *Manager
}

// ListTargets 列出所有备份目标
func (h *handler) ListTargets(c *gin.Context) {
	targets := h.mgr.ListTargets()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    targets,
	})
}

// CreateTarget 创建备份目标
func (h *handler) CreateTarget(c *gin.Context) {
	var req BackupTarget
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	target, err := h.mgr.CreateTarget(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    target,
	})
}

// UpdateTarget 更新备份目标
func (h *handler) UpdateTarget(c *gin.Context) {
	id := c.Param("id")

	var req BackupTarget
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	target, err := h.mgr.UpdateTarget(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    target,
	})
}

// DeleteTarget 删除备份目标
func (h *handler) DeleteTarget(c *gin.Context) {
	id := c.Param("id")

	if err := h.mgr.DeleteTarget(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
	})
}

// TestConnection 测试连接
func (h *handler) TestConnection(c *gin.Context) {
	id := c.Param("id")

	if err := h.mgr.TestConnection(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "连接测试成功",
	})
}

// ListJobs 列出所有备份任务
func (h *handler) ListJobs(c *gin.Context) {
	jobs := h.mgr.ListJobs()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    jobs,
	})
}

// CreateJob 创建备份任务
func (h *handler) CreateJob(c *gin.Context) {
	var req BackupJob
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	job, err := h.mgr.CreateJob(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    job,
	})
}

// RunJob 手动执行备份任务
func (h *handler) RunJob(c *gin.Context) {
	id := c.Param("id")

	version, err := h.mgr.RunJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "备份任务执行完成",
		Data:    version,
	})
}

// CancelJob 取消备份任务
func (h *handler) CancelJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.mgr.CancelJob(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "任务已取消",
	})
}

// ListVersions 列出任务版本
func (h *handler) ListVersions(c *gin.Context) {
	id := c.Param("id")

	versions, err := h.mgr.ListVersions(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    versions,
	})
}

// Restore 恢复数据
func (h *handler) Restore(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	result, err := h.mgr.Restore(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "恢复完成",
		Data:    result,
	})
}
