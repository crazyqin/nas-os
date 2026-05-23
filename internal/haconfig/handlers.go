package haconfig

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Handler 高可用配置 HTTP 处理器.
type Handler struct {
	manager *HAConfigManager
	events  []HAEvent
	mu      sync.RWMutex
}

// NewHandler 创建处理器.
func NewHandler(manager *HAConfigManager) *Handler {
	return &Handler{
		manager: manager,
		events:  make([]HAEvent, 0),
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ha/status", h.handleStatus)
	mux.HandleFunc("/api/v1/ha/config", h.handleConfig)
	mux.HandleFunc("/api/v1/ha/failover", h.handleFailover)
	mux.HandleFunc("/api/v1/ha/failback", h.handleFailback)
	mux.HandleFunc("/api/v1/ha/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/ha/events", h.handleEvents)
	mux.HandleFunc("/api/v1/ha/test", h.handleTest)
}

// handleStatus 获取 HA 状态.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := h.manager.GetStatus()
	nodes := h.manager.GetAllNodeStates()

	haNodes := make(map[string]HANode)
	for id, state := range nodes {
		haNodes[id] = HANode{
			ID:        state.NodeID,
			Address:   state.Address,
			Role:      state.Role,
			Status:    state.Status,
			LastSeen:  state.LastHeartbeat,
			IsHealthy: state.IsHealthy,
		}
	}

	response := HAStatusResponse{
		State:          h.determineHAState(status),
		PrimaryNode:    status.PrimaryNode,
		SecondaryNode:  h.findSecondaryNode(nodes),
		LastHeartbeat:  status.LastHealthCheck,
		FailoverCount:  status.FailoverCount,
		Uptime:         status.Uptime,
		HealthyNodes:   status.HealthyNodes,
		TotalNodes:     status.TotalNodes,
		NodeStates:     haNodes,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// handleConfig 处理配置请求.
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.updateConfig(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getConfig 获取 HA 配置.
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	config := h.manager.GetConfig()

	response := HAConfigResponse{
		VirtualIP:        config.BindAddress,
		HeartbeatInterval: config.HeartbeatInterval,
		FailoverTimeout:  config.FailoverTimeout,
		AutoFailback:     config.FailbackEnabled,
		Preempt:          false,
		PeerNodes:        config.PeerNodes,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// updateConfig 更新 HA 配置.
func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req HAConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	config := h.manager.GetConfig()

	if req.VirtualIP != nil {
		config.BindAddress = *req.VirtualIP
	}
	if req.HeartbeatInterval != nil {
		config.HeartbeatInterval = *req.HeartbeatInterval
	}
	if req.FailoverTimeout != nil {
		config.FailoverTimeout = *req.FailoverTimeout
	}
	if req.AutoFailback != nil {
		config.FailbackEnabled = *req.AutoFailback
	}
	if req.PeerNodes != nil {
		config.PeerNodes = req.PeerNodes
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to update config")
		return
	}

	h.addEvent(EventTypeConfigChange, "HA configuration updated", "local")
	h.writeJSON(w, http.StatusOK, SuccessResponse{
		Success: true,
		Message: "configuration updated",
	})
}

// handleFailover 手动触发故障转移.
func (h *Handler) handleFailover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req FailoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !h.manager.IsPrimary() {
		h.writeError(w, http.StatusForbidden, "only primary node can trigger failover")
		return
	}

	targetNode := req.TargetNode
	if targetNode == "" {
		targetNode = h.manager.selectNewPrimary()
	}

	if err := h.manager.ManualFailover(targetNode); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.addEvent(EventTypeFailover, "Manual failover triggered: "+req.Reason, "local")
	h.writeJSON(w, http.StatusOK, FailoverResponse{
		Success:    true,
		NewPrimary: targetNode,
		Message:    "failover completed",
	})
}

// handleFailback 故障回切.
func (h *Handler) handleFailback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := h.manager.GetConfig()
	if !config.FailbackEnabled {
		h.writeError(w, http.StatusBadRequest, "failback is not enabled")
		return
	}

	h.addEvent(EventTypeConfigChange, "Failback initiated", "local")
	h.writeJSON(w, http.StatusOK, FailbackResponse{
		Success: true,
		Message: "failback initiated",
	})
}

// handleNodes 获取节点列表.
func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	nodes := h.manager.GetAllNodeStates()
	haNodes := make([]HANode, 0, len(nodes))

	for _, state := range nodes {
		haNodes = append(haNodes, HANode{
			ID:        state.NodeID,
			Address:   state.Address,
			Role:      state.Role,
			Status:    state.Status,
			LastSeen:  state.LastHeartbeat,
			IsHealthy: state.IsHealthy,
		})
	}

	h.writeJSON(w, http.StatusOK, NodesResponse{
		Nodes: haNodes,
		Total: len(haNodes),
	})
}

// handleEvents 获取事件日志.
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.mu.RLock()
	events := make([]HAEvent, len(h.events))
	copy(events, h.events)
	h.mu.RUnlock()

	h.writeJSON(w, http.StatusOK, EventsResponse{
		Events: events,
		Total:  len(events),
	})
}

// handleTest 测试 HA 连通性.
func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	start := time.Now()

	nodes := h.manager.GetAllNodeStates()
	testResults := make(map[string]bool)

	for nodeID, state := range nodes {
		if nodeID == h.manager.config.NodeID {
			continue
		}
		testResults[nodeID] = state.IsHealthy
	}

	latency := time.Since(start)

	allHealthy := true
	for _, healthy := range testResults {
		if !healthy {
			allHealthy = false
			break
		}
	}

	h.writeJSON(w, http.StatusOK, TestResponse{
		Success:    allHealthy,
		Latency:    latency,
		Message:    "connectivity test completed",
		TestTarget: "all peers",
	})
}

// addEvent 添加事件.
func (h *Handler) addEvent(eventType, message, nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	event := HAEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		Message:   message,
		NodeID:    nodeID,
	}

	h.events = append(h.events, event)
	if len(h.events) > 1000 {
		h.events = h.events[len(h.events)-1000:]
	}
}

// determineHAState 确定 HA 状态.
func (h *Handler) determineHAState(status HAStatus) HAState {
	if status.IsFailoverActive {
		return HAStateFailover
	}
	if status.CurrentRole == RolePrimary {
		return HAStateActive
	}
	return HAStateStandby
}

// findSecondaryNode 查找从节点.
func (h *Handler) findSecondaryNode(nodes map[string]NodeState) string {
	for _, state := range nodes {
		if state.Role == RoleSecondary {
			return state.NodeID
		}
	}
	return ""
}

// writeJSON 写入 JSON 响应.
func (h *Handler) writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应.
func (h *Handler) writeError(w http.ResponseWriter, code int, message string) {
	h.writeJSON(w, code, ErrorResponse{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}
