// Package clip - Vector index implementation for image embeddings
package clip

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FlatIndex implements VectorIndex using flat (brute-force) search
type FlatIndex struct {
	vectors   map[string]*ImageEmbedding
	metadata  map[string]map[string]any
	mu        sync.RWMutex
	config    *Config
	stats     IndexStats
	dirty     bool
	savePath  string
}

// NewFlatIndex creates a new flat index
func NewFlatIndex(config *Config) *FlatIndex {
	return &FlatIndex{
		vectors:  make(map[string]*ImageEmbedding),
		metadata: make(map[string]map[string]any),
		config:   config,
		stats: IndexStats{
			IndexType:   "flat",
			Capacity:    config.IndexCapacity,
			LastUpdated: time.Now(),
		},
	}
}

// Add adds a vector to the index
func (idx *FlatIndex) Add(ctx context.Context, id string, vector []float32, metadata map[string]any) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(idx.vectors) >= idx.config.IndexCapacity {
		return ErrIndexFull
	}

	embedding := &ImageEmbedding{
		ID:        id,
		Embedding: Embedding{Vector: vector, Dim: len(vector), ModelName: idx.config.ModelName, Norm: vectorNorm(vector)},
		IndexedAt: time.Now(),
	}

	if metadata != nil {
		if photoID, ok := metadata["photo_id"].(string); ok {
			embedding.PhotoID = photoID
		}
		if path, ok := metadata["path"].(string); ok {
			embedding.Path = path
		}
		if tags, ok := metadata["tags"].([]string); ok {
			embedding.Tags = tags
		}
		if caption, ok := metadata["caption"].(string); ok {
			embedding.Caption = caption
		}
	}

	idx.vectors[id] = embedding
	idx.metadata[id] = metadata
	idx.stats.TotalVectors++
	idx.stats.LastUpdated = time.Now()
	idx.dirty = true

	return nil
}

// BatchAdd adds multiple vectors
func (idx *FlatIndex) BatchAdd(ctx context.Context, ids []string, vectors [][]float32, metadata []map[string]any) error {
	for i, id := range ids {
		var meta map[string]any
		if i < len(metadata) {
			meta = metadata[i]
		}
		if err := idx.Add(ctx, id, vectors[i], meta); err != nil {
			return fmt.Errorf("failed to add vector %s: %w", id, err)
		}
	}
	return nil
}

// Search finds nearest neighbors using brute-force cosine similarity
func (idx *FlatIndex) Search(ctx context.Context, query []float32, topK int, minScore float64) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if topK <= 0 {
		topK = idx.config.DefaultTopK
	}

	results := make([]SearchResult, 0)

	for _, emb := range idx.vectors {
		sim := cosineSimilarity(query, emb.Embedding.Vector)
		if sim >= minScore {
			results = append(results, SearchResult{
				PhotoID:   emb.PhotoID,
				Path:      emb.Path,
				Score:     sim,
				MatchType: MatchTypeSemantic,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply topK limit
	if len(results) > topK {
		results = results[:topK]
	}

	// Set ranks
	for i := range results {
		results[i].Rank = i + 1
	}

	return results, nil
}

// Delete removes a vector from the index
func (idx *FlatIndex) Delete(ctx context.Context, id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.vectors[id]; !exists {
		return nil // Already doesn't exist
	}

	delete(idx.vectors, id)
	delete(idx.metadata, id)
	idx.stats.TotalVectors--
	idx.stats.LastUpdated = time.Now()
	idx.dirty = true

	return nil
}

// Get retrieves a vector by ID
func (idx *FlatIndex) Get(ctx context.Context, id string) (*ImageEmbedding, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	emb, exists := idx.vectors[id]
	if !exists {
		return nil, false
	}
	return emb, true
}

// Size returns the number of vectors
func (idx *FlatIndex) Size() int64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return int64(len(idx.vectors))
}

// Clear removes all vectors
func (idx *FlatIndex) Clear(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.vectors = make(map[string]*ImageEmbedding)
	idx.metadata = make(map[string]map[string]any)
	idx.stats.TotalVectors = 0
	idx.stats.LastUpdated = time.Now()
	idx.dirty = true

	return nil
}

// Save persists the index to disk
func (idx *FlatIndex) Save(ctx context.Context, path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Serialize index
	data := indexData{
		Vectors:  idx.vectors,
		Metadata: idx.metadata,
		Stats:    idx.stats,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	idx.dirty = false
	idx.savePath = path

	return nil
}

// Load loads the index from disk
func (idx *FlatIndex) Load(ctx context.Context, path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Read file
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing index, that's fine
		}
		return fmt.Errorf("failed to read index file: %w", err)
	}

	// Deserialize
	var data indexData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	idx.vectors = data.Vectors
	idx.metadata = data.Metadata
	idx.stats = data.Stats
	idx.savePath = path

	return nil
}

// GetStats returns index statistics
func (idx *FlatIndex) GetStats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	stats := idx.stats
	stats.Utilization = float64(len(idx.vectors)) / float64(idx.config.IndexCapacity) * 100
	stats.IndexSize = int64(len(idx.vectors) * idx.config.EmbeddingDim * 4) // rough estimate

	return stats
}

// indexData for serialization
type indexData struct {
	Vectors  map[string]*ImageEmbedding `json:"vectors"`
	Metadata map[string]map[string]any  `json:"metadata"`
	Stats    IndexStats                 `json:"stats"`
}

// --- HNSW Index (Hierarchical Navigable Small World) ---

// HNSWIndex implements approximate nearest neighbor search
type HNSWIndex struct {
	config      *Config
	nodes       map[string]*HNSWNode
	levels      [][]string // node IDs at each level
	maxLevel    int
	ef          int     // search parameter
	ml          float64 // level multiplier
	mu          sync.RWMutex
	stats       IndexStats
}

// HNSWNode represents a node in the HNSW graph
type HNSWNode struct {
	ID        string
	Vector    []float32
	Level     int
	Neighbors map[int][]string // level -> neighbor IDs
	Metadata  map[string]any
}

// NewHNSWIndex creates a new HNSW index
func NewHNSWIndex(config *Config) *HNSWIndex {
	return &HNSWIndex{
		config: config,
		nodes:  make(map[string]*HNSWNode),
		levels: make([][]string, 16), // max 16 levels
		ef:     50,
		ml:     1.0 / math.Log(float64(16)),
		stats: IndexStats{
			IndexType: "hnsw",
			Capacity:  config.IndexCapacity,
		},
	}
}

// Add adds a vector to HNSW index
func (idx *HNSWIndex) Add(ctx context.Context, id string, vector []float32, metadata map[string]any) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if len(idx.nodes) >= idx.config.IndexCapacity {
		return ErrIndexFull
	}

	// Calculate level for new node
	level := idx.randomLevel()
	if level > idx.maxLevel {
		idx.maxLevel = level
	}

	node := &HNSWNode{
		ID:        id,
		Vector:    vector,
		Level:     level,
		Neighbors: make(map[int][]string),
		Metadata:  metadata,
	}

	// Initialize neighbor lists
	for l := 0; l <= level; l++ {
		node.Neighbors[l] = make([]string, 0)
	}

	idx.nodes[id] = node

	// Add to level lists
	for l := 0; l <= level; l++ {
		idx.levels[l] = append(idx.levels[l], id)
	}

	idx.stats.TotalVectors++
	idx.stats.LastUpdated = time.Now()

	return nil
}

// BatchAdd adds multiple vectors
func (idx *HNSWIndex) BatchAdd(ctx context.Context, ids []string, vectors [][]float32, metadata []map[string]any) error {
	for i, id := range ids {
		var meta map[string]any
		if i < len(metadata) {
			meta = metadata[i]
		}
		if err := idx.Add(ctx, id, vectors[i], meta); err != nil {
			return err
		}
	}
	return nil
}

// Search performs approximate nearest neighbor search
func (idx *HNSWIndex) Search(ctx context.Context, query []float32, topK int, minScore float64) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.nodes) == 0 {
		return []SearchResult{}, nil
	}

	// Find entry point
	var entryID string
	for l := idx.maxLevel; l >= 0; l-- {
		if len(idx.levels[l]) > 0 {
			entryID = idx.levels[l][0]
			break
		}
	}

	if entryID == "" {
		return []SearchResult{}, nil
	}

	// Greedy search from top level
	current := entryID
	for l := idx.maxLevel; l > 0; l-- {
		changed := true
		for changed {
			changed = false
			node := idx.nodes[current]
			for _, neighborID := range node.Neighbors[l] {
				if neighbor, exists := idx.nodes[neighborID]; exists {
					if cosineSimilarity(query, neighbor.Vector) > cosineSimilarity(query, idx.nodes[current].Vector) {
						current = neighborID
						changed = true
					}
				}
			}
		}
	}

	// Search at layer 0
	visited := make(map[string]bool)
	candidates := []string{current}
	results := make([]SearchResult, 0)

	for len(candidates) > 0 && len(results) < topK*2 {
		// Get best candidate
		bestIdx := 0
		bestScore := cosineSimilarity(query, idx.nodes[candidates[0]].Vector)

		for i, c := range candidates {
			if sim := cosineSimilarity(query, idx.nodes[c].Vector); sim > bestScore {
				bestScore = sim
				bestIdx = i
			}
		}

		best := candidates[bestIdx]
		candidates = append(candidates[:bestIdx], candidates[bestIdx+1:]...)

		if visited[best] {
			continue
		}
		visited[best] = true

		if bestScore >= minScore {
			node := idx.nodes[best]
			photoID, _ := node.Metadata["photo_id"].(string)
			path, _ := node.Metadata["path"].(string)
			results = append(results, SearchResult{
				PhotoID:   photoID,
				Path:      path,
				Score:     bestScore,
				MatchType: MatchTypeSemantic,
			})
		}

		// Add neighbors to candidates
		for _, neighborID := range idx.nodes[best].Neighbors[0] {
			if !visited[neighborID] {
				candidates = append(candidates, neighborID)
			}
		}
	}

	// Sort and limit
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	for i := range results {
		results[i].Rank = i + 1
	}

	return results, nil
}

// Delete removes a vector
func (idx *HNSWIndex) Delete(ctx context.Context, id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, exists := idx.nodes[id]
	if !exists {
		return nil
	}

	// Remove from level lists
	for l := 0; l <= node.Level; l++ {
		for i, nodeID := range idx.levels[l] {
			if nodeID == id {
				idx.levels[l] = append(idx.levels[l][:i], idx.levels[l][i+1:]...)
				break
			}
		}
	}

	// Remove from neighbor lists
	for _, other := range idx.nodes {
		for l, neighbors := range other.Neighbors {
			for i, neighborID := range neighbors {
				if neighborID == id {
					other.Neighbors[l] = append(neighbors[:i], neighbors[i+1:]...)
					break
				}
			}
		}
	}

	delete(idx.nodes, id)
	idx.stats.TotalVectors--

	return nil
}

// Get retrieves a vector by ID
func (idx *HNSWIndex) Get(ctx context.Context, id string) (*ImageEmbedding, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	node, exists := idx.nodes[id]
	if !exists {
		return nil, false
	}

	return &ImageEmbedding{
		ID:        node.ID,
		Embedding: Embedding{Vector: node.Vector, Dim: len(node.Vector)},
	}, true
}

// Size returns index size
func (idx *HNSWIndex) Size() int64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return int64(len(idx.nodes))
}

// Clear clears the index
func (idx *HNSWIndex) Clear(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.nodes = make(map[string]*HNSWNode)
	idx.levels = make([][]string, 16)
	idx.maxLevel = 0
	idx.stats.TotalVectors = 0

	return nil
}

// Save saves the index
func (idx *HNSWIndex) Save(ctx context.Context, path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	data := hnswIndexData{
		Nodes:    idx.nodes,
		MaxLevel: idx.maxLevel,
		Stats:    idx.stats,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// Load loads the index
func (idx *HNSWIndex) Load(ctx context.Context, path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data hnswIndexData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	idx.nodes = data.Nodes
	idx.maxLevel = data.MaxLevel
	idx.stats = data.Stats

	return nil
}

// GetStats returns statistics
func (idx *HNSWIndex) GetStats() IndexStats {
	return idx.stats
}

// randomLevel generates a random level for new nodes
func (idx *HNSWIndex) randomLevel() int {
	level := 0
	for float64(fastRand()) < idx.ml && level < 15 {
		level++
	}
	return level
}

// fastRand generates a fast random float [0, 1)
func fastRand() float64 {
	return float64(time.Now().UnixNano()%10000) / 10000.0
}

type hnswIndexData struct {
	Nodes    map[string]*HNSWNode `json:"nodes"`
	MaxLevel int                  `json:"max_level"`
	Stats    IndexStats           `json:"stats"`
}

// NewVectorIndex creates a vector index based on config
func NewVectorIndex(config *Config) (VectorIndex, error) {
	switch config.IndexType {
	case "flat":
		return NewFlatIndex(config), nil
	case "hnsw":
		return NewHNSWIndex(config), nil
	case "ivf":
		// IVF index - simplified implementation
		return NewFlatIndex(config), nil
	default:
		return NewFlatIndex(config), nil
	}
}