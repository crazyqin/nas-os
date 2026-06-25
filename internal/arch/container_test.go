package arch

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestContainer_RegisterAndGet(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	c.Register("test", "value")
	val, ok := c.Get("test")
	if !ok || val != "value" {
		t.Errorf("expected value, got %v", val)
	}

	_, ok = c.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent")
	}
}

func TestContainer_MustGet(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	c.Register("test", "value")
	val := c.MustGet("test")
	if val != "value" {
		t.Errorf("expected value, got %v", val)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing service")
		}
	}()
	c.MustGet("nonexistent")
}

func TestContainer_RegisterModule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	mod := &BaseModule{NameStr: "test", Logger: logger}
	c.RegisterModule(mod)

	names := c.ListModules()
	if len(names) != 1 || names[0] != "test" {
		t.Errorf("expected [test], got %v", names)
	}
}

func TestContainer_TopoSort(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	c.RegisterModule(&BaseModule{NameStr: "a", Logger: logger})
	c.RegisterModule(&BaseModule{NameStr: "b", Deps: []string{"a"}, Logger: logger})
	c.RegisterModule(&BaseModule{NameStr: "c", Deps: []string{"a", "b"}, Logger: logger})

	sorted, err := c.topoSort()
	if err != nil {
		t.Fatal(err)
	}

	idxA, idxB, idxC := -1, -1, -1
	for i, s := range sorted {
		switch s {
		case "a":
			idxA = i
		case "b":
			idxB = i
		case "c":
			idxC = i
		}
	}

	if idxA >= idxB || idxA >= idxC || idxB >= idxC {
		t.Errorf("wrong order: a=%d b=%d c=%d", idxA, idxB, idxC)
	}
}

func TestContainer_TopoSortCircular(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	c.RegisterModule(&BaseModule{NameStr: "a", Deps: []string{"b"}, Logger: logger})
	c.RegisterModule(&BaseModule{NameStr: "b", Deps: []string{"a"}, Logger: logger})

	_, err := c.topoSort()
	if err == nil {
		t.Error("expected circular dependency error")
	}
}

type testModule struct {
	*BaseModule
	initCalled  bool
	startCalled bool
	stopCalled  bool
}

func (t *testModule) Init(ctx context.Context) error  { t.initCalled = true; return nil }
func (t *testModule) Start(ctx context.Context) error  { t.startCalled = true; return nil }
func (t *testModule) Stop(ctx context.Context) error   { t.stopCalled = true; return nil }

func TestContainer_InitStartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	mod := &testModule{BaseModule: &BaseModule{NameStr: "test", Logger: logger}}
	c.RegisterModule(mod)

	ctx := context.Background()
	c.InitAll(ctx)
	if !mod.initCalled {
		t.Error("Init not called")
	}

	c.StartAll(ctx)
	if !mod.startCalled {
		t.Error("Start not called")
	}

	c.StopAll(ctx)
	if !mod.stopCalled {
		t.Error("Stop not called")
	}
}

func TestContainer_GetModulesStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewContainer(logger)

	c.RegisterModule(&BaseModule{NameStr: "healthy", Logger: logger})
	c.RegisterModule(&BaseModule{NameStr: "also-healthy", Logger: logger})

	ctx := context.Background()
	statuses := c.GetModulesStatus(ctx)
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if !s.Healthy {
			t.Errorf("module %s should be healthy", s.Name)
		}
	}
}

func TestModuleAdapter(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	initCalled := false
	adapter := NewModuleAdapter("test", nil, logger).
		WithInit(func(ctx context.Context) error { initCalled = true; return nil })

	ctx := context.Background()
	adapter.Init(ctx)
	if !initCalled {
		t.Error("Init not called")
	}

	if adapter.Name() != "test" {
		t.Errorf("expected test, got %s", adapter.Name())
	}
}
