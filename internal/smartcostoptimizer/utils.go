package smartcostoptimizer

import (
	"fmt"
	"time"
)

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("cost-%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString 生成指定长度的随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1) // 确保每次不同
	}
	return string(b)
}
