package secureboot

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SignatureVerifier 签名验证器。
//
// 验证启动组件的数字签名，支持：
//   - X.509 证书签名验证
//   - SHA256 哈希验证
//   - 证书链验证（根 CA → 中间 CA → 叶证书）
//   - db/dbx 查询
type SignatureVerifier struct {
	keyManager *KeyManager
	logger     *zap.Logger
}

// NewSignatureVerifier 创建签名验证器。
func NewSignatureVerifier(km *KeyManager, logger *zap.Logger) *SignatureVerifier {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SignatureVerifier{
		keyManager: km,
		logger:     logger,
	}
}

// VerifySignature 验证签名数据。
//
// 检查流程：
//  1. 计算数据哈希
//  2. 在 dbx 中检查是否被吊销
//  3. 在 db 中查找匹配的证书
//  4. 验证证书链到受信任根
func (sv *SignatureVerifier) VerifySignature(data, signature []byte, cert *x509.Certificate) *VerificationResult {
	result := &VerificationResult{
		VerifiedAt: time.Now(),
	}

	if len(data) == 0 {
		result.Valid = false
		result.Reason = "数据为空"
		return result
	}

	if cert == nil {
		result.Valid = false
		result.Reason = "证书不能为空"
		return result
	}

	// 计算哈希
	hash := sha256.Sum256(data)
	result.SignerCN = cert.Subject.CommonName

	// 检查是否在 dbx（吊销列表）中
	certHash := hashCertificate(cert)
	if sv.keyManager.IsRevoked(certHash) {
		result.Valid = false
		result.Reason = "证书已被吊销（在 dbx 中）"
		return result
	}

	// 检查是否在 db（信任列表）中
	if !sv.isInTrustDB(cert) {
		result.Valid = false
		result.Reason = "证书不在信任数据库 (db) 中"
		return result
	}

	// 验证证书有效期
	if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		result.Valid = false
		result.Reason = "证书已过期或尚未生效"
		return result
	}

	// 验证签名（如果有签名数据）
	if len(signature) > 0 {
		if err := cert.CheckSignature(x509.SHA256WithRSA, hash[:], signature); err != nil {
			// 尝试 ECDSA
			if err2 := cert.CheckSignature(x509.ECDSAWithSHA256, hash[:], signature); err2 != nil {
				result.Valid = false
				result.Reason = fmt.Sprintf("签名验证失败：%v", err)
				return result
			}
		}
	}

	// 验证证书链
	chain, trusted := sv.verifyCertChain(cert)
	result.Chain = chain
	result.TrustedRoot = trusted

	result.Valid = true
	return result
}

// VerifyHash 验证数据哈希是否匹配。
func (sv *SignatureVerifier) VerifyHash(data []byte, expected [32]byte) bool {
	actual := sha256.Sum256(data)
	return actual == expected
}

// VerifyCertificate 验证单个证书的有效性。
func (sv *SignatureVerifier) VerifyCertificate(cert *x509.Certificate) *VerificationResult {
	result := &VerificationResult{
		VerifiedAt: time.Now(),
	}

	if cert == nil {
		result.Valid = false
		result.Reason = "证书为 nil"
		return result
	}

	result.SignerCN = cert.Subject.CommonName

	// 检查吊销状态
	certHash := hashCertificate(cert)
	if sv.keyManager.IsRevoked(certHash) {
		result.Valid = false
		result.Reason = "证书已被吊销"
		return result
	}

	// 检查有效期
	now := time.Now()
	if now.Before(cert.NotBefore) {
		result.Valid = false
		result.Reason = "证书尚未生效"
		return result
	}
	if now.After(cert.NotAfter) {
		result.Valid = false
		result.Reason = "证书已过期"
		return result
	}

	// 验证证书链
	chain, trusted := sv.verifyCertChain(cert)
	result.Chain = chain
	result.TrustedRoot = trusted

	if !trusted {
		result.Valid = false
		result.Reason = "证书链不可信"
		return result
	}

	result.Valid = true
	return result
}

// VerifyBootComponent 验证启动组件。
func (sv *SignatureVerifier) VerifyComponent(component *BootComponent) *ComponentResult {
	cr := &ComponentResult{
		Name: component.Name,
	}

	if len(component.Data) == 0 {
		cr.Valid = false
		cr.Reason = "组件数据为空"
		return cr
	}

	// 计算并验证哈希
	actualHash := sha256.Sum256(component.Data)
	if component.Hash != ([32]byte{}) && actualHash != component.Hash {
		cr.Valid = false
		cr.Reason = "哈希不匹配"
		return cr
	}

	cr.Valid = true
	return cr
}

// isInTrustDB 检查证书是否在信任数据库中。
func (sv *SignatureVerifier) isInTrustDB(cert *x509.Certificate) bool {
	dbEntries := sv.keyManager.ListKeys(KeyTypeDB)
	certHash := hashCertificate(cert)

	for _, entry := range dbEntries {
		if entry.Hash == certHash {
			return true
		}
		// 也检查是否由 db 中的证书签发
		if entry.Certificate != nil {
			if err := cert.CheckSignatureFrom(entry.Certificate); err == nil {
				return true
			}
		}
	}
	return false
}

// verifyCertChain 验证证书链。
//
// 返回值：
//   - chain: 证书链中的主体名称列表
//   - trusted: 是否链接到受信任的根
func (sv *SignatureVerifier) verifyCertChain(cert *x509.Certificate) ([]string, bool) {
	chain := []string{cert.Subject.CommonName}
	current := cert
	maxDepth := 10 // 防止无限循环

	for depth := 0; depth < maxDepth; depth++ {
		// 自签名证书 —— 检查是否是受信任的根
		if current.Issuer.CommonName == current.Subject.CommonName {
			// 检查是否在 db 中
			if sv.isInTrustDB(current) {
				return chain, true
			}
			// 检查是否是 PK
			pk := sv.keyManager.GetPK()
			if pk != nil && hashCertificate(pk) == hashCertificate(current) {
				return chain, true
			}
			return chain, false
		}

		// 尝试在 db 中查找签发者
		issuer := sv.findIssuer(current)
		if issuer == nil {
			return chain, false
		}

		// 验证签名
		if err := current.CheckSignatureFrom(issuer); err != nil {
			return chain, false
		}

		chain = append(chain, issuer.Subject.CommonName)
		current = issuer
	}

	return chain, false
}

// findIssuer 在密钥库中查找证书的签发者。
func (sv *SignatureVerifier) findIssuer(cert *x509.Certificate) *x509.Certificate {
	// 先查 PK
	pk := sv.keyManager.GetPK()
	if pk != nil && pk.Subject.CommonName == cert.Issuer.CommonName {
		if err := cert.CheckSignatureFrom(pk); err == nil {
			return pk
		}
	}

	// 查 KEK
	for _, entry := range sv.keyManager.ListKeys(KeyTypeKEK) {
		if entry.Certificate != nil && entry.Certificate.Subject.CommonName == cert.Issuer.CommonName {
			if err := cert.CheckSignatureFrom(entry.Certificate); err == nil {
				return entry.Certificate
			}
		}
	}

	// 查 db
	for _, entry := range sv.keyManager.ListKeys(KeyTypeDB) {
		if entry.Certificate != nil && entry.Certificate.Subject.CommonName == cert.Issuer.CommonName {
			if err := cert.CheckSignatureFrom(entry.Certificate); err == nil {
				return entry.Certificate
			}
		}
	}

	return nil
}

// ValidateCertChain 独立函数：验证证书链。
func ValidateCertChain(cert *x509.Certificate, roots *x509.CertPool, intermediates *x509.CertPool) error {
	if cert == nil {
		return errors.New("证书为 nil")
	}

	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	_, err := cert.Verify(opts)
	return err
}

// BuildCertPool 从 KeyEntry 列表构建证书池。
func BuildCertPool(entries []*KeyEntry) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, entry := range entries {
		if entry.Certificate != nil {
			pool.AddCert(entry.Certificate)
		}
	}
	return pool
}
