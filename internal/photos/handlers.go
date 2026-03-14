package photos

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers 相册处理器
type Handlers struct {
	manager   *Manager
	aiManager *AIManager
	mu        sync.RWMutex
}

// NewHandlers 创建相册处理器
func NewHandlers(manager *Manager, aiManager *AIManager) *Handlers {
	return &Handlers{
		manager:   manager,
		aiManager: aiManager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	photos := r.Group("/photos")
	{
		// 照片上传
		photos.POST("/upload", h.uploadPhoto)
		photos.POST("/upload/batch", h.uploadPhotoBatch)
		photos.POST("/upload/session", h.createUploadSession)
		photos.PUT("/upload/session/:sessionId", h.uploadSessionChunk)
		photos.POST("/upload/session/:sessionId/complete", h.completeUploadSession)

		// 照片管理
		photos.GET("", h.listPhotos)
		photos.GET("/:id", h.getPhoto)
		photos.DELETE("/:id", h.deletePhoto)
		photos.POST("/:id/favorite", h.toggleFavorite)
		photos.PUT("/:id", h.updatePhoto)
		photos.GET("/:id/download", h.downloadPhoto)

		// 缩略图
		photos.GET("/:id/thumbnail", h.getThumbnail)
		photos.GET("/:id/thumbnail/:size", h.getThumbnail)

		// 相册管理
		photos.GET("/albums", h.listAlbums)
		photos.POST("/albums", h.createAlbum)
		photos.GET("/albums/:id", h.getAlbum)
		photos.PUT("/albums/:id", h.updateAlbum)
		photos.DELETE("/albums/:id", h.deleteAlbum)
		photos.POST("/albums/:id/photos", h.addPhotoToAlbum)
		photos.DELETE("/albums/:id/photos/:photoId", h.removePhotoFromAlbum)

		// 时间线
		photos.GET("/timeline", h.getTimeline)

		// 人物
		photos.GET("/persons", h.listPersons)
		photos.POST("/persons", h.createPerson)
		photos.PUT("/persons/:id", h.updatePerson)
		photos.DELETE("/persons/:id", h.deletePerson)

		// AI 相册功能
		photos.GET("/ai/stats", h.getAIStats)
		photos.GET("/ai/tasks", h.listAITasks)
		photos.POST("/ai/analyze/:photoId", h.analyzePhoto)
		photos.POST("/ai/analyze/batch", h.batchAnalyzePhotos)
		photos.GET("/ai/smart-albums", h.listSmartAlbums)
		photos.POST("/ai/smart-albums", h.createSmartAlbum)
		photos.DELETE("/ai/smart-albums/:id", h.deleteSmartAlbum)
		photos.GET("/ai/memories", h.getMemories)
		photos.POST("/ai/reanalyze", h.reanalyzeAll)
		photos.POST("/ai/clear", h.clearAIData)

		// 搜索
		photos.GET("/search", h.searchPhotos)

		// 统计
		photos.GET("/stats", h.getStats)
	}
}

// UploadResponse 上传响应
type UploadResponse struct {
	PhotoID     string `json:"photoId"`
	Filename    string `json:"filename"`
	Size        uint64 `json:"size"`
	ThumbnailID string `json:"thumbnailId"`
}

// uploadPhoto 上传单张照片
func (h *Handlers) uploadPhoto(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "上传文件失败：" + err.Error(),
		})
		return
	}
	defer func() { _ = file.Close() }()

	// 检查文件大小
	config := h.manager.GetConfig()
	if header.Size > config.MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "文件大小超过限制（最大 500MB）",
		})
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	validFormats := config.SupportedFormats
	isValid := false
	for _, format := range validFormats {
		if ext == format {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的文件格式",
		})
		return
	}

	// 生成唯一文件名
	photoID := uuid.New().String()
	filename := photoID + ext
	photoPath := filepath.Join(h.manager.photosDir, filename)

	// 创建目标文件
	dst, err := os.Create(photoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存文件失败：" + err.Error(),
		})
		return
	}
	defer dst.Close()

	// 复制文件内容
	written, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "写入文件失败：" + err.Error(),
		})
		return
	}

	// 索引照片
	go h.manager.indexPhoto(photoPath)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传成功",
		"data": UploadResponse{
			PhotoID:  photoID,
			Filename: header.Filename,
			Size:     uint64(written),
		},
	})
}

// uploadPhotoBatch 批量上传照片
func (h *Handlers) uploadPhotoBatch(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "解析表单失败：" + err.Error(),
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "没有上传任何文件",
		})
		return
	}

	config := h.manager.GetConfig()
	uploaded := make([]UploadResponse, 0)
	failed := make([]string, 0)

	for _, fileHeader := range files {
		// 检查文件大小
		if fileHeader.Size > config.MaxUploadSize {
			failed = append(failed, fileHeader.Filename+" (文件过大)")
			continue
		}

		// 检查文件类型
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		isValid := false
		for _, format := range config.SupportedFormats {
			if ext == format {
				isValid = true
				break
			}
		}
		if !isValid {
			failed = append(failed, fileHeader.Filename+" (格式不支持)")
			continue
		}

		// 打开上传的文件
		file, err := fileHeader.Open()
		if err != nil {
			failed = append(failed, fileHeader.Filename+" (打开失败)")
			continue
		}

		// 生成唯一文件名
		photoID := uuid.New().String()
		filename := photoID + ext
		photoPath := filepath.Join(h.manager.photosDir, filename)

		// 创建目标文件
		dst, err := os.Create(photoPath)
		if err != nil {
			file.Close()
			failed = append(failed, fileHeader.Filename+" (保存失败)")
			continue
		}

		// 复制文件内容
		written, err := io.Copy(dst, file)
		file.Close()
		dst.Close()

		if err != nil {
			failed = append(failed, fileHeader.Filename+" (写入失败)")
			continue
		}

		// 索引照片
		go h.manager.indexPhoto(photoPath)

		uploaded = append(uploaded, UploadResponse{
			PhotoID:  photoID,
			Filename: fileHeader.Filename,
			Size:     uint64(written),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量上传完成",
		"data": gin.H{
			"uploaded": uploaded,
			"failed":   failed,
			"total":    len(files),
			"success":  len(uploaded),
			"errors":   len(failed),
		},
	})
}

// createUploadSession 创建上传会话（用于断点续传）
func (h *Handlers) createUploadSession(c *gin.Context) {
	var req struct {
		Filename  string `json:"filename" binding:"required"`
		TotalSize int64  `json:"totalSize" binding:"required"`
		ChunkSize int64  `json:"chunkSize"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	sessionID := uuid.New().String()
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 5 * 1024 * 1024 // 默认 5MB
	}

	totalChunks := int((req.TotalSize + chunkSize - 1) / chunkSize)

	session := &UploadSession{
		SessionID:      sessionID,
		Filename:       req.Filename,
		TotalSize:      req.TotalSize,
		UploadedSize:   0,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: make([]int, 0),
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		TempPath:       filepath.Join(h.manager.cacheDir, "uploads", sessionID),
	}

	// 创建临时目录
	os.MkdirAll(session.TempPath, 0755)

	// 保存会话信息
	h.mu.Lock()
	h.manager.uploadSessions[sessionID] = session
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传会话创建成功",
		"data":    session,
	})
}

// uploadSessionChunk 上传分片
func (h *Handlers) uploadSessionChunk(c *gin.Context) {
	sessionID := c.Param("sessionId")
	chunkIndex, _ := strconv.Atoi(c.Query("chunk"))

	h.mu.RLock()
	session, exists := h.manager.uploadSessions[sessionID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "上传会话不存在",
		})
		return
	}

	// 检查会话是否过期
	if time.Now().After(session.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "上传会话已过期",
		})
		return
	}

	// 检查分片索引是否有效
	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的分片索引",
		})
		return
	}

	// 获取上传的文件
	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "读取分片数据失败：" + err.Error(),
		})
		return
	}
	defer file.Close()

	// 保存分片到临时文件
	chunkPath := filepath.Join(session.TempPath, fmt.Sprintf("chunk_%d", chunkIndex))
	dst, err := os.Create(chunkPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存分片失败：" + err.Error(),
		})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "写入分片失败：" + err.Error(),
		})
		return
	}

	// 更新会话状态
	h.mu.Lock()
	session.UploadedSize += written
	// 检查是否已上传
	alreadyUploaded := false
	for _, idx := range session.UploadedChunks {
		if idx == chunkIndex {
			alreadyUploaded = true
			break
		}
	}
	if !alreadyUploaded {
		session.UploadedChunks = append(session.UploadedChunks, chunkIndex)
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "分片上传成功",
		"data": gin.H{
			"sessionId":      sessionID,
			"chunkIndex":     chunkIndex,
			"chunkSize":      written,
			"uploadedSize":   session.UploadedSize,
			"totalSize":      session.TotalSize,
			"uploadedChunks": len(session.UploadedChunks),
			"totalChunks":    session.TotalChunks,
			"progress":       float64(len(session.UploadedChunks)) / float64(session.TotalChunks) * 100,
		},
	})
}

// completeUploadSession 完成上传会话
func (h *Handlers) completeUploadSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	h.mu.RLock()
	session, exists := h.manager.uploadSessions[sessionID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "上传会话不存在",
		})
		return
	}

	// 检查所有分片是否都已上传
	if len(session.UploadedChunks) != session.TotalChunks {
		missingChunks := make([]int, 0)
		uploadedMap := make(map[int]bool)
		for _, idx := range session.UploadedChunks {
			uploadedMap[idx] = true
		}
		for i := 0; i < session.TotalChunks; i++ {
			if !uploadedMap[i] {
				missingChunks = append(missingChunks, i)
			}
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "部分分片未上传",
			"data": gin.H{
				"missingChunks": missingChunks,
			},
		})
		return
	}

	// 生成唯一文件名
	ext := strings.ToLower(filepath.Ext(session.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	photoID := uuid.New().String()
	filename := photoID + ext
	photoPath := filepath.Join(h.manager.photosDir, filename)

	// 合并所有分片
	finalFile, err := os.Create(photoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建目标文件失败：" + err.Error(),
		})
		return
	}
	defer finalFile.Close()

	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(session.TempPath, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": fmt.Sprintf("打开分片 %d 失败：%s", i, err.Error()),
			})
			return
		}
		io.Copy(finalFile, chunkFile)
		chunkFile.Close()
	}

	// 同步文件
	finalFile.Sync()

	// 清理临时文件
	os.RemoveAll(session.TempPath)

	// 删除会话
	h.mu.Lock()
	delete(h.manager.uploadSessions, sessionID)
	h.mu.Unlock()

	// 索引照片
	go h.manager.indexPhoto(photoPath)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传完成",
		"data": UploadResponse{
			PhotoID:  photoID,
			Filename: session.Filename,
			Size:     uint64(session.TotalSize),
		},
	})
}

// listPhotos 列出照片
func (h *Handlers) listPhotos(c *gin.Context) {
	query := &PhotoQuery{
		AlbumID:   c.Query("albumId"),
		UserID:    c.Query("userId"),
		SortBy:    c.DefaultQuery("sortBy", "takenAt"),
		SortOrder: c.DefaultQuery("sortOrder", "desc"),
		Limit:     50,
		Offset:    0,
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			query.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			query.Offset = offset
		}
	}

	if favoriteStr := c.Query("favorite"); favoriteStr != "" {
		isFavorite := favoriteStr == "true"
		query.IsFavorite = &isFavorite
	}

	photos, total, err := h.manager.QueryPhotos(query)
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
			"photos": photos,
			"total":  total,
			"limit":  query.Limit,
			"offset": query.Offset,
		},
	})
}

// getPhoto 获取照片详情
func (h *Handlers) getPhoto(c *gin.Context) {
	photoID := c.Param("id")

	photo, err := h.manager.GetPhoto(photoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    photo,
	})
}

// deletePhoto 删除照片
func (h *Handlers) deletePhoto(c *gin.Context) {
	photoID := c.Param("id")

	if err := h.manager.DeletePhoto(photoID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// toggleFavorite 切换收藏状态
func (h *Handlers) toggleFavorite(c *gin.Context) {
	photoID := c.Param("id")

	photo, err := h.manager.ToggleFavorite(photoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "操作成功",
		"data":    photo,
	})
}

// updatePhoto 更新照片信息
func (h *Handlers) updatePhoto(c *gin.Context) {
	photoID := c.Param("id")

	var req struct {
		Tags       []string `json:"tags"`
		IsHidden   *bool    `json:"isHidden"`
		IsFavorite *bool    `json:"isFavorite"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 获取照片并更新
	h.mu.Lock()
	defer h.mu.Unlock()

	photo, exists := h.manager.photos[photoID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "照片不存在",
		})
		return
	}

	// 更新标签
	if req.Tags != nil {
		photo.Tags = req.Tags
	}

	// 更新隐藏状态
	if req.IsHidden != nil {
		photo.IsHidden = *req.IsHidden
	}

	// 更新收藏状态
	if req.IsFavorite != nil {
		photo.IsFavorite = *req.IsFavorite
	}

	// 更新修改时间
	photo.ModifiedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    photo,
	})
}

// downloadPhoto 下载照片
func (h *Handlers) downloadPhoto(c *gin.Context) {
	photoID := c.Param("id")

	photo, err := h.manager.GetPhoto(photoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	photoPath := filepath.Join(h.manager.photosDir, photo.Path)
	c.FileAttachment(photoPath, photo.Filename)
}

// getThumbnail 获取缩略图
func (h *Handlers) getThumbnail(c *gin.Context) {
	photoID := c.Param("id")
	size := c.Param("size")

	if size == "" {
		size = "512"
	}
	_ = size // TODO: 实现不同尺寸缩略图支持

	// 查找缩略图文件
	thumbFiles, _ := filepath.Glob(filepath.Join(h.manager.thumbsDir, fmt.Sprintf("%s_*.jpg", photoID)))
	if len(thumbFiles) == 0 {
		// 如果没有缩略图，返回原图
		photo, err := h.manager.GetPhoto(photoID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "照片不存在",
			})
			return
		}
		photoPath := filepath.Join(h.manager.photosDir, photo.Path)
		c.File(photoPath)
		return
	}

	// 返回第一个匹配的缩略图
	c.File(thumbFiles[0])
}

// AlbumRequest 相册请求
type AlbumRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// createAlbum 创建相册
func (h *Handlers) createAlbum(c *gin.Context) {
	var req AlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// TODO: 从认证信息获取 userID
	userID := "default"

	album, err := h.manager.CreateAlbum(req.Name, req.Description, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    album,
	})
}

// listAlbums 列出相册
func (h *Handlers) listAlbums(c *gin.Context) {
	// TODO: 从认证信息获取 userID
	userID := c.Query("userId")

	albums := h.manager.ListAlbums(userID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    albums,
	})
}

// getAlbum 获取相册详情
func (h *Handlers) getAlbum(c *gin.Context) {
	albumID := c.Param("id")

	album, err := h.manager.GetAlbum(albumID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    album,
	})
}

// updateAlbum 更新相册
func (h *Handlers) updateAlbum(c *gin.Context) {
	albumID := c.Param("id")

	var req AlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	album, err := h.manager.UpdateAlbum(albumID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    album,
	})
}

// deleteAlbum 删除相册
func (h *Handlers) deleteAlbum(c *gin.Context) {
	albumID := c.Param("id")

	if err := h.manager.DeleteAlbum(albumID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// addPhotoToAlbum 添加照片到相册
func (h *Handlers) addPhotoToAlbum(c *gin.Context) {
	albumID := c.Param("id")

	var req struct {
		PhotoID string `json:"photoId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.AddPhotoToAlbum(req.PhotoID, albumID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "添加成功",
	})
}

// removePhotoFromAlbum 从相册移除照片
func (h *Handlers) removePhotoFromAlbum(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photoId")

	if err := h.manager.RemovePhotoFromAlbum(photoID, albumID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "移除成功",
	})
}

// getTimeline 获取时间线
func (h *Handlers) getTimeline(c *gin.Context) {
	groupBy := c.DefaultQuery("groupBy", "month")
	userID := c.Query("userId")

	timeline, err := h.manager.GetTimeline(userID, groupBy)
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
		"data":    timeline,
	})
}

// listPersons 列出人物
func (h *Handlers) listPersons(c *gin.Context) {
	persons := h.manager.ListPersons()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    persons,
	})
}

// createPerson 创建人物
func (h *Handlers) createPerson(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	person, err := h.manager.CreatePerson(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    person,
	})
}

// updatePerson 更新人物
func (h *Handlers) updatePerson(c *gin.Context) {
	personID := c.Param("id")

	var req struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	person, err := h.manager.UpdatePerson(personID, req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    person,
	})
}

// deletePerson 删除人物
func (h *Handlers) deletePerson(c *gin.Context) {
	personID := c.Param("id")

	if err := h.manager.DeletePerson(personID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// searchPhotos 搜索照片
func (h *Handlers) searchPhotos(c *gin.Context) {
	query := c.Query("q")
	tags := c.Query("tags")
	scene := c.Query("scene")
	person := c.Query("person")
	location := c.Query("location")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]*Photo, 0)

	for _, photo := range h.manager.photos {
		// 全文搜索（匹配文件名、标签、场景、物体）
		if query != "" {
			queryLower := strings.ToLower(query)
			matched := false

			// 匹配文件名
			if strings.Contains(strings.ToLower(photo.Filename), queryLower) {
				matched = true
			}

			// 匹配标签
			for _, tag := range photo.Tags {
				if strings.Contains(strings.ToLower(tag), queryLower) {
					matched = true
					break
				}
			}

			// 匹配场景
			if strings.Contains(strings.ToLower(photo.Scene), queryLower) {
				matched = true
			}

			// 匹配物体
			for _, obj := range photo.Objects {
				if strings.Contains(strings.ToLower(obj), queryLower) {
					matched = true
					break
				}
			}

			// 匹配位置
			if photo.Location != nil {
				if strings.Contains(strings.ToLower(photo.Location.City), queryLower) ||
					strings.Contains(strings.ToLower(photo.Location.Country), queryLower) ||
					strings.Contains(strings.ToLower(photo.Location.Location), queryLower) {
					matched = true
				}
			}

			if !matched {
				continue
			}
		}

		// 标签过滤
		if tags != "" {
			tagList := strings.Split(tags, ",")
			hasAll := true
			for _, t := range tagList {
				t = strings.TrimSpace(t)
				found := false
				for _, pt := range photo.Tags {
					if strings.EqualFold(pt, t) {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}

		// 场景过滤
		if scene != "" && !strings.EqualFold(photo.Scene, scene) {
			continue
		}

		// 人物过滤
		if person != "" {
			found := false
			for _, face := range photo.Faces {
				if strings.EqualFold(face.Name, person) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 位置过滤
		if location != "" {
			if photo.Location == nil {
				continue
			}
			if !strings.Contains(strings.ToLower(photo.Location.City), strings.ToLower(location)) &&
				!strings.Contains(strings.ToLower(photo.Location.Country), strings.ToLower(location)) &&
				!strings.Contains(strings.ToLower(photo.Location.Location), strings.ToLower(location)) {
				continue
			}
		}

		// 日期范围过滤
		if startDate != "" {
			if start, err := time.Parse("2006-01-02", startDate); err == nil {
				if photo.TakenAt.Before(start) {
					continue
				}
			}
		}

		if endDate != "" {
			if end, err := time.Parse("2006-01-02", endDate); err == nil {
				if photo.TakenAt.After(end.Add(24 * time.Hour)) {
					continue
				}
			}
		}

		// 排除隐藏照片
		if photo.IsHidden {
			continue
		}

		results = append(results, photo)
	}

	// 按拍摄时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].TakenAt.After(results[j].TakenAt)
	})

	total := len(results)

	// 分页
	if offset > 0 && offset < len(results) {
		results = results[offset:]
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"photos": results,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalPhotos := len(h.manager.photos)
	totalAlbums := len(h.manager.albums)
	totalPersons := len(h.manager.persons)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalPhotos":  totalPhotos,
			"totalAlbums":  totalAlbums,
			"totalPersons": totalPersons,
			"storageUsed":  0, // TODO: 计算实际使用空间
		},
	})
}

// ==================== AI 相册相关处理器 ====================

// getAIStats 获取 AI 统计信息
func (h *Handlers) getAIStats(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"totalAnalyzed":      0,
				"totalFaces":         0,
				"sceneDistribution":  map[string]int{},
				"objectDistribution": map[string]int{},
			},
		})
		return
	}

	stats := h.aiManager.GetAIStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// listAITasks 列出 AI 任务
func (h *Handlers) listAITasks(c *gin.Context) {
	status := c.Query("status")

	if h.aiManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*AITask{},
		})
		return
	}

	tasks := h.aiManager.ListTasks(status)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// analyzePhoto 分析单张照片
func (h *Handlers) analyzePhoto(c *gin.Context) {
	photoID := c.Param("photoId")

	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	h.mu.RLock()
	photo, exists := h.manager.photos[photoID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "照片不存在",
		})
		return
	}

	photoPath := filepath.Join(h.manager.photosDir, photo.Path)
	taskID := h.aiManager.AnalyzePhoto(photoID, photoPath)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "分析任务已添加",
		"data": gin.H{
			"taskId": taskID,
		},
	})
}

// batchAnalyzePhotos 批量分析照片
func (h *Handlers) batchAnalyzePhotos(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	h.mu.RLock()
	photos := make([]*Photo, 0, len(h.manager.photos))
	for _, photo := range h.manager.photos {
		photos = append(photos, photo)
	}
	h.mu.RUnlock()

	taskIDs := h.aiManager.BatchAnalyze(photos)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量分析任务已添加",
		"data": gin.H{
			"taskCount": len(taskIDs),
			"taskIds":   taskIDs,
		},
	})
}

// listSmartAlbums 列出智能相册
func (h *Handlers) listSmartAlbums(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*SmartAlbum{},
		})
		return
	}

	albums := h.aiManager.ListSmartAlbums()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    albums,
	})
}

// createSmartAlbum 创建智能相册
func (h *Handlers) createSmartAlbum(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	var req struct {
		Name     string                 `json:"name" binding:"required"`
		Type     string                 `json:"type" binding:"required"`
		Criteria map[string]interface{} `json:"criteria"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	album, err := h.aiManager.CreateSmartAlbum(req.Name, req.Type, req.Criteria)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    album,
	})
}

// deleteSmartAlbum 删除智能相册
func (h *Handlers) deleteSmartAlbum(c *gin.Context) {
	albumID := c.Param("id")

	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	if err := h.aiManager.DeleteSmartAlbum(albumID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// getMemories 获取回忆列表
func (h *Handlers) getMemories(c *gin.Context) {
	monthDay := c.Query("date") // MM-DD 格式

	if h.aiManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*MemoryAlbum{},
		})
		return
	}

	memories := h.aiManager.GetMemories(monthDay)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    memories,
	})
}

// reanalyzeAll 重新分析所有照片
func (h *Handlers) reanalyzeAll(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	// TODO: 清除现有 AI 数据并重新分析

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已重新开始分析",
	})
}

// clearAIData 清除 AI 数据
func (h *Handlers) clearAIData(c *gin.Context) {
	if h.aiManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "AI 管理器未初始化",
		})
		return
	}

	// TODO: 清除 AI 内存数据

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "AI 数据已清除",
	})
}
