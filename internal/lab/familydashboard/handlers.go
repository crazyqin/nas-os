package familydashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/family")
	{
		g.GET("/members", h.ListMembers)
		g.POST("/members", h.AddMember)
		g.GET("/members/:id", h.GetMember)
		g.PUT("/members/:id", h.UpdateMember)
		g.DELETE("/members/:id", h.RemoveMember)

		g.GET("/chores", h.ListChores)
		g.POST("/chores", h.CreateChore)
		g.POST("/chores/:id/complete", h.CompleteChore)

		g.GET("/allowance", h.GetAllowance)
		g.POST("/allowance", h.AddAllowance)
		g.GET("/allowance/:id/balance", h.GetBalance)

		g.GET("/notes", h.ListNotes)
		g.POST("/notes", h.CreateNote)
		g.PUT("/notes/:id", h.UpdateNote)
		g.DELETE("/notes/:id", h.DeleteNote)

		g.POST("/screen-time", h.SetScreenTime)
		g.GET("/screen-time", h.GetScreenTime)

		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) ListMembers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListMembers()})
}

func (h *Handlers) AddMember(c *gin.Context) {
	var m FamilyMember
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.AddMember(&m); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": m})
}

func (h *Handlers) GetMember(c *gin.Context) {
	m, err := h.mgr.GetMember(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": m})
}

func (h *Handlers) UpdateMember(c *gin.Context) {
	var m FamilyMember
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	m.ID = c.Param("id")
	if err := h.mgr.UpdateMember(&m); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": m})
}

func (h *Handlers) RemoveMember(c *gin.Context) {
	if err := h.mgr.RemoveMember(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "removed"})
}

func (h *Handlers) ListChores(c *gin.Context) {
	assigneeID := c.Query("assignee_id")
	pending := c.Query("pending") == "true"
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListChores(assigneeID, pending)})
}

func (h *Handlers) CreateChore(c *gin.Context) {
	var chore Chore
	if err := c.ShouldBindJSON(&chore); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateChore(&chore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": chore})
}

func (h *Handlers) CompleteChore(c *gin.Context) {
	if err := h.mgr.CompleteChore(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "completed"})
}

func (h *Handlers) GetAllowance(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetAllowance(c.Query("member_id"))})
}

func (h *Handlers) AddAllowance(c *gin.Context) {
	var a Allowance
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.AddAllowance(&a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": a})
}

func (h *Handlers) GetBalance(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "balance": h.mgr.GetBalance(c.Param("id"))})
}

func (h *Handlers) ListNotes(c *gin.Context) {
	pinned := c.Query("pinned") == "true"
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListNotes(pinned)})
}

func (h *Handlers) CreateNote(c *gin.Context) {
	var note FamilyNote
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateNote(&note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": note})
}

func (h *Handlers) UpdateNote(c *gin.Context) {
	var note FamilyNote
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	note.ID = c.Param("id")
	if err := h.mgr.UpdateNote(&note); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": note})
}

func (h *Handlers) DeleteNote(c *gin.Context) {
	if err := h.mgr.DeleteNote(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) SetScreenTime(c *gin.Context) {
	var st ScreenTime
	if err := c.ShouldBindJSON(&st); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.SetScreenTime(&st); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": st})
}

func (h *Handlers) GetScreenTime(c *gin.Context) {
	st := h.mgr.GetScreenTime(c.Query("member_id"), c.Query("date"))
	if st == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": st})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}
