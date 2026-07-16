package mcpserver

import (
	"encoding/json"
	"net/http"
)

// HTTPHandler provides HTTP endpoints for the MCP server.
type HTTPHandler struct {
	server *MCPServer
}

// NewHTTPHandler creates a new HTTP handler for the MCP server.
func NewHTTPHandler(server *MCPServer) *HTTPHandler {
	return &HTTPHandler{server: server}
}

// RegisterRoutes registers MCP HTTP routes.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/mcp/tools", h.handleTools)
	mux.HandleFunc("/mcp/tools/invoke", h.handleInvokeTool)
	mux.HandleFunc("/mcp/resources", h.handleResources)
	mux.HandleFunc("/mcp/resources/read", h.handleReadResource)
	mux.HandleFunc("/mcp/prompts", h.handlePrompts)
	mux.HandleFunc("/mcp/prompts/get", h.handleGetPrompt)
	mux.HandleFunc("/mcp/status", h.handleStatus)
	mux.HandleFunc("/mcp/metrics", h.handleMetrics)
}

func (h *HTTPHandler) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tools := h.server.ListTools()
	writeJSON(w, tools)
}

func (h *HTTPHandler) handleInvokeTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string                 `json:"name"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.server.InvokeTool(r.Context(), req.Name, req.Params)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, result)
}

func (h *HTTPHandler) handleResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resources := h.server.ListResources()
	writeJSON(w, resources)
}

func (h *HTTPHandler) handleReadResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uri := r.URL.Query().Get("uri")
	if uri == "" {
		http.Error(w, "Missing 'uri' parameter", http.StatusBadRequest)
		return
	}

	data, err := h.server.ReadResource(r.Context(), uri)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func (h *HTTPHandler) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prompts := h.server.ListPrompts()
	writeJSON(w, prompts)
}

func (h *HTTPHandler) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string            `json:"name"`
		Args map[string]string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.server.GetPrompt(r.Context(), req.Name, req.Args)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"result": result})
}

func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := h.server.ToJSON()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *HTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics := h.server.GetMetrics()
	writeJSON(w, metrics)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
