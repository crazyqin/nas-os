package saasbackup

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers SaaS 备份 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 SaaS 备份处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册 SaaS 备份路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	saas := apiGroup.Group("/saasbackup")
	{
		// 租户管理
		saas.POST("/tenants", h.connectTenant)
		saas.GET("/tenants", h.listTenants)
		saas.DELETE("/tenants/:id", h.disconnectTenant)

		// 备份任务管理
		saas.POST("/jobs", h.createJob)
		saas.GET("/jobs", h.listJobs)
		saas.POST("/jobs/:id/backup", h.executeBackup)

		// 数据恢复
		saas.POST("/restore", h.restoreData)

		// 统计和查询
		saas.GET("/stats", h.getStats)
		saas.GET("/items", h.listItems)
	}
}

// ========== 租户处理 ==========

// connectTenant 连接 SaaS 租户
// @Summary 连接 SaaS 租户
// @Description 连接新的 SaaS 租户（Microsoft 365 或 Google Workspace）
// @Tags saasbackup
// @Accept json
// @Produce json
// @Param request body ConnectTenantRequest true "租户信息"
// @Success 201 {object} api.Response{data=SaaSTenant}
// @Failure 400 {object} api.Response
// @Router /saasbackup/tenants [post].
func (h *Handlers) connectTenant(c *gin.Context) {
	var req ConnectTenantRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	tenant, err := h.manager.ConnectTenant(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, tenant)
}

// listTenants 列出所有租户
// @Summary 列出 SaaS 租户
// @Description 获取所有已连接的 SaaS 租户
// @Tags saasbackup
// @Produce json
// @Success 200 {object} api.Response{data=[]SaaSTenant}
// @Router /saasbackup/tenants [get].
func (h *Handlers) listTenants(c *gin.Context) {
	tenants := h.manager.ListTenants()
	api.OK(c, tenants)
}

// disconnectTenant 断开 SaaS 租户连接
// @Summary 断开 SaaS 租户
// @Description 断开指定 SaaS 租户的连接
// @Tags saasbackup
// @Produce json
// @Param id path string true "租户 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /saasbackup/tenants/{id} [delete].
func (h *Handlers) disconnectTenant(c *gin.Context) {
	tenantID := c.Param("id")
	if tenantID == "" {
		api.BadRequest(c, "租户 ID 不能为空")
		return
	}

	if err := h.manager.DisconnectTenant(tenantID); err != nil {
		if err == ErrTenantNotFound {
			api.NotFound(c, "租户不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "租户已断开", nil)
}

// ========== 备份任务处理 ==========

// createJob 创建备份任务
// @Summary 创建备份任务
// @Description 创建新的 SaaS 数据备份任务
// @Tags saasbackup
// @Accept json
// @Produce json
// @Param request body CreateJobRequest true "任务信息"
// @Success 201 {object} api.Response{data=BackupJob}
// @Failure 400 {object} api.Response
// @Router /saasbackup/jobs [post].
func (h *Handlers) createJob(c *gin.Context) {
	var req CreateJobRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	job, err := h.manager.CreateJob(req)
	if err != nil {
		if err == ErrTenantNotFound {
			api.NotFound(c, "租户不存在")
			return
		}
		if err == ErrTenantNotConnected {
			api.BadRequest(c, "租户未连接")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, job)
}

// listJobs 列出所有备份任务
// @Summary 列出备份任务
// @Description 获取所有 SaaS 备份任务
// @Tags saasbackup
// @Produce json
// @Success 200 {object} api.Response{data=[]BackupJob}
// @Router /saasbackup/jobs [get].
func (h *Handlers) listJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	api.OK(c, jobs)
}

// executeBackup 立即执行备份任务
// @Summary 执行备份
// @Description 立即执行指定的备份任务
// @Tags saasbackup
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response{data=BackupJob}
// @Failure 404 {object} api.Response
// @Failure 409 {object} api.Response
// @Router /saasbackup/jobs/{id}/backup [post].
func (h *Handlers) executeBackup(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		api.BadRequest(c, "任务 ID 不能为空")
		return
	}

	job, err := h.manager.ExecuteBackup(jobID)
	if err != nil {
		if err == ErrJobNotFound {
			api.NotFound(c, "备份任务不存在")
			return
		}
		if err == ErrJobAlreadyRunning {
			api.BadRequest(c, "任务已在运行中")
			return
		}
		if err == ErrTenantNotConnected {
			api.BadRequest(c, "租户未连接")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, job)
}

// ========== 数据恢复处理 ==========

// restoreData 恢复备份数据
// @Summary 恢复数据
// @Description 从备份中恢复数据到指定位置
// @Tags saasbackup
// @Accept json
// @Produce json
// @Param request body RestoreRequest true "恢复请求"
// @Success 200 {object} api.Response{data=[]BackupItem}
// @Failure 400 {object} api.Response
// @Router /saasbackup/restore [post].
func (h *Handlers) restoreData(c *gin.Context) {
	var req RestoreRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	items, err := h.manager.RestoreData(req)
	if err != nil {
		if err == ErrJobNotFound {
			api.NotFound(c, "备份任务不存在")
			return
		}
		if err == ErrInvalidRestoreMode {
			api.BadRequest(c, "无效的恢复模式，支持 original 和 cross_user")
			return
		}
		if err == ErrCrossUserRequiresTarget {
			api.BadRequest(c, "跨用户恢复需要指定目标用户 ID")
			return
		}
		if err == ErrItemNotFound {
			api.NotFound(c, "未找到可恢复的备份项")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, items)
}

// ========== 统计和查询处理 ==========

// getStats 获取备份统计信息
// @Summary 获取统计
// @Description 获取 SaaS 备份统计概览
// @Tags saasbackup
// @Produce json
// @Success 200 {object} api.Response{data=BackupStats}
// @Router /saasbackup/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// listItems 列出备份项
// @Summary 列出备份项
// @Description 获取指定任务的所有备份项
// @Tags saasbackup
// @Produce json
// @Param jobID query string true "任务 ID"
// @Success 200 {object} api.Response{data=[]BackupItem}
// @Failure 400 {object} api.Response
// @Router /saasbackup/items [get].
func (h *Handlers) listItems(c *gin.Context) {
	jobID := c.Query("jobID")
	if jobID == "" {
		api.BadRequest(c, "jobID 参数不能为空")
		return
	}

	items, err := h.manager.ListItems(jobID)
	if err != nil {
		if err == ErrJobNotFound {
			api.NotFound(c, "备份任务不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, items)
}
