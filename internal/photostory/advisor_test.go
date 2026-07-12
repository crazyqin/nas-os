package photostory

import (
	"testing"
	"time"
)

func TestAnalyze_EmptyCollection(t *testing.T) {
	recs := Analyze(Signal{Photos: nil})
	found := false
	for _, r := range recs {
		if r.ID == "story-empty-collection" {
			found = true
		}
	}
	if !found {
		t.Error("expected story-empty-collection recommendation")
	}
}

func TestAnalyze_TravelStory(t *testing.T) {
	photos := []PhotoMetadata{
		{ID: "1", Location: "Beijing", DateTaken: time.Now().Add(-24 * time.Hour)},
		{ID: "2", Location: "Beijing", DateTaken: time.Now().Add(-23 * time.Hour)},
		{ID: "3", Location: "Beijing", DateTaken: time.Now().Add(-22 * time.Hour)},
		{ID: "4", Location: "Beijing", DateTaken: time.Now().Add(-21 * time.Hour)},
	}
	recs := Analyze(Signal{
		Photos: photos,
		MaxStories: 10,
		IncludeLocationless: true,
	})
	found := false
	for _, r := range recs {
		if r.StoryTheme == ThemeTravel {
			found = true
		}
	}
	if !found {
		t.Error("expected travel story recommendation")
	}
}

func TestAnalyze_FamilyStory(t *testing.T) {
	photos := []PhotoMetadata{
		{ID: "1", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-24 * time.Hour)},
		{ID: "2", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-48 * time.Hour)},
		{ID: "3", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-72 * time.Hour)},
		{ID: "4", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-96 * time.Hour)},
		{ID: "5", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-120 * time.Hour)},
		{ID: "6", FaceNames: []string{"Alice"}, DateTaken: time.Now().Add(-144 * time.Hour)},
	}
	recs := Analyze(Signal{
		Photos: photos,
		MaxStories: 10,
		MinPhotosForStory: 3,
	})
	found := false
	for _, r := range recs {
		if r.StoryTheme == ThemeFamily {
			found = true
		}
	}
	if !found {
		t.Error("expected family story recommendation")
	}
}

func TestAnalyze_ThrowbackStory(t *testing.T) {
	now := time.Now()
	photos := []PhotoMetadata{
		{ID: "1", DateTaken: time.Date(now.Year()-2, now.Month(), now.Day(), 10, 0, 0, 0, time.UTC)},
		{ID: "2", DateTaken: time.Date(now.Year()-2, now.Month(), now.Day(), 11, 0, 0, 0, time.UTC)},
		{ID: "3", DateTaken: time.Date(now.Year()-2, now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)},
		{ID: "4", DateTaken: time.Date(now.Year()-3, now.Month(), now.Day(), 10, 0, 0, 0, time.UTC)},
		{ID: "5", DateTaken: time.Date(now.Year()-3, now.Month(), now.Day(), 11, 0, 0, 0, time.UTC)},
		{ID: "6", DateTaken: time.Date(now.Year()-3, now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)},
	}
	recs := Analyze(Signal{
		Photos: photos,
		ThrowbackMode: true,
		MaxStories: 10,
		MinPhotosForStory: 3,
	})
	found := false
	for _, r := range recs {
		if r.StoryTheme == ThemeThrowback {
			found = true
		}
	}
	if !found {
		t.Error("expected throwback story recommendation")
	}
}

func TestAnalyze_AdventureStory(t *testing.T) {
	photos := []PhotoMetadata{
		{ID: "1", SceneTags: []string{"hiking"}, DateTaken: time.Now().Add(-24 * time.Hour)},
		{ID: "2", SceneTags: []string{"mountain"}, DateTaken: time.Now().Add(-48 * time.Hour)},
		{ID: "3", SceneTags: []string{"beach"}, DateTaken: time.Now().Add(-72 * time.Hour)},
		{ID: "4", SceneTags: []string{"cycling"}, DateTaken: time.Now().Add(-96 * time.Hour)},
	}
	recs := Analyze(Signal{
		Photos: photos,
		MaxStories: 10,
		MinPhotosForStory: 3,
	})
	found := false
	for _, r := range recs {
		if r.StoryTheme == ThemeAdventure {
			found = true
		}
	}
	if !found {
		t.Error("expected adventure story recommendation")
	}
}

func TestAnalyze_BestOfStory(t *testing.T) {
	photos := make([]PhotoMetadata, 20)
	for i := range photos {
		photos[i] = PhotoMetadata{
			ID:          string(rune(i)),
			IsFavorite:  true,
			QualityScore: 0.9,
			DateTaken:    time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	recs := Analyze(Signal{
		Photos: photos,
		MaxStories: 10,
		MinPhotosForStory: 3,
	})
	found := false
	for _, r := range recs {
		if r.StoryTheme == ThemeMilestone && r.ID == "story-best-of" {
			found = true
		}
	}
	if !found {
		t.Error("expected best-of story recommendation")
	}
}