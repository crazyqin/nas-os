package smartcostoptimizer

import (
	"crypto/rand"
	"fmt"
	"time"
)

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("cost-%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString 生成指定长度的随机字符串.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if crypto/rand fails
		for i := range b {
			b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = letters[b[i]%byte(len(letters))]
	}
	return string(b)
}
