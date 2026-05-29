// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 辅助函数 ==========

// setupTestManager 创建测试用管理器.
func setupTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	mgr, err := NewManager(configPath, []string{tmpDir})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 添加测试曲目
	tracks := []*Track{
		{
			ID:          "track_001",
			Title:       "夜曲",
			Artist:      "周杰伦",
			Album:       "十一月的萧邦",
			AlbumArtist: "周杰伦",
			Genre:       "流行",
			Year:        2005,
			TrackNum:    1,
			Duration:    235,
			Bitrate:     320,
			Format:      FormatMP3,
			FileSize:    9400000,
			FilePath:    filepath.Join(tmpDir, "track001.mp3"),
		},
		{
			ID:          "track_002",
			Title:       "七里香",
			Artist:      "周杰伦",
			Album:       "七里香",
			AlbumArtist: "周杰伦",
			Genre:       "流行",
			Year:        2004,
			TrackNum:    3,
			Duration:    299,
			Bitrate:     320,
			Format:      FormatMP3,
			FileSize:    11960000,
			FilePath:    filepath.Join(tmpDir, "track002.mp3"),
		},
		{
			ID:          "track_003",
			Title:       "Hotel California",
			Artist:      "Eagles",
			Album:       "Hotel California",
			AlbumArtist: "Eagles",
			Genre:       "Rock",
			Year:        1977,
			TrackNum:    1,
			Duration:    391,
			Bitrate:     320,
			Format:      FormatFLAC,
			FileSize:    35000000,
			FilePath:    filepath.Join(tmpDir, "track003.flac"),
		},
	}

	mgr.mu.Lock()
	for _, t2 := range tracks {
		mgr.addTrackToIndex(t2)
	}
	mgr.mu.Unlock()

	return mgr, func() {
		os.RemoveAll(tmpDir)
	}
}

// setupTestHandlers 创建测试用处理器.
func setupTestHandlers(t *testing.T) (*Handlers, *Manager, func()) {
	t.Helper()

	mgr, cleanup := setupTestManager(t)
	handlers := NewHandlers(mgr)

	return handlers, mgr, cleanup
}

// setupTestRouter 创建测试用路由.
func setupTestRouter(t *testing.T) (*gin.Engine, *Handlers, *Manager, func()) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	handlers, mgr, cleanup := setupTestHandlers(t)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return router, handlers, mgr, cleanup
}

// ========== 测试用例 ==========

// TestNewManager 测试创建管理器.
func TestNewManager(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	if mgr == nil {
		t.Fatal("管理器不应为nil")
	}

	if len(mgr.tracks) != 3 {
		t.Errorf("期望 3 个曲目，实际 %d", len(mgr.tracks))
	}
}

// TestListTracks 测试音乐库列表.
func TestListTracks(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 测试无过滤
	tracks, total := mgr.ListTracks(LibraryQuery{Page: 1, PerPage: 10})
	if total != 3 {
		t.Errorf("期望总曲目数 3，实际 %d", total)
	}
	if len(tracks) != 3 {
		t.Errorf("期望返回 3 个曲目，实际 %d", len(tracks))
	}

	// 测试搜索
	tracks, total = mgr.ListTracks(LibraryQuery{Search: "周杰伦", Page: 1, PerPage: 10})
	if total != 2 {
		t.Errorf("搜索'周杰伦'期望 2 个结果，实际 %d", total)
	}

	// 测试流派过滤
	tracks, total = mgr.ListTracks(LibraryQuery{Genre: "Rock", Page: 1, PerPage: 10})
	if total != 1 {
		t.Errorf("过滤'Rock'期望 1 个结果，实际 %d", total)
	}
	if total > 0 && tracks[0].Title != "Hotel California" {
		t.Errorf("期望曲目 'Hotel California'，实际 '%s'", tracks[0].Title)
	}

	// 测试分页
	tracks, total = mgr.ListTracks(LibraryQuery{Page: 1, PerPage: 2})
	if len(tracks) != 2 {
		t.Errorf("每页2条期望返回 2 个曲目，实际 %d", len(tracks))
	}
	if total != 3 {
		t.Errorf("总曲目数期望 3，实际 %d", total)
	}
}

// TestAlbums 测试专辑功能.
func TestAlbums(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 列出专辑
	albums := mgr.ListAlbums("", "")
	if len(albums) != 3 {
		t.Errorf("期望 3 个专辑，实际 %d", len(albums))
	}

	// 按艺术家过滤
	albums = mgr.ListAlbums("周杰伦", "")
	if len(albums) != 2 {
		t.Errorf("周杰伦的专辑期望 2 个，实际 %d", len(albums))
	}

	// 获取专辑详情
	if len(albums) > 0 {
		album, err := mgr.GetAlbum(albums[0].ID)
		if err != nil {
			t.Fatalf("获取专辑详情失败: %v", err)
		}
		if album.Tracks == nil {
			t.Error("专辑详情应包含曲目列表")
		}
	}
}

// TestArtists 测试艺术家列表.
func TestArtists(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	artists := mgr.ListArtists()
	if len(artists) != 2 {
		t.Errorf("期望 2 个艺术家，实际 %d", len(artists))
	}
}

// TestGenres 测试流派列表.
func TestGenres(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	genres := mgr.ListGenres()
	if len(genres) != 2 {
		t.Errorf("期望 2 个流派，实际 %d", len(genres))
	}
}

// TestFavorites 测试收藏功能.
func TestFavorites(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 初始无收藏
	favs := mgr.ListFavorites()
	if len(favs) != 0 {
		t.Errorf("初始收藏应为空，实际 %d", len(favs))
	}

	// 添加收藏
	isFav, err := mgr.ToggleFavorite("track_001")
	if err != nil {
		t.Fatalf("切换收藏失败: %v", err)
	}
	if !isFav {
		t.Error("应为收藏状态")
	}

	// 再次切换（取消收藏）
	isFav, err = mgr.ToggleFavorite("track_001")
	if err != nil {
		t.Fatalf("切换收藏失败: %v", err)
	}
	if isFav {
		t.Error("应为非收藏状态")
	}

	// 不存在的曲目
	_, err = mgr.ToggleFavorite("nonexistent")
	if err != ErrTrackNotFound {
		t.Errorf("期望 ErrTrackNotFound，实际 %v", err)
	}
}

// TestPlayQueue 测试播放队列.
func TestPlayQueue(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	player := NewPlayer(mgr)

	// 添加到队列
	err := player.AddToQueue([]string{"track_001", "track_002", "track_003"}, -1)
	if err != nil {
		t.Fatalf("添加到队列失败: %v", err)
	}

	if player.QueueLength() != 3 {
		t.Errorf("队列长度期望 3，实际 %d", player.QueueLength())
	}

	// 获取队列
	queue := player.GetQueue()
	if queue.TotalCount != 3 {
		t.Errorf("队列总数期望 3，实际 %d", queue.TotalCount)
	}
	if queue.Mode != PlayModeOrder {
		t.Errorf("播放模式期望 order，实际 %s", queue.Mode)
	}

	// 下一曲
	nextID, err := player.Next()
	if err != nil {
		t.Fatalf("下一曲失败: %v", err)
	}
	if nextID != "track_002" {
		t.Errorf("期望 track_002，实际 %s", nextID)
	}

	// 上一曲
	prevID, err := player.Prev()
	if err != nil {
		t.Fatalf("上一曲失败: %v", err)
	}
	if prevID != "track_001" {
		t.Errorf("期望 track_001，实际 %s", prevID)
	}

	// 移除队列项
	err = player.RemoveFromQueue(0)
	if err != nil {
		t.Fatalf("移除队列项失败: %v", err)
	}
	if player.QueueLength() != 2 {
		t.Errorf("队列长度期望 2，实际 %d", player.QueueLength())
	}

	// 设置随机模式
	player.SetMode(PlayModeRandom)
	if player.GetMode() != PlayModeRandom {
		t.Errorf("播放模式期望 random，实际 %s", player.GetMode())
	}
}

// TestPlaylists 测试播放列表.
func TestPlaylists(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建播放列表
	playlist, err := mgr.CreatePlaylist(PlaylistInput{
		Name:        "我的最爱",
		Description: "测试播放列表",
		TrackIDs:    []string{"track_001", "track_002"},
	})
	if err != nil {
		t.Fatalf("创建播放列表失败: %v", err)
	}
	if playlist.Name != "我的最爱" {
		t.Errorf("名称期望 '我的最爱'，实际 '%s'", playlist.Name)
	}
	if playlist.TrackCount != 2 {
		t.Errorf("曲目数期望 2，实际 %d", playlist.TrackCount)
	}

	// 获取播放列表
	got, err := mgr.GetPlaylist(playlist.ID)
	if err != nil {
		t.Fatalf("获取播放列表失败: %v", err)
	}
	if got.ID != playlist.ID {
		t.Errorf("ID不匹配: 期望 %s，实际 %s", playlist.ID, got.ID)
	}

	// 列出播放列表
	playlists := mgr.ListPlaylists()
	if len(playlists) != 1 {
		t.Errorf("播放列表数期望 1，实际 %d", len(playlists))
	}

	// 更新播放列表
	updated, err := mgr.UpdatePlaylist(playlist.ID, PlaylistInput{
		Name:        "更新后的列表",
		Description: "更新描述",
		TrackIDs:    []string{"track_001"},
	})
	if err != nil {
		t.Fatalf("更新播放列表失败: %v", err)
	}
	if updated.Name != "更新后的列表" {
		t.Errorf("更新后名称期望 '更新后的列表'，实际 '%s'", updated.Name)
	}

	// 删除播放列表
	err = mgr.DeletePlaylist(playlist.ID)
	if err != nil {
		t.Fatalf("删除播放列表失败: %v", err)
	}

	// 确认已删除
	_, err = mgr.GetPlaylist(playlist.ID)
	if err != ErrPlaylistNotFound {
		t.Errorf("期望 ErrPlaylistNotFound，实际 %v", err)
	}
}

// TestLibraryStats 测试音乐库统计.
func TestLibraryStats(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	stats := mgr.GetStats()
	if stats.TotalTracks != 3 {
		t.Errorf("总曲目数期望 3，实际 %d", stats.TotalTracks)
	}
	if stats.TotalAlbums != 3 {
		t.Errorf("总专辑数期望 3，实际 %d", stats.TotalAlbums)
	}
	if stats.TotalArtists != 2 {
		t.Errorf("总艺术家数期望 2，实际 %d", stats.TotalArtists)
	}
	if stats.TotalDuration != 925 {
		t.Errorf("总时长期望 925 秒，实际 %d", stats.TotalDuration)
	}
}

// TestRecentPlayed 测试最近播放.
func TestRecentPlayed(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 初始无播放记录
	recent := mgr.GetRecentPlayed(10)
	if len(recent) != 0 {
		t.Errorf("初始最近播放应为空，实际 %d", len(recent))
	}

	// 记录播放
	err := mgr.RecordPlay("track_001")
	if err != nil {
		t.Fatalf("记录播放失败: %v", err)
	}
	err = mgr.RecordPlay("track_003")
	if err != nil {
		t.Fatalf("记录播放失败: %v", err)
	}

	// 获取最近播放
	recent = mgr.GetRecentPlayed(10)
	if len(recent) != 2 {
		t.Errorf("最近播放数期望 2，实际 %d", len(recent))
	}

	// 最近播放应按时间倒序（track_003 最新）
	if len(recent) > 0 && recent[0].ID != "track_003" {
		t.Errorf("最近播放第一个应为 track_003，实际 %s", recent[0].ID)
	}

	// 重复播放同一首歌
	err = mgr.RecordPlay("track_001")
	if err != nil {
		t.Fatalf("重复记录播放失败: %v", err)
	}
	recent = mgr.GetRecentPlayed(10)
	if len(recent) != 2 {
		t.Errorf("重复播放后最近播放数应仍为 2，实际 %d", len(recent))
	}
	if len(recent) > 0 && recent[0].ID != "track_001" {
		t.Errorf("重复播放后第一个应为 track_001，实际 %s", recent[0].ID)
	}

	// 验证播放次数
	track, _ := mgr.GetTrack("track_001")
	if track.PlayCount != 2 {
		t.Errorf("track_001 播放次数期望 2，实际 %d", track.PlayCount)
	}
}

// TestDLNADevices 测试 DLNA 设备管理.
func TestDLNADevices(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 初始无设备
	devices := mgr.ListDLNADevices()
	if len(devices) != 0 {
		t.Errorf("初始设备应为空，实际 %d", len(devices))
	}

	// 注册设备
	mgr.RegisterDLNADevice(&DLNADevice{
		ID:       "dlna_001",
		Name:     "客厅音箱",
		Type:     "Speaker",
		IP:       "192.168.1.100",
		Port:     1400,
		IsOnline: true,
	})

	devices = mgr.ListDLNADevices()
	if len(devices) != 1 {
		t.Errorf("设备数期望 1，实际 %d", len(devices))
	}

	// 推送到设备
	err := mgr.CastToDLNA("dlna_001", "track_001")
	if err != nil {
		t.Fatalf("推送到DLNA失败: %v", err)
	}

	// 推送到不存在的设备
	err = mgr.CastToDLNA("nonexistent", "track_001")
	if err != ErrDLNADeviceNotFound {
		t.Errorf("期望 ErrDLNADeviceNotFound，实际 %v", err)
	}
}

// TestQueueReorder 测试队列重排序.
func TestQueueReorder(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	player := NewPlayer(mgr)

	// 添加到队列
	_ = player.AddToQueue([]string{"track_001", "track_002", "track_003"}, -1)

	// 重排序：将第一个移到最后一个
	err := player.ReorderQueue(0, 2)
	if err != nil {
		t.Fatalf("重排序失败: %v", err)
	}

	queue := player.GetQueue()
	if queue.Items[0].TrackID != "track_002" {
		t.Errorf("重排序后第一个应为 track_002，实际 %s", queue.Items[0].TrackID)
	}
	if queue.Items[1].TrackID != "track_003" {
		t.Errorf("重排序后第二个应为 track_003，实际 %s", queue.Items[1].TrackID)
	}
	if queue.Items[2].TrackID != "track_001" {
		t.Errorf("重排序后第三个应为 track_001，实际 %s", queue.Items[2].TrackID)
	}

	// 清空队列
	player.ClearQueue()
	if player.QueueLength() != 0 {
		t.Errorf("清空后期望队列长度 0，实际 %d", player.QueueLength())
	}
}

// TestConfigPersistence 测试配置持久化.
func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// 创建管理器并添加数据
	mgr1, err := NewManager(configPath, []string{tmpDir})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 手动添加曲目并保存
	mgr1.mu.Lock()
	mgr1.tracks["test_001"] = &Track{
		ID:       "test_001",
		Title:    "测试曲目",
		Artist:   "测试艺术家",
		Album:    "测试专辑",
		Duration: 180,
		Format:   FormatMP3,
		FilePath: "/test/path.mp3",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mgr1.mu.Unlock()

	if err := mgr1.saveConfig(); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("配置文件应存在")
	}

	// 读取并验证内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		t.Fatalf("解析配置文件失败: %v", err)
	}

	if len(pc.Tracks) != 1 {
		t.Errorf("配置中曲目数期望 1，实际 %d", len(pc.Tracks))
	}
	if pc.Tracks[0].Title != "测试曲目" {
		t.Errorf("曲目标题期望 '测试曲目'，实际 '%s'", pc.Tracks[0].Title)
	}
}

// TestAPIHandlers 测试 HTTP API handlers.
func TestAPIHandlers(t *testing.T) {
	router, _, _, cleanup := setupTestRouter(t)
	defer cleanup()

	// 测试 GET /api/v1/audiostation/library
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audiostation/library?page=1&per_page=10", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /library 期望 200，实际 %d", w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("响应码期望 0，实际 %d", resp.Code)
	}

	// 测试 GET /api/v1/audiostation/albums
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/albums", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /albums 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/artists
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/artists", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /artists 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/genres
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/genres", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /genres 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/library/stats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/library/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /library/stats 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/favorites
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/favorites", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /favorites 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/recent
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/recent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /recent 期望 200，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/queue
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/queue", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /queue 期望 200，实际 %d", w.Code)
	}

	// 测试 POST /api/v1/audiostation/playlists
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/audiostation/playlists",
		bytes.NewBufferString(`{"name":"测试列表","description":"测试描述"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST /playlists 期望 201，实际 %d", w.Code)
	}

	// 测试 GET /api/v1/audiostation/playlists
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/playlists", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /playlists 期望 200，实际 %d", w.Code)
	}
}

// TestSupportedFormats 测试支持格式判断.
func TestSupportedFormats(t *testing.T) {
	if !isSupportedFormat(".mp3") {
		t.Error(".mp3 应为支持的格式")
	}
	if !isSupportedFormat(".flac") {
		t.Error(".flac 应为支持的格式")
	}
	if isSupportedFormat(".txt") {
		t.Error(".txt 不应为支持的格式")
	}
	if isSupportedFormat(".xyz") {
		t.Error(".xyz 不应为支持的格式")
	}
}

// TestPlayerModes 测试播放模式.
func TestPlayerModes(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	player := NewPlayer(mgr)
	_ = player.AddToQueue([]string{"track_001", "track_002", "track_003"}, -1)

	// 顺序播放模式
	player.SetMode(PlayModeOrder)
	_, _ = player.JumpTo(0) // 从第一首开始
	second, err := player.Next()
	if err != nil {
		t.Fatalf("顺序播放下一曲失败: %v", err)
	}
	if second != "track_002" {
		t.Errorf("顺序播放第二首期望 track_002，实际 %s", second)
	}
	third, err := player.Next()
	if err != nil {
		t.Fatalf("顺序播放下一曲失败: %v", err)
	}
	if third != "track_003" {
		t.Errorf("顺序播放第三首期望 track_003，实际 %s", third)
	}
	// 到最后一首再Next应返回错误
	_, err = player.Next()
	if err != ErrQueueEmpty {
		t.Errorf("顺序播放超出队列应返回 ErrQueueEmpty，实际 %v", err)
	}

	// 单曲循环模式
	player.SetMode(PlayModeRepeatOne)
	_, _ = player.JumpTo(0)
	id1, _ := player.Next()
	id2, _ := player.Next()
	if id1 != id2 {
		t.Errorf("单曲循环应返回同一首: %s vs %s", id1, id2)
	}

	// 列表循环模式
	player.SetMode(PlayModeRepeatAll)
	_, _ = player.JumpTo(2)
	nextID, _ := player.Next()
	if nextID != "track_001" {
		t.Errorf("列表循环应循环到第一首: %s", nextID)
	}
}

// TestGetArtist 测试艺术家详情.
func TestGetArtist(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 列出艺术家获取 ID
	artists := mgr.ListArtists()
	if len(artists) == 0 {
		t.Fatal("期望有艺术家")
	}

	// 获取艺术家详情
	artist, err := mgr.GetArtist(artists[0].ID)
	if err != nil {
		t.Fatalf("获取艺术家详情失败: %v", err)
	}
	if artist.Name == "" {
		t.Error("艺术家名不应为空")
	}
	if artist.Albums == nil {
		t.Error("艺术家详情应包含专辑列表")
	}

	// 不存在的艺术家
	_, err = mgr.GetArtist("nonexistent")
	if err != ErrArtistNotFound {
		t.Errorf("期望 ErrArtistNotFound，实际 %v", err)
	}
}

// TestHLSStream 测试 HLS 流媒体.
func TestHLSStream(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建临时音频文件
	tmpDir := t.TempDir()
	for _, track := range mgr.tracks {
		os.MkdirAll(filepath.Dir(track.FilePath), 0750)
		os.WriteFile(track.FilePath, make([]byte, 1024), 0644)
	}

	hlsMgr := NewHLSManager(mgr, tmpDir, 10.0)

	// 创建 HLS 会话
	session, err := hlsMgr.CreateStream("track_001", "http://localhost:8080")
	if err != nil {
		t.Fatalf("创建 HLS 会话失败: %v", err)
	}
	if session.ID == "" {
		t.Error("会话 ID 不应为空")
	}
	if session.Playlist == nil {
		t.Fatal("播放列表不应为 nil")
	}
	if len(session.Playlist.Segments) == 0 {
		t.Error("应有切片")
	}
	if session.Playlist.MasterURL == "" {
		t.Error("Master URL 不应为空")
	}

	// 获取会话
	got, err := hlsMgr.GetStream(session.ID)
	if err != nil {
		t.Fatalf("获取 HLS 会话失败: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("会话 ID 不匹配")
	}

	// 生成 Master Playlist
	master, err := hlsMgr.GenerateMasterPlaylist(session.ID)
	if err != nil {
		t.Fatalf("生成 Master Playlist 失败: %v", err)
	}
	if !strings.Contains(master, "#EXTM3U") {
		t.Error("Master Playlist 应包含 #EXTM3U")
	}
	if !strings.Contains(master, "#EXT-X-STREAM-INF") {
		t.Error("Master Playlist 应包含 #EXT-X-STREAM-INF")
	}

	// 生成 Media Playlist
	media, err := hlsMgr.GenerateMediaPlaylist(session.ID)
	if err != nil {
		t.Fatalf("生成 Media Playlist 失败: %v", err)
	}
	if !strings.Contains(media, "#EXTM3U") {
		t.Error("Media Playlist 应包含 #EXTM3U")
	}
	if !strings.Contains(media, "#EXT-X-ENDLIST") {
		t.Error("Media Playlist 应包含 #EXT-X-ENDLIST")
	}
	if !strings.Contains(media, "#EXTINF") {
		t.Error("Media Playlist 应包含 #EXTINF")
	}

	// 不存在的曲目
	_, err = hlsMgr.CreateStream("nonexistent", "http://localhost:8080")
	if err == nil {
		t.Error("不存在的曲目应返回错误")
	}

	// 验证活跃会话数
	if hlsMgr.GetActiveSessions() != 1 {
		t.Errorf("活跃会话数期望 1，实际 %d", hlsMgr.GetActiveSessions())
	}
}

// TestTagEditor 测试标签编辑器.
func TestTagEditor(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	tagEditor := NewTagEditor(mgr)

	// 更新标签
	track, err := tagEditor.UpdateTrackTag("track_001", TagUpdateRequest{
		Title:  "新标题",
		Artist: "新艺术家",
	})
	if err != nil {
		t.Fatalf("更新标签失败: %v", err)
	}
	if track.Title != "新标题" {
		t.Errorf("标题期望 '新标题'，实际 '%s'\n", track.Title)
	}
	if track.Artist != "新艺术家" {
		t.Errorf("艺术家期望 '新艺术家'，实际 '%s'\n", track.Artist)
	}

	// 验证更新后的数据
	got, _ := mgr.GetTrack("track_001")
	if got.Title != "新标题" {
		t.Errorf("获取更新后的标题期望 '新标题'，实际 '%s'\n", got.Title)
	}

	// 不存在的曲目
	_, err = tagEditor.UpdateTrackTag("nonexistent", TagUpdateRequest{Title: "test"})
	if err != ErrTrackNotFound {
		t.Errorf("期望 ErrTrackNotFound，实际 %v", err)
	}

	// 批量更新
	tracks, err := tagEditor.BatchUpdateTags(
		[]string{"track_002", "track_003"},
		TagUpdateRequest{Genre: "NewGenre"},
	)
	if err != nil {
		t.Fatalf("批量更新失败: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("批量更新后期望 2 个曲目，实际 %d", len(tracks))
	}
	for _, tr := range tracks {
		if tr.Genre != "NewGenre" {
			t.Errorf("流派期望 'NewGenre'，实际 '%s'\n", tr.Genre)
		}
	}
}

// TestSharePlaylist 测试分享播放列表.
func TestSharePlaylist(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建播放列表
	playlist, err := mgr.CreatePlaylist(PlaylistInput{
		Name:     "分享测试",
		TrackIDs: []string{"track_001"},
	})
	if err != nil {
		t.Fatalf("创建播放列表失败: %v", err)
	}

	// 分享播放列表
	shared, err := mgr.SharePlaylist(playlist.ID, 24)
	if err != nil {
		t.Fatalf("分享播放列表失败: %v", err)
	}
	if shared.ShareToken == "" {
		t.Error("分享令牌不应为空")
	}
	if shared.PlaylistID != playlist.ID {
		t.Errorf("播放列表 ID 不匹配")
	}
	if shared.ExpiresAt.Before(time.Now()) {
		t.Error("过期时间应在未来")
	}

	// 不存在的播放列表
	_, err = mgr.SharePlaylist("nonexistent", 24)
	if err != ErrPlaylistNotFound {
		t.Errorf("期望 ErrPlaylistNotFound，实际 %v", err)
	}
}

// TestHLSAPIHandlers 测试 HLS API handlers.
func TestHLSAPIHandlers(t *testing.T) {
	router, _, mgr, cleanup := setupTestRouter(t)
	defer cleanup()

	// 创建临时音频文件
	for _, track := range mgr.tracks {
		os.MkdirAll(filepath.Dir(track.FilePath), 0750)
		os.WriteFile(track.FilePath, make([]byte, 1024), 0644)
	}

	// POST /api/v1/audiostation/hls/stream/:id
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/audiostation/hls/stream/track_001", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST /hls/stream/track_001 期望 200，实际 %d", w.Code)
	}

	// 解析响应获取 session ID
	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// GET /api/v1/audiostation/hls/status
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/audiostation/hls/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /hls/status 期望 200，实际 %d", w.Code)
	}

	// POST /hls/stream/:id 不存在的曲目
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/audiostation/hls/stream/nonexistent", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("POST /hls/stream/nonexistent 期望 404，实际 %d", w.Code)
	}
}

// TestTagEditorAPIHandlers 测试标签编辑 API handlers.
func TestTagEditorAPIHandlers(t *testing.T) {
	router, _, _, cleanup := setupTestRouter(t)
	defer cleanup()

	// PUT /api/v1/audiostation/tracks/:id/tag
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/audiostation/tracks/track_001/tag",
		bytes.NewBufferString(`{"title":"API更新","artist":"API艺术家"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT /tracks/track_001/tag 期望 200，实际 %d", w.Code)
	}

	// PUT /tracks/:id/tag 不存在的曲目
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/audiostation/tracks/nonexistent/tag",
		bytes.NewBufferString(`{"title":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("PUT /tracks/nonexistent/tag 期望 404，实际 %d", w.Code)
	}
}

// TestSharePlaylistAPIHandlers 测试分享 API handlers.
func TestSharePlaylistAPIHandlers(t *testing.T) {
	router, _, mgr, cleanup := setupTestRouter(t)
	defer cleanup()

	// 先创建播放列表
	playlist, _ := mgr.CreatePlaylist(PlaylistInput{
		Name:     "API分享测试",
		TrackIDs: []string{"track_001"},
	})

	// POST /api/v1/audiostation/playlists/:id/share
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/audiostation/playlists/"+playlist.ID+"/share",
		bytes.NewBufferString(`{"expire_hours":48}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST /playlists/:id/share 期望 200，实际 %d", w.Code)
	}

	// 分享不存在的播放列表
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/audiostation/playlists/nonexistent/share",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("POST /playlists/nonexistent/share 期望 404，实际 %d", w.Code)
	}
}
