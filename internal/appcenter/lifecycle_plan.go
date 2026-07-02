package appcenter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// LifecycleAction describes an app lifecycle operation that can be planned before execution.
type LifecycleAction string

const (
	LifecycleActionInstall   LifecycleAction = "install"
	LifecycleActionUpdate    LifecycleAction = "update"
	LifecycleActionUninstall LifecycleAction = "uninstall"
	LifecycleActionStart     LifecycleAction = "start"
	LifecycleActionStop      LifecycleAction = "stop"
)

// LifecyclePlan is a dry-run execution plan for an app lifecycle operation.
// It is inspired by NAS app centers that show dependency changes, blockers, and
// operational impact before mutating an installed application.
type LifecyclePlan struct {
	AppID       string              `json:"app_id"`
	Action      LifecycleAction     `json:"action"`
	Executable  bool                `json:"executable"`
	Steps       []LifecyclePlanStep `json:"steps"`
	Blockers    []LifecycleBlocker  `json:"blockers,omitempty"`
	Warnings    []string            `json:"warnings,omitempty"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// LifecyclePlanStep is one ordered step in a lifecycle plan.
type LifecyclePlanStep struct {
	Order   int             `json:"order"`
	Action  LifecycleAction `json:"action"`
	AppID   string          `json:"app_id"`
	AppName string          `json:"app_name"`
	Reason  string          `json:"reason"`
}

// LifecycleBlocker explains why a lifecycle operation should not run.
type LifecycleBlocker struct {
	AppID   string `json:"app_id,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PlanLifecycle builds a dry-run plan for an app operation without mutating store state.
func (as *AppStore) PlanLifecycle(ctx context.Context, appID string, action LifecycleAction) (*LifecyclePlan, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	app, exists := as.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app %s not found", appID)
	}

	plan := &LifecyclePlan{
		AppID:       appID,
		Action:      action,
		GeneratedAt: time.Now(),
	}

	switch action {
	case LifecycleActionInstall:
		as.planInstall(app, plan)
	case LifecycleActionUpdate:
		as.planUpdate(app, plan)
	case LifecycleActionUninstall:
		as.planUninstall(app, plan)
	case LifecycleActionStart:
		as.planStart(app, plan)
	case LifecycleActionStop:
		as.planStop(app, plan)
	default:
		return nil, fmt.Errorf("unsupported lifecycle action %q", action)
	}

	plan.Executable = len(plan.Blockers) == 0
	return plan, nil
}

func (as *AppStore) planInstall(app *App, plan *LifecyclePlan) {
	if app.Installed {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "already_installed", Message: fmt.Sprintf("%s is already installed", app.Name)})
		return
	}

	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	for _, depID := range sortedStrings(app.Dependencies) {
		as.collectDependencyInstallSteps(depID, app.ID, visiting, visited, plan)
	}

	as.addPortConflictWarnings(app, plan)
	plan.addStep(LifecycleActionInstall, app, "requested application")
}

func (as *AppStore) collectDependencyInstallSteps(depID, requestedBy string, visiting, visited map[string]bool, plan *LifecyclePlan) {
	if visited[depID] {
		return
	}
	if visiting[depID] {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: depID, Code: "dependency_cycle", Message: fmt.Sprintf("dependency cycle detected at %s", depID)})
		return
	}

	dep, exists := as.apps[depID]
	if !exists {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: depID, Code: "missing_dependency", Message: fmt.Sprintf("dependency %s required by %s is not available", depID, requestedBy)})
		return
	}

	visiting[depID] = true
	for _, transitiveID := range sortedStrings(dep.Dependencies) {
		as.collectDependencyInstallSteps(transitiveID, depID, visiting, visited, plan)
	}
	visiting[depID] = false
	visited[depID] = true

	if !dep.Installed {
		as.addPortConflictWarnings(dep, plan)
		plan.addStep(LifecycleActionInstall, dep, fmt.Sprintf("dependency of %s", requestedBy))
	}
}

func (as *AppStore) planUpdate(app *App, plan *LifecyclePlan) {
	if !app.Installed {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "not_installed", Message: fmt.Sprintf("%s is not installed", app.Name)})
		return
	}
	if app.Status == "running" {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s is running and may be restarted during update", app.Name))
	}
	for _, dependent := range as.installedDependents(app.ID) {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s depends on %s; verify compatibility after update", dependent.Name, app.Name))
	}
	plan.addStep(LifecycleActionUpdate, app, "apply repository version")
}

func (as *AppStore) planUninstall(app *App, plan *LifecyclePlan) {
	if !app.Installed {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "not_installed", Message: fmt.Sprintf("%s is not installed", app.Name)})
		return
	}
	if app.Status == "running" {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "app_running", Message: fmt.Sprintf("%s is running; stop it before uninstall", app.Name)})
	}
	for _, dependent := range as.installedDependents(app.ID) {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: dependent.ID, Code: "required_by_installed_app", Message: fmt.Sprintf("%s is required by installed app %s", app.Name, dependent.Name)})
	}
	plan.addStep(LifecycleActionUninstall, app, "remove requested application")
}

func (as *AppStore) planStart(app *App, plan *LifecyclePlan) {
	if !app.Installed {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "not_installed", Message: fmt.Sprintf("%s is not installed", app.Name)})
		return
	}
	if !app.Enabled {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "disabled", Message: fmt.Sprintf("%s is disabled", app.Name)})
	}
	as.addPortConflictWarnings(app, plan)
	plan.addStep(LifecycleActionStart, app, "start service")
}

func (as *AppStore) planStop(app *App, plan *LifecyclePlan) {
	if !app.Installed {
		plan.Blockers = append(plan.Blockers, LifecycleBlocker{AppID: app.ID, Code: "not_installed", Message: fmt.Sprintf("%s is not installed", app.Name)})
		return
	}
	for _, dependent := range as.runningDependents(app.ID) {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("running app %s depends on %s and may be interrupted", dependent.Name, app.Name))
	}
	plan.addStep(LifecycleActionStop, app, "stop service")
}

func (as *AppStore) addPortConflictWarnings(app *App, plan *LifecyclePlan) {
	for _, port := range app.Ports {
		if port.HostPort == 0 {
			continue
		}
		for _, other := range as.apps {
			if other.ID == app.ID || !other.Installed {
				continue
			}
			for _, otherPort := range other.Ports {
				if samePort(port, otherPort) {
					plan.Warnings = append(plan.Warnings, fmt.Sprintf("port %d/%s is already used by installed app %s", port.HostPort, normalizedProtocol(port.Protocol), other.Name))
				}
			}
		}
	}
}

func (as *AppStore) installedDependents(appID string) []*App {
	dependents := make([]*App, 0)
	for _, app := range as.apps {
		if app.Installed && containsExact(app.Dependencies, appID) {
			dependents = append(dependents, app)
		}
	}
	sortApps(dependents)
	return dependents
}

func (as *AppStore) runningDependents(appID string) []*App {
	dependents := make([]*App, 0)
	for _, app := range as.apps {
		if app.Status == "running" && containsExact(app.Dependencies, appID) {
			dependents = append(dependents, app)
		}
	}
	sortApps(dependents)
	return dependents
}

func (p *LifecyclePlan) addStep(action LifecycleAction, app *App, reason string) {
	p.Steps = append(p.Steps, LifecyclePlanStep{
		Order:   len(p.Steps) + 1,
		Action:  action,
		AppID:   app.ID,
		AppName: app.Name,
		Reason:  reason,
	})
}

func samePort(a, b PortMapping) bool {
	return a.HostPort == b.HostPort && normalizedProtocol(a.Protocol) == normalizedProtocol(b.Protocol)
}

func normalizedProtocol(protocol string) string {
	if protocol == "" {
		return "tcp"
	}
	return strings.ToLower(protocol)
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedStrings(values []string) []string {
	clone := append([]string(nil), values...)
	sort.Strings(clone)
	return clone
}

func sortApps(apps []*App) {
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].ID < apps[j].ID
	})
}
