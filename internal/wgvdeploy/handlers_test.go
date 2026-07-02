// Package wgvdeploy 提供 WireGuard 一键部署 API 测试
package wgvdeploy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Engine) {
	gin.SetMode(gin.TestMode)
	engine := NewEngine()
	handlers := NewHandlers(engine)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return r, engine
}

// TestGetStatus 测试获取服务状态.
func TestGetStatus(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Errorf("期望响应码 %d，实际 %d", http.StatusOK, resp.Code)
	}
}

// TestStartStopService 测试启动和停止服务.
func TestStartStopService(t *testing.T) {
	r, _ := setupTestRouter()

	// 启动服务
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/wgvdeploy/start", nil)
	startW := httptest.NewRecorder()
	r.ServeHTTP(startW, startReq)

	if startW.Code != http.StatusOK {
		t.Errorf("启动服务失败，期望状态码 %d，实际 %d", http.StatusOK, startW.Code)
	}

	// 重复启动应该返回冲突
	startW2 := httptest.NewRecorder()
	r.ServeHTTP(startW2, startReq)

	if startW2.Code != http.StatusConflict {
		t.Errorf("重复启动应返回冲突，期望 %d，实际 %d", http.StatusConflict, startW2.Code)
	}

	// 停止服务
	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/wgvdeploy/stop", nil)
	stopW := httptest.NewRecorder()
	r.ServeHTTP(stopW, stopReq)

	if stopW.Code != http.StatusOK {
		t.Errorf("停止服务失败，期望状态码 %d，实际 %d", http.StatusOK, stopW.Code)
	}
}

// TestAddPeer 测试添加对端.
func TestAddPeer(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreatePeerRequest{
		Name: "test-phone",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wgvdeploy/peers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("添加对端失败，期望状态码 %d，实际 %d", http.StatusCreated, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证对端数据
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["name"] != "test-phone" {
		t.Errorf("期望对端名称 'test-phone'，实际 '%v'", data["name"])
	}
	if data["id"] == nil || data["id"] == "" {
		t.Error("对端 ID 不应为空")
	}
	if data["public_key"] == nil || data["public_key"] == "" {
		t.Error("公钥不应为空")
	}
	if data["assigned_ipv4"] == nil || data["assigned_ipv4"] == "" {
		t.Error("IPv4 地址不应为空")
	}
}

// TestListPeers 测试获取对端列表.
func TestListPeers(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加几个对端
	engine.AddPeer(CreatePeerRequest{Name: "peer1"})
	engine.AddPeer(CreatePeerRequest{Name: "peer2"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/peers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取对端列表失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if len(data) != 2 {
		t.Errorf("期望 2 个对端，实际 %d", len(data))
	}
}

// TestDeletePeer 测试删除对端.
func TestDeletePeer(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加对端
	peer, _ := engine.AddPeer(CreatePeerRequest{Name: "delete-me"})

	// 删除对端
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/wgvdeploy/peers/"+peer.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("删除对端失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 验证已删除
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/peers/"+peer.ID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("删除后查询应返回 404，实际 %d", getW.Code)
	}
}

// TestUpdatePeer 测试更新对端.
func TestUpdatePeer(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加对端
	peer, _ := engine.AddPeer(CreatePeerRequest{Name: "update-me"})

	// 更新对端
	newName := "updated-name"
	reqBody := UpdatePeerRequest{
		Name: &newName,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/wgvdeploy/peers/"+peer.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("更新对端失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["name"] != "updated-name" {
		t.Errorf("期望名称 'updated-name'，实际 '%v'", data["name"])
	}
}

// TestGetPeerConfig 测试获取客户端配置.
func TestGetPeerConfig(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加对端
	peer, _ := engine.AddPeer(CreatePeerRequest{Name: "config-test"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/peers/"+peer.ID+"/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取客户端配置失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	config, ok := data["config"].(string)
	if !ok || config == "" {
		t.Error("配置内容不应为空")
	}

	// 验证配置内容包含关键部分
	if !contains(config, "[Interface]") {
		t.Error("配置应包含 [Interface] 部分")
	}
	if !contains(config, "[Peer]") {
		t.Error("配置应包含 [Peer] 部分")
	}
}

// TestGetPeerQRCode 测试获取 QR 码.
func TestGetPeerQRCode(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加对端
	peer, _ := engine.AddPeer(CreatePeerRequest{Name: "qr-test"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/peers/"+peer.ID+"/qrcode?format=svg", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取 QR 码失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["base64"] == nil || data["base64"] == "" {
		t.Error("QR 码 Base64 数据不应为空")
	}
	if data["format"] != "svg" {
		t.Errorf("期望格式 'svg'，实际 '%v'", data["format"])
	}
}

// TestGetTrafficStats 测试获取流量统计.
func TestGetTrafficStats(t *testing.T) {
	r, engine := setupTestRouter()

	// 先添加对端
	engine.AddPeer(CreatePeerRequest{Name: "traffic-peer1"})
	engine.AddPeer(CreatePeerRequest{Name: "traffic-peer2"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/traffic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取流量统计失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	// 验证统计字段存在
	if data["total_peers"] == nil {
		t.Error("总对端数字段不应为空")
	}
	if data["peer_stats"] == nil {
		t.Error("对端统计字段不应为空")
	}
}

// TestGetTrafficHistory 测试获取历史流量.
func TestGetTrafficHistory(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/traffic/history?interval=hour", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取历史流量失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["interval"] != "hour" {
		t.Errorf("期望间隔 'hour'，实际 '%v'", data["interval"])
	}

	dataPoints, ok := data["data_points"].([]interface{})
	if !ok {
		t.Fatal("数据点格式错误")
	}

	if len(dataPoints) != 24 {
		t.Errorf("期望 24 个数据点，实际 %d", len(dataPoints))
	}
}

// TestGetTemplates 测试获取配置模板.
func TestGetTemplates(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取模板失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	templates, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("模板数据格式错误")
	}

	if len(templates) != 4 {
		t.Errorf("期望 4 个模板，实际 %d", len(templates))
	}
}

// TestDeploy 测试一键部署.
func TestDeploy(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := DeployRequest{
		Template:      "home-access",
		ServerAddress: "vpn.example.com",
		ListenPort:    51820,
		Network:       "10.10.0.0/24",
		DNS:           "1.1.1.1",
		ClientCount:   3,
		EnableNAT:     true,
		EnableDNS:     true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wgvdeploy/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("部署失败，期望状态码 %d，实际 %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["success"] != true {
		t.Error("部署应该成功")
	}

	clients, ok := data["clients"].([]interface{})
	if !ok {
		t.Fatal("客户端数据格式错误")
	}

	if len(clients) != 3 {
		t.Errorf("期望 3 个客户端，实际 %d", len(clients))
	}
}

// TestGetServerConfig 测试获取服务端配置.
func TestGetServerConfig(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wgvdeploy/server/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取服务端配置失败，期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if data["interface_name"] != "wg0" {
		t.Errorf("期望接口名 'wg0'，实际 '%v'", data["interface_name"])
	}
	if data["listen_port"] == nil {
		t.Error("监听端口字段不应为空")
	}
	if data["public_key"] == nil || data["public_key"] == "" {
		t.Error("公钥字段不应为空")
	}
}

// TestIPv6Allocation 测试 IPv6 地址分配.
func TestIPv6Allocation(t *testing.T) {
	engine := NewEngine()

	peer1, err := engine.AddPeer(CreatePeerRequest{Name: "ipv6-test1"})
	if err != nil {
		t.Fatalf("添加对端失败: %v", err)
	}

	peer2, err := engine.AddPeer(CreatePeerRequest{Name: "ipv6-test2"})
	if err != nil {
		t.Fatalf("添加对端失败: %v", err)
	}

	if peer1.AssignedIPv6 == "" {
		t.Error("对端 1 的 IPv6 地址不应为空")
	}
	if peer2.AssignedIPv6 == "" {
		t.Error("对端 2 的 IPv6 地址不应为空")
	}
	if peer1.AssignedIPv6 == peer2.AssignedIPv6 {
		t.Error("两个对端的 IPv6 地址不应相同")
	}
}

// TestKeyPairGeneration 测试密钥对生成.
func TestKeyPairGeneration(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	if kp1.PrivateKey == "" || kp1.PublicKey == "" {
		t.Error("密钥对不应包含空值")
	}
	if kp1.PrivateKey == kp2.PrivateKey {
		t.Error("两次生成的私钥不应相同")
	}
	if kp1.PublicKey == kp2.PublicKey {
		t.Error("两次生成的公钥不应相同")
	}
}

// TestGeneratePresharedKey 测试预共享密钥生成.
func TestGeneratePresharedKey(t *testing.T) {
	psk1, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("生成预共享密钥失败: %v", err)
	}

	psk2, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("生成预共享密钥失败: %v", err)
	}

	if psk1 == "" || psk2 == "" {
		t.Error("预共享密钥不应为空")
	}
	if psk1 == psk2 {
		t.Error("两次生成的预共享密钥不应相同")
	}
}

// TestDisablePeer 测试禁用对端.
func TestDisablePeer(t *testing.T) {
	engine := NewEngine()

	peer, err := engine.AddPeer(CreatePeerRequest{Name: "disable-test"})
	if err != nil {
		t.Fatalf("添加对端失败: %v", err)
	}

	if !peer.Enabled {
		t.Error("新添加的对端应该默认启用")
	}

	if err := engine.DisablePeer(peer.ID); err != nil {
		t.Fatalf("禁用对端失败: %v", err)
	}

	updatedPeer, _ := engine.GetPeer(peer.ID)
	if updatedPeer.Enabled {
		t.Error("对端应该已被禁用")
	}
}

// TestServerConfGeneration 测试服务端配置生成.
func TestServerConfGeneration(t *testing.T) {
	engine := NewEngine()

	// 添加对端
	engine.AddPeer(CreatePeerRequest{Name: "conf-peer1"})
	engine.AddPeer(CreatePeerRequest{Name: "conf-peer2"})

	conf := engine.GenerateServerConf()

	if !contains(conf, "[Interface]") {
		t.Error("配置应包含 [Interface] 部分")
	}
	if !contains(conf, "[Peer]") {
		t.Error("配置应包含 [Peer] 部分")
	}
	if !contains(conf, "ListenPort") {
		t.Error("配置应包含 ListenPort")
	}
	if !contains(conf, "PrivateKey") {
		t.Error("配置应包含 PrivateKey")
	}
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
