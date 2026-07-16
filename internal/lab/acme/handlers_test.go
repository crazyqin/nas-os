package acme

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestHandler() *Handler {
	manager := NewManager()
	return NewHandler(manager)
}

func TestListCertificates(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/certificates", nil)
	w := httptest.NewRecorder()

	handler.handleCertificates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var certs []Certificate
	if err := json.NewDecoder(w.Body).Decode(&certs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(certs) < 3 {
		t.Errorf("expected at least 3 certificates, got %d", len(certs))
	}
}

func TestRequestCertificate(t *testing.T) {
	handler := setupTestHandler()

	reqBody := CreateCertRequest{
		Domain:    "new.example.com",
		AutoRenew: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleCertificates(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var cert Certificate
	if err := json.NewDecoder(w.Body).Decode(&cert); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cert.ID == "" {
		t.Error("expected certificate ID to be set")
	}
	if cert.Domain != "new.example.com" {
		t.Errorf("expected domain 'new.example.com', got '%s'", cert.Domain)
	}
	if cert.Issuer != "Let's Encrypt" {
		t.Errorf("expected issuer 'Let's Encrypt', got '%s'", cert.Issuer)
	}
	if cert.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", cert.Status)
	}
	if !cert.AutoRenew {
		t.Error("expected auto_renew to be true")
	}
}

func TestGetCertificate(t *testing.T) {
	handler := setupTestHandler()

	// First request a certificate
	reqBody := CreateCertRequest{
		Domain: "get-test.example.com",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleCertificates(createW, createReq)

	var createdCert Certificate
	json.NewDecoder(createW.Body).Decode(&createdCert)

	// Then get it
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/acme/certificates/"+createdCert.ID, nil)
	getW := httptest.NewRecorder()
	handler.handleCertificateByID(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, getW.Code)
	}

	var cert Certificate
	if err := json.NewDecoder(getW.Body).Decode(&cert); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cert.ID != createdCert.ID {
		t.Errorf("expected certificate ID '%s', got '%s'", createdCert.ID, cert.ID)
	}
}

func TestRenewCertificate(t *testing.T) {
	handler := setupTestHandler()

	// First request a certificate
	reqBody := CreateCertRequest{
		Domain: "renew-test.example.com",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleCertificates(createW, createReq)

	var createdCert Certificate
	json.NewDecoder(createW.Body).Decode(&createdCert)

	// Then renew it
	renewReq := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates/"+createdCert.ID+"/renew", nil)
	renewW := httptest.NewRecorder()
	handler.handleCertificateByID(renewW, renewReq)

	if renewW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, renewW.Code)
	}

	var cert Certificate
	if err := json.NewDecoder(renewW.Body).Decode(&cert); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cert.NotAfter.Before(time.Now().Add(89 * 24 * time.Hour)) {
		t.Error("expected certificate to be renewed (NotAfter should be ~90 days from now)")
	}
	if cert.RenewedAt.IsZero() {
		t.Error("expected renewed_at to be set")
	}
}

func TestRevokeCertificate(t *testing.T) {
	handler := setupTestHandler()

	// First request a certificate
	reqBody := CreateCertRequest{
		Domain: "revoke-test.example.com",
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	createW := httptest.NewRecorder()
	handler.handleCertificates(createW, createReq)

	var createdCert Certificate
	json.NewDecoder(createW.Body).Decode(&createdCert)

	// Then revoke it
	revokeReq := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates/"+createdCert.ID+"/revoke", nil)
	revokeW := httptest.NewRecorder()
	handler.handleCertificateByID(revokeW, revokeReq)

	if revokeW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, revokeW.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(revokeW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestCheckExpiry(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/check-expiry", nil)
	w := httptest.NewRecorder()

	handler.handleCheckExpiry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var expiring []Certificate
	if err := json.NewDecoder(w.Body).Decode(&expiring); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// api.example.com has 25 days until expiry, should be in the list
	found := false
	for _, c := range expiring {
		if c.Domain == "api.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected api.example.com to be in expiring list (25 days until expiry)")
	}
}

func TestGetConfig(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/config", nil)
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var config ACMEConfig
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if config.Email != "admin@example.com" {
		t.Errorf("expected email 'admin@example.com', got '%s'", config.Email)
	}
	if config.CAProvider != "letsencrypt" {
		t.Errorf("expected CA provider 'letsencrypt', got '%s'", config.CAProvider)
	}
}

func TestUpdateConfig(t *testing.T) {
	handler := setupTestHandler()

	newConfig := ACMEConfig{
		Email:       "new@example.com",
		CAProvider:  "zerossl",
		DNSProvider: "route53",
		RenewalDays: 14,
	}
	body, _ := json.Marshal(newConfig)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/acme/config", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var config ACMEConfig
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if config.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got '%s'", config.Email)
	}
	if config.CAProvider != "zerossl" {
		t.Errorf("expected CA provider 'zerossl', got '%s'", config.CAProvider)
	}
}

func TestGetStats(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/acme/stats", nil)
	w := httptest.NewRecorder()

	handler.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var stats CertStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.TotalCerts < 3 {
		t.Errorf("expected at least 3 total certificates, got %d", stats.TotalCerts)
	}
	if stats.ActiveCerts < 3 {
		t.Errorf("expected at least 3 active certificates, got %d", stats.ActiveCerts)
	}
	if stats.ExpiringSoon < 1 {
		t.Errorf("expected at least 1 certificate expiring soon, got %d", stats.ExpiringSoon)
	}
}

func TestConfigureDNS(t *testing.T) {
	handler := setupTestHandler()

	reqBody := struct {
		Provider    string `json:"provider"`
		Credentials string `json:"credentials"`
	}{
		Provider:    "cloudflare",
		Credentials: "test-api-key",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/acme/dns", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleDNS(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequestCertificateDuplicate(t *testing.T) {
	handler := setupTestHandler()

	// app.example.com already exists in mock data
	reqBody := CreateCertRequest{
		Domain: "app.example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleCertificates(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestRequestCertificateValidation(t *testing.T) {
	handler := setupTestHandler()

	// Missing domain
	reqBody := CreateCertRequest{
		AutoRenew: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/acme/certificates", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleCertificates(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing domain, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAutoRenew(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/acme/auto-renew", nil)
	w := httptest.NewRecorder()

	handler.handleAutoRenew(w, req)

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

func TestMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/acme/certificates", nil)
	w := httptest.NewRecorder()

	handler.handleCertificates(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
