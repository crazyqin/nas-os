// Package passkey 测试
package passkey

import (
	"testing"
)

func TestNewPasskeyManager(t *testing.T) {
	rp := RelyingParty{ID: "nas.local", Name: "NAS-OS"}
	m := NewPasskeyManager(rp)
	if m == nil {
		t.Fatal("NewPasskeyManager returned nil")
	}
}

func TestBeginRegistration(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	ch, err := m.BeginRegistration("user1")
	if err != nil {
		t.Fatalf("BeginRegistration failed: %v", err)
	}
	if ch.ID == "" {
		t.Fatal("challenge ID empty")
	}
	if len(ch.Challenge) != 32 {
		t.Fatalf("expected 32-byte challenge, got %d", len(ch.Challenge))
	}
	if ch.RP.ID != "nas.local" {
		t.Fatalf("expected nas.local, got %s", ch.RP.ID)
	}
}

func TestCompleteRegistration(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	ch, _ := m.BeginRegistration("user1")

	cred, err := m.CompleteRegistration(ch.ID, "iPhone Touch ID", AuthenticatorPlatform, "iPhone 15", "iOS 18", []byte("fake-public-key"))
	if err != nil {
		t.Fatalf("CompleteRegistration failed: %v", err)
	}
	if cred.ID == "" {
		t.Fatal("credential ID empty")
	}
	if cred.UserID != "user1" {
		t.Fatalf("expected user1, got %s", cred.UserID)
	}
	if cred.Status != RegStatusVerified {
		t.Fatalf("expected verified, got %s", cred.Status)
	}

	creds := m.ListCredentials("user1")
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
}

func TestCompleteRegistrationChallengeNotFound(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})
	if _, err := m.CompleteRegistration("nonexistent", "key", AuthenticatorPlatform, "device", "os", []byte("pk")); err != ErrChallengeNotFound {
		t.Fatalf("expected ErrChallengeNotFound, got %v", err)
	}
}

func TestBeginAuthentication(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	// 先注册
	regCh, _ := m.BeginRegistration("user1")
	m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))

	// 开始认证
	authCh, err := m.BeginAuthentication("user1")
	if err != nil {
		t.Fatalf("BeginAuthentication failed: %v", err)
	}
	if len(authCh.Challenge) != 32 {
		t.Fatal("challenge length mismatch")
	}
	if len(authCh.AllowedCreds) != 1 {
		t.Fatalf("expected 1 allowed cred, got %d", len(authCh.AllowedCreds))
	}
}

func TestBeginAuthenticationUserNotFound(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})
	if _, err := m.BeginAuthentication("nonexistent"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestVerifyAuthentication(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	regCh, _ := m.BeginRegistration("user1")
	cred, _ := m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))

	authCh, _ := m.BeginAuthentication("user1")

	ok, err := m.VerifyAuthentication(authCh.ID, cred.ID, []byte("fake-signature"))
	if err != nil {
		t.Fatalf("VerifyAuthentication failed: %v", err)
	}
	if !ok {
		t.Fatal("expected verification success")
	}

	// 检查签名计数
	creds := m.ListCredentials("user1")
	if creds[0].SignCount != 1 {
		t.Fatalf("expected sign count 1, got %d", creds[0].SignCount)
	}
}

func TestVerifyAuthenticationEmptySignature(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	regCh, _ := m.BeginRegistration("user1")
	cred, _ := m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))

	authCh, _ := m.BeginAuthentication("user1")
	ok, err := m.VerifyAuthentication(authCh.ID, cred.ID, []byte{})
	if err != ErrVerificationFailed {
		t.Fatalf("expected ErrVerificationFailed, got %v", err)
	}
	if ok {
		t.Fatal("expected verification failure")
	}
}

func TestRevokeCredential(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	regCh, _ := m.BeginRegistration("user1")
	cred, _ := m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))

	if err := m.RevokeCredential(cred.ID); err != nil {
		t.Fatalf("RevokeCredential failed: %v", err)
	}

	creds := m.ListCredentials("user1")
	if creds[0].Status != RegStatusRevoked {
		t.Fatalf("expected revoked, got %s", creds[0].Status)
	}
}

func TestRevokeCredentialNotFound(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})
	if err := m.RevokeCredential("nonexistent"); err != ErrPasskeyNotFound {
		t.Fatalf("expected ErrPasskeyNotFound, got %v", err)
	}
}

func TestVerifyRevokedCredential(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	regCh, _ := m.BeginRegistration("user1")
	cred, _ := m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))
	m.RevokeCredential(cred.ID)

	authCh, _ := m.BeginAuthentication("user1")
	_, err := m.VerifyAuthentication(authCh.ID, cred.ID, []byte("sig"))
	if err != ErrPasskeyRevoked {
		t.Fatalf("expected ErrPasskeyRevoked, got %v", err)
	}
}

func TestGetEvents(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})

	regCh, _ := m.BeginRegistration("user1")
	cred, _ := m.CompleteRegistration(regCh.ID, "key", AuthenticatorPlatform, "dev", "os", []byte("pk"))
	m.RevokeCredential(cred.ID)

	events := m.GetEvents(10)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
}

func TestExportEvents(t *testing.T) {
	m := NewPasskeyManager(RelyingParty{ID: "nas.local", Name: "NAS-OS"})
	data, err := m.ExportEvents()
	if err != nil {
		t.Fatalf("ExportEvents failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}
}
