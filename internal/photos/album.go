// Package photos 提供智能相册管理功能
// 功能：照片自动分类、智能相册、照片去重
// 参考：群晖 Photos 设计
package photos

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ==================== 自动分类功能 ====================

// AutoClassifier 自动分类器
type AutoClassifier struct {
	manager    *Manager
	categories map[string]*PhotoCategory
	mu         sync.RWMutex
}

// PhotoCategory 照片分类
type PhotoCategory struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // person, location, time, scene, object
	PhotoIDs    []string          `json:"photoIds"`
	CoverID     string            `json:"coverId"`
	SubCategories []*PhotoCategory `json:"subCategories,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	PersonCategories  []*PhotoCategory `json:"personCategories"`
	LocationCategories []*PhotoCategory `json:"locationCategories"`
	TimeCategories    []*PhotoCategory `json:"timeCategories"`
	SceneCategories   []*PhotoCategory `json:"sceneCategories"`
}

// NewAutoClassifier 创建自动分类器
func NewAutoClassifier(manager *Manager) *AutoClassifier {
	return &AutoClassifier{
		manager:    manager,
		categories: make(map[string]*PhotoCategory),
	}
}

// ClassifyAll 自动分类所有照片
func (ac *AutoClassifier) ClassifyAll() (*ClassificationResult, error) {
	ac.manager.mu.RLock()
	photos := make([]*Photo, 0, len(ac.manager.photos))
	for _, p := range ac.manager.photos {
		photos = append(photos, p)
	}
	ac.manager.mu.RUnlock()

	result := &ClassificationResult{
		PersonCategories:  make([]*PhotoCategory, 0),
		LocationCategories: make([]*PhotoCategory, 0),
		TimeCategories:    make([]*PhotoCategory, 0),
		SceneCategories:   make([]*PhotoCategory, 0),
	}

	// 按人物分类
	personCats := ac.ClassifyByPerson(photos)
	result.PersonCategories = personCats

	// 按地点分类
	locationCats := ac.ClassifyByLocation(photos)
	result.LocationCategories = locationCats

	// 按时间分类
	timeCats := ac.ClassifyByTime(photos)
	result.TimeCategories = timeCats

	// 按场景分类
	sceneCats := ac.ClassifyByScene(photos)
	result.SceneCategories = sceneCats

	// 保存分类结果
	if err := ac.saveCategories(); err != nil {
		return nil, fmt.Errorf("保存分类结果失败: %w", err)
	}

	return result, nil
}

// ClassifyByPerson 按人物分类照片
// 参考：群晖 Photos 人物相册功能
func (ac *AutoClassifier) ClassifyByPerson(photos []*Photo) []*PhotoCategory {
	personPhotos := make(map[string][]*Photo)
	unnamedPhotos := make([]*Photo, 0)

	for _, photo := range photos {
		if len(photo.Faces) == 0 {
			continue
		}

		hasNamedFace := false
		for _, face := range photo.Faces {
			if face.Name != "" {
				personPhotos[face.Name] = append(personPhotos[face.Name], photo)
				hasNamedFace = true
			}
		}

		if !hasNamedFace && len(photo.Faces) > 0 {
			unnamedPhotos = append(unnamedPhotos, photo)
		}
	}

	categories := make([]*PhotoCategory, 0)

	// 创建已命名人物分类
	for name, personPhotoList := range personPhotos {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      name,
			Type:      "person",
			PhotoIDs:  make([]string, 0, len(personPhotoList)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		for _, p := range personPhotoList {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		// 设置封面为第一张照片
		if len(personPhotoList) > 0 {
			cat.CoverID = personPhotoList[0].ID
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 创建"未命名人物"分类
	if len(unnamedPhotos) > 0 {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      "未命名人物",
			Type:      "person",
			PhotoIDs:  make([]string, 0, len(unnamedPhotos)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"isUnnamed": true,
			},
		}

		for _, p := range unnamedPhotos {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		if len(unnamedPhotos) > 0 {
			cat.CoverID = unnamedPhotos[0].ID
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 按照片数量排序
	sort.Slice(categories, func(i, j int) bool {
		return len(categories[i].PhotoIDs) > len(categories[j].PhotoIDs)
	})

	return categories
}

// ClassifyByLocation 按地点分类照片
// 参考：群晖 Photos 地点相册功能，支持 GPS 聚类
func (ac *AutoClassifier) ClassifyByLocation(photos []*Photo) []*PhotoCategory {
	locationPhotos := make(map[string][]*Photo)
	noLocationPhotos := make([]*Photo, 0)

	for _, photo := range photos {
		if photo.Location == nil {
			noLocationPhotos = append(noLocationPhotos, photo)
			continue
		}

		// 使用城市作为一级分类
		locationKey := photo.Location.City
		if locationKey == "" {
			locationKey = photo.Location.Country
		}
		if locationKey == "" {
			// 使用 GPS 坐标的大致区域
			locationKey = fmt.Sprintf("%.2f,%.2f", 
				roundTo(photo.Location.Latitude, 2),
				roundTo(photo.Location.Longitude, 2))
		}

		locationPhotos[locationKey] = append(locationPhotos[locationKey], photo)
	}

	categories := make([]*PhotoCategory, 0)

	// 创建地点分类
	for location, locPhotos := range locationPhotos {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      location,
			Type:      "location",
			PhotoIDs:  make([]string, 0, len(locPhotos)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		for _, p := range locPhotos {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		if len(locPhotos) > 0 {
			cat.CoverID = locPhotos[0].ID
		}

		// 存储地点元数据
		if locPhotos[0].Location != nil {
			cat.Metadata = map[string]interface{}{
				"latitude":  locPhotos[0].Location.Latitude,
				"longitude": locPhotos[0].Location.Longitude,
				"country":   locPhotos[0].Location.Country,
				"city":      locPhotos[0].Location.City,
			}
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 创建"无地点信息"分类
	if len(noLocationPhotos) > 0 {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      "无地点信息",
			Type:      "location",
			PhotoIDs:  make([]string, 0, len(noLocationPhotos)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"noLocation": true,
			},
		}

		for _, p := range noLocationPhotos {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 按照片数量排序
	sort.Slice(categories, func(i, j int) bool {
		return len(categories[i].PhotoIDs) > len(categories[j].PhotoIDs)
	})

	return categories
}

// ClassifyByTime 按时间分类照片
// 参考：群晖 Photos 时间线功能，支持年/月/日分组
func (ac *AutoClassifier) ClassifyByTime(photos []*Photo) []*PhotoCategory {
	yearPhotos := make(map[int][]*Photo)
	monthPhotos := make(map[string][]*Photo) // "2024-01" 格式
	recentPhotos := make([]*Photo, 0)        // 最近30天

	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	for _, photo := range photos {
		if photo.TakenAt.IsZero() {
			continue
		}

		year := photo.TakenAt.Year()
		yearPhotos[year] = append(yearPhotos[year], photo)

		monthKey := photo.TakenAt.Format("2006-01")
		monthPhotos[monthKey] = append(monthPhotos[monthKey], photo)

		// 最近照片
		if photo.TakenAt.After(thirtyDaysAgo) {
			recentPhotos = append(recentPhotos, photo)
		}
	}

	categories := make([]*PhotoCategory, 0)

	// 创建年份分类
	for year, yearPhotoList := range yearPhotos {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      fmt.Sprintf("%d年", year),
			Type:      "time",
			PhotoIDs:  make([]string, 0, len(yearPhotoList)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"year": year,
			},
		}

		for _, p := range yearPhotoList {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		// 按月份创建子分类
		subCats := make([]*PhotoCategory, 0)
		for month, monthPhotoList := range monthPhotos {
			if !strings.HasPrefix(month, fmt.Sprintf("%d-", year)) {
				continue
			}
			subCat := &PhotoCategory{
				ID:        uuid.New().String(),
				Name:      fmt.Sprintf("%s月", month[5:]),
				Type:      "time",
				PhotoIDs:  make([]string, 0, len(monthPhotoList)),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Metadata: map[string]interface{}{
					"month": month,
				},
			}
			for _, p := range monthPhotoList {
				subCat.PhotoIDs = append(subCat.PhotoIDs, p.ID)
			}
			if len(monthPhotoList) > 0 {
				subCat.CoverID = monthPhotoList[0].ID
			}
			subCats = append(subCats, subCat)
		}
		// 按月份排序
		sort.Slice(subCats, func(i, j int) bool {
			return subCats[i].Name < subCats[j].Name
		})
		cat.SubCategories = subCats

		if len(yearPhotoList) > 0 {
			cat.CoverID = yearPhotoList[0].ID
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 创建"最近照片"分类
	if len(recentPhotos) > 0 {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      "最近30天",
			Type:      "time",
			PhotoIDs:  make([]string, 0, len(recentPhotos)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"recent": true,
			},
		}

		for _, p := range recentPhotos {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		if len(recentPhotos) > 0 {
			cat.CoverID = recentPhotos[0].ID
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 按年份降序排序
	sort.Slice(categories, func(i, j int) bool {
		yearI, _ := categories[i].Metadata["year"].(int)
		yearJ, _ := categories[j].Metadata["year"].(int)
		return yearI > yearJ
	})

	return categories
}

// ClassifyByScene 按场景分类照片
// 参考：群晖 Photos 场景识别功能
func (ac *AutoClassifier) ClassifyByScene(photos []*Photo) []*PhotoCategory {
	scenePhotos := make(map[string][]*Photo)
	noScenePhotos := make([]*Photo, 0)

	for _, photo := range photos {
		if photo.Scene == "" {
			noScenePhotos = append(noScenePhotos, photo)
			continue
		}

		scenePhotos[photo.Scene] = append(scenePhotos[photo.Scene], photo)
	}

	categories := make([]*PhotoCategory, 0)

	// 创建场景分类
	for scene, scenePhotoList := range scenePhotos {
		displayName := ac.getSceneDisplayName(scene)
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      displayName,
			Type:      "scene",
			PhotoIDs:  make([]string, 0, len(scenePhotoList)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"scene": scene,
			},
		}

		for _, p := range scenePhotoList {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		if len(scenePhotoList) > 0 {
			cat.CoverID = scenePhotoList[0].ID
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 创建"未分类场景"分类
	if len(noScenePhotos) > 0 {
		cat := &PhotoCategory{
			ID:        uuid.New().String(),
			Name:      "未分类场景",
			Type:      "scene",
			PhotoIDs:  make([]string, 0, len(noScenePhotos)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		for _, p := range noScenePhotos {
			cat.PhotoIDs = append(cat.PhotoIDs, p.ID)
		}

		categories = append(categories, cat)
		ac.mu.Lock()
		ac.categories[cat.ID] = cat
		ac.mu.Unlock()
	}

	// 按照片数量排序
	sort.Slice(categories, func(i, j int) bool {
		return len(categories[i].PhotoIDs) > len(categories[j].PhotoIDs)
	})

	return categories
}

// getSceneDisplayName 获取场景的显示名称
func (ac *AutoClassifier) getSceneDisplayName(scene string) string {
	sceneNames := map[string]string{
		"indoor":    "室内",
		"outdoor":   "户外",
		"nature":    "自然风光",
		"landscape": "风景",
		"sky":       "天空",
		"night":     "夜景",
		"sunset":    "日落",
		"beach":     "海滩",
		"mountain":  "山脉",
		"city":      "城市",
		"portrait":  "人像",
		"food":      "美食",
	}

	if name, ok := sceneNames[strings.ToLower(scene)]; ok {
		return name
	}
	return scene
}

// GetCategory 获取分类
func (ac *AutoClassifier) GetCategory(categoryID string) (*PhotoCategory, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	cat, exists := ac.categories[categoryID]
	if !exists {
		return nil, fmt.Errorf("分类不存在")
	}

	return cat, nil
}

// ListCategories 列出所有分类
func (ac *AutoClassifier) ListCategories(categoryType string) []*PhotoCategory {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	result := make([]*PhotoCategory, 0)
	for _, cat := range ac.categories {
		if categoryType == "" || cat.Type == categoryType {
			result = append(result, cat)
		}
	}

	return result
}

// saveCategories 保存分类数据
func (ac *AutoClassifier) saveCategories() error {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	categories := make([]PhotoCategory, 0, len(ac.categories))
	for _, cat := range ac.categories {
		categories = append(categories, *cat)
	}

	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(ac.manager.dataDir, "photo-categories.json")
	return os.WriteFile(path, data, 0640)
}

// loadCategories 加载分类数据
func (ac *AutoClassifier) loadCategories() error {
	path := filepath.Join(ac.manager.dataDir, "photo-categories.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var categories []PhotoCategory
	if err := json.Unmarshal(data, &categories); err != nil {
		return err
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	for i := range categories {
		ac.categories[categories[i].ID] = &categories[i]
	}

	return nil
}

// ==================== 智能相册功能 ====================

// SmartAlbumManager 智能相册管理器
// 参考：群晖 Photos 智能相册功能
type SmartAlbumManager struct {
	manager      *Manager
	classifier   *AutoClassifier
	smartAlbums  map[string]*SmartAlbumConfig
	rules        map[string]*SmartAlbumRule
	mu           sync.RWMutex
}

// SmartAlbumConfig 智能相册配置
type SmartAlbumConfig struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon"`
	Rules       []SmartAlbumRule       `json:"rules"`
	MatchMode   string                 `json:"matchMode"` // "all" (AND) or "any" (OR)
	AutoUpdate  bool                   `json:"autoUpdate"`
	PhotoIDs    []string               `json:"photoIds"`
	CoverID     string                 `json:"coverId"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SmartAlbumRule 智能相册规则
type SmartAlbumRule struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`   // person, location, time, scene, object, tag, quality
	Field       string      `json:"field"`  // 具体字段名
	Operator    string      `json:"operator"` // equals, contains, gt, lt, between, in
	Value       interface{} `json:"value"`
	DisplayName string      `json:"displayName"`
}

// SmartAlbumTemplate 智能相册模板
type SmartAlbumTemplate struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Icon        string           `json:"icon"`
	Rules       []SmartAlbumRule `json:"rules"`
	MatchMode   string           `json:"matchMode"`
	IsBuiltin   bool             `json:"isBuiltin"`
}

// NewSmartAlbumManager 创建智能相册管理器
func NewSmartAlbumManager(manager *Manager, classifier *AutoClassifier) *SmartAlbumManager {
	sam := &SmartAlbumManager{
		manager:     manager,
		classifier:  classifier,
		smartAlbums: make(map[string]*SmartAlbumConfig),
		rules:       make(map[string]*SmartAlbumRule),
	}

	// 加载已保存的智能相册
	_ = sam.loadSmartAlbums()

	// 创建内置模板
	sam.createBuiltinTemplates()

	return sam
}

// CreateSmartAlbum 创建智能相册
func (sam *SmartAlbumManager) CreateSmartAlbum(name, description string, rules []SmartAlbumRule, matchMode string) (*SmartAlbumConfig, error) {
	if name == "" {
		return nil, fmt.Errorf("相册名称不能为空")
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("至少需要一条规则")
	}

	album := &SmartAlbumConfig{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Rules:       rules,
		MatchMode:   matchMode,
		AutoUpdate:  true,
		PhotoIDs:    make([]string, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 立即匹配照片
	sam.matchPhotos(album)

	sam.mu.Lock()
	sam.smartAlbums[album.ID] = album
	sam.mu.Unlock()

	// 保存
	_ = sam.saveSmartAlbums()

	return album, nil
}

// CreateFromTemplate 从模板创建智能相册
func (sam *SmartAlbumManager) CreateFromTemplate(templateID string, customName string) (*SmartAlbumConfig, error) {
	template := sam.GetTemplate(templateID)
	if template == nil {
		return nil, fmt.Errorf("模板不存在")
	}

	name := customName
	if name == "" {
		name = template.Name
	}

	return sam.CreateSmartAlbum(name, template.Description, template.Rules, template.MatchMode)
}

// matchPhotos 匹配照片
func (sam *SmartAlbumManager) matchPhotos(album *SmartAlbumConfig) {
	sam.manager.mu.RLock()
	defer sam.manager.mu.RUnlock()

	matchedIDs := make([]string, 0)

	for _, photo := range sam.manager.photos {
		if sam.photoMatchesRules(photo, album.Rules, album.MatchMode) {
			matchedIDs = append(matchedIDs, photo.ID)
		}
	}

	album.PhotoIDs = matchedIDs
	album.UpdatedAt = time.Now()

	// 设置封面
	if len(matchedIDs) > 0 {
		album.CoverID = matchedIDs[0]
	} else {
		album.CoverID = ""
	}
}

// photoMatchesRules 检查照片是否匹配规则
func (sam *SmartAlbumManager) photoMatchesRules(photo *Photo, rules []SmartAlbumRule, matchMode string) bool {
	if len(rules) == 0 {
		return false
	}

	for _, rule := range rules {
		matches := sam.photoMatchesRule(photo, rule)

		if matchMode == "any" && matches {
			return true
		}

		if matchMode == "all" && !matches {
			return false
		}
	}

	return matchMode == "all"
}

// photoMatchesRule 检查照片是否匹配单条规则
func (sam *SmartAlbumManager) photoMatchesRule(photo *Photo, rule SmartAlbumRule) bool {
	switch rule.Type {
	case "person":
		return sam.matchPerson(photo, rule)
	case "location":
		return sam.matchLocation(photo, rule)
	case "time":
		return sam.matchTime(photo, rule)
	case "scene":
		return sam.matchScene(photo, rule)
	case "object":
		return sam.matchObject(photo, rule)
	case "tag":
		return sam.matchTag(photo, rule)
	case "quality":
		return sam.matchQuality(photo, rule)
	case "rating":
		return sam.matchRating(photo, rule)
	default:
		return false
	}
}

// matchPerson 匹配人物
func (sam *SmartAlbumManager) matchPerson(photo *Photo, rule SmartAlbumRule) bool {
	personName, ok := rule.Value.(string)
	if !ok {
		return false
	}

	for _, face := range photo.Faces {
		if face.Name == personName {
			return true
		}
	}

	return false
}

// matchLocation 匹配地点
func (sam *SmartAlbumManager) matchLocation(photo *Photo, rule SmartAlbumRule) bool {
	if photo.Location == nil {
		return false
	}

	switch rule.Operator {
	case "equals":
		if city, ok := rule.Value.(string); ok {
			return photo.Location.City == city
		}
	case "contains":
		if country, ok := rule.Value.(string); ok {
			return strings.Contains(photo.Location.Country, country)
		}
	}

	return false
}

// matchTime 匹配时间
func (sam *SmartAlbumManager) matchTime(photo *Photo, rule SmartAlbumRule) bool {
	if photo.TakenAt.IsZero() {
		return false
	}

	switch rule.Operator {
	case "between":
		if dateRange, ok := rule.Value.(map[string]interface{}); ok {
			if startStr, ok := dateRange["start"].(string); ok {
				if endStr, ok := dateRange["end"].(string); ok {
					start, _ := time.Parse("2006-01-02", startStr)
					end, _ := time.Parse("2006-01-02", endStr)
					return (photo.TakenAt.Equal(start) || photo.TakenAt.After(start)) &&
						(photo.TakenAt.Equal(end) || photo.TakenAt.Before(end))
				}
			}
		}
	case "year":
		if year, ok := rule.Value.(float64); ok {
			return photo.TakenAt.Year() == int(year)
		}
	case "month":
		if month, ok := rule.Value.(float64); ok {
			return int(photo.TakenAt.Month()) == int(month)
		}
	}

	return false
}

// matchScene 匹配场景
func (sam *SmartAlbumManager) matchScene(photo *Photo, rule SmartAlbumRule) bool {
	scene, ok := rule.Value.(string)
	if !ok {
		return false
	}

	switch rule.Operator {
	case "equals":
		return photo.Scene == scene
	case "contains":
		return strings.Contains(strings.ToLower(photo.Scene), strings.ToLower(scene))
	}

	return false
}

// matchObject 匹配物体
func (sam *SmartAlbumManager) matchObject(photo *Photo, rule SmartAlbumRule) bool {
	object, ok := rule.Value.(string)
	if !ok {
		return false
	}

	for _, obj := range photo.Objects {
		if rule.Operator == "equals" && obj == object {
			return true
		}
		if rule.Operator == "contains" && strings.Contains(strings.ToLower(obj), strings.ToLower(object)) {
			return true
		}
	}

	return false
}

// matchTag 匹配标签
func (sam *SmartAlbumManager) matchTag(photo *Photo, rule SmartAlbumRule) bool {
	tag, ok := rule.Value.(string)
	if !ok {
		return false
	}

	for _, t := range photo.Tags {
		if t == tag {
			return true
		}
	}

	return false
}

// matchQuality 匹配质量
func (sam *SmartAlbumManager) matchQuality(photo *Photo, rule SmartAlbumRule) bool {
	// 从 metadata 获取质量评分
	if sam.manager.photos[photo.ID] == nil {
		return false
	}

	// 简化实现：基于分辨率判断
	switch rule.Operator {
	case "gt":
		if minRes, ok := rule.Value.(float64); ok {
			return photo.Width > int(minRes) || photo.Height > int(minRes)
		}
	}

	return false
}

// matchRating 匹配评分
func (sam *SmartAlbumManager) matchRating(photo *Photo, rule SmartAlbumRule) bool {
	// 基于收藏状态判断
	if rule.Operator == "equals" && rule.Value == "favorite" {
		return photo.IsFavorite
	}
	return false
}

// UpdateSmartAlbum 更新智能相册
func (sam *SmartAlbumManager) UpdateSmartAlbum(albumID string, rules []SmartAlbumRule, matchMode string) (*SmartAlbumConfig, error) {
	sam.mu.Lock()
	album, exists := sam.smartAlbums[albumID]
	if !exists {
		sam.mu.Unlock()
		return nil, fmt.Errorf("智能相册不存在")
	}

	album.Rules = rules
	album.MatchMode = matchMode
	album.UpdatedAt = time.Now()
	sam.mu.Unlock()

	// 重新匹配照片（不在锁内执行，避免长时间持锁）
	sam.matchPhotos(album)

	sam.mu.Lock()
	_ = sam.saveSmartAlbumsInternal()
	sam.mu.Unlock()

	return album, nil
}

// DeleteSmartAlbum 删除智能相册
func (sam *SmartAlbumManager) DeleteSmartAlbum(albumID string) error {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	if _, exists := sam.smartAlbums[albumID]; !exists {
		return fmt.Errorf("智能相册不存在")
	}

	delete(sam.smartAlbums, albumID)
	_ = sam.saveSmartAlbums()

	return nil
}

// GetSmartAlbum 获取智能相册
func (sam *SmartAlbumManager) GetSmartAlbum(albumID string) (*SmartAlbumConfig, error) {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	album, exists := sam.smartAlbums[albumID]
	if !exists {
		return nil, fmt.Errorf("智能相册不存在")
	}

	return album, nil
}

// ListSmartAlbums 列出所有智能相册
func (sam *SmartAlbumManager) ListSmartAlbums() []*SmartAlbumConfig {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	result := make([]*SmartAlbumConfig, 0, len(sam.smartAlbums))
	for _, album := range sam.smartAlbums {
		result = append(result, album)
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// RefreshSmartAlbum 刷新智能相册
func (sam *SmartAlbumManager) RefreshSmartAlbum(albumID string) error {
	sam.mu.Lock()
	album, exists := sam.smartAlbums[albumID]
	sam.mu.Unlock()

	if !exists {
		return fmt.Errorf("智能相册不存在")
	}

	// 在锁外执行匹配
	sam.matchPhotos(album)

	sam.mu.Lock()
	_ = sam.saveSmartAlbumsInternal()
	sam.mu.Unlock()

	return nil
}

// RefreshAllSmartAlbums 刷新所有智能相册
func (sam *SmartAlbumManager) RefreshAllSmartAlbums() error {
	sam.mu.RLock()
	albums := make([]*SmartAlbumConfig, 0, len(sam.smartAlbums))
	for _, album := range sam.smartAlbums {
		if album.AutoUpdate {
			albums = append(albums, album)
		}
	}
	sam.mu.RUnlock()

	// 在锁外执行匹配
	for _, album := range albums {
		sam.matchPhotos(album)
	}

	sam.mu.Lock()
	_ = sam.saveSmartAlbumsInternal()
	sam.mu.Unlock()

	return nil
}

// GetTemplates 获取智能相册模板
func (sam *SmartAlbumManager) GetTemplate(templateID string) *SmartAlbumTemplate {
	templates := sam.GetBuiltinTemplates()
	for _, t := range templates {
		if t.ID == templateID {
			return &t
		}
	}
	return nil
}

// GetBuiltinTemplates 获取内置模板
func (sam *SmartAlbumManager) GetBuiltinTemplates() []SmartAlbumTemplate {
	return []SmartAlbumTemplate{
		{
			ID:          "template-favorites",
			Name:        "收藏照片",
			Description: "自动收集所有收藏的照片",
			Icon:        "star",
			MatchMode:   "all",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite", DisplayName: "收藏照片"},
			},
		},
		{
			ID:          "template-recent",
			Name:        "最近照片",
			Description: "最近30天拍摄的照片",
			Icon:        "clock",
			MatchMode:   "all",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "time", Operator: "recent", Value: 30, DisplayName: "最近30天"},
			},
		},
		{
			ID:          "template-portrait",
			Name:        "人像照片",
			Description: "包含人脸的照片",
			Icon:        "user",
			MatchMode:   "all",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "scene", Operator: "equals", Value: "portrait", DisplayName: "人像场景"},
			},
		},
		{
			ID:          "template-nature",
			Name:        "自然风光",
			Description: "自然风景照片",
			Icon:        "leaf",
			MatchMode:   "any",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "scene", Operator: "equals", Value: "nature", DisplayName: "自然"},
				{ID: "r2", Type: "scene", Operator: "equals", Value: "landscape", DisplayName: "风景"},
				{ID: "r3", Type: "object", Operator: "contains", Value: "vegetation", DisplayName: "植物"},
			},
		},
		{
			ID:          "template-night",
			Name:        "夜景照片",
			Description: "夜景和夜晚拍摄的照片",
			Icon:        "moon",
			MatchMode:   "all",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "scene", Operator: "equals", Value: "night", DisplayName: "夜景"},
			},
		},
		{
			ID:          "template-sunset",
			Name:        "日落黄昏",
			Description: "日落和黄昏时分拍摄的照片",
			Icon:        "sunset",
			MatchMode:   "any",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "scene", Operator: "equals", Value: "sunset", DisplayName: "日落"},
				{ID: "r2", Type: "object", Operator: "contains", Value: "sunset", DisplayName: "黄昏"},
			},
		},
		{
			ID:          "template-video",
			Name:        "视频",
			Description: "所有视频文件",
			Icon:        "video",
			MatchMode:   "all",
			IsBuiltin:   true,
			Rules: []SmartAlbumRule{
				{ID: "r1", Type: "media", Operator: "equals", Value: "video", DisplayName: "视频类型"},
			},
		},
	}
}

// createBuiltinTemplates 创建内置模板对应的相册
func (sam *SmartAlbumManager) createBuiltinTemplates() {
	// 不自动创建，由用户选择创建
}

// saveSmartAlbumsInternal 保存智能相册（内部方法，调用者需持有锁）
func (sam *SmartAlbumManager) saveSmartAlbumsInternal() error {
	albums := make([]SmartAlbumConfig, 0, len(sam.smartAlbums))
	for _, album := range sam.smartAlbums {
		albums = append(albums, *album)
	}

	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(sam.manager.dataDir, "smart-albums-config.json")
	return os.WriteFile(path, data, 0640)
}

// saveSmartAlbums 保存智能相册
func (sam *SmartAlbumManager) saveSmartAlbums() error {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	return sam.saveSmartAlbumsInternal()
}

// loadSmartAlbums 加载智能相册
func (sam *SmartAlbumManager) loadSmartAlbums() error {
	path := filepath.Join(sam.manager.dataDir, "smart-albums-config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var albums []SmartAlbumConfig
	if err := json.Unmarshal(data, &albums); err != nil {
		return err
	}

	sam.mu.Lock()
	defer sam.mu.Unlock()

	for i := range albums {
		sam.smartAlbums[albums[i].ID] = &albums[i]
	}

	return nil
}

// ==================== 照片去重功能 ====================

// DuplicateDetector 重复照片检测器
// 参考：群晖 Photos 相似照片检测功能
type DuplicateDetector struct {
	manager       *Manager
	hashCache     map[string]string // photoID -> hash
	featureCache  map[string][]float64 // photoID -> feature vector
	mu            sync.RWMutex
}

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // "exact", "similar", "burst"
	Photos       []*Photo  `json:"photos"`
	KeepPhotoID  string    `json:"keepPhotoId"` // 推荐保留的照片
	Similarity   float64   `json:"similarity"`  // 相似度 0-1
	CreatedAt    time.Time `json:"createdAt"`
}

// DuplicateReport 重复检测报告
type DuplicateReport struct {
	TotalPhotos     int               `json:"totalPhotos"`
	ExactDuplicates []*DuplicateGroup `json:"exactDuplicates"`
	SimilarPhotos   []*DuplicateGroup `json:"similarPhotos"`
	BurstGroups     []*DuplicateGroup `json:"burstGroups"`
	SpaceSavings    uint64            `json:"spaceSavings"` // 可节省空间
	GeneratedAt     time.Time         `json:"generatedAt"`
}

// NewDuplicateDetector 创建重复检测器
func NewDuplicateDetector(manager *Manager) *DuplicateDetector {
	return &DuplicateDetector{
		manager:      manager,
		hashCache:    make(map[string]string),
		featureCache: make(map[string][]float64),
	}
}

// DetectDuplicates 检测重复照片
func (dd *DuplicateDetector) DetectDuplicates() (*DuplicateReport, error) {
	dd.manager.mu.RLock()
	photos := make([]*Photo, 0, len(dd.manager.photos))
	for _, p := range dd.manager.photos {
		photos = append(photos, p)
	}
	dd.manager.mu.RUnlock()

	report := &DuplicateReport{
		TotalPhotos:     len(photos),
		ExactDuplicates: make([]*DuplicateGroup, 0),
		SimilarPhotos:   make([]*DuplicateGroup, 0),
		BurstGroups:     make([]*DuplicateGroup, 0),
		GeneratedAt:     time.Now(),
	}

	// 1. 检测完全重复（MD5 哈希相同）
	exactDups := dd.detectExactDuplicates(photos)
	report.ExactDuplicates = exactDups

	// 2. 检测相似照片（特征向量相似）
	similarDups := dd.detectSimilarPhotos(photos)
	report.SimilarPhotos = similarDups

	// 3. 检测连拍照片
	burstGroups := dd.detectBurstPhotos(photos)
	report.BurstGroups = burstGroups

	// 计算可节省空间
	report.SpaceSavings = dd.calculateSpaceSavings(exactDups, similarDups, burstGroups)

	return report, nil
}

// detectExactDuplicates 检测完全重复的照片
func (dd *DuplicateDetector) detectExactDuplicates(photos []*Photo) []*DuplicateGroup {
	hashGroups := make(map[string][]*Photo)

	for _, photo := range photos {
		hash := dd.calculateFileHash(photo)
		if hash != "" {
			hashGroups[hash] = append(hashGroups[hash], photo)
			dd.mu.Lock()
			dd.hashCache[photo.ID] = hash
			dd.mu.Unlock()
		}
	}

	groups := make([]*DuplicateGroup, 0)
	for _, photoList := range hashGroups {
		if len(photoList) > 1 {
			group := &DuplicateGroup{
				ID:         uuid.New().String(),
				Type:       "exact",
				Photos:     photoList,
				Similarity: 1.0,
				CreatedAt:  time.Now(),
			}

			// 推荐保留分辨率最高或最新的照片
			group.KeepPhotoID = dd.selectBestPhoto(photoList).ID

			groups = append(groups, group)
		}
	}

	return groups
}

// detectSimilarPhotos 检测相似照片
func (dd *DuplicateDetector) detectSimilarPhotos(photos []*Photo) []*DuplicateGroup {
	// 提取特征向量
	features := make(map[string][]float64)
	for _, photo := range photos {
		feature := dd.extractPhotoFeatures(photo)
		if len(feature) > 0 {
			features[photo.ID] = feature
			dd.mu.Lock()
			dd.featureCache[photo.ID] = feature
			dd.mu.Unlock()
		}
	}

	// 查找相似组
	groups := make([]*DuplicateGroup, 0)
	processed := make(map[string]bool)

	for id1, f1 := range features {
		if processed[id1] {
			continue
		}

		similar := make([]*Photo, 0)
		similar = append(similar, dd.manager.photos[id1])

		for id2, f2 := range features {
			if id1 == id2 || processed[id2] {
				continue
			}

			similarity := dd.calculateFeatureSimilarity(f1, f2)
			if similarity >= 0.85 { // 85% 相似度阈值
				similar = append(similar, dd.manager.photos[id2])
				processed[id2] = true
			}
		}

		if len(similar) > 1 {
			group := &DuplicateGroup{
				ID:         uuid.New().String(),
				Type:       "similar",
				Photos:     similar,
				Similarity: dd.calculateFeatureSimilarity(f1, features[similar[1].ID]),
				CreatedAt:  time.Now(),
			}
			group.KeepPhotoID = dd.selectBestPhoto(similar).ID
			groups = append(groups, group)
		}

		processed[id1] = true
	}

	return groups
}

// detectBurstPhotos 检测连拍照片
// 参考：群晖 Photos 连拍分组功能
func (dd *DuplicateDetector) detectBurstPhotos(photos []*Photo) []*DuplicateGroup {
	// 按时间排序
	sort.Slice(photos, func(i, j int) bool {
		return photos[i].TakenAt.Before(photos[j].TakenAt)
	})

	groups := make([]*DuplicateGroup, 0)
	currentGroup := make([]*Photo, 0)
	var lastTime time.Time

	for _, photo := range photos {
		if photo.TakenAt.IsZero() {
			continue
		}

		// 如果与上一张照片时间间隔小于 3 秒，认为是连拍
		if len(currentGroup) == 0 || photo.TakenAt.Sub(lastTime) < 3*time.Second {
			currentGroup = append(currentGroup, photo)
		} else {
			// 当前组结束，检查是否为有效连拍组
			if len(currentGroup) >= 3 {
				group := &DuplicateGroup{
					ID:         uuid.New().String(),
					Type:       "burst",
					Photos:     currentGroup,
					Similarity: 0.7, // 连拍照片相似度通常较高
					CreatedAt:  time.Now(),
				}
				group.KeepPhotoID = dd.selectBestPhoto(currentGroup).ID
				groups = append(groups, group)
			}
			currentGroup = []*Photo{photo}
		}

		lastTime = photo.TakenAt
	}

	// 处理最后一组
	if len(currentGroup) >= 3 {
		group := &DuplicateGroup{
			ID:         uuid.New().String(),
			Type:       "burst",
			Photos:     currentGroup,
			Similarity: 0.7,
			CreatedAt:  time.Now(),
		}
		group.KeepPhotoID = dd.selectBestPhoto(currentGroup).ID
		groups = append(groups, group)
	}

	return groups
}

// calculateFileHash 计算文件哈希
func (dd *DuplicateDetector) calculateFileHash(photo *Photo) string {
	photoPath := filepath.Join(dd.manager.photosDir, photo.Path)

	data, err := os.ReadFile(photoPath)
	if err != nil {
		return ""
	}

	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// extractPhotoFeatures 提取照片特征向量
// 使用简单的颜色直方图作为特征
func (dd *DuplicateDetector) extractPhotoFeatures(photo *Photo) []float64 {
	photoPath := filepath.Join(dd.manager.photosDir, photo.Path)

	f, err := os.Open(photoPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}

	bounds := img.Bounds()

	// 计算颜色直方图
	histogram := make([]float64, 64) // 4x4x4 直方图

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			// 量化到 4 个级别
			ri := (r >> 8) / 64
			gi := (g >> 8) / 64
			bi := (b >> 8) / 64
			if ri > 3 {
				ri = 3
			}
			if gi > 3 {
				gi = 3
			}
			if bi > 3 {
				bi = 3
			}
			idx := ri*16 + gi*4 + bi
			histogram[idx]++
		}
	}

	// 归一化
	total := float64(0)
	for _, v := range histogram {
		total += v
	}
	if total > 0 {
		for i := range histogram {
			histogram[i] /= total
		}
	}

	return histogram
}

// calculateFeatureSimilarity 计算特征相似度
func (dd *DuplicateDetector) calculateFeatureSimilarity(f1, f2 []float64) float64 {
	if len(f1) != len(f2) {
		return 0
	}

	// 使用余弦相似度
	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for i := range f1 {
		dotProduct += f1[i] * f2[i]
		norm1 += f1[i] * f1[i]
		norm2 += f2[i] * f2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// selectBestPhoto 选择最佳照片（推荐保留）
func (dd *DuplicateDetector) selectBestPhoto(photos []*Photo) *Photo {
	if len(photos) == 0 {
		return nil
	}

	best := photos[0]
	bestScore := dd.scorePhoto(best)

	for _, p := range photos[1:] {
		score := dd.scorePhoto(p)
		if score > bestScore {
			best = p
			bestScore = score
		}
	}

	return best
}

// scorePhoto 为照片评分
func (dd *DuplicateDetector) scorePhoto(photo *Photo) float64 {
	score := 0.0

	// 分辨率越高越好
	resolution := photo.Width * photo.Height
	score += float64(resolution) / 1000000 // 百万像素

	// 文件越大越好（通常质量更好）
	score += float64(photo.Size) / 1000000 // MB

	// 收藏的照片优先
	if photo.IsFavorite {
		score += 10
	}

	// 有描述的照片优先
	if photo.EXIF != nil && photo.EXIF.Artist != "" {
		score += 5
	}

	return score
}

// calculateSpaceSavings 计算可节省空间
func (dd *DuplicateDetector) calculateSpaceSavings(exactDups, similarDups, burstGroups []*DuplicateGroup) uint64 {
	var savings uint64

	for _, group := range exactDups {
		for _, photo := range group.Photos {
			if photo.ID != group.KeepPhotoID {
				savings += photo.Size
			}
		}
	}

	for _, group := range similarDups {
		for _, photo := range group.Photos {
			if photo.ID != group.KeepPhotoID {
				savings += photo.Size
			}
		}
	}

	// 连拍组不自动删除，不计入节省空间

	return savings
}

// RemoveDuplicates 删除重复照片
func (dd *DuplicateDetector) RemoveDuplicates(groupIDs []string, keepOriginals bool) (int, error) {
	removed := 0

	// 获取重复报告
	report, err := dd.DetectDuplicates()
	if err != nil {
		return 0, err
	}

	// 创建 ID 到组的映射
	groupMap := make(map[string]*DuplicateGroup)
	for _, g := range report.ExactDuplicates {
		groupMap[g.ID] = g
	}
	for _, g := range report.SimilarPhotos {
		groupMap[g.ID] = g
	}

	for _, groupID := range groupIDs {
		group, exists := groupMap[groupID]
		if !exists {
			continue
		}

		for _, photo := range group.Photos {
			if photo.ID == group.KeepPhotoID {
				continue // 保留推荐的照片
			}

			if err := dd.manager.DeletePhoto(photo.ID); err != nil {
				continue
			}
			removed++
		}
	}

	return removed, nil
}

// GetDuplicatePhotos 获取指定照片的重复照片
func (dd *DuplicateDetector) GetDuplicatePhotos(photoID string) ([]*Photo, float64, error) {
	dd.manager.mu.RLock()
	photo, exists := dd.manager.photos[photoID]
	dd.manager.mu.RUnlock()

	if !exists {
		return nil, 0, fmt.Errorf("照片不存在")
	}

	// 获取哈希
	hash := dd.hashCache[photoID]
	if hash == "" {
		hash = dd.calculateFileHash(photo)
	}

	// 获取特征
	feature := dd.featureCache[photoID]
	if len(feature) == 0 {
		feature = dd.extractPhotoFeatures(photo)
	}

	duplicates := make([]*Photo, 0)
	var maxSimilarity float64

	dd.manager.mu.RLock()
	defer dd.manager.mu.RUnlock()

	for _, p := range dd.manager.photos {
		if p.ID == photoID {
			continue
		}

		// 检查哈希匹配
		pHash := dd.hashCache[p.ID]
		if pHash == "" {
			pHash = dd.calculateFileHash(p)
		}

		if hash != "" && pHash == hash {
			duplicates = append(duplicates, p)
			maxSimilarity = 1.0
			continue
		}

		// 检查特征相似度
		pFeature := dd.featureCache[p.ID]
		if len(pFeature) == 0 {
			pFeature = dd.extractPhotoFeatures(p)
		}

		if len(feature) > 0 && len(pFeature) > 0 {
			similarity := dd.calculateFeatureSimilarity(feature, pFeature)
			if similarity >= 0.85 {
				duplicates = append(duplicates, p)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
		}
	}

	return duplicates, maxSimilarity, nil
}

// ==================== 辅助函数 ====================

// roundTo 将浮点数精确到指定小数位
func roundTo(val float64, precision int) float64 {
	multiplier := math.Pow(10, float64(precision))
	return math.Round(val*multiplier) / multiplier
}

// ColorToHex 将颜色转换为十六进制字符串
func ColorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}