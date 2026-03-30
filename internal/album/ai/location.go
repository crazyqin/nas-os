// Package ai - Location clustering implementation
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// LocationClusterer provides geographic clustering for photos
type LocationClusterer struct {
	geocodingProvider string
	geocodingAPIKey   string
	httpClient        *http.Client
	cache             map[string]*PlaceInfo
	cacheMu           sync.RWMutex
}

// NewLocationClusterer creates a new location clusterer
func NewLocationClusterer(provider, apiKey string) *LocationClusterer {
	return &LocationClusterer{
		geocodingProvider: provider,
		geocodingAPIKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]*PlaceInfo),
	}
}

// PhotoLocation represents a photo with GPS data
type PhotoLocation struct {
	PhotoID string
	Lat     float64
	Lng     float64
	TakenAt time.Time
}

// ClusterPhotos clusters photos by geographic location
func (lc *LocationClusterer) ClusterPhotos(ctx context.Context, photos []PhotoLocation, radiusMeters float64) ([]*LocationCluster, error) {
	if len(photos) == 0 {
		return []*LocationCluster{}, nil
	}

	// Use DBSCAN-like clustering
	clusters := lc.dbscanClustering(photos, radiusMeters)

	// Enhance clusters with geocoding
	for _, cluster := range clusters {
		if placeInfo, err := lc.ReverseGeocode(ctx, cluster.CenterLat, cluster.CenterLng); err == nil {
			cluster.PlaceInfo = placeInfo
			if cluster.Name == "" {
				cluster.Name = lc.generateClusterName(placeInfo)
			}
		}
	}

	return clusters, nil
}

// dbscanClustering performs DBSCAN clustering on photo locations
func (lc *LocationClusterer) dbscanClustering(photos []PhotoLocation, radiusMeters float64) []*LocationCluster {
	n := len(photos)
	visited := make([]bool, n)
	labels := make([]int, n)
	for i := range labels {
		labels[i] = -1 // -1 = noise
	}

	clusterID := 0
	minPts := 2 // Minimum points to form a cluster

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		visited[i] = true

		neighbors := lc.getNeighbors(photos, i, radiusMeters)
		if len(neighbors) < minPts {
			// Mark as noise
			continue
		}

		// Start new cluster
		clusterID++
		labels[i] = clusterID

		// Expand cluster
		seedSet := make([]int, len(neighbors))
		copy(seedSet, neighbors)

		for len(seedSet) > 0 {
			current := seedSet[0]
			seedSet = seedSet[1:]

			if !visited[current] {
				visited[current] = true
				currentNeighbors := lc.getNeighbors(photos, current, radiusMeters)
				if len(currentNeighbors) >= minPts {
					seedSet = append(seedSet, currentNeighbors...)
				}
			}

			if labels[current] == -1 {
				labels[current] = clusterID
			}
		}
	}

	// Build cluster objects
	clusterMap := make(map[int]*LocationCluster)
	for i, label := range labels {
		if label == -1 {
			continue
		}

		if _, exists := clusterMap[label]; !exists {
			clusterMap[label] = &LocationCluster{
				ID:        fmt.Sprintf("loc_%d", label),
				PhotoIDs:  make([]string, 0),
				CreatedAt: time.Now(),
			}
		}

		cluster := clusterMap[label]
		cluster.PhotoIDs = append(cluster.PhotoIDs, photos[i].PhotoID)
		cluster.PhotoCount++
	}

	// Calculate cluster centers and date ranges
	for _, cluster := range clusterMap {
		lc.calculateClusterStats(cluster, photos, labels)
	}

	// Convert to slice
	result := make([]*LocationCluster, 0, len(clusterMap))
	for _, cluster := range clusterMap {
		result = append(result, cluster)
	}

	return result
}

// getNeighbors returns indices of photos within radius
func (lc *LocationClusterer) getNeighbors(photos []PhotoLocation, idx int, radiusMeters float64) []int {
	neighbors := make([]int, 0)
	for i, p := range photos {
		if i == idx {
			continue
		}
		dist := haversineDistance(photos[idx].Lat, photos[idx].Lng, p.Lat, p.Lng)
		if dist <= radiusMeters {
			neighbors = append(neighbors, i)
		}
	}
	return neighbors
}

// calculateClusterStats calculates cluster center and date range
func (lc *LocationClusterer) calculateClusterStats(cluster *LocationCluster, photos []PhotoLocation, labels []int) {
	var sumLat, sumLng float64
	var minTime, maxTime time.Time
	count := 0

	clusterPhotoIDs := make(map[string]bool)
	for _, id := range cluster.PhotoIDs {
		clusterPhotoIDs[id] = true
	}

	for i, p := range photos {
		if labels[i] == -1 {
			continue
		}
		if clusterID := fmt.Sprintf("loc_%d", labels[i]); clusterID == cluster.ID {
			sumLat += p.Lat
			sumLng += p.Lng
			count++

			if minTime.IsZero() || p.TakenAt.Before(minTime) {
				minTime = p.TakenAt
			}
			if maxTime.IsZero() || p.TakenAt.After(maxTime) {
				maxTime = p.TakenAt
			}
		}
	}

	if count > 0 {
		cluster.CenterLat = sumLat / float64(count)
		cluster.CenterLng = sumLng / float64(count)
		cluster.DateRange = DateRange{
			Start: minTime,
			End:   maxTime,
		}
	}
}

// ReverseGeocode performs reverse geocoding
func (lc *LocationClusterer) ReverseGeocode(ctx context.Context, lat, lng float64) (*PlaceInfo, error) {
	// Check cache
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lng)
	lc.cacheMu.RLock()
	if cached, ok := lc.cache[cacheKey]; ok {
		lc.cacheMu.RUnlock()
		return cached, nil
	}
	lc.cacheMu.RUnlock()

	var placeInfo *PlaceInfo
	var err error

	switch lc.geocodingProvider {
	case "nominatim":
		placeInfo, err = lc.nominatimReverseGeocode(ctx, lat, lng)
	case "google":
		placeInfo, err = lc.googleReverseGeocode(ctx, lat, lng)
	case "baidu":
		placeInfo, err = lc.baiduReverseGeocode(ctx, lat, lng)
	default:
		placeInfo, err = lc.nominatimReverseGeocode(ctx, lat, lng)
	}

	if err == nil && placeInfo != nil {
		lc.cacheMu.Lock()
		lc.cache[cacheKey] = placeInfo
		lc.cacheMu.Unlock()
	}

	return placeInfo, err
}

// nominatimReverseGeocode uses OpenStreetMap Nominatim
func (lc *LocationClusterer) nominatimReverseGeocode(ctx context.Context, lat, lng float64) (*PlaceInfo, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f&zoom=14", lat, lng)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NAS-OS-PhotoAlbum/1.0")

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned %d", resp.StatusCode)
	}

	var result struct {
		Address struct {
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
			State       string `json:"state"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			County      string `json:"county"`
			Suburb      string `json:"suburb"`
			POI         string `json:"poi"`
		} `json:"address"`
		PlaceID uint64 `json:"place_id"`
		POI     string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	city := result.Address.City
	if city == "" {
		city = result.Address.Town
	}
	if city == "" {
		city = result.Address.Village
	}

	return &PlaceInfo{
		Country:     result.Address.Country,
		CountryCode: result.Address.CountryCode,
		Province:    result.Address.State,
		City:        city,
		District:    result.Address.County,
		POI:         result.POI,
		PlaceID:     fmt.Sprintf("%d", result.PlaceID),
	}, nil
}

// googleReverseGeocode uses Google Maps Geocoding API
func (lc *LocationClusterer) googleReverseGeocode(ctx context.Context, lat, lng float64) (*PlaceInfo, error) {
	if lc.geocodingAPIKey == "" {
		return nil, fmt.Errorf("Google API key not configured")
	}

	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?latlng=%f,%f&key=%s", lat, lng, lc.geocodingAPIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			AddressComponents []struct {
				LongName string   `json:"long_name"`
				Types    []string `json:"types"`
			} `json:"address_components"`
			PlaceID string `json:"place_id"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("no results from Google Geocoding")
	}

	placeInfo := &PlaceInfo{}
	for _, comp := range result.Results[0].AddressComponents {
		for _, t := range comp.Types {
			switch t {
			case "country":
				placeInfo.Country = comp.LongName
			case "administrative_area_level_1":
				placeInfo.Province = comp.LongName
			case "locality":
				placeInfo.City = comp.LongName
			case "administrative_area_level_2":
				placeInfo.District = comp.LongName
			}
		}
	}
	placeInfo.PlaceID = result.Results[0].PlaceID

	return placeInfo, nil
}

// baiduReverseGeocode uses Baidu Maps API
func (lc *LocationClusterer) baiduReverseGeocode(ctx context.Context, lat, lng float64) (*PlaceInfo, error) {
	if lc.geocodingAPIKey == "" {
		return nil, fmt.Errorf("Baidu API key not configured")
	}

	apiURL := fmt.Sprintf("https://api.map.baidu.com/reverse_geocoding/v3/?output=json&coordtype=wgs84ll&location=%f,%f&ak=%s", lat, lng, lc.geocodingAPIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status int `json:"status"`
		Result struct {
			AddressComponent struct {
				Country  string `json:"country"`
				Province string `json:"province"`
				City     string `json:"city"`
				District string `json:"district"`
				Street   string `json:"street"`
			} `json:"addressComponent"`
			SematicDescription string `json:"sematic_description"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("Baidu API returned status %d", result.Status)
	}

	return &PlaceInfo{
		Country:     result.Result.AddressComponent.Country,
		CountryCode: "CN",
		Province:    result.Result.AddressComponent.Province,
		City:        result.Result.AddressComponent.City,
		District:    result.Result.AddressComponent.District,
		POI:         result.Result.SematicDescription,
	}, nil
}

// generateClusterName generates a human-readable cluster name
func (lc *LocationClusterer) generateClusterName(placeInfo *PlaceInfo) string {
	if placeInfo == nil {
		return "Unknown Location"
	}

	// Priority: POI > City > Province > Country
	if placeInfo.POI != "" {
		return placeInfo.POI
	}
	if placeInfo.City != "" {
		return placeInfo.City
	}
	if placeInfo.Province != "" {
		return placeInfo.Province
	}
	if placeInfo.Country != "" {
		return placeInfo.Country
	}

	return "Unknown Location"
}

// GroupPhotosByCity groups photos by city
func (lc *LocationClusterer) GroupPhotosByCity(ctx context.Context, photos []PhotoLocation) (map[string][]string, error) {
	cityMap := make(map[string][]string)

	for _, p := range photos {
		placeInfo, err := lc.ReverseGeocode(ctx, p.Lat, p.Lng)
		if err != nil {
			continue
		}

		city := placeInfo.City
		if city == "" {
			city = "Unknown"
		}

		cityMap[city] = append(cityMap[city], p.PhotoID)
	}

	return cityMap, nil
}

// GroupPhotosByCountry groups photos by country
func (lc *LocationClusterer) GroupPhotosByCountry(ctx context.Context, photos []PhotoLocation) (map[string][]string, error) {
	countryMap := make(map[string][]string)

	for _, p := range photos {
		placeInfo, err := lc.ReverseGeocode(ctx, p.Lat, p.Lng)
		if err != nil {
			continue
		}

		country := placeInfo.Country
		if country == "" {
			country = "Unknown"
		}

		countryMap[country] = append(countryMap[country], p.PhotoID)
	}

	return countryMap, nil
}

// SearchLocation searches for locations by name
func (lc *LocationClusterer) SearchLocation(ctx context.Context, query string) ([]PlaceInfo, error) {
	switch lc.geocodingProvider {
	case "nominatim":
		return lc.nominatimSearch(ctx, query)
	case "google":
		return lc.googleSearch(ctx, query)
	default:
		return lc.nominatimSearch(ctx, query)
	}
}

// nominatimSearch uses Nominatim for location search
func (lc *LocationClusterer) nominatimSearch(ctx context.Context, query string) ([]PlaceInfo, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/search?format=json&q=%s&limit=5", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NAS-OS-PhotoAlbum/1.0")

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []struct {
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
		PlaceID uint64 `json:"place_id"`
		Name    string `json:"display_name"`
		Type    string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	places := make([]PlaceInfo, len(results))
	for i, r := range results {
		places[i] = PlaceInfo{
			PlaceID: fmt.Sprintf("%d", r.PlaceID),
			POI:     r.Name,
		}
	}

	return places, nil
}

// googleSearch uses Google Places API for location search
func (lc *LocationClusterer) googleSearch(ctx context.Context, query string) ([]PlaceInfo, error) {
	if lc.geocodingAPIKey == "" {
		return nil, fmt.Errorf("Google API key not configured")
	}

	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/textsearch/json?query=%s&key=%s", url.QueryEscape(query), lc.geocodingAPIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			PlaceID   string `json:"place_id"`
			Name      string `json:"name"`
			Formatted string `json:"formatted_address"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	places := make([]PlaceInfo, len(result.Results))
	for i, r := range result.Results {
		places[i] = PlaceInfo{
			PlaceID: r.PlaceID,
			POI:     r.Name,
		}
	}

	return places, nil
}

// haversineDistance calculates distance between two GPS coordinates in meters
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000 // meters

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// Close releases resources
func (lc *LocationClusterer) Close() error {
	return nil
}
