package iscsi

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages iSCSI targets
type Manager struct {
	mu         sync.RWMutex
	targets    map[string]*Target
	config     *Config
	configPath string
	lunMgr     *LUNManager
	chapMgr    *CHAPManager
	baseDomain string
}

// persistentConfig is the on-disk configuration structure
type persistentConfig struct {
	Config  *Config            `json:"config"`
	Targets map[string]*Target `json:"targets"`
}

// NewManager creates a new iSCSI manager
func NewManager(configPath, basePath string) (*Manager, error) {
	m := &Manager{
		targets:    make(map[string]*Target),
		config: &Config{
			Enabled:       true,
			PortalIP:      "0.0.0.0",
			PortalPort:    3260,
			DiscoveryAuth: false,
		},
		configPath: configPath,
		lunMgr:     NewLUNManager(basePath),
		chapMgr:    NewCHAPManager(),
		baseDomain: "nas-os.local",
	}

	// Load existing configuration
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return m, nil
}

// loadConfig loads configuration from disk
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if pc.Config != nil {
		m.config = pc.Config
	}
	if pc.Targets != nil {
		m.targets = pc.Targets
	}

	return nil
}

// saveConfig saves configuration to disk (acquires lock)
func (m *Manager) saveConfig() error {
	m.mu.RLock()
	pc := persistentConfig{
		Config:  m.config,
		Targets: m.targets,
	}
	m.mu.RUnlock()

	return m.writeConfigFile(pc)
}

// saveConfigLocked saves configuration (caller holds lock)
func (m *Manager) saveConfigLocked() error {
	pc := persistentConfig{
		Config:  m.config,
		Targets: m.targets,
	}
	return m.writeConfigFile(pc)
}

// writeConfigFile writes config to file
func (m *Manager) writeConfigFile(pc persistentConfig) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// CreateTarget creates a new iSCSI target
func (m *Manager) CreateTarget(input TargetInput) (*Target, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing target with same name
	for _, t := range m.targets {
		if t.Name == input.Name {
			return nil, ErrTargetExists
		}
	}

	// Generate IQN if not provided
	iqn := input.IQN
	if iqn == "" {
		var err error
		iqn, err = GenerateIQN(m.baseDomain, input.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate IQN: %w", err)
		}
	} else {
		if err := ValidateIQN(iqn); err != nil {
			return nil, err
		}
	}

	// Normalize IQN
	iqn = NormalizeIQN(iqn)

	// Check for duplicate IQN
	for _, t := range m.targets {
		if t.IQN == iqn {
			return nil, fmt.Errorf("target with IQN %s already exists", iqn)
		}
	}

	// Validate CHAP input
	if input.CHAP != nil {
		if err := m.chapMgr.ValidateInput(input.CHAP); err != nil {
			return nil, err
		}
	}

	// Set defaults
	maxSessions := input.MaxSessions
	if maxSessions <= 0 {
		maxSessions = 16
	}

	// Create target
	targetID := uuid.New().String()
	target := &Target{
		ID:               targetID,
		IQN:              iqn,
		Name:             input.Name,
		Alias:            input.Alias,
		LUNs:             make([]*LUN, 0),
		MaxSessions:      maxSessions,
		CurrentSessions:  0,
		AllowedInitiators: input.AllowedInitiators,
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Configure CHAP
	if input.CHAP != nil && input.CHAP.Enabled {
		target.CHAP = m.chapMgr.CreateConfig(targetID, input.CHAP)
	}

	m.targets[targetID] = target

	// Save configuration
	if err := m.saveConfigLocked(); err != nil {
		delete(m.targets, targetID)
		return nil, err
	}

	return target, nil
}

// GetTarget retrieves a target by ID
func (m *Manager) GetTarget(id string) (*Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, exists := m.targets[id]
	if !exists {
		return nil, ErrTargetNotFound
	}
	return target, nil
}

// GetTargetByIQN retrieves a target by IQN
func (m *Manager) GetTargetByIQN(iqn string) (*Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iqn = NormalizeIQN(iqn)
	for _, target := range m.targets {
		if target.IQN == iqn {
			return target, nil
		}
	}
	return nil, ErrTargetNotFound
}

// ListTargets lists all targets
func (m *Manager) ListTargets() []*Target {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*Target, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	return targets
}

// UpdateTarget updates a target
func (m *Manager) UpdateTarget(id string, input TargetInput) (*Target, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return nil, ErrTargetNotFound
	}

	// Validate CHAP input
	if input.CHAP != nil {
		if err := m.chapMgr.ValidateInput(input.CHAP); err != nil {
			return nil, err
		}
	}

	// Update fields
	if input.Alias != "" {
		target.Alias = input.Alias
	}
	if input.MaxSessions > 0 {
		target.MaxSessions = input.MaxSessions
	}
	if input.AllowedInitiators != nil {
		target.AllowedInitiators = input.AllowedInitiators
	}

	// Update CHAP
	if input.CHAP != nil {
		target.CHAP = m.chapMgr.UpdateConfig(id, input.CHAP)
	}

	target.UpdatedAt = time.Now()

	if err := m.saveConfigLocked(); err != nil {
		return nil, err
	}

	return target, nil
}

// DeleteTarget deletes a target
func (m *Manager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return ErrTargetNotFound
	}

	// Delete all LUNs
	for _, lun := range target.LUNs {
		if err := m.lunMgr.Delete(lun); err != nil {
			// Log but continue
		}
	}

	// Remove CHAP config
	m.chapMgr.DeleteConfig(id)

	delete(m.targets, id)

	return m.saveConfigLocked()
}

// AddLUN adds a LUN to a target
func (m *Manager) AddLUN(targetID string, input LUNInput) (*LUN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetID]
	if !exists {
		return nil, ErrTargetNotFound
	}

	// Check for duplicate LUN name
	for _, lun := range target.LUNs {
		if lun.Name == input.Name {
			return nil, ErrLUNExists
		}
	}

	// Create LUN
	lun, err := m.lunMgr.Create(targetID, input)
	if err != nil {
		return nil, err
	}

	// Assign LUN number
	lunNumber := len(target.LUNs)
	if lunNumber > 255 {
		return nil, fmt.Errorf("maximum LUN count reached")
	}
	lun.Number = lunNumber

	target.LUNs = append(target.LUNs, lun)
	target.UpdatedAt = time.Now()

	if err := m.saveConfigLocked(); err != nil {
		return nil, err
	}

	return lun, nil
}

// GetLUN retrieves a LUN from a target
func (m *Manager) GetLUN(targetID, lunID string) (*LUN, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, exists := m.targets[targetID]
	if !exists {
		return nil, ErrTargetNotFound
	}

	for _, lun := range target.LUNs {
		if lun.ID == lunID {
			return lun, nil
		}
	}

	return nil, ErrLUNNotFound
}

// RemoveLUN removes a LUN from a target
func (m *Manager) RemoveLUN(targetID, lunID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetID]
	if !exists {
		return ErrTargetNotFound
	}

	for i, lun := range target.LUNs {
		if lun.ID == lunID {
			if err := m.lunMgr.Delete(lun); err != nil {
				return err
			}
			target.LUNs = append(target.LUNs[:i], target.LUNs[i+1:]...)
			target.UpdatedAt = time.Now()
			return m.saveConfigLocked()
		}
	}

	return ErrLUNNotFound
}

// ExpandLUN expands a LUN
func (m *Manager) ExpandLUN(targetID, lunID string, newSize int64) (*LUN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetID]
	if !exists {
		return nil, ErrTargetNotFound
	}

	for _, lun := range target.LUNs {
		if lun.ID == lunID {
			if err := m.lunMgr.Expand(lun, newSize); err != nil {
				return nil, err
			}
			target.UpdatedAt = time.Now()
			if err := m.saveConfigLocked(); err != nil {
				return nil, err
			}
			return lun, nil
		}
	}

	return nil, ErrLUNNotFound
}

// CreateLUNSnapshot creates a snapshot of a LUN
func (m *Manager) CreateLUNSnapshot(targetID, lunID string, input LUNSnapshotInput) (*LUNSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetID]
	if !exists {
		return nil, ErrTargetNotFound
	}

	for _, lun := range target.LUNs {
		if lun.ID == lunID {
			snapshot, err := m.lunMgr.CreateSnapshot(lun, input)
			if err != nil {
				return nil, err
			}
			target.UpdatedAt = time.Now()
			if err := m.saveConfigLocked(); err != nil {
				return nil, err
			}
			return snapshot, nil
		}
	}

	return nil, ErrLUNNotFound
}

// GetTargetStatus gets the operational status of a target
func (m *Manager) GetTargetStatus(id string) (*TargetStatus, error) {
	m.mu.RLock()
	target, exists := m.targets[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrTargetNotFound
	}

	sessions, err := m.getSessions(id)
	if err != nil {
		sessions = make([]*Session, 0)
	}

	return &TargetStatus{
		IQN:          target.IQN,
		Running:      true, // Simplified - would check actual service
		Sessions:     sessions,
		SessionCount: len(sessions),
		MaxSessions:  target.MaxSessions,
		LUNCount:     len(target.LUNs),
	}, nil
}

// getSessions gets active sessions for a target
func (m *Manager) getSessions(targetID string) ([]*Session, error) {
	// In production, this would query the kernel target driver
	// For now, return simulated data
	return make([]*Session, 0), nil
}

// ApplyConfig applies the configuration to the system
func (m *Manager) ApplyConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if targetcli is available
	if _, err := exec.LookPath("targetcli"); err != nil {
		return fmt.Errorf("targetcli not found - install scst or lio-utils")
	}

	// Generate targetcli commands
	commands := m.generateTargetCLICommands()

	// Execute commands
	for _, cmd := range commands {
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		execCmd := exec.Command(parts[0], parts[1:]...)
		if output, err := execCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to execute %s: %w (%s)", cmd, err, string(output))
		}
	}

	return nil
}

// generateTargetCLICommands generates targetcli configuration commands
func (m *Manager) generateTargetCLICommands() []string {
	var commands []string

	// Clear existing configuration
	commands = append(commands, "targetcli clearconfig confirm=true")

	for _, target := range m.targets {
		if !target.Enabled {
			continue
		}

		// Create target
		commands = append(commands, fmt.Sprintf("targetcli /backstores/block create name=%s dev=/dev/null", target.Name))

		// Create LUNs
		for _, lun := range target.LUNs {
			if lun.Type == LUNTypeFile {
				commands = append(commands, fmt.Sprintf("targetcli /backstores/fileio create name=%s size=%d path=%s",
					lun.Name, lun.Size, lun.Path))
			} else {
				commands = append(commands, fmt.Sprintf("targetcli /backstores/block create name=%s dev=%s",
					lun.Name, lun.Path))
			}
		}

		// Create iSCSI target
		commands = append(commands, fmt.Sprintf("targetcli /iscsi create %s", target.IQN))

		// Configure CHAP
		if target.CHAP != nil && target.CHAP.Enabled {
			username, secret, ok := m.chapMgr.GetSecret(target.ID)
			if ok {
				commands = append(commands, fmt.Sprintf("targetcli /iscsi/%s/tpg1/auth set attribute authentication=1", target.IQN))
				commands = append(commands, fmt.Sprintf("targetcli /iscsi/%s/tpg1/auth set userid=%s", target.IQN, username))
				commands = append(commands, fmt.Sprintf("targetcli /iscsi/%s/tpg1/auth set password=%s", target.IQN, secret))
			}
		}

		// Set max sessions
		commands = append(commands, fmt.Sprintf("targetcli /iscsi/%s/tpg1 set attribute max_sessions=%d", target.IQN, target.MaxSessions))
	}

	// Save configuration
	commands = append(commands, "targetcli saveconfig")

	return commands
}

// Start starts the iSCSI target service
func (m *Manager) Start() error {
	cmd := exec.Command("systemctl", "start", "target")
	return cmd.Run()
}

// Stop stops the iSCSI target service
func (m *Manager) Stop() error {
	cmd := exec.Command("systemctl", "stop", "target")
	return cmd.Run()
}

// Restart restarts the iSCSI target service
func (m *Manager) Restart() error {
	cmd := exec.Command("systemctl", "restart", "target")
	return cmd.Run()
}

// GetStatus checks if the iSCSI target service is running
func (m *Manager) GetStatus() (bool, error) {
	cmd := exec.Command("systemctl", "is-active", "target")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "active", nil
}

// EnableTarget enables a target
func (m *Manager) EnableTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return ErrTargetNotFound
	}

	target.Enabled = true
	target.UpdatedAt = time.Now()

	return m.saveConfigLocked()
}

// DisableTarget disables a target
func (m *Manager) DisableTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return ErrTargetNotFound
	}

	target.Enabled = false
	target.UpdatedAt = time.Now()

	return m.saveConfigLocked()
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetBaseDomain sets the base domain for IQN generation
func (m *Manager) SetBaseDomain(domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseDomain = domain
}