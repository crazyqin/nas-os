package ragserver

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewRAGServer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	vs := NewSimpleVectorStore()
	embedder := &mockEmbedder{}

	server := NewRAGServer(logger, vs, embedder)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestRAGServer_CreateCollection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := NewRAGServer(logger, NewSimpleVectorStore(), &mockEmbedder{})

	err := server.CreateCollection(context.Background(), "test", 512, 50)
	if err != nil {
		t.Fatal(err)
	}

	// 重复创建应报错
	err = server.CreateCollection(context.Background(), "test", 512, 50)
	if err == nil {
		t.Error("expected error for duplicate collection")
	}
}

func TestRAGServer_AddDocument(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := NewRAGServer(logger, NewSimpleVectorStore(), &mockEmbedder{})

	server.CreateCollection(context.Background(), "test", 100, 10)

	doc := &Document{
		ID:       "doc1",
		Content:  "This is a test document with some content for testing chunking",
		Metadata: map[string]string{"source": "test"},
	}

	err := server.AddDocument(context.Background(), "test", doc)
	if err != nil {
		t.Fatal(err)
	}

	if len(doc.Chunks) == 0 {
		t.Error("expected chunks to be created")
	}
}

func TestRAGServer_AddDocument_CollectionNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := NewRAGServer(logger, NewSimpleVectorStore(), &mockEmbedder{})

	doc := &Document{ID: "doc1", Content: "test"}
	err := server.AddDocument(context.Background(), "nonexistent", doc)
	if err == nil {
		t.Error("expected error for missing collection")
	}
}

func TestChunker_Chunk(t *testing.T) {
	chunker := &Chunker{Size: 10, Overlap: 2}

	chunks := chunker.Chunk("abcdefghijklmnopqrstuvwxyz", 10, 2)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}

	// 第一块应该是前10个字符
	if chunks[0].Content != "abcdefghij" {
		t.Errorf("expected abcdefghij, got %s", chunks[0].Content)
	}
}

func TestChunker_SmallText(t *testing.T) {
	chunker := &Chunker{Size: 100, Overlap: 10}

	chunks := chunker.Chunk("short", 100, 10)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "short" {
		t.Errorf("expected short, got %s", chunks[0].Content)
	}
}

func TestSimpleVectorStore_Insert(t *testing.T) {
	vs := NewSimpleVectorStore()

	chunks := []*Chunk{
		{ID: "c1", Content: "test1", Embedding: []float32{1, 0, 0}},
		{ID: "c2", Content: "test2", Embedding: []float32{0, 1, 0}},
	}

	err := vs.Insert(context.Background(), "col1", chunks)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSimpleVectorStore_Search(t *testing.T) {
	vs := NewSimpleVectorStore()

	chunks := []*Chunk{
		{ID: "c1", Content: "hello", Embedding: []float32{1, 0, 0}},
		{ID: "c2", Content: "world", Embedding: []float32{0, 1, 0}},
	}
	vs.Insert(context.Background(), "col1", chunks)

	results, err := vs.Search(context.Background(), "col1", []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Chunk.ID != "c1" {
		t.Errorf("expected c1 first, got %s", results[0].Chunk.ID)
	}
}

func TestSimpleVectorStore_Delete(t *testing.T) {
	vs := NewSimpleVectorStore()

	chunks := []*Chunk{
		{ID: "c1", Content: "test", Embedding: []float32{1, 0, 0}},
	}
	vs.Insert(context.Background(), "col1", chunks)

	err := vs.Delete(context.Background(), "col1", []string{"c1"})
	if err != nil {
		t.Fatal(err)
	}

	results, _ := vs.Search(context.Background(), "col1", []float32{1, 0, 0}, 1)
	if len(results) != 0 {
		t.Error("expected 0 results after delete")
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		a, b []float32
		want float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{[]float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{[]float32{1, 0}, []float32{0, 1}, 0.0},
	}

	for _, tt := range tests {
		got := CosineSimilarity(tt.a, tt.b)
		if got < tt.want-0.001 || got > tt.want+0.001 {
			t.Errorf("CosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.want)
		}
	}
}

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}
