package semanticsearch

import (
	"sort"
	"strings"
	"time"
)

// LocalAIQuery is an on-prem semantic query request with governance metadata.
type LocalAIQuery struct {
	Text        string            `json:"text"`
	Limit       int               `json:"limit"`
	Threshold   float64           `json:"threshold"`
	Requester   string            `json:"requester,omitempty"`
	Purpose     string            `json:"purpose,omitempty"`
	DataPolicy  string            `json:"data_policy,omitempty"`
	Constraints map[string]string `json:"constraints,omitempty"`
}

// GovernedResult wraps search results with policy/audit context.
type GovernedResult struct {
	Results       []*SearchResult `json:"results"`
	Policy        string          `json:"policy"`
	Redacted      bool            `json:"redacted"`
	AuditRequired bool            `json:"audit_required"`
	ExecutedAt    time.Time       `json:"executed_at"`
}

// SearchLocalAI executes local semantic search without sending data off-device.
func (e *SemanticEngine) SearchLocalAI(query LocalAIQuery) (*GovernedResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	results, err := e.Search(&SearchQuery{Text: query.Text, Limit: limit, Threshold: query.Threshold})
	if err != nil {
		return nil, err
	}

	policy := strings.TrimSpace(query.DataPolicy)
	if policy == "" {
		policy = "local_only"
	}
	redacted := strings.Contains(strings.ToLower(policy), "redact")
	if redacted {
		for _, result := range results {
			redactResult(result)
		}
	}

	return &GovernedResult{
		Results:       results,
		Policy:        policy,
		Redacted:      redacted,
		AuditRequired: query.Requester != "" || query.Purpose != "",
		ExecutedAt:    time.Now(),
	}, nil
}

// RecommendRelated returns related documents for knowledge-base style browsing.
func (e *SemanticEngine) RecommendRelated(docID string, limit int) []*SearchResult {
	e.mu.RLock()
	doc := e.documents[docID]
	e.mu.RUnlock()
	if doc == nil {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	results, err := e.Search(&SearchQuery{Text: doc.Title + " " + doc.Content, Limit: limit + 1})
	if err != nil {
		return nil
	}
	filtered := make([]*SearchResult, 0, limit)
	for _, result := range results {
		if result.Document.ID == docID {
			continue
		}
		filtered = append(filtered, result)
		if len(filtered) == limit {
			break
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Score > filtered[j].Score })
	return filtered
}

func redactResult(result *SearchResult) {
	if result == nil || result.Document == nil {
		return
	}
	if len(result.Document.Content) > 160 {
		result.Document.Content = result.Document.Content[:160] + "..."
	}
	result.Matches = nil
}
