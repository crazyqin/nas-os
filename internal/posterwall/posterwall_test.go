package posterwall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TMDBMockSource 测试
// ---------------------------------------------------------------------------

func TestTMDBMockSource_Scrape(t *testing.T) {
	s := NewTMDBMockSource()
	item, err := s.Scrape(context.Background(), "Inception", 2010, MediaTypeMovie)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}
	if item.Title != "Inception" {
		t.Errorf("expected title Inception, got %s", item.Title)
	}
	if item.Rating != 8.8 {
		t.Errorf("expected rating 8.8, got %f", item.Rating)
	}
	if item.Director != "Christopher Nolan" {
		t.Errorf("expected director Christopher Nolan, got %s", item.Director)
	}
	if item.ScrapeStatus != "ok" {
		t.Errorf("expected scrape_status ok, got %s", item.ScrapeStatus)
	}
}

func TestTMDBMockSource_Cache(t *testing.T) {
	s := NewTMDBMockSource()
	item1, _ := s.Scrape(context.Background(), "Test", 2020, MediaTypeMovie)
	item2, _ := s.Scrape(context.Background(), "Test", 2020, MediaTypeMovie)
	if item1.ID != item2.ID {
		t.Error("cache should return same item")
	}
}

func TestTMDBMockSource_TheMatrix(t *testing.T) {
	s := NewTMDBMockSource()
	item, _ := s.Scrape(context.Background(), "The Matrix", 1999, MediaTypeMovie)
	if item.Rating != 8.7 {
		t.Errorf("expected rating 8.7, got %f", item.Rating)
	}
	if len(item.Cast) != 3 || item.Cast[0] != "Keanu Reeves" {
		t.Errorf("unexpected cast: %v", item.Cast)
	}
}

// ---------------------------------------------------------------------------
// MediaScraper 测试
// ---------------------------------------------------------------------------

func TestMediaScraper_ScrapeFile(t *testing.T) {
	ms := NewMediaScraper(NewTMDBMockSource())
	item, err := ms.ScrapeFile(context.Background(), "Inception", 2010, MediaTypeMovie)
	if err != nil {
		t.Fatalf("ScrapeFile failed: %v", err)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}

	// 验证 GetItem
	got, ok := ms.GetItem(item.ID)
	if !ok {
		t.Error("GetItem should find item")
	}
	if got.Title != "Inception" {
		t.Errorf("expected Inception, got %s", got.Title)
	}
}

func TestMediaScraper_ScrapeBatch(t *testing.T) {
	ms := NewMediaScraper(NewTMDBMockSource())
	reqs := []ScrapeRequest{
		{Title: "Inception", Year: 2010, Type: MediaTypeMovie},
		{Title: "The Matrix", Year: 1999, Type: MediaTypeMovie},
		{Title: "Unknown Movie", Year: 2023, Type: MediaTypeMovie},
	}
	items, errs := ms.ScrapeBatch(context.Background(), reqs)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestMediaScraper_NilSource(t *testing.T) {
	ms := NewMediaScraper(nil)
	_, err := ms.ScrapeFile(context.Background(), "Test", 2020, MediaTypeMovie)
	if err == nil {
		t.Error("expected error with nil source")
	}
}

func TestMediaScraper_AllItems(t *testing.T) {
	ms := NewMediaScraper(NewTMDBMockSource())
	_, _ = ms.ScrapeFile(context.Background(), "A", 2020, MediaTypeMovie)
	_, _ = ms.ScrapeFile(context.Background(), "B", 2021, MediaTypeMovie)
	items := ms.AllItems()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// ProgressSync 测试
// ---------------------------------------------------------------------------

func TestProgressSync_UpdateAndGet(t *testing.T) {
	ps := NewProgressSync()
	p := &ProgressEntry{Position: 600, Duration: 7200, Device: "iPhone"}
	ps.UpdateProgress("media_1", p)

	got, ok := ps.GetProgress("media_1")
	if !ok {
		t.Fatal("expected to find progress")
	}
	if got.Position != 600 {
		t.Errorf("expected position 600, got %d", got.Position)
	}
	if got.Device != "iPhone" {
		t.Errorf("expected device iPhone, got %s", got.Device)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestProgressSync_Completed(t *testing.T) {
	ps := NewProgressSync()
	p := &ProgressEntry{Position: 6500, Duration: 7200, Device: "TV"}
	ps.UpdateProgress("media_1", p)

	got, _ := ps.GetProgress("media_1")
	// 6500 >= 7200*9/10 = 6480
	if !got.Completed {
		t.Error("expected completed=true")
	}
}

func TestProgressSync_NotCompleted(t *testing.T) {
	ps := NewProgressSync()
	p := &ProgressEntry{Position: 100, Duration: 7200, Device: "TV"}
	ps.UpdateProgress("media_1", p)

	got, _ := ps.GetProgress("media_1")
	if got.Completed {
		t.Error("expected completed=false")
	}
}

func TestProgressSync_Devices(t *testing.T) {
	ps := NewProgressSync()
	ps.UpdateProgress("m1", &ProgressEntry{Position: 100, Device: "iPhone"})
	ps.UpdateProgress("m1", &ProgressEntry{Position: 200, Device: "iPad"})
	ps.UpdateProgress("m1", &ProgressEntry{Position: 300, Device: "iPhone"}) // 重复

	devices := ps.GetDevices("m1")
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d: %v", len(devices), devices)
	}
}

func TestProgressSync_Listener(t *testing.T) {
	ps := NewProgressSync()
	called := false
	var gotID string
	var gotP *ProgressEntry
	ps.OnProgressUpdate(func(id string, p *ProgressEntry) {
		called = true
		gotID = id
		gotP = p
	})
	ps.UpdateProgress("m1", &ProgressEntry{Position: 100, Device: "TV"})
	if !called {
		t.Error("listener was not called")
	}
	if gotID != "m1" {
		t.Errorf("expected m1, got %s", gotID)
	}
	if gotP == nil || gotP.Position != 100 {
		t.Error("invalid progress in listener")
	}
}

func TestProgressSync_AllProgress(t *testing.T) {
	ps := NewProgressSync()
	ps.UpdateProgress("m1", &ProgressEntry{Position: 100, Device: "A"})
	ps.UpdateProgress("m2", &ProgressEntry{Position: 200, Device: "B"})
	all := ps.AllProgress()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestProgressSync_NilEntry(t *testing.T) {
	ps := NewProgressSync()
	ps.UpdateProgress("m1", nil)
	_, ok := ps.GetProgress("m1")
	if ok {
		t.Error("should not have progress for nil entry")
	}
}

// ---------------------------------------------------------------------------
// CategoryManager 测试
// ---------------------------------------------------------------------------

func TestCategoryManager_RebuildAndGet(t *testing.T) {
	cm := NewCategoryManager()
	items := []*MediaItem{
		{ID: "1", Type: MediaTypeMovie, Year: 2020, Genres: []string{"科幻", "动作"}, Rating: 8.5},
		{ID: "2", Type: MediaTypeTVShow, Year: 2021, Genres: []string{"喜剧"}, Rating: 7.2},
		{ID: "3", Type: MediaTypeMovie, Year: 2020, Genres: []string{"科幻"}, Rating: 6.5},
	}
	cm.Rebuild(items)

	// 按类型
	movies := cm.GetByCategory("type", "movie")
	if len(movies) != 2 {
		t.Errorf("expected 2 movies, got %d", len(movies))
	}

	// 按年份
	y2020 := cm.GetByCategory("year", "2020")
	if len(y2020) != 2 {
		t.Errorf("expected 2 items in 2020, got %d", len(y2020))
	}

	// 按类型
	genreSciFi := cm.GetByCategory("genre", "科幻")
	if len(genreSciFi) != 2 {
		t.Errorf("expected 2 sci-fi, got %d", len(genreSciFi))
	}

	// 按评分
	rating8 := cm.GetByCategory("rating", "8-9")
	if len(rating8) != 1 {
		t.Errorf("expected 1 item in 8-9, got %d", len(rating8))
	}
}

func TestCategoryManager_ListCategories(t *testing.T) {
	cm := NewCategoryManager()
	items := []*MediaItem{
		{ID: "1", Type: MediaTypeMovie, Genres: []string{"科幻"}, Rating: 8.0},
		{ID: "2", Type: MediaTypeTVShow, Genres: []string{"喜剧"}, Rating: 7.0},
	}
	cm.Rebuild(items)

	types := cm.ListCategories("type")
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}

	// 验证排序
	if types[0] != "movie" {
		t.Errorf("expected first type movie, got %s", types[0])
	}
}

func TestCategoryManager_AllFields(t *testing.T) {
	cm := NewCategoryManager()
	items := []*MediaItem{
		{ID: "1", Type: MediaTypeMovie, Year: 2020, Genres: []string{"科幻"}, Rating: 8.0},
	}
	cm.Rebuild(items)
	fields := cm.AllCategoryFields()
	if len(fields) != 4 { // type, year, genre, rating
		t.Errorf("expected 4 fields, got %d: %v", len(fields), fields)
	}
}

func TestCategoryManager_Empty(t *testing.T) {
	cm := NewCategoryManager()
	cm.Rebuild(nil)
	if cm.ListCategories("type") != nil {
		t.Error("expected nil for empty index")
	}
	if cm.GetByCategory("type", "movie") != nil {
		t.Error("expected nil for empty index")
	}
}

// ---------------------------------------------------------------------------
// PosterLayout 测试
// ---------------------------------------------------------------------------

func TestPosterLayout_GridMode(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "Beta", Year: 2021, Rating: 7.0, Genres: []string{"动作"}},
		{ID: "2", Title: "Alpha", Year: 2020, Rating: 9.0, Genres: []string{"科幻"}},
		{ID: "3", Title: "Gamma", Year: 2022, Rating: 8.0, Genres: []string{"喜剧"}},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{Mode: LayoutGrid, SortBy: SortByTitle, PageSize: 2, Page: 1})
	if page.Total != 3 {
		t.Errorf("expected total 3, got %d", page.Total)
	}
	if page.Pages != 2 {
		t.Errorf("expected 2 pages, got %d", page.Pages)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(page.Items))
	}
	if page.Items[0].Title != "Alpha" {
		t.Errorf("expected first item Alpha, got %s", page.Items[0].Title)
	}
}

func TestPosterLayout_SortByRating(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "B", Rating: 7.0},
		{ID: "2", Title: "A", Rating: 9.0},
		{ID: "3", Title: "C", Rating: 8.0},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{SortBy: SortByRating, PageSize: 10})
	if page.Items[0].Title != "A" {
		t.Errorf("expected highest rated A, got %s", page.Items[0].Title)
	}
}

func TestPosterLayout_SortByYear(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "Old", Year: 2000},
		{ID: "2", Title: "New", Year: 2024},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{SortBy: SortByYear, PageSize: 10})
	if page.Items[0].Title != "New" {
		t.Errorf("expected New first, got %s", page.Items[0].Title)
	}
}

func TestPosterLayout_Filter(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "Inception", Genres: []string{"科幻"}},
		{ID: "2", Title: "Titanic", Genres: []string{"爱情"}},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{Filter: "ince", PageSize: 10})
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(page.Items))
	}
	if page.Items[0].Title != "Inception" {
		t.Errorf("expected Inception, got %s", page.Items[0].Title)
	}
}

func TestPosterLayout_FilterByGenre(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "A", Genres: []string{"科幻"}},
		{ID: "2", Title: "B", Genres: []string{"爱情"}},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{Filter: "科幻", PageSize: 10})
	if len(page.Items) != 1 {
		t.Errorf("expected 1 item matching genre 科幻, got %d", len(page.Items))
	}
}

func TestPosterLayout_CategoryMode(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := []*MediaItem{
		{ID: "1", Title: "A", Type: MediaTypeMovie, Genres: []string{"科幻"}},
		{ID: "2", Title: "B", Type: MediaTypeTVShow, Genres: []string{"科幻"}},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{
		Mode:          LayoutByCategory,
		CategoryField: "genre",
		Category:      "科幻",
		PageSize:      10,
	})
	if len(page.Items) != 2 {
		t.Errorf("expected 2 sci-fi items, got %d", len(page.Items))
	}
	if page.Categories == nil {
		t.Error("expected categories in result")
	}
}

func TestPosterLayout_EmptyPage(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	pl.SetItems(nil)

	page := pl.Render(LayoutRequest{PageSize: 10})
	if page.Total != 0 {
		t.Errorf("expected total 0, got %d", page.Total)
	}
	if len(page.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(page.Items))
	}
	if page.Pages != 1 {
		t.Errorf("expected min 1 page, got %d", page.Pages)
	}
}

func TestPosterLayout_DefaultPageSize(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	items := make([]*MediaItem, 30)
	for i := range items {
		items[i] = &MediaItem{ID: string(rune('A' + i)), Title: string(rune('A' + i))}
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{Page: 1})
	if page.PageSize != 24 {
		t.Errorf("expected default pageSize 24, got %d", page.PageSize)
	}
}

// ---------------------------------------------------------------------------
// WatchList 测试
// ---------------------------------------------------------------------------

func TestWatchList_AddRemove(t *testing.T) {
	wl := NewWatchList()
	wl.Add("m1", "想看")

	entry, ok := wl.GetEntry("m1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if entry.Note != "想看" {
		t.Errorf("expected note 想看, got %s", entry.Note)
	}
	if entry.Status != "planned" {
		t.Errorf("expected status planned, got %s", entry.Status)
	}

	if !wl.Remove("m1") {
		t.Error("Remove should return true")
	}
	if _, ok := wl.GetEntry("m1"); ok {
		t.Error("should not find removed entry")
	}

	if wl.Remove("nonexistent") {
		t.Error("Remove should return false for nonexistent")
	}
}

func TestWatchList_UpdateStatus(t *testing.T) {
	wl := NewWatchList()
	wl.Add("m1", "")

	if !wl.UpdateStatus("m1", "completed", 9.0) {
		t.Error("UpdateStatus should return true")
	}
	entry, _ := wl.GetEntry("m1")
	if entry.Status != "completed" {
		t.Errorf("expected completed, got %s", entry.Status)
	}
	if entry.Rating != 9.0 {
		t.Errorf("expected rating 9.0, got %f", entry.Rating)
	}

	if wl.UpdateStatus("nonexistent", "completed", 0) {
		t.Error("UpdateStatus should return false for nonexistent")
	}
}

func TestWatchList_ByStatus(t *testing.T) {
	wl := NewWatchList()
	wl.Add("m1", "")
	wl.Add("m2", "")
	wl.UpdateStatus("m1", "completed", 0)
	wl.Add("m3", "")

	planned := wl.ByStatus("planned")
	if len(planned) != 2 {
		t.Errorf("expected 2 planned, got %d", len(planned))
	}
	completed := wl.ByStatus("completed")
	if len(completed) != 1 {
		t.Errorf("expected 1 completed, got %d", len(completed))
	}
}

func TestWatchList_AllEntries(t *testing.T) {
	wl := NewWatchList()
	wl.Add("m1", "")
	wl.Add("m2", "")
	if len(wl.AllEntries()) != 2 {
		t.Errorf("expected 2 entries, got %d", len(wl.AllEntries()))
	}
}

func TestWatchList_Recommend(t *testing.T) {
	wl := NewWatchList()
	wl.Add("m1", "")
	wl.UpdateStatus("m1", "completed", 9.0)

	// m1 is 科幻, m2 is also 科幻 (should get boosted)
	items := []*MediaItem{
		{ID: "m1", Title: "Watched", Genres: []string{"科幻"}, Rating: 8.0},
		{ID: "m2", Title: "Recommended SciFi", Genres: []string{"科幻"}, Rating: 7.5},
		{ID: "m3", Title: "Romance", Genres: []string{"爱情"}, Rating: 8.0},
	}

	recs := wl.Recommend(items, 2)
	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	// m2 should rank higher than m3 due to genre affinity
	if recs[0].ID != "m2" {
		t.Errorf("expected m2 first, got %s", recs[0].ID)
	}
}

func TestWatchList_RecommendNoHistory(t *testing.T) {
	wl := NewWatchList()
	items := []*MediaItem{
		{ID: "m1", Title: "A", Rating: 9.0},
		{ID: "m2", Title: "B", Rating: 7.0},
	}
	recs := wl.Recommend(items, 5)
	if len(recs) != 2 {
		t.Errorf("expected 2, got %d", len(recs))
	}
	if recs[0].ID != "m1" {
		t.Errorf("expected higher rated m1 first, got %s", recs[0].ID)
	}
}

// ---------------------------------------------------------------------------
// PosterWall 集成测试
// ---------------------------------------------------------------------------

func TestPosterWall_ScrapeAndAdd(t *testing.T) {
	pw := NewPosterWall()
	item, err := pw.ScrapeAndAdd(context.Background(), "Inception", 2010, MediaTypeMovie)
	if err != nil {
		t.Fatalf("ScrapeAndAdd failed: %v", err)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}

	got, ok := pw.GetItem(item.ID)
	if !ok {
		t.Fatal("GetItem should find item")
	}
	if got.Title != "Inception" {
		t.Errorf("expected Inception, got %s", got.Title)
	}
}

func TestPosterWall_AddAndRemoveItem(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "test1", Title: "Test"})
	if len(pw.AllItems()) != 1 {
		t.Errorf("expected 1 item, got %d", len(pw.AllItems()))
	}
	if !pw.RemoveItem("test1") {
		t.Error("RemoveItem should return true")
	}
	if len(pw.AllItems()) != 0 {
		t.Errorf("expected 0 items, got %d", len(pw.AllItems()))
	}
	if pw.RemoveItem("nonexistent") {
		t.Error("RemoveItem should return false for nonexistent")
	}
}

func TestPosterWall_UpdateProgress(t *testing.T) {
	pw := NewPosterWall()
	pw.UpdateProgress("m1", 300, 3600, "iPhone")

	p, ok := pw.GetProgress("m1")
	if !ok {
		t.Fatal("expected progress")
	}
	if p.Position != 300 {
		t.Errorf("expected position 300, got %d", p.Position)
	}
	if p.Completed {
		t.Error("should not be completed")
	}
}

func TestPosterWall_UpdateProgressCompleted(t *testing.T) {
	pw := NewPosterWall()
	pw.UpdateProgress("m1", 3300, 3600, "TV")

	p, _ := pw.GetProgress("m1")
	if !p.Completed {
		t.Error("expected completed")
	}
}

func TestPosterWall_RenderLayout(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "Alpha", Year: 2020, Rating: 8.0, Genres: []string{"科幻"}, Type: MediaTypeMovie})
	pw.AddItem(&MediaItem{ID: "2", Title: "Beta", Year: 2021, Rating: 7.0, Genres: []string{"动作"}, Type: MediaTypeMovie})

	page := pw.RenderLayout(LayoutRequest{SortBy: SortByTitle, PageSize: 10})
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
	if page.Items[0].Title != "Alpha" {
		t.Errorf("expected Alpha first, got %s", page.Items[0].Title)
	}
}

func TestPosterWall_GetCategories(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "A", Type: MediaTypeMovie, Genres: []string{"科幻"}, Rating: 8.0})
	pw.AddItem(&MediaItem{ID: "2", Title: "B", Type: MediaTypeTVShow, Genres: []string{"动作"}, Rating: 7.0})

	genres := pw.GetCategories("genre")
	if len(genres) != 2 {
		t.Errorf("expected 2 genres, got %d: %v", len(genres), genres)
	}
}

func TestPosterWall_WatchList(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "m1", Title: "Movie", Rating: 8.0, Genres: []string{"科幻"}})
	pw.AddToWatchList("m1", "想看")

	recs := pw.Recommend(5)
	// m1 is in watchlist (planned, not completed), so it could be recommended
	// since nothing is completed, all unwatched items are eligible
	if len(recs) == 0 {
		t.Error("expected at least 1 recommendation")
	}
}

// ---------------------------------------------------------------------------
// API Handler 测试
// ---------------------------------------------------------------------------

func TestAPIHandler_Scrape(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	body := `{"title":"Inception","year":2010,"type":"movie"}`
	req := httptest.NewRequest(http.MethodPost, "/api/posterwall/scrape", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var item MediaItem
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "Inception" {
		t.Errorf("expected Inception, got %s", item.Title)
	}
}

func TestAPIHandler_ItemsList(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "Test"})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/items", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	total, _ := resp["total"].(float64)
	if int(total) != 1 {
		t.Errorf("expected total 1, got %v", resp["total"])
	}
}

func TestAPIHandler_ItemGet(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "abc", Title: "Test"})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/items/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var item MediaItem
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "Test" {
		t.Errorf("expected Test, got %s", item.Title)
	}
}

func TestAPIHandler_ItemDelete(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "abc", Title: "Test"})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodDelete, "/api/posterwall/items/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIHandler_ItemNotFound(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/items/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIHandler_Layout(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "A", Rating: 8.0, Type: MediaTypeMovie})
	pw.AddItem(&MediaItem{ID: "2", Title: "B", Rating: 7.0, Type: MediaTypeMovie})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/layout?sort=rating&page_size=1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var page LayoutPage
	json.NewDecoder(w.Body).Decode(&page)
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Errorf("expected 1 item on page, got %d", len(page.Items))
	}
	if page.Items[0].Title != "A" {
		t.Errorf("expected A first (higher rating), got %s", page.Items[0].Title)
	}
}

func TestAPIHandler_Progress(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	// POST progress
	body := `{"position":600,"duration":7200,"device":"iPhone"}`
	req := httptest.NewRequest(http.MethodPut, "/api/posterwall/progress/m1", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// GET progress
	req = httptest.NewRequest(http.MethodGet, "/api/posterwall/progress/m1", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var p ProgressEntry
	json.NewDecoder(w.Body).Decode(&p)
	if p.Position != 600 {
		t.Errorf("expected position 600, got %d", p.Position)
	}
}

func TestAPIHandler_Categories(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "A", Type: MediaTypeMovie, Genres: []string{"科幻"}, Rating: 8.0})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/categories?field=genre", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string][]string
	json.NewDecoder(w.Body).Decode(&resp)
	cats := resp["categories"]
	if len(cats) != 1 || cats[0] != "科幻" {
		t.Errorf("expected [科幻], got %v", cats)
	}
}

func TestAPIHandler_CategoriesAllFields(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "A", Type: MediaTypeMovie, Genres: []string{"科幻"}, Rating: 8.0})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/categories", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIHandler_WatchListAdd(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "m1", Title: "Movie"})
	h := NewAPIHandler(pw)

	body := `{"media_id":"m1","note":"想看"}`
	req := httptest.NewRequest(http.MethodPost, "/api/posterwall/watchlist", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestAPIHandler_WatchListGet(t *testing.T) {
	pw := NewPosterWall()
	pw.AddToWatchList("m1", "想看")
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/watchlist/m1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var entry WatchEntry
	json.NewDecoder(w.Body).Decode(&entry)
	if entry.MediaID != "m1" {
		t.Errorf("expected m1, got %s", entry.MediaID)
	}
}

func TestAPIHandler_WatchListDelete(t *testing.T) {
	pw := NewPosterWall()
	pw.AddToWatchList("m1", "")
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodDelete, "/api/posterwall/watchlist/m1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIHandler_Recommend(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "A", Rating: 9.0, Genres: []string{"科幻"}})
	pw.AddItem(&MediaItem{ID: "2", Title: "B", Rating: 7.0, Genres: []string{"动作"}})
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/recommend?limit=1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	count, _ := resp["count"].(float64)
	if int(count) != 1 {
		t.Errorf("expected count 1, got %v", resp["count"])
	}
}

func TestAPIHandler_NotFound(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/unknown", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIHandler_BadPath(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/wrong/path", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIHandler_ScrapeMissingTitle(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	body := `{"year":2020}`
	req := httptest.NewRequest(http.MethodPost, "/api/posterwall/scrape", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIHandler_ScrapeBadMethod(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/scrape", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestAPIHandler_ProgressNotFound(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/progress/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIHandler_ProgressMissingMediaID(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	body := `{"position":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/posterwall/progress", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIHandler_WatchListNotFound(t *testing.T) {
	pw := NewPosterWall()
	h := NewAPIHandler(pw)

	req := httptest.NewRequest(http.MethodGet, "/api/posterwall/watchlist/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPosterWall_ItemIsolation(t *testing.T) {
	pw := NewPosterWall()
	pw.AddItem(&MediaItem{ID: "1", Title: "Original"})

	item, _ := pw.GetItem("1")
	item.Title = "Modified"

	item2, _ := pw.GetItem("1")
	if item2.Title != "Original" {
		t.Error("item was mutated through returned pointer")
	}
}

func TestProgressSync_GetNonexistent(t *testing.T) {
	ps := NewProgressSync()
	_, ok := ps.GetProgress("nonexistent")
	if ok {
		t.Error("expected false for nonexistent progress")
	}
}

func TestCategoryManager_GetNonexistent(t *testing.T) {
	cm := NewCategoryManager()
	cm.Rebuild([]*MediaItem{{ID: "1", Type: MediaTypeMovie}})
	if cm.GetByCategory("nonexistent", "value") != nil {
		t.Error("expected nil for nonexistent field")
	}
	if cm.GetByCategory("type", "nonexistent") != nil {
		t.Error("expected nil for nonexistent value")
	}
}

func TestPosterLayout_SortByRecent(t *testing.T) {
	cm := NewCategoryManager()
	pl := NewPosterLayout(cm)
	now := time.Now()
	later := now.Add(1 * time.Hour)
	items := []*MediaItem{
		{ID: "1", Title: "Old", ScrapedAt: &now},
		{ID: "2", Title: "New", ScrapedAt: &later},
	}
	pl.SetItems(items)

	page := pl.Render(LayoutRequest{SortBy: SortByRecent, PageSize: 10})
	if page.Items[0].Title != "New" {
		t.Errorf("expected New first, got %s", page.Items[0].Title)
	}
}

func TestPosterWall_ScrapeBatch(t *testing.T) {
	pw := NewPosterWall()
	items, errs := pw.scraper.ScrapeBatch(context.Background(), []ScrapeRequest{
		{Title: "Inception", Year: 2010, Type: MediaTypeMovie},
		{Title: "The Matrix", Year: 1999, Type: MediaTypeMovie},
	})
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestRatingBucket(t *testing.T) {
	cases := []struct {
		rating float64
		bucket  string
	}{
		{9.5, "9+"},
		{8.5, "8-9"},
		{7.5, "7-8"},
		{6.5, "6-7"},
		{5.5, "5-6"},
		{3.0, "<5"},
	}
	for _, c := range cases {
		got := ratingBucket(c.rating)
		if got != c.bucket {
			t.Errorf("ratingBucket(%.1f) = %s, want %s", c.rating, got, c.bucket)
		}
	}
}
