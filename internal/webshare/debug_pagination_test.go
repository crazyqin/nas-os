package webshare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestDebugPagination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "debug-pagination")
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

	// 第一页
	req := SearchRequest{
		Query:      "",
		MaxResults: 5,
		Offset:     0,
	}

	resp1, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	fmt.Println("Page 1 results:")
	for i, r := range resp1.Results {
		fmt.Printf("  [%d] %s\n", i, r.Path)
	}

	// 第二页
	req.Offset = 5
	resp2, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	fmt.Println("Page 2 results:")
	for i, r := range resp2.Results {
		fmt.Printf("  [%d] %s\n", i, r.Path)
	}

	// 检查重复
	seen := make(map[string]bool)
	for _, r := range resp1.Results {
		seen[r.Path] = true
	}
	for _, r := range resp2.Results {
		if seen[r.Path] {
			t.Errorf("重复: %s", r.Path)
		}
	}
}
