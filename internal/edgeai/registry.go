// Package edgeai 提供模型注册表和版本管理功能
package edgeai

import (
	"fmt"
	"sync"
	"time"
)

// AdvancedModelRegistry 高级模型注册表
type AdvancedModelRegistry struct {
	mu          sync.RWMutex
	models      map[string]*Model
	versions    map[string][]*ModelVersion
	tags        map[string][]string
	categories  map[string][]string
	metadata    map[string]map[string]string
}

// NewAdvancedModelRegistry 创建高级模型注册表
func NewAdvancedModelRegistry() *AdvancedModelRegistry {
	return &AdvancedModelRegistry{
		models:     make(map[string]*Model),
		versions:   make(map[string][]*ModelVersion),
		tags:       make(map[string][]string),
		categories: make(map[string][]string),
		metadata:   make(map[string]map[string]string),
	}
}

// Register 注册模型
func (r *AdvancedModelRegistry) Register(model *Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[model.ID]; exists {
		return fmt.Errorf("模型 %s 已注册", model.ID)
	}

	r.models[model.ID] = model
	r.versions[model.ID] = make([]*ModelVersion, 0)
	r.tags[model.ID] = make([]string, 0)
	r.categories[model.ID] = make([]string, 0)
	r.metadata[model.ID] = make(map[string]string)

	// 添加初始版本
	r.versions[model.ID] = append(r.versions[model.ID], &ModelVersion{
		Version:   model.Version,
		FilePath:  model.FilePath,
		IsActive:  true,
		CreatedAt: model.CreatedAt,
	})

	return nil
}

// Unregister 注销模型
func (r *AdvancedModelRegistry) Unregister(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	delete(r.models, modelID)
	delete(r.versions, modelID)
	delete(r.tags, modelID)
	delete(r.categories, modelID)
	delete(r.metadata, modelID)

	return nil
}

// Get 获取模型
func (r *AdvancedModelRegistry) Get(modelID string) (*Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.models[modelID]
	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	return model, nil
}

// Update 更新模型
func (r *AdvancedModelRegistry) Update(model *Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[model.ID]; !exists {
		return fmt.Errorf("模型 %s 不存在", model.ID)
	}

	model.UpdatedAt = time.Now()
	r.models[model.ID] = model

	return nil
}

// List 列出所有模型
func (r *AdvancedModelRegistry) List() []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}

	return models
}

// ListByTaskType 按任务类型列出模型
func (r *AdvancedModelRegistry) ListByTaskType(taskType TaskType) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	for _, model := range r.models {
		if model.TaskType == taskType {
			models = append(models, model)
		}
	}

	return models
}

// ListByStatus 按状态列出模型
func (r *AdvancedModelRegistry) ListByStatus(status ModelStatus) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	for _, model := range r.models {
		if model.Status == status {
			models = append(models, model)
		}
	}

	return models
}

// AddTag 添加标签
func (r *AdvancedModelRegistry) AddTag(modelID, tag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	// 检查标签是否已存在
	for _, t := range r.tags[modelID] {
		if t == tag {
			return nil
		}
	}

	r.tags[modelID] = append(r.tags[modelID], tag)
	return nil
}

// RemoveTag 移除标签
func (r *AdvancedModelRegistry) RemoveTag(modelID, tag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	tags := r.tags[modelID]
	for i, t := range tags {
		if t == tag {
			r.tags[modelID] = append(tags[:i], tags[i+1:]...)
			return nil
		}
	}

	return nil
}

// GetTags 获取模型标签
func (r *AdvancedModelRegistry) GetTags(modelID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.models[modelID]; !exists {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	tags := make([]string, len(r.tags[modelID]))
	copy(tags, r.tags[modelID])
	return tags, nil
}

// SearchByTag 按标签搜索模型
func (r *AdvancedModelRegistry) SearchByTag(tag string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	for modelID, tags := range r.tags {
		for _, t := range tags {
			if t == tag {
				models = append(models, r.models[modelID])
				break
			}
		}
	}

	return models
}

// SetMetadata 设置元数据
func (r *AdvancedModelRegistry) SetMetadata(modelID, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	r.metadata[modelID][key] = value
	return nil
}

// GetMetadata 获取元数据
func (r *AdvancedModelRegistry) GetMetadata(modelID, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.models[modelID]; !exists {
		return "", fmt.Errorf("模型 %s 不存在", modelID)
	}

	value, ok := r.metadata[modelID][key]
	if !ok {
		return "", nil
	}

	return value, nil
}

// GetAllMetadata 获取所有元数据
func (r *AdvancedModelRegistry) GetAllMetadata(modelID string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.models[modelID]; !exists {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	metadata := make(map[string]string)
	for k, v := range r.metadata[modelID] {
		metadata[k] = v
	}

	return metadata, nil
}

// AddVersion 添加模型版本
func (r *AdvancedModelRegistry) AddVersion(modelID string, version *ModelVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	// 将之前的版本设为非活跃
	for _, v := range r.versions[modelID] {
		v.IsActive = false
	}

	version.IsActive = true
	version.CreatedAt = time.Now()
	r.versions[modelID] = append(r.versions[modelID], version)

	// 更新模型版本
	r.models[modelID].Version = version.Version
	r.models[modelID].FilePath = version.FilePath
	r.models[modelID].UpdatedAt = time.Now()

	return nil
}

// GetVersions 获取模型版本
func (r *AdvancedModelRegistry) GetVersions(modelID string) ([]*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.models[modelID]; !exists {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	versions := make([]*ModelVersion, len(r.versions[modelID]))
	copy(versions, r.versions[modelID])
	return versions, nil
}

// GetActiveVersion 获取活跃版本
func (r *AdvancedModelRegistry) GetActiveVersion(modelID string) (*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.models[modelID]; !exists {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	for _, v := range r.versions[modelID] {
		if v.IsActive {
			return v, nil
		}
	}

	return nil, fmt.Errorf("模型 %s 没有活跃版本", modelID)
}

// RollbackVersion 回滚到指定版本
func (r *AdvancedModelRegistry) RollbackVersion(modelID, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	versions := r.versions[modelID]
	for _, v := range versions {
		if v.Version == version {
			// 将所有版本设为非活跃
			for _, ver := range versions {
				ver.IsActive = false
			}

			// 激活目标版本
			v.IsActive = true
			model.Version = v.Version
			model.FilePath = v.FilePath
			model.UpdatedAt = time.Now()

			return nil
		}
	}

	return fmt.Errorf("版本 %s 不存在", version)
}

// Search 搜索模型
func (r *AdvancedModelRegistry) Search(query string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	query = toLower(query)

	for _, model := range r.models {
		if contains(toLower(model.Name), query) ||
			contains(toLower(model.Description), query) ||
			contains(toLower(string(model.TaskType)), query) {
			models = append(models, model)
		}
	}

	return models
}

// GetStats 获取注册表统计
func (r *AdvancedModelRegistry) GetStats() *RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &RegistryStats{
		TotalModels: len(r.models),
		ByStatus:    make(map[ModelStatus]int),
		ByTaskType:  make(map[TaskType]int),
		ByFormat:    make(map[ModelFormat]int),
		TotalVersions: 0,
	}

	for _, model := range r.models {
		stats.ByStatus[model.Status]++
		stats.ByTaskType[model.TaskType]++
		stats.ByFormat[model.Format]++
	}

	for _, versions := range r.versions {
		stats.TotalVersions += len(versions)
	}

	return stats
}

// RegistryStats 注册表统计
type RegistryStats struct {
	TotalModels   int              `json:"totalModels"`
	ByStatus      map[ModelStatus]int `json:"byStatus"`
	ByTaskType    map[TaskType]int    `json:"byTaskType"`
	ByFormat      map[ModelFormat]int `json:"byFormat"`
	TotalVersions int              `json:"totalVersions"`
}

// ModelStore 模型存储
type ModelStore struct {
	mu       sync.RWMutex
	models   map[string]*Model
	indexes  map[string]map[string]bool
}

// NewModelStore 创建模型存储
func NewModelStore() *ModelStore {
	return &ModelStore{
		models:  make(map[string]*Model),
		indexes: make(map[string]map[string]bool),
	}
}

// Save 保存模型
func (s *ModelStore) Save(model *Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.models[model.ID] = model

	// 更新索引
	s.updateIndex("status", string(model.Status), model.ID)
	s.updateIndex("taskType", string(model.TaskType), model.ID)
	s.updateIndex("format", string(model.Format), model.ID)
	s.updateIndex("device", string(model.Device), model.ID)

	return nil
}

// Load 加载模型
func (s *ModelStore) Load(modelID string) (*Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	model, ok := s.models[modelID]
	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	return model, nil
}

// Delete 删除模型
func (s *ModelStore) Delete(modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.models[modelID]
	if !ok {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	// 清理索引
	s.removeFromIndex("status", string(model.Status), modelID)
	s.removeFromIndex("taskType", string(model.TaskType), modelID)
	s.removeFromIndex("format", string(model.Format), modelID)
	s.removeFromIndex("device", string(model.Device), modelID)

	delete(s.models, modelID)
	return nil
}

// FindByIndex 通过索引查找
func (s *ModelStore) FindByIndex(indexName, value string) []*Model {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]*Model, 0)
	key := indexName + ":" + value
	index, ok := s.indexes[key]
	if !ok {
		return models
	}

	for id := range index {
		if model, exists := s.models[id]; exists {
			models = append(models, model)
		}
	}

	return models
}

// updateIndex 更新索引
func (s *ModelStore) updateIndex(indexName, value, modelID string) {
	if _, ok := s.indexes[indexName]; !ok {
		s.indexes[indexName] = make(map[string]bool)
	}
	key := indexName + ":" + value
	if _, ok := s.indexes[key]; !ok {
		s.indexes[key] = make(map[string]bool)
	}
	s.indexes[key][modelID] = true
}

// removeFromIndex 从索引中移除
func (s *ModelStore) removeFromIndex(indexName, value, modelID string) {
	key := indexName + ":" + value
	if index, ok := s.indexes[key]; ok {
		delete(index, modelID)
	}
}

// List 列出所有模型
func (s *ModelStore) List() []*Model {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]*Model, 0, len(s.models))
	for _, model := range s.models {
		models = append(models, model)
	}

	return models
}

// Count 统计模型数量
func (s *ModelStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.models)
}

// 辅助函数
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
