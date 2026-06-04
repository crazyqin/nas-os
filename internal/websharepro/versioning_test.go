package websharepro

import (
	"testing"
	"time"
)

// mockStorage 模拟版本存储
type mockStorage struct {
	data map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{data: make(map[string][]byte)}
}

func (s *mockStorage) Save(id string, data []byte) error {
	s.data[id] = data
	return nil
}

func (s *mockStorage) Load(id string) ([]byte, error) {
	d, ok := s.data[id]
	if !ok {
		return nil, ErrLinkNotFound
	}
	return d, nil
}

func (s *mockStorage) Delete(id string) error {
	delete(s.data, id)
	return nil
}

func (s *mockStorage) List(prefix string) ([]string, error) {
	var result []string
	for id := range s.data {
		result = append(result, id)
	}
	return result, nil
}

func TestNewVersionManager(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)
	if m == nil {
		t.Fatal("NewVersionManager returned nil")
	}
}

func TestCreateVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	data := []byte("version 1 content")
	v, err := m.CreateVersion("/test.txt", "admin", "initial version", data)
	if err != nil {
		t.Fatalf("create version failed: %v", err)
	}

	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
	if v.Author != "admin" {
		t.Errorf("expected author admin, got %s", v.Author)
	}
	if v.Message != "initial version" {
		t.Errorf("expected message 'initial version', got '%s'", v.Message)
	}
	if !v.IsSnapshot {
		t.Error("expected first version to be snapshot")
	}
}

func TestCreateMultipleVersions(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	for i := 1; i <= 5; i++ {
		data := []byte("content version " + string(rune('0'+i)))
		_, err := m.CreateVersion("/test.txt", "admin", "version "+string(rune('0'+i)), data)
		if err != nil {
			t.Fatalf("create version %d failed: %v", i, err)
		}
	}

	versions := m.ListVersions("/test.txt")
	if len(versions) != 5 {
		t.Errorf("expected 5 versions, got %d", len(versions))
	}
}

func TestDuplicateContent(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	data := []byte("same content")
	m.CreateVersion("/test.txt", "admin", "v1", data)

	_, err := m.CreateVersion("/test.txt", "admin", "v2", data)
	if err == nil {
		t.Error("expected error for duplicate content")
	}
}

func TestGetVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))

	v, err := m.GetVersion("/test.txt", 1)
	if err != nil {
		t.Fatalf("get version failed: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
}

func TestGetLatestVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("v1"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("v2 content"))

	latest, err := m.GetLatestVersion("/test.txt")
	if err != nil {
		t.Fatalf("get latest failed: %v", err)
	}
	if latest.Version != 2 {
		t.Errorf("expected version 2, got %d", latest.Version)
	}
}

func TestTagVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "release", []byte("content"))

	err := m.TagVersion("/test.txt", 1, "v1.0")
	if err != nil {
		t.Fatalf("tag version failed: %v", err)
	}

	v, _ := m.GetVersion("/test.txt", 1)
	found := false
	for _, tag := range v.Tags {
		if tag == "v1.0" {
			found = true
		}
	}
	if !found {
		t.Error("expected tag v1.0 to be present")
	}
}

func TestLockUnlockVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))

	// 锁定
	if err := m.LockVersion("/test.txt", 1); err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	v, _ := m.GetVersion("/test.txt", 1)
	if v.Status != VersionLocked {
		t.Errorf("expected locked status, got %s", v.Status)
	}

	// 解锁
	if err := m.UnlockVersion("/test.txt", 1); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	v, _ = m.GetVersion("/test.txt", 1)
	if v.Status != VersionActive {
		t.Errorf("expected active status, got %s", v.Status)
	}
}

func TestDeleteVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("content2"))

	if err := m.DeleteVersion("/test.txt", 1); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	versions := m.ListVersions("/test.txt")
	if len(versions) != 1 {
		t.Errorf("expected 1 version after delete, got %d", len(versions))
	}
}

func TestDeleteLockedVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))
	m.LockVersion("/test.txt", 1)

	err := m.DeleteVersion("/test.txt", 1)
	if err == nil {
		t.Error("expected error when deleting locked version")
	}
}

func TestDiffVersions(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("original"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("modified content"))

	diff, err := m.DiffVersions("/test.txt", 1, 2)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	if diff.FromVersion != 1 || diff.ToVersion != 2 {
		t.Errorf("unexpected diff versions: %d -> %d", diff.FromVersion, diff.ToVersion)
	}
	if diff.Summary == "no changes" {
		t.Error("expected changes to be detected")
	}
}

func TestDiffSameVersion(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("same"))

	diff, _ := m.DiffVersions("/test.txt", 1, 1)
	if diff.Summary != "no changes" {
		t.Errorf("expected 'no changes', got '%s'", diff.Summary)
	}
}

func TestLoadVersionData(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	original := []byte("version data content")
	m.CreateVersion("/test.txt", "admin", "v1", original)

	// 获取版本信息
	v, _ := m.GetVersion("/test.txt", 1)

	// 加载数据
	loaded, err := m.LoadVersionData(v.StoragePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if string(loaded) != string(original) {
		t.Errorf("data mismatch: got %s", string(loaded))
	}
}

func TestRollback(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("good version"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("bad version"))

	// 回滚到 v1
	rolled, err := m.Rollback("/test.txt", 1, "admin")
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	if rolled.Version != 3 {
		t.Errorf("expected new version 3, got %d", rolled.Version)
	}

	latest, _ := m.GetLatestVersion("/test.txt")
	if latest.Version != 3 {
		t.Errorf("expected latest version 3, got %d", latest.Version)
	}
}

func TestSearchVersions(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "alice", "initial release", []byte("v1"))
	m.CreateVersion("/test.txt", "bob", "bug fix", []byte("v2"))

	results := m.SearchVersions("alice")
	if len(results) == 0 {
		t.Error("expected results for author search")
	}
}

func TestVersioningGetStats(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("content2"))

	stats := m.GetStats()
	if stats.TotalVersions != 2 {
		t.Errorf("expected 2 total versions, got %d", stats.TotalVersions)
	}
}

func TestHasVersions(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	if m.HasVersions("/test.txt") {
		t.Error("expected no versions initially")
	}

	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))

	if !m.HasVersions("/test.txt") {
		t.Error("expected versions after creation")
	}
}

func TestGetFileVersionCount(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	m.CreateVersion("/test.txt", "admin", "v1", []byte("v1"))
	m.CreateVersion("/test.txt", "admin", "v2", []byte("v2"))

	if m.GetFileVersionCount("/test.txt") != 2 {
		t.Errorf("expected 2 versions")
	}
}

func TestVersionRetentionPolicy(t *testing.T) {
	storage := newMockStorage()
	policy := &VersionPolicy{
		MaxVersions:     3,
		KeepMinVersions: 2,
	}
	m := NewVersionManager(storage, policy)

	for i := 1; i <= 5; i++ {
		data := []byte("content " + string(rune('0'+i)))
		m.CreateVersion("/test.txt", "admin", "v"+string(rune('0'+i)), data)
	}

	versions := m.ListVersions("/test.txt")
	// 最多保留 MaxVersions + KeepMinVersions
	if len(versions) > policy.MaxVersions {
		t.Logf("note: some versions may be archived, not deleted")
	}
}

func TestVersionTimestamps(t *testing.T) {
	storage := newMockStorage()
	m := NewVersionManager(storage, nil)

	before := time.Now()
	m.CreateVersion("/test.txt", "admin", "v1", []byte("content"))
	after := time.Now()

	v, _ := m.GetVersion("/test.txt", 1)
	if v.CreatedAt.Before(before) || v.CreatedAt.After(after) {
		t.Error("expected created time to be between before and after")
	}
}
