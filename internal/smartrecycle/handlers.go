package smartrecycle

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/smart-recycle")
	{
		g.POST("/scan", h.Scan)
		g.GET("/scans", h.ListScans)
		g.GET("/scans/:id", h.GetScan)
		g.POST("/scans/:id/cleanup", h.Cleanup)
		g.GET("/stats", h.GetStats)
		g.GET("/policy", h.GetPolicy)
	}
}

func (h *Handlers) Scan(c *gin.Context) {
	var req struct {
		Path   string      `json:"path"`
		Policy *ScanPolicy `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	result, err := h.mgr.ScanPath(req.Path, req.Policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) ListScans(c *gin.Context) {
	results := h.mgr.ListScanResults()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results, "total": len(results)})
}

func (h *Handlers) GetScan(c *gin.Context) {
	result, err := h.mgr.GetScanResult(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) Cleanup(c *gin.Context) {
	var req struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	report, err := h.mgr.ExecuteCleanup(c.Param("id"), req.ItemIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}

func (h *Handlers) GetPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.DefaultPolicy()})
}
