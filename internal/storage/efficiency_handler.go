// Package storage provides HTTP handlers for storage efficiency statistics.
package storage

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// EfficiencyHandler provides HTTP handlers for storage efficiency.
type EfficiencyHandler struct {
	collector *EfficiencyCollector
}

// NewEfficiencyHandler creates a new efficiency handler.
func NewEfficiencyHandler(collector *EfficiencyCollector) *EfficiencyHandler {
	return &EfficiencyHandler{collector: collector}
}

// RegisterRoutes registers efficiency API routes.
func (h *EfficiencyHandler) RegisterRoutes(rg *gin.RouterGroup) {
	efficiency := rg.Group("/storage/efficiency")
	{
		efficiency.GET("", h.getEfficiency)
		efficiency.GET("/pools", h.listPools)
		efficiency.GET("/pools/:name", h.getPool)
		efficiency.POST("/refresh", h.refresh)
	}
}

func (h *EfficiencyHandler) getEfficiency(c *gin.Context) {
	stats := h.collector.GetEfficiency()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *EfficiencyHandler) listPools(c *gin.Context) {
	pools := h.collector.ListPoolEfficiencies()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": pools})
}

func (h *EfficiencyHandler) getPool(c *gin.Context) {
	name := c.Param("name")
	pool, err := h.collector.GetPoolEfficiency(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": pool})
}

func (h *EfficiencyHandler) refresh(c *gin.Context) {
	if err := h.collector.Refresh(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Storage efficiency stats refreshed"})
}
