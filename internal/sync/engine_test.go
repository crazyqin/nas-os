package sync

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockProvider 实现 Provider 接口的内存 mock.
type mockProvider struct {
	mu   sync.RWMutex
	files map[string]*FileEntry
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		files: make(map[string]*FileEntry),
	}
}

func (m *mockProvider) List(ctx context.Context, remotePath string, recursive bool) ([]FileEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var entries []FileEntry
	for _, e := range m.files {
		entries = append(entries, *e)
	}
	return entries, nil
}

func (m *mockProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	m.files[remotePath] = &FileEntry{
		Path:    remotePath,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	return nil
}

func (m *mockProvider) Download(ctx context.Context, remotePath, localPath string) error {
	m.mu.RLock()
	_, ok := m.files[remotePath]
	m.mu.RUnlock()
	if !ok {
		return os.ErrNotExist
	}
	return os.WriteFile(localPath, []byte("mock content"), 0600)
}

func (m *mockProvider) Delete(ctx context.Context, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, remotePath)
	return nil
}

func (m *mockProvider) Mkdir(ctx context.Context, remotePath string) error {
	return nil
}

func (m *mockProvider) Stat(ctx context.Context, remotePath string) (*FileEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.files[remotePath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return e, nil
}

func (m *mockProvider) GetChecksum(ctx context.Context, remotePath string) (string, error) {
	return "", nil
}

func TestComputeDelta_BasicOperations(t *testing.T) {
	oldSnap := &Snapshot{
		Rev: 1,
		Entries: map[string]*FileEntry{
			"unchanged.txt": {Path: "unchanged.txt", Size: 100, ModTime: time.Now(), Checksum: "abc123"},
			"modified.txt":  {Path: "modified.txt", Size: 200, ModTime: time.Now(), Checksum: "def456"},
			"deleted.txt":   {Path: "deleted.txt", Size: 300, ModTime: time.Now(), Checksum: "ghi789"},
		},
	}

	newSnap := &Snapshot{
		Rev: 2,
		Entries: map[string]*FileEntry{
			"unchanged.txt": {Path: "unchanged.txt", Size: 100, ModTime: time.Now(), Checksum: "abc123"},
			"modified.txt":  {Path: "modified.txt", Size: 250, ModTime: time.Now(), Checksum: "jkl012"},
			"added.txt":     {Path: "added.txt", Size: 150, ModTime: time.Now(), Checksum: "mno345"},
		},
	}

	delta := ComputeDelta(oldSnap, newSnap)

	if len(delta.Adds) != 1 {
		t.Errorf("expected 1 add, got %d", len(delta.Adds))
	}
	if len(delta.Mods) != 1 {
		t.Errorf("expected 1 mod, got %d", len(delta.Mods))
	}
	if len(delta.Dels) != 1 {
		t.Errorf("expected 1 del, got %d", len(delta.Dels))
	}

	if delta.Adds[0].RelPath != "added.txt" {
		t.Errorf("expected add 'added.txt', got %s", delta.Adds[0].RelPath)
	}
	if delta.Mods[0].RelPath != "modified.txt" {
		t.Errorf("expected mod 'modified.txt', got %s", delta.Mods[0].RelPath)
	}
	if delta.Dels[0].RelPath != "deleted.txt" {
		t.Errorf("expected del 'deleted.txt', got %s", delta.Dels[0].RelPath)
	}
}

func TestComputeDelta_EmptyOld(t *testing.T) {
	oldSnap := &Snapshot{Rev: 0, Entries: map[string]*FileEntry{}}
	newSnap := &Snapshot{
		Rev: 1,
		Entries: map[string]*FileEntry{
			"a.txt": {Path: "a.txt", Size: 100},
			"b.txt": {Path: "b.txt", Size: 200},
		},
	}

	delta := ComputeDelta(oldSnap, newSnap)

	if len(delta.Adds) != 2 {
		t.Errorf("expected 2 adds, got %d", len(delta.Adds))
	}
	if len(delta.Mods) != 0 {
		t.Errorf("expected 0 mods, got %d", len(delta.Mods))
	}
	if len(delta.Dels) != 0 {
		t.Errorf("expected 0 dels, got %d", len(delta.Dels))
	}
}

func TestComputeDelta_RenameDetection(t *testing.T) {
	oldSnap := &Snapshot{
		Rev: 1,
		Entries: map[string]*FileEntry{
			"old.txt": {Path: "old.txt", Size: 100, Checksum: "same_hash"},
		},
	}
	newSnap := &Snapshot{
		Rev: 2,
		Entries: map[string]*FileEntry{
			"new.txt": {Path: "new.txt", Size: 100, Checksum: "same_hash"},
		},
	}

	delta := ComputeDelta(oldSnap, newSnap)

	if len(delta.Renames) != 1 {
		t.Errorf("expected 1 rename, got %d", len(delta.Renames))
	}
	if len(delta.Renames) > 0 {
		r := delta.Renames[0]
		if r.SrcPath != "old.txt" || r.DstPath != "new.txt" {
			t.Errorf("expected rename old.txt -> new.txt, got %s -> %s", r.SrcPath, r.DstPath)
		}
	}
}

func TestConflictDetector_BothModified(t *testing.T) {
	now := time.Now()
	localDelta := &Delta{
		Mods: []*DeltaItem{
			{
				RelPath:    "file.txt",
				ChangeType: ChangeModify,
				OldEntry:   &FileEntry{Path: "file.txt", Size: 100, Checksum: "old"},
				NewEntry:   &FileEntry{Path: "file.txt", Size: 110, ModTime: now, Checksum: "local_new"},
			},
		},
	}
	remoteDelta := &Delta{
		Mods: []*DeltaItem{
			{
				RelPath:    "file.txt",
				ChangeType: ChangeModify,
				OldEntry:   &FileEntry{Path: "file.txt", Size: 100, Checksum: "old"},
				NewEntry:   &FileEntry{Path: "file.txt", Size: 120, ModTime: now, Checksum: "remote_new"},
			},
		},
	}

	detector := NewConflictDetector(ConflictNewer)
	conflicts := detector.DetectAll("task-1", localDelta, remoteDelta, nil)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	op := detector.Resolve(conflicts[0])
	if op != SyncOpDownload {
		// same time, larger = remote (120 > 110)
		t.Errorf("expected SyncOpDownload (larger remote), got %s", op)
	}
}

func TestConflictDetector_LocalWins(t *testing.T) {
	now := time.Now()
	localDelta := &Delta{
		Mods: []*DeltaItem{
			{
				RelPath:    "f.txt",
				ChangeType: ChangeModify,
				NewEntry:   &FileEntry{Path: "f.txt", Size: 50, ModTime: now, Checksum: "lc"},
			},
		},
	}
	remoteDelta := &Delta{
		Mods: []*DeltaItem{
			{
				RelPath:    "f.txt",
				ChangeType: ChangeModify,
				NewEntry:   &FileEntry{Path: "f.txt", Size: 60, ModTime: now, Checksum: "rc"},
			},
		},
	}

	detector := NewConflictDetector(ConflictLocal)
	conflicts := detector.DetectAll("task-2", localDelta, remoteDelta, nil)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	op := detector.Resolve(conflicts[0])
	if op != SyncOpUpload {
		t.Errorf("expected SyncOpUpload, got %s", op)
	}
}

func TestStateStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStateStore(tmpDir)

	ts := store.GetOrCreateTaskState("test-task")
	ts.LastSyncRev = 42
	ts.LastSyncTime = time.Now()
	ts.FileStates["hello.txt"] = &FileState{
		RelPath:     "hello.txt",
		LocalRev:    1,
		RemoteRev:   1,
		LastSyncRev: 42,
		LocalCS:     "abc",
		RemoteCS:    "abc",
	}

	if err := store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 重新加载
	store2 := NewStateStore(tmpDir)
	if err := store2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	ts2 := store2.GetTaskState("test-task")
	if ts2 == nil {
		t.Fatal("task state not found after reload")
	}
	if ts2.LastSyncRev != 42 {
		t.Errorf("expected rev 42, got %d", ts2.LastSyncRev)
	}
	fs := ts2.FileStates["hello.txt"]
	if fs == nil || fs.LocalCS != "abc" {
		t.Errorf("file state mismatch: %v", fs)
	}
}

func TestVersionManager_StoreAndPrune(t *testing.T) {
	tmpDir := t.TempDir()
	vm := NewVersionManager(tmpDir, 3)

	// 创建测试文件
	srcFile := filepath.Join(tmpDir, "test_src.txt")
	_ = os.WriteFile(srcFile, []byte("hello world"), 0600)

	for i := 0; i < 5; i++ {
		if err := vm.StoreVersion("task-1", "docs/file.txt", srcFile, int64(i)); err != nil {
			t.Fatalf("store version %d: %v", i, err)
		}
	}

	// 清理
	if err := vm.PruneVersions("task-1", "docs/file.txt"); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// 应该只剩 3 个版本
	versions, err := vm.ListVersions("task-1", "docs/file.txt")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) > 3 {
		t.Errorf("expected <= 3 versions after prune, got %d", len(versions))
	}
}

func TestNotifier_Events(t *testing.T) {
	notifier := NewNotifier()

	received := make([]*Event, 0)
	notifier.RegisterFunc(func(ctx context.Context, event *Event) error {
		received = append(received, event)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifier.Start(ctx)
	defer notifier.Stop()

	notifier.EmitMigrationStart("task-1", DirectionUpload)
	time.Sleep(100 * time.Millisecond)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != EventMigrationStart {
		t.Errorf("expected %s, got %s", EventMigrationStart, received[0].Type)
	}
}

func TestChecksum_FileChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(f, []byte("hello checksum test"), 0600)

	cs := NewChecksummer()
	h1, err := cs.FileChecksum(f)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	if h1 == "" {
		t.Error("expected non-empty checksum")
	}

	// 同一文件应产生相同校验和
	h2, _ := cs.FileChecksum(f)
	if h1 != h2 {
		t.Errorf("checksum mismatch: %s != %s", h1, h2)
	}

	// 不同文件应产生不同校验和
	f2 := filepath.Join(tmpDir, "test2.txt")
	_ = os.WriteFile(f2, []byte("different content"), 0600)
	h3, _ := cs.FileChecksum(f2)
	if h1 == h3 {
		t.Error("different files should have different checksums")
	}
}

func TestChecksum_FileChunks(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "chunked.bin")
	// 创建一个大于 DefaultChunkSize 的文件
	data := make([]byte, DefaultChunkSize*2+100)
	for i := range data {
		data[i] = byte(i % 256)
	}
	_ = os.WriteFile(f, data, 0600)

	cs := NewChecksummer()
	chunks, err := cs.FileChunks(f)
	if err != nil {
		t.Fatalf("chunks: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("expected >= 2 chunks for file > chunkSize, got %d", len(chunks))
	}
}

func TestEngine_SyncUpload(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	stateDir := filepath.Join(tmpDir, "state")
	versionDir := filepath.Join(tmpDir, "versions")
	_ = os.MkdirAll(localDir, 0750)

	// 创建测试文件
	_ = os.WriteFile(filepath.Join(localDir, "a.txt"), []byte("aaa"), 0600)
	_ = os.WriteFile(filepath.Join(localDir, "b.txt"), []byte("bbb"), 0600)

	task := &Task{
		ID:        "test-upload",
		Name:      "Test Upload",
		LocalPath: localDir,
		RemotePath: "/remote/test",
		Direction: DirectionUpload,
		Enabled:   true,
		WatchMode: "scan",
	}

	provider := newMockProvider()

	engine, err := NewEngine(task, provider, EngineConfig{
		StateDir:   stateDir,
		VersionDir: versionDir,
		MaxKeep:    3,
		Workers:    2,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx := context.Background()
	if err := engine.Sync(ctx, provider); err != nil {
		t.Fatalf("sync: %v", err)
	}

	p := engine.GetProgress()
	if p.State != "completed" {
		t.Errorf("expected completed, got %s", p.State)
	}
	if p.UploadedFiles != 2 {
		t.Errorf("expected 2 uploads, got %d", p.UploadedFiles)
	}
}
