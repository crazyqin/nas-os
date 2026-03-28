package photos

import (
	"time"
)

// AlbumRule defines a rule for conditional album generation
type AlbumRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        RuleType  `json:"type"`
	Conditions  []Condition `json:"conditions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AutoUpdate  bool      `json:"auto_update"`
	Active      bool      `json:"active"`
}

// RuleType defines the type of conditional album
type RuleType string

const (
	RuleTypePerson   RuleType = "person"   // 人物相册
	RuleTypeObject   RuleType = "object"   // 物体相册（猫、狗、车等）
	RuleTypeLocation RuleType = "location" // 地点相册
	RuleTypeTime     RuleType = "time"     // 时间相册（年/月/日）
	RuleTypeCamera   RuleType = "camera"   // 镜头相册（相机型号）
	RuleTypeCustom   RuleType = "custom"   // 自定义规则
)

// Condition defines a single condition for album matching
type Condition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// Operator defines comparison operators
type Operator string

const (
	OpEquals    Operator = "equals"
	OpContains  Operator = "contains"
	OpInRange   Operator = "in_range"
	OpBefore    Operator = "before"
	OpAfter     Operator = "after"
	OpMatches   Operator = "matches"
)

// ConditionalAlbumManager manages conditional albums
type ConditionalAlbumManager struct {
	rules     map[string]*AlbumRule
	albums    map[string]*ConditionalAlbum
	storage   AlbumStorage
	detector  FaceDetector
}

// ConditionalAlbum represents an auto-generated album
type ConditionalAlbum struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	Name        string    `json:"name"`
	Photos      []string  `json:"photos"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PhotoCount  int       `json:"photo_count"`
	CoverPhoto  string    `json:"cover_photo"`
}

// NewConditionalAlbumManager creates a new manager
func NewConditionalAlbumManager(storage AlbumStorage, detector FaceDetector) *ConditionalAlbumManager {
	return &ConditionalAlbumManager{
		rules:    make(map[string]*AlbumRule),
		albums:   make(map[string]*ConditionalAlbum),
		storage:  storage,
		detector: detector,
	}
}

// CreateRule creates a new album rule
func (m *ConditionalAlbumManager) CreateRule(rule *AlbumRule) error {
	rule.ID = generateID()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	
	// Generate initial album
	return m.UpdateAlbum(rule.ID)
}

// UpdateAlbum regenerates the album based on current rule
func (m *ConditionalAlbumManager) UpdateAlbum(ruleID string) error {
	rule, exists := m.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}
	
	// Find matching photos
	photos, err := m.findMatchingPhotos(rule)
	if err != nil {
		return err
	}
	
	// Create or update album
	album := &ConditionalAlbum{
		ID:         ruleID + "_album",
		RuleID:     ruleID,
		Name:       rule.Name,
		Photos:     photos,
		UpdatedAt:  time.Now(),
		PhotoCount: len(photos),
	}
	
	if len(photos) > 0 {
		album.CoverPhoto = photos[0]
	}
	
	if existing, exists := m.albums[album.ID]; exists {
		existing.Photos = photos
		existing.PhotoCount = len(photos)
		existing.UpdatedAt = time.Now()
	} else {
		album.CreatedAt = time.Now()
		m.albums[album.ID] = album
	}
	
	return m.storage.SaveAlbum(album)
}

// findMatchingPhotos finds all photos matching the rule conditions
func (m *ConditionalAlbumManager) findMatchingPhotos(rule *AlbumRule) ([]string, error) {
	allPhotos, err := m.storage.GetAllPhotos()
	if err != nil {
		return nil, err
	}
	
	var matching []string
	for _, photoID := range allPhotos {
		photo, err := m.storage.GetPhotoMetadata(photoID)
		if err != nil {
			continue
		}
		
		if m.matchesConditions(photo, rule.Conditions) {
			matching = append(matching, photoID)
		}
	}
	
	return matching, nil
}

// matchesConditions checks if photo matches all conditions
func (m *ConditionalAlbumManager) matchesConditions(photo *PhotoMetadata, conditions []Condition) bool {
	for _, cond := range conditions {
		if !m.matchCondition(photo, cond) {
			return false
		}
	}
	return true
}

// matchCondition checks a single condition
func (m *ConditionalAlbumManager) matchCondition(photo *PhotoMetadata, cond Condition) bool {
	switch cond.Field {
	case "person_id":
		// Check if person is in photo
		personID, ok := cond.Value.(string)
		if !ok {
			return false
		}
		for _, face := range photo.Faces {
			if face.PersonID == personID {
				return true
			}
		}
		return cond.Operator == OpEquals && len(photo.Faces) == 0
		
	case "location":
		return m.matchLocation(photo, cond)
		
	case "date_taken":
		return m.matchDate(photo, cond)
		
	case "camera":
		return m.matchCamera(photo, cond)
		
	case "object":
		return m.matchObject(photo, cond)
	}
	return false
}

func (m *ConditionalAlbumManager) matchLocation(photo *PhotoMetadata, cond Condition) bool {
	if photo.Location == nil {
		return false
	}
	switch cond.Operator {
	case OpEquals:
		return photo.Location.Name == cond.Value
	case OpContains:
		name, ok := cond.Value.(string)
		if !ok {
			return false
		}
		return contains(photo.Location.Name, name)
	}
	return false
}

func (m *ConditionalAlbumManager) matchDate(photo *PhotoMetadata, cond Condition) bool {
	if photo.DateTaken == nil {
		return false
	}
	switch cond.Operator {
	case OpInRange:
		rangeVal, ok := cond.Value.([]string)
		if !ok || len(rangeVal) != 2 {
			return false
		}
		start, _ := time.Parse("2006-01-02", rangeVal[0])
		end, _ := time.Parse("2006-01-02", rangeVal[1])
		return photo.DateTaken.After(start) && photo.DateTaken.Before(end)
	case OpBefore:
		date, _ := time.Parse("2006-01-02", cond.Value.(string))
		return photo.DateTaken.Before(date)
	case OpAfter:
		date, _ := time.Parse("2006-01-02", cond.Value.(string))
		return photo.DateTaken.After(date)
	}
	return false
}

func (m *ConditionalAlbumManager) matchCamera(photo *PhotoMetadata, cond Condition) bool {
	if photo.Camera == nil {
		return false
	}
	switch cond.Operator {
	case OpEquals:
		return photo.Camera.Model == cond.Value
	case OpContains:
		model, ok := cond.Value.(string)
		if !ok {
			return false
		}
		return contains(photo.Camera.Model, model)
	}
	return false
}

func (m *ConditionalAlbumManager) matchObject(photo *PhotoMetadata, cond Condition) bool {
	for _, obj := range photo.Objects {
		if obj.Label == cond.Value {
			return true
		}
	}
	return false
}

// GetAlbums returns all conditional albums
func (m *ConditionalAlbumManager) GetAlbums() []*ConditionalAlbum {
	result := make([]*ConditionalAlbum, 0, len(m.albums))
	for _, album := range m.albums {
		result = append(result, album)
	}
	return result
}

// GetRules returns all rules
func (m *ConditionalAlbumManager) GetRules() []*AlbumRule {
	result := make([]*AlbumRule, 0, len(m.rules))
	for _, rule := range m.rules {
		result = append(result, rule)
	}
	return result
}

// DeleteRule deletes a rule and its album
func (m *ConditionalAlbumManager) DeleteRule(ruleID string) error {
	delete(m.rules, ruleID)
	
	// Delete associated album
	for albumID, album := range m.albums {
		if album.RuleID == ruleID {
			delete(m.albums, albumID)
			m.storage.DeleteAlbum(albumID)
		}
	}
	return nil
}

// Helper functions
func generateID() string {
	return time.Now().Format("20060102") + "_" + randomString(8)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func randomString(n int) string {
	// Simple implementation for now
	return "xxxxxxxx"
}

// Errors
var (
	ErrRuleNotFound = errors.New("rule not found")
)

import "errors"