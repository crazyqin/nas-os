package secretmgr

import (
	"testing"
	"time"
)

func TestManager_CreateSecret(t *testing.T) {
	m := NewManager()

	req := CreateSecretRequest{
		Name:        "数据库密码",
		Type:        SecretTypePassword,
		Description: "生产环境数据库密码",
		Value:       "super-secret-password",
		Tags:        []string{"database", "production"},
	}

	secret, err := m.CreateSecret(req)
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}
	if secret.Name != "数据库密码" {
		t.Errorf("期望名称 '数据库密码', 得到 '%s'", secret.Name)
	}
	if secret.Status != SecretStatusActive {
		t.Errorf("期望状态 active, 得到 %s", secret.Status)
	}
	if secret.Version != 1 {
		t.Errorf("期望版本 1, 得到 %d", secret.Version)
	}
}

func TestManager_CreateSecret_DefaultType(t *testing.T) {
	m := NewManager()

	req := CreateSecretRequest{
		Name:  "默认类型",
		Value: "test-value",
	}

	secret, err := m.CreateSecret(req)
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}
	if secret.Type != SecretTypeGeneric {
		t.Errorf("期望类型 generic, 得到 %s", secret.Type)
	}
}

func TestManager_GetSecret(t *testing.T) {
	m := NewManager()

	secret, _ := m.CreateSecret(CreateSecretRequest{
		Name:  "获取测试",
		Value: "test",
	})

	got, err := m.GetSecret(secret.ID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}
	if got.ID != secret.ID {
		t.Errorf("ID不匹配")
	}
	if got.LastUsed == nil {
		t.Error("LastUsed应该被设置")
	}
}

func TestManager_GetSecret_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetSecret("nonexistent")
	if err != ErrSecretNotFound {
		t.Errorf("期望 ErrSecretNotFound, 得到 %v", err)
	}
}

func TestManager_GetSecret_Expired(t *testing.T) {
	m := NewManager()

	past := time.Now().Add(-1 * time.Hour)
	secret, _ := m.CreateSecret(CreateSecretRequest{
		Name:      "过期测试",
		Value:     "test",
		ExpiresAt: &past,
	})

	got, _ := m.GetSecret(secret.ID)
	if got.Status != SecretStatusExpired {
		t.Errorf("期望 expired, 得到 %s", got.Status)
	}
}

func TestManager_ListSecrets(t *testing.T) {
	m := NewManager()

	m.CreateSecret(CreateSecretRequest{Name: "s1", Value: "v1"})
	m.CreateSecret(CreateSecretRequest{Name: "s2", Value: "v2"})

	secrets := m.ListSecrets()
	if len(secrets) != 2 {
		t.Errorf("期望2个密钥, 得到 %d", len(secrets))
	}
}

func TestManager_DeleteSecret(t *testing.T) {
	m := NewManager()

	secret, _ := m.CreateSecret(CreateSecretRequest{Name: "删除测试", Value: "test"})

	if err := m.DeleteSecret(secret.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

func TestManager_UpdateSecret(t *testing.T) {
	m := NewManager()

	secret, _ := m.CreateSecret(CreateSecretRequest{Name: "更新测试", Value: "old"})

	if err := m.UpdateSecret(secret.ID, "new"); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	got, _ := m.GetSecret(secret.ID)
	if got.Value != "new" {
		t.Errorf("期望值 'new', 得到 '%s'", got.Value)
	}
	if got.Version != 2 {
		t.Errorf("期望版本 2, 得到 %d", got.Version)
	}
}

func TestManager_GetVersions(t *testing.T) {
	m := NewManager()

	secret, _ := m.CreateSecret(CreateSecretRequest{Name: "版本测试", Value: "v1"})
	m.UpdateSecret(secret.ID, "v2")
	m.UpdateSecret(secret.ID, "v3")

	versions := m.GetVersions(secret.ID)
	if len(versions) != 3 {
		t.Errorf("期望3个版本, 得到 %d", len(versions))
	}
}

func TestManager_RevokeSecret(t *testing.T) {
	m := NewManager()

	secret, _ := m.CreateSecret(CreateSecretRequest{Name: "撤销测试", Value: "test"})

	if err := m.RevokeSecret(secret.ID); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}

	got, _ := m.GetSecret(secret.ID)
	if got.Status != SecretStatusRevoked {
		t.Errorf("期望 revoked, 得到 %s", got.Status)
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.CreateSecret(CreateSecretRequest{Name: "统计测试", Value: "test"})

	stats := m.GetStats()
	if stats.TotalSecrets != 1 {
		t.Errorf("期望1个密钥, 得到 %d", stats.TotalSecrets)
	}
}
