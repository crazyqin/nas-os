package webshare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestDebugPagination2(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "debug-pagination2")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建多个测试文件
	for i := 0; i < 20; i++ {
		name := filepath.Join(tmpDir, fmt.Sprintf("file%02d.txt", i))
		os.WriteFile(name, []byte("content"), 0644)
	}

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	ctx := context.Background()
	engine.BuildIndex(ctx, tmpDir)

	// 检查 nameIndex
	engine.mu.RLock()
	fmt.Println("\nnameIndex contents:")
	for token, paths := range engine.nameIndex {
		fmt.Printf("  token=%q paths=%v\n", token, paths)
	}
	engine.mu.RUnlock()

	// 检查 matchPaths
	req := SearchRequest{Query: ""}
	matchedPaths := engine.matchPaths(req)
	fmt.Printf("\nmatchPaths returned %d paths:\n", len(matchedPaths))
	for i, p := range matchedPaths {
		fmt.Printf("  [%d] %s\n", i, p)
	}
}
