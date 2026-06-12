package containerlens

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Container represents a running container
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"` // running, stopped, suspicious
	PID         int               `json:"pid"`
	CreatedAt   time.Time         `json:"created_at"`
	Labels      map[string]string `json:"labels"`
	NetworkMode string            `json:"network_mode"`
	Privileged  bool              `json:"privileged"`
}

// SecurityEvent represents a security event detected in a container
type SecurityEvent struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id"`
	Type        string    `json:"type"` // anomaly, vulnerability, policy_violation
	Severity    string    `json:"severity"` // low, medium, high, critical
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"` // runtime, image, network
	Remediation string    `json:"remediation"`
	Resolved    bool      `json:"resolved"`
}

// Vulnerability represents a container image vulnerability
type Vulnerability struct {
	ID          string `json:"id"`
	CVE         string `json:"cve"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixedIn     string `json:"fixed_in,omitempty"`
	Severity    string `json:"severity"`
	Score       float64 `json:"score"` // CVSS score
	Description string `json:"description"`
	Image       string `json:"image"`
}

// PolicyRule defines a security policy rule
type PolicyRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // image, runtime, network
	Enabled     bool     `json:"enabled"`
	Conditions  []string `json:"conditions"`
	Action      string   `json:"action"` // alert, block, quarantine
	Severity    string   `json:"severity"`
}

// ComplianceCheck represents a compliance check result
type ComplianceCheck struct {
	ID          string    `json:"id"`
	Standard    string    `json:"standard"` // CIS, NIST, PCI-DSS
	Rule        string    `json:"rule"`
	Status      string    `json:"status"` // pass, fail, warn
	ContainerID string    `json:"container_id"`
	Timestamp   time.Time `json:"timestamp"`
	Details     string    `json:"details"`
}

// ScanResult represents an image scan result
type ScanResult struct {
	Image         string           `json:"image"`
	ScanTime      time.Time        `json:"scan_time"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	TotalCVEs     int              `json:"total_cves"`
	CriticalCount int              `json:"critical_count"`
	HighCount     int              `json:"high_count"`
	MediumCount   int              `json:"medium_count"`
	LowCount      int              `json:"low_count"`
	PassStatus    bool             `json:"pass_status"`
}

// ContainerLens provides container runtime security monitoring
type ContainerLens struct {
	mu           sync.RWMutex
	containers   map[string]*Container
	events       []SecurityEvent
	vulns        []Vulnerability
	policies     map[string]*PolicyRule
	compliance   []ComplianceCheck
	scans        map[string]*ScanResult
}

// NewContainerLens creates a new container security monitor
func NewContainerLens() *ContainerLens {
	return &ContainerLens{
		containers: make(map[string]*Container),
		events:     make([]SecurityEvent, 0),
		vulns:      make([]Vulnerability, 0),
		policies:   make(map[string]*PolicyRule),
		compliance: make([]ComplianceCheck, 0),
		scans:      make(map[string]*ScanResult),
	}
}

// RegisterContainer registers a container for monitoring
func (cl *ContainerLens) RegisterContainer(ctx context.Context, container *Container) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if container.ID == "" {
		return fmt.Errorf("container ID is required")
	}

	cl.containers[container.ID] = container
	return nil
}

// DetectAnomaly detects anomalous behavior in a container
func (cl *ContainerLens) DetectAnomaly(ctx context.Context, containerID string, behavior string) (*SecurityEvent, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	container, ok := cl.containers[containerID]
	if !ok {
		return nil, fmt.Errorf("container %s not found", containerID)
	}

	severity := "low"
	if container.Privileged {
		severity = "high"
	}

	event := &SecurityEvent{
		ID:          fmt.Sprintf("evt-%s-%d", containerID, time.Now().Unix()),
		ContainerID: containerID,
		Type:        "anomaly",
		Severity:    severity,
		Title:       fmt.Sprintf("Anomalous behavior detected: %s", behavior),
		Description: fmt.Sprintf("Container %s exhibited unusual behavior: %s", container.Name, behavior),
		Timestamp:   time.Now(),
		Source:      "runtime",
		Remediation: "Review container logs and resource usage",
	}

	cl.events = append(cl.events, *event)
	return event, nil
}

// ScanImage scans a container image for vulnerabilities
func (cl *ContainerLens) ScanImage(ctx context.Context, image string) (*ScanResult, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Simulate scan
	result := &ScanResult{
		Image:    image,
		ScanTime: time.Now(),
		Vulnerabilities: []Vulnerability{},
		TotalCVEs: 0,
		PassStatus: true,
	}

	cl.scans[image] = result
	return result, nil
}

// AddPolicy adds a security policy rule
func (cl *ContainerLens) AddPolicy(ctx context.Context, rule *PolicyRule) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	cl.policies[rule.ID] = rule
	return nil
}

// RunComplianceCheck runs a compliance check on a container
func (cl *ContainerLens) RunComplianceCheck(ctx context.Context, containerID string, standard string) ([]ComplianceCheck, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	container, ok := cl.containers[containerID]
	if !ok {
		return nil, fmt.Errorf("container %s not found", containerID)
	}

	checks := []ComplianceCheck{
		{
			ID:          fmt.Sprintf("chk-%s-1", containerID),
			Standard:    standard,
			Rule:        "privileged_mode",
			ContainerID: containerID,
			Timestamp:   time.Now(),
		},
		{
			ID:          fmt.Sprintf("chk-%s-2", containerID),
			Standard:    standard,
			Rule:        "network_mode",
			ContainerID: containerID,
			Timestamp:   time.Now(),
		},
	}

	// Check privileged mode
	if container.Privileged {
		checks[0].Status = "fail"
		checks[0].Details = "Container running in privileged mode"
	} else {
		checks[0].Status = "pass"
	}

	// Check network mode
	if container.NetworkMode == "host" {
		checks[1].Status = "fail"
		checks[1].Details = "Container using host network"
	} else {
		checks[1].Status = "pass"
	}

	cl.compliance = append(cl.compliance, checks...)
	return checks, nil
}

// GetEvents returns security events
func (cl *ContainerLens) GetEvents(ctx context.Context, containerID string) []SecurityEvent {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if containerID == "" {
		return cl.events
	}

	var result []SecurityEvent
	for _, e := range cl.events {
		if e.ContainerID == containerID {
			result = append(result, e)
		}
	}
	return result
}

// GetVulnerabilities returns vulnerabilities
func (cl *ContainerLens) GetVulnerabilities(ctx context.Context, severity string) []Vulnerability {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if severity == "" {
		return cl.vulns
	}

	var result []Vulnerability
	for _, v := range cl.vulns {
		if v.Severity == severity {
			result = append(result, v)
		}
	}
	return result
}

// GetScanResult returns the scan result for an image
func (cl *ContainerLens) GetScanResult(ctx context.Context, image string) (*ScanResult, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	result, ok := cl.scans[image]
	if !ok {
		return nil, fmt.Errorf("no scan result for image %s", image)
	}

	return result, nil
}

// GetPolicies returns all policies
func (cl *ContainerLens) GetPolicies(ctx context.Context) []*PolicyRule {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	policies := make([]*PolicyRule, 0, len(cl.policies))
	for _, p := range cl.policies {
		policies = append(policies, p)
	}
	return policies
}
