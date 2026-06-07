package composevisual

import (
	"fmt"
	"sort"
	"strings"
)

// ExportCompose 导出为 docker-compose.yml
func (m *Manager) ExportCompose(projectID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, exists := m.projects[projectID]
	if !exists {
		return "", fmt.Errorf("项目 %s 不存在", projectID)
	}
	var sb strings.Builder
	sb.WriteString("version: '3.8'\n\n")
	sb.WriteString("services:\n")
	svcNames := make([]string, 0, len(project.Services))
	for name := range project.Services {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := project.Services[name]
		sb.WriteString(fmt.Sprintf("  %s:\n", name))
		sb.WriteString(fmt.Sprintf("    image: %s\n", svc.Image))
		if svc.ContainerName != "" {
			sb.WriteString(fmt.Sprintf("    container_name: %s\n", svc.ContainerName))
		}
		if len(svc.Ports) > 0 {
			sb.WriteString("    ports:\n")
			for _, p := range svc.Ports {
				proto := p.Protocol
				if proto == "" {
					proto = "tcp"
				}
				if p.IP != "" {
					sb.WriteString(fmt.Sprintf("      - \"%s:%d:%d/%s\"\n", p.IP, p.HostPort, p.ContainerPort, proto))
				} else {
					sb.WriteString(fmt.Sprintf("      - \"%d:%d/%s\"\n", p.HostPort, p.ContainerPort, proto))
				}
			}
		}
		if len(svc.Volumes) > 0 {
			sb.WriteString("    volumes:\n")
			for _, v := range svc.Volumes {
				ro := ""
				if v.ReadOnly {
					ro = ":ro"
				}
				sb.WriteString(fmt.Sprintf("      - %s:%s%s\n", v.Source, v.Target, ro))
			}
		}
		if len(svc.Environment) > 0 {
			sb.WriteString("    environment:\n")
			keys := make([]string, 0, len(svc.Environment))
			for k := range svc.Environment {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				sb.WriteString(fmt.Sprintf("      - %s=%s\n", k, svc.Environment[k]))
			}
		}
		if len(svc.DependsOn) > 0 {
			sb.WriteString("    depends_on:\n")
			for _, dep := range svc.DependsOn {
				sb.WriteString(fmt.Sprintf("      - %s\n", dep))
			}
		}
		if len(svc.Command) > 0 {
			sb.WriteString("    command:\n")
			for _, c := range svc.Command {
				sb.WriteString(fmt.Sprintf("      - \"%s\"\n", c))
			}
		}
		if svc.Restart != "" {
			sb.WriteString(fmt.Sprintf("    restart: %s\n", svc.Restart))
		}
		if svc.Resources != nil {
			sb.WriteString("    deploy:\n      resources:\n")
			if svc.Resources.CPUs != "" || svc.Resources.Memory != "" {
				sb.WriteString("        limits:\n")
				if svc.Resources.CPUs != "" {
					sb.WriteString(fmt.Sprintf("          cpus: '%s'\n", svc.Resources.CPUs))
				}
				if svc.Resources.Memory != "" {
					sb.WriteString(fmt.Sprintf("          memory: %s\n", svc.Resources.Memory))
				}
			}
		}
		if svc.HealthCheck != nil {
			sb.WriteString("    healthcheck:\n      test:\n")
			for _, t := range svc.HealthCheck.Test {
				sb.WriteString(fmt.Sprintf("        - \"%s\"\n", t))
			}
			if svc.HealthCheck.Interval != "" {
				sb.WriteString(fmt.Sprintf("      interval: %s\n", svc.HealthCheck.Interval))
			}
			if svc.HealthCheck.Timeout != "" {
				sb.WriteString(fmt.Sprintf("      timeout: %s\n", svc.HealthCheck.Timeout))
			}
			if svc.HealthCheck.Retries > 0 {
				sb.WriteString(fmt.Sprintf("      retries: %d\n", svc.HealthCheck.Retries))
			}
			if svc.HealthCheck.StartPeriod != "" {
				sb.WriteString(fmt.Sprintf("      start_period: %s\n", svc.HealthCheck.StartPeriod))
			}
		}
		sb.WriteString("\n")
	}
	if len(project.Networks) > 0 {
		sb.WriteString("networks:\n")
		netNames := make([]string, 0, len(project.Networks))
		for n := range project.Networks {
			netNames = append(netNames, n)
		}
		sort.Strings(netNames)
		for _, name := range netNames {
			net := project.Networks[name]
			sb.WriteString(fmt.Sprintf("  %s:\n", name))
			if net.Driver != "" {
				sb.WriteString(fmt.Sprintf("    driver: %s\n", net.Driver))
			}
			if net.External {
				sb.WriteString("    external: true\n")
			}
		}
		sb.WriteString("\n")
	}
	if len(project.Volumes) > 0 {
		sb.WriteString("volumes:\n")
		volNames := make([]string, 0, len(project.Volumes))
		for v := range project.Volumes {
			volNames = append(volNames, v)
		}
		sort.Strings(volNames)
		for _, name := range volNames {
			vol := project.Volumes[name]
			sb.WriteString(fmt.Sprintf("  %s:\n", name))
			if vol.Driver != "" {
				sb.WriteString(fmt.Sprintf("    driver: %s\n", vol.Driver))
			}
			if vol.External {
				sb.WriteString("    external: true\n")
			}
		}
	}
	return sb.String(), nil
}

// ImportCompose 导入 docker-compose.yml
func (m *Manager) ImportCompose(content, name string) (*ComposeProject, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("内容为空")
	}
	project := &ComposeProject{
		ID:          GenerateUUID(),
		Name:        name,
		Description: "从 docker-compose.yml 导入",
		Services:    make(map[string]*ServiceNode),
		Networks:    make(map[string]*NetworkConfig),
		Volumes:     make(map[string]*VolumeConfig),
		EnvVars:     make(map[string]string),
		Layout: &VisualLayout{
			CanvasWidth:  1200,
			CanvasHeight: 800,
			Nodes:        make(map[string]*NodePosition),
			Connections:  make([]Connection, 0),
		},
		Status:    StatusDraft,
		CreatedAt: Now(),
		UpdatedAt: Now(),
	}
	if name == "" {
		project.Name = "导入的项目"
	}

	inServices := false
	currentService := ""
	inPorts, inVolumes, inEnvironment, inDependsOn := false, false, false, false
	svcIdx := 0

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		c := strings.TrimSpace(trimmed)
		if c == "" || (len(c) > 0 && c[0] == '#') {
			continue
		}
		indent := len(trimmed) - len(strings.TrimLeft(trimmed, " "))
		if indent == 0 {
			inPorts, inVolumes, inEnvironment, inDependsOn = false, false, false, false
			if c == "services:" {
				inServices = true
				continue
			}
			if c == "networks:" || c == "volumes:" || strings.HasPrefix(c, "version:") {
				inServices = false
			}
			continue
		}
		if !inServices {
			continue
		}
		// 服务名（2格缩进）
		if indent == 2 && strings.HasSuffix(c, ":") && !strings.Contains(c, " ") {
			currentService = strings.TrimSuffix(c, ":")
			project.Services[currentService] = &ServiceNode{
				ID:          GenerateUUID(),
				Name:        currentService,
				Ports:       make([]PortMapping, 0),
				Volumes:     make([]VolumeMapping, 0),
				Environment: make(map[string]string),
				DependsOn:   make([]string, 0),
				Restart:     "unless-stopped",
				Position:    CalculateNodePosition(svcIdx),
				Status:      ServiceDraft,
			}
			project.Layout.Nodes[currentService] = project.Services[currentService].Position
			svcIdx++
			inPorts, inVolumes, inEnvironment, inDependsOn = false, false, false, false
			continue
		}
		if currentService == "" {
			continue
		}
		svc := project.Services[currentService]
		// 服务属性（4格缩进）
		if indent == 4 {
			inPorts, inVolumes, inEnvironment, inDependsOn = false, false, false, false
			switch {
			case strings.HasPrefix(c, "image:"):
				svc.Image = strings.Trim(strings.TrimSpace(strings.TrimPrefix(c, "image:")), "\"'")
			case strings.HasPrefix(c, "container_name:"):
				svc.ContainerName = strings.Trim(strings.TrimSpace(strings.TrimPrefix(c, "container_name:")), "\"'")
			case strings.HasPrefix(c, "restart:"):
				svc.Restart = strings.Trim(strings.TrimSpace(strings.TrimPrefix(c, "restart:")), "\"'")
			case c == "ports:":
				inPorts = true
			case c == "volumes:":
				inVolumes = true
			case c == "environment:":
				inEnvironment = true
			case c == "depends_on:":
				inDependsOn = true
			case strings.HasPrefix(c, "command:"):
				cmdVal := strings.TrimSpace(strings.TrimPrefix(c, "command:"))
				cmdVal = strings.Trim(cmdVal, "\"'")
				if cmdVal != "" && !strings.HasSuffix(cmdVal, ":") {
					svc.Command = []string{cmdVal}
				}
			}
			continue
		}
		// 列表项（6+格缩进）
		if indent >= 6 {
			val := strings.TrimSpace(strings.TrimPrefix(c, "- "))
			switch {
			case inPorts:
				pm := ParsePortMapping(val)
				if pm != nil {
					svc.Ports = append(svc.Ports, *pm)
				}
			case inVolumes:
				vm := ParseVolumeMapping(val)
				if vm != nil {
					svc.Volumes = append(svc.Volumes, *vm)
				}
			case inEnvironment:
				parts := strings.SplitN(val, "=", 2)
				if len(parts) == 2 {
					svc.Environment[strings.Trim(parts[0], "\"'")] = strings.Trim(parts[1], "\"'")
				}
			case inDependsOn:
				dep := strings.Trim(val, "\"'")
				if dep != "" {
					svc.DependsOn = append(svc.DependsOn, dep)
				}
			}
		}
	}
	// 从依赖关系生成连接
	for _, svc := range project.Services {
		for _, dep := range svc.DependsOn {
			project.Layout.Connections = append(project.Layout.Connections, Connection{
				From: dep, To: svc.Name, Type: "depends", Animated: true,
			})
		}
	}
	m.mu.Lock()
	m.projects[project.ID] = project
	m.mu.Unlock()
	return project, nil
}

// GenerateTopology 生成拓扑图
func (m *Manager) GenerateTopology(projectID string) (*TopologyData, [][]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, exists := m.projects[projectID]
	if !exists {
		return nil, nil, fmt.Errorf("项目 %s 不存在", projectID)
	}
	topology := &TopologyData{
		Nodes:  make([]TopologyNode, 0),
		Edges:  make([]TopologyEdge, 0),
		Groups: make([]TopologyGroup, 0),
		Layers: []string{"入口层", "应用层", "数据层", "基础层"},
	}
	tierMap := ClassifyServices(project.Services)
	for name, svc := range project.Services {
		tier := tierMap[name]
		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID: name, Label: svc.Name, Type: ClassifyServiceType(svc.Image),
			Image: svc.Image, Status: svc.Status,
			Position: NodePosition{X: svc.Position.X, Y: svc.Position.Y, Width: svc.Position.Width, Height: svc.Position.Height},
			Ports:    svc.Ports, Tier: tier,
		})
	}
	edgeID := 0
	edgeSet := make(map[string]bool)
	for _, svc := range project.Services {
		for _, dep := range svc.DependsOn {
			key := dep + "->" + svc.Name
			if !edgeSet[key] {
				topology.Edges = append(topology.Edges, TopologyEdge{
					ID: fmt.Sprintf("edge-%d", edgeID), Source: dep, Target: svc.Name, Type: "depends",
				})
				edgeSet[key] = true
				edgeID++
			}
		}
	}
	for _, conn := range project.Layout.Connections {
		key := conn.From + "->" + conn.To
		if !edgeSet[key] {
			topology.Edges = append(topology.Edges, TopologyEdge{
				ID: fmt.Sprintf("edge-%d", edgeID), Source: conn.From, Target: conn.To, Type: conn.Type,
			})
			edgeSet[key] = true
			edgeID++
		}
	}
	startOrder := CalculateStartOrder(project.Services)
	tierGroups := make(map[int][]string)
	for name, tier := range tierMap {
		tierGroups[tier] = append(tierGroups[tier], name)
	}
	tierNames := []string{"入口层", "应用层", "数据层", "基础层"}
	for tier, nodes := range tierGroups {
		if tier < len(tierNames) {
			topology.Groups = append(topology.Groups, TopologyGroup{
				ID: fmt.Sprintf("group-%d", tier), Label: tierNames[tier], NodeIDs: nodes,
			})
		}
	}
	return topology, startOrder, nil
}

// ListTemplates 列出模板
func (m *Manager) ListTemplates(query, category string, minRating float64, sortBy string) []*ComposeTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	templates := make([]*ComposeTemplate, 0)
	for _, t := range m.templates {
		if category != "" && category != "all" && t.Category != category {
			continue
		}
		if minRating > 0 && t.Rating < minRating {
			continue
		}
		if query != "" {
			q := strings.ToLower(query)
			if !strings.Contains(strings.ToLower(t.Name), q) &&
				!strings.Contains(strings.ToLower(t.Description), q) &&
				!strings.Contains(strings.ToLower(strings.Join(t.Tags, " ")), q) {
				continue
			}
		}
		templates = append(templates, t)
	}
	switch sortBy {
	case "downloads":
		sort.Slice(templates, func(i, j int) bool { return templates[i].Downloads > templates[j].Downloads })
	case "name":
		sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	default:
		sort.Slice(templates, func(i, j int) bool { return templates[i].Rating > templates[j].Rating })
	}
	return templates
}

// InstantiateTemplate 从模板创建项目
func (m *Manager) InstantiateTemplate(templateID, projectName, description string, envVars map[string]string) (*ComposeProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tmpl, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}
	now := Now()
	project := &ComposeProject{
		ID: GenerateUUID(), Name: projectName, Description: description,
		Services: make(map[string]*ServiceNode), Networks: make(map[string]*NetworkConfig),
		Volumes: make(map[string]*VolumeConfig), EnvVars: envVars,
		Layout: &VisualLayout{
			CanvasWidth: 1200, CanvasHeight: 800,
			Nodes: make(map[string]*NodePosition), Connections: make([]Connection, 0),
		},
		Status: StatusDraft, CreatedAt: now, UpdatedAt: now,
	}
	if project.EnvVars == nil {
		project.EnvVars = make(map[string]string)
	}
	if project.Description == "" {
		project.Description = tmpl.Description
	}
	svcIdx := 0
	for name, tmplSvc := range tmpl.Services {
		svcCopy := *tmplSvc
		svcCopy.ID = GenerateUUID()
		svcCopy.Position = CalculateNodePosition(svcIdx)
		svcCopy.Status = ServiceDraft
		if svcCopy.Environment == nil {
			svcCopy.Environment = make(map[string]string)
		}
		if svcCopy.Ports == nil {
			svcCopy.Ports = make([]PortMapping, 0)
		}
		if svcCopy.Volumes == nil {
			svcCopy.Volumes = make([]VolumeMapping, 0)
		}
		if svcCopy.DependsOn == nil {
			svcCopy.DependsOn = make([]string, 0)
		}
		project.Services[name] = &svcCopy
		project.Layout.Nodes[name] = svcCopy.Position
		svcIdx++
	}
	for name, net := range tmpl.Networks {
		netCopy := *net
		project.Networks[name] = &netCopy
	}
	for name, vol := range tmpl.Volumes {
		volCopy := *vol
		project.Volumes[name] = &volCopy
	}
	for _, svc := range project.Services {
		for _, dep := range svc.DependsOn {
			project.Layout.Connections = append(project.Layout.Connections, Connection{
				From: dep, To: svc.Name, Type: "depends", Animated: true,
			})
		}
	}
	tmpl.Downloads++
	m.projects[project.ID] = project
	return project, nil
}

// Deploy 部署项目（模拟）
func (m *Manager) Deploy(projectID string) (*DeployResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", projectID)
	}
	if len(project.Services) == 0 {
		return nil, fmt.Errorf("项目没有服务，无法部署")
	}
	serviceNames := make([]string, 0, len(project.Services))
	for name := range project.Services {
		serviceNames = append(serviceNames, name)
	}
	now := Now()
	project.DeployedAt = &now
	project.Status = StatusDeployed
	return &DeployResult{
		ProjectID: projectID, Status: "success", Services: serviceNames,
		Output: fmt.Sprintf("成功部署 %d 个服务", len(serviceNames)),
	}, nil
}
