// fileindex_test.go - 文件索引测试
package fileindex

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 创建测试文件
	files := map[string]string{
		"readme.md":          "# Hello World\nThis is a test project.",
		"main.go":            "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"config.yaml":        "server:\n  port: 8080\n  host: localhost\n",
		"docs/guide.txt":     "User guide content\nStep 1: Install\nStep 2: Configure\n",
		"src/app.js":         "const express = require('express');\nconst app = express();\n",
		"src/utils/helper.js": "function add(a, b) { return a + b; }\nmodule.exports = { add };\n",
		"data.csv":           "name,age\nAlice,30\nBob,25\n",
		".git/config":        "[core]\n\trepositoryformatversion = 0\n",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}
	return dir
}

func newIndexer(t *testing.T) *Indexer {
	t.Helper()
	dir := setupTestDir(t)
	logger, _ := zap.NewDevelopment()
	return NewIndexer(logger, dir)
}

func TestBuild(t *testing.T) {
	idx := newIndexer(t)
	stats, err := idx.Build()
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	if stats.TotalFiles == 0 {
		t.Error("TotalFiles 不应为 0")
	}
	t.Logf("索引: %d 文件, %d 目录, 耗时 %s", stats.TotalFiles, stats.TotalDirs, stats.Duration)
}

func TestSearchByName(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	results := idx.Search(SearchQuery{
		Keyword:    "readme",
		SearchType: "name",
		Limit:      10,
	})
	if len(results) == 0 {
		t.Fatal("搜索 readme 应有结果")
	}
	if results[0].Entry.Name != "readme.md" {
		t.Errorf("期望 readme.md, 实际 %s", results[0].Entry.Name)
	}
}

func TestSearchByContent(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	results := idx.Search(SearchQuery{
		Keyword:    "hello",
		SearchType: "content",
		Limit:      10,
	})
	if len(results) == 0 {
		t.Fatal("搜索 hello 应有结果")
	}
}

func TestSearchByExtension(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	results := idx.Search(SearchQuery{
		Extensions: []string{".js"},
		Limit:      10,
	})
	if len(results) != 2 {
		t.Fatalf("JS 文件: 期望 2, 实际 %d", len(results))
	}
}

func TestSearchEmpty(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	results := idx.Search(SearchQuery{
		Keyword: "zzzznonexistent",
		Limit:   10,
	})
	if len(results) != 0 {
		t.Errorf("不存在的关键词应返回 0 结果, 实际 %d", len(results))
	}
}

func TestListRecent(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	recent := idx.ListRecent(5)
	if len(recent) == 0 {
		t.Error("最近文件不应为空")
	}
	if len(recent) > 5 {
		t.Errorf("限制 5: 实际 %d", len(recent))
	}
}

func TestListLargest(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	largest := idx.ListLargest(5)
	if len(largest) == 0 {
		t.Error("最大文件不应为空")
	}
	// 检查降序
	for i := 1; i < len(largest); i++ {
		if largest[i].Size > largest[i-1].Size {
			t.Error("结果应按大小降序排列")
			break
		}
	}
}

func TestStats(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	stats := idx.Stats()
	if stats.TotalFiles == 0 {
		t.Error("TotalFiles 不应为 0")
	}
	if len(stats.Extensions) == 0 {
		t.Error("Extensions 不应为空")
	}
}

func TestCount(t *testing.T) {
	idx := newIndexer(t)
	if idx.Count() != 0 {
		t.Error("未构建索引时应为 0")
	}
	idx.Build()
	if idx.Count() == 0 {
		t.Error("构建后不应为 0")
	}
}

func TestExcludes(t *testing.T) {
	dir := setupTestDir(t)
	logger, _ := zap.NewDevelopment()
	idx := NewIndexer(logger, dir)

	// 排除 .js 文件（通过名称匹配排除不了扩展名，但可以排除目录）
	idx.SetExcludes([]string{".git", "src"})
	idx.Build()

	// src 下的文件不应在索引中
	for _, entry := range idx.entries {
		if filepath.Base(filepath.Dir(entry.Path)) == "src" {
			t.Errorf("src 下的文件不应在索引中: %s", entry.Path)
		}
	}
}

func TestGetEntry(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()

	// 找一个存在的文件
	entry, ok := idx.GetEntry(filepath.Join(idx.basePath, "readme.md"))
	if !ok {
		t.Fatal("readme.md 应在索引中")
	}
	if entry.Name != "readme.md" {
		t.Errorf("期望 readme.md, 实际 %s", entry.Name)
	}

	_, ok = idx.GetEntry("/nonexistent/path")
	if ok {
		t.Error("不存在的路径不应找到")
	}
}
