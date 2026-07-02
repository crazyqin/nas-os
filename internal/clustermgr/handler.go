// Package clustermgr 提供集群管理器功能
// 参考群晖 DSM Cluster Manager，实现多节点集群管理、工作负载迁移、
// QoS 控管、集中化保护及节点健康监控
package clustermgr

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler 集群管理器 API 处理器.
// 注册到 /api/v1/clustermgr/ 路由.
type Handler struct {
	service *Service
}

// NewHandler 创建集群管理器处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由到 /api/v1/clustermgr/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/clustermgr")
	{
		g.POST("/clusters", h.createCluster)                                  // 创建集群
		g.GET("/clusters", h.listClusters)                                    // 列出集群
		g.GET("/clusters/:clusterId", h.getCluster)                           // 获取集群详情
		g.POST("/clusters/:clusterId/nodes", h.addNode)                       // 添加节点
		g.DELETE("/clusters/:clusterId/nodes/:nodeId", h.removeNode)          // 移除节点
		g.GET("/clusters/:clusterId/nodes", h.getNodes)                       // 获取节点列表
		g.POST("/clusters/:clusterId/migrate", h.migrateWorkload)             // 工作负载迁移
		g.GET("/clusters/:clusterId/migrations/:migrationId", h.getMigration) // 获取迁移状态
		g.POST("/clusters/:clusterId/qos", h.createQoSRule)                   // 创建 QoS 规则
		g.GET("/clusters/:clusterId/qos", h.getQoSRules)                      // 获取 QoS 规则
		g.DELETE("/clusters/:clusterId/qos/:ruleId", h.deleteQoSRule)         // 删除 QoS 规则
		g.POST("/clusters/:clusterId/protections", h.createProtection)        // 创建保护策略
		g.GET("/clusters/:clusterId/protections", h.getProtections)           // 获取保护策略
		g.GET("/clusters/:clusterId/health", h.checkHealth)                   // 健康检查
	}
}

// createCluster 创建集群.
func (h *Handler) createCluster(c *gin.Context) {
	var req CreateClusterRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreateCluster(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, result)
}

// listClusters 列出所有集群.
func (h *Handler) listClusters(c *gin.Context) {
	result, err := h.service.ListClusters()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// getCluster 获取集群详情.
func (h *Handler) getCluster(c *gin.Context) {
	clusterID := c.Param("clusterId")
	if clusterID == "" {
		api.BadRequest(c, "clusterId 参数不能为空")
		return
	}

	result, err := h.service.GetCluster(clusterID)
	if err != nil {
		api.HandleError(c, err, "集群不存在")
		return
	}

	api.OK(c, result)
}

// addNode 添加节点到集群.
func (h *Handler) addNode(c *gin.Context) {
	clusterID := c.Param("clusterId")
	var req AddNodeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	req.ClusterID = clusterID

	result, err := h.service.AddNode(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "添加节点失败")
		return
	}

	api.Created(c, result)
}

// removeNode 从集群移除节点.
func (h *Handler) removeNode(c *gin.Context) {
	clusterID := c.Param("clusterId")
	nodeID := c.Param("nodeId")

	var req RemoveNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		api.BadRequest(c, err.Error())
		return
	}
	req.ClusterID = clusterID
	req.NodeID = nodeID

	result, err := h.service.RemoveNode(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "移除节点失败")
		return
	}

	api.OK(c, result)
}

// getNodes 获取集群所有节点.
func (h *Handler) getNodes(c *gin.Context) {
	clusterID := c.Param("clusterId")

	result, err := h.service.GetNodes(clusterID)
	if err != nil {
		api.HandleError(c, err, "集群不存在")
		return
	}

	api.OK(c, result)
}

// migrateWorkload 迁移工作负载.
func (h *Handler) migrateWorkload(c *gin.Context) {
	clusterID := c.Param("clusterId")
	var req MigrateWorkloadRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	req.ClusterID = clusterID

	result, err := h.service.MigrateWorkload(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "工作负载迁移失败")
		return
	}

	api.OK(c, result)
}

// getMigration 获取迁移状态.
func (h *Handler) getMigration(c *gin.Context) {
	clusterID := c.Param("clusterId")
	migrationID := c.Param("migrationId")

	result, err := h.service.GetMigration(clusterID, migrationID)
	if err != nil {
		api.HandleError(c, err, "迁移任务不存在")
		return
	}

	api.OK(c, result)
}

// createQoSRule 创建 QoS 规则.
func (h *Handler) createQoSRule(c *gin.Context) {
	clusterID := c.Param("clusterId")
	var req CreateQoSRuleRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	req.ClusterID = clusterID

	result, err := h.service.CreateQoSRule(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "创建 QoS 规则失败")
		return
	}

	api.Created(c, result)
}

// getQoSRules 获取 QoS 规则列表.
func (h *Handler) getQoSRules(c *gin.Context) {
	clusterID := c.Param("clusterId")

	result, err := h.service.GetQoSRules(clusterID)
	if err != nil {
		api.HandleError(c, err, "集群不存在")
		return
	}

	api.OK(c, result)
}

// deleteQoSRule 删除 QoS 规则.
func (h *Handler) deleteQoSRule(c *gin.Context) {
	clusterID := c.Param("clusterId")
	ruleID := c.Param("ruleId")

	if err := h.service.DeleteQoSRule(clusterID, ruleID); err != nil {
		api.HandleError(c, err, "删除 QoS 规则失败")
		return
	}

	api.NoContent(c)
}

// createProtection 创建保护策略.
func (h *Handler) createProtection(c *gin.Context) {
	clusterID := c.Param("clusterId")
	var req CreateProtectionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	req.ClusterID = clusterID

	result, err := h.service.CreateProtection(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "创建保护策略失败")
		return
	}

	api.Created(c, result)
}

// getProtections 获取保护策略列表.
func (h *Handler) getProtections(c *gin.Context) {
	clusterID := c.Param("clusterId")

	result, err := h.service.GetProtections(clusterID)
	if err != nil {
		api.HandleError(c, err, "集群不存在")
		return
	}

	api.OK(c, result)
}

// checkHealth 健康检查.
func (h *Handler) checkHealth(c *gin.Context) {
	clusterID := c.Param("clusterId")

	result, err := h.service.CheckNodeHealth(c.Request.Context(), clusterID)
	if err != nil {
		api.HandleError(c, err, "健康检查失败")
		return
	}

	api.OK(c, result)
}
