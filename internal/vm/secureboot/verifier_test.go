package secureboot

import (
	"crypto/sha256"
	"testing"
)

func TestNewSignatureVerifier(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	if sv == nil {
		t.Fatal("NewSignatureVerifier 返回 nil")
	}
	if sv.keyManager != km {
		t.Error("keyManager 应匹配")
	}
}

func TestVerifySignatureEmptyData(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	result := sv.VerifySignature(nil, nil, nil)
	if result.Valid {
		t.Error("空数据不应验证通过")
	}
	if result.Reason != "数据为空" {
		t.Errorf("原因应为 '数据为空'，实际 %q", result.Reason)
	}
}

func TestVerifySignatureNilCert(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	result := sv.VerifySignature([]byte("data"), nil, nil)
	if result.Valid {
		t.Error("nil 证书不应验证通过")
	}
	if result.Reason != "证书不能为空" {
		t.Errorf("原因应为 '证书不能为空'，实际 %q", result.Reason)
	}
}

func TestVerifySignatureRevokedCert(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()
	_ = km.InitDefaultKeys()

	sv := NewSignatureVerifier(km, nil)

	// 生成证书并添加到 db
	cert := generateTestCert(t, "Revoked Cert")
	_ = km.AddDBEntry(cert)

	// 吊销该证书
	hash := hashCertificate(cert)
	_ = km.AddDBXEntry(hash, "测试吊销")

	result := sv.VerifySignature([]byte("data"), nil, cert)
	if result.Valid {
		t.Error("被吊销的证书不应验证通过")
	}
	if result.Reason != "证书已被吊销（在 dbx 中）" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifySignatureNotInDB(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	sv := NewSignatureVerifier(km, nil)

	// 生成证书但不添加到 db
	cert := generateTestCert(t, "Unknown Cert")

	result := sv.VerifySignature([]byte("data"), nil, cert)
	if result.Valid {
		t.Error("不在 db 中的证书不应验证通过")
	}
	if result.Reason != "证书不在信任数据库 (db) 中" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifySignatureValid(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	sv := NewSignatureVerifier(km, nil)

	// 生成并添加证书到 db
	cert := generateTestCert(t, "Valid Cert")
	_ = km.AddDBEntry(cert)

	result := sv.VerifySignature([]byte("data"), nil, cert)
	if !result.Valid {
		t.Errorf("有效证书应验证通过，原因：%s", result.Reason)
	}
	if result.SignerCN != "Valid Cert" {
		t.Errorf("签名者 CN 应为 Valid Cert，实际 %s", result.SignerCN)
	}
}

func TestVerifyHash(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	data := []byte("test data")
	hash := sha256.Sum256(data)

	if !sv.VerifyHash(data, hash) {
		t.Error("正确哈希应验证通过")
	}

	wrongHash := sha256.Sum256([]byte("other data"))
	if sv.VerifyHash(data, wrongHash) {
		t.Error("错误哈希不应验证通过")
	}
}

func TestVerifyCertificateValid(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()
	_ = km.InitDefaultKeys()

	sv := NewSignatureVerifier(km, nil)

	// 添加一个证书到 db
	cert := generateTestCert(t, "DB Cert")
	_ = km.AddDBEntry(cert)

	result := sv.VerifyCertificate(cert)
	if !result.Valid {
		t.Errorf("有效证书应验证通过，原因：%s", result.Reason)
	}
}

func TestVerifyCertificateNil(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	result := sv.VerifyCertificate(nil)
	if result.Valid {
		t.Error("nil 证书不应验证通过")
	}
	if result.Reason != "证书为 nil" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifyCertificateRevoked(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	sv := NewSignatureVerifier(km, nil)

	cert := generateTestCert(t, "Revoked")
	_ = km.AddDBEntry(cert)

	hash := hashCertificate(cert)
	_ = km.AddDBXEntry(hash, "吊销")

	result := sv.VerifyCertificate(cert)
	if result.Valid {
		t.Error("被吊销证书不应验证通过")
	}
	if result.Reason != "证书已被吊销" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifyComponentValid(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	data := []byte("firmware data")
	hash := sha256.Sum256(data)

	comp := &BootComponent{
		Name: "test-component",
		Data: data,
		Hash: hash,
	}

	result := sv.VerifyComponent(comp)
	if !result.Valid {
		t.Errorf("有效组件应验证通过，原因：%s", result.Reason)
	}
	if result.Name != "test-component" {
		t.Errorf("名称应为 test-component，实际 %s", result.Name)
	}
}

func TestVerifyComponentEmptyData(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	comp := &BootComponent{
		Name: "empty",
		Data: nil,
	}

	result := sv.VerifyComponent(comp)
	if result.Valid {
		t.Error("空数据组件不应验证通过")
	}
	if result.Reason != "组件数据为空" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifyComponentHashMismatch(t *testing.T) {
	km := NewKeyManager(nil)
	sv := NewSignatureVerifier(km, nil)

	comp := &BootComponent{
		Name: "bad-hash",
		Data: []byte("data"),
		Hash: [32]byte{0xff}, // 错误的哈希
	}

	result := sv.VerifyComponent(comp)
	if result.Valid {
		t.Error("哈希不匹配的组件不应验证通过")
	}
	if result.Reason != "哈希不匹配" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestBuildCertPool(t *testing.T) {
	cert := generateTestCert(t, "Pool Cert")
	entries := []*KeyEntry{
		{Certificate: cert},
		{Certificate: nil}, // 应跳过
	}

	pool := BuildCertPool(entries)
	if pool == nil {
		t.Fatal("证书池不应为 nil")
	}
}

func TestValidateCertChain(t *testing.T) {
	caCert, caKey := generateTestCertWithCA(t, "Test CA")
	childCert := generateTestCertSigned(t, "Child Cert", caCert, caKey)

	roots := BuildCertPool([]*KeyEntry{{Certificate: caCert}})

	err := ValidateCertChain(childCert, roots, nil)
	if err != nil {
		t.Errorf("有效证书链应验证通过：%v", err)
	}

	// nil 证书
	err = ValidateCertChain(nil, roots, nil)
	if err == nil {
		t.Error("nil 证书应返回错误")
	}
}

func TestVerifySignatureChain(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()
	_ = km.InitDefaultKeys()

	sv := NewSignatureVerifier(km, nil)

	// 获取 db 中的证书（由 CA 签发）
	dbCert := generateTestCert(t, "Chained")
	_ = km.AddDBEntry(dbCert)

	result := sv.VerifySignature([]byte("data"), nil, dbCert)
	if !result.Valid {
		t.Errorf("证书链验证应通过，原因：%s", result.Reason)
	}
}
