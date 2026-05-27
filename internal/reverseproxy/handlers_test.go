package reverseproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandler() *Handler {
	manager := NewManager()
	return NewHandler(manager)
}

func TestListProxies(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reverse-proxy/proxies", nil)
	w := httptest.NewRecorder()

	handler.handleProxies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var proxies []ReverseProxy
	if err := json.NewDecoder(w.Body).Decode(&proxies); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(proxies) < 3 {
		t.Errorf("expected at least 3 proxies, got %d", len(proxies))
	}
}

func TestCreateProxy(t *testing.T) {
	handler := setupTestHandler()

	reqBody := CreateProxyRequest{
		Name:       "test-proxy",
		Domain:     "test.example.com",
		TargetURL:  "http://localhost:4000",
		SSLEnabled: true,
		Headers:    map[string]string{"X-Custom": "value"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleProxies(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var proxy ReverseProxy
	if err := json.NewDecoder(w.Body).Decode(&proxy); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if proxy.ID == "" {
		t.Error("expected proxy ID to be set")
	}
	if proxy.Name != "test-proxy" {
		t.Errorf("expected name 'test-proxy', got '%s'", proxy.Name)
	}
	if proxy.Domain != "test.example.com" {
		t.Errorf("expected domain 'test.example.com', got '%s'", proxy.Domain)
	}
	if !proxy.SSLEnabled {
		t.Error("expected SSL to be enabled")
	}
	if proxy.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", proxy.Status)
	}
}

func TestGetProxy(t *testing.T) {
	handler := setupTestHandler()

	// First create a proxy
	reqBody := CreateProxyRequest{
		Name:      "get-test",
		Domain:    "get.example.com",
		TargetURL: "http://localhost:5000",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleProxies(createW, createReq)

	var createdProxy ReverseProxy
	json.NewDecoder(createW.Body).Decode(&createdProxy)

	// Then get it
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID, nil)
	getW := httptest.NewRecorder()
	handler.handleProxyByID(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, getW.Code)
	}

	var proxy ReverseProxy
	if err := json.NewDecoder(getW.Body).Decode(&proxy); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if proxy.ID != createdProxy.ID {
		t.Errorf("expected proxy ID '%s', got '%s'", createdProxy.ID, proxy.ID)
	}
}

func TestUpdateProxy(t *testing.T) {
	handler := setupTestHandler()

	// First create a proxy
	reqBody := CreateProxyRequest{
		Name:      "update-test",
		Domain:    "update.example.com",
		TargetURL: "http://localhost:6000",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleProxies(createW, createReq)

	var createdProxy ReverseProxy
	json.NewDecoder(createW.Body).Decode(&createdProxy)

	// Then update it
	newTarget := "http://localhost:7000"
	newName := "updated-proxy"
	updateBody := UpdateProxyRequest{
		Name:      &newName,
		TargetURL: &newTarget,
	}
	updateBytes, _ := json.Marshal(updateBody)

	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID, bytes.NewReader(updateBytes))
	updateW := httptest.NewRecorder()
	handler.handleProxyByID(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, updateW.Code)
	}

	var proxy ReverseProxy
	if err := json.NewDecoder(updateW.Body).Decode(&proxy); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if proxy.Name != "updated-proxy" {
		t.Errorf("expected name 'updated-proxy', got '%s'", proxy.Name)
	}
	if proxy.TargetURL != "http://localhost:7000" {
		t.Errorf("expected target URL 'http://localhost:7000', got '%s'", proxy.TargetURL)
	}
}

func TestDeleteProxy(t *testing.T) {
	handler := setupTestHandler()

	// First create a proxy
	reqBody := CreateProxyRequest{
		Name:      "delete-test",
		Domain:    "delete.example.com",
		TargetURL: "http://localhost:8000",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleProxies(createW, createReq)

	var createdProxy ReverseProxy
	json.NewDecoder(createW.Body).Decode(&createdProxy)

	// Then delete it
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID, nil)
	deleteW := httptest.NewRecorder()
	handler.handleProxyByID(deleteW, deleteReq)

	if deleteW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, deleteW.Code)
	}

	// Verify it's gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID, nil)
	getW := httptest.NewRecorder()
	handler.handleProxyByID(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected status %d after delete, got %d", http.StatusNotFound, getW.Code)
	}
}

func TestGetStats(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reverse-proxy/stats", nil)
	w := httptest.NewRecorder()

	handler.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var stats ProxyStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.TotalProxies < 3 {
		t.Errorf("expected at least 3 total proxies, got %d", stats.TotalProxies)
	}
	if stats.ActiveProxies < 3 {
		t.Errorf("expected at least 3 active proxies, got %d", stats.ActiveProxies)
	}
}

func TestAddRule(t *testing.T) {
	handler := setupTestHandler()

	// First create a proxy
	reqBody := CreateProxyRequest{
		Name:      "rule-test",
		Domain:    "rule.example.com",
		TargetURL: "http://localhost:9000",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleProxies(createW, createReq)

	var createdProxy ReverseProxy
	json.NewDecoder(createW.Body).Decode(&createdProxy)

	// Then add a rule
	ruleBody := AddRuleRequest{
		Path:          "/api/v1",
		TargetURL:     "http://localhost:9001",
		LoadBalancing: "round-robin",
		RateLimit:     100,
	}
	ruleBytes, _ := json.Marshal(ruleBody)

	ruleReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID+"/rules", bytes.NewReader(ruleBytes))
	ruleW := httptest.NewRecorder()
	handler.handleProxyByID(ruleW, ruleReq)

	if ruleW.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, ruleW.Code)
	}

	var rule ProxyRule
	if err := json.NewDecoder(ruleW.Body).Decode(&rule); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if rule.Path != "/api/v1" {
		t.Errorf("expected path '/api/v1', got '%s'", rule.Path)
	}
	if rule.LoadBalancing != "round-robin" {
		t.Errorf("expected load balancing 'round-robin', got '%s'", rule.LoadBalancing)
	}
}

func TestGetRules(t *testing.T) {
	handler := setupTestHandler()

	// First create a proxy
	reqBody := CreateProxyRequest{
		Name:      "rules-list-test",
		Domain:    "rules-list.example.com",
		TargetURL: "http://localhost:10000",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleProxies(createW, createReq)

	var createdProxy ReverseProxy
	json.NewDecoder(createW.Body).Decode(&createdProxy)

	// Add two rules
	for i, path := range []string{"/api", "/web"} {
		ruleBody := AddRuleRequest{
			Path:      path,
			TargetURL: "http://localhost:1000" + string(rune('0'+i)),
		}
		ruleBytes, _ := json.Marshal(ruleBody)
		ruleReq := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID+"/rules", bytes.NewReader(ruleBytes))
		ruleW := httptest.NewRecorder()
		handler.handleProxyByID(ruleW, ruleReq)
	}

	// Get rules
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reverse-proxy/proxies/"+createdProxy.ID+"/rules", nil)
	getW := httptest.NewRecorder()
	handler.handleProxyByID(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, getW.Code)
	}

	var rules []ProxyRule
	if err := json.NewDecoder(getW.Body).Decode(&rules); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestCreateProxyDuplicateDomain(t *testing.T) {
	handler := setupTestHandler()

	reqBody := CreateProxyRequest{
		Name:      "dup-test",
		Domain:    "app.example.com", // Already exists in mock data
		TargetURL: "http://localhost:11000",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleProxies(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestCreateProxyValidation(t *testing.T) {
	handler := setupTestHandler()

	// Missing name
	reqBody := CreateProxyRequest{
		Domain:    "validation.example.com",
		TargetURL: "http://localhost:12000",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/proxies", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleProxies(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing name, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/reverse-proxy/proxies", nil)
	w := httptest.NewRecorder()

	handler.handleProxies(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestReloadConfig(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reverse-proxy/reload", nil)
	w := httptest.NewRecorder()

	handler.handleReload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}
