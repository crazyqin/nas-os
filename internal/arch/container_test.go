package arch

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
func (t *testModule) Start(ctx context.Context) error { t.startCalled = true; return nil }
func (t *testModule) Stop(ctx context.Context) error  { t.stopCalled = true; return nil }

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

type orderedModule struct {
	*BaseModule
	events   *[]string
	startErr error
	stopErr  error
}

func (m *orderedModule) Start(context.Context) error {
	*m.events = append(*m.events, "start:"+m.Name())
	return m.startErr
}

func (m *orderedModule) Stop(context.Context) error {
	*m.events = append(*m.events, "stop:"+m.Name())
	return m.stopErr
}

func TestContainerRejectsMissingDependency(t *testing.T) {
	c := NewContainer(zap.NewNop())
	if err := c.RegisterModule(&BaseModule{NameStr: "sharing", Deps: []string{"identity"}, Logger: zap.NewNop()}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.topoSort(); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestContainerRejectsDuplicateModule(t *testing.T) {
	c := NewContainer(zap.NewNop())
	if err := c.RegisterModule(&BaseModule{NameStr: "identity"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RegisterModule(&BaseModule{NameStr: "identity"}); err == nil {
		t.Fatal("expected duplicate module error")
	}
}

type initRollbackModule struct {
	*BaseModule
	events  *[]string
	initErr error
	stopErr error
}

func (m *initRollbackModule) Init(context.Context) error {
	*m.events = append(*m.events, "init:"+m.Name())
	return m.initErr
}

func (m *initRollbackModule) Stop(context.Context) error {
	*m.events = append(*m.events, "stop:"+m.Name())
	return m.stopErr
}

func TestContainerInitFailureRollsBackInitializedModules(t *testing.T) {
	c := NewContainer(zap.NewNop())
	events := []string{}
	initFailure := fmt.Errorf("init boom")
	mods := []*initRollbackModule{
		{BaseModule: &BaseModule{NameStr: "identity"}, events: &events},
		{BaseModule: &BaseModule{NameStr: "storage", Deps: []string{"identity"}}, events: &events},
		{BaseModule: &BaseModule{NameStr: "sharing", Deps: []string{"identity", "storage"}}, events: &events, initErr: initFailure},
	}
	for _, mod := range mods {
		if err := c.RegisterModule(mod); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.InitAll(context.Background()); !errors.Is(err, initFailure) {
		t.Fatalf("expected init failure, got %v", err)
	}
	want := []string{"init:identity", "init:storage", "init:sharing", "stop:storage", "stop:identity"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestContainerInitRollbackAggregatesStopErrors(t *testing.T) {
	c := NewContainer(zap.NewNop())
	events := []string{}
	initFailure := fmt.Errorf("init boom")
	stopFailure := fmt.Errorf("stop boom")
	mods := []*initRollbackModule{
		{BaseModule: &BaseModule{NameStr: "identity"}, events: &events, stopErr: stopFailure},
		{BaseModule: &BaseModule{NameStr: "sharing", Deps: []string{"identity"}}, events: &events, initErr: initFailure},
	}
	for _, mod := range mods {
		if err := c.RegisterModule(mod); err != nil {
			t.Fatal(err)
		}
	}

	err := c.InitAll(context.Background())
	if !errors.Is(err, initFailure) || !errors.Is(err, stopFailure) {
		t.Fatalf("expected joined init and rollback errors, got %v", err)
	}
}

func TestContainerStartFailureRollsBack(t *testing.T) {
	c := NewContainer(zap.NewNop())
	events := []string{}
	startFailure := fmt.Errorf("boom")
	mods := []*orderedModule{
		{BaseModule: &BaseModule{NameStr: "identity"}, events: &events},
		{BaseModule: &BaseModule{NameStr: "storage", Deps: []string{"identity"}}, events: &events},
		{BaseModule: &BaseModule{NameStr: "sharing", Deps: []string{"identity", "storage"}}, events: &events, startErr: startFailure},
	}
	for _, mod := range mods {
		if err := c.RegisterModule(mod); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.StartAll(context.Background()); !errors.Is(err, startFailure) {
		t.Fatalf("expected start failure, got %v", err)
	}
	want := []string{"start:identity", "start:storage", "start:sharing", "stop:storage", "stop:identity"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestContainerStopAggregatesErrorsAndContinues(t *testing.T) {
	c := NewContainer(zap.NewNop())
	events := []string{}
	identityErr := fmt.Errorf("identity stop")
	storageErr := fmt.Errorf("storage stop")
	mods := []*orderedModule{
		{BaseModule: &BaseModule{NameStr: "identity"}, events: &events, stopErr: identityErr},
		{BaseModule: &BaseModule{NameStr: "storage", Deps: []string{"identity"}}, events: &events, stopErr: storageErr},
	}
	for _, mod := range mods {
		if err := c.RegisterModule(mod); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.StopAll(context.Background()); !errors.Is(err, identityErr) || !errors.Is(err, storageErr) {
		t.Fatalf("expected joined errors, got %v", err)
	}
	want := []string{"stop:storage", "stop:identity"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestContainer_ModuleQueriesAreDeterministic(t *testing.T) {
	c := NewContainer(zap.NewNop())
	for _, name := range []string{"storage", "identity", "network"} {
		if err := c.RegisterModule(&BaseModule{NameStr: name, Logger: zap.NewNop()}); err != nil {
			t.Fatal(err)
		}
	}

	want := []string{"identity", "network", "storage"}
	if got := c.ListModules(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ListModules() = %v, want %v", got, want)
	}
	statuses := c.GetModulesStatus(context.Background())
	got := make([]string, 0, len(statuses))
	for _, status := range statuses {
		got = append(got, status.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status order = %v, want %v", got, want)
	}
}

type registeringHealthModule struct {
	*BaseModule
	container *Container
}

func (m *registeringHealthModule) Health(context.Context) error {
	return m.container.RegisterModule(&BaseModule{NameStr: "registered-during-health", Logger: zap.NewNop()})
}

func TestContainer_HealthCallbacksRunOutsideLock(t *testing.T) {
	c := NewContainer(zap.NewNop())
	module := &registeringHealthModule{
		BaseModule: &BaseModule{NameStr: "health", Logger: zap.NewNop()},
		container:  c,
	}
	if err := c.RegisterModule(module); err != nil {
		t.Fatal(err)
	}

	statuses := c.GetModulesStatus(context.Background())
	if len(statuses) != 1 || !statuses[0].Healthy {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
	if _, ok := c.GetModule("registered-during-health"); !ok {
		t.Fatal("health callback could not register module; container lock may be held")
	}
}

func TestContainer_HealthAllCallbacksRunOutsideLock(t *testing.T) {
	c := NewContainer(zap.NewNop())
	module := &registeringHealthModule{
		BaseModule: &BaseModule{NameStr: "health-all", Logger: zap.NewNop()},
		container:  c,
	}
	if err := c.RegisterModule(module); err != nil {
		t.Fatal(err)
	}

	results := c.HealthAll(context.Background())
	if err := results["health-all"]; err != nil {
		t.Fatalf("unexpected health error: %v", err)
	}
	if _, ok := c.GetModule("registered-during-health"); !ok {
		t.Fatal("health callback could not register module; container lock may be held")
	}
}
