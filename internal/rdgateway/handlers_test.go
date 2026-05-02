// Package rdgateway HTTP handler 测试
package rdgateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandlers() *Handlers {
	sm := NewSessionManager(nil)
	tunnel := &WSTunnel{tunnels: make(map[string]*Tunnel)}
	cs := NewClipboardSync(100)
	ft := NewFileTransfer()
	return NewHandlers(sm, tunnel, cs, ft)
}

func TestHandlersCreateSession(t *testing.T) {
	h := setupTestHandlers()
	body := `{"user_id":"u1","protocol":"rdp","host":"192.168.1.100"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleSessions(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为201: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 0 {
		t.Errorf("响应码应为0: %d", resp.Code)
	}
}

func TestHandlersListSessions(t *testing.T) {
	h := setupTestHandlers()

	// 创建会话
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	// 列出会话
	req = httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/sessions", nil)
	w = httptest.NewRecorder()
	h.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if int(data["total"].(float64)) != 1 {
		t.Errorf("应有1个会话: %v", data["total"])
	}
}

func TestHandlersGetSession(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	sessionData := createResp.Data.(map[string]interface{})
	sessionID := sessionData["id"].(string)

	// 获取
	req = httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersGetSessionNotFound(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("状态码应为404: %d", w.Code)
	}
}

func TestHandlersDeleteSession(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	sessionData := createResp.Data.(map[string]interface{})
	sessionID := sessionData["id"].(string)

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/rdgateway/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersDisconnectAndReconnect(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	sessionID := createResp.Data.(map[string]interface{})["id"].(string)

	// 断开
	req = httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions/"+sessionID+"/disconnect", nil)
	w = httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("断开状态码应为200: %d", w.Code)
	}

	// 重连
	req = httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions/"+sessionID+"/reconnect", nil)
	w = httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("重连状态码应为200: %d", w.Code)
	}
}

func TestHandlersClipboard(t *testing.T) {
	h := setupTestHandlers()

	// 创建会话
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	sessionID := createResp.Data.(map[string]interface{})["id"].(string)

	// 更新剪贴板
	clipBody := `{"format":"text","content":"hello"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/rdgateway/clipboard/"+sessionID, bytes.NewBufferString(clipBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("更新剪贴板状态码应为200: %d", w.Code)
	}

	// 获取剪贴板
	req = httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/clipboard/"+sessionID, nil)
	w = httptest.NewRecorder()
	h.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取剪贴板状态码应为200: %d", w.Code)
	}
}

func TestHandlersClipboardNotFound(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/clipboard/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleClipboard(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("状态码应为404: %d", w.Code)
	}
}

func TestHandlersClipboardClear(t *testing.T) {
	h := setupTestHandlers()

	// 创建会话
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	sessionID := createResp.Data.(map[string]interface{})["id"].(string)

	// 清除剪贴板
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/rdgateway/clipboard/"+sessionID, nil)
	w = httptest.NewRecorder()
	h.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("清除剪贴板状态码应为200: %d", w.Code)
	}
}

func TestHandlersTransfers(t *testing.T) {
	h := setupTestHandlers()

	// 列出传输（空）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/transfers", nil)
	w := httptest.NewRecorder()
	h.handleTransfers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersAudit(t *testing.T) {
	h := setupTestHandlers()

	// 创建会话
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	// 获取审计日志
	req = httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/audit", nil)
	w = httptest.NewRecorder()
	h.handleAudit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if int(data["total"].(float64)) < 1 {
		t.Errorf("应至少有1条审计记录: %v", data["total"])
	}
}

func TestHandlersAuditWithLimit(t *testing.T) {
	h := setupTestHandlers()

	// 创建会话
	body := `{"user_id":"u1","protocol":"rdp","host":"host1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/audit?limit=1", nil)
	w = httptest.NewRecorder()
	h.handleAudit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersMethodNotAllowed(t *testing.T) {
	h := setupTestHandlers()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rdgateway/sessions", nil)
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码应为405: %d", w.Code)
	}
}

func TestHandlersTestConnectionNoHost(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/test", nil)
	w := httptest.NewRecorder()
	h.handleTestConnection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为400: %d", w.Code)
	}
}

func TestHandlersTransferNotFound(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/transfers/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleTransferByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("状态码应为404: %d", w.Code)
	}
}

func TestHandlersRegisterRoutes(t *testing.T) {
	h := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 验证路由注册成功，不 panic
	if mux == nil {
		t.Error("mux不应为nil")
	}
}

func TestHandlersInvalidJSON(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdgateway/sessions", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("无效JSON应返回400: %d", w.Code)
	}
}

func TestHandlersSessionByIDEmpty(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdgateway/sessions/", nil)
	w := httptest.NewRecorder()
	h.handleSessionByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("空ID应返回404: %d", w.Code)
	}
}
