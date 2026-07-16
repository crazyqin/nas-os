package nascopilot

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers NAS Copilot HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器实例.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	copilot := r.Group("/copilot")
	{
		copilot.POST("/chat", h.handleChat)
		copilot.GET("/conversations", h.handleListConversations)
		copilot.GET("/conversations/:id", h.handleGetConversation)
		copilot.DELETE("/conversations/:id", h.handleDeleteConversation)
		copilot.POST("/parse", h.handleParse)
		copilot.POST("/execute", h.handleExecute)
		copilot.POST("/knowledge", h.handleAddKnowledge)
		copilot.GET("/knowledge", h.handleListKnowledge)
		copilot.GET("/knowledge/search", h.handleSearchKnowledge)
		copilot.POST("/tasks", h.handleCreateTask)
		copilot.GET("/tasks", h.handleListTasks)
		copilot.PUT("/tasks/:id", h.handleUpdateTask)
		copilot.DELETE("/tasks/:id", h.handleDeleteTask)
		copilot.GET("/stats", h.handleGetStats)
		copilot.GET("/audit", h.handleListAudit)
	}
}

// handleChat 发送聊天消息.
func (h *Handlers) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果没有对话 ID，自动创建
	conversationID := req.ConversationID
	if conversationID == "" {
		conv := h.manager.CreateConversation(req.UserID, "新对话")
		conversationID = conv.ID
	}

	resp, err := h.manager.SendMessage(conversationID, req.Message, req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleListConversations 获取对话列表.
func (h *Handlers) handleListConversations(c *gin.Context) {
	userID := c.Query("userId")
	conversations := h.manager.ListConversations(userID)
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

// handleGetConversation 获取对话详情.
func (h *Handlers) handleGetConversation(c *gin.Context) {
	id := c.Param("id")
	conv, messages, err := h.manager.GetConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"conversation": conv,
		"messages":     messages,
	})
}

// handleDeleteConversation 删除对话.
func (h *Handlers) handleDeleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteConversation(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "对话已删除"})
}

// handleParse 解析意图.
func (h *Handlers) handleParse(c *gin.Context) {
	var req ParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	intent := h.manager.ParseIntent(req.Text)
	c.JSON(http.StatusOK, ParseResponse{Intent: *intent})
}

// handleExecute 执行命令.
func (h *Handlers) handleExecute(c *gin.Context) {
	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.manager.ExecuteCommand(req.Command)
	c.JSON(http.StatusOK, gin.H{"result": result})
}

// handleAddKnowledge 添加知识条目.
func (h *Handlers) handleAddKnowledge(c *gin.Context) {
	var req AddKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry := h.manager.AddKnowledge(KnowledgeEntry{
		Category: req.Category,
		Title:    req.Title,
		Content:  req.Content,
		Tags:     req.Tags,
	})
	c.JSON(http.StatusCreated, entry)
}

// handleListKnowledge 列出知识条目.
func (h *Handlers) handleListKnowledge(c *gin.Context) {
	entries := h.manager.ListKnowledge()
	c.JSON(http.StatusOK, gin.H{"knowledge": entries})
}

// handleSearchKnowledge 搜索知识库.
func (h *Handlers) handleSearchKnowledge(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}
	results := h.manager.SearchKnowledge(query)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// handleCreateTask 创建定时任务.
func (h *Handlers) handleCreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	task := h.manager.CreateScheduledTask(req.Description, req.CronExpr, req.Command, enabled)
	c.JSON(http.StatusCreated, task)
}

// handleListTasks 列出定时任务.
func (h *Handlers) handleListTasks(c *gin.Context) {
	tasks := h.manager.ListScheduledTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// handleUpdateTask 更新定时任务.
func (h *Handlers) handleUpdateTask(c *gin.Context) {
	id := c.Param("id")
	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.UpdateScheduledTask(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// handleDeleteTask 删除定时任务.
func (h *Handlers) handleDeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteScheduledTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// handleGetStats 获取统计.
func (h *Handlers) handleGetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// handleListAudit 获取审计日志.
func (h *Handlers) handleListAudit(c *gin.Context) {
	entries := h.manager.ListAuditEntries()
	c.JSON(http.StatusOK, gin.H{"audit": entries})
}
