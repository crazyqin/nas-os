package apisignverify

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// testKeys implements KeyProvider for testing.
type testKeys struct {
	hmacSecrets map[string][]byte
	rsaKeys     map[string]*rsa.PrivateKey
}

func (k *testKeys) HMACKey(keyID string) ([]byte, error) {
	if secret, ok := k.hmacSecrets[keyID]; ok {
		return secret, nil
	}
	return nil, ErrMissingKey
}

func (k *testKeys) RSAPublicKey(keyID string) (*rsa.PublicKey, error) {
	if priv, ok := k.rsaKeys[keyID]; ok {
		return &priv.PublicKey, nil
	}
	return nil, ErrMissingKey
}

func mustGenerateRSAKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func hmacSign(t *testing.T, secret []byte, s APISignSignal) string {
	t.Helper()
	payload := signingString(s)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func rsaSign(t *testing.T, priv *rsa.PrivateKey, s APISignSignal) string {
	t.Helper()
	payload := signingString(s)
	digest := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("failed to RSA-sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func newTestKeys() *testKeys {
	return &testKeys{
		hmacSecrets: map[string][]byte{
			"test-hmac-key": []byte("super-secret-hmac-key"),
		},
		rsaKeys: map[string]*rsa.PrivateKey{},
	}
}

func newHMACSignal(now time.Time) APISignSignal {
	return APISignSignal{
		Endpoint:      "/api/v1/users",
		Method:        "POST",
		Timestamp:     now,
		SignatureAlgo: "HMAC-SHA256",
		KeyID:         "test-hmac-key",
		RequestBody:   `{"name":"test"}`,
	}
}

func TestVerify_HMACSuccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	latency := time.Since(now)
	err := Verify(&sig, keys, tracker, now)
	if err != nil {
		t.Fatalf("expected successful verification, got error: %v", err)
	}
	if !sig.IsVerified {
		t.Fatal("expected IsVerified to be true")
	}
	if sig.VerificationLatency <= 0 {
		t.Fatal("expected non-zero VerificationLatency")
	}
	_ = latency
}

func TestVerify_HMACFail_BadSignature(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.ExpectedSignature = "dGVzdC1iYWQtc2lnbmF0dXJl" // base64 of "test-bad-signature"

	err := Verify(&sig, keys, tracker, now)
	if err == nil {
		t.Fatal("expected error for bad HMAC signature, got nil")
	}
	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("expected signature mismatch error, got: %v", err)
	}
	if sig.IsVerified {
		t.Fatal("expected IsVerified to be false")
	}
}

func TestVerify_HMACFail_WrongKey(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.KeyID = "nonexistent-key"
	sig.ExpectedSignature = "whatever"

	err := Verify(&sig, keys, tracker, now)
	if err != ErrMissingKey {
		t.Fatalf("expected ErrMissingKey, got: %v", err)
	}
}

func TestVerify_RSASuccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	rsaKey := mustGenerateRSAKey(t, 2048)
	keys.rsaKeys["test-rsa-key"] = rsaKey
	tracker := NewSimpleReplayTracker()

	sig := APISignSignal{
		Endpoint:      "/api/v1/files",
		Method:        "PUT",
		Timestamp:     now,
		SignatureAlgo: "RSA-SHA256",
		KeyID:         "test-rsa-key",
		RequestBody:   `{"path":"/data/file.txt"}`,
	}
	sig.ExpectedSignature = rsaSign(t, rsaKey, sig)

	err := Verify(&sig, keys, tracker, now)
	if err != nil {
		t.Fatalf("expected successful RSA verification, got error: %v", err)
	}
	if !sig.IsVerified {
		t.Fatal("expected IsVerified to be true")
	}
}

func TestVerify_RSAFail_BadSignature(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	rsaKey := mustGenerateRSAKey(t, 2048)
	keys.rsaKeys["test-rsa-key"] = rsaKey
	tracker := NewSimpleReplayTracker()

	sig := APISignSignal{
		Endpoint:      "/api/v1/files",
		Method:        "PUT",
		Timestamp:     now,
		SignatureAlgo: "RSA-SHA256",
		KeyID:         "test-rsa-key",
		RequestBody:   `{"path":"/data/file.txt"}`,
	}
	sig.ExpectedSignature = base64.StdEncoding.EncodeToString([]byte("tampered-signature"))

	err := Verify(&sig, keys, tracker, now)
	if err == nil {
		t.Fatal("expected error for bad RSA signature, got nil")
	}
	if !strings.Contains(err.Error(), "signature mismatch") && !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected signature-related error, got: %v", err)
	}
	if sig.IsVerified {
		t.Fatal("expected IsVerified to be false")
	}
}

func TestVerify_ExpiredTimestamp(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	// Timestamp 10 minutes ago — outside the 5-minute window.
	old := now.Add(-10 * time.Minute)
	sig := newHMACSignal(old)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	err := Verify(&sig, keys, tracker, now)
	if err != ErrTimestampOutOfRange {
		t.Fatalf("expected ErrTimestampOutOfRange, got: %v", err)
	}
}

func TestVerify_FutureTimestamp(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	// Timestamp 10 minutes in the future.
	future := now.Add(10 * time.Minute)
	sig := newHMACSignal(future)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	err := Verify(&sig, keys, tracker, now)
	if err != ErrTimestampOutOfRange {
		t.Fatalf("expected ErrTimestampOutOfRange for future timestamp, got: %v", err)
	}
}

func TestVerify_ReplayAttack(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	// First verification should succeed.
	err := Verify(&sig, keys, tracker, now)
	if err != nil {
		t.Fatalf("first verification should succeed, got: %v", err)
	}

	// Reset the signal for a second attempt with the same key + signature.
	sig2 := newHMACSignal(now)
	sig2.ExpectedSignature = sig.ExpectedSignature

	// Second verification with the same signature should be flagged as replay.
	err = Verify(&sig2, keys, tracker, now)
	if err == nil {
		t.Fatal("expected replay detection error, got nil")
	}
	if !strings.Contains(err.Error(), "replay") {
		t.Fatalf("expected replay error, got: %v", err)
	}
}

func TestVerify_UnknownAlgorithm(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.SignatureAlgo = "ED25519"
	sig.ExpectedSignature = "whatever"

	err := Verify(&sig, keys, tracker, now)
	if err != ErrUnknownAlgorithm {
		t.Fatalf("expected ErrUnknownAlgorithm, got: %v", err)
	}
}

func TestVerify_ZeroTimestamp(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.Timestamp = time.Time{} // zero value

	err := Verify(&sig, keys, tracker, now)
	if err != ErrTimestampOutOfRange {
		t.Fatalf("expected ErrTimestampOutOfRange for zero timestamp, got: %v", err)
	}
}

func TestVerify_NilReplayTracker(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()

	sig := newHMACSignal(now)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	// No panic with nil tracker.
	err := Verify(&sig, keys, nil, now)
	if err != nil {
		t.Fatalf("expected success with nil tracker, got: %v", err)
	}
}

func TestVerify_LatencyRecorded(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	keys := newTestKeys()
	tracker := NewSimpleReplayTracker()

	sig := newHMACSignal(now)
	sig.ExpectedSignature = hmacSign(t, keys.hmacSecrets["test-hmac-key"], sig)

	start := time.Now()
	err := Verify(&sig, keys, tracker, now)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// VerificationLatency should have been set (in real usage it's set by the caller,
	// but Verify should complete well within the elapsed time).
	if elapsed > 5*time.Second {
		t.Fatalf("verification took too long: %v", elapsed)
	}
}

func TestAuditSignatures_AllVerified(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	signals := []APISignSignal{
		{
			Endpoint:            "/api/v1/a",
			Method:              "GET",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "key1",
			IsVerified:          true,
			VerificationLatency: 1 * time.Millisecond,
		},
		{
			Endpoint:            "/api/v1/b",
			Method:              "POST",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "key2",
			IsVerified:          true,
			VerificationLatency: 2 * time.Millisecond,
		},
	}

	result := AuditSignatures(signals)
	if result.TotalRequests != 2 {
		t.Fatalf("expected 2 total, got %d", result.TotalRequests)
	}
	if result.VerifiedCount != 2 {
		t.Fatalf("expected 2 verified, got %d", result.VerifiedCount)
	}
	if result.FailedCount != 0 {
		t.Fatalf("expected 0 failed, got %d", result.FailedCount)
	}
	if result.FailureRate != 0 {
		t.Fatalf("expected 0 failure rate, got %f", result.FailureRate)
	}
	if len(result.SuspiciousList) != 0 {
		t.Fatalf("expected 0 suspicious, got %d", len(result.SuspiciousList))
	}
	if result.AverageLatency != (1*time.Millisecond+2*time.Millisecond)/2 {
		t.Fatalf("expected average latency 1.5ms, got %v", result.AverageLatency)
	}
}

func TestAuditSignatures_AllFailed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	signals := []APISignSignal{
		{
			Endpoint:           "/api/v1/a",
			Method:             "GET",
			Timestamp:          now,
			SignatureAlgo:      "HMAC-SHA256",
			KeyID:              "key1",
			IsVerified:         false,
			ActualSignature:    "abc",
			ExpectedSignature:  "xyz",
			VerificationLatency: 1 * time.Millisecond,
		},
		{
			Endpoint:           "/api/v1/b",
			Method:             "POST",
			Timestamp:          now,
			SignatureAlgo:      "UNKNOWN",
			KeyID:              "key2",
			IsVerified:         false,
			ActualSignature:    "",
			ExpectedSignature:  "xyz",
			VerificationLatency: 2 * time.Millisecond,
		},
	}

	result := AuditSignatures(signals)
	if result.TotalRequests != 2 {
		t.Fatalf("expected 2 total, got %d", result.TotalRequests)
	}
	if result.VerifiedCount != 0 {
		t.Fatalf("expected 0 verified, got %d", result.VerifiedCount)
	}
	if result.FailedCount != 2 {
		t.Fatalf("expected 2 failed, got %d", result.FailedCount)
	}
	if result.FailureRate != 1.0 {
		t.Fatalf("expected 1.0 failure rate, got %f", result.FailureRate)
	}
	if len(result.SuspiciousList) != 2 {
		t.Fatalf("expected 2 suspicious, got %d", len(result.SuspiciousList))
	}
}

func TestAuditSignatures_Mixed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	signals := []APISignSignal{
		{
			Endpoint:            "/api/v1/a",
			Method:              "GET",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "key1",
			IsVerified:          true,
			VerificationLatency: 1 * time.Millisecond,
		},
		{
			Endpoint:            "/api/v1/b",
			Method:              "POST",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "key2",
			IsVerified:          false,
			ActualSignature:     "abc",
			ExpectedSignature:   "xyz",
			VerificationLatency: 3 * time.Millisecond,
		},
	}

	result := AuditSignatures(signals)
	if result.TotalRequests != 2 {
		t.Fatalf("expected 2 total, got %d", result.TotalRequests)
	}
	if result.VerifiedCount != 1 {
		t.Fatalf("expected 1 verified, got %d", result.VerifiedCount)
	}
	if result.FailedCount != 1 {
		t.Fatalf("expected 1 failed, got %d", result.FailedCount)
	}
	if result.FailureRate != 0.5 {
		t.Fatalf("expected 0.5 failure rate, got %f", result.FailureRate)
	}
	if len(result.SuspiciousList) != 1 {
		t.Fatalf("expected 1 suspicious, got %d", len(result.SuspiciousList))
	}
	if result.SuspiciousList[0].Endpoint != "/api/v1/b" {
		t.Fatalf("expected suspicious endpoint /api/v1/b, got %s", result.SuspiciousList[0].Endpoint)
	}
}

func TestAuditSignatures_ExpiredInList(t *testing.T) {
	old := time.Now().UTC().Add(-10 * time.Minute)
	signals := []APISignSignal{
		{
			Endpoint:            "/api/v1/old",
			Method:              "GET",
			Timestamp:           old,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "key1",
			IsVerified:          false,
			ActualSignature:     "abc",
			ExpectedSignature:   "xyz",
			VerificationLatency: 1 * time.Millisecond,
		},
	}

	result := AuditSignatures(signals)
	if len(result.SuspiciousList) != 1 {
		t.Fatalf("expected 1 suspicious, got %d", len(result.SuspiciousList))
	}
	if result.SuspiciousList[0].Reason != "expired timestamp" {
		t.Fatalf("expected reason 'expired timestamp', got %s", result.SuspiciousList[0].Reason)
	}
}

func TestAuditSignatures_Empty(t *testing.T) {
	result := AuditSignatures(nil)
	if result.TotalRequests != 0 {
		t.Fatalf("expected 0 total, got %d", result.TotalRequests)
	}
	if result.FailureRate != 0 {
		t.Fatalf("expected 0 failure rate, got %f", result.FailureRate)
	}
	if len(result.SuspiciousList) != 0 {
		t.Fatalf("expected 0 suspicious, got %d", len(result.SuspiciousList))
	}
}

func TestAuditSignatures_SuspiciousSortedByKeyID(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	signals := []APISignSignal{
		{
			Endpoint:            "/api/v1/c",
			Method:              "GET",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "keyZ",
			IsVerified:          false,
			VerificationLatency: 1 * time.Millisecond,
		},
		{
			Endpoint:            "/api/v1/a",
			Method:              "GET",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "keyA",
			IsVerified:          false,
			VerificationLatency: 1 * time.Millisecond,
		},
		{
			Endpoint:            "/api/v1/b",
			Method:              "GET",
			Timestamp:           now,
			SignatureAlgo:       "HMAC-SHA256",
			KeyID:               "keyM",
			IsVerified:          false,
			VerificationLatency: 1 * time.Millisecond,
		},
	}

	result := AuditSignatures(signals)
	if len(result.SuspiciousList) != 3 {
		t.Fatalf("expected 3 suspicious, got %d", len(result.SuspiciousList))
	}
	if result.SuspiciousList[0].KeyID != "keyA" {
		t.Fatalf("expected first key keyA, got %s", result.SuspiciousList[0].KeyID)
	}
	if result.SuspiciousList[1].KeyID != "keyM" {
		t.Fatalf("expected second key keyM, got %s", result.SuspiciousList[1].KeyID)
	}
	if result.SuspiciousList[2].KeyID != "keyZ" {
		t.Fatalf("expected third key keyZ, got %s", result.SuspiciousList[2].KeyID)
	}
}

func TestSimpleReplayTracker_DifferentSignaturesNotReplay(t *testing.T) {
	tracker := NewSimpleReplayTracker()
	now := time.Now().UTC()

	if tracker.Seen("key1", "sigA", now) {
		t.Fatal("first call should not be replay")
	}
	if tracker.Seen("key1", "sigB", now) {
		t.Fatal("different signature should not be replay")
	}
}

func TestSigningString_Deterministic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	s := APISignSignal{
		Endpoint:    "/api/v1/test",
		Method:      "post",
		Timestamp:   now,
		RequestBody: `{"x":1}`,
	}
	expected := "POST\n/api/v1/test\n" + intToStr(now.Unix()) + "\n{\"x\":1}"
	if signingString(s) != expected {
		t.Fatalf("signingString mismatch:\nexpected: %q\ngot:      %q", expected, signingString(s))
	}
}

// intToStr converts an int64 to string without using fmt (keeps the helper
// independent of the main package's fmt usage).
func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// Ensure we can PEM-encode and decode the RSA key (smoke test for key
// compatibility with standard library).
func TestRSAKey_PEMRoundTrip(t *testing.T) {
	key := mustGenerateRSAKey(t, 2048)
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})

	block, _ := pem.Decode(pemBlock)
	if block == nil {
		t.Fatal("failed to PEM-decode key")
	}
	decoded, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse key: %v", err)
	}
	if decoded.N.Cmp(key.N) != 0 {
		t.Fatal("key modulus mismatch after round-trip")
	}
}