package web3storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// ===================== Manager Business Logic =====================

// Start initialises the manager: discovers storage nodes and starts cache eviction.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	// In production this would dial IPFS API and discover nodes.
	log.Printf("[web3storage] started with %d configured nodes, replication=%d",
		len(m.cfg.StorageNodes), m.cfg.ReplicationFactor)
	return nil
}

// Stop gracefully shuts down the manager.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	log.Println("[web3storage] stopped")
	return nil
}

// IsRunning reports whether the manager is active.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ===================== CID helpers =====================

// GenerateCID produces a CID v1 from raw content.
// This is a deterministic simulation: sha2-256 of content → "bafy" prefix CIDv1.
func GenerateCID(content []byte, codec string) CID {
	if codec == "" {
		codec = "raw"
	}
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])
	return CID{
		Value:    "bafybeig" + hash[:52],
		Codec:    codec,
		HashFunc: "sha2-256",
		Version:  1,
		Size:     int64(len(content)),
	}
}

// ValidateCID performs syntactic validation of a CID string.
func ValidateCID(value string) error {
	if value == "" {
		return fmt.Errorf("empty CID")
	}
	if len(value) < 10 {
		return fmt.Errorf("CID too short: %d chars", len(value))
	}
	// CIDv0 starts with Qm, CIDv1 with a multibase prefix (b, f, z, etc.)
	if value[0] == 'Q' && len(value) >= 46 {
		return nil // likely CIDv0
	}
	if len(value) >= 10 && value[:4] == "bafy" {
		return nil // CIDv1 dag-pb / raw
	}
	return fmt.Errorf("unrecognised CID format: %q", value)
}

// ===================== Pin management =====================

// Pin adds or updates a content pin.
// If req.CID is set, that CID is pinned. If req.Content is set, it is added
// first (simulated) and the resulting CID is pinned.
func (m *Manager) Pin(req PinRequest) (*ContentPin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cid CID
	if len(req.Content) > 0 {
		codec := "raw"
		if strings.HasSuffix(strings.ToLower(req.FileName), ".json") {
			codec = "dag-cbor"
		}
		cid = GenerateCID(req.Content, codec)
	} else if req.CID != "" {
		if err := ValidateCID(req.CID); err != nil {
			return nil, fmt.Errorf("invalid CID: %w", err)
		}
		cid = CID{Value: req.CID, Codec: "raw", HashFunc: "sha2-256", Version: 1}
	} else {
		return nil, fmt.Errorf("either cid or content must be provided")
	}

	now := time.Now()
	pin := &ContentPin{
		CID:       cid,
		Status:    PinStatusPinned, // fast-path: assume pinned for simulation
		Name:      req.Name,
		Tags:      req.Tags,
		Metadata:  req.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Determine replication target.
	repl := m.cfg.ReplicationFactor
	if req.Replication > 0 {
		repl = req.Replication
	}

	// Assign replicas to available online nodes.
	var replicas []string
	for _, node := range m.nodes {
		if node.Status == NodeStatusOnline && len(replicas) < repl {
			replicas = append(replicas, node.ID)
		}
	}
	pin.PinnedByNodes = replicas
	pin.ReplicationCount = len(replicas)

	if pin.FileName() == "" && req.FileName != "" {
		if pin.Metadata == nil {
			pin.Metadata = make(map[string]string)
		}
		pin.Metadata["fileName"] = req.FileName
	}

	m.pins[cid.Value] = pin
	return pin, nil
}

// FileName returns the fileName metadata if present (helper on ContentPin).
func (p *ContentPin) FileName() string {
	if p.Metadata != nil {
		if fn, ok := p.Metadata["fileName"]; ok {
			return fn
		}
	}
	return ""
}

// Unpin removes a pin by CID.
func (m *Manager) Unpin(cid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pins[cid]
	if !ok {
		return fmt.Errorf("pin not found: %s", cid)
	}
	p.Status = PinStatusUnpinned
	p.UpdatedAt = time.Now()
	p.ReplicationCount = 0
	p.PinnedByNodes = nil
	return nil
}

// GetPin returns a pin by CID value.
func (m *Manager) GetPin(cid string) (*ContentPin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.pins[cid]
	if !ok {
		return nil, fmt.Errorf("pin not found: %s", cid)
	}
	// Return a shallow copy so callers can't mutate internal state.
	cp := *p
	return &cp, nil
}

// ListPins returns all pins that match the query filters.
func (m *Manager) ListPins(q PinQuery) PinListResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []ContentPin
	for _, p := range m.pins {
		if !matchPin(p, q) {
			continue
		}
		matched = append(matched, *p)
	}

	// Sort by CreatedAt descending for deterministic ordering.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)

	// Apply pagination.
	offset := q.Offset
	if offset > total {
		offset = total
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := matched[offset:end]

	return PinListResponse{
		Pins:    page,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: end < total,
	}
}

// matchPin checks if a pin satisfies the query filters.
func matchPin(p *ContentPin, q PinQuery) bool {
	// Status filter.
	if len(q.Status) > 0 {
		found := false
		for _, s := range q.Status {
			if p.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// Tag filter – pin must contain ALL queried tags.
	for _, t := range q.Tags {
		has := false
		for _, pt := range p.Tags {
			if pt == t {
				has = true
				break
			}
		}
		if !has {
			return false
		}
	}
	// Name substring match.
	if q.Name != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(q.Name)) {
		return false
	}
	// Metadata match – all queried k/v must be present.
	for k, v := range q.Metadata {
		if p.Metadata == nil {
			return false
		}
		if pv, ok := p.Metadata[k]; !ok || pv != v {
			return false
		}
	}
	// Time range.
	if q.Before != nil && p.CreatedAt.After(*q.Before) {
		return false
	}
	if q.After != nil && p.CreatedAt.Before(*q.After) {
		return false
	}
	return true
}

// ===================== Storage node management =====================

// AddNode registers or updates a storage node.
func (m *Manager) AddNode(node StorageNode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if node.LastSeen.IsZero() {
		node.LastSeen = time.Now()
	}
	m.nodes[node.ID] = &node
}

// RemoveNode unregisters a storage node.
func (m *Manager) RemoveNode(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, id)
}

// GetNode returns a storage node by ID.
func (m *Manager) GetNode(id string) (*StorageNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	cp := *n
	return &cp, nil
}

// ListNodes returns all registered storage nodes.
func (m *Manager) ListNodes() []StorageNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]StorageNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ===================== Decentralized backup / replication =====================

// Replicate ensures a CID is replicated to at least the configured number of nodes.
// Returns the node IDs that now hold the content.
func (m *Manager) Replicate(cid string, minCopies int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pins[cid]
	if !ok {
		return nil, fmt.Errorf("pin not found: %s", cid)
	}
	if minCopies <= 0 {
		minCopies = m.cfg.ReplicationFactor
	}

	// Already satisfied?
	if p.ReplicationCount >= minCopies {
		return p.PinnedByNodes, nil
	}

	// Find online nodes that don't already hold this pin.
	var candidates []string
	for _, node := range m.nodes {
		if node.Status != NodeStatusOnline {
			continue
		}
		already := false
		for _, id := range p.PinnedByNodes {
			if id == node.ID {
				already = true
				break
			}
		}
		if !already {
			candidates = append(candidates, node.ID)
		}
	}

	needed := minCopies - p.ReplicationCount
	if needed > len(candidates) {
		needed = len(candidates)
	}

	for i := 0; i < needed; i++ {
		p.PinnedByNodes = append(p.PinnedByNodes, candidates[i])
	}
	p.ReplicationCount = len(p.PinnedByNodes)
	p.UpdatedAt = time.Now()

	if p.ReplicationCount < minCopies {
		log.Printf("[web3storage] warning: CID %s replicated to %d/%d nodes",
			cid, p.ReplicationCount, minCopies)
	}
	return p.PinnedByNodes, nil
}

// ===================== Filecoin deal management =====================

// CreateDeal initiates a Filecoin storage deal (simulation).
func (m *Manager) CreateDeal(cid string, provider string, epochs int64, label string) (*FilecoinDeal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pins[cid]; !ok {
		return nil, fmt.Errorf("pin not found: %s – content must be pinned before making a deal", cid)
	}
	if provider == "" {
		return nil, fmt.Errorf("provider (miner address) is required")
	}
	if epochs <= 0 {
		epochs = int64(m.cfg.MinDealDuration.Seconds() / 30) // ~30s epochs
	}

	deal := &FilecoinDeal{
		DealID:        fmt.Sprintf("deal-%d", time.Now().UnixNano()),
		CID:           CID{Value: cid, Version: 1},
		Provider:      provider,
		State:         DealStateProposing,
		PricePerEpoch: "0",
		StartEpoch:    0,
		EndEpoch:      epochs,
		CreatedAt:     time.Now(),
		Label:         label,
	}
	m.deals[deal.DealID] = deal
	return deal, nil
}

// GetDeal returns a Filecoin deal by ID.
func (m *Manager) GetDeal(dealID string) (*FilecoinDeal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.deals[dealID]
	if !ok {
		return nil, fmt.Errorf("deal not found: %s", dealID)
	}
	cp := *d
	return &cp, nil
}

// ListDeals returns all deals, optionally filtered by state.
func (m *Manager) ListDeals(state *DealState) []FilecoinDeal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []FilecoinDeal
	for _, d := range m.deals {
		if state != nil && d.State != *state {
			continue
		}
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// UpdateDealState changes the state of an existing deal.
func (m *Manager) UpdateDealState(dealID string, newState DealState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deals[dealID]
	if !ok {
		return fmt.Errorf("deal not found: %s", dealID)
	}
	d.State = newState
	return nil
}

// ===================== Local cache =====================

// CacheContent stores content locally for fast gateway serving.
func (m *Manager) CacheContent(cid CID, localPath string, size int64) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Evict if necessary.
	for m.cacheSize+size > m.cfg.LocalCacheMaxSize && len(m.cache) > 0 {
		m.evictOldest()
	}

	now := time.Now()
	entry := &CacheEntry{
		CID:          cid,
		LocalPath:    localPath,
		Size:         size,
		HitCount:     0,
		LastAccessed: now,
		CreatedAt:    now,
		ExpiresAt:    now.Add(m.cfg.CacheTTL),
	}
	m.cache[cid.Value] = entry
	m.cacheSize += size
	return entry, nil
}

// GetFromCache retrieves a CID from the local cache.
// Returns nil, false if the entry is missing or expired.
func (m *Manager) GetFromCache(cid string) (*CacheEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.cache[cid]
	if !ok {
		atomic.AddInt64(&m.cacheMisses, 1)
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		// Expired – remove.
		m.cacheSize -= entry.Size
		delete(m.cache, cid)
		atomic.AddInt64(&m.cacheMisses, 1)
		return nil, false
	}
	entry.HitCount++
	entry.LastAccessed = time.Now()
	atomic.AddInt64(&m.cacheHits, 1)
	cp := *entry
	return &cp, true
}

// EvictCache removes a specific CID from cache.
func (m *Manager) EvictCache(cid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.cache[cid]; ok {
		m.cacheSize -= e.Size
		delete(m.cache, cid)
	}
}

// evictOldest removes the least-recently-accessed cache entry. Caller must hold m.mu.
func (m *Manager) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, e := range m.cache {
		if oldestKey == "" || e.LastAccessed.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.LastAccessed
		}
	}
	if oldestKey != "" {
		m.cacheSize -= m.cache[oldestKey].Size
		delete(m.cache, oldestKey)
	}
}

// GetCacheStats returns aggregate cache statistics.
func (m *Manager) GetCacheStats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hits := atomic.LoadInt64(&m.cacheHits)
	misses := atomic.LoadInt64(&m.cacheMisses)
	total := hits + misses
	var rate float64
	if total > 0 {
		rate = float64(hits) / float64(total)
	}
	return CacheStats{
		TotalEntries: len(m.cache),
		TotalSize:    m.cacheSize,
		MaxSize:      m.cfg.LocalCacheMaxSize,
		HitCount:     hits,
		MissCount:    misses,
		HitRate:      rate,
	}
}

// ===================== Gateway statistics =====================

// RecordGatewayRequest increments the gateway request counter and byte total.
func (m *Manager) RecordGatewayRequest(bytesServed int64) {
	atomic.AddInt64(&m.gatewayReqs, 1)
	atomic.AddInt64(&m.gatewayBytes, bytesServed)
}

// GetGatewayStats returns aggregate gateway statistics.
func (m *Manager) GetGatewayStats() GatewayStats {
	return GatewayStats{
		TotalRequests:    atomic.LoadInt64(&m.gatewayReqs),
		CacheHits:        atomic.LoadInt64(&m.cacheHits),
		CacheMisses:      atomic.LoadInt64(&m.cacheMisses),
		TotalBytesServed: atomic.LoadInt64(&m.gatewayBytes),
	}
}
