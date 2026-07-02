package ragserver

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// RAGServer RAG知识库服务器.
type RAGServer struct {
	logger      *zap.Logger
	collections map[string]*Collection
	vectorStore VectorStore
	chunker     *Chunker
	embedder    Embedder
	mu          sync.RWMutex
}

// Collection 文档集合.
type Collection struct {
	Name         string      `json:"name"`
	Documents    []*Document `json:"documents"`
	EmbedModel   string      `json:"embed_model"`
	ChunkSize    int         `json:"chunk_size"`
	ChunkOverlap int         `json:"chunk_overlap"`
}

// Document 文档.
type Document struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
	Chunks   []*Chunk          `json:"chunks"`
}

// Chunk 文本块.
type Chunk struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding"`
	StartIdx  int       `json:"start_idx"`
	EndIdx    int       `json:"end_idx"`
}

// VectorStore 向量存储接口.
type VectorStore interface {
	Insert(ctx context.Context, collection string, chunks []*Chunk) error
	Search(ctx context.Context, collection string, query []float32, topK int) ([]SearchResult, error)
	Delete(ctx context.Context, collection string, ids []string) error
}

// Embedder 向量化接口.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// SearchResult 搜索结果.
type SearchResult struct {
	Chunk    *Chunk  `json:"chunk"`
	Score    float64 `json:"score"`
	Document string  `json:"document"`
}

// Chunker 文本分块器.
type Chunker struct {
	Size    int
	Overlap int
}

// QueryRequest 查询请求.
type QueryRequest struct {
	Collection string  `json:"collection"`
	Query      string  `json:"query"`
	TopK       int     `json:"top_k"`
	Threshold  float64 `json:"threshold"`
}

// QueryResponse 查询响应.
type QueryResponse struct {
	Results []SearchResult `json:"results"`
	Context string         `json:"context"`
}

// NewRAGServer 创建RAG服务器.
func NewRAGServer(logger *zap.Logger, vectorStore VectorStore, embedder Embedder) *RAGServer {
	return &RAGServer{
		logger:      logger,
		collections: make(map[string]*Collection),
		vectorStore: vectorStore,
		chunker:     &Chunker{Size: 512, Overlap: 50},
		embedder:    embedder,
	}
}

// CreateCollection 创建集合.
func (rs *RAGServer) CreateCollection(ctx context.Context, name string, chunkSize, chunkOverlap int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.collections[name]; exists {
		return fmt.Errorf("collection %s already exists", name)
	}

	rs.collections[name] = &Collection{
		Name:         name,
		Documents:    make([]*Document, 0),
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}

	rs.logger.Info("Created RAG collection", zap.String("name", name))
	return nil
}

// AddDocument 添加文档.
func (rs *RAGServer) AddDocument(ctx context.Context, collection string, doc *Document) error {
	rs.mu.Lock()
	col, exists := rs.collections[collection]
	rs.mu.Unlock()

	if !exists {
		return fmt.Errorf("collection %s not found", collection)
	}

	// 分块
	chunks := rs.chunker.Chunk(doc.Content, col.ChunkSize, col.ChunkOverlap)

	// 向量化
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	embeddings, err := rs.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	for i, chunk := range chunks {
		chunk.Embedding = embeddings[i]
		chunk.ID = fmt.Sprintf("%s-chunk-%d", doc.ID, i)
	}

	doc.Chunks = chunks

	// 存储到向量库
	if err := rs.vectorStore.Insert(ctx, collection, chunks); err != nil {
		return fmt.Errorf("vector store insert failed: %w", err)
	}

	rs.mu.Lock()
	col.Documents = append(col.Documents, doc)
	rs.mu.Unlock()

	rs.logger.Info("Added document to RAG",
		zap.String("collection", collection),
		zap.String("doc", doc.ID),
		zap.Int("chunks", len(chunks)))

	return nil
}

// Query 查询.
func (rs *RAGServer) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	rs.mu.RLock()
	_, exists := rs.collections[req.Collection]
	rs.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("collection %s not found", req.Collection)
	}

	// 查询向量化
	queryEmbed, err := rs.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("query embedding failed: %w", err)
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	// 向量搜索
	results, err := rs.vectorStore.Search(ctx, req.Collection, queryEmbed, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 过滤低分结果
	var filtered []SearchResult
	for _, r := range results {
		if r.Score >= req.Threshold {
			filtered = append(filtered, r)
		}
	}

	// 构建上下文
	contextText := ""
	for _, r := range filtered {
		contextText += r.Chunk.Content + "\n\n"
	}

	return &QueryResponse{
		Results: filtered,
		Context: contextText,
	}, nil
}

// Chunk 分块.
func (c *Chunker) Chunk(text string, size, overlap int) []*Chunk {
	if size <= 0 {
		size = c.Size
	}
	if overlap <= 0 {
		overlap = c.Overlap
	}

	var chunks []*Chunk
	runes := []rune(text)
	totalLen := len(runes)

	for i := 0; i < totalLen; i += size - overlap {
		end := i + size
		if end > totalLen {
			end = totalLen
		}

		chunks = append(chunks, &Chunk{
			Content:  string(runes[i:end]),
			StartIdx: i,
			EndIdx:   end,
		})

		if end == totalLen {
			break
		}
	}

	return chunks
}

// CosineSimilarity 余弦相似度.
func CosineSimilarity(a, b []float32) float64 {
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

// SimpleVectorStore 简单向量存储(内存).
type SimpleVectorStore struct {
	data map[string]map[string]*Chunk // collection -> id -> chunk
	mu   sync.RWMutex
}

// NewSimpleVectorStore 创建简单向量存储.
func NewSimpleVectorStore() *SimpleVectorStore {
	return &SimpleVectorStore{
		data: make(map[string]map[string]*Chunk),
	}
}

func (s *SimpleVectorStore) Insert(ctx context.Context, collection string, chunks []*Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data[collection] == nil {
		s.data[collection] = make(map[string]*Chunk)
	}

	for _, chunk := range chunks {
		s.data[collection][chunk.ID] = chunk
	}
	return nil
}

func (s *SimpleVectorStore) Search(ctx context.Context, collection string, query []float32, topK int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chunks, ok := s.data[collection]
	if !ok {
		return nil, nil
	}

	var results []SearchResult
	for _, chunk := range chunks {
		score := CosineSimilarity(query, chunk.Embedding)
		results = append(results, SearchResult{
			Chunk: chunk,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func (s *SimpleVectorStore) Delete(ctx context.Context, collection string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.data[collection], id)
	}
	return nil
}
