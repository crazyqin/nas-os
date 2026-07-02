package aiplatform

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler AI平台API handler.
type Handler struct {
	platform *AIPlatform
}

// NewHandler 创建handler.
func NewHandler(platform *AIPlatform) *Handler {
	return &Handler{platform: platform}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai/platform")
	{
		ai.POST("/complete", h.Complete)
		ai.POST("/embed", h.Embed)
		ai.GET("/models", h.ListModels)
		ai.GET("/providers", h.ListProviders)
		ai.GET("/stats", h.GetStats)
	}
}

type CompleteRequest struct {
	Model       string    `json:"model" binding:"required"`
	Messages    []Message `json:"messages" binding:"required"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float32   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

func (h *Handler) Complete(c *gin.Context) {
	var req CompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	completionReq := &CompletionRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	if req.Stream {
		ch, err := h.platform.Stream(c.Request.Context(), completionReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
			return
		}

		for resp := range ch {
			data, _ := json.Marshal(resp)
			c.Writer.Write(append(data, '\n'))
			flusher.Flush()
		}
		return
	}

	resp, err := h.platform.Complete(c.Request.Context(), completionReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

type EmbedRequest struct {
	Text  string `json:"text" binding:"required"`
	Model string `json:"model"`
}

func (h *Handler) Embed(c *gin.Context) {
	var req EmbedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	embedding, err := h.platform.Embed(c.Request.Context(), req.Text, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embedding": embedding, "dimension": len(embedding)})
}

func (h *Handler) ListModels(c *gin.Context) {
	models := h.platform.ListModels()
	c.JSON(http.StatusOK, models)
}

func (h *Handler) ListProviders(c *gin.Context) {
	providers := h.platform.ListProviders()
	c.JSON(http.StatusOK, providers)
}

func (h *Handler) GetStats(c *gin.Context) {
	stats := h.platform.GetProviderStats()
	c.JSON(http.StatusOK, stats)
}
