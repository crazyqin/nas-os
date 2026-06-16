package containerguardian

import (
	"context"
	"fmt"
	"time"
)

// MonitorContainer monitors a running container for runtime anomalies.
// In production, this would integrate with Docker API, cgroup stats, etc.
func (g *Guardian) MonitorContainer(ctx context.Context, containerID string) (*RuntimeStatus, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	status := &RuntimeStatus{
		ContainerID:     containerID,
		Running:         true,
		CPUUsage:        45.5,
		MemoryUsage:     512 * 1024 * 1024, // 512MB
		NetworkIO:       1024 * 1024,       // 1MB
		PidsCount:       15,
		Uptime:          2 * time.Hour,
		Privileged:      false,
		ReadOnlyRoot:    false,
		SeccompProfile:  "default",
		AppArmorProfile: "docker-default",
		RootNamespace:   false,
	}

	// Detect runtime anomalies
	status.Anomalies = g.detectRuntimeAnomalies(status)

	// Audit log
	g.addAuditEntry(containerID, "", "MonitorContainer",
		fmt.Sprintf("container %s: cpu=%.1f%% mem=%dMB pids=%d anomalies=%d",
			containerID, status.CPUUsage, status.MemoryUsage/(1024*1024), status.PidsCount, len(status.Anomalies)),
		"INFO", true)

	return status, nil
}

// detectRuntimeAnomalies detects anomalous behavior in container runtime
func (g *Guardian) detectRuntimeAnomalies(status *RuntimeStatus) []Anomaly {
	anomalies := make([]Anomaly, 0)
	now := time.Now()

	// CPU usage anomaly
	if status.CPUUsage > 90.0 {
		anomalies = append(anomalies, Anomaly{
			Type:        "high_cpu",
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("CPU usage at %.1f%% exceeds 90%% threshold", status.CPUUsage),
			DetectedAt:  now,
		})
	}

	// Memory usage anomaly (>1GB)
	if status.MemoryUsage > 1024*1024*1024 {
		anomalies = append(anomalies, Anomaly{
			Type:        "high_memory",
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("Memory usage at %dMB exceeds 1024MB threshold", status.MemoryUsage/(1024*1024)),
			DetectedAt:  now,
		})
	}

	// Network IO anomaly (>100MB)
	if status.NetworkIO > 100*1024*1024 {
		anomalies = append(anomalies, Anomaly{
			Type:        "high_network_io",
			Severity:    SeverityMedium,
			Description: fmt.Sprintf("Network IO at %dMB exceeds 100MB threshold", status.NetworkIO/(1024*1024)),
			DetectedAt:  now,
		})
	}

	// Privileged mode
	if status.Privileged {
		anomalies = append(anomalies, Anomaly{
			Type:        "privileged_mode",
			Severity:    SeverityCritical,
			Description: "Container is running in privileged mode with full host access",
			DetectedAt:  now,
		})
	}

	// PID explosion detection
	if status.PidsCount > 200 {
		anomalies = append(anomalies, Anomaly{
			Type:        "pid_explosion",
			Severity:    SeverityCritical,
			Description: fmt.Sprintf("Process count at %d exceeds 200, possible fork bomb", status.PidsCount),
			DetectedAt:  now,
		})
	}

	// Writable root filesystem
	if !status.ReadOnlyRoot {
		anomalies = append(anomalies, Anomaly{
			Type:        "writable_root",
			Severity:    SeverityMedium,
			Description: "Container root filesystem is writable, increasing attack surface",
			DetectedAt:  now,
		})
	}

	// No seccomp profile
	if status.SeccompProfile == "" || status.SeccompProfile == "unconfined" {
		anomalies = append(anomalies, Anomaly{
			Type:        "no_seccomp",
			Severity:    SeverityHigh,
			Description: "No seccomp profile applied, container has unrestricted system call access",
			DetectedAt:  now,
		})
	}

	// Running as root namespace
	if status.RootNamespace {
		anomalies = append(anomalies, Anomaly{
			Type:        "root_namespace",
			Severity:    SeverityHigh,
			Description: "Container is running in root user namespace, potential privilege escalation",
			DetectedAt:  now,
		})
	}

	return anomalies
}

// CheckResourceLimits validates container resource limits are properly configured
func (g *Guardian) CheckResourceLimits(ctx context.Context, containerID string, limits *ResourceLimits) ([]ComplianceRule, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	rules := make([]ComplianceRule, 0)

	if limits == nil {
		rules = append(rules, ComplianceRule{
			ID:          "RES-001",
			Name:        "Resource limits configured",
			Description: "Container should have resource limits configured to prevent resource exhaustion",
			Category:    "resource-limits",
			Status:      ComplianceFail,
			Message:     "No resource limits configured",
			Severity:    SeverityHigh,
		})
		return rules, nil
	}

	// Check CPU
	if limits.CPUQuota <= 0 {
		rules = append(rules, ComplianceRule{
			ID:          "RES-002",
			Name:        "CPU limit set",
			Description: "Container should have CPU limits to prevent CPU starvation",
			Category:    "resource-limits",
			Status:      ComplianceFail,
			Message:     "CPU quota not set",
			Severity:    SeverityMedium,
		})
	} else {
		rules = append(rules, ComplianceRule{
			ID:       "RES-002",
			Name:     "CPU limit set",
			Category: "resource-limits",
			Status:   CompliancePass,
			Message:  fmt.Sprintf("CPU quota: %dμs", limits.CPUQuota),
			Severity: SeverityMedium,
		})
	}

	// Check Memory
	if limits.MemoryLimit <= 0 {
		rules = append(rules, ComplianceRule{
			ID:          "RES-003",
			Name:        "Memory limit set",
			Description: "Container should have memory limits to prevent OOM situations",
			Category:    "resource-limits",
			Status:      ComplianceFail,
			Message:     "Memory limit not set",
			Severity:    SeverityHigh,
		})
	} else {
		rules = append(rules, ComplianceRule{
			ID:       "RES-003",
			Name:     "Memory limit set",
			Category: "resource-limits",
			Status:   CompliancePass,
			Message:  fmt.Sprintf("Memory limit: %dMB", limits.MemoryLimit/(1024*1024)),
			Severity: SeverityHigh,
		})
	}

	// Check PIDs
	if limits.PidsLimit <= 0 {
		rules = append(rules, ComplianceRule{
			ID:          "RES-004",
			Name:        "PID limit set",
			Description: "Container should have PID limits to prevent fork bombs",
			Category:    "resource-limits",
			Status:      ComplianceFail,
			Message:     "PID limit not set",
			Severity:    SeverityMedium,
		})
	} else {
		rules = append(rules, ComplianceRule{
			ID:       "RES-004",
			Name:     "PID limit set",
			Category: "resource-limits",
			Status:   CompliancePass,
			Message:  fmt.Sprintf("PID limit: %d", limits.PidsLimit),
			Severity: SeverityMedium,
		})
	}

	return rules, nil
}
