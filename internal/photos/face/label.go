// Package face 人脸标签管理实现
package face

import (
	"context"
	"sync"
	"time"
)

// MemoryLabelManager 内存标签管理器
type MemoryLabelManager struct {
	persons     map[string]*Person
	personNames map[string]string
	faces       map[string]*Face
	photoFaces  map[string][]string
	personFaces map[string][]string
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

	persons := make([]Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, *p)
	}

	total := len(persons)

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

	for key, value := range updates {
		switch key {
		case "name":
			if name, ok := value.(string); ok {
				delete(m.personNames, person.Name)
				person.Name = name
				m.personNames[name] = personID
			}
		case "coverFaceId":
			if coverFaceID, ok := value.(string); ok {
				person.CoverFaceID = coverFaceID
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

	delete(m.personNames, person.Name)

	for _, faceID := range m.personFaces[personID] {
		if face, exists := m.faces[faceID]; exists {
			face.PersonID = ""
			face.PersonName = ""
		}
	}

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

	if face.PersonID != "" && face.PersonID != personID {
		m.removeFromPerson(face.PersonID, faceID)
	}

	face.PersonID = personID
	face.PersonName = person.Name

	if !contains(m.personFaces[personID], faceID) {
		m.personFaces[personID] = append(m.personFaces[personID], faceID)
	}

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

	m.removeFromPerson(face.PersonID, faceID)

	personID := face.PersonID
	face.PersonID = ""
	face.PersonName = ""

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

// AddFace 添加人脸
func (m *MemoryLabelManager) AddFace(face *Face) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.faces[face.ID] = face

	if face.PhotoID != "" {
		if !contains(m.photoFaces[face.PhotoID], face.ID) {
			m.photoFaces[face.PhotoID] = append(m.photoFaces[face.PhotoID], face.ID)
		}
	}

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

	if face.PhotoID != "" {
		m.photoFaces[face.PhotoID] = removeFromSlice(m.photoFaces[face.PhotoID], faceID)
	}

	if face.PersonID != "" {
		m.removeFromPerson(face.PersonID, faceID)
	}

	delete(m.faces, faceID)
}

func (m *MemoryLabelManager) removeFromPerson(personID, faceID string) {
	m.personFaces[personID] = removeFromSlice(m.personFaces[personID], faceID)
	if person, exists := m.persons[personID]; exists {
		person.FaceCount = len(m.personFaces[personID])
		person.UpdatedAt = time.Now()
	}
}

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