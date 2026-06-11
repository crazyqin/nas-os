// Package containerha 提供容器高可用故障转移功能
package containerha

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// ContainerHAHandler HTTP API处理器
type ContainerHAHandler struct {
	manager *FailoverManager
}

// NewContainerHAHandler 创建新的处理器
func NewContainerHAHandler(manager *FailoverManager) *ContainerHAHandler {
	return &ContainerHAHandler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *ContainerHAHandler) RegisterRoutes(mux *http.ServeMux) {
	// 容器HA状态
	mux.HandleFunc("/api/v1/containerha/status", h.handleStatus)

	// 故障转移操作
	mux.HandleFunc("/api/v1/containerha/failover", h.handleFailover)

	// 配置管理
	mux.HandleFunc("/api/v1/containerha/config", h.handleConfig)

	// 节点管理
	mux.HandleFunc("/api/v1/containerha/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/containerha/nodes/", h.handleNodeByID)

	// 容器管理
	mux.HandleFunc("/api/v1/containerha/containers", h.handleContainers)
	mux.HandleFunc("/api/v1/containerha/containers/", h.handleContainerByID)

	// 同步状态
	mux.HandleFunc("/api/v1/containerha/sync", h.handleSync)
	mux.HandleFunc("/api/v1/containerha/sync/status", h.handleSyncStatus)

	// 故障转移历史
	mux.HandleFunc("/api/v1/containerha/history", h.handleHistory)

	// 健康检查
	mux.HandleFunc("/api/v1/containerha/health", h.handleHealth)
	mux.HandleFunc("/api/v1/containerha/health/", h.handleHealthByNode)

	// 心跳接收
	mux.HandleFunc("/api/v1/containerha/heartbeat", h.handleHeartbeat)
}

// handleStatus 处理状态查询
func (h *ContainerHAHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	status := h.manager.GetStatus()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// handleFailover 处理故障转移操作
func (h *ContainerHAHandler) handleFailover(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取故障转移历史
		history := h.manager.GetFailoverHistory()
		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    history,
		})

	case http.MethodPost:
		// 执行故障转移
		var request FailoverRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("请求解析失败: %v", err))
			return
		}

		if len(request.Containers) == 0 {
			// 如果未指定容器，故障转移所有受保护容器
			containers := h.manager.GetAllProtectedContainers()
			for _, c := range containers {
				if c.Status == "running" {
					request.Containers = append(request.Containers, c.ContainerID)
				}
			}
		}

		if len(request.Containers) == 0 {
			h.writeError(w, http.StatusBadRequest, "没有可故障转移的容器")
			return
		}

		response, err := h.manager.ExecuteFailover(&request)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("故障转移失败: %v", err))
			return
		}

		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    response,
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET/POST 方法")
	}
}

// handleConfig 处理配置操作
func (h *ContainerHAHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取配置
		config := h.manager.GetConfig()
		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    config,
		})

	case http.MethodPut:
		// 更新配置
		var config ContainerHAConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("配置解析失败: %v", err))
			return
		}

		if err := h.manager.UpdateConfig(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("配置更新失败: %v", err))
			return
		}

		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Message: "配置更新成功",
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET/PUT 方法")
	}
}

// handleNodes 处理节点操作
func (h *ContainerHAHandler) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	nodes := h.manager.GetNodes()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    nodes,
	})
}

// handleNodeByID 处理单个节点操作
func (h *ContainerHAHandler) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 从路径中提取节点ID
	nodeID := extractIDFromPath(r.URL.Path, "/api/v1/containerha/nodes/")
	if nodeID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少节点ID")
		return
	}

	node, err := h.manager.GetNode(nodeID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("节点不存在: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    node,
	})
}

// handleContainers 处理容器操作
func (h *ContainerHAHandler) handleContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	containers := h.manager.GetAllProtectedContainers()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    containers,
	})
}

// handleContainerByID 处理单个容器操作
func (h *ContainerHAHandler) handleContainerByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 从路径中提取容器ID
	containerID := extractIDFromPath(r.URL.Path, "/api/v1/containerha/containers/")
	if containerID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少容器ID")
		return
	}

	container, err := h.manager.GetProtectedContainer(containerID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("容器不存在: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    container,
	})
}

// handleSync 处理同步操作
func (h *ContainerHAHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	// 触发同步
	if err := h.manager.SyncNow(); err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("同步触发失败: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "同步已触发",
	})
}

// handleSyncStatus 处理同步状态查询
func (h *ContainerHAHandler) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	status := h.manager.GetSyncStatus()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// handleHistory 处理历史查询
func (h *ContainerHAHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	history := h.manager.GetFailoverHistory()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    history,
	})
}

// handleHealth 处理健康检查
func (h *ContainerHAHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 返回集群整体健康状态
	status := h.manager.GetStatus()
	healthStatus := map[string]interface{}{
		"clusterStatus": status.ClusterStatus,
		"nodeCount":     len(status.Nodes),
		"onlineNodes":   countOnlineNodes(status.Nodes),
		"uptime":        status.Uptime,
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    healthStatus,
	})
}

// handleHealthByNode 处理节点健康检查
func (h *ContainerHAHandler) handleHealthByNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 从路径中提取节点ID
	nodeID := extractIDFromPath(r.URL.Path, "/api/v1/containerha/health/")
	if nodeID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少节点ID")
		return
	}

	result := h.manager.healthChecker.GetCheckResult(nodeID)
	if result == nil {
		h.writeError(w, http.StatusNotFound, "没有该节点的健康检查结果")
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// handleHeartbeat 处理心跳消息
func (h *ContainerHAHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var heartbeat HeartbeatMessage
	if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("心跳消息解析失败: %v", err))
		return
	}

	if err := h.manager.ProcessHeartbeat(&heartbeat); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("心跳处理失败: %v", err))
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "心跳已处理",
	})
}

// writeJSON 写入JSON响应
func (h *ContainerHAHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[ContainerHA] JSON编码失败: %v", err)
	}
}

// writeError 写入错误响应
func (h *ContainerHAHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	h.writeJSON(w, statusCode, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    statusCode,
			Message: message,
		},
	})
}

// extractIDFromPath 从路径中提取ID
func extractIDFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	id := strings.TrimPrefix(path, prefix)
	// 移除尾部的斜杠
	id = strings.TrimSuffix(id, "/")

	if id == "" {
		return ""
	}

	return id
}

// countOnlineNodes 计算在线节点数
func countOnlineNodes(nodes []ContainerHANode) int {
	count := 0
	for _, node := range nodes {
		if node.Status == "online" {
			count++
		}
	}
	return count
}

// APIInfo API信息响应
type APIInfo struct {
	Version     string   `json:"version"`
	Endpoints   []string `json:"endpoints"`
	Description string   `json:"description"`
}

// handleAPIInfo 处理API信息查询
func (h *ContainerHAHandler) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	info := APIInfo{
		Version: "1.0.0",
		Endpoints: []string{
			"GET  /api/v1/containerha/status - 获取集群状态",
			"POST /api/v1/containerha/failover - 执行故障转移",
			"GET  /api/v1/containerha/config - 获取配置",
			"PUT  /api/v1/containerha/config - 更新配置",
			"GET  /api/v1/containerha/nodes - 获取所有节点",
			"GET  /api/v1/containerha/nodes/{id} - 获取指定节点",
			"GET  /api/v1/containerha/containers - 获取所有容器",
			"GET  /api/v1/containerha/containers/{id} - 获取指定容器",
			"POST /api/v1/containerha/sync - 触发同步",
			"GET  /api/v1/containerha/sync/status - 获取同步状态",
			"GET  /api/v1/containerha/history - 获取故障转移历史",
			"GET  /api/v1/containerha/health - 获取健康状态",
			"GET  /api/v1/containerha/health/{nodeId} - 获取节点健康状态",
			"POST /api/v1/containerha/heartbeat - 接收心跳消息",
		},
		Description: "容器高可用故障转移API",
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    info,
	})
}

// SetupHTTPServer 设置HTTP服务器
func SetupHTTPServer(handler *ContainerHAHandler, addr string) *http.Server {
	mux := http.NewServeMux()

	// 注册路由
	handler.RegisterRoutes(mux)

	// 添加API信息端点
	mux.HandleFunc("/api/v1/containerha", handler.handleAPIInfo)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server
}

// StartHTTPServer 启动HTTP服务器
func StartHTTPServer(handler *ContainerHAHandler, addr string) error {
	server := SetupHTTPServer(handler, addr)

	log.Printf("[ContainerHA] HTTP服务器启动在 %s", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP服务器启动失败: %v", err)
	}

	return nil
}

// CorsMiddleware CORS中间件
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[ContainerHA] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
