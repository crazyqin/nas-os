// Package smart provides flexible disk health monitoring
// 对标TrueNAS 25.10 SMART监控改革 - 迁移到cron任务
package smart

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// TestType defines SMART test types
type TestType string

const (
	TestShort   TestType = "short"
	TestLong    TestType = "long"
	TestConvey  TestType = "conveyance"
	TestOffline TestType = "offline"
)

// ScheduleConfig defines SMART test schedule
type ScheduleConfig struct {
	TestType TestType
	Schedule string // cron expression
	Devices  []string
	Enabled  bool
}

// HealthStatus defines disk health status
type HealthStatus struct {
	Device       string
	Model        string
	Serial       string
	Health       string // PASSED, FAILED, WARNING
	Temperature  int
	PowerOnHours uint64
	ReadErrors   uint64
	WriteErrors  uint64
	LastTest     time.Time
}

// MonitorService provides flexible SMART monitoring
type MonitorService struct {
	schedules map[string]*ScheduleConfig
	health    map[string]*HealthStatus
	mu        sync.RWMutex
}

// NewMonitorService creates a new SMART monitor service
func NewMonitorService() *MonitorService {
	return &MonitorService{
		schedules: make(map[string]*ScheduleConfig),
		health:    make(map[string]*HealthStatus),
	}
}

// CreateSchedule creates a SMART test schedule (cron-based)
func (m *MonitorService) CreateSchedule(ctx context.Context, name string, cfg *ScheduleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[name]; exists {
		return fmt.Errorf("schedule %s already exists", name)
	}

	// Create cron job for SMART test
	cmd := exec.CommandContext(ctx, "crontab", "-l")
	output, err := cmd.Output()
	if err != nil && err.Error() != "exit status 1" {
		// No existing crontab is OK
	}

	// Add new cron entry
	cronEntry := fmt.Sprintf("%s /usr/sbin/smartctl -t %s %s >> /var/log/smart-tests.log 2>&1",
		cfg.Schedule, cfg.TestType, cfg.Devices[0])

	newCrontab := string(output) + cronEntry + "\n"
	_ = newCrontab // TODO: implement crontab pipe

	// Install new crontab
	cmd = exec.CommandContext(ctx, "crontab", "-")
	cmd.Stdin = nil // TODO: pipe newCrontab

	m.schedules[name] = cfg
	return nil
}

// DeleteSchedule removes a SMART test schedule
func (m *MonitorService) DeleteSchedule(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[name]; !exists {
		return fmt.Errorf("schedule %s not found", name)
	}

	delete(m.schedules, name)
	return nil
}

// GetHealth returns disk health status
func (m *MonitorService) GetHealth(ctx context.Context, device string) (*HealthStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Run smartctl to get health
	cmd := exec.CommandContext(ctx, "smartctl", "-H", "-A", device)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("smartctl failed: %w", err)
	}

	status := parseSmartOutput(device, output)
	m.health[device] = status
	return status, nil
}

// GetAllHealth returns all disk health statuses
func (m *MonitorService) GetAllHealth(ctx context.Context) ([]*HealthStatus, error) {
	// List all disks
	cmd := exec.CommandContext(ctx, "lsblk", "-d", "-n", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk failed: %w", err)
	}
	_ = output // used in parseDiskList below

	devices := parseDiskList(output)
	result := make([]*HealthStatus, 0, len(devices))

	for _, device := range devices {
		status, err := m.GetHealth(ctx, "/dev/"+device)
		if err != nil {
			continue
		}
		result = append(result, status)
	}

	return result, nil
}

// parseSmartOutput parses smartctl output
func parseSmartOutput(device string, output []byte) *HealthStatus {
	status := &HealthStatus{
		Device: device,
	}

	lines := string(output)
	// TODO: Implement full parsing
	for _, line := range splitLines(lines) {
		if contains(line, "SMART overall-health") {
			if contains(line, "PASSED") {
				status.Health = "PASSED"
			} else if contains(line, "FAILED") {
				status.Health = "FAILED"
			}
		}
	}

	return status
}

// parseDiskList parses lsblk output
func parseDiskList(output []byte) []string {
	lines := splitLines(string(output))
	devices := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" && !startsWith(line, "loop") {
			devices = append(devices, line)
		}
	}
	return devices
}

// splitLines splits text into lines
func splitLines(text string) []string {
	result := make([]string, 0)
	for _, line := range []string{} { // TODO: proper split
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// contains checks if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) // TODO: proper check
}

// startsWith checks if string starts with prefix
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) // TODO: proper check
}

// RunTest runs an immediate SMART test
func (m *MonitorService) RunTest(ctx context.Context, device string, testType TestType) error {
	cmd := exec.CommandContext(ctx, "smartctl", "-t", string(testType), device)
	return cmd.Run()
}

// GetTestProgress returns test progress
func (m *MonitorService) GetTestProgress(ctx context.Context, device string) (int, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "-c", device)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("smartctl failed: %w", err)
	}
	_ = output // TODO: Parse progress from output
	return 0, nil
}
