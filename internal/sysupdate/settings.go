// settings.go - 更新检查设置持久化.
package sysupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// settingsFile 配置目录下的持久化文件名.
const settingsFile = "update-settings.json"

// Settings 更新检查设置.
type Settings struct {
	// AutoCheck 打开设置页时自动检查更新（前端每 24 小时至多一次）.
	AutoCheck bool `json:"autoCheck"`
}

// SettingsStore 设置持久化（JSON 文件，原子写入）.
type SettingsStore struct {
	path string
	mu   sync.Mutex
}

// NewSettingsStore 创建设置存储，dir 为状态目录（DataDir；
// ConfigDir 在容器部署常为只读挂载，运行时可变状态不放那里）.
func NewSettingsStore(dir string) *SettingsStore {
	return &SettingsStore{path: filepath.Join(dir, settingsFile)}
}

// Load 读取设置，文件缺失或损坏时返回零值（不报错）.
func (s *SettingsStore) Load() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return Settings{}
	}
	var set Settings
	if err := json.Unmarshal(data, &set); err != nil {
		return Settings{}
	}
	return set
}

// Save 写入设置（临时文件 + 原子改名）.
func (s *SettingsStore) Save(set Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
