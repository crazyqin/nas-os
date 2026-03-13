package compress

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 压缩 API 处理器
type Handlers struct {
	manager *Manager
	fs      *FileSystem
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager, fs *FileSystem) *Handlers {
	return &Handlers{
		manager: manager,
		fs:      fs,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	compress := api.Group("/compress")
	{
		compress.GET("/config", h.GetConfig)
		compress.PUT("/config", h.UpdateConfig)
		compress.GET("/stats", h.GetStats)
		compress.POST("/stats/reset", h.ResetStats)
		compress.GET("/files", h.ListCompressedFiles)
		compress.POST("/batch", h.BatchCompress)
		compress.POST("/compress", h.CompressFile)
		compress.POST("/decompress", h.DecompressFile)
		compress.GET("/algorithms", h.ListAlgorithms)
	}
}

// GetConfig 获取配置
// @Summary 获取压缩配置
// @Description 获取压缩服务配置
// @Tags compress
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/config [get]
func (h *Handlers) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.manager.GetConfig(),
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled           bool      `json:"enabled"`
	DefaultAlgorithm  Algorithm `json:"default_algorithm"`
	CompressionLevel  int       `json:"compression_level"`
	MinSize           int64     `json:"min_size"`
	ExcludeExtensions []string  `json:"exclude_extensions"`
	ExcludeDirs       []string  `json:"exclude_dirs"`
	IncludeDirs       []string  `json:"include_dirs"`
	CompressOnWrite   bool      `json:"compress_on_write"`
	DecompressOnRead  bool      `json:"decompress_on_read"`
	StatsEnabled      bool      `json:"stats_enabled"`
}

// UpdateConfig 更新配置
// @Summary 更新压缩配置
// @Description 更新压缩服务配置
// @Tags compress
// @Accept json
// @Produce json
// @Param config body UpdateConfigRequest true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/config [put]
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	config := &Config{
		Enabled:           req.Enabled,
		DefaultAlgorithm:  req.DefaultAlgorithm,
		CompressionLevel:  req.CompressionLevel,
		MinSize:           req.MinSize,
		ExcludeExtensions: req.ExcludeExtensions,
		ExcludeDirs:       req.ExcludeDirs,
		IncludeDirs:       req.IncludeDirs,
		CompressOnWrite:   req.CompressOnWrite,
		DecompressOnRead:  req.DecompressOnRead,
		StatsEnabled:      req.StatsEnabled,
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.manager.GetConfig(),
	})
}

// GetStats 获取统计
// @Summary 获取压缩统计
// @Description 获取压缩统计信息
// @Tags compress
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/stats [get]
func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.manager.GetStats(),
	})
}

// ResetStats 重置统计
// @Summary 重置压缩统计
// @Description 重置压缩统计信息
// @Tags compress
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/stats/reset [post]
func (h *Handlers) ResetStats(c *gin.Context) {
	h.manager.GetStats().Reset()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "统计已重置",
	})
}

// ListCompressedFilesRequest 列表请求
type ListCompressedFilesRequest struct {
	Dir string `form:"dir" binding:"required"`
}

// ListCompressedFiles 列出压缩文件
// @Summary 列出压缩文件
// @Description 列出指定目录下的压缩文件
// @Tags compress
// @Accept json
// @Produce json
// @Param dir query string true "目录路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/files [get]
func (h *Handlers) ListCompressedFiles(c *gin.Context) {
	dir := c.Query("dir")
	if dir == "" {
		dir = "/"
	}

	files, err := h.fs.GetCompressedFiles(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"files": files,
			"count": len(files),
		},
	})
}

// BatchCompressRequest 批量压缩请求
type BatchCompressRequest struct {
	Dir       string `json:"dir" binding:"required"`
	Recursive bool   `json:"recursive"`
}

// BatchCompress 批量压缩
// @Summary 批量压缩
// @Description 批量压缩指定目录下的文件
// @Tags compress
// @Accept json
// @Produce json
// @Param request body BatchCompressRequest true "请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/batch [post]
func (h *Handlers) BatchCompress(c *gin.Context) {
	var req BatchCompressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	result, err := h.fs.BatchCompress(req.Dir, req.Recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// CompressFileRequest 压缩文件请求
type CompressFileRequest struct {
	SrcPath string `json:"src_path" binding:"required"`
	DstPath string `json:"dst_path"`
}

// CompressFile 压缩单个文件
// @Summary 压缩单个文件
// @Description 压缩指定的文件
// @Tags compress
// @Accept json
// @Produce json
// @Param request body CompressFileRequest true "请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/compress [post]
func (h *Handlers) CompressFile(c *gin.Context) {
	var req CompressFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	dstPath := req.DstPath
	if dstPath == "" {
		dstPath = req.SrcPath
	}

	result, err := h.manager.CompressFile(req.SrcPath, dstPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// DecompressFileRequest 解压文件请求
type DecompressFileRequest struct {
	SrcPath string `json:"src_path" binding:"required"`
	DstPath string `json:"dst_path" binding:"required"`
}

// DecompressFile 解压单个文件
// @Summary 解压单个文件
// @Description 解压指定的文件
// @Tags compress
// @Accept json
// @Produce json
// @Param request body DecompressFileRequest true "请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/decompress [post]
func (h *Handlers) DecompressFile(c *gin.Context) {
	var req DecompressFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.DecompressFile(req.SrcPath, req.DstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "文件已解压",
	})
}

// AlgorithmInfo 算法信息
type AlgorithmInfo struct {
	Name        Algorithm `json:"name"`
	Extension   string    `json:"extension"`
	Description string    `json:"description"`
	Speed       string    `json:"speed"`
	Ratio       string    `json:"ratio"`
}

// ListAlgorithms 列出可用算法
// @Summary 列出可用算法
// @Description 列出所有可用的压缩算法
// @Tags compress
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /compress/algorithms [get]
func (h *Handlers) ListAlgorithms(c *gin.Context) {
	algorithms := []AlgorithmInfo{
		{
			Name:        AlgorithmZstd,
			Extension:   ".zst",
			Description: "Zstandard 压缩算法",
			Speed:       "快",
			Ratio:       "高",
		},
		{
			Name:        AlgorithmLz4,
			Extension:   ".lz4",
			Description: "LZ4 压缩算法",
			Speed:       "极快",
			Ratio:       "中",
		},
		{
			Name:        AlgorithmGzip,
			Extension:   ".gz",
			Description: "Gzip 压缩算法",
			Speed:       "中",
			Ratio:       "中",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    algorithms,
	})
}

// GetStatus 获取服务状态
func (h *Handlers) GetStatus(c *gin.Context) {
	config := h.manager.GetConfig()
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled":     config.Enabled,
			"algorithm":   config.DefaultAlgorithm,
			"total_files": stats.TotalFiles,
			"saved_bytes": stats.SavedBytes,
			"avg_ratio":   stats.AvgRatio,
		},
	})
}
