package application

import (
	"context"
	"reflect"
	"testing"

	"nas-os/internal/arch"

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
