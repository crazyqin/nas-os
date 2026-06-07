// Package tags 提供共享标签协作系统
// 支持标签的创建、分享、文件关联和模糊搜索
package tags

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SharedLabel 共享标签.
type SharedLabel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	SharedWith  []string  `json:"sharedWith"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SharedLabelInput 创建/更新共享标签输入.
type SharedLabelInput struct {
	Name        string `json:"name" binding:"required"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Owner       string `json:"owner" binding:"required"`
}

// ShareLabelInput 分享标签输入.
type ShareLabelInput struct {
	Users []string `json:"users" binding:"required"`
}

// AssignLabelInput 分配标签输入.
type AssignLabelInput struct {
	FileID string `json:"fileId" binding:"required"`
}

// 共享标签错误定义.
var (
	ErrSharedLabelNotFound  = fmt.Errorf("共享标签不存在")
	ErrSharedLabelExists    = fmt.Errorf("标签名称已存在")
	ErrNotSharedOwner       = fmt.Errorf("非标签所有者")
	ErrAlreadySharedWith    = fmt.Errorf("已经分享给该用户")
	ErrNotSharedWith        = fmt.Errorf("未分享给该用户")
	ErrLabelAlreadyAssigned = fmt.Errorf("标签已分配给该文件")
	ErrLabelNotAssigned     = fmt.Errorf("标签未分配给该文件")
)

// LabelManager 共享标签管理器.
type LabelManager struct {
	db *sql.DB
}

// NewLabelManager 创建共享标签管理器.
func NewLabelManager(db *sql.DB) (*LabelManager, error) {
	lm := &LabelManager{db: db}
	if err := lm.initDB(); err != nil {
		return nil, fmt.Errorf("初始化共享标签数据库失败：%w", err)
	}
	return lm, nil
}

// initDB 初始化数据库表.
func (lm *LabelManager) initDB() error {
	ctx := context.Background()
	schema := `
	CREATE TABLE IF NOT EXISTS shared_labels (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		color TEXT DEFAULT '#3498db',
		description TEXT DEFAULT '',
		owner TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_shared_labels_owner ON shared_labels(owner);
	CREATE INDEX IF NOT EXISTS idx_shared_labels_name ON shared_labels(name);

	CREATE TABLE IF NOT EXISTS shared_label_users (
		label_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		PRIMARY KEY (label_id, user_id),
		FOREIGN KEY (label_id) REFERENCES shared_labels(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_shared_label_users_user ON shared_label_users(user_id);

	CREATE TABLE IF NOT EXISTS label_files (
		label_id TEXT NOT NULL,
		file_id TEXT NOT NULL,
		assigned_at DATETIME NOT NULL,
		PRIMARY KEY (label_id, file_id),
		FOREIGN KEY (label_id) REFERENCES shared_labels(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_label_files_file ON label_files(file_id);
	CREATE INDEX IF NOT EXISTS idx_label_files_label ON label_files(label_id);
	`
	_, err := lm.db.ExecContext(ctx, schema)
	return err
}

// CreateLabel 创建共享标签.
func (lm *LabelManager) CreateLabel(input SharedLabelInput) (*SharedLabel, error) {
	ctx := context.Background()

	// 检查同名标签
	var exists int
	err := lm.db.QueryRowContext(ctx, "SELECT 1 FROM shared_labels WHERE name = ? AND owner = ?", input.Name, input.Owner).Scan(&exists)
	if err == nil {
		return nil, ErrSharedLabelExists
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now()
	label := &SharedLabel{
		ID:          generateID(),
		Name:        strings.TrimSpace(input.Name),
		Color:       input.Color,
		Description: input.Description,
		Owner:       input.Owner,
		SharedWith:  []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if label.Color == "" {
		label.Color = "#3498db"
	}

	query := `INSERT INTO shared_labels (id, name, color, description, owner, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = lm.db.ExecContext(ctx, query, label.ID, label.Name, label.Color, label.Description, label.Owner, label.CreatedAt, label.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("创建共享标签失败：%w", err)
	}

	return label, nil
}

// GetLabel 获取共享标签.
func (lm *LabelManager) GetLabel(id string) (*SharedLabel, error) {
	ctx := context.Background()

	label := &SharedLabel{}
	query := `SELECT id, name, color, description, owner, created_at, updated_at FROM shared_labels WHERE id = ?`
	err := lm.db.QueryRowContext(ctx, query, id).Scan(
		&label.ID, &label.Name, &label.Color, &label.Description,
		&label.Owner, &label.CreatedAt, &label.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrSharedLabelNotFound
	}
	if err != nil {
		return nil, err
	}

	// 加载分享用户列表
	label.SharedWith, _ = lm.getSharedUsers(id)

	return label, nil
}

// DeleteLabel 删除共享标签.
func (lm *LabelManager) DeleteLabel(id string, owner string) error {
	ctx := context.Background()

	// 验证所有者
	var actualOwner string
	err := lm.db.QueryRowContext(ctx, "SELECT owner FROM shared_labels WHERE id = ?", id).Scan(&actualOwner)
	if err == sql.ErrNoRows {
		return ErrSharedLabelNotFound
	}
	if err != nil {
		return err
	}
	if actualOwner != owner {
		return ErrNotSharedOwner
	}

	// 删除关联数据（CASCADE 会处理，但显式清理更安全）
	_, _ = lm.db.ExecContext(ctx, "DELETE FROM shared_label_users WHERE label_id = ?", id)
	_, _ = lm.db.ExecContext(ctx, "DELETE FROM label_files WHERE label_id = ?", id)

	result, err := lm.db.ExecContext(ctx, "DELETE FROM shared_labels WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除共享标签失败：%w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrSharedLabelNotFound
	}
	return nil
}

// UpdateLabel 更新共享标签.
func (lm *LabelManager) UpdateLabel(id string, input SharedLabelInput) (*SharedLabel, error) {
	ctx := context.Background()

	// 验证存在和所有者
	var actualOwner string
	err := lm.db.QueryRowContext(ctx, "SELECT owner FROM shared_labels WHERE id = ?", id).Scan(&actualOwner)
	if err == sql.ErrNoRows {
		return nil, ErrSharedLabelNotFound
	}
	if err != nil {
		return nil, err
	}
	if actualOwner != input.Owner {
		return nil, ErrNotSharedOwner
	}

	// 检查同名冲突（排除自身）
	if strings.TrimSpace(input.Name) != "" {
		var conflictID string
		err := lm.db.QueryRowContext(ctx, "SELECT id FROM shared_labels WHERE name = ? AND owner = ? AND id != ?",
			input.Name, input.Owner, id).Scan(&conflictID)
		if err == nil {
			return nil, ErrSharedLabelExists
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	now := time.Now()
	query := `UPDATE shared_labels SET 
		name = COALESCE(NULLIF(?, ''), name), 
		color = COALESCE(NULLIF(?, ''), color), 
		description = COALESCE(NULLIF(?, ''), description), 
		updated_at = ? 
		WHERE id = ?`
	_, err = lm.db.ExecContext(ctx, query, input.Name, input.Color, input.Description, now, id)
	if err != nil {
		return nil, fmt.Errorf("更新共享标签失败：%w", err)
	}

	return lm.GetLabel(id)
}

// ListLabels 列出用户可见的所有标签（拥有的 + 被分享的）.
func (lm *LabelManager) ListLabels(owner string) ([]*SharedLabel, error) {
	ctx := context.Background()

	query := `
		SELECT DISTINCT l.id, l.name, l.color, l.description, l.owner, l.created_at, l.updated_at
		FROM shared_labels l
		LEFT JOIN shared_label_users su ON l.id = su.label_id
		WHERE l.owner = ? OR su.user_id = ?
		ORDER BY l.name
	`
	rows, err := lm.db.QueryContext(ctx, query, owner, owner)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var labels []*SharedLabel
	for rows.Next() {
		label := &SharedLabel{}
		err := rows.Scan(
			&label.ID, &label.Name, &label.Color, &label.Description,
			&label.Owner, &label.CreatedAt, &label.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		label.SharedWith, _ = lm.getSharedUsers(label.ID)
		labels = append(labels, label)
	}

	return labels, rows.Err()
}

// ShareLabel 分享标签给指定用户.
func (lm *LabelManager) ShareLabel(labelID string, users []string, owner string) error {
	ctx := context.Background()

	// 验证所有者
	var actualOwner string
	err := lm.db.QueryRowContext(ctx, "SELECT owner FROM shared_labels WHERE id = ?", labelID).Scan(&actualOwner)
	if err == sql.ErrNoRows {
		return ErrSharedLabelNotFound
	}
	if err != nil {
		return err
	}
	if actualOwner != owner {
		return ErrNotSharedOwner
	}

	// 批量插入分享关系
	for _, userID := range users {
		userID = strings.TrimSpace(userID)
		if userID == "" || userID == owner {
			continue
		}
		_, err := lm.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO shared_label_users (label_id, user_id) VALUES (?, ?)",
			labelID, userID,
		)
		if err != nil {
			return fmt.Errorf("分享标签失败：%w", err)
		}
	}
	return nil
}

// UnshareLabel 取消分享标签给指定用户.
func (lm *LabelManager) UnshareLabel(labelID string, users []string, owner string) error {
	ctx := context.Background()

	// 验证所有者
	var actualOwner string
	err := lm.db.QueryRowContext(ctx, "SELECT owner FROM shared_labels WHERE id = ?", labelID).Scan(&actualOwner)
	if err == sql.ErrNoRows {
		return ErrSharedLabelNotFound
	}
	if err != nil {
		return err
	}
	if actualOwner != owner {
		return ErrNotSharedOwner
	}

	for _, userID := range users {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		_, err := lm.db.ExecContext(ctx,
			"DELETE FROM shared_label_users WHERE label_id = ? AND user_id = ?",
			labelID, userID,
		)
		if err != nil {
			return fmt.Errorf("取消分享失败：%w", err)
		}
	}
	return nil
}

// AssignLabel 分配标签给文件.
func (lm *LabelManager) AssignLabel(fileID string, labelID string) error {
	ctx := context.Background()

	// 验证标签存在
	var exists int
	err := lm.db.QueryRowContext(ctx, "SELECT 1 FROM shared_labels WHERE id = ?", labelID).Scan(&exists)
	if err == sql.ErrNoRows {
		return ErrSharedLabelNotFound
	}
	if err != nil {
		return err
	}

	_, err = lm.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO label_files (label_id, file_id, assigned_at) VALUES (?, ?, ?)",
		labelID, fileID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("分配标签失败：%w", err)
	}
	return nil
}

// RemoveLabel 移除文件上的标签.
func (lm *LabelManager) RemoveLabel(fileID string, labelID string) error {
	ctx := context.Background()

	result, err := lm.db.ExecContext(ctx,
		"DELETE FROM label_files WHERE label_id = ? AND file_id = ?",
		labelID, fileID,
	)
	if err != nil {
		return fmt.Errorf("移除标签失败：%w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrLabelNotAssigned
	}
	return nil
}

// GetFilesByLabel 获取标签关联的所有文件ID.
func (lm *LabelManager) GetFilesByLabel(labelID string) ([]string, error) {
	ctx := context.Background()

	rows, err := lm.db.QueryContext(ctx, "SELECT file_id FROM label_files WHERE label_id = ? ORDER BY assigned_at", labelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var files []string
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err != nil {
			return nil, err
		}
		files = append(files, fileID)
	}
	return files, rows.Err()
}

// SearchLabels 模糊搜索标签（按名称和描述）.
func (lm *LabelManager) SearchLabels(query string, owner string) ([]*SharedLabel, error) {
	ctx := context.Background()

	sqlQuery := `
		SELECT DISTINCT l.id, l.name, l.color, l.description, l.owner, l.created_at, l.updated_at
		FROM shared_labels l
		LEFT JOIN shared_label_users su ON l.id = su.label_id
		WHERE (l.owner = ? OR su.user_id = ?) AND (l.name LIKE ? OR l.description LIKE ?)
		ORDER BY l.name
	`
	pattern := "%" + query + "%"
	rows, err := lm.db.QueryContext(ctx, sqlQuery, owner, owner, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var labels []*SharedLabel
	for rows.Next() {
		label := &SharedLabel{}
		err := rows.Scan(
			&label.ID, &label.Name, &label.Color, &label.Description,
			&label.Owner, &label.CreatedAt, &label.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		label.SharedWith, _ = lm.getSharedUsers(label.ID)
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

// GetLabelStats 获取标签统计信息.
func (lm *LabelManager) GetLabelStats(labelID string) (fileCount int, shareCount int, err error) {
	ctx := context.Background()

	err = lm.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM label_files WHERE label_id = ?", labelID).Scan(&fileCount)
	if err != nil {
		return 0, 0, err
	}

	err = lm.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shared_label_users WHERE label_id = ?", labelID).Scan(&shareCount)
	if err != nil {
		return 0, 0, err
	}

	return fileCount, shareCount, nil
}

// Close 关闭管理器.
func (lm *LabelManager) Close() error {
	// 数据库连接由外部管理，此处不关闭
	return nil
}

// getSharedUsers 获取标签分享的用户列表.
func (lm *LabelManager) getSharedUsers(labelID string) ([]string, error) {
	ctx := context.Background()

	rows, err := lm.db.QueryContext(ctx, "SELECT user_id FROM shared_label_users WHERE label_id = ?", labelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		users = append(users, userID)
	}
	return users, rows.Err()
}
