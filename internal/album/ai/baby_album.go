// Package ai - Baby album with face growth tracking
package ai

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"
	"time"
)

// BabyAlbumManager manages baby growth tracking albums
type BabyAlbumManager struct {
	faceRecognizer *FaceRecognizer
	albums         map[string]*BabyAlbum
	faceHistory    map[string]*FaceGrowthTracker // baby_id -> tracker
	mu             sync.RWMutex
	storage        BabyAlbumStorage
}

// BabyAlbumStorage interface for persisting baby albums
type BabyAlbumStorage interface {
	SaveBabyAlbum(album *BabyAlbum) error
	LoadBabyAlbum(id string) (*BabyAlbum, error)
	ListBabyAlbums() ([]*BabyAlbum, error)
	DeleteBabyAlbum(id string) error

	SaveFaceGrowthTracker(tracker *FaceGrowthTracker) error
	LoadFaceGrowthTracker(babyID string) (*FaceGrowthTracker, error)
}

// NewBabyAlbumManager creates a new baby album manager
func NewBabyAlbumManager(faceRecognizer *FaceRecognizer, storage BabyAlbumStorage) (*BabyAlbumManager, error) {
	bam := &BabyAlbumManager{
		faceRecognizer: faceRecognizer,
		albums:         make(map[string]*BabyAlbum),
		faceHistory:    make(map[string]*FaceGrowthTracker),
		storage:        storage,
	}

	// Load existing albums
	if storage != nil {
		albums, err := storage.ListBabyAlbums()
		if err == nil {
			for _, album := range albums {
				bam.albums[album.ID] = album
			}
		}
	}

	return bam, nil
}

// CreateBabyAlbum creates a new baby album
func (bam *BabyAlbumManager) CreateBabyAlbum(ctx context.Context, name string, birthDate time.Time, gender string) (*BabyAlbum, error) {
	bam.mu.Lock()
	defer bam.mu.Unlock()

	album := &BabyAlbum{
		ID:        generateID("baby"),
		Name:      name,
		BirthDate: birthDate,
		Gender:    gender,
		Milestones: make([]Milestone, 0),
		GrowthPhotos: make([]GrowthPhoto, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	bam.albums[album.ID] = album
	bam.faceHistory[album.ID] = &FaceGrowthTracker{
		BabyID:      album.ID,
		FaceHistory: make([]FaceRecord, 0),
		GrowthPoints: make([]GrowthPoint, 0),
	}

	if bam.storage != nil {
		if err := bam.storage.SaveBabyAlbum(album); err != nil {
			delete(bam.albums, album.ID)
			return nil, fmt.Errorf("failed to save baby album: %w", err)
		}
	}

	return album, nil
}

// AddPhotoToBabyAlbum adds a photo to a baby album with face tracking
func (bam *BabyAlbumManager) AddPhotoToBabyAlbum(ctx context.Context, babyID string, photoID string, img image.Image, faceID string, takenAt time.Time) error {
	bam.mu.Lock()
	defer bam.mu.Unlock()

	album, exists := bam.albums[babyID]
	if !exists {
		return fmt.Errorf("baby album not found: %s", babyID)
	}

	// Calculate age
	ageMonths, ageDays := calculateAge(album.BirthDate, takenAt)

	// Create growth photo entry
	growthPhoto := GrowthPhoto{
		PhotoID:   photoID,
		Date:      takenAt,
		AgeMonths: ageMonths,
		AgeDays:   ageDays,
		FaceID:    faceID,
	}

	// Extract face embedding for growth tracking
	if bam.faceRecognizer != nil && faceID != "" {
		faces, err := bam.faceRecognizer.DetectFaces(ctx, img)
		if err == nil && len(faces) > 0 {
			// Find the matching face
			for _, face := range faces {
				if face.ID == faceID {
					embedding, err := bam.faceRecognizer.ExtractEmbedding(ctx, img, face)
					if err == nil {
						faceRecord := FaceRecord{
							FaceID:      face.ID,
							PhotoID:     photoID,
							Date:        takenAt,
							AgeMonths:   ageMonths,
							Embedding:   embedding,
							BoundingBox: face.BoundingBox,
						}

						// Add to face history
						tracker := bam.faceHistory[babyID]
						if tracker == nil {
							tracker = &FaceGrowthTracker{
								BabyID:      babyID,
								FaceHistory: make([]FaceRecord, 0),
								GrowthPoints: make([]GrowthPoint, 0),
							}
							bam.faceHistory[babyID] = tracker
						}
						tracker.FaceHistory = append(tracker.FaceHistory, faceRecord)

						// Calculate growth metrics
						growthPoints := bam.calculateGrowthMetrics(tracker.FaceHistory)
						tracker.GrowthPoints = growthPoints

						if bam.storage != nil {
							bam.storage.SaveFaceGrowthTracker(tracker)
						}
					}
					break
				}
			}
		}
	}

	// Add to growth photos
	album.GrowthPhotos = append(album.GrowthPhotos, growthPhoto)
	album.UpdatedAt = time.Now()

	// Set cover photo if first photo
	if album.CoverPhotoID == "" {
		album.CoverPhotoID = photoID
	}

	if bam.storage != nil {
		return bam.storage.SaveBabyAlbum(album)
	}

	return nil
}

// AddMilestone adds a milestone to a baby album
func (bam *BabyAlbumManager) AddMilestone(ctx context.Context, babyID string, milestoneType string, date time.Time, photoID string, description string) (*Milestone, error) {
	bam.mu.Lock()
	defer bam.mu.Unlock()

	album, exists := bam.albums[babyID]
	if !exists {
		return nil, fmt.Errorf("baby album not found: %s", babyID)
	}

	ageMonths, _ := calculateAge(album.BirthDate, date)

	milestone := Milestone{
		ID:          generateID("milestone"),
		Type:        milestoneType,
		Date:        date,
		PhotoID:     photoID,
		Description: description,
		AgeMonths:   ageMonths,
	}

	album.Milestones = append(album.Milestones, milestone)
	album.UpdatedAt = time.Now()

	if bam.storage != nil {
		if err := bam.storage.SaveBabyAlbum(album); err != nil {
			return nil, err
		}
	}

	return &milestone, nil
}

// GetBabyAlbum retrieves a baby album
func (bam *BabyAlbumManager) GetBabyAlbum(babyID string) (*BabyAlbum, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	album, exists := bam.albums[babyID]
	if !exists {
		return nil, fmt.Errorf("baby album not found: %s", babyID)
	}

	return album, nil
}

// ListBabyAlbums lists all baby albums
func (bam *BabyAlbumManager) ListBabyAlbums() []*BabyAlbum {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	albums := make([]*BabyAlbum, 0, len(bam.albums))
	for _, album := range bam.albums {
		albums = append(albums, album)
	}

	// Sort by creation date
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].CreatedAt.After(albums[j].CreatedAt)
	})

	return albums
}

// DeleteBabyAlbum deletes a baby album
func (bam *BabyAlbumManager) DeleteBabyAlbum(babyID string) error {
	bam.mu.Lock()
	defer bam.mu.Unlock()

	if _, exists := bam.albums[babyID]; !exists {
		return fmt.Errorf("baby album not found: %s", babyID)
	}

	delete(bam.albums, babyID)
	delete(bam.faceHistory, babyID)

	if bam.storage != nil {
		return bam.storage.DeleteBabyAlbum(babyID)
	}

	return nil
}

// GetTimeline generates a timeline of baby growth
func (bam *BabyAlbumManager) GetTimeline(babyID string) ([]TimelineEntry, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	album, exists := bam.albums[babyID]
	if !exists {
		return nil, fmt.Errorf("baby album not found: %s", babyID)
	}

	// Combine milestones and growth photos into timeline
	entries := make([]TimelineEntry, 0)

	// Add milestones
	for _, m := range album.Milestones {
		entries = append(entries, TimelineEntry{
			Date:      m.Date,
			Type:      "milestone",
			Title:     milestoneTitle(m.Type),
			PhotoID:   m.PhotoID,
			AgeMonths: m.AgeMonths,
			Content:   m.Description,
		})
	}

	// Add growth photos (one per month)
	monthPhotos := make(map[int]GrowthPhoto)
	for _, gp := range album.GrowthPhotos {
		if existing, ok := monthPhotos[gp.AgeMonths]; !ok || gp.Date.Before(existing.Date) {
			monthPhotos[gp.AgeMonths] = gp
		}
	}

	for _, gp := range monthPhotos {
		entries = append(entries, TimelineEntry{
			Date:      gp.Date,
			Type:      "photo",
			Title:     formatAge(gp.AgeMonths, gp.AgeDays),
			PhotoID:   gp.PhotoID,
			AgeMonths: gp.AgeMonths,
		})
	}

	// Sort by date
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date.Before(entries[j].Date)
	})

	return entries, nil
}

// TimelineEntry represents a timeline entry
type TimelineEntry struct {
	Date      time.Time `json:"date"`
	Type      string    `json:"type"` // "milestone", "photo"
	Title     string    `json:"title"`
	PhotoID   string    `json:"photo_id,omitempty"`
	AgeMonths int       `json:"age_months"`
	Content   string    `json:"content,omitempty"`
}

// GetGrowthReport generates a growth report for the baby
func (bam *BabyAlbumManager) GetGrowthReport(babyID string) (*GrowthReport, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	album, exists := bam.albums[babyID]
	if !exists {
		return nil, fmt.Errorf("baby album not found: %s", babyID)
	}

	report := &GrowthReport{
		BabyID:     babyID,
		Name:       album.Name,
		TotalPhotos: len(album.GrowthPhotos),
		TotalMilestones: len(album.Milestones),
	}

	// Calculate age
	now := time.Now()
	report.CurrentAgeMonths, report.CurrentAgeDays = calculateAge(album.BirthDate, now)

	// Get growth tracker
	if tracker, exists := bam.faceHistory[babyID]; exists {
		report.FaceRecords = len(tracker.FaceHistory)

		// Calculate face growth trend
		if len(tracker.FaceHistory) >= 2 {
			report.GrowthTrend = bam.calculateGrowthTrend(tracker.FaceHistory)
		}
	}

	// Milestone statistics
	report.MilestonesByType = make(map[string]int)
	for _, m := range album.Milestones {
		report.MilestonesByType[m.Type]++
	}

	return report, nil
}

// GrowthReport represents a baby growth report
type GrowthReport struct {
	BabyID            string         `json:"baby_id"`
	Name              string         `json:"name"`
	CurrentAgeMonths  int            `json:"current_age_months"`
	CurrentAgeDays    int            `json:"current_age_days"`
	TotalPhotos       int            `json:"total_photos"`
	TotalMilestones   int            `json:"total_milestones"`
	FaceRecords       int            `json:"face_records"`
	GrowthTrend       []GrowthPoint  `json:"growth_trend"`
	MilestonesByType  map[string]int `json:"milestones_by_type"`
}

// calculateAge calculates age in months and days
func calculateAge(birthDate, date time.Time) (int, int) {
	if date.Before(birthDate) {
		return 0, 0
	}

	years := date.Year() - birthDate.Year()
	months := int(date.Month() - birthDate.Month())
	days := date.Day() - birthDate.Day()

	if days < 0 {
		months--
		// Get days in previous month
		prevMonth := date.AddDate(0, -1, 0)
		days += daysInMonth(prevMonth.Year(), prevMonth.Month())
	}

	if months < 0 {
		years--
		months += 12
	}

	totalMonths := years*12 + months

	return totalMonths, days
}

// daysInMonth returns the number of days in a month
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// formatAge formats age as a human-readable string
func formatAge(months, days int) string {
	if months == 0 {
		return fmt.Sprintf("%d days old", days)
	}
	if months < 12 {
		return fmt.Sprintf("%d months old", months)
	}
	years := months / 12
	remainingMonths := months % 12
	if remainingMonths == 0 {
		return fmt.Sprintf("%d years old", years)
	}
	return fmt.Sprintf("%d years %d months old", years, remainingMonths)
}

// milestoneTitle returns a title for milestone type
func milestoneTitle(milestoneType string) string {
	titles := map[string]string{
		"first_smile":    "First Smile",
		"first_step":     "First Steps",
		"first_word":     "First Word",
		"first_tooth":    "First Tooth",
		"first_crawl":    "Started Crawling",
		"first_solid":    "First Solid Food",
		"first_haircut":  "First Haircut",
		"first_birthday": "First Birthday",
		"first_day":      "First Day",
	}

	if title, ok := titles[milestoneType]; ok {
		return title
	}
	return milestoneType
}

// calculateGrowthMetrics calculates growth metrics from face records
func (bam *BabyAlbumManager) calculateGrowthMetrics(history []FaceRecord) []GrowthPoint {
	if len(history) < 2 {
		return nil
	}

	// Sort by date
	sorted := make([]FaceRecord, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	points := make([]GrowthPoint, 0)

	// Calculate face width ratio over time
	for i, record := range sorted {
		// Face width from bounding box
		faceWidth := record.BoundingBox.Width

		points = append(points, GrowthPoint{
			Date:      record.Date,
			AgeMonths: record.AgeMonths,
			Metric:    "face_width",
			Value:     faceWidth,
		})

		// Compare with first record
		if i > 0 {
			growth := (faceWidth - sorted[0].BoundingBox.Width) / sorted[0].BoundingBox.Width * 100
			points = append(points, GrowthPoint{
				Date:      record.Date,
				AgeMonths: record.AgeMonths,
				Metric:    "face_growth_percent",
				Value:     growth,
			})
		}
	}

	return points
}

// calculateGrowthTrend calculates face growth trend
func (bam *BabyAlbumManager) calculateGrowthTrend(history []FaceRecord) []GrowthPoint {
	// Sort by age
	sorted := make([]FaceRecord, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AgeMonths < sorted[j].AgeMonths
	})

	// Group by month and calculate average face size
	monthSizes := make(map[int][]float64)
	for _, record := range sorted {
		size := record.BoundingBox.Width * record.BoundingBox.Height
		monthSizes[record.AgeMonths] = append(monthSizes[record.AgeMonths], size)
	}

	points := make([]GrowthPoint, 0)
	for month, sizes := range monthSizes {
		var sum float64
		for _, s := range sizes {
			sum += s
		}
		avg := sum / float64(len(sizes))

		// Get date for this month
		var date time.Time
		for _, record := range sorted {
			if record.AgeMonths == month {
				date = record.Date
				break
			}
		}

		points = append(points, GrowthPoint{
			Date:      date,
			AgeMonths: month,
			Metric:    "face_area",
			Value:     avg,
		})
	}

	// Sort by age
	sort.Slice(points, func(i, j int) bool {
		return points[i].AgeMonths < points[j].AgeMonths
	})

	return points
}

// RecognizeBabyFace recognizes a baby's face in a photo
func (bam *BabyAlbumManager) RecognizeBabyFace(ctx context.Context, img image.Image) (map[string]float64, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	if bam.faceRecognizer == nil {
		return nil, fmt.Errorf("face recognizer not configured")
	}

	// Detect faces in image
	faces, err := bam.faceRecognizer.DetectFaces(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("face detection failed: %w", err)
	}

	if len(faces) == 0 {
		return nil, nil
	}

	// Compare with known baby faces
	scores := make(map[string]float64)

	for babyID, tracker := range bam.faceHistory {
		if len(tracker.FaceHistory) == 0 {
			continue
		}

		// Use most recent face embedding for comparison
		recentFace := tracker.FaceHistory[len(tracker.FaceHistory)-1]

		for _, face := range faces {
			embedding, err := bam.faceRecognizer.ExtractEmbedding(ctx, img, face)
			if err != nil {
				continue
			}

			similarity := bam.faceRecognizer.CompareFaces(recentFace.Embedding, embedding)

			if existing, ok := scores[babyID]; !ok || similarity > existing {
				scores[babyID] = similarity
			}
		}
	}

	return scores, nil
}

// CompareFacesAtDifferentAges compares baby's face at different ages
func (bam *BabyAlbumManager) CompareFacesAtDifferentAges(babyID string, age1, age2 int) (float64, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()

	tracker, exists := bam.faceHistory[babyID]
	if !exists {
		return 0, fmt.Errorf("baby not found: %s", babyID)
	}

	// Find face records at the specified ages
	var face1, face2 *FaceRecord
	for _, record := range tracker.FaceHistory {
		if record.AgeMonths == age1 && face1 == nil {
			face1 = &record
		}
		if record.AgeMonths == age2 && face2 == nil {
			face2 = &record
		}
	}

	if face1 == nil || face2 == nil {
		return 0, fmt.Errorf("no face records found for specified ages")
	}

	// Calculate similarity
	if bam.faceRecognizer == nil {
		return 0, fmt.Errorf("face recognizer not configured")
	}

	return bam.faceRecognizer.CompareFaces(face1.Embedding, face2.Embedding), nil
}

// generateID generates a unique ID
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// DefaultBabyAlbumStorage provides in-memory storage (for testing)
type DefaultBabyAlbumStorage struct {
	albums  map[string]*BabyAlbum
	trackers map[string]*FaceGrowthTracker
	mu      sync.RWMutex
}

// NewDefaultBabyAlbumStorage creates default storage
func NewDefaultBabyAlbumStorage() *DefaultBabyAlbumStorage {
	return &DefaultBabyAlbumStorage{
		albums:   make(map[string]*BabyAlbum),
		trackers: make(map[string]*FaceGrowthTracker),
	}
}

func (s *DefaultBabyAlbumStorage) SaveBabyAlbum(album *BabyAlbum) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.albums[album.ID] = album
	return nil
}

func (s *DefaultBabyAlbumStorage) LoadBabyAlbum(id string) (*BabyAlbum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	album, ok := s.albums[id]
	if !ok {
		return nil, fmt.Errorf("album not found")
	}
	return album, nil
}

func (s *DefaultBabyAlbumStorage) ListBabyAlbums() ([]*BabyAlbum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	albums := make([]*BabyAlbum, 0, len(s.albums))
	for _, album := range s.albums {
		albums = append(albums, album)
	}
	return albums, nil
}

func (s *DefaultBabyAlbumStorage) DeleteBabyAlbum(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.albums, id)
	delete(s.trackers, id)
	return nil
}

func (s *DefaultBabyAlbumStorage) SaveFaceGrowthTracker(tracker *FaceGrowthTracker) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trackers[tracker.BabyID] = tracker
	return nil
}

func (s *DefaultBabyAlbumStorage) LoadFaceGrowthTracker(babyID string) (*FaceGrowthTracker, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tracker, ok := s.trackers[babyID]
	if !ok {
		return nil, fmt.Errorf("tracker not found")
	}
	return tracker, nil
}

// FaceRecognizer interface for face recognition
type FaceRecognizer interface {
	DetectFaces(ctx context.Context, img image.Image) ([]FaceDetection, error)
	ExtractEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error)
	CompareFaces(embedding1, embedding2 []float32) float64
}

// FaceDetection represents a detected face
type FaceDetection struct {
	ID          string
	BoundingBox BoundingBox
	Quality     float64
}

// Math helper
func init() {
	_ = math.Inf(1) // Ensure math package is used
}