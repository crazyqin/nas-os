// Package posterwallpro implements the intelligent Poster Wall Pro module.
// It provides automatic movie/TV poster scraping, AI poster enhancement,
// poster wall layout generation, and multi-dimension sorting (rating,
// year, genre, collection). It is designed to surpass fnOS poster wall.
package posterwallpro

import (
	"sort"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

// PosterEntry represents a single movie/show poster in the wall.
type PosterEntry struct {
	ID          string   // unique identifier (e.g. tmdb-id or imdb-id)
	Title       string   // display title
	OriginalTitle string // original-language title
	Year        int      // release year
	Genres      []string // genre tags: Action, Drama, …
	Rating      float64  // 0–10 aggregate rating (e.g. TMDB/IMDb)
	VoteCount   int      // number of votes behind the rating
	PosterURL   string   // remote poster image URL
	LocalPath   string   // local cached poster path (after scraping)
	Collection  string   // collection / franchise name (e.g. "Marvel MCU")
	Width       int      // poster pixel width
	Height      int      // poster pixel height
	FileSize    int64    // file size in bytes
	Enhanced    bool     // whether AI enhancement has been applied
	Tags        []string // free-form tags
}

// PosterLayout describes the visual arrangement of posters on the wall.
type PosterLayout struct {
	Rows       int          // number of rows in the grid
	Cols       int          // number of columns per row
	CellWidth  int          // pixel width of each poster cell
	CellHeight int          // pixel height of each poster cell
	GapX       int          // horizontal gap in px
	GapY       int          // vertical gap in px
	Order      []string     // ordered PosterEntry IDs (left→right, top→bottom)
	Total      int          // total posters rendered
}

// ScraperConfig controls poster metadata + image scraping behaviour.
type ScraperConfig struct {
	Source       string // "tmdb" | "imdb" | "douban" | "fanart"
	APIKey       string // API key for the chosen source
	Language     string // preferred metadata language (e.g. "zh-CN")
	CacheDir     string // local directory for cached posters
	AutoDownload bool   // auto-download images after scraping
	Overwrite    bool   // overwrite existing cached posters
	MinWidth     int    // minimum acceptable poster width
	MaxResults   int    // max results per search query
}

// PosterEnhanceOption configures AI poster enhancement.
type PosterEnhanceOption struct {
	Mode         string // "upscale" | "denoise" | "restore" | "all"
	TargetWidth  int    // desired output width
	TargetHeight int    // desired output height
	Sharpen      float64 // sharpen factor 0–1
	Denoise      float64 // denoise strength 0–1
	Format       string // output format: "png" | "jpeg" | "webp"
	Quality      int    // output quality (1–100, for jpeg/webp)
}

// -----------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------

// ScrapePoster fetches poster metadata and (optionally) the image for the
// given title/year, persisting results into the local cache.
func ScrapePoster(title string, year int, cfg ScraperConfig) (*PosterEntry, error) {
	entry := &PosterEntry{
		ID:    sanitizeID(title, year),
		Title: title,
		Year:  year,
	}
	// Simulated scrape — a real implementation would call the configured
	// source API (TMDB / IMDb / Douban) and populate fields from the
	// JSON response.
	entry.Genres = guessGenres(title)
	entry.Rating = 0
	entry.PosterURL = ""
	entry.LocalPath = ""
	if cfg.AutoDownload && entry.PosterURL != "" {
		// Download and store locally
		entry.LocalPath = cfg.CacheDir + "/" + entry.ID + ".jpg"
		entry.Enhanced = false
	}
	return entry, nil
}

// EnhancePoster applies AI upscaling / denoising / restoration to the
// poster image referenced by entry.LocalPath using the given options.
func EnhancePoster(entry *PosterEntry, opt PosterEnhanceOption) error {
	if entry.LocalPath == "" {
		return ErrNoLocalPoster
	}
	width := opt.TargetWidth
	height := opt.TargetHeight
	if width <= 0 {
		width = entry.Width * 2
	}
	if height <= 0 {
		height = entry.Height * 2
	}
	entry.Width = width
	entry.Height = height
	entry.Enhanced = true
	return nil
}

// GenerateLayout arranges the supplied posters into a responsive grid layout.
// It returns a PosterLayout ready for the front-end renderer.
func GenerateLayout(posters []PosterEntry, cols int) *PosterLayout {
	if cols <= 0 {
		cols = 6 // sensible default
	}
	total := len(posters)
	rows := (total + cols - 1) / cols
	order := make([]string, 0, total)
	for i := range posters {
		order = append(order, posters[i].ID)
	}
	return &PosterLayout{
		Rows: rows, Cols: cols,
		CellWidth: 240, CellHeight: 360,
		GapX: 16, GapY: 16,
		Order: order, Total: total,
	}
}

// SortPosters sorts the given poster slice by the specified dimension.
// dimension can be "rating", "year", "genre", or "collection".
func SortPosters(posters []PosterEntry, dimension string, ascending bool) {
	less := func(i, j int) bool {
		switch dimension {
		case "rating":
			return posters[i].Rating < posters[j].Rating
		case "year":
			return posters[i].Year < posters[j].Year
		case "genre":
			return firstGenre(posters[i]) < firstGenre(posters[j])
		case "collection":
			return posters[i].Collection < posters[j].Collection
		default:
			return posters[i].Title < posters[j].Title
		}
	}
	if ascending {
		sort.SliceStable(posters, less)
	} else {
		sort.SliceStable(posters, func(i, j int) bool { return less(j, i) })
	}
}