package kmipclient

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers provides HTTP handlers for KMIPClient
type Handlers struct {
	client *Client
	logger *zap.Logger
}

// NewHandlers creates new KMIPClient handlers
func NewHandlers(client *Client, logger *zap.Logger) *Handlers {
	return &Handlers{client: client, logger: logger}
}

// RegisterRoutes registers KMIPClient API routes
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	kmip := rg.Group("/kmip")
	{
		kmip.GET("/stats", h.GetStats)
		kmip.GET("/keys", h.ListKeys)
		kmip.POST("/keys", h.CreateKey)
		kmip.GET("/keys/:id", h.GetKey)
		kmip.POST("/keys/:id/activate", h.ActivateKey)
		kmip.POST("/keys/:id/revoke", h.RevokeKey)
		kmip.POST("/keys/:id/rotate", h.RotateKey)
		kmip.DELETE("/keys/:id", h.DestroyKey)
	}
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.client.GetStats()})
}

func (h *Handlers) ListKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.client.ListKeys(c.Request.Context())})
}

func (h *Handlers) CreateKey(c *gin.Context) {
	var req struct {
		Name    string  `json:"name"`
		Type    KeyType `json:"type"`
		KeySize int     `json:"key_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	key, err := h.client.CreateKey(c.Request.Context(), req.Name, req.Type, req.KeySize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": key})
}

func (h *Handlers) GetKey(c *gin.Context) {
	id := c.Param("id")
	key, err := h.client.GetKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": key})
}

func (h *Handlers) ActivateKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.client.ActivateKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "key activated"})
}

func (h *Handlers) RevokeKey(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.client.RevokeKey(c.Request.Context(), id, req.Reason); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "key revoked"})
}

func (h *Handlers) RotateKey(c *gin.Context) {
	id := c.Param("id")
	key, err := h.client.RotateKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": key})
}

func (h *Handlers) DestroyKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.client.DestroyKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "key destroyed"})
}
