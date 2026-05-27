// Package dataclassify provides automatic data classification and tagging for NAS-OS
// Features: AI-powered classification, auto-tagging, sensitivity detection, compliance
// Competitor benchmark: 对标群晖AI分类, 超越TrueNAS数据管理
package dataclassify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Classification represents data classification level
type Classification string

const (
	ClassPublic       Classification = "public"
	ClassInternal     Classification = "internal"
	ClassConfidential Classification = "confidential"
	ClassRestricted   Classification = "restricted"
	ClassTopSecret    Classification = "top_secret"
)

// SensitivityLevel represents data sensitivity
type SensitivityLevel string

const (
	SensitivityLow      SensitivityLevel = "low"
	SensitivityMedium   SensitivityLevel = "medium"
	SensitivityHigh     SensitivityLevel = "high"
	SensitivityCritical SensitivityLevel = "critical"
)

// DataType represents the type of data
type DataType string

const (
	DataTypeDocument DataType = "document"
	DataTypeImage    DataType = "image"
	DataTypeVideo    DataType = "video"
	DataTypeAudio    DataType = "audio"
	DataTypeCode     DataType = "code"
	DataTypeArchive  DataType = "archive"
	DataTypeDatabase DataType = "database"
	DataTypeFinancial DataType = "financial"
	DataTypeMedical  DataType = "medical"
	DataTypeLegal    DataType = "legal"
	DataTypePersonal DataType = "personal"
)

// PIIType represents types of PII detected
type PIIType string

const (
	PIIEmail    PIIType = "email"
	PIIPhone    PIIType = "phone"
	PIISSN      PIIType = "ssn"
	PIICreditCard PIIType = "credit_card"
	PIIPassport PIIType = "passport"
	PIIAddress  PIIType = "address"
	PIIName     PIIType = "name"
	PIIDOB      PIIType = "date_of_birth"
)

// ClassifiedFile represents a classified file
type ClassifiedFile struct {
	ID             string            `json:"id"`
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Size           int64             `json:"size"`
	MimeType       string            `json:"mime_type"`
	DataType       DataType          `json:"data_type"`
	Classification Classification    `json:"classification"`
	Sensitivity    SensitivityLevel  `json:"sensitivity"`
	Tags           []string          `json:"tags"`
	PIIDetected    []PIIDetection    `json:"pii_detected"`
	Keywords       []string          `json:"keywords"`
	Summary        string            `json:"summary"`
	Confidence     float64           `json:"confidence"`
	IndexedAt      time.Time         `json:"indexed_at"`
	ModifiedAt     time.Time         `json:"modified_at"`
	Metadata       map[string]string `json:"metadata"`
}

// PIIDetection represents detected PII
type PIIDetection struct {
	Type       PIIType `json:"type"`
	Value      string  `json:"value"` // Masked value
	Confidence float64 `json:"confidence"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
}

// ClassificationRule represents an auto-classification rule
type ClassificationRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Priority    int            `json:"priority"`
	Conditions  []Condition    `json:"conditions"`
	Tags        []string       `json:"tags"`
	Classification Classification `json:"classification"`
	Sensitivity SensitivityLevel `json:"sensitivity"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Condition represents a rule condition
type Condition struct {
	Field    string   `json:"field"`    // path, mime_type, size, content
	Operator string   `json:"operator"` // contains, matches, gt, lt, eq
	Values   []string `json:"values"`
}

// ScanConfig represents a scan configuration
type ScanConfig struct {
	Paths          []string `json:"paths"`
	ExcludePaths   []string `json:"exclude_paths"`
	FileTypes      []string `json:"file_types"`
	MaxFileSize    int64    `json:"max_file_size"`
	ScanContent    bool     `json:"scan_content"`
	AutoTag        bool     `json:"auto_tag"`
	AutoClassify   bool     `json:"auto_classify"`
	DetectPII      bool     `json:"detect_pii"`
	Concurrent     int      `json:"concurrent"`
}

// ScanResult represents scan results
type ScanResult struct {
	ScanID          string           `json:"scan_id"`
	StartTime       time.Time        `json:"start_time"`
	EndTime         time.Time        `json:"end_time"`
	FilesScanned    int              `json:"files_scanned"`
	FilesClassified int              `json:"files_classified"`
	PIIFound        int              `json:"pii_found"`
	TagsApplied     int              `json:"tags_applied"`
	ByClassification map[string]int  `json:"by_classification"`
	ByDataType      map[string]int   `json:"by_data_type"`
	BySensitivity   map[string]int   `json:"by_sensitivity"`
	TopTags         []TagCount       `json:"top_tags"`
	Errors          []string         `json:"errors"`
}

// TagCount represents a tag and its count
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ClassificationStats represents classification statistics
type ClassificationStats struct {
	TotalFiles       int                    `json:"total_files"`
	ByClassification map[string]int         `json:"by_classification"`
	ByDataType       map[string]int         `json:"by_data_type"`
	BySensitivity    map[string]int         `json:"by_sensitivity"`
	PIICount         int                    `json:"pii_count"`
	TopTags          []TagCount             `json:"top_tags"`
	LastScan         time.Time              `json:"last_scan"`
	ComplianceScore  float64                `json:"compliance_score"`
}

// Config holds data classification configuration
type Config struct {
	Enabled          bool    `json:"enabled"`
	AutoClassify     bool    `json:"auto_classify"`
	AutoTag          bool    `json:"auto_tag"`
	DetectPII        bool    `json:"detect_pii"`
	ScanInterval     int     `json:"scan_interval_hours"`
	MaxFileSize      int64   `json:"max_file_size"`
	ConcurrentScans  int     `json:"concurrent_scans"`
	DefaultClassification string `json:"default_classification"`
}

// Manager manages data classification
type Manager struct {
	config    *Config
	files     map[string]*ClassifiedFile
	rules     map[string]*ClassificationRule
	scans     []*ScanResult
	stats     *ClassificationStats
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new data classification manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config: config,
		files:  make(map[string]*ClassifiedFile),
		rules:  make(map[string]*ClassificationRule),
		scans:  make([]*ScanResult, 0),
		ctx:    ctx,
		cancel: cancel,
		stats: &ClassificationStats{
			ByClassification: make(map[string]int),
			ByDataType:       make(map[string]int),
			BySensitivity:    make(map[string]int),
		},
	}
}

// ClassifyFile classifies a file
func (m *Manager) ClassifyFile(file *ClassifiedFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file.IndexedAt = time.Now()
	m.files[file.ID] = file

	m.stats.TotalFiles = len(m.files)
	m.stats.ByClassification[string(file.Classification)]++
	m.stats.ByDataType[string(file.DataType)]++
	m.stats.BySensitivity[string(file.Sensitivity)]++

	if len(file.PIIDetected) > 0 {
		m.stats.PIICount++
	}

	return nil
}

// AddRule adds a classification rule
func (m *Manager) AddRule(rule *ClassificationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule.CreatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// GetFile returns classification info for a file
func (m *Manager) GetFile(id string) (*ClassifiedFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	file, exists := m.files[id]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", id)
	}
	return file, nil
}

// GetStats returns classification statistics
func (m *Manager) GetStats() *ClassificationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SearchByClassification finds files by classification level
func (m *Manager) SearchByClassification(class Classification) []*ClassifiedFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ClassifiedFile
	for _, f := range m.files {
		if f.Classification == class {
			results = append(results, f)
		}
	}
	return results
}

// SearchByTag finds files by tag
func (m *Manager) SearchByTag(tag string) []*ClassifiedFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ClassifiedFile
	for _, f := range m.files {
		for _, t := range f.Tags {
			if t == tag {
				results = append(results, f)
				break
			}
		}
	}
	return results
}

// SearchPII finds files containing PII
func (m *Manager) SearchPII() []*ClassifiedFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ClassifiedFile
	for _, f := range m.files {
		if len(f.PIIDetected) > 0 {
			results = append(results, f)
		}
	}
	return results
}

// Stop stops the classification manager
func (m *Manager) Stop() {
	m.cancel()
}
