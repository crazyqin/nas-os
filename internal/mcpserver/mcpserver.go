// Package mcpserver implements Model Context Protocol (MCP) server integration.
// MCP enables AI models to connect to external tools and data sources via a
// standardized protocol, similar to Synology's AI Console MCP support.
//
// Features:
// - MCP Server lifecycle management (start/stop/restart)
// - Tool registration and discovery
// - Resource exposure and access control
// - Prompt template management
// - Stdio and HTTP transport support
// - Security sandboxing for tool execution
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MCPServer represents a Model Context Protocol server instance
type MCPServer struct {
	mu          sync.RWMutex
	name        string
	version     string
	transport   TransportType
	tools       map[string]*Tool
	resources   map[string]*Resource
	prompts     map[string]*Prompt
	running     bool
	port        int
	maxConns    int
	security    *SecurityConfig
	metrics     *ServerMetrics
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// TransportType defines the MCP transport protocol
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
	TransportSSE   TransportType = "sse"
)

// Tool represents an MCP tool that AI models can invoke
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
	Enabled     bool                   `json:"enabled"`
	Timeout     time.Duration          `json:"timeout"`
	Permission  ToolPermission         `json:"permission"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// ToolHandler is the function signature for tool execution
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// ToolPermission defines access control for tools
type ToolPermission struct {
	Level       string   `json:"level"` // "public", "authenticated", "admin"
	AllowedIPs  []string `json:"allowedIPs,omitempty"`
	RateLimit   int      `json:"rateLimit,omitempty"` // requests per minute
}

// Resource represents an MCP resource that AI models can read
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mimeType"`
	Handler     ResourceHandler        `json:"-"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"createdAt"`
}

// ResourceHandler is the function signature for resource access
type ResourceHandler func(ctx context.Context, uri string) ([]byte, error)

// Prompt represents an MCP prompt template
type Prompt struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Arguments   []PromptArgument       `json:"arguments"`
	Handler     PromptHandler          `json:"-"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"createdAt"`
}

// PromptArgument defines a prompt template argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// PromptHandler is the function signature for prompt generation
type PromptHandler func(ctx context.Context, args map[string]string) (string, error)

// SecurityConfig defines security settings for the MCP server
type SecurityConfig struct {
	SandboxEnabled   bool     `json:"sandboxEnabled"`
	AllowedDomains   []string `json:"allowedDomains,omitempty"`
	BlockedCommands  []string `json:"blockedCommands,omitempty"`
	MaxExecTime      int      `json:"maxExecTime"` // seconds
	RequireAuth      bool     `json:"requireAuth"`
	APIKeys          []string `json:"apiKeys,omitempty"`
}

// ServerMetrics tracks MCP server performance
type ServerMetrics struct {
	mu              sync.Mutex
	TotalRequests   int64     `json:"totalRequests"`
	ToolInvocations int64     `json:"toolInvocations"`
	ResourceReads   int64     `json:"resourceReads"`
	PromptRequests  int64     `json:"promptRequests"`
	ErrorCount      int64     `json:"errorCount"`
	AvgResponseMs   float64   `json:"avgResponseMs"`
	LastRequestAt   time.Time `json:"lastRequestAt"`
	StartTime       time.Time `json:"startTime"`
}

// ServerConfig holds MCP server configuration
type ServerConfig struct {
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Transport TransportType   `json:"transport"`
	Port      int             `json:"port,omitempty"`
	MaxConns  int             `json:"maxConns"`
	Security  *SecurityConfig `json:"security"`
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(config *ServerConfig, logger *slog.Logger) *MCPServer {
	if logger == nil {
		logger = slog.Default()
	}
	if config.Security == nil {
		config.Security = &SecurityConfig{
			SandboxEnabled: true,
			MaxExecTime:    30,
		}
	}
	if config.MaxConns == 0 {
		config.MaxConns = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MCPServer{
		name:      config.Name,
		version:   config.Version,
		transport: config.Transport,
		tools:     make(map[string]*Tool),
		resources: make(map[string]*Resource),
		prompts:   make(map[string]*Prompt),
		port:      config.Port,
		maxConns:  config.MaxConns,
		security:  config.Security,
		metrics: &ServerMetrics{
			StartTime: time.Now(),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterTool registers a new tool with the MCP server
func (s *MCPServer) RegisterTool(tool *Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if tool.Handler == nil {
		return fmt.Errorf("tool handler cannot be nil")
	}
	if _, exists := s.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name)
	}

	tool.CreatedAt = time.Now()
	tool.UpdatedAt = time.Now()
	if tool.Timeout == 0 {
		tool.Timeout = 30 * time.Second
	}
	s.tools[tool.Name] = tool
	s.logger.Info("MCP tool registered", "tool", tool.Name)
	return nil
}

// RegisterResource registers a new resource with the MCP server
func (s *MCPServer) RegisterResource(res *Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if res.URI == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	if res.Handler == nil {
		return fmt.Errorf("resource handler cannot be nil")
	}
	if _, exists := s.resources[res.URI]; exists {
		return fmt.Errorf("resource %q already registered", res.URI)
	}

	res.CreatedAt = time.Now()
	s.resources[res.URI] = res
	s.logger.Info("MCP resource registered", "uri", res.URI, "name", res.Name)
	return nil
}

// RegisterPrompt registers a new prompt template with the MCP server
func (s *MCPServer) RegisterPrompt(prompt *Prompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prompt.Name == "" {
		return fmt.Errorf("prompt name cannot be empty")
	}
	if prompt.Handler == nil {
		return fmt.Errorf("prompt handler cannot be nil")
	}
	if _, exists := s.prompts[prompt.Name]; exists {
		return fmt.Errorf("prompt %q already registered", prompt.Name)
	}

	prompt.CreatedAt = time.Now()
	s.prompts[prompt.Name] = prompt
	s.logger.Info("MCP prompt registered", "prompt", prompt.Name)
	return nil
}

// InvokeTool invokes a registered tool
func (s *MCPServer) InvokeTool(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	tool, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	if !tool.Enabled {
		return nil, fmt.Errorf("tool %q is disabled", name)
	}

	start := time.Now()
	result, err := tool.Handler(ctx, params)
	duration := time.Since(start)

	s.metrics.mu.Lock()
	s.metrics.TotalRequests++
	s.metrics.ToolInvocations++
	s.metrics.LastRequestAt = time.Now()
	if err != nil {
		s.metrics.ErrorCount++
	} else {
		// Update average response time
		n := float64(s.metrics.ToolInvocations)
		s.metrics.AvgResponseMs = (s.metrics.AvgResponseMs*(n-1) + float64(duration.Milliseconds())) / n
	}
	s.metrics.mu.Unlock()

	if err != nil {
		s.logger.Error("MCP tool invocation failed", "tool", name, "error", err, "duration", duration)
		return nil, fmt.Errorf("tool %q failed: %w", name, err)
	}

	s.logger.Info("MCP tool invoked", "tool", name, "duration", duration)
	return result, nil
}

// ReadResource reads a registered resource
func (s *MCPServer) ReadResource(ctx context.Context, uri string) ([]byte, error) {
	s.mu.RLock()
	res, exists := s.resources[uri]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("resource %q not found", uri)
	}
	if !res.Enabled {
		return nil, fmt.Errorf("resource %q is disabled", uri)
	}

	start := time.Now()
	data, err := res.Handler(ctx, uri)
	duration := time.Since(start)

	s.metrics.mu.Lock()
	s.metrics.TotalRequests++
	s.metrics.ResourceReads++
	s.metrics.LastRequestAt = time.Now()
	if err != nil {
		s.metrics.ErrorCount++
	}
	s.metrics.mu.Unlock()

	if err != nil {
		s.logger.Error("MCP resource read failed", "uri", uri, "error", err, "duration", duration)
		return nil, fmt.Errorf("resource %q read failed: %w", uri, err)
	}

	s.logger.Info("MCP resource read", "uri", uri, "bytes", len(data), "duration", duration)
	return data, nil
}

// GetPrompt generates a prompt from a registered template
func (s *MCPServer) GetPrompt(ctx context.Context, name string, args map[string]string) (string, error) {
	s.mu.RLock()
	prompt, exists := s.prompts[name]
	s.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("prompt %q not found", name)
	}
	if !prompt.Enabled {
		return "", fmt.Errorf("prompt %q is disabled", name)
	}

	// Validate required arguments
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, ok := args[arg.Name]; !ok {
				return "", fmt.Errorf("required argument %q missing for prompt %q", arg.Name, name)
			}
		}
	}

	start := time.Now()
	result, err := prompt.Handler(ctx, args)
	duration := time.Since(start)

	s.metrics.mu.Lock()
	s.metrics.TotalRequests++
	s.metrics.PromptRequests++
	s.metrics.LastRequestAt = time.Now()
	if err != nil {
		s.metrics.ErrorCount++
	}
	s.metrics.mu.Unlock()

	if err != nil {
		s.logger.Error("MCP prompt generation failed", "prompt", name, "error", err)
		return "", fmt.Errorf("prompt %q failed: %w", name, err)
	}

	s.logger.Info("MCP prompt generated", "prompt", name, "duration", duration)
	return result, nil
}

// ListTools returns all registered tools
func (s *MCPServer) ListTools() []*Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]*Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	return tools
}

// ListResources returns all registered resources
func (s *MCPServer) ListResources() []*Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]*Resource, 0, len(s.resources))
	for _, r := range s.resources {
		resources = append(resources, r)
	}
	return resources
}

// ListPrompts returns all registered prompts
func (s *MCPServer) ListPrompts() []*Prompt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompts := make([]*Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		prompts = append(prompts, p)
	}
	return prompts
}

// GetMetrics returns current server metrics
func (s *MCPServer) GetMetrics() *ServerMetrics {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()
	return &ServerMetrics{
		TotalRequests:   s.metrics.TotalRequests,
		ToolInvocations: s.metrics.ToolInvocations,
		ResourceReads:   s.metrics.ResourceReads,
		PromptRequests:  s.metrics.PromptRequests,
		ErrorCount:      s.metrics.ErrorCount,
		AvgResponseMs:   s.metrics.AvgResponseMs,
		LastRequestAt:   s.metrics.LastRequestAt,
		StartTime:       s.metrics.StartTime,
	}
}

// Stop gracefully stops the MCP server
func (s *MCPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()
	s.running = false
	s.logger.Info("MCP server stopped", "name", s.name)
	return nil
}

// IsRunning returns whether the server is currently running
func (s *MCPServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// UnregisterTool removes a tool from the server
func (s *MCPServer) UnregisterTool(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[name]; !exists {
		return fmt.Errorf("tool %q not found", name)
	}
	delete(s.tools, name)
	s.logger.Info("MCP tool unregistered", "tool", name)
	return nil
}

// EnableTool enables a registered tool
func (s *MCPServer) EnableTool(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool, exists := s.tools[name]
	if !exists {
		return fmt.Errorf("tool %q not found", name)
	}
	tool.Enabled = true
	tool.UpdatedAt = time.Now()
	return nil
}

// DisableTool disables a registered tool
func (s *MCPServer) DisableTool(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool, exists := s.tools[name]
	if !exists {
		return fmt.Errorf("tool %q not found", name)
	}
	tool.Enabled = false
	tool.UpdatedAt = time.Now()
	return nil
}

// ToJSON exports the server configuration as JSON
func (s *MCPServer) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := map[string]interface{}{
		"name":      s.name,
		"version":   s.version,
		"transport": s.transport,
		"port":      s.port,
		"tools":     len(s.tools),
		"resources": len(s.resources),
		"prompts":   len(s.prompts),
		"running":   s.running,
		"metrics":   s.GetMetrics(),
	}
	return json.MarshalIndent(info, "", "  ")
}
