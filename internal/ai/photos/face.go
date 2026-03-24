// Package photos - Face recognition implementation
package photos

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

// FaceRecognitionConfig holds face recognition specific settings
type FaceRecognitionConfig struct {
	Model            string  `json:"model"`             // "arcface", "facenet", "insightface"
	ModelPath        string  `json:"model_path"`        // path to model weights
	MinFaceSize      int     `json:"min_face_size"`     // minimum face size in pixels
	ConfidenceThresh float64 `json:"confidence_thresh"` // detection confidence threshold
	ClusterThreshold float64 `json:"cluster_threshold"` // clustering similarity threshold
	MaxFacesPerPhoto int     `json:"max_faces_per_photo"`
	UseGPU           bool    `json:"use_gpu"`
}

// DefaultFaceRecognitionConfig returns defaults
func DefaultFaceRecognitionConfig() *FaceRecognitionConfig {
	return &FaceRecognitionConfig{
		Model:            "arcface",
		MinFaceSize:      30,
		ConfidenceThresh: 0.8,
		ClusterThreshold: 0.6,
		MaxFacesPerPhoto: 50,
		UseGPU:           false,
	}
}

// FaceRecognizer implements FaceDetector interface
type FaceRecognizer struct {
	config       *FaceRecognitionConfig
	model        FaceModel
	clusterCache map[int][]FaceDetection
	mu           sync.RWMutex
}

// FaceModel is the interface for face ML models
type FaceModel interface {
	// Detect detects faces in image
	Detect(ctx context.Context, img image.Image) ([]FaceDetection, error)

	// GetEmbedding extracts face embedding
	GetEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error)

	// Close releases model resources
	Close() error
}

// NewFaceRecognizer creates a new face recognizer
func NewFaceRecognizer(config *FaceRecognitionConfig) (*FaceRecognizer, error) {
	if config == nil {
		config = DefaultFaceRecognitionConfig()
	}

	// Initialize model based on config
	var model FaceModel
	var err error

	switch config.Model {
	case "arcface":
		model, err = NewArcFaceModel(config)
	case "facenet":
		model, err = NewFaceNetModel(config)
	case "insightface":
		model, err = NewInsightFaceModel(config)
	default:
		return nil, fmt.Errorf("unsupported face model: %s", config.Model)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to init face model: %w", err)
	}

	return &FaceRecognizer{
		config:       config,
		model:        model,
		clusterCache: make(map[int][]FaceDetection),
	}, nil
}

// DetectFaces implements FaceDetector.DetectFaces
func (fr *FaceRecognizer) DetectFaces(ctx context.Context, img image.Image) ([]FaceDetection, error) {
	faces, err := fr.model.Detect(ctx, img)
	if err != nil {
		return nil, err
	}

	// Filter by confidence and size
	filtered := make([]FaceDetection, 0, len(faces))
	for _, face := range faces {
		if face.Quality < fr.config.ConfidenceThresh {
			continue
		}

		// Check minimum face size
		bounds := img.Bounds()
		faceWidth := int(face.BoundingBox.Width * float64(bounds.Dx()))
		faceHeight := int(face.BoundingBox.Height * float64(bounds.Dy()))
		if faceWidth < fr.config.MinFaceSize || faceHeight < fr.config.MinFaceSize {
			continue
		}

		face.ID = generateID("face")
		face.CreatedAt = time.Now()
		filtered = append(filtered, face)
	}

	// Limit max faces
	if len(filtered) > fr.config.MaxFacesPerPhoto {
		// Sort by quality and keep top N
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Quality > filtered[j].Quality
		})
		filtered = filtered[:fr.config.MaxFacesPerPhoto]
	}

	return filtered, nil
}

// ExtractEmbedding implements FaceDetector.ExtractEmbedding
func (fr *FaceRecognizer) ExtractEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error) {
	return fr.model.GetEmbedding(ctx, img, face)
}

// CompareFaces computes cosine similarity between two embeddings
func (fr *FaceRecognizer) CompareFaces(embedding1, embedding2 []float32) float64 {
	if len(embedding1) != len(embedding2) {
		return 0
	}

	// Cosine similarity
	var dotProduct, norm1, norm2 float64
	for i := range embedding1 {
		dotProduct += float64(embedding1[i]) * float64(embedding2[i])
		norm1 += float64(embedding1[i]) * float64(embedding1[i])
		norm2 += float64(embedding2[i]) * float64(embedding2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// ClusterFaces implements FaceDetector.ClusterFaces using DBSCAN-like clustering
func (fr *FaceRecognizer) ClusterFaces(faces []FaceDetection) (*ClusterResult, error) {
	if len(faces) == 0 {
		return &ClusterResult{}, nil
	}

	// Build similarity matrix
	n := len(faces)
	similarity := make([][]float64, n)
	for i := range similarity {
		similarity[i] = make([]float64, n)
		for j := range similarity[i] {
			if i == j {
				similarity[i][j] = 1.0
			} else if faces[i].Embedding != nil && faces[j].Embedding != nil {
				similarity[i][j] = fr.CompareFaces(faces[i].Embedding, faces[j].Embedding)
			}
		}
	}

	// DBSCAN clustering
	visited := make([]bool, n)
	labels := make([]int, n)
	for i := range labels {
		labels[i] = -1 // -1 means unassigned/noise
	}

	clusterID := 0
	epsilon := fr.config.ClusterThreshold
	minPts := 2

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		visited[i] = true

		neighbors := fr.getNeighbors(similarity, i, epsilon)
		if len(neighbors) < minPts {
			// Noise point
			continue
		}

		// Start new cluster
		clusterID++
		labels[i] = clusterID

		// Expand cluster
		seedSet := make([]int, len(neighbors))
		copy(seedSet, neighbors)

		for len(seedSet) > 0 {
			current := seedSet[0]
			seedSet = seedSet[1:]

			if !visited[current] {
				visited[current] = true
				currentNeighbors := fr.getNeighbors(similarity, current, epsilon)
				if len(currentNeighbors) >= minPts {
					seedSet = append(seedSet, currentNeighbors...)
				}
			}

			if labels[current] == -1 {
				labels[current] = clusterID
			}
			if labels[current] == -1 {
				labels[current] = clusterID
			}
		}
	}

	// Build result
	persons := make([]Person, 0)
	personFaces := make(map[int][]FaceDetection)
	unassigned := make([]FaceDetection, 0)

	for i, face := range faces {
		if labels[i] == -1 {
			face.ClusterID = -1
			unassigned = append(unassigned, face)
		} else {
			face.ClusterID = labels[i]
			personFaces[labels[i]] = append(personFaces[labels[i]], face)
		}
	}

	for _, clusterFaces := range personFaces {
		// Find representative face (highest quality)
		sort.Slice(clusterFaces, func(i, j int) bool {
			return clusterFaces[i].Quality > clusterFaces[j].Quality
		})

		person := Person{
			ID:                 generateID("person"),
			FaceCount:          len(clusterFaces),
			RepresentativeFace: clusterFaces[0].ID,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		persons = append(persons, person)
	}

	// Sort by face count
	sort.Slice(persons, func(i, j int) bool {
		return persons[i].FaceCount > persons[j].FaceCount
	})

	return &ClusterResult{
		Persons:      persons,
		Unassigned:   unassigned,
		ClusterCount: len(persons),
	}, nil
}

// getNeighbors returns indices of points within epsilon distance
func (fr *FaceRecognizer) getNeighbors(similarity [][]float64, idx int, epsilon float64) []int {
	neighbors := make([]int, 0)
	for i, sim := range similarity[idx] {
		if i != idx && sim >= epsilon {
			neighbors = append(neighbors, i)
		}
	}
	return neighbors
}

// Close releases resources
func (fr *FaceRecognizer) Close() error {
	if fr.model != nil {
		return fr.model.Close()
	}
	return nil
}

// --- Model Implementations (Stubs for external model integration) ---

// ArcFaceModel implements FaceModel using ArcFace
type ArcFaceModel struct {
	config *FaceRecognitionConfig
	// TODO: Add actual model loading (ONNX Runtime, TensorFlow, etc.)
}

// NewArcFaceModel creates a new ArcFace model instance
func NewArcFaceModel(config *FaceRecognitionConfig) (*ArcFaceModel, error) {
	// Stub: In production, load actual model weights
	return &ArcFaceModel{config: config}, nil
}

// Detect detects faces in an image using ArcFace
func (m *ArcFaceModel) Detect(ctx context.Context, img image.Image) ([]FaceDetection, error) {
	// Stub: Use actual face detection model
	// For now, return empty (no faces detected)
	// Production: integrate with RetinaFace, MTCNN, or similar
	return []FaceDetection{}, nil
}

// GetEmbedding extracts face embedding using ArcFace
func (m *ArcFaceModel) GetEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error) {
	// Stub: Extract face embedding
	// Production: crop face and pass through ArcFace model
	embedding := make([]float32, 512) // ArcFace typically outputs 512-dim embedding
	return embedding, nil
}

// Close releases ArcFace model resources
func (m *ArcFaceModel) Close() error {
	return nil
}

// FaceNetModel implements FaceModel using FaceNet
type FaceNetModel struct {
	config *FaceRecognitionConfig
}

// NewFaceNetModel creates a new FaceNet model instance
func NewFaceNetModel(config *FaceRecognitionConfig) (*FaceNetModel, error) {
	return &FaceNetModel{config: config}, nil
}

// Detect detects faces in an image using FaceNet
func (m *FaceNetModel) Detect(ctx context.Context, img image.Image) ([]FaceDetection, error) {
	return []FaceDetection{}, nil
}

// GetEmbedding extracts face embedding using FaceNet
func (m *FaceNetModel) GetEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error) {
	embedding := make([]float32, 128) // FaceNet outputs 128-dim embedding
	return embedding, nil
}

// Close releases FaceNet model resources
func (m *FaceNetModel) Close() error {
	return nil
}

// InsightFaceModel implements FaceModel using InsightFace
type InsightFaceModel struct {
	config *FaceRecognitionConfig
}

// NewInsightFaceModel creates a new InsightFace model instance
func NewInsightFaceModel(config *FaceRecognitionConfig) (*InsightFaceModel, error) {
	return &InsightFaceModel{config: config}, nil
}

// Detect detects faces in an image using InsightFace
func (m *InsightFaceModel) Detect(ctx context.Context, img image.Image) ([]FaceDetection, error) {
	return []FaceDetection{}, nil
}

// GetEmbedding extracts face embedding using InsightFace
func (m *InsightFaceModel) GetEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error) {
	embedding := make([]float32, 512) // InsightFace typically outputs 512-dim
	return embedding, nil
}

// Close releases InsightFace model resources
func (m *InsightFaceModel) Close() error {
	return nil
}

// --- Face Alignment and Preprocessing ---

// FaceAligner aligns faces to canonical pose
type FaceAligner struct {
	targetSize int
}

// NewFaceAligner creates a new face aligner with the specified target size
func NewFaceAligner(targetSize int) *FaceAligner {
	if targetSize <= 0 {
		targetSize = 112 // standard face size for embedding
	}
	return &FaceAligner{targetSize: targetSize}
}

// Align aligns a face using detected landmarks
func (fa *FaceAligner) Align(img image.Image, face FaceDetection) (image.Image, error) {
	// Simplified: just crop and resize
	// Production: implement proper affine transformation with gonum or gocv
	return fa.cropFace(img, face)
}

func (fa *FaceAligner) cropFace(img image.Image, face FaceDetection) (image.Image, error) {
	bounds := img.Bounds()
	x := int(face.BoundingBox.X * float64(bounds.Dx()))
	y := int(face.BoundingBox.Y * float64(bounds.Dy()))
	w := int(face.BoundingBox.Width * float64(bounds.Dx()))
	h := int(face.BoundingBox.Height * float64(bounds.Dy()))

	// Add padding
	padding := int(float64(w) * 0.2)
	x -= padding
	y -= padding
	w += padding * 2
	h += padding * 2

	// Clamp to bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > bounds.Dx() {
		w = bounds.Dx() - x
	}
	if y+h > bounds.Dy() {
		h = bounds.Dy() - y
	}

	cropped := imaging.Crop(img, image.Rect(x, y, x+w, y+h))
	resized := imaging.Resize(cropped, fa.targetSize, fa.targetSize, imaging.Linear)

	return resized, nil
}

func (fa *FaceAligner) getLandmarkPoints(landmarks []Landmark) []Point {
	points := make([]Point, len(landmarks))
	for i, lm := range landmarks {
		points[i] = Point{X: lm.X, Y: lm.Y}
	}
	return points
}

func (fa *FaceAligner) getCanonicalLandmarks() []Point {
	// Standard 5-point face template (normalized 0-1)
	return []Point{
		{X: 0.3419, Y: 0.4646}, // left eye
		{X: 0.6581, Y: 0.4646}, // right eye
		{X: 0.5000, Y: 0.6191}, // nose
		{X: 0.3814, Y: 0.8243}, // left mouth
		{X: 0.6186, Y: 0.8243}, // right mouth
	}
}

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// --- Utility Functions ---

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// FaceIndex manages face embeddings for fast similarity search
type FaceIndex struct {
	embeddings map[string][]float32 // face_id -> embedding
	personMap  map[string]string    // face_id -> person_id
	mu         sync.RWMutex
}

// NewFaceIndex creates a new face index for similarity search
func NewFaceIndex() *FaceIndex {
	return &FaceIndex{
		embeddings: make(map[string][]float32),
		personMap:  make(map[string]string),
	}
}

// Add adds a face embedding to the index
func (fi *FaceIndex) Add(faceID string, embedding []float32, personID string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.embeddings[faceID] = embedding
	fi.personMap[faceID] = personID
}

// Search finds faces similar to the given embedding
func (fi *FaceIndex) Search(embedding []float32, threshold float64, topK int) []FaceSearchResult {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	results := make([]FaceSearchResult, 0)
	for faceID, emb := range fi.embeddings {
		sim := cosineSimilarity(embedding, emb)
		if sim >= threshold {
			results = append(results, FaceSearchResult{
				FaceID:   faceID,
				PersonID: fi.personMap[faceID],
				Score:    sim,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// Delete removes a face from the index
func (fi *FaceIndex) Delete(faceID string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	delete(fi.embeddings, faceID)
	delete(fi.personMap, faceID)
}

// FaceSearchResult for face search
type FaceSearchResult struct {
	FaceID   string  `json:"face_id"`
	PersonID string  `json:"person_id"`
	Score    float64 `json:"score"`
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
