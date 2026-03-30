package ftp

// Config FTP 服务器配置.
type Config struct {
	Enabled        bool              `json:"enabled"`
	Port           int               `json:"port"`            // FTP 端口 (默认 21)
	PasvPortStart  int               `json:"pasv_port_start"` // 被动模式端口范围起始
	PasvPortEnd    int               `json:"pasv_port_end"`   // 被动模式端口范围结束
	PasvHost       string            `json:"pasv_host"`       // 被动模式对外 IP
	RootPath       string            `json:"root_path"`       // 根目录
	AllowAnonymous bool              `json:"allow_anonymous"` // 允许匿名登录
	MaxConnections int               `json:"max_connections"` // 最大连接数
	BandwidthLimit BandwidthConfig   `json:"bandwidth_limit"` // 带宽限制
	VirtualDirs    map[string]string `json:"virtual_dirs"`    // 虚拟目录映射: 虚拟路径 -> 实际路径
}

// BandwidthConfig 带宽限制配置.
type BandwidthConfig struct {
	Enabled      bool  `json:"enabled"`
	DownloadKBps int64 `json:"download_kbps"` // 下载速率限制 (KB/s), 0 表示无限制
	UploadKBps   int64 `json:"upload_kbps"`   // 上传速率限制 (KB/s), 0 表示无限制
	PerUser      bool  `json:"per_user"`      // 是否按用户独立限制
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Port:           21,
		PasvPortStart:  50000,
		PasvPortEnd:    51000,
		PasvHost:       "",
		RootPath:       "/data/ftp",
		AllowAnonymous: false,
		MaxConnections: 100,
		BandwidthLimit: BandwidthConfig{
			Enabled:      false,
			DownloadKBps: 0,
			UploadKBps:   0,
			PerUser:      false,
		},
		VirtualDirs: make(map[string]string),
	}
}
