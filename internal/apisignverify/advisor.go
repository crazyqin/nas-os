package apisignverify

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// MaxTimestampSkew defines the allowable clock drift window for request
// timestamps. Requests with timestamps older than this window are rejected
// as expired or potential replay attacks.
const MaxTimestampSkew = 5 * time.Minute

// ErrTimestampOutOfRange indicates the request timestamp falls outside the
// acceptable skew window.
var ErrTimestampOutOfRange = errors.New("apisignverify: timestamp outside acceptable window")

// ErrSignatureMismatch indicates the computed signature does not match the
// expected signature.
var ErrSignatureMismatch = errors.New("apisignverify: signature mismatch")

// ErrUnknownAlgorithm indicates the signature algorithm is not supported.
var ErrUnknownAlgorithm = errors.New("apisignverify: unknown signature algorithm")

// ErrMissingKey indicates the signing key could not be resolved for the
// given KeyID.
var ErrMissingKey = errors.New("apisignverify: signing key not found")

// APISignSignal represents a single API request signing verification signal.
type APISignSignal struct {
	Endpoint           string
	Method             string
	Timestamp          time.Time
	SignatureAlgo      string // "HMAC-SHA256" or "RSA-SHA256"
	KeyID              string
	RequestBody        string
	ExpectedSignature  string
	ActualSignature    string
	IsVerified         bool
	VerificationLatency time.Duration
}

// KeyProvider resolves a signing key by its identifier. For HMAC this
// returns the shared secret; for RSA this returns the public key.
type KeyProvider interface {
	HMACKey(keyID string) ([]byte, error)
	RSAPublicKey(keyID string) (*rsa.PublicKey, error)
}

// ReplayTracker tracks previously seen (KeyID, Timestamp, Signature)
// tuples to detect replay attacks within the timestamp window.
type ReplayTracker interface {
	Seen(keyID, signature string, ts time.Time) bool
}

// SimpleReplayTracker is an in-memory ReplayTracker suitable for testing
// and single-instance deployments.
type SimpleReplayTracker struct {
	seen map[string]time.Time
}

// NewSimpleReplayTracker returns a new SimpleReplayTracker.
func NewSimpleReplayTracker() *SimpleReplayTracker {
	return &SimpleReplayTracker{seen: make(map[string]time.Time)}
}

// Seen records the tuple and returns true if it has already been observed
// within the valid window.
func (t *SimpleReplayTracker) Seen(keyID, signature string, ts time.Time) bool {
	key := keyID + "|" + signature
	if prev, ok := t.seen[key]; ok {
		if time.Since(prev) <= MaxTimestampSkew {
			return true
		}
	}
	t.seen[key] = time.Now()
	return false
}

// signingString builds the canonical signing payload from an APISignSignal.
func signingString(s APISignSignal) string {
	return fmt.Sprintf("%s\n%s\n%d\n%s",
		strings.ToUpper(s.Method),
		s.Endpoint,
		s.Timestamp.Unix(),
		s.RequestBody,
	)
}

// Verify validates the API request signature stored in s. It checks the
// timestamp window, detects replays, computes the expected signature using
// the appropriate algorithm, and performs a constant-time comparison.
func Verify(s *APISignSignal, keys KeyProvider, tracker ReplayTracker, now time.Time) error {
	// Timestamp window check (replay / expiry).
	if s.Timestamp.IsZero() {
		return ErrTimestampOutOfRange
	}
	if now.Sub(s.Timestamp) > MaxTimestampSkew || s.Timestamp.Sub(now) > MaxTimestampSkew {
		return ErrTimestampOutOfRange
	}

	// Replay detection.
	if tracker != nil && tracker.Seen(s.KeyID, s.ExpectedSignature, s.Timestamp) {
		return fmt.Errorf("apisignverify: replay detected for key %s", s.KeyID)
	}

	payload := signingString(*s)

	start := time.Now()

	switch strings.ToUpper(s.SignatureAlgo) {
	case "HMAC-SHA256":
		secret, err := keys.HMACKey(s.KeyID)
		if err != nil {
			return ErrMissingKey
		}
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(payload))
		computed := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		s.ActualSignature = computed
		if subtle.ConstantTimeCompare([]byte(computed), []byte(s.ExpectedSignature)) != 1 {
			s.VerificationLatency = time.Since(start)
			return ErrSignatureMismatch
		}

	case "RSA-SHA256":
		pub, err := keys.RSAPublicKey(s.KeyID)
		if err != nil {
			return ErrMissingKey
		}
		digest := sha256.Sum256([]byte(payload))
		sigBytes, err := base64.StdEncoding.DecodeString(s.ExpectedSignature)
		if err != nil {
			return fmt.Errorf("apisignverify: invalid base64 signature: %w", err)
		}
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sigBytes); err != nil {
			s.ActualSignature = ""
			s.VerificationLatency = time.Since(start)
			return ErrSignatureMismatch
		}
		s.ActualSignature = s.ExpectedSignature

	default:
		return ErrUnknownAlgorithm
	}

	s.IsVerified = true
	s.VerificationLatency = time.Since(start)
	return nil
}

// AuditResult summarises a batch of signature verification records.
type AuditResult struct {
	TotalRequests    int
	VerifiedCount    int
	FailedCount      int
	FailureRate      float64
	AverageLatency   time.Duration
	SuspiciousList   []SuspiciousRequest
}

// SuspiciousRequest highlights a request that failed verification or
// exhibits characteristics worthy of investigation.
type SuspiciousRequest struct {
	KeyID      string
	Endpoint   string
	Method     string
	Reason     string
	Timestamp  time.Time
}

// AuditSignatures analyses a slice of APISignSignal records and returns
// aggregate statistics including failure rate, average verification
// latency, and a list of suspicious requests.
func AuditSignatures(signals []APISignSignal) AuditResult {
	result := AuditResult{TotalRequests: len(signals)}
	var totalLatency time.Duration

	for _, s := range signals {
		totalLatency += s.VerificationLatency

		if s.IsVerified {
			result.VerifiedCount++
		} else {
			result.FailedCount++
			reason := "signature mismatch"
			if s.ActualSignature == "" {
				reason = "missing or unresolvable signature"
			}
			now := time.Now()
			if !s.Timestamp.IsZero() && now.Sub(s.Timestamp) > MaxTimestampSkew {
				reason = "expired timestamp"
			}
			if s.SignatureAlgo != "HMAC-SHA256" && s.SignatureAlgo != "RSA-SHA256" {
				reason = "unknown algorithm: " + s.SignatureAlgo
			}
			result.SuspiciousList = append(result.SuspiciousList, SuspiciousRequest{
				KeyID:     s.KeyID,
				Endpoint:  s.Endpoint,
				Method:    s.Method,
				Reason:    reason,
				Timestamp: s.Timestamp,
			})
		}
	}

	if result.TotalRequests > 0 {
		result.FailureRate = float64(result.FailedCount) / float64(result.TotalRequests)
		result.AverageLatency = totalLatency / time.Duration(result.TotalRequests)
	}

	// Sort suspicious list by KeyID then Endpoint for deterministic output.
	sort.Slice(result.SuspiciousList, func(i, j int) bool {
		if result.SuspiciousList[i].KeyID != result.SuspiciousList[j].KeyID {
			return result.SuspiciousList[i].KeyID < result.SuspiciousList[j].KeyID
		}
		return result.SuspiciousList[i].Endpoint < result.SuspiciousList[j].Endpoint
	})

	return result
}