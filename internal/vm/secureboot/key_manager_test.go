package secureboot

import (
	"testing"
)

func TestNewKeyManager(t *testing.T) {
	km := NewKeyManager(nil)
	if km == nil {
		t.Fatal("NewKeyManager 返回 nil")
	}
	if km.store == nil {
		t.Error("store 应已初始化")
	}
}

func TestGeneratePlatformCA(t *testing.T) {
	km := NewKeyManager(nil)

	err := km.GeneratePlatformCA()
	if err != nil {
		t.Fatalf("生成平台 CA 失败：%v", err)
	}

	if km.caCert == nil {
		t.Fatal("CA 证书应不为 nil")
	}
	if km.caCert.Subject.CommonName != "NAS-OS Secure Boot CA" {
		t.Errorf("CA 主题应为 NAS-OS Secure Boot CA，实际 %s", km.caCert.Subject.CommonName)
	}
	if !km.caCert.IsCA {
		t.Error("CA 证书应标记为 CA")
	}

	// 重复生成应失败
	err = km.GeneratePlatformCA()
	if err == nil {
		t.Error("重复生成 CA 应返回错误")
	}
}

func TestInitDefaultKeys(t *testing.T) {
	km := NewKeyManager(nil)

	err := km.InitDefaultKeys()
	if err != nil {
		t.Fatalf("初始化默认密钥失败：%v", err)
	}

	// 检查 PK
	pk := km.GetPK()
	if pk == nil {
		t.Fatal("PK 应不为 nil")
	}
	if pk.Subject.CommonName != "NAS-OS Platform Key" {
		t.Errorf("PK 主题应为 NAS-OS Platform Key，实际 %s", pk.Subject.CommonName)
	}

	// 检查 KEK
	keks := km.ListKeys(KeyTypeKEK)
	if len(keks) == 0 {
		t.Error("应有至少一个 KEK")
	}

	// 检查 db
	dbs := km.ListKeys(KeyTypeDB)
	if len(dbs) == 0 {
		t.Error("应有至少一个 db 条目")
	}

	// 检查总数
	if km.KeyCount() < 3 {
		t.Errorf("密钥总数应至少 3，实际 %d", km.KeyCount())
	}
}

func TestSetPK(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	cert := generateTestCert(t, "Test PK")
	err := km.SetPK(cert)
	if err != nil {
		t.Fatalf("设置 PK 失败：%v", err)
	}

	pk := km.GetPK()
	if pk == nil {
		t.Fatal("PK 应不为 nil")
	}
	if pk.Subject.CommonName != "Test PK" {
		t.Errorf("PK 主题应为 Test PK，实际 %s", pk.Subject.CommonName)
	}

	// PK 列表应只有 1 个
	pks := km.ListKeys(KeyTypePK)
	if len(pks) != 1 {
		t.Errorf("PK 列表应只有 1 个，实际 %d", len(pks))
	}

	// 设置 nil 应失败
	err = km.SetPK(nil)
	if err == nil {
		t.Error("设置 nil PK 应返回错误")
	}
}

func TestAddKEK(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	// 未设置 PK 时添加 KEK 应失败
	cert := generateTestCert(t, "Test KEK")
	err := km.AddKEK(cert)
	if err == nil {
		t.Error("PK 未设置时添加 KEK 应返回错误")
	}

	// 设置 PK 后添加
	pkCert := generateTestCert(t, "Platform Key")
	_ = km.SetPK(pkCert)

	err = km.AddKEK(cert)
	if err != nil {
		t.Fatalf("添加 KEK 失败：%v", err)
	}

	keks := km.ListKeys(KeyTypeKEK)
	if len(keks) != 1 {
		t.Errorf("应有 1 个 KEK，实际 %d", len(keks))
	}

	// nil 证书
	err = km.AddKEK(nil)
	if err == nil {
		t.Error("添加 nil KEK 应返回错误")
	}
}

func TestAddDBEntry(t *testing.T) {
	km := NewKeyManager(nil)

	cert := generateTestCert(t, "Test DB Entry")
	err := km.AddDBEntry(cert)
	if err != nil {
		t.Fatalf("添加 db 条目失败：%v", err)
	}

	dbs := km.ListKeys(KeyTypeDB)
	if len(dbs) != 1 {
		t.Errorf("应有 1 个 db 条目，实际 %d", len(dbs))
	}

	// nil 证书
	err = km.AddDBEntry(nil)
	if err == nil {
		t.Error("添加 nil db 条目应返回错误")
	}
}

func TestAddDBXEntry(t *testing.T) {
	km := NewKeyManager(nil)

	hash := hashCertificate(generateTestCert(t, "Revoked"))
	err := km.AddDBXEntry(hash, "吊销测试证书")
	if err != nil {
		t.Fatalf("添加 dbx 条目失败：%v", err)
	}

	if !km.IsRevoked(hash) {
		t.Error("应被标记为吊销")
	}

	otherHash := hashCertificate(generateTestCert(t, "Other"))
	if km.IsRevoked(otherHash) {
		t.Error("其他证书不应被吊销")
	}
}

func TestRemoveDBEntry(t *testing.T) {
	km := NewKeyManager(nil)

	cert := generateTestCert(t, "To Remove")
	_ = km.AddDBEntry(cert)
	hash := hashCertificate(cert)

	if km.KeyCount() != 1 {
		t.Fatalf("添加后期望 1 个条目，实际 %d", km.KeyCount())
	}

	removed := km.RemoveDBEntry(hash)
	if !removed {
		t.Error("应成功移除")
	}
	if km.KeyCount() != 0 {
		t.Error("移除后应为 0")
	}

	// 再次移除应返回 false
	removed = km.RemoveDBEntry(hash)
	if removed {
		t.Error("重复移除应返回 false")
	}
}

func TestGenerateSignedCertificate(t *testing.T) {
	km := NewKeyManager(nil)

	// CA 未初始化时应失败
	_, _, err := km.GenerateSignedCertificate("Test")
	if err == nil {
		t.Error("CA 未初始化时应返回错误")
	}

	// 初始化 CA
	_ = km.GeneratePlatformCA()

	cert, key, err := km.GenerateSignedCertificate("Signed Test")
	if err != nil {
		t.Fatalf("签发证书失败：%v", err)
	}
	if cert == nil {
		t.Fatal("证书应不为 nil")
	}
	if key == nil {
		t.Fatal("私钥应不为 nil")
	}
	if cert.Subject.CommonName != "Signed Test" {
		t.Errorf("证书主题应为 Signed Test，实际 %s", cert.Subject.CommonName)
	}

	// 验证签名链
	if err := cert.CheckSignatureFrom(km.caCert); err != nil {
		t.Errorf("证书应由 CA 签发：%v", err)
	}
}

func TestExportPKPEM(t *testing.T) {
	km := NewKeyManager(nil)

	// PK 未设置时
	_, err := km.ExportPKPEM()
	if err == nil {
		t.Error("PK 未设置时应返回错误")
	}

	// 设置 PK
	_ = km.GeneratePlatformCA()
	_ = km.InitDefaultKeys()

	pemData, err := km.ExportPKPEM()
	if err != nil {
		t.Fatalf("导出 PK PEM 失败：%v", err)
	}
	if len(pemData) == 0 {
		t.Error("PEM 数据不应为空")
	}
}

func TestStatus(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.InitDefaultKeys()

	cfg := DefaultSecureBootConfig()
	status := km.Status(cfg)

	if !status.Enabled {
		t.Error("应为启用状态")
	}
	if status.State != "enabled" {
		t.Errorf("状态应为 enabled，实际 %s", status.State)
	}
	if status.KeyCount < 3 {
		t.Errorf("密钥数应至少 3，实际 %d", status.KeyCount)
	}
}

func TestKeyManagerConcurrency(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()
	_ = km.SetPK(generateTestCert(t, "PK"))

	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func(n int) {
			cert := generateTestCert(t, "Concurrent DB")
			_ = km.AddDBEntry(cert)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		go func() {
			_ = km.ListKeys(KeyTypeDB)
			_ = km.KeyCount()
			_ = km.GetPK()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}
