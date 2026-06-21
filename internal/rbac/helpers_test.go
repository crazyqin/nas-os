package rbac

import "testing"

// newTestManager 创建用于测试的 RBAC 管理器.
func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	m, err := NewManager(DefaultConfig())
	if err != nil {
		t.Fatalf("创建测试管理器失败: %v", err)
	}
	cleanup := func() {}
	return m, cleanup
}
