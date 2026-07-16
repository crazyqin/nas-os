// Package offlineai 模型管理器，支持模型加载/卸载、模型切换和量化支持
package offlineai

import (
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// ModelManager 模型管理器.
type ModelManager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	engine *Engine
	models map[string]*ModelRegistry
}

// ModelRegistry 模型注册信息.
type ModelRegistry struct {
	Model      *Model  `json:"model"`
	LoadCount  int     `json:"load_count"` // 加载次数
	LastUsed   int64   `json:"last_used"`  // 上次使用时间戳（秒）
	Popularity float64 `json:"popularity"` // 热度评分
}

// NewModelManager 创建模型管理器.
func NewModelManager(logger *zap.Logger, engine *Engine) *ModelManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ModelManager{
		logger: logger,
		engine: engine,
		models: make(map[string]*ModelRegistry),
	}
}

// Register 注册模型.
func (mm *ModelManager) Register(model *Model) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.models[model.Name]; exists {
		return fmt.Errorf("model %s already registered", model.Name)
	}

	mm.models[model.Name] = &ModelRegistry{
		Model:      model,
		LoadCount:  0,
		Popularity: 0,
	}

	mm.logger.Info("model registered",
		zap.String("name", model.Name),
		zap.String("format", string(model.Format)),
		zap.String("quant", string(model.QuantType)),
	)
	return nil
}

// Unregister 注销模型.
func (mm *ModelManager) Unregister(name string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.models[name]; !exists {
		return fmt.Errorf("model %s not registered", name)
	}

	// 先卸载
	mm.engine.UnloadModel(name)
	delete(mm.models, name)

	mm.logger.Info("model unregistered", zap.String("name", name))
	return nil
}

// Load 加载模型到内存.
func (mm *ModelManager) Load(name string) error {
	mm.mu.RLock()
	reg, exists := mm.models[name]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("model %s not registered", name)
	}

	// 检查是否已加载
	_, err := mm.engine.GetModel(name)
	if err == nil {
		// 模型已在内存中，仍记录访问次数
		mm.mu.Lock()
		reg.LoadCount++
		mm.mu.Unlock()
		return nil
	}

	// 加载模型
	if err := mm.engine.LoadModel(nil, reg.Model); err != nil {
		return fmt.Errorf("load model %s: %w", name, err)
	}

	mm.mu.Lock()
	reg.LoadCount++
	mm.mu.Unlock()

	return nil
}

// Unload 从内存卸载模型.
func (mm *ModelManager) Unload(name string) error {
	mm.mu.RLock()
	_, exists := mm.models[name]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("model %s not registered", name)
	}

	return mm.engine.UnloadModel(name)
}

// SwitchTo 切换到指定模型.
func (mm *ModelManager) SwitchTo(name string) error {
	mm.mu.RLock()
	_, exists := mm.models[name]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("model %s not registered", name)
	}

	// 确保模型已加载
	if err := mm.Load(name); err != nil {
		return err
	}

	return mm.engine.SwitchModel(name)
}

// GetModelInfo 获取模型信息.
func (mm *ModelManager) GetModelInfo(name string) (*ModelRegistry, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	reg, exists := mm.models[name]
	if !exists {
		return nil, fmt.Errorf("model %s not registered", name)
	}
	return reg, nil
}

// ListModels 列出所有注册模型.
func (mm *ModelManager) ListModels() []*ModelRegistry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]*ModelRegistry, 0, len(mm.models))
	for _, reg := range mm.models {
		result = append(result, reg)
	}
	return result
}

// ListLoadedModels 列出已加载模型.
func (mm *ModelManager) ListLoadedModels() []*Model {
	return mm.engine.ListModels()
}

// GetModelsByFormat 按格式筛选模型.
func (mm *ModelManager) GetModelsByFormat(format ModelFormat) []*ModelRegistry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]*ModelRegistry, 0)
	for _, reg := range mm.models {
		if reg.Model.Format == format {
			result = append(result, reg)
		}
	}
	return result
}

// GetModelsByQuant 按量化类型筛选模型.
func (mm *ModelManager) GetModelsByQuant(quant QuantType) []*ModelRegistry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]*ModelRegistry, 0)
	for _, reg := range mm.models {
		if reg.Model.QuantType == quant {
			result = append(result, reg)
		}
	}
	return result
}

// GetPopularModels 获取热门模型（按加载次数排序）.
func (mm *ModelManager) GetPopularModels(limit int) []*ModelRegistry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]*ModelRegistry, 0, len(mm.models))
	for _, reg := range mm.models {
		result = append(result, reg)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LoadCount > result[j].LoadCount
	})

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

// GetSupportedQuantTypes 获取支持的量化类型.
func GetSupportedQuantTypes() []QuantType {
	return []QuantType{
		QuantNone,
		QuantQ4_0,
		QuantQ4_1,
		QuantQ5_0,
		QuantQ5_1,
		QuantQ8_0,
		QuantF16,
		QuantF32,
	}
}

// EstimateVRAMEstimate 估算模型显存需求.
func EstimateVRAMEstimate(params int64, quant QuantType) int64 {
	// 每参数字节数
	var bytesPerParam float64
	switch quant {
	case QuantF32:
		bytesPerParam = 4.0
	case QuantF16:
		bytesPerParam = 2.0
	case QuantQ8_0:
		bytesPerParam = 1.0
	case QuantQ5_0, QuantQ5_1:
		bytesPerParam = 0.625
	case QuantQ4_0, QuantQ4_1:
		bytesPerParam = 0.5
	default:
		bytesPerParam = 0.5
	}

	// 基础估算 + 开销
	base := float64(params) * bytesPerParam
	overhead := base * 0.1 // 10% 开销

	return int64(base + overhead)
}
