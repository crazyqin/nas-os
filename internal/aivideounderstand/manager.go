// Package aivideounderstand 提供 AI 视频理解功能.
package aivideounderstand

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Manager 视频理解管理器.
type Manager struct {
	mu        sync.RWMutex
	analyses  map[string]*VideoAnalysis
	scenes    map[string][]*Scene
	objects   map[string][]*DetectedObject
	highlights map[string][]*VideoHighlight
}

// NewManager 创建新的 Manager 实例.
func NewManager() *Manager {
	return &Manager{
		analyses:   make(map[string]*VideoAnalysis),
		scenes:     make(map[string][]*Scene),
		objects:    make(map[string][]*DetectedObject),
		highlights: make(map[string][]*VideoHighlight),
	}
}

// AnalyzeVideo 提交视频分析 (模拟分析).
func (m *Manager) AnalyzeVideo(videoPath string) (*VideoAnalysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("va-%d", time.Now().UnixNano())
	start := time.Now()

	analysis := &VideoAnalysis{
		ID:        id,
		VideoPath: videoPath,
		Status:    "processing",
		AnalyzedAt: start,
	}

	// 模拟分析结果
	analysis.Duration = 30.0 + rand.Float64()*120.0
	analysis.Resolution = "1920x1080"
	analysis.FPS = 30.0
	analysis.Codec = "h264"
	analysis.FileSize = int64(analysis.Duration * 5 * 1024 * 1024) // ~5MB/s
	analysis.ProcessingTimeMs = time.Since(start).Milliseconds()
	analysis.Status = "completed"

	m.analyses[id] = analysis

	// 生成模拟场景
	sceneTypes := []string{"action", "dialogue", "transition", "landscape", "indoor", "outdoor"}
	sceneDescriptions := []string{
		"城市街景，车辆行驶",
		"室内对话场景",
		"自然风光，山川河流",
		"人物活动，运动场景",
		"产品展示，特写镜头",
		"夜景灯光，霓虹闪烁",
	}

	numScenes := 3 + rand.Intn(5)
	currentTime := 0.0
	for i := 0; i < numScenes; i++ {
		sceneDur := 5.0 + rand.Float64()*15.0
		scene := &Scene{
			ID:         fmt.Sprintf("scene-%s-%d", id, i),
			AnalysisID: id,
			StartTime:  currentTime,
			EndTime:    currentTime + sceneDur,
			Description: sceneDescriptions[rand.Intn(len(sceneDescriptions))],
			Tags:       []string{"scene", sceneTypes[rand.Intn(len(sceneTypes))]},
			Confidence: 0.7 + rand.Float64()*0.3,
			SceneType:  sceneTypes[rand.Intn(len(sceneTypes))],
		}
		m.scenes[id] = append(m.scenes[id], scene)
		currentTime += sceneDur
	}

	// 生成模拟物体
	objectLabels := []string{"person", "car", "dog", "cat", "bicycle", "laptop", "phone", "chair", "table", "book"}
	numObjects := 2 + rand.Intn(4)
	for i := 0; i < numObjects; i++ {
		obj := &DetectedObject{
			ID:         fmt.Sprintf("obj-%s-%d", id, i),
			AnalysisID: id,
			Label:      objectLabels[rand.Intn(len(objectLabels))],
			Confidence: 0.6 + rand.Float64()*0.4,
			BoundingBox: BoundingBox{
				X: rand.Float64() * 800,
				Y: rand.Float64() * 600,
				W: 50 + rand.Float64()*200,
				H: 50 + rand.Float64()*200,
			},
			FirstSeen: rand.Float64() * analysis.Duration,
			LastSeen:  rand.Float64() * analysis.Duration,
			TrackID:   i + 1,
		}
		m.objects[id] = append(m.objects[id], obj)
	}

	// 生成模拟高光时刻
	numHighlights := 1 + rand.Intn(3)
	for i := 0; i < numHighlights; i++ {
		highlightStart := rand.Float64() * (analysis.Duration - 10)
		highlight := &VideoHighlight{
			ID:         fmt.Sprintf("hl-%s-%d", id, i),
			AnalysisID: id,
			StartTime:  highlightStart,
			EndTime:    highlightStart + 5.0 + rand.Float64()*10.0,
			Reason:     "精彩瞬间",
			Score:      0.8 + rand.Float64()*0.2,
		}
		m.highlights[id] = append(m.highlights[id], highlight)
	}

	return analysis, nil
}

// ListAnalyses 列出所有分析.
func (m *Manager) ListAnalyses() []*VideoAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*VideoAnalysis, 0, len(m.analyses))
	for _, a := range m.analyses {
		results = append(results, a)
	}
	return results
}

// GetAnalysis 获取单个分析详情.
func (m *Manager) GetAnalysis(id string) (*VideoAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysis, exists := m.analyses[id]
	if !exists {
		return nil, fmt.Errorf("analysis not found: %s", id)
	}
	return analysis, nil
}

// GetScenes 获取分析的场景列表.
func (m *Manager) GetScenes(analysisID string) ([]*Scene, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.analyses[analysisID]; !exists {
		return nil, fmt.Errorf("analysis not found: %s", analysisID)
	}

	return m.scenes[analysisID], nil
}

// GetObjects 获取分析的物体列表.
func (m *Manager) GetObjects(analysisID string) ([]*DetectedObject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.analyses[analysisID]; !exists {
		return nil, fmt.Errorf("analysis not found: %s", analysisID)
	}

	return m.objects[analysisID], nil
}

// SearchVideos 基于文本匹配场景描述进行语义搜索.
func (m *Manager) SearchVideos(query *VideoSearchQuery) []*VideoSearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*VideoSearchResult

	maxResults := query.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	for _, analysis := range m.analyses {
		if query.DateFrom != nil && analysis.AnalyzedAt.Before(*query.DateFrom) {
			continue
		}
		if query.DateTo != nil && analysis.AnalyzedAt.After(*query.DateTo) {
			continue
		}

		var matchingScenes []Scene
		scenes := m.scenes[analysis.ID]

		for _, scene := range scenes {
			if query.MinConfidence > 0 && scene.Confidence < query.MinConfidence {
				continue
			}

			if len(query.SceneTypes) > 0 {
				found := false
				for _, st := range query.SceneTypes {
					if scene.SceneType == st {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if len(query.Tags) > 0 {
				hasTag := false
				for _, tag := range query.Tags {
					for _, sceneTag := range scene.Tags {
						if strings.EqualFold(tag, sceneTag) {
							hasTag = true
							break
						}
					}
					if hasTag {
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			// 文本匹配
			if query.Query != "" {
				queryLower := strings.ToLower(query.Query)
				descLower := strings.ToLower(scene.Description)
				if !strings.Contains(descLower, queryLower) {
					// 也检查标签
					matched := false
					for _, tag := range scene.Tags {
						if strings.Contains(strings.ToLower(tag), queryLower) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
			}

			matchingScenes = append(matchingScenes, *scene)
		}

		if len(matchingScenes) > 0 {
			relevance := float64(len(matchingScenes)) / float64(len(scenes))
			results = append(results, &VideoSearchResult{
				VideoPath:      analysis.VideoPath,
				MatchingScenes: matchingScenes,
				TotalMatches:   len(matchingScenes),
				RelevanceScore: math.Min(relevance, 1.0),
			})

			if len(results) >= maxResults {
				break
			}
		}
	}

	return results
}

// GetHighlights 获取视频高光时刻.
func (m *Manager) GetHighlights(analysisID string) ([]*VideoHighlight, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.analyses[analysisID]; !exists {
		return nil, fmt.Errorf("analysis not found: %s", analysisID)
	}

	return m.highlights[analysisID], nil
}

// DeleteAnalysis 删除分析结果.
func (m *Manager) DeleteAnalysis(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.analyses[id]; !exists {
		return fmt.Errorf("analysis not found: %s", id)
	}

	delete(m.analyses, id)
	delete(m.scenes, id)
	delete(m.objects, id)
	delete(m.highlights, id)

	return nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *AnalysisStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AnalysisStats{
		TotalVideos: len(m.analyses),
		ModelName:   "mock-video-analyzer-v1",
	}

	for _, scenes := range m.scenes {
		stats.TotalScenes += len(scenes)
	}

	for _, objects := range m.objects {
		stats.TotalObjects += len(objects)
	}

	for _, a := range m.analyses {
		stats.ProcessingHours += float64(a.ProcessingTimeMs) / 1000.0 / 3600.0
	}

	return stats
}
