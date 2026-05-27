// Package containersec provides container image vulnerability scanning and security policy enforcement.
package containersec

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PolicyEngine evaluates images against security policies
type PolicyEngine struct {
	logger   *zap.Logger
	policies map[string]*SecurityPolicy
	mu       sync.RWMutex
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine(logger *zap.Logger) *PolicyEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	engine := &PolicyEngine{
		logger:   logger,
		policies: make(map[string]*SecurityPolicy),
	}
	// Add default policy
	defaultPolicy := DefaultSecurityPolicy()
	engine.policies[defaultPolicy.ID] = &defaultPolicy
	return engine
}

// AddPolicy adds or updates a security policy
func (pe *PolicyEngine) AddPolicy(policy SecurityPolicy) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.policies[policy.ID] = &policy
	pe.logger.Info("policy added/updated", zap.String("id", policy.ID), zap.String("name", policy.Name))
}

// RemovePolicy removes a policy by ID
func (pe *PolicyEngine) RemovePolicy(id string) bool {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	if _, exists := pe.policies[id]; exists {
		delete(pe.policies, id)
		pe.logger.Info("policy removed", zap.String("id", id))
		return true
	}
	return false
}

// GetPolicy returns a policy by ID
func (pe *PolicyEngine) GetPolicy(id string) *SecurityPolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	if p, exists := pe.policies[id]; exists {
		copy := *p
		return &copy
	}
	return nil
}

// ListPolicies returns all policies
func (pe *PolicyEngine) ListPolicies() []SecurityPolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	policies := make([]SecurityPolicy, 0, len(pe.policies))
	for _, p := range pe.policies {
		policies = append(policies, *p)
	}
	return policies
}

// EvaluateImage checks an image against all enabled policies
func (pe *PolicyEngine) EvaluateImage(result *ScanResult) []PolicyViolation {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var violations []PolicyViolation

	for _, policy := range pe.policies {
		if !policy.Enabled {
			continue
		}

		policyViolations := pe.evaluatePolicy(result, policy)
		violations = append(violations, policyViolations...)
	}

	return violations
}

// evaluatePolicy checks an image against a single policy
func (pe *PolicyEngine) evaluatePolicy(result *ScanResult, policy *SecurityPolicy) []PolicyViolation {
	var violations []PolicyViolation

	// Check critical vuln count
	if policy.MaxCritical >= 0 && result.Summary.Critical > policy.MaxCritical {
		violations = append(violations, PolicyViolation{
			PolicyID: policy.ID,
			Rule:     "max_critical",
			Message:  fmt.Sprintf("Image has %d critical vulnerabilities, policy allows max %d", result.Summary.Critical, policy.MaxCritical),
			Severity: "blocking",
		})
	}

	// Check high vuln count
	if policy.MaxHigh >= 0 && result.Summary.High > policy.MaxHigh {
		violations = append(violations, PolicyViolation{
			PolicyID: policy.ID,
			Rule:     "max_high",
			Message:  fmt.Sprintf("Image has %d high vulnerabilities, policy allows max %d", result.Summary.High, policy.MaxHigh),
			Severity: "blocking",
		})
	}

	// Check total vuln count
	if policy.MaxTotal >= 0 && result.Summary.Total > policy.MaxTotal {
		violations = append(violations, PolicyViolation{
			PolicyID: policy.ID,
			Rule:     "max_total",
			Message:  fmt.Sprintf("Image has %d total vulnerabilities, policy allows max %d", result.Summary.Total, policy.MaxTotal),
			Severity: "blocking",
		})
	}

	// Check blocked packages
	for _, vuln := range result.Vulns {
		for _, blocked := range policy.BlockedPackages {
			if vuln.InstalledPkg == blocked {
				violations = append(violations, PolicyViolation{
					PolicyID: policy.ID,
					Rule:     "blocked_package",
					Message:  fmt.Sprintf("Image contains blocked package: %s", blocked),
					Severity: "blocking",
				})
				break
			}
		}
	}

	// Check allowed registries
	if len(policy.AllowedRegistries) > 0 && result.Registry != "" {
		allowed := false
		for _, reg := range policy.AllowedRegistries {
			if result.Registry == reg {
				allowed = true
				break
			}
		}
		if !allowed {
			violations = append(violations, PolicyViolation{
				PolicyID: policy.ID,
				Rule:     "allowed_registries",
				Message:  fmt.Sprintf("Image from registry %s not in allowed list", result.Registry),
				Severity: "blocking",
			})
		}
	}

	// Check image age
	if policy.MaxImageAge > 0 {
		oldestLayer := time.Now()
		for _, layer := range result.Layers {
			if layer.CreatedAt.Before(oldestLayer) {
				oldestLayer = layer.CreatedAt
			}
		}
		if age := time.Since(oldestLayer); age > policy.MaxImageAge {
			violations = append(violations, PolicyViolation{
				PolicyID: policy.ID,
				Rule:     "max_image_age",
				Message:  fmt.Sprintf("Image base is %v old, policy allows max %v", age.Round(time.Hour*24), policy.MaxImageAge),
				Severity: "warning",
			})
		}
	}

	// Check CIS benchmark score
	if policy.BenchmarkMinScore > 0 && result.BenchmarkScore != nil {
		if result.BenchmarkScore.TotalScore < policy.BenchmarkMinScore {
			violations = append(violations, PolicyViolation{
				PolicyID: policy.ID,
				Rule:     "benchmark_min_score",
				Message:  fmt.Sprintf("CIS benchmark score %.1f below minimum %.1f", result.BenchmarkScore.TotalScore, policy.BenchmarkMinScore),
				Severity: "warning",
			})
		}
	}

	return violations
}

// GenerateRemediation provides fix suggestions for policy violations
func (pe *PolicyEngine) GenerateRemediation(violations []PolicyViolation, result *ScanResult) []string {
	var suggestions []string

	for _, v := range violations {
		switch v.Rule {
		case "max_critical", "max_high", "max_total":
			suggestions = append(suggestions, pe.vulnRemediation(result)...)
		case "blocked_package":
			suggestions = append(suggestions, "Remove blocked packages from image or update security policy")
		case "allowed_registries":
			suggestions = append(suggestions, "Use an approved registry or add current registry to policy whitelist")
		case "max_image_age":
			suggestions = append(suggestions, "Rebuild image with latest base image")
		case "benchmark_min_score":
			suggestions = append(suggestions, pe.benchmarkRemediation(result)...)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, s := range suggestions {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}

	return unique
}

// vulnRemediation suggests fixes for vulnerability violations
func (pe *PolicyEngine) vulnRemediation(result *ScanResult) []string {
	var suggestions []string

	for _, vuln := range result.Vulns {
		if vuln.FixedIn != "" {
			suggestion := fmt.Sprintf("Update %s from %s to %s to fix %s",
				vuln.InstalledPkg, vuln.Version, vuln.FixedIn, vuln.ID)
			suggestions = append(suggestions, suggestion)
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Rebuild image with updated base image to resolve vulnerabilities")
	}

	return suggestions
}

// benchmarkRemediation suggests fixes for CIS benchmark failures
func (pe *PolicyEngine) benchmarkRemediation(result *ScanResult) []string {
	var suggestions []string

	if result.BenchmarkScore == nil {
		return suggestions
	}

	for _, check := range result.BenchmarkScore.Checks {
		if check.Status == "FAIL" {
			switch check.ID {
			case "CIS-2.8":
				suggestions = append(suggestions, "Add read-only filesystem flag: docker run --read-only")
			case "CIS-2.1":
				suggestions = append(suggestions, "Add non-root user: USER 1000 in Dockerfile")
			default:
				suggestions = append(suggestions, fmt.Sprintf("Fix CIS check %s: %s", check.ID, check.Title))
			}
		}
	}

	return suggestions
}

// ApproveImage determines if an image should be allowed to deploy
func (pe *PolicyEngine) ApproveImage(result *ScanResult) (bool, []string) {
	violations := pe.EvaluateImage(result)
	if len(violations) == 0 {
		return true, nil
	}

	// Check if any violations are blocking
	hasBlocking := false
	for _, v := range violations {
		if v.Severity == "blocking" {
			hasBlocking = true
			break
		}
	}

	if hasBlocking {
		suggestions := pe.GenerateRemediation(violations, result)
		return false, suggestions
	}

	// Only warnings - allow with advisory
	return true, pe.GenerateRemediation(violations, result)
}
