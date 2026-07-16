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

// ========== Manager 测试 ==========

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

func TestManager_CreateAutoID(t *testing.T) {
	m := newTestManager()
	err := m.Create(&BrandConfig{
		Name:        "AutoIDBrand",
		CompanyName: "Auto Corp",
		Theme:       Theme{Mode: "light", PrimaryColor: "#00ff00"},
	})
	require.NoError(t, err)

	brands := m.List()
	found := false
	for _, b := range brands {
		if b.Name == "AutoIDBrand" {
			found = true
			assert.NotEmpty(t, b.ID)
			assert.Contains(t, b.ID, "brand-")
		}
	}
	assert.True(t, found, "应找到自动生成ID的品牌")
}

func TestManager_DuplicateBrand(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "b1", Name: "Dup"})
	err := m.Create(&BrandConfig{ID: "b2", Name: "Dup"})
	assert.ErrorIs(t, err, ErrDuplicateBrand)
}

func TestManager_Get_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.Get("nonexistent")
	assert.ErrorIs(t, err, ErrBrandNotFound)
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

func TestManager_SetTheme_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.SetTheme("nonexistent", "dark")
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_SetTheme_AllModes(t *testing.T) {
	m := newTestManager()
	for _, mode := range []string{"light", "dark", "auto"} {
		err := m.SetTheme("default", mode)
		require.NoError(t, err)
		b, _ := m.Get("default")
		assert.Equal(t, mode, b.Theme.Mode)
	}
}

func TestManager_SetActive(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "alt", Name: "Alt", Theme: Theme{Mode: "light"}})
	err := m.SetActive("alt")
	require.NoError(t, err)
	assert.Equal(t, "alt", m.GetActive().ID)
}

func TestManager_SetActive_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.SetActive("nonexistent")
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_GetActive(t *testing.T) {
	m := newTestManager()
	active := m.GetActive()
	require.NotNil(t, active)
	assert.Equal(t, "default", active.ID)
	assert.Equal(t, "NAS-OS", active.Name)
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
	assert.Contains(t, err.Error(), "cannot delete default brand")
}

func TestManager_Delete_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.Delete("nonexistent")
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_Delete_ResetsActive(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "active-del", Name: "ActiveDel", Theme: Theme{Mode: "light"}})
	m.SetActive("active-del")
	assert.Equal(t, "active-del", m.GetActive().ID)

	err := m.Delete("active-del")
	require.NoError(t, err)
	assert.Equal(t, "default", m.GetActive().ID)
}

func TestManager_Update(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "upd-1", Name: "Original", Theme: Theme{Mode: "light"}})

	err := m.Update("upd-1", &BrandConfig{
		Name:        "Updated",
		CompanyName: "New Corp",
		Theme:       Theme{Mode: "dark", PrimaryColor: "#333"},
	})
	require.NoError(t, err)

	got, _ := m.Get("upd-1")
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, "New Corp", got.CompanyName)
	assert.Equal(t, "#333", got.Theme.PrimaryColor)
	assert.Equal(t, "upd-1", got.ID) // ID 应保持不变
}

func TestManager_Update_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.Update("nonexistent", &BrandConfig{Name: "X"})
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_UpdateLogo(t *testing.T) {
	m := newTestManager()
	err := m.UpdateLogo("default", Logo{LightURL: "/img/logo-light.png", DarkURL: "/img/logo-dark.png"})
	require.NoError(t, err)
	b, _ := m.Get("default")
	assert.Equal(t, "/img/logo-light.png", b.Logo.LightURL)
	assert.Equal(t, "/img/logo-dark.png", b.Logo.DarkURL)
}

func TestManager_UpdateLogo_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.UpdateLogo("nonexistent", Logo{LightURL: "/img/x.png"})
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_UpdateLoginScreen(t *testing.T) {
	m := newTestManager()
	ls := LoginScreen{
		Title:         "Welcome",
		Subtitle:      "Custom Login",
		BackgroundURL: "/bg/login.jpg",
		BgColor:       "#000",
		ShowLogo:      true,
		ShowTagline:   true,
		FooterText:    "© 2024",
	}
	err := m.UpdateLoginScreen("default", ls)
	require.NoError(t, err)

	b, _ := m.Get("default")
	assert.Equal(t, "Welcome", b.LoginScreen.Title)
	assert.Equal(t, "Custom Login", b.LoginScreen.Subtitle)
	assert.Equal(t, "/bg/login.jpg", b.LoginScreen.BackgroundURL)
	assert.True(t, b.LoginScreen.ShowLogo)
}

func TestManager_UpdateLoginScreen_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.UpdateLoginScreen("nonexistent", LoginScreen{Title: "X"})
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_UpdateCustomCSS(t *testing.T) {
	m := newTestManager()
	css := CustomCSS{
		Enabled: true,
		Content: "body { background: #fff; }",
		URL:     "https://example.com/custom.css",
	}
	err := m.UpdateCustomCSS("default", css)
	require.NoError(t, err)

	b, _ := m.Get("default")
	assert.True(t, b.CustomCSS.Enabled)
	assert.Equal(t, "body { background: #fff; }", b.CustomCSS.Content)
	assert.Equal(t, "https://example.com/custom.css", b.CustomCSS.URL)
}

func TestManager_UpdateCustomCSS_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.UpdateCustomCSS("nonexistent", CustomCSS{Enabled: true})
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_UpdateFonts(t *testing.T) {
	m := newTestManager()
	fonts := Fonts{
		Primary:   "Roboto",
		Secondary: "Open Sans",
		Monospace: "Fira Code",
		GoogleURL: "https://fonts.googleapis.com/css2?family=Roboto",
	}
	err := m.UpdateFonts("default", fonts)
	require.NoError(t, err)

	b, _ := m.Get("default")
	assert.Equal(t, "Roboto", b.Fonts.Primary)
	assert.Equal(t, "Open Sans", b.Fonts.Secondary)
	assert.Equal(t, "Fira Code", b.Fonts.Monospace)
}

func TestManager_UpdateFonts_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.UpdateFonts("nonexistent", Fonts{Primary: "X"})
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_List(t *testing.T) {
	m := newTestManager()
	brands := m.List()
	assert.Len(t, brands, 1) // default

	m.Create(&BrandConfig{ID: "list-1", Name: "List1", Theme: Theme{Mode: "light"}})
	m.Create(&BrandConfig{ID: "list-2", Name: "List2", Theme: Theme{Mode: "dark"}})

	brands = m.List()
	assert.Len(t, brands, 3)
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

func TestManager_Export_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.Export("nonexistent")
	assert.ErrorIs(t, err, ErrBrandNotFound)
}

func TestManager_Import_InvalidJSON(t *testing.T) {
	m := newTestManager()
	_, err := m.Import([]byte("not json"))
	assert.Error(t, err)
}

func TestManager_ExportAll(t *testing.T) {
	m := newTestManager()
	data, err := m.ExportAll()
	require.NoError(t, err)
	assert.Contains(t, string(data), "default")
}

func TestManager_Import_DuplicateName(t *testing.T) {
	m := newTestManager()
	// 导入一个同名但不同ID的品牌，应该失败
	data := []byte(`{"id":"other-id","name":"NAS-OS","theme":{"mode":"light","primary_color":"#000"}}`)
	_, err := m.Import(data)
	assert.Error(t, err)
}

// ========== Handler 测试 ==========

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

func TestHandlers_CreateBrand_Conflict(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"id":"dup-1","name":"NAS-OS","theme":{"mode":"light","primary_color":"#000"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlers_CreateBrand_InvalidJSON(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/default", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NAS-OS")
}

func TestHandlers_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetActiveBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/active", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NAS-OS")
}

func TestHandlers_SetActiveBrand(t *testing.T) {
	m := newTestManager()
	m.Create(&BrandConfig{ID: "alt-h", Name: "AltH", Theme: Theme{Mode: "light"}})
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/active/alt-h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "alt-h", m.GetActive().ID)
}

func TestHandlers_SetActiveBrand_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/active/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UpdateBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"name":"UpdatedBrand","theme":{"mode":"dark","primary_color":"#111"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_UpdateBrand_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"name":"X","theme":{"mode":"light","primary_color":"#000"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
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

func TestHandlers_DeleteBrand_Default(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/branding/default", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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

func TestHandlers_SetTheme_InvalidMode(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/theme", bytes.NewBufferString(`{"mode":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_SetTheme_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent/theme", bytes.NewBufferString(`{"mode":"dark"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_UpdateLogo(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"light_url":"/img/light.png","dark_url":"/img/dark.png","width":200,"height":60}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/logo", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	b, _ := m.Get("default")
	assert.Equal(t, "/img/light.png", b.Logo.LightURL)
}

func TestHandlers_UpdateLogo_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent/logo", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UpdateLoginScreen(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"title":"Welcome","subtitle":"To NAS","show_logo":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/login-screen", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	b, _ := m.Get("default")
	assert.Equal(t, "Welcome", b.LoginScreen.Title)
}

func TestHandlers_UpdateLoginScreen_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent/login-screen", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UpdateCustomCSS(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"enabled":true,"content":"body{color:red}"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/custom-css", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	b, _ := m.Get("default")
	assert.True(t, b.CustomCSS.Enabled)
}

func TestHandlers_UpdateCustomCSS_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent/custom-css", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UpdateFonts(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	body := `{"primary":"Roboto","monospace":"Fira Code"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/default/fonts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	b, _ := m.Get("default")
	assert.Equal(t, "Roboto", b.Fonts.Primary)
}

func TestHandlers_UpdateFonts_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/branding/nonexistent/fonts", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_ExportBrand(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/default/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NAS-OS")
}

func TestHandlers_ExportBrand_NotFound(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/nonexistent/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_ExportAll(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/branding/export-all", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "default")
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

func TestHandlers_ImportBrand_InvalidJSON(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding/import", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_ImportBrand_EmptyBody(t *testing.T) {
	m := newTestManager()
	r := newTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/branding/import", bytes.NewBufferString(""))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
