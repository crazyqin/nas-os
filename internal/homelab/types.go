// Package homelab 家庭实验室管理器 - 统一管理Docker/VM/服务
package homelab

import (
	"errors"
	"time"
)

// ServiceType 服务类型.
type ServiceType string

const (
	ServiceDocker  ServiceType = "docker"
	ServiceVM      ServiceType = "vm"
	ServiceCompose ServiceType = "compose"
	ServiceK3s     ServiceType = "k3s"
	ServiceSystemd ServiceType = "systemd"
)

// ServiceStatus 服务状态.
type ServiceStatus string

const (
	StatusRunning  ServiceStatus = "running"
	StatusStopped  ServiceStatus = "stopped"
	StatusError    ServiceStatus = "error"
	StatusStarting ServiceStatus = "starting"
	StatusStopping ServiceStatus = "stopping"
)

// Service 服务定义.
type Service struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         ServiceType       `json:"type"`
	Status       ServiceStatus     `json:"status"`
	Image        string            `json:"image,omitempty"`
	Port         int               `json:"port,omitempty"`
	Ports        []string          `json:"ports,omitempty"`
	Volumes      []string          `json:"volumes,omitempty"`
	Env          []string          `json:"env,omitempty"`
	CPUUsage     float64           `json:"cpu_usage"`
	MemUsage     int64             `json:"mem_usage"`
	MemLimit     int64             `json:"mem_limit"`
	NetRx        int64             `json:"net_rx"`
	NetTx        int64             `json:"net_tx"`
	Uptime       int64             `json:"uptime"`
	RestartCount int               `json:"restart_count"`
	HealthCheck  string            `json:"health_check,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// Stack 编排栈.
type Stack struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Services  []string          `json:"services"`
	Status    ServiceStatus     `json:"status"`
	Env       map[string]string `json:"env,omitempty"`
	Networks  []string          `json:"networks,omitempty"`
	Volumes   []string          `json:"volumes,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Template 服务模板.
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Image       string            `json:"image"`
	Ports       []string          `json:"ports"`
	Volumes     []string          `json:"volumes"`
	Env         map[string]string `json:"env"`
	Icon        string            `json:"icon,omitempty"`
	Downloads   int               `json:"downloads"`
	Rating      float64           `json:"rating"`
}

// Config 配置.
type Config struct {
	DockerHost     string `json:"docker_host"`
	K3sConfig      string `json:"k3s_config"`
	AutoRestart    bool   `json:"auto_restart"`
	HealthInterval int    `json:"health_interval"`
	MaxServices    int    `json:"max_services"`
}

// 预定义错误.
var (
	ErrServiceNotFound  = errors.New("service not found")
	ErrStackNotFound    = errors.New("stack not found")
	ErrServiceExists    = errors.New("service already exists")
	ErrMaxServices      = errors.New("max services reached")
	ErrInvalidType      = errors.New("invalid service type")
	ErrTemplateNotFound = errors.New("template not found")
)
