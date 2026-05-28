// Package zeroknowledge 零知识加密核心实现
// 刑部 - Zero Knowledge Crypto Core
package zeroknowledge

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

// ========== 密钥派生 ==========

// DeriveKeyFromPassword 使用指定算法从密码派生密钥.
// 客户端调用，服务端永远无法获取原始密码.
func DeriveKeyFromPassword(password string, salt []byte, cfg *ZKConfig) ([]byte, KeyDerivationAlgorithm, error) {
	if password == "" {
		return nil, "", fmt.Errorf("密码不能为空")
	}
	if len(salt) < SaltLength {
		return nil, "", fmt.Errorf("盐值长度不足，需要至少 %d 字节", SaltLength)
	}

	algo := cfg.DefaultKDF
	var key []byte

	switch algo {
	case KDFPBKDF2:
		key = pbkdf2.Key([]byte(password), salt, cfg.PBKDF2Iterations, KeyLength, sha256.New)
	case KDFArgon2:
		key = argon2.IDKey([]byte(password), salt, cfg.Argon2Iterations, cfg.Argon2Memory, cfg.Argon2Parallelism, KeyLength)
	default:
		return nil, "", fmt.Errorf("不支持的密钥派生算法: %s", algo)
	}

	return key, algo, nil
}

// DeriveKeyWithPBKDF2 使用 PBKDF2 派生密钥.
func DeriveKeyWithPBKDF2(password string, salt []byte, iterations int) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}
	if iterations <= 0 {
		iterations = DefaultPBKDF2Iterations
	}
	return pbkdf2.Key([]byte(password), salt, iterations, KeyLength, sha256.New), nil
}

// DeriveKeyWithArgon2 使用 Argon2id 派生密钥.
func DeriveKeyWithArgon2(password string, salt []byte, memory uint32, iterations uint32, parallelism uint8) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}
	if memory == 0 {
		memory = DefaultArgon2Memory
	}
	if iterations == 0 {
		iterations = DefaultArgon2Iterations
	}
	if parallelism == 0 {
		parallelism = DefaultArgon2Parallelism
	}
	return argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, KeyLength), nil
}

// ========== AES-256-GCM 加解密 ==========

// EncryptAES256GCM 使用 AES-256-GCM 加密数据.
// 返回格式: nonce(12字节) + ciphertext + tag(16字节).
func EncryptAES256GCM(plaintext, key []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, fmt.Errorf("密钥长度必须为 %d 字节", KeyLength)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// Seal 会将 nonce 附加到密文前面
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptAES256GCM 使用 AES-256-GCM 解密数据.
func DecryptAES256GCM(ciphertext, key []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, fmt.Errorf("密钥长度必须为 %d 字节", KeyLength)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("密文长度不足")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// EncryptDataWithKey 使用数据加密密钥(DEK)加密数据.
// DEK 本身使用用户密钥加密后存储.
func EncryptDataWithKey(plaintext, dek, userKey []byte) (encryptedData, encryptedDEK []byte, err error) {
	// 使用 DEK 加密数据
	encryptedData, err = EncryptAES256GCM(plaintext, dek)
	if err != nil {
		return nil, nil, fmt.Errorf("加密数据失败: %w", err)
	}

	// 使用用户密钥加密 DEK
	encryptedDEK, err = EncryptAES256GCM(dek, userKey)
	if err != nil {
		return nil, nil, fmt.Errorf("加密 DEK 失败: %w", err)
	}

	return encryptedData, encryptedDEK, nil
}

// DecryptDataWithKey 使用数据加密密钥(DEK)解密数据.
func DecryptDataWithKey(encryptedData, encryptedDEK, userKey []byte) ([]byte, error) {
	// 使用用户密钥解密 DEK
	dek, err := DecryptAES256GCM(encryptedDEK, userKey)
	if err != nil {
		return nil, fmt.Errorf("解密 DEK 失败: %w", err)
	}

	// 使用 DEK 解密数据
	plaintext, err := DecryptAES256GCM(encryptedData, dek)
	if err != nil {
		return nil, fmt.Errorf("解密数据失败: %w", err)
	}

	return plaintext, nil
}

// ========== Shamir 秘密分片 ==========

// ShamirSplit 将秘密分割为多个分片.
// threshold 为恢复所需的最小分片数，totalShares 为总分片数.
func ShamirSplit(secret []byte, threshold, totalShares int) ([]ShamirShare, error) {
	if threshold < MinShardShares {
		return nil, fmt.Errorf("阈值不能小于 %d", MinShardShares)
	}
	if totalShares < threshold {
		return nil, fmt.Errorf("总分片数不能小于阈值")
	}
	if totalShares > MaxShardShares {
		return nil, fmt.Errorf("总分片数不能超过 %d", MaxShardShares)
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("秘密不能为空")
	}

	// 对秘密的每个字节独立进行 Shamir 分片
	shareLen := len(secret)
	shares := make([]ShamirShare, totalShares)
	for i := range shares {
		shares[i].X = i + 1
		shares[i].Y = make([]byte, shareLen)
	}

	// 对每个字节位置生成多项式并计算分片
	for byteIdx := 0; byteIdx < shareLen; byteIdx++ {
		// 生成随机系数 (threshold-1 个)
		coefficients := make([]*big.Int, threshold)
		coefficients[0] = big.NewInt(int64(secret[byteIdx])) // a0 = secret byte

		prime := big.NewInt(257) // 使用大于 255 的素数
		for i := 1; i < threshold; i++ {
			coef, err := rand.Int(rand.Reader, prime)
			if err != nil {
				return nil, fmt.Errorf("生成随机系数失败: %w", err)
			}
			coefficients[i] = coef
		}

		// 计算每个分片在该字节位置的值
		for shareIdx := 0; shareIdx < totalShares; shareIdx++ {
			x := big.NewInt(int64(shares[shareIdx].X))
			y := evaluatePolynomial(coefficients, x, prime)
			shares[shareIdx].Y[byteIdx] = byte(y.Int64() & 0xFF)
		}
	}

	return shares, nil
}

// ShamirRecover 从分片恢复秘密.
func ShamirRecover(shares []ShamirShare) ([]byte, error) {
	if len(shares) < MinShardShares {
		return nil, fmt.Errorf("至少需要 %d 个分片来恢复秘密", MinShardShares)
	}

	// 验证所有分片长度一致
	shareLen := len(shares[0].Y)
	for i := 1; i < len(shares); i++ {
		if len(shares[i].Y) != shareLen {
			return nil, fmt.Errorf("分片长度不一致")
		}
	}

	prime := big.NewInt(257)
	secret := make([]byte, shareLen)

	// 对每个字节位置使用拉格朗日插值恢复
	for byteIdx := 0; byteIdx < shareLen; byteIdx++ {
		result := big.NewInt(0)

		for i, share := range shares {
			numerator := big.NewInt(1)
			denominator := big.NewInt(1)

			for j, otherShare := range shares {
				if i == j {
					continue
				}
				// numerator *= (0 - xj)
				xj := big.NewInt(int64(otherShare.X))
				negXj := new(big.Int).Neg(xj)
				negXj.Mod(negXj, prime)
				numerator.Mul(numerator, negXj)
				numerator.Mod(numerator, prime)

				// denominator *= (xi - xj)
				xi := big.NewInt(int64(share.X))
				diff := new(big.Int).Sub(xi, xj)
				diff.Mod(diff, prime)
				denominator.Mul(denominator, diff)
				denominator.Mod(denominator, prime)
			}

			// lagrange = numerator / denominator (mod prime)
			denomInv := new(big.Int).ModInverse(denominator, prime)
			if denomInv == nil {
				return nil, fmt.Errorf("模逆不存在，分片数据可能损坏")
			}
			lagrange := new(big.Int).Mul(numerator, denomInv)
			lagrange.Mod(lagrange, prime)

			// result += y_i * lagrange
			yi := big.NewInt(int64(share.Y[byteIdx]))
			term := new(big.Int).Mul(yi, lagrange)
			term.Mod(term, prime)
			result.Add(result, term)
			result.Mod(result, prime)
		}

		secret[byteIdx] = byte(result.Int64() & 0xFF)
	}

	return secret, nil
}

// evaluatePolynomial 计算多项式值: f(x) = a0 + a1*x + a2*x^2 + ... (mod p).
func evaluatePolynomial(coefficients []*big.Int, x, p *big.Int) *big.Int {
	result := big.NewInt(0)
	xPower := big.NewInt(1)

	for _, coef := range coefficients {
		term := new(big.Int).Mul(coef, xPower)
		term.Mod(term, p)
		result.Add(result, term)
		result.Mod(result, p)
		xPower.Mul(xPower, x)
		xPower.Mod(xPower, p)
	}

	return result
}

// ========== 辅助函数 ==========

// GenerateSalt 生成随机盐值.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("生成盐值失败: %w", err)
	}
	return salt, nil
}

// GenerateDEK 生成随机数据加密密钥.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("生成 DEK 失败: %w", err)
	}
	return dek, nil
}

// GenerateID 生成唯一 ID.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ComputeChecksum 计算 SHA-256 校验和.
func ComputeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(h[:])
}

// ZeroBytes 安全清零字节切片.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
