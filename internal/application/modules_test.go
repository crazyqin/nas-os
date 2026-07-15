package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nas-os/internal/arch"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCoreModuleDependencyOrder(t *testing.T) {
	container := arch.NewContainer(zap.NewNop())
	registrations := []struct {
		name string
		deps []string
	}{
		{name: moduleIdentity},
		{name: moduleStorage},
		{name: moduleNetwork},
		{name: moduleSharing, deps: []string{moduleIdentity, moduleStorage, moduleNetwork}},
		{name: moduleSystem, deps: []string{moduleIdentity, moduleStorage, moduleNetwork, moduleSharing}},
	}
	for _, registration := range registrations {
		if err := container.RegisterModule(arch.NewModuleAdapter(registration.name, registration.deps, zap.NewNop())); err != nil {
			t.Fatal(err)
		}
	}

	order := []string{}
	for _, name := range []string{moduleIdentity, moduleStorage, moduleNetwork, moduleSharing, moduleSystem} {
		module, ok := container.GetModule(name)
		if !ok {
			t.Fatalf("module %s missing", name)
		}
		adapter := module.(*arch.ModuleAdapter)
		adapter.WithStart(func(moduleName string) func(context.Context) error {
			return func(context.Context) error {
				order = append(order, moduleName)
				return nil
			}
		}(name))
	}
	if err := container.StartAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{moduleIdentity, moduleNetwork, moduleStorage, moduleSharing, moduleSystem}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("start order = %v, want %v", order, want)
	}
}

func TestRequireService(t *testing.T) {
	if err := requireService("test", nil)(context.Background()); err == nil {
		t.Fatal("expected nil service error")
	}
	if err := requireService("test", struct{}{})(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoreRouteContracts(t *testing.T) {
	identity := &identityModule{}
	storage := &storageModule{}
	network := &networkModule{}
	sharing := &sharingModule{}

	if _, ok := interface{}(identity).(arch.PublicRouteRegistrar); !ok {
		t.Fatal("identity must own public routes")
	}
	if _, ok := interface{}(identity).(arch.AuthenticatedRouteRegistrar); !ok {
		t.Fatal("identity must own authenticated routes")
	}
	if _, ok := interface{}(identity).(arch.RouteRegistrar); ok {
		t.Fatal("identity must not be registered as admin-only module")
	}
	for name, module := range map[string]interface{}{
		moduleStorage: storage,
		moduleNetwork: network,
		moduleSharing: sharing,
	} {
		if _, ok := module.(arch.RouteRegistrar); !ok {
			t.Fatalf("%s must own admin routes", name)
		}
		if _, ok := module.(arch.PublicRouteRegistrar); ok {
			t.Fatalf("%s must not expose public routes", name)
		}
	}
}

func TestSystemModuleReportsContainerHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	container := arch.NewContainer(zap.NewNop())
	if err := container.RegisterModule(&coreModule{name: moduleSystem}); err != nil {
		t.Fatal(err)
	}
	module := &systemModule{coreModule: coreModule{name: moduleSystem}, container: container}
	router := gin.New()
	api := router.Group("/api/v1")
	module.RegisterRoutes(api)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/modules", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"name":"system"`) {
		t.Fatalf("module status missing: %s", recorder.Body.String())
	}
}

type routeRecorder struct{ called bool }

func (r *routeRecorder) RegisterRoutes(*gin.RouterGroup) { r.called = true }

func TestStorageModuleDelegatesCompatibilityRoutes(t *testing.T) {
	recorder := &routeRecorder{}
	module := &storageModule{compatRoutes: recorder}
	module.RegisterRoutes(gin.New().Group("/api/v1"))
	if !recorder.called {
		t.Fatal("storage compatibility routes were not registered")
	}
}
