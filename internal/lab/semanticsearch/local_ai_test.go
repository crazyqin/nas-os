package semanticsearch

import (
	"log/slog"
	"testing"
)

func TestSearchLocalAIRedactsAndAudits(t *testing.T) {
	engine := NewSemanticEngine(slog.Default())
	if err := engine.IndexDocument(&Document{ID: "1", Title: "backup", Content: "sensitive backup runbook with many recovery steps and local only details that should be truncated before returning to callers"}); err != nil {
		t.Fatal(err)
	}

	res, err := engine.SearchLocalAI(LocalAIQuery{Text: "backup recovery", Requester: "admin", Purpose: "ops", DataPolicy: "local_only_redact", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Redacted || !res.AuditRequired || res.Policy != "local_only_redact" {
		t.Fatalf("unexpected governed result: %+v", res)
	}
	if len(res.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(res.Results))
	}
}

func TestRecommendRelatedSkipsSource(t *testing.T) {
	engine := NewSemanticEngine(slog.Default())
	_ = engine.IndexDocument(&Document{ID: "1", Title: "photos", Content: "ai photo album face search"})
	_ = engine.IndexDocument(&Document{ID: "2", Title: "albums", Content: "photo album semantic search"})

	related := engine.RecommendRelated("1", 3)
	if len(related) == 0 || related[0].Document.ID == "1" {
		t.Fatalf("unexpected related results: %+v", related)
	}
}
