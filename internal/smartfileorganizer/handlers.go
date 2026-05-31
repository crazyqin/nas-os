// Package smartfileorganizer provides HTTP handlers for the smart file organizer.
package smartfileorganizer

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler smart file organizer HTTP handler
type Handler struct {
	organizer *Organizer
}

// NewHandler creates a new handler
func NewHandler(organizer *Organizer) *Handler {
	return &Handler{organizer: organizer}
}

// RegisterRoutes registers routes
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	organizeGroup := r.Group("/smart-organize")
	{
		// Scan
		organizeGroup.POST("/scan", h.HandleScan)

		// Rules
		organizeGroup.POST("/rules", h.HandleAddRule)
		organizeGroup.GET("/rules", h.HandleGetRules)
		organizeGroup.DELETE("/rules/:id", h.HandleRemoveRule)

		// Organize
		organizeGroup.POST("/organize", h.HandleOrganize)

		// Duplicates
		organizeGroup.GET("/duplicates", h.HandleFindDuplicates)

		// Category
		organizeGroup.GET("/category/:category", h.HandleGetByCategory)

		// Stats
		organizeGroup.GET("/stats", h.HandleGetStats)
	}
}

// HandleScan handles scan request
func (h *Handler) HandleScan(c *gin.Context) {
	report, err := h.organizer.Scan()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "scan_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, report)
}

// HandleAddRule handles add rule request
func (h *Handler) HandleAddRule(c *gin.Context) {
	var rule OrganizationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "请求参数无效: " + err.Error(),
		})
		return
	}

	h.organizer.AddRule(rule)
	c.JSON(http.StatusCreated, rule)
}

// HandleGetRules handles get rules request
func (h *Handler) HandleGetRules(c *gin.Context) {
	rules := h.organizer.GetRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// HandleRemoveRule handles remove rule request
func (h *Handler) HandleRemoveRule(c *gin.Context) {
	ruleID := c.Param("id")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_id",
			"message": "规则ID不能为空",
		})
		return
	}

	if !h.organizer.RemoveRule(ruleID) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "规则不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "规则删除成功",
	})
}

// HandleOrganize handles organize request
func (h *Handler) HandleOrganize(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		req.DryRun = true // Default to dry run
	}

	report, err := h.organizer.Organize(req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "organize_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, report)
}

// HandleFindDuplicates handles find duplicates request
func (h *Handler) HandleFindDuplicates(c *gin.Context) {
	groups := h.organizer.FindDuplicates()
	c.JSON(http.StatusOK, gin.H{
		"duplicates": groups,
		"count":      len(groups),
	})
}

// HandleGetByCategory handles get by category request
func (h *Handler) HandleGetByCategory(c *gin.Context) {
	category := FileCategory(c.Param("category"))
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_category",
			"message": "分类不能为空",
		})
		return
	}

	files := h.organizer.GetByCategory(category)
	c.JSON(http.StatusOK, gin.H{
		"files":    files,
		"total":    len(files),
		"category": category,
	})
}

// HandleGetStats handles get stats request
func (h *Handler) HandleGetStats(c *gin.Context) {
	stats := h.organizer.GetStats()
	c.JSON(http.StatusOK, stats)
}
