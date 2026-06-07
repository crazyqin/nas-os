package wireguard

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

func TestGetInterface(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/interface", nil)
	w := httptest.NewRecorder()

	handler.handleInterface(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var iface WireGuardInterface
	if err := json.NewDecoder(w.Body).Decode(&iface); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if iface.Name != "wg0" {
		t.Errorf("expected interface name 'wg0', got '%s'", iface.Name)
	}
	if iface.ListenPort != 51820 {
		t.Errorf("expected listen port 51820, got %d", iface.ListenPort)
	}
}

func TestUpdateInterface(t *testing.T) {
	handler := setupTestHandler()

	newPort := 51821
	reqBody := InterfaceConfigRequest{
		ListenPort: &newPort,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/wireguard/interface", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleInterface(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var iface WireGuardInterface
	if err := json.NewDecoder(w.Body).Decode(&iface); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if iface.ListenPort != 51821 {
		t.Errorf("expected listen port 51821, got %d", iface.ListenPort)
	}
}

func TestListPeers(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers", nil)
	w := httptest.NewRecorder()

	handler.handlePeers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var peers []WireGuardPeer
	if err := json.NewDecoder(w.Body).Decode(&peers); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(peers) < 3 {
		t.Errorf("expected at least 3 peers, got %d", len(peers))
	}
}

func TestCreatePeer(t *testing.T) {
	handler := setupTestHandler()

	reqBody := CreatePeerRequest{
		PublicKey:           "test-public-key-123",
		AllowedIPs:          "10.0.0.10/32",
		PersistentKeepalive: 25,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handlePeers(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var peer WireGuardPeer
	if err := json.NewDecoder(w.Body).Decode(&peer); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if peer.ID == "" {
		t.Error("expected peer ID to be set")
	}
	if peer.PublicKey != "test-public-key-123" {
		t.Errorf("expected public key 'test-public-key-123', got '%s'", peer.PublicKey)
	}
}

func TestGetPeer(t *testing.T) {
	handler := setupTestHandler()

	// First create a peer
	reqBody := CreatePeerRequest{
		PublicKey:  "test-get-key",
		AllowedIPs: "10.0.0.11/32",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handlePeers(createW, createReq)

	var createdPeer WireGuardPeer
	json.NewDecoder(createW.Body).Decode(&createdPeer)

	// Then get it
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers/"+createdPeer.ID, nil)
	getW := httptest.NewRecorder()
	handler.handlePeerByID(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, getW.Code)
	}

	var peer WireGuardPeer
	if err := json.NewDecoder(getW.Body).Decode(&peer); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if peer.ID != createdPeer.ID {
		t.Errorf("expected peer ID '%s', got '%s'", createdPeer.ID, peer.ID)
	}
}

func TestUpdatePeer(t *testing.T) {
	handler := setupTestHandler()

	// First create a peer
	reqBody := CreatePeerRequest{
		PublicKey:  "test-update-key",
		AllowedIPs: "10.0.0.12/32",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handlePeers(createW, createReq)

	var createdPeer WireGuardPeer
	json.NewDecoder(createW.Body).Decode(&createdPeer)

	// Then update it
	newIPs := "10.0.0.12/32,10.0.0.13/32"
	updateBody := UpdatePeerRequest{
		AllowedIPs: &newIPs,
	}
	updateBytes, _ := json.Marshal(updateBody)

	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/wireguard/peers/"+createdPeer.ID, bytes.NewReader(updateBytes))
	updateW := httptest.NewRecorder()
	handler.handlePeerByID(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, updateW.Code)
	}

	var peer WireGuardPeer
	if err := json.NewDecoder(updateW.Body).Decode(&peer); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if peer.AllowedIPs != newIPs {
		t.Errorf("expected allowed IPs '%s', got '%s'", newIPs, peer.AllowedIPs)
	}
}

func TestDeletePeer(t *testing.T) {
	handler := setupTestHandler()

	// First create a peer
	reqBody := CreatePeerRequest{
		PublicKey:  "test-delete-key",
		AllowedIPs: "10.0.0.14/32",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handlePeers(createW, createReq)

	var createdPeer WireGuardPeer
	json.NewDecoder(createW.Body).Decode(&createdPeer)

	// Then delete it
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/wireguard/peers/"+createdPeer.ID, nil)
	deleteW := httptest.NewRecorder()
	handler.handlePeerByID(deleteW, deleteReq)

	if deleteW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, deleteW.Code)
	}

	// Verify it's gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers/"+createdPeer.ID, nil)
	getW := httptest.NewRecorder()
	handler.handlePeerByID(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected status %d after delete, got %d", http.StatusNotFound, getW.Code)
	}
}

func TestGetStats(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/stats", nil)
	w := httptest.NewRecorder()

	handler.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var stats WireGuardStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.TotalPeers < 3 {
		t.Errorf("expected at least 3 total peers, got %d", stats.TotalPeers)
	}
}

func TestGenerateKeyPair(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/generate-keypair", nil)
	w := httptest.NewRecorder()

	handler.handleGenerateKeyPair(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var keyPair KeyPairResponse
	if err := json.NewDecoder(w.Body).Decode(&keyPair); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if keyPair.PublicKey == "" {
		t.Error("expected public key to be set")
	}
	if keyPair.PrivateKey == "" {
		t.Error("expected private key to be set")
	}
}

func TestGetPeerConfig(t *testing.T) {
	handler := setupTestHandler()

	// First create a peer
	reqBody := CreatePeerRequest{
		PublicKey:  "test-config-key",
		AllowedIPs: "10.0.0.15/32",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handlePeers(createW, createReq)

	var createdPeer WireGuardPeer
	json.NewDecoder(createW.Body).Decode(&createdPeer)

	// Then get config
	configReq := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers/"+createdPeer.ID+"/config", nil)
	configW := httptest.NewRecorder()
	handler.handlePeerByID(configW, configReq)

	if configW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, configW.Code)
	}

	var configResp PeerConfigResponse
	if err := json.NewDecoder(configW.Body).Decode(&configResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if configResp.Config == "" {
		t.Error("expected config to be set")
	}
	if !bytes.Contains([]byte(configResp.Config), []byte("[Interface]")) {
		t.Error("expected config to contain [Interface] section")
	}
	if !bytes.Contains([]byte(configResp.Config), []byte("[Peer]")) {
		t.Error("expected config to contain [Peer] section")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/interface", nil)
	w := httptest.NewRecorder()

	handler.handleInterface(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestCreatePeerValidation(t *testing.T) {
	handler := setupTestHandler()

	// Missing public key
	reqBody := CreatePeerRequest{
		AllowedIPs: "10.0.0.20/32",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handlePeers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing public key, got %d", http.StatusBadRequest, w.Code)
	}
}
