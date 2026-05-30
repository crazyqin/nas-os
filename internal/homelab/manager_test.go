package homelab

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	mgr := NewManager(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, mgr.Initialize())
	return mgr
}

// === Service CRUD ===

func TestCreateService(t *testing.T) {
	mgr := newTestManager(t)
	svc := &Service{ID: "svc-1", Name: "NGINX", Type: ServiceDocker, Image: "nginx:latest", Port: 80}
	require.NoError(t, mgr.CreateService(svc))
	assert.Equal(t, StatusStopped, svc.Status)
	assert.False(t, svc.CreatedAt.IsZero())
}

func TestCreateServiceDuplicate(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "dup-1", Name: "A", Type: ServiceDocker}))
	err := mgr.CreateService(&Service{ID: "dup-1", Name: "B", Type: ServiceDocker})
	assert.ErrorIs(t, err, ErrServiceExists)
}

func TestCreateServiceInvalidType(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.CreateService(&Service{ID: "bad", Name: "X", Type: "foobar"})
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestGetService(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "svc-g", Name: "Redis", Type: ServiceDocker}))
	svc, err := mgr.GetService("svc-g")
	require.NoError(t, err)
	assert.Equal(t, "Redis", svc.Name)
}

func TestGetServiceNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.GetService("nonexistent")
	assert.ErrorIs(t, err, ErrServiceNotFound)
}

func TestDeleteService(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "del-1", Name: "Tmp", Type: ServiceDocker}))
	require.NoError(t, mgr.DeleteService("del-1"))
	_, err := mgr.GetService("del-1")
	assert.ErrorIs(t, err, ErrServiceNotFound)
}

func TestDeleteServiceNotFound(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.DeleteService("ghost")
	assert.ErrorIs(t, err, ErrServiceNotFound)
}

func TestListServicesFilterType(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "d1", Name: "D", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "v1", Name: "V", Type: ServiceVM}))
	require.NoError(t, mgr.CreateService(&Service{ID: "d2", Name: "D2", Type: ServiceDocker}))

	docker := mgr.ListServices(ServiceDocker, "")
	assert.Len(t, docker, 2)

	vm := mgr.ListServices(ServiceVM, "")
	assert.Len(t, vm, 1)
}

func TestListServicesFilterStatus(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "r1", Name: "R1", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "r2", Name: "R2", Type: ServiceDocker}))
	require.NoError(t, mgr.StartService("r1"))

	running := mgr.ListServices("", StatusRunning)
	assert.Len(t, running, 1)
	assert.Equal(t, "r1", running[0].ID)

	stopped := mgr.ListServices("", StatusStopped)
	assert.Len(t, stopped, 1)
}

// === Lifecycle ===

func TestStartStopRestart(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "lc", Name: "LC", Type: ServiceDocker}))

	require.NoError(t, mgr.StartService("lc"))
	svc, _ := mgr.GetService("lc")
	assert.Equal(t, StatusRunning, svc.Status)

	require.NoError(t, mgr.StopService("lc"))
	svc, _ = mgr.GetService("lc")
	assert.Equal(t, StatusStopped, svc.Status)

	require.NoError(t, mgr.RestartService("lc"))
	svc, _ = mgr.GetService("lc")
	assert.Equal(t, StatusRunning, svc.Status)
	assert.Equal(t, 1, svc.RestartCount)
}

func TestStartNotFound(t *testing.T) {
	mgr := newTestManager(t)
	assert.ErrorIs(t, mgr.StartService("nope"), ErrServiceNotFound)
}

func TestStopNotFound(t *testing.T) {
	mgr := newTestManager(t)
	assert.ErrorIs(t, mgr.StopService("nope"), ErrServiceNotFound)
}

func TestRestartNotFound(t *testing.T) {
	mgr := newTestManager(t)
	assert.ErrorIs(t, mgr.RestartService("nope"), ErrServiceNotFound)
}

// === Stacks ===

func TestCreateAndGetStack(t *testing.T) {
	mgr := newTestManager(t)
	stack := &Stack{ID: "st-1", Name: "WebApp", Services: []string{"s1", "s2"}}
	require.NoError(t, mgr.CreateStack(stack))
	assert.Equal(t, StatusStopped, stack.Status)

	got, err := mgr.GetStack("st-1")
	require.NoError(t, err)
	assert.Equal(t, "WebApp", got.Name)
}

func TestGetStackNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.GetStack("nope")
	assert.ErrorIs(t, err, ErrStackNotFound)
}

func TestStartStopStack(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "st-s1", Name: "S1", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "st-s2", Name: "S2", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateStack(&Stack{ID: "st-main", Name: "Main", Services: []string{"st-s1", "st-s2"}}))

	require.NoError(t, mgr.StartStack("st-main"))
	stack, _ := mgr.GetStack("st-main")
	assert.Equal(t, StatusRunning, stack.Status)
	s1, _ := mgr.GetService("st-s1")
	assert.Equal(t, StatusRunning, s1.Status)

	require.NoError(t, mgr.StopStack("st-main"))
	stack, _ = mgr.GetStack("st-main")
	assert.Equal(t, StatusStopped, stack.Status)
	s1, _ = mgr.GetService("st-s1")
	assert.Equal(t, StatusStopped, s1.Status)
}

func TestStartStackNotFound(t *testing.T) {
	mgr := newTestManager(t)
	assert.ErrorIs(t, mgr.StartStack("nope"), ErrStackNotFound)
}

func TestStopStackNotFound(t *testing.T) {
	mgr := newTestManager(t)
	assert.ErrorIs(t, mgr.StopStack("nope"), ErrStackNotFound)
}

func TestListStacks(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateStack(&Stack{ID: "a", Name: "A"}))
	require.NoError(t, mgr.CreateStack(&Stack{ID: "b", Name: "B"}))
	assert.Len(t, mgr.ListStacks(), 2)
}

// === Templates ===

func TestListTemplates(t *testing.T) {
	mgr := newTestManager(t)
	all := mgr.ListTemplates("")
	assert.True(t, len(all) >= 8, "should have at least 8 default templates")

	media := mgr.ListTemplates("媒体")
	for _, tpl := range media {
		assert.Equal(t, "媒体", tpl.Category)
	}
}

func TestManagerDeployFromTemplate(t *testing.T) {
	mgr := newTestManager(t)
	svc, err := mgr.DeployFromTemplate("nextcloud", "my-cloud", map[string]string{"DB": "postgres"})
	require.NoError(t, err)
	assert.Equal(t, "my-cloud", svc.Name)
	assert.Equal(t, ServiceDocker, svc.Type)
	assert.Equal(t, StatusRunning, svc.Status)
	assert.Equal(t, "nextcloud", svc.Labels["template"])

	templates := mgr.ListTemplates("")
	for _, tpl := range templates {
		if tpl.ID == "nextcloud" {
			assert.Equal(t, 50001, tpl.Downloads, "downloads should increment")
			break
		}
	}
}

func TestManagerDeployFromTemplateNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.DeployFromTemplate("nonexistent", "x", nil)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

// === Stats ===

func TestGetStats(t *testing.T) {
	mgr := newTestManager(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "st-a", Name: "A", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "st-b", Name: "B", Type: ServiceDocker}))
	require.NoError(t, mgr.StartService("st-a"))
	require.NoError(t, mgr.CreateStack(&Stack{ID: "sx", Name: "SX"}))

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats["total_services"])
	assert.Equal(t, 1, stats["running"])
	assert.Equal(t, 1, stats["stopped"])
	assert.Equal(t, 1, stats["total_stacks"])
	assert.True(t, stats["total_templates"].(int) > 0)
}

// === Persistence ===

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")

	mgr1 := NewManager(file)
	require.NoError(t, mgr1.Initialize())
	require.NoError(t, mgr1.CreateService(&Service{ID: "persist-1", Name: "Persist", Type: ServiceDocker}))
	require.NoError(t, mgr1.StartService("persist-1"))

	mgr2 := NewManager(file)
	require.NoError(t, mgr2.Initialize())
	svc, err := mgr2.GetService("persist-1")
	require.NoError(t, err)
	assert.Equal(t, "Persist", svc.Name)
	assert.Equal(t, StatusRunning, svc.Status)
}

func TestPersistenceStacks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")

	mgr1 := NewManager(file)
	require.NoError(t, mgr1.Initialize())
	require.NoError(t, mgr1.CreateStack(&Stack{ID: "ps-1", Name: "PS", Services: []string{"a", "b"}}))

	mgr2 := NewManager(file)
	require.NoError(t, mgr2.Initialize())
	stack, err := mgr2.GetStack("ps-1")
	require.NoError(t, err)
	assert.Equal(t, "PS", stack.Name)
}

func TestNoDataFile(t *testing.T) {
	mgr := NewManager("")
	require.NoError(t, mgr.Initialize())
	require.NoError(t, mgr.CreateService(&Service{ID: "mem-1", Name: "Mem", Type: ServiceDocker}))
	svc, _ := mgr.GetService("mem-1")
	assert.NotNil(t, svc)
}

// === Max Services ===

func TestMaxServicesLimit(t *testing.T) {
	mgr := newTestManager(t)
	mgr.config.MaxServices = 2

	require.NoError(t, mgr.CreateService(&Service{ID: "m1", Name: "M1", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "m2", Name: "M2", Type: ServiceDocker}))
	err := mgr.CreateService(&Service{ID: "m3", Name: "M3", Type: ServiceDocker})
	assert.ErrorIs(t, err, ErrMaxServices)
}
