package smartgallery

import (
	"fmt"
	"testing"
	"time"
)

func TestNewGallery(t *testing.T) {
	gallery := NewGallery(nil)
	if gallery == nil {
		t.Fatal("expected non-nil gallery")
	}
	if gallery.Count() != 0 {
		t.Errorf("expected 0 items, got %d", gallery.Count())
	}
}

func TestAddAndGet(t *testing.T) {
	gallery := NewGallery(nil)

	item := &MediaItem{
		ID:       "test1",
		Title:    "Test Movie",
		Type:     MediaTypeMovie,
		Year:     2024,
		FilePath: "/data/movies/test.mp4",
	}

	if err := gallery.Add(item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gallery.Count() != 1 {
		t.Errorf("expected 1 item, got %d", gallery.Count())
	}

	got, exists := gallery.Get("test1")
	if !exists {
		t.Fatal("expected item to exist")
	}
	if got.Title != "Test Movie" {
		t.Errorf("expected title 'Test Movie', got %q", got.Title)
	}
}

func TestAddDuplicate(t *testing.T) {
	gallery := NewGallery(nil)

	item := &MediaItem{
		ID:       "test1",
		Title:    "Test Movie",
		FilePath: "/data/movies/test.mp4",
	}

	gallery.Add(item)

	item2 := &MediaItem{
		ID:       "test2",
		Title:    "Another Movie",
		FilePath: "/data/movies/test.mp4", // 相同路径
	}

	if err := gallery.Add(item2); err == nil {
		t.Error("expected error for duplicate path")
	}
}

func TestDelete(t *testing.T) {
	gallery := NewGallery(nil)

	item := &MediaItem{
		ID:       "test1",
		Title:    "Test Movie",
		FilePath: "/data/movies/test.mp4",
	}

	gallery.Add(item)

	if err := gallery.Delete("test1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gallery.Count() != 0 {
		t.Errorf("expected 0 items, got %d", gallery.Count())
	}
}

func TestListWithFilter(t *testing.T) {
	gallery := NewGallery(nil)

	gallery.Add(&MediaItem{
		ID:       "movie1",
		Title:    "Movie 1",
		Type:     MediaTypeMovie,
		FilePath: "/data/movies/movie1.mp4",
	})

	gallery.Add(&MediaItem{
		ID:       "tv1",
		Title:    "TV Show 1",
		Type:     MediaTypeTVShow,
		FilePath: "/data/tv/tv1.mp4",
	})

	// 过滤电影
	mediaType := MediaTypeMovie
	filter := &MediaFilter{Type: &mediaType}
	items := gallery.List(filter)

	if len(items) != 1 {
		t.Errorf("expected 1 movie, got %d", len(items))
	}
	if items[0].Type != MediaTypeMovie {
		t.Errorf("expected movie type, got %v", items[0].Type)
	}
}

func TestGetRecent(t *testing.T) {
	gallery := NewGallery(nil)

	for i := 0; i < 5; i++ {
		gallery.Add(&MediaItem{
			ID:        fmt.Sprintf("item%d", i),
			Title:     fmt.Sprintf("Item %d", i),
			FilePath:  fmt.Sprintf("/data/item%d.mp4", i),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	recent := gallery.GetRecent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 items, got %d", len(recent))
	}
}

func TestGetStats(t *testing.T) {
	gallery := NewGallery(nil)

	gallery.Add(&MediaItem{
		ID:       "movie1",
		Title:    "Movie 1",
		Type:     MediaTypeMovie,
		FilePath: "/data/movies/movie1.mp4",
	})

	stats := gallery.GetStats()
	totalItems := stats["total_items"].(int)
	if totalItems != 1 {
		t.Errorf("expected 1 total item, got %d", totalItems)
	}
}

func TestGetGenres(t *testing.T) {
	gallery := NewGallery(nil)

	gallery.Add(&MediaItem{
		ID:       "movie1",
		Title:    "Movie 1",
		Type:     MediaTypeMovie,
		Genres:   []string{"Action", "Comedy"},
		FilePath: "/data/movies/movie1.mp4",
	})

	gallery.Add(&MediaItem{
		ID:       "movie2",
		Title:    "Movie 2",
		Type:     MediaTypeMovie,
		Genres:   []string{"Action", "Drama"},
		FilePath: "/data/movies/movie2.mp4",
	})

	genres := gallery.GetGenres()
	if len(genres) != 3 {
		t.Errorf("expected 3 genres, got %d", len(genres))
	}
}

// 辅助函数
func fmt_Sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}
