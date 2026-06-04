package composevisual

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		projects:  make(map[string]*ComposeProject),
		templates: make(map[string]*ComposeTemplate),
	}
	m.initTemplates()
	return m
}

// ListProjects 列出所有项目
func (m *Manager) ListProjects() []*ComposeProject {
	m.mu.RLock()
	defer m.mu.RUnlock()
	projects := make([]*ComposeProject, 0, len(m.projects))
	for _, p := range m.projects {
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt.After(projects[j].UpdatedAt)
	})
	return projects
}

// CreateProject 创建项目
func (m *Manager) CreateProject(req *CreateProjectRequest) *ComposeProject {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	project := &ComposeProject{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Services:    make(map[string]*ServiceNode),
		Networks:    make(map[string]*NetworkConfig),
		Volumes:     make(map[string]*VolumeConfig),
		EnvVars:     req.EnvVars,
		Layout: &VisualLayout{
			CanvasWidth:  1200,
			CanvasHeight: 800,
			Nodes:        make(map[string]*NodePosition),
			Connections:  make([]Connection, 0),
		},
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      req.Tags,
	}
	if project.EnvVars == nil {
		project.EnvVars = make(map[string]string)
	}
	m.projects[project.ID] = project
	return project
}

// GetProject 获取项目
func (m *Manager) GetProject(id string) (*ComposeProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, exists := m.projects[id]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", id)
	}
	return project, nil
}

// UpdateProject 更新项目
func (m *Manager) UpdateProject(id string, req *UpdateProjectRequest) (*ComposeProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[id]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", id)
	}
	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.EnvVars != nil {
		project.EnvVars = req.EnvVars
	}
	if req.Tags != nil {
		project.Tags = req.Tags
	}
	project.UpdatedAt = time.Now()
	return project, nil
}

// DeleteProject 删除项目
func (m *Manager) DeleteProject(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[id]; !exists {
		return fmt.Errorf("项目 %s 不存在", id)
	}
	delete(m.projects, id)
	return nil
}

// AddService 添加服务
func (m *Manager) AddService(projectID string, req *AddServiceRequest) (*ServiceNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", projectID)
	}
	service := &ServiceNode{
		ID:            uuid.New().String(),
		Name:          req.Name,
		ContainerName: req.ContainerName,
		Image:         req.Image,
		Ports:         req.Ports,
		Volumes:       req.Volumes,
		Environment:   req.Environment,
		DependsOn:     req.DependsOn,
		Command:       req.Command,
		EntryPoint:    req.EntryPoint,
		WorkingDir:    req.WorkingDir,
		Restart:       req.Restart,
		Resources:     SuggestResources(req.Image),
		Position:      CalculateNodePosition(len(project.Services)),
		Status:        ServiceDraft,
	}
	if service.ContainerName == "" {
		service.ContainerName = req.Name
	}
	if service.Restart == "" {
		service.Restart = "unless-stopped"
	}
	if service.Ports == nil {
		service.Ports = make([]PortMapping, 0)
	}
	if service.Volumes == nil {
		service.Volumes = make([]VolumeMapping, 0)
	}
	if service.Environment == nil {
		service.Environment = make(map[string]string)
	}
	if service.DependsOn == nil {
		service.DependsOn = make([]string, 0)
	}
	project.Services[req.Name] = service
	project.Layout.Nodes[req.Name] = service.Position
	project.UpdatedAt = time.Now()
	if project.Status == StatusDraft {
		project.Status = StatusReady
	}
	return service, nil
}

// UpdateService 更新服务
func (m *Manager) UpdateService(projectID, serviceName string, req *UpdateServiceRequest) (*ServiceNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", projectID)
	}
	service, exists := project.Services[serviceName]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", serviceName)
	}
	if req.Name != "" {
		service.Name = req.Name
	}
	if req.Image != "" {
		service.Image = req.Image
	}
	if req.ContainerName != "" {
		service.ContainerName = req.ContainerName
	}
	if req.Ports != nil {
		service.Ports = req.Ports
	}
	if req.Volumes != nil {
		service.Volumes = req.Volumes
	}
	if req.Environment != nil {
		service.Environment = req.Environment
	}
	if req.DependsOn != nil {
		service.DependsOn = req.DependsOn
	}
	if req.Command != nil {
		service.Command = req.Command
	}
	if req.EntryPoint != nil {
		service.EntryPoint = req.EntryPoint
	}
	if req.WorkingDir != "" {
		service.WorkingDir = req.WorkingDir
	}
	if req.Restart != "" {
		service.Restart = req.Restart
	}
	if req.Resources != nil {
		service.Resources = req.Resources
	}
	if req.HealthCheck != nil {
		service.HealthCheck = req.HealthCheck
	}
	project.UpdatedAt = time.Now()
	return service, nil
}

// DeleteService 删除服务
func (m *Manager) DeleteService(projectID, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[projectID]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectID)
	}
	if _, exists := project.Services[serviceName]; !exists {
		return fmt.Errorf("服务 %s 不存在", serviceName)
	}
	delete(project.Services, serviceName)
	delete(project.Layout.Nodes, serviceName)
	// 清理依赖关系
	for _, svc := range project.Services {
		newDeps := make([]string, 0, len(svc.DependsOn))
		for _, dep := range svc.DependsOn {
			if dep != serviceName {
				newDeps = append(newDeps, dep)
			}
		}
		svc.DependsOn = newDeps
	}
	// 清理连接
	newConns := make([]Connection, 0, len(project.Layout.Connections))
	for _, conn := range project.Layout.Connections {
		if conn.From != serviceName && conn.To != serviceName {
			newConns = append(newConns, conn)
		}
	}
	project.Layout.Connections = newConns
	project.UpdatedAt = time.Now()
	return nil
}

// ConnectServices 连接服务
func (m *Manager) ConnectServices(projectID string, req *ConnectServicesRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[projectID]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectID)
	}
	if _, exists := project.Services[req.From]; !exists {
		return fmt.Errorf("服务 %s 不存在", req.From)
	}
	if _, exists := project.Services[req.To]; !exists {
		return fmt.Errorf("服务 %s 不存在", req.To)
	}
	connType := req.Type
	if connType == "" {
		connType = "depends"
	}
	connection := Connection{
		From: req.From, To: req.To, Type: connType, Animated: connType == "depends",
	}
	project.Layout.Connections = append(project.Layout.Connections, connection)
	if connType == "depends" {
		targetSvc := project.Services[req.To]
		if targetSvc != nil {
			found := false
			for _, dep := range targetSvc.DependsOn {
				if dep == req.From {
					found = true
					break
				}
			}
			if !found {
				targetSvc.DependsOn = append(targetSvc.DependsOn, req.From)
			}
		}
	}
	project.UpdatedAt = time.Now()
	return nil
}
