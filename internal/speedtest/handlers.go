package speedtest

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 速度测试 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建新的速度测试处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册速度测试路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	speedtest := rg.Group("/speedtest")
	{
		speedtest.POST("/network", h.runNetworkTest)
		speedtest.POST("/disk", h.runDiskTest)
		speedtest.POST("/full", h.runFullTest)
		speedtest.GET("/history", h.getHistory)
		speedtest.DELETE("/history", h.clearHistory)
		speedtest.GET("/latest", h.getLatest)
	}
}

func (h *Handlers) runNetworkTest(c *gin.Context) {
	result, err := h.manager.RunNetworkTest()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) runDiskTest(c *gin.Context) {
	var req struct {
		TargetPath string `json:"target_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if req.TargetPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "target_path 不能为空"})
		return
	}

	result, err := h.manager.RunDiskTest(req.TargetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) runFullTest(c *gin.Context) {
	var req struct {
		TargetPath string `json:"target_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if req.TargetPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "target_path 不能为空"})
		return
	}

	result, err := h.manager.RunFullTest(req.TargetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 10 // 默认返回 10 条
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history := h.manager.GetHistory(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": history})
}

func (h *Handlers) clearHistory(c *gin.Context) {
	h.manager.ClearHistory()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "历史记录已清空"})
}

func (h *Handlers) getLatest(c *gin.Context) {
	result := h.manager.GetLatestResult()
	if result == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil, "message": "暂无测试记录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}
