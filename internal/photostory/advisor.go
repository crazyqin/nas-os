// Package photostory implements AI-powered photo story generation inspired by
// Synology Photos, fnOS AI album, and Apple Memories.
package photostory

import (
	"fmt"
	"sort"
	"time"
)

// StoryTheme indicates the type of story generated from photos.
type StoryTheme string

const (
	ThemeTravel      StoryTheme = "travel"       // location-based travel story
	ThemeFamily      StoryTheme = "family"       // family gatherings and milestones
	ThemeSeasonal    StoryTheme = "seasonal"     // seasonal/holiday events
	ThemeMilestone   StoryTheme = "milestone"    // birthdays, graduations, anniversaries
	ThemeAdventure   StoryTheme = "adventure"    // outdoor activities, sports
	ThemeDaily       StoryTheme = "daily"        // everyday moments
	ThemeThrowback   StoryTheme = "throwback"    // "on this day" memories
)

// PhotoMetadata describes a photo with AI-extracted metadata.
type PhotoMetadata struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	DateTaken     time.Time `json:"date_taken"`
	Location      string    `json:"location,omitempty"`
	GPSLat        float64   `json:"gps_lat,omitempty"`
	GPSLon        float64   `json:"gps_lon,omitempty"`
	HasFaces      int       `json:"has_faces"`     // number of faces detected
	FaceNames     []string  `json:"face_names,omitempty"`
	SceneTags     []string  `json:"scene_tags,omitempty"`   // beach, sunset, party, etc.
	Objects       []string  `json:"objects,omitempty"`      // detected objects
	EmotionTags   []string  `json:"emotion_tags,omitempty"` // happy, serene, energetic
	IsFavorite    bool      `json:"is_favorite"`
	QualityScore   float64   `json:"quality_score"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
}

// Story represents a generated photo story.
type Story struct {
	Title       string         `json:"title"`
	Theme       StoryTheme     `json:"theme"`
	Summary     string         `json:"summary"`
	PhotoIDs    []string       `json:"photo_ids"`
	StartDate   time.Time     `json:"start_date"`
	EndDate     time.Time     `json:"end_date"`
	Location    string         `json:"location,omitempty"`
	Tags        []string       `json:"tags"`
	CoverPhotoID string        `json:"cover_photo_id"`
}

// Signal aggregates photo collection data for story generation.
type Signal struct {
	Photos              []PhotoMetadata `json:"photos"`
	TimeRangeDays       int             `json:"time_range_days"` // look back N days
	ThrowbackMode       bool            `json:"throwback_mode"`  // "on this day" mode
	MinPhotosForStory   int             `json:"min_photos_for_story"`
	PreferredThemes     []StoryTheme    `json:"preferred_themes,omitempty"`
	IncludeLocationless  bool            `json:"include_locationless"` // include photos without GPS
	MaxStories          int             `json:"max_stories"`
}

// Recommendation is an actionable photo story or suggestion.
type Recommendation struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Priority    string     `json:"priority"`
	Action      string     `json:"action"`
	Reason      string     `json:"reason"`
	StoryTheme  StoryTheme `json:"story_theme,omitempty"`
	PhotoCount  int        `json:"photo_count,omitempty"`
	DateString  string     `json:"date_string,omitempty"`
}

// Analyze evaluates the photo collection and generates story recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if len(s.Photos) == 0 {
		recs = append(recs, Recommendation{
			ID:       "story-empty-collection",
			Title:    "No photos available for story generation",
			Priority: "low",
			Action:   "Upload photos to enable AI story generation",
			Reason:   "Photo collection is empty; stories require at least a few photos",
		})
		return recs
	}

	minPhotos := s.MinPhotosForStory
	if minPhotos <= 0 {
		minPhotos = 3
	}

	maxStories := s.MaxStories
	if maxStories <= 0 {
		maxStories = 5
	}

	// Group photos by date clusters
	clusters := clusterByDate(s.Photos, s.TimeRangeDays)

	// Throwback mode - "on this day" in previous years
	if s.ThrowbackMode {
		throwbackStories := generateThrowbacks(s.Photos, minPhotos)
		for _, ts := range throwbackStories {
			if len(recs) >= maxStories {
				break
			}
			recs = append(recs, ts)
		}
	}

	// Travel stories (location-based)
	travelClusters := clusterByLocation(s.Photos, s.IncludeLocationless)
	for _, cluster := range travelClusters {
		if len(cluster) >= minPhotos {
			if len(recs) >= maxStories {
				break
			}
			recs = append(recs, Recommendation{
				ID:         "story-travel-" + cluster[0].Location,
				Title:      fmt.Sprintf("Travel story: %s", cluster[0].Location),
				Priority:   "medium",
				Action:     fmt.Sprintf("Generate travel story for %s with %d photos", cluster[0].Location, len(cluster)),
				Reason:     fmt.Sprintf("%d photos in %s over multiple dates suggest a travel story", len(cluster), cluster[0].Location),
				StoryTheme: ThemeTravel,
				PhotoCount: len(cluster),
			})
		}
	}

	// Family/milestone stories (faces detected)
	familyClusters := clusterByFaces(s.Photos)
	for _, cluster := range familyClusters {
		if len(cluster) >= minPhotos*2 { // need more photos for family story
			if len(recs) >= maxStories {
				break
			}
			faceName := cluster[0].FaceNames[0]
			recs = append(recs, Recommendation{
				ID:         "story-family-" + faceName,
				Title:      fmt.Sprintf("Family story: %s", faceName),
				Priority:   "medium",
				Action:     fmt.Sprintf("Generate family story featuring %s with %d photos", faceName, len(cluster)),
				Reason:     fmt.Sprintf("%d photos featuring %s suggest family milestone story", len(cluster), faceName),
				StoryTheme: ThemeFamily,
				PhotoCount: len(cluster),
			})
		}
	}

	// Seasonal stories
	seasonalClusters := clusterBySeason(s.Photos)
	for season, cluster := range seasonalClusters {
		if len(cluster) >= minPhotos {
			if len(recs) >= maxStories {
				break
			}
			recs = append(recs, Recommendation{
				ID:         "story-seasonal-" + season,
				Title:      fmt.Sprintf("Seasonal story: %s", season),
				Priority:   "low",
				Action:     fmt.Sprintf("Generate %s seasonal story with %d photos", season, len(cluster)),
				Reason:     fmt.Sprintf("%d photos from %s season capture recurring moments", len(cluster), season),
				StoryTheme: ThemeSeasonal,
				PhotoCount: len(cluster),
			})
		}
	}

	// Adventure stories (outdoor/sports scene tags)
	adventurePhotos := filterBySceneTags(s.Photos, []string{"hiking", "beach", "sports", "mountain", "diving", "cycling"})
	if len(adventurePhotos) >= minPhotos {
		if len(recs) < maxStories {
			recs = append(recs, Recommendation{
				ID:         "story-adventure",
				Title:      "Adventure story",
				Priority:   "medium",
				Action:     fmt.Sprintf("Generate adventure story with %d outdoor/activity photos", len(adventurePhotos)),
				Reason:     "Photos tagged with outdoor activities suggest adventure story",
				StoryTheme: ThemeAdventure,
				PhotoCount: len(adventurePhotos),
			})
		}
	}

	// High quality favorites collection
	favorites := filterFavorites(s.Photos, 0.8)
	if len(favorites) >= minPhotos*3 {
		if len(recs) < maxStories {
			recs = append(recs, Recommendation{
				ID:         "story-best-of",
				Title:      "Best of collection",
				Priority:   "low",
				Action:     fmt.Sprintf("Generate 'best of' story with %d high-quality favorite photos", len(favorites)),
				Reason:     fmt.Sprintf("%d favorite/high-quality photos deserve a curated collection", len(favorites)),
				StoryTheme: ThemeMilestone,
				PhotoCount: len(favorites),
			})
		}
	}

	// Date cluster stories (general)
	for _, cluster := range clusters {
		if len(cluster) >= minPhotos*2 {
			if len(recs) >= maxStories {
				break
			}
			startDate := cluster[0].DateTaken
			endDate := cluster[len(cluster)-1].DateTaken
			recs = append(recs, Recommendation{
				ID:         "story-cluster-" + startDate.Format("2006-01-02"),
				Title:      fmt.Sprintf("Story: %s", startDate.Format("Jan 2, 2006")),
				Priority:   "low",
				Action:     fmt.Sprintf("Generate story for %s-%s with %d photos", startDate.Format("Jan 2"), endDate.Format("Jan 2"), len(cluster)),
				Reason:     fmt.Sprintf("%d photos clustered in a short period suggest an event worth narrating", len(cluster)),
				StoryTheme:  ThemeDaily,
				PhotoCount:  len(cluster),
				DateString:  startDate.Format("2006-01-02"),
			})
		}
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityValue(recs[i].Priority) > priorityValue(recs[j].Priority)
	})

	return recs
}

// clusterByDate groups photos into date clusters where consecutive photos are within the given day threshold.
func clusterByDate(photos []PhotoMetadata, rangeDays int) [][]PhotoMetadata {
	if rangeDays <= 0 {
		rangeDays = 1
	}
	threshold := time.Duration(rangeDays*24) * time.Hour

	sorted := make([]PhotoMetadata, len(photos))
	copy(sorted, photos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DateTaken.Before(sorted[j].DateTaken)
	})

	var clusters [][]PhotoMetadata
	var current []PhotoMetadata

	for i, p := range sorted {
		if len(current) == 0 {
			current = append(current, p)
			continue
		}
		gap := p.DateTaken.Sub(current[len(current)-1].DateTaken)
		if gap > threshold {
			if len(current) > 0 {
				clusters = append(clusters, current)
			}
			current = []PhotoMetadata{p}
		} else {
			current = append(current, p)
		}
		_ = i
	}
	if len(current) > 0 {
		clusters = append(clusters, current)
	}
	return clusters
}

func clusterByLocation(photos []PhotoMetadata, includeLocationless bool) [][]PhotoMetadata {
	locationMap := make(map[string][]PhotoMetadata)
	for _, p := range photos {
		if p.Location == "" && !includeLocationless {
			continue
		}
		loc := p.Location
		if loc == "" {
			loc = "Unknown"
		}
		locationMap[loc] = append(locationMap[loc], p)
	}
	var result [][]PhotoMetadata
	for _, cluster := range locationMap {
		if len(cluster) >= 2 {
			result = append(result, cluster)
		}
	}
	return result
}

func clusterByFaces(photos []PhotoMetadata) [][]PhotoMetadata {
	faceMap := make(map[string][]PhotoMetadata)
	for _, p := range photos {
		if len(p.FaceNames) == 0 {
			continue
		}
		for _, name := range p.FaceNames {
			faceMap[name] = append(faceMap[name], p)
		}
	}
	var result [][]PhotoMetadata
	for _, cluster := range faceMap {
		if len(cluster) >= 2 {
			result = append(result, cluster)
		}
	}
	return result
}

func clusterBySeason(photos []PhotoMetadata) map[string][]PhotoMetadata {
	seasons := make(map[string][]PhotoMetadata)
	for _, p := range photos {
		month := int(p.DateTaken.Month())
		var season string
		switch {
		case month >= 3 && month <= 5:
			season = "Spring"
		case month >= 6 && month <= 8:
			season = "Summer"
		case month >= 9 && month <= 11:
			season = "Autumn"
		default:
			season = "Winter"
		}
		seasons[season] = append(seasons[season], p)
	}
	return seasons
}

func filterBySceneTags(photos []PhotoMetadata, tags []string) []PhotoMetadata {
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}
	var result []PhotoMetadata
	for _, p := range photos {
		for _, st := range p.SceneTags {
			if tagSet[st] {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func filterFavorites(photos []PhotoMetadata, minQuality float64) []PhotoMetadata {
	var result []PhotoMetadata
	for _, p := range photos {
		if p.IsFavorite && p.QualityScore >= minQuality {
			result = append(result, p)
		}
	}
	return result
}

func generateThrowbacks(photos []PhotoMetadata, minPhotos int) []Recommendation {
	var recs []Recommendation
	now := time.Now()
	yearsAgo := make(map[int][]PhotoMetadata)
	for _, p := range photos {
		years := now.Year() - p.DateTaken.Year()
		if years >= 1 && p.DateTaken.Month() == now.Month() && p.DateTaken.Day() == now.Day() {
			yearsAgo[years] = append(yearsAgo[years], p)
		}
	}
	for years, cluster := range yearsAgo {
		if len(cluster) >= minPhotos {
			recs = append(recs, Recommendation{
				ID:         fmt.Sprintf("story-throwback-%d", years),
				Title:      fmt.Sprintf("On this day %d years ago", years),
				Priority:   "medium",
				Action:     fmt.Sprintf("Generate throwback story with %d photos from %d years ago", len(cluster), years),
				Reason:     fmt.Sprintf("%d photos taken on this day %d years ago", len(cluster), years),
				StoryTheme:  ThemeThrowback,
				PhotoCount:  len(cluster),
				DateString:  fmt.Sprintf("%d years ago", years),
			})
		}
	}
	return recs
}

func priorityValue(p string) int {
	switch p {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}