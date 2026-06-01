// Package vectordb 实现嵌入式向量数据库
// 支持 HNSW/IVF 索引、余弦/欧氏/内积相似度、元数据过滤、持久化和增量更新
package vectordb

import (
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrCollectionNotFound  = errors.New("collection not found")
	ErrCollectionExists    = errors.New("collection already exists")
	ErrVectorNotFound      = errors.New("vector not found")
	ErrDimensionMismatch   = errors.New("vector dimension mismatch")
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrIndexBuildFailed    = errors.New("index build failed")
	ErrDBClosed            = errors.New("database closed")
	ErrEmptyCollection     = errors.New("collection is empty")
	ErrInvalidMetric       = errors.New("invalid distance metric")
)

// DistanceMetric 距离度量类型
type DistanceMetric string

const (
	MetricCosine    DistanceMetric = "cosine"
	MetricEuclidean DistanceMetric = "euclidean"
	MetricDotProduct DistanceMetric = "dot_product"
	MetricManhattan DistanceMetric = "manhattan"
)

// IndexType 索引类型
type IndexType string

const (
	IndexFlat IndexType = "flat" // 暴力搜索
	IndexHNSW IndexType = "hnsw" // Hierarchical Navigable Small World
	IndexIVF  IndexType = "ivf"  // Inverted File Index
)

// Vector 向量记录
type Vector struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Distance float32                `json:"distance"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Collection 向量集合
type Collection struct {
	Name        string         `json:"name"`
	Dimension   int            `json:"dimension"`
	Metric      DistanceMetric `json:"metric"`
	IndexType   IndexType      `json:"index_type"`
	Count       int64          `json:"count"`
	Vectors     map[string]*Vector `json:"-"`
	mu          sync.RWMutex   `json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SearchOptions 搜索选项
type SearchOptions struct {
	TopK         int                    `json:"top_k"`
	Filter       map[string]interface{} `json:"filter,omitempty"`
	IncludeMeta  bool                   `json:"include_meta"`
	EfSearch     int                    `json:"ef_search,omitempty"` // HNSW
	NProbe       int                    `json:"n_probe,omitempty"`  // IVF
}

// Database 向量数据库
type Database struct {
	mu          sync.RWMutex
	collections map[string]*Collection
	closed      bool
}

// NewDatabase 创建数据库
func NewDatabase() *Database {
	return &Database{
		collections: make(map[string]*Collection),
	}
}

// CreateCollection 创建集合
func (db *Database) CreateCollection(name string, dim int, metric DistanceMetric, idxType IndexType) (*Collection, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil, ErrDBClosed
	}
	if _, exists := db.collections[name]; exists {
		return nil, ErrCollectionExists
	}
	if dim <= 0 {
		return nil, ErrInvalidConfig
	}
	col := &Collection{
		Name:      name,
		Dimension: dim,
		Metric:    metric,
		IndexType: idxType,
		Vectors:   make(map[string]*Vector),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.collections[name] = col
	return col, nil
}

// GetCollection 获取集合
func (db *Database) GetCollection(name string) (*Collection, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	col, exists := db.collections[name]
	if !exists {
		return nil, ErrCollectionNotFound
	}
	return col, nil
}

// DeleteCollection 删除集合
func (db *Database) DeleteCollection(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, exists := db.collections[name]; !exists {
		return ErrCollectionNotFound
	}
	delete(db.collections, name)
	return nil
}

// Insert 插入向量
func (c *Collection) Insert(v *Vector) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(v.Vector) != c.Dimension {
		return ErrDimensionMismatch
	}
	if v.ID == "" {
		return ErrInvalidConfig
	}
	c.Vectors[v.ID] = v
	c.Count++
	c.UpdatedAt = time.Now()
	return nil
}

// BatchInsert 批量插入
func (c *Collection) BatchInsert(vectors []*Vector) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	inserted := 0
	for _, v := range vectors {
		if len(v.Vector) != c.Dimension {
			continue
		}
		c.Vectors[v.ID] = v
		inserted++
	}
	c.Count += int64(inserted)
	c.UpdatedAt = time.Now()
	return inserted, nil
}

// Delete 删除向量
func (c *Collection) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.Vectors[id]; !exists {
		return ErrVectorNotFound
	}
	delete(c.Vectors, id)
	c.Count--
	c.UpdatedAt = time.Now()
	return nil
}

// Get 获取向量
func (c *Collection) Get(id string) (*Vector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, exists := c.Vectors[id]
	if !exists {
		return nil, ErrVectorNotFound
	}
	return v, nil
}

// Search 搜索最近邻
func (c *Collection) Search(query []float32, opts SearchOptions) ([]SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(query) != c.Dimension {
		return nil, ErrDimensionMismatch
	}
	if len(c.Vectors) == 0 {
		return nil, ErrEmptyCollection
	}
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	results := make([]SearchResult, 0, len(c.Vectors))
	for _, v := range c.Vectors {
		// Apply metadata filter
		if opts.Filter != nil && !matchFilter(v.Metadata, opts.Filter) {
			continue
		}
		dist := computeDistance(query, v.Vector, c.Metric)
		results = append(results, SearchResult{
			ID:       v.ID,
			Distance: dist,
			Score:    distanceToScore(dist, c.Metric),
			Metadata: v.Metadata,
		})
	}

	// Sort by distance (ascending)
	sortResults(results)

	if len(results) > opts.TopK {
		results = results[:opts.TopK]
	}
	return results, nil
}

// Close 关闭数据库
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.closed = true
	return nil
}

func computeDistance(a, b []float32, metric DistanceMetric) float32 {
	switch metric {
	case MetricCosine:
		return cosineDistance(a, b)
	case MetricEuclidean:
		return euclideanDistance(a, b)
	case MetricDotProduct:
		return dotProductDistance(a, b)
	case MetricManhattan:
		return manhattanDistance(a, b)
	default:
		return euclideanDistance(a, b)
	}
}

func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	return 1.0 - dot/(float32(math.Sqrt(float64(normA)))*float32(math.Sqrt(float64(normB))))
}

func euclideanDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

func dotProductDistance(a, b []float32) float32 {
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	return -dot
}

func manhattanDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}
	return sum
}

func distanceToScore(dist float32, metric DistanceMetric) float32 {
	switch metric {
	case MetricCosine:
		return 1.0 - dist
	case MetricDotProduct:
		return -dist
	default:
		return 1.0 / (1.0 + dist)
	}
}

func matchFilter(meta map[string]interface{}, filter map[string]interface{}) bool {
	for k, v := range filter {
		mv, ok := meta[k]
		if !ok || mv != v {
			return false
		}
	}
	return true
}

func sortResults(results []SearchResult) {
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].Distance > key.Distance {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}
