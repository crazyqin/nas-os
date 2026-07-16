// Package iacengine provides Infrastructure as Code capabilities for managing
// NAS resources and services through declarative configuration templates.
package iacengine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages IaC templates, stacks, and resources.
type Manager struct {
	logger    *zap.Logger
	templates map[string]*IaCTemplate
	tmplMu    sync.RWMutex
	stacks    map[string]*Stack
	stackMu   sync.RWMutex
	resources map[string]*Resource
	resMu     sync.RWMutex
	drifts    map[string]*DriftReport
	driftMu   sync.RWMutex
}

// NewManager creates a new IaC engine manager.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:    logger,
		templates: make(map[string]*IaCTemplate),
		stacks:    make(map[string]*Stack),
		resources: make(map[string]*Resource),
		drifts:    make(map[string]*DriftReport),
	}
}

// ParseTemplate parses and validates an IaC template.
func (m *Manager) ParseTemplate(template *IaCTemplate) error {
	if template.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Content == "" {
		return fmt.Errorf("template content is required")
	}

	// Validate template content (simplified validation)
	if len(template.Content) < 10 {
		return fmt.Errorf("template content too short")
	}

	now := time.Now()
	template.CreatedAt = now
	template.UpdatedAt = now
	if template.Version == "" {
		template.Version = "1.0.0"
	}

	m.tmplMu.Lock()
	m.templates[template.ID] = template
	m.tmplMu.Unlock()

	m.logger.Info("template parsed",
		zap.String("template_id", template.ID),
		zap.String("name", template.Name))

	return nil
}

// DeployStack deploys a new stack from a template.
func (m *Manager) DeployStack(ctx context.Context, req DeployStackRequest) (*Stack, error) {
	m.tmplMu.RLock()
	tmpl, exists := m.templates[req.TemplateID]
	m.tmplMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template %s not found", req.TemplateID)
	}

	stackID := fmt.Sprintf("stack-%s-%d", req.Name, time.Now().UnixNano())
	now := time.Now()

	stack := &Stack{
		ID:         stackID,
		Name:       req.Name,
		TemplateID: req.TemplateID,
		Status:     StackStatusCreating,
		Variables:  req.Variables,
		Resources:  make([]Resource, 0),
		Outputs:    make(map[string]string),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Store stack
	m.stackMu.Lock()
	m.stacks[stackID] = stack
	m.stackMu.Unlock()

	// Simulate deployment
	go func() {
		time.Sleep(500 * time.Millisecond)

		// Create resources
		resources := []Resource{
			{
				ID:      fmt.Sprintf("res-%s-vol-1", stackID),
				StackID: stackID,
				Kind:    "volume",
				Name:    fmt.Sprintf("%s-data", req.Name),
				Status:  ResourceStatusActive,
				Config:  map[string]string{"size": "100GB", "type": "ssd"},
			},
			{
				ID:      fmt.Sprintf("res-%s-share-1", stackID),
				StackID: stackID,
				Kind:    "share",
				Name:    fmt.Sprintf("%s-share", req.Name),
				Status:  ResourceStatusActive,
				Config:  map[string]string{"path": fmt.Sprintf("/volume1/%s", req.Name)},
			},
			{
				ID:      fmt.Sprintf("res-%s-net-1", stackID),
				StackID: stackID,
				Kind:    "network",
				Name:    fmt.Sprintf("%s-network", req.Name),
				Status:  ResourceStatusActive,
				Config:  map[string]string{"subnet": "172.20.0.0/16"},
			},
		}

		// Store resources
		m.resMu.Lock()
		for i := range resources {
			resources[i].CreatedAt = now
			resources[i].UpdatedAt = time.Now()
			m.resources[resources[i].ID] = &resources[i]
		}
		m.resMu.Unlock()

		// Update stack
		m.stackMu.Lock()
		stack.Resources = resources
		stack.Status = StackStatusActive
		stack.Outputs = map[string]string{
			"volume_path": fmt.Sprintf("/volume1/%s", req.Name),
			"share_name":  fmt.Sprintf("%s-share", req.Name),
		}
		deployedAt := time.Now()
		stack.DeployedAt = &deployedAt
		stack.UpdatedAt = deployedAt
		m.stackMu.Unlock()

		m.logger.Info("stack deployed",
			zap.String("stack_id", stackID),
			zap.String("name", req.Name),
			zap.Int("resources", len(resources)),
			zap.String("template", tmpl.Name))
	}()

	return stack, nil
}

// DetectDrift detects configuration drift for a stack.
func (m *Manager) DetectDrift(ctx context.Context, stackID string) (*DriftReport, error) {
	m.stackMu.RLock()
	stack, exists := m.stacks[stackID]
	m.stackMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stack %s not found", stackID)
	}

	reportID := fmt.Sprintf("drift-%s-%d", stackID, time.Now().UnixNano())
	now := time.Now()

	report := &DriftReport{
		ID:        reportID,
		StackID:   stackID,
		StackName: stack.Name,
		CheckedAt: now,
		HasDrift:  false,
		Drifts:    make([]ResourceDrift, 0),
	}

	// Check each resource for drift
	m.resMu.RLock()
	var totalResources, driftedResources int
	for _, res := range m.resources {
		if res.StackID != stackID {
			continue
		}
		totalResources++

		// Simulate drift detection
		drift := ResourceDrift{
			ResourceID:   res.ID,
			ResourceName: res.Name,
			ResourceKind: res.Kind,
			Status:       DriftStatusNone,
		}

		// Simulate some resources having drift
		if res.Kind == "share" {
			drift.Status = DriftStatusModified
			drift.Expected = res.Config["path"]
			drift.Actual = res.Config["path"] + "/modified"
			drift.Diff = "  - path: /volume1/test\n  + path: /volume1/test/modified"
			report.HasDrift = true
			driftedResources++
		}

		report.Drifts = append(report.Drifts, drift)
	}
	m.resMu.RUnlock()

	report.Summary = DriftSummary{
		TotalResources:     totalResources,
		DriftedResources:   driftedResources,
		UnchangedResources: totalResources - driftedResources,
	}

	// Store drift report
	m.driftMu.Lock()
	m.drifts[reportID] = report
	m.driftMu.Unlock()

	// Update stack drift status
	if report.HasDrift {
		m.stackMu.Lock()
		stack.DriftStatus = DriftStatusDrifted
		stack.Status = StackStatusDrifted
		stack.UpdatedAt = now
		m.stackMu.Unlock()
	}

	m.logger.Info("drift detection completed",
		zap.String("stack_id", stackID),
		zap.Bool("has_drift", report.HasDrift),
		zap.Int("drifted_resources", driftedResources))

	return report, nil
}

// DestroyStack destroys a stack and all its resources.
func (m *Manager) DestroyStack(ctx context.Context, stackID string) error {
	m.stackMu.RLock()
	stack, exists := m.stacks[stackID]
	m.stackMu.RUnlock()

	if !exists {
		return fmt.Errorf("stack %s not found", stackID)
	}

	// Update status
	m.stackMu.Lock()
	stack.Status = StackStatusDeleting
	stack.UpdatedAt = time.Now()
	m.stackMu.Unlock()

	// Simulate deletion
	go func() {
		time.Sleep(300 * time.Millisecond)

		// Delete resources
		m.resMu.Lock()
		for id, res := range m.resources {
			if res.StackID == stackID {
				delete(m.resources, id)
			}
		}
		m.resMu.Unlock()

		// Update stack
		m.stackMu.Lock()
		now := time.Now()
		stack.Status = StackStatusActive // Mark as deleted
		stack.Resources = nil
		stack.DestroyedAt = &now
		stack.UpdatedAt = now
		m.stackMu.Unlock()

		m.logger.Info("stack destroyed",
			zap.String("stack_id", stackID),
			zap.String("name", stack.Name))
	}()

	return nil
}

// ListStacks returns all stacks.
func (m *Manager) ListStacks() []*Stack {
	m.stackMu.RLock()
	defer m.stackMu.RUnlock()

	stacks := make([]*Stack, 0, len(m.stacks))
	for _, s := range m.stacks {
		stacks = append(stacks, s)
	}
	return stacks
}

// GetStack returns a stack by ID.
func (m *Manager) GetStack(id string) *Stack {
	m.stackMu.RLock()
	defer m.stackMu.RUnlock()
	return m.stacks[id]
}

// DeleteStack removes a stack record (does not destroy resources).
func (m *Manager) DeleteStack(id string) bool {
	m.stackMu.Lock()
	defer m.stackMu.Unlock()

	if _, exists := m.stacks[id]; !exists {
		return false
	}
	delete(m.stacks, id)
	return true
}

// GetTemplate returns a template by ID.
func (m *Manager) GetTemplate(id string) *IaCTemplate {
	m.tmplMu.RLock()
	defer m.tmplMu.RUnlock()
	return m.templates[id]
}

// ListTemplates returns all templates.
func (m *Manager) ListTemplates() []*IaCTemplate {
	m.tmplMu.RLock()
	defer m.tmplMu.RUnlock()

	templates := make([]*IaCTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}
	return templates
}

// DeleteTemplate deletes a template.
func (m *Manager) DeleteTemplate(id string) bool {
	m.tmplMu.Lock()
	defer m.tmplMu.Unlock()

	if _, exists := m.templates[id]; !exists {
		return false
	}
	delete(m.templates, id)
	return true
}

// GetDriftReport returns a drift report by ID.
func (m *Manager) GetDriftReport(id string) *DriftReport {
	m.driftMu.RLock()
	defer m.driftMu.RUnlock()
	return m.drifts[id]
}

// ListDriftReports returns all drift reports.
func (m *Manager) ListDriftReports() []*DriftReport {
	m.driftMu.RLock()
	defer m.driftMu.RUnlock()

	reports := make([]*DriftReport, 0, len(m.drifts))
	for _, r := range m.drifts {
		reports = append(reports, r)
	}
	return reports
}

// GetResource returns a resource by ID.
func (m *Manager) GetResource(id string) *Resource {
	m.resMu.RLock()
	defer m.resMu.RUnlock()
	return m.resources[id]
}

// ListResources returns all resources for a stack.
func (m *Manager) ListResources(stackID string) []*Resource {
	m.resMu.RLock()
	defer m.resMu.RUnlock()

	resources := make([]*Resource, 0)
	for _, r := range m.resources {
		if r.StackID == stackID {
			resources = append(resources, r)
		}
	}
	return resources
}
