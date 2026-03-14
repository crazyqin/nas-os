package plugin

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PermissionType represents a plugin permission type
type PermissionType string

const (
	// File system permissions
	PermReadFiles   PermissionType = "fs.read"
	PermWriteFiles  PermissionType = "fs.write"
	PermDeleteFiles PermissionType = "fs.delete"
	PermExecFiles   PermissionType = "fs.exec"

	// Network permissions
	PermNetworkListen PermissionType = "network.listen"
	PermNetworkDial   PermissionType = "network.dial"
	PermNetworkHTTP   PermissionType = "network.http"

	// System permissions
	PermSystemInfo  PermissionType = "system.info"
	PermSystemExec  PermissionType = "system.exec"
	PermSystemMount PermissionType = "system.mount"

	// User permissions
	PermUserRead   PermissionType = "user.read"
	PermUserWrite  PermissionType = "user.write"

	// Storage permissions
	PermStorageRead  PermissionType = "storage.read"
	PermStorageWrite PermissionType = "storage.write"

	// Admin permissions
	PermAdmin PermissionType = "admin"
)

// PermissionSet defines a set of permissions
type PermissionSet struct {
	Permissions []PermissionType `json:"permissions"`
	Deny        []PermissionType `json:"deny,omitempty"`
}

// SandboxConfig defines sandbox configuration for a plugin
type SandboxConfig struct {
	// Permissions granted to the plugin
	Permissions PermissionSet `json:"permissions"`

	// File system restrictions
	AllowedPaths []string `json:"allowedPaths,omitempty"`
	DeniedPaths  []string `json:"deniedPaths,omitempty"`

	// Network restrictions
	AllowedHosts []string `json:"allowedHosts,omitempty"`
	DeniedHosts  []string `json:"deniedHosts,omitempty"`
	AllowedPorts []int    `json:"allowedPorts,omitempty"`

	// Resource limits
	MaxMemoryMB   int `json:"maxMemoryMb,omitempty"`
	MaxCPUPercent int `json:"maxCpuPercent,omitempty"`
	MaxFileSizeMB int `json:"maxFileSizeMb,omitempty"`

	// Execution restrictions
	NoNewPrivileges bool `json:"noNewPrivileges"`
	ReadOnlyRoot    bool `json:"readOnlyRoot"`
}

// Sandbox manages plugin isolation and permissions
type Sandbox struct {
	config    SandboxConfig
	pluginID  string
	violations []Violation
	mu        sync.RWMutex
}

// Violation represents a permission violation
type Violation struct {
	Timestamp int64          `json:"timestamp"`
	Permission PermissionType `json:"permission"`
	Resource   string         `json:"resource"`
	Action     string         `json:"action"`
	Denied     bool           `json:"denied"`
	Message    string         `json:"message"`
}

// NewSandbox creates a new sandbox for a plugin
func NewSandbox(pluginID string, config SandboxConfig) *Sandbox {
	// Set defaults
	if config.MaxMemoryMB == 0 {
		config.MaxMemoryMB = 256
	}
	if config.MaxCPUPercent == 0 {
		config.MaxCPUPercent = 50
	}
	if config.MaxFileSizeMB == 0 {
		config.MaxFileSizeMB = 100
	}

	return &Sandbox{
		config:    config,
		pluginID:  pluginID,
		violations: make([]Violation, 0),
	}
}

// CheckPermission checks if a permission is granted
func (s *Sandbox) CheckPermission(perm PermissionType) bool {
	// Check if in deny list
	for _, d := range s.config.Permissions.Deny {
		if d == perm || d == PermAdmin {
			s.recordViolation(perm, "", "permission_denied", true)
			return false
		}
	}

	// Check if explicitly granted or has admin
	for _, p := range s.config.Permissions.Permissions {
		if p == perm || p == PermAdmin {
			return true
		}
	}

	s.recordViolation(perm, "", "permission_not_granted", true)
	return false
}

// CheckFileAccess checks if file access is allowed
func (s *Sandbox) CheckFileAccess(path string, op string) error {
	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check denied paths first
	for _, denied := range s.config.DeniedPaths {
		if strings.HasPrefix(absPath, denied) {
			s.recordViolation(PermReadFiles, path, op, true)
			return fmt.Errorf("access to path %s is denied", path)
		}
	}

	// Check if any allowed path matches
	if len(s.config.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range s.config.AllowedPaths {
			if strings.HasPrefix(absPath, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			s.recordViolation(PermReadFiles, path, op, true)
			return fmt.Errorf("access to path %s is not allowed", path)
		}
	}

	// Check permission based on operation
	var perm PermissionType
	switch op {
	case "read", "stat":
		perm = PermReadFiles
	case "write", "create":
		perm = PermWriteFiles
	case "delete", "remove":
		perm = PermDeleteFiles
	case "exec":
		perm = PermExecFiles
	default:
		perm = PermReadFiles
	}

	if !s.CheckPermission(perm) {
		return fmt.Errorf("permission %s denied for operation %s", perm, op)
	}

	// Check file size limit for write operations
	if op == "write" || op == "create" {
		if s.config.MaxFileSizeMB > 0 {
			// This would be checked during actual write operations
		}
	}

	return nil
}

// CheckNetworkAccess checks if network access is allowed
func (s *Sandbox) CheckNetworkAccess(host string, port int) error {
	// Check denied hosts
	for _, denied := range s.config.DeniedHosts {
		if host == denied {
			s.recordViolation(PermNetworkDial, host, "connect", true)
			return fmt.Errorf("access to host %s is denied", host)
		}
	}

	// Check allowed hosts
	if len(s.config.AllowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range s.config.AllowedHosts {
			if host == allowedHost || allowedHost == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			s.recordViolation(PermNetworkDial, host, "connect", true)
			return fmt.Errorf("access to host %s is not allowed", host)
		}
	}

	// Check allowed ports
	if len(s.config.AllowedPorts) > 0 {
		allowed := false
		for _, allowedPort := range s.config.AllowedPorts {
			if port == allowedPort {
				allowed = true
				break
			}
		}
		if !allowed {
			s.recordViolation(PermNetworkDial, fmt.Sprintf("%s:%d", host, port), "connect", true)
			return fmt.Errorf("access to port %d is not allowed", port)
		}
	}

	// Check network permission
	if !s.CheckPermission(PermNetworkDial) {
		return fmt.Errorf("network access denied")
	}

	return nil
}

// recordViolation records a permission violation
func (s *Sandbox) recordViolation(perm PermissionType, resource, action string, denied bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := Violation{
		Timestamp:  0, // Would be set to time.Now().Unix() in real impl
		Permission: perm,
		Resource:   resource,
		Action:     action,
		Denied:     denied,
		Message:    fmt.Sprintf("Permission %s violation for resource %s", perm, resource),
	}
	s.violations = append(s.violations, v)
}

// GetViolations returns all recorded violations
func (s *Sandbox) GetViolations() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Violation, len(s.violations))
	copy(result, s.violations)
	return result
}

// GetConfig returns sandbox configuration
func (s *Sandbox) GetConfig() SandboxConfig {
	return s.config
}

// FileSystem wraps os file operations with sandbox checks
type FileSystem struct {
	sandbox *Sandbox
	root    string // Chroot root
}

// NewFileSystem creates a sandboxed file system
func NewFileSystem(sandbox *Sandbox, root string) *FileSystem {
	return &FileSystem{
		sandbox: sandbox,
		root:    root,
	}
}

// Open opens a file with sandbox checks
func (fs *FileSystem) Open(path string) (*os.File, error) {
	if err := fs.sandbox.CheckFileAccess(path, "read"); err != nil {
		return nil, err
	}
	return os.Open(fs.resolvePath(path))
}

// Create creates a file with sandbox checks
func (fs *FileSystem) Create(path string) (*os.File, error) {
	if err := fs.sandbox.CheckFileAccess(path, "write"); err != nil {
		return nil, err
	}
	return os.Create(fs.resolvePath(path))
}

// ReadFile reads a file with sandbox checks
func (fs *FileSystem) ReadFile(path string) ([]byte, error) {
	if err := fs.sandbox.CheckFileAccess(path, "read"); err != nil {
		return nil, err
	}
	return os.ReadFile(fs.resolvePath(path))
}

// WriteFile writes a file with sandbox checks
func (fs *FileSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if err := fs.sandbox.CheckFileAccess(path, "write"); err != nil {
		return err
	}

	// Check size limit
	maxSize := fs.sandbox.config.MaxFileSizeMB * 1024 * 1024
	if maxSize > 0 && len(data) > maxSize {
		return fmt.Errorf("file size exceeds limit of %d MB", fs.sandbox.config.MaxFileSizeMB)
	}

	return os.WriteFile(fs.resolvePath(path), data, perm)
}

// Remove removes a file with sandbox checks
func (fs *FileSystem) Remove(path string) error {
	if err := fs.sandbox.CheckFileAccess(path, "delete"); err != nil {
		return err
	}
	return os.Remove(fs.resolvePath(path))
}

// Mkdir creates a directory with sandbox checks
func (fs *FileSystem) Mkdir(path string, perm fs.FileMode) error {
	if err := fs.sandbox.CheckFileAccess(path, "write"); err != nil {
		return err
	}
	return os.Mkdir(fs.resolvePath(path), perm)
}

// ReadDir reads a directory with sandbox checks
func (fs *FileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	if err := fs.sandbox.CheckFileAccess(path, "read"); err != nil {
		return nil, err
	}
	return os.ReadDir(fs.resolvePath(path))
}

// Stat gets file info with sandbox checks
func (fs *FileSystem) Stat(path string) (os.FileInfo, error) {
	if err := fs.sandbox.CheckFileAccess(path, "read"); err != nil {
		return nil, err
	}
	return os.Stat(fs.resolvePath(path))
}

// resolvePath resolves path relative to sandbox root
func (fs *FileSystem) resolvePath(path string) string {
	if fs.root == "" {
		return path
	}

	// Ensure path is absolute
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(fs.root, path)
	}

	// Prevent path traversal
	cleanPath := filepath.Clean(absPath)
	if !strings.HasPrefix(cleanPath, fs.root) {
		return fs.root // Return root for invalid paths
	}

	return cleanPath
}

// SandboxManager manages all plugin sandboxes
type SandboxManager struct {
	sandboxes map[string]*Sandbox
	profiles  map[string]SandboxConfig
	mu        sync.RWMutex
}

// NewSandboxManager creates a new sandbox manager
func NewSandboxManager() *SandboxManager {
	sm := &SandboxManager{
		sandboxes: make(map[string]*Sandbox),
		profiles:  make(map[string]SandboxConfig),
	}

	// Register default profiles
	sm.registerDefaultProfiles()

	return sm
}

// registerDefaultProfiles registers default sandbox profiles
func (sm *SandboxManager) registerDefaultProfiles() {
	// Minimal profile - read-only, no network
	sm.profiles["minimal"] = SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{PermReadFiles},
		},
		ReadOnlyRoot:    true,
		NoNewPrivileges: true,
		MaxMemoryMB:     64,
		MaxCPUPercent:   10,
	}

	// Standard profile - file read/write, limited network
	sm.profiles["standard"] = SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{
				PermReadFiles,
				PermWriteFiles,
				PermNetworkHTTP,
				PermSystemInfo,
			},
		},
		MaxMemoryMB:   256,
		MaxCPUPercent: 50,
		MaxFileSizeMB: 100,
	}

	// Full profile - most permissions
	sm.profiles["full"] = SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{
				PermReadFiles,
				PermWriteFiles,
				PermDeleteFiles,
				PermNetworkHTTP,
				PermNetworkDial,
				PermSystemInfo,
				PermStorageRead,
			},
		},
		MaxMemoryMB:   512,
		MaxCPUPercent: 80,
		MaxFileSizeMB: 500,
	}

	// Admin profile - full access
	sm.profiles["admin"] = SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{PermAdmin},
		},
		MaxMemoryMB:   1024,
		MaxCPUPercent: 100,
		MaxFileSizeMB: 1024,
	}
}

// CreateSandbox creates a sandbox for a plugin
func (sm *SandboxManager) CreateSandbox(pluginID string, profile string) (*Sandbox, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get profile
	config, exists := sm.profiles[profile]
	if !exists {
		config = sm.profiles["standard"]
	}

	sandbox := NewSandbox(pluginID, config)
	sm.sandboxes[pluginID] = sandbox

	return sandbox, nil
}

// GetSandbox returns a plugin's sandbox
func (sm *SandboxManager) GetSandbox(pluginID string) (*Sandbox, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sandbox, exists := sm.sandboxes[pluginID]
	return sandbox, exists
}

// RemoveSandbox removes a plugin's sandbox
func (sm *SandboxManager) RemoveSandbox(pluginID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sandboxes, pluginID)
}

// ListSandboxes lists all sandboxes
func (sm *SandboxManager) ListSandboxes() map[string]SandboxConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]SandboxConfig)
	for id, sandbox := range sm.sandboxes {
		result[id] = sandbox.config
	}
	return result
}

// GetProfiles returns available profiles
func (sm *SandboxManager) GetProfiles() map[string]SandboxConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]SandboxConfig)
	for name, config := range sm.profiles {
		result[name] = config
	}
	return result
}

// AddProfile adds a custom profile
func (sm *SandboxManager) AddProfile(name string, config SandboxConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.profiles[name] = config
}

// GetViolations returns all violations across sandboxes
func (sm *SandboxManager) GetViolations() map[string][]Violation {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string][]Violation)
	for id, sandbox := range sm.sandboxes {
		result[id] = sandbox.GetViolations()
	}
	return result
}

// ToJSON converts sandbox config to JSON
func (c SandboxConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseSandboxConfig parses sandbox config from JSON
func ParseSandboxConfig(jsonStr string) (SandboxConfig, error) {
	var config SandboxConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	return config, err
}

