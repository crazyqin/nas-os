package snapshotmgr

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ZFSSnapshotConfig ZFS 专用快照配置
// 参考群晖 Snapshot Replication 和 TrueNAS ZFS 快照管理。
type ZFSSnapshotConfig struct {
	Pool         string `json:"pool"`          // ZFS pool 名
	Dataset      string `json:"dataset"`       // 数据集路径（空表示整个 pool）
	Recursive    bool   `json:"recursive"`     // 递归快照（包含所有子数据集）
	NamingFormat string `json:"naming_format"` // 快照命名格式，默认 "snap-%Y%m%d-%H%M%S"
}

// DefaultNamingFormat 默认快照命名格式.
const DefaultNamingFormat = "snap-20060102-150405"

// ZFSBookmark ZFS bookmark.
type ZFSBookmark struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`       // bookmark 名称
	SnapName  string    `json:"snap_name"`  // 源快照名称
	Pool      string    `json:"pool"`       // 所属 pool
	Dataset   string    `json:"dataset"`    // 所属数据集
	CreatedAt time.Time `json:"created_at"` // 创建时间
	GUID      string    `json:"guid"`       // ZFS 内部 GUID
}

// ZFSDiffResult 快照差异对比结果.
type ZFSDiffResult struct {
	SnapshotA  string         `json:"snapshot_a"` // 快照 A 名称
	SnapshotB  string         `json:"snapshot_b"` // 快照 B 名称
	Dataset    string         `json:"dataset"`    // 数据集
	Changes    []ZFSDiffEntry `json:"changes"`    // 变更列表
	TotalAdded int64          `json:"total_added"`
	TotalSize  int64          `json:"total_size"` // 总变更大小
}

// ZFSDiffEntry 单个差异条目.
type ZFSDiffEntry struct {
	Type     string `json:"type"`      // added / modified / removed / renamed
	Path     string `json:"path"`      // 文件/目录路径
	OldPath  string `json:"old_path"`  // 重命名时的旧路径（Type=renamed）
	Size     int64  `json:"size"`      // 文件大小
	Modify   string `json:"modify"`    // 修改类型标识
	RefQuota int64  `json:"ref_quota"` // 引用配额变化
}

// ZFSHold ZFS 快照保护（防止误删）.
type ZFSHold struct {
	SnapshotID string    `json:"snapshot_id"` // 快照标识 (pool/dataset@snap)
	Tag        string    `json:"tag"`         // 保护标签
	Reason     string    `json:"reason"`      // 保护原因
	CreatedAt  time.Time `json:"created_at"`  // 保护时间
	HolderRef  string    `json:"holder_ref"`  // 持有者引用（如 replication job ID）
}

// ZFSSnapshotManager ZFS 快照增强管理器.
type ZFSSnapshotManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	bookmarks map[string]*ZFSBookmark // key: "pool/dataset#bookmark"
	holds     map[string][]ZFSHold    // key: snapshot_id
}

// NewZFSSnapshotManager 创建 ZFS 快照管理器.
func NewZFSSnapshotManager(logger *zap.Logger) *ZFSSnapshotManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ZFSSnapshotManager{
		logger:    logger,
		bookmarks: make(map[string]*ZFSBookmark),
		holds:     make(map[string][]ZFSHold),
	}
}

// GenerateSnapshotName 根据命名格式生成 ZFS 快照名.
func (z *ZFSSnapshotManager) GenerateSnapshotName(config *ZFSSnapshotConfig) string {
	layout := config.NamingFormat
	if layout == "" {
		layout = DefaultNamingFormat
	}
	return time.Now().Format(layout)
}

// CreateSnapshotCommand 构建 zfs snapshot 命令参数.
func (z *ZFSSnapshotManager) CreateSnapshotCommand(config *ZFSSnapshotConfig, snapName string) []string {
	var target string
	if config.Dataset != "" {
		target = config.Dataset + "@" + snapName
	} else {
		target = config.Pool + "@" + snapName
	}

	args := []string{"snapshot"}
	if config.Recursive {
		args = append(args, "-r")
	}
	args = append(args, target)

	z.logger.Info("ZFS snapshot command",
		zap.Strings("args", args),
		zap.Bool("recursive", config.Recursive),
	)

	return args
}

// CreateBookmark 创建 ZFS bookmark
// ZFS bookmark 是对快照的轻量引用，即使快照被删除，bookmark 仍可用于增量复制。
func (z *ZFSSnapshotManager) CreateBookmark(pool, dataset, snapName, bookmarkName string) (*ZFSBookmark, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	var source string
	if dataset != "" {
		source = dataset + "@" + snapName
	} else {
		source = pool + "@" + snapName
	}

	var bmTarget string
	if dataset != "" {
		bmTarget = dataset + "#" + bookmarkName
	} else {
		bmTarget = pool + "#" + bookmarkName
	}

	// 检查是否已存在
	if _, exists := z.bookmarks[bmTarget]; exists {
		return nil, fmt.Errorf("bookmark %s already exists", bmTarget)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	bm := &ZFSBookmark{
		ID:        id,
		Name:      bookmarkName,
		SnapName:  snapName,
		Pool:      pool,
		Dataset:   dataset,
		CreatedAt: time.Now(),
		GUID:      id[:16],
	}

	z.bookmarks[bmTarget] = bm

	z.logger.Info("ZFS bookmark created",
		zap.String("bookmark", bmTarget),
		zap.String("source", source),
	)

	return bm, nil
}

// ListBookmarks 列出所有 bookmark.
func (z *ZFSSnapshotManager) ListBookmarks(pool, dataset string) []ZFSBookmark {
	z.mu.RLock()
	defer z.mu.RUnlock()

	var result []ZFSBookmark
	for key, bm := range z.bookmarks {
		if pool != "" && bm.Pool != pool {
			continue
		}
		if dataset != "" && bm.Dataset != dataset {
			continue
		}
		_ = key
		result = append(result, *bm)
	}
	return result
}

// DeleteBookmark 删除 bookmark.
func (z *ZFSSnapshotManager) DeleteBookmark(pool, dataset, bookmarkName string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	var bmTarget string
	if dataset != "" {
		bmTarget = dataset + "#" + bookmarkName
	} else {
		bmTarget = pool + "#" + bookmarkName
	}

	if _, exists := z.bookmarks[bmTarget]; !exists {
		return fmt.Errorf("bookmark %s not found", bmTarget)
	}

	delete(z.bookmarks, bmTarget)
	z.logger.Info("ZFS bookmark deleted", zap.String("bookmark", bmTarget))
	return nil
}

// Diff 快照差异对比
// 对比两个快照间的文件系统变更，对应 zfs diff 命令。
func (z *ZFSSnapshotManager) Diff(pool, dataset, snapA, snapB string) (*ZFSDiffResult, error) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	var sourceA, sourceB string
	if dataset != "" {
		sourceA = dataset + "@" + snapA
		sourceB = dataset + "@" + snapB
	} else {
		sourceA = pool + "@" + snapA
		sourceB = pool + "@" + snapB
	}

	result := &ZFSDiffResult{
		SnapshotA: sourceA,
		SnapshotB: sourceB,
		Dataset:   dataset,
		Changes:   []ZFSDiffEntry{},
	}

	// 实际实现需调用 zfs diff 命令并解析输出
	// 这里提供框架结构，具体条目在集成层填充
	z.logger.Info("ZFS diff computed",
		zap.String("snapshot_a", sourceA),
		zap.String("snapshot_b", sourceB),
	)

	return result, nil
}

// AddHold 添加快照保护.
func (z *ZFSSnapshotManager) AddHold(snapshotID, tag, reason, holderRef string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	// 同一 tag 不能重复添加
	for _, h := range z.holds[snapshotID] {
		if h.Tag == tag {
			return fmt.Errorf("hold tag %q already exists on snapshot %s", tag, snapshotID)
		}
	}

	hold := ZFSHold{
		SnapshotID: snapshotID,
		Tag:        tag,
		Reason:     reason,
		CreatedAt:  time.Now(),
		HolderRef:  holderRef,
	}

	z.holds[snapshotID] = append(z.holds[snapshotID], hold)

	z.logger.Info("ZFS hold added",
		zap.String("snapshot_id", snapshotID),
		zap.String("tag", tag),
		zap.String("reason", reason),
	)

	return nil
}

// RemoveHold 移除快照保护.
func (z *ZFSSnapshotManager) RemoveHold(snapshotID, tag string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	holds, ok := z.holds[snapshotID]
	if !ok {
		return fmt.Errorf("no holds found on snapshot %s", snapshotID)
	}

	for i, h := range holds {
		if h.Tag == tag {
			z.holds[snapshotID] = append(holds[:i], holds[i+1:]...)
			if len(z.holds[snapshotID]) == 0 {
				delete(z.holds, snapshotID)
			}
			z.logger.Info("ZFS hold removed",
				zap.String("snapshot_id", snapshotID),
				zap.String("tag", tag),
			)
			return nil
		}
	}

	return fmt.Errorf("hold tag %q not found on snapshot %s", tag, snapshotID)
}

// ListHolds 列出快照的所有保护.
func (z *ZFSSnapshotManager) ListHolds(snapshotID string) []ZFSHold {
	z.mu.RLock()
	defer z.mu.RUnlock()

	return append([]ZFSHold{}, z.holds[snapshotID]...)
}

// IsHeld 检查快照是否有保护（有保护的快照不可删除）.
func (z *ZFSSnapshotManager) IsHeld(snapshotID string) bool {
	z.mu.RLock()
	defer z.mu.RUnlock()

	return len(z.holds[snapshotID]) > 0
}

// CanDelete 检查快照是否可以删除（考虑 hold 保护）.
func (z *ZFSSnapshotManager) CanDelete(snapshotID string) (bool, []ZFSHold) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	holds := z.holds[snapshotID]
	return len(holds) == 0, append([]ZFSHold{}, holds...)
}

// ParseZFSPath 解析 ZFS 路径 (pool/dataset@snap 或 pool/dataset#bookmark).
func ParseZFSPath(path string) (pool, dataset, suffix, suffixType string) {
	var mainPart string
	if idx := strings.LastIndex(path, "@"); idx != -1 {
		mainPart = path[:idx]
		suffix = path[idx+1:]
		suffixType = "snapshot"
	} else if idx := strings.LastIndex(path, "#"); idx != -1 {
		mainPart = path[:idx]
		suffix = path[idx+1:]
		suffixType = "bookmark"
	} else {
		mainPart = path
	}

	parts := strings.SplitN(mainPart, "/", 2)
	pool = parts[0]
	if len(parts) > 1 {
		dataset = parts[1]
	}

	return
}
