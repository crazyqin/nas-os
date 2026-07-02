package mediascraper

import (
	"testing"
	"time"
)

func TestPosterWallBuilder_BuildDiscoveryDigest(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()
	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
		"/media/movies/Parasite 2019 REMUX 2160p.mkv",
		"/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv",
		"/media/tv/Game.of.Thrones.s01e01.2011.mkv",
		"/media/tv/Stranger Things S02E05 2016 WEB-DL.mkv",
	}

	wall := b.BuildFromResults(s.ScrapeBatch(files))
	digest := b.BuildDiscoveryDigest(wall, 3)

	if digest.Hero == nil {
		t.Fatal("首页主推不应为空")
	}
	if digest.Hero.Title != "Breaking Bad" {
		t.Fatalf("主推应选择最高评分内容 Breaking Bad, got %s", digest.Hero.Title)
	}
	if digest.Stats.Total != 7 || digest.Stats.Movies != 4 || digest.Stats.TVSeries != 3 {
		t.Fatalf("统计不正确: %+v", digest.Stats)
	}
	if digest.Stats.LatestYear != 2019 {
		t.Fatalf("最新年份 got %d, want 2019", digest.Stats.LatestYear)
	}
	if digest.Stats.AverageRate != 8.9 {
		t.Fatalf("平均评分 got %.1f, want 8.9", digest.Stats.AverageRate)
	}
	if len(digest.Shelves) < 4 {
		t.Fatalf("应包含基础内容架和至少一个类型内容架, got %d", len(digest.Shelves))
	}
	assertShelf(t, digest, ShelfTopRated, 3)
	assertShelf(t, digest, ShelfRecentlyAdd, 3)
	assertShelf(t, digest, ShelfNewRelease, 3)
	if len(digest.GenreStats) == 0 || digest.GenreStats[0].Genre != "剧情" || digest.GenreStats[0].Count != 5 {
		t.Fatalf("类型统计应按数量降序，第一应为剧情(5), got %+v", digest.GenreStats)
	}
	if digest.UpdatedAt.IsZero() {
		t.Fatal("更新时间不应为空")
	}
}

func TestPosterWallBuilder_BuildDiscoveryDigest_ShelfOrderingAndLimit(t *testing.T) {
	b := NewPosterWallBuilder()
	now := time.Now()
	items := []*MediaItem{
		{ID: "old-high", Title: "Old High", Type: MediaTypeMovie, Year: 2001, Rating: 9.8, Genres: []string{"剧情"}, ScrapedAt: now.Add(-2 * time.Hour)},
		{ID: "new-low", Title: "New Low", Type: MediaTypeMovie, Year: 2024, Rating: 7.1, Genres: []string{"剧情"}, ScrapedAt: now.Add(-1 * time.Hour)},
		{ID: "fresh-mid", Title: "Fresh Mid", Type: MediaTypeTVSeries, Year: 2020, Rating: 8.2, Genres: []string{"科幻"}, ScrapedAt: now},
	}
	wall := b.Build(items)

	digest := b.BuildDiscoveryDigest(wall, 2)
	topRated := findShelf(digest, ShelfTopRated)
	if topRated == nil || len(topRated.Items) != 2 {
		t.Fatalf("高分内容架应按 limit 截断为 2 项")
	}
	if topRated.Items[0].Title != "Old High" || topRated.Items[1].Title != "Fresh Mid" {
		t.Fatalf("高分内容架排序错误: %s, %s", topRated.Items[0].Title, topRated.Items[1].Title)
	}

	recent := findShelf(digest, ShelfRecentlyAdd)
	if recent == nil || recent.Items[0].Title != "Fresh Mid" || recent.Items[1].Title != "New Low" {
		t.Fatalf("最近入库内容架排序错误: %+v", shelfTitles(recent))
	}

	newRelease := findShelf(digest, ShelfNewRelease)
	if newRelease == nil || newRelease.Items[0].Title != "New Low" || newRelease.Items[1].Title != "Fresh Mid" {
		t.Fatalf("新片新剧内容架排序错误: %+v", shelfTitles(newRelease))
	}
}

func TestPosterWallBuilder_BuildDiscoveryDigest_EmptyAndNil(t *testing.T) {
	b := NewPosterWallBuilder()

	for _, wall := range []*PosterWall{nil, b.Build(nil)} {
		digest := b.BuildDiscoveryDigest(wall, 0)
		if digest.Hero != nil {
			t.Fatal("空内容库不应有主推")
		}
		if digest.Stats.Total != 0 {
			t.Fatalf("空内容库 total got %d, want 0", digest.Stats.Total)
		}
		if len(digest.Shelves) != 0 || len(digest.GenreStats) != 0 {
			t.Fatalf("空内容库不应有内容架或类型统计: %+v", digest)
		}
	}
}

func TestPosterWallBuilder_BuildDiscoveryDigest_DeduplicatesItems(t *testing.T) {
	b := NewPosterWallBuilder()
	item := &MediaItem{ID: "same", Title: "Same", Type: MediaTypeMovie, Year: 2024, Rating: 8.8, Genres: []string{"剧情"}, ScrapedAt: time.Now()}
	wall := &PosterWall{
		Groups: []*PosterWallGroup{
			{Type: MediaTypeMovie, Title: "电影", Items: []*MediaItem{item}, Count: 1},
			{Type: MediaTypeTVSeries, Title: "电视剧", Items: []*MediaItem{item}, Count: 1},
		},
		Total: 2,
	}

	digest := b.BuildDiscoveryDigest(wall, 8)
	if digest.Stats.Total != 1 {
		t.Fatalf("重复媒体应只统计一次, got %d", digest.Stats.Total)
	}
	if digest.Hero == nil || digest.Hero.ID != "same" {
		t.Fatal("去重后仍应保留媒体项")
	}
}

func TestPosterWallBuilder_BuildDiscoveryDigest_NormalizesGenreAndRatings(t *testing.T) {
	b := NewPosterWallBuilder()
	items := []*MediaItem{
		{ID: "a", Title: "Rated", Type: MediaTypeMovie, Year: 2024, Rating: 8.0, Genres: []string{" 剧情 "}, ScrapedAt: time.Now()},
		{ID: "b", Title: "Unrated", Type: MediaTypeMovie, Year: 2024, Rating: 0, Genres: []string{"剧情"}, ScrapedAt: time.Now()},
	}

	digest := b.BuildDiscoveryDigest(b.Build(items), 8)
	if digest.Stats.AverageRate != 8.0 {
		t.Fatalf("unrated items should not lower average rating, got %.1f", digest.Stats.AverageRate)
	}
	if len(digest.GenreStats) != 1 || digest.GenreStats[0].Genre != "剧情" || digest.GenreStats[0].Count != 2 {
		t.Fatalf("genres should be trimmed and aggregated, got %+v", digest.GenreStats)
	}
	shelf := findShelf(digest, ShelfGenrePrefix+"剧情")
	if shelf == nil || len(shelf.Items) != 2 {
		t.Fatalf("genre shelf should include trimmed genre items, got %+v", shelf)
	}
}

func TestPosterWallBuilder_BuildDiscoveryDigest_DeduplicatesByFallbackIdentity(t *testing.T) {
	b := NewPosterWallBuilder()
	item1 := &MediaItem{Title: "Same Title", Type: MediaTypeMovie, Year: 2024, Rating: 8.8, Genres: []string{"剧情"}, ScrapedAt: time.Now()}
	item2 := &MediaItem{Title: "Same Title", Type: MediaTypeMovie, Year: 2024, Rating: 8.8, Genres: []string{"剧情"}, ScrapedAt: time.Now()}
	wall := &PosterWall{Groups: []*PosterWallGroup{{Type: MediaTypeMovie, Title: "电影", Items: []*MediaItem{item1, item2}, Count: 2}}, Total: 2}

	digest := b.BuildDiscoveryDigest(wall, 8)
	if digest.Stats.Total != 1 {
		t.Fatalf("duplicate empty ID/file path media should be collapsed by fallback identity, got %d", digest.Stats.Total)
	}
}

func assertShelf(t *testing.T, digest *DiscoveryDigest, id string, wantMax int) {
	t.Helper()
	shelf := findShelf(digest, id)
	if shelf == nil {
		t.Fatalf("未找到内容架 %s", id)
	}
	if len(shelf.Items) == 0 || len(shelf.Items) > wantMax {
		t.Fatalf("内容架 %s 数量 got %d, want 1..%d", id, len(shelf.Items), wantMax)
	}
}

func findShelf(digest *DiscoveryDigest, id string) *DiscoveryShelf {
	if digest == nil {
		return nil
	}
	for _, shelf := range digest.Shelves {
		if shelf.ID == id {
			return shelf
		}
	}
	return nil
}

func shelfTitles(shelf *DiscoveryShelf) []string {
	if shelf == nil {
		return nil
	}
	titles := make([]string, 0, len(shelf.Items))
	for _, item := range shelf.Items {
		titles = append(titles, item.Title)
	}
	return titles
}
