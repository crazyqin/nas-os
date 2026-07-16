package appcenter

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestPlanLifecycleInstallIncludesDependenciesInOrder(t *testing.T) {
	store := NewAppStore(zap.NewNop(), t.TempDir())
	ctx := context.Background()

	registerApps(t, store,
		&App{ID: "postgres", Name: "PostgreSQL", Version: "16", Category: "development"},
		&App{ID: "redis", Name: "Redis", Version: "7", Category: "development"},
		&App{ID: "project", Name: "Project Hub", Version: "1", Category: "productivity", Dependencies: []string{"redis", "postgres"}},
	)

	plan, err := store.PlanLifecycle(ctx, "project", LifecycleActionInstall)
	if err != nil {
		t.Fatalf("PlanLifecycle returned error: %v", err)
	}
	if !plan.Executable {
		t.Fatalf("expected executable plan, blockers: %#v", plan.Blockers)
	}

	want := []string{"postgres", "redis", "project"}
	if len(plan.Steps) != len(want) {
		t.Fatalf("expected %d steps, got %d: %#v", len(want), len(plan.Steps), plan.Steps)
	}
	for i, appID := range want {
		if plan.Steps[i].Order != i+1 {
			t.Fatalf("step %d order = %d", i, plan.Steps[i].Order)
		}
		if plan.Steps[i].AppID != appID {
			t.Fatalf("step %d app = %q, want %q", i, plan.Steps[i].AppID, appID)
		}
		if plan.Steps[i].Action != LifecycleActionInstall {
			t.Fatalf("step %d action = %q", i, plan.Steps[i].Action)
		}
	}

	app, err := store.GetApp(ctx, "project")
	if err != nil {
		t.Fatal(err)
	}
	if app.Installed {
		t.Fatal("planning must not mutate app installation state")
	}
}

func TestPlanLifecycleInstallBlocksMissingDependency(t *testing.T) {
	store := NewAppStore(zap.NewNop(), t.TempDir())
	ctx := context.Background()
	registerApps(t, store, &App{ID: "wiki", Name: "Wiki", Version: "1", Category: "productivity", Dependencies: []string{"mysql"}})

	plan, err := store.PlanLifecycle(ctx, "wiki", LifecycleActionInstall)
	if err != nil {
		t.Fatalf("PlanLifecycle returned error: %v", err)
	}
	if plan.Executable {
		t.Fatal("expected missing dependency to block plan")
	}
	if len(plan.Blockers) != 1 || plan.Blockers[0].Code != "missing_dependency" || plan.Blockers[0].AppID != "mysql" {
		t.Fatalf("unexpected blockers: %#v", plan.Blockers)
	}
}

func TestPlanLifecycleWarnsPortConflict(t *testing.T) {
	store := NewAppStore(zap.NewNop(), t.TempDir())
	ctx := context.Background()
	registerApps(t, store,
		&App{ID: "nginx", Name: "Nginx", Version: "1", Category: "network", Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}}},
		&App{ID: "docs", Name: "Docs", Version: "1", Category: "productivity", Ports: []PortMapping{{HostPort: 8080, ContainerPort: 8080}}},
	)
	if err := store.InstallApp(ctx, "nginx"); err != nil {
		t.Fatalf("InstallApp nginx: %v", err)
	}

	plan, err := store.PlanLifecycle(ctx, "docs", LifecycleActionInstall)
	if err != nil {
		t.Fatalf("PlanLifecycle returned error: %v", err)
	}
	if !plan.Executable {
		t.Fatalf("port conflict should warn but not block: %#v", plan.Blockers)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", plan.Warnings)
	}
}

func TestPlanLifecycleUninstallBlocksRunningAndDependents(t *testing.T) {
	store := NewAppStore(zap.NewNop(), t.TempDir())
	ctx := context.Background()
	registerApps(t, store,
		&App{ID: "db", Name: "Database", Version: "1", Category: "development"},
		&App{ID: "crm", Name: "CRM", Version: "1", Category: "productivity", Dependencies: []string{"db"}},
	)
	if err := store.InstallApp(ctx, "db"); err != nil {
		t.Fatalf("InstallApp db: %v", err)
	}
	if err := store.InstallApp(ctx, "crm"); err != nil {
		t.Fatalf("InstallApp crm: %v", err)
	}
	if err := store.StartApp(ctx, "db"); err != nil {
		t.Fatalf("StartApp db: %v", err)
	}

	plan, err := store.PlanLifecycle(ctx, "db", LifecycleActionUninstall)
	if err != nil {
		t.Fatalf("PlanLifecycle returned error: %v", err)
	}
	if plan.Executable {
		t.Fatal("expected uninstall to be blocked")
	}
	codes := map[string]bool{}
	for _, blocker := range plan.Blockers {
		codes[blocker.Code] = true
	}
	if !codes["app_running"] || !codes["required_by_installed_app"] {
		t.Fatalf("expected running and dependent blockers, got %#v", plan.Blockers)
	}
}

func TestPlanLifecycleUnsupportedAction(t *testing.T) {
	store := NewAppStore(zap.NewNop(), t.TempDir())
	registerApps(t, store, &App{ID: "demo", Name: "Demo", Version: "1", Category: "tools"})

	if _, err := store.PlanLifecycle(context.Background(), "demo", LifecycleAction("backup")); err == nil {
		t.Fatal("expected unsupported action error")
	}
}

func registerApps(t *testing.T, store *AppStore, apps ...*App) {
	t.Helper()
	for _, app := range apps {
		if err := store.RegisterApp(context.Background(), app); err != nil {
			t.Fatalf("RegisterApp(%s): %v", app.ID, err)
		}
	}
}
