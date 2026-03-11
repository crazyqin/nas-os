// Package e2e 提供 NAS-OS 端到端测试框架
// 测试完整的 API 端到端流程，模拟真实用户操作场景
package e2e

import (
	"fmt"
	"os"
	"testing"
)

// TestMain E2E 测试入口
func TestMain(m *testing.M) {
	fmt.Println("🚀 NAS-OS E2E 测试套件 v1.0")
	fmt.Println("=====================================")

	// 检查环境
	if os.Getenv("NAS_OS_E2E") == "" {
		fmt.Println("⚠️  设置 NAS_OS_E2E=1 启用 E2E 测试")
		os.Exit(0)
	}

	code := m.Run()
	fmt.Println("=====================================")
	fmt.Println("✅ E2E 测试完成")
	os.Exit(code)
}