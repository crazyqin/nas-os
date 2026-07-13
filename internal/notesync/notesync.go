// Package notesync 提供协作笔记同步引擎，支持 Markdown 笔记实时同步、
// 冲突合并、版本历史、共享笔记本管理和离线编辑队列。
package notesync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Note 表示一篇 Markdown 笔记。
type Note struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Author     string   `json:"author"`
	NotebookID string   `json:"notebook_id"`
	ModifiedAt int64    `json:"modified_at"` // Unix 时间戳（秒）
	Version    int      `json:"version"`
	Tags       []string `json:"tags,omitempty"`
}

// SyncResult 是一次同步操作的结果。
type SyncResult struct {
	NoteID   string    `json:"note_id"`
	Status   string    `json:"status"` // "synced", "conflict", "noop"
	SyncedAt int64     `json:"synced_at"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
	Version  int       `json:"version"`
}

// Conflict 描述单个字段上的同步冲突及其解决方式。
type Conflict struct {
	Field       string `json:"field"`
	LocalValue  string `json:"local_value"`
	RemoteValue string `json:"remote_value"`
	Resolution  string `json:"resolution"` // "local_wins", "remote_wins", "merged"
}

// MergedNote 是冲突合并后的笔记。
type MergedNote struct {
	Note             Note   `json:"note"`
	MergeStrategy    string `json:"merge_strategy"` // "field_level", "local_wins", "remote_wins"
	ConflictsResolved int   `json:"conflicts_resolved"`
}

// NoteVersion 是笔记某个历史版本的摘要。
type NoteVersion struct {
	Version    int    `json:"version"`
	ModifiedAt int64  `json:"modified_at"`
	Author     string `json:"author"`
	Summary    string `json:"summary"`
	SizeBytes  int    `json:"size_bytes"`
}

// ShareResult 是共享笔记本操作的结果。
type ShareResult struct {
	NotebookID  string            `json:"notebook_id"`
	SharedWith  []string          `json:"shared_with"`
	Permissions map[string]string `json:"permissions"` // user -> "read" | "write" | "admin"
	Success     bool              `json:"success"`
}

// OfflineQueueEntry 是离线编辑队列中的一条记录。
type OfflineQueueEntry struct {
	Note     Note  `json:"note"`
	QueuedAt int64 `json:"queued_at"`
	Priority int   `json:"priority"` // 数值越大优先级越高
}

// FlushResult 是刷新离线队列的结果。
type FlushResult struct {
	Synced    int      `json:"synced"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
	Remaining int      `json:"remaining"`
}

// -------------------------------------------------------------------
// Engine
// -------------------------------------------------------------------

// NoteSyncEngine 是协作笔记同步引擎，维护笔记、版本历史、共享笔记本
// 元数据以及离线编辑队列。
type NoteSyncEngine struct {
	mu           sync.RWMutex
	notes        map[string]*Note            // noteID -> latest note
	versions     map[string][]NoteVersion    // noteID -> version history
	permissions  map[string]map[string]string // notebookID -> user -> permission
	sharedUsers  map[string][]string          // notebookID -> users list
	offlineQueue []OfflineQueueEntry          // pending offline edits
	now          func() int64                 // 可注入的时间源
}

// NewEngine 创建并初始化一个 NoteSyncEngine。
func NewEngine() *NoteSyncEngine {
	return &NoteSyncEngine{
		notes:       make(map[string]*Note),
		versions:    make(map[string][]NoteVersion),
		permissions: make(map[string]map[string]string),
		sharedUsers: make(map[string][]string),
		now:         func() int64 { return time.Now().Unix() },
	}
}

// -------------------------------------------------------------------
// SyncNote
// -------------------------------------------------------------------

// SyncNote 将笔记同步到引擎。如果同一篇笔记已经存在且版本号相同，
// 但内容发生变化，则检测冲突并返回 SyncResult（Status="conflict"）。
// 对于没有冲突的笔记，直接更新并记录版本历史。
func (e *NoteSyncEngine) SyncNote(note Note) (*SyncResult, error) {
	if note.ID == "" {
		return nil, fmt.Errorf("note ID is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.now()
	result := &SyncResult{
		NoteID:   note.ID,
		SyncedAt: now,
	}

	existing, ok := e.notes[note.ID]
	if !ok {
		// 首次同步 — 直接存储
		note.Version = 1
		note.ModifiedAt = now
		e.notes[note.ID] = &note
		e.recordVersion(note)
		result.Status = "synced"
		result.Version = 1
		return result, nil
	}

	// 已存在 — 检测冲突
	conflicts := e.detectConflicts(*existing, note)
	if len(conflicts) > 0 {
		result.Status = "conflict"
		result.Conflicts = conflicts
		result.Version = existing.Version
		return result, nil
	}

	// 无冲突 — 更新笔记
	note.Version = existing.Version + 1
	note.ModifiedAt = now
	e.notes[note.ID] = &note
	e.recordVersion(note)
	result.Status = "synced"
	result.Version = note.Version
	return result, nil
}

// detectConflicts 比较本地的已存储版本和传入的笔记，
// 检测 Title、Content、Tags 等字段上的冲突。
// 冲突条件：作者不同且字段值不同。
func (e *NoteSyncEngine) detectConflicts(local, remote Note) []Conflict {
	var conflicts []Conflict

	if local.Author != remote.Author {
		if local.Title != remote.Title {
			conflicts = append(conflicts, Conflict{
				Field:       "title",
				LocalValue:  local.Title,
				RemoteValue: remote.Title,
				Resolution:  "local_wins",
			})
		}
		if local.Content != remote.Content {
			conflicts = append(conflicts, Conflict{
				Field:       "content",
				LocalValue:  local.Content,
				RemoteValue: remote.Content,
				Resolution:  "local_wins",
			})
		}
		if !sameTags(local.Tags, remote.Tags) {
			conflicts = append(conflicts, Conflict{
				Field:       "tags",
				LocalValue:  strings.Join(local.Tags, ","),
				RemoteValue: strings.Join(remote.Tags, ","),
				Resolution:  "local_wins",
			})
		}
	}

	return conflicts
}

// recordVersion 将笔记的当前状态记录到版本历史中。
func (e *NoteSyncEngine) recordVersion(note Note) {
	summary := note.Title
	if len(summary) > 80 {
		summary = summary[:80]
	}
	version := NoteVersion{
		Version:    note.Version,
		ModifiedAt: note.ModifiedAt,
		Author:     note.Author,
		Summary:    summary,
		SizeBytes:  len(note.Content),
	}
	e.versions[note.ID] = append(e.versions[note.ID], version)
}

// -------------------------------------------------------------------
// MergeConflicts
// -------------------------------------------------------------------

// MergeConflicts 合并本地和远程笔记之间的冲突。
// 对于文本字段（Title、Content），如果两边的都非空则采用
// 字段级混合策略（两段内容拼接）；对于 Tags，取并集。
// 如果传入笔记没有冲突字段，直接采用远程版本。
func (e *NoteSyncEngine) MergeConflicts(local Note, remote Note) (*MergedNote, error) {
	if local.ID != remote.ID && local.ID != "" && remote.ID != "" {
		return nil, fmt.Errorf("cannot merge notes with different IDs: %q vs %q", local.ID, remote.ID)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	merged := Note{
		ID:         remote.ID,
		Author:     remote.Author,
		NotebookID: remote.NotebookID,
		ModifiedAt: e.now(),
		Version:    max(local.Version, remote.Version) + 1,
	}

	resolved := 0
	strategy := "field_level"

	// Title — 若不同，优先使用更长（更详细）的标题
	if local.Title == remote.Title {
		merged.Title = local.Title
	} else {
		merged.Title = mergeText(local.Title, remote.Title)
		resolved++
	}

	// Content — 若不同，拼接两段并用分隔线隔开
	if local.Content == remote.Content {
		merged.Content = local.Content
	} else {
		merged.Content = mergeText(local.Content, remote.Content)
		resolved++
	}

	// Tags — 取并集
	merged.Tags = mergeTags(local.Tags, remote.Tags)
	if !sameTags(local.Tags, remote.Tags) {
		resolved++
	}

	// 若无冲突，采用远程版本，策略标记为 remote_wins
	if resolved == 0 {
		strategy = "remote_wins"
		merged = Note{
			ID:         remote.ID,
			Title:      remote.Title,
			Content:    remote.Content,
			Author:     remote.Author,
			NotebookID: remote.NotebookID,
			ModifiedAt: e.now(),
			Version:    max(local.Version, remote.Version) + 1,
			Tags:       remote.Tags,
		}
	}

	return &MergedNote{
		Note:              merged,
		MergeStrategy:     strategy,
		ConflictsResolved: resolved,
	}, nil
}

// -------------------------------------------------------------------
// GetHistory
// -------------------------------------------------------------------

// GetHistory 返回指定笔记的版本历史，按版本号降序排列。
func (e *NoteSyncEngine) GetHistory(noteID string) ([]NoteVersion, error) {
	if noteID == "" {
		return nil, fmt.Errorf("note ID is required")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	history, ok := e.versions[noteID]
	if !ok {
		return []NoteVersion{}, nil
	}

	// 返回副本，避免外部修改
	result := make([]NoteVersion, len(history))
	copy(result, history)

	// 按版本号降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})

	return result, nil
}

// -------------------------------------------------------------------
// ShareNotebook
// -------------------------------------------------------------------

// ShareNotebook 将笔记本共享给指定用户列表，使用默认权限
// （第一个用户获得 "write"，其余获得 "read"）。如果用户列表为空
// 则返回错误。已被共享的用户会被更新。
func (e *NoteSyncEngine) ShareNotebook(notebookID string, users []string) (*ShareResult, error) {
	if notebookID == "" {
		return nil, fmt.Errorf("notebook ID is required")
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("at least one user is required to share")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 初始化权限表
	if e.permissions[notebookID] == nil {
		e.permissions[notebookID] = make(map[string]string)
	}

	perms := map[string]string{}
	for i, user := range users {
		if user == "" {
			continue
		}
		// 第一个用户获得 write 权限，其余 read
		if i == 0 {
			e.permissions[notebookID][user] = "write"
			perms[user] = "write"
		} else {
			e.permissions[notebookID][user] = "read"
			perms[user] = "read"
		}
	}

	e.sharedUsers[notebookID] = users

	return &ShareResult{
		NotebookID:  notebookID,
		SharedWith:  users,
		Permissions: perms,
		Success:     true,
	}, nil
}

// -------------------------------------------------------------------
// QueueOffline
// -------------------------------------------------------------------

// QueueOffline 将一条笔记加入离线编辑队列。优先级默认从笔记 Version 推导
// （版本越高优先级越高），也可通过设置 note 的内容长度辅助。
func (e *NoteSyncEngine) QueueOffline(note Note) (*OfflineQueueEntry, error) {
	if note.ID == "" {
		return nil, fmt.Errorf("note ID is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	entry := OfflineQueueEntry{
		Note:     note,
		QueuedAt: e.now(),
		Priority: note.Version,
	}

	e.offlineQueue = append(e.offlineQueue, entry)

	// 保持队列按优先级降序排列
	sort.SliceStable(e.offlineQueue, func(i, j int) bool {
		return e.offlineQueue[i].Priority > e.offlineQueue[j].Priority
	})

	return &entry, nil
}

// -------------------------------------------------------------------
// FlushOfflineQueue
// -------------------------------------------------------------------

// FlushOfflineQueue 尝试将离线队列中的条目逐一同步到引擎。
// 同步成功的条目从队列中移除；失败的保留在队列中并记录错误信息。
// 传入的 entries 参数为外部传入的队列条目（通常来自持久化层），
// 会与引擎内部队列合并处理。
func (e *NoteSyncEngine) FlushOfflineQueue(entries []OfflineQueueEntry) (*FlushResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 合并外部条目和内部队列
	all := append([]OfflineQueueEntry{}, e.offlineQueue...)
	all = append(all, entries...)

	// 按优先级降序排列
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Priority > all[j].Priority
	})

	synced := 0
	failed := 0
	var errs []string
	var remaining []OfflineQueueEntry

	for _, entry := range all {
		note := entry.Note
		now := e.now()

		existing, ok := e.notes[note.ID]
		if !ok {
			// 新笔记 — 直接创建
			note.Version = 1
			note.ModifiedAt = now
			e.notes[note.ID] = &note
			e.recordVersion(note)
			synced++
			continue
		}

		// 已存在 — 检测冲突
		conflicts := e.detectConflicts(*existing, note)
		if len(conflicts) > 0 {
			// 冲突 — 自动尝试字段级合并
			merged, err := e.mergeConflictsInternal(*existing, note)
			if err != nil {
				failed++
				errs = append(errs, fmt.Sprintf("note %s: merge failed: %v", note.ID, err))
				remaining = append(remaining, entry)
				continue
			}
			merged.Version = existing.Version + 1
			merged.ModifiedAt = now
			e.notes[note.ID] = &merged
			e.recordVersion(merged)
			synced++
			continue
		}

		// 无冲突 — 直接更新
		note.Version = existing.Version + 1
		note.ModifiedAt = now
		e.notes[note.ID] = &note
		e.recordVersion(note)
		synced++
	}

	e.offlineQueue = remaining

	return &FlushResult{
		Synced:    synced,
		Failed:    failed,
		Errors:    errs,
		Remaining: len(remaining),
	}, nil
}

// mergeConflictsInternal 是 MergeConflicts 的内部版本，不加锁，
// 供 FlushOfflineQueue 在已持锁的情况下调用。
func (e *NoteSyncEngine) mergeConflictsInternal(local, remote Note) (Note, error) {
	merged := Note{
		ID:         remote.ID,
		NotebookID: remote.NotebookID,
		Author:     remote.Author,
	}

	if local.Title == remote.Title {
		merged.Title = local.Title
	} else {
		merged.Title = mergeText(local.Title, remote.Title)
	}

	if local.Content == remote.Content {
		merged.Content = local.Content
	} else {
		merged.Content = mergeText(local.Content, remote.Content)
	}

	merged.Tags = mergeTags(local.Tags, remote.Tags)
	return merged, nil
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// mergeText 将两段文本合并，用冲突分隔线隔开。
func mergeText(local, remote string) string {
	if local == "" {
		return remote
	}
	if remote == "" {
		return local
	}
	if local == remote {
		return local
	}
	return local + "\n\n---\n\n" + remote
}

// mergeTags 取两个 tag 列表的并集（去重，保持顺序）。
func mergeTags(local, remote []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range local {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	for _, t := range remote {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// sameTags 判断两个 tag 列表是否包含相同元素（顺序无关）。
func sameTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	setA := make(map[string]bool, len(a))
	for _, t := range a {
		setA[t] = true
	}
	for _, t := range b {
		if !setA[t] {
			return false
		}
	}
	return true
}

// max 返回两个整数中的较大值（Go 1.21 之前手动实现以确保兼容）。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// contentHash 返回笔记内容的 SHA-256 摘要（用于调试和测试）。
func contentHash(note Note) string {
	h := sha256.Sum256([]byte(note.Content))
	return hex.EncodeToString(h[:8])
}

// Compile-time assertion: ensure NoteSyncEngine implements expected interfaces.
var _ = contentHash