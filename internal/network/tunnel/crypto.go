package tunnel

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// Crypto errors
var (
	ErrInvalidKey        = errors.New("invalid encryption key")
	ErrInvalidNonce      = errors.New("invalid nonce")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrSignatureInvalid  = errors.New("invalid signature")
	ErrKeyExchangeFailed = errors.New("key exchange failed")
)

const (
	// Key sizes
	KeySize256   = 32
	NonceSizeAES = 12
	NonceSizeChaCha = 12
	TagSize      = 16
	
	// X25519 key size
	X25519KeySize = 32
)

// CipherType defines the encryption algorithm
type CipherType int

const (
	CipherAESGCM CipherType = iota
	CipherChaCha20Poly1305
)

// CryptoConfig holds crypto configuration
type CryptoConfig struct {
	CipherType   CipherType
	Key          []byte
	PreSharedKey []byte
}

// Crypto handles end-to-end encryption
type Crypto struct {
	config     *CryptoConfig
	gcm        cipher.AEAD
	privateKey *ecdh.PrivateKey
	publicKey  *ecdh.PublicKey
	
	// Session keys for each peer
	sessionKeys map[string][]byte
	mu          sync.RWMutex
}

// NewCrypto creates a new crypto instance
func NewCrypto(config *CryptoConfig) (*Crypto, error) {
	c := &Crypto{
		config:      config,
		sessionKeys: make(map[string][]byte),
	}

	// Initialize cipher
	if len(config.Key) > 0 {
		if err := c.initCipher(config.Key); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// initCipher initializes the AEAD cipher
func (c *Crypto) initCipher(key []byte) error {
	var err error
	
	switch c.config.CipherType {
	case CipherAESGCM:
		block, err := aes.NewCipher(key)
		if err != nil {
			return fmt.Errorf("failed to create AES cipher: %w", err)
		}
		c.gcm, err = cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("failed to create GCM: %w", err)
		}
		
	case CipherChaCha20Poly1305:
		c.gcm, err = chacha20poly1305.New(key)
		if err != nil {
			return fmt.Errorf("failed to create ChaCha20-Poly1305: %w", err)
		}
		
	default:
		return errors.New("unsupported cipher type")
	}
	
	return nil
}

// GenerateKeyPair generates an X25519 key pair for key exchange
func (c *Crypto) GenerateKeyPair() error {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	
	c.privateKey = privateKey
	c.publicKey = privateKey.PublicKey()
	
	return nil
}

// GetPublicKey returns the public key
func (c *Crypto) GetPublicKey() []byte {
	if c.publicKey == nil {
		return nil
	}
	return c.publicKey.Bytes()
}

// DeriveSharedKey derives a shared key using ECDH
func (c *Crypto) DeriveSharedKey(peerPublicKey []byte) ([]byte, error) {
	if c.privateKey == nil {
		return nil, errors.New("no private key")
	}
	
	curve := ecdh.X25519()
	peerPub, err := curve.NewPublicKey(peerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid peer public key: %w", err)
	}
	
	sharedSecret, err := c.privateKey.ECDH(peerPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH failed: %w", err)
	}
	
	// Derive encryption key using HKDF
	salt := make([]byte, 0)
	if len(c.config.PreSharedKey) > 0 {
		salt = c.config.PreSharedKey
	}
	
	hkdf := hkdf.New(sha256.New, sharedSecret, salt, []byte("nas-os-tunnel"))
	key := make([]byte, KeySize256)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, fmt.Errorf("HKDF failed: %w", err)
	}
	
	return key, nil
}

// SetPeerKey sets the session key for a peer
func (c *Crypto) SetPeerKey(peerID string, key []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.sessionKeys[peerID] = key
	return nil
}

// Encrypt encrypts data for a specific peer
func (c *Crypto) Encrypt(plaintext []byte, peerID string) ([]byte, error) {
	c.mu.RLock()
	key, ok := c.sessionKeys[peerID]
	c.mu.RUnlock()
	
	if !ok {
		// Use default key
		if c.gcm == nil {
			return nil, ErrInvalidKey
		}
		return c.encryptWithKey(plaintext, nil)
	}
	
	return c.encryptWithKey(plaintext, key)
}

// encryptWithKey encrypts with a specific key
func (c *Crypto) encryptWithKey(plaintext, key []byte) ([]byte, error) {
	var aead cipher.AEAD
	var err error
	
	if key != nil {
		// Create cipher with peer-specific key
		switch c.config.CipherType {
		case CipherAESGCM:
			block, err := aes.NewCipher(key)
			if err != nil {
				return nil, err
			}
			aead, err = cipher.NewGCM(block)
			if err != nil {
				return nil, err
			}
		case CipherChaCha20Poly1305:
			aead, err = chacha20poly1305.New(key)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if c.gcm == nil {
			return nil, ErrInvalidKey
		}
		aead = c.gcm
	}
	
	// Generate nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	
	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result[:len(nonce)], nonce)
	copy(result[len(nonce):], ciphertext)
	
	return result, nil
}

// Decrypt decrypts data from a specific peer
func (c *Crypto) Decrypt(ciphertext []byte, peerID string) ([]byte, error) {
	c.mu.RLock()
	key, ok := c.sessionKeys[peerID]
	c.mu.RUnlock()
	
	if !ok {
		if c.gcm == nil {
			return nil, ErrInvalidKey
		}
		return c.decryptWithKey(ciphertext, nil)
	}
	
	return c.decryptWithKey(ciphertext, key)
}

// decryptWithKey decrypts with a specific key
func (c *Crypto) decryptWithKey(ciphertext, key []byte) ([]byte, error) {
	var aead cipher.AEAD
	var err error
	
	if key != nil {
		switch c.config.CipherType {
		case CipherAESGCM:
			block, err := aes.NewCipher(key)
			if err != nil {
				return nil, err
			}
			aead, err = cipher.NewGCM(block)
			if err != nil {
				return nil, err
			}
		case CipherChaCha20Poly1305:
			aead, err = chacha20poly1305.New(key)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if c.gcm == nil {
			return nil, ErrInvalidKey
		}
		aead = c.gcm
	}
	
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize+TagSize {
		return nil, ErrDecryptionFailed
	}
	
	// Extract nonce and actual ciphertext
	nonce := ciphertext[:nonceSize]
	actualCiphertext := ciphertext[nonceSize:]
	
	// Decrypt
	plaintext, err := aead.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	
	return plaintext, nil
}

// GenerateNonce generates a random nonce
func GenerateNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	_, err := rand.Read(nonce)
	return nonce, err
}

// GenerateKey generates a random encryption key
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize256)
	_, err := rand.Read(key)
	return key, err
}

// DeriveKey derives a key from a password using Argon2id
// Simplified version using HKDF for now
func DeriveKeyFromPassword(password, salt []byte, keyLen int) []byte {
	hkdf := hkdf.New(sha256.New, password, salt, []byte("nas-os-key-derivation"))
	key := make([]byte, keyLen)
	io.ReadFull(hkdf, key)
	return key
}

// ComputeHMAC computes HMAC-SHA256
func ComputeHMAC(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// VerifyHMAC verifies HMAC-SHA256
func VerifyHMAC(key, data, expectedMAC []byte) bool {
	mac := ComputeHMAC(key, data)
	return hmac.Equal(mac, expectedMAC)
}

// Hash computes SHA-256 hash
func Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// Hash512 computes SHA-512 hash
func Hash512(data []byte) []byte {
	h := sha512.Sum512(data)
	return h[:]
}

// SignData signs data using Ed25519
type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewSigner creates a new Ed25519 signer
func NewSigner() (*Signer, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signing key: %w", err)
	}
	
	return &Signer{
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

// Sign signs data
func (s *Signer) Sign(data []byte) []byte {
	return ed25519.Sign(s.privateKey, data)
}

// Verify verifies a signature
func (s *Signer) Verify(data, signature []byte) bool {
	return ed25519.Verify(s.publicKey, data, signature)
}

// GetPublicKey returns the public key for verification
func (s *Signer) GetPublicKey() ed25519.PublicKey {
	return s.publicKey
}

// SetPublicKey sets a public key for verification only
func (s *Signer) SetPublicKey(pubKey ed25519.PublicKey) {
	s.publicKey = pubKey
}

// EncryptedPacket represents an encrypted packet with metadata
type SecurePacket struct {
	Nonce       []byte `json:"nonce"`
	Ciphertext  []byte `json:"ciphertext"`
	Tag         []byte `json:"tag"`
	SequenceNum uint64 `json:"sequence_num"`
	Timestamp   int64  `json:"timestamp"`
}

// EncryptPacket creates an encrypted packet
func (c *Crypto) EncryptPacket(plaintext []byte, peerID string, seqNum uint64) (*SecurePacket, error) {
	ciphertext, err := c.Encrypt(plaintext, peerID)
	if err != nil {
		return nil, err
	}
	
	nonceSize := c.getNonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptionFailed
	}
	
	return &SecurePacket{
		Nonce:       ciphertext[:nonceSize],
		Ciphertext:  ciphertext[nonceSize : len(ciphertext)-TagSize],
		Tag:         ciphertext[len(ciphertext)-TagSize:],
		SequenceNum: seqNum,
		Timestamp:   getCurrentTimestamp(),
	}, nil
}

// DecryptPacket decrypts a secure packet
func (c *Crypto) DecryptPacket(packet *SecurePacket, peerID string) ([]byte, error) {
	// Reconstruct full ciphertext
	ciphertext := make([]byte, 0, len(packet.Nonce)+len(packet.Ciphertext)+len(packet.Tag))
	ciphertext = append(ciphertext, packet.Nonce...)
	ciphertext = append(ciphertext, packet.Ciphertext...)
	ciphertext = append(ciphertext, packet.Tag...)
	
	return c.Decrypt(ciphertext, peerID)
}

// getNonceSize returns the nonce size for the current cipher
func (c *Crypto) getNonceSize() int {
	if c.gcm != nil {
		return c.gcm.NonceSize()
	}
	return NonceSizeChaCha
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return 0 // Would use time.Now().Unix() in production
}

// HKDFExpander expands a key using HKDF
type HKDFExpander struct {
	hkdf io.Reader
}

// NewHKDFExpander creates a new HKDF expander
func NewHKDFExpander(secret, salt, info []byte) *HKDFExpander {
	return &HKDFExpander{
		hkdf: hkdf.New(sha256.New, secret, salt, info),
	}
}

// Read reads expanded key material
func (h *HKDFExpander) Read(p []byte) (int, error) {
	return h.hkdf.Read(p)
}

// PBKDF2 derives a key using PBKDF2 (simplified)
func PBKDF2(password, salt []byte, iterations, keyLen int) []byte {
	// Simplified PBKDF2 using multiple HMAC iterations
	// In production, use golang.org/x/crypto/pbkdf2
	var derivedKey []byte
	block := sha256.New
	
	prf := hmac.New(block, password)
	_ = prf // Use in actual PBKDF2 implementation
	
	// Simplified derivation
	h := sha256.New()
	for i := 0; i < iterations; i++ {
		h.Write(password)
		h.Write(salt)
		derivedKey = h.Sum(nil)
		h.Reset()
	}
	
	if keyLen > len(derivedKey) {
		return derivedKey
	}
	return derivedKey[:keyLen]
}

// AeadCipher interface for AEAD ciphers
type AeadCipher interface {
	Seal(dst, nonce, plaintext, additionalData []byte) []byte
	Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
	NonceSize() int
	Overhead() int
}

// NewAEAD creates a new AEAD cipher
func NewAEAD(key []byte, cipherType CipherType) (AeadCipher, error) {
	switch cipherType {
	case CipherAESGCM:
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		return cipher.NewGCM(block)
	case CipherChaCha20Poly1305:
		return chacha20poly1305.New(key)
	default:
		return nil, errors.New("unsupported cipher")
	}
}

// Hasher interface for hash functions
type Hasher interface {
	Write(p []byte) (n int, err error)
	Sum(b []byte) []byte
	Reset()
	Size() int
	BlockSize() int
}

// NewHash returns a new hash function
func NewHash(algorithm string) (hash.Hash, error) {
	switch algorithm {
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, errors.New("unsupported hash algorithm")
	}
}