package aiconnector

import (
	"context"
	"sync"
)

// Embedder 嵌入向量生成器
type Embedder struct {
	mu       sync.RWMutex
	model    string
	cache    map[string][]float64
	cacheMax int
}

// NewEmbedder 创建嵌入向量生成器
func NewEmbedder(model string) *Embedder {
	return &Embedder{
		model:    model,
		cache:    make(map[string][]float64),
		cacheMax: 1000,
	}
}

// Embed 生成嵌入向量
func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	e.mu.RLock()
	// 检查缓存
	if vector, exists := e.cache[text]; exists {
		e.mu.RUnlock()
		return vector, nil
	}
	e.mu.RUnlock()

	// 生成向量（简化实现）
	vector := e.generateVector(text)

	// 缓存结果
	e.mu.Lock()
	if len(e.cache) >= e.cacheMax {
		// 清理旧缓存
		e.cache = make(map[string][]float64)
	}
	e.cache[text] = vector
	e.mu.Unlock()

	return vector, nil
}

// generateVector 生成向量（简化实现）
func (e *Embedder) generateVector(text string) []float64 {
	// 简化的向量生成，实际应该调用AI模型
	vector := make([]float64, 128)
	for i := 0; i < len(text) && i < 128; i++ {
		vector[i] = float64(text[i]) / 255.0
	}
	return vector
}

// ClearCache 清理缓存
func (e *Embedder) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = make(map[string][]float64)
}
