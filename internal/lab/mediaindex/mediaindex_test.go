package mediaindex

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestIndexer() *Indexer {
	return NewIndexer()
}

func newTestRouter(ix *Indexer) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	h := NewHandlers(ix)
	h.RegisterRoutes(api)
	return r
}

// 创建临时测试文件.
func createTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestIndexer_IndexFile(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	// 创建测试图片文件
	jpgPath := createTempFile(t, dir, "test.jpg", "fake jpeg content")

	mf, err := ix.IndexFile(jpgPath)
	require.NoError(t, err)
	assert.Equal(t, "test.jpg", mf.Name)
	assert.Equal(t, MediaTypeImage, mf.Type)
	assert.Equal(t, "image/jpeg", mf.MIMEType)
	assert.NotEmpty(t, mf.Checksum)
}

func TestIndexer_IndexDirectory(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "photo1.jpg", "photo1")
	createTempFile(t, dir, "photo2.png", "photo2")
	createTempFile(t, dir, "video.mp4", "video")
	createTempFile(t, dir, "readme.txt", "text") // 应被跳过

	count, err := ix.IndexDirectory(dir)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	stats := ix.GetStats()
	assert.Equal(t, 3, stats.TotalFiles)
}

func TestIndexer_DuplicateDetection(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	content := "same content"
	path1 := createTempFile(t, dir, "file1.jpg", content)
	path2 := createTempFile(t, dir, "file2.jpg", content)

	mf1, err := ix.IndexFile(path1)
	require.NoError(t, err)
	assert.False(t, mf1.IsDuplicate)

	mf2, err := ix.IndexFile(path2)
	require.NoError(t, err)
	assert.True(t, mf2.IsDuplicate)
	assert.Equal(t, mf1.ID, mf2.DuplicateOf)
}

func TestIndexer_Tags(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	path := createTempFile(t, dir, "photo.jpg", "photo")
	mf, _ := ix.IndexFile(path)

	tag := ix.AddTag("vacation")
	err := ix.TagFile(mf.ID, tag.ID)
	require.NoError(t, err)

	// 验证标签已添加
	f, _ := ix.Get(mf.ID)
	assert.Contains(t, f.Tags, tag.ID)

	// 重复打标签应无错误
	err = ix.TagFile(mf.ID, tag.ID)
	require.NoError(t, err)
}

func TestIndexer_Collections(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	path := createTempFile(t, dir, "photo.jpg", "photo")
	mf, _ := ix.IndexFile(path)

	col := ix.CreateCollection("Summer 2024", "Summer vacation photos")
	err := ix.AddToCollection(col.ID, mf.ID)
	require.NoError(t, err)

	cols := ix.GetCollections()
	assert.Len(t, cols, 1)
	assert.Len(t, cols[0].FileIDs, 1)
}

func TestIndexer_Timeline(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "a.jpg", "a")
	createTempFile(t, dir, "b.jpg", "b")
	ix.IndexDirectory(dir)

	timeline := ix.GetTimeline()
	assert.NotEmpty(t, timeline)
	assert.Equal(t, 2, timeline[0].Count)
}

func TestIndexer_GetStats(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "img.jpg", "image")
	createTempFile(t, dir, "vid.mp4", "video")
	ix.IndexDirectory(dir)

	stats := ix.GetStats()
	assert.Equal(t, 2, stats.TotalFiles)
	assert.Equal(t, 1, stats.ByType["image"])
	assert.Equal(t, 1, stats.ByType["video"])
}

func TestIndexer_UnsupportedType(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	path := createTempFile(t, dir, "doc.txt", "text")
	_, err := ix.IndexFile(path)
	assert.ErrorIs(t, err, ErrUnsupportedType)
}

func TestSearchEngine_Search(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "sunset.jpg", "sunset")
	createTempFile(t, dir, "beach.jpg", "beach")
	createTempFile(t, dir, "song.mp3", "song")
	ix.IndexDirectory(dir)

	se := NewSearchEngine(ix)

	// 按类型搜索
	result := se.Search(SearchQuery{Type: MediaTypeImage})
	assert.Equal(t, 2, result.Total)

	// 按关键词搜索
	result = se.Search(SearchQuery{Keyword: "sunset"})
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "sunset.jpg", result.Files[0].Name)
}

func TestSearchEngine_Duplicates(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "orig.jpg", "content")
	createTempFile(t, dir, "copy.jpg", "content")
	ix.IndexDirectory(dir)

	se := NewSearchEngine(ix)
	dups := se.SearchDuplicates()
	assert.Len(t, dups, 1)
	assert.True(t, dups[0].IsDuplicate)
}

func TestSearchEngine_Recent(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	createTempFile(t, dir, "a.jpg", "a")
	createTempFile(t, dir, "b.jpg", "b")
	ix.IndexDirectory(dir)

	se := NewSearchEngine(ix)
	recent := se.GetRecent(5)
	assert.Len(t, recent, 2)
	// 最新的在前
	assert.True(t, recent[0].IndexedAt.After(recent[1].IndexedAt) || recent[0].IndexedAt.Equal(recent[1].IndexedAt))
}

func TestSearchEngine_Pagination(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		createTempFile(t, dir, "file"+string(rune('a'+i))+".jpg", "content-"+string(rune('a'+i)))
	}
	ix.IndexDirectory(dir)

	se := NewSearchEngine(ix)
	result := se.Search(SearchQuery{Page: 1, PageSize: 2})
	assert.Equal(t, 5, result.Total)
	assert.Len(t, result.Files, 2)
	assert.Equal(t, 1, result.Page)
}

func TestHandlers_IndexFile(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.jpg", "content")

	r := newTestRouter(ix)
	body := `{"path":"` + path + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/mediaindex/index", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "test.jpg")
}

func TestHandlers_GetStats(t *testing.T) {
	ix := newTestIndexer()
	r := newTestRouter(ix)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/mediaindex/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total_files")
}

func TestHandlers_SearchFiles(t *testing.T) {
	ix := newTestIndexer()
	r := newTestRouter(ix)

	body := `{"type":"image","keyword":"test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/mediaindex/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_CreateTag(t *testing.T) {
	ix := newTestIndexer()
	r := newTestRouter(ix)

	body := `{"name":"vacation"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/mediaindex/tags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "vacation")
}

func TestHandlers_CreateCollection(t *testing.T) {
	ix := newTestIndexer()
	r := newTestRouter(ix)

	body := `{"name":"Summer 2024","description":"Summer photos"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/mediaindex/collections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Summer 2024")
}

func TestHandlers_FileNotFound(t *testing.T) {
	ix := newTestIndexer()
	r := newTestRouter(ix)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/mediaindex/files/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIndexer_DeleteFile(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.jpg", "content")
	mf, _ := ix.IndexFile(path)

	err := ix.Delete(mf.ID)
	require.NoError(t, err)

	_, err = ix.Get(mf.ID)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		path string
		want MediaType
	}{
		{"photo.jpg", MediaTypeImage},
		{"photo.png", MediaTypeImage},
		{"video.mp4", MediaTypeVideo},
		{"video.mkv", MediaTypeVideo},
		{"song.mp3", MediaTypeAudio},
		{"song.flac", MediaTypeAudio},
		{"doc.txt", ""},
		{"file.pdf", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, detectMediaType(tt.path), tt.path)
	}
}

func TestDetectMIME(t *testing.T) {
	assert.Equal(t, "image/jpeg", detectMIME("photo.jpg"))
	assert.Equal(t, "video/mp4", detectMIME("video.mp4"))
	assert.Equal(t, "audio/mpeg", detectMIME("song.mp3"))
	assert.Equal(t, "application/octet-stream", detectMIME("file.xyz"))
}

func TestTagFile_FileNotFound(t *testing.T) {
	ix := newTestIndexer()
	tag := ix.AddTag("test")
	err := ix.TagFile("nonexistent", tag.ID)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestTagFile_TagNotFound(t *testing.T) {
	ix := newTestIndexer()
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.jpg", "content")
	mf, _ := ix.IndexFile(path)

	err := ix.TagFile(mf.ID, "nonexistent-tag")
	assert.ErrorIs(t, err, ErrTagNotFound)
}

func TestAddToCollection_Errors(t *testing.T) {
	ix := newTestIndexer()

	err := ix.AddToCollection("nonexistent-col", "file-id")
	assert.ErrorIs(t, err, ErrCollectionNotFound)
}
