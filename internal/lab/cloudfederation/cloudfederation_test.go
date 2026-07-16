package cloudfederation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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

func registerTestProvider(t *testing.T, r *gin.Engine, id string, provider CloudProvider) {
	t.Helper()
	cfg := CloudProviderConfig{
		ID:        id,
		Name:      "Test " + string(provider),
		Provider:  provider,
		Region:    "cn-north-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/cloudfederation/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterAndListProviders(t *testing.T) {
	_, r := setupTest(t)

	registerTestProvider(t, r, "aws-1", ProviderAWS)

	req := httptest.NewRequest(http.MethodGet, "/cloudfederation/providers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDuplicateProvider(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)

	err := mgr.RegisterProvider(&CloudProviderConfig{
		ID:       "aws-1",
		Name:     "Duplicate",
		Provider: ProviderAWS,
	})
	assert.ErrorIs(t, err, ErrProviderExists)
}

func TestProviderCRUD(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "ali-1", ProviderAliyun)

	// Get
	req := httptest.NewRequest(http.MethodGet, "/cloudfederation/providers/ali-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Update
	updateCfg := CloudProviderConfig{
		Name:      "Updated Aliyun",
		Provider:  ProviderAliyun,
		Region:    "cn-east-1",
		AccessKey: "new-key",
		SecretKey: "new-secret",
	}
	body, _ := json.Marshal(updateCfg)
	req = httptest.NewRequest(http.MethodPut, "/cloudfederation/providers/ali-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	provider, err := mgr.GetProvider("ali-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Aliyun", provider.Name)

	// Health Check
	req = httptest.NewRequest(http.MethodPost, "/cloudfederation/providers/ali-1/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/cloudfederation/providers/ali-1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateAndListNamespaces(t *testing.T) {
	_, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	registerTestProvider(t, r, "gcs-1", ProviderGCS)

	ns := Namespace{
		ID:          "ns-1",
		Name:        "Test Namespace",
		Description: "Test description",
		Providers:   []string{"aws-1", "gcs-1"},
		Strategy:    StrategyBalanced,
	}
	body, _ := json.Marshal(ns)
	req := httptest.NewRequest(http.MethodPost, "/cloudfederation/namespaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/cloudfederation/namespaces", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDuplicateNamespace(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)

	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1"},
		Strategy:  StrategyBalanced,
	}))

	err := mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Duplicate",
		Providers: []string{"aws-1"},
	})
	assert.ErrorIs(t, err, ErrNamespaceExists)
}

func TestObjectLifecycle(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1"},
		Strategy:  StrategyBalanced,
	}))

	// Place object
	obj := StorageObject{
		Key:  "file.txt",
		Size: 1024,
	}
	body, _ := json.Marshal(obj)
	req := httptest.NewRequest(http.MethodPost, "/cloudfederation/namespaces/ns-1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get object
	req2 := httptest.NewRequest(http.MethodGet, "/cloudfederation/namespaces/ns-1/objects/file.txt", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// List objects
	req3 := httptest.NewRequest(http.MethodGet, "/cloudfederation/namespaces/ns-1/objects", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])

	// Delete object
	req4 := httptest.NewRequest(http.MethodDelete, "/cloudfederation/namespaces/ns-1/objects/file.txt", nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
}

func TestSyncTask(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	registerTestProvider(t, r, "gcs-1", ProviderGCS)
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1", "gcs-1"},
		Strategy:  StrategyBalanced,
	}))

	task := SyncTask{
		ID:             "sync-1",
		Namespace:      "ns-1",
		SourceProvider: "aws-1",
		TargetProvider: "gcs-1",
		TotalObjects:   100,
		TotalBytes:     1024000,
	}
	body, _ := json.Marshal(task)
	req := httptest.NewRequest(http.MethodPost, "/cloudfederation/syncs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for async task
	time.Sleep(200 * time.Millisecond)

	// Get sync task
	req2 := httptest.NewRequest(http.MethodGet, "/cloudfederation/syncs/sync-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// List sync tasks
	req3 := httptest.NewRequest(http.MethodGet, "/cloudfederation/syncs", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestMigrationTask(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	registerTestProvider(t, r, "ali-1", ProviderAliyun)
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1", "ali-1"},
		Strategy:  StrategyBalanced,
	}))

	task := MigrationTask{
		ID:             "mig-1",
		Namespace:      "ns-1",
		SourceProvider: "aws-1",
		TargetProvider: "ali-1",
		TotalObjects:   50,
		TotalBytes:     512000,
		DeleteSource:   true,
	}
	body, _ := json.Marshal(task)
	req := httptest.NewRequest(http.MethodPost, "/cloudfederation/migrations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for async task
	time.Sleep(200 * time.Millisecond)

	// Get migration task
	req2 := httptest.NewRequest(http.MethodGet, "/cloudfederation/migrations/mig-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestCancelMigration(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.RegisterProvider(&CloudProviderConfig{
		ID:        "aws-1",
		Name:      "AWS",
		Provider:  ProviderAWS,
		AccessKey: "k",
		SecretKey: "s",
	}))
	require.NoError(t, mgr.RegisterProvider(&CloudProviderConfig{
		ID:        "gcs-1",
		Name:      "GCS",
		Provider:  ProviderGCS,
		AccessKey: "k",
		SecretKey: "s",
	}))
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1", "gcs-1"},
		Strategy:  StrategyBalanced,
	}))

	err := mgr.CreateMigrationTask(&MigrationTask{
		ID:             "mig-1",
		Namespace:      "ns-1",
		SourceProvider: "aws-1",
		TargetProvider: "gcs-1",
		TotalObjects:   10,
		TotalBytes:     1024,
	})
	require.NoError(t, err)

	// Cancel
	err = mgr.CancelMigrationTask("mig-1")
	assert.NoError(t, err)

	task, err := mgr.GetMigrationTask("mig-1")
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusCancelled, task.Status)
}

func TestCostAnalysis(t *testing.T) {
	_, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)

	req := httptest.NewRequest(http.MethodGet, "/cloudfederation/costs?period=monthly", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.True(t, data["total_cost"].(float64) > 0)
}

func TestFederationStats(t *testing.T) {
	mgr, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1"},
		Strategy:  StrategyBalanced,
	}))

	req := httptest.NewRequest(http.MethodGet, "/cloudfederation/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total_providers"])
	assert.Equal(t, float64(1), data["total_namespaces"])
}

func TestSameProviderSync(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.RegisterProvider(&CloudProviderConfig{
		ID:        "aws-1",
		Name:      "AWS",
		Provider:  ProviderAWS,
		AccessKey: "k",
		SecretKey: "s",
	}))
	require.NoError(t, mgr.CreateNamespace(&Namespace{
		ID:        "ns-1",
		Name:      "Test",
		Providers: []string{"aws-1"},
		Strategy:  StrategyBalanced,
	}))

	err := mgr.CreateSyncTask(&SyncTask{
		ID:             "sync-1",
		Namespace:      "ns-1",
		SourceProvider: "aws-1",
		TargetProvider: "aws-1",
	})
	assert.ErrorIs(t, err, ErrSameProvider)
}

func TestProviderTypeFilter(t *testing.T) {
	_, r := setupTest(t)
	registerTestProvider(t, r, "aws-1", ProviderAWS)
	registerTestProvider(t, r, "ali-1", ProviderAliyun)

	req := httptest.NewRequest(http.MethodGet, "/cloudfederation/providers?type=aws", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}
