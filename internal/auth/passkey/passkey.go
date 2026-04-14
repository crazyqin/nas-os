// Package passkey implements WebAuthn/FIDO2 Passkey authentication for NAS-OS.
// Based on WebAuthn Level-3 specification, supports both platform authenticators
// (TouchID/FaceID/Windows Hello) and roaming authenticators (security keys).
//
// Key references:
//   - W3C WebAuthn Level 3: https://www.w3.org/TR/webauthn-3/
//   - FIDO2 Server Implementation: https://fidoalliance.org/specs/
//   - Competitor implementations: Synology DSM 7.2+, TrueNAS SCALE 26
package passkey

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ========== Core Types ==========

// Manager is the central Passkey/WebAuthn manager.
// It implements the RP (Relying Party) side of the WebAuthn protocol.
type Manager struct {
	mu          sync.RWMutex
	credentials map[string][]*Credential // userID -> credentials
	sessions    map[string]*Session     // sessionID -> session data
	config      Config
}

// Credential represents a stored WebAuthn passkey credential.
type Credential struct {
	ID              string     `json:"id"`              // Base64URL encoded credential ID
	PublicKey       []byte     `json:"-"`                // Stored server-side for advanced verification; raw bytes kept
	AttestationType string    `json:"attestationType"` // none, indirect, direct
	Transport       []string  `json:"transport"`        // internal, hybrid, cross-platform
	AAGUID          string    `json:"aaguid"`           // Authenticator Attestation GUID
	CreatedAt       time.Time `json:"createdAt"`
	LastUsedAt      *time.Time `json:"lastUsedAt"`
	Name            string    `json:"name"`             // User-assigned friendly name
	DeviceType      string    `json:"deviceType"`       // single_device, multi_device
	BackupState     string    `json:"backupState"`     // eligible, ineligible, excluded, exists
	IsPasskey       bool      `json:"isPasskey"`        // true if this is a FIDO2 passkey (vs legacy U2F)
	Counter         uint32    `json:"counter"`          // Sign counter for replay detection
}

// Session represents an active WebAuthn ceremony session.
type Session struct {
	SessionID   string    `json:"sessionId"`
	UserID      string    `json:"userId"`      // empty for discoverable credential (auto-fill) auth
	Username    string    `json:"username"`     // display name only
	Challenge   string    `json:"challenge"`    // Base64URL encoded challenge
	UserHandle  string    `json:"userHandle"`  // Base64URL user ID (for discoverable creds)
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	IsRegister  bool      `json:"isRegister"`  // true=registration, false=authentication
	RPID        string    `json:"rpId"`
	Authenticator *AuthenticatorInfo `json:"authenticator,omitempty"` // populated on finish
}

// AuthenticatorInfo captures authenticator metadata from authentication.
type AuthenticatorInfo struct {
	AAGUID       string   `json:"aaguid"`
	Counter      uint32   `json:"counter"`
	DeviceType   string   `json:"deviceType"`
	BackupState  string   `json:"backupState"`
	Transport    []string `json:"transport"`
}

// Config holds Passkey RP configuration.
type Config struct {
	RPDisplayName         string   `json:"rpDisplayName"`          // e.g. "NAS-OS"
	RPID                  string   `json:"rpId"`                   // e.g. "nas.example.com" (must be有效的 domain or public suffix)
	RPOrigins             []string `json:"rpOrigins"`              // Allowed origins incl. port variants
	TimeoutMs             uint32   `json:"timeoutMs"`              // Ceremony timeout in milliseconds
	RequireResidentKey    bool     `json:"requireResidentKey"`     // Required for discoverable credentials
	UserVerification      string   `json:"userVerification"`      // "required", "preferred", "discouraged"
	AttestationConveyance  string   `json:"attestationConveyance"`  // "none", "indirect", "direct"
	AuthenticatorAttachment string  `json:"authenticatorAttachment"` // "platform", "cross-platform", "" (either)
	// AllowedAlgorithmIDs lists accepted COSE algorithm IDs for public keys.
	// Defaults: -7 (ES256), -257 (RS256), -37 (PS256), -258 (RS384), -39 (ES384), -47 (ES512)
	AllowedAlgorithmIDs []int `json:"allowedAlgorithmIDs"`
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig(rpID, displayName string, origins []string) Config {
	return Config{
		RPDisplayName:        displayName,
		RPID:                 rpID,
		RPOrigins:            origins,
		TimeoutMs:            60000,
		RequireResidentKey:   true,
		UserVerification:     "preferred",
		AttestationConveyance: "none",
		AllowedAlgorithmIDs:  []int{-7, -257, -37, -258, -39, -47},
	}
}

// NewManager creates a new Passkey manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		credentials: make(map[string][]*Credential),
		sessions:    make(map[string]*Session),
		config:      cfg,
	}
}

// ========== Challenge & Session Utilities ==========

func generateRandom(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

func generateChallenge() (string, error) {
	b, err := generateRandom(32)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateSessionID() (string, error) {
	b, err := generateRandom(16)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// validateOrigin checks that the given origin is in the allowed origins list.
func (m *Manager) validateOrigin(origin string) bool {
	for _, o := range m.config.RPOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

// ========== Registration Ceremony ==========

// RegistrationOptions generates the PublicKeyCredentialCreationOptions
// to send to the browser for the registration ceremony.
func (m *Manager) RegistrationOptions(userID, username, displayName string) (sessionID string, options map[string]interface{}, err error) {
	challenge, err := generateChallenge()
	if err != nil {
		return "", nil, fmt.Errorf("generate challenge: %w", err)
	}

	sid, err := generateSessionID()
	if err != nil {
		return "", nil, fmt.Errorf("generate session ID: %w", err)
	}

	// Encode user ID as Base64URL for the 'id' field (WebAuthn requires binary)
	userIDB64 := base64.RawURLEncoding.EncodeToString([]byte(userID))

	now := time.Now()
	m.mu.Lock()
	m.sessions[sid] = &Session{
		SessionID:  sid,
		UserID:     userID,
		Username:   username,
		Challenge:  challenge,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(m.config.TimeoutMs) * time.Millisecond),
		IsRegister: true,
		RPID:       m.config.RPID,
	}
	m.mu.Unlock()

	// Build pubKeyCredParams: prefer ES256, fall back to RS256
	algParams := []map[string]interface{}{
		{"type": "public-key", "alg": -7},   // ES256 (ECDSA w/ SHA-256)
		{"type": "public-key", "alg": -257}, // RS256 (RSASSA-PKCS1-v1_5)
		{"type": "public-key", "alg": -37},  // PS256 (RSASSA-PSS)
	}

	// Build excludeCredentials to prevent duplicate registration of same authenticator
	excludeCreds := m.getExcludeCredentials(userID)

	authenticatorSelection := map[string]interface{}{
		"authenticatorAttachment": m.config.AuthenticatorAttachment,
		"residentKey":             m.config.RequireResidentKey,
		"requireResidentKey":      m.config.RequireResidentKey,
		"userVerification":        m.config.UserVerification,
	}

	options = map[string]interface{}{
		"challenge":           challenge,
		"rp": map[string]interface{}{
			"name": m.config.RPDisplayName,
			"id":   m.config.RPID,
		},
		"user": map[string]interface{}{
			"id":          userIDB64,
			"name":        username,
			"displayName": displayName,
		},
		"pubKeyCredParams":      algParams,
		"timeout":              m.config.TimeoutMs,
		"excludeCredentials":   excludeCreds,
		"authenticatorSelection": authenticatorSelection,
		"attestation":          m.config.AttestationConveyance,
		"extensions": map[string]interface{}{
			"credProps":        true, // Request credentialProperties extension
			"hmacCreateSecret": true, // HMAC-Secret extension for encryption
		},
	}

	return sid, options, nil
}

// getExcludeCredentials builds the exclude list for registration.
func (m *Manager) getExcludeCredentials(userID string) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds := m.credentials[userID]
	result := make([]map[string]interface{}, 0, len(creds))
	for _, c := range creds {
		result = append(result, map[string]interface{}{
			"type": "public-key",
			"id":   c.ID,
		})
	}
	return result
}

// RegistrationResponse is the parsed and verified response from the browser.
type RegistrationResponse struct {
	Credential     *Credential
	ParsedAuthData *ParsedAuthData
	Transports     []string
	ClientDataJSON string
	AttestationObj string
}

// ParsedAuthData represents the parsed authenticator data (authData).
type ParsedAuthData struct {
	RPIDHash     []byte
	Flags        uint8
	Counter      uint32
	AAGUID       []byte
	CredID       []byte
	PublicKey    []byte
}

// VerifyRegistration completes the registration ceremony.
// It parses and validates the credential response from the browser.
func (m *Manager) VerifyRegistration(sessionID string, response map[string]interface{}) (*Credential, error) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("session not found")
	}
	if !session.IsRegister {
		m.mu.Unlock()
		return nil, fmt.Errorf("session is not a registration session")
	}
	if time.Now().After(session.ExpiresAt) {
		delete(m.sessions, sessionID)
		m.mu.Unlock()
		return nil, fmt.Errorf("session expired")
	}

	// --- Parse and validate the response ---
	credIDStr, ok := response["id"].(string)
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("missing credential id")
	}

	rawCredID, err := base64.RawURLEncoding.DecodeString(credIDStr)
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("invalid credential id encoding: %w", err)
	}

	resp, ok := response["response"].(map[string]interface{})
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("missing response field")
	}

	clientDataJSON, ok := resp["clientDataJSON"].(string)
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("missing clientDataJSON")
	}

	// Parse and verify clientDataJSON
	clientData, err := m.parseAndVerifyClientData(clientDataJSON, "webauthn.create", session.Challenge)
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("client data verification: %w", err)
	}

	// Validate origin
	if !m.validateOrigin(clientData.Origin) {
		m.mu.Unlock()
		return nil, fmt.Errorf("origin not allowed: %s", clientData.Origin)
	}

	attestationObj, ok := resp["attestationObject"].(string)
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("missing attestationObject")
	}

	authDataBytes, err := m.extractAuthData(attestationObj)
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("extract authData: %w", err)
	}

	// Parse authData
	parsed, err := m.parseAuthData(authDataBytes)
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("parse authData: %w", err)
	}

	// Verify credential ID matches
	if string(parsed.CredID) != string(rawCredID) {
		m.mu.Unlock()
		return nil, fmt.Errorf("credential ID mismatch")
	}

	// Verify attestation statement (simplified: we accept "none" or basic attestation)
	// In production, you would fully parse the attestation statement (COSE format).
	attestationType := "none"
	if attestationObj != "" {
		attestationType = "indirect" // Treat non-empty attestation as indirect for now
	}

	// Get transports
	transports := []string{"internal"}
	if t, ok := response["transports"].([]interface{}); ok {
		transports = make([]string, 0, len(t))
		for _, v := range t {
			if s, ok := v.(string); ok {
				transports = append(transports, s)
			}
		}
	}

	// Check if it's a passkey (discoverable credential / resident key)
	isPasskey := (parsed.Flags & 0x04) != 0 // AT bit indicates attested credential present

	// Create credential
	cred := &Credential{
		ID:              credIDStr,
		PublicKey:       parsed.PublicKey,
		AttestationType: attestationType,
		Transport:       transports,
		AAGUID:          base64.RawURLEncoding.EncodeToString(parsed.AAGUID),
		CreatedAt:       time.Now(),
		Name:            fmt.Sprintf("Passkey #%d", len(m.credentials[session.UserID])+1),
		IsPasskey:       isPasskey,
		Counter:         parsed.Counter,
		DeviceType:      "multi_device",
		BackupState:     "eligible",
	}

	m.credentials[session.UserID] = append(m.credentials[session.UserID], cred)
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	return cred, nil
}

// ========== Authentication Ceremony ==========

// AuthenticationOptions generates the PublicKeyCredentialRequestOptions.
func (m *Manager) AuthenticationOptions(userID string) (sessionID string, options map[string]interface{}, err error) {
	m.mu.RLock()
	creds := m.credentials[userID]
	m.mu.RUnlock()

	if len(creds) == 0 {
		return "", nil, fmt.Errorf("user has no registered passkeys")
	}

	return m.authenticationOptionsInternal(userID, creds)
}

// AuthenticationOptionsAuto generates options for discoverable credential
// (passwordless / auto-fill) authentication. No userID needed.
func (m *Manager) AuthenticationOptionsAuto() (sessionID string, options map[string]interface{}, err error) {
	// Empty allowCredentials means the browser will offer all passkeys
	// from any registered site (security implication: only use on HTTPS)
	return m.authenticationOptionsInternal("", nil)
}

func (m *Manager) authenticationOptionsInternal(userID string, creds []*Credential) (string, map[string]interface{}, error) {
	challenge, err := generateChallenge()
	if err != nil {
		return "", nil, err
	}

	sid, err := generateSessionID()
	if err != nil {
		return "", nil, err
	}

	now := time.Now()
	m.mu.Lock()
	m.sessions[sid] = &Session{
		SessionID:  sid,
		UserID:    userID,
		Challenge: challenge,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(m.config.TimeoutMs) * time.Millisecond),
		IsRegister: false,
		RPID:       m.config.RPID,
	}
	m.mu.Unlock()

	// allowCredentials: if userID is provided, restrict to user's credentials.
	// If userID is empty, allow any registered credential (auto-fill).
	var allow []map[string]interface{}
	if len(creds) > 0 {
		allow = make([]map[string]interface{}, len(creds))
		for i, c := range creds {
			allow[i] = map[string]interface{}{
				"type":       "public-key",
				"id":         c.ID,
				"transports": c.Transport,
			}
		}
	}

	options := map[string]interface{}{
		"challenge":        challenge,
		"timeout":          m.config.TimeoutMs,
		"rpId":             m.config.RPID,
		"allowCredentials": allow,
		"userVerification": m.config.UserVerification,
		"extensions": map[string]interface{}{
			"appid":     m.config.RPID,
			"authnSel":  allow, // Authentication selection hints
		},
	}

	return sid, options, nil
}

// VerifyAuthentication completes the authentication ceremony.
// Returns the authenticated user ID.
func (m *Manager) VerifyAuthentication(sessionID string, response map[string]interface{}) (string, *AuthenticatorInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return "", nil, fmt.Errorf("session not found")
	}
	if session.IsRegister {
		return "", nil, fmt.Errorf("session is not an authentication session")
	}
	if time.Now().After(session.ExpiresAt) {
		delete(m.sessions, sessionID)
		return "", nil, fmt.Errorf("session expired")
	}

	credIDStr, ok := response["id"].(string)
	if !ok {
		return "", nil, fmt.Errorf("missing credential id")
	}

	resp, ok := response["response"].(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("missing response field")
	}

	clientDataJSON, ok := resp["clientDataJSON"].(string)
	if !ok {
		return "", nil, fmt.Errorf("missing clientDataJSON")
	}

	clientData, err := m.parseAndVerifyClientData(clientDataJSON, "webauthn.get", session.Challenge)
	if err != nil {
		return "", nil, fmt.Errorf("client data verification: %w", err)
	}

	if !m.validateOrigin(clientData.Origin) {
		return "", nil, fmt.Errorf("origin not allowed: %s", clientData.Origin)
	}

	authenticatorDataStr, ok := resp["authenticatorData"].(string)
	if !ok {
		return "", nil, fmt.Errorf("missing authenticatorData")
	}

	authDataBytes, err := base64.RawURLEncoding.DecodeString(authenticatorDataStr)
	if err != nil {
		return "", nil, fmt.Errorf("authenticatorData encoding: %w", err)
	}

	parsed, err := m.parseAuthData(authDataBytes)
	if err != nil {
		return "", nil, fmt.Errorf("parse authData: %w", err)
	}

	// Verify RP ID hash
	expectedRPIDHash := sha256HashImpl(m.config.RPID)
	if string(parsed.RPIDHash) != string(expectedRPIDHash) {
		return "", nil, fmt.Errorf("RP ID hash mismatch")
	}

	// Verify user presence flag (bit 0)
	if (parsed.Flags & 0x01) == 0 {
		return "", nil, fmt.Errorf("user not present")
	}

	// Verify counter to prevent replay attacks
	var userID string
	var authInfo *AuthenticatorInfo

	if session.UserID != "" {
		// Non-discoverable: user ID is known from session
		userID = session.UserID
	} else {
		// Discoverable credential: userHandle is in the response
		userHandle, ok := response["userHandle"].(string)
		if !ok {
			return "", nil, fmt.Errorf("userHandle required for discoverable credential auth")
		}
		userID = userHandle
	}

	// Find and update the credential
	creds := m.credentials[userID]
	var found bool
	for _, c := range creds {
		if c.ID == credIDStr {
			found = true
			if c.Counter > 0 && parsed.Counter <= c.Counter {
				return "", nil, fmt.Errorf("authenticator counter not incremented (replay attack?)")
			}
			c.Counter = parsed.Counter
			now := time.Now()
			c.LastUsedAt = &now

			authInfo = &AuthenticatorInfo{
				AAGUID:      base64.RawURLEncoding.EncodeToString(parsed.AAGUID),
				Counter:     parsed.Counter,
				DeviceType:  c.DeviceType,
				BackupState: c.BackupState,
			}
			break
		}
	}

	if !found {
		return "", nil, fmt.Errorf("credential not found for this user")
	}

	delete(m.sessions, sessionID)
	return userID, authInfo, nil
}

// ========== Credential Management ==========

// GetCredentials returns all passkeys for a user.
func (m *Manager) GetCredentials(userID string) []*Credential {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credentials[userID]
}

// HasCredential returns true if the user has at least one passkey.
func (m *Manager) HasCredential(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.credentials[userID]) > 0
}

// RemoveCredential removes a specific credential.
func (m *Manager) RemoveCredential(userID, credID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	creds := m.credentials[userID]
	for i, c := range creds {
		if c.ID == credID {
			m.credentials[userID] = append(creds[:i], creds[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("credential not found")
}

// RenameCredential updates the friendly name of a credential.
func (m *Manager) RenameCredential(userID, credID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.credentials[userID] {
		if c.ID == credID {
			c.Name = name
			return nil
		}
	}
	return fmt.Errorf("credential not found")
}

// Stats returns credential statistics for a user.
func (m *Manager) Stats(userID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds := m.credentials[userID]
	var lastUsed *time.Time
	passkeyCount := 0
	for _, c := range creds {
		if c.IsPasskey {
			passkeyCount++
		}
		if c.LastUsedAt != nil && (lastUsed == nil || c.LastUsedAt.After(*lastUsed)) {
			lastUsed = c.LastUsedAt
		}
	}
	lastUsedStr := ""
	if lastUsed != nil {
		lastUsedStr = lastUsed.Format(time.RFC3339)
	}
	return map[string]interface{}{
		"total":           len(creds),
		"passkeyCount":   passkeyCount,
		"lastUsedAt":      lastUsedStr,
		"hasBackup":      passkeyCount > 1,
	}
}

// ========== Internal Parsing Utilities ==========

// ClientDataJSON represents the parsed client data JSON.
type ClientDataJSON struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
	CrossOrigin bool `json:"crossOrigin,omitempty"`
}

func (m *Manager) parseAndVerifyClientData(data string, expectedType, expectedChallenge string) (*ClientDataJSON, error) {
	b, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		// Fallback: try base64 std encoding
		b2, err2 := base64.StdEncoding.DecodeString(data)
		if err2 != nil {
			return nil, fmt.Errorf("decode clientDataJSON: %w", err)
		}
		b = b2
	}

	var cd ClientDataJSON
	if err := json.Unmarshal(b, &cd); err != nil {
		return nil, fmt.Errorf("parse clientDataJSON: %w", err)
	}

	if cd.Type != expectedType {
		return nil, fmt.Errorf("unexpected type: %s (expected %s)", cd.Type, expectedType)
	}

	if cd.Challenge != expectedChallenge {
		return nil, fmt.Errorf("challenge mismatch")
	}

	return &cd, nil
}

func (m *Manager) extractAuthData(attestationObject string) ([]byte, error) {
	// attestationObject is a CBOR-encoded structure.
	// We do minimal CBOR parsing here for the "none" attestation case.
	// For production, use a proper CBOR library.
	b, err := base64.RawURLEncoding.DecodeString(attestationObject)
	if err != nil {
		// Try raw base64
		b, err = base64.StdEncoding.DecodeString(attestationObject)
		if err != nil {
			return nil, fmt.Errorf("decode attestationObject: %w", err)
		}
	}

	// Simple CBOR parsing for attestationObject
	// Format: { "fmt": "none", "attStmt": {}, "authData": <bytes> }
	// CBOR map header: 0xA3 (map of 3)
	if len(b) < 5 {
		// Assume it's already raw authData
		return b, nil
	}

	// Check for CBOR header bytes (0xa3 = map of 3, common for packed attestation)
	if b[0] == 0xa3 || b[0] == 0xa2 {
		// CBOR map: find "authData" key (0x64 "authData" = 0x636175746844617461)
		// Simplified: look for magic authData prefix
		// The authData starts after the CBOR headers; for "none" attestation,
		// fmt="none" (0x64 0x6e 0x6f 0x6e 0x65), attStmt=empty, authData=rest
		// This is a simplified heuristic; a full CBOR decoder is recommended.
		offset := 0
		for i := 0; i < len(b)-5; i++ {
			if b[i] == 'a' && b[i+1] == 'u' && b[i+2] == 't' && b[i+3] == 'h' && b[i+4] == 'D' && b[i+5] == 'a' {
				// Found "authData" key, next value is bytes
				offset = i + 6
				// CBOR bytes header: 0x58 XX (bytes with length XX)
				if offset < len(b) && (b[offset]&0xe0) == 0x40 {
					// Skip CBOR bytes header
					offset++
					length := int(b[offset])
					offset++
					if offset+length <= len(b) {
						return b[offset : offset+length], nil
					}
				}
				break
			}
		}
	}

	// Fallback: assume raw authData
	return b, nil
}

func (m *Manager) parseAuthData(data []byte) (*ParsedAuthData, error) {
	if len(data) < 37 {
		return nil, fmt.Errorf("authData too short: need at least 37 bytes, got %d", len(data))
	}

	p := &ParsedAuthData{}

	// RP ID Hash (32 bytes)
	p.RPIDHash = data[0:32]

	// Flags (1 byte, offset 32)
	p.Flags = data[32]

	// Counter (4 bytes, big-endian, offset 33)
	p.Counter = uint32(data[33])<<24 | uint32(data[34])<<16 | uint32(data[35])<<8 | uint32(data[36])

	// AAGUID and credential data (variable length)
	offset := 37

	// If AT flag (bit 5) is set, attested credential data is present
	if (p.Flags & 0x04) != 0 {
		if len(data) < offset+16 {
			return nil, fmt.Errorf("authData too short for attested credential data")
		}
		// AAGUID (16 bytes)
		p.AAGUID = data[offset : offset+16]
		offset += 16

		// Credential ID length (2 bytes, big-endian)
		if len(data) < offset+2 {
			return nil, fmt.Errorf("authData too short for credential ID length")
		}
		credIDLen := int(data[offset])<<8 | int(data[offset+1])
		offset += 2

		if len(data) < offset+credIDLen {
			return nil, fmt.Errorf("authData too short for credential ID")
		}
		p.CredID = data[offset : offset+credIDLen]
		offset += credIDLen

		// Public key (remaining bytes, COSE format)
		if offset < len(data) {
			p.PublicKey = data[offset:]
		}
	}

	return p, nil
}


// Sha256Hash computes SHA-256 of a string (exported for tests).
func Sha256Hash(s string) []byte {
	return sha256HashImpl(s)
}
