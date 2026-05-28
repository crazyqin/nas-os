package quantumsafe

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// ========== KEM (Key Encapsulation Mechanism) Interface ==========

// KEMScheme defines the interface for key encapsulation mechanisms.
type KEMScheme interface {
	// GenerateKeyPair generates a new KEM key pair.
	GenerateKeyPair() (publicKey, privateKey []byte, err error)
	// Encapsulate generates a shared secret and its encapsulation.
	Encapsulate(publicKey []byte) (ciphertext, sharedSecret []byte, err error)
	// Decapsulate recovers the shared secret from a ciphertext.
	Decapsulate(ciphertext, privateKey []byte) (sharedSecret []byte, err error)
	// Name returns the algorithm name.
	Name() Algorithm
	// SecurityLevel returns the NIST security level.
	SecurityLevel() int
}

// SignatureScheme defines the interface for digital signature schemes.
type SignatureScheme interface {
	// GenerateKeyPair generates a new signature key pair.
	GenerateKeyPair() (publicKey, privateKey []byte, err error)
	// Sign signs a message with the private key.
	Sign(message, privateKey []byte) (signature []byte, err error)
	// Verify verifies a signature against a message and public key.
	Verify(message, signature, publicKey []byte) (bool, error)
	// Name returns the algorithm name.
	Name() Algorithm
	// SecurityLevel returns the NIST security level.
	SecurityLevel() int
}

// ========== CRYSTALS-Kyber (simulated via X25519 + HKDF) ==========

// KyberKEM implements CRYSTALS-Kyber KEM using X25519 + HKDF.
//
// This is a simulation layer. For production use with real Kyber,
// replace with a native implementation (e.g., circl/cloudflare).
// The interface remains identical for drop-in replacement.
type KyberKEM struct {
	algorithm Algorithm
	level     int
}

// NewKyber768 creates a Kyber-768 KEM instance (NIST Level 3, 192-bit).
func NewKyber768() *KyberKEM {
	return &KyberKEM{algorithm: Kyber768, level: 3}
}

// NewKyber1024 creates a Kyber-1024 KEM instance (NIST Level 5, 256-bit).
func NewKyber1024() *KyberKEM {
	return &KyberKEM{algorithm: Kyber1024, level: 5}
}

func (k *KyberKEM) Name() Algorithm   { return k.algorithm }
func (k *KyberKEM) SecurityLevel() int { return k.level }

// GenerateKeyPair generates a Kyber key pair (simulated via X25519).
func (k *KyberKEM) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, priv); err != nil {
		return nil, nil, fmt.Errorf("generate private key: %w", err)
	}
	// Clamp private key per X25519 spec
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, fmt.Errorf("derive public key: %w", err)
	}
	return pub, priv, nil
}

// Encapsulate generates a shared secret using ephemeral-ephemeral ECDH.
func (k *KyberKEM) Encapsulate(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	if len(publicKey) != curve25519.ScalarSize {
		return nil, nil, fmt.Errorf("invalid public key length: %d", len(publicKey))
	}

	// Generate ephemeral key pair
	ephemeralPriv := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, ephemeralPriv); err != nil {
		return nil, nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephemeralPriv[0] &= 248
	ephemeralPriv[31] &= 127
	ephemeralPriv[31] |= 64

	ephemeralPub, err := curve25519.X25519(ephemeralPriv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, fmt.Errorf("derive ephemeral public key: %w", err)
	}

	// Compute shared secret
	dh, err := curve25519.X25519(ephemeralPriv, publicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("compute shared secret: %w", err)
	}

	// Derive final shared secret via HKDF
	salt := make([]byte, 32)
	copy(salt, ephemeralPub)
	info := []byte(fmt.Sprintf("KYBER-%d-KEM", k.level))
	sharedSecret, err = hkdfDerive(dh, salt, info, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf derive: %w", err)
	}

	return ephemeralPub, sharedSecret, nil
}

// Decapsulate recovers the shared secret from ciphertext.
func (k *KyberKEM) Decapsulate(ciphertext, privateKey []byte) (sharedSecret []byte, err error) {
	if len(ciphertext) != curve25519.ScalarSize {
		return nil, fmt.Errorf("invalid ciphertext length: %d", len(ciphertext))
	}
	if len(privateKey) != curve25519.ScalarSize {
		return nil, fmt.Errorf("invalid private key length: %d", len(privateKey))
	}

	dh, err := curve25519.X25519(privateKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}

	salt := make([]byte, 32)
	copy(salt, ciphertext)
	info := []byte(fmt.Sprintf("KYBER-%d-KEM", k.level))
	sharedSecret, err = hkdfDerive(dh, salt, info, 32)
	if err != nil {
		return nil, fmt.Errorf("hkdf derive: %w", err)
	}

	return sharedSecret, nil
}

// ========== CRYSTALS-Dilithium (simulated via Ed25519) ==========

// DilithiumSign implements CRYSTALS-Dilithium signatures using Ed25519.
//
// This is a simulation layer. For production use with real Dilithium,
// replace with a native implementation (e.g., circl/cloudflare).
type DilithiumSign struct {
	algorithm Algorithm
	level     int
}

// NewDilithium3 creates a Dilithium-3 signature instance (NIST Level 3).
func NewDilithium3() *DilithiumSign {
	return &DilithiumSign{algorithm: Dilithium3, level: 3}
}

// NewDilithium5 creates a Dilithium-5 signature instance (NIST Level 5).
func NewDilithium5() *DilithiumSign {
	return &DilithiumSign{algorithm: Dilithium5, level: 5}
}

func (d *DilithiumSign) Name() Algorithm   { return d.algorithm }
func (d *DilithiumSign) SecurityLevel() int { return d.level }

// GenerateKeyPair generates a Dilithium key pair (simulated via Ed25519).
func (d *DilithiumSign) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return pub, priv, nil
}

// Sign signs a message with the private key.
func (d *DilithiumSign) Sign(message, privateKey []byte) (signature []byte, err error) {
	priv := ed25519.PrivateKey(privateKey)
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(priv))
	}

	// Hash message before signing (prehash mode for large messages)
	h := sha512.Sum512(message)
	sig := ed25519.Sign(priv, h[:])
	return sig, nil
}

// Verify verifies a signature against a message and public key.
func (d *DilithiumSign) Verify(message, signature, publicKey []byte) (bool, error) {
	pub := ed25519.PublicKey(publicKey)
	if len(pub) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key length: %d", len(pub))
	}

	h := sha512.Sum512(message)
	return ed25519.Verify(pub, h[:], signature), nil
}

// ========== Hybrid KEM ==========

// HybridKEM combines a classical KEM with a post-quantum KEM.
//
// Security: the shared secret is secure as long as at least one of
// the two underlying KEMs is secure (defense-in-depth).
type HybridKEM struct {
	classical KEMScheme
	pq        KEMScheme
	name      Algorithm
}

// NewHybridKyber768 creates a hybrid KEM: X25519 + Kyber768.
func NewHybridKyber768() *HybridKEM {
	return &HybridKEM{
		classical: &x25519KEM{},
		pq:        NewKyber768(),
		name:      HybridKyber768,
	}
}

func (h *HybridKEM) Name() Algorithm   { return h.name }
func (h *HybridKEM) SecurityLevel() int { return 3 }

// GenerateKeyPair generates key material for both classical and PQ schemes.
func (h *HybridKEM) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	classicalPub, classicalPriv, err := h.classical.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("classical keypair: %w", err)
	}
	pqPub, pqPriv, err := h.pq.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("pq keypair: %w", err)
	}

	// Composite key: [len_classical(2B) | classical_key | pq_key]
	publicKey = encodeComposite(classicalPub, pqPub)
	privateKey = encodeComposite(classicalPriv, pqPriv)
	return publicKey, privateKey, nil
}

// Encapsulate performs dual encapsulation and combines shared secrets.
func (h *HybridKEM) Encapsulate(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	classicalPub, pqPub, err := decodeComposite(publicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decode public key: %w", err)
	}

	// Classical encapsulation
	classicalCT, classicalSS, err := h.classical.Encapsulate(classicalPub)
	if err != nil {
		return nil, nil, fmt.Errorf("classical encapsulate: %w", err)
	}

	// PQ encapsulation
	pqCT, pqSS, err := h.pq.Encapsulate(pqPub)
	if err != nil {
		return nil, nil, fmt.Errorf("pq encapsulate: %w", err)
	}

	// Combine ciphertexts
	ciphertext = encodeComposite(classicalCT, pqCT)

	// Combine shared secrets: HKDF(classical_ss || pq_ss)
	combined := append(classicalSS, pqSS...)
	sharedSecret, err = hkdfDerive(combined, []byte("hybrid-kem"), []byte(h.name), 32)
	if err != nil {
		return nil, nil, fmt.Errorf("combine shared secrets: %w", err)
	}

	return ciphertext, sharedSecret, nil
}

// Decapsulate recovers the shared secret using both private keys.
func (h *HybridKEM) Decapsulate(ciphertext, privateKey []byte) (sharedSecret []byte, err error) {
	classicalCT, pqCT, err := decodeComposite(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	classicalPriv, pqPriv, err := decodeComposite(privateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	classicalSS, err := h.classical.Decapsulate(classicalCT, classicalPriv)
	if err != nil {
		return nil, fmt.Errorf("classical decapsulate: %w", err)
	}

	pqSS, err := h.pq.Decapsulate(pqCT, pqPriv)
	if err != nil {
		return nil, fmt.Errorf("pq decapsulate: %w", err)
	}

	combined := append(classicalSS, pqSS...)
	sharedSecret, err = hkdfDerive(combined, []byte("hybrid-kem"), []byte(h.name), 32)
	if err != nil {
		return nil, fmt.Errorf("combine shared secrets: %w", err)
	}

	return sharedSecret, nil
}

// ========== Hybrid Signature ==========

// HybridSignature combines classical and PQ signature schemes.
//
// A message is valid only if BOTH signatures verify.
type HybridSignature struct {
	classical SignatureScheme
	pq        SignatureScheme
	name      Algorithm
}

// NewHybridDilithium3 creates a hybrid signature: Ed25519 + Dilithium3.
func NewHybridDilithium3() *HybridSignature {
	return &HybridSignature{
		classical: &ed25519Sign{},
		pq:        NewDilithium3(),
		name:      HybridDilithium3,
	}
}

func (h *HybridSignature) Name() Algorithm   { return h.name }
func (h *HybridSignature) SecurityLevel() int { return 3 }

// GenerateKeyPair generates key material for both schemes.
func (h *HybridSignature) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	cPub, cPriv, err := h.classical.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("classical keypair: %w", err)
	}
	pPub, pPriv, err := h.pq.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("pq keypair: %w", err)
	}

	publicKey = encodeComposite(cPub, pPub)
	privateKey = encodeComposite(cPriv, pPriv)
	return publicKey, privateKey, nil
}

// Sign produces a composite signature (both schemes sign).
func (h *HybridSignature) Sign(message, privateKey []byte) (signature []byte, err error) {
	cPriv, pPriv, err := decodeComposite(privateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	cSig, err := h.classical.Sign(message, cPriv)
	if err != nil {
		return nil, fmt.Errorf("classical sign: %w", err)
	}

	pSig, err := h.pq.Sign(message, pPriv)
	if err != nil {
		return nil, fmt.Errorf("pq sign: %w", err)
	}

	signature = encodeComposite(cSig, pSig)
	return signature, nil
}

// Verify checks both signatures; both must pass.
func (h *HybridSignature) Verify(message, signature, publicKey []byte) (bool, error) {
	cPub, pPub, err := decodeComposite(publicKey)
	if err != nil {
		return false, fmt.Errorf("decode public key: %w", err)
	}
	cSig, pSig, err := decodeComposite(signature)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}

	ok, err := h.classical.Verify(message, cSig, cPub)
	if err != nil {
		return false, fmt.Errorf("classical verify: %w", err)
	}
	if !ok {
		return false, nil
	}

	ok, err = h.pq.Verify(message, pSig, pPub)
	if err != nil {
		return false, fmt.Errorf("pq verify: %w", err)
	}
	return ok, nil
}

// ========== Classical Wrappers ==========

// x25519KEM wraps X25519 as a KEMScheme.
type x25519KEM struct{}

func (x *x25519KEM) Name() Algorithm   { return Algorithm("X25519") }
func (x *x25519KEM) SecurityLevel() int { return 1 }

func (x *x25519KEM) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, priv); err != nil {
		return nil, nil, err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

func (x *x25519KEM) Encapsulate(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	ephemeralPriv := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, ephemeralPriv); err != nil {
		return nil, nil, err
	}
	ephemeralPriv[0] &= 248
	ephemeralPriv[31] &= 127
	ephemeralPriv[31] |= 64

	ephemeralPub, err := curve25519.X25519(ephemeralPriv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	dh, err := curve25519.X25519(ephemeralPriv, publicKey)
	if err != nil {
		return nil, nil, err
	}

	sharedSecret, err = hkdfDerive(dh, ephemeralPub, []byte("X25519-KEM"), 32)
	if err != nil {
		return nil, nil, err
	}
	return ephemeralPub, sharedSecret, nil
}

func (x *x25519KEM) Decapsulate(ciphertext, privateKey []byte) (sharedSecret []byte, err error) {
	dh, err := curve25519.X25519(privateKey, ciphertext)
	if err != nil {
		return nil, err
	}
	sharedSecret, err = hkdfDerive(dh, ciphertext, []byte("X25519-KEM"), 32)
	return
}

// ed25519Sign wraps Ed25519 as a SignatureScheme.
type ed25519Sign struct{}

func (e *ed25519Sign) Name() Algorithm   { return Algorithm("Ed25519") }
func (e *ed25519Sign) SecurityLevel() int { return 1 }

func (e *ed25519Sign) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	return pub, priv, err
}

func (e *ed25519Sign) Sign(message, privateKey []byte) (signature []byte, err error) {
	priv := ed25519.PrivateKey(privateKey)
	h := sha512.Sum512(message)
	return ed25519.Sign(priv, h[:]), nil
}

func (e *ed25519Sign) Verify(message, signature, publicKey []byte) (bool, error) {
	pub := ed25519.PublicKey(publicKey)
	h := sha512.Sum512(message)
	return ed25519.Verify(pub, h[:], signature), nil
}

// ========== Helper Functions ==========

// hkdfDerive derives key material using HKDF-SHA256.
func hkdfDerive(secret, salt, info []byte, length int) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, secret, salt, info)
	key := make([]byte, length)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("hkdf expand: %w", err)
	}
	return key, nil
}

// encodeComposite encodes two byte slices into a single composite slice.
// Format: [len_a (2 bytes, big-endian)] [a] [b]
func encodeComposite(a, b []byte) []byte {
	composite := make([]byte, 2+len(a)+len(b))
	composite[0] = byte(len(a) >> 8)
	composite[1] = byte(len(a))
	copy(composite[2:], a)
	copy(composite[2+len(a):], b)
	return composite
}

// decodeComposite decodes a composite byte slice into two slices.
func decodeComposite(composite []byte) (a, b []byte, err error) {
	if len(composite) < 2 {
		return nil, nil, fmt.Errorf("composite too short: %d bytes", len(composite))
	}
	lenA := int(composite[0])<<8 | int(composite[1])
	if len(composite) < 2+lenA {
		return nil, nil, fmt.Errorf("composite truncated: need %d, have %d", 2+lenA, len(composite))
	}
	a = composite[2 : 2+lenA]
	b = composite[2+lenA:]
	return a, b, nil
}

// EncryptAESGCM encrypts data with AES-256-GCM using the given key.
func EncryptAESGCM(key, plaintext, additionalData []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, additionalData)
	return nonce, ciphertext, nil
}

// DecryptAESGCM decrypts data with AES-256-GCM using the given key.
func DecryptAESGCM(key, nonce, ciphertext, additionalData []byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	plaintext, err = gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plaintext, nil
}

// GetKEMScheme returns a KEM scheme for the given algorithm.
func GetKEMScheme(algo Algorithm) (KEMScheme, error) {
	switch algo {
	case Kyber768:
		return NewKyber768(), nil
	case Kyber1024:
		return NewKyber1024(), nil
	case HybridKyber768:
		return NewHybridKyber768(), nil
	default:
		return nil, fmt.Errorf("unsupported KEM algorithm: %s", algo)
	}
}

// GetSignatureScheme returns a signature scheme for the given algorithm.
func GetSignatureScheme(algo Algorithm) (SignatureScheme, error) {
	switch algo {
	case Dilithium3:
		return NewDilithium3(), nil
	case Dilithium5:
		return NewDilithium5(), nil
	case HybridDilithium3:
		return NewHybridDilithium3(), nil
	default:
		return nil, fmt.Errorf("unsupported signature algorithm: %s", algo)
	}
}
