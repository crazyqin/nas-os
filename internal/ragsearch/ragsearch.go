package ragsearch

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Manager manages RAG-enhanced search
type Manager struct {
	config      *Config
	index       map[string]*IndexEntry
	invertedIdx map[string]map[string]int // term -> docID -> term freq
	docFreq     map[string]int            // term -> number of docs containing term
	totalDocLen int                       // total content length of all docs
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	stats       *IndexStats
	history     []*SearchHistory
	historyMu   sync.RWMutex
}

// NewManager creates a new RAG search manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:      config,
		index:       make(map[string]*IndexEntry),
		invertedIdx: make(map[string]map[string]int),
		docFreq:     make(map[string]int),
		ctx:         ctx,
		cancel:      cancel,
		stats: &IndexStats{
			EntriesByType:   make(map[string]int64),
			IndexHealth:     "healthy",
			VectorDimension: config.VectorDimension,
		},
		history: make([]*SearchHistory, 0),
	}
}

// AddDocument adds a document to the index
func (m *Manager) AddDocument(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}
	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tokens := tokenize(doc.Content + " " + doc.Title)
	termFreq := computeTermFreq(tokens)
	embedding := generateEmbedding(doc.Content, m.config.VectorDimension)

	entry := &IndexEntry{
		Document:      *doc,
		IndexedAt:     time.Now(),
		Tokens:        tokens,
		TermFreq:      termFreq,
		Embedding:     embedding,
		ContentLength: len(doc.Content),
	}

	// Remove old entry from inverted index if exists
	if old, exists := m.index[doc.ID]; exists {
		m.removeFromInvertedIndex(doc.ID, old.TermFreq)
		m.totalDocLen -= old.ContentLength
	}

	m.index[doc.ID] = entry
	m.addToInvertedIndex(doc.ID, termFreq)
	m.totalDocLen += len(doc.Content)
	m.updateStats()
	return nil
}

// UpdateDocument updates an existing document
func (m *Manager) UpdateDocument(doc *Document) error {
	return m.AddDocument(doc)
}

// RemoveDocument removes a document from the index
func (m *Manager) RemoveDocument(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.index[id]
	if !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	m.removeFromInvertedIndex(id, entry.TermFreq)
	m.totalDocLen -= entry.ContentLength
	delete(m.index, id)
	m.updateStats()
	return nil
}

// GetDocument returns a document by ID
func (m *Manager) GetDocument(id string) (*IndexEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.index[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return entry, nil
}

// Search performs a search based on the query mode
func (m *Manager) Search(query *SearchQuery) (*SearchResponse, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if query.Query == "" {
		return &SearchResponse{
			Query:      "",
			TotalHits:  0,
			Results:    []*SearchResult{},
			SearchTime: 0,
			Limit:      query.Limit,
			Offset:     query.Offset,
		}, nil
	}

	if query.Limit <= 0 {
		query.Limit = 20
	}

	start := time.Now()
	m.recordHistory(query.Query, 0)

	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SearchResult

	switch query.Mode {
	case ModeFullText:
		results = m.fullTextSearch(query)
	case ModeSemantic:
		results = m.semanticSearch(query)
	case ModeHybrid, ModeAuto:
		results = m.hybridSearch(query)
	default:
		results = m.hybridSearch(query)
	}

	// Apply filters
	if query.Filters != nil {
		results = m.applyFilters(results, query.Filters)
	}

	// Sort results
	results = m.sortResults(results, query.Sort)

	// Apply pagination
	totalHits := len(results)
	offset := query.Offset
	if offset > totalHits {
		offset = totalHits
	}
	end := offset + query.Limit
	if end > totalHits {
		end = totalHits
	}
	pagedResults := results[offset:end]

	// Update history hit count
	m.updateHistoryHitCount(query.Query, totalHits)

	// Generate suggestions
	suggestions := m.generateSuggestions(query.Query)

	// Generate facets
	var facets map[string]int
	if query.Facets {
		facets = m.generateFacets(results)
	}

	return &SearchResponse{
		Query:       query.Query,
		TotalHits:   totalHits,
		Results:     pagedResults,
		Facets:      facets,
		Suggestions: suggestions,
		SearchTime:  time.Since(start).Milliseconds(),
		Limit:       query.Limit,
		Offset:      query.Offset,
		HasMore:     end < totalHits,
	}, nil
}

// fullTextSearch performs BM25-based full text search
func (m *Manager) fullTextSearch(query *SearchQuery) []*SearchResult {
	queryTokens := tokenize(query.Query)
	if len(queryTokens) == 0 {
		return nil
	}

	numDocs := len(m.index)
	if numDocs == 0 {
		return nil
	}

	avgDocLen := float64(0)
	if numDocs > 0 {
		avgDocLen = float64(m.totalDocLen) / float64(numDocs)
	}

	var results []*SearchResult
	for _, entry := range m.index {
		bm25Score := m.calculateBM25(entry, queryTokens, numDocs, avgDocLen)
		if bm25Score > 0 {
			result := &SearchResult{
				ID:         entry.ID,
				Title:      entry.Title,
				Content:    entry.Content,
				Path:       entry.Path,
				DocType:    entry.DocType,
				MimeType:   entry.MimeType,
				Size:       entry.Size,
				Score:      bm25Score,
				Tags:       entry.Tags,
				Metadata:   entry.Metadata,
				Source:     entry.Source,
				CreatedAt:  entry.CreatedAt,
				ModifiedAt: entry.ModifiedAt,
				RankScore: &RankScore{
					BM25Score:     bm25Score,
					CombinedScore: bm25Score,
				},
			}
			if query.Highlight {
				result.Highlight = generateHighlight(entry.Content, queryTokens)
			}
			results = append(results, result)
		}
	}

	return results
}

// semanticSearch performs vector-based semantic search
func (m *Manager) semanticSearch(query *SearchQuery) []*SearchResult {
	queryEmbedding := generateEmbedding(query.Query, m.config.VectorDimension)

	var results []*SearchResult
	for _, entry := range m.index {
		if len(entry.Embedding) == 0 {
			continue
		}
		similarity := cosineSimilarity(queryEmbedding, entry.Embedding)
		if similarity > 0.01 { // threshold
			result := &SearchResult{
				ID:         entry.ID,
				Title:      entry.Title,
				Content:    entry.Content,
				Path:       entry.Path,
				DocType:    entry.DocType,
				MimeType:   entry.MimeType,
				Size:       entry.Size,
				Score:      similarity,
				Tags:       entry.Tags,
				Metadata:   entry.Metadata,
				Source:     entry.Source,
				CreatedAt:  entry.CreatedAt,
				ModifiedAt: entry.ModifiedAt,
				RankScore: &RankScore{
					SemanticScore: similarity,
					CombinedScore: similarity,
				},
			}
			results = append(results, result)
		}
	}

	return results
}

// hybridSearch performs combined BM25 + semantic search with RRF fusion
func (m *Manager) hybridSearch(query *SearchQuery) []*SearchResult {
	queryTokens := tokenize(query.Query)
	queryEmbedding := generateEmbedding(query.Query, m.config.VectorDimension)

	numDocs := len(m.index)
	if numDocs == 0 {
		return nil
	}

	avgDocLen := float64(0)
	if numDocs > 0 {
		avgDocLen = float64(m.totalDocLen) / float64(numDocs)
	}

	// Collect BM25 scores
	bm25Scores := make(map[string]float64)
	for docID, entry := range m.index {
		score := m.calculateBM25(entry, queryTokens, numDocs, avgDocLen)
		if score > 0 {
			bm25Scores[docID] = score
		}
	}

	// Collect semantic scores
	semanticScores := make(map[string]float64)
	for docID, entry := range m.index {
		if len(entry.Embedding) > 0 {
			similarity := cosineSimilarity(queryEmbedding, entry.Embedding)
			if similarity > 0.01 {
				semanticScores[docID] = similarity
			}
		}
	}

	// Rank by BM25
	bm25Ranked := rankByScore(bm25Scores)
	// Rank by semantic
	semanticRanked := rankByScore(semanticScores)

	// RRF fusion
	rrfScores := make(map[string]float64)
	k := float64(m.config.RRFK)

	for rank, docID := range bm25Ranked {
		rrfScores[docID] += 1.0 / (k + float64(rank+1))
	}
	for rank, docID := range semanticRanked {
		rrfScores[docID] += 1.0 / (k + float64(rank+1))
	}

	// Build results
	var results []*SearchResult
	for docID, rrfScore := range rrfScores {
		entry, exists := m.index[docID]
		if !exists {
			continue
		}

		bm25 := bm25Scores[docID]
		semantic := semanticScores[docID]
		freshness := m.calculateFreshness(entry)
		combined := m.config.FullTextWeight*bm25 +
			m.config.SemanticWeight*semantic +
			m.config.FreshnessWeight*freshness +
			rrfScore

		result := &SearchResult{
			ID:         entry.ID,
			Title:      entry.Title,
			Content:    entry.Content,
			Path:       entry.Path,
			DocType:    entry.DocType,
			MimeType:   entry.MimeType,
			Size:       entry.Size,
			Score:      combined,
			Tags:       entry.Tags,
			Metadata:   entry.Metadata,
			Source:     entry.Source,
			CreatedAt:  entry.CreatedAt,
			ModifiedAt: entry.ModifiedAt,
			RankScore: &RankScore{
				BM25Score:      bm25,
				SemanticScore:  semantic,
				RRFScore:       rrfScore,
				FreshnessScore: freshness,
				CombinedScore:  combined,
			},
		}
		if query.Highlight {
			result.Highlight = generateHighlight(entry.Content, queryTokens)
		}
		results = append(results, result)
	}

	return results
}

// calculateBM25 computes BM25 score for a document
func (m *Manager) calculateBM25(entry *IndexEntry, queryTokens []string, numDocs int, avgDocLen float64) float64 {
	k1 := m.config.BM25K1
	b := m.config.BM25B
	score := 0.0
	docLen := float64(entry.ContentLength)

	for _, term := range queryTokens {
		tf := entry.TermFreq[term]
		if tf == 0 {
			continue
		}

		df := m.docFreq[term]
		if df == 0 {
			continue
		}

		// IDF component: log((N - df + 0.5) / (df + 0.5) + 1)
		idf := math.Log((float64(numDocs)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)

		// TF component
		tfNorm := (float64(tf) * (k1 + 1)) / (float64(tf) + k1*(1-b+b*docLen/avgDocLen))

		score += idf * tfNorm
	}

	return score
}

// calculateFreshness computes freshness score based on modification time
func (m *Manager) calculateFreshness(entry *IndexEntry) float64 {
	hours := time.Since(entry.ModifiedAt).Hours()
	if hours < 0 {
		hours = 0
	}
	// Exponential decay: score = e^(-hours/720) (30-day half-life)
	return math.Exp(-hours / 720.0)
}

// applyFilters applies search filters to results
func (m *Manager) applyFilters(results []*SearchResult, filters *SearchFilter) []*SearchResult {
	if filters == nil {
		return results
	}

	var filtered []*SearchResult
	for _, r := range results {
		if !matchesFilter(r, filters) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// matchesFilter checks if a result matches the given filter
func matchesFilter(r *SearchResult, f *SearchFilter) bool {
	// Doc type filter
	if len(f.DocTypes) > 0 {
		found := false
		for _, dt := range f.DocTypes {
			if r.DocType == dt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Tags filter
	if len(f.Tags) > 0 {
		hasTag := false
		for _, ft := range f.Tags {
			for _, rt := range r.Tags {
				if strings.EqualFold(ft, rt) {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// Date range filter
	if f.DateFrom != nil && r.ModifiedAt.Before(*f.DateFrom) {
		return false
	}
	if f.DateTo != nil && r.ModifiedAt.After(*f.DateTo) {
		return false
	}

	// Size filter
	if f.SizeMin != nil && r.Size < *f.SizeMin {
		return false
	}
	if f.SizeMax != nil && r.Size > *f.SizeMax {
		return false
	}

	// Source filter
	if len(f.Sources) > 0 {
		found := false
		for _, s := range f.Sources {
			if r.Source == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// MIME type filter
	if f.MimeType != "" && !strings.EqualFold(r.MimeType, f.MimeType) {
		return false
	}

	// Path prefix filter
	if f.PathPrefix != "" && !strings.HasPrefix(r.Path, f.PathPrefix) {
		return false
	}

	return true
}

// sortResults sorts results by the specified order
func (m *Manager) sortResults(results []*SearchResult, order SortOrder) []*SearchResult {
	if len(results) == 0 {
		return results
	}

	sorted := make([]*SearchResult, len(results))
	copy(sorted, results)

	switch order {
	case SortDate:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ModifiedAt.After(sorted[j].ModifiedAt)
		})
	case SortName:
		sort.Slice(sorted, func(i, j int) bool {
			return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
		})
	case SortSize:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Size > sorted[j].Size
		})
	case SortScore, SortRelevance:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Score > sorted[j].Score
		})
	default:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Score > sorted[j].Score
		})
	}

	return sorted
}

// GetSuggestions returns search suggestions based on prefix
func (m *Manager) GetSuggestions(req *SuggestionRequest) (*SuggestionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.Prefix == "" {
		return &SuggestionResponse{
			Suggestions: []Suggestion{},
			Query:       "",
		}, nil
	}

	limit := req.Limit
	if limit <= 0 {
		limit = m.config.SuggestionLimit
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := strings.ToLower(req.Prefix)
	suggestionMap := make(map[string]*Suggestion)

	for _, entry := range m.index {
		// Filter by doc type if specified
		if len(req.DocTypes) > 0 {
			found := false
			for _, dt := range req.DocTypes {
				if entry.DocType == dt {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check title words
		words := tokenize(entry.Title)
		for _, w := range words {
			if strings.HasPrefix(w, prefix) && len(w) > len(prefix) {
				if s, exists := suggestionMap[w]; exists {
					s.Frequency++
				} else {
					suggestionMap[w] = &Suggestion{
						Text:      w,
						Score:     1.0,
						Frequency: 1,
					}
				}
			}
		}

		// Check tags
		for _, tag := range entry.Tags {
			tagLower := strings.ToLower(tag)
			if strings.HasPrefix(tagLower, prefix) && len(tagLower) > len(prefix) {
				if s, exists := suggestionMap[tagLower]; exists {
					s.Frequency++
				} else {
					suggestionMap[tagLower] = &Suggestion{
						Text:      tag,
						Score:     0.8,
						Frequency: 1,
					}
				}
			}
		}
	}

	// Also suggest from history
	m.historyMu.RLock()
	for _, h := range m.history {
		histLower := strings.ToLower(h.Query)
		if strings.HasPrefix(histLower, prefix) && len(histLower) > len(prefix) {
			if s, exists := suggestionMap[histLower]; exists {
				s.Frequency += h.HitCount
				s.Score += 0.5
			} else {
				suggestionMap[histLower] = &Suggestion{
					Text:      h.Query,
					Score:     0.5,
					Frequency: h.HitCount,
				}
			}
		}
	}
	m.historyMu.RUnlock()

	// Convert to slice and sort
	suggestions := make([]Suggestion, 0, len(suggestionMap))
	for _, s := range suggestionMap {
		s.Score += float64(s.Frequency) * 0.1
		suggestions = append(suggestions, *s)
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score ||
			(suggestions[i].Score == suggestions[j].Score && suggestions[i].Frequency > suggestions[j].Frequency)
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return &SuggestionResponse{
		Suggestions: suggestions,
		Query:       req.Prefix,
	}, nil
}

// GetSearchHistory returns search history
func (m *Manager) GetSearchHistory(limit int) []*SearchHistory {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// Return most recent first
	result := make([]*SearchHistory, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.history[len(m.history)-1-i]
	}
	return result
}

// GetHotQueries returns popular/trending queries
func (m *Manager) GetHotQueries(limit int) []*HotQuery {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	queryCount := make(map[string]*HotQuery)
	for _, h := range m.history {
		if q, exists := queryCount[h.Query]; exists {
			q.Count += h.HitCount + 1
			if h.Timestamp.After(q.LastUsed) {
				q.LastUsed = h.Timestamp
			}
		} else {
			queryCount[h.Query] = &HotQuery{
				Query:    h.Query,
				Count:    h.HitCount + 1,
				LastUsed: h.Timestamp,
			}
		}
	}

	hotQueries := make([]*HotQuery, 0, len(queryCount))
	for _, q := range queryCount {
		hotQueries = append(hotQueries, q)
	}

	sort.Slice(hotQueries, func(i, j int) bool {
		return hotQueries[i].Count > hotQueries[j].Count
	})

	if len(hotQueries) > limit {
		hotQueries = hotQueries[:limit]
	}

	return hotQueries
}

// GetStats returns index statistics
func (m *Manager) GetStats() *IndexStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// RebuildIndex rebuilds the entire search index
func (m *Manager) RebuildIndex() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.index = make(map[string]*IndexEntry)
	m.invertedIdx = make(map[string]map[string]int)
	m.docFreq = make(map[string]int)
	m.totalDocLen = 0
	m.updateStats()
	return nil
}

// Stop stops the search manager
func (m *Manager) Stop() {
	m.cancel()
}

// addToInvertedIndex adds term frequencies to the inverted index
func (m *Manager) addToInvertedIndex(docID string, termFreq map[string]int) {
	for term, freq := range termFreq {
		if m.invertedIdx[term] == nil {
			m.invertedIdx[term] = make(map[string]int)
		}
		m.invertedIdx[term][docID] = freq
		m.docFreq[term] = len(m.invertedIdx[term])
	}
}

// removeFromInvertedIndex removes a document from the inverted index
func (m *Manager) removeFromInvertedIndex(docID string, termFreq map[string]int) {
	for term := range termFreq {
		if postings, exists := m.invertedIdx[term]; exists {
			delete(postings, docID)
			if len(postings) == 0 {
				delete(m.invertedIdx, term)
				delete(m.docFreq, term)
			} else {
				m.docFreq[term] = len(postings)
			}
		}
	}
}

// updateStats updates index statistics
func (m *Manager) updateStats() {
	m.stats.TotalEntries = int64(len(m.index))
	m.stats.EntriesByType = make(map[string]int64)
	for _, entry := range m.index {
		m.stats.EntriesByType[string(entry.DocType)]++
	}
	m.stats.LastIndexed = time.Now()
}

// recordHistory records a search query in history
func (m *Manager) recordHistory(query string, hitCount int) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	m.history = append(m.history, &SearchHistory{
		Query:     query,
		Timestamp: time.Now(),
		HitCount:  hitCount,
	})

	// Trim history if too large
	if len(m.history) > m.config.HistoryMaxSize {
		m.history = m.history[len(m.history)-m.config.HistoryMaxSize:]
	}
}

// updateHistoryHitCount updates the hit count for the most recent matching query
func (m *Manager) updateHistoryHitCount(query string, hitCount int) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Query == query {
			m.history[i].HitCount = hitCount
			break
		}
	}
}

// generateSuggestions generates query suggestions based on query terms
func (m *Manager) generateSuggestions(query string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	lastToken := queryTokens[len(queryTokens)-1]
	suggestionSet := make(map[string]bool)

	for term := range m.invertedIdx {
		if strings.HasPrefix(term, lastToken) && term != lastToken {
			suggestionSet[term] = true
		}
	}

	suggestions := make([]string, 0, len(suggestionSet))
	for s := range suggestionSet {
		suggestions = append(suggestions, s)
	}

	sort.Strings(suggestions)
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions
}

// generateFacets generates facet counts by doc type
func (m *Manager) generateFacets(results []*SearchResult) map[string]int {
	facets := make(map[string]int)
	for _, r := range results {
		facets[string(r.DocType)]++
	}
	return facets
}

// tokenize splits text into lowercase tokens
func tokenize(text string) []string {
	if text == "" {
		return nil
	}

	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				token := current.String()
				if len(token) > 1 { // skip single-char tokens
					tokens = append(tokens, token)
				}
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		token := current.String()
		if len(token) > 1 {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// computeTermFreq computes term frequency map from tokens
func computeTermFreq(tokens []string) map[string]int {
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}

// generateEmbedding generates a deterministic embedding vector from text
func generateEmbedding(text string, dimension int) []float64 {
	if dimension <= 0 {
		dimension = 128
	}

	embedding := make([]float64, dimension)
	tokens := tokenize(text)

	if len(tokens) == 0 {
		return embedding
	}

	// Generate deterministic embedding based on token hashes
	for _, token := range tokens {
		hash := simpleHash(token)
		for i := 0; i < dimension; i++ {
			idx := (hash + uint32(i)) % uint32(dimension)
			embedding[idx] += 1.0 / (1.0 + float64(i%10))
		}
	}

	// Normalize the embedding
	norm := 0.0
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// simpleHash computes a simple hash of a string
func simpleHash(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// cosineSimilarity computes cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// rankByScore ranks document IDs by their scores (descending)
func rankByScore(scores map[string]float64) []string {
	type docScore struct {
		id    string
		score float64
	}

	docs := make([]docScore, 0, len(scores))
	for id, score := range scores {
		docs = append(docs, docScore{id, score})
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].score > docs[j].score
	})

	result := make([]string, len(docs))
	for i, d := range docs {
		result[i] = d.id
	}

	return result
}

// generateHighlight generates highlighted text snippet
func generateHighlight(content string, queryTokens []string) string {
	if len(queryTokens) == 0 || content == "" {
		return ""
	}

	contentLower := strings.ToLower(content)
	bestPos := -1

	// Find first occurrence of any query token
	for _, token := range queryTokens {
		pos := strings.Index(contentLower, token)
		if pos >= 0 && (bestPos < 0 || pos < bestPos) {
			bestPos = pos
		}
	}

	if bestPos < 0 {
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}

	// Extract context around the match
	start := bestPos - 50
	if start < 0 {
		start = 0
	}
	end := bestPos + 150
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}
