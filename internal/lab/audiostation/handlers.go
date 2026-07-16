// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handlers 音乐中心 HTTP 处理器.
type Handlers struct {
	manager   *Manager
	scanner   *Scanner
	player    *Player
	hls       *HLSManager
	tagEditor *TagEditor
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	tmpDir := os.TempDir()
	return &Handlers{
		manager:   mgr,
		scanner:   NewScanner(mgr),
		player:    NewPlayer(mgr),
		hls:       NewHLSManager(mgr, filepath.Join(tmpDir, "audiostation-hls"), 10.0),
		tagEditor: NewTagEditor(mgr),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	as := api.Group("/audiostation")
	{
		// ========== 音乐库 ==========
		as.GET("/library", h.listLibrary)
		as.POST("/library/scan", h.triggerScan)
		as.GET("/library/stats", h.getStats)

		// ========== 分类浏览 ==========
		as.GET("/albums", h.listAlbums)
		as.GET("/albums/:id", h.getAlbum)
		as.GET("/artists", h.listArtists)
		as.GET("/genres", h.listGenres)

		// ========== 播放列表 ==========
		as.POST("/playlists", h.createPlaylist)
		as.GET("/playlists", h.listPlaylists)
		as.GET("/playlists/:id", h.getPlaylist)
		as.PUT("/playlists/:id", h.updatePlaylist)
		as.DELETE("/playlists/:id", h.deletePlaylist)

		// ========== 播放控制 ==========
		as.GET("/play/:id", h.playTrack)
		as.POST("/queue/add", h.addToQueue)
		as.GET("/queue", h.getQueue)
		as.PUT("/queue/reorder", h.reorderQueue)
		as.DELETE("/queue/:index", h.removeFromQueue)

		// ========== 收藏与历史 ==========
		as.GET("/favorites", h.listFavorites)
		as.POST("/favorites/:id", h.toggleFavorite)
		as.GET("/recent", h.getRecent)

		// ========== DLNA ==========
		as.GET("/dlna/devices", h.listDLNADevices)
		as.POST("/dlna/cast", h.castToDLNA)

		// ========== HLS 流媒体 ==========
		as.POST("/hls/stream/:id", h.createHLSStream)
		as.GET("/hls/:sessionId/master.m3u8", h.getHLSMasterPlaylist)
		as.GET("/hls/:sessionId/media.m3u8", h.getHLSMediaPlaylist)
		as.GET("/hls/status", h.getHLSStatus)

		// ========== 标签编辑 ==========
		as.PUT("/tracks/:id/tag", h.updateTrackTag)
		as.POST("/tracks/tag/batch", h.batchUpdateTags)

		// ========== 分享 ==========
		as.POST("/playlists/:id/share", h.sharePlaylist)
	}
}

// ========== 音乐库 API ==========

// listLibrary 音乐库列表（支持分页、搜索、排序）.
func (h *Handlers) listLibrary(c *gin.Context) {
	var query LibraryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	tracks, total := h.manager.ListTracks(query)
	c.JSON(http.StatusOK, APISuccess(gin.H{
		"tracks":   tracks,
		"total":    total,
		"page":     query.Page,
		"per_page": query.PerPage,
	}))
}

// triggerScan 触发音乐库扫描.
func (h *Handlers) triggerScan(c *gin.Context) {
	status := h.manager.GetScanStatus()
	if status.IsRunning {
		c.JSON(http.StatusConflict, APIError(409, ErrScanInProgress.Error()))
		return
	}

	var req ScanRequest
	_ = c.ShouldBindJSON(&req)

	paths := req.Paths
	if len(paths) == 0 {
		paths = h.manager.libraryPaths
	}

	if len(paths) == 0 {
		c.JSON(http.StatusBadRequest, APIError(400, "未配置音乐库路径"))
		return
	}

	go h.scanner.Scan(paths, req.Recursive, req.Force)

	c.JSON(http.StatusOK, APISuccess(gin.H{"message": "扫描已启动"}))
}

// getStats 音乐库统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, APISuccess(stats))
}

// ========== 分类浏览 API ==========

// listAlbums 专辑列表.
func (h *Handlers) listAlbums(c *gin.Context) {
	artist := c.Query("artist")
	genre := c.Query("genre")
	albums := h.manager.ListAlbums(artist, genre)
	c.JSON(http.StatusOK, APISuccess(albums))
}

// getAlbum 专辑详情.
func (h *Handlers) getAlbum(c *gin.Context) {
	id := c.Param("id")
	album, err := h.manager.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, APISuccess(album))
}

// listArtists 艺术家列表.
func (h *Handlers) listArtists(c *gin.Context) {
	artists := h.manager.ListArtists()
	c.JSON(http.StatusOK, APISuccess(artists))
}

// listGenres 流派列表.
func (h *Handlers) listGenres(c *gin.Context) {
	genres := h.manager.ListGenres()
	c.JSON(http.StatusOK, APISuccess(genres))
}

// ========== 播放列表 API ==========

// createPlaylist 创建播放列表.
func (h *Handlers) createPlaylist(c *gin.Context) {
	var req PlaylistInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	playlist, err := h.manager.CreatePlaylist(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, APISuccess(playlist))
}

// listPlaylists 播放列表列表.
func (h *Handlers) listPlaylists(c *gin.Context) {
	playlists := h.manager.ListPlaylists()
	c.JSON(http.StatusOK, APISuccess(playlists))
}

// getPlaylist 播放列表详情.
func (h *Handlers) getPlaylist(c *gin.Context) {
	id := c.Param("id")
	playlist, err := h.manager.GetPlaylist(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, APISuccess(playlist))
}

// updatePlaylist 更新播放列表.
func (h *Handlers) updatePlaylist(c *gin.Context) {
	id := c.Param("id")
	var req PlaylistInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	playlist, err := h.manager.UpdatePlaylist(id, req)
	if err != nil {
		if err == ErrPlaylistNotFound {
			c.JSON(http.StatusNotFound, APIError(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(playlist))
}

// deletePlaylist 删除播放列表.
func (h *Handlers) deletePlaylist(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePlaylist(id); err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, APISuccess(nil))
}

// ========== 播放控制 API ==========

// playTrack 播放音乐（流式，支持 Range 请求）.
func (h *Handlers) playTrack(c *gin.Context) {
	id := c.Param("id")

	track, err := h.manager.GetTrack(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(track.FilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, "音乐文件不存在"))
		return
	}

	// 记录播放
	go func() {
		_ = h.manager.RecordPlay(id)
	}()

	// 确定 MIME 类型
	ext := strings.ToLower(filepath.Ext(track.FilePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 设置响应头
	c.Header("Content-Type", mimeType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s%s\"", track.Title, ext))

	// 使用 http.ServeContent 支持 Range 请求
	file, err := os.Open(track.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIError(500, "打开文件失败"))
		return
	}
	defer file.Close()

	http.ServeContent(c.Writer, c.Request, track.Title+ext, fileInfo.ModTime(), file)
}

// addToQueue 添加到播放队列.
func (h *Handlers) addToQueue(c *gin.Context) {
	var req QueueAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	if err := h.player.AddToQueue(req.TrackIDs, req.Position); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(h.player.GetQueue()))
}

// getQueue 获取当前播放队列.
func (h *Handlers) getQueue(c *gin.Context) {
	queue := h.player.GetQueue()
	c.JSON(http.StatusOK, APISuccess(queue))
}

// reorderQueue 重排序播放队列.
func (h *Handlers) reorderQueue(c *gin.Context) {
	var req QueueReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	if err := h.player.ReorderQueue(req.FromIndex, req.ToIndex); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(h.player.GetQueue()))
}

// removeFromQueue 从播放队列移除.
func (h *Handlers) removeFromQueue(c *gin.Context) {
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, "无效的索引"))
		return
	}

	if err := h.player.RemoveFromQueue(index); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(h.player.GetQueue()))
}

// ========== 收藏与历史 API ==========

// listFavorites 收藏列表.
func (h *Handlers) listFavorites(c *gin.Context) {
	favorites := h.manager.ListFavorites()
	c.JSON(http.StatusOK, APISuccess(favorites))
}

// toggleFavorite 切换收藏状态.
func (h *Handlers) toggleFavorite(c *gin.Context) {
	id := c.Param("id")
	isFav, err := h.manager.ToggleFavorite(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, APISuccess(gin.H{"is_favorite": isFav}))
}

// getRecent 最近播放.
func (h *Handlers) getRecent(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	recent := h.manager.GetRecentPlayed(limit)
	c.JSON(http.StatusOK, APISuccess(recent))
}

// ========== DLNA API ==========

// listDLNADevices DLNA设备列表.
func (h *Handlers) listDLNADevices(c *gin.Context) {
	devices := h.manager.ListDLNADevices()
	c.JSON(http.StatusOK, APISuccess(devices))
}

// castToDLNA 推送到DLNA设备.
func (h *Handlers) castToDLNA(c *gin.Context) {
	var req DLNACastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	if err := h.manager.CastToDLNA(req.DeviceID, req.TrackID); err != nil {
		if err == ErrDLNADeviceNotFound {
			c.JSON(http.StatusNotFound, APIError(404, err.Error()))
			return
		}
		if err == ErrTrackNotFound {
			c.JSON(http.StatusNotFound, APIError(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(gin.H{"status": "casting"}))
}

// ========== HLS 流媒体 API ==========

// createHLSStream 创建 HLS 流媒体会话.
func (h *Handlers) createHLSStream(c *gin.Context) {
	id := c.Param("id")

	// 验证曲目存在
	if _, err := h.manager.GetTrack(id); err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}

	baseURL := fmt.Sprintf("http://%s", c.Request.Host)
	session, err := h.hls.CreateStream(id, baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(session))
}

// getHLSMasterPlaylist 获取 HLS Master Playlist.
func (h *Handlers) getHLSMasterPlaylist(c *gin.Context) {
	sessionID := c.Param("sessionId")

	m3u8, err := h.hls.GenerateMasterPlaylist(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, m3u8)
}

// getHLSMediaPlaylist 获取 HLS Media Playlist.
func (h *Handlers) getHLSMediaPlaylist(c *gin.Context) {
	sessionID := c.Param("sessionId")

	m3u8, err := h.hls.GenerateMediaPlaylist(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIError(404, err.Error()))
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, m3u8)
}

// getHLSStatus 获取 HLS 服务状态.
func (h *Handlers) getHLSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, APISuccess(gin.H{
		"active_sessions": h.hls.GetActiveSessions(),
	}))
}

// ========== 标签编辑 API ==========

// updateTrackTag 更新曲目标签.
func (h *Handlers) updateTrackTag(c *gin.Context) {
	id := c.Param("id")

	var req TagUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	track, err := h.tagEditor.UpdateTrackTag(id, req)
	if err != nil {
		if err == ErrTrackNotFound {
			c.JSON(http.StatusNotFound, APIError(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(track))
}

// batchUpdateTags 批量更新标签.
func (h *Handlers) batchUpdateTags(c *gin.Context) {
	var req struct {
		TrackIDs []string         `json:"track_ids" binding:"required"`
		Tags     TagUpdateRequest `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIError(400, err.Error()))
		return
	}

	tracks, err := h.tagEditor.BatchUpdateTags(req.TrackIDs, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(gin.H{
		"updated": len(tracks),
		"tracks":  tracks,
	}))
}

// ========== 分享 API ==========

// sharePlaylist 分享播放列表.
func (h *Handlers) sharePlaylist(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ExpireHours int `json:"expire_hours"`
	}
	_ = c.ShouldBindJSON(&req)

	shared, err := h.manager.SharePlaylist(id, req.ExpireHours)
	if err != nil {
		if err == ErrPlaylistNotFound {
			c.JSON(http.StatusNotFound, APIError(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, APIError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, APISuccess(shared))
}
