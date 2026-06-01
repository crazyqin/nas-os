package unifieddatalake

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API处理器
type Handler struct {
	lake *DataLake
}

// NewHandler 创建Handler
func NewHandler(lake *DataLake) *Handler {
	return &Handler{lake: lake}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	dl := rg.Group("/unifieddatalake")
	{
		// 数据源管理
		dl.POST("/sources", h.AddSource)
		dl.GET("/sources", h.ListSources)
		dl.GET("/sources/:id", h.GetSource)
		dl.PUT("/sources/:id", h.UpdateSource)
		dl.DELETE("/sources/:id", h.RemoveSource)

		// 数据对象管理
		dl.POST("/objects", h.RegisterObject)
		dl.GET("/objects", h.ListObjects)
		dl.GET("/objects/:id", h.GetObject)
		dl.DELETE("/objects/:id", h.RemoveObject)

		// 数据目录管理
		dl.POST("/catalogs", h.AddCatalogEntry)
		dl.GET("/catalogs", h.ListCatalogEntries)
		dl.GET("/catalogs/:id", h.GetCatalogEntry)
		dl.PUT("/catalogs/:id", h.UpdateCatalogEntry)
		dl.GET("/catalogs/search", h.SearchCatalog)

		// 数据血缘
		dl.POST("/lineages", h.CreateLineage)
		dl.GET("/lineages", h.ListLineages)
		dl.GET("/lineages/:id", h.GetLineage)
		dl.POST("/lineages/:id/nodes", h.AddLineageNode)
		dl.POST("/lineages/:id/edges", h.AddLineageEdge)
		dl.GET("/lineages/object/:objectId", h.GetLineageByObject)
		dl.GET("/lineages/:id/nodes/:nodeId/upstream", h.GetUpstream)
		dl.GET("/lineages/:id/nodes/:nodeId/downstream", h.GetDownstream)

		// 数据质量管理
		dl.POST("/quality/rules", h.AddQualityRule)
		dl.GET("/quality/rules", h.ListQualityRules)
		dl.GET("/quality/rules/:id", h.GetQualityRule)
		dl.DELETE("/quality/rules/:id", h.RemoveQualityRule)
		dl.POST("/quality/check", h.RunQualityCheck)
		dl.GET("/quality/results/:objectId", h.GetQualityResults)

		// 统计
		dl.GET("/stats", h.GetStats)
	}
}

// ==================== 数据源Handler ====================

// AddSource 添加数据源
func (h *Handler) AddSource(c *gin.Context) {
	var source DataSource
	if err := c.ShouldBindJSON(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.lake.AddSource(&source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, source)
}

// ListSources 列出数据源
func (h *Handler) ListSources(c *gin.Context) {
	sources := h.lake.ListSources()
	c.JSON(http.StatusOK, gin.H{"sources": sources})
}

// GetSource 获取数据源
func (h *Handler) GetSource(c *gin.Context) {
	id := c.Param("id")
	source, ok := h.lake.GetSource(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	c.JSON(http.StatusOK, source)
}

// UpdateSource 更新数据源
func (h *Handler) UpdateSource(c *gin.Context) {
	id := c.Param("id")
	var source DataSource
	if err := c.ShouldBindJSON(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	source.ID = id
	if err := h.lake.UpdateSource(&source); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, source)
}

// RemoveSource 移除数据源
func (h *Handler) RemoveSource(c *gin.Context) {
	id := c.Param("id")
	if err := h.lake.RemoveSource(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ==================== 数据对象Handler ====================

// RegisterObject 注册数据对象
func (h *Handler) RegisterObject(c *gin.Context) {
	var obj DataObject
	if err := c.ShouldBindJSON(&obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.lake.RegisterObject(&obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, obj)
}

// ListObjects 列出数据对象
func (h *Handler) ListObjects(c *gin.Context) {
	sourceID := c.Query("source_id")
	objects := h.lake.ListObjects(sourceID)
	c.JSON(http.StatusOK, gin.H{"objects": objects})
}

// GetObject 获取数据对象
func (h *Handler) GetObject(c *gin.Context) {
	id := c.Param("id")
	obj, ok := h.lake.GetObject(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
		return
	}
	c.JSON(http.StatusOK, obj)
}

// RemoveObject 移除数据对象
func (h *Handler) RemoveObject(c *gin.Context) {
	id := c.Param("id")
	if err := h.lake.RemoveObject(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ==================== 数据目录Handler ====================

// AddCatalogEntry 添加目录条目
func (h *Handler) AddCatalogEntry(c *gin.Context) {
	var entry CatalogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.lake.AddCatalogEntry(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// ListCatalogEntries 列出目录条目
func (h *Handler) ListCatalogEntries(c *gin.Context) {
	category := c.Query("category")
	entries := h.lake.ListCatalogEntries(category)
	c.JSON(http.StatusOK, gin.H{"catalogs": entries})
}

// GetCatalogEntry 获取目录条目
func (h *Handler) GetCatalogEntry(c *gin.Context) {
	id := c.Param("id")
	entry, ok := h.lake.GetCatalogEntry(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalog entry not found"})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// UpdateCatalogEntry 更新目录条目
func (h *Handler) UpdateCatalogEntry(c *gin.Context) {
	id := c.Param("id")
	var entry CatalogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.ID = id
	if err := h.lake.UpdateCatalogEntry(&entry); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// SearchCatalog 搜索目录
func (h *Handler) SearchCatalog(c *gin.Context) {
	query := c.Query("q")
	entries := h.lake.SearchCatalog(query)
	c.JSON(http.StatusOK, gin.H{"results": entries})
}

// ==================== 数据血缘Handler ====================

// CreateLineage 创建血缘图
func (h *Handler) CreateLineage(c *gin.Context) {
	var graph LineageGraph
	if err := c.ShouldBindJSON(&graph); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.lake.CreateLineage(&graph); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, graph)
}

// ListLineages 列出血缘图
func (h *Handler) ListLineages(c *gin.Context) {
	lineages := h.lake.ListLineages()
	c.JSON(http.StatusOK, gin.H{"lineages": lineages})
}

// GetLineage 获取血缘图
func (h *Handler) GetLineage(c *gin.Context) {
	id := c.Param("id")
	graph, ok := h.lake.GetLineage(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "lineage not found"})
		return
	}
	c.JSON(http.StatusOK, graph)
}

// AddLineageNodeRequest 添加血缘节点请求
type AddLineageNodeRequest struct {
	ID       string `json:"id"`
	ObjectID string `json:"object_id" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`
}

// AddLineageNode 添加血缘节点
func (h *Handler) AddLineageNode(c *gin.Context) {
	lineageID := c.Param("id")
	var req AddLineageNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	node := &LineageNode{
		ID:       req.ID,
		ObjectID: req.ObjectID,
		Name:     req.Name,
		Type:     req.Type,
	}
	if err := h.lake.AddLineageNode(lineageID, node); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, node)
}

// AddLineageEdgeRequest 添加血缘边请求
type AddLineageEdgeRequest struct {
	ID           string `json:"id"`
	SourceNodeID string `json:"source_node_id" binding:"required"`
	TargetNodeID string `json:"target_node_id" binding:"required"`
	Transform    string `json:"transform"`
}

// AddLineageEdge 添加血缘边
func (h *Handler) AddLineageEdge(c *gin.Context) {
	lineageID := c.Param("id")
	var req AddLineageEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	edge := &LineageEdge{
		ID:           req.ID,
		SourceNodeID: req.SourceNodeID,
		TargetNodeID: req.TargetNodeID,
		Transform:    req.Transform,
	}
	if err := h.lake.AddLineageEdge(lineageID, edge); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, edge)
}

// GetLineageByObject 根据对象获取血缘
func (h *Handler) GetLineageByObject(c *gin.Context) {
	objectID := c.Param("objectId")
	graphs := h.lake.GetLineageByObject(objectID)
	c.JSON(http.StatusOK, gin.H{"lineages": graphs})
}

// GetUpstream 获取上游节点
func (h *Handler) GetUpstream(c *gin.Context) {
	lineageID := c.Param("id")
	nodeID := c.Param("nodeId")
	nodes, err := h.lake.GetUpstream(lineageID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// GetDownstream 获取下游节点
func (h *Handler) GetDownstream(c *gin.Context) {
	lineageID := c.Param("id")
	nodeID := c.Param("nodeId")
	nodes, err := h.lake.GetDownstream(lineageID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// ==================== 数据质量Handler ====================

// AddQualityRule 添加质量规则
func (h *Handler) AddQualityRule(c *gin.Context) {
	var rule QualityRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.lake.AddQualityRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// ListQualityRules 列出质量规则
func (h *Handler) ListQualityRules(c *gin.Context) {
	rules := h.lake.ListQualityRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// GetQualityRule 获取质量规则
func (h *Handler) GetQualityRule(c *gin.Context) {
	id := c.Param("id")
	rule, ok := h.lake.GetQualityRule(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// RemoveQualityRule 移除质量规则
func (h *Handler) RemoveQualityRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.lake.RemoveQualityRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// QualityCheckRequest 质量检查请求
type QualityCheckRequest struct {
	ObjectID string   `json:"object_id" binding:"required"`
	RuleIDs  []string `json:"rule_ids" binding:"required"`
}

// RunQualityCheck 执行质量检查
func (h *Handler) RunQualityCheck(c *gin.Context) {
	var req QualityCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.lake.RunQualityCheck(req.ObjectID, req.RuleIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// GetQualityResults 获取质量检查结果
func (h *Handler) GetQualityResults(c *gin.Context) {
	objectID := c.Param("objectId")
	results := h.lake.GetQualityResults(objectID)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// ==================== 统计Handler ====================

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.lake.GetStats()
	c.JSON(http.StatusOK, stats)
}

// unused import guard
var _ = time.Now
