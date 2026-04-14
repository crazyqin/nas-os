package passkey

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig("localhost", "NAS-OS Test", []string{"http://localhost:8080"})
	mgr := NewManager(cfg)

	assert.NotNil(t, mgr)
	assert.Equal(t, "localhost", mgr.config.RPID)
	assert.Equal(t, "NAS-OS Test", mgr.config.RPDisplayName)
	assert.True(t, mgr.config.RequireResidentKey)
	assert.Equal(t, "preferred", mgr.config.UserVerification)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("nas.example.com", "My NAS", []string{"https://nas.example.com"})
	assert.Equal(t, "nas.example.com", cfg.RPID)
	assert.Equal(t, "My NAS", cfg.RPDisplayName)
	assert.Contains(t, cfg.RPOrigins, "https://nas.example.com")
	assert.Equal(t, uint32(60000), cfg.TimeoutMs)
}

// ========== Registration Tests ==========

func TestRegistrationOptions(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	sessionID, options, err := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.NotNil(t, options)

	// Verify required fields
	assert.Contains(t, options, "challenge")
	assert.Contains(t, options, "rp")
	assert.Contains(t, options, "user")
	assert.Contains(t, options, "pubKeyCredParams")
	assert.Contains(t, options, "authenticatorSelection")
	assert.Contains(t, options, "timeout")
	assert.Contains(t, options, "attestation")

	// Verify RP
	rp := options["rp"].(map[string]interface{})
	assert.Equal(t, "Test", rp["name"])
	assert.Equal(t, "localhost", rp["id"])

	// Verify user
	user := options["user"].(map[string]interface{})
	assert.Equal(t, "alice", user["name"])
	assert.Equal(t, "Alice Smith", user["displayName"])

	// Verify pubKeyCredParams
	params := options["pubKeyCredParams"].([]map[string]interface{})
	assert.Len(t, params, 3) // ES256, RS256, PS256

	// Verify session was stored
	mgr.mu.RLock()
	sess, ok := mgr.sessions[sessionID]
	mgr.mu.RUnlock()
	assert.True(t, ok)
	assert.True(t, sess.IsRegister)
	assert.Equal(t, "user-1", sess.UserID)
}

func TestVerifyRegistration(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Generate registration options
	sessionID, _, err := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	require.NoError(t, err)

	// Simulate browser response
	credID := base64.RawURLEncoding.EncodeToString([]byte("test-cred-id-12345678"))
	challenge := ""
	mgr.mu.RLock()
	challenge = mgr.sessions[sessionID].Challenge
	mgr.mu.RUnlock()

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	// Minimal authData: 32-byte RP hash + flags + counter + AAGUID + credID_len + credID
	rpHash := make([]byte, 32) // simplified
	flags := byte(0x41)         // UP + AT flags
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	credIDBytes := []byte("test-cred-id-12345678")
	credIDLen := []byte{byte(len(credIDBytes) >> 8), byte(len(credIDBytes))}
	pubKey := []byte("mock-pub-key")
	authData := append(rpHash, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, credIDBytes...)
	authData = append(authData, pubKey...)

	response := map[string]interface{}{
		"id": credID,
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": base64.RawURLEncoding.EncodeToString(authData),
		},
		"transports": []interface{}{"internal"},
		"type":       "public-key",
	}

	cred, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)
	assert.Equal(t, credID, cred.ID)
	assert.True(t, cred.IsPasskey)
	assert.Equal(t, []string{"internal"}, cred.Transport)
	assert.Equal(t, "Passkey #1", cred.Name)

	// Verify credential is stored
	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 1)
}

func TestVerifyRegistrationSessionExpired(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	sessionID, _, err := mgr.RegistrationOptions("user-1", "alice", "Alice")
	require.NoError(t, err)

	// Expire the session manually
	mgr.mu.Lock()
	mgr.sessions[sessionID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	mgr.mu.Unlock()

	response := map[string]interface{}{
		"id": "test",
		"response": map[string]interface{}{
			"clientDataJSON":    "dGVzdA==",
			"attestationObject": "dGVzdA==",
		},
	}

	_, err = mgr.VerifyRegistration(sessionID, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestVerifyRegistrationSessionNotFound(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	_, err := mgr.VerifyRegistration("nonexistent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========== Authentication Tests ==========

func TestAuthenticationOptions(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// First register a credential
	registerCredential(t, mgr, "user-1", "alice", "Alice")

	// Now generate auth options
	sessionID, options, err := mgr.AuthenticationOptions("user-1")
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.Contains(t, options, "challenge")
	assert.Contains(t, options, "allowCredentials")
	assert.Contains(t, options, "rpId")

	allowCreds := options["allowCredentials"].([]map[string]interface{})
	assert.Len(t, allowCreds, 1)
}

func TestAuthenticationOptionsNoCredentials(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	_, _, err := mgr.AuthenticationOptions("user-no-creds")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no registered passkeys")
}

func TestAuthenticationOptionsAuto(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	sessionID, options, err := mgr.AuthenticationOptionsAuto()
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.Contains(t, options, "challenge")

	// allowCredentials should be nil/empty for auto-fill
	allowCreds := options["allowCredentials"]
	assert.Nil(t, allowCreds)
}

func TestVerifyAuthentication(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	cred := registerCredential(t, mgr, "user-1", "alice", "Alice")

	// Generate auth options
	sessionID, _, err := mgr.AuthenticationOptions("user-1")
	require.NoError(t, err)

	// Get challenge
	challenge := ""
	mgr.mu.RLock()
	challenge = mgr.sessions[sessionID].Challenge
	mgr.mu.Unlock()

	// Simulate browser auth response
	clientData := map[string]interface{}{
		"type":      "webauthn.get",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	// AuthData with only RP hash + flags + counter
	rpHash := make([]byte, 32)
	flags := byte(0x01) // UP only
	counter := []byte{0, 0, 0, 2}
	authData := append(rpHash, flags)
	authData = append(authData, counter...)

	response := map[string]interface{}{
		"id": cred.ID,
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
		},
	}

	userID, authInfo, err := mgr.VerifyAuthentication(sessionID, response)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
	assert.NotNil(t, authInfo)
}

func TestVerifyAuthenticationChallengeMismatch(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	registerCredential(t, mgr, "user-1", "alice", "Alice")

	sessionID, _, err := mgr.AuthenticationOptions("user-1")
	require.NoError(t, err)

	clientData := map[string]interface{}{
		"type":      "webauthn.get",
		"challenge": "wrong-challenge",
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := make([]byte, 32)
	flags := byte(0x01)
	counter := []byte{0, 0, 0, 2}
	authData := append(rpHash, flags)
	authData = append(authData, counter...)

	response := map[string]interface{}{
		"id": "test-cred",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":         "dGVzdA==",
		},
	}

	_, _, err = mgr.VerifyAuthentication(sessionID, response)
	assert.Error(t, err)
}

func TestVerifyAuthenticationOriginNotAllowed(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	registerCredential(t, mgr, "user-1", "alice", "Alice")

	sessionID, _, err := mgr.AuthenticationOptions("user-1")
	require.NoError(t, err)

	challenge := ""
	mgr.mu.RLock()
	challenge = mgr.sessions[sessionID].Challenge
	mgr.mu.RUnlock()

	clientData := map[string]interface{}{
		"type":      "webauthn.get",
		"challenge": challenge,
		"origin":    "https://evil.example.com",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := make([]byte, 32)
	flags := byte(0x01)
	counter := []byte{0, 0, 0, 2}
	authData := append(rpHash, flags)
	authData = append(authData, counter...)

	response := map[string]interface{}{
		"id": "test-cred",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":         "dGVzdA==",
		},
	}

	_, _, err = mgr.VerifyAuthentication(sessionID, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "origin")
}

// ========== Credential Management Tests ==========

func TestCredentialManagement(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register
	cred := registerCredential(t, mgr, "user-1", "alice", "Alice")
	assert.NotNil(t, cred)

	// HasCredential
	assert.True(t, mgr.HasCredential("user-1"))
	assert.False(t, mgr.HasCredential("user-999"))

	// GetCredentials
	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 1)

	// RenameCredential
	err := mgr.RenameCredential("user-1", cred.ID, "My YubiKey")
	require.NoError(t, err)
	creds = mgr.GetCredentials("user-1")
	assert.Equal(t, "My YubiKey", creds[0].Name)

	// Stats
	stats := mgr.Stats("user-1")
	assert.Equal(t, 1, stats["total"])

	// RemoveCredential
	err = mgr.RemoveCredential("user-1", cred.ID)
	require.NoError(t, err)
	assert.False(t, mgr.HasCredential("user-1"))

	// Remove non-existent
	err = mgr.RemoveCredential("user-1", "nonexistent")
	assert.Error(t, err)
}

func TestExcludeCredentials(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register first credential
	cred1 := registerCredential(t, mgr, "user-1", "alice", "Alice")
	assert.NotNil(t, cred1)

	// Register second credential - should have first in exclude list
	_, options, err := mgr.RegistrationOptions("user-1", "alice", "Alice")
	require.NoError(t, err)

	exclude := options["excludeCredentials"].([]map[string]interface{})
	assert.Len(t, exclude, 1)
	assert.Equal(t, cred1.ID, exclude[0]["id"])
}

func TestMultipleCredentials(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register two credentials
	cred1 := registerCredential(t, mgr, "user-1", "alice", "Alice")
	cred2 := registerCredential(t, mgr, "user-1", "alice", "Alice")

	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 2)
	assert.Equal(t, "Passkey #1", cred1.Name)
	assert.Equal(t, "Passkey #2", cred2.Name)
}

// ========== Session Tests ==========

func TestSessionCleanup(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	sessionID, _, err := mgr.RegistrationOptions("user-1", "alice", "Alice")
	require.NoError(t, err)

	// Session exists
	mgr.mu.RLock()
	_, ok := mgr.sessions[sessionID]
	mgr.mu.RUnlock()
	assert.True(t, ok)

	// Expire and verify cleanup
	mgr.mu.Lock()
	mgr.sessions[sessionID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	mgr.mu.Unlock()

	_, err = mgr.VerifyRegistration(sessionID, map[string]interface{}{
		"id":       "test",
		"response": map[string]interface{}{},
	})
	assert.Error(t, err)

	// Session should be cleaned up
	mgr.mu.RLock()
	_, ok = mgr.sessions[sessionID]
	mgr.mu.RUnlock()
	assert.False(t, ok)
}

// ========== Helpers ==========

func registerCredential(t *testing.T, mgr *Manager, userID, username, displayName string) *Credential {
	t.Helper()

	sessionID, _, err := mgr.RegistrationOptions(userID, username, displayName)
	require.NoError(t, err)

	// Get challenge from session
	challenge := ""
	mgr.mu.RLock()
	challenge = mgr.sessions[sessionID].Challenge
	mgr.mu.RUnlock()

	credIDBytes := make([]byte, 32)
	_, _ = rand.Read(credIDBytes)
	credID := base64.RawURLEncoding.EncodeToString(credIDBytes)

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	// Build authData with AT flag set
	rpHash := make([]byte, 32)
	flags := byte(0x45) // UP + AT + UV flags
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	credIDLen := []byte{byte(len(credIDBytes) >> 8), byte(len(credIDBytes))}
	pubKey := []byte("mock-pub-key-data")

	authData := append(rpHash, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, credIDBytes...)
	authData = append(authData, pubKey...)

	response := map[string]interface{}{
		"id": credID,
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": base64.RawURLEncoding.EncodeToString(authData),
		},
		"transports": []interface{}{"internal"},
		"type":       "public-key",
	}

	cred, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)
	return cred
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("json.Marshal failed: %v", err))
	}
	return b
}
