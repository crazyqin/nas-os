package smartsearch2

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSearchService(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	assert.NotNil(t, service)
}

func TestStartStop(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)

	err = service.Stop()
	assert.NoError(t, err)
}

func TestIndexFile(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	entry := &IndexEntry{
		Path:      "/data/documents/report.pdf",
		Name:      "report.pdf",
		Extension: ".pdf",
		Size:      1024 * 1024,
		MimeType:  "application/pdf",
		ModTime:   time.Now(),
		Content:   "This is a quarterly financial report with important data",
		Tags:      []string{"finance", "report", "quarterly"},
	}

	err = service.IndexFile(entry)
	assert.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 1, stats.TotalIndexed)
}

func TestSearch(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	// Index some files
	files := []*IndexEntry{
		{
			Path:      "/data/documents/report.pdf",
			Name:      "report.pdf",
			Extension: ".pdf",
			Size:      1024,
			MimeType:  "application/pdf",
			ModTime:   time.Now(),
			Content:   "Quarterly financial report",
			Tags:      []string{"finance"},
		},
		{
			Path:      "/data/photos/vacation.jpg",
			Name:      "vacation.jpg",
			Extension: ".jpg",
			Size:      2048,
			MimeType:  "image/jpeg",
			ModTime:   time.Now(),
		},
		{
			Path:      "/data/music/song.mp3",
			Name:      "song.mp3",
			Extension: ".mp3",
			Size:      4096,
			MimeType:  "audio/mpeg",
			ModTime:   time.Now(),
		},
	}

	for _, f := range files {
		err := service.IndexFile(f)
		require.NoError(t, err)
	}

	// Search for "report"
	results, err := service.Search("report", SearchOptions{IncludeSnippet: true})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "report.pdf", results[0].Name)
}

func TestSearch_EmptyQuery(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	_, err = service.Search("", SearchOptions{})
	assert.Error(t, err)
}

func TestSearch_NoResults(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	entry := &IndexEntry{
		Path:      "/data/test.txt",
		Name:      "test.txt",
		Extension: ".txt",
		Size:      100,
		MimeType:  "text/plain",
		ModTime:   time.Now(),
	}
	err = service.IndexFile(entry)
	require.NoError(t, err)

	results, err := service.Search("nonexistent", SearchOptions{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_MaxResults(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	// Index multiple files
	for i := 0; i < 10; i++ {
		entry := &IndexEntry{
			Path:      fmt.Sprintf("/data/file%d.txt", i),
			Name:      fmt.Sprintf("file%d.txt", i),
			Extension: ".txt",
			Size:      100,
			MimeType:  "text/plain",
			ModTime:   time.Now(),
			Content:   "test content",
		}
		err := service.IndexFile(entry)
		require.NoError(t, err)
	}

	results, err := service.Search("file", SearchOptions{MaxResults: 5})
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestRemoveFile(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	entry := &IndexEntry{
		Path:      "/data/test.txt",
		Name:      "test.txt",
		Extension: ".txt",
		Size:      100,
		MimeType:  "text/plain",
		ModTime:   time.Now(),
	}
	err = service.IndexFile(entry)
	require.NoError(t, err)

	err = service.RemoveFile("/data/test.txt")
	assert.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 0, stats.TotalIndexed)
}

func TestRemoveFile_NotFound(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	err = service.RemoveFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestGetStats(t *testing.T) {
	service := NewSearchService(SearchConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	entry := &IndexEntry{
		Path:      "/data/test.txt",
		Name:      "test.txt",
		Extension: ".txt",
		Size:      100,
		MimeType:  "text/plain",
		ModTime:   time.Now(),
	}
	err = service.IndexFile(entry)
	require.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 1, stats.TotalIndexed)
	assert.False(t, stats.LastIndexed.IsZero())
}
