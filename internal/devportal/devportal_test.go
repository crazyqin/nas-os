package devportal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	mgr := NewManager(filepath.Join(tmpDir, "data.json"))
	require.NoError(t, mgr.Initialize())
	r := gin.New()
	grp := r.Group("")
	NewHandlers(mgr).RegisterRoutes(grp)
	return mgr, r
}

func TestCreateAndListAPIKeys(t *testing.T) {
	_, r := setupTest(t)

	reqBody := map[string]interface{}{
		"name":        "test-key",
		"owner_id":    "user-1",
		"scopes":      []string{"read", "write"},
		"rate_limit":  100,
		"daily_quota": 5000,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/devportal/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["key"])
	assert.NotEmpty(t, data["secret"])
	assert.Equal(t, "active", data["status"])

	req2 := httptest.NewRequest(http.MethodGet, "/devportal/apikeys?owner_id=user-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, float64(1), resp2["total"])
}

func TestRevokeAPIKey(t *testing.T) {
	mgr, r := setupTest(t)

	key, err := mgr.CreateAPIKey("revoke-test", "user-1", []APIScope{ScopeRead}, 0, 0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/devportal/apikeys/"+key.ID+"/revoke", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	k, _ := mgr.GetAPIKey(key.ID)
	assert.Equal(t, KeyRevoked, k.Status)
}

func TestValidateAPIKey(t *testing.T) {
	mgr, _ := setupTest(t)

	key, err := mgr.CreateAPIKey("validate-test", "user-1", []APIScope{ScopeRead}, 60, 100)
	require.NoError(t, err)

	validated, err := mgr.ValidateAPIKey(key.Key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), validated.TotalCalls)
	assert.Equal(t, 1, validated.UsedToday)
}

func TestValidateRevokedKey(t *testing.T) {
	mgr, _ := setupTest(t)

	key, err := mgr.CreateAPIKey("revoked-validate", "user-1", []APIScope{ScopeRead}, 0, 0)
	require.NoError(t, err)
	require.NoError(t, mgr.RevokeAPIKey(key.ID))

	_, err = mgr.ValidateAPIKey(key.Key)
	assert.ErrorIs(t, err, ErrAPIKeyRevoked)
}

func TestQuotaExceeded(t *testing.T) {
	mgr, _ := setupTest(t)

	key, err := mgr.CreateAPIKey("quota-test", "user-1", []APIScope{ScopeRead}, 60, 2)
	require.NoError(t, err)

	_, err = mgr.ValidateAPIKey(key.Key)
	require.NoError(t, err)
	_, err = mgr.ValidateAPIKey(key.Key)
	require.NoError(t, err)
	_, err = mgr.ValidateAPIKey(key.Key)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestWebhookLifecycle(t *testing.T) {
	_, r := setupTest(t)

	reqBody := map[string]interface{}{
		"name":     "test-webhook",
		"url":      "https://example.com/hook",
		"owner_id": "user-1",
		"events":   []string{"service.start", "service.stop"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/devportal/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])
	assert.NotEmpty(t, data["secret"])
	assert.Equal(t, "active", data["status"])

	webhookID := data["id"].(string)

	req2 := httptest.NewRequest(http.MethodGet, "/devportal/webhooks?owner_id=user-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, float64(1), resp2["total"])

	updateBody := map[string]interface{}{
		"name": "updated-webhook",
	}
	body2, _ := json.Marshal(updateBody)
	req3 := httptest.NewRequest(http.MethodPut, "/devportal/webhooks/"+webhookID, bytes.NewReader(body2))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	req4 := httptest.NewRequest(http.MethodDelete, "/devportal/webhooks/"+webhookID, nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
}

func TestDeveloperAppLifecycle(t *testing.T) {
	_, r := setupTest(t)

	reqBody := map[string]interface{}{
		"name":          "Test App",
		"owner_id":      "user-1",
		"description":   "A test application",
		"redirect_uris": []string{"https://example.com/callback"},
		"grant_types":   []string{"authorization_code"},
		"scopes":        []string{"read", "write"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/devportal/apps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["client_id"])
	assert.NotEmpty(t, data["client_secret"])

	appID := data["id"].(string)

	req2 := httptest.NewRequest(http.MethodGet, "/devportal/apps/"+appID, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodGet, "/devportal/apps?owner_id=user-1", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	var resp3 map[string]interface{}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &resp3))
	assert.Equal(t, float64(1), resp3["total"])

	req4 := httptest.NewRequest(http.MethodDelete, "/devportal/apps/"+appID, nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
}

func TestOAuthTokenFlow(t *testing.T) {
	mgr, r := setupTest(t)

	app, err := mgr.RegisterApp("oauth-test", "user-1", "test", []string{"https://example.com/cb"}, []OAuthGrantType{GrantAuthCode}, []APIScope{ScopeRead, ScopeWrite})
	require.NoError(t, err)

	reqBody := map[string]interface{}{
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"grant_type":    "authorization_code",
		"scopes":        []string{"read"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/devportal/oauth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"])
	assert.Equal(t, "Bearer", data["token_type"])

	token, err := mgr.ValidateToken(data["access_token"].(string))
	require.NoError(t, err)
	assert.Equal(t, app.ID, token.AppID)
}

func TestInvalidOAuthCredentials(t *testing.T) {
	_, r := setupTest(t)

	reqBody := map[string]interface{}{
		"client_id":     "invalid",
		"client_secret": "invalid",
		"grant_type":    "authorization_code",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/devportal/oauth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOpenAPISpec(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/devportal/openapi.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	assert.Equal(t, "3.0.3", spec["openapi"])
	assert.NotNil(t, spec["info"])
	assert.NotNil(t, spec["paths"])
}

func TestSDKGeneration(t *testing.T) {
	_, r := setupTest(t)

	for _, lang := range []string{"python", "go", "javascript"} {
		req := httptest.NewRequest(http.MethodGet, "/devportal/sdk/"+lang, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data := resp["data"].(map[string]interface{})
		assert.NotEmpty(t, data["code"])
		assert.Equal(t, lang, data["language"])
	}

	req := httptest.NewRequest(http.MethodGet, "/devportal/sdk/rust", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUsageStats(t *testing.T) {
	mgr, r := setupTest(t)

	mgr.RecordUsage("user-1", true, 50)
	mgr.RecordUsage("user-1", true, 30)
	mgr.RecordUsage("user-1", false, 200)

	req := httptest.NewRequest(http.MethodGet, "/devportal/usage/user-1?days=7", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
	record := data[0].(map[string]interface{})
	assert.Equal(t, float64(3), record["total"])
	assert.Equal(t, float64(2), record["success"])
	assert.Equal(t, float64(1), record["failed"])
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)

	_, err := mgr.CreateAPIKey("stat-key-1", "user-1", []APIScope{ScopeRead}, 0, 0)
	require.NoError(t, err)
	_, err = mgr.RegisterWebhook("stat-wh-1", "https://example.com/hook", "user-1", []WebhookEvent{EventServiceStart})
	require.NoError(t, err)
	_, err = mgr.RegisterApp("stat-app-1", "user-1", "test", nil, []OAuthGrantType{GrantAuthCode}, []APIScope{ScopeRead})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/devportal/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total_api_keys"])
	assert.Equal(t, float64(1), data["active_api_keys"])
	assert.Equal(t, float64(1), data["total_webhooks"])
	assert.Equal(t, float64(1), data["total_apps"])
}

func TestWebhookHMAC(t *testing.T) {
	mgr, _ := setupTest(t)

	payload := []byte(`{"event":"service.start"}`)
	secret := "test-secret-123"
	sig := mgr.computeHMAC(payload, secret)
	assert.NotEmpty(t, sig)
	assert.Contains(t, sig, "sha256=")

	sig2 := mgr.computeHMAC(payload, secret)
	assert.Equal(t, sig, sig2)

	sig3 := mgr.computeHMAC(payload, "different-secret")
	assert.NotEqual(t, sig, sig3)
}

func TestMaxAPIKeysPerOwner(t *testing.T) {
	mgr, _ := setupTest(t)

	mgr.config.Quota.MaxAPIKeys = 2

	_, err := mgr.CreateAPIKey("key-1", "user-limited", []APIScope{ScopeRead}, 0, 0)
	require.NoError(t, err)
	_, err = mgr.CreateAPIKey("key-2", "user-limited", []APIScope{ScopeRead}, 0, 0)
	require.NoError(t, err)
	_, err = mgr.CreateAPIKey("key-3", "user-limited", []APIScope{ScopeRead}, 0, 0)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestResetDailyUsage(t *testing.T) {
	mgr, _ := setupTest(t)

	key, err := mgr.CreateAPIKey("reset-test", "user-1", []APIScope{ScopeRead}, 60, 100)
	require.NoError(t, err)

	_, err = mgr.ValidateAPIKey(key.Key)
	require.NoError(t, err)
	assert.Equal(t, 1, key.UsedToday)

	mgr.ResetDailyUsage()
	k, _ := mgr.GetAPIKey(key.ID)
	assert.Equal(t, 0, k.UsedToday)
}
