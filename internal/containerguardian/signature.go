package containerguardian

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// verifySignature verifies the digital signature of a container image.
// In production, this would integrate with Notary, Cosign, or other signing systems.
func (g *Guardian) verifySignature(ctx context.Context, image string) *SignatureResult {
	// Simulate signature verification logic
	// Images with explicit digests or from known signed registries are considered signed
	result := &SignatureResult{
		Status: SignatureUnknown,
	}

	// Check if image has a digest (sign of proper provenance)
	if strings.Contains(image, "@sha256:") {
		now := time.Now()
		result.Status = SignatureValid
		result.Signer = "cosign"
		result.SignedAt = &now
		result.Algorithm = "ECDSA-P256-SHA256"
		result.KeyID = "key-" + image[:min(8, len(image))]
		result.Fingerprint = "sha256:abcdef1234567890"
		return result
	}

	// Known unsigned images
	knownUnsigned := []string{"test:", "dev:", "debug:", "latest"}
	for _, tag := range knownUnsigned {
		if strings.HasSuffix(image, tag) || strings.Contains(image, tag) {
			result.Status = SignatureMissing
			result.Error = "image is not signed; consider using Cosign or Notary for signing"
			return result
		}
	}

	// Check if image is from a known signed registry pattern
	if strings.Contains(image, "ghcr.io/") || strings.Contains(image, "registry.access.redhat.com/") {
		now := time.Now()
		result.Status = SignatureValid
		result.Signer = "ghcr-attestation"
		result.SignedAt = &now
		result.Algorithm = "RSA-SHA256"
		result.KeyID = "github-actions"
		return result
	}

	// Default: assume unsigned for unknown images
	result.Status = SignatureMissing
	result.Error = fmt.Sprintf("no signature found for image %s", image)
	return result
}

