// Package hometheater 家庭影院系统测试
package hometheater

import (
	"testing"
	"time"
)

// ========== Engine 测试 ==========

func TestEngine_StartStop(t *testing.T) {
	engine := NewEngine()

	t.Run("启动引擎", func(t *testing.T) {
		if err := engine.Start(); err != nil {
			t.Fatalf("启动引擎失败: %v", err)
		}
		if !engine.IsRunning() {
			t.Fatal("引擎应该处于运行状态")
		}
	})

	t.Run("重复启动", func(t *testing.T) {
		if err := engine.Start(); err != nil {
			t.Fatalf("重复启动不应报错: %v", err)
		}
	})

	t.Run("停止引擎", func(t *testing.T) {
		if err := engine.Stop(); err != nil {
			t.Fatalf("停止引擎失败: %v", err)
		}
		if engine.IsRunning() {
			t.Fatal("引擎应该处于停止状态")
		}
	})
}

func TestEngine_LibraryManagement(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	t.Run("添加媒体库", func(t *testing.T) {
		lib := &MediaLibrary{
			ID:   "lib1",
			Name: "电影库",
			Path: "/media/movies",
			Type: MediaTypeMovie,
		}
		if err := engine.AddLibrary(lib); err != nil {
			t.Fatalf("添加媒体库失败: %v", err)
		}
	})

	t.Run("重复添加", func(t *testing.T) {
		lib := &MediaLibrary{
			ID:   "lib1",
			Name: "重复库",
			Path: "/media/movies",
			Type: MediaTypeMovie,
		}
		if err := engine.AddLibrary(lib); err == nil {
			t.Fatal("重复添加应该报错")
		}
	})

	t.Run("获取媒体库", func(t *testing.T) {
		lib, err := engine.GetLibrary("lib1")
		if err != nil {
			t.Fatalf("获取媒体库失败: %v", err)
		}
		if lib.Name != "电影库" {
			t.Fatalf("期望电影库，实际: %s", lib.Name)
		}
	})

	t.Run("列出媒体库", func(t *testing.T) {
		libs := engine.ListLibraries()
		if len(libs) != 1 {
			t.Fatalf("期望1个媒体库，实际: %d", len(libs))
		}
	})

	t.Run("删除媒体库", func(t *testing.T) {
		if err := engine.RemoveLibrary("lib1"); err != nil {
			t.Fatalf("删除媒体库失败: %v", err)
		}
		if _, err := engine.GetLibrary("lib1"); err == nil {
			t.Fatal("删除后应该找不到媒体库")
		}
	})
}

func TestEngine_MovieManagement(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	// 添加媒体库
	lib := &MediaLibrary{ID: "lib1", Name: "电影库", Path: "/media/movies", Type: MediaTypeMovie}
	engine.AddLibrary(lib)

	t.Run("添加电影", func(t *testing.T) {
		movie := &Movie{
			ID:        "movie1",
			LibraryID: "lib1",
			Title:     "测试电影",
			Year:      2024,
			Runtime:   120,
			FilePath:  "/media/movies/test.mkv",
			FileSize:  1024 * 1024 * 1024,
		}
		if err := engine.AddMovie(movie); err != nil {
			t.Fatalf("添加电影失败: %v", err)
		}
	})

	t.Run("获取电影", func(t *testing.T) {
		movie, err := engine.GetMovie("movie1")
		if err != nil {
			t.Fatalf("获取电影失败: %v", err)
		}
		if movie.Title != "测试电影" {
			t.Fatalf("期望测试电影，实际: %s", movie.Title)
		}
	})

	t.Run("更新电影", func(t *testing.T) {
		movie, _ := engine.GetMovie("movie1")
		movie.Rating = 8.5
		if err := engine.UpdateMovie(movie); err != nil {
			t.Fatalf("更新电影失败: %v", err)
		}
		updated, _ := engine.GetMovie("movie1")
		if updated.Rating != 8.5 {
			t.Fatalf("评分应为8.5，实际: %f", updated.Rating)
		}
	})

	t.Run("列出电影", func(t *testing.T) {
		movies := engine.ListMovies()
		if len(movies) != 1 {
			t.Fatalf("期望1部电影，实际: %d", len(movies))
		}
	})

	t.Run("删除电影", func(t *testing.T) {
		if err := engine.RemoveMovie("movie1"); err != nil {
			t.Fatalf("删除电影失败: %v", err)
		}
	})
}

func TestEngine_TVShowManagement(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	t.Run("添加剧集", func(t *testing.T) {
		show := &TVShow{
			ID:    "show1",
			Title: "测试剧集",
			Year:  2024,
		}
		if err := engine.AddTVShow(show); err != nil {
			t.Fatalf("添加剧集失败: %v", err)
		}
	})

	t.Run("添加剧集分集", func(t *testing.T) {
		episode := &Episode{
			ID:            "ep1",
			ShowID:        "show1",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			Title:         "第一集",
			Runtime:       45,
		}
		if err := engine.AddEpisode(episode); err != nil {
			t.Fatalf("添加剧集分集失败: %v", err)
		}
	})

	t.Run("获取剧集", func(t *testing.T) {
		show, err := engine.GetTVShow("show1")
		if err != nil {
			t.Fatalf("获取剧集失败: %v", err)
		}
		if show.EpisodeCount != 1 {
			t.Fatalf("期望1集，实际: %d", show.EpisodeCount)
		}
	})
}

func TestEngine_PlaylistManagement(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	t.Run("创建播放列表", func(t *testing.T) {
		pl := &Playlist{
			ID:     "pl1",
			Name:   "我的列表",
			UserID: "user1",
		}
		if err := engine.CreatePlaylist(pl); err != nil {
			t.Fatalf("创建播放列表失败: %v", err)
		}
	})

	t.Run("重复创建", func(t *testing.T) {
		pl := &Playlist{ID: "pl1", Name: "重复列表"}
		if err := engine.CreatePlaylist(pl); err == nil {
			t.Fatal("重复创建应该报错")
		}
	})

	t.Run("添加媒体到播放列表", func(t *testing.T) {
		item := &MediaItem{
			ID:    "movie1",
			Type:  MediaTypeMovie,
			Title: "测试电影",
		}
		if err := engine.AddToPlaylist("pl1", item); err != nil {
			t.Fatalf("添加媒体到播放列表失败: %v", err)
		}
	})

	t.Run("获取播放列表", func(t *testing.T) {
		pl, err := engine.GetPlaylist("pl1")
		if err != nil {
			t.Fatalf("获取播放列表失败: %v", err)
		}
		if len(pl.Items) != 1 {
			t.Fatalf("期望1个媒体项，实际: %d", len(pl.Items))
		}
	})

	t.Run("列出播放列表", func(t *testing.T) {
		playlists := engine.ListPlaylists("user1")
		if len(playlists) != 1 {
			t.Fatalf("期望1个播放列表，实际: %d", len(playlists))
		}
	})
}

func TestEngine_WatchProgress(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	// 添加电影
	movie := &Movie{ID: "movie1", Title: "测试电影", Runtime: 120}
	engine.AddMovie(movie)

	t.Run("更新观看进度", func(t *testing.T) {
		progress := &WatchProgress{
			Position:    3600,
			Duration:    7200,
			Percentage:  50,
			Completed:   false,
			LastUpdated: time.Now(),
		}
		if err := engine.UpdateWatchProgress("movie1", progress); err != nil {
			t.Fatalf("更新观看进度失败: %v", err)
		}
	})

	t.Run("获取继续观看", func(t *testing.T) {
		items := engine.GetContinueWatching("", 10)
		if len(items) != 1 {
			t.Fatalf("期望1个继续观看项，实际: %d", len(items))
		}
	})
}

func TestEngine_UserConfig(t *testing.T) {
	engine := NewEngine()

	t.Run("设置用户配置", func(t *testing.T) {
		config := &UserConfig{
			UserID:          "user1",
			PreferredLang:   "zh",
			SubtitleEnabled: true,
			SubtitleLang:    "zh",
			AutoPlay:        true,
		}
		if err := engine.SetUserConfig(config); err != nil {
			t.Fatalf("设置用户配置失败: %v", err)
		}
	})

	t.Run("获取用户配置", func(t *testing.T) {
		config, err := engine.GetUserConfig("user1")
		if err != nil {
			t.Fatalf("获取用户配置失败: %v", err)
		}
		if config.PreferredLang != "zh" {
			t.Fatalf("期望zh，实际: %s", config.PreferredLang)
		}
	})

	t.Run("获取默认配置", func(t *testing.T) {
		config, err := engine.GetUserConfig("nonexistent")
		if err != nil {
			t.Fatalf("获取默认配置失败: %v", err)
		}
		if config.SubtitleEnabled != true {
			t.Fatal("默认配置应启用字幕")
		}
	})
}

// ========== Scanner 测试 ==========

func TestScanner_ParseMovieName(t *testing.T) {
	scanner := NewScanner(NewEngine())

	tests := []struct {
		name      string
		filename  string
		wantTitle string
		wantYear  int
	}{
		{"带年份", "The.Matrix.1999.mkv", "The.Matrix", 1999},
		{"带括号年份", "Inception (2010).mp4", "Inception", 2010},
		{"无年份", "MyMovie.mkv", "MyMovie", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year := scanner.ParseMovieName(tt.filename)
			if title != tt.wantTitle {
				t.Errorf("标题: 期望 %q, 实际 %q", tt.wantTitle, title)
			}
			if year != tt.wantYear {
				t.Errorf("年份: 期望 %d, 实际 %d", tt.wantYear, year)
			}
		})
	}
}

func TestScanner_ParseEpisodeName(t *testing.T) {
	scanner := NewScanner(NewEngine())

	tests := []struct {
		name       string
		filename   string
		wantShow   string
		wantSeason int
		wantEp     int
		wantErr    bool
	}{
		{"标准S01E01", "Breaking.Bad.S01E01.mkv", "Breaking.Bad.", 1, 1, false},
		{"带空格", "Game of Thrones S01E01.mp4", "Game of Thrones", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			show, season, ep, err := scanner.ParseEpisodeName(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatal("应该返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if show != tt.wantShow {
				t.Errorf("剧名: 期望 %q, 实际 %q", tt.wantShow, show)
			}
			if season != tt.wantSeason {
				t.Errorf("季: 期望 %d, 实际 %d", tt.wantSeason, season)
			}
			if ep != tt.wantEp {
				t.Errorf("集: 期望 %d, 实际 %d", tt.wantEp, ep)
			}
		})
	}
}

func TestScanner_SearchTMDB(t *testing.T) {
	engine := NewEngine()
	scanner := NewScanner(engine)

	t.Run("TMDB未配置", func(t *testing.T) {
		_, err := scanner.SearchTMDB("Test", MediaTypeMovie)
		if err == nil {
			t.Fatal("TMDB未配置应返回错误")
		}
	})

	t.Run("搜索电影", func(t *testing.T) {
		scanner.SetTMDBKey("test_key")
		result, err := scanner.SearchTMDB("Test Movie", MediaTypeMovie)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		if result == nil {
			t.Fatal("结果不应为空")
		}
	})

	t.Run("搜索剧集", func(t *testing.T) {
		scanner.SetTMDBKey("test_key")
		result, err := scanner.SearchTMDB("Test Show", MediaTypeTVShow)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		if result == nil {
			t.Fatal("结果不应为空")
		}
	})
}

// ========== Transcoder 测试 ==========

func TestTranscoder_SubmitJob(t *testing.T) {
	engine := NewEngine()
	transcoder := NewTranscoder(engine)

	// 添加转码配置
	profile := &TranscodeProfile{
		ID:           "p1",
		Name:         "1080p",
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5000,
		AudioBitrate: 128,
	}
	engine.AddTranscodeProfile(profile)

	t.Run("提交转码任务", func(t *testing.T) {
		req := &TranscodeRequest{
			MediaID:    "movie1",
			ProfileID:  "p1",
			InputPath:  "/input.mkv",
			OutputPath: "/output.mp4",
		}
		job, err := transcoder.SubmitJob(req)
		if err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}
		if job.ID == "" {
			t.Fatal("任务ID不应为空")
		}
	})

	t.Run("获取任务", func(t *testing.T) {
		jobs := transcoder.ListJobs("")
		if len(jobs) == 0 {
			t.Fatal("应该有任务")
		}
		job, err := transcoder.GetJob(jobs[0].ID)
		if err != nil {
			t.Fatalf("获取任务失败: %v", err)
		}
		if job.MediaID != "movie1" {
			t.Fatalf("期望movie1，实际: %s", job.MediaID)
		}
	})
}

func TestTranscoder_GetOptimalProfile(t *testing.T) {
	engine := NewEngine()
	transcoder := NewTranscoder(engine)

	t.Run("4K视频降级", func(t *testing.T) {
		videoInfo := &VideoInfo{
			Width:  3840,
			Height: 2160,
			Codec:  CodecH265,
		}
		profile := transcoder.GetOptimalProfile(videoInfo, 1920, 1080)
		if profile == nil {
			t.Fatal("4K视频需要转码")
		}
		if profile.Width > 1920 {
			t.Fatalf("宽度应<=1920，实际: %d", profile.Width)
		}
	})

	t.Run("720p视频不需要转码", func(t *testing.T) {
		videoInfo := &VideoInfo{
			Width:  1280,
			Height: 720,
			Codec:  CodecH264,
		}
		profile := transcoder.GetOptimalProfile(videoInfo, 1920, 1080)
		if profile != nil {
			t.Fatal("720p H264不需要转码")
		}
	})
}

// ========== Streamer 测试 ==========

func TestStreamer_CreateSession(t *testing.T) {
	engine := NewEngine()
	engine.Start()
	transcoder := NewTranscoder(engine)
	streamer := NewStreamer(engine, transcoder)

	// 添加测试电影
	movie := &Movie{
		ID:       "movie1",
		Title:    "测试电影",
		Runtime:  120,
		FilePath: "/media/test.mkv",
	}
	engine.AddMovie(movie)

	t.Run("创建HLS会话", func(t *testing.T) {
		req := &StreamRequest{
			MediaID:  "movie1",
			UserID:   "user1",
			Protocol: ProtocolHLS,
			Quality:  Quality1080p,
		}
		session, err := streamer.CreateSession(req)
		if err != nil {
			t.Fatalf("创建会话失败: %v", err)
		}
		if session.ID == "" {
			t.Fatal("会话ID不应为空")
		}
		if session.Protocol != ProtocolHLS {
			t.Fatalf("期望HLS协议，实际: %s", session.Protocol)
		}
	})

	t.Run("创建DASH会话", func(t *testing.T) {
		req := &StreamRequest{
			MediaID:  "movie1",
			UserID:   "user1",
			Protocol: ProtocolDASH,
			Quality:  Quality720p,
		}
		session, err := streamer.CreateSession(req)
		if err != nil {
			t.Fatalf("创建会话失败: %v", err)
		}
		if session.Protocol != ProtocolDASH {
			t.Fatalf("期望DASH协议，实际: %s", session.Protocol)
		}
	})

	t.Run("获取会话", func(t *testing.T) {
		sessions := streamer.ListSessions("user1")
		if len(sessions) < 2 {
			t.Fatalf("期望至少2个会话，实际: %d", len(sessions))
		}
	})
}

func TestStreamer_SessionControl(t *testing.T) {
	engine := NewEngine()
	engine.Start()
	transcoder := NewTranscoder(engine)
	streamer := NewStreamer(engine, transcoder)

	movie := &Movie{ID: "movie1", Title: "测试", Runtime: 120, FilePath: "/test.mkv"}
	engine.AddMovie(movie)

	req := &StreamRequest{
		MediaID:  "movie1",
		UserID:   "user1",
		Protocol: ProtocolHLS,
	}
	session, _ := streamer.CreateSession(req)

	t.Run("暂停", func(t *testing.T) {
		if err := streamer.PauseSession(session.ID); err != nil {
			t.Fatalf("暂停失败: %v", err)
		}
		s, _ := streamer.GetSession(session.ID)
		if s.State != PlaybackPaused {
			t.Fatalf("期望暂停状态，实际: %s", s.State)
		}
	})

	t.Run("恢复", func(t *testing.T) {
		if err := streamer.ResumeSession(session.ID); err != nil {
			t.Fatalf("恢复失败: %v", err)
		}
		s, _ := streamer.GetSession(session.ID)
		if s.State != PlaybackPlaying {
			t.Fatalf("期望播放状态，实际: %s", s.State)
		}
	})

	t.Run("跳转", func(t *testing.T) {
		if err := streamer.SeekSession(session.ID, 3600); err != nil {
			t.Fatalf("跳转失败: %v", err)
		}
		s, _ := streamer.GetSession(session.ID)
		if s.Position != 3600 {
			t.Fatalf("期望位置3600，实际: %f", s.Position)
		}
	})

	t.Run("心跳", func(t *testing.T) {
		if err := streamer.Heartbeat(session.ID, 3700); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
	})

	t.Run("结束会话", func(t *testing.T) {
		if err := streamer.EndSession(session.ID); err != nil {
			t.Fatalf("结束会话失败: %v", err)
		}
		if _, err := streamer.GetSession(session.ID); err == nil {
			t.Fatal("会话应该已删除")
		}
	})
}

func TestStreamer_DLNADevice(t *testing.T) {
	engine := NewEngine()
	transcoder := NewTranscoder(engine)
	streamer := NewStreamer(engine, transcoder)

	t.Run("注册DLNA设备", func(t *testing.T) {
		device := &DLNADevice{
			ID:   "dlna1",
			Name: "客厅电视",
			Type: "SmartTV",
		}
		streamer.RegisterDLNADevice(device)
		devices := streamer.ListDLNADevices()
		if len(devices) != 1 {
			t.Fatalf("期望1个设备，实际: %d", len(devices))
		}
	})

	t.Run("设备注销", func(t *testing.T) {
		streamer.UnregisterDLNADevice("dlna1")
		devices := streamer.ListDLNADevices()
		if devices[0].Online {
			t.Fatal("设备应该离线")
		}
	})
}

func TestStreamer_HLSPlaylist(t *testing.T) {
	engine := NewEngine()
	engine.Start()
	transcoder := NewTranscoder(engine)
	streamer := NewStreamer(engine, transcoder)

	movie := &Movie{ID: "movie1", Title: "测试", Runtime: 120, FilePath: "/test.mkv"}
	engine.AddMovie(movie)

	req := &StreamRequest{
		MediaID:  "movie1",
		UserID:   "user1",
		Protocol: ProtocolHLS,
	}
	session, _ := streamer.CreateSession(req)

	t.Run("获取HLS播放列表", func(t *testing.T) {
		playlist, err := streamer.GetHLSPlaylist(session.ID)
		if err != nil {
			t.Fatalf("获取播放列表失败: %v", err)
		}
		if playlist == "" {
			t.Fatal("播放列表不应为空")
		}
	})
}

func TestStreamer_BandwidthEstimation(t *testing.T) {
	engine := NewEngine()
	transcoder := NewTranscoder(engine)
	streamer := NewStreamer(engine, transcoder)

	t.Run("估计带宽", func(t *testing.T) {
		bandwidth := streamer.EstimateBandwidth("session1", 1000000, time.Second)
		if bandwidth != 8000000 {
			t.Fatalf("期望8000000 bps，实际: %d", bandwidth)
		}
	})

	t.Run("推荐画质", func(t *testing.T) {
		quality := streamer.GetRecommendedQuality("session1")
		if quality == "" {
			t.Fatal("推荐画质不应为空")
		}
	})
}

// ========== 辅助函数测试 ==========

func TestCalculateScale(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		maxW       int
		maxH       int
		wantW      int
		wantH      int
	}{
		{"不需要缩放", 1280, 720, 1920, 1080, 1280, 720},
		{"宽度超限", 3840, 2160, 1920, 1080, 1920, 1080},
		{"高度超限", 1920, 2160, 1920, 1080, 960, 1080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := calculateScale(tt.width, tt.height, tt.maxW, tt.maxH)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("期望 %dx%d，实际 %dx%d", tt.wantW, tt.wantH, w, h)
			}
		})
	}
}

func TestCalculateBitrate(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   int
	}{
		{"4K", 3840, 2160, 20000},
		{"1080p", 1920, 1080, 5000},
		{"720p", 1280, 720, 2500},
		{"480p", 854, 480, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitrate := calculateBitrate(tt.width, tt.height)
			if bitrate != tt.want {
				t.Errorf("期望 %d kbps，实际 %d kbps", tt.want, bitrate)
			}
		})
	}
}

func TestGetSupportedCodecs(t *testing.T) {
	codecs := GetSupportedCodecs()
	if len(codecs["video"]) == 0 {
		t.Fatal("视频编码列表不应为空")
	}
	if len(codecs["audio"]) == 0 {
		t.Fatal("音频编码列表不应为空")
	}
}

func TestGetHWAccelCapabilities(t *testing.T) {
	caps := GetHWAccelCapabilities()
	if !caps[AccelNVENC] {
		t.Fatal("应支持NVENC")
	}
	if !caps[AccelVAAPI] {
		t.Fatal("应支持VAAPI")
	}
}
