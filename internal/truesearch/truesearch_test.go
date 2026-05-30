package truesearch

import (
	"testing"
)

func TestNewSearchEngine(t *testing.T) {
	se := NewSearchEngine(nil)
	if se == nil {
		t.Fatal("NewSearchEngine returned nil")
	}
	if se.stats.TotalDocuments != 0 {
		t.Errorf("expected 0 documents, got %d", se.stats.TotalDocuments)
	}
}

func TestIndexAndSearch(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{
		ID:       "doc1",
		FilePath: "/docs/readme.md",
		FileName: "readme.md",
		FileType: "markdown",
		Size:     1024,
		Content:  "This is a test document about NAS storage",
	})

	se.IndexDocument(&IndexEntry{
		ID:       "doc2",
		FilePath: "/docs/guide.txt",
		FileName: "guide.txt",
		FileType: "text",
		Size:     2048,
		Content:  "Guide to setting up your NAS system",
	})

	results := se.Search("NAS", 10)
	if len(results) == 0 {
		t.Fatal("expected search results, got 0")
	}

	found := false
	for _, r := range results {
		if r.ID == "doc1" || r.ID == "doc2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find doc1 or doc2 in results")
	}
}

func TestSearchFilenameMatch(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{
		ID:       "doc1",
		FilePath: "/files/config.yaml",
		FileName: "config.yaml",
		Content:  "some content",
	})

	results := se.Search("config", 10)
	if len(results) == 0 {
		t.Fatal("expected results for filename match")
	}

	if results[0].MatchType != "filename" {
		t.Errorf("expected match type 'filename', got '%s'", results[0].MatchType)
	}
}

func TestRemoveDocument(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{
		ID:       "doc1",
		FilePath: "/test.txt",
		FileName: "test.txt",
		Content:  "test content",
	})

	se.RemoveDocument("doc1")

	results := se.Search("test", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results after removal, got %d", len(results))
	}
}

func TestGetSuggestions(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{ID: "1", FileName: "config.yaml", Content: ""})
	se.IndexDocument(&IndexEntry{ID: "2", FileName: "config.json", Content: ""})
	se.IndexDocument(&IndexEntry{ID: "3", FileName: "data.csv", Content: ""})

	suggestions := se.GetSuggestions("conf", 10)
	if len(suggestions) == 0 {
		t.Error("expected suggestions for 'conf'")
	}
}

func TestGetStats(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{ID: "1", FileName: "a.txt", Content: ""})
	se.IndexDocument(&IndexEntry{ID: "2", FileName: "b.txt", Content: ""})

	stats := se.GetStats()
	if stats.TotalDocuments != 2 {
		t.Errorf("expected 2 documents, got %d", stats.TotalDocuments)
	}
}

func TestRebuildIndex(t *testing.T) {
	se := NewSearchEngine(nil)

	se.IndexDocument(&IndexEntry{ID: "1", FileName: "test.txt", Content: "hello world"})
	se.RebuildIndex()

	results := se.Search("hello", 10)
	if len(results) == 0 {
		t.Error("expected results after rebuild")
	}
}

func TestTrieIndex(t *testing.T) {
	trie := NewTrieIndex()

	trie.Insert("hello", "id1")
	trie.Insert("help", "id2")
	trie.Insert("world", "id3")

	ids := trie.Search("hel")
	if len(ids) != 2 {
		t.Errorf("expected 2 results for 'hel', got %d", len(ids))
	}

	trie.Remove("hello", "id1")
	ids = trie.Search("hel")
	if len(ids) != 1 {
		t.Errorf("expected 1 result after removal, got %d", len(ids))
	}
}
