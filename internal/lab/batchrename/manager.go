// Package batchrename 提供批量文件重命名功能
package batchrename

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Manager 批量重命名管理器.
type Manager struct{}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{}
}

// Preview 预览重命名结果.
func (m *Manager) Preview(files []string, rule RenameRule) []RenamePreview {
	result := make([]RenamePreview, len(files))
	for i, file := range files {
		renamed, err := m.applyRule(file, rule, i)
		result[i] = RenamePreview{
			Original: file,
			Renamed:  renamed,
			Changed:  renamed != file,
		}
		if err != nil {
			result[i].Error = err.Error()
		}
	}
	return result
}

// Rename 执行批量重命名.
func (m *Manager) Rename(files []string, rule RenameRule) RenameResult {
	result := RenameResult{
		Total: len(files),
	}

	for i, file := range files {
		renamed, err := m.applyRule(file, rule, i)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file, err))
			continue
		}

		if renamed == file {
			continue
		}

		// 模拟重命名（实际文件系统操作由调用方处理）
		result.Files = append(result.Files, renamed)
		result.Renamed++
	}

	return result
}

// applyRule 应用重命名规则.
func (m *Manager) applyRule(file string, rule RenameRule, index int) (string, error) {
	dir := filepath.Dir(file)
	base := filepath.Base(file)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch rule.Mode {
	case ModeRegex:
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return file, fmt.Errorf("invalid regex: %v", err)
		}
		name = re.ReplaceAllString(name, rule.Replace)

	case ModeSequence:
		num := rule.StartNum + index
		numStr := strconv.Itoa(num)
		if rule.PadWidth > 0 {
			numStr = fmt.Sprintf("%0*d", rule.PadWidth, num)
		}
		if rule.Replace != "" {
			name = strings.Replace(rule.Replace, "#", numStr, 1)
		} else {
			name = name + numStr
		}

	case ModeDate:
		dateStr := time.Now().Format(rule.DateFormat)
		if dateStr == "" {
			dateStr = "20060102"
			dateStr = time.Now().Format(dateStr)
		}
		name = dateStr + "_" + name

	case ModeCase:
		switch rule.CaseType {
		case CaseUpper:
			name = strings.ToUpper(name)
		case CaseLower:
			name = strings.ToLower(name)
		case CaseTitle:
			name = toTitleCase(name)
		}

	case ModeReplace:
		if rule.Pattern != "" {
			name = strings.ReplaceAll(name, rule.Pattern, rule.Replace)
		}

	case ModeExtension:
		if rule.Extension != "" {
			ext = rule.Extension
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
		}

	case ModePrefix:
		name = rule.Prefix + name

	case ModeSuffix:
		name = name + rule.Suffix
	}

	return filepath.Join(dir, name+ext), nil
}

func toTitleCase(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToUpper(r)
		}
		return r
	}, s[:1]) + s[1:]
}
