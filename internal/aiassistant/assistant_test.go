package aiassistant

import (
	"testing"
)

func TestAIAssistant(t *testing.T) {
	tmpDir := t.TempDir()
	assistant := NewAIAssistant(tmpDir)

	assistant.ConfigureProvider(&ProviderConfig{
		Provider: ProviderLocal,
		Endpoint: "http://localhost:11434",
		Model:    "llama3",
		Enabled:  true,
	})

	providers := assistant.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
}

func TestProcessRequest(t *testing.T) {
	tmpDir := t.TempDir()
	assistant := NewAIAssistant(tmpDir)

	assistant.ConfigureProvider(&ProviderConfig{
		Provider: ProviderLocal,
		Endpoint: "http://localhost:11434",
		Model:    "llama3",
		Enabled:  true,
	})

	req := &AIRequest{
		Task:     TaskSummarize,
		Provider: ProviderLocal,
		Messages: []AIMessage{{Role: "user", Content: "Hello"}},
	}

	resp, err := assistant.ProcessRequest(req)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if resp.Content == "" {
		t.Fatal("response should not be empty")
	}
}

func TestDocumentManagement(t *testing.T) {
	tmpDir := t.TempDir()
	assistant := NewAIAssistant(tmpDir)

	assistant.RegisterDocument(&Document{
		ID:   "doc1",
		Name: "README.md",
		Path: "/docs/README.md",
		Size: 1024,
	})

	docs := assistant.GetDocuments()
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}

	results := assistant.SearchDocuments("README", 10)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
}
