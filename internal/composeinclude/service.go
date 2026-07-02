// Package composeinclude 提供 Docker Compose Include 支持。
package composeinclude

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IncludeManager Compose Include 管理器.
type IncludeManager struct {
	mu      sync.RWMutex
	results map[string]*ParseResult
}

// NewIncludeManager 创建 Compose Include 管理器.
func NewIncludeManager() *IncludeManager {
	return &IncludeManager{
		results: make(map[string]*ParseResult),
	}
}

// Parse 解析 Compose 文件并合并 include 引用.
func (m *IncludeManager) Parse(req *ParseRequest) (*ParseResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// 解析主 Compose 文件
	var composeFile ComposeFile
	if err := json.Unmarshal([]byte(req.Content), &composeFile); err != nil {
		return nil, fmt.Errorf("failed to parse compose content: %w", err)
	}

	baseDir := req.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	// 收集 include 路径并验证文件是否存在
	var allIncludePaths []string
	var missingFiles []string
	allFilesExist := true

	for _, inc := range composeFile.Include {
		for _, path := range inc.Paths {
			fullPath := path
			if !filepath.IsAbs(path) {
				fullPath = filepath.Join(baseDir, path)
			}
			allIncludePaths = append(allIncludePaths, fullPath)

			if _, err := os.Stat(fullPath); err != nil {
				missingFiles = append(missingFiles, fullPath)
				allFilesExist = false
			}
		}
	}

	// 合并 include 的服务定义
	mergedServices := make(map[string]ServiceDefinition)
	for name, svc := range composeFile.Services {
		mergedServices[name] = svc
	}

	if allFilesExist {
		for _, inc := range composeFile.Include {
			for _, path := range inc.Paths {
				fullPath := path
				if !filepath.IsAbs(path) {
					fullPath = filepath.Join(baseDir, path)
				}
				external, err := parseExternalCompose(fullPath)
				if err != nil {
					continue // 跳过解析失败的文件
				}
				// 合并外部文件的服务定义
				for name, svc := range external.Services {
					if _, exists := mergedServices[name]; !exists {
						mergedServices[name] = svc
					}
				}
			}
		}
	}

	result := &ParseResult{
		ID:             generateID(),
		SourceFile:     baseDir,
		MergedServices: mergedServices,
		IncludePaths:   allIncludePaths,
		MissingFiles:   missingFiles,
		AllFilesExist:  allFilesExist,
		ServiceCount:   len(mergedServices),
		ParsedAt:       time.Now(),
	}

	m.mu.Lock()
	m.results[result.ID] = result
	m.mu.Unlock()

	return result, nil
}

// GetResult 获取解析结果.
func (m *IncludeManager) GetResult(id string) (*ParseResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.results[id]
	if !ok {
		return nil, fmt.Errorf("result not found: %s", id)
	}
	return result, nil
}

// ListResults 列出所有解析结果.
func (m *IncludeManager) ListResults() []*ParseResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]*ParseResult, 0, len(m.results))
	for _, r := range m.results {
		results = append(results, r)
	}
	return results
}

// RemoveResult 移除解析结果.
func (m *IncludeManager) RemoveResult(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.results, id)
}

// ValidateIncludePaths 验证 include 引用的文件是否存在.
func (m *IncludeManager) ValidateIncludePaths(paths []string, baseDir string) ([]string, []string) {
	var existing []string
	var missing []string

	for _, path := range paths {
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(baseDir, path)
		}
		if _, err := os.Stat(fullPath); err == nil {
			existing = append(existing, fullPath)
		} else {
			missing = append(missing, fullPath)
		}
	}
	return existing, missing
}

// MergeServices 合并两个服务定义映射，后者覆盖前者.
func (m *IncludeManager) MergeServices(base, overlay map[string]ServiceDefinition) map[string]ServiceDefinition {
	merged := make(map[string]ServiceDefinition)
	for name, svc := range base {
		merged[name] = svc
	}
	for name, svc := range overlay {
		merged[name] = svc
	}
	return merged
}

// parseExternalCompose 解析外部 Compose 文件.
func parseExternalCompose(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	var composeFile ComposeFile
	if err := json.Unmarshal(data, &composeFile); err != nil {
		return nil, fmt.Errorf("failed to parse compose file %s: %w", path, err)
	}
	return &composeFile, nil
}

// generateID 生成唯一标识.
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
