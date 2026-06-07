package privacyscore

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/privacy-score")
	{
		g.POST("/scan", h.Scan)
		g.GET("/reports", h.ListReports)
		g.GET("/reports/latest", h.GetLatest)
		g.GET("/reports/:id", h.GetReport)
		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) Scan(c *gin.Context) {
	report := h.mgr.RunScan()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

func (h *Handlers) ListReports(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListReports()})
}

func (h *Handlers) GetLatest(c *gin.Context) {
	report, err := h.mgr.GetLatestReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

func (h *Handlers) GetReport(c *gin.Context) {
	report, err := h.mgr.GetReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}
