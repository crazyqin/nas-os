package web3storage

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Handler provides HTTP handlers for Web3Storage management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Web3Storage HTTP handler.
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes registers all Web3Storage API routes on the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Pin management
	mux.HandleFunc("/api/v1/web3storage/pins", h.handlePins)
	mux.HandleFunc("/api/v1/web3storage/pins/query", h.handlePinQuery)
	mux.HandleFunc("/api/v1/web3storage/pins/", h.handlePinByCID)

	// Content add
	mux.HandleFunc("/api/v1/web3storage/add", h.handleAdd)

	// Nodes
	mux.HandleFunc("/api/v1/web3storage/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/web3storage/nodes/", h.handleNodeByID)

	// Replication
	mux.HandleFunc("/api/v1/web3storage/replicate", h.handleReplicate)

	// Filecoin deals
	mux.HandleFunc("/api/v1/web3storage/deals", h.handleDeals)
	mux.HandleFunc("/api/v1/web3storage/deals/", h.handleDealByID)

	// Cache
	mux.HandleFunc("/api/v1/web3storage/cache/stats", h.handleCacheStats)
	mux.HandleFunc("/api/v1/web3storage/cache/evict", h.handleCacheEvict)

	// Gateway proxy
	mux.HandleFunc("/api/v1/web3storage/gateway/", h.handleGateway)
	mux.HandleFunc("/api/v1/web3storage/gateway/stats", h.handleGatewayStats)

	// Service control
	mux.HandleFunc("/api/v1/web3storage/status", h.handleStatus)
	mux.HandleFunc("/api/v1/web3storage/start", h.handleStart)
	mux.HandleFunc("/api/v1/web3storage/stop", h.handleStop)
}

// ===================== Pin Handlers =====================

// handlePins handles GET (list) and POST (create) for /pins.
func (h *Handler) handlePins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListPins(w, r)
	case http.MethodPost:
		h.handleCreatePin(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleListPins(w http.ResponseWriter, _ *http.Request) {
	result := h.manager.ListPins(PinQuery{})
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCreatePin(w http.ResponseWriter, r *http.Request) {
	var req PinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.CID == "" && len(req.Content) == 0 {
		http.Error(w, "either cid or content is required", http.StatusBadRequest)
		return
	}
	pin, err := h.manager.Pin(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, pin)
}

// handlePinQuery handles POST /pins/query with a PinQuery body.
func (h *Handler) handlePinQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var q PinQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := h.manager.ListPins(q)
	writeJSON(w, http.StatusOK, result)
}

// handlePinByCID handles GET and DELETE for /pins/{cid}.
func (h *Handler) handlePinByCID(w http.ResponseWriter, r *http.Request) {
	cid := strings.TrimPrefix(r.URL.Path, "/api/v1/web3storage/pins/")
	if cid == "" {
		http.Error(w, "cid is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		pin, err := h.manager.GetPin(cid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, pin)
	case http.MethodDelete:
		if err := h.manager.Unpin(cid); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "unpinned", "cid": cid})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ===================== Content Add =====================

// handleAdd handles POST /add – accepts raw binary content in the body.
func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	fileName := r.Header.Get("X-File-Name")
	name := r.Header.Get("X-Name")

	pin, err := h.manager.Pin(PinRequest{
		Content:  body,
		FileName: fileName,
		Name:     name,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"cid":  pin.CID.Value,
		"size": pin.CID.Size,
		"pin":  pin,
	})
}

// ===================== Node Handlers =====================

func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.manager.ListNodes())
	case http.MethodPost:
		var node StorageNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if node.ID == "" {
			http.Error(w, "node id is required", http.StatusBadRequest)
			return
		}
		h.manager.AddNode(node)
		writeJSON(w, http.StatusCreated, node)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/web3storage/nodes/")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		node, err := h.manager.GetNode(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, node)
	case http.MethodDelete:
		h.manager.RemoveNode(id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ===================== Replication =====================

func (h *Handler) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		CID        string `json:"cid"`
		MinCopies  int    `json:"minCopies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.CID == "" {
		http.Error(w, "cid is required", http.StatusBadRequest)
		return
	}
	nodes, err := h.manager.Replicate(req.CID, req.MinCopies)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cid":           req.CID,
		"replicaNodes":  nodes,
		"replicaCount":  len(nodes),
	})
}

// ===================== Filecoin Deal Handlers =====================

func (h *Handler) handleDeals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deals := h.manager.ListDeals(nil)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deals": deals,
			"total": len(deals),
		})
	case http.MethodPost:
		var req struct {
			CID      string `json:"cid"`
			Provider string `json:"provider"`
			Epochs   int64  `json:"epochs"`
			Label    string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.CID == "" || req.Provider == "" {
			http.Error(w, "cid and provider are required", http.StatusBadRequest)
			return
		}
		deal, err := h.manager.CreateDeal(req.CID, req.Provider, req.Epochs, req.Label)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, deal)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleDealByID(w http.ResponseWriter, r *http.Request) {
	dealID := strings.TrimPrefix(r.URL.Path, "/api/v1/web3storage/deals/")
	if dealID == "" {
		http.Error(w, "deal id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		deal, err := h.manager.GetDeal(dealID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, deal)
	case http.MethodPatch:
		var req struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		newState, err := parseDealState(req.State)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateDealState(dealID, newState); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		deal, _ := h.manager.GetDeal(dealID)
		writeJSON(w, http.StatusOK, deal)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseDealState(s string) (DealState, error) {
	switch strings.ToLower(s) {
	case "proposing":
		return DealStateProposing, nil
	case "published":
		return DealStatePublished, nil
	case "active":
		return DealStateActive, nil
	case "expired":
		return DealStateExpired, nil
	case "slashed":
		return DealStateSlashed, nil
	case "failed":
		return DealStateFailed, nil
	default:
		return 0, fmt.Errorf("unknown deal state: %s", s)
	}
}

// ===================== Cache Handlers =====================

func (h *Handler) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetCacheStats())
}

func (h *Handler) handleCacheEvict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.CID == "" {
		http.Error(w, "cid is required", http.StatusBadRequest)
		return
	}
	h.manager.EvictCache(req.CID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "evicted", "cid": req.CID})
}

// ===================== Gateway Proxy =====================

// handleGateway proxies IPFS content retrieval. Tries local cache first, then
// returns the public gateway URL for the client to fetch.
func (h *Handler) handleGateway(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cid := strings.TrimPrefix(r.URL.Path, "/api/v1/web3storage/gateway/")
	if cid == "" {
		http.Error(w, "cid is required", http.StatusBadRequest)
		return
	}

	start := time.Now()

	// Try cache.
	if entry, ok := h.manager.GetFromCache(cid); ok {
		// Serve from local file.
		http.ServeFile(w, r, entry.LocalPath)
		h.manager.RecordGatewayRequest(entry.Size)
		return
	}

	// Cache miss – redirect to public gateway.
	publicURL := h.manager.cfg.GatewayURL + cid
	h.manager.RecordGatewayRequest(0)
	elapsed := time.Since(start)
	log.Printf("[web3storage] gateway cache miss for %s, redirect took %v", cid, elapsed)
	http.Redirect(w, r, publicURL, http.StatusTemporaryRedirect)
}

func (h *Handler) handleGatewayStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetGatewayStats())
}

// ===================== Service Control =====================

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":      h.manager.IsRunning(),
		"pins":         len(h.manager.pins),
		"nodes":        len(h.manager.nodes),
		"deals":        len(h.manager.deals),
		"cacheEntries": len(h.manager.cache),
		"cacheSize":    h.manager.cacheSize,
	})
}

func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.manager.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.manager.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// ===================== Helpers =====================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}


