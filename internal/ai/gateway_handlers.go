// Package ai provides AI service integration for NAS-OS
// gateway_handlers.go - HTTP API handlers for AI Gateway
package ai

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GatewayHandlers provides HTTP handlers for the AI Gateway
type GatewayHandlers struct {
	gateway      *Gateway
	modelManager *ModelManager
	resourceMon  *ResourceMonitor
}

// NewGatewayHandlers creates new gateway handlers
func NewGatewayHandlers(gateway *Gateway, modelManager *ModelManager) *GatewayHandlers {
	return &GatewayHandlers{
		gateway:      gateway,
		modelManager: modelManager,
		resourceMon:  NewResourceMonitor(),
	}
}

// RegisterRoutes registers API routes
func (h *GatewayHandlers) RegisterRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai")
	{
		// OpenAI-compatible endpoints
		ai.POST("/v1/chat/completions", h.ChatCompletions)
		ai.POST("/v1/completions", h.Completions)
		ai.POST("/v1/embeddings", h.Embeddings)
		ai.GET("/v1/models", h.ListModels)
		ai.GET("/v1/models/:model", h.GetModel)

		// Gateway management
		ai.GET("/gateway/status", h.GetGatewayStatus)
		ai.GET("/gateway/metrics", h.GetGatewayMetrics)
		ai.GET("/gateway/backends", h.GetBackends)
		ai.POST("/gateway/route", h.SetModelRoute)

		// Model management
		ai.GET("/models", h.ListLocalModels)
		ai.GET("/models/:name", h.GetLocalModel)
		ai.POST("/models/download", h.DownloadModel)
		ai.DELETE("/models/:name", h.DeleteModel)
		ai.GET("/models/:name/progress", h.GetDownloadProgress)
		ai.POST("/models/search", h.SearchModels)
		ai.GET("/models/storage", h.GetStorageUsage)

		// Resource monitoring
		ai.GET("/resources/gpu", h.GetGPUInfo)
		ai.GET("/resources/memory", h.GetMemoryInfo)
		ai.GET("/resources", h.GetAllResources)
	}
}

// ==================== OpenAI-Compatible API ====================

// ChatCompletions handles /v1/chat/completions
// @Summary Chat completions (OpenAI-compatible)
// @Description Generate chat completions using local LLM backends
// @Tags ai
// @Accept json
// @Produce json
// @Param request body ChatRequest true "Chat request"
// @Success 200 {object} ChatResponse "Success"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 500 {object} ErrorResponse "Server error"
// @Router /ai/v1/chat/completions [post]
func (h *GatewayHandlers) ChatCompletions(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	// Check for streaming
	if req.Stream {
		h.streamChatCompletions(c, &req)
		return
	}

	resp, err := h.gateway.Chat(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// streamChatCompletions handles streaming chat completions
func (h *GatewayHandlers) streamChatCompletions(c *gin.Context, req *ChatRequest) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	err := h.gateway.StreamChat(c.Request.Context(), req, func(chunk string) error {
		data := ChatResponse{
			ID:      "chatcmpl-" + req.Model,
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   req.Model,
			Choices: []ChatChoice{
				{
					Index: 0,
					Delta: &ChatMessage{
						Role:    "assistant",
						Content: chunk,
					},
				},
			},
		}

		jsonData, _ := json.Marshal(data)
		_, _ = c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
		flusher.Flush()
		return nil
	})

	if err != nil {
		_, _ = c.Writer.Write([]byte("data: {\"error\": \"" + err.Error() + "\"}\n\n"))
		flusher.Flush()
		return
	}

	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// Completions handles /v1/completions (legacy)
func (h *GatewayHandlers) Completions(c *gin.Context) {
	var req struct {
		Model       string   `json:"model"`
		Prompt      string   `json:"prompt"`
		MaxTokens   int      `json:"max_tokens,omitempty"`
		Temperature float64  `json:"temperature,omitempty"`
		Stop        []string `json:"stop,omitempty"`
		Stream      bool     `json:"stream,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to chat format
	chatReq := &ChatRequest{
		Model:       req.Model,
		Messages:    []ChatMessage{{Role: "user", Content: req.Prompt}},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stop:        req.Stop,
		Stream:      req.Stream,
	}

	if req.Stream {
		h.streamChatCompletions(c, chatReq)
		return
	}

	resp, err := h.gateway.Chat(c.Request.Context(), chatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to completion format
	c.JSON(http.StatusOK, gin.H{
		"id":      resp.ID,
		"object":  "text_completion",
		"created": resp.Created,
		"model":   resp.Model,
		"choices": []gin.H{
			{
				"text":          resp.Choices[0].Message.Content,
				"index":         0,
				"finish_reason": resp.Choices[0].FinishReason,
			},
		},
		"usage": resp.Usage,
	})
}

// Embeddings handles /v1/embeddings
// @Summary Generate embeddings
// @Description Generate embeddings for text
// @Tags ai
// @Accept json
// @Produce json
// @Param request body EmbedRequest true "Embedding request"
// @Success 200 {object} EmbedResponse "Success"
// @Router /ai/v1/embeddings [post]
func (h *GatewayHandlers) Embeddings(c *gin.Context) {
	var req EmbedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input is required"})
		return
	}

	resp, err := h.gateway.Embed(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListModels handles /v1/models
// @Summary List available models
// @Description List all available models across backends
// @Tags ai
// @Produce json
// @Success 200 {object} map[string]interface{} "Success"
// @Router /ai/v1/models [get]
func (h *GatewayHandlers) ListModels(c *gin.Context) {
	models, err := h.gateway.ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to OpenAI format
	data := make([]gin.H, len(models))
	for i, m := range models {
		data[i] = gin.H{
			"id":       m.Name,
			"object":   "model",
			"created":  m.ModifiedAt.Unix(),
			"owned_by": m.Details,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// GetModel handles /v1/models/:model
func (h *GatewayHandlers) GetModel(c *gin.Context) {
	modelName := c.Param("model")

	models, err := h.gateway.ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, m := range models {
		if m.Name == modelName {
			c.JSON(http.StatusOK, gin.H{
				"id":       m.Name,
				"object":   "model",
				"created":  m.ModifiedAt.Unix(),
				"owned_by": m.Details,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
}

// ==================== Gateway Management ====================

// GetGatewayStatus returns gateway status
func (h *GatewayHandlers) GetGatewayStatus(c *gin.Context) {
	status := gin.H{
		"healthy":  h.gateway.IsHealthy(),
		"backends": h.gateway.GetBackendStatus(c.Request.Context()),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// GetGatewayMetrics returns gateway metrics
func (h *GatewayHandlers) GetGatewayMetrics(c *gin.Context) {
	metrics := h.gateway.GetMetrics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// GetBackends returns backend status
func (h *GatewayHandlers) GetBackends(c *gin.Context) {
	status := h.gateway.GetBackendStatus(c.Request.Context())

	backends := make([]gin.H, 0, len(status))
	for name, s := range status {
		backends = append(backends, gin.H{
			"name":    name,
			"healthy": s.Healthy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    backends,
	})
}

// SetModelRoute sets model routing
func (h *GatewayHandlers) SetModelRoute(c *gin.Context) {
	var req struct {
		Model   string `json:"model" binding:"required"`
		Backend string `json:"backend" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	h.gateway.SetModelRouting(req.Model, BackendType(req.Backend))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "routing updated",
	})
}

// ==================== Model Management ====================

// ListLocalModels lists locally installed models
func (h *GatewayHandlers) ListLocalModels(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []interface{}{}})
		return
	}

	models := h.modelManager.ListModels()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    models,
	})
}

// GetLocalModel gets a local model
func (h *GatewayHandlers) GetLocalModel(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "model manager not available"})
		return
	}

	name := c.Param("name")
	model := h.modelManager.GetModel(name)
	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "model not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    model,
	})
}

// DownloadModel downloads a model
func (h *GatewayHandlers) DownloadModel(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "model manager not available"})
		return
	}

	var req ModelDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	progress, err := h.modelManager.DownloadModel(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "download started",
		"data":    progress,
	})
}

// DeleteModel deletes a model
func (h *GatewayHandlers) DeleteModel(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "model manager not available"})
		return
	}

	name := c.Param("name")
	if err := h.modelManager.DeleteModel(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "model deleted",
	})
}

// GetDownloadProgress gets download progress
func (h *GatewayHandlers) GetDownloadProgress(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "model manager not available"})
		return
	}

	name := c.Param("name")
	progress := h.modelManager.GetDownloadProgress(name)
	if progress == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "no download in progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    progress,
	})
}

// SearchModels searches for models
func (h *GatewayHandlers) SearchModels(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "model manager not available"})
		return
	}

	var req struct {
		Query  string `json:"query"`
		Source string `json:"source"` // ollama, huggingface, all
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if req.Source == "" {
		req.Source = "all"
	}

	results, err := h.modelManager.SearchModels(c.Request.Context(), req.Query, req.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    results,
	})
}

// GetStorageUsage gets storage usage
func (h *GatewayHandlers) GetStorageUsage(c *gin.Context) {
	if h.modelManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "model manager not available"})
		return
	}

	usage, err := h.modelManager.GetStorageUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalBytes": usage,
		},
	})
}

// ==================== Resource Monitoring ====================

// GetGPUInfo gets GPU information
func (h *GatewayHandlers) GetGPUInfo(c *gin.Context) {
	gpus, err := h.resourceMon.GetGPUInfo()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "no gpu available",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gpus,
	})
}

// GetMemoryInfo gets memory information
func (h *GatewayHandlers) GetMemoryInfo(c *gin.Context) {
	mem, err := h.resourceMon.GetMemoryInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    mem,
	})
}

// GetAllResources gets all resource information
func (h *GatewayHandlers) GetAllResources(c *gin.Context) {
	result := gin.H{}

	gpus, err := h.resourceMon.GetGPUInfo()
	if err == nil {
		result["gpu"] = gpus
	}

	mem, err := h.resourceMon.GetMemoryInfo()
	if err == nil {
		result["memory"] = mem
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// SSEStreamHandler handles SSE streaming responses
type SSEStreamHandler struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

// NewSSEStreamHandler creates a new SSE handler
func NewSSEStreamHandler(w http.ResponseWriter) *SSEStreamHandler {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	return &SSEStreamHandler{
		writer:  w,
		flusher: flusher,
	}
}

// WriteEvent writes an SSE event
func (h *SSEStreamHandler) WriteEvent(event string, data string) error {
	if _, err := io.WriteString(h.writer, "event: "+event+"\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(h.writer, "data: "+data+"\n\n"); err != nil {
		return err
	}
	h.flusher.Flush()
	return nil
}

// WriteData writes data-only SSE message
func (h *SSEStreamHandler) WriteData(data string) error {
	if _, err := io.WriteString(h.writer, "data: "+data+"\n\n"); err != nil {
		return err
	}
	h.flusher.Flush()
	return nil
}

// Close sends the [DONE] message
func (h *SSEStreamHandler) Close() {
	_, _ = io.WriteString(h.writer, "data: [DONE]\n\n")
	h.flusher.Flush()
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TrimModelName extracts model name from potential full path
func TrimModelName(model string) string {
	// Handle models like "llama2:7b" or "registry/model:tag"
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		model = model[idx+1:]
	}
	if idx := strings.Index(model, ":"); idx >= 0 {
		model = model[:idx]
	}
	return model
}
