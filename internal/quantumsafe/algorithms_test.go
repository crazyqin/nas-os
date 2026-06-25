package quantumsafe

import (
	"testing"
)

func TestNewKyber768(t *testing.T) {
	kem := NewKyber768()
	if kem == nil {
		t.Fatal("expected non-nil KEM")
	}
	if kem.Name() != Kyber768 {
		t.Errorf("expected kyber768, got %s", kem.Name())
	}
}

func TestNewKyber1024(t *testing.T) {
	kem := NewKyber1024()
	if kem == nil {
		t.Fatal("expected non-nil KEM")
	}
	if kem.Name() != Kyber1024 {
		t.Errorf("expected kyber1024, got %s", kem.Name())
	}
}

func TestNewDilithium3(t *testing.T) {
	sig := NewDilithium3()
	if sig == nil {
		t.Fatal("expected non-nil signature")
	}
	if sig.Name() != Dilithium3 {
		t.Errorf("expected dilithium3, got %s", sig.Name())
	}
}

func TestNewHybridKyber768(t *testing.T) {
	kem := NewHybridKyber768()
	if kem == nil {
		t.Fatal("expected non-nil KEM")
	}
}

func TestNewHybridDilithium3(t *testing.T) {
	sig := NewHybridDilithium3()
	if sig == nil {
		t.Fatal("expected non-nil signature")
	}
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, quantum-safe world!")
	additionalData := []byte("test-aad")

	nonce, ciphertext, err := EncryptAESGCM(key, plaintext, additionalData)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if len(nonce) == 0 {
		t.Fatal("expected non-empty nonce")
	}
	if len(ciphertext) == 0 {
		t.Fatal("expected non-empty ciphertext")
	}

	decrypted, err := DecryptAESGCM(key, nonce, ciphertext, additionalData)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("expected %s, got %s", plaintext, decrypted)
	}
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	key := make([]byte, 32)
	wrongKey := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
		wrongKey[i] = byte(i + 1)
	}

	plaintext := []byte("secret")
	nonce, ciphertext, _ := EncryptAESGCM(key, plaintext, nil)

	_, err := DecryptAESGCM(wrongKey, nonce, ciphertext, nil)
	if err == nil {
		t.Error("expected error with wrong key")
	}
}

func TestGetKEMScheme(t *testing.T) {
	scheme, err := GetKEMScheme(Kyber768)
	if err != nil {
		t.Fatalf("GetKEMScheme failed: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}
}

func TestGetKEMScheme_Unknown(t *testing.T) {
	_, err := GetKEMScheme(Algorithm("Unknown"))
	if err == nil {
		t.Error("expected error for unknown algorithm")
	}
}

func TestGetSignatureScheme(t *testing.T) {
	scheme, err := GetSignatureScheme(Dilithium3)
	if err != nil {
		t.Fatalf("GetSignatureScheme failed: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}
}

func TestAlgorithm_Constants(t *testing.T) {
	tests := []struct {
		algo Algorithm
		want string
	}{
		{Kyber768, "kyber768"},
		{Kyber1024, "kyber1024"},
		{Dilithium3, "dilithium3"},
		{Dilithium5, "dilithium5"},
	}

	for _, tt := range tests {
		if string(tt.algo) != tt.want {
			t.Errorf("Algorithm %v = %s, want %s", tt.algo, string(tt.algo), tt.want)
		}
	}
}
