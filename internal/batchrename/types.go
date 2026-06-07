// Package batchrename 提供批量文件重命名功能，对标群晖File Station批量重命名
package batchrename

import ()

// RenameMode 重命名模式.
type RenameMode string

const (
	ModeRegex     RenameMode = "regex"     // 正则替换
	ModeSequence  RenameMode = "sequence"  // 序号添加
	ModeDate      RenameMode = "date"      // 日期前缀
	ModeCase      RenameMode = "case"      // 大小写转换
	ModeReplace   RenameMode = "replace"   // 文本替换
	ModeExtension RenameMode = "extension" // 扩展名替换
	ModePrefix    RenameMode = "prefix"    // 添加前缀
	ModeSuffix    RenameMode = "suffix"    // 添加后缀
)

// CaseType 大小写类型.
type CaseType string

const (
	CaseUpper CaseType = "upper"
	CaseLower CaseType = "lower"
	CaseTitle CaseType = "title"
)

// RenameRule 重命名规则.
type RenameRule struct {
	Mode       RenameMode `json:"mode"`
	Pattern    string     `json:"pattern"`     // 正则/查找文本
	Replace    string     `json:"replace"`     // 替换文本
	Prefix     string     `json:"prefix"`      // 前缀
	Suffix     string     `json:"suffix"`      // 后缀
	CaseType   CaseType   `json:"case_type"`   // 大小写类型
	StartNum   int        `json:"start_num"`   // 起始序号
	PadWidth   int        `json:"pad_width"`   // 序号补零宽度
	DateFormat string     `json:"date_format"` // 日期格式
	Extension  string     `json:"extension"`   // 新扩展名
}

// RenamePreview 重命名预览.
type RenamePreview struct {
	Original string `json:"original"`
	Renamed  string `json:"renamed"`
	Changed  bool   `json:"changed"`
	Error    string `json:"error,omitempty"`
}

// RenameResult 重命名结果.
type RenameResult struct {
	Total   int      `json:"total"`
	Renamed int      `json:"renamed"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
	Files   []string `json:"files"`
}

// BatchRenameRequest 批量重命名请求.
type BatchRenameRequest struct {
	Files  []string   `json:"files" binding:"required"`
	Rule   RenameRule `json:"rule" binding:"required"`
	DryRun bool       `json:"dry_run"` // 仅预览不执行
}
