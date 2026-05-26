package passkey

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Helper Functions ==========

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func generateTestCredentialID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig("localhost", "NAS-OS Test", []string{"http://localhost:8080"})
	mgr := NewManager(cfg)

	assert.NotNil(t, mgr)
	assert.Equal(t, "localhost", mgr.config.RPID)
	assert.Equal(t, "NAS-OS Test", mgr.config.RPDisplayName)
	assert.True(t, mgr.config.RequireResidentKey)
	assert.Equal(t, "preferred", mgr.config.UserVerification)
	assert.True(t, mgr.config.FallbackEnabled)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("nas.example.com", "My NAS", []string{"https://nas.example.com"})
	assert.Equal(t, "nas.example.com", cfg.RPID)
	assert.Equal(t, "My NAS", cfg.RPDisplayName)
	assert.Contains(t, cfg.RPOrigins, "https://nas.example.com")
	assert.Equal(t, uint32(60000), cfg.TimeoutMs)
	assert.Equal(t, 32, cfg.ChallengeLength)
	assert.Equal(t, uint32(300000), cfg.SessionTimeoutMs)
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

	// Create raw credential ID bytes
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)

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

	// Minimal authData: RP hash + flags + counter + AAGUID + credID_len + credID
	rpHash := sha256Hash("localhost")
	flags := byte(0x45) // UP + AT + UV
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16) // Zero AAGUID
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20} // Minimal COSE key

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	// Build attestationObject (simplified: just authData wrapped)
	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	// Verify registration
	cred, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)
	assert.NotNil(t, cred)
	assert.Equal(t, credID, cred.ID)
	assert.True(t, cred.IsPasskey)
	assert.Equal(t, []string{"internal"}, cred.Transport)

	// Verify credential was stored
	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 1)
	assert.Equal(t, credID, creds[0].ID)
}

// ========== Authentication Tests ==========

func TestAuthenticationOptions(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register a credential first
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	// Create minimal valid response
	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	mgr.VerifyRegistration(sessionID, response)

	// Test authentication options
	authSessionID, options, err := mgr.AuthenticationOptions("user-1")
	require.NoError(t, err)
	assert.NotEmpty(t, authSessionID)
	assert.NotNil(t, options)

	assert.Contains(t, options, "challenge")
	assert.Contains(t, options, "rpId")
	assert.Contains(t, options, "allowCredentials")
	assert.Contains(t, options, "userVerification")

	allowCreds := options["allowCredentials"].([]map[string]interface{})
	assert.Len(t, allowCreds, 1)
	assert.Equal(t, credID, allowCreds[0]["id"])
}

func TestAuthenticationOptionsAuto(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	sessionID, options, err := mgr.AuthenticationOptionsAuto()
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.NotNil(t, options)

	assert.Contains(t, options, "challenge")
	assert.Contains(t, options, "rpId")
	assert.Contains(t, options, "allowCredentials")

	// Auto should have empty allowCredentials
	allowCreds := options["allowCredentials"].([]map[string]interface{})
	assert.Empty(t, allowCreds)
}

func TestVerifyAuthentication(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register first
	regSessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	regChallenge := mgr.sessions[regSessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": regChallenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	regResponse := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(regSessionID, regResponse)
	require.NoError(t, err)

	// Now authenticate
	authSessionID, _, _ := mgr.AuthenticationOptions("user-1")
	authChallenge := mgr.sessions[authSessionID].Challenge

	authClientData := map[string]interface{}{
		"type":      "webauthn.get",
		"challenge": authChallenge,
		"origin":    "http://localhost:8080",
	}
	authClientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(authClientData))

	// Auth authData: RP hash + flags + counter (no attested credential data)
	authFlags := byte(0x05) // UP + UV
	authCounter := []byte{0, 0, 0, 2}

	authAuthData := make([]byte, 0, 37)
	authAuthData = append(authAuthData, rpHash...)
	authAuthData = append(authAuthData, authFlags)
	authAuthData = append(authAuthData, authCounter...)

	signature := []byte{0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x01} // Dummy signature

	authResponse := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    authClientDataJSON,
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authAuthData),
			"signature":         base64.RawURLEncoding.EncodeToString(signature),
			"userHandle":        "user-1",
		},
	}

	userID, authInfo, err := mgr.VerifyAuthentication(authSessionID, authResponse)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
	assert.NotNil(t, authInfo)
	assert.Equal(t, uint32(2), authInfo.Counter)
}

// ========== Device Management Tests ==========

func TestListDevices(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register a credential (which auto-creates a device)
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)

	// List devices
	devices := mgr.ListDevices("user-1")
	assert.Len(t, devices, 1)
	assert.Equal(t, "user-1", devices[0].UserID)
	assert.False(t, devices[0].Revoked)
}

func TestRevokeDevice(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register a credential
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)

	// Get device ID
	devices := mgr.ListDevices("user-1")
	require.Len(t, devices, 1)
	deviceID := devices[0].ID

	// Revoke device
	err = mgr.RevokeDevice("user-1", deviceID)
	require.NoError(t, err)

	// Verify device is revoked
	devices = mgr.ListDevices("user-1")
	assert.Len(t, devices, 0) // Should be empty since device is revoked

	// Verify credential was removed
	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 0)
}

func TestGetDevice(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// No devices initially
	devices := mgr.ListDevices("user-1")
	assert.Len(t, devices, 0)
}

// ========== Credential Management Tests ==========

func TestRemoveCredential(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)

	// Remove credential
	err = mgr.RemoveCredential("user-1", credID)
	require.NoError(t, err)

	// Verify removed
	creds := mgr.GetCredentials("user-1")
	assert.Len(t, creds, 0)
}

func TestRenameCredential(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)

	// Rename credential
	err = mgr.RenameCredential("user-1", credID, "My iPhone")
	require.NoError(t, err)

	creds := mgr.GetCredentials("user-1")
	require.Len(t, creds, 1)
	assert.Equal(t, "My iPhone", creds[0].Name)
}

func TestStats(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Register
	sessionID, _, _ := mgr.RegistrationOptions("user-1", "alice", "Alice Smith")
	rawCredID := make([]byte, 32)
	rand.Read(rawCredID)
	credID := base64.RawURLEncoding.EncodeToString(rawCredID)
	challenge := mgr.sessions[sessionID].Challenge

	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    "http://localhost:8080",
	}
	clientDataJSON := base64.RawURLEncoding.EncodeToString(mustJSON(clientData))

	rpHash := sha256Hash("localhost")
	flags := byte(0x45)
	counter := []byte{0, 0, 0, 1}
	aaguid := make([]byte, 16)
	
	credIDLen := []byte{byte(len(rawCredID) >> 8), byte(len(rawCredID))}
	publicKey := []byte{0xA5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}

	authData := make([]byte, 0, 37+16+2+len(rawCredID)+len(publicKey))
	authData = append(authData, rpHash...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, rawCredID...)
	authData = append(authData, publicKey...)

	attestationObj := base64.RawURLEncoding.EncodeToString(authData)

	response := map[string]interface{}{
		"id":   credID,
		"type": "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSON,
			"attestationObject": attestationObj,
		},
		"transports": []interface{}{"internal"},
	}

	_, err := mgr.VerifyRegistration(sessionID, response)
	require.NoError(t, err)

	// Get stats
	stats := mgr.Stats("user-1")
	assert.Equal(t, 1, stats["total"])
	assert.Equal(t, 1, stats["passkeyCount"])
	assert.Equal(t, 1, stats["deviceCount"])
	assert.Equal(t, false, stats["hasBackup"])
}

// ========== Error Cases ==========

func TestSessionNotFound(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	_, err := mgr.VerifyRegistration("nonexistent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionExpired(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Create session and manually expire it
	mgr.mu.Lock()
	sid := "test-session"
	mgr.sessions[sid] = &Session{
		SessionID:  sid,
		UserID:     "user-1",
		Challenge:  "test",
		IsRegister: true,
		ExpiresAt:  time.Now().Add(-1 * time.Hour), // Expired
	}
	mgr.mu.Unlock()

	_, err := mgr.VerifyRegistration(sid, map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session expired")
}

func TestNoPasskeysForAuth(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	_, _, err := mgr.AuthenticationOptions("user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user has no registered passkeys")
}

func TestRemoveNonexistentCredential(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	err := mgr.RemoveCredential("user-1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential not found")
}

func TestRenameNonexistentCredential(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	err := mgr.RenameCredential("user-1", "nonexistent", "Test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential not found")
}

// ========== Fallback Tests ==========

func TestFallbackPassword(t *testing.T) {
	cfg := DefaultConfig("localhost", "Test", []string{"http://localhost:8080"})
	cfg.FallbackEnabled = true
	mgr := NewManager(cfg)

	// Create session
	sessionID, _, _ := mgr.AuthenticationOptionsAuto()

	// Verify fallback password
	verifier := func(userID, password string) bool {
		return password == "correct-password"
	}

	// This will fail because we don't have a user ID in auto mode
	_, err := mgr.VerifyFallbackPassword(sessionID, "correct-password", verifier)
	assert.Error(t, err) // Expected: fallback requires known user ID
}

func TestFallbackDisabled(t *testing.T) {
	cfg := DefaultConfig("localhost", "Test", []string{"http://localhost:8080"})
	cfg.FallbackEnabled = false
	mgr := NewManager(cfg)

	_, err := mgr.VerifyFallbackPassword("session", "password", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

// ========== Edge Cases ==========

func TestConcurrentAccess(t *testing.T) {
	mgr := NewManager(DefaultConfig("localhost", "Test", []string{"http://localhost:8080"}))

	// Test concurrent registration options
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			userID := "user-" + string(rune('0'+i))
			_, _, err := mgr.RegistrationOptions(userID, "user", "User")
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all sessions were created
	mgr.mu.RLock()
	assert.Len(t, mgr.sessions, 10)
	mgr.mu.RUnlock()
}

func TestChallengeGeneration(t *testing.T) {
	c1, err := generateChallenge(32)
	require.NoError(t, err)

	c2, err := generateChallenge(32)
	require.NoError(t, err)

	assert.NotEmpty(t, c1)
	assert.NotEmpty(t, c2)
	assert.NotEqual(t, c1, c2) // Challenges should be unique
}

func TestSessionIDGeneration(t *testing.T) {
	s1, err := generateSessionID()
	require.NoError(t, err)

	s2, err := generateSessionID()
	require.NoError(t, err)

	assert.NotEmpty(t, s1)
	assert.NotEmpty(t, s2)
	assert.NotEqual(t, s1, s2) // Session IDs should be unique
}
