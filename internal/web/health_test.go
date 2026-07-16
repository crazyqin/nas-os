package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/arch"

	"github.com/gin-gonic/gin"
)

type stubModule struct {
	name string
	tier arch.ModuleTier
	err  error
}

func (m *stubModule) Name() string                 { return m.name }
func (m *stubModule) Tier() arch.ModuleTier        { return m.tier }
func (m *stubModule) Dependencies() []string       { return nil }
func (m *stubModule) Init(context.Context) error   { return nil }
func (m *stubModule) Start(context.Context) error  { return nil }
func (m *stubModule) Stop(context.Context) error   { return nil }
func (m *stubModule) Health(context.Context) error { return m.err }

func TestAggregateCoreHealthAllHealthy(t *testing.T) {
	mods := []arch.Module{
		&stubModule{name: "identity", tier: arch.ModuleTierCore},
		&stubModule{name: "storage", tier: arch.ModuleTierCore},
		&stubModule{name: "lab-thing", tier: arch.ModuleTierLab, err: errors.New("ignored")},
	}
	report := AggregateCoreHealth(context.Background(), mods)
	if report.Code != 0 || report.Data.Status != "healthy" || report.Message != "healthy" {
		t.Fatalf("want healthy report, got code=%d status=%q msg=%q", report.Code, report.Data.Status, report.Message)
	}
	if len(report.Data.Modules) != 2 {
		t.Fatalf("want 2 core modules, got %d", len(report.Data.Modules))
	}
}

func TestAggregateCoreHealthReportsFailure(t *testing.T) {
	mods := []arch.Module{
		&stubModule{name: "identity", tier: arch.ModuleTierCore},
		&stubModule{name: "storage", tier: arch.ModuleTierCore, err: errors.New("disk missing")},
		&stubModule{name: "network", tier: arch.ModuleTierCore},
	}
	report := AggregateCoreHealth(context.Background(), mods)
	if report.Code == 0 || report.Data.Status != "unhealthy" {
		t.Fatalf("want unhealthy, got code=%d status=%q", report.Code, report.Data.Status)
	}
	var foundFail bool
	for _, m := range report.Data.Modules {
		if m.Name == "storage" {
			foundFail = true
			if m.Healthy || m.Error != "disk missing" {
				t.Fatalf("storage entry: %+v", m)
			}
		}
	}
	if !foundFail {
		t.Fatal("storage failure not reported")
	}
}

// TestGetHealthHandlerReflectsCoreFailure drives the real getHealth handler.
func TestGetHealthHandlerReflectsCoreFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{
		modules: []arch.Module{
			&stubModule{name: "identity", tier: arch.ModuleTierCore},
			&stubModule{name: "storage", tier: arch.ModuleTierCore, err: errors.New("pool down")},
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil)

	s.getHealth(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body CoreHealthReport
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	if body.Data.Status != "unhealthy" || body.Code == 0 {
		t.Fatalf("handler must surface core failure, got %+v", body)
	}
	if body.Message == "healthy" {
		t.Fatal("message still constant healthy")
	}
}
