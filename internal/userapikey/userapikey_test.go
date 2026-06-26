package userapikey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateKey(t *testing.T) {
	m := NewManager()

	req := &CreateKeyRequest{
		Name: "my-api-key",
		Permissions: []Permission{
			{Resource: "storage:*", Actions: []string{"read", "write"}},
		},
	}

	result, err := m.CreateKey("user-001", req)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.NewKey)
	assert.Contains(t, result.Prefix, "nas_")

	// 原始 Key 应该只返回一次
	key, err := m.GetKey("user-001", result.ID)
	require.NoError(t, err)
	assert.Equal(t, "user-001", key.UserID)
	assert.Equal(t, "my-api-key", key.Name)
	assert.Equal(t, KeyStatusActive, key.Status)
	assert.Len(t, key.Permissions, 1)
}

func TestCreateKeyInvalidInput(t *testing.T) {
	m := NewManager()

	_, err := m.CreateKey("", &CreateKeyRequest{Name: "test"})
	assert.ErrorIs(t, err, ErrInvalidInput)

	_, err = m.CreateKey("user-001", &CreateKeyRequest{Name: ""})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestRevokeKey(t *testing.T) {
	m := NewManager()

	result, err := m.CreateKey("user-001", &CreateKeyRequest{Name: "revoke-me"})
	require.NoError(t, err)

	err = m.RevokeKey("user-001", result.ID)
	require.NoError(t, err)

	key, _ := m.GetKey("user-001", result.ID)
	assert.Equal(t, KeyStatusRevoked, key.Status)
	assert.NotNil(t, key.RevokedAt)

	// 重复撤销
	err = m.RevokeKey("user-001", result.ID)
	assert.ErrorIs(t, err, ErrKeyRevoked)
}

func TestRevokeKeyPermissionDenied(t *testing.T) {
	m := NewManager()

	result, err := m.CreateKey("user-001", &CreateKeyRequest{Name: "mine"})
	require.NoError(t, err)

	err = m.RevokeKey("user-002", result.ID)
	assert.ErrorIs(t, err, ErrPermissionDenied)
}

func TestRotateKey(t *testing.T) {
	m := NewManager()

	result, err := m.CreateKey("user-001", &CreateKeyRequest{
		Name: "rotate-me",
		Permissions: []Permission{
			{Resource: "system:*", Actions: []string{"read"}},
		},
	})
	require.NoError(t, err)
	oldID := result.ID

	newResult, err := m.RotateKey("user-001", oldID)
	require.NoError(t, err)
	assert.NotEqual(t, oldID, newResult.ID)
	assert.NotEmpty(t, newResult.NewKey)

	// 旧 Key 应被撤销
	oldKey, _ := m.GetKey("user-001", oldID)
	assert.Equal(t, KeyStatusRevoked, oldKey.Status)

	// 新 Key 应保持相同权限
	newKey, _ := m.GetKey("user-001", newResult.ID)
	assert.Equal(t, KeyStatusActive, newKey.Status)
	assert.Equal(t, "rotate-me", newKey.Name)
}

func TestValidateKey(t *testing.T) {
	m := NewManager()

	result, err := m.CreateKey("user-001", &CreateKeyRequest{Name: "validate-me"})
	require.NoError(t, err)

	key, err := m.ValidateKey(result.NewKey)
	require.NoError(t, err)
	assert.Equal(t, "user-001", key.UserID)
	assert.NotNil(t, key.LastUsedAt)
}

func TestValidateKeyRevoked(t *testing.T) {
	m := NewManager()

	result, err := m.CreateKey("user-001", &CreateKeyRequest{Name: "will-revoke"})
	require.NoError(t, err)

	_ = m.RevokeKey("user-001", result.ID)

	_, err = m.ValidateKey(result.NewKey)
	assert.ErrorIs(t, err, ErrKeyRevoked)
}

func TestValidateKeyExpired(t *testing.T) {
	m := NewManager()

	past := time.Now().Add(-1 * time.Hour)
	result, err := m.CreateKey("user-001", &CreateKeyRequest{
		Name:      "expired-key",
		ExpiresAt: &past,
	})
	require.NoError(t, err)

	_, err = m.ValidateKey(result.NewKey)
	assert.ErrorIs(t, err, ErrKeyExpired)
}

func TestListKeys(t *testing.T) {
	m := NewManager()

	_, _ = m.CreateKey("user-001", &CreateKeyRequest{Name: "key1"})
	_, _ = m.CreateKey("user-001", &CreateKeyRequest{Name: "key2"})
	_, _ = m.CreateKey("user-002", &CreateKeyRequest{Name: "other"})

	keys := m.ListKeys("user-001", nil)
	assert.Len(t, keys, 2)

	active := KeyStatusActive
	keys = m.ListKeys("user-001", &ListKeysOptions{Status: &active})
	assert.Len(t, keys, 2)

	keys = m.ListKeys("user-002", nil)
	assert.Len(t, keys, 1)
}

func TestGetKeyNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetKey("user-001", "nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}
