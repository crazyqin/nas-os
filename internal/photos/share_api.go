// Package photos 提供照片分享功能
// 参考 Synology Photos 的分享功能设计
package photos

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ShareAPI 照片分享API
type ShareAPI struct {
	manager    *Manager
	shareStore *ShareStore
}

// NewShareAPI 创建分享API
func NewShareAPI(manager *Manager) *ShareAPI {
	return &ShareAPI{
		manager:    manager,
		shareStore: NewShareStore(),
	}
}

// RegisterRoutes 注册路由
func (api *ShareAPI) RegisterRoutes(r *gin.RouterGroup) {
	share := r.Group("/share")
	{
		// 创建分享链接
		share.POST("/photos", api.SharePhotos)
		share.POST("/album/:id", api.ShareAlbum)

		// 获取分享内容（公开访问）
		share.GET("/p/:code", api.GetPublicShare)
		share.GET("/p/:code/photos", api.GetSharePhotos)
		share.GET("/p/:code/album", api.GetShareAlbum)
		share.GET("/p/:code/photo/:photoId", api.GetSharePhoto)

		// 分享管理
		share.GET("/list", api.ListShares)
		share.GET("/:id", api.GetShare)
		share.PUT("/:id", api.UpdateShare)
		share.DELETE("/:id", api.DeleteShare)

		// 分享统计
		share.GET("/:id/stats", api.GetShareStats)
	}
}

// ShareLink 分享链接
type ShareLink struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`        // 公开访问码
	Type          string     `json:"type"`        // "photos", "album"
	ResourceID    string     `json:"resource_id"` // 相册ID或照片ID列表
	PhotoIDs      []string   `json:"photo_ids,omitempty"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Password      string     `json:"password,omitempty"` // 访问密码（可选）
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	ViewLimit     int        `json:"view_limit"` // 查看次数限制
	Downloads     int        `json:"downloads"`  // 允许下载次数
	Views         int        `json:"views"`      // 已查看次数
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	AllowDownload bool       `json:"allow_download"`
	AllowComment  bool       `json:"allow_comment"`
	PublicAccess  bool       `json:"public_access"` // 是否公开可访问
}

// ShareStore 分享存储
type ShareStore struct {
	mu     sync.RWMutex
	shares map[string]*ShareLink
	file   string
}

// NewShareStore 创建分享存储
func NewShareStore() *ShareStore {
	s := &ShareStore{
		shares: make(map[string]*ShareLink),
		file:   "/var/lib/nas-os/photo_shares.json",
	}
	s.load()
	return s
}

// load 加载分享数据
func (s *ShareStore) load() {
	data, err := os.ReadFile(s.file)
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.shares)
}

// save 保存分享数据
func (s *ShareStore) save() {
	data, err := json.MarshalIndent(s.shares, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.file, data, 0644)
}

// generateCode 生成分享码
func generateCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:8]
}

// SharePhotosRequest 分享照片请求
type SharePhotosRequest struct {
	PhotoIDs      []string   `json:"photo_ids" binding:"required"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Password      string     `json:"password"`
	ExpiresAt     *time.Time `json:"expires_at"`
	ViewLimit     int        `json:"view_limit"`
	AllowDownload bool       `json:"allow_download"`
	AllowComment  bool       `json:"allow_comment"`
}

// SharePhotos 分享照片
func (api *ShareAPI) SharePhotos(c *gin.Context) {
	var req SharePhotosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证照片存在
	for _, photoID := range req.PhotoIDs {
		if _, err := api.manager.GetPhoto(photoID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "照片不存在: " + photoID})
			return
		}
	}

	share := &ShareLink{
		ID:            fmt.Sprintf("share_%d", time.Now().UnixNano()),
		Code:          generateCode(),
		Type:          "photos",
		PhotoIDs:      req.PhotoIDs,
		Title:         req.Title,
		Description:   req.Description,
		Password:      req.Password,
		ExpiresAt:     req.ExpiresAt,
		ViewLimit:     req.ViewLimit,
		AllowDownload: req.AllowDownload,
		AllowComment:  req.AllowComment,
		PublicAccess:  true,
		CreatedBy:     c.GetString("user_id"),
		CreatedAt:     time.Now(),
	}

	api.shareStore.mu.Lock()
	api.shareStore.shares[share.ID] = share
	api.shareStore.save()
	api.shareStore.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"share": share,
		"url":   fmt.Sprintf("/share/p/%s", share.Code),
	})
}

// ShareAlbum 分享相册
func (api *ShareAPI) ShareAlbum(c *gin.Context) {
	albumID := c.Param("id")

	album, err := api.manager.GetAlbum(albumID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "相册不存在"})
		return
	}

	var req SharePhotosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req.Title = album.Name
		req.Description = album.Description
	}

	share := &ShareLink{
		ID:            fmt.Sprintf("share_%d", time.Now().UnixNano()),
		Code:          generateCode(),
		Type:          "album",
		ResourceID:    albumID,
		Title:         req.Title,
		Description:   req.Description,
		Password:      req.Password,
		ExpiresAt:     req.ExpiresAt,
		ViewLimit:     req.ViewLimit,
		AllowDownload: req.AllowDownload,
		AllowComment:  req.AllowComment,
		PublicAccess:  true,
		CreatedBy:     c.GetString("user_id"),
		CreatedAt:     time.Now(),
	}

	api.shareStore.mu.Lock()
	api.shareStore.shares[share.ID] = share
	api.shareStore.save()
	api.shareStore.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"share": share,
		"url":   fmt.Sprintf("/share/p/%s", share.Code),
	})
}

// PublicShareResponse 公开分享响应
type PublicShareResponse struct {
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Type          string     `json:"type"`
	PhotoCount    int        `json:"photo_count"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	AllowDownload bool       `json:"allow_download"`
	AllowComment  bool       `json:"allow_comment"`
}

// GetPublicShare 获取公开分享（入口）
func (api *ShareAPI) GetPublicShare(c *gin.Context) {
	code := c.Param("code")

	share := api.findShareByCode(code)
	if share == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 检查过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "分享已过期"})
		return
	}

	// 检查查看限制
	if share.ViewLimit > 0 && share.Views >= share.ViewLimit {
		c.JSON(http.StatusForbidden, gin.H{"error": "分享已达查看上限"})
		return
	}

	// 检查密码
	if share.Password != "" {
		password := c.Query("password")
		if password != share.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误", "require_password": true})
			return
		}
	}

	// 增加查看次数
	api.shareStore.mu.Lock()
	share.Views++
	api.shareStore.save()
	api.shareStore.mu.Unlock()

	photoCount := len(share.PhotoIDs)
	if share.Type == "album" {
		album, err := api.manager.GetAlbum(share.ResourceID)
		if err == nil {
			photoCount = len(album.PhotoIDs)
		}
	}

	c.JSON(http.StatusOK, PublicShareResponse{
		Title:         share.Title,
		Description:   share.Description,
		Type:          share.Type,
		PhotoCount:    photoCount,
		ExpiresAt:     share.ExpiresAt,
		AllowDownload: share.AllowDownload,
		AllowComment:  share.AllowComment,
	})
}

// GetSharePhotos 获取分享的照片列表
func (api *ShareAPI) GetSharePhotos(c *gin.Context) {
	code := c.Param("code")

	share := api.findShareByCode(code)
	if share == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 权限检查
	if !checkShareAccess(share, c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要密码"})
		return
	}

	var photos []*Photo

	if share.Type == "album" {
		album, err := api.manager.GetAlbum(share.ResourceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, photoID := range album.PhotoIDs {
			photo, err := api.manager.GetPhoto(photoID)
			if err == nil {
				photos = append(photos, photo)
			}
		}
	} else {
		for _, photoID := range share.PhotoIDs {
			photo, err := api.manager.GetPhoto(photoID)
			if err == nil {
				photos = append(photos, photo)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"photos": photos})
}

// GetShareAlbum 获取分享的相册
func (api *ShareAPI) GetShareAlbum(c *gin.Context) {
	code := c.Param("code")

	share := api.findShareByCode(code)
	if share == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	if share.Type != "album" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不是相册分享"})
		return
	}

	// 权限检查
	if !checkShareAccess(share, c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要密码"})
		return
	}

	album, err := api.manager.GetAlbum(share.ResourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"album": album})
}

// GetSharePhoto 获取单张分享照片
func (api *ShareAPI) GetSharePhoto(c *gin.Context) {
	code := c.Param("code")
	photoID := c.Param("photoId")

	share := api.findShareByCode(code)
	if share == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 权限检查
	if !checkShareAccess(share, c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要密码"})
		return
	}

	// 检查照片是否在分享中
	inShare := false
	if share.Type == "album" {
		album, err := api.manager.GetAlbum(share.ResourceID)
		if err == nil {
			for _, id := range album.PhotoIDs {
				if id == photoID {
					inShare = true
					break
				}
			}
		}
	} else {
		for _, id := range share.PhotoIDs {
			if id == photoID {
				inShare = true
				break
			}
		}
	}

	if !inShare {
		c.JSON(http.StatusForbidden, gin.H{"error": "照片不在分享范围"})
		return
	}

	photo, err := api.manager.GetPhoto(photoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "照片不存在"})
		return
	}

	// 返回照片文件
	if share.AllowDownload {
		c.File(photo.Path)
	} else {
		// 仅预览，返回缩略图
		thumbPath := filepath.Join(filepath.Dir(photo.Path), "thumbnails", photoID+".jpg")
		if _, err := os.Stat(thumbPath); err == nil {
			c.File(thumbPath)
		} else {
			c.File(photo.Path)
		}
	}
}

// ListShares 列出用户的分享
func (api *ShareAPI) ListShares(c *gin.Context) {
	userID := c.GetString("user_id")

	api.shareStore.mu.RLock()
	var shares []*ShareLink
	for _, share := range api.shareStore.shares {
		if share.CreatedBy == userID {
			shares = append(shares, share)
		}
	}
	api.shareStore.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

// GetShare 获取分享详情
func (api *ShareAPI) GetShare(c *gin.Context) {
	shareID := c.Param("id")

	api.shareStore.mu.RLock()
	share, ok := api.shareStore.shares[shareID]
	api.shareStore.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 检查所有权
	if share.CreatedBy != c.GetString("user_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"share": share})
}

// UpdateShareRequest 更新分享请求
type UpdateShareRequest struct {
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Password      string     `json:"password"`
	ExpiresAt     *time.Time `json:"expires_at"`
	ViewLimit     int        `json:"view_limit"`
	AllowDownload bool       `json:"allow_download"`
	AllowComment  bool       `json:"allow_comment"`
}

// UpdateShare 更新分享
func (api *ShareAPI) UpdateShare(c *gin.Context) {
	shareID := c.Param("id")

	var req UpdateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	api.shareStore.mu.Lock()
	share, ok := api.shareStore.shares[shareID]
	if !ok {
		api.shareStore.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 检查所有权
	if share.CreatedBy != c.GetString("user_id") {
		api.shareStore.mu.Unlock()
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改"})
		return
	}

	// 更新字段
	if req.Title != "" {
		share.Title = req.Title
	}
	if req.Description != "" {
		share.Description = req.Description
	}
	share.Password = req.Password
	share.ExpiresAt = req.ExpiresAt
	share.ViewLimit = req.ViewLimit
	share.AllowDownload = req.AllowDownload
	share.AllowComment = req.AllowComment

	api.shareStore.save()
	api.shareStore.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"share": share})
}

// DeleteShare 删除分享
func (api *ShareAPI) DeleteShare(c *gin.Context) {
	shareID := c.Param("id")

	api.shareStore.mu.Lock()
	share, ok := api.shareStore.shares[shareID]
	if !ok {
		api.shareStore.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 检查所有权
	if share.CreatedBy != c.GetString("user_id") {
		api.shareStore.mu.Unlock()
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}

	delete(api.shareStore.shares, shareID)
	api.shareStore.save()
	api.shareStore.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "分享已删除"})
}

// ShareStats 分享统计
type ShareStats struct {
	Views       int        `json:"views"`
	Downloads   int        `json:"downloads"`
	LastView    *time.Time `json:"last_view,omitempty"`
	ViewHistory []ViewLog  `json:"view_history"`
}

// ViewLog 查看日志
type ViewLog struct {
	Time      time.Time `json:"time"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
}

// GetShareStats 获取分享统计
func (api *ShareAPI) GetShareStats(c *gin.Context) {
	shareID := c.Param("id")

	api.shareStore.mu.RLock()
	share, ok := api.shareStore.shares[shareID]
	api.shareStore.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 检查所有权
	if share.CreatedBy != c.GetString("user_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	stats := ShareStats{
		Views:     share.Views,
		Downloads: share.Downloads,
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// findShareByCode 通过code查找分享
func (api *ShareAPI) findShareByCode(code string) *ShareLink {
	api.shareStore.mu.RLock()
	defer api.shareStore.mu.RUnlock()

	for _, share := range api.shareStore.shares {
		if share.Code == code {
			return share
		}
	}
	return nil
}

// checkShareAccess 检查分享访问权限
func checkShareAccess(share *ShareLink, c *gin.Context) bool {
	// 检查过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return false
	}

	// 检查密码
	if share.Password != "" {
		password := c.Query("password")
		if password != share.Password {
			return false
		}
	}

	return true
}
