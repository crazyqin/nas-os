package sftp

// Config SFTP 服务器配置
type Config struct {
	Enabled        bool              `json:"enabled"`
	Port           int               `json:"port"`            // SFTP 端口 (默认 22)
	HostKeyPath    string            `json:"host_key_path"`   // SSH 主机密钥路径
	RootPath       string            `json:"root_path"`       // 根目录
	AllowAnonymous bool              `json:"allow_anonymous"` // 不建议启用
	MaxConnections int               `json:"max_connections"` // 最大连接数
	IdleTimeout    int               `json:"idle_timeout"`    // 空闲超时（秒）
	ChrootEnabled  bool              `json:"chroot_enabled"`  // 是否启用 chroot
	UserChroots    map[string]string `json:"user_chroots"`    // 用户 chroot 目录: username -> path
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Port:           22,
		HostKeyPath:    "/etc/nas-os/ssh_host_key",
		RootPath:       "/data/sftp",
		AllowAnonymous: false,
		MaxConnections: 100,
		IdleTimeout:    300, // 5 分钟
		ChrootEnabled:  true,
		UserChroots:    make(map[string]string),
	}
}
