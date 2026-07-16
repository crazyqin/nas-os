package featureroadmap

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var featureCounter int64

// Feature 功能特性.
type Feature struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // storage, security, ai, network, ui, devops
	Status      string    `json:"status"`   // planned, in_progress, testing, released, deprecated
	Priority    string    `json:"priority"` // critical, high, medium, low
	Version     string    `json:"version"`
	Assignee    string    `json:"assignee"` // 兵部, 户部, 礼部, 工部, 吏部, 刑部
	Progress    int       `json:"progress"` // 0-100
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Tags        []string  `json:"tags"`
}

// Milestone 里程碑.
type Milestone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	TargetDate  time.Time `json:"targetDate"`
	Status      string    `json:"status"`   // upcoming, active, completed
	Features    []string  `json:"features"` // feature IDs
	Progress    int       `json:"progress"`
}

// RoadmapStats 路线图统计.
type RoadmapStats struct {
	TotalFeatures   int            `json:"totalFeatures"`
	ByStatus        map[string]int `json:"byStatus"`
	ByPriority      map[string]int `json:"byPriority"`
	ByAssignee      map[string]int `json:"byAssignee"`
	OverallProgress float64        `json:"overallProgress"`
}

// FeatureRoadmap 功能路线图管理.
type FeatureRoadmap struct {
	mu         sync.RWMutex
	features   map[string]*Feature
	milestones map[string]*Milestone
	stopCh     chan struct{}
	running    bool
}

// NewFeatureRoadmap 创建路线图.
func NewFeatureRoadmap() *FeatureRoadmap {
	return &FeatureRoadmap{
		features:   make(map[string]*Feature),
		milestones: make(map[string]*Milestone),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动.
func (f *FeatureRoadmap) Start() {
	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return
	}
	f.running = true
	f.mu.Unlock()
	log.Println("[FeatureRoadmap] 功能路线图已启动")
}

// Stop 停止.
func (f *FeatureRoadmap) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.running {
		return
	}
	close(f.stopCh)
	f.running = false
}

// AddFeature 添加功能.
func (f *FeatureRoadmap) AddFeature(feature Feature) *Feature {
	f.mu.Lock()
	defer f.mu.Unlock()
	feature.ID = fmt.Sprintf("feat-%s-%04d", time.Now().Format("20060102150405"), atomic.AddInt64(&featureCounter, 1))
	feature.CreatedAt = time.Now()
	feature.UpdatedAt = time.Now()
	if feature.Status == "" {
		feature.Status = "planned"
	}
	f.features[feature.ID] = &feature
	return &feature
}

// UpdateFeature 更新功能.
func (f *FeatureRoadmap) UpdateFeature(id string, updates map[string]interface{}) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	feat, ok := f.features[id]
	if !ok {
		return false
	}
	if v, ok := updates["status"].(string); ok {
		feat.Status = v
	}
	if v, ok := updates["progress"].(float64); ok {
		feat.Progress = int(v)
	}
	if v, ok := updates["priority"].(string); ok {
		feat.Priority = v
	}
	if v, ok := updates["assignee"].(string); ok {
		feat.Assignee = v
	}
	feat.UpdatedAt = time.Now()
	return true
}

// AddMilestone 添加里程碑.
func (f *FeatureRoadmap) AddMilestone(ms Milestone) *Milestone {
	f.mu.Lock()
	defer f.mu.Unlock()
	ms.ID = fmt.Sprintf("ms-%s-%04d", time.Now().Format("20060102150405"), atomic.AddInt64(&featureCounter, 1))
	if ms.Status == "" {
		ms.Status = "upcoming"
	}
	f.milestones[ms.ID] = &ms
	return &ms
}

// GetFeatures 获取功能列表.
func (f *FeatureRoadmap) GetFeatures() []Feature {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]Feature, 0, len(f.features))
	for _, feat := range f.features {
		result = append(result, *feat)
	}
	sort.Slice(result, func(i, j int) bool {
		return priorityOrder(result[i].Priority) < priorityOrder(result[j].Priority)
	})
	return result
}

// GetMilestones 获取里程碑列表.
func (f *FeatureRoadmap) GetMilestones() []Milestone {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]Milestone, 0, len(f.milestones))
	for _, ms := range f.milestones {
		result = append(result, *ms)
	}
	return result
}

// GetStats 获取统计.
func (f *FeatureRoadmap) GetStats() RoadmapStats {
	f.mu.RLock()
	defer f.mu.RUnlock()
	stats := RoadmapStats{
		TotalFeatures: len(f.features),
		ByStatus:      make(map[string]int),
		ByPriority:    make(map[string]int),
		ByAssignee:    make(map[string]int),
	}
	totalProgress := 0
	for _, feat := range f.features {
		stats.ByStatus[feat.Status]++
		stats.ByPriority[feat.Priority]++
		if feat.Assignee != "" {
			stats.ByAssignee[feat.Assignee]++
		}
		totalProgress += feat.Progress
	}
	if stats.TotalFeatures > 0 {
		stats.OverallProgress = float64(totalProgress) / float64(stats.TotalFeatures)
	}
	return stats
}

func priorityOrder(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
