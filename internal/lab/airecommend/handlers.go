package airecommend

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器.
type Handler struct {
	engine *Engine
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	recommend := rg.Group("/airecommend")
	{
		recommend.GET("/recommend/:user_id", h.GetRecommendations)
		recommend.POST("/record", h.AddAccessRecord)
		recommend.POST("/file", h.AddFile)
		recommend.POST("/user", h.AddUser)
		recommend.GET("/user/:user_id/profile", h.GetUserProfile)
		recommend.GET("/user/:user_id/history", h.GetAccessHistory)
		recommend.DELETE("/cache/:user_id", h.InvalidateCache)
		recommend.DELETE("/cache", h.InvalidateAllCache)
	}
}

// GetRecommendations 获取推荐.
func (h *Handler) GetRecommendations(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	recommendations := h.engine.GetRecommendations(userID, limit)
	if recommendations == nil {
		recommendations = []Recommendation{}
	}

	c.JSON(http.StatusOK, GetUserRecommendationsResponse{
		UserID:          userID,
		Recommendations: recommendations,
		CachedAt:        time.Now(),
		ExpiresAt:       time.Now().Add(h.engine.config.CacheTTL),
	})
}

// AddAccessRecord 添加访问记录.
func (h *Handler) AddAccessRecord(c *gin.Context) {
	var req AddAccessRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := &AccessRecord{
		UserID: req.UserID,
		FileID: req.FileID,
		Action: req.Action,
	}

	h.engine.AddAccessRecord(record)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// AddFile 添加文件.
func (h *Handler) AddFile(c *gin.Context) {
	var req AddFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	file := &FileItem{
		FileID:   req.FileID,
		Name:     req.Name,
		Path:     req.Path,
		Type:     req.Type,
		Size:     req.Size,
		Tags:     req.Tags,
		Metadata: req.Metadata,
	}

	h.engine.AddFile(file)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// AddUser 添加用户.
func (h *Handler) AddUser(c *gin.Context) {
	var req AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.engine.AddUser(req.UserID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetUserProfile 获取用户画像.
func (h *Handler) GetUserProfile(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	profile := h.engine.GetUserProfile(userID)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GetAccessHistory 获取访问历史.
func (h *Handler) GetAccessHistory(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	history := h.engine.GetAccessLog(userID, limit)
	if history == nil {
		history = []AccessRecord{}
	}

	c.JSON(http.StatusOK, gin.H{"user_id": userID, "history": history})
}

// InvalidateCache 使用户缓存失效.
func (h *Handler) InvalidateCache(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	h.engine.InvalidateCache(userID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// InvalidateAllCache 使所有缓存失效.
func (h *Handler) InvalidateAllCache(c *gin.Context) {
	h.engine.InvalidateAllCache()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
