package cinemarec

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine 返回 nil")
	}
}

func TestAddToLibrary(t *testing.T) {
	e := NewEngine()
	e.AddToLibrary(&MediaItem{
		ID: "m1", Title: "盗梦空间", Type: MediaMovie,
		Genres: []string{"科幻", "动作"}, Year: 2010, Rating: 9.3,
	})

	if len(e.library) != 1 {
		t.Errorf("媒体库应有 1 条, 实际 %d", len(e.library))
	}
}

func TestRecordWatch(t *testing.T) {
	e := NewEngine()
	e.RecordWatch(&WatchHistory{
		ID: "h1", UserID: "u1", MediaID: "m1",
		MediaType: MediaMovie, Title: "盗梦空间",
		Genres: []string{"科幻"}, WatchedAt: time.Now(),
		Duration: 9000, Position: 7200, Completed: false, Rating: 5,
	})

	profile := e.GetUserProfile()
	if profile.TotalWatched != 1 {
		t.Errorf("观看数应为 1, 实际 %d", profile.TotalWatched)
	}
	if profile.PreferredGenres["科幻"] != 1 {
		t.Error("应记录科幻偏好")
	}
}

func TestRecommendations(t *testing.T) {
	e := NewEngine()

	// 添加媒体库
	e.AddToLibrary(&MediaItem{ID: "m1", Title: "盗梦空间", Type: MediaMovie, Genres: []string{"科幻"}, Year: 2010, Rating: 9.3})
	e.AddToLibrary(&MediaItem{ID: "m2", Title: "星际穿越", Type: MediaMovie, Genres: []string{"科幻"}, Year: 2014, Rating: 9.5})
	e.AddToLibrary(&MediaItem{ID: "m3", Title: "功夫", Type: MediaMovie, Genres: []string{"喜剧"}, Year: 2004, Rating: 8.5})

	// 记录观看历史
	e.RecordWatch(&WatchHistory{
		ID: "h1", MediaID: "m0", MediaType: MediaMovie,
		Title: "流浪地球", Genres: []string{"科幻"},
		WatchedAt: time.Now(), Duration: 9000, Position: 9000,
		Completed: true, Rating: 5,
	})

	recs := e.GetRecommendations(10)
	if len(recs) == 0 {
		t.Error("应返回推荐")
	}
}

func TestContinueWatching(t *testing.T) {
	e := NewEngine()
	e.AddToLibrary(&MediaItem{ID: "m1", Title: "盗梦空间", Type: MediaMovie, Rating: 9.0})
	e.RecordWatch(&WatchHistory{
		ID: "h1", MediaID: "m1", MediaType: MediaMovie,
		Title: "盗梦空间", WatchedAt: time.Now(),
		Duration: 9000, Position: 4500, Completed: false,
	})

	recs := e.GetRecommendations(10)
	found := false
	for _, r := range recs {
		if r.Category == RecContinueWatching && r.Media != nil && r.Media.ID == "m1" {
			found = true
		}
	}
	if !found {
		t.Error("应包含继续观看推荐")
	}
}

func TestDedup(t *testing.T) {
	e := NewEngine()
	recs := []*Recommendation{
		{Media: &MediaItem{ID: "m1"}, Score: 0.9},
		{Media: &MediaItem{ID: "m1"}, Score: 0.8},
		{Media: &MediaItem{ID: "m2"}, Score: 0.7},
	}
	result := e.dedup(recs)
	if len(result) != 2 {
		t.Errorf("去重后应为 2 条, 实际 %d", len(result))
	}
}

func TestFormatRecommendations(t *testing.T) {
	e := NewEngine()
	output := e.FormatRecommendations(nil)
	if output == "" {
		t.Error("空推荐应有默认提示")
	}

	e.AddToLibrary(&MediaItem{ID: "m1", Title: "盗梦空间", Year: 2010, Rating: 9.3})
	recs := e.GetRecommendations(5)
	output = e.FormatRecommendations(recs)
	if output == "暂无推荐，请先添加媒体和观看历史" {
		t.Error("有推荐时不应返回空提示")
	}
}
