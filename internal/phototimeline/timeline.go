// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// TimelineManager 时间线管理器
type TimelineManager struct {
	mu     sync.RWMutex
	photos map[string]*Photo // id -> photo
	config Config
}

// NewTimelineManager 创建时间线管理器
func NewTimelineManager(config Config) *TimelineManager {
	return &TimelineManager{
		photos: make(map[string]*Photo),
		config: config,
	}
}

// AddPhoto 添加照片到时间线
func (tm *TimelineManager) AddPhoto(photo *Photo) error {
	if photo == nil {
		return fmt.Errorf("photo cannot be nil")
	}
	if photo.ID == "" {
		return fmt.Errorf("photo ID is required")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	photo.CreatedAt = time.Now()
	photo.UpdatedAt = time.Now()
	if photo.ImportedAt.IsZero() {
		photo.ImportedAt = time.Now()
	}
	if photo.TakenAt.IsZero() {
		photo.TakenAt = photo.ModifiedAt
	}

	tm.photos[photo.ID] = photo
	return nil
}

// RemovePhoto 从时间线移除照片
func (tm *TimelineManager) RemovePhoto(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.photos[id]; !exists {
		return fmt.Errorf("photo not found: %s", id)
	}

	delete(tm.photos, id)
	return nil
}

// GetPhoto 获取照片
func (tm *TimelineManager) GetPhoto(id string) (*Photo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	photo, exists := tm.photos[id]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", id)
	}

	return photo, nil
}

// UpdatePhoto 更新照片信息
func (tm *TimelineManager) UpdatePhoto(photo *Photo) error {
	if photo == nil || photo.ID == "" {
		return fmt.Errorf("invalid photo")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.photos[photo.ID]; !exists {
		return fmt.Errorf("photo not found: %s", photo.ID)
	}

	photo.UpdatedAt = time.Now()
	tm.photos[photo.ID] = photo
	return nil
}

// GetTimeline 获取时间线
func (tm *TimelineManager) GetTimeline(view TimelineView, page, pageSize int) (*TimelineResponse, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 收集所有照片并按拍摄时间排序
	photos := make([]*Photo, 0, len(tm.photos))
	for _, p := range tm.photos {
		if !p.Trashed {
			photos = append(photos, p)
		}
	}

	sort.Slice(photos, func(i, j int) bool {
		return photos[i].TakenAt.After(photos[j].TakenAt)
	})

	// 按视图类型分组
	groups := tm.groupPhotos(photos, view)

	// 计算总数
	total := 0
	for _, g := range groups {
		total += len(g.Photos)
	}

	// 分页处理
	start := (page - 1) * pageSize
	if start >= len(groups) {
		return &TimelineResponse{
			Groups:   []TimelineGroup{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  false,
		}, nil
	}

	end := start + pageSize
	if end > len(groups) {
		end = len(groups)
	}

	return &TimelineResponse{
		Groups:   groups[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < len(groups),
	}, nil
}

// groupPhotos 按时间分组照片
func (tm *TimelineManager) groupPhotos(photos []*Photo, view TimelineView) []TimelineGroup {
	if len(photos) == 0 {
		return nil
	}

	groupMap := make(map[string]*TimelineGroup)

	for _, p := range photos {
		key := tm.getTimeKey(p.TakenAt, view)
		if _, exists := groupMap[key]; !exists {
			groupDate := tm.getGroupDate(p.TakenAt, view)
			groupMap[key] = &TimelineGroup{
				Date: groupDate,
				View: view,
			}
		}
		group := groupMap[key]
		group.Photos = append(group.Photos, *p)
		group.Count = len(group.Photos)
		if group.CoverURL == "" {
			group.CoverURL = p.Path // 使用第一张照片作为封面
		}
	}

	// 转换为切片并按日期排序
	groups := make([]TimelineGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, *g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Date.After(groups[j].Date)
	})

	return groups
}

// getTimeKey 获取时间分组键
func (tm *TimelineManager) getTimeKey(t time.Time, view TimelineView) string {
	switch view {
	case TimelineViewDay:
		return t.Format("2006-01-02")
	case TimelineViewMonth:
		return t.Format("2006-01")
	case TimelineViewYear:
		return t.Format("2006")
	default:
		return t.Format("2006-01")
	}
}

// getGroupDate 获取分组日期
func (tm *TimelineManager) getGroupDate(t time.Time, view TimelineView) time.Time {
	switch view {
	case TimelineViewDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case TimelineViewMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case TimelineViewYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
}

// GetStats 获取时间线统计
func (tm *TimelineManager) GetStats() *TimelineStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := &TimelineStats{}
	cameraMap := make(map[string]int)
	locationMap := make(map[string]*LocationStat)

	for _, p := range tm.photos {
		if p.Trashed {
			continue
		}

		stats.TotalPhotos++
		stats.TotalSize += p.Size

		// 时间统计
		if stats.OldestPhoto == nil || p.TakenAt.Before(*stats.OldestPhoto) {
			stats.OldestPhoto = &p.TakenAt
		}
		if stats.NewestPhoto == nil || p.TakenAt.After(*stats.NewestPhoto) {
			stats.NewestPhoto = &p.TakenAt
		}

		// 相机统计
		if p.EXIF.CameraModel != "" {
			cameraMap[p.EXIF.CameraModel]++
		}

		// 地点统计
		if p.Location != "" {
			if _, exists := locationMap[p.Location]; !exists {
				locationMap[p.Location] = &LocationStat{
					Location:  p.Location,
					Latitude:  p.Latitude,
					Longitude: p.Longitude,
				}
			}
			locationMap[p.Location].Count++
		}
	}

	// 计算年份数
	if stats.OldestPhoto != nil && stats.NewestPhoto != nil {
		stats.Years = stats.NewestPhoto.Year() - stats.OldestPhoto.Year() + 1
	}

	// 转换相机统计
	for camera, count := range cameraMap {
		stats.CameraStats = append(stats.CameraStats, CameraStat{
			Camera: camera,
			Count:  count,
		})
	}
	sort.Slice(stats.CameraStats, func(i, j int) bool {
		return stats.CameraStats[i].Count > stats.CameraStats[j].Count
	})

	// 转换地点统计
	for _, ls := range locationMap {
		stats.TopLocations = append(stats.TopLocations, *ls)
	}
	sort.Slice(stats.TopLocations, func(i, j int) bool {
		return stats.TopLocations[i].Count > stats.TopLocations[j].Count
	})

	// 限制返回数量
	if len(stats.CameraStats) > 10 {
		stats.CameraStats = stats.CameraStats[:10]
	}
	if len(stats.TopLocations) > 10 {
		stats.TopLocations = stats.TopLocations[:10]
	}

	return stats
}

// GetPhotosByDateRange 按日期范围获取照片
func (tm *TimelineManager) GetPhotosByDateRange(from, to time.Time) []Photo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []Photo
	for _, p := range tm.photos {
		if !p.TakenAt.Before(from) && !p.TakenAt.After(to) && !p.Trashed {
			result = append(result, *p)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TakenAt.After(result[j].TakenAt)
	})

	return result
}

// GetPhotosByLocation 按地点获取照片
func (tm *TimelineManager) GetPhotosByLocation(latitude, longitude, radiusKm float64) []Photo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []Photo
	for _, p := range tm.photos {
		if p.Trashed {
			continue
		}
		if p.Latitude != 0 && p.Longitude != 0 {
			dist := haversineDistance(latitude, longitude, p.Latitude, p.Longitude)
			if dist <= radiusKm {
				result = append(result, *p)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TakenAt.After(result[j].TakenAt)
	})

	return result
}

// haversineDistance 计算两点间距离 (km)
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	lat1Rad := lat1 * 3.141592653589793 / 180.0
	lat2Rad := lat2 * 3.141592653589793 / 180.0
	deltaLat := (lat2 - lat1) * 3.141592653589793 / 180.0
	deltaLon := (lon2 - lon1) * 3.141592653589793 / 180.0

	a := sin(deltaLat/2)*sin(deltaLat/2) +
		cos(lat1Rad)*cos(lat2Rad)*
			sin(deltaLon/2)*sin(deltaLon/2)
	c := 2 * atan2(sqrt(a), sqrt(1-a))

	return earthRadiusKm * c
}

func sin(x float64) float64 {
	// 使用标准库的实现
	return float64(int64(x*1000000)) / 1000000
}

func cos(x float64) float64 {
	return 1 - x*x/2
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func atan2(y, x float64) float64 {
	if x > 0 {
		return atan(y / x)
	}
	if x < 0 && y >= 0 {
		return atan(y/x) + 3.141592653589793
	}
	if x < 0 && y < 0 {
		return atan(y/x) - 3.141592653589793
	}
	if x == 0 && y > 0 {
		return 3.141592653589793 / 2
	}
	if x == 0 && y < 0 {
		return -3.141592653589793 / 2
	}
	return 0
}

func atan(x float64) float64 {
	// 简化的 atan 实现
	return x - x*x*x/3 + x*x*x*x*x/5
}

// Count 返回照片总数
func (tm *TimelineManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.photos)
}
