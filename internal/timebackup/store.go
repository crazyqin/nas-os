// Package timebackup 提供文件/目录版本备份与恢复功能
package timebackup

import (
	"database/sql"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// Store 备份任务与快照持久化存储.
type Store struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewStore 创建存储实例.
func NewStore(db *sql.DB, logger *zap.Logger) *Store {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Store{db: db, logger: logger}
}

// Init 初始化数据库表.
func (s *Store) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS timebackup_tasks (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS timebackup_snapshots (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_tb_snapshots_task ON timebackup_snapshots(task_id);
	`
	_, err := s.db.Exec(query)
	if err != nil {
		s.logger.Error("failed to init timebackup store", zap.Error(err))
	}
	return err
}

// SaveTask 保存或更新任务.
func (s *Store) SaveTask(task *BackupTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO timebackup_tasks (id, data, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = ?, updated_at = ?
	`, task.ID, string(data), task.CreatedAt.Format(time.RFC3339),
		task.UpdatedAt.Format(time.RFC3339),
		string(data), task.UpdatedAt.Format(time.RFC3339))
	return err
}

// LoadTasks 加载所有任务.
func (s *Store) LoadTasks() ([]*BackupTask, error) {
	rows, err := s.db.Query(`SELECT data FROM timebackup_tasks`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tasks []*BackupTask
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		t := &BackupTask{}
		if err := json.Unmarshal([]byte(data), t); err != nil {
			s.logger.Warn("failed to unmarshal task", zap.Error(err))
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// DeleteTask 删除任务.
func (s *Store) DeleteTask(taskID string) error {
	_, err := s.db.Exec(`DELETE FROM timebackup_tasks WHERE id = ?`, taskID)
	return err
}

// SaveSnapshot 保存快照.
func (s *Store) SaveSnapshot(snap *Snapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO timebackup_snapshots (id, task_id, data, created_at)
		VALUES (?, ?, ?, ?)
	`, snap.ID, snap.TaskID, string(data), snap.CreatedAt.Format(time.RFC3339))
	return err
}

// LoadSnapshots 加载所有快照.
func (s *Store) LoadSnapshots() ([]*Snapshot, error) {
	rows, err := s.db.Query(`SELECT data FROM timebackup_snapshots`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var snapshots []*Snapshot
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		snap := &Snapshot{}
		if err := json.Unmarshal([]byte(data), snap); err != nil {
			s.logger.Warn("failed to unmarshal snapshot", zap.Error(err))
			continue
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, rows.Err()
}

// DeleteSnapshot 删除快照.
func (s *Store) DeleteSnapshot(snapshotID string) error {
	_, err := s.db.Exec(`DELETE FROM timebackup_snapshots WHERE id = ?`, snapshotID)
	return err
}

// DeleteSnapshotsByTask 删除任务的所有快照.
func (s *Store) DeleteSnapshotsByTask(taskID string) error {
	_, err := s.db.Exec(`DELETE FROM timebackup_snapshots WHERE task_id = ?`, taskID)
	return err
}
