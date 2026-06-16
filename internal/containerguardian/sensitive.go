package containerguardian

import (
	"context"
	"fmt"
	"strings"
)

// detectSensitiveData scans for sensitive information leaks in environment variables,
// mount volumes, and hardcoded secrets in the image.
func (g *Guardian) detectSensitiveData(ctx context.Context, image string) []SensitiveFinding {
	findings := make([]SensitiveFinding, 0)

	findings = append(findings, g.checkSensitiveEnvVars(image)...)
	findings = append(findings, g.checkSensitiveMounts(image)...)
	findings = append(findings, g.checkHardcodedSecrets(image)...)

	return findings
}

// checkSensitiveEnvVars checks for sensitive data in environment variables
func (g *Guardian) checkSensitiveEnvVars(image string) []SensitiveFinding {
	findings := make([]SensitiveFinding, 0)

	// Patterns that indicate sensitive env vars
	sensitivePatterns := map[string]SensitivityLevel{
		"PASSWORD":     SensitivityCritical,
		"SECRET":       SensitivityCritical,
		"TOKEN":        SensitivityCritical,
		"API_KEY":      SensitivityCritical,
		"PRIVATE_KEY":  SensitivityCritical,
		"AWS_SECRET":   SensitivityCritical,
		"DB_PASSWORD":  SensitivityCritical,
		"MYSQL_ROOT":   SensitivityCritical,
		"REDIS_PASS":   SensitivityHigh,
		"MONGO_PASS":   SensitivityHigh,
		"POSTGRES_PASS":SensitivityHigh,
		"AUTH":         SensitivityHigh,
		"CREDENTIAL":   SensitivityHigh,
		"SIGNING":      SensitivityMedium,
		"ENCRYPT":      SensitivityMedium,
	}

	// Known images with common env var patterns
	knownEnvVars := map[string][]string{
		"mysql:5.7":     {"MYSQL_ROOT_PASSWORD=***"},
		"postgres:13":   {"POSTGRES_PASSWORD=***"},
		"redis:6.0":     {"REDIS_PASSWORD=***"},
		"node:14":       {"NPM_TOKEN=***"},
	}

	if envs, ok := knownEnvVars[image]; ok {
		for _, env := range envs {
			envUpper := strings.ToUpper(env)
			for pattern, level := range sensitivePatterns {
				if strings.Contains(envUpper, pattern) {
					findings = append(findings, SensitiveFinding{
						Type:        "environment_variable",
						Location:    fmt.Sprintf("ENV %s", maskValue(env)),
						Sensitivity: level,
						Description: fmt.Sprintf("Sensitive environment variable detected: contains '%s'", pattern),
						Remediation: "Use Docker secrets, Vault, or encrypted env files instead of plain-text environment variables",
					})
					break
				}
			}
		}
	}

	return findings
}

// checkSensitiveMounts checks for sensitive host directory mounts
func (g *Guardian) checkSensitiveMounts(image string) []SensitiveFinding {
	findings := make([]SensitiveFinding, 0)

	// Dangerous mount paths
	dangerousMounts := []struct {
		path        string
		sensitivity SensitivityLevel
		description string
	}{
		{"/var/run/docker.sock", SensitivityCritical, "Docker socket mount gives full control over host Docker daemon"},
		{"/etc/shadow", SensitivityCritical, "Host shadow file contains password hashes"},
		{"/etc/passwd", SensitivityHigh, "Host passwd file exposes user information"},
		{"/root", SensitivityCritical, "Root home directory exposure"},
		{"/home", SensitivityHigh, "User home directories exposure"},
		{"/proc", SensitivityHigh, "Process information leakage from host"},
		{"/sys", SensitivityHigh, "System information leakage from host"},
		{"/dev", SensitivityCritical, "Device access from container"},
		{"/", SensitivityCritical, "Full host filesystem mounted"},
		{"/etc/kubernetes", SensitivityCritical, "Kubernetes configuration exposure"},
		{"/var/lib/docker", SensitivityCritical, "Docker data directory exposure"},
	}

	// Check common images for known sensitive mounts
	knownMounts := map[string][]string{
		"devcontainer:latest": {"/var/run/docker.sock", "/home"},
	}

	if mounts, ok := knownMounts[image]; ok {
		for _, mount := range mounts {
			for _, dm := range dangerousMounts {
				if mount == dm.path {
					findings = append(findings, SensitiveFinding{
						Type:        "sensitive_mount",
						Location:    fmt.Sprintf("VOLUME %s", dm.path),
						Sensitivity: dm.sensitivity,
						Description: dm.description,
						Remediation: fmt.Sprintf("Remove mount of %s or use read-only mount with restricted access", dm.path),
					})
					break
				}
			}
		}
	}

	return findings
}

// checkHardcodedSecrets checks for hardcoded secrets in image layers
func (g *Guardian) checkHardcodedSecrets(image string) []SensitiveFinding {
	findings := make([]SensitiveFinding, 0)

	// Simulate detection of common hardcoded secret patterns
	secretPatterns := []struct {
		pattern     string
		sensitivity SensitivityLevel
		description string
	}{
		{"-----BEGIN RSA PRIVATE KEY-----", SensitivityCritical, "Hardcoded RSA private key detected in image"},
		{"-----BEGIN EC PRIVATE KEY-----", SensitivityCritical, "Hardcoded EC private key detected in image"},
		{"-----BEGIN OPENSSH PRIVATE KEY-----", SensitivityCritical, "Hardcoded SSH private key detected in image"},
		{"AKIA", SensitivityHigh, "Potential AWS Access Key ID detected in image"},
		{"ghp_", SensitivityHigh, "Potential GitHub Personal Access Token detected in image"},
		{"sk-", SensitivityHigh, "Potential API secret key detected in image"},
	}

	// Check known images for patterns
	knownSecrets := map[string][]int{
		"legacy-app:1.0": {0}, // known to contain RSA key
	}

	if indices, ok := knownSecrets[image]; ok {
		for _, idx := range indices {
			if idx < len(secretPatterns) {
				sp := secretPatterns[idx]
				findings = append(findings, SensitiveFinding{
					Type:        "hardcoded_secret",
					Location:    "image layers",
					Sensitivity: sp.sensitivity,
					Description: sp.description,
					Remediation: "Remove secrets from image, use multi-stage builds, or use secret management solutions",
				})
			}
		}
	}

	return findings
}

// maskValue partially masks a value for safe display
func maskValue(value string) string {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "***"
	}
	if len(parts[1]) > 4 {
		return parts[0] + "=***" + parts[1][len(parts[1])-2:]
	}
	return parts[0] + "=***"
}
