package office

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// CollabHandlers 协作编辑 HTTP 处理器.
type CollabHandlers struct {
	engine *CollabEngine
}

// NewCollabHandlers 创建协作处理器.
func NewCollabHandlers(engine *CollabEngine) *CollabHandlers {
	return &CollabHandlers{engine: engine}
}

// RegisterCollabRoutes 注册协作编辑路由.
func (h *CollabHandlers) RegisterCollabRoutes(api *gin.RouterGroup) {
	collab := api.Group("/office/collaborate")
	{
		// 文档管理
		collab.POST("/documents", h.openDocument)
		collab.GET("/documents/:docId", h.getDocument)
		collab.DELETE("/documents/:docId", h.closeDocument)

		// OT 操作
		collab.POST("/documents/:docId/operations", h.applyOperation)

		// 版本管理
		collab.POST("/documents/:docId/versions", h.saveVersion)
		collab.GET("/documents/:docId/versions", h.getVersionHistory)
		collab.GET("/documents/:docId/versions/:version", h.getVersion)
		collab.POST("/documents/:docId/versions/:version/restore", h.restoreVersion)

		// 评论和批注
		collab.POST("/documents/:docId/comments", h.addComment)
		collab.GET("/documents/:docId/comments", h.getComments)
		collab.PUT("/documents/:docId/comments/:commentId/resolve", h.resolveComment)
		collab.POST("/documents/:docId/comments/:commentId/replies", h.replyComment)
		collab.DELETE("/documents/:docId/comments/:commentId", h.deleteComment)

		// 在线用户
		collab.GET("/documents/:docId/users", h.getOnlineUsers)

		// 统计
		collab.GET("/documents/:docId/stats", h.getStats)

		// WebSocket
		collab.GET("/ws/:docId", h.handleWebSocket)
	}
}

// upgrader WebSocket升级器.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发环境允许所有来源
	},
}

// ========== 文档管理 ==========

// openDocument 打开文档进行协作.
func (h *CollabHandlers) openDocument(c *gin.Context) {
	var req struct {
		DocID   string `json:"doc_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求: " + err.Error()})
		return
	}

	if req.DocID == "" {
		req.DocID = uuid.New().String()
	}

	doc := h.engine.OpenDocument(req.DocID, req.Content)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"doc_id":     doc.DocID,
			"version":    doc.Version,
			"content":    doc.Content,
			"created_at": doc.CreatedAt,
		},
	})
}

// getDocument 获取文档信息.
func (h *CollabHandlers) getDocument(c *gin.Context) {
	docID := c.Param("docId")

	doc, err := h.engine.GetDocument(docID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"doc_id":     doc.DocID,
			"content":    doc.Content,
			"version":    doc.Version,
			"created_at": doc.CreatedAt,
			"updated_at": doc.UpdatedAt,
		},
	})
}

// closeDocument 关闭文档.
func (h *CollabHandlers) closeDocument(c *gin.Context) {
	docID := c.Param("docId")

	if err := h.engine.CloseDocument(docID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "文档已关闭"})
}

// ========== OT 操作 ==========

// applyOperation 应用OT操作.
func (h *CollabHandlers) applyOperation(c *gin.Context) {
	docID := c.Param("docId")

	var op Operation
	if err := c.ShouldBindJSON(&op); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求: " + err.Error()})
		return
	}

	op.UserID = c.GetString("user_id")
	if op.UserID == "" {
		op.UserID = "anonymous"
	}

	result, err := h.engine.ApplyOperation(docID, &op)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	code := 0
	message := "success"
	if !result.Applied {
		code = 400
		message = result.Error
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    code,
		"message": message,
		"data":    result,
	})
}

// ========== 版本管理 ==========

// saveVersion 保存版本.
func (h *CollabHandlers) saveVersion(c *gin.Context) {
	docID := c.Param("docId")

	var req struct {
		Message string `json:"message"`
	}
	_ = c.ShouldBindJSON(&req)

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("username")
	if userName == "" {
		userName = "匿名用户"
	}

	snapshot, err := h.engine.SaveVersion(docID, userID, userName, req.Message)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "版本已保存",
		"data":    snapshot,
	})
}

// getVersionHistory 获取版本历史.
func (h *CollabHandlers) getVersionHistory(c *gin.Context) {
	docID := c.Param("docId")

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	versions, total, err := h.engine.GetVersionHistory(docID, limit, offset)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"versions": versions,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		},
	})
}

// getVersion 获取指定版本.
func (h *CollabHandlers) getVersion(c *gin.Context) {
	docID := c.Param("docId")
	versionStr := c.Param("version")

	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的版本号"})
		return
	}

	snapshot, err := h.engine.GetVersion(docID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    snapshot,
	})
}

// restoreVersion 恢复版本.
func (h *CollabHandlers) restoreVersion(c *gin.Context) {
	docID := c.Param("docId")
	versionStr := c.Param("version")

	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的版本号"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("username")
	if userName == "" {
		userName = "匿名用户"
	}

	if err := h.engine.RestoreVersion(docID, version, userID, userName); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "版本已恢复"})
}

// ========== 评论 ==========

// addComment 添加评论.
func (h *CollabHandlers) addComment(c *gin.Context) {
	docID := c.Param("docId")

	var req struct {
		Content string        `json:"content" binding:"required"`
		Range   *CommentRange `json:"range"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("username")
	if userName == "" {
		userName = "匿名用户"
	}

	comment, err := h.engine.AddComment(docID, userID, userName, req.Content, req.Range)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "评论已添加",
		"data":    comment,
	})
}

// getComments 获取评论.
func (h *CollabHandlers) getComments(c *gin.Context) {
	docID := c.Param("docId")

	comments, err := h.engine.GetComments(docID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    comments,
	})
}

// resolveComment 解决评论.
func (h *CollabHandlers) resolveComment(c *gin.Context) {
	docID := c.Param("docId")
	commentID := c.Param("commentId")

	if err := h.engine.ResolveComment(docID, commentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "评论已解决"})
}

// replyComment 回复评论.
func (h *CollabHandlers) replyComment(c *gin.Context) {
	docID := c.Param("docId")
	commentID := c.Param("commentId")

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("username")
	if userName == "" {
		userName = "匿名用户"
	}

	if err := h.engine.ReplyComment(docID, commentID, userID, userName, req.Content); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "回复已添加"})
}

// deleteComment 删除评论.
func (h *CollabHandlers) deleteComment(c *gin.Context) {
	docID := c.Param("docId")
	commentID := c.Param("commentId")

	if err := h.engine.DeleteComment(docID, commentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "评论已删除"})
}

// ========== 在线用户 ==========

// getOnlineUsers 获取在线用户.
func (h *CollabHandlers) getOnlineUsers(c *gin.Context) {
	docID := c.Param("docId")

	users := h.engine.GetOnlineUsers(docID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"users": users,
			"total": len(users),
		},
	})
}

// ========== 统计 ==========

// getStats 获取统计.
func (h *CollabHandlers) getStats(c *gin.Context) {
	docID := c.Param("docId")

	stats, err := h.engine.GetStats(docID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== WebSocket ==========

// handleWebSocket 处理WebSocket连接.
func (h *CollabHandlers) handleWebSocket(c *gin.Context) {
	docID := c.Param("docId")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		userID = "anonymous-" + uuid.New().String()[:8]
	}
	userName := c.Query("user_name")
	if userName == "" {
		userName = "匿名用户"
	}

	client := &WSClient{
		ID:         uuid.New().String(),
		UserID:     userID,
		UserName:   userName,
		DocID:      docID,
		Conn:       conn,
		RemoteAddr: c.ClientIP(),
		JoinedAt:   time.Now(),
		LastPing:   time.Now(),
	}

	// 确保文档存在
	h.engine.OpenDocument(docID, "")

	if err := h.engine.AddClient(client); err != nil {
		client.SendError(err.Error())
		conn.Close()
		return
	}

	// 发送同步数据
	doc, _ := h.engine.GetDocument(docID)
	if doc != nil {
		doc.mu.RLock()
		syncData := map[string]interface{}{
			"content": doc.Content,
			"version": doc.Version,
		}
		doc.mu.RUnlock()
		_ = client.SendMessage(WSTypeSync, syncData)
	}

	defer h.engine.RemoveClient(client.ID)

	// 读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			client.SendError("无效的消息格式")
			continue
		}

		h.handleWSMessage(client, &wsMsg)
	}
}

// handleWSMessage 处理WebSocket消息.
func (h *CollabHandlers) handleWSMessage(client *WSClient, msg *WSMessage) {
	switch msg.Type {
	case WSTypeOp:
		var op Operation
		if err := json.Unmarshal(msg.Payload, &op); err != nil {
			client.SendError("无效的操作格式")
			return
		}
		op.UserID = client.UserID
		result, err := h.engine.ApplyOperation(client.DocID, &op)
		if err != nil {
			client.SendError(err.Error())
			return
		}
		_ = client.SendMessage(WSTypeAck, result)

	case WSTypeCursor:
		// 广播光标位置
		var cursorData map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &cursorData); err == nil {
			cursorData["user_id"] = client.UserID
			cursorData["user_name"] = client.UserName
			for _, c := range h.engine.GetDocClients(client.DocID) {
				if c.ID != client.ID {
					_ = c.SendMessage(WSTypeCursor, cursorData)
				}
			}
		}

	case WSTypePing:
		client.mu.Lock()
		client.LastPing = time.Now()
		client.mu.Unlock()
		_ = client.SendMessage(WSTypePong, map[string]string{"status": "ok"})

	case WSTypeComment:
		var commentReq struct {
			Content string        `json:"content"`
			Range   *CommentRange `json:"range"`
		}
		if err := json.Unmarshal(msg.Payload, &commentReq); err != nil {
			client.SendError("无效的评论格式")
			return
		}
		_, _ = h.engine.AddComment(client.DocID, client.UserID, client.UserName, commentReq.Content, commentReq.Range)

	case WSTypeVersionSave:
		var versionReq struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Payload, &versionReq); err == nil {
			_, _ = h.engine.SaveVersion(client.DocID, client.UserID, client.UserName, versionReq.Message)
		}

	default:
		client.SendError("未知的消息类型: " + msg.Type)
	}
}
