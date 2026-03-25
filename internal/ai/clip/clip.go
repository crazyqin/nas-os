// Package clip - CLIP model implementation for text-to-image search
package clip

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

// CLIPModelImpl implements CLIPModel interface
type CLIPModelImpl struct {
	config    *Config
	modelInfo ModelInfo
	imageSize int
	dim       int

	// Tokenizer for text encoding (simplified)
	vocab     map[string]int
	vocabSize int

	// Model weights (simplified - production would use ONNX/TensorFlow)
	imageWeights *ImageEncoderWeights
	textWeights  *TextEncoderWeights

	mu sync.RWMutex
}

// ImageEncoderWeights holds image encoder weights (placeholder)
type ImageEncoderWeights struct {
	// Production: actual model weights
	Initialized bool
}

// TextEncoderWeights holds text encoder weights (placeholder)
type TextEncoderWeights struct {
	// Production: actual model weights
	Initialized bool
}

// NewCLIPModel creates a new CLIP model instance
func NewCLIPModel(config *Config) (*CLIPModelImpl, error) {
	if config == nil {
		config = DefaultConfig()
	}

	model := &CLIPModelImpl{
		config:    config,
		imageSize: config.ImageSize,
		dim:       config.EmbeddingDim,
		modelInfo: ModelInfo{
			Name:         config.ModelName,
			Version:      "1.0.0",
			ImageSize:    config.ImageSize,
			EmbeddingDim: config.EmbeddingDim,
			GPUEnabled:   config.UseGPU,
		},
		vocab:        make(map[string]int),
		imageWeights: &ImageEncoderWeights{Initialized: true},
		textWeights:  &TextEncoderWeights{Initialized: true},
	}

	// Initialize vocabulary (simplified)
	model.initVocabulary()

	return model, nil
}

// initVocabulary initializes a simple vocabulary
func (m *CLIPModelImpl) initVocabulary() {
	// Simplified vocabulary for demonstration
	// Production: load from BPE tokenizer file
	commonWords := []string{
		"a", "an", "the", "is", "are", "was", "were", "be", "been", "being",
		"have", "has", "had", "do", "does", "did", "will", "would", "could", "should",
		"photo", "picture", "image", "of", "with", "in", "on", "at", "by", "for",
		"person", "people", "man", "woman", "child", "baby", "girl", "boy",
		"dog", "cat", "pet", "animal", "bird", "fish",
		"car", "bike", "bus", "train", "plane", "boat",
		"house", "building", "room", "office", "school",
		"tree", "flower", "plant", "grass", "garden",
		"water", "sea", "ocean", "river", "lake", "beach",
		"mountain", "hill", "valley", "forest", "desert",
		"sky", "cloud", "sun", "moon", "star", "rain", "snow",
		"food", "meal", "dinner", "lunch", "breakfast",
		"red", "blue", "green", "yellow", "black", "white",
		"big", "small", "large", "tiny", "huge",
		"beautiful", "nice", "good", "bad", "old", "new",
		"day", "night", "morning", "evening", "afternoon",
		"summer", "winter", "spring", "autumn", "fall",
		"happy", "sad", "smile", "laugh", "cry",
		"sport", "game", "play", "run", "walk", "swim",
		"travel", "trip", "vacation", "holiday", "journey",
		"family", "friend", "love", "wedding", "party",
	}

	for i, word := range commonWords {
		m.vocab[word] = i + 4 // reserve 0-3 for special tokens
	}
	m.vocabSize = len(commonWords) + 4
}

// EncodeImage generates embedding for an image file.
func (m *CLIPModelImpl) EncodeImage(ctx context.Context, imagePath string) (*Embedding, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read image file
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return m.EncodeImageFromBytes(ctx, m.imageToBytes(img))
}

// EncodeImageFromBytes generates embedding from image data
func (m *CLIPModelImpl) EncodeImageFromBytes(ctx context.Context, data []byte) (*Embedding, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Create image from bytes
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image bytes: %w", err)
	}

	// Preprocess image
	processed := m.preprocessImage(img)

	// Generate embedding (simplified - production uses actual model)
	vector := m.generateImageEmbedding(processed)

	// Normalize
	normalized := normalizeVector(vector)

	return &Embedding{
		Vector:    normalized,
		Dim:       m.dim,
		ModelName: m.config.ModelName,
		Norm:      vectorNorm(normalized),
	}, nil
}

// EncodeText generates embedding for text
func (m *CLIPModelImpl) EncodeText(ctx context.Context, text string) (*Embedding, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Tokenize text
	tokens := m.tokenize(text)

	// Generate embedding
	vector := m.generateTextEmbedding(tokens)

	// Normalize
	normalized := normalizeVector(vector)

	return &Embedding{
		Vector:    normalized,
		Dim:       m.dim,
		ModelName: m.config.ModelName,
		Norm:      vectorNorm(normalized),
	}, nil
}

// BatchEncodeImages encodes multiple images in parallel
func (m *CLIPModelImpl) BatchEncodeImages(ctx context.Context, paths []string) ([]*Embedding, error) {
	results := make([]*Embedding, len(paths))
	errs := make([]error, len(paths))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.config.NumWorkers)

	for i, path := range paths {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			emb, err := m.EncodeImage(ctx, p)
			results[idx] = emb
			errs[idx] = err
		}(i, path)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errs {
		if err != nil {
			return results, fmt.Errorf("batch encoding had errors: %w", err)
		}
	}

	return results, nil
}

// BatchEncodeTexts encodes multiple texts in parallel
func (m *CLIPModelImpl) BatchEncodeTexts(ctx context.Context, texts []string) ([]*Embedding, error) {
	results := make([]*Embedding, len(texts))

	// Text encoding is fast, no need for parallelization at model level
	for i, text := range texts {
		emb, err := m.EncodeText(ctx, text)
		if err != nil {
			return results, fmt.Errorf("failed to encode text %d: %w", i, err)
		}
		results[i] = emb
	}

	return results, nil
}

// GetModelInfo returns model information
func (m *CLIPModelImpl) GetModelInfo() ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.modelInfo
}

// Close releases model resources
func (m *CLIPModelImpl) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imageWeights = nil
	m.textWeights = nil
	m.vocab = nil

	return nil
}

// --- Internal Methods ---

// preprocessImage preprocesses image for CLIP
func (m *CLIPModelImpl) preprocessImage(img image.Image) image.Image {
	// Resize to model input size
	resized := imaging.Resize(img, m.imageSize, m.imageSize, imaging.Linear)

	// Crop to square if needed
	cropped := imaging.Fill(resized, m.imageSize, m.imageSize, imaging.Center, imaging.Linear)

	return cropped
}

// imageToBytes converts image to bytes.
func (m *CLIPModelImpl) imageToBytes(img image.Image) []byte {
	// Simplified - convert to JPEG bytes
	buf := new(bytes.Buffer)
	_ = imaging.Encode(buf, img, imaging.JPEG)
	return buf.Bytes()
}

// generateImageEmbedding generates embedding from preprocessed image
func (m *CLIPModelImpl) generateImageEmbedding(img image.Image) []float32 {
	// Simplified embedding generation
	// Production: actual forward pass through Vision Transformer

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create pseudo-embedding from image statistics
	vector := make([]float32, m.dim)

	// Use pixel statistics as features (simplified)
	// Production: use actual CNN/ViT features
	for y := 0; y < height && y < 32; y++ {
		for x := 0; x < width && x < 32; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			idx := (y*32 + x) % m.dim
			vector[idx] += float32(r+g+b) / 3.0 / 65535.0
		}
	}

	// Add hash-based features for diversity
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d%d", width, height)))
	for i := 0; i < len(hash) && i < m.dim/8; i++ {
		vector[i*8] = float32(hash[i]) / 255.0
	}

	return vector
}

// tokenize tokenizes text into token IDs
func (m *CLIPModelImpl) tokenize(text string) []int {
	// Simplified tokenization
	// Production: use BPE tokenizer

	words := splitWords(text)
	tokens := make([]int, 0, len(words)+2)

	// Start token
	tokens = append(tokens, 1) // BOS

	for _, word := range words {
		if id, ok := m.vocab[word]; ok {
			tokens = append(tokens, id)
		} else {
			// Unknown token
			tokens = append(tokens, 2) // UNK
		}
	}

	// End token
	tokens = append(tokens, 3) // EOS

	return tokens
}

// generateTextEmbedding generates embedding from tokens
func (m *CLIPModelImpl) generateTextEmbedding(tokens []int) []float32 {
	// Simplified embedding generation
	// Production: actual forward pass through Text Transformer

	vector := make([]float32, m.dim)

	// Use token statistics as features (simplified)
	for i, token := range tokens {
		if i >= m.dim {
			break
		}
		// Simple hash-based embedding
		vector[i] = float32(token%1000) / 1000.0
	}

	// Add semantic features based on token patterns
	for _, token := range tokens {
		// Map tokens to embedding dimensions
		idx := token % m.dim
		vector[idx] += 0.1
	}

	return vector
}

// --- Utility Functions ---

// splitWords splits text into words
func splitWords(text string) []string {
	// Simple word splitting
	// Production: use proper tokenizer with punctuation handling

	words := make([]string, 0)
	current := ""

	for _, r := range text {
		if isAlphaNumeric(r) {
			current += string(r)
		} else {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		}
	}

	if current != "" {
		words = append(words, current)
	}

	return words
}

// isAlphaNumeric checks if rune is alphanumeric
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// normalizeVector normalizes a vector to unit length
func normalizeVector(v []float32) []float32 {
	norm := vectorNorm(v)
	if norm == 0 {
		return v
	}

	normalized := make([]float32, len(v))
	for i, val := range v {
		normalized[i] = float32(float64(val) / norm)
	}

	return normalized
}

// vectorNorm computes L2 norm
func vectorNorm(v []float32) float64 {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	return math.Sqrt(sum)
}

// cosineSimilarity computes cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

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

// --- Mock Model for Testing ---

// MockCLIPModel is a mock implementation for testing
type MockCLIPModel struct {
	dim      int
	latency  time.Duration
	failMode bool
}

// NewMockCLIPModel creates a mock CLIP model
func NewMockCLIPModel(dim int) *MockCLIPModel {
	return &MockCLIPModel{dim: dim}
}

// SetLatency sets artificial latency for testing
func (m *MockCLIPModel) SetLatency(d time.Duration) {
	m.latency = d
}

// SetFailMode sets failure mode for testing
func (m *MockCLIPModel) SetFailMode(fail bool) {
	m.failMode = fail
}

// EncodeImage generates mock embedding
func (m *MockCLIPModel) EncodeImage(ctx context.Context, imagePath string) (*Embedding, error) {
	if m.failMode {
		return nil, ErrEncodingFailed
	}
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return &Embedding{
		Vector:    generateMockVector(m.dim, imagePath),
		Dim:       m.dim,
		ModelName: "mock-clip",
		Norm:      1.0,
	}, nil
}

// EncodeImageFromBytes generates mock embedding
func (m *MockCLIPModel) EncodeImageFromBytes(ctx context.Context, data []byte) (*Embedding, error) {
	if m.failMode {
		return nil, ErrEncodingFailed
	}
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return &Embedding{
		Vector:    generateMockVector(m.dim, string(data)),
		Dim:       m.dim,
		ModelName: "mock-clip",
		Norm:      1.0,
	}, nil
}

// EncodeText generates mock embedding
func (m *MockCLIPModel) EncodeText(ctx context.Context, text string) (*Embedding, error) {
	if m.failMode {
		return nil, ErrEncodingFailed
	}
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return &Embedding{
		Vector:    generateMockVector(m.dim, text),
		Dim:       m.dim,
		ModelName: "mock-clip",
		Norm:      1.0,
	}, nil
}

// BatchEncodeImages batch encodes images
func (m *MockCLIPModel) BatchEncodeImages(ctx context.Context, paths []string) ([]*Embedding, error) {
	results := make([]*Embedding, len(paths))
	for i, path := range paths {
		emb, err := m.EncodeImage(ctx, path)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

// BatchEncodeTexts batch encodes texts
func (m *MockCLIPModel) BatchEncodeTexts(ctx context.Context, texts []string) ([]*Embedding, error) {
	results := make([]*Embedding, len(texts))
	for i, text := range texts {
		emb, err := m.EncodeText(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

// GetModelInfo returns mock model info
func (m *MockCLIPModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:         "mock-clip",
		Version:      "test",
		ImageSize:    224,
		EmbeddingDim: m.dim,
		GPUEnabled:   false,
	}
}

// Close closes mock model
func (m *MockCLIPModel) Close() error {
	return nil
}

// generateMockVector generates a deterministic mock vector
func generateMockVector(dim int, seed string) []float32 {
	vector := make([]float32, dim)
	hash := sha256.Sum256([]byte(seed))

	// Generate deterministic pseudo-random vector
	for i := range vector {
		byteIdx := i % len(hash)
		vector[i] = float32(hash[byteIdx])/255.0*2.0 - 1.0
	}

	// Normalize
	return normalizeVector(vector)
}
