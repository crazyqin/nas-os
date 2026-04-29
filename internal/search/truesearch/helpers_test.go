package truesearch

import (
	"testing"

	"go.uber.org/zap"
)

// newTestLogger 创建测试用 logger。
func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	return logger
}
