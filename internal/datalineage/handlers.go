// Package datalineage handlers - HTTP API
package datalineage

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/datalineage")
	{
		// 数据节点管理
		g.POST("/nodes", h.CreateNode)
		g.GET("/nodes", h.ListNodes)
		g.GET("/nodes/:id", h.GetNode)
		g.PUT("/nodes/:id", h.UpdateNode)
		g.DELETE("/nodes/:id", h.DeleteNode)

		// 血缘关系管理
		g.POST("/edges", h.CreateEdge)
		g.GET("/edges", h.ListEdges)
		g.GET("/edges/:id", h.GetEdge)
		g.DELETE("/edges/:id", h.DeleteEdge)

		// 血缘图
		g.GET("/graph/:node_id", h.GetLineageGraph)

		// 影响分析
		g.GET("/impact/:node_id", h.ImpactAnalysis)

		// 数据溯源
		g.GET("/trace/:node_id", h.TraceSource)

		// 合规审计
		g.POST("/records", h.AddProcessingRecord)
		g.GET("/records", h.GetProcessingRecords)
		g.GET("/compliance/report", h.GenerateComplianceReport)

		// 数据分类管理
		g.PUT("/nodes/:id/classification", h.ManageClassification)

		// 自动采集
		g.POST("/collect", h.AutoCollectLineage)

		// 统计
		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) CreateNode(c *gin.Context) {
	var node DataNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateNode(&node); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": node})
}

func (h *Handlers) ListNodes(c *gin.Context) {
	srcType := DataSourceType(c.Query("type"))
	classification := DataClassification(c.Query("classification"))
	nodes := h.mgr.ListNodes(srcType, classification)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": nodes, "total": len(nodes)})
}

func (h *Handlers) GetNode(c *gin.Context) {
	node, err := h.mgr.GetNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": node})
}

func (h *Handlers) UpdateNode(c *gin.Context) {
	var update DataNode
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	node, err := h.mgr.UpdateNode(c.Param("id"), &update)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": node})
}

func (h *Handlers) DeleteNode(c *gin.Context) {
	if err := h.mgr.DeleteNode(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) CreateEdge(c *gin.Context) {
	var edge LineageEdge
	if err := c.ShouldBindJSON(&edge); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateEdge(&edge); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": edge})
}

func (h *Handlers) ListEdges(c *gin.Context) {
	nodeID := c.Query("node_id")
	edges := h.mgr.ListEdges(nodeID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": edges, "total": len(edges)})
}

func (h *Handlers) GetEdge(c *gin.Context) {
	edge, err := h.mgr.GetEdge(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": edge})
}

func (h *Handlers) DeleteEdge(c *gin.Context) {
	if err := h.mgr.DeleteEdge(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) GetLineageGraph(c *gin.Context) {
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "5"))
	graph, err := h.mgr.GetLineageGraph(c.Param("node_id"), depth)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": graph})
}

func (h *Handlers) ImpactAnalysis(c *gin.Context) {
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "5"))
	result, err := h.mgr.ImpactAnalysis(c.Param("node_id"), depth)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) TraceSource(c *gin.Context) {
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "5"))
	result, err := h.mgr.TraceSource(c.Param("node_id"), depth)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) AddProcessingRecord(c *gin.Context) {
	var record ProcessingRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.AddProcessingRecord(&record); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": record})
}

func (h *Handlers) GetProcessingRecords(c *gin.Context) {
	nodeID := c.Query("node_id")
	regulation := ComplianceRegulation(c.Query("regulation"))
	records := h.mgr.GetProcessingRecords(nodeID, regulation)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": records, "total": len(records)})
}

func (h *Handlers) GenerateComplianceReport(c *gin.Context) {
	regulation := ComplianceRegulation(c.Query("regulation"))
	report := h.mgr.GenerateComplianceReport(regulation)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

func (h *Handlers) ManageClassification(c *gin.Context) {
	var req struct {
		Classification DataClassification `json:"classification"`
		Tags           []string           `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	node, err := h.mgr.ManageClassification(c.Param("id"), req.Classification, req.Tags)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": node})
}

func (h *Handlers) AutoCollectLineage(c *gin.Context) {
	var records []AutoCollectRecord
	if err := c.ShouldBindJSON(&records); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	collected, err := h.mgr.AutoCollectLineage(records)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "collected": collected})
}

func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
