package aifileorganizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), t.TempDir())
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		ext      string
		expected Category
	}{
		{".jpg", CategoryImage},
		{".mp4", CategoryVideo},
		{".go", CategoryCode},
		{".pdf", CategoryDocument},
		{".zip", CategoryArchive},
		{".xyz", CategoryOther},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyFile(tt.ext))
		})
	}
}

func TestScanAndClassify(t *testing.T) {
	mgr := setupTestManager(t)
	dir := t.TempDir()
	for _, f := range []string{"a.jpg", "b.mp4", "c.go", "d.pdf", "e.zip"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("test"), 0o644))
	}
	count, stats, err := mgr.ScanAndClassify(dir)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.Equal(t, 1, stats[CategoryImage])
	assert.Equal(t, 1, stats[CategoryCode])
}

func TestCreateRule(t *testing.T) {
	mgr := setupTestManager(t)
	rule := mgr.CreateRule("Organize Photos", "/photos", "/sorted", []string{"image"}, false)
	assert.Equal(t, "Organize Photos", rule.Name)
	assert.True(t, rule.Enabled)
}

func TestApplyRule(t *testing.T) {
	mgr := setupTestManager(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.jpg"), []byte("test"), 0o644))
	_, _, _ = mgr.ScanAndClassify(dir)
	rule := mgr.CreateRule("Images", dir, filepath.Join(dir, "sorted"), []string{"image"}, true)
	count, err := mgr.ApplyRule(rule.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.go"), []byte("x"), 0o644))
	_, _, _ = mgr.ScanAndClassify(dir)
	stats := mgr.GetStats()
	assert.Equal(t, 1, stats["total_files"])
}

func TestDeleteRule(t *testing.T) {
	mgr := setupTestManager(t)
	rule := mgr.CreateRule("Test", "/a", "/b", nil, false)
	require.NoError(t, mgr.DeleteRule(rule.ID))
	assert.Error(t, mgr.DeleteRule(rule.ID))
}
