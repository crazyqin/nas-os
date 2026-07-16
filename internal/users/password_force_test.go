package users

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddlewareBlocksUntilPasswordChanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	// Use config path under temp so we control users map
	mgr := &Manager{
		users:      map[string]*User{},
		groups:     map[string]*Group{},
		tokens:     map[string]*Token{},
		configPath: filepath.Join(dir, "users.json"),
	}
	mgr.users["forceuser"] = &User{
		ID:                 "u1",
		Username:           "forceuser",
		Role:               RoleUser,
		MustChangePassword: true,
		PasswordHash:       mustHash(t, "oldpass1"),
	}
	tok := &Token{
		Token:     "tok-force-1",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	mgr.tokens[tok.Token] = tok

	r := gin.New()
	r.Use(AuthMiddleware(mgr))
	r.GET("/api/v1/me", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/api/v1/users", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.POST("/api/v1/me/password", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	// List users should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", tok.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for normal API while MustChangePassword, got %d body=%s", w.Code, w.Body.String())
	}

	// me allowed
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", tok.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d body=%s", w.Code, w.Body.String())
	}

	// me/password allowed
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/password", nil)
	req.Header.Set("Authorization", tok.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me/password, got %d", w.Code)
	}

	// After clear flag, normal API works
	mgr.users["forceuser"].MustChangePassword = false
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", tok.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 after password change flag cleared, got %d", w.Code)
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	// reuse bcrypt via ChangePassword path - simple placeholder hash for ValidateToken path
	// ValidateToken only looks up tokens map, not password hash.
	return "x"
}

func TestIsPasswordForceAllowedPath(t *testing.T) {
	if !isPasswordForceAllowedPath(http.MethodPost, "/api/v1/me/password", "/api/v1/me/password", "admin") {
		t.Fatal("me/password must be allowed")
	}
	if isPasswordForceAllowedPath(http.MethodGet, "/api/v1/storage/volumes", "/api/v1/storage/volumes", "admin") {
		t.Fatal("storage must not be allowed while force-change")
	}
}
