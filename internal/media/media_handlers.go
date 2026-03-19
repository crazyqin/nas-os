package media

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ExtendedHandlers 扩展媒体处理器
type ExtendedHandlers struct {
	libraryMgr   *LibraryManager
	transcoder   *Transcoder
	subtitleMgr  *SubtitleManager
	thumbnailGen *ThumbnailGenerator
	streamServer *StreamServer
}

// NewExtendedHandlers 创建扩展媒体处理器
func NewExtendedHandlers(
	libraryMgr *LibraryManager,
	transcoder *Transcoder,
	subtitleMgr *SubtitleManager,
	thumbnailGen *ThumbnailGenerator,
	streamServer *StreamServer,
) *ExtendedHandlers {
	return &ExtendedHandlers{
		libraryMgr:   libraryMgr,
		transcoder:   transcoder,
		subtitleMgr:  subtitleMgr,
		thumbnailGen: thumbnailGen,
		streamServer: streamServer,
	}
}

// RegisterExtendedRoutes 注册扩展路由
func (h *ExtendedHandlers) RegisterExtendedRoutes(r *gin.RouterGroup) {
	media := r.Group("/media")
	{
		// 转码相关
		media.POST("/transcode", h.createTranscodeJob)
		media.GET("/transcode/:id", h.getTranscodeJob)
		media.DELETE("/transcode/:id", h.deleteTranscodeJob)
		media.POST("/transcode/:id/cancel", h.cancelTranscodeJob)
		media.GET("/transcode", h.listTranscodeJobs)
		media.POST("/transcode/quick", h.quickConvert)
		media.GET("/video/info", h.getVideoInfo)

		// 字幕相关
		media.POST("/subtitle/parse", h.parseSubtitle)
		media.POST("/subtitle/convert", h.convertSubtitle)
		media.POST("/subtitle/merge", h.mergeSubtitles)
		media.POST("/subtitle/shift", h.shiftSubtitleTime)
		media.POST("/subtitle/extract", h.extractSubtitle)

		// 缩略图相关
		media.POST("/thumbnail/video", h.generateThumbnail)
		media.POST("/thumbnail/multiple", h.generateMultipleThumbnails)
		media.POST("/thumbnail/sprite", h.generateSprite)
		media.POST("/thumbnail/gif", h.generatePreviewGif)
		media.POST("/thumbnail/image", h.generateImageThumbnail)
		media.POST("/thumbnail/batch", h.batchGenerateThumbnails)

		// 流媒体相关
		media.POST("/stream/hls", h.createHLSSession)
		media.POST("/stream/dash", h.createDASHSession)
		media.POST("/stream/adaptive", h.createAdaptiveStream)
		media.GET("/stream/:id", h.getStreamSession)
		media.DELETE("/stream/:id", h.deleteStreamSession)
		media.GET("/stream", h.listStreamSessions)
		media.GET("/stream/:id/manifest", h.getStreamManifest)
		media.GET("/play/:id", h.streamMediaFile)
	}
}

// === 转码相关 ===

// createTranscodeJob 创建转码任务
func (h *ExtendedHandlers) createTranscodeJob(c *gin.Context) {
	var req struct {
		InputPath  string          `json:"inputPath" binding:"required"`
		OutputPath string          `json:"outputPath" binding:"required"`
		Config     TranscodeConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 如果没有配置，使用默认配置
	if req.Config.VideoCodec == "" {
		req.Config = DefaultTranscodeConfig()
	}

	job, err := h.transcoder.CreateJob(req.InputPath, req.OutputPath, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 自动启动任务
	if err := h.transcoder.StartJob(job.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "转码任务已创建",
		"data":    job,
	})
}

// getTranscodeJob 获取转码任务状态
func (h *ExtendedHandlers) getTranscodeJob(c *gin.Context) {
	id := c.Param("id")

	job := h.transcoder.GetJob(id)
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    job,
	})
}

// deleteTranscodeJob 删除转码任务
func (h *ExtendedHandlers) deleteTranscodeJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.transcoder.DeleteJob(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// cancelTranscodeJob 取消转码任务
func (h *ExtendedHandlers) cancelTranscodeJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.transcoder.CancelJob(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已取消",
	})
}

// listTranscodeJobs 列出转码任务
func (h *ExtendedHandlers) listTranscodeJobs(c *gin.Context) {
	jobs := h.transcoder.ListJobs()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    jobs,
	})
}

// quickConvert 快速转换
func (h *ExtendedHandlers) quickConvert(c *gin.Context) {
	var req struct {
		InputPath  string `json:"inputPath" binding:"required"`
		OutputPath string `json:"outputPath" binding:"required"`
		Format     string `json:"format" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	job, err := h.transcoder.QuickConvert(req.InputPath, req.OutputPath, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 自动启动
	if err := h.transcoder.StartJob(job.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "快速转换任务已创建",
		"data":    job,
	})
}

// getVideoInfo 获取视频信息
func (h *ExtendedHandlers) getVideoInfo(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供视频路径",
		})
		return
	}

	info, err := h.transcoder.GetVideoInfo(path)
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
		"data":    info,
	})
}

// === 字幕相关 ===

// parseSubtitle 解析字幕文件
func (h *ExtendedHandlers) parseSubtitle(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供字幕文件路径",
		})
		return
	}

	subtitle, err := h.subtitleMgr.ParseSubtitle(path)
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
		"data":    subtitle,
	})
}

// convertSubtitle 转换字幕格式
func (h *ExtendedHandlers) convertSubtitle(c *gin.Context) {
	var req struct {
		InputPath  string `json:"inputPath" binding:"required"`
		OutputPath string `json:"outputPath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.subtitleMgr.ConvertSubtitle(req.InputPath, req.OutputPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "转换成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// mergeSubtitles 合并字幕文件
func (h *ExtendedHandlers) mergeSubtitles(c *gin.Context) {
	var req struct {
		Paths      []string `json:"paths" binding:"required"`
		OutputPath string   `json:"outputPath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供字幕文件列表",
		})
		return
	}

	if err := h.subtitleMgr.MergeSubtitles(req.Paths, req.OutputPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "合并成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// shiftSubtitleTime 时间偏移
func (h *ExtendedHandlers) shiftSubtitleTime(c *gin.Context) {
	var req struct {
		Path   string `json:"path" binding:"required"`
		Offset int    `json:"offset"` // 毫秒
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	subtitle, err := h.subtitleMgr.ParseSubtitle(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	h.subtitleMgr.ShiftTime(subtitle, time.Duration(req.Offset)*time.Millisecond)

	// 保存修改
	if err := h.subtitleMgr.SaveSubtitle(subtitle, req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "时间偏移已应用",
	})
}

// extractSubtitle 提取字幕
func (h *ExtendedHandlers) extractSubtitle(c *gin.Context) {
	var req struct {
		MKVPath     string `json:"mkvPath" binding:"required"`
		OutputPath  string `json:"outputPath" binding:"required"`
		StreamIndex int    `json:"streamIndex"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.subtitleMgr.ExtractSubtitleFromMKV(req.MKVPath, req.OutputPath, req.StreamIndex); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "字幕提取成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// === 缩略图相关 ===

// generateThumbnail 生成缩略图
func (h *ExtendedHandlers) generateThumbnail(c *gin.Context) {
	var req struct {
		VideoPath  string          `json:"videoPath" binding:"required"`
		OutputPath string          `json:"outputPath" binding:"required"`
		Config     ThumbnailConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 使用默认配置填充
	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}

	if err := h.thumbnailGen.GenerateFromVideo(req.VideoPath, req.OutputPath, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "缩略图生成成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// generateMultipleThumbnails 生成多个缩略图
func (h *ExtendedHandlers) generateMultipleThumbnails(c *gin.Context) {
	var req struct {
		VideoPath string          `json:"videoPath" binding:"required"`
		OutputDir string          `json:"outputDir" binding:"required"`
		Count     int             `json:"count"`
		Config    ThumbnailConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Count <= 0 {
		req.Count = 10
	}
	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}

	paths, err := h.thumbnailGen.GenerateMultiple(req.VideoPath, req.OutputDir, req.Count, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "缩略图生成成功",
		"data": gin.H{
			"paths": paths,
			"total": len(paths),
		},
	})
}

// generateSprite 生成精灵图
func (h *ExtendedHandlers) generateSprite(c *gin.Context) {
	var req struct {
		VideoPath  string          `json:"videoPath" binding:"required"`
		OutputPath string          `json:"outputPath" binding:"required"`
		Cols       int             `json:"cols"`
		Rows       int             `json:"rows"`
		Config     ThumbnailConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Cols <= 0 {
		req.Cols = 5
	}
	if req.Rows <= 0 {
		req.Rows = 5
	}
	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}

	info, err := h.thumbnailGen.GenerateSprite(req.VideoPath, req.OutputPath, req.Cols, req.Rows, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "精灵图生成成功",
		"data":    info,
	})
}

// generatePreviewGif 生成预览 GIF
func (h *ExtendedHandlers) generatePreviewGif(c *gin.Context) {
	var req struct {
		VideoPath  string          `json:"videoPath" binding:"required"`
		OutputPath string          `json:"outputPath" binding:"required"`
		Duration   float64         `json:"duration"`
		Config     ThumbnailConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}

	if err := h.thumbnailGen.GeneratePreviewGif(req.VideoPath, req.OutputPath, req.Duration, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "GIF 生成成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// generateImageThumbnail 生成图片缩略图
func (h *ExtendedHandlers) generateImageThumbnail(c *gin.Context) {
	var req struct {
		InputPath  string          `json:"inputPath" binding:"required"`
		OutputPath string          `json:"outputPath" binding:"required"`
		Config     ThumbnailConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}

	if err := h.thumbnailGen.GenerateFromImage(req.InputPath, req.OutputPath, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "图片缩略图生成成功",
		"data": gin.H{
			"outputPath": req.OutputPath,
		},
	})
}

// batchGenerateThumbnails 批量生成缩略图
func (h *ExtendedHandlers) batchGenerateThumbnails(c *gin.Context) {
	var req struct {
		Videos      []string        `json:"videos" binding:"required"`
		OutputDir   string          `json:"outputDir" binding:"required"`
		Config      ThumbnailConfig `json:"config"`
		Concurrency int             `json:"concurrency"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Config.Width == 0 {
		req.Config = DefaultThumbnailConfig()
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 2
	}

	results := h.thumbnailGen.BatchGenerate(req.Videos, req.OutputDir, req.Config, req.Concurrency)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量生成完成",
		"data":    results,
	})
}

// === 流媒体相关 ===

// createHLSSession 创建 HLS 会话
func (h *ExtendedHandlers) createHLSSession(c *gin.Context) {
	var req struct {
		SourcePath string `json:"sourcePath" binding:"required"`
		OutputDir  string `json:"outputDir" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	session, err := h.streamServer.CreateHLSSession(req.SourcePath, req.OutputDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "HLS 会话已创建",
		"data":    session,
	})
}

// createDASHSession 创建 DASH 会话
func (h *ExtendedHandlers) createDASHSession(c *gin.Context) {
	var req struct {
		SourcePath string `json:"sourcePath" binding:"required"`
		OutputDir  string `json:"outputDir" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	session, err := h.streamServer.CreateDASHSession(req.SourcePath, req.OutputDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "DASH 会话已创建",
		"data":    session,
	})
}

// createAdaptiveStream 创建自适应流
func (h *ExtendedHandlers) createAdaptiveStream(c *gin.Context) {
	var req struct {
		SourcePath string           `json:"sourcePath" binding:"required"`
		OutputDir  string           `json:"outputDir" binding:"required"`
		Qualities  []AdaptiveStream `json:"qualities"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	session, err := h.streamServer.CreateAdaptiveHLS(req.SourcePath, req.OutputDir, req.Qualities)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "自适应流会话已创建",
		"data":    session,
	})
}

// getStreamSession 获取流会话
func (h *ExtendedHandlers) getStreamSession(c *gin.Context) {
	id := c.Param("id")

	session := h.streamServer.GetSession(id)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "会话不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    session,
	})
}

// deleteStreamSession 删除流会话
func (h *ExtendedHandlers) deleteStreamSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.streamServer.StopSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	if err := h.streamServer.DeleteSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "会话已删除",
	})
}

// listStreamSessions 列出流会话
func (h *ExtendedHandlers) listStreamSessions(c *gin.Context) {
	sessions := h.streamServer.ListSessions()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    sessions,
	})
}

// getStreamManifest 获取流播放列表
func (h *ExtendedHandlers) getStreamManifest(c *gin.Context) {
	id := c.Param("id")

	session := h.streamServer.GetSession(id)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "会话不存在",
		})
		return
	}

	// 返回播放列表 URL
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"manifestUrl": session.ManifestURL,
			"type":        session.Type,
		},
	})
}

// streamMediaFile 流式播放媒体文件
func (h *ExtendedHandlers) streamMediaFile(c *gin.Context) {
	// 通过 ID 获取媒体项
	id := c.Param("id")

	item, _ := h.libraryMgr.GetItemByID(id)
	if item == nil {
		// 尝试直接使用路径
		path := c.Query("path")
		if path == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "媒体文件不存在",
			})
			return
		}

		if err := h.streamServer.StreamFile(c.Writer, c.Request, path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": err.Error(),
			})
		}
		return
	}

	if err := h.streamServer.StreamFile(c.Writer, c.Request, item.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
	}
}

// GetTranscoder 获取转码器
func (h *ExtendedHandlers) GetTranscoder() *Transcoder {
	return h.transcoder
}

// GetSubtitleManager 获取字幕管理器
func (h *ExtendedHandlers) GetSubtitleManager() *SubtitleManager {
	return h.subtitleMgr
}

// GetThumbnailGenerator 获取缩略图生成器
func (h *ExtendedHandlers) GetThumbnailGenerator() *ThumbnailGenerator {
	return h.thumbnailGen
}

// GetStreamServer 获取流媒体服务器
func (h *ExtendedHandlers) GetStreamServer() *StreamServer {
	return h.streamServer
}
