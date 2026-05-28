package digitallegacy

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct{ mgr *Manager }
func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/digital-legacy")
	{
		g.GET("/beneficiaries", h.ListBeneficiaries)
		g.POST("/beneficiaries", h.AddBeneficiary)
		g.GET("/beneficiaries/:id", h.GetBeneficiary)
		g.POST("/beneficiaries/:id/verify", h.VerifyBeneficiary)
		g.DELETE("/beneficiaries/:id", h.RemoveBeneficiary)

		g.GET("/plans", h.ListPlans)
		g.POST("/plans", h.CreatePlan)
		g.GET("/plans/:id", h.GetPlan)
		g.POST("/plans/:id/seal", h.SealPlan)

		g.GET("/deadman/check", h.CheckDeadman)
		g.GET("/access-logs", h.GetAccessLogs)
		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) ListBeneficiaries(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListBeneficiaries()})
}

func (h *Handlers) AddBeneficiary(c *gin.Context) {
	var b Beneficiary
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.AddBeneficiary(&b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": b})
}

func (h *Handlers) GetBeneficiary(c *gin.Context) {
	b, err := h.mgr.GetBeneficiary(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": b})
}

func (h *Handlers) VerifyBeneficiary(c *gin.Context) {
	if err := h.mgr.VerifyBeneficiary(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "verified"})
}

func (h *Handlers) RemoveBeneficiary(c *gin.Context) {
	if err := h.mgr.RemoveBeneficiary(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "removed"})
}

func (h *Handlers) ListPlans(c *gin.Context) {
	var plans []*LegacyPlan
	for _, p := range h.mgr.plans {
		plans = append(plans, p)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": plans})
}

func (h *Handlers) CreatePlan(c *gin.Context) {
	var plan LegacyPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreatePlan(&plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": plan})
}

func (h *Handlers) GetPlan(c *gin.Context) {
	plan, err := h.mgr.GetPlan(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": plan})
}

func (h *Handlers) SealPlan(c *gin.Context) {
	if err := h.mgr.SealPlan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sealed"})
}

func (h *Handlers) CheckDeadman(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.CheckDeadman()})
}

func (h *Handlers) GetAccessLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetAccessLogs(c.Query("beneficiary_id"))})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}
