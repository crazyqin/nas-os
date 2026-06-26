// Package motionphoto - HTTP handlers
package motionphoto

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// Handler 动态照片 HTTP 处理器
type Handler struct {
	parser *Parser
}

// NewHandler 创建动态照片处理器
func NewHandler(parser *Parser) *Handler {
	return &Handler{parser: parser}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	mp := api.Group("/motionphoto")
	{
		mp.POST("/parse", h.handleParse)
		mp.POST("/extract", h.handleExtract)
		mp.POST("/parse-extract", h.handleParseAndExtract)
		mp.POST("/detect-vendor", h.handleDetectVendor)
		mp.POST("/convert-webp", h.handleConvertWebP)
		mp.GET("/status", h.handleStatus)
	}
}

// handleParse 解析动态照片
func (h *Handler) handleParse(c *gin.Context) {
	var req struct {
		FilePath string `json:"filePath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mp, err := h.parser.Parse(c.Request.Context(), req.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          mp.ID,
		"vendor":      mp.Vendor,
		"photoSize":   mp.PhotoSize,
		"videoSize":   mp.VideoSize,
		"videoOffset": mp.VideoOffset,
		"videoType":   mp.VideoType,
		"photoType":   mp.PhotoType,
		"width":       mp.Width,
		"height":      mp.Height,
		"duration":    mp.Duration,
		"createdAt":   mp.CreatedAt,
	})
}

// handleExtract 提取静态帧和视频
func (h *Handler) handleExtract(c *gin.Context) {
	var req struct {
		ID       string `json:"id" binding:"required"`
		FilePath string `json:"filePath" binding:"required"`
		Vendor   string `json:"vendor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendor := Vendor(req.Vendor)
	if vendor == "" {
		vendor = VendorUnknown
	}

	mp := &MotionPhoto{
		ID:       req.ID,
		FilePath: req.FilePath,
		Vendor:   vendor,
	}

	result, err := h.parser.Extract(c.Request.Context(), mp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"photoPath": result.PhotoPath,
		"videoPath": result.VideoPath,
		"webpPath":  result.WebPPath,
		"photoSize": result.PhotoSize,
		"videoSize": result.VideoSize,
		"webpSize":  result.WebPSize,
		"duration":  result.Duration.String(),
	})
}

// handleParseAndExtract 解析并提取
func (h *Handler) handleParseAndExtract(c *gin.Context) {
	var req struct {
		FilePath string `json:"filePath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mp, result, err := h.parser.ParseAndExtract(c.Request.Context(), req.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"motionPhoto": gin.H{
			"id":     mp.ID,
			"vendor": mp.Vendor,
		},
		"result": gin.H{
			"photoPath": result.PhotoPath,
			"videoPath": result.VideoPath,
			"webpPath":  result.WebPPath,
			"duration":  result.Duration.String(),
		},
	})
}

// handleDetectVendor 检测文件厂商类型
func (h *Handler) handleDetectVendor(c *gin.Context) {
	var req struct {
		FilePath string `json:"filePath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendor, err := DetectVendor(req.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filePath": req.FilePath,
		"vendor":   vendor,
	})
}

// handleConvertWebP 将静态帧转换为 WebP
func (h *Handler) handleConvertWebP(c *gin.Context) {
	var req struct {
		PhotoPath string   `json:"photoPath" binding:"required"`
		Quality   *float64 `json:"quality"`
		Lossless  *bool    `json:"lossless"`
		Width     *int     `json:"width"`
		Height    *int     `json:"height"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &WebPConfig{Quality: 85}
	if req.Quality != nil {
		config.Quality = *req.Quality
	}
	if req.Lossless != nil {
		config.Lossless = *req.Lossless
	}
	if req.Width != nil {
		config.Width = *req.Width
	}
	if req.Height != nil {
		config.Height = *req.Height
	}

	webpPath, err := h.parser.ConvertToWebP(c.Request.Context(), req.PhotoPath, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"webpPath":  webpPath,
		"extension": filepath.Ext(webpPath),
	})
}

// handleStatus 返回模块状态
func (h *Handler) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"module":    "motionphoto",
		"supported": []string{string(VendorHuawei), string(VendorXiaomi), string(VendorOPPO), string(VendorSamsung)},
		"features": gin.H{
			"parse":    true,
			"extract":  true,
			"webpConv": h.parser.config.EnableWebP,
		},
	})
}
