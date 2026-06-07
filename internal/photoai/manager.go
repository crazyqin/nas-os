// Package photoai 提供照片AI管理功能，包括智能分类、人脸聚类、EXIF提取、智能相册等。
package photoai

import (
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 照片AI管理器
type Manager struct {
	mu       sync.RWMutex
	config   *PhotoAIConfig
	photos   map[string]*Photo      // photoID -> Photo
	persons  map[string]*Person     // personID -> Person
	albums   map[string]*SmartAlbum // albumID -> SmartAlbum
	shares   map[string]*ShareLink  // shareID -> ShareLink
	faceMap  map[string][]string    // personID -> []faceID
	hashMap  map[string][]string    // perceptualHash -> []photoID
	scanning bool
}

// NewManager 创建管理器
func NewManager(cfg *PhotoAIConfig) *Manager {
	if cfg == nil {
		cfg = DefaultPhotoAIConfig()
	}
	return &Manager{
		config:  cfg,
		photos:  make(map[string]*Photo),
		persons: make(map[string]*Person),
		albums:  make(map[string]*SmartAlbum),
		shares:  make(map[string]*ShareLink),
		faceMap: make(map[string][]string),
		hashMap: make(map[string][]string),
	}
}

// ========== 照片管理 ==========

// AddPhoto 添加照片
func (m *Manager) AddPhoto(photo *Photo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if photo.ID == "" {
		photo.ID = uuid.New().String()
	}
	if _, exists := m.photos[photo.ID]; exists {
		return fmt.Errorf("photo %s already exists", photo.ID)
	}

	now := time.Now()
	if photo.CreatedAt.IsZero() {
		photo.CreatedAt = now
	}
	photo.UpdatedAt = now
	if photo.Status == "" {
		photo.Status = StatusPending
	}

	m.photos[photo.ID] = photo
	log.Printf("[photoai] photo added: %s (%s)", photo.ID, photo.Filename)
	return nil
}

// GetPhoto 获取照片
func (m *Manager) GetPhoto(id string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, ok := m.photos[id]
	if !ok {
		return nil, fmt.Errorf("photo %s not found", id)
	}
	return photo, nil
}

// ListPhotos 列出照片（分页）
func (m *Manager) ListPhotos(page, pageSize int) ([]*Photo, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := m.sortedPhotos()
	total := len(all)

	start := (page - 1) * pageSize
	if start >= total {
		return []*Photo{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total
}

// UpdatePhoto 更新照片信息
func (m *Manager) UpdatePhoto(photo *Photo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.photos[photo.ID]; !exists {
		return fmt.Errorf("photo %s not found", photo.ID)
	}

	photo.UpdatedAt = time.Now()
	m.photos[photo.ID] = photo
	return nil
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[id]
	if !exists {
		return fmt.Errorf("photo %s not found", id)
	}

	// 从相册中移除
	for _, albumID := range photo.Albums {
		if album, ok := m.albums[albumID]; ok {
			album.PhotoIDs = removeStr(album.PhotoIDs, id)
			album.PhotoCount = len(album.PhotoIDs)
		}
	}

	delete(m.photos, id)
	log.Printf("[photoai] photo deleted: %s", id)
	return nil
}

// SetFavorite 设置收藏
func (m *Manager) SetFavorite(id string, isFav bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[id]
	if !exists {
		return fmt.Errorf("photo %s not found", id)
	}

	photo.IsFavorite = isFav
	photo.UpdatedAt = time.Now()
	return nil
}

// BatchTag 批量标签操作
func (m *Manager) BatchTag(req *BatchTagRequest) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := 0
	for _, pid := range req.PhotoIDs {
		photo, ok := m.photos[pid]
		if !ok {
			continue
		}
		switch req.Action {
		case "set":
			photo.Tags = req.Tags
		case "remove":
			photo.Tags = removeSlice(photo.Tags, req.Tags)
		default: // add
			photo.Tags = mergeTags(photo.Tags, req.Tags)
		}
		photo.UpdatedAt = time.Now()
		updated++
	}
	return updated, nil
}

// ========== 扫描 ==========

// Scan 扫描目录
func (m *Manager) Scan(req *ScanRequest) (*ScanResult, error) {
	if req.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	m.mu.Lock()
	if m.scanning {
		m.mu.Unlock()
		return nil, fmt.Errorf("scan already in progress")
	}
	m.scanning = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.scanning = false
		m.mu.Unlock()
	}()

	start := time.Now()

	// 模拟扫描目录（实际实现需遍历文件系统）
	result := &ScanResult{
		TotalFound:  0,
		NewImported: 0,
		Skipped:     0,
		Duration:    time.Since(start).String(),
	}

	log.Printf("[photoai] scan completed: dir=%s new=%d skipped=%d",
		req.Directory, result.NewImported, result.Skipped)
	return result, nil
}

// ImportPhotos 导入照片
func (m *Manager) ImportPhotos(req *ImportRequest) (*ImportResult, error) {
	if len(req.Paths) == 0 {
		return nil, fmt.Errorf("paths is required")
	}

	result := &ImportResult{
		TotalFiles: len(req.Paths),
	}

	for _, p := range req.Paths {
		// 检查是否已存在
		m.mu.RLock()
		exists := false
		for _, ph := range m.photos {
			if ph.FilePath == p {
				exists = true
				break
			}
		}
		m.mu.RUnlock()

		if exists {
			result.Skipped++
			continue
		}

		photo := &Photo{
			ID:       uuid.New().String(),
			Filename: p,
			FilePath: p,
			Status:   StatusPending,
		}
		if err := m.AddPhoto(photo); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		result.Imported++
	}

	return result, nil
}

// ========== AI 分析 ==========

// AnalyzePhoto 分析单张照片
func (m *Manager) AnalyzePhoto(photoID string) (*AIAnalysisResult, error) {
	m.mu.RLock()
	photo, exists := m.photos[photoID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("photo %s not found", photoID)
	}

	// 更新状态
	m.mu.Lock()
	photo.Status = StatusProcessing
	m.mu.Unlock()

	// 模拟 AI 分析（实际实现需调用 AI 模型）
	result := &AIAnalysisResult{
		Categories: []PhotoCategory{CategoryOther},
		Tags:       []string{},
		Score:      50.0,
	}

	// 更新照片信息
	m.mu.Lock()
	photo.Status = StatusReady
	photo.Categories = result.Categories
	photo.Tags = result.Tags
	photo.Score = result.Score
	photo.Faces = result.Faces
	photo.UpdatedAt = time.Now()
	m.mu.Unlock()

	log.Printf("[photoai] photo analyzed: %s score=%.1f", photoID, result.Score)
	return result, nil
}

// AnalyzePending 分析所有待处理照片
func (m *Manager) AnalyzePending() (int, int, error) {
	m.mu.RLock()
	var pending []string
	for id, p := range m.photos {
		if p.Status == StatusPending {
			pending = append(pending, id)
		}
	}
	m.mu.RUnlock()

	success := 0
	failed := 0
	for _, id := range pending {
		if _, err := m.AnalyzePhoto(id); err != nil {
			failed++
			m.mu.Lock()
			if p, ok := m.photos[id]; ok {
				p.Status = StatusFailed
			}
			m.mu.Unlock()
			continue
		}
		success++
	}
	return success, failed, nil
}

// ========== 人脸识别 & 聚类 ==========

// GetPersons 获取所有人物列表
func (m *Manager) GetPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	persons := make([]*Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, p)
	}
	sort.Slice(persons, func(i, j int) bool {
		return persons[i].PhotoCount > persons[j].PhotoCount
	})
	return persons
}

// GetPerson 获取人物信息
func (m *Manager) GetPerson(id string) (*Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	person, ok := m.persons[id]
	if !ok {
		return nil, fmt.Errorf("person %s not found", id)
	}
	return person, nil
}

// RenamePerson 重命名人物
func (m *Manager) RenamePerson(id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, ok := m.persons[id]
	if !ok {
		return fmt.Errorf("person %s not found", id)
	}

	person.Name = name
	person.UpdatedAt = time.Now()
	return nil
}

// MergePersons 合并两个人物
func (m *Manager) MergePersons(targetID, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.persons[targetID]
	if !ok {
		return fmt.Errorf("person %s not found", targetID)
	}
	source, ok := m.persons[sourceID]
	if !ok {
		return fmt.Errorf("person %s not found", sourceID)
	}

	// 将 source 的 face 移到 target
	for _, faceID := range source.FaceIDs {
		target.FaceIDs = append(target.FaceIDs, faceID)
		m.faceMap[targetID] = append(m.faceMap[targetID], faceID)
	}
	for _, pid := range source.PhotoIDs {
		if !containsStr(target.PhotoIDs, pid) {
			target.PhotoIDs = append(target.PhotoIDs, pid)
		}
	}
	target.PhotoCount = len(target.PhotoIDs)
	target.UpdatedAt = time.Now()

	// 更新照片中的 person_id
	for _, faceID := range source.FaceIDs {
		for _, photo := range m.photos {
			for _, face := range photo.Faces {
				if face.ID == faceID {
					face.PersonID = targetID
					face.PersonName = target.Name
				}
			}
		}
	}

	delete(m.persons, sourceID)
	delete(m.faceMap, sourceID)

	log.Printf("[photoai] persons merged: %s <- %s", targetID, sourceID)
	return nil
}

// RunFaceClustering 运行人脸聚类
func (m *Manager) RunFaceClustering() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟聚类：将有 embedding 的 face 按相似度聚类
	personCount := len(m.persons)

	// 实际实现需用向量相似度算法（如 DBSCAN 或层次聚类）
	log.Printf("[photoai] face clustering completed: %d persons", personCount)
	return personCount, nil
}

// ========== 搜索 ==========

// SearchPhotos 搜索照片
func (m *Manager) SearchPhotos(query *SearchQuery) *SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	var matched []*Photo
	for _, photo := range m.photos {
		if m.matchPhoto(photo, query) {
			matched = append(matched, photo)
		}
	}

	// 排序
	m.sortPhotos(matched, query.SortBy, query.SortOrder)

	total := len(matched)
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return &SearchResult{
			Photos:   []*Photo{},
			Total:    total,
			Page:     query.Page,
			PageSize: query.PageSize,
		}
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return &SearchResult{
		Photos:   matched[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}
}

func (m *Manager) matchPhoto(photo *Photo, q *SearchQuery) bool {
	// 关键词匹配
	if q.Keywords != "" {
		kw := strings.ToLower(q.Keywords)
		if !strings.Contains(strings.ToLower(photo.Filename), kw) &&
			!matchTags(photo.Tags, kw) {
			return false
		}
	}

	// 分类匹配
	if len(q.Categories) > 0 {
		if !matchCategories(photo.Categories, q.Categories) {
			return false
		}
	}

	// 标签匹配
	if len(q.Tags) > 0 {
		if !matchAnyTag(photo.Tags, q.Tags) {
			return false
		}
	}

	// 人物匹配
	if len(q.PersonIDs) > 0 {
		hasPerson := false
		for _, face := range photo.Faces {
			for _, pid := range q.PersonIDs {
				if face.PersonID == pid {
					hasPerson = true
					break
				}
			}
			if hasPerson {
				break
			}
		}
		if !hasPerson {
			return false
		}
	}

	// 日期范围
	if q.DateFrom != nil && photo.TakenAt != nil && photo.TakenAt.Before(*q.DateFrom) {
		return false
	}
	if q.DateTo != nil && photo.TakenAt != nil && photo.TakenAt.After(*q.DateTo) {
		return false
	}

	// 位置过滤
	if q.Location != nil && photo.EXIF != nil && photo.EXIF.GPS != nil {
		dist := haversineKm(
			q.Location.Latitude, q.Location.Longitude,
			photo.EXIF.GPS.Latitude, photo.EXIF.GPS.Longitude,
		)
		if dist > q.Location.RadiusKm {
			return false
		}
	}

	// 评分过滤
	if q.MinScore != nil && photo.Score < *q.MinScore {
		return false
	}

	return true
}

func (m *Manager) sortPhotos(photos []*Photo, sortBy, order string) {
	desc := order == "desc"

	switch sortBy {
	case "score":
		sort.Slice(photos, func(i, j int) bool {
			if desc {
				return photos[i].Score > photos[j].Score
			}
			return photos[i].Score < photos[j].Score
		})
	case "filename":
		sort.Slice(photos, func(i, j int) bool {
			if desc {
				return photos[i].Filename > photos[j].Filename
			}
			return photos[i].Filename < photos[j].Filename
		})
	default: // date
		sort.Slice(photos, func(i, j int) bool {
			ti := takenOrCreated(photos[i])
			tj := takenOrCreated(photos[j])
			if desc {
				return ti.After(tj)
			}
			return ti.Before(tj)
		})
	}
}

// ========== 智能相册 ==========

// CreateAlbum 创建智能相册
func (m *Manager) CreateAlbum(req *AlbumRequest) (*SmartAlbum, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("album name is required")
	}
	if len(req.Rules) == 0 {
		return nil, fmt.Errorf("album rules is required")
	}

	album := &SmartAlbum{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Rules:       req.Rules,
		IsSystem:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.albums[album.ID] = album
	log.Printf("[photoai] album created: %s (%s)", album.ID, album.Name)
	return album, nil
}

// GetAlbum 获取相册
func (m *Manager) GetAlbum(id string) (*SmartAlbum, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album %s not found", id)
	}
	return album, nil
}

// ListAlbums 列出所有相册
func (m *Manager) ListAlbums() []*SmartAlbum {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*SmartAlbum, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, a)
	}
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].Name < albums[j].Name
	})
	return albums
}

// UpdateAlbum 更新相册
func (m *Manager) UpdateAlbum(id string, req *AlbumRequest) (*SmartAlbum, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album %s not found", id)
	}

	if req.Name != "" {
		album.Name = req.Name
	}
	if req.Description != "" {
		album.Description = req.Description
	}
	if req.Type != "" {
		album.Type = req.Type
	}
	if len(req.Rules) > 0 {
		album.Rules = req.Rules
	}
	album.UpdatedAt = time.Now()

	return album, nil
}

// DeleteAlbum 删除相册
func (m *Manager) DeleteAlbum(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[id]
	if !ok {
		return fmt.Errorf("album %s not found", id)
	}
	if album.IsSystem {
		return fmt.Errorf("cannot delete system album")
	}

	delete(m.albums, id)
	log.Printf("[photoai] album deleted: %s", id)
	return nil
}

// RefreshAlbums 刷新所有智能相册（重新匹配规则）
func (m *Manager) RefreshAlbums() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := 0
	for _, album := range m.albums {
		var matchedIDs []string
		for _, photo := range m.photos {
			if m.matchAlbumRules(photo, album.Rules) {
				matchedIDs = append(matchedIDs, photo.ID)
			}
		}
		album.PhotoIDs = matchedIDs
		album.PhotoCount = len(matchedIDs)
		album.UpdatedAt = time.Now()

		// 设置封面
		if len(matchedIDs) > 0 {
			album.CoverPhoto = matchedIDs[0]
		}
		updated++
	}

	log.Printf("[photoai] albums refreshed: %d", updated)
	return updated
}

func (m *Manager) matchAlbumRules(photo *Photo, rules []AlbumRule) bool {
	for _, rule := range rules {
		if !matchRule(photo, rule) {
			return false
		}
	}
	return true
}

// ========== 去重 ==========

// DetectDuplicates 检测重复照片
func (m *Manager) DetectDuplicates() []*DuplicateGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟感知哈希（实际实现需用 pHash/dHash）
	hashPhoto := func(p *Photo) string {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%d", p.Filename, p.Width, p.Height)))
		return fmt.Sprintf("%x", h[:8])
	}

	hashMap := make(map[string][]string)
	for _, photo := range m.photos {
		hash := hashPhoto(photo)
		hashMap[hash] = append(hashMap[hash], photo.ID)
	}

	var groups []*DuplicateGroup
	for hash, ids := range hashMap {
		if len(ids) > 1 {
			groups = append(groups, &DuplicateGroup{
				Hash:     hash,
				PhotoIDs: ids,
			})
			// 更新照片的 duplicates 字段
			for _, id := range ids {
				if p, ok := m.photos[id]; ok {
					p.Duplicates = ids
				}
			}
		}
	}

	m.hashMap = hashMap
	log.Printf("[photoai] duplicate detection: %d groups found", len(groups))
	return groups
}

// ========== 分享 ==========

// CreateShareLink 创建分享链接
func (m *Manager) CreateShareLink(req *ShareRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(req.PhotoIDs) == 0 {
		return nil, fmt.Errorf("photo_ids is required")
	}

	link := &ShareLink{
		ID:        uuid.New().String(),
		Token:     uuid.New().String()[:8],
		PhotoIDs:  req.PhotoIDs,
		Password:  req.Password,
		MaxViews:  req.MaxViews,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		link.ExpiresAt = &exp
	}

	m.shares[link.ID] = link
	log.Printf("[photoai] share link created: %s (photos=%d)", link.ID, len(req.PhotoIDs))
	return link, nil
}

// GetShareLink 获取分享链接
func (m *Manager) GetShareLink(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.shares {
		if link.Token == token {
			// 检查是否过期
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("share link expired")
			}
			if link.MaxViews > 0 && link.ViewCount >= link.MaxViews {
				return nil, fmt.Errorf("share link max views reached")
			}
			return link, nil
		}
	}
	return nil, fmt.Errorf("share link not found")
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.photos)
	ready := 0
	pending := 0
	failed := 0
	favorites := 0
	totalScore := 0.0
	categories := make(map[PhotoCategory]int)

	for _, p := range m.photos {
		switch p.Status {
		case StatusReady:
			ready++
		case StatusPending:
			pending++
		case StatusFailed:
			failed++
		}
		if p.IsFavorite {
			favorites++
		}
		totalScore += p.Score
		for _, cat := range p.Categories {
			categories[cat]++
		}
	}

	avgScore := 0.0
	if total > 0 {
		avgScore = totalScore / float64(total)
	}

	return map[string]interface{}{
		"total_photos": total,
		"ready":        ready,
		"pending":      pending,
		"failed":       failed,
		"favorites":    favorites,
		"persons":      len(m.persons),
		"albums":       len(m.albums),
		"share_links":  len(m.shares),
		"avg_score":    math.Round(avgScore*10) / 10,
		"categories":   categories,
	}
}

// GetCategoryStats 获取分类统计
func (m *Manager) GetCategoryStats() map[PhotoCategory]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categories := make(map[PhotoCategory]int)
	for _, p := range m.photos {
		for _, cat := range p.Categories {
			categories[cat]++
		}
	}
	return categories
}

// GetTimeline 获取时间线统计（按月）
func (m *Manager) GetTimeline() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	timeline := make(map[string]int)
	for _, p := range m.photos {
		t := takenOrCreated(p)
		key := t.Format("2006-01")
		timeline[key]++
	}
	return timeline
}

// ========== 辅助函数 ==========

func (m *Manager) sortedPhotos() []*Photo {
	photos := make([]*Photo, 0, len(m.photos))
	for _, p := range m.photos {
		photos = append(photos, p)
	}
	sort.Slice(photos, func(i, j int) bool {
		return photos[i].CreatedAt.After(photos[j].CreatedAt)
	})
	return photos
}

func takenOrCreated(p *Photo) time.Time {
	if p.TakenAt != nil {
		return *p.TakenAt
	}
	return p.CreatedAt
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func removeStr(ss []string, s string) []string {
	var result []string
	for _, v := range ss {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

func removeSlice(tags, toRemove []string) []string {
	removeSet := make(map[string]bool, len(toRemove))
	for _, t := range toRemove {
		removeSet[t] = true
	}
	var result []string
	for _, t := range tags {
		if !removeSet[t] {
			result = append(result, t)
		}
	}
	return result
}

func mergeTags(existing, newTags []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, t := range existing {
		seen[t] = true
	}
	for _, t := range newTags {
		if !seen[t] {
			existing = append(existing, t)
			seen[t] = true
		}
	}
	return existing
}

func matchTags(tags []string, keyword string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), keyword) {
			return true
		}
	}
	return false
}

func matchCategories(photoCats, queryCats []PhotoCategory) bool {
	for _, pc := range photoCats {
		for _, qc := range queryCats {
			if pc == qc {
				return true
			}
		}
	}
	return false
}

func matchAnyTag(tags []string, queryTags []string) bool {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, qt := range queryTags {
		if tagSet[strings.ToLower(qt)] {
			return true
		}
	}
	return false
}

func matchRule(photo *Photo, rule AlbumRule) bool {
	switch rule.Field {
	case "category":
		for _, cat := range photo.Categories {
			if string(cat) == fmt.Sprintf("%v", rule.Value) {
				return true
			}
		}
	case "tag":
		val := fmt.Sprintf("%v", rule.Value)
		for _, t := range photo.Tags {
			if strings.EqualFold(t, val) {
				return true
			}
		}
	case "score":
		if fv, ok := toFloat(rule.Value); ok {
			switch rule.Operator {
			case "gt":
				return photo.Score > fv
			case "lt":
				return photo.Score < fv
			default:
				return photo.Score >= fv
			}
		}
	case "person_id":
		pid := fmt.Sprintf("%v", rule.Value)
		for _, face := range photo.Faces {
			if face.PersonID == pid {
				return true
			}
		}
	}
	return false
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// haversineKm 计算两点之间的距离（公里）
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // 地球半径（公里）
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
