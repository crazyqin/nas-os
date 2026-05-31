// Package musicserver 提供 REST API 处理器
package musicserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 音乐服务模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/music 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ms := r.Group("/music")
	{
		// 歌曲 CRUD
		ms.GET("/songs", h.listSongs)
		ms.POST("/songs", h.addSong)
		ms.GET("/songs/:id", h.getSong)
		ms.DELETE("/songs/:id", h.deleteSong)
		ms.GET("/songs/:id/lyrics", h.getSongLyrics)
		ms.POST("/songs/:id/lyrics", h.setSongLyrics)
		ms.GET("/songs/:id/cover", h.getSongCover)

		// 专辑
		ms.GET("/albums", h.listAlbums)
		ms.GET("/albums/:id", h.getAlbum)
		ms.GET("/albums/artist/:artist", h.listAlbumsByArtist)
		ms.GET("/albums/genre/:genre", h.listAlbumsByGenre)

		// 艺术家
		ms.GET("/artists", h.listArtists)
		ms.GET("/artists/:id", h.getArtist)

		// 播放列表 CRUD
		ms.GET("/playlists", h.listPlaylists)
		ms.POST("/playlists", h.createPlaylist)
		ms.GET("/playlists/:id", h.getPlaylist)
		ms.PUT("/playlists/:id", h.updatePlaylist)
		ms.DELETE("/playlists/:id", h.deletePlaylist)
		ms.GET("/playlists/:id/songs", h.getPlaylistSongs)
		ms.POST("/playlists/:id/songs", h.addSongToPlaylist)
		ms.DELETE("/playlists/:id/songs/:songId", h.removeSongFromPlaylist)

		// 播放队列
		ms.GET("/queue", h.getPlayQueue)
		ms.PUT("/queue", h.updatePlayQueue)

		// 收藏
		ms.GET("/favorites", h.getFavorites)
		ms.POST("/songs/:id/favorite", h.setFavorite)

		// 播放记录
		ms.POST("/songs/:id/play", h.recordPlay)
		ms.GET("/recent", h.getRecentlyPlayed)

		// 搜索
		ms.GET("/search", h.search)

		// 统计
		ms.GET("/stats", h.getStats)

		// Subsonic API 兼容端点
		ms.GET("/subsonic/ping", h.subsonicPing)
		ms.GET("/subsonic/search2", h.subsonicSearch)
		ms.GET("/subsonic/getArtists", h.subsonicGetArtists)
		ms.GET("/subsonic/getAlbumList2", h.subsonicGetAlbumList)
		ms.GET("/subsonic/getPlaylist", h.subsonicGetPlaylist)
		ms.GET("/subsonic/getPlaylists", h.subsonicGetPlaylists)
		ms.GET("/subsonic/stream", h.subsonicStream)
		ms.GET("/subsonic/getCoverArt", h.subsonicGetCoverArt)
		ms.GET("/subsonic/getLyrics", h.subsonicGetLyrics)
		ms.GET("/subsonic/scrobble", h.subsonicScrobble)
		ms.GET("/subsonic/getNowPlaying", h.subsonicGetNowPlaying)
	}
}

// ========== 歌曲处理 ==========

func (h *Handlers) listSongs(c *gin.Context) {
	owner := c.Query("owner")
	songs := h.manager.ListSongs(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(songs),
			"songs": songs,
		},
	})
}

func (h *Handlers) addSong(c *gin.Context) {
	var song Song
	if err := c.ShouldBindJSON(&song); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result := h.manager.AddSong(&song)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: result})
}

func (h *Handlers) getSong(c *gin.Context) {
	id := c.Param("id")
	song, err := h.manager.GetSong(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: song})
}

func (h *Handlers) deleteSong(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSong(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) getSongLyrics(c *gin.Context) {
	songID := c.Param("id")
	lyrics, err := h.manager.GetLyrics(songID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: lyrics})
}

func (h *Handlers) setSongLyrics(c *gin.Context) {
	songID := c.Param("id")
	var req struct {
		Format  string `json:"format" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.SetLyrics(songID, req.Format, req.Content); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success"})
}

func (h *Handlers) getSongCover(c *gin.Context) {
	songID := c.Param("id")
	cover, err := h.manager.GetCoverArtBySongID(songID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cover})
}

// ========== 专辑处理 ==========

func (h *Handlers) listAlbums(c *gin.Context) {
	owner := c.Query("owner")
	albums := h.manager.ListAlbums(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(albums),
			"albums": albums,
		},
	})
}

func (h *Handlers) getAlbum(c *gin.Context) {
	id := c.Param("id")
	album, err := h.manager.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: album})
}

func (h *Handlers) listAlbumsByArtist(c *gin.Context) {
	artist := c.Param("artist")
	owner := c.Query("owner")
	albums := h.manager.ListAlbumsByArtist(artist, owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(albums),
			"albums": albums,
		},
	})
}

func (h *Handlers) listAlbumsByGenre(c *gin.Context) {
	genre := c.Param("genre")
	owner := c.Query("owner")
	albums := h.manager.ListAlbumsByGenre(genre, owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(albums),
			"albums": albums,
		},
	})
}

// ========== 艺术家处理 ==========

func (h *Handlers) listArtists(c *gin.Context) {
	owner := c.Query("owner")
	artists := h.manager.ListArtists(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(artists),
			"artists": artists,
		},
	})
}

func (h *Handlers) getArtist(c *gin.Context) {
	id := c.Param("id")
	artist, err := h.manager.GetArtist(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: artist})
}

// ========== 播放列表处理 ==========

func (h *Handlers) listPlaylists(c *gin.Context) {
	owner := c.Query("owner")
	playlists := h.manager.ListPlaylists(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(playlists),
			"playlists": playlists,
		},
	})
}

func (h *Handlers) createPlaylist(c *gin.Context) {
	var req CreatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	playlist := h.manager.CreatePlaylist(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: playlist})
}

func (h *Handlers) getPlaylist(c *gin.Context) {
	id := c.Param("id")
	playlist, err := h.manager.GetPlaylist(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: playlist})
}

func (h *Handlers) updatePlaylist(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	playlist, err := h.manager.UpdatePlaylist(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: playlist})
}

func (h *Handlers) deletePlaylist(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePlaylist(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) getPlaylistSongs(c *gin.Context) {
	id := c.Param("id")
	songs, err := h.manager.GetPlaylistSongs(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(songs),
			"songs": songs,
		},
	})
}

func (h *Handlers) addSongToPlaylist(c *gin.Context) {
	playlistID := c.Param("id")
	var req AddSongToPlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	owner := c.Query("owner")
	if err := h.manager.AddSongToPlaylist(playlistID, req.SongID, owner, req.Position); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "added"})
}

func (h *Handlers) removeSongFromPlaylist(c *gin.Context) {
	playlistID := c.Param("id")
	songID := c.Param("songId")

	if err := h.manager.RemoveSongFromPlaylist(playlistID, songID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

// ========== 播放队列处理 ==========

func (h *Handlers) getPlayQueue(c *gin.Context) {
	owner := c.Query("owner")
	queue := h.manager.GetPlayQueue(owner)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: queue})
}

func (h *Handlers) updatePlayQueue(c *gin.Context) {
	owner := c.Query("owner")
	var req UpdatePlayQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	queue := h.manager.UpdatePlayQueue(owner, req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: queue})
}

// ========== 收藏处理 ==========

func (h *Handlers) getFavorites(c *gin.Context) {
	owner := c.Query("owner")
	songs := h.manager.GetFavorites(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(songs),
			"songs": songs,
		},
	})
}

func (h *Handlers) setFavorite(c *gin.Context) {
	songID := c.Param("id")
	owner := c.Query("owner")

	var req FavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.SetFavorite(owner, songID, req.IsFavorite); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success"})
}

// ========== 播放记录处理 ==========

func (h *Handlers) recordPlay(c *gin.Context) {
	songID := c.Param("id")
	owner := c.Query("owner")

	if err := h.manager.RecordPlay(owner, songID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "recorded"})
}

func (h *Handlers) getRecentlyPlayed(c *gin.Context) {
	owner := c.Query("owner")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	songs := h.manager.GetRecentlyPlayed(owner, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(songs),
			"songs": songs,
		},
	})
}

// ========== 搜索处理 ==========

func (h *Handlers) search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'q' is required"})
		return
	}

	searchType := c.Query("type")
	owner := c.Query("owner")

	results := h.manager.Search(query, owner, searchType)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: results})
}

// ========== 统计处理 ==========

func (h *Handlers) getStats(c *gin.Context) {
	owner := c.Query("owner")
	stats := h.manager.GetStats(owner)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== Subsonic API 处理 ==========

func (h *Handlers) subsonicPing(c *gin.Context) {
	c.JSON(http.StatusOK, SubsonicResponse{
		Status:        "ok",
		Version:       "1.16.1",
		Type:          "nas-os",
		ServerVersion: "1.0.0",
		OpenSubsonic:  true,
	})
}

func (h *Handlers) subsonicSearch(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusOK, SubsonicResponse{
			Status: "ok",
			Error: &SubsonicError{
				Code:    10,
				Message: "Missing query parameter",
			},
		})
		return
	}

	owner := c.Query("u")
	results := h.manager.Search(query, owner, "all")

	resp := SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
		SearchResult: &SubsonicSearchResult{},
	}

	for _, song := range results.Songs {
		resp.SearchResult.Songs = append(resp.SearchResult.Songs, SubsonicSong{
			ID:        song.ID,
			Title:     song.Title,
			Artist:    song.Artist,
			Album:     song.Album,
			Genre:     song.Genre,
			Year:      song.Year,
			Track:     song.Track,
			Duration:  song.Duration,
			Suffix:    song.Format,
			BitRate:   song.Bitrate,
			Path:      song.FilePath,
			PlayCount: song.PlayCount,
			CoverArt:  song.CoverArtID,
		})
	}

	for _, album := range results.Albums {
		resp.SearchResult.Albums = append(resp.SearchResult.Albums, SubsonicAlbum{
			ID:        album.ID,
			Name:      album.Name,
			Artist:    album.Artist,
			Genre:     album.Genre,
			Year:      album.Year,
			SongCount: album.SongCount,
			Duration:  album.TotalDuration,
			CoverArt:  album.CoverArtID,
		})
	}

	for _, artist := range results.Artists {
		resp.SearchResult.Artists = append(resp.SearchResult.Artists, SubsonicArtist{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: artist.AlbumCount,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) subsonicGetArtists(c *gin.Context) {
	owner := c.Query("u")
	artists := h.manager.ListArtists(owner)

	resp := SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	}

	for _, artist := range artists {
		resp.Artists = append(resp.Artists, SubsonicArtist{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: artist.AlbumCount,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) subsonicGetAlbumList(c *gin.Context) {
	owner := c.Query("u")
	albums := h.manager.ListAlbums(owner)

	resp := SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	}

	for _, album := range albums {
		resp.Albums = append(resp.Albums, SubsonicAlbum{
			ID:        album.ID,
			Name:      album.Name,
			Artist:    album.Artist,
			Genre:     album.Genre,
			Year:      album.Year,
			SongCount: album.SongCount,
			Duration:  album.TotalDuration,
			CoverArt:  album.CoverArtID,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) subsonicGetPlaylist(c *gin.Context) {
	id := c.Query("id")
	playlist, err := h.manager.GetPlaylist(id)
	if err != nil {
		c.JSON(http.StatusOK, SubsonicResponse{
			Status: "ok",
			Error: &SubsonicError{
				Code:    70,
				Message: "Playlist not found",
			},
		})
		return
	}

	songs, _ := h.manager.GetPlaylistSongs(id)

	resp := SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	}

	for _, song := range songs {
		resp.Songs = append(resp.Songs, SubsonicSong{
			ID:        song.ID,
			Title:     song.Title,
			Artist:    song.Artist,
			Album:     song.Album,
			Genre:     song.Genre,
			Year:      song.Year,
			Track:     song.Track,
			Duration:  song.Duration,
			Suffix:    song.Format,
			BitRate:   song.Bitrate,
			Path:      song.FilePath,
			PlayCount: song.PlayCount,
			CoverArt:  song.CoverArtID,
		})
	}

	_ = playlist // 使用 playlist 变量
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) subsonicGetPlaylists(c *gin.Context) {
	owner := c.Query("u")
	playlists := h.manager.ListPlaylists(owner)

	resp := SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	}

	for _, playlist := range playlists {
		resp.Playlists = append(resp.Playlists, SubsonicPlaylist{
			ID:        playlist.ID,
			Name:      playlist.Name,
			SongCount: playlist.SongCount,
			Duration:  playlist.TotalDuration,
			Owner:     playlist.Owner,
			Public:    playlist.IsPublic,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) subsonicStream(c *gin.Context) {
	id := c.Query("id")
	song, err := h.manager.GetSong(id)
	if err != nil {
		c.JSON(http.StatusNotFound, SubsonicResponse{
			Status: "ok",
			Error: &SubsonicError{
				Code:    70,
				Message: "Song not found",
			},
		})
		return
	}

	// 记录播放
	owner := c.Query("u")
	h.manager.RecordPlay(owner, id)

	c.Header("Content-Type", getContentType(song.Format))
	c.File(song.FilePath)
}

func (h *Handlers) subsonicGetCoverArt(c *gin.Context) {
	id := c.Query("id")
	cover, err := h.manager.GetCoverArt(id)
	if err != nil {
		// 尝试作为歌曲ID查找
		cover, err = h.manager.GetCoverArtBySongID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, SubsonicResponse{
				Status: "ok",
				Error: &SubsonicError{
					Code:    70,
					Message: "Cover art not found",
				},
			})
			return
		}
	}

	c.Header("Content-Type", cover.MimeType)
	c.File(cover.FilePath)
}

func (h *Handlers) subsonicGetLyrics(c *gin.Context) {
	artist := c.Query("artist")
	title := c.Query("title")

	// 搜索歌曲
	results := h.manager.Search(title, "", "song")
	var song *Song
	for _, s := range results.Songs {
		if strings.EqualFold(s.Artist, artist) && strings.EqualFold(s.Title, title) {
			song = s
			break
		}
	}

	if song == nil {
		c.JSON(http.StatusOK, SubsonicResponse{
			Status:  "ok",
			Version: "1.16.1",
		})
		return
	}

	lyrics, err := h.manager.GetLyrics(song.ID)
	if err != nil {
		c.JSON(http.StatusOK, SubsonicResponse{
			Status:  "ok",
			Version: "1.16.1",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subsonic-response": gin.H{
			"status":  "ok",
			"version": "1.16.1",
			"lyrics": gin.H{
				"artist": artist,
				"title":  title,
				"value":  lyrics.Content,
			},
		},
	})
}

func (h *Handlers) subsonicScrobble(c *gin.Context) {
	id := c.Query("id")
	owner := c.Query("u")

	if err := h.manager.RecordPlay(owner, id); err != nil {
		c.JSON(http.StatusOK, SubsonicResponse{
			Status: "ok",
			Error: &SubsonicError{
				Code:    70,
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	})
}

func (h *Handlers) subsonicGetNowPlaying(c *gin.Context) {
	// 简化实现：返回空列表
	c.JSON(http.StatusOK, SubsonicResponse{
		Status:  "ok",
		Version: "1.16.1",
	})
}

// ========== 辅助函数 ==========

// getContentType 根据音频格式返回 Content-Type.
func getContentType(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "aac", "m4a":
		return "audio/mp4"
	case "ogg":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}
