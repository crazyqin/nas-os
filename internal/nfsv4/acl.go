package nfsv4

import (
	"errors"
	"sync"
	"time"
)

// ACLType ACL类型
type ACLType string

const (
	ACLTypeAllow ACLType = "allow"
	ACLTypeDeny  ACLType = "deny"
	ACLTypeAudit ACLType = "audit"
	ACLTypeAlarm ACLType = "alarm"
)

// ACLFlag ACL标志
type ACLFlag int

const (
	ACLFlagFileInherit       ACLFlag = 0x00000001 // 文件继承
	ACLFlagDirInherit        ACLFlag = 0x00000002 // 目录继承
	ACLFlagNoPropagateInherit ACLFlag = 0x00000004 // 不传播继承
	ACLFlagInheritOnly       ACLFlag = 0x00000008 // 仅继承
	ACLFlagInherited         ACLFlag = 0x00000010 // 已继承
)

// ACLPermission ACL权限
type ACLPermission int

const (
	ACLPermReadData    ACLPermission = 0x00000001 // 读数据
	ACLPermListDir     ACLPermission = 0x00000001 // 列目录 (同读数据)
	ACLPermWriteData   ACLPermission = 0x00000002 // 写数据
	ACLPermAddFile     ACLPermission = 0x00000002 // 添加文件 (同写数据)
	ACLPermAppendData  ACLPermission = 0x00000004 // 追加数据
	ACLPermAddSubdir   ACLPermission = 0x00000004 // 添加子目录 (同追加数据)
	ACLPermReadXattr   ACLPermission = 0x00000008 // 读扩展属性
	ACLPermWriteXattr  ACLPermission = 0x00000010 // 写扩展属性
	ACLPermExecute     ACLPermission = 0x00000020 // 执行
	ACLPermDeleteChild ACLPermission = 0x00000040 // 删除子项
	ACLPermReadACL     ACLPermission = 0x00000080 // 读ACL
	ACLPermWriteACL    ACLPermission = 0x00000100 // 写ACL
	ACLPermWriteOwner  ACLPermission = 0x00000200 // 写所有者
	ACLPermSynchronize ACLPermission = 0x00000400 // 同步
)

// NFSv4ACE NFSv4 ACE 条目
type NFSv4ACE struct {
	ID         string         `json:"id"`
	Type       ACLType        `json:"type"`
	Flags      ACLFlag        `json:"flags"`
	Principal  string         `json:"principal"`  // 用户或组
	Permissions ACLPermission `json:"permissions"`
	Path       string         `json:"path"`       // 适用路径
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// NFSv4ACL NFSv4 ACL 集合
type NFSv4ACL struct {
	ID        string      `json:"id"`
	Path      string      `json:"path"`
	Owner     string      `json:"owner"`
	Group     string      `json:"group"`
	ACEs      []*NFSv4ACE `json:"aces"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ACLManager ACL管理器
type ACLManager struct {
	mu    sync.RWMutex
	acls  map[string]*NFSv4ACL // path -> ACL
	aces  map[string]*NFSv4ACE // id -> ACE
}

// NewACLManager 创建ACL管理器
func NewACLManager() *ACLManager {
	return &ACLManager{
		acls: make(map[string]*NFSv4ACL),
		aces: make(map[string]*NFSv4ACE),
	}
}

// SetACL 设置路径的ACL
func (m *ACLManager) SetACL(path, owner, group string, aces []*NFSv4ACE) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 为每个ACE分配ID
	for _, ace := range aces {
		if ace.ID == "" {
			ace.ID = generateID()
		}
		ace.Path = path
		ace.CreatedAt = now
		ace.UpdatedAt = now
		m.aces[ace.ID] = ace
	}

	acl := &NFSv4ACL{
		ID:        generateID(),
		Path:      path,
		Owner:     owner,
		Group:     group,
		ACEs:      aces,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.acls[path] = acl

	return nil
}

// GetACL 获取路径的ACL
func (m *ACLManager) GetACL(path string) (*NFSv4ACL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	acl, exists := m.acls[path]
	if !exists {
		return nil, errors.New("ACL not found for path: " + path)
	}

	return acl, nil
}

// DeleteACL 删除路径的ACL
func (m *ACLManager) DeleteACL(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[path]
	if !exists {
		return errors.New("ACL not found for path: " + path)
	}

	// 删除关联的ACE
	for _, ace := range acl.ACEs {
		delete(m.aces, ace.ID)
	}

	delete(m.acls, path)

	return nil
}

// AddACE 添加ACE条目
func (m *ACLManager) AddACE(path string, ace *NFSv4ACE) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[path]
	if !exists {
		// 自动创建ACL
		acl = &NFSv4ACL{
			ID:        generateID(),
			Path:      path,
			CreatedAt: time.Now(),
		}
		m.acls[path] = acl
	}

	if ace.ID == "" {
		ace.ID = generateID()
	}
	ace.Path = path
	ace.CreatedAt = time.Now()
	ace.UpdatedAt = time.Now()

	acl.ACEs = append(acl.ACEs, ace)
	acl.UpdatedAt = time.Now()

	m.aces[ace.ID] = ace

	return nil
}

// RemoveACE 移除ACE条目
func (m *ACLManager) RemoveACE(aceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ace, exists := m.aces[aceID]
	if !exists {
		return errors.New("ACE not found: " + aceID)
	}

	acl, exists := m.acls[ace.Path]
	if exists {
		for i, a := range acl.ACEs {
			if a.ID == aceID {
				acl.ACEs = append(acl.ACEs[:i], acl.ACEs[i+1:]...)
				break
			}
		}
		acl.UpdatedAt = time.Now()
	}

	delete(m.aces, aceID)

	return nil
}

// UpdateACE 更新ACE条目
func (m *ACLManager) UpdateACE(aceID string, update func(*NFSv4ACE)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ace, exists := m.aces[aceID]
	if !exists {
		return errors.New("ACE not found: " + aceID)
	}

	update(ace)
	ace.UpdatedAt = time.Now()

	return nil
}

// ListACLs 列出所有ACL
func (m *ACLManager) ListACLs() []*NFSv4ACL {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*NFSv4ACL, 0, len(m.acls))
	for _, acl := range m.acls {
		result = append(result, acl)
	}

	return result
}

// CheckPermission 检查权限
func (m *ACLManager) CheckPermission(path, principal string, perm ACLPermission) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	acl, exists := m.acls[path]
	if !exists {
		return false, nil
	}

	// 检查所有ACE条目
	for _, ace := range acl.ACEs {
		if ace.Principal == principal || ace.Principal == "*" {
			if ace.Permissions&perm != 0 {
				if ace.Type == ACLTypeAllow {
					return true, nil
				}
				if ace.Type == ACLTypeDeny {
					return false, nil
				}
			}
		}
	}

	return false, nil
}

// GetStats 获取ACL统计信息
func (m *ACLManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_acls": len(m.acls),
		"total_aces": len(m.aces),
		"by_type":    make(map[ACLType]int),
	}

	byType := stats["by_type"].(map[ACLType]int)

	for _, ace := range m.aces {
		byType[ace.Type]++
	}

	return stats
}

// generateID 生成唯一ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}
