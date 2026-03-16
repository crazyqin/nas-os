package quota

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========== StorageAdapter 测试 ==========

func TestNewStorageAdapter(t *testing.T) {
	adapter := NewStorageAdapter(nil)
	assert.NotNil(t, adapter)
}

func TestStorageAdapter_GetVolume_NilManager(t *testing.T) {
	adapter := NewStorageAdapter(nil)
	result := adapter.GetVolume("test-vol")
	assert.Nil(t, result)
}

// ========== UserAdapter 测试 ==========

func TestNewUserAdapter(t *testing.T) {
	adapter := NewUserAdapter(nil)
	assert.NotNil(t, adapter)
}

func TestUserAdapter_UserExists_NilManager(t *testing.T) {
	adapter := NewUserAdapter(nil)
	result := adapter.UserExists("testuser")
	assert.False(t, result)
}

func TestUserAdapter_GroupExists_NilManager(t *testing.T) {
	adapter := NewUserAdapter(nil)
	result := adapter.GroupExists("testgroup")
	assert.False(t, result)
}

func TestUserAdapter_GetUserHomeDir_NilManager(t *testing.T) {
	adapter := NewUserAdapter(nil)
	result := adapter.GetUserHomeDir("testuser")
	assert.Empty(t, result)
}
