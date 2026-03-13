package tags

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Tag 标签定义
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`     // 颜色代码，如 #FF5733
	Icon      string    `json:"icon"`      // 图标名称或 emoji
	Group     string    `json:"group"`     // 标签分组
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TagInput 创建/更新标签输入
type TagInput struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
	Group string `json:"group"`
}

// FileTag 文件标签关联
type FileTag struct {
	FilePath string    `json:"filePath"`
	TagIDs   []string  `json:"tagIds"`
	AddedAt  time.Time `json:"addedAt"`
}

// FileTagInput 文件标签操作输入
type FileTagInput struct {
	FilePath string   `json:"filePath" binding:"required"`
	TagIDs   []string `json:"tagIds" binding:"required"`
}

// TagGroup 标签分组
type TagGroup struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Count       int    `json:"count"` // 该分组下的标签数量
}

// 统计信息
type Stats struct {
	TotalTags    int `json:"totalTags"`
	TotalFiles   int `json:"totalFiles"`
	TotalGrouped int `json:"totalGrouped"`
}

// 错误定义
var (
	ErrTagNotFound      = errors.New("标签不存在")
	ErrTagExists        = errors.New("标签名称已存在")
	ErrInvalidTagID     = errors.New("无效的标签ID")
	ErrInvalidFilePath  = errors.New("无效的文件路径")
	ErrTagAlreadyAdded  = errors.New("标签已添加到该文件")
	ErrTagNotOnFile     = errors.New("文件没有此标签")
)

// Manager 标签管理器
type Manager struct {
	db      *sql.DB
	dbPath  string
	mu      sync.RWMutex
}

// NewManager 创建标签管理器
func NewManager(dbPath string) (*Manager, error) {
	m := &Manager{
		dbPath: dbPath,
	}

	if err := m.initDB(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败：%w", err)
	}

	return m, nil
}

// initDB 初始化数据库
func (m *Manager) initDB() error {
	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		return err
	}
	m.db = db

	// 创建标签表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS tags (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		color TEXT DEFAULT '#3498db',
		icon TEXT DEFAULT '',
		grp TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
	CREATE INDEX IF NOT EXISTS idx_tags_grp ON tags(grp);

	CREATE TABLE IF NOT EXISTS file_tags (
		file_path TEXT NOT NULL,
		tag_id TEXT NOT NULL,
		added_at DATETIME NOT NULL,
		PRIMARY KEY (file_path, tag_id),
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_file_tags_file ON file_tags(file_path);
	CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag_id);
	`

	_, err = db.Exec(createTableSQL)
	return err
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== 标签 CRUD ==========

// CreateTag 创建标签
func (m *Manager) CreateTag(input TagInput) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查名称是否已存在
	var exists int
	err := m.db.QueryRow("SELECT 1 FROM tags WHERE name = ?", input.Name).Scan(&exists)
	if err == nil {
		return nil, ErrTagExists
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now()
	tag := &Tag{
		ID:        generateID(),
		Name:      input.Name,
		Color:     input.Color,
		Icon:      input.Icon,
		Group:     input.Group,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 设置默认值
	if tag.Color == "" {
		tag.Color = "#3498db"
	}

	query := `INSERT INTO tags (id, name, color, icon, grp, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = m.db.Exec(query, tag.ID, tag.Name, tag.Color, tag.Icon, tag.Group, tag.CreatedAt, tag.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("创建标签失败：%w", err)
	}

	return tag, nil
}

// GetTag 获取标签
func (m *Manager) GetTag(id string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag := &Tag{}
	query := `SELECT id, name, color, icon, grp, created_at, updated_at FROM tags WHERE id = ?`
	err := m.db.QueryRow(query, id).Scan(
		&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
		&tag.CreatedAt, &tag.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTagNotFound
	}
	if err != nil {
		return nil, err
	}

	return tag, nil
}

// GetTagByName 通过名称获取标签
func (m *Manager) GetTagByName(name string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag := &Tag{}
	query := `SELECT id, name, color, icon, grp, created_at, updated_at FROM tags WHERE name = ?`
	err := m.db.QueryRow(query, name).Scan(
		&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
		&tag.CreatedAt, &tag.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTagNotFound
	}
	if err != nil {
		return nil, err
	}

	return tag, nil
}

// ListTags 列出所有标签
func (m *Manager) ListTags() ([]*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `SELECT id, name, color, icon, grp, created_at, updated_at FROM tags ORDER BY name`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag := &Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
			&tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// ListTagsByGroup 按分组列出标签
func (m *Manager) ListTagsByGroup(group string) ([]*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `SELECT id, name, color, icon, grp, created_at, updated_at FROM tags WHERE grp = ? ORDER BY name`
	rows, err := m.db.Query(query, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag := &Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
			&tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// UpdateTag 更新标签
func (m *Manager) UpdateTag(id string, input TagInput) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查标签是否存在
	var existingTag Tag
	err := m.db.QueryRow("SELECT id, name, color, icon, grp, created_at, updated_at FROM tags WHERE id = ?",
		id).Scan(&existingTag.ID, &existingTag.Name, &existingTag.Color, &existingTag.Icon,
		&existingTag.Group, &existingTag.CreatedAt, &existingTag.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrTagNotFound
	}
	if err != nil {
		return nil, err
	}

	// 检查新名称是否已被其他标签使用
	if input.Name != "" {
		var existingID string
		err := m.db.QueryRow("SELECT id FROM tags WHERE name = ? AND id != ?", input.Name, id).Scan(&existingID)
		if err == nil {
			return nil, ErrTagExists
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	now := time.Now()
	query := `UPDATE tags SET name = COALESCE(NULLIF(?, ''), name), color = COALESCE(NULLIF(?, ''), color), 
	          icon = COALESCE(NULLIF(?, ''), icon), grp = COALESCE(NULLIF(?, ''), grp), updated_at = ? WHERE id = ?`
	_, err = m.db.Exec(query, input.Name, input.Color, input.Icon, input.Group, now, id)
	if err != nil {
		return nil, fmt.Errorf("更新标签失败：%w", err)
	}

	// 返回更新后的标签
	tag := &Tag{
		ID:        id,
		Name:      input.Name,
		Color:     input.Color,
		Icon:      input.Icon,
		Group:     input.Group,
		UpdatedAt: now,
		CreatedAt: existingTag.CreatedAt,
	}
	if tag.Name == "" {
		tag.Name = existingTag.Name
	}
	if tag.Color == "" {
		tag.Color = existingTag.Color
	}
	if tag.Icon == "" {
		tag.Icon = existingTag.Icon
	}
	if tag.Group == "" {
		tag.Group = existingTag.Group
	}

	return tag, nil
}

// DeleteTag 删除标签
func (m *Manager) DeleteTag(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先删除文件关联
	_, err := m.db.Exec("DELETE FROM file_tags WHERE tag_id = ?", id)
	if err != nil {
		return fmt.Errorf("删除文件标签关联失败：%w", err)
	}

	// 删除标签
	result, err := m.db.Exec("DELETE FROM tags WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除标签失败：%w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrTagNotFound
	}

	return nil
}

// ========== 文件标签操作 ==========

// AddTagsToFile 为文件添加标签
func (m *Manager) AddTagsToFile(filePath string, tagIDs []string) error {
	if filePath == "" {
		return ErrInvalidFilePath
	}
	if len(tagIDs) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, tagID := range tagIDs {
		// 检查标签是否存在
		var exists int
		err := m.db.QueryRow("SELECT 1 FROM tags WHERE id = ?", tagID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrInvalidTagID
		}
		if err != nil {
			return err
		}

		// 添加关联（使用 INSERT OR IGNORE 避免重复）
		_, err = m.db.Exec(
			"INSERT OR IGNORE INTO file_tags (file_path, tag_id, added_at) VALUES (?, ?, ?)",
			filePath, tagID, now,
		)
		if err != nil {
			return fmt.Errorf("添加文件标签失败：%w", err)
		}
	}

	return nil
}

// RemoveTagsFromFile 从文件移除标签
func (m *Manager) RemoveTagsFromFile(filePath string, tagIDs []string) error {
	if filePath == "" {
		return ErrInvalidFilePath
	}
	if len(tagIDs) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tagID := range tagIDs {
		_, err := m.db.Exec(
			"DELETE FROM file_tags WHERE file_path = ? AND tag_id = ?",
			filePath, tagID,
		)
		if err != nil {
			return fmt.Errorf("移除文件标签失败：%w", err)
		}
	}

	return nil
}

// SetFileTags 设置文件的标签（替换所有现有标签）
func (m *Manager) SetFileTags(filePath string, tagIDs []string) error {
	if filePath == "" {
		return ErrInvalidFilePath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 开始事务
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除现有标签
	_, err = tx.Exec("DELETE FROM file_tags WHERE file_path = ?", filePath)
	if err != nil {
		return err
	}

	// 添加新标签
	now := time.Now()
	for _, tagID := range tagIDs {
		// 检查标签是否存在
		var exists int
		err := tx.QueryRow("SELECT 1 FROM tags WHERE id = ?", tagID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrInvalidTagID
		}
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			"INSERT INTO file_tags (file_path, tag_id, added_at) VALUES (?, ?, ?)",
			filePath, tagID, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTagsForFile 获取文件的所有标签
func (m *Manager) GetTagsForFile(filePath string) ([]*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `
		SELECT t.id, t.name, t.color, t.icon, t.grp, t.created_at, t.updated_at 
		FROM tags t 
		INNER JOIN file_tags ft ON t.id = ft.tag_id 
		WHERE ft.file_path = ?
		ORDER BY t.name
	`
	rows, err := m.db.Query(query, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag := &Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
			&tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetFilesByTags 获取拥有指定标签的文件列表
// matchAll=true 表示必须包含所有标签，false 表示包含任意一个标签
func (m *Manager) GetFilesByTags(tagIDs []string, matchAll bool) ([]string, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var query string
	var args []interface{}

	if matchAll {
		// 必须包含所有标签
		// 使用子查询确保文件包含所有指定的标签
		query = `
			SELECT DISTINCT file_path FROM file_tags 
			WHERE tag_id IN (` + placeholders(len(tagIDs)) + `)
			GROUP BY file_path 
			HAVING COUNT(DISTINCT tag_id) = ?
		`
		for _, id := range tagIDs {
			args = append(args, id)
		}
		args = append(args, len(tagIDs))
	} else {
		// 包含任意一个标签
		query = `
			SELECT DISTINCT file_path FROM file_tags 
			WHERE tag_id IN (` + placeholders(len(tagIDs)) + `)
		`
		for _, id := range tagIDs {
			args = append(args, id)
		}
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, err
		}
		files = append(files, filePath)
	}

	return files, rows.Err()
}

// GetFileTagCount 获取文件的标签数量
func (m *Manager) GetFileTagCount(filePath string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM file_tags WHERE file_path = ?", filePath).Scan(&count)
	return count, err
}

// GetTagUsageCount 获取标签的使用次数
func (m *Manager) GetTagUsageCount(tagID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM file_tags WHERE tag_id = ?", tagID).Scan(&count)
	return count, err
}

// ========== 分组管理 ==========

// ListGroups 列出所有标签分组
func (m *Manager) ListGroups() ([]*TagGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `
		SELECT grp, COUNT(*) as count 
		FROM tags 
		WHERE grp != '' 
		GROUP BY grp 
		ORDER BY grp
	`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*TagGroup
	for rows.Next() {
		group := &TagGroup{}
		if err := rows.Scan(&group.Name, &group.Count); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, rows.Err()
}

// ========== 统计信息 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{}

	// 标签总数
	err := m.db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&stats.TotalTags)
	if err != nil {
		return nil, err
	}

	// 有标签的文件数
	err = m.db.QueryRow("SELECT COUNT(DISTINCT file_path) FROM file_tags").Scan(&stats.TotalFiles)
	if err != nil {
		return nil, err
	}

	// 有分组的标签数
	err = m.db.QueryRow("SELECT COUNT(*) FROM tags WHERE grp != ''").Scan(&stats.TotalGrouped)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ========== 批量操作 ==========

// BatchAddTagsToFile 批量为多个文件添加标签
func (m *Manager) BatchAddTagsToFile(filePaths []string, tagIDs []string) error {
	if len(filePaths) == 0 || len(tagIDs) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, filePath := range filePaths {
		for _, tagID := range tagIDs {
			_, err := tx.Exec(
				"INSERT OR IGNORE INTO file_tags (file_path, tag_id, added_at) VALUES (?, ?, ?)",
				filePath, tagID, now,
			)
			if err != nil {
				return fmt.Errorf("批量添加文件标签失败：%w", err)
			}
		}
	}

	return tx.Commit()
}

// ClearFileTags 清除文件的所有标签
func (m *Manager) ClearFileTags(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("DELETE FROM file_tags WHERE file_path = ?", filePath)
	return err
}

// ClearAllTags 清除所有标签数据
func (m *Manager) ClearAllTags() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM file_tags")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM tags")
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ========== 搜索 ==========

// SearchTags 搜索标签（按名称模糊匹配）
func (m *Manager) SearchTags(keyword string) ([]*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `SELECT id, name, color, icon, grp, created_at, updated_at FROM tags WHERE name LIKE ? ORDER BY name`
	rows, err := m.db.Query(query, "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag := &Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Color, &tag.Icon, &tag.Group,
			&tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// SearchFilesByTags 按标签搜索文件（支持关键词和标签组合）
func (m *Manager) SearchFilesByTags(keyword string, tagIDs []string, matchAll bool) ([]string, error) {
	if keyword == "" && len(tagIDs) == 0 {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var query string
	var args []interface{}

	if len(tagIDs) == 0 {
		// 仅按关键词搜索文件路径
		query = `SELECT DISTINCT file_path FROM file_tags WHERE file_path LIKE ?`
		args = append(args, "%"+keyword+"%")
	} else if keyword == "" {
		// 仅按标签搜索
		return m.GetFilesByTags(tagIDs, matchAll)
	} else {
		// 组合搜索
		if matchAll {
			query = `
				SELECT DISTINCT file_path FROM file_tags 
				WHERE file_path LIKE ? AND tag_id IN (` + placeholders(len(tagIDs)) + `)
				GROUP BY file_path 
				HAVING COUNT(DISTINCT tag_id) = ?
			`
			args = append(args, "%"+keyword+"%")
			for _, id := range tagIDs {
				args = append(args, id)
			}
			args = append(args, len(tagIDs))
		} else {
			query = `
				SELECT DISTINCT file_path FROM file_tags 
				WHERE file_path LIKE ? AND tag_id IN (` + placeholders(len(tagIDs)) + `)
			`
			args = append(args, "%"+keyword+"%")
			for _, id := range tagIDs {
				args = append(args, id)
			}
		}
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, err
		}
		files = append(files, filePath)
	}

	return files, rows.Err()
}

// Close 关闭数据库连接
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// placeholders 生成 SQL 占位符
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	result := "?"
	for i := 1; i < n; i++ {
		result += ", ?"
	}
	return result
}