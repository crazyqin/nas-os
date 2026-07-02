package dockercompose

import (
	"fmt"
	"strings"
)

// ComposeFile 表示解析后的 docker-compose.yml 内容.
type ComposeFile struct {
	Version  string                 `yaml:"version"  json:"version"`
	Services map[string]CompService `yaml:"services" json:"services"`
	Networks map[string]CompNetwork `yaml:"networks" json:"networks"`
	Volumes  map[string]CompVolume  `yaml:"volumes"  json:"volumes"`
}

// CompService Compose 文件中的服务定义.
type CompService struct {
	Image         string            `yaml:"image"          json:"image"`
	Ports         []string          `yaml:"ports"          json:"ports"`
	Volumes       []string          `yaml:"volumes"        json:"volumes"`
	Environment   map[string]string `yaml:"environment"    json:"environment"`
	DependsOn     []string          `yaml:"depends_on"     json:"depends_on"`
	Restart       string            `yaml:"restart"        json:"restart"`
	Command       interface{}       `yaml:"command"        json:"command"`
	Networks      []string          `yaml:"networks"       json:"networks"`
	HealthCheck   *CompHealthCheck  `yaml:"healthcheck"    json:"healthcheck"`
	Deploy        *CompDeploy       `yaml:"deploy"         json:"deploy"`
	Build         interface{}       `yaml:"build"          json:"build"`
	ContainerName string            `yaml:"container_name" json:"container_name"`
}

// CompHealthCheck Compose 文件中的健康检查.
type CompHealthCheck struct {
	Test        interface{} `yaml:"test"         json:"test"`
	Interval    string      `yaml:"interval"     json:"interval"`
	Timeout     string      `yaml:"timeout"      json:"timeout"`
	Retries     int         `yaml:"retries"      json:"retries"`
	StartPeriod string      `yaml:"start_period" json:"start_period"`
}

// CompDeploy Compose 文件中的部署配置.
type CompDeploy struct {
	Replicas  int          `yaml:"replicas"  json:"replicas"`
	Resources *CompResSpec `yaml:"resources" json:"resources"`
}

// CompResSpec 资源规格.
type CompResSpec struct {
	Limits  *CompRes `yaml:"limits"   json:"limits"`
	Reserve *CompRes `yaml:"reservations" json:"reservations"`
}

// CompRes 资源.
type CompRes struct {
	CPUs   string `yaml:"cpus"   json:"cpus"`
	Memory string `yaml:"memory" json:"memory"`
}

// CompNetwork Compose 文件中的网络.
type CompNetwork struct {
	Driver   string            `yaml:"driver"     json:"driver"`
	External bool              `yaml:"external"   json:"external"`
	Labels   map[string]string `yaml:"labels"     json:"labels"`
	IPAM     *CompIPAM         `yaml:"ipam"       json:"ipam"`
}

// CompIPAM IP 地址管理.
type CompIPAM struct {
	Driver string       `yaml:"driver" json:"driver"`
	Config []CompSubnet `yaml:"config" json:"config"`
}

// CompSubnet 子网配置.
type CompSubnet struct {
	Subnet string `yaml:"subnet" json:"subnet"`
}

// CompVolume Compose 文件中的卷.
type CompVolume struct {
	Driver   string            `yaml:"driver"     json:"driver"`
	External bool              `yaml:"external"   json:"external"`
	Labels   map[string]string `yaml:"labels"     json:"labels"`
}

// ValidationResult 验证结果.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// ValidationError 验证错误.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ParseComposeFile 从 map 数据解析 Compose 文件.
func ParseComposeFile(data map[string]interface{}) (*ComposeFile, error) {
	if data == nil {
		return nil, fmt.Errorf("compose 数据不能为空")
	}

	cf := &ComposeFile{
		Services: make(map[string]CompService),
		Networks: make(map[string]CompNetwork),
		Volumes:  make(map[string]CompVolume),
	}

	if v, ok := data["version"].(string); ok {
		cf.Version = v
	}

	// 解析 services
	if services, ok := data["services"].(map[string]interface{}); ok {
		for name, svc := range services {
			svcMap, ok := svc.(map[string]interface{})
			if !ok {
				continue
			}
			cs := CompService{}
			if img, ok := svcMap["image"].(string); ok {
				cs.Image = img
			}
			if ports, ok := svcMap["ports"].([]interface{}); ok {
				for _, p := range ports {
					if s, ok := p.(string); ok {
						cs.Ports = append(cs.Ports, s)
					}
				}
			}
			if vols, ok := svcMap["volumes"].([]interface{}); ok {
				for _, v := range vols {
					if s, ok := v.(string); ok {
						cs.Volumes = append(cs.Volumes, s)
					}
				}
			}
			if env, ok := svcMap["environment"].(map[string]interface{}); ok {
				cs.Environment = make(map[string]string)
				for k, val := range env {
					if s, ok := val.(string); ok {
						cs.Environment[k] = s
					}
				}
			}
			if deps, ok := svcMap["depends_on"].([]interface{}); ok {
				for _, d := range deps {
					if s, ok := d.(string); ok {
						cs.DependsOn = append(cs.DependsOn, s)
					}
				}
			}
			if r, ok := svcMap["restart"].(string); ok {
				cs.Restart = r
			}
			if nets, ok := svcMap["networks"].([]interface{}); ok {
				for _, n := range nets {
					if s, ok := n.(string); ok {
						cs.Networks = append(cs.Networks, s)
					}
				}
			}
			cf.Services[name] = cs
		}
	}

	// 解析 networks
	if networks, ok := data["networks"].(map[string]interface{}); ok {
		for name, net := range networks {
			netMap, ok := net.(map[string]interface{})
			if !ok {
				continue
			}
			cn := CompNetwork{}
			if d, ok := netMap["driver"].(string); ok {
				cn.Driver = d
			}
			if e, ok := netMap["external"].(bool); ok {
				cn.External = e
			}
			cf.Networks[name] = cn
		}
	}

	// 解析 volumes
	if volumes, ok := data["volumes"].(map[string]interface{}); ok {
		for name, vol := range volumes {
			volMap, ok := vol.(map[string]interface{})
			if !ok {
				continue
			}
			cv := CompVolume{}
			if d, ok := volMap["driver"].(string); ok {
				cv.Driver = d
			}
			if e, ok := volMap["external"].(bool); ok {
				cv.External = e
			}
			cf.Volumes[name] = cv
		}
	}

	return cf, nil
}

// ValidateComposeFile 验证 Compose 文件.
func ValidateComposeFile(cf *ComposeFile) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if cf == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "root",
			Message: "compose 文件为空",
		})
		return result
	}

	if len(cf.Services) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "services",
			Message: "至少需要一个服务定义",
		})
	}

	for name, svc := range cf.Services {
		// 镜像或 build 至少一个
		if svc.Image == "" && svc.Build == nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("services.%s", name),
				Message: "必须指定 image 或 build",
			})
		}

		// 检查依赖是否存在
		for _, dep := range svc.DependsOn {
			if _, exists := cf.Services[dep]; !exists {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("services.%s.depends_on", name),
					Message: fmt.Sprintf("依赖的服务 %s 不存在", dep),
				})
			}
		}

		// 检查网络是否存在
		for _, netName := range svc.Networks {
			if _, exists := cf.Networks[netName]; !exists {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("services.%s.networks", name),
					Message: fmt.Sprintf("网络 %s 未在顶层 networks 中定义", netName),
				})
			}
		}

		// 端口格式校验
		for _, port := range svc.Ports {
			if !validatePortMapping(port) {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("services.%s.ports", name),
					Message: fmt.Sprintf("端口映射格式可能不正确: %s", port),
				})
			}
		}
	}

	// 检查循环依赖
	if hasCyclicDependency(cf.Services) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "services",
			Message: "检测到循环依赖",
		})
	}

	return result
}

// validatePortMapping 校验端口映射格式.
func validatePortMapping(port string) bool {
	// 支持 "8080:80", "8080:80/tcp", "8080:80/udp", "80"
	parts := strings.Split(port, ":")
	switch len(parts) {
	case 1:
		return true // "80"
	case 2:
		return true // "8080:80"
	default:
		return false
	}
}

// hasCyclicDependency 检测循环依赖.
func hasCyclicDependency(services map[string]CompService) bool {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var dfs func(name string) bool
	dfs = func(name string) bool {
		if inStack[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visited[name] = true
		inStack[name] = true
		for _, dep := range services[name].DependsOn {
			if dfs(dep) {
				return true
			}
		}
		inStack[name] = false
		return false
	}

	for name := range services {
		if dfs(name) {
			return true
		}
	}
	return false
}
