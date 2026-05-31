// Package aivideounderstand 提供 AI 视频理解功能.
package aivideounderstand

import "time"

// VideoAnalysis 视频分析结果.
type VideoAnalysis struct {
	ID               string    `json:"id"`
	VideoPath        string    `json:"video_path"`
	Status           string    `json:"status"` // pending/processing/completed/failed
	Duration         float64   `json:"duration"`
	Resolution       string    `json:"resolution"`
	FPS              float64   `json:"fps"`
	Codec            string    `json:"codec"`
	FileSize         int64     `json:"file_size"`
	AnalyzedAt       time.Time `json:"analyzed_at"`
	ProcessingTimeMs int64     `json:"processing_time_ms"`
}

// Scene 视频场景.
type Scene struct {
	ID           string    `json:"id"`
	AnalysisID   string    `json:"analysis_id"`
	StartTime    float64   `json:"start_time"`
	EndTime      float64   `json:"end_time"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	Confidence   float64   `json:"confidence"`
	ThumbnailPath string   `json:"thumbnail_path,omitempty"`
	SceneType    string    `json:"scene_type"` // action/dialogue/transition/landscape/indoor/outdoor
}

// DetectedObject 检测到的物体.
type DetectedObject struct {
	ID           string  `json:"id"`
	AnalysisID   string  `json:"analysis_id"`
	Label        string  `json:"label"`
	Confidence   float64 `json:"confidence"`
	BoundingBox  BoundingBox `json:"bounding_box"`
	FirstSeen    float64 `json:"first_seen"`
	LastSeen     float64 `json:"last_seen"`
	TrackID      int     `json:"track_id"`
}

// BoundingBox 边界框.
type BoundingBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// VideoSearchQuery 视频搜索查询.
type VideoSearchQuery struct {
	Query        string   `json:"query"`
	MinConfidence float64  `json:"min_confidence,omitempty"`
	SceneTypes   []string `json:"scene_types,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	DateFrom     *time.Time `json:"date_from,omitempty"`
	DateTo       *time.Time `json:"date_to,omitempty"`
	MaxResults   int      `json:"max_results,omitempty"`
}

// VideoSearchResult 视频搜索结果.
type VideoSearchResult struct {
	VideoPath     string   `json:"video_path"`
	MatchingScenes []Scene `json:"matching_scenes"`
	TotalMatches  int      `json:"total_matches"`
	RelevanceScore float64 `json:"relevance_score"`
}

// VideoHighlight 视频高光时刻.
type VideoHighlight struct {
	ID           string  `json:"id"`
	AnalysisID   string  `json:"analysis_id"`
	StartTime    float64 `json:"start_time"`
	EndTime      float64 `json:"end_time"`
	Reason       string  `json:"reason"`
	Score        float64 `json:"score"`
	ThumbnailPath string `json:"thumbnail_path,omitempty"`
}

// AnalysisStats 分析统计信息.
type AnalysisStats struct {
	TotalVideos     int     `json:"total_videos"`
	TotalScenes     int     `json:"total_scenes"`
	TotalObjects    int     `json:"total_objects"`
	ProcessingHours float64 `json:"processing_hours"`
	ModelName       string  `json:"model_name"`
}
