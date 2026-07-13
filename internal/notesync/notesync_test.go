package notesync

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------
// NewEngine / SyncNote
// -------------------------------------------------------------------

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.notes == nil || e.versions == nil || e.permissions == nil {
		t.Fatal("NewEngine did not initialize internal maps")
	}
}

func TestSyncNote_NewNote(t *testing.T) {
	e := NewEngine()
	note := Note{
		ID:         "note-1",
		Title:      "Hello World",
		Content:    "# Hello\n\nThis is a test note.",
		Author:     "alice",
		NotebookID: "nb-1",
		Tags:       []string{"test", "hello"},
	}

	result, err := e.SyncNote(note)
	if err != nil {
		t.Fatalf("SyncNote failed: %v", err)
	}
	if result.NoteID != "note-1" {
		t.Errorf("expected NoteID note-1, got %s", result.NoteID)
	}
	if result.Status != "synced" {
		t.Errorf("expected status synced, got %s", result.Status)
	}
	if result.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Version)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(result.Conflicts))
	}
}

func TestSyncNote_UpdateNoConflict(t *testing.T) {
	e := NewEngine()
	note := Note{
		ID:      "note-1",
		Title:   "Original",
		Content: "original content",
		Author:  "alice",
	}
	if _, err := e.SyncNote(note); err != nil {
		t.Fatal(err)
	}

	updated := Note{
		ID:      "note-1",
		Title:   "Updated",
		Content: "updated content",
		Author:  "alice",
	}
	result, err := e.SyncNote(updated)
	if err != nil {
		t.Fatalf("SyncNote failed: %v", err)
	}
	if result.Status != "synced" {
		t.Errorf("expected synced, got %s", result.Status)
	}
	if result.Version != 2 {
		t.Errorf("expected version 2, got %d", result.Version)
	}
}

func TestSyncNote_Conflict(t *testing.T) {
	e := NewEngine()
	note := Note{
		ID:      "note-1",
		Title:   "Title A",
		Content:  "content A",
		Author:  "alice",
	}
	if _, err := e.SyncNote(note); err != nil {
		t.Fatal(err)
	}

	// 不同作者修改同一篇笔记 → 冲突
	conflicting := Note{
		ID:      "note-1",
		Title:   "Title B",
		Content:  "content B",
		Author:  "bob",
	}
	result, err := e.SyncNote(conflicting)
	if err != nil {
		t.Fatalf("SyncNote failed: %v", err)
	}
	if result.Status != "conflict" {
		t.Errorf("expected conflict, got %s", result.Status)
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts to be detected")
	}
}

func TestSyncNote_EmptyID(t *testing.T) {
	e := NewEngine()
	_, err := e.SyncNote(Note{})
	if err == nil {
		t.Error("expected error for empty note ID")
	}
}

// -------------------------------------------------------------------
// MergeConflicts
// -------------------------------------------------------------------

func TestMergeConflicts_DifferentContent(t *testing.T) {
	e := NewEngine()
	local := Note{
		ID:      "n1",
		Title:   "Local Title",
		Content: "local content",
		Author:  "alice",
		Version: 2,
	}
	remote := Note{
		ID:      "n1",
		Title:   "Remote Title",
		Content: "remote content",
		Author:  "bob",
		Version: 3,
	}

	merged, err := e.MergeConflicts(local, remote)
	if err != nil {
		t.Fatalf("MergeConflicts failed: %v", err)
	}
	if merged.ConflictsResolved < 2 {
		t.Errorf("expected at least 2 resolved conflicts, got %d", merged.ConflictsResolved)
	}
	if merged.MergeStrategy != "field_level" {
		t.Errorf("expected field_level strategy, got %s", merged.MergeStrategy)
	}
	if !strings.Contains(merged.Note.Content, "local content") || !strings.Contains(merged.Note.Content, "remote content") {
		t.Errorf("merged content should contain both local and remote: %s", merged.Note.Content)
	}
	if merged.Note.Version != 4 {
		t.Errorf("expected version 4 (max(2,3)+1), got %d", merged.Note.Version)
	}
}

func TestMergeConflicts_NoConflicts(t *testing.T) {
	e := NewEngine()
	local := Note{
		ID:      "n1",
		Title:   "Same Title",
		Content: "same content",
		Author:  "alice",
		Version: 1,
	}
	remote := Note{
		ID:      "n1",
		Title:   "Same Title",
		Content: "same content",
		Author:  "alice",
		Version: 1,
	}

	merged, err := e.MergeConflicts(local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if merged.ConflictsResolved != 0 {
		t.Errorf("expected 0 conflicts resolved, got %d", merged.ConflictsResolved)
	}
	if merged.MergeStrategy != "remote_wins" {
		t.Errorf("expected remote_wins strategy, got %s", merged.MergeStrategy)
	}
}

func TestMergeConflicts_MergesTags(t *testing.T) {
	e := NewEngine()
	local := Note{
		ID:    "n1",
		Title: "T",
		Content: "c",
		Tags:  []string{"a", "b"},
	}
	remote := Note{
		ID:    "n1",
		Title: "T",
		Content: "c",
		Tags:  []string{"b", "c"},
	}
	merged, err := e.MergeConflicts(local, remote)
	if err != nil {
		t.Fatal(err)
	}
	// 应包含并集 a, b, c
	expected := map[string]bool{"a": true, "b": true, "c": true}
	for _, tag := range merged.Note.Tags {
		if !expected[tag] {
			t.Errorf("unexpected tag: %s", tag)
		}
		delete(expected, tag)
	}
	if len(expected) > 0 {
		t.Errorf("missing tags: %v", expected)
	}
}

// -------------------------------------------------------------------
// GetHistory
// -------------------------------------------------------------------

func TestGetHistory(t *testing.T) {
	e := NewEngine()
	note := Note{ID: "h1", Title: "V1", Content: "v1", Author: "alice"}
	if _, err := e.SyncNote(note); err != nil {
		t.Fatal(err)
	}

	updated := Note{ID: "h1", Title: "V2", Content: "v2", Author: "alice"}
	if _, err := e.SyncNote(updated); err != nil {
		t.Fatal(err)
	}

	history, err := e.GetHistory("h1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(history))
	}
	// 降序排列
	if history[0].Version < history[1].Version {
		t.Error("expected descending order")
	}
	if history[0].Summary != "V2" {
		t.Errorf("expected summary V2, got %s", history[0].Summary)
	}
}

func TestGetHistory_EmptyID(t *testing.T) {
	e := NewEngine()
	if _, err := e.GetHistory(""); err == nil {
		t.Error("expected error for empty note ID")
	}
}

func TestGetHistory_NoHistory(t *testing.T) {
	e := NewEngine()
	history, err := e.GetHistory("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}
}

// -------------------------------------------------------------------
// ShareNotebook
// -------------------------------------------------------------------

func TestShareNotebook(t *testing.T) {
	e := NewEngine()
	result, err := e.ShareNotebook("nb-1", []string{"alice", "bob", "carol"})
	if err != nil {
		t.Fatalf("ShareNotebook failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.SharedWith) != 3 {
		t.Errorf("expected 3 shared users, got %d", len(result.SharedWith))
	}
	if result.Permissions["alice"] != "write" {
		t.Errorf("expected alice=write, got %s", result.Permissions["alice"])
	}
	if result.Permissions["bob"] != "read" {
		t.Errorf("expected bob=read, got %s", result.Permissions["bob"])
	}
}

func TestShareNotebook_EmptyID(t *testing.T) {
	e := NewEngine()
	if _, err := e.ShareNotebook("", []string{"a"}); err == nil {
		t.Error("expected error for empty notebook ID")
	}
}

func TestShareNotebook_NoUsers(t *testing.T) {
	e := NewEngine()
	if _, err := e.ShareNotebook("nb-1", []string{}); err == nil {
		t.Error("expected error for empty users list")
	}
}

// -------------------------------------------------------------------
// QueueOffline / FlushOfflineQueue
// -------------------------------------------------------------------

func TestQueueOffline(t *testing.T) {
	e := NewEngine()
	note := Note{ID: "q1", Title: "Offline Note", Content: "content", Author: "alice", Version: 3}
	entry, err := e.QueueOffline(note)
	if err != nil {
		t.Fatalf("QueueOffline failed: %v", err)
	}
	if entry.Note.ID != "q1" {
		t.Errorf("expected note ID q1, got %s", entry.Note.ID)
	}
	if entry.Priority != 3 {
		t.Errorf("expected priority 3 (from version), got %d", entry.Priority)
	}
	if entry.QueuedAt == 0 {
		t.Error("expected non-zero QueuedAt")
	}
}

func TestQueueOffline_EmptyID(t *testing.T) {
	e := NewEngine()
	if _, err := e.QueueOffline(Note{}); err == nil {
		t.Error("expected error for empty note ID")
	}
}

func TestQueueOffline_PriorityOrdering(t *testing.T) {
	e := NewEngine()
	// 先入队低优先级
	e.QueueOffline(Note{ID: "low", Title: "Low", Content: "c", Author: "alice", Version: 1})
	// 再入队高优先级
	e.QueueOffline(Note{ID: "high", Title: "High", Content: "c", Author: "alice", Version: 5})

	// 队列应按优先级降序：high 在前
	if e.offlineQueue[0].Note.ID != "high" {
		t.Errorf("expected high priority first, got %s", e.offlineQueue[0].Note.ID)
	}
}

func TestFlushOfflineQueue_NewNotes(t *testing.T) {
	e := NewEngine()
	entries := []OfflineQueueEntry{
		{Note: Note{ID: "f1", Title: "Note 1", Content: "c1", Author: "alice"}, Priority: 1},
		{Note: Note{ID: "f2", Title: "Note 2", Content: "c2", Author: "bob"}, Priority: 2},
	}

	result, err := e.FlushOfflineQueue(entries)
	if err != nil {
		t.Fatalf("FlushOfflineQueue failed: %v", err)
	}
	if result.Synced != 2 {
		t.Errorf("expected 2 synced, got %d", result.Synced)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if result.Remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", result.Remaining)
	}
}

func TestFlushOfflineQueue_WithConflictAutoMerge(t *testing.T) {
	e := NewEngine()
	// 先同步一篇笔记
	e.SyncNote(Note{ID: "c1", Title: "Original", Content: "original", Author: "alice"})

	// 离线队列中有一篇冲突修改
	entries := []OfflineQueueEntry{
		{Note: Note{ID: "c1", Title: "Offline Edit", Content: "offline content", Author: "bob"}, Priority: 2},
	}

	result, err := e.FlushOfflineQueue(entries)
	if err != nil {
		t.Fatalf("FlushOfflineQueue failed: %v", err)
	}
	if result.Synced != 1 {
		t.Errorf("expected 1 synced, got %d", result.Synced)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
}

func TestFlushOfflineQueue_Empty(t *testing.T) {
	e := NewEngine()
	result, err := e.FlushOfflineQueue(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 0 || result.Failed != 0 || result.Remaining != 0 {
		t.Errorf("expected all zeros, got synced=%d failed=%d remaining=%d",
			result.Synced, result.Failed, result.Remaining)
	}
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

func TestSameTags(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{nil, nil, true},
	}
	for _, tc := range tests {
		got := sameTags(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("sameTags(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMergeText(t *testing.T) {
	// 一边为空
	if got := mergeText("hello", ""); got != "hello" {
		t.Errorf("mergeText with empty remote: got %q", got)
	}
	if got := mergeText("", "world"); got != "world" {
		t.Errorf("mergeText with empty local: got %q", got)
	}
	// 相同内容
	if got := mergeText("same", "same"); got != "same" {
		t.Errorf("mergeText same content: got %q", got)
	}
	// 不同内容 — 应包含分隔线和两段
	got := mergeText("local", "remote")
	if !strings.Contains(got, "local") || !strings.Contains(got, "remote") {
		t.Errorf("mergeText should contain both, got %q", got)
	}
	if !strings.Contains(got, "---") {
		t.Errorf("mergeText should contain separator, got %q", got)
	}
}

func TestMergeTags(t *testing.T) {
	result := mergeTags([]string{"a", "b"}, []string{"b", "c", "d"})
	if len(result) != 4 {
		t.Errorf("expected 4 unique tags, got %d: %v", len(result), result)
	}
	expected := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	for _, t2 := range result {
		if !expected[t2] {
			t.Errorf("unexpected tag %s", t2)
		}
	}
}

func TestMax(t *testing.T) {
	if max(1, 2) != 2 {
		t.Error("max(1,2) should be 2")
	}
	if max(5, 3) != 5 {
		t.Error("max(5,3) should be 5")
	}
	if max(7, 7) != 7 {
		t.Error("max(7,7) should be 7")
	}
}