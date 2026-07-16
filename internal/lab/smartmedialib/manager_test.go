package smartmedialib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	return NewManager(zap.NewNop(), tmpDir)
}

func TestClassifyMedia(t *testing.T) {
	tests := []struct {
		ext      string
		expected MediaType
	}{
		{".jpg", MediaTypePhoto},
		{".png", MediaTypePhoto},
		{".mp4", MediaTypeVideo},
		{".mkv", MediaTypeVideo},
		{".mp3", MediaTypeAudio},
		{".pdf", MediaTypeDoc},
		{".xyz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyMedia(tt.ext))
		})
	}
}

func TestScanDirectory(t *testing.T) {
	mgr := setupTestManager(t)
	tmpDir := t.TempDir()
	for _, f := range []string{"photo.jpg", "video.mp4", "doc.pdf", "readme.txt", "audio.mp3"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0o644))
	}
	count, err := mgr.ScanDirectory(context.Background(), tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestSearchAndFavorite(t *testing.T) {
	mgr := setupTestManager(t)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vacation.jpg"), []byte("test"), 0o644))
	_, err := mgr.ScanDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	items := mgr.SearchItems("vacation", MediaTypePhoto, nil)
	assert.Len(t, items, 1)

	require.NoError(t, mgr.ToggleFavorite(items[0].ID))
	item, ok := mgr.GetItem(items[0].ID)
	require.True(t, ok)
	assert.True(t, item.Favorite)
}

func TestAlbums(t *testing.T) {
	mgr := setupTestManager(t)
	album := mgr.CreateAlbum("Summer Trip", "Beach photos", "manual")
	assert.Equal(t, "Summer Trip", album.Name)

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "beach.png"), []byte("test"), 0o644))
	_, _ = mgr.ScanDirectory(context.Background(), tmpDir)
	items := mgr.SearchItems("", MediaTypePhoto, nil)
	require.Len(t, items, 1)

	err := mgr.AddToAlbum(album.ID, items[0].ID)
	require.NoError(t, err)
}

func TestRating(t *testing.T) {
	mgr := setupTestManager(t)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.jpg"), []byte("test"), 0o644))
	_, _ = mgr.ScanDirectory(context.Background(), tmpDir)
	items := mgr.SearchItems("", MediaTypePhoto, nil)
	require.Len(t, items, 1)

	assert.Error(t, mgr.SetRating(items[0].ID, 6))
	require.NoError(t, mgr.SetRating(items[0].ID, 5))
	item, _ := mgr.GetItem(items[0].ID)
	assert.Equal(t, 5, item.Rating)
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager(t)
	tmpDir := t.TempDir()
	for _, f := range []string{"a.jpg", "b.mp4"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, f), []byte("data"), 0o644))
	}
	_, _ = mgr.ScanDirectory(context.Background(), tmpDir)
	stats := mgr.GetStats()
	assert.Equal(t, 2, stats["total_items"])
}

func TestHasAnyTag(t *testing.T) {
	assert.True(t, hasAnyTag([]string{"photo", "beach"}, []string{"beach"}))
	assert.False(t, hasAnyTag([]string{"photo"}, []string{"beach", "sun"}))
}
