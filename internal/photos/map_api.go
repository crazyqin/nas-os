// Package photos - 照片地图视图 API
// 实现 EXIF GPS 解析、地图聚合查询、时间轴聚合
package photos

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ========== GPS 解析和地图聚合 ==========

// GPSExtractor GPS 信息提取器
type GPSExtractor struct {
	manager *Manager
	cache   map[string]*GPSInfo
	mu      sync.RWMutex
}

// NewGPSExtractor 创建 GPS 提取器
func NewGPSExtractor(manager *Manager) *GPSExtractor {
	return &GPSExtractor{
		manager: manager,
		cache:   make(map[string]*GPSInfo),
	}
}

// ExtractGPSFromFile 从文件提取 GPS 信息
func (g *GPSExtractor) ExtractGPSFromFile(filePath string) (*GPSInfo, error) {
	// 检查缓存
	g.mu.RLock()
	if cached, ok := g.cache[filePath]; ok {
		g.mu.RUnlock()
		return cached, nil
	}
	g.mu.RUnlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 解析 EXIF
	x, err := exif.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("解析 EXIF 失败: %w", err)
	}

	// 提取 GPS 信息
	lat, lon, err := x.LatLong()
	if err != nil {
		return nil, fmt.Errorf("提取 GPS 失败: %w", err)
	}

	gpsInfo := &GPSInfo{
		Latitude:  lat,
		Longitude: lon,
	}

	// 提取海拔
	if alt, err := g.getGPSAltitude(x); err == nil {
		gpsInfo.Altitude = alt
	}

	// 缓存结果
	g.mu.Lock()
	g.cache[filePath] = gpsInfo
	g.mu.Unlock()

	return gpsInfo, nil
}

// getGPSAltitude 获取 GPS 海拔
func (g *GPSExtractor) getGPSAltitude(x *exif.Exif) (float64, error) {
	tag, err := x.Get(exif.GPSAltitude)
	if err != nil {
		return 0, err
	}

	num, denom, err := tag.Rat2(0)
	if err != nil {
		return 0, err
	}
	alt := float64(num) / float64(denom)

	// 检查海拔参考（海平面以上/以下）
	refTag, err := x.Get(exif.GPSAltitudeRef)
	if err == nil {
		ref, _ := refTag.StringVal()
		if ref == "1" || ref == "\x01" {
			alt = -alt
		}
	}

	return float64(alt), nil
}

// ExtractDateTime 提取拍摄时间
func (g *GPSExtractor) ExtractDateTime(filePath string) (time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return time.Time{}, err
	}

	// 尝试 DateTimeOriginal
	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		// 回退到 DateTime
		tag, err = x.Get(exif.DateTime)
		if err != nil {
			return time.Time{}, err
		}
	}

	timeStr, err := tag.StringVal()
	if err != nil {
		return time.Time{}, err
	}

	// EXIF 时间格式: "2006:01:02 15:04:05"
	t, err := time.Parse("2006:01:02 15:04:05", timeStr)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

// ExtractCameraModel 提取相机型号
func (g *GPSExtractor) ExtractCameraModel(filePath string) (*CameraInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return nil, err
	}

	camera := &CameraInfo{}

	// 提取制造商
	if makeTag, err := x.Get(exif.Make); err == nil {
		camera.Make, _ = makeTag.StringVal()
	}

	// 提取型号
	if modelTag, err := x.Get(exif.Model); err == nil {
		camera.Model, _ = modelTag.StringVal()
	}

	// 提取镜头
	if lensTag, err := x.Get(exif.LensModel); err == nil {
		camera.Lens, _ = lensTag.StringVal()
	}

	// 提取光圈
	if apertureTag, err := x.Get(exif.FNumber); err == nil {
		num, denom, _ := apertureTag.Rat2(0)
		camera.Aperture = fmt.Sprintf("f/%.1f", float64(num)/float64(denom))
	}

	// 提取快门速度
	if shutterTag, err := x.Get(exif.ExposureTime); err == nil {
		camera.ShutterSpeed = string(shutterTag.Val)
	}

	// 提取 ISO
	if isoTag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		camera.ISO, _ = isoTag.Int(0)
	}

	// 提取焦距
	if focalTag, err := x.Get(exif.FocalLength); err == nil {
		num, denom, _ := focalTag.Rat2(0)
		camera.FocalLength = fmt.Sprintf("%.0fmm", float64(num)/float64(denom))
	}

	return camera, nil
}

// GPSInfo GPS 信息
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
}

// CameraInfo 相机信息
type CameraInfo struct {
	Make         string `json:"make,omitempty"`
	Model        string `json:"model,omitempty"`
	Lens         string `json:"lens,omitempty"`
	Aperture     string `json:"aperture,omitempty"`
	ShutterSpeed string `json:"shutterSpeed,omitempty"`
	ISO          int    `json:"iso,omitempty"`
	FocalLength  string `json:"focalLength,omitempty"`
}

// ========== 地图聚合查询 ==========

// MapCluster 地图聚合簇
type MapCluster struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CenterLat  float64    `json:"centerLat"`
	CenterLng  float64    `json:"centerLng"`
	Radius     float64    `json:"radius"` // 米
	PhotoCount int        `json:"photoCount"`
	PhotoIDs   []string   `json:"photoIds"`
	Bounds     GeoBounds  `json:"bounds"`
	DateRange  *DateRange `json:"dateRange,omitempty"`
	PlaceName  string     `json:"placeName,omitempty"`
	Thumbnails []string   `json:"thumbnails,omitempty"`
}

// GeoBounds 地理边界
type GeoBounds struct {
	MinLat float64 `json:"minLat"`
	MaxLat float64 `json:"maxLat"`
	MinLng float64 `json:"minLng"`
	MaxLng float64 `json:"maxLng"`
}

// MapAggregator 地图聚合器
type MapAggregator struct {
	manager    *Manager
	gps        *GPSExtractor
	geocoder   *GeocoderService
	clusterMap map[string]*MapCluster // 网格索引
	mu         sync.RWMutex
}

// NewMapAggregator 创建地图聚合器
func NewMapAggregator(manager *Manager, geocoder *GeocoderService) *MapAggregator {
	return &MapAggregator{
		manager:    manager,
		gps:        NewGPSExtractor(manager),
		geocoder:   geocoder,
		clusterMap: make(map[string]*MapCluster),
	}
}

// AggregateByRegion 按区域聚合照片
func (m *MapAggregator) AggregateByRegion(ctx context.Context, zoomLevel int, bounds GeoBounds) ([]*MapCluster, error) {
	m.manager.mu.RLock()
	photos := make([]*Photo, 0, len(m.manager.photos))
	for _, p := range m.manager.photos {
		photos = append(photos, p)
	}
	m.manager.mu.RUnlock()

	// 根据缩放级别计算网格大小
	gridSize := m.calculateGridSize(zoomLevel)

	// 按网格分组
	gridMap := make(map[string][]*Photo)

	for _, photo := range photos {
		if photo.Location == nil {
			continue
		}

		lat, lng := photo.Location.Latitude, photo.Location.Longitude
		if lat < bounds.MinLat || lat > bounds.MaxLat || lng < bounds.MinLng || lng > bounds.MaxLng {
			continue
		}

		// 计算网格坐标
		gridKey := m.getGridKey(lat, lng, gridSize)
		gridMap[gridKey] = append(gridMap[gridKey], photo)
	}

	// 生成聚合结果
	clusters := make([]*MapCluster, 0, len(gridMap))

	for gridKey, gridPhotos := range gridMap {
		if len(gridPhotos) == 0 {
			continue
		}

		cluster := &MapCluster{
			ID:         gridKey,
			PhotoCount: len(gridPhotos),
			PhotoIDs:   make([]string, 0, len(gridPhotos)),
		}

		// 计算中心点和边界
		var sumLat, sumLng float64
		var minLat, maxLat, minLng, maxLng float64
		minLat = 90
		maxLat = -90
		minLng = 180
		maxLng = -180
		var minTime, maxTime time.Time

		for _, p := range gridPhotos {
			cluster.PhotoIDs = append(cluster.PhotoIDs, p.ID)

			lat := p.Location.Latitude
			lng := p.Location.Longitude

			sumLat += lat
			sumLng += lng

			if lat < minLat {
				minLat = lat
			}
			if lat > maxLat {
				maxLat = lat
			}
			if lng < minLng {
				minLng = lng
			}
			if lng > maxLng {
				maxLng = lng
			}

			// 更新时间范围
			if minTime.IsZero() || p.TakenAt.Before(minTime) {
				minTime = p.TakenAt
			}
			if maxTime.IsZero() || p.TakenAt.After(maxTime) {
				maxTime = p.TakenAt
			}
		}

		cluster.CenterLat = sumLat / float64(len(gridPhotos))
		cluster.CenterLng = sumLng / float64(len(gridPhotos))
		cluster.Bounds = GeoBounds{
			MinLat: minLat,
			MaxLat: maxLat,
			MinLng: minLng,
			MaxLng: maxLng,
		}

		// 计算半径
		cluster.Radius = m.calculateClusterRadius(cluster.CenterLat, cluster.CenterLng, cluster.Bounds)

		// 设置时间范围
		if !minTime.IsZero() && !maxTime.IsZero() {
			cluster.DateRange = &DateRange{Start: minTime, End: maxTime}
		}

		// 获取地名
		if m.geocoder != nil {
			if placeName, err := m.geocoder.ReverseGeocode(ctx, cluster.CenterLat, cluster.CenterLng); err == nil {
				cluster.PlaceName = placeName
				cluster.Name = placeName
			}
		}

		// 设置缩略图（最多4张）
		for i := 0; i < 4 && i < len(gridPhotos); i++ {
			if gridPhotos[i].ThumbnailPath != "" {
				cluster.Thumbnails = append(cluster.Thumbnails, gridPhotos[i].ThumbnailPath)
			}
		}

		clusters = append(clusters, cluster)
	}

	// 按照片数量排序
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].PhotoCount > clusters[j].PhotoCount
	})

	return clusters, nil
}

// calculateGridSize 根据缩放级别计算网格大小（度）
func (m *MapAggregator) calculateGridSize(zoomLevel int) float64 {
	// 缩放级别越高，网格越小
	// 典型范围：zoom 1-20
	gridSizes := map[int]float64{
		1:  45.0,
		2:  22.5,
		3:  11.25,
		4:  5.625,
		5:  2.8125,
		6:  1.40625,
		7:  0.703125,
		8:  0.3515625,
		9:  0.17578125,
		10: 0.087890625,
		11: 0.0439453125,
		12: 0.02197265625,
		13: 0.010986328125,
		14: 0.0054931640625,
		15: 0.00274658203125,
		16: 0.001373291015625,
		17: 0.0006866455078125,
		18: 0.00034332275390625,
		19: 0.000171661376953125,
		20: 0.0000858306884765625,
	}

	if size, ok := gridSizes[zoomLevel]; ok {
		return size
	}
	return 0.01 // 默认
}

// getGridKey 获取网格键
func (m *MapAggregator) getGridKey(lat, lng, gridSize float64) string {
	latIdx := int(math.Floor(lat / gridSize))
	lngIdx := int(math.Floor(lng / gridSize))
	return fmt.Sprintf("%d_%d", latIdx, lngIdx)
}

// calculateClusterRadius 计算聚合半径
func (m *MapAggregator) calculateClusterRadius(centerLat, centerLng float64, bounds GeoBounds) float64 {
	// 使用 Haversine 公式计算最大距离
	d1 := haversineDistance(centerLat, centerLng, bounds.MinLat, bounds.MinLng)
	d2 := haversineDistance(centerLat, centerLng, bounds.MaxLat, bounds.MaxLng)
	d3 := haversineDistance(centerLat, centerLng, bounds.MinLat, bounds.MaxLng)
	d4 := haversineDistance(centerLat, centerLng, bounds.MaxLat, bounds.MinLng)

	maxDist := math.Max(math.Max(d1, d2), math.Max(d3, d4))
	return maxDist
}

// haversineDistance Haversine 公式计算距离（米）
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000 // 地球半径（米）

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// ========== 时间轴聚合优化 ==========

// TimelineAggregator 时间轴聚合器
type TimelineAggregator struct {
	manager *Manager
}

// NewTimelineAggregator 创建时间轴聚合器
func NewTimelineAggregator(manager *Manager) *TimelineAggregator {
	return &TimelineAggregator{manager: manager}
}

// AggregateByTime 按时间聚合照片
func (t *TimelineAggregator) AggregateByTime(groupBy string, limit, offset int) ([]*TimelineGroup, int64, error) {
	t.manager.mu.RLock()
	defer t.manager.mu.RUnlock()

	// 按时间分组
	groupMap := make(map[string]*TimelineGroup)

	for _, photo := range t.manager.photos {
		if photo.TakenAt.IsZero() {
			continue
		}

		var key string
		var displayTime string

		switch groupBy {
		case "year":
			key = photo.TakenAt.Format("2006")
			displayTime = photo.TakenAt.Format("2006年")
		case "month":
			key = photo.TakenAt.Format("2006-01")
			displayTime = photo.TakenAt.Format("2006年1月")
		case "day":
			key = photo.TakenAt.Format("2006-01-02")
			displayTime = photo.TakenAt.Format("2006年1月2日")
		case "week":
			year, week := photo.TakenAt.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)
			displayTime = fmt.Sprintf("%d年 第%d周", year, week)
		default:
			key = photo.TakenAt.Format("2006-01")
			displayTime = photo.TakenAt.Format("2006年1月")
		}

		group, exists := groupMap[key]
		if !exists {
			group = &TimelineGroup{
				Key:         key,
				DisplayTime: displayTime,
				StartTime:   photo.TakenAt,
				EndTime:     photo.TakenAt,
				Photos:      make([]*Photo, 0),
			}
			groupMap[key] = group
		}

		group.Photos = append(group.Photos, photo)
		group.PhotoCount++

		// 更新时间范围
		if photo.TakenAt.Before(group.StartTime) {
			group.StartTime = photo.TakenAt
		}
		if photo.TakenAt.After(group.EndTime) {
			group.EndTime = photo.TakenAt
		}

		// 设置封面（第一张）
		if group.CoverPhoto == "" {
			group.CoverPhoto = photo.ID
		}
	}

	// 转换为切片并排序
	groups := make([]*TimelineGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, g)
	}

	// 按时间降序排序
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].StartTime.After(groups[j].StartTime)
	})

	total := int64(len(groups))

	// 分页
	if offset > 0 && offset < len(groups) {
		groups = groups[offset:]
	}
	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}

	return groups, total, nil
}

// AggregateByLocationAndTime 按位置和时间混合聚合
func (t *TimelineAggregator) AggregateByLocationAndTime(days int) ([]*TimelineGroup, error) {
	t.manager.mu.RLock()
	defer t.manager.mu.RUnlock()

	// 先按日期分组，再按位置子分组
	dateGroups := make(map[string][]*Photo)

	for _, photo := range t.manager.photos {
		if photo.TakenAt.IsZero() {
			continue
		}

		dateKey := photo.TakenAt.Format("2006-01-02")
		dateGroups[dateKey] = append(dateGroups[dateKey], photo)
	}

	// 按时间排序
	dates := make([]string, 0, len(dateGroups))
	for date := range dateGroups {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	result := make([]*TimelineGroup, 0)

	for _, date := range dates {
		photos := dateGroups[date]

		// 按位置子分组
		locationGroups := make(map[string][]*Photo)

		for _, p := range photos {
			locKey := "unknown"
			if p.Location != nil {
				locKey = p.Location.City
				if locKey == "" {
					locKey = p.Location.Country
				}
			}
			locationGroups[locKey] = append(locationGroups[locKey], p)
		}

		// 创建分组
		for loc, locPhotos := range locationGroups {
			takenAt, _ := time.Parse("2006-01-02", date)

			group := &TimelineGroup{
				Key:         date + "_" + loc,
				DisplayTime: date + " - " + loc,
				PhotoCount:  len(locPhotos),
				StartTime:   takenAt,
				EndTime:     takenAt.Add(24 * time.Hour),
				Location:    loc,
			}

			if len(locPhotos) > 0 {
				group.CoverPhoto = locPhotos[0].ID
			}

			result = append(result, group)
		}
	}

	// 限制数量
	if days > 0 && len(result) > days*3 {
		result = result[:days*3]
	}

	return result, nil
}

// GetTimeDistribution 获取时间分布统计
func (t *TimelineAggregator) GetTimeDistribution() *TimeDistribution {
	t.manager.mu.RLock()
	defer t.manager.mu.RUnlock()

	dist := &TimeDistribution{
		ByYear:  make(map[int]int),
		ByMonth: make(map[string]int),
		ByHour:  make(map[int]int),
		ByWeekday: map[string]int{
			"周一": 0, "周二": 0, "周三": 0, "周四": 0,
			"周五": 0, "周六": 0, "周日": 0,
		},
	}

	weekdays := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

	for _, photo := range t.manager.photos {
		if photo.TakenAt.IsZero() {
			continue
		}

		// 按年统计
		dist.ByYear[photo.TakenAt.Year()]++

		// 按月统计
		monthKey := photo.TakenAt.Format("2006-01")
		dist.ByMonth[monthKey]++

		// 按小时统计
		dist.ByHour[photo.TakenAt.Hour()]++

		// 按星期统计
		weekday := weekdays[photo.TakenAt.Weekday()]
		dist.ByWeekday[weekday]++
	}

	return dist
}

// TimeDistribution 时间分布统计
type TimeDistribution struct {
	ByYear    map[int]int    `json:"byYear"`
	ByMonth   map[string]int `json:"byMonth"`
	ByHour    map[int]int    `json:"byHour"`
	ByWeekday map[string]int `json:"byWeekday"`
}

// ========== 地理编码服务 ==========

// GeocoderService 地理编码服务
type GeocoderService struct {
	provider  string
	apiKey    string
	cache     map[string]string
	cacheFile string
	mu        sync.RWMutex
}

// NewGeocoderService 创建地理编码服务
func NewGeocoderService(provider, apiKey, cacheFile string) *GeocoderService {
	g := &GeocoderService{
		provider:  provider,
		apiKey:    apiKey,
		cache:     make(map[string]string),
		cacheFile: cacheFile,
	}

	// 加载缓存
	g.loadCache()

	return g
}

// ReverseGeocode 逆地理编码
func (g *GeocoderService) ReverseGeocode(ctx context.Context, lat, lng float64) (string, error) {
	// 生成缓存键（保留4位小数）
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lng)

	g.mu.RLock()
	if name, ok := g.cache[cacheKey]; ok {
		g.mu.RUnlock()
		return name, nil
	}
	g.mu.RUnlock()

	// 根据提供商调用不同API
	var name string
	var err error

	switch g.provider {
	case "nominatim":
		name, err = g.nominatimReverse(ctx, lat, lng)
	case "baidu":
		name, err = g.baiduReverse(ctx, lat, lng)
	default:
		name, err = g.nominatimReverse(ctx, lat, lng)
	}

	if err != nil {
		return "", err
	}

	// 缓存结果
	g.mu.Lock()
	g.cache[cacheKey] = name
	g.mu.Unlock()

	// 异步保存缓存
	go g.saveCache()

	return name, nil
}

// nominatimReverse 使用 Nominatim 逆地理编码
func (g *GeocoderService) nominatimReverse(ctx context.Context, lat, lng float64) (string, error) {
	// 使用 curl 调用 Nominatim API
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f&zoom=14", lat, lng)

	cmd := exec.CommandContext(ctx, "curl", "-s", "-H", "User-Agent: NAS-OS-PhotoAlbum/1.0", url)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var result struct {
		Address struct {
			City    string `json:"city"`
			Town    string `json:"town"`
			Village string `json:"village"`
			County  string `json:"county"`
			Country string `json:"country"`
		} `json:"address"`
		DisplayName string `json:"display_name"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	// 优先返回城市/城镇
	if result.Address.City != "" {
		return result.Address.City, nil
	}
	if result.Address.Town != "" {
		return result.Address.Town, nil
	}
	if result.Address.Village != "" {
		return result.Address.Village, nil
	}
	if result.Address.County != "" {
		return result.Address.County, nil
	}

	return result.Address.Country, nil
}

// baiduReverse 使用百度地图逆地理编码
func (g *GeocoderService) baiduReverse(ctx context.Context, lat, lng float64) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("百度地图 API Key 未配置")
	}

	url := fmt.Sprintf("https://api.map.baidu.com/reverse_geocoding/v3/?output=json&coordtype=wgs84ll&location=%f,%f&ak=%s",
		lat, lng, g.apiKey)

	cmd := exec.CommandContext(ctx, "curl", "-s", url)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var result struct {
		Status int `json:"status"`
		Result struct {
			AddressComponent struct {
				City     string `json:"city"`
				District string `json:"district"`
				Province string `json:"province"`
			} `json:"addressComponent"`
		} `json:"result"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	if result.Status != 0 {
		return "", fmt.Errorf("百度地图 API 返回错误: %d", result.Status)
	}

	if result.Result.AddressComponent.City != "" {
		return result.Result.AddressComponent.City, nil
	}
	if result.Result.AddressComponent.District != "" {
		return result.Result.AddressComponent.District, nil
	}

	return result.Result.AddressComponent.Province, nil
}

// loadCache 加载缓存
func (g *GeocoderService) loadCache() error {
	if g.cacheFile == "" {
		return nil
	}

	data, err := os.ReadFile(g.cacheFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &g.cache)
}

// saveCache 保存缓存
func (g *GeocoderService) saveCache() error {
	if g.cacheFile == "" {
		return nil
	}

	data, err := json.MarshalIndent(g.cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(g.cacheFile, data, 0640)
}

// ========== GPS 坐标转换工具 ==========

// DMSToDecimal 度分秒转十进制
func DMSToDecimal(degrees, minutes, seconds float64, ref string) float64 {
	decimal := degrees + minutes/60 + seconds/3600
	if ref == "S" || ref == "W" {
		decimal = -decimal
	}
	return decimal
}

// ParseGPSCoordinate 解析 GPS 坐标（支持多种格式）
func ParseGPSCoordinate(coord string) (float64, error) {
	// 尝试解析十进制格式
	if dec, err := strconv.ParseFloat(coord, 64); err == nil {
		return dec, nil
	}

	// 尝试解析度分秒格式
	// 格式: "45°30'15.6\"N" 或 "45 30 15.6 N"
	parts := strings.FieldsFunc(coord, func(r rune) bool {
		return r == '°' || r == '\'' || r == '"' || r == ' '
	})

	if len(parts) >= 3 {
		deg, _ := strconv.ParseFloat(parts[0], 64)
		min, _ := strconv.ParseFloat(parts[1], 64)
		sec, _ := strconv.ParseFloat(parts[2], 64)

		decimal := deg + min/60 + sec/3600

		// 检查方向
		if len(parts) >= 4 {
			if parts[3] == "S" || parts[3] == "W" {
				decimal = -decimal
			}
		}

		return decimal, nil
	}

	return 0, fmt.Errorf("无法解析坐标: %s", coord)
}

// BatchExtractGPS 批量提取 GPS 信息
func (g *GPSExtractor) BatchExtractGPS(photoPaths []string, concurrency int) map[string]*GPSInfo {
	results := make(map[string]*GPSInfo)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 限制并发数
	sem := make(chan struct{}, concurrency)

	for _, path := range photoPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			gps, err := g.ExtractGPSFromFile(p)
			if err == nil {
				mu.Lock()
				results[p] = gps
				mu.Unlock()
			}
		}(path)
	}

	wg.Wait()
	return results
}

// GetPhotosInBounds 获取边界内的照片
func (m *MapAggregator) GetPhotosInBounds(bounds GeoBounds) []*Photo {
	m.manager.mu.RLock()
	defer m.manager.mu.RUnlock()

	var photos []*Photo

	for _, photo := range m.manager.photos {
		if photo.Location == nil {
			continue
		}

		lat := photo.Location.Latitude
		lng := photo.Location.Longitude

		if lat >= bounds.MinLat && lat <= bounds.MaxLat &&
			lng >= bounds.MinLng && lng <= bounds.MaxLng {
			photos = append(photos, photo)
		}
	}

	return photos
}

// GetPhotosNearLocation 获取位置附近的照片
func (m *MapAggregator) GetPhotosNearLocation(lat, lng, radiusMeters float64) []*Photo {
	m.manager.mu.RLock()
	defer m.manager.mu.RUnlock()

	var photos []*Photo

	for _, photo := range m.manager.photos {
		if photo.Location == nil {
			continue
		}

		dist := haversineDistance(lat, lng, photo.Location.Latitude, photo.Location.Longitude)
		if dist <= radiusMeters {
			photos = append(photos, photo)
		}
	}

	// 按距离排序
	sort.Slice(photos, func(i, j int) bool {
		di := haversineDistance(lat, lng, photos[i].Location.Latitude, photos[i].Location.Longitude)
		dj := haversineDistance(lat, lng, photos[j].Location.Latitude, photos[j].Location.Longitude)
		return di < dj
	})

	return photos
}
