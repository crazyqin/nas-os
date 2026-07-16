// Package posterscraper implements intelligent media poster scraping inspired by
// Synology Video Station, TrueNAS media plugins, and fnOS poster wall.
package posterscraper

import (
	"sort"
	"strings"
	"time"
)

// MediaType indicates the type of media item.
type MediaType string

const (
	MediaMovie    MediaType = "movie"
	MediaTVShow   MediaType = "tv_show"
	MediaMusic    MediaType = "music"
	MediaHomePage MediaType = "home_video"
)

// ScrapingSource indicates the metadata provider.
type ScrapingSource string

const (
	SourceTMDB     ScrapingSource = "tmdb"
	SourceTVDB     ScrapingSource = "tvdb"
	SourceIMDB     ScrapingSource = "imdb"
	SourceMusicBrainz ScrapingSource = "musicbrainz"
	SourceLocal    ScrapingSource = "local_embedded"
)

// MediaItem describes a single media file needing poster scraping.
type MediaItem struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Type          MediaType     `json:"type"`
	FilePath      string        `json:"file_path"`
	HasPoster     bool          `json:"has_poster"`
	HasMetadata   bool          `json:"has_metadata"`
	HasSubtitle   bool          `json:"has_subtitle"`
	Source         ScrapingSource `json:"source,omitempty"`
	Resolution    string        `json:"resolution,omitempty"`
	DurationMin   int           `json:"duration_min,omitempty"`
	Year          int           `json:"year,omitempty"`
	ParseConfidence float64     `json:"parse_confidence,omitempty"`
}

// Signal describes the current media library scraping state.
type Signal struct {
	TotalItems           int
	ItemsWithoutPoster    int
	ItemsWithoutMetadata  int
	ItemsWithoutSubtitle  int
	ParseFailures         int
	Items                 []MediaItem
	LibraryLastScrapedAt  time.Time
	AutoScrapeEnabled     bool
	PosterCacheGB         float64
	MaxPosterCacheGB      float64
	ParseLowConfidence    int
}

// Recommendation is an actionable poster scraping suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates media poster scraping signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.TotalItems == 0 {
		return recs
	}

	missingPct := 0
	if s.TotalItems > 0 {
		missingPct = s.ItemsWithoutPoster * 100 / s.TotalItems
	}

	if missingPct > 30 {
		recs = append(recs, Recommendation{
			ID:       "poster-batch-scrape",
			Title:    "Batch scrape missing posters",
			Priority: "high",
			Action:   "Run full library scrape to download missing posters from TMDB/TVDB",
			Reason:   "Over 30% of library items are missing poster artwork",
		})
	}

	if s.ParseFailures > 0 && s.TotalItems > 0 {
		failRate := s.ParseFailures * 100 / s.TotalItems
		if failRate > 10 {
			recs = append(recs, Recommendation{
				ID:       "poster-parse-rules",
				Title:    "Fix file naming parse failures",
				Priority: "high",
				Action:   "Rename files to match scraper naming conventions (e.g., Movie Title (Year).mkv)",
				Reason:   "Over 10% of files failed to parse, preventing correct metadata matching",
			})
		}
	}

	if s.ItemsWithoutSubtitle > 0 && s.TotalItems > 0 {
		subPct := s.ItemsWithoutSubtitle * 100 / s.TotalItems
		if subPct > 50 {
			recs = append(recs, Recommendation{
				ID:       "poster-subtitle-fetch",
				Title:    "Fetch missing subtitles",
				Priority: "medium",
				Action:   "Use OpenSubtitles integration to download subtitles for media without them",
				Reason:   "Over 50% of media items lack subtitles, reducing accessibility",
			})
		}
	}

	if s.ParseLowConfidence > 0 && s.TotalItems > 0 {
		confRate := s.ParseLowConfidence * 100 / s.TotalItems
		if confRate > 20 {
			recs = append(recs, Recommendation{
				ID:       "poster-low-confidence",
				Title:    "Review low confidence matches",
				Priority: "medium",
				Action:   "Manually verify and correct items with parse confidence below 60%",
				Reason:   "Low confidence matches often result in wrong posters and metadata",
			})
		}
	}

	if !s.AutoScrapeEnabled && s.TotalItems > 50 {
		recs = append(recs, Recommendation{
			ID:       "poster-auto-scrape",
			Title:    "Enable automatic scraping",
			Priority: "low",
			Action:   "Enable scheduled auto-scrape to keep library updated when new files are added",
			Reason:   "Library has many items; manual scraping is error-prone and hard to keep current",
		})
	}

	if s.PosterCacheGB > s.MaxPosterCacheGB && s.MaxPosterCacheGB > 0 {
		recs = append(recs, Recommendation{
			ID:       "poster-cache-prune",
			Title:    "Poster cache exceeds limit",
			Priority: "low",
			Action:   "Clean up orphaned poster images for deleted media items",
			Reason:   "Poster cache exceeds configured limit, occupying unnecessary disk space",
		})
	}

	if !s.LibraryLastScrapedAt.IsZero() && time.Since(s.LibraryLastScrapedAt) > 30*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "poster-stale-library",
			Title:    "Library scrape is stale",
			Priority: "low",
			Action:   "Run a full library refresh to update metadata and posters",
			Reason:   "Last scrape is over 30 days old; new media may have wrong or missing artwork",
		})
	}

	for _, item := range s.Items {
		if item.Type == MediaTVShow && !item.HasMetadata {
			episodeMode := strings.Contains(item.FilePath, "S") && strings.Contains(item.FilePath, "E")
			if !episodeMode {
				recs = append(recs, Recommendation{
					ID:       "poster-tv-naming-" + item.ID,
					Title:    "TV show file naming needs SxxExx format",
					Priority: "medium",
					Action:   "Rename TV show files to include season and episode numbers (S01E01)",
					Reason:   "TV show files without SxxExx pattern cannot be properly episode-matched",
				})
			}
			break
		}
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

func priorityRank(p string) int {
	switch p {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}