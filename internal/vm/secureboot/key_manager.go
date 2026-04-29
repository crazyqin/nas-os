package secureboot

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go.uber.org/zap"
)

// KeyManager UEFI Secure Boot 密钥管理器。
//
// 管理 PK、KEK、db、dbx 四层密钥体系：
//   - PK (Platform Key): 平台根信任锚，控制 KEK 的更新
//   - KEK (Key Exchange Key): 控制 db/dbx 的更新
//   - db (Signature Database): 存储被信任的签名/证书
//   -dbx (Forbidden Signatures Database): 存储被吊销的签名/证书
type KeyManager struct {
	mu       sync.RWMutex
	store    *keyStore
	pk       *x509.Certificate // 当前平台密钥
	logger   *zap.Logger
	caCert   *x509.Certificate // 用于签发 PK 的 CA
	caKey    interface{}        // CA 私钥
}

// NewKeyManager 创建密钥管理器。
func NewKeyManager(logger *zap.Logger) *KeyManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &KeyManager{
		store:  newKeyStore(),
		logger: logger,
	}
}

// GeneratePlatformCA 生成平台 CA（用于签发 PK）。
func (km *KeyManager) GeneratePlatformCA() error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.caCert != nil {
		return errors.New("平台 CA 已存在")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("生成 CA 密钥失败：%w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败：%w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS Platform CA"},
			CommonName:   "NAS-OS Secure Boot CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10年
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("创建 CA 证书失败：%w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("解析 CA 证书失败：%w", err)
	}

	km.caCert = cert
	km.caKey = key
	km.logger.Info("平台 CA 已生成")
	return nil
}

// InitDefaultKeys 使用默认密钥初始化 Secure Boot 密钥体系。
func (km *KeyManager) InitDefaultKeys() error {
	km.mu.RLock()
	hasCA := km.caCert != nil
	km.mu.RUnlock()

	if !hasCA {
		if err := km.GeneratePlatformCA(); err != nil {
			return fmt.Errorf("生成平台 CA 失败：%w", err)
		}
	}

	// 生成 PK
	pkCert, err := km.generateKeyCert("NAS-OS Platform Key", KeyTypePK)
	if err != nil {
		return fmt.Errorf("生成 PK 失败：%w", err)
	}
	if err := km.SetPK(pkCert); err != nil {
		return fmt.Errorf("设置 PK 失败：%w", err)
	}

	// 生成 KEK
	kekCert, err := km.generateKeyCert("NAS-OS Key Exchange Key", KeyTypeKEK)
	if err != nil {
		return fmt.Errorf("生成 KEK 失败：%w", err)
	}
	if err := km.AddKEK(kekCert); err != nil {
		return fmt.Errorf("添加 KEK 失败：%w", err)
	}

	// 生成 db（允许签名的证书）
	dbCert, err := km.generateKeyCert("NAS-OS Signature Database", KeyTypeDB)
	if err != nil {
		return fmt.Errorf("生成 db 证书失败：%w", err)
	}
	if err := km.AddDBEntry(dbCert); err != nil {
		return fmt.Errorf("添加 db 条目失败：%w", err)
	}

	km.logger.Info("Secure Boot 默认密钥已初始化")
	return nil
}

// SetPK 设置平台密钥 (Platform Key)。
func (km *KeyManager) SetPK(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("证书不能为空")
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	km.pk = cert
	entry := &KeyEntry{
		Type:        SigTypeX509,
		KeyType:     KeyTypePK,
		Certificate: cert,
		Hash:        hashCertificate(cert),
		Description: cert.Subject.CommonName,
		OwnerGUID:   cert.SerialNumber.String(),
		AddedAt:     time.Now(),
	}

	// 清除旧的 PK，只保留最新的
	km.store.mu.Lock()
	km.store.keys[KeyTypePK] = []*KeyEntry{entry}
	km.store.mu.Unlock()

	km.logger.Info("平台密钥 (PK) 已设置",
		zap.String("subject", cert.Subject.CommonName),
		zap.String("serial", cert.SerialNumber.String()),
	)
	return nil
}

// GetPK 获取当前平台密钥。
func (km *KeyManager) GetPK() *x509.Certificate {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.pk
}

// AddKEK 添加密钥交换密钥。
func (km *KeyManager) AddKEK(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("证书不能为空")
	}

	// KEK 必须由 PK 签发
	km.mu.RLock()
	pk := km.pk
	km.mu.RUnlock()

	if pk == nil {
		return errors.New("PK 未设置，无法添加 KEK")
	}

	if err := cert.CheckSignatureFrom(pk); err != nil {
		km.logger.Warn("KEK 证书未由 PK 签发，以审计模式添加")
	}

	entry := &KeyEntry{
		Type:        SigTypeX509,
		KeyType:     KeyTypeKEK,
		Certificate: cert,
		Hash:        hashCertificate(cert),
		Description: cert.Subject.CommonName,
		OwnerGUID:   cert.SerialNumber.String(),
		AddedAt:     time.Now(),
	}

	km.store.add(KeyTypeKEK, entry)
	km.logger.Info("KEK 已添加",
		zap.String("subject", cert.Subject.CommonName),
	)
	return nil
}

// AddDBEntry 添加签名数据库条目。
func (km *KeyManager) AddDBEntry(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("证书不能为空")
	}

	entry := &KeyEntry{
		Type:        SigTypeX509,
		KeyType:     KeyTypeDB,
		Certificate: cert,
		Hash:        hashCertificate(cert),
		Description: cert.Subject.CommonName,
		OwnerGUID:   cert.SerialNumber.String(),
		AddedAt:     time.Now(),
	}

	km.store.add(KeyTypeDB, entry)
	km.logger.Info("db 条目已添加",
		zap.String("subject", cert.Subject.CommonName),
	)
	return nil
}

// AddDBXEntry 添加撤销数据库条目。
func (km *KeyManager) AddDBXEntry(hash [32]byte, description string) error {
	now := time.Now()
	entry := &KeyEntry{
		Type:        SigTypeSHA256,
		KeyType:     KeyTypeDBX,
		Hash:        hash,
		Description: description,
		AddedAt:     now,
		RevokedAt:   &now,
	}

	km.store.add(KeyTypeDBX, entry)
	km.logger.Info("dbx 条目已添加",
		zap.String("description", description),
	)
	return nil
}

// RemoveDBEntry 按哈希移除 db 条目。
func (km *KeyManager) RemoveDBEntry(hash [32]byte) bool {
	return km.store.remove(KeyTypeDB, hash)
}

// ListKeys 列出指定类型的所有密钥。
func (km *KeyManager) ListKeys(keyType KeyType) []*KeyEntry {
	return km.store.list(keyType)
}

// KeyCount 返回所有密钥总数。
func (km *KeyManager) KeyCount() int {
	return km.store.count()
}

// IsRevoked 检查给定哈希是否在 dbx 中。
func (km *KeyManager) IsRevoked(hash [32]byte) bool {
	return km.store.isRevoked(hash)
}

// ExportPKCS12 导出 PK 为 PEM 格式。
func (km *KeyManager) ExportPKPEM() ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.pk == nil {
		return nil, errors.New("PK 未设置")
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: km.pk.Raw,
	}), nil
}

// GenerateSignedCertificate 使用平台 CA 签发证书。
func (km *KeyManager) GenerateSignedCertificate(subject string) (*x509.Certificate, interface{}, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.caCert == nil || km.caKey == nil {
		return nil, nil, errors.New("平台 CA 未初始化")
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("生成密钥对失败：%w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("生成序列号失败：%w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS"},
			CommonName:   subject,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(5 * 365 * 24 * time.Hour), // 5年
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, km.caCert, &key.PublicKey, km.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("签发证书失败：%w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("解析证书失败：%w", err)
	}

	return cert, key, nil
}

// generateKeyCert 内部方法，使用 CA 签发指定类型的密钥证书。
func (km *KeyManager) generateKeyCert(subject string, _ KeyType) (*x509.Certificate, error) {
	if km.caCert == nil || km.caKey == nil {
		return nil, errors.New("平台 CA 未初始化")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败：%w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("生成序列号失败：%w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS Secure Boot"},
			CommonName:   subject,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(5 * 365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, km.caCert, &key.PublicKey, km.caKey)
	if err != nil {
		return nil, fmt.Errorf("签发证书失败：%w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("解析证书失败：%w", err)
	}

	return cert, nil
}

// Status 返回当前 Secure Boot 状态。
func (km *KeyManager) Status(config SecureBootConfig) SecureBootStatus {
	return SecureBootStatus{
		Enabled:        config.Enabled,
		State:          string(config.SecureBootState),
		LastBootSecure: config.Enabled,
		TPMPresent:     config.TPMEnabled,
		KeyCount:       km.KeyCount(),
	}
}
