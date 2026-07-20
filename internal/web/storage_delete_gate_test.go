package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

func TestStorageHandlers_DeleteVolume_RequiresConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Manager with no real volumes; confirmation must fail before wipe path.
	mgr := &storage.Manager{}
	// Use unexported volumes via DeleteVolumeConfirmed which validates first.
	// Build a manager with empty map through NewManager on temp — may fail btrfs scan.
	// Prefer zero-value with volumes set via DeleteVolumeConfirmed tests on storage package;
	// here we only exercise the HTTP gate via a thin stub path.

	// Use real storage package validation through handler with a manager that has empty volumes.
	// NewManager requires btrfs; construct via package test helper pattern.
	m, err := storage.NewManager(t.TempDir())
	if err != nil {
		// scan may fail without btrfs tools — still construct empty manager for gate test
		m = &storage.Manager{}
	}
	_ = mgr

	h := NewStorageHandlers(m)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	// Unconfirmed DELETE → 400
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/storage/volumes/tank?force=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed delete: status=%d body=%s", w.Code, w.Body.String())
	}

	// Mismatched confirm → 400
	body, _ := json.Marshal(storage.DeleteVolumeOptions{
		ConfirmName: "wrong",
		AllowWipe:   true,
		Force:       true,
	})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/storage/volumes/tank", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mismatched confirm: status=%d body=%s", w.Code, w.Body.String())
	}

	// Soft delete (confirm only, no allow_wipe) still passes gate → manager error if missing
	body, _ = json.Marshal(storage.DeleteVolumeOptions{
		ConfirmName: "tank",
		AllowWipe:   false,
	})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/storage/volumes/tank", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("soft-delete confirm should pass gate; body=%s", w.Body.String())
	}

	// Fully confirmed wipe but volume missing → 500 (gate passed, manager error)
	body, _ = json.Marshal(storage.DeleteVolumeOptions{
		ConfirmName: "tank",
		AllowWipe:   true,
		Force:       true,
	})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/storage/volumes/tank", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("confirmed path should not return 400 for missing volume; body=%s", w.Body.String())
	}
	// 500 is expected when volume does not exist
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Logf("confirmed delete status=%d body=%s (ok if volume missing → 500)", w.Code, w.Body.String())
	}
}
