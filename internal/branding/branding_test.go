package branding

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager() *Manager {
	return NewManager()
}

func newTestRouter(m *Manager) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(api)
	return r
}

func TestManager_CreateAndGet(t *testing.T) {
	m := newTestManager()
	err := m.Create(&BrandConfig{
		ID:          "custom-1",
		Name:        "CustomBrand",
		CompanyName: "Custom Corp",
		Theme:       Theme{Mode: "light", PrimaryColor: "#ff0000"},
	})
	require.NoError(t, err)

	got, err := m.Get("custom-1")
	require.NoError(t, err)
	assert.Equal(t, "CustomBrand", got.Name)
	assert.Equal(t, "#ff0000", got.Theme.PrimaryColor)
}

func TestManager_DuplicateBrand(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "b1", Name: "Dup"})
	err := m.Create(&BrandConfig{ID: "b2", Name: "Dup"})
	assert.ErrorIs(t, err, ErrDuplicateBrand)
}

func TestManager_SetTheme(t *testing.T) {
	m := newTestManager()
	err := m.SetTheme("default", "dark")
	require.NoError(t, err)
	b, _ := m.Get("default")
	assert.Equal(t, "dark", b.Theme.Mode)

	err = m.SetTheme("default", "invalid")
	assert.ErrorIs(t, err, ErrInvalidTheme)
}

func TestManager_SetActive(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "alt", Name: "Alt", Theme: Theme{Mode: "light"}})
	err := m.SetActive("alt")
	require.NoError(t, err)
	assert.Equal(t, "alt", m.GetActive().ID)
}

func TestManager_Delete(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "del-1", Name: "ToDelete"})
	err := m.Delete("del-1")
	require.NoError(t, err)
	_, err = m.Get("del-1")
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_DeleteDefault(t *testing.T) {
	m := newTestManager()
	err := m.Delete("default")
	assert.Error(t, err)
}

func TestManager_UpdateLogo(t *testing.T) {
	m := newTestManager()
	err := m.UpdateLogo("default", Logo{LightURL: "/img/logo-light.png", DarkURL: "/img/logo-dark.png"})
	require.NoError(t, err)
	b, _ := m.Get("default")
	assert.Equal(t, "/img/logo-light.png", b.Logo.LightURL)
}

func TestManager_ExportImport(t *testing.T) {
	m := newTestManager()
	data, err := m.Export("default")
	require.NoError(t, err)
	assert.Contains(t, string(data), "NAS-OS")

	m2 := newTestManager()
	cfg, err := m2.Import(data)
	require.NoError(t, err)
	assert.Equal(t, "NAS-OS", cfg.Name)
}

func TestManager_ExportAll(t *testing.T) {
	m := newTestManager()
	data, err := m.ExportAll()
	require.NoError(t, err)
	assert.Contains(t, string(data), "default")
}

func TestHandlers_ListBrands(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "default")
}

func TestHandlers_CreateBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"id":"test-1","name":"TestBrand","company_name":"Test Co","theme":{"mode":"light","primary_color":"#000"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "TestBrand")
}

func TestHandlers_SetTheme(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/theme", bytes.NewBufferString(`{"mode":"dark"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_ImportBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"id":"imported","name":"ImportedBrand","theme":{"mode":"auto","primary_color":"#333"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "ImportedBrand")
}

func TestHandlers_DeleteBrand(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "to-del", Name: "ToDel"})
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/branding/to-del", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
