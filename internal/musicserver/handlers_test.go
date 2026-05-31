// Package musicserver 提供 REST API 处理器测试
package musicserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	h := NewHandlers(mgr)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, mgr
}

func addTestSong(mgr *Manager, title, artist, album string) *Song {
	return mgr.AddSong(&Song{
		Title:    title,
		Artist:   artist,
		Album:    album,
		Genre:    "Rock",
		Year:     2024,
		Duration: 180,
		Format:   "mp3",
		Bitrate:  320,
		Owner:    "testuser",
	})
}

// TestAddSong 测试添加歌曲.
func TestAddSong(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := Song{
		Title:    "Test Song",
		Artist:   "Test Artist",
		Album:    "Test Album",
		Genre:    "Pop",
		Year:     2024,
		Duration: 240,
		Format:   "mp3",
		Bitrate:  320,
		Owner:    "testuser",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/songs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Test Song", data["title"])
	assert.Equal(t, "Test Artist", data["artist"])
	assert.Equal(t, "testuser", data["owner"])
}

// TestGetSong 测试获取歌曲.
func TestGetSong(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "My Song", "My Artist", "My Album")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/songs/"+song.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "My Song", data["title"])
}

// TestListSongs 测试列出歌曲.
func TestListSongs(t *testing.T) {
	r, mgr := setupTestRouter()

	addTestSong(mgr, "Song 1", "Artist 1", "Album 1")
	addTestSong(mgr, "Song 2", "Artist 2", "Album 2")
	addTestSong(mgr, "Song 3", "Artist 1", "Album 1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/songs?owner=testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 3, int(data["total"].(float64)))
}

// TestDeleteSong 测试删除歌曲.
func TestDeleteSong(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "Delete Me", "Artist", "Album")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/music/songs/"+song.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证歌曲已删除
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/music/songs/"+song.ID, nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

// TestCreatePlaylist 测试创建播放列表.
func TestCreatePlaylist(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreatePlaylistRequest{
		Name:        "My Playlist",
		Description: "Test playlist",
		Owner:       "testuser",
		IsPublic:    true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/playlists", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "My Playlist", data["name"])
	assert.Equal(t, true, data["is_public"])
}

// TestAddSongToPlaylist 测试添加歌曲到播放列表.
func TestAddSongToPlaylist(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "Playlist Song", "Artist", "Album")
	playlist := mgr.CreatePlaylist(CreatePlaylistRequest{
		Name:  "Test Playlist",
		Owner: "testuser",
	})

	reqBody := AddSongToPlaylistRequest{
		SongID: song.ID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/playlists/"+playlist.ID+"/songs?owner=testuser", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证歌曲已添加
	songs, _ := mgr.GetPlaylistSongs(playlist.ID)
	assert.Equal(t, 1, len(songs))
}

// TestPlayQueue 测试播放队列.
func TestPlayQueue(t *testing.T) {
	r, mgr := setupTestRouter()

	song1 := addTestSong(mgr, "Queue Song 1", "Artist", "Album")
	song2 := addTestSong(mgr, "Queue Song 2", "Artist", "Album")

	reqBody := UpdatePlayQueueRequest{
		SongIDs:      []string{song1.ID, song2.ID},
		CurrentIndex: intPtr(0),
		Shuffle:      boolPtr(false),
		Repeat:       repeatModePtr(RepeatAll),
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/music/queue?owner=testuser", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	queue := mgr.GetPlayQueue("testuser")
	assert.Equal(t, 2, len(queue.Songs))
	assert.Equal(t, RepeatAll, queue.Repeat)
}

// TestFavorites 测试收藏功能.
func TestFavorites(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "Favorite Song", "Artist", "Album")

	reqBody := FavoriteRequest{
		IsFavorite: true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/songs/"+song.ID+"/favorite?owner=testuser", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 获取收藏列表
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/music/favorites?owner=testuser", nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var resp response
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 1, int(data["total"].(float64)))
}

// TestSearch 测试搜索功能.
func TestSearch(t *testing.T) {
	r, mgr := setupTestRouter()

	addTestSong(mgr, "Rock Song", "Rock Artist", "Rock Album")
	addTestSong(mgr, "Pop Song", "Pop Artist", "Pop Album")
	addTestSong(mgr, "Jazz Song", "Jazz Artist", "Jazz Album")

	// 搜索 "Rock"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/search?q=Rock&owner=testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	songs := data["songs"].([]interface{})
	assert.Equal(t, 1, len(songs))

	_ = mgr
}

// TestRecordPlay 测试播放记录.
func TestRecordPlay(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "Play Song", "Artist", "Album")

	// 记录播放
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/songs/"+song.ID+"/play?owner=testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 获取最近播放
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/music/recent?owner=testuser&limit=10", nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var resp response
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 1, int(data["total"].(float64)))

	// 验证播放次数
	songData, _ := mgr.GetSong(song.ID)
	assert.Equal(t, 1, songData.PlayCount)
}

// TestLyrics 测试歌词功能.
func TestLyrics(t *testing.T) {
	r, mgr := setupTestRouter()

	song := addTestSong(mgr, "Lyrics Song", "Artist", "Album")

	// 设置歌词
	lrcContent := `[00:00.00]First line
[00:05.00]Second line
[00:10.00]Third line`

	reqBody := struct {
		Format  string `json:"format"`
		Content string `json:"content"`
	}{
		Format:  "lrc",
		Content: lrcContent,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/music/songs/"+song.ID+"/lyrics", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 获取歌词
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/music/songs/"+song.ID+"/lyrics", nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var resp response
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "lrc", data["format"])
	assert.Equal(t, 3, len(data["lines"].([]interface{})))

	_ = mgr
}

// TestStats 测试统计功能.
func TestStats(t *testing.T) {
	r, mgr := setupTestRouter()

	addTestSong(mgr, "Song 1", "Artist 1", "Album 1")
	addTestSong(mgr, "Song 2", "Artist 2", "Album 2")
	addTestSong(mgr, "Song 3", "Artist 1", "Album 1")

	// 记录一些播放
	mgr.RecordPlay("testuser", "song1")
	mgr.RecordPlay("testuser", "song1")
	mgr.RecordPlay("testuser", "song2")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/stats?owner=testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 3, int(data["total_songs"].(float64)))
}

// TestSubsonicPing 测试 Subsonic ping 接口.
func TestSubsonicPing(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/subsonic/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SubsonicResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "1.16.1", resp.Version)
}

// TestSubsonicSearch 测试 Subsonic 搜索接口.
func TestSubsonicSearch(t *testing.T) {
	r, mgr := setupTestRouter()

	addTestSong(mgr, "Subsonic Song", "Subsonic Artist", "Subsonic Album")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/music/subsonic/search2?query=Subsonic&u=testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SubsonicResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, 1, len(resp.SearchResult.Songs))

	_ = mgr
}

// ========== 辅助函数 ==========

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func repeatModePtr(v RepeatMode) *RepeatMode {
	return &v
}
