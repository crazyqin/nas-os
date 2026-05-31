// Package knowledgemap 提供 REST API 处理器与业务逻辑
package knowledgemap

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Manager 知识图谱管理器
type Manager struct {
	mu              sync.RWMutex
	nodes           map[string]*KnowledgeNode
	relations       map[string]*NodeRelation
	classifications map[string]*Classification
	reviewQueue     []string
}

// NewManager 创建知识图谱管理器
func NewManager() *Manager {
	return &Manager{
		nodes:           make(map[string]*KnowledgeNode),
		relations:       make(map[string]*NodeRelation),
		classifications: make(map[string]*Classification),
		reviewQueue:     make([]string, 0),
	}
}

// Handlers 知识图谱 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	km := r.Group("/knowledgemap")
	{
		// 知识节点管理
		km.GET("/nodes", h.listNodes)
		km.POST("/nodes", h.createNode)
		km.GET("/nodes/:id", h.getNode)
		km.PUT("/nodes/:id", h.updateNode)
		km.DELETE("/nodes/:id", h.deleteNode)
		km.POST("/nodes/:id/review", h.reviewNode)

		// 关联关系管理
		km.GET("/relations", h.listRelations)
		km.POST("/relations", h.createRelation)
		km.DELETE("/relations/:id", h.deleteRelation)
		km.GET("/nodes/:id/related", h.getNodeRelated)

		// 智能检索
		km.POST("/search", h.searchNodes)
		km.GET("/search/tags", h.searchByTags)

		// 知识分类
		km.GET("/classifications", h.listClassifications)
		km.POST("/classifications", h.createClassification)
		km.GET("/classifications/:id", h.getClassification)
		km.DELETE("/classifications/:id", h.deleteClassification)
		km.POST("/classifications/:id/nodes", h.addNodeToClassification)
		km.DELETE("/classifications/:id/nodes/:nodeId", h.removeNodeFromClassification)

		// 图谱可视化
		km.GET("/graph", h.getGraph)
		km.GET("/graph/subgraph/:id", h.getSubgraph)

		// 导入导出
		km.POST("/import", h.importData)
		km.POST("/export", h.exportData)

		// 学习追踪
		km.GET("/stats", h.getStats)
		km.GET("/stats/growth", h.getGrowthTrend)
		km.GET("/review/pending", h.getPendingReviews)

		// 元数据
		km.GET("/types/nodes", h.getNodeTypes)
		km.GET("/types/relations", h.getRelationTypes)
	}
}

// ==================== 节点管理 Handler ====================

func (h *Handlers) listNodes(c *gin.Context) {
	nodeType := c.Query("type")
	tag := c.Query("tag")
	nodes := h.manager.ListNodes(nodeType, tag)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: nodes})
}

func (h *Handlers) createNode(c *gin.Context) {
	var req NodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if !IsValidNodeType(req.Type) {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid node type"})
		return
	}
	node, err := h.manager.CreateNode(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ApiResponse{Code: 0, Message: "node created", Data: node})
}

func (h *Handlers) getNode(c *gin.Context) {
	id := c.Param("id")
	node, err := h.manager.GetNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: node})
}

func (h *Handlers) updateNode(c *gin.Context) {
	id := c.Param("id")
	var req NodeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	node, err := h.manager.UpdateNode(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "node updated", Data: node})
}

func (h *Handlers) deleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteNode(id); err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "node deleted"})
}

func (h *Handlers) reviewNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ReviewNode(id); err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "review recorded"})
}

// ==================== 关联关系 Handler ====================

func (h *Handlers) listRelations(c *gin.Context) {
	relType := c.Query("type")
	relations := h.manager.ListRelations(relType)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: relations})
}

func (h *Handlers) createRelation(c *gin.Context) {
	var req RelationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if !IsValidRelationType(req.Type) {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid relation type"})
		return
	}
	rel, err := h.manager.CreateRelation(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ApiResponse{Code: 0, Message: "relation created", Data: rel})
}

func (h *Handlers) deleteRelation(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRelation(id); err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "relation deleted"})
}

func (h *Handlers) getNodeRelated(c *gin.Context) {
	id := c.Param("id")
	relType := c.Query("type")
	related := h.manager.GetNodeRelated(id, relType)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: related})
}

// ==================== 智能检索 Handler ====================

func (h *Handlers) searchNodes(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	result := h.manager.SearchNodes(&query)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: result})
}

func (h *Handlers) searchByTags(c *gin.Context) {
	tagsStr := c.Query("tags")
	if tagsStr == "" {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "tags parameter is required"})
		return
	}
	tags := strings.Split(tagsStr, ",")
	nodes := h.manager.SearchByTags(tags)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: nodes})
}

// ==================== 分类 Handler ====================

func (h *Handlers) listClassifications(c *gin.Context) {
	dimension := c.Query("dimension")
	classifications := h.manager.ListClassifications(dimension)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: classifications})
}

func (h *Handlers) createClassification(c *gin.Context) {
	var req ClassificationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	classification, err := h.manager.CreateClassification(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ApiResponse{Code: 0, Message: "classification created", Data: classification})
}

func (h *Handlers) getClassification(c *gin.Context) {
	id := c.Param("id")
	classification, err := h.manager.GetClassification(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: classification})
}

func (h *Handlers) deleteClassification(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteClassification(id); err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "classification deleted"})
}

func (h *Handlers) addNodeToClassification(c *gin.Context) {
	classID := c.Param("id")
	var req NodeAddToClassification
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if err := h.manager.AddNodeToClassification(classID, req.NodeID); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "node added to classification"})
}

func (h *Handlers) removeNodeFromClassification(c *gin.Context) {
	classID := c.Param("id")
	nodeID := c.Param("nodeId")
	if err := h.manager.RemoveNodeFromClassification(classID, nodeID); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "node removed from classification"})
}

// ==================== 图谱可视化 Handler ====================

func (h *Handlers) getGraph(c *gin.Context) {
	maxNodes := 100
	if v := c.Query("max_nodes"); v != "" {
		fmt.Sscanf(v, "%d", &maxNodes)
	}
	graph := h.manager.GetGraph(maxNodes)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: graph})
}

func (h *Handlers) getSubgraph(c *gin.Context) {
	id := c.Param("id")
	depth := 2
	if v := c.Query("depth"); v != "" {
		fmt.Sscanf(v, "%d", &depth)
	}
	graph, err := h.manager.GetSubgraph(id, depth)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: graph})
}

// ==================== 导入导出 Handler ====================

func (h *Handlers) importData(c *gin.Context) {
	var req ImportData
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	count, err := h.manager.ImportData(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: fmt.Sprintf("imported %d nodes", count), Data: map[string]int{"imported": count}})
}

func (h *Handlers) exportData(c *gin.Context) {
	var req ExportData
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	result, err := h.manager.ExportData(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: result})
}

// ==================== 学习追踪 Handler ====================

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: stats})
}

func (h *Handlers) getGrowthTrend(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		fmt.Sscanf(v, "%d", &days)
	}
	trend := h.manager.GetGrowthTrend(days)
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: trend})
}

func (h *Handlers) getPendingReviews(c *gin.Context) {
	nodes := h.manager.GetPendingReviews()
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: nodes})
}

// ==================== 元数据 Handler ====================

func (h *Handlers) getNodeTypes(c *gin.Context) {
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: DefaultNodeTypes()})
}

func (h *Handlers) getRelationTypes(c *gin.Context) {
	c.JSON(http.StatusOK, ApiResponse{Code: 0, Message: "success", Data: DefaultRelationTypes()})
}
