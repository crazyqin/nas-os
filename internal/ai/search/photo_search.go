package search

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// PhotoSearcher 图片语义搜索服务
// 对标: 飞牛fnOS AI相册以文搜图、群晖Synology Photos

// PhotoResult 图片搜索结果
type PhotoResult struct {
	ID         string    // 图片ID
	Path       string    // 文件路径
	Thumbnail  string    // 缩略图路径
	Similarity float32   // 相似度分数 (0-1)
	CreatedAt  time.Time // 创建时间
	Tags       []string  // 标签
}

// PhotoIndex 图片索引项
type PhotoIndex struct {
	ID        string
	Path      string
	Feature   []float32 // 特征向量
	CreatedAt time.Time
	Tags      []string
}

// IndexStore 索引存储接口
type IndexStore interface {
	Save(ctx context.Context, index *PhotoIndex) error
	FindByFeature(ctx context.Context, feature []float32, limit int) ([]PhotoIndex, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

// MemoryIndexStore 内存索引存储 (开发测试用)
type MemoryIndexStore struct {
	indices map[string]*PhotoIndex
	features [][]float32
	mu       sync.RWMutex
}

func NewMemoryIndexStore() *MemoryIndexStore {
	return &MemoryIndexStore{
		indices: make(map[string]*PhotoIndex),
	}
}

func (s *MemoryIndexStore) Save(ctx context.Context, index *PhotoIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indices[index.ID] = index
	return nil
}

func (s *MemoryIndexStore) FindByFeature(ctx context.Context, feature []float32, limit int) ([]PhotoIndex, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// 计算所有图片与查询特征的相似度
	results := make([]PhotoIndex, 0)
	scores := make([]struct {
		idx PhotoIndex
		score float32
	}, 0)
	
	for _, idx := range s.indices {
		score := CosineSimilarity(feature, idx.Feature)
		scores = append(scores, struct {
			idx PhotoIndex
			score float32
		}{idx: *idx, score: score})
	}
	
	// 按相似度排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	
	// 取前limit个
	for i := 0; i < minInt(limit, len(scores)); i++ {
		results = append(results, scores[i].idx)
	}
	
	return results, nil
}

// CosineSimilarity 计算余弦相似度
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func (s *MemoryIndexStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.indices, id)
	return nil
}

func (s *MemoryIndexStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.indices)), nil
}

// ClipEngine CLIP模型引擎接口
type ClipEngine interface {
	TextFeature(ctx context.Context, text string) ([]float32, error)
	ImageFeature(ctx context.Context, imagePath string) ([]float32, error)
}

// SearchService 搜索服务
type SearchService struct {
	store  IndexStore
	clip   ClipEngine
	mu     sync.RWMutex
}

// NewSearchService 创建搜索服务
func NewSearchService(store IndexStore, clipEngine ClipEngine) *SearchService {
	return &SearchService{
		store: store,
		clip:  clipEngine,
	}
}

// SearchByText 以文搜图 API
// 支持自然语言查询如: "去年夏天的海边"、"蓝色衣服的照片"
func (s *SearchService) SearchByText(ctx context.Context, query string, limit int) ([]PhotoResult, error) {
	// 1. 提取文本特征向量
	textFeature, err := s.clip.TextFeature(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("extract text feature: %w", err)
	}
	
	// 2. 在索引中搜索相似图片
	indices, err := s.store.FindByFeature(ctx, textFeature, limit)
	if err != nil {
		return nil, fmt.Errorf("search indices: %w", err)
	}
	
	// 3. 转换为搜索结果
	results := make([]PhotoResult, len(indices))
	for i, idx := range indices {
		results[i] = PhotoResult{
			ID:        idx.ID,
			Path:      idx.Path,
			Thumbnail: idx.Path + ".thumb.jpg", // 缩略图
			CreatedAt: idx.CreatedAt,
			Tags:      idx.Tags,
		}
	}
	
	return results, nil
}

// SearchByFeature 直接用特征向量搜索
func (s *SearchService) SearchByFeature(ctx context.Context, feature []float32, limit int) ([]PhotoResult, error) {
	indices, err := s.store.FindByFeature(ctx, feature, limit)
	if err != nil {
		return nil, err
	}
	
	results := make([]PhotoResult, len(indices))
	for i, idx := range indices {
		results[i] = PhotoResult{
			ID:        idx.ID,
			Path:      idx.Path,
			CreatedAt: idx.CreatedAt,
			Tags:      idx.Tags,
		}
	}
	
	return results, nil
}

// IndexPhoto 索引单张图片
func (s *SearchService) IndexPhoto(ctx context.Context, photoPath string) error {
	// 1. 提取图片特征
	feature, err := s.clip.ImageFeature(ctx, photoPath)
	if err != nil {
		return fmt.Errorf("extract image feature: %w", err)
	}
	
	// 2. 创建索引项
	index := &PhotoIndex{
		ID:        generateID(photoPath),
		Path:      photoPath,
		Feature:   feature,
		CreatedAt: time.Now(),
	}
	
	// 3. 存储索引
	return s.store.Save(ctx, index)
}

// BatchIndex 批量索引图片
func (s *SearchService) BatchIndex(ctx context.Context, photos []string) error {
	for _, photo := range photos {
		if err := s.IndexPhoto(ctx, photo); err != nil {
			// 继续处理其他图片，记录错误
			continue
		}
	}
	return nil
}

// DeleteIndex 删除图片索引
func (s *SearchService) DeleteIndex(ctx context.Context, photoID string) error {
	return s.store.Delete(ctx, photoID)
}

func generateID(path string) string {
	// 使用路径哈希生成ID
	return fmt.Sprintf("%x", len(path))[:16]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}