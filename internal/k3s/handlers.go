// Package k3s 提供 REST API 处理器
package k3s

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers K3s 模块 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/k3s 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	k := r.Group("/k3s")
	{
		// 集群管理
		k.GET("/cluster", h.getClusterInfo)
		k.GET("/cluster/health", h.getClusterHealth)

		// 节点管理
		k.GET("/nodes", h.listNodes)
		k.GET("/nodes/:name", h.getNode)
		k.PUT("/nodes/:name/status", h.updateNodeStatus)
		k.DELETE("/nodes/:name", h.removeNode)

		// Helm Chart 管理
		k.POST("/helm/releases", h.deployChart)
		k.GET("/helm/releases", h.listHelmReleases)
		k.GET("/helm/releases/:namespace/:name", h.getHelmRelease)
		k.PUT("/helm/releases/:namespace/:name", h.upgradeHelmRelease)
		k.POST("/helm/releases/:namespace/:name/rollback", h.rollbackHelmRelease)
		k.DELETE("/helm/releases/:namespace/:name", h.uninstallHelmRelease)

		// 工作负载管理
		k.GET("/workloads/deployments", h.listDeployments)
		k.GET("/workloads/deployments/:namespace/:name", h.getDeployment)
		k.GET("/workloads/services", h.listServices)
		k.GET("/workloads/services/:namespace/:name", h.getService)
		k.GET("/workloads/pods", h.listPods)
		k.GET("/workloads/pods/:namespace/:name", h.getPod)
		k.POST("/workloads/pods/logs", h.getPodLogs)

		// 服务网格
		k.GET("/mesh", h.getServiceMeshConfig)
		k.POST("/mesh/enable", h.enableServiceMesh)
		k.POST("/mesh/disable", h.disableServiceMesh)
		k.PUT("/mesh", h.updateServiceMeshConfig)

		// HPA 自动扩缩容
		k.POST("/hpa", h.createHPA)
		k.GET("/hpa", h.listHPAs)
		k.GET("/hpa/:namespace/:name", h.getHPA)
		k.PUT("/hpa/:namespace/:name", h.updateHPA)
		k.DELETE("/hpa/:namespace/:name", h.deleteHPA)

		// 应用商店集成
		k.GET("/appstore", h.listAppStoreApps)
		k.POST("/appstore/deploy", h.deployFromAppStore)

		// 资源配额管理
		k.POST("/quotas", h.createQuota)
		k.GET("/quotas", h.listQuotas)
		k.GET("/quotas/:namespace/:name", h.getQuota)
		k.PUT("/quotas/:namespace/:name", h.updateQuota)
		k.DELETE("/quotas/:namespace/:name", h.deleteQuota)

		// 集群事件
		k.GET("/events", h.listEvents)
		k.GET("/events/severity/:severity", h.getEventsBySeverity)
		k.DELETE("/events", h.clearEvents)
	}
}

// ========== 集群管理 Handlers ==========

// getClusterInfo 获取集群信息
func (h *Handlers) getClusterInfo(c *gin.Context) {
	info := h.manager.GetClusterInfo()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: info})
}

// getClusterHealth 获取集群健康状态
func (h *Handlers) getClusterHealth(c *gin.Context) {
	health := h.manager.GetClusterHealth()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: health})
}

// ========== 节点管理 Handlers ==========

// listNodes 列出节点
func (h *Handlers) listNodes(c *gin.Context) {
	nodes := h.manager.ListNodes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(nodes),
			"nodes": nodes,
		},
	})
}

// getNode 获取节点详情
func (h *Handlers) getNode(c *gin.Context) {
	name := c.Param("name")
	node, err := h.manager.GetNode(name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: node})
}

// updateNodeStatus 更新节点状态
func (h *Handlers) updateNodeStatus(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Status NodeStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdateNodeStatus(name, req.Status); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "状态已更新"})
}

// removeNode 移除节点
func (h *Handlers) removeNode(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.RemoveNode(name); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "节点已移除"})
}

// ========== Helm Chart Handlers ==========

// deployChart 部署 Chart
func (h *Handlers) deployChart(c *gin.Context) {
	var req DeployChartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	release, err := h.manager.DeployChart(req)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrHelmReleaseExists {
			code = http.StatusConflict
		}
		c.JSON(code, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "部署成功", Data: release})
}

// listHelmReleases 列出 Helm Release
func (h *Handlers) listHelmReleases(c *gin.Context) {
	namespace := c.Query("namespace")
	releases := h.manager.ListHelmReleases(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(releases),
			"releases": releases,
		},
	})
}

// getHelmRelease 获取 Helm Release
func (h *Handlers) getHelmRelease(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	release, err := h.manager.GetHelmRelease(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: release})
}

// upgradeHelmRelease 升级 Helm Release
func (h *Handlers) upgradeHelmRelease(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req UpgradeChartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	release, err := h.manager.UpgradeHelmRelease(namespace, name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "升级成功", Data: release})
}

// rollbackHelmRelease 回滚 Helm Release
func (h *Handlers) rollbackHelmRelease(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req RollbackChartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	if err := h.manager.RollbackHelmRelease(namespace, name, req); err != nil {
		code := http.StatusNotFound
		if err == ErrRollbackFailed {
			code = http.StatusBadRequest
		}
		c.JSON(code, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "回滚成功"})
}

// uninstallHelmRelease 卸载 Helm Release
func (h *Handlers) uninstallHelmRelease(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if err := h.manager.UninstallHelmRelease(namespace, name); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "卸载成功"})
}

// ========== 工作负载管理 Handlers ==========

// listDeployments 列出 Deployment
func (h *Handlers) listDeployments(c *gin.Context) {
	namespace := c.Query("namespace")
	deps := h.manager.ListDeployments(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":       len(deps),
			"deployments": deps,
		},
	})
}

// getDeployment 获取 Deployment
func (h *Handlers) getDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	dep, err := h.manager.GetDeployment(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: dep})
}

// listServices 列出 Service
func (h *Handlers) listServices(c *gin.Context) {
	namespace := c.Query("namespace")
	svcs := h.manager.ListServices(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(svcs),
			"services": svcs,
		},
	})
}

// getService 获取 Service
func (h *Handlers) getService(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	svc, err := h.manager.GetService(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: svc})
}

// listPods 列出 Pod
func (h *Handlers) listPods(c *gin.Context) {
	namespace := c.Query("namespace")
	pods := h.manager.ListPods(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(pods),
			"pods":  pods,
		},
	})
}

// getPod 获取 Pod
func (h *Handlers) getPod(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	pod, err := h.manager.GetPod(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: pod})
}

// getPodLogs 获取 Pod 日志
func (h *Handlers) getPodLogs(c *gin.Context) {
	var req PodLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	logs, err := h.manager.GetPodLogs(req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: logs})
}

// ========== 服务网格 Handlers ==========

// getServiceMeshConfig 获取服务网格配置
func (h *Handlers) getServiceMeshConfig(c *gin.Context) {
	cfg := h.manager.GetServiceMeshConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// enableServiceMesh 启用服务网格
func (h *Handlers) enableServiceMesh(c *gin.Context) {
	var req struct {
		Type      ServiceMeshType `json:"type" binding:"required"`
		Namespace string          `json:"namespace" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	if req.Type != ServiceMeshIstio && req.Type != ServiceMeshLinkerd {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "不支持的服务网格类型，可选: istio, linkerd"})
		return
	}

	if err := h.manager.EnableServiceMesh(req.Type, req.Namespace); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "服务网格已启用"})
}

// disableServiceMesh 禁用服务网格
func (h *Handlers) disableServiceMesh(c *gin.Context) {
	if err := h.manager.DisableServiceMesh(); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "服务网格已禁用"})
}

// updateServiceMeshConfig 更新服务网格配置
func (h *Handlers) updateServiceMeshConfig(c *gin.Context) {
	var req ServiceMeshConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdateServiceMeshConfig(req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "配置已更新"})
}

// ========== HPA Handlers ==========

// createHPA 创建 HPA
func (h *Handlers) createHPA(c *gin.Context) {
	var req CreateHPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	hpa := h.manager.CreateHPA(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "创建成功", Data: hpa})
}

// listHPAs 列出 HPA
func (h *Handlers) listHPAs(c *gin.Context) {
	namespace := c.Query("namespace")
	hpas := h.manager.ListHPAs(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(hpas),
			"hpas":  hpas,
		},
	})
}

// getHPA 获取 HPA
func (h *Handlers) getHPA(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	hpa, err := h.manager.GetHPA(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: hpa})
}

// updateHPA 更新 HPA
func (h *Handlers) updateHPA(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req UpdateHPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	hpa, err := h.manager.UpdateHPA(namespace, name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "更新成功", Data: hpa})
}

// deleteHPA 删除 HPA
func (h *Handlers) deleteHPA(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if err := h.manager.DeleteHPA(namespace, name); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "删除成功"})
}

// ========== 应用商店集成 Handlers ==========

// listAppStoreApps 列出可部署应用
func (h *Handlers) listAppStoreApps(c *gin.Context) {
	apps := h.manager.ListAppStoreApps()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(apps),
			"apps":  apps,
		},
	})
}

// deployFromAppStore 从应用商店部署
func (h *Handlers) deployFromAppStore(c *gin.Context) {
	var req AppStoreDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	release, err := h.manager.DeployFromAppStore(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "部署成功", Data: release})
}

// ========== 资源配额 Handlers ==========

// createQuota 创建配额
func (h *Handlers) createQuota(c *gin.Context) {
	var req CreateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	quota := h.manager.CreateQuota(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "创建成功", Data: quota})
}

// listQuotas 列出配额
func (h *Handlers) listQuotas(c *gin.Context) {
	namespace := c.Query("namespace")
	quotas := h.manager.ListQuotas(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(quotas),
			"quotas": quotas,
		},
	})
}

// getQuota 获取配额
func (h *Handlers) getQuota(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	quota, err := h.manager.GetQuota(namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: quota})
}

// updateQuota 更新配额
func (h *Handlers) updateQuota(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req UpdateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数无效: " + err.Error()})
		return
	}

	quota, err := h.manager.UpdateQuota(namespace, name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "更新成功", Data: quota})
}

// deleteQuota 删除配额
func (h *Handlers) deleteQuota(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if err := h.manager.DeleteQuota(namespace, name); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "删除成功"})
}

// ========== 集群事件 Handlers ==========

// listEvents 列出事件
func (h *Handlers) listEvents(c *gin.Context) {
	namespace := c.Query("namespace")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	events := h.manager.ListEvents(namespace, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// getEventsBySeverity 按严重级别获取事件
func (h *Handlers) getEventsBySeverity(c *gin.Context) {
	severity := EventSeverity(c.Param("severity"))
	events := h.manager.GetEventsBySeverity(severity)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(events),
			"severity": severity,
			"events":   events,
		},
	})
}

// clearEvents 清除事件
func (h *Handlers) clearEvents(c *gin.Context) {
	namespace := c.Query("namespace")
	removed := h.manager.ClearEvents(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "事件已清除",
		Data: gin.H{
			"removed": removed,
		},
	})
}
