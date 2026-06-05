package federatedlearn

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/federated/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/federated/node", h.handleNode)
	mux.HandleFunc("/api/v1/federated/models", h.handleModels)
	mux.HandleFunc("/api/v1/federated/model", h.handleModel)
	mux.HandleFunc("/api/v1/federated/train", h.handleTrain)
	mux.HandleFunc("/api/v1/federated/round", h.handleRound)
	mux.HandleFunc("/api/v1/federated/rounds", h.handleRounds)
	mux.HandleFunc("/api/v1/federated/predict", h.handlePredict)
	mux.HandleFunc("/api/v1/federated/stats", h.handleStats)
}

// handleNodes 处理节点列表
func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := NodeStatus(r.URL.Query().Get("status"))
		nodes := h.service.ListNodes(status)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": nodes,
			"total": len(nodes),
		})

	case http.MethodPost:
		var node FederatedNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		result, err := h.service.RegisterNode(r.Context(), &node)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNode 处理单个节点
func (h *Handler) handleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing node id", http.StatusBadRequest)
		return
	}

	node, err := h.service.GetNode(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// handleModels 处理模型列表
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		modelType := ModelType(r.URL.Query().Get("type"))
		models := h.service.ListModels(modelType)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": models,
			"total":  len(models),
		})

	case http.MethodPost:
		var model GlobalModel
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		result, err := h.service.CreateModel(r.Context(), &model)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleModel 处理单个模型
func (h *Handler) handleModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing model id", http.StatusBadRequest)
		return
	}

	model, err := h.service.GetModel(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

// handleTrain 处理训练请求
func (h *Handler) handleTrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelID      string  `json:"model_id"`
		Epochs       int     `json:"epochs"`
		BatchSize    int     `json:"batch_size"`
		LearningRate float64 `json:"learning_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Epochs == 0 {
		req.Epochs = 10
	}
	if req.BatchSize == 0 {
		req.BatchSize = 32
	}
	if req.LearningRate == 0 {
		req.LearningRate = 0.01
	}

	round, err := h.service.StartTraining(r.Context(), req.ModelID, req.Epochs, req.BatchSize, req.LearningRate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(round)
}

// handleRound 处理训练轮次请求
func (h *Handler) handleRound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing round id", http.StatusBadRequest)
		return
	}

	round, err := h.service.GetRound(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(round)
}

// handleRounds 处理训练轮次列表
func (h *Handler) handleRounds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("model_id")
	rounds := h.service.ListRounds(modelID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rounds": rounds,
		"total":  len(rounds),
	})
}

// handlePredict 处理预测请求
func (h *Handler) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelID string    `json:"model_id"`
		Input   []float64 `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.Predict(r.Context(), req.ModelID, req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleStats 处理统计请求
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.service.GetStatistics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
