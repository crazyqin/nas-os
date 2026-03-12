package photos

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers 相册处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建相册处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
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
	defer file.Close()

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

	// TODO: 保存会话信息到内存或数据库

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

	// TODO: 实现分片上传逻辑

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "分片上传成功",
		"data": gin.H{
			"sessionId":  sessionID,
			"chunkIndex": chunkIndex,
		},
	})
}

// completeUploadSession 完成上传会话
func (h *Handlers) completeUploadSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// TODO: 合并分片并完成上传

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传完成",
		"data": gin.H{
			"sessionId": sessionID,
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
	_ = c.Param("id")

	var req struct {
		Tags     []string `json:"tags"`
		IsHidden *bool    `json:"isHidden"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// TODO: 实现更新逻辑

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
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
	_ = c.Query("q")
	
	// TODO: 实现搜索逻辑

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    []Photo{},
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	// TODO: 实现统计逻辑

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalPhotos":   0,
			"totalAlbums":   0,
			"totalPersons":  0,
			"storageUsed":   0,
		},
	})
}
