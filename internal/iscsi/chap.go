package iscsi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// CHAPManager handles CHAP authentication for iSCSI targets
type CHAPManager struct {
	// In production, secrets would be stored securely (e.g., encrypted in database)
	secrets map[string]*CHAPConfig // targetID -> config
}

// NewCHAPManager creates a new CHAP manager
func NewCHAPManager() *CHAPManager {
	return &CHAPManager{
		secrets: make(map[string]*CHAPConfig),
	}
}

// ValidateInput validates CHAP input
func (cm *CHAPManager) ValidateInput(input *CHAPInput) error {
	if input == nil {
		return nil
	}

	if input.Enabled {
		if input.Username == "" {
			return fmt.Errorf("CHAP username is required when enabled")
		}
		if input.Secret == "" {
			return fmt.Errorf("CHAP secret is required when enabled")
		}
		if len(input.Secret) < 12 {
			return fmt.Errorf("CHAP secret must be at least 12 characters")
		}
		if len(input.Secret) > 16 {
			return fmt.Errorf("CHAP secret must be at most 16 characters")
		}

		if input.Mutual {
			if input.MutualUser == "" {
				return fmt.Errorf("mutual CHAP username is required")
			}
			if input.MutualSecret == "" {
				return fmt.Errorf("mutual CHAP secret is required")
			}
			if len(input.MutualSecret) < 12 || len(input.MutualSecret) > 16 {
				return fmt.Errorf("mutual CHAP secret must be 12-16 characters")
			}
		}
	}

	return nil
}

// CreateConfig creates CHAP configuration from input
func (cm *CHAPManager) CreateConfig(targetID string, input *CHAPInput) *CHAPConfig {
	if input == nil || !input.Enabled {
		return nil
	}

	config := &CHAPConfig{
		Enabled:      true,
		Username:     input.Username,
		Secret:       input.Secret,
		Mutual:       input.Mutual,
		MutualUser:   input.MutualUser,
		MutualSecret: input.MutualSecret,
	}

	cm.secrets[targetID] = config
	return config
}

// UpdateConfig updates CHAP configuration
func (cm *CHAPManager) UpdateConfig(targetID string, input *CHAPInput) *CHAPConfig {
	if input == nil {
		delete(cm.secrets, targetID)
		return nil
	}

	return cm.CreateConfig(targetID, input)
}

// GetConfig retrieves CHAP configuration (secret hidden)
func (cm *CHAPManager) GetConfig(targetID string) *CHAPConfig {
	config, exists := cm.secrets[targetID]
	if !exists {
		return nil
	}

	// Return copy with secret hidden
	return &CHAPConfig{
		Enabled:    config.Enabled,
		Username:   config.Username,
		Mutual:     config.Mutual,
		MutualUser: config.MutualUser,
		// Secrets not included for security
	}
}

// GetSecret retrieves the actual secret (for internal use)
func (cm *CHAPManager) GetSecret(targetID string) (username, secret string, ok bool) {
	config, exists := cm.secrets[targetID]
	if !exists || !config.Enabled {
		return "", "", false
	}
	return config.Username, config.Secret, true
}

// GetMutualSecret retrieves mutual CHAP credentials
func (cm *CHAPManager) GetMutualSecret(targetID string) (username, secret string, ok bool) {
	config, exists := cm.secrets[targetID]
	if !exists || !config.Enabled || !config.Mutual {
		return "", "", false
	}
	return config.MutualUser, config.MutualSecret, true
}

// DeleteConfig removes CHAP configuration
func (cm *CHAPManager) DeleteConfig(targetID string) {
	delete(cm.secrets, targetID)
}

// GenerateSecret generates a random CHAP secret
func GenerateSecret() (string, error) {
	bytes := make([]byte, 8) // 16 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateUserID generates a random user ID
func generateUserID() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ValidateIQN validates iSCSI Qualified Name format
func ValidateIQN(iqn string) error {
	if iqn == "" {
		return nil // Will be auto-generated
	}

	// IQN format: iqn.yyyy-mm.reverse-domain-name:identifier
	// Example: iqn.2024-03.com.example:nas-os.target1
	pattern := `^iqn\.\d{4}-\d{2}\.[a-z0-9.-]+:[a-z0-9.-]+$`
	matched, err := regexp.MatchString(pattern, strings.ToLower(iqn))
	if err != nil {
		return err
	}
	if !matched {
		return ErrInvalidIQN
	}
	return nil
}

// GenerateIQN generates an IQN from base domain and name
func GenerateIQN(baseDomain, name string) (string, error) {
	id, err := generateUserID()
	if err != nil {
		return "", err
	}

	// Format: iqn.yyyy-mm.reverse-domain:identifier
	// Reverse domain: com.example -> example.com
	parts := strings.Split(baseDomain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	reversedDomain := strings.Join(parts, ".")

	return fmt.Sprintf("iqn.%s.%s:%s-%s",
		"2024-03", // Version date
		reversedDomain,
		strings.ToLower(name),
		id), nil
}

// NormalizeIQN normalizes IQN to lowercase
func NormalizeIQN(iqn string) string {
	return strings.ToLower(iqn)
}

// HasAuth checks if target requires authentication
func (cm *CHAPManager) HasAuth(targetID string) bool {
	config, exists := cm.secrets[targetID]
	return exists && config.Enabled
}

// Authenticate verifies CHAP credentials
func (cm *CHAPManager) Authenticate(targetID, username, secret string) bool {
	config, exists := cm.secrets[targetID]
	if !exists || !config.Enabled {
		return true // No auth required
	}

	return config.Username == username && config.Secret == secret
}
