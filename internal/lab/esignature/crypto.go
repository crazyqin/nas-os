// Package esignature 提供电子签名功能
package esignature

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"sync"
	"time"
)

// CryptoEngine 加密引擎.
type CryptoEngine struct {
	mu        sync.RWMutex
	engine    *Engine
	certs     map[string]*CertKeyPair
	idCounter int64
}

// CertKeyPair 证书密钥对.
type CertKeyPair struct {
	Cert       *Certificate
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
}

// NewCryptoEngine 创建加密引擎.
func NewCryptoEngine(engine *Engine) *CryptoEngine {
	return &CryptoEngine{
		engine: engine,
		certs:  make(map[string]*CertKeyPair),
	}
}

// generateID 生成唯一ID.
func (ce *CryptoEngine) generateID(prefix string) string {
	ce.idCounter++
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + string(rune('A'+ce.idCounter%26))
}

// CreateCertificate 创建证书.
func (ce *CryptoEngine) CreateCertificate(req CreateCertRequest) (*Certificate, error) {
	if req.Name == "" {
		return nil, errors.New("名称不能为空")
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()

	algorithm := req.Algorithm
	if algorithm == "" {
		algorithm = "RSA"
	}

	validDays := req.ValidDays
	if validDays <= 0 {
		validDays = 365
	}

	var privateKey crypto.PrivateKey
	var publicKey crypto.PublicKey
	var err error

	switch algorithm {
	case "RSA":
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		publicKey = &privateKey.(*rsa.PrivateKey).PublicKey
	case "ECDSA":
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		publicKey = &privateKey.(*ecdsa.PrivateKey).PublicKey
	default:
		return nil, errors.New("不支持的算法")
	}

	// 序列化公钥
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cert := &Certificate{
		ID:        ce.generateID("cert"),
		UserID:    req.UserID,
		Name:      req.Name,
		Issuer:    req.Issuer,
		Subject:   req.Name,
		Serial:    ce.generateSerial(),
		NotBefore: now,
		NotAfter:  now.AddDate(0, 0, validDays),
		PublicKey: base64.StdEncoding.EncodeToString(publicKeyBytes),
		Algorithm: algorithm,
		IsCA:      req.IsCA,
		CreatedAt: now,
	}

	ce.certs[cert.ID] = &CertKeyPair{
		Cert:       cert,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}

	return cert, nil
}

// generateSerial 生成序列号.
func (ce *CryptoEngine) generateSerial() string {
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return serial.Text(16)
}

// GetCertificate 获取证书.
func (ce *CryptoEngine) GetCertificate(id string) (*Certificate, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	pair, ok := ce.certs[id]
	if !ok {
		return nil, errors.New("证书不存在")
	}
	return pair.Cert, nil
}

// ListCertificates 列出证书.
func (ce *CryptoEngine) ListCertificates(userID string) []*Certificate {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	result := make([]*Certificate, 0)
	for _, pair := range ce.certs {
		if userID == "" || pair.Cert.UserID == userID {
			result = append(result, pair.Cert)
		}
	}
	return result
}

// RevokeCertificate 吊销证书.
func (ce *CryptoEngine) RevokeCertificate(id string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	pair, ok := ce.certs[id]
	if !ok {
		return errors.New("证书不存在")
	}

	if pair.Cert.Revoked {
		return errors.New("证书已吊销")
	}

	now := time.Now()
	pair.Cert.Revoked = true
	pair.Cert.RevokedAt = &now

	return nil
}

// SignWithCertificate 使用证书签名.
func (ce *CryptoEngine) SignWithCertificate(req SignWithCertRequest) (*Signature, error) {
	ce.mu.RLock()
	pair, ok := ce.certs[req.CertID]
	ce.mu.RUnlock()

	if !ok {
		return nil, errors.New("证书不存在")
	}

	if pair.Cert.Revoked {
		return nil, errors.New("证书已吊销")
	}

	if time.Now().After(pair.Cert.NotAfter) {
		return nil, errors.New("证书已过期")
	}

	// 获取文档
	doc, err := ce.engine.GetDocument(req.DocumentID)
	if err != nil {
		return nil, err
	}

	// 计算文档哈希
	hash := sha256.Sum256([]byte(doc.Content))

	// 签名
	var signatureBytes []byte
	algorithm := req.Algorithm
	if algorithm == "" {
		algorithm = "SHA256withRSA"
	}

	switch algorithm {
	case "SHA256withRSA":
		rsaKey, ok := pair.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("证书不是RSA类型")
		}
		signatureBytes, err = rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hash[:])
		if err != nil {
			return nil, err
		}
	case "SHA256withECDSA":
		ecdsaKey, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("证书不是ECDSA类型")
		}
		signatureBytes, err = ecdsa.SignASN1(rand.Reader, ecdsaKey, hash[:])
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("不支持的签名算法")
	}

	// 创建时间戳
	timestamp := time.Now()

	signature := &Signature{
		ID:         ce.generateID("sig"),
		DocumentID: req.DocumentID,
		Type:       "digital",
		Data:       base64.StdEncoding.EncodeToString(signatureBytes),
		CertID:     req.CertID,
		Algorithm:  algorithm,
		Hash:       hex.EncodeToString(hash[:]),
		Timestamp:  timestamp,
	}

	// 存储签名到文档
	ce.engine.mu.Lock()
	if doc, ok := ce.engine.documents[req.DocumentID]; ok {
		doc.Signatures = append(doc.Signatures, *signature)
	}
	ce.engine.mu.Unlock()

	// 添加审计记录
	ce.engine.addAudit(req.DocumentID, pair.Cert.UserID, "sign_digital",
		"数字签名 - 算法: "+algorithm, "", "")

	return signature, nil
}

// VerifySignature 验证签名.
func (ce *CryptoEngine) VerifySignature(req VerifySignatureRequest) (*VerifyResult, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// 查找签名
	var signature *Signature
	var doc *Document

	for _, d := range ce.engine.documents {
		for _, sig := range d.Signatures {
			if sig.ID == req.SignatureID {
				signature = &sig
				doc = d
				break
			}
		}
		if signature != nil {
			break
		}
	}

	if signature == nil {
		return nil, errors.New("签名不存在")
	}

	result := &VerifyResult{
		SignatureID: signature.ID,
		SignerID:    signature.SignerID,
		Timestamp:   signature.Timestamp,
	}

	// 验证证书
	if signature.CertID != "" {
		pair, ok := ce.certs[signature.CertID]
		if !ok {
			result.Valid = false
			result.Reason = "证书不存在"
			return result, nil
		}

		if pair.Cert.Revoked {
			result.Valid = false
			result.CertValid = false
			result.Reason = "证书已吊销"
			return result, nil
		}

		if time.Now().After(pair.Cert.NotAfter) {
			result.Valid = false
			result.CertValid = false
			result.CertExpiry = pair.Cert.NotAfter
			result.Reason = "证书已过期"
			return result, nil
		}

		result.CertValid = true
		result.CertExpiry = pair.Cert.NotAfter

		// 验证签名
		hash := sha256.Sum256([]byte(doc.Content))
		sigBytes, err := base64.StdEncoding.DecodeString(signature.Data)
		if err != nil {
			result.Valid = false
			result.Reason = "签名数据无效"
			return result, nil
		}

		switch signature.Algorithm {
		case "SHA256withRSA":
			rsaKey, ok := pair.PublicKey.(*rsa.PublicKey)
			if !ok {
				result.Valid = false
				result.Reason = "公钥类型错误"
				return result, nil
			}
			err = rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, hash[:], sigBytes)
		case "SHA256withECDSA":
			ecdsaKey, ok := pair.PublicKey.(*ecdsa.PublicKey)
			if !ok {
				result.Valid = false
				result.Reason = "公钥类型错误"
				return result, nil
			}
			if !ecdsa.VerifyASN1(ecdsaKey, hash[:], sigBytes) {
				err = errors.New("ECDSA签名验证失败")
			}
		default:
			result.Valid = false
			result.Reason = "不支持的算法"
			return result, nil
		}

		if err != nil {
			result.Valid = false
			result.Reason = "签名验证失败: " + err.Error()
		} else {
			result.Valid = true
		}
	} else {
		// 电子签名，无法验证
		result.Valid = true
		result.Reason = "电子签名，无法验证"
	}

	return result, nil
}

// GenerateTimestamp 生成时间戳.
func (ce *CryptoEngine) GenerateTimestamp(data []byte) (string, error) {
	hash := sha256.Sum256(data)
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// 简单的时间戳令牌 (用 || 作为分隔符，避免与RFC3339中的冒号冲突)
	token := base64.StdEncoding.EncodeToString([]byte(timestamp + "||" + hex.EncodeToString(hash[:])))

	return token, nil
}

// VerifyTimestamp 验证时间戳.
func (ce *CryptoEngine) VerifyTimestamp(token string) (time.Time, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, errors.New("无效的时间戳令牌")
	}

	// 使用 || 作为分隔符
	s := string(decoded)
	separatorIdx := -1
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '|' && s[i+1] == '|' {
			separatorIdx = i
			break
		}
	}

	if separatorIdx < 0 {
		return time.Time{}, errors.New("时间戳格式错误")
	}

	timeStr := s[:separatorIdx]
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, errors.New("时间戳解析失败")
	}

	return t, nil
}

// HashDocument 计算文档哈希.
func (ce *CryptoEngine) HashDocument(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// VerifyDocumentHash 验证文档哈希.
func (ce *CryptoEngine) VerifyDocumentHash(content, expectedHash string) bool {
	actualHash := ce.HashDocument(content)
	return actualHash == expectedHash
}

// ExportPublicKey 导出公钥.
func (ce *CryptoEngine) ExportPublicKey(certID string) (string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	pair, ok := ce.certs[certID]
	if !ok {
		return "", errors.New("证书不存在")
	}

	return pair.Cert.PublicKey, nil
}

// ExportPrivateKey 导出私钥.
func (ce *CryptoEngine) ExportPrivateKey(certID string) (string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	pair, ok := ce.certs[certID]
	if !ok {
		return "", errors.New("证书不存在")
	}

	// 私钥导出需要序列化
	var keyBytes []byte
	var err error

	switch pair.Cert.Algorithm {
	case "RSA":
		keyBytes, err = x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	case "ECDSA":
		keyBytes, err = x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	default:
		return "", errors.New("不支持的算法")
	}

	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(keyBytes), nil
}

// ImportCertificate 导入证书.
func (ce *CryptoEngine) ImportCertificate(certPEM string, userID string) (*Certificate, error) {
	// 解析证书
	block, _ := base64.StdEncoding.DecodeString(certPEM)
	if block == nil {
		return nil, errors.New("无效的证书数据")
	}

	// 这里简化处理，实际需要解析PEM格式
	return nil, errors.New("导入功能待实现")
}

// GenerateKeyPair 生成密钥对.
func (ce *CryptoEngine) GenerateKeyPair(algorithm string) (publicKey, privateKey string, err error) {
	switch algorithm {
	case "RSA":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", "", err
		}
		pubBytes, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
		privBytes, _ := x509.MarshalPKCS8PrivateKey(key)
		return base64.StdEncoding.EncodeToString(pubBytes),
			base64.StdEncoding.EncodeToString(privBytes), nil

	case "ECDSA":
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return "", "", err
		}
		pubBytes, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
		privBytes, _ := x509.MarshalPKCS8PrivateKey(key)
		return base64.StdEncoding.EncodeToString(pubBytes),
			base64.StdEncoding.EncodeToString(privBytes), nil

	default:
		return "", "", errors.New("不支持的算法")
	}
}

// ValidateCertificate 验证证书有效性.
func (ce *CryptoEngine) ValidateCertificate(certID string) (bool, string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	pair, ok := ce.certs[certID]
	if !ok {
		return false, "", errors.New("证书不存在")
	}

	now := time.Now()

	if pair.Cert.Revoked {
		return false, "证书已吊销", nil
	}

	if now.Before(pair.Cert.NotBefore) {
		return false, "证书尚未生效", nil
	}

	if now.After(pair.Cert.NotAfter) {
		return false, "证书已过期", nil
	}

	return true, "证书有效", nil
}

// GetCertificateChain 获取证书链.
func (ce *CryptoEngine) GetCertificateChain(certID string) ([]*Certificate, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	chain := make([]*Certificate, 0)
	currentID := certID

	for currentID != "" {
		pair, ok := ce.certs[currentID]
		if !ok {
			break
		}

		chain = append(chain, pair.Cert)

		if pair.Cert.IsCA {
			break
		}

		// 查找颁发者
		found := false
		for id, p := range ce.certs {
			if p.Cert.Subject == pair.Cert.Issuer {
				currentID = id
				found = true
				break
			}
		}

		if !found {
			break
		}
	}

	return chain, nil
}

// CreateSelfSignedCert 创建自签名证书.
func (ce *CryptoEngine) CreateSelfSignedCert(name string, validDays int) (*Certificate, error) {
	if name == "" {
		return nil, errors.New("名称不能为空")
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()

	// 生成RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// 创建证书模板
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: name,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(0, 0, validDays),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	// 自签名
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// 解析证书
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)

	result := &Certificate{
		ID:        ce.generateID("cert"),
		Name:      name,
		Issuer:    name,
		Subject:   name,
		Serial:    cert.SerialNumber.Text(16),
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		PublicKey: base64.StdEncoding.EncodeToString(publicKeyBytes),
		Algorithm: "RSA",
		IsCA:      true,
		CreatedAt: time.Now(),
	}

	ce.certs[result.ID] = &CertKeyPair{
		Cert:       result,
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}

	return result, nil
}
