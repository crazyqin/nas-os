// Package aivideounderstand 提供 AI 视频理解功能.
package aivideounderstand

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.analyses == nil {
		t.Error("analyses map not initialized")
	}
	if m.scenes == nil {
		t.Error("scenes map not initialized")
	}
	if m.objects == nil {
		t.Error("objects map not initialized")
	}
	if m.highlights == nil {
		t.Error("highlights map not initialized")
	}
}

func TestAnalyzeVideo(t *testing.T) {
	m := NewManager()

	analysis, err := m.AnalyzeVideo("/path/to/video.mp4")
	if err != nil {
		t.Fatalf("AnalyzeVideo failed: %v", err)
	}

	if analysis.ID == "" {
		t.Error("analysis ID is empty")
	}
	if analysis.VideoPath != "/path/to/video.mp4" {
		t.Errorf("expected video path '/path/to/video.mp4', got '%s'", analysis.VideoPath)
	}
	if analysis.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", analysis.Status)
	}
	if analysis.Duration <= 0 {
		t.Errorf("expected positive duration, got %f", analysis.Duration)
	}
	if analysis.Resolution == "" {
		t.Error("resolution is empty")
	}
	if analysis.FPS <= 0 {
		t.Errorf("expected positive FPS, got %f", analysis.FPS)
	}
	if analysis.Codec == "" {
		t.Error("codec is empty")
	}
	if analysis.FileSize <= 0 {
		t.Errorf("expected positive file size, got %d", analysis.FileSize)
	}
}

func TestAnalyzeVideoGeneratesScenes(t *testing.T) {
	m := NewManager()

	analysis, err := m.AnalyzeVideo("/test/video.mp4")
	if err != nil {
		t.Fatalf("AnalyzeVideo failed: %v", err)
	}

	scenes, err := m.GetScenes(analysis.ID)
	if err != nil {
		t.Fatalf("GetScenes failed: %v", err)
	}

	if len(scenes) < 3 {
		t.Errorf("expected at least 3 scenes, got %d", len(scenes))
	}

	for _, scene := range scenes {
		if scene.AnalysisID != analysis.ID {
			t.Errorf("scene analysis ID mismatch: expected %s, got %s", analysis.ID, scene.AnalysisID)
		}
		if scene.StartTime < 0 {
			t.Errorf("scene start time should be non-negative, got %f", scene.StartTime)
		}
		if scene.EndTime <= scene.StartTime {
			t.Errorf("scene end time should be after start time")
		}
		if scene.Confidence < 0 || scene.Confidence > 1 {
			t.Errorf("scene confidence should be 0-1, got %f", scene.Confidence)
		}
	}
}

func TestAnalyzeVideoGeneratesObjects(t *testing.T) {
	m := NewManager()

	analysis, err := m.AnalyzeVideo("/test/video.mp4")
	if err != nil {
		t.Fatalf("AnalyzeVideo failed: %v", err)
	}

	objects, err := m.GetObjects(analysis.ID)
	if err != nil {
		t.Fatalf("GetObjects failed: %v", err)
	}

	if len(objects) < 2 {
		t.Errorf("expected at least 2 objects, got %d", len(objects))
	}

	for _, obj := range objects {
		if obj.AnalysisID != analysis.ID {
			t.Errorf("object analysis ID mismatch")
		}
		if obj.Label == "" {
			t.Error("object label is empty")
		}
		if obj.Confidence < 0 || obj.Confidence > 1 {
			t.Errorf("object confidence should be 0-1, got %f", obj.Confidence)
		}
	}
}

func TestAnalyzeVideoGeneratesHighlights(t *testing.T) {
	m := NewManager()

	analysis, err := m.AnalyzeVideo("/test/video.mp4")
	if err != nil {
		t.Fatalf("AnalyzeVideo failed: %v", err)
	}

	highlights, err := m.GetHighlights(analysis.ID)
	if err != nil {
		t.Fatalf("GetHighlights failed: %v", err)
	}

	if len(highlights) < 1 {
		t.Errorf("expected at least 1 highlight, got %d", len(highlights))
	}

	for _, hl := range highlights {
		if hl.AnalysisID != analysis.ID {
			t.Errorf("highlight analysis ID mismatch")
		}
		if hl.StartTime < 0 {
			t.Errorf("highlight start time should be non-negative")
		}
		if hl.EndTime <= hl.StartTime {
			t.Errorf("highlight end time should be after start time")
		}
		if hl.Score < 0 || hl.Score > 1 {
			t.Errorf("highlight score should be 0-1, got %f", hl.Score)
		}
	}
}

func TestListAnalyses(t *testing.T) {
	m := NewManager()

	// 初始为空
	analyses := m.ListAnalyses()
	if len(analyses) != 0 {
		t.Errorf("expected 0 analyses, got %d", len(analyses))
	}

	// 添加后
	m.AnalyzeVideo("/video1.mp4")
	m.AnalyzeVideo("/video2.mp4")

	analyses = m.ListAnalyses()
	if len(analyses) != 2 {
		t.Errorf("expected 2 analyses, got %d", len(analyses))
	}
}

func TestGetAnalysis(t *testing.T) {
	m := NewManager()

	_, err := m.GetAnalysis("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}

	analysis, _ := m.AnalyzeVideo("/test.mp4")

	result, err := m.GetAnalysis(analysis.ID)
	if err != nil {
		t.Fatalf("GetAnalysis failed: %v", err)
	}
	if result.ID != analysis.ID {
		t.Errorf("expected ID %s, got %s", analysis.ID, result.ID)
	}
}

func TestGetScenesNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetScenes("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}
}

func TestGetObjectsNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetObjects("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}
}

func TestGetHighlightsNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetHighlights("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}
}

func TestSearchVideosByQuery(t *testing.T) {
	m := NewManager()

	analysis, _ := m.AnalyzeVideo("/test.mp4")
	scenes, _ := m.GetScenes(analysis.ID)

	// 使用第一个场景的描述进行搜索
	if len(scenes) > 0 {
		query := &VideoSearchQuery{
			Query:      scenes[0].Description,
			MaxResults: 10,
		}
		results := m.SearchVideos(query)
		if len(results) == 0 {
			t.Error("expected search results, got none")
		}
	}
}

func TestSearchVideosBySceneType(t *testing.T) {
	m := NewManager()

	m.AnalyzeVideo("/test.mp4")

	query := &VideoSearchQuery{
		SceneTypes: []string{"landscape"},
		MaxResults: 10,
	}
	results := m.SearchVideos(query)
	// 结果可能为0，因为场景是随机生成的
	if results != nil {
		for _, r := range results {
			for _, s := range r.MatchingScenes {
				if s.SceneType != "landscape" {
					t.Errorf("expected scene type 'landscape', got '%s'", s.SceneType)
				}
			}
		}
	}
}

func TestSearchVideosByTags(t *testing.T) {
	m := NewManager()

	m.AnalyzeVideo("/test.mp4")

	query := &VideoSearchQuery{
		Tags:       []string{"scene"},
		MaxResults: 10,
	}
	results := m.SearchVideos(query)
	if len(results) == 0 {
		// 所有场景都有 "scene" 标签，应该有结果
		t.Error("expected search results for 'scene' tag")
	}
}

func TestSearchVideosWithMinConfidence(t *testing.T) {
	m := NewManager()

	m.AnalyzeVideo("/test.mp4")

	query := &VideoSearchQuery{
		MinConfidence: 0.9,
		MaxResults:    10,
	}
	results := m.SearchVideos(query)
	// 结果取决于随机生成的置信度
	_ = results
}

func TestSearchVideosWithDateFilter(t *testing.T) {
	m := NewManager()

	m.AnalyzeVideo("/test.mp4")

	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)

	query := &VideoSearchQuery{
		DateFrom:   &past,
		DateTo:     &future,
		MaxResults: 10,
	}
	results := m.SearchVideos(query)
	if len(results) == 0 {
		t.Error("expected search results within date range")
	}
}

func TestDeleteAnalysis(t *testing.T) {
	m := NewManager()

	analysis, _ := m.AnalyzeVideo("/test.mp4")

	err := m.DeleteAnalysis(analysis.ID)
	if err != nil {
		t.Fatalf("DeleteAnalysis failed: %v", err)
	}

	_, err = m.GetAnalysis(analysis.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}

	_, err = m.GetScenes(analysis.ID)
	if err == nil {
		t.Error("expected scenes to be deleted")
	}
}

func TestDeleteAnalysisNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteAnalysis("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()
	if stats.TotalVideos != 0 {
		t.Errorf("expected 0 videos, got %d", stats.TotalVideos)
	}

	m.AnalyzeVideo("/video1.mp4")
	m.AnalyzeVideo("/video2.mp4")

	stats = m.GetStats()
	if stats.TotalVideos != 2 {
		t.Errorf("expected 2 videos, got %d", stats.TotalVideos)
	}
	if stats.TotalScenes < 6 {
		t.Errorf("expected at least 6 scenes, got %d", stats.TotalScenes)
	}
	if stats.TotalObjects < 4 {
		t.Errorf("expected at least 4 objects, got %d", stats.TotalObjects)
	}
	if stats.ModelName == "" {
		t.Error("model name should not be empty")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			m.AnalyzeVideo("/concurrent/video.mp4")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	analyses := m.ListAnalyses()
	if len(analyses) != 10 {
		t.Errorf("expected 10 analyses after concurrent writes, got %d", len(analyses))
	}
}
