// Package containerscan provides Docker image vulnerability scanning with CVE detection,
// layer analysis, severity rating, auto-fix suggestions, scheduled scanning, and report generation.
package containerscan

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Handler provides HTTP handlers for container scan operations.
type Handler struct {
	manager *Manager
	scanner *Scanner
	logger  *zap.Logger
}

// NewHandler creates a new container scan HTTP handler.
func NewHandler(manager *Manager, scanner *Scanner, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		scanner: scanner,
		logger:  logger,
	}
}

// RegisterRoutes registers container scan API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/containerscan/scan", h.handleScan)
	mux.HandleFunc("/api/v1/containerscan/scan/", h.handleScanResult)
	mux.HandleFunc("/api/v1/containerscan/cache", h.handleCache)
	mux.HandleFunc("/api/v1/containerscan/schedules", h.handleSchedules)
	mux.HandleFunc("/api/v1/containerscan/schedules/", h.handleScheduleByID)
	mux.HandleFunc("/api/v1/containerscan/whitelist", h.handleWhitelist)
	mux.HandleFunc("/api/v1/containerscan/whitelist/", h.handleWhitelistEntry)
	mux.HandleFunc("/api/v1/containerscan/blacklist", h.handleBlacklist)
	mux.HandleFunc("/api/v1/containerscan/blacklist/", h.handleBlacklistEntry)
	mux.HandleFunc("/api/v1/containerscan/reports", h.handleReports)
	mux.HandleFunc("/api/v1/containerscan/reports/", h.handleReportByID)
}

// handleScan handles scan requests.
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.startScan(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleScanResult handles scan result requests.
func (h *Handler) handleScanResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract image from path: /api/v1/containerscan/scan/{image}
	image := strings.TrimPrefix(r.URL.Path, "/api/v1/containerscan/scan/")
	if image == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image parameter required"})
		return
	}

	result := h.scanner.GetCachedResult(image)
	if result == nil {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no scan result found for image"})
		return
	}

	h.writeJSON(w, http.StatusOK, result)
}

// handleCache handles cache operations.
func (h *Handler) handleCache(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		images := h.scanner.ListCachedImages()
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"images": images})
	case http.MethodDelete:
		h.scanner.ClearCache()
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "cache cleared"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSchedules handles schedule list/create requests.
func (h *Handler) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		schedules := h.manager.ListSchedules()
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"schedules": schedules})
	case http.MethodPost:
		h.createSchedule(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleScheduleByID handles schedule get/update/delete by ID.
func (h *Handler) handleScheduleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/containerscan/schedules/")
	if id == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		schedule := h.manager.GetSchedule(id)
		if schedule == nil {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
			return
		}
		h.writeJSON(w, http.StatusOK, schedule)
	case http.MethodPut:
		h.updateSchedule(w, r, id)
	case http.MethodDelete:
		if !h.manager.DeleteSchedule(id) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "schedule deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWhitelist handles whitelist list/add requests.
func (h *Handler) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := h.manager.ListWhitelist()
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"whitelist": entries})
	case http.MethodPost:
		h.addToList(w, r, ListTypeWhite)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWhitelistEntry handles whitelist entry removal.
func (h *Handler) handleWhitelistEntry(w http.ResponseWriter, r *http.Request) {
	image := strings.TrimPrefix(r.URL.Path, "/api/v1/containerscan/whitelist/")
	if image == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image parameter required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !h.manager.IsWhitelisted(image) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not in whitelist"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"image": image, "status": "whitelisted"})
	case http.MethodDelete:
		if !h.manager.RemoveFromWhitelist(image) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not in whitelist"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "image removed from whitelist"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBlacklist handles blacklist list/add requests.
func (h *Handler) handleBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := h.manager.ListBlacklist()
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"blacklist": entries})
	case http.MethodPost:
		h.addToList(w, r, ListTypeBlack)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBlacklistEntry handles blacklist entry removal.
func (h *Handler) handleBlacklistEntry(w http.ResponseWriter, r *http.Request) {
	image := strings.TrimPrefix(r.URL.Path, "/api/v1/containerscan/blacklist/")
	if image == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image parameter required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !h.manager.IsBlacklisted(image) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not in blacklist"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"image": image, "status": "blacklisted"})
	case http.MethodDelete:
		if !h.manager.RemoveFromBlacklist(image) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not in blacklist"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "image removed from blacklist"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReports handles report list requests.
func (h *Handler) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reports := h.manager.ListReports()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{"reports": reports})
}

// handleReportByID handles report get/delete by ID.
func (h *Handler) handleReportByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/containerscan/reports/")
	if id == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "report ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		report := h.manager.GetReport(id)
		if report == nil {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "report not found"})
			return
		}
		h.writeJSON(w, http.StatusOK, report)
	case http.MethodDelete:
		if !h.manager.DeleteReport(id) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "report not found"})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "report deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// startScan initiates a new scan.
func (h *Handler) startScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Image == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}

	// Check blacklist
	if h.manager.IsBlacklisted(req.Image) {
		h.writeJSON(w, http.StatusForbidden, map[string]string{"error": "image is blacklisted"})
		return
	}

	scanID := generateScanID()

	// Start scan in background
	go func() {
		result, err := h.scanner.ScanImage(r.Context(), req.Image, req.Registry, req.ForceRescan)
		if err != nil {
			h.logger.Error("scan failed",
				zap.String("scan_id", scanID),
				zap.String("image", req.Image),
				zap.Error(err))
			return
		}

		h.logger.Info("scan completed",
			zap.String("scan_id", scanID),
			zap.String("image", req.Image),
			zap.Int("vulns", result.Summary.Total))
	}()

	h.writeJSON(w, http.StatusAccepted, ScanResponse{
		ScanID: scanID,
		Image:  req.Image,
		Status: StatusQueued,
	})
}

// createSchedule creates a new scan schedule.
func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule ScanSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.manager.AddSchedule(&schedule); err != nil {
		h.writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, schedule)
}

// updateSchedule updates an existing schedule.
func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request, id string) {
	var schedule ScanSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.manager.UpdateSchedule(id, &schedule); err != nil {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "schedule updated"})
}

// addToList adds an image to whitelist or blacklist.
func (h *Handler) addToList(w http.ResponseWriter, r *http.Request, listType ListType) {
	var entry ImageListEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if entry.Image == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}

	switch listType {
	case ListTypeWhite:
		h.manager.AddToWhitelist(entry.Image, entry.Reason, entry.AddedBy)
	case ListTypeBlack:
		h.manager.AddToBlacklist(entry.Image, entry.Reason, entry.AddedBy)
	}

	h.writeJSON(w, http.StatusCreated, entry)
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to write JSON response", zap.Error(err))
	}
}

// generateScanID creates a unique scan ID.
func generateScanID() string {
	return fmt.Sprintf("scan-%s-%s", time.Now().Format("20060102-150405"), randomHex(4))
}

// randomHex generates a random hex string.
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}
