// Package smartfolders provides rule-based virtual folders for NAS files.
package smartfolders

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileClass is a broad, user-facing file category.
type FileClass string

const (
	ClassDocument FileClass = "document"
	ClassPhoto    FileClass = "photo"
	ClassVideo    FileClass = "video"
	ClassAudio    FileClass = "audio"
	ClassArchive  FileClass = "archive"
	ClassCode     FileClass = "code"
	ClassOther    FileClass = "other"
)

// Rule describes a virtual folder filter. Empty fields are ignored.
type Rule struct {
	Name       string        `json:"name"`
	Classes    []FileClass   `json:"classes,omitempty"`
	Extensions []string      `json:"extensions,omitempty"`
	NameQuery  string        `json:"name_query,omitempty"`
	MinSize    int64         `json:"min_size,omitempty"`
	MaxSize    int64         `json:"max_size,omitempty"`
	ModifiedIn time.Duration `json:"-"`
	Limit      int           `json:"limit,omitempty"`
}

// Item is one file surfaced by a smart folder.
type Item struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	Extension string    `json:"extension"`
	Class     FileClass `json:"class"`
}

// Summary captures aggregate metadata for the files matched by a smart folder.
type Summary struct {
	TotalSize   int64               `json:"total_size"`
	ByClass     map[FileClass]int   `json:"by_class"`
	SizeByClass map[FileClass]int64 `json:"size_by_class"`
}

// Result is a deterministic smart-folder listing.
type Result struct {
	Rule      Rule    `json:"rule"`
	Items     []Item  `json:"items"`
	Scanned   int     `json:"scanned"`
	Matched   int     `json:"matched"`
	Summary   Summary `json:"summary"`
	Truncated bool    `json:"truncated"`
}

// Engine scans a root directory and evaluates smart-folder rules safely inside it.
type Engine struct {
	root string
	now  func() time.Time
}

// New creates a smart-folder engine rooted at root.
func New(root string) (*Engine, error) {
	if root == "" {
		return nil, fmt.Errorf("root path is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	return &Engine{root: filepath.Clean(realRoot), now: time.Now}, nil
}

// List evaluates rule below relPath. relPath may be empty, relative, or an absolute path under root.
func (e *Engine) List(relPath string, rule Rule) (*Result, error) {
	if e == nil {
		return nil, fmt.Errorf("nil smart folder engine")
	}
	start, err := e.resolve(relPath)
	if err != nil {
		return nil, err
	}
	if rule.Limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}

	matcher := compileRule(rule, e.now())
	res := &Result{
		Rule:  rule,
		Items: make([]Item, 0),
		Summary: Summary{
			ByClass:     make(map[FileClass]int),
			SizeByClass: make(map[FileClass]int64),
		},
	}
	err = filepath.WalkDir(start, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // keep listings useful when one subtree is unreadable
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		res.Scanned++
		item := Item{
			Path:      path,
			Name:      d.Name(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Extension: normalizedExt(d.Name()),
			Class:     Classify(d.Name()),
		}
		if !matcher(item) {
			return nil
		}
		res.Matched++
		res.Summary.TotalSize += item.Size
		res.Summary.ByClass[item.Class]++
		res.Summary.SizeByClass[item.Class] += item.Size
		if rule.Limit > 0 && len(res.Items) >= rule.Limit {
			res.Truncated = true
			return filepath.SkipAll
		}
		res.Items = append(res.Items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(res.Items, func(i, j int) bool {
		if !res.Items[i].ModTime.Equal(res.Items[j].ModTime) {
			return res.Items[i].ModTime.After(res.Items[j].ModTime)
		}
		return strings.ToLower(res.Items[i].Path) < strings.ToLower(res.Items[j].Path)
	})
	return res, nil
}

// BuiltInRules returns Synology/TrueNAS-style useful starter virtual folders.
func BuiltInRules() []Rule {
	return []Rule{
		{Name: "recent", ModifiedIn: 7 * 24 * time.Hour},
		{Name: "large-files", MinSize: 1 << 30},
		{Name: "photos", Classes: []FileClass{ClassPhoto}},
		{Name: "videos", Classes: []FileClass{ClassVideo}},
		{Name: "documents", Classes: []FileClass{ClassDocument}},
	}
}

// Classify maps a file name to a broad class by extension.
func Classify(name string) FileClass {
	switch normalizedExt(name) {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "heic", "heif", "tif", "tiff", "svg", "raw", "dng":
		return ClassPhoto
	case "mp4", "m4v", "mov", "mkv", "avi", "wmv", "webm", "flv", "mts", "m2ts":
		return ClassVideo
	case "mp3", "flac", "wav", "aac", "ogg", "m4a", "wma", "opus":
		return ClassAudio
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "txt", "md", "rtf", "csv":
		return ClassDocument
	case "zip", "tar", "gz", "bz2", "xz", "7z", "rar", "tgz":
		return ClassArchive
	case "go", "py", "js", "ts", "java", "c", "cpp", "h", "hpp", "rs", "sh", "html", "css", "json", "yaml", "yml", "xml", "sql":
		return ClassCode
	default:
		return ClassOther
	}
}

func (e *Engine) resolve(p string) (string, error) {
	if p == "" {
		return e.root, nil
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(e.root, p)
	}
	clean := filepath.Clean(p)
	realPath, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", err
	}
	if realPath != e.root && !strings.HasPrefix(realPath, e.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root: %s", p)
	}
	return realPath, nil
}

func compileRule(rule Rule, now time.Time) func(Item) bool {
	classes := make(map[FileClass]struct{}, len(rule.Classes))
	for _, c := range rule.Classes {
		classes[c] = struct{}{}
	}
	exts := make(map[string]struct{}, len(rule.Extensions))
	for _, ext := range rule.Extensions {
		exts[strings.TrimPrefix(strings.ToLower(ext), ".")] = struct{}{}
	}
	query := strings.ToLower(rule.NameQuery)
	cutoff := time.Time{}
	if rule.ModifiedIn > 0 {
		cutoff = now.Add(-rule.ModifiedIn)
	}
	return func(item Item) bool {
		if len(classes) > 0 {
			if _, ok := classes[item.Class]; !ok {
				return false
			}
		}
		if len(exts) > 0 {
			if _, ok := exts[item.Extension]; !ok {
				return false
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name), query) {
			return false
		}
		if rule.MinSize > 0 && item.Size < rule.MinSize {
			return false
		}
		if rule.MaxSize > 0 && item.Size > rule.MaxSize {
			return false
		}
		if !cutoff.IsZero() && item.ModTime.Before(cutoff) {
			return false
		}
		return true
	}
}

func normalizedExt(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}
