// Package face 人脸标签管理实现
package face

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ==================== 内存标签管理器 ====================

// MemoryLabelManager 内存标签管理器
type MemoryLabelManager struct {
	persons     map[string]*Person
	personNames map[string]string // name -> personID
	faces       map[string]*Face
	photoFaces  map[string][]string // photoID -> []faceID
	personFaces map[string][]string // personID -> []faceID
	mu          sync.RWMutex
}

// NewMemoryLabelManager 创建内存标签管理器
func NewMemoryLabelManager() *MemoryLabelManager {
	return &MemoryLabelManager{
		persons:     make(map[string]*Person),
		personNames: make(map[string]string),
		faces:       make(map[string]*Face),
		photoFaces:  make(map[string][]string),
		personFaces: make(map[string][]string),
	}
}

// CreatePerson 创建人物
func (m *MemoryLabelManager) CreatePerson(ctx context.Context, name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查名称是否已存在
	if personID, exists := m.personNames[name]; exists {
		return m.persons[personID], nil
	}

	person := &Person{
		ID:        generateID("person"),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.persons[person.ID] = person
	m.personNames[name] = person.ID
	m.personFaces[person.ID] = make([]string, 0)

	return person, nil
}

// GetPerson 获取人物
func (m *MemoryLabelManager) GetPerson(ctx context.Context, personID string) (*Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	person, exists := m.persons[personID]
	if !exists {
		return nil, NewError(ErrCodePersonNotFound, "人物不存在", personID)
	}

	return person, nil
}

// GetPersonByName 根据名称获取人物
func (m *MemoryLabelManager) GetPersonByName(ctx context.Context, name string) (*Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	personID, exists := m.personNames[name]
	if !exists {
		return nil, NewError(ErrCodePersonNotFound, "人物不存在", name)
	}

	return m.persons[personID], nil
}

// ListPersons 列出所有人物
func (m *MemoryLabelManager) ListPersons(ctx context.Context, limit, offset int) ([]Person, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 转换为切片
	persons := make([]Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, *p)
	}

	// 按人脸数排序
	// 使用简单排序
	for i := 0; i < len(persons)-1; i++ {
		for j := i + 1; j < len(persons); j++ {
			if persons[i].FaceCount < persons[j].FaceCount {
				persons[i], persons[j] = persons[j], persons[i]
			}
		}
	}

	total := len(persons)

	// 分页
	if offset >= total {
		return []Person{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return persons[offset:end], total, nil
}

// UpdatePerson 更新人物
func (m *MemoryLabelManager) UpdatePerson(ctx context.Context, personID string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, exists := m.persons[personID]
	if !exists {
		return NewError(ErrCodePersonNotFound, "人物不存在", personID)
	}

	// 更新字段
	for key, value := range updates {
		switch key {
		case "name":
			if name, ok := value.(string); ok {
				// 删除旧名称映射
				delete(m.personNames, person.Name)
				// 更新名称
				person.Name = name
				m.personNames[name] = personID
			}
		case "coverFaceId":
			if coverFaceID, ok := value.(string); ok {
				person.CoverFaceID = coverFaceID
			}
		case "coverPhotoId":
			if coverPhotoID, ok := value.(string); ok {
				person.CoverPhotoID = coverPhotoID
			}
		}
	}

	person.UpdatedAt = time.Now()
	return nil
}

// DeletePerson 删除人物
func (m *MemoryLabelManager) DeletePerson(ctx context.Context, personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, exists := m.persons[personID]
	if !exists {
		return NewError(ErrCodePersonNotFound, "人物不存在", personID)
	}

	// 删除名称映射
	delete(m.personNames, person.Name)

	// 清除关联人脸的PersonID
	for _, faceID := range m.personFaces[personID] {
		if face, exists := m.faces[faceID]; exists {
			face.PersonID = ""
			face.PersonName = ""
		}
	}

	// 删除人物
	delete(m.persons, personID)
	delete(m.personFaces, personID)

	return nil
}

// AssignFaceToPerson 将人脸分配给人物
func (m *MemoryLabelManager) AssignFaceToPerson(ctx context.Context, faceID, personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, exists := m.faces[faceID]
	if !exists {
		return NewError(ErrCodeFaceNotFound, "人脸不存在", faceID)
	}

	person, exists := m.persons[personID]
	if !exists {
		return NewError(ErrCodePersonNotFound, "人物不存在", personID)
	}

	// 如果人脸已分配给其他人物，先移除
	if face.PersonID != "" && face.PersonID != personID {
		m.removeFromPerson(face.PersonID, faceID)
	}

	// 分配新人物
	face.PersonID = personID
	face.PersonName = person.Name

	// 添加到人物的人脸列表
	if !contains(m.personFaces[personID], faceID) {
		m.personFaces[personID] = append(m.personFaces[personID], faceID)
	}

	// 更新人物计数
	person.FaceCount = len(m.personFaces[personID])
	person.UpdatedAt = time.Now()

	return nil
}

// UnassignFace 取消人脸分配
func (m *MemoryLabelManager) UnassignFace(ctx context.Context, faceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, exists := m.faces[faceID]
	if !exists {
		return NewError(ErrCodeFaceNotFound, "人脸不存在", faceID)
	}

	if face.PersonID == "" {
		return nil
	}

	// 从人物列表移除
	m.removeFromPerson(face.PersonID, faceID)

	// 清除人脸的人物信息
	personID := face.PersonID
	face.PersonID = ""
	face.PersonName = ""

	// 更新人物计数
	if person, exists := m.persons[personID]; exists {
		person.FaceCount = len(m.personFaces[personID])
		person.UpdatedAt = time.Now()
	}

	return nil
}

// GetFacesByPerson 获取人物的所有人脸
func (m *MemoryLabelManager) GetFacesByPerson(ctx context.Context, personID string) ([]Face, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.persons[personID]; !exists {
		return nil, NewError(ErrCodePersonNotFound, "人物不存在", personID)
	}

	faceIDs := m.personFaces[personID]
	faces := make([]Face, 0, len(faceIDs))

	for _, faceID := range faceIDs {
		if face, exists := m.faces[faceID]; exists {
			faces = append(faces, *face)
		}
	}

	return faces, nil
}

// GetFacesByPhoto 获取照片中的所有人脸
func (m *MemoryLabelManager) GetFacesByPhoto(ctx context.Context, photoID string) ([]Face, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	faceIDs := m.photoFaces[photoID]
	faces := make([]Face, 0, len(faceIDs))

	for _, faceID := range faceIDs {
		if face, exists := m.faces[faceID]; exists {
			faces = append(faces, *face)
		}
	}

	return faces, nil
}

// 内部方法

// AddFace 添加人脸
func (m *MemoryLabelManager) AddFace(face *Face) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.faces[face.ID] = face

	// 添加到照片人脸列表
	if face.PhotoID != "" {
		if !contains(m.photoFaces[face.PhotoID], face.ID) {
			m.photoFaces[face.PhotoID] = append(m.photoFaces[face.PhotoID], face.ID)
		}
	}

	// 如果已分配人物，添加到人物人脸列表
	if face.PersonID != "" {
		if !contains(m.personFaces[face.PersonID], face.ID) {
			m.personFaces[face.PersonID] = append(m.personFaces[face.PersonID], face.ID)
		}
		if person, exists := m.persons[face.PersonID]; exists {
			person.FaceCount = len(m.personFaces[face.PersonID])
		}
	}
}

// RemoveFace 移除人脸
func (m *MemoryLabelManager) RemoveFace(faceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, exists := m.faces[faceID]
	if !exists {
		return
	}

	// 从照片人脸列表移除
	if face.PhotoID != "" {
		m.photoFaces[face.PhotoID] = removeFromSlice(m.photoFaces[face.PhotoID], faceID)
	}

	// 从人物人脸列表移除
	if face.PersonID != "" {
		m.removeFromPerson(face.PersonID, faceID)
	}

	// 删除人脸
	delete(m.faces, faceID)
}

// removeFromPerson 从人物列表移除人脸
func (m *MemoryLabelManager) removeFromPerson(personID, faceID string) {
	m.personFaces[personID] = removeFromSlice(m.personFaces[personID], faceID)
	if person, exists := m.persons[personID]; exists {
		person.FaceCount = len(m.personFaces[personID])
		person.UpdatedAt = time.Now()
	}
}

// ==================== 辅助函数 ====================

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// ==================== 人脸存储接口 ====================

// FaceStorage 人脸存储接口
type FaceStorage interface {
	// SaveFace 保存人脸
	SaveFace(ctx context.Context, face *Face) error

	// GetFace 获取人脸
	GetFace(ctx context.Context, faceID string) (*Face, error)

	// DeleteFace 删除人脸
	DeleteFace(ctx context.Context, faceID string) error

	// ListFacesByPhoto 列出照片的人脸
	ListFacesByPhoto(ctx context.Context, photoID string) ([]Face, error)

	// ListFacesByPerson 列出人物的人脸
	ListFacesByPerson(ctx context.Context, personID string) ([]Face, error)

	// SavePerson 保存人物
	SavePerson(ctx context.Context, person *Person) error

	// GetPerson 获取人物
	GetPerson(ctx context.Context, personID string) (*Person, error)

	// DeletePerson 删除人物
	DeletePerson(ctx context.Context, personID string) error

	// ListPersons 列出所有人物
	ListPersons(ctx context.Context, limit, offset int) ([]Person, int, error)
}

// StorageLabelManager 基于存储的标签管理器
type StorageLabelManager struct {
	storage FaceStorage
	cache   *MemoryLabelManager
}

// NewStorageLabelManager 创建存储标签管理器
func NewStorageLabelManager(storage FaceStorage) *StorageLabelManager {
	return &StorageLabelManager{
		storage: storage,
		cache:   NewMemoryLabelManager(),
	}
}

// CreatePerson 创建人物
func (m *StorageLabelManager) CreatePerson(ctx context.Context, name string) (*Person, error) {
	person, err := m.cache.CreatePerson(ctx, name)
	if err != nil {
		return nil, err
	}

	// 持久化
	if err := m.storage.SavePerson(ctx, person); err != nil {
		return nil, fmt.Errorf("保存人物失败: %w", err)
	}

	return person, nil
}

// GetPerson 获取人物
func (m *StorageLabelManager) GetPerson(ctx context.Context, personID string) (*Person, error) {
	return m.cache.GetPerson(ctx, personID)
}

// GetPersonByName 根据名称获取人物
func (m *StorageLabelManager) GetPersonByName(ctx context.Context, name string) (*Person, error) {
	return m.cache.GetPersonByName(ctx, name)
}

// ListPersons 列出所有人物
func (m *StorageLabelManager) ListPersons(ctx context.Context, limit, offset int) ([]Person, int, error) {
	return m.cache.ListPersons(ctx, limit, offset)
}

// UpdatePerson 更新人物
func (m *StorageLabelManager) UpdatePerson(ctx context.Context, personID string, updates map[string]interface{}) error {
	if err := m.cache.UpdatePerson(ctx, personID, updates); err != nil {
		return err
	}

	person, _ := m.cache.GetPerson(ctx, personID)
	return m.storage.SavePerson(ctx, person)
}

// DeletePerson 删除人物
func (m *StorageLabelManager) DeletePerson(ctx context.Context, personID string) error {
	if err := m.cache.DeletePerson(ctx, personID); err != nil {
		return err
	}

	return m.storage.DeletePerson(ctx, personID)
}

// AssignFaceToPerson 将人脸分配给人物
func (m *StorageLabelManager) AssignFaceToPerson(ctx context.Context, faceID, personID string) error {
	if err := m.cache.AssignFaceToPerson(ctx, faceID, personID); err != nil {
		return err
	}

	face, _ := m.cache.faces[faceID]
	return m.storage.SaveFace(ctx, face)
}

// UnassignFace 取消人脸分配
func (m *StorageLabelManager) UnassignFace(ctx context.Context, faceID string) error {
	if err := m.cache.UnassignFace(ctx, faceID); err != nil {
		return err
	}

	face, _ := m.cache.faces[faceID]
	return m.storage.SaveFace(ctx, face)
}

// GetFacesByPerson 获取人物的所有人脸
func (m *StorageLabelManager) GetFacesByPerson(ctx context.Context, personID string) ([]Face, error) {
	return m.cache.GetFacesByPerson(ctx, personID)
}

// GetFacesByPhoto 获取照片中的所有人脸
func (m *StorageLabelManager) GetFacesByPhoto(ctx context.Context, photoID string) ([]Face, error) {
	return m.cache.GetFacesByPhoto(ctx, photoID)
}