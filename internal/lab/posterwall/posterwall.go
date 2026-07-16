// Package posterwall 智能海报墙系统
// 提供影视信息刮削、海报墙布局展示、多端播放进度同步、分类管理、观影清单与推荐
// 与 internal/mediaposter 不同：mediaposter 侧重单张海报生成，
// posterwall 是完整的海报墙视图 + 刮削 + 进度同步系统
package posterwall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// 数据模型
// ---------------------------------------------------------------------------

type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeTVShow  MediaType = "tvshow"
	MediaTypeMusic   MediaType = "music"
	MediaTypeUnknown MediaType = "unknown"
)

type MediaItem struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	OriginalTitle string     `json:"original_title,omitempty"`
	Type          MediaType  `json:"type"`
	Year          int        `json:"year,omitempty"`
	Genres        []string   `json:"genres,omitempty"`
	Overview      string     `json:"overview,omitempty"`
	Director      string     `json:"director,omitempty"`
	Cast          []string   `json:"cast,omitempty"`
	Rating        float64    `json:"rating,omitempty"`
	PosterURL     string     `json:"poster_url,omitempty"`
	BackdropURL   string     `json:"backdrop_url,omitempty"`
	Duration      int        `json:"duration,omitempty"`
	FilePath      string     `json:"file_path,omitempty"`
	ScrapedAt     *time.Time `json:"scraped_at,omitempty"`
	ScrapeStatus  string     `json:"scrape_status,omitempty"`
}

type ProgressEntry struct {
	MediaID   string    `json:"media_id"`
	Position  int       `json:"position"`
	Duration  int       `json:"duration"`
	Device    string    `json:"device"`
	UpdatedAt time.Time `json:"updated_at"`
	Completed bool      `json:"completed"`
}

type LayoutMode string

const (
	LayoutGrid       LayoutMode = "grid"
	LayoutList       LayoutMode = "list"
	LayoutByCategory LayoutMode = "category"
)

type SortOrder string

const (
	SortByTitle  SortOrder = "title"
	SortByYear   SortOrder = "year"
	SortByRating SortOrder = "rating"
	SortByRecent SortOrder = "recent"
)

type WatchEntry struct {
	MediaID string    `json:"media_id"`
	AddedAt time.Time `json:"added_at"`
	Note    string    `json:"note,omitempty"`
	Status  string    `json:"status"`
	Rating  float64   `json:"user_rating,omitempty"`
}

type ScrapeRequest struct {
	Title    string
	Year     int
	Type     MediaType
	FilePath string
}

type LayoutRequest struct {
	Mode          LayoutMode `json:"mode"`
	SortBy        SortOrder  `json:"sort_by"`
	Category      string     `json:"category,omitempty"`
	CategoryField string     `json:"category_field,omitempty"`
	Filter        string     `json:"filter,omitempty"`
	Page          int        `json:"page"`
	PageSize      int        `json:"page_size"`
}

type LayoutPage struct {
	Items      []*MediaItem `json:"items"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	Pages      int          `json:"pages"`
	Categories []string     `json:"categories,omitempty"`
	Mode       LayoutMode   `json:"mode"`
}

// ---------------------------------------------------------------------------
// ScraperSource + TMDBMockSource
// ---------------------------------------------------------------------------

type ScraperSource interface {
	Name() string
	Scrape(ctx context.Context, title string, year int, mediaType MediaType) (*MediaItem, error)
}

type TMDBMockSource struct {
	mu    sync.Mutex
	cache map[string]*MediaItem
}

func NewTMDBMockSource() *TMDBMockSource {
	return &TMDBMockSource{cache: make(map[string]*MediaItem)}
}

func (s *TMDBMockSource) Name() string { return "tmdb-mock" }

func (s *TMDBMockSource) Scrape(ctx context.Context, title string, year int, mediaType MediaType) (*MediaItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s|%d|%s", strings.ToLower(title), year, mediaType)
	if item, ok := s.cache[key]; ok {
		return item, nil
	}
	now := time.Now()
	item := &MediaItem{
		ID:            fmt.Sprintf("tmdb_%s_%d", strings.ReplaceAll(strings.ToLower(title), " ", "_"), year),
		Title:         title,
		OriginalTitle: title,
		Type:          mediaType,
		Year:          year,
		Overview:      fmt.Sprintf("自动刮削简介：%s（%d年）", title, year),
		Director:      "未知导演",
		Cast:          []string{"演员A", "演员B", "演员C"},
		Rating:        7.5,
		PosterURL:     fmt.Sprintf("https://image.tmdb.example.com/poster/%s_%d.jpg", strings.ReplaceAll(title, " ", "_"), year),
		BackdropURL:   fmt.Sprintf("https://image.tmdb.example.com/backdrop/%s_%d.jpg", strings.ReplaceAll(title, " ", "_"), year),
		Duration:      120,
		ScrapedAt:     &now,
		ScrapeStatus:  "ok",
		Genres:        []string{"剧情", "科幻"},
	}
	switch strings.ToLower(title) {
	case "inception":
		item.Rating = 8.8
		item.Director = "Christopher Nolan"
		item.Cast = []string{"Leonardo DiCaprio", "Joseph Gordon-Levitt", "Elliot Page"}
		item.Genres = []string{"科幻", "动作", "悬疑"}
		item.Overview = "盗梦空间：一个能在梦境中窃取秘密的盗贼接到一个不可能的任务。"
	case "the matrix":
		item.Rating = 8.7
		item.Director = "The Wachowskis"
		item.Cast = []string{"Keanu Reeves", "Laurence Fishburne", "Carrie-Anne Moss"}
		item.Genres = []string{"科幻", "动作"}
		item.Overview = "黑客帝国：程序员尼奥发现现实世界是由机器创造的虚拟世界。"
	}
	s.cache[key] = item
	return item, nil
}

// ---------------------------------------------------------------------------
// MediaScraper
// ---------------------------------------------------------------------------

type MediaScraper struct {
	source ScraperSource
	mu     sync.RWMutex
	items  map[string]*MediaItem
}

func NewMediaScraper(source ScraperSource) *MediaScraper {
	return &MediaScraper{source: source, items: make(map[string]*MediaItem)}
}

func (ms *MediaScraper) ScrapeFile(ctx context.Context, title string, year int, mediaType MediaType) (*MediaItem, error) {
	if ms.source == nil {
		return nil, fmt.Errorf("未配置刮削数据源")
	}
	item, err := ms.source.Scrape(ctx, title, year, mediaType)
	if err != nil {
		return nil, fmt.Errorf("刮削失败: %w", err)
	}
	ms.mu.Lock()
	ms.items[item.ID] = item
	ms.mu.Unlock()
	return item, nil
}

func (ms *MediaScraper) ScrapeBatch(ctx context.Context, requests []ScrapeRequest) ([]*MediaItem, []error) {
	items := make([]*MediaItem, 0, len(requests))
	errs := make([]error, 0)
	for _, req := range requests {
		item, err := ms.ScrapeFile(ctx, req.Title, req.Year, req.Type)
		if err != nil {
			errs = append(errs, fmt.Errorf("刮削 %q 失败: %w", req.Title, err))
			continue
		}
		items = append(items, item)
	}
	return items, errs
}

func (ms *MediaScraper) GetItem(id string) (*MediaItem, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	item, ok := ms.items[id]
	if !ok {
		return nil, false
	}
	copied := *item
	return &copied, true
}

func (ms *MediaScraper) AllItems() []*MediaItem {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make([]*MediaItem, 0, len(ms.items))
	for _, item := range ms.items {
		copied := *item
		result = append(result, &copied)
	}
	return result
}

// ---------------------------------------------------------------------------
// ProgressSync
// ---------------------------------------------------------------------------

type ProgressSync struct {
	mu        sync.RWMutex
	progress  map[string]*ProgressEntry
	devices   map[string][]string
	listeners []func(mediaID string, p *ProgressEntry)
}

func NewProgressSync() *ProgressSync {
	return &ProgressSync{progress: make(map[string]*ProgressEntry), devices: make(map[string][]string)}
}

func (ps *ProgressSync) UpdateProgress(mediaID string, p *ProgressEntry) {
	if p == nil {
		return
	}
	p.MediaID = mediaID
	p.UpdatedAt = time.Now()
	if p.Duration > 0 {
		p.Completed = p.Position >= p.Duration*9/10
	}
	ps.mu.Lock()
	ps.progress[mediaID] = p
	found := false
	for _, d := range ps.devices[mediaID] {
		if d == p.Device {
			found = true
			break
		}
	}
	if !found {
		ps.devices[mediaID] = append(ps.devices[mediaID], p.Device)
	}
	listeners := make([]func(string, *ProgressEntry), len(ps.listeners))
	copy(listeners, ps.listeners)
	ps.mu.Unlock()
	for _, fn := range listeners {
		fn(mediaID, p)
	}
}

func (ps *ProgressSync) GetProgress(mediaID string) (*ProgressEntry, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	p, ok := ps.progress[mediaID]
	if !ok {
		return nil, false
	}
	copied := *p
	return &copied, true
}

func (ps *ProgressSync) GetDevices(mediaID string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	devices := make([]string, len(ps.devices[mediaID]))
	copy(devices, ps.devices[mediaID])
	return devices
}

func (ps *ProgressSync) OnProgressUpdate(fn func(mediaID string, p *ProgressEntry)) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.listeners = append(ps.listeners, fn)
}

func (ps *ProgressSync) AllProgress() map[string]*ProgressEntry {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	result := make(map[string]*ProgressEntry, len(ps.progress))
	for k, v := range ps.progress {
		copied := *v
		result[k] = &copied
	}
	return result
}

// ---------------------------------------------------------------------------
// CategoryManager
// ---------------------------------------------------------------------------

type CategoryManager struct {
	mu      sync.RWMutex
	indexes map[string]map[string][]string
}

func NewCategoryManager() *CategoryManager {
	return &CategoryManager{indexes: make(map[string]map[string][]string)}
}

func (cm *CategoryManager) Rebuild(items []*MediaItem) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.indexes = make(map[string]map[string][]string)
	for _, item := range items {
		cm.addToIndex("type", string(item.Type), item.ID)
		if item.Year > 0 {
			cm.addToIndex("year", fmt.Sprintf("%d", item.Year), item.ID)
		}
		for _, g := range item.Genres {
			cm.addToIndex("genre", g, item.ID)
		}
		if item.Rating > 0 {
			cm.addToIndex("rating", ratingBucket(item.Rating), item.ID)
		}
	}
}

func ratingBucket(r float64) string {
	switch {
	case r >= 9.0:
		return "9+"
	case r >= 8.0:
		return "8-9"
	case r >= 7.0:
		return "7-8"
	case r >= 6.0:
		return "6-7"
	case r >= 5.0:
		return "5-6"
	default:
		return "<5"
	}
}

func (cm *CategoryManager) addToIndex(field, value, id string) {
	if cm.indexes[field] == nil {
		cm.indexes[field] = make(map[string][]string)
	}
	cm.indexes[field][value] = append(cm.indexes[field][value], id)
}

func (cm *CategoryManager) GetByCategory(field, value string) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ids, ok := cm.indexes[field][value]
	if !ok {
		return nil
	}
	result := make([]string, len(ids))
	copy(result, ids)
	return result
}

func (cm *CategoryManager) ListCategories(field string) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	m, ok := cm.indexes[field]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func (cm *CategoryManager) AllCategoryFields() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]string, 0, len(cm.indexes))
	for k := range cm.indexes {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// ---------------------------------------------------------------------------
// PosterLayout
// ---------------------------------------------------------------------------

type PosterLayout struct {
	mu    sync.RWMutex
	items []*MediaItem
	cm    *CategoryManager
}

func NewPosterLayout(cm *CategoryManager) *PosterLayout {
	return &PosterLayout{cm: cm}
}

func (pl *PosterLayout) SetItems(items []*MediaItem) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.items = make([]*MediaItem, len(items))
	copy(pl.items, items)
	pl.cm.Rebuild(items)
}

func (pl *PosterLayout) Render(req LayoutRequest) *LayoutPage {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	filtered := pl.filterAndSort(req)
	total := len(filtered)
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 24
	}
	pages := (total + pageSize - 1) / pageSize
	if pages == 0 {
		pages = 1
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	var pageItems []*MediaItem
	if start < end {
		pageItems = filtered[start:end]
	} else {
		pageItems = []*MediaItem{}
	}
	result := &LayoutPage{
		Items:    pageItems,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
		Mode:     req.Mode,
	}
	if req.Mode == LayoutByCategory && req.CategoryField != "" {
		result.Categories = pl.cm.ListCategories(req.CategoryField)
	}
	return result
}

func (pl *PosterLayout) filterAndSort(req LayoutRequest) []*MediaItem {
	var result []*MediaItem
	if req.Mode == LayoutByCategory && req.CategoryField != "" && req.Category != "" {
		ids := pl.cm.GetByCategory(req.CategoryField, req.Category)
		idSet := make(map[string]bool, len(ids))
		for _, id := range ids {
			idSet[id] = true
		}
		for _, item := range pl.items {
			if idSet[item.ID] {
				result = append(result, item)
			}
		}
	} else {
		result = append(result, pl.items...)
	}
	if req.Filter != "" {
		lower := strings.ToLower(req.Filter)
		filtered := make([]*MediaItem, 0, len(result))
		for _, item := range result {
			if strings.Contains(strings.ToLower(item.Title), lower) {
				filtered = append(filtered, item)
				continue
			}
			for _, g := range item.Genres {
				if strings.Contains(strings.ToLower(g), lower) {
					filtered = append(filtered, item)
					break
				}
			}
		}
		result = filtered
	}
	pl.sortItems(result, req.SortBy)
	return result
}

func (pl *PosterLayout) sortItems(items []*MediaItem, order SortOrder) {
	switch order {
	case SortByTitle:
		sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	case SortByYear:
		sort.Slice(items, func(i, j int) bool { return items[i].Year > items[j].Year })
	case SortByRating:
		sort.Slice(items, func(i, j int) bool { return items[i].Rating > items[j].Rating })
	case SortByRecent:
		sort.Slice(items, func(i, j int) bool {
			ai, aj := items[i].ScrapedAt, items[j].ScrapedAt
			if ai == nil {
				return false
			}
			if aj == nil {
				return true
			}
			return ai.After(*aj)
		})
	}
}

// ---------------------------------------------------------------------------
// WatchList
// ---------------------------------------------------------------------------

type WatchList struct {
	mu      sync.RWMutex
	entries map[string]*WatchEntry
}

func NewWatchList() *WatchList {
	return &WatchList{entries: make(map[string]*WatchEntry)}
}

func (wl *WatchList) Add(mediaID string, note string) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	wl.entries[mediaID] = &WatchEntry{
		MediaID: mediaID,
		AddedAt: time.Now(),
		Note:    note,
		Status:  "planned",
	}
}

func (wl *WatchList) Remove(mediaID string) bool {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	if _, ok := wl.entries[mediaID]; !ok {
		return false
	}
	delete(wl.entries, mediaID)
	return true
}

func (wl *WatchList) UpdateStatus(mediaID string, status string, rating float64) bool {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	entry, ok := wl.entries[mediaID]
	if !ok {
		return false
	}
	entry.Status = status
	if rating > 0 {
		entry.Rating = rating
	}
	return true
}

func (wl *WatchList) GetEntry(mediaID string) (*WatchEntry, bool) {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	entry, ok := wl.entries[mediaID]
	if !ok {
		return nil, false
	}
	copied := *entry
	return &copied, true
}

func (wl *WatchList) AllEntries() []*WatchEntry {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	result := make([]*WatchEntry, 0, len(wl.entries))
	for _, e := range wl.entries {
		copied := *e
		result = append(result, &copied)
	}
	return result
}

func (wl *WatchList) ByStatus(status string) []*WatchEntry {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	result := make([]*WatchEntry, 0)
	for _, e := range wl.entries {
		if e.Status == status {
			copied := *e
			result = append(result, &copied)
		}
	}
	return result
}

func (wl *WatchList) Recommend(items []*MediaItem, limit int) []*MediaItem {
	wl.mu.RLock()
	watchedGenres := make(map[string]float64)
	watchedIDs := make(map[string]bool)
	for _, entry := range wl.entries {
		if entry.Status == "completed" {
			watchedIDs[entry.MediaID] = true
		}
	}
	wl.mu.RUnlock()
	for _, item := range items {
		if watchedIDs[item.ID] {
			w := item.Rating
			if w == 0 {
				w = 7.0
			}
			for _, g := range item.Genres {
				watchedGenres[g] += w
			}
		}
	}
	type scored struct {
		item  *MediaItem
		score float64
	}
	candidates := make([]scored, 0)
	for _, item := range items {
		if watchedIDs[item.ID] {
			continue
		}
		score := item.Rating
		for _, g := range item.Genres {
			score += watchedGenres[g] * 0.1
		}
		candidates = append(candidates, scored{item, score})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if limit <= 0 || limit > len(candidates) {
		limit = len(candidates)
	}
	result := make([]*MediaItem, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, candidates[i].item)
	}
	return result
}

// ---------------------------------------------------------------------------
// PosterWall 核心管理器
// ---------------------------------------------------------------------------

type PosterWall struct {
	scraper  *MediaScraper
	progress *ProgressSync
	layout   *PosterLayout
	category *CategoryManager
	watch    *WatchList
	mu       sync.RWMutex
	items    map[string]*MediaItem
}

func NewPosterWall() *PosterWall {
	cm := NewCategoryManager()
	return &PosterWall{
		scraper:  NewMediaScraper(NewTMDBMockSource()),
		progress: NewProgressSync(),
		layout:   NewPosterLayout(cm),
		category: cm,
		watch:    NewWatchList(),
		items:    make(map[string]*MediaItem),
	}
}

func (pw *PosterWall) ScrapeAndAdd(ctx context.Context, title string, year int, mediaType MediaType) (*MediaItem, error) {
	item, err := pw.scraper.ScrapeFile(ctx, title, year, mediaType)
	if err != nil {
		return nil, err
	}
	pw.mu.Lock()
	pw.items[item.ID] = item
	items := make([]*MediaItem, 0, len(pw.items))
	for _, it := range pw.items {
		copied := *it
		items = append(items, &copied)
	}
	pw.mu.Unlock()
	pw.layout.SetItems(items)
	return item, nil
}

func (pw *PosterWall) AddItem(item *MediaItem) {
	pw.mu.Lock()
	pw.items[item.ID] = item
	items := make([]*MediaItem, 0, len(pw.items))
	for _, it := range pw.items {
		copied := *it
		items = append(items, &copied)
	}
	pw.mu.Unlock()
	pw.layout.SetItems(items)
}

func (pw *PosterWall) RemoveItem(id string) bool {
	pw.mu.Lock()
	if _, ok := pw.items[id]; !ok {
		pw.mu.Unlock()
		return false
	}
	delete(pw.items, id)
	items := make([]*MediaItem, 0, len(pw.items))
	for _, item := range pw.items {
		copied := *item
		items = append(items, &copied)
	}
	pw.mu.Unlock()
	pw.layout.SetItems(items)
	return true
}

func (pw *PosterWall) GetItem(id string) (*MediaItem, bool) {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	item, ok := pw.items[id]
	if !ok {
		return nil, false
	}
	copied := *item
	return &copied, true
}

func (pw *PosterWall) AllItems() []*MediaItem {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	result := make([]*MediaItem, 0, len(pw.items))
	for _, item := range pw.items {
		copied := *item
		result = append(result, &copied)
	}
	return result
}

func (pw *PosterWall) refreshLayout() {
	pw.layout.SetItems(pw.AllItems())
}

func (pw *PosterWall) UpdateProgress(mediaID string, position, duration int, device string) {
	p := &ProgressEntry{
		MediaID:   mediaID,
		Position:  position,
		Duration:  duration,
		Device:    device,
		Completed: duration > 0 && position >= duration*9/10,
	}
	pw.progress.UpdateProgress(mediaID, p)
}

func (pw *PosterWall) GetProgress(mediaID string) (*ProgressEntry, bool) {
	return pw.progress.GetProgress(mediaID)
}

func (pw *PosterWall) RenderLayout(req LayoutRequest) *LayoutPage {
	return pw.layout.Render(req)
}

func (pw *PosterWall) GetCategories(field string) []string {
	return pw.category.ListCategories(field)
}

func (pw *PosterWall) AddToWatchList(mediaID string, note string) {
	pw.watch.Add(mediaID, note)
}

func (pw *PosterWall) RemoveFromWatchList(mediaID string) bool {
	return pw.watch.Remove(mediaID)
}

func (pw *PosterWall) Recommend(limit int) []*MediaItem {
	return pw.watch.Recommend(pw.AllItems(), limit)
}

// ---------------------------------------------------------------------------
// RESTful API Handler
// ---------------------------------------------------------------------------

type APIHandler struct {
	pw *PosterWall
}

func NewAPIHandler(pw *PosterWall) *APIHandler {
	return &APIHandler{pw: pw}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] != "api" || parts[1] != "posterwall" {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	resource := ""
	if len(parts) >= 3 {
		resource = parts[2]
	}
	switch resource {
	case "items":
		h.handleItems(w, r, parts[3:])
	case "scrape":
		h.handleScrape(w, r)
	case "layout":
		h.handleLayout(w, r)
	case "progress":
		h.handleProgress(w, r, parts[3:])
	case "categories":
		h.handleCategories(w, r)
	case "watchlist":
		h.handleWatchList(w, r, parts[3:])
	case "recommend":
		h.handleRecommend(w, r)
	default:
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown endpoint"})
	}
}

func (h *APIHandler) handleItems(w http.ResponseWriter, r *http.Request, rest []string) {
	switch r.Method {
	case http.MethodGet:
		if len(rest) > 0 && rest[0] != "" {
			item, ok := h.pw.GetItem(rest[0])
			if !ok {
				h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
				return
			}
			h.writeJSON(w, http.StatusOK, item)
		} else {
			h.writeJSON(w, http.StatusOK, map[string]any{"items": h.pw.AllItems(), "total": len(h.pw.AllItems())})
		}
	case http.MethodDelete:
		if len(rest) > 0 && rest[0] != "" {
			if h.pw.RemoveItem(rest[0]) {
				h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			} else {
				h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			}
		} else {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item id required"})
		}
	default:
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *APIHandler) handleScrape(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Title string    `json:"title"`
		Year  int       `json:"year"`
		Type  MediaType `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Title == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if req.Type == "" {
		req.Type = MediaTypeMovie
	}
	item, err := h.pw.ScrapeAndAdd(r.Context(), req.Title, req.Year, req.Type)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusCreated, item)
}

func (h *APIHandler) handleLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	req := LayoutRequest{
		Mode:          LayoutMode(q.Get("mode")),
		SortBy:        SortOrder(q.Get("sort")),
		CategoryField: q.Get("field"),
		Category:      q.Get("category"),
		Filter:        q.Get("filter"),
	}
	fmt.Sscanf(q.Get("page"), "%d", &req.Page)
	fmt.Sscanf(q.Get("page_size"), "%d", &req.PageSize)
	if req.Mode == "" {
		req.Mode = LayoutGrid
	}
	if req.SortBy == "" {
		req.SortBy = SortByTitle
	}
	page := h.pw.RenderLayout(req)
	h.writeJSON(w, http.StatusOK, page)
}

func (h *APIHandler) handleProgress(w http.ResponseWriter, r *http.Request, rest []string) {
	switch r.Method {
	case http.MethodGet:
		if len(rest) > 0 && rest[0] != "" {
			p, ok := h.pw.GetProgress(rest[0])
			if !ok {
				h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no progress"})
				return
			}
			h.writeJSON(w, http.StatusOK, p)
		} else {
			h.writeJSON(w, http.StatusOK, h.pw.progress.AllProgress())
		}
	case http.MethodPost, http.MethodPut:
		var p ProgressEntry
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		mediaID := p.MediaID
		if len(rest) > 0 && rest[0] != "" {
			mediaID = rest[0]
		}
		if mediaID == "" {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "media_id is required"})
			return
		}
		h.pw.progress.UpdateProgress(mediaID, &p)
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *APIHandler) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	field := q.Get("field")
	if field == "" {
		h.writeJSON(w, http.StatusOK, map[string][]string{"fields": h.pw.category.AllCategoryFields()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string][]string{"categories": h.pw.GetCategories(field)})
}

func (h *APIHandler) handleWatchList(w http.ResponseWriter, r *http.Request, rest []string) {
	switch r.Method {
	case http.MethodGet:
		if len(rest) > 0 && rest[0] != "" {
			entry, ok := h.pw.watch.GetEntry(rest[0])
			if !ok {
				h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
				return
			}
			h.writeJSON(w, http.StatusOK, entry)
		} else {
			h.writeJSON(w, http.StatusOK, map[string]any{"entries": h.pw.watch.AllEntries(), "total": len(h.pw.watch.AllEntries())})
		}
	case http.MethodPost:
		var req struct {
			MediaID string `json:"media_id"`
			Note    string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.MediaID == "" {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "media_id is required"})
			return
		}
		h.pw.AddToWatchList(req.MediaID, req.Note)
		h.writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
	case http.MethodDelete:
		if len(rest) > 0 && rest[0] != "" {
			if h.pw.watch.Remove(rest[0]) {
				h.writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
			} else {
				h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
			}
		}
	default:
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *APIHandler) handleRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	limit := 10
	fmt.Sscanf(q.Get("limit"), "%d", &limit)
	items := h.pw.Recommend(limit)
	h.writeJSON(w, http.StatusOK, map[string]any{"recommendations": items, "count": len(items)})
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
