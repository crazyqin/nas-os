package emailoauth

import (
	"testing"
)

func TestSetConfig(t *testing.T) {
	n := NewMailNotifier()
	cfg := &OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client123",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	}
	if err := n.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	if !n.IsConfigured() {
		t.Error("expected to be configured")
	}
}

func TestSetConfigNil(t *testing.T) {
	n := NewMailNotifier()
	if err := n.SetConfig(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

func TestSetConfigInvalidProvider(t *testing.T) {
	n := NewMailNotifier()
	if err := n.SetConfig(&OAuthConfig{Provider: "yahoo", ClientID: "x", ClientSecret: "y"}); err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestSetConfigEmptyCredentials(t *testing.T) {
	n := NewMailNotifier()
	if err := n.SetConfig(&OAuthConfig{Provider: ProviderGmail}); err == nil {
		t.Error("expected error for empty credentials")
	}
}

func TestGetConfigHidesSecrets(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client123",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	cfg := n.GetConfig()
	if cfg.ClientSecret != "***" {
		t.Errorf("expected secret to be hidden, got %q", cfg.ClientSecret)
	}
	if cfg.RefreshToken != "***" {
		t.Errorf("expected refresh token to be hidden, got %q", cfg.RefreshToken)
	}
}

func TestIsTokenValidNotConfigured(t *testing.T) {
	n := NewMailNotifier()
	if n.IsTokenValid() {
		t.Error("expected token invalid when not configured")
	}
}

func TestRefreshToken(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client123",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	if err := n.RefreshToken(); err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if !n.IsTokenValid() {
		t.Error("expected token to be valid after refresh")
	}
}

func TestRefreshTokenNotConfigured(t *testing.T) {
	n := NewMailNotifier()
	if err := n.RefreshToken(); err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendMail(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderOutlook,
		ClientID:     "client",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	result, err := n.SendMail(&MailMessage{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Body:    "Hello",
	})
	if err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}
	if result.Provider != ProviderOutlook {
		t.Errorf("expected outlook provider, got %q", result.Provider)
	}
}

func TestSendMailNotConfigured(t *testing.T) {
	n := NewMailNotifier()
	_, err := n.SendMail(&MailMessage{To: []string{"user@example.com"}})
	if err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendMailNoRecipients(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	_, err := n.SendMail(&MailMessage{To: []string{}})
	if err == nil {
		t.Error("expected error for no recipients")
	}
}

func TestSendTestMail(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	result, err := n.SendTestMail("user@example.com", "Test", "Body")
	if err != nil {
		t.Fatalf("SendTestMail failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestSendTestMailEmptyRecipient(t *testing.T) {
	n := NewMailNotifier()
	if _, err := n.SendTestMail("", "", ""); err == nil {
		t.Error("expected error for empty recipient")
	}
}

func TestSendTestMailDefaults(t *testing.T) {
	n := NewMailNotifier()
	n.SetConfig(&OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		Method:       SendMethodOAuth2,
	})
	result, err := n.SendTestMail("user@example.com", "", "")
	if err != nil {
		t.Fatalf("SendTestMail failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success with defaults")
	}
}

func TestDefaultMethod(t *testing.T) {
	n := NewMailNotifier()
	cfg := &OAuthConfig{
		Provider:     ProviderGmail,
		ClientID:     "client",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		FromEmail:    "nas@example.com",
		// Method not set
	}
	n.SetConfig(cfg)
	got := n.GetConfig()
	if got.Method != SendMethodOAuth2 {
		t.Errorf("expected default method oauth2, got %q", got.Method)
	}
}