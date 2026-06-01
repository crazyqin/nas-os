package vectordb

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistanceMetrics(t *testing.T) {
	assert.Equal(t, "cosine", string(MetricCosine))
	assert.Equal(t, "euclidean", string(MetricEuclidean))
	assert.Equal(t, "dot_product", string(MetricDotProduct))
	assert.Equal(t, "manhattan", string(MetricManhattan))
}

func TestIndexTypes(t *testing.T) {
	assert.Equal(t, "flat", string(IndexFlat))
	assert.Equal(t, "hnsw", string(IndexHNSW))
	assert.Equal(t, "ivf", string(IndexIVF))
}

func TestCreateCollection(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, err := db.CreateCollection("test", 128, MetricCosine, IndexFlat)
	require.NoError(t, err)
	assert.Equal(t, "test", col.Name)
	assert.Equal(t, 128, col.Dimension)
	assert.Equal(t, int64(0), col.Count)

	// Duplicate
	_, err = db.CreateCollection("test", 128, MetricCosine, IndexFlat)
	assert.ErrorIs(t, err, ErrCollectionExists)
}

func TestInsertAndSearch(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, err := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	require.NoError(t, err)

	v1 := &Vector{ID: "v1", Vector: []float32{1, 0, 0}}
	v2 := &Vector{ID: "v2", Vector: []float32{0, 1, 0}}
	v3 := &Vector{ID: "v3", Vector: []float32{0.7, 0.7, 0}}

	require.NoError(t, col.Insert(v1))
	require.NoError(t, col.Insert(v2))
	require.NoError(t, col.Insert(v3))
	assert.Equal(t, int64(3), col.Count)

	// Search nearest to [1, 0, 0]
	results, err := col.Search([]float32{1, 0, 0}, SearchOptions{TopK: 2})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "v1", results[0].ID) // closest
}

func TestInsertDimensionMismatch(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	err := col.Insert(&Vector{ID: "v1", Vector: []float32{1, 0}})
	assert.ErrorIs(t, err, ErrDimensionMismatch)
}

func TestDeleteVector(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	_ = col.Insert(&Vector{ID: "v1", Vector: []float32{1, 0, 0}})

	err := col.Delete("v1")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), col.Count)

	err = col.Delete("v1")
	assert.ErrorIs(t, err, ErrVectorNotFound)
}

func TestSearchWithFilter(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	_ = col.Insert(&Vector{ID: "v1", Vector: []float32{1, 0, 0}, Metadata: map[string]interface{}{"category": "a"}})
	_ = col.Insert(&Vector{ID: "v2", Vector: []float32{0, 1, 0}, Metadata: map[string]interface{}{"category": "b"}})

	results, err := col.Search([]float32{1, 0, 0}, SearchOptions{
		TopK:   10,
		Filter: map[string]interface{}{"category": "a"},
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "v1", results[0].ID)
}

func TestBatchInsert(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	vectors := []*Vector{
		{ID: "v1", Vector: []float32{1, 0, 0}},
		{ID: "v2", Vector: []float32{0, 1, 0}},
		{ID: "v3", Vector: []float32{0, 0, 1}},
	}

	inserted, err := col.BatchInsert(vectors)
	require.NoError(t, err)
	assert.Equal(t, 3, inserted)
	assert.Equal(t, int64(3), col.Count)
}

func TestDeleteCollection(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	_, err := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	require.NoError(t, err)

	err = db.DeleteCollection("test")
	assert.NoError(t, err)

	_, err = db.GetCollection("test")
	assert.ErrorIs(t, err, ErrCollectionNotFound)
}

func TestConcurrency(t *testing.T) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("test", 3, MetricCosine, IndexFlat)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			v := &Vector{
				ID:     fmt.Sprintf("v%d", id),
				Vector: []float32{float32(id), 0, 0},
			}
			_ = col.Insert(v)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int64(100), col.Count)
}

func TestCosineDistance(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	assert.InDelta(t, float32(0), cosineDistance(a, b), 0.001)

	c := []float32{0, 1, 0}
	assert.InDelta(t, float32(1), cosineDistance(a, c), 0.001)
}

func TestEuclideanDistance(t *testing.T) {
	a := []float32{0, 0}
	b := []float32{3, 4}
	assert.InDelta(t, float32(5), euclideanDistance(a, b), 0.001)
}

func TestDBClosed(t *testing.T) {
	db := NewDatabase()
	db.Close()

	_, err := db.CreateCollection("test", 3, MetricCosine, IndexFlat)
	assert.ErrorIs(t, err, ErrDBClosed)
}

func BenchmarkSearch(b *testing.B) {
	db := NewDatabase()
	defer db.Close()

	col, _ := db.CreateCollection("bench", 128, MetricCosine, IndexFlat)
	for i := 0; i < 1000; i++ {
		vec := make([]float32, 128)
		vec[i%128] = 1.0
		_ = col.Insert(&Vector{ID: fmt.Sprintf("v%d", i), Vector: vec})
	}

	query := make([]float32, 128)
	query[0] = 1.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = col.Search(query, SearchOptions{TopK: 10})
	}
}
