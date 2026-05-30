// Package photos 测试文件
package photos

import (
	"context"
	"testing"
)

func TestManager_CreateAlbum(t *testing.T) {
	m := NewManager("/tmp/test-photos")
	ctx := context.Background()

	album, err := m.CreateAlbum(ctx, "Test Album", "Test Description", "user1")
	if err != nil {
		t.Fatalf("CreateAlbum failed: %v", err)
	}

	if album.Name != "Test Album" {
		t.Errorf("Expected album name 'Test Album', got '%s'", album.Name)
	}

	if album.OwnerID != "user1" {
		t.Errorf("Expected owner 'user1', got '%s'", album.OwnerID)
	}
}

func TestManager_SearchPhotos(t *testing.T) {
	m := NewManager("/tmp/test-photos")
	ctx := context.Background()

	// 添加测试照片
	m.mu.Lock()
	m.photos["test1"] = &Photo{
		ID:       "test1",
		Filename: "test1.jpg",
		Tags:     []string{"nature", "landscape"},
	}
	m.mu.Unlock()

	// 搜索
	query := SearchQuery{
		Keyword:  "nature",
		Page:     1,
		PageSize: 10,
	}

	result, err := m.SearchPhotos(ctx, query)
	if err != nil {
		t.Fatalf("SearchPhotos failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected 1 result, got %d", result.Total)
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager("/tmp/test-photos")
	ctx := context.Background()

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["total_photos"] != 0 {
		t.Errorf("Expected 0 photos, got %v", stats["total_photos"])
	}
}
