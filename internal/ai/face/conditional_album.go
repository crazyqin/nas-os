// Package face provides AI-powered face recognition for photo management
package face

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ConditionalAlbumRule defines rules for automatic album generation
type ConditionalAlbumRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Conditions  []Condition       `json:"conditions"`
	MatchMode   MatchMode         `json:"matchMode"` // all, any
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Condition represents a single condition for album generation
type Condition struct {
	Type      ConditionType `json:"type"`      // person, object, location, date, camera
	Field     string        `json:"field"`     // specific field name
	Operator  Operator      `json:"operator"`  // equals, contains, between, greaterThan, lessThan
	Value     interface{}   `json:"value"`     // condition value
	ValueEnd  interface{}   `json:"valueEnd"`  // for range conditions
	Weight    float64       `json:"weight"`    // confidence weight (0-1)
}

// ConditionType defines the type of condition
type ConditionType string

const (
	ConditionPerson   ConditionType = "person"    // Face cluster/person
	ConditionObject   ConditionType = "object"    // Detected object
	ConditionLocation ConditionType = "location"  // GPS or place name
	ConditionDate     ConditionType = "date"      // Date range
	ConditionCamera   ConditionType = "camera"    // Camera model/lens
	ConditionTag      ConditionType = "tag"       // User-defined tag
	ConditionRating   ConditionType = "rating"    // Photo rating
)

// Operator defines comparison operators
type Operator string

const (
	OpEquals     Operator = "equals"
	OpContains   Operator = "contains"
	OpBetween    Operator = "between"
	OpGreaterThan Operator = "greaterThan"
	OpLessThan   Operator = "lessThan"
	OpNotEquals  Operator = "notEquals"
)

// MatchMode defines how conditions are combined
type MatchMode string

const (
	MatchAll MatchMode = "all" // All conditions must match
	MatchAny MatchMode = "any" // Any condition can match
)

// ConditionalAlbum represents an auto-generated album
type ConditionalAlbum struct {
	ID           string                   `json:"id"`
	RuleID       string                   `json:"ruleId"`
	Name         string                   `json:"name"`
	PhotoCount   int                      `json:"photoCount"`
	CoverPhotoID string                  `json:"coverPhotoId,omitempty"`
	LastUpdated  time.Time                `json:"lastUpdated"`
	Photos       []ConditionalAlbumPhoto  `json:"photos,omitempty"`
}

// ConditionalAlbumPhoto represents a photo in a conditional album
type ConditionalAlbumPhoto struct {
	PhotoID    string    `json:"photoId"`
	Thumbnail  string    `json:"thumbnail,omitempty"`
	MatchScore float64   `json:"matchScore"` // How well it matches conditions
	AddedAt    time.Time `json:"addedAt"`
}

// ConditionalAlbumManager manages conditional album rules and generation
type ConditionalAlbumManager struct {
	mu          sync.RWMutex
	rules       map[string]*ConditionalAlbumRule
	albums      map[string]*ConditionalAlbum
	detector    *Detector
	storage     PhotoStorage
	indexer     PhotoIndexer
}

// PhotoStorage defines the interface for photo storage operations
type PhotoStorage interface {
	GetPhoto(ctx context.Context, id string) (*Photo, error)
	ListPhotos(ctx context.Context, filter PhotoFilter) ([]*Photo, error)
	GetPhotoMetadata(ctx context.Context, id string) (*PhotoMetadata, error)
}

// PhotoIndexer defines the interface for photo indexing
type PhotoIndexer interface {
	IndexPhoto(ctx context.Context, photo *Photo) error
	GetIndexedPhotos(ctx context.Context, conditions []Condition) ([]*IndexedPhoto, error)
}

// Photo represents a photo in the system
type Photo struct {
	ID         string            `json:"id"`
	Path       string            `json:"path"`
	Filename   string            `json:"filename"`
	Size       int64             `json:"size"`
	CreatedAt  time.Time         `json:"createdAt"`
	ModifiedAt time.Time         `json:"modifiedAt"`
	Metadata   *PhotoMetadata    `json:"metadata,omitempty"`
}

// PhotoMetadata contains extracted photo metadata
type PhotoMetadata struct {
	DateTaken    time.Time         `json:"dateTaken,omitempty"`
	CameraModel  string            `json:"cameraModel,omitempty"`
	Lens         string            `json:"lens,omitempty"`
	Location     *Location         `json:"location,omitempty"`
	Persons      []DetectedPerson  `json:"persons,omitempty"`
	Objects      []DetectedObject  `json:"objects,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Rating       int               `json:"rating,omitempty"`
	Width        int               `json:"width,omitempty"`
	Height       int               `json:"height,omitempty"`
}

// Location represents GPS coordinates and place name
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
}

// DetectedPerson represents a detected face/person
type DetectedPerson struct {
	ClusterID   string  `json:"clusterId"`
	Name        string  `json:"name,omitempty"`
	Confidence  float64 `json:"confidence"`
	BoundingBox []int   `json:"boundingBox"` // [x, y, width, height]
}

// DetectedObject represents a detected object
type DetectedObject struct {
	Label       string  `json:"label"`
	Confidence  float64 `json:"confidence"`
	BoundingBox []int   `json:"boundingBox"`
}

// IndexedPhoto represents a photo in the search index
type IndexedPhoto struct {
	PhotoID     string                 `json:"photoId"`
	Vectors     map[string][]float32   `json:"vectors,omitempty"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// PhotoFilter for listing photos
type PhotoFilter struct {
	StartDate  *time.Time `json:"startDate,omitempty"`
	EndDate    *time.Time `json:"endDate,omitempty"`
	Location   *Location  `json:"location,omitempty"`
	PersonID   string     `json:"personId,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// NewConditionalAlbumManager creates a new manager instance
func NewConditionalAlbumManager(detector *Detector, storage PhotoStorage, indexer PhotoIndexer) *ConditionalAlbumManager {
	return &ConditionalAlbumManager{
		rules:    make(map[string]*ConditionalAlbumRule),
		albums:   make(map[string]*ConditionalAlbum),
		detector: detector,
		storage:  storage,
		indexer:  indexer,
	}
}

// CreateRule creates a new conditional album rule
func (m *ConditionalAlbumManager) CreateRule(ctx context.Context, rule *ConditionalAlbumRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID("rule")
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	// Trigger album generation if enabled
	if rule.Enabled {
		go m.generateAlbum(context.Background(), rule.ID)
	}

	return nil
}

// UpdateRule updates an existing rule
func (m *ConditionalAlbumManager) UpdateRule(ctx context.Context, rule *ConditionalAlbumRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.rules[rule.ID]
	if !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule

	// Regenerate album if enabled
	if rule.Enabled {
		go m.generateAlbum(context.Background(), rule.ID)
	}

	return nil
}

// DeleteRule deletes a rule and its associated album
func (m *ConditionalAlbumManager) DeleteRule(ctx context.Context, ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rules, ruleID)

	// Delete associated album
	for id, album := range m.albums {
		if album.RuleID == ruleID {
			delete(m.albums, id)
		}
	}

	return nil
}

// GetRule retrieves a rule by ID
func (m *ConditionalAlbumManager) GetRule(ctx context.Context, ruleID string) (*ConditionalAlbumRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", ruleID)
	}
	return rule, nil
}

// ListRules lists all rules
func (m *ConditionalAlbumManager) ListRules(ctx context.Context) ([]*ConditionalAlbumRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*ConditionalAlbumRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})

	return rules, nil
}

// GetAlbum retrieves the album generated by a rule
func (m *ConditionalAlbumManager) GetAlbum(ctx context.Context, ruleID string) (*ConditionalAlbum, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, album := range m.albums {
		if album.RuleID == ruleID {
			return album, nil
		}
	}

	return nil, fmt.Errorf("album not found for rule: %s", ruleID)
}

// RegenerateAllAlbums regenerates all conditional albums
func (m *ConditionalAlbumManager) RegenerateAllAlbums(ctx context.Context) error {
	m.mu.RLock()
	ruleIDs := make([]string, 0, len(m.rules))
	for id, rule := range m.rules {
		if rule.Enabled {
			ruleIDs = append(ruleIDs, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range ruleIDs {
		if err := m.generateAlbum(ctx, id); err != nil {
			// Log error but continue with other albums
			continue
		}
	}

	return nil
}

// generateAlbum generates an album based on rule conditions
func (m *ConditionalAlbumManager) generateAlbum(ctx context.Context, ruleID string) error {
	m.mu.RLock()
	rule, ok := m.rules[ruleID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("rule not found: %s", ruleID)
	}
	m.mu.RUnlock()

	// Get photos matching conditions
	indexedPhotos, err := m.indexer.GetIndexedPhotos(ctx, rule.Conditions)
	if err != nil {
		return fmt.Errorf("failed to get indexed photos: %w", err)
	}

	// Filter and score photos
	albumPhotos := make([]ConditionalAlbumPhoto, 0)
	for _, ip := range indexedPhotos {
		score := m.calculateMatchScore(ip, rule.Conditions, rule.MatchMode)
		if score > 0 {
			albumPhotos = append(albumPhotos, ConditionalAlbumPhoto{
				PhotoID:    ip.PhotoID,
				MatchScore: score,
				AddedAt:    time.Now(),
			})
		}
	}

	// Sort by match score
	sort.Slice(albumPhotos, func(i, j int) bool {
		return albumPhotos[i].MatchScore > albumPhotos[j].MatchScore
	})

	// Create or update album
	m.mu.Lock()
	defer m.mu.Unlock()

	albumID := "album_" + ruleID
	album := &ConditionalAlbum{
		ID:          albumID,
		RuleID:      ruleID,
		Name:        rule.Name,
		PhotoCount:  len(albumPhotos),
		LastUpdated: time.Now(),
		Photos:      albumPhotos,
	}

	if len(albumPhotos) > 0 {
		album.CoverPhotoID = albumPhotos[0].PhotoID
	}

	m.albums[albumID] = album

	return nil
}

// calculateMatchScore calculates how well a photo matches the conditions
func (m *ConditionalAlbumManager) calculateMatchScore(photo *IndexedPhoto, conditions []Condition, mode MatchMode) float64 {
	if len(conditions) == 0 {
		return 0
	}

	var totalScore float64
	var matchCount int

	for _, cond := range conditions {
		score := m.evaluateCondition(photo, cond)
		if score > 0 {
			matchCount++
			totalScore += score * cond.Weight
		}
	}

	if mode == MatchAll {
		if matchCount != len(conditions) {
			return 0
		}
		return totalScore / float64(len(conditions))
	}

	// MatchAny
	if matchCount == 0 {
		return 0
	}
	return totalScore / float64(matchCount)
}

// evaluateCondition evaluates a single condition against a photo
func (m *ConditionalAlbumManager) evaluateCondition(photo *IndexedPhoto, cond Condition) float64 {
	attrValue, ok := photo.Attributes[string(cond.Type)]
	if !ok {
		return 0
	}

	switch cond.Operator {
	case OpEquals:
		if attrValue == cond.Value {
			return 1.0
		}
	case OpContains:
		if str, ok := attrValue.(string); ok {
			if target, ok := cond.Value.(string); ok {
				if contains(str, target) {
					return 1.0
				}
			}
		}
	case OpBetween:
		// Handle numeric or date ranges
		if num, ok := attrValue.(float64); ok {
			start, ok1 := cond.Value.(float64)
			end, ok2 := cond.ValueEnd.(float64)
			if ok1 && ok2 && num >= start && num <= end {
				return 1.0
			}
		}
	case OpGreaterThan:
		if num, ok := attrValue.(float64); ok {
			if target, ok := cond.Value.(float64); ok && num > target {
				return 1.0
			}
		}
	case OpLessThan:
		if num, ok := attrValue.(float64); ok {
			if target, ok := cond.Value.(float64); ok && num < target {
				return 1.0
			}
		}
	case OpNotEquals:
		if attrValue != cond.Value {
			return 1.0
		}
	}

	return 0
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		(str == substr || len(str) > 0 && containsIgnoreCase(str, substr))
}

func containsIgnoreCase(str, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := str[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// generateID generates a unique ID with prefix
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}