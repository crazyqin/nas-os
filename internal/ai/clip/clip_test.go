// Package clip - Unit tests for CLIP text-to-image search
package clip

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Types Tests ---

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ModelName != "clip-vit-base-32" {
		t.Errorf("expected default model name, got %s", config.ModelName)
	}

	if config.ImageSize != 224 {
		t.Errorf("expected default image size 224, got %d", config.ImageSize)
	}

	if config.EmbeddingDim != 512 {
		t.Errorf("expected default embedding dim 512, got %d", config.EmbeddingDim)
	}

	if config.DefaultTopK != 20 {
		t.Errorf("expected default topK 20, got %d", config.DefaultTopK)
	}
}

func TestCLIPError(t *testing.T) {
	err := ErrModelNotLoaded

	if err.Code != "model_not_loaded" {
		t.Errorf("expected error code, got %s", err.Code)
	}

	if !IsCLIPError(err) {
		t.Error("expected IsCLIPError to return true")
	}
}

// --- Mock Model Tests ---

func TestMockCLIPModel(t *testing.T) {
	model := NewMockCLIPModel(512)

	// Test model info
	info := model.GetModelInfo()
	if info.Name != "mock-clip" {
		t.Errorf("expected mock-clip, got %s", info.Name)
	}

	// Test text encoding
	emb, err := model.EncodeText(context.Background(), "a dog playing in the park")
	if err != nil {
		t.Fatalf("failed to encode text: %v", err)
	}

	if len(emb.Vector) != 512 {
		t.Errorf("expected embedding dim 512, got %d", len(emb.Vector))
	}

	// Test embedding normalization
	norm := vectorNorm(emb.Vector)
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("embedding should be normalized, norm=%f", norm)
	}
}

func TestMockCLIPModelWithLatency(t *testing.T) {
	model := NewMockCLIPModel(512)
	model.SetLatency(10 * time.Millisecond)

	start := time.Now()
	_, err := model.EncodeText(context.Background(), "test")
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected latency >= 10ms, got %v", elapsed)
	}
}

func TestMockCLIPModelFailMode(t *testing.T) {
	model := NewMockCLIPModel(512)
	model.SetFailMode(true)

	_, err := model.EncodeText(context.Background(), "test")
	if err == nil {
		t.Error("expected error in fail mode")
	}
}

func TestMockCLIPModelBatchEncode(t *testing.T) {
	model := NewMockCLIPModel(512)

	texts := []string{"cat", "dog", "bird"}
	embeddings, err := model.BatchEncodeTexts(context.Background(), texts)
	if err != nil {
		t.Fatalf("failed to batch encode: %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("expected 3 embeddings, got %d", len(embeddings))
	}

	// Each embedding should be different (deterministic based on input)
	for i, emb := range embeddings {
		if len(emb.Vector) != 512 {
			t.Errorf("embedding %d has wrong dim", i)
		}
	}
}

// --- Vector Index Tests ---

func TestFlatIndex(t *testing.T) {
	config := DefaultConfig()
	index := NewFlatIndex(config)

	ctx := context.Background()

	// Test Add
	vector := generateMockVector(512, "test1")
	err := index.Add(ctx, "photo1", vector, map[string]any{
		"photo_id": "photo1",
		"path":     "/photos/test1.jpg",
	})
	if err != nil {
		t.Fatalf("failed to add vector: %v", err)
	}

	// Test Size
	if index.Size() != 1 {
		t.Errorf("expected size 1, got %d", index.Size())
	}

	// Test Get
	emb, exists := index.Get(ctx, "photo1")
	if !exists {
		t.Error("expected to find photo1")
	}
	if emb.PhotoID != "photo1" {
		t.Errorf("wrong photo_id: %s", emb.PhotoID)
	}

	// Test Search
	query := generateMockVector(512, "test1") // Same vector
	results, err := index.Search(ctx, query, 10, 0.0)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Score < 0.99 {
		t.Errorf("expected high similarity score, got %f", results[0].Score)
	}

	// Test Delete
	err = index.Delete(ctx, "photo1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if index.Size() != 0 {
		t.Errorf("expected size 0 after delete, got %d", index.Size())
	}
}

func TestFlatIndexBatchAdd(t *testing.T) {
	config := DefaultConfig()
	index := NewFlatIndex(config)

	ctx := context.Background()

	ids := []string{"p1", "p2", "p3"}
	vectors := make([][]float32, 3)
	metadata := make([]map[string]any, 3)

	for i := range ids {
		vectors[i] = generateMockVector(512, ids[i])
		metadata[i] = map[string]any{
			"photo_id": ids[i],
			"path":     "/photos/" + ids[i] + ".jpg",
		}
	}

	err := index.BatchAdd(ctx, ids, vectors, metadata)
	if err != nil {
		t.Fatalf("batch add failed: %v", err)
	}

	if index.Size() != 3 {
		t.Errorf("expected size 3, got %d", index.Size())
	}
}

func TestFlatIndexCapacity(t *testing.T) {
	config := DefaultConfig()
	config.IndexCapacity = 5
	index := NewFlatIndex(config)

	ctx := context.Background()

	// Add up to capacity
	for i := 0; i < 5; i++ {
		vector := generateMockVector(512, string(rune('a'+i)))
		err := index.Add(ctx, string(rune('a'+i)), vector, nil)
		if err != nil {
			t.Fatalf("failed to add vector %d: %v", i, err)
		}
	}

	// Try to add one more - should fail
	vector := generateMockVector(512, "overflow")
	err := index.Add(ctx, "overflow", vector, nil)
	if err != ErrIndexFull {
		t.Errorf("expected ErrIndexFull, got %v", err)
	}
}

func TestFlatIndexSaveLoad(t *testing.T) {
	config := DefaultConfig()
	config.IndexPath = filepath.Join(t.TempDir(), "test-index.json")
	index := NewFlatIndex(config)

	ctx := context.Background()

	// Add some vectors
	for i := 0; i < 3; i++ {
		vector := generateMockVector(512, string(rune('a'+i)))
		err := index.Add(ctx, string(rune('a'+i)), vector, map[string]any{
			"photo_id": string(rune('a' + i)),
		})
		if err != nil {
			t.Fatalf("failed to add: %v", err)
		}
	}

	// Save
	err := index.Save(ctx, config.IndexPath)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Create new index and load
	newIndex := NewFlatIndex(config)
	err = newIndex.Load(ctx, config.IndexPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if newIndex.Size() != 3 {
		t.Errorf("expected size 3 after load, got %d", newIndex.Size())
	}
}

// --- HNSW Index Tests ---

func TestHNSWIndex(t *testing.T) {
	config := DefaultConfig()
	index := NewHNSWIndex(config)

	ctx := context.Background()

	// Add vectors
	for i := 0; i < 10; i++ {
		vector := generateMockVector(512, string(rune('a'+i)))
		err := index.Add(ctx, string(rune('a'+i)), vector, map[string]any{
			"photo_id": string(rune('a' + i)),
			"path":     "/photos/" + string(rune('a'+i)) + ".jpg",
		})
		if err != nil {
			t.Fatalf("failed to add: %v", err)
		}
	}

	if index.Size() != 10 {
		t.Errorf("expected size 10, got %d", index.Size())
	}

	// Search
	query := generateMockVector(512, "a")
	results, err := index.Search(ctx, query, 5, 0.0)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

// --- Embedding Cache Tests ---

func TestEmbeddingCache(t *testing.T) {
	cache := NewEmbeddingCache(100, 1*time.Hour)

	// Test miss
	_, hit := cache.GetTextEmbedding("test query")
	if hit {
		t.Error("expected cache miss")
	}

	// Test set and get
	emb := &Embedding{
		Vector:    generateMockVector(512, "test"),
		Dim:       512,
		ModelName: "test",
	}

	cache.SetTextEmbedding("test query", emb)

	// Test hit
	cached, hit := cache.GetTextEmbedding("test query")
	if !hit {
		t.Error("expected cache hit")
	}

	if cached.Dim != 512 {
		t.Errorf("wrong dim: %d", cached.Dim)
	}

	// Test stats
	stats := cache.GetStats()
	if stats.Size != 1 {
		t.Errorf("expected cache size 1, got %d", stats.Size)
	}

	// Test clear
	cache.Clear()
	_, hit = cache.GetTextEmbedding("test query")
	if hit {
		t.Error("expected cache miss after clear")
	}
}

func TestEmbeddingCacheTTL(t *testing.T) {
	cache := NewEmbeddingCache(100, 100*time.Millisecond)

	emb := &Embedding{
		Vector:    generateMockVector(512, "test"),
		Dim:       512,
		ModelName: "test",
	}

	cache.SetTextEmbedding("test query", emb)

	// Should hit immediately
	_, hit := cache.GetTextEmbedding("test query")
	if !hit {
		t.Error("expected cache hit")
	}

	// Wait for TTL
	time.Sleep(150 * time.Millisecond)

	// Should miss after TTL
	_, hit = cache.GetTextEmbedding("test query")
	if hit {
		t.Error("expected cache miss after TTL")
	}
}

// --- Text Search Service Tests ---

func TestTextSearchService(t *testing.T) {
	config := DefaultConfig()
	model := NewMockCLIPModel(512)

	service, err := NewTextSearchServiceWithModel(config, model)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Test index
	indexReq := &IndexRequest{
		PhotoID: "photo1",
		Path:    "/photos/test.jpg",
		Tags:    []string{"cat", "pet"},
	}

	indexResp, err := service.Index(ctx, indexReq)
	if err != nil {
		t.Fatalf("index failed: %v", err)
	}

	if !indexResp.Success {
		t.Errorf("index should succeed: %s", indexResp.Error)
	}

	// Test search
	searchReq := &SearchRequest{
		Query: "a cat photo",
		TopK:  10,
	}

	searchResp, err := service.Search(ctx, searchReq)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if searchResp.Total != 1 {
		t.Errorf("expected 1 result, got %d", searchResp.Total)
	}

	// Test stats
	stats, err := service.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}

	if stats.TotalIndexed != 1 {
		t.Errorf("expected 1 indexed, got %d", stats.TotalIndexed)
	}
}

func TestTextSearchServiceBatchIndex(t *testing.T) {
	config := DefaultConfig()
	model := NewMockCLIPModel(512)

	service, err := NewTextSearchServiceWithModel(config, model)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	photos := []IndexRequest{
		{PhotoID: "p1", Path: "/photos/p1.jpg"},
		{PhotoID: "p2", Path: "/photos/p2.jpg"},
		{PhotoID: "p3", Path: "/photos/p3.jpg"},
	}

	resp, err := service.BatchIndex(ctx, &BatchIndexRequest{Photos: photos})
	if err != nil {
		t.Fatalf("batch index failed: %v", err)
	}

	if resp.Success != 3 {
		t.Errorf("expected 3 successful, got %d", resp.Success)
	}
}

func TestTextSearchServiceRemove(t *testing.T) {
	config := DefaultConfig()
	model := NewMockCLIPModel(512)

	service, err := NewTextSearchServiceWithModel(config, model)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Index a photo
	service.Index(ctx, &IndexRequest{
		PhotoID: "photo1",
		Path:    "/photos/test.jpg",
	})

	// Remove it
	err = service.Remove(ctx, "photo1")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify removed
	stats, _ := service.GetStats(ctx)
	if stats.TotalIndexed != 0 {
		t.Errorf("expected 0 indexed after remove, got %d", stats.TotalIndexed)
	}
}

// --- Query Processor Tests ---

func TestQueryProcessor(t *testing.T) {
	qp := NewQueryProcessor()

	// Test stop word removal
	processed := qp.Process("a photo of a cat in the garden")
	if processed == "a photo of a cat in the garden" {
		t.Error("stop words should be removed")
	}

	// Test expansion
	expanded := qp.Expand("dog photo")
	if len(expanded) < 2 {
		t.Errorf("expected expanded queries, got %d", len(expanded))
	}
}

// --- Utility Function Tests ---

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors
	v1 := []float32{1.0, 0.0, 0.0}
	sim := cosineSimilarity(v1, v1)
	if sim < 0.99 {
		t.Errorf("identical vectors should have similarity ~1, got %f", sim)
	}

	// Orthogonal vectors
	v2 := []float32{0.0, 1.0, 0.0}
	sim = cosineSimilarity(v1, v2)
	if sim > 0.01 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}

	// Opposite vectors
	v3 := []float32{-1.0, 0.0, 0.0}
	sim = cosineSimilarity(v1, v3)
	if sim > -0.99 {
		t.Errorf("opposite vectors should have similarity ~-1, got %f", sim)
	}
}

func TestNormalizeVector(t *testing.T) {
	v := []float32{3.0, 4.0}
	normalized := normalizeVector(v)

	norm := vectorNorm(normalized)
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("normalized vector should have norm 1, got %f", norm)
	}
}

func TestVectorNorm(t *testing.T) {
	v := []float32{3.0, 4.0}
	norm := vectorNorm(v)

	expected := 5.0
	if norm < expected-0.01 || norm > expected+0.01 {
		t.Errorf("expected norm %f, got %f", expected, norm)
	}
}

// --- Benchmark Tests ---

func BenchmarkFlatIndexSearch(b *testing.B) {
	config := DefaultConfig()
	index := NewFlatIndex(config)
	ctx := context.Background()

	// Add 1000 vectors
	for i := 0; i < 1000; i++ {
		vector := generateMockVector(512, string(rune(i)))
		index.Add(ctx, string(rune(i)), vector, nil) //nolint:errcheck
	}

	query := generateMockVector(512, "benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Search(ctx, query, 20, 0.0) //nolint:errcheck
	}
}

func BenchmarkTextEncoding(b *testing.B) {
	model := NewMockCLIPModel(512)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.EncodeText(ctx, "a beautiful sunset over the ocean") //nolint:errcheck
	}
}

func BenchmarkBatchEncoding(b *testing.B) {
	model := NewMockCLIPModel(512)
	ctx := context.Background()
	texts := []string{"cat", "dog", "bird", "fish", "tree"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.BatchEncodeTexts(ctx, texts) //nolint:errcheck
	}
}

// --- Integration Test ---

func TestIntegrationTextToImageSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp directory for test images
	tmpDir := t.TempDir()

	// Create test image files (minimal JPEG)
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "photo"+string(rune('1'+i))+".jpg")
		// Create minimal test file (would need actual image in real test)
		os.WriteFile(path, []byte("fake image"), 0644) //nolint:errcheck
	}

	config := DefaultConfig()
	config.IndexPath = filepath.Join(tmpDir, "index.json")
	model := NewMockCLIPModel(512)

	service, err := NewTextSearchServiceWithModel(config, model)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Index photos
	photos := []IndexRequest{
		{PhotoID: "photo1", Path: filepath.Join(tmpDir, "photo1.jpg"), Tags: []string{"cat", "pet"}},
		{PhotoID: "photo2", Path: filepath.Join(tmpDir, "photo2.jpg"), Tags: []string{"dog", "pet"}},
		{PhotoID: "photo3", Path: filepath.Join(tmpDir, "photo3.jpg"), Tags: []string{"sunset", "nature"}},
	}

	_, err = service.BatchIndex(ctx, &BatchIndexRequest{Photos: photos})
	if err != nil {
		t.Fatalf("batch index failed: %v", err)
	}

	// Search
	resp, err := service.Search(ctx, &SearchRequest{
		Query: "a cute pet photo",
		TopK:  10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("Search returned %d results", resp.Total)
	for _, r := range resp.Results {
		t.Logf("  - %s (score: %.3f)", r.PhotoID, r.Score)
	}

	// Verify index is saved
	service.Close()

	// Load and verify
	service2, err := NewTextSearchServiceWithModel(config, model)
	if err != nil {
		t.Fatalf("failed to load service: %v", err)
	}
	defer service2.Close()

	stats, _ := service2.GetStats(ctx)
	if stats.TotalIndexed != 3 {
		t.Errorf("expected 3 indexed photos after reload, got %d", stats.TotalIndexed)
	}
}