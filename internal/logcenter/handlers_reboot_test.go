package logcenter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHandlersRebootHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(zap.NewNop(), &LogConfig{MaxEntries: 100, RetentionDays: 30})
	h := NewHandlers(zap.NewNop(), mgr)

	router := gin.New()
	h.RegisterRoutes(router.Group("/api/v1"))

	body := bytes.NewBufferString(`{"node":"nas-1","source":"watchdog","details":"kernel panic watchdog reset"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logcenter/reboots", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST status=%d body=%s", w.Code, w.Body.String())
	}

	var created RebootEvent
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Reason != RebootReasonKernelPanic || created.Node != "nas-1" {
		t.Fatalf("unexpected created reboot: %+v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/logcenter/reboots?limit=10", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", w.Code, w.Body.String())
	}

	var listed struct {
		Reboots []RebootEvent `json:"reboots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Reboots) != 1 || listed.Reboots[0].Reason != RebootReasonKernelPanic {
		t.Fatalf("unexpected reboot list: %+v", listed.Reboots)
	}
}
