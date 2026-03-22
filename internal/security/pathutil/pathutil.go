// Package pathutil 提供路径安全验证工具
// 解决 G304 文件路径注入漏洞 (CWE-22)
package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal 路径遍历攻击错误
var ErrPathTraversal = errors.New("path traversal detected")

// ErrInvalidPath 无效路径错误
var ErrInvalidPath = errors.New("invalid path")

// SafePath 验证用户提供的路径是否在基目录内
// 防止路径遍历攻击 (../ 等)
// 返回清理后的绝对路径，如果路径逃出基目录则返回错误
func SafePath(baseDir, userPath string) (string, error) {
	// 清理基目录
	baseDir = filepath.Clean(baseDir)
	if baseDir == "" {
		return "", ErrInvalidPath
	}

	// 获取基目录的绝对路径
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	// 清理用户路径
	cleanPath := filepath.Clean(userPath)

	// 移除开头的路径分隔符，防止绝对路径绕过
	cleanPath = strings.TrimPrefix(cleanPath, string(os.PathSeparator))
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "\\")

	// 检查路径遍历
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	// 构建完整路径
	fullPath := filepath.Join(absBase, cleanPath)

	// 获取完整路径的绝对路径
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	// 验证最终路径在基目录内
	relPath, err := filepath.Rel(absBase, absFull)
	if err != nil {
		return "", err
	}

	// 如果相对路径以 .. 开头，说明路径逃出了基目录
	if strings.HasPrefix(relPath, "..") {
		return "", ErrPathTraversal
	}

	// 对于已存在的路径，检查符号链接目标
	if info, err := os.Lstat(absFull); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// 解析符号链接目标
			resolved, err := filepath.EvalSymlinks(absFull)
			if err != nil {
				return "", err
			}
			// 验证符号链接目标也在基目录内
			resolvedRel, err := filepath.Rel(absBase, resolved)
			if err != nil {
				return "", err
			}
			if strings.HasPrefix(resolvedRel, "..") {
				return "", errors.New("symlink points outside base directory")
			}
		}
	}

	return absFull, nil
}

// SafePathMust 安全路径验证，出错时 panic
func SafePathMust(baseDir, userPath string) string {
	path, err := SafePath(baseDir, userPath)
	if err != nil {
		panic(err)
	}
	return path
}

// ValidatePath 验证路径是否包含危险字符
// 用于简单的路径验证，不需要基目录
func ValidatePath(path string) error {
	if path == "" {
		return ErrInvalidPath
	}

	// 检查路径遍历
	if strings.Contains(path, "..") {
		return ErrPathTraversal
	}

	// 检查空字节注入
	if strings.Contains(path, "\x00") {
		return errors.New("null byte in path")
	}

	return nil
}

// SafeJoin 安全地连接路径，防止路径遍历
func SafeJoin(baseDir, userPath string) (string, error) {
	return SafePath(baseDir, userPath)
}

// IsWithinBase 检查路径是否在基目录内
func IsWithinBase(baseDir, targetPath string) bool {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return false
	}

	absTarget, err := filepath.Abs(filepath.Clean(targetPath))
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..")
}
