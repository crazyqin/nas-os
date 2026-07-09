package posterscraper

import (
	"testing"
	"time"
)

func TestAnalyze_BatchScrapeMissing(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:         100,
		ItemsWithoutPoster: 50,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-batch-scrape" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected poster-batch-scrape recommendation")
	}
}

func TestAnalyze_ParseFailures(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:    100,
		ParseFailures: 15,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-parse-rules" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-parse-rules recommendation")
	}
}

func TestAnalyze_MissingSubtitles(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:          100,
		ItemsWithoutSubtitle: 60,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-subtitle-fetch" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-subtitle-fetch recommendation")
	}
}

func TestAnalyze_LowConfidence(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:        100,
		ParseLowConfidence: 25,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-low-confidence" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-low-confidence recommendation")
	}
}

func TestAnalyze_AutoScrape(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:        60,
		AutoScrapeEnabled: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-auto-scrape" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-auto-scrape recommendation")
	}
}

func TestAnalyze_CacheExceeded(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:       10,
		PosterCacheGB:    10,
		MaxPosterCacheGB: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-cache-prune" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-cache-prune recommendation")
	}
}

func TestAnalyze_StaleLibrary(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:          100,
		LibraryLastScrapedAt: time.Now().AddDate(0, 0, -35),
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-stale-library" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-stale-library recommendation")
	}
}

func TestAnalyze_TVSHowNaming(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems: 10,
		Items: []MediaItem{
			{ID: "tv1", Type: MediaTVShow, FilePath: "/media/show/episode01.mkv", HasMetadata: false},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-tv-naming-tv1" {
			found = true
		}
	}
	if !found {
		t.Error("expected poster-tv-naming-tv1 recommendation")
	}
}

func TestAnalyze_TVSHowCorrectNaming(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems: 10,
		Items: []MediaItem{
			{ID: "tv2", Type: MediaTVShow, FilePath: "/media/show/S01E01.mkv", HasMetadata: false},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "poster-tv-naming-tv2" {
			found = true
		}
	}
	if found {
		t.Error("should not recommend renaming for correctly named SxxExx files")
	}
}

func TestAnalyze_EmptyLibrary(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations for empty library, got %d", len(recs))
	}
}

func TestAnalyze_PriorityOrdering(t *testing.T) {
	recs := Analyze(Signal{
		TotalItems:          100,
		ItemsWithoutPoster:   50,
		ParseFailures:        15,
		ItemsWithoutSubtitle: 60,
		AutoScrapeEnabled:    false,
	})
	if len(recs) < 2 {
		t.Fatal("expected multiple recommendations")
	}
	for i := 0; i < len(recs)-1; i++ {
		if priorityRank(recs[i].Priority) > priorityRank(recs[i+1].Priority) {
			t.Errorf("recommendations not sorted by priority at index %d", i)
		}
	}
}