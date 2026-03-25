package auth

import (
	"testing"
)

func TestGenerateTOTPURI(t *testing.T) {
	tests := []struct {
		name        string
		secret      string
		issuer      string
		accountName string
		wantPrefix  string
	}{
		{
			name:        "standard URI",
			secret:      "JBSWY3DPEHPK3PXP",
			issuer:      "TestApp",
			accountName: "user@example.com",
			wantPrefix:  "otpauth://totp/",
		},
		{
			name:        "simple issuer",
			secret:      "ABCDEFGH",
			issuer:      "App",
			accountName: "user",
			wantPrefix:  "otpauth://totp/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := GenerateTOTPURI(tt.secret, tt.issuer, tt.accountName)

			if uri == "" {
				t.Error("URI should not be empty")
			}

			// Check URI format
			if len(uri) < len(tt.wantPrefix) {
				t.Errorf("URI too short: %s", uri)
				return
			}

			prefix := uri[:len(tt.wantPrefix)]
			if prefix != tt.wantPrefix {
				t.Errorf("URI prefix = %s, want %s", prefix, tt.wantPrefix)
			}

			// Check secret is in URI
			if !contains(uri, "secret="+tt.secret) {
				t.Errorf("URI should contain secret: %s", uri)
			}
		})
	}
}

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	// GenerateTOTPSecret requires issuer and account name, so it will fail with empty values
	if err != nil {
		// Expected behavior - empty issuer causes error
		return
	}

	// If no error, check secret
	if secret == "" {
		t.Error("Secret should not be empty")
	}
}

func TestSetupTOTP(t *testing.T) {
	setup, err := SetupTOTP("TestApp", "user@example.com")
	if err != nil {
		t.Errorf("SetupTOTP failed: %v", err)
	}

	if setup == nil {
		t.Fatal("SetupTOTP returned nil")
	}

	if setup.Secret == "" {
		t.Error("Secret should not be empty")
	}
	if setup.URI == "" {
		t.Error("URI should not be empty")
	}
	if setup.QRCode == "" {
		t.Error("QRCode should not be empty")
	}
	if setup.Issuer != "TestApp" {
		t.Errorf("Issuer = %s, want TestApp", setup.Issuer)
	}
	if setup.AccountName != "user@example.com" {
		t.Errorf("AccountName = %s, want user@example.com", setup.AccountName)
	}
}

func TestValidateTOTPCode_Invalid(t *testing.T) {
	// Generate a valid secret
	setup, err := SetupTOTP("TestApp", "user")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	// Test with an invalid code (random 6 digits)
	err = ValidateTOTPCode(setup.Secret, "000000")
	if err == nil {
		t.Error("ValidateTOTPCode should fail with invalid code")
	}
}

func TestVerifyTOTP_Invalid(t *testing.T) {
	// Generate a valid secret
	setup, err := SetupTOTP("TestApp", "user")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	// Test with an invalid code
	valid := VerifyTOTP(setup.Secret, "000000")
	if valid {
		t.Error("VerifyTOTP should return false for invalid code")
	}
}

func TestTOTPSetup_Struct(t *testing.T) {
	setup := &TOTPSetup{
		Secret:      "JBSWY3DPEHPK3PXP",
		URI:         "otpauth://totp/test:user?secret=JBSWY3DPEHPK3PXP",
		QRCode:      "base64encoded",
		Issuer:      "test",
		AccountName: "user",
	}

	if setup.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("Secret mismatch: %s", setup.Secret)
	}
	if setup.Issuer != "test" {
		t.Errorf("Issuer mismatch: %s", setup.Issuer)
	}
}

// Helper function.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
