package docker

// Builtin application templates (extracted from appstore.go for maintainability).

func (s *AppStore) loadBuiltinTemplates() {
	templates := []*AppTemplate{
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储服务，支持文件同步、分享、在线办公",
			Category:    "Productivity",
			Icon:        "☁️",
			Version:     "latest",
			Image:       "nextcloud:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8080},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/www/html", Description: "数据目录", Default: "/opt/nas/apps/nextcloud/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nextcloud:
    image: nextcloud:latest
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/var/www/html
    environment:
      - NEXTCLOUD_TRUSTED_DOMAINS={{.TrustedDomains}}
`,
			Notes:   "首次访问需要创建管理员账户",
			Website: "https://nextcloud.com",
			Source:  "https://github.com/nextcloud",
		},
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "开源媒体服务器，支持电影、电视剧、音乐播放",
			Category:    "Media",
			Icon:        "🎬",
			Version:     "latest",
			Image:       "jellyfin/jellyfin:latest",
			Ports: []PortConfig{
				{Port: 8096, Protocol: "tcp", Description: "Web 界面", Default: 8096},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/jellyfin/config"},
				{ContainerPath: "/cache", Description: "缓存目录", Default: "/opt/nas/apps/jellyfin/cache"},
				{ContainerPath: "/media", Description: "媒体目录", Default: "/opt/nas/media"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8096"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.CacheDir}}:/cache
      - {{.MediaDir}}:/media
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次启动需要设置媒体库",
			Website: "https://jellyfin.org",
			Source:  "https://github.com/jellyfin/jellyfin",
		},
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			DisplayName: "Home Assistant",
			Description: "开源智能家居平台，支持数千种设备集成",
			Category:    "Smart Home",
			Icon:        "🏠",
			Version:     "stable",
			Image:       "homeassistant/home-assistant:stable",
			Ports: []PortConfig{
				{Port: 8123, Protocol: "tcp", Description: "Web 界面", Default: 8123},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/homeassistant/config"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  homeassistant:
    image: homeassistant/home-assistant:stable
    container_name: homeassistant
    restart: unless-stopped
    privileged: true
    network_mode: host
    volumes:
      - {{.ConfigDir}}:/config
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "使用 host 网络模式以支持设备发现",
			Website: "https://www.home-assistant.io",
			Source:  "https://github.com/home-assistant/core",
		},
		{
			ID:          "pihole",
			Name:        "pihole",
			DisplayName: "Pi-hole",
			Description: "网络级广告拦截器，DNS 服务器",
			Category:    "Network",
			Icon:        "🛡️",
			Version:     "latest",
			Image:       "pihole/pihole:latest",
			Ports: []PortConfig{
				{Port: 53, Protocol: "tcp", Description: "DNS (TCP)", Default: 53},
				{Port: 53, Protocol: "udp", Description: "DNS (UDP)", Default: 53},
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8081},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/etc/pihole", Description: "配置目录", Default: "/opt/nas/apps/pihole/etc"},
				{ContainerPath: "/etc/dnsmasq.d", Description: "DNS 配置", Default: "/opt/nas/apps/pihole/dnsmasq"},
			},
			Environment: map[string]string{
				"TZ": "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  pihole:
    image: pihole/pihole:latest
    container_name: pihole
    restart: unless-stopped
    ports:
      - "{{.DNSPort}}:53/tcp"
      - "{{.DNSPort}}:53/udp"
      - "{{.WebPort}}:80"
    volumes:
      - {{.ConfigDir}}:/etc/pihole
      - {{.DnsmasqDir}}:/etc/dnsmasq.d
    environment:
      - TZ=Asia/Shanghai
      - WEBPASSWORD={{.WebPassword}}
`,
			Notes:   "设置路由器 DNS 指向 Pi-hole 以全局拦截广告",
			Website: "https://pi-hole.net",
			Source:  "https://github.com/pi-hole/pi-hole",
		},
		{
			ID:          "transmission",
			Name:        "transmission",
			DisplayName: "Transmission",
			Description: "轻量级 BitTorrent 客户端",
			Category:    "Download",
			Icon:        "📥",
			Version:     "latest",
			Image:       "linuxserver/transmission:latest",
			Ports: []PortConfig{
				{Port: 9091, Protocol: "tcp", Description: "Web 界面", Default: 9091},
				{Port: 51413, Protocol: "tcp", Description: "BT 端口", Default: 51413},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/transmission/config"},
				{ContainerPath: "/downloads", Description: "下载目录", Default: "/opt/nas/downloads"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
				"TZ":   "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  transmission:
    image: linuxserver/transmission:latest
    container_name: transmission
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9091"
      - "{{.BTPort}}:51413"
      - "{{.BTPort}}:51413/udp"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.DownloadDir}}:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
			Notes:   "默认用户名/密码: admin/admin",
			Website: "https://transmissionbt.com",
			Source:  "https://github.com/transmission/transmission",
		},
		{
			ID:          "syncthing",
			Name:        "syncthing",
			DisplayName: "Syncthing",
			Description: "开源文件同步工具，支持多设备同步",
			Category:    "Productivity",
			Icon:        "🔄",
			Version:     "latest",
			Image:       "syncthing/syncthing:latest",
			Ports: []PortConfig{
				{Port: 8384, Protocol: "tcp", Description: "Web 界面", Default: 8384},
				{Port: 22000, Protocol: "tcp", Description: "同步端口", Default: 22000},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/syncthing", Description: "数据目录", Default: "/opt/nas/apps/syncthing/data"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  syncthing:
    image: syncthing/syncthing:latest
    container_name: syncthing
    restart: unless-stopped
    hostname: nas-syncthing
    ports:
      - "{{.WebPort}}:8384"
      - "{{.SyncPort}}:22000/tcp"
      - "{{.SyncPort}}:22000/udp"
    volumes:
      - {{.DataDir}}:/var/syncthing
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次访问需要设置用户名密码",
			Website: "https://syncthing.net",
			Source:  "https://github.com/syncthing/syncthing",
		},
		{
			ID:          "gitea",
			Name:        "gitea",
			DisplayName: "Gitea",
			Description: "轻量级 Git 服务，自建代码仓库",
			Category:    "Development",
			Icon:        "🐙",
			Version:     "latest",
			Image:       "gitea/gitea:latest",
			Ports: []PortConfig{
				{Port: 3000, Protocol: "tcp", Description: "Web 界面", Default: 3000},
				{Port: 22, Protocol: "tcp", Description: "SSH", Default: 2222},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/gitea/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  gitea:
    image: gitea/gitea:latest
    container_name: gitea
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3000"
      - "{{.SSHPort}}:22"
    volumes:
      - {{.DataDir}}:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "首次访问需要完成安装向导",
			Website: "https://gitea.io",
			Source:  "https://github.com/go-gitea/gitea",
		},
		{
			ID:          "vaultwarden",
			Name:        "vaultwarden",
			DisplayName: "Vaultwarden",
			Description: "Bitwarden 密码管理器服务端，管理所有密码",
			Category:    "Security",
			Icon:        "🔐",
			Version:     "latest",
			Image:       "vaultwarden/server:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8082},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/vaultwarden/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    container_name: vaultwarden
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/data
    environment:
      - SIGNUPS_ALLOWED=true
`,
			Notes:   "建议配合 HTTPS 使用，可搭配 Bitwarden 客户端",
			Website: "https://github.com/dani-garcia/vaultwarden",
			Source:  "https://github.com/dani-garcia/vaultwarden",
		},
		{
			ID:          "immich",
			Name:        "immich",
			DisplayName: "Immich",
			Description: "自托管照片和视频备份方案，类似 Google Photos",
			Category:    "Media",
			Icon:        "📸",
			Version:     "latest",
			Image:       "ghcr.io/immich-app/immich-server:latest",
			Ports: []PortConfig{
				{Port: 2283, Protocol: "tcp", Description: "Web 界面", Default: 2283},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/usr/src/app/upload", Description: "上传目录", Default: "/opt/nas/apps/immich/upload"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:latest
    container_name: immich-server
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3001"
    volumes:
      - {{.UploadDir}}:/usr/src/app/upload
      - /etc/localtime:/etc/localtime:ro
    environment:
      - DB_HOSTNAME=immich-db
      - DB_USERNAME=postgres
      - DB_PASSWORD=postgres
      - DB_DATABASE_NAME=immich
      - REDIS_HOSTNAME=immich-redis
    depends_on:
      - immich-db
      - immich-redis

  immich-db:
    image: tensorchord/pgvecto-rs:pg14-v0.2.0
    container_name: immich-db
    restart: unless-stopped
    volumes:
      - /opt/nas/apps/immich/db:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=immich

  immich-redis:
    image: redis:6.2-alpine
    container_name: immich-redis
    restart: unless-stopped
`,
			Notes:   "首次访问需要创建管理员账户，需要较多资源",
			Website: "https://immich.app",
			Source:  "https://github.com/immich-app/immich",
		},
		{
			ID:          "nginxproxymanager",
			Name:        "nginxproxymanager",
			DisplayName: "Nginx Proxy Manager",
			Description: "反向代理管理界面，SSL 证书自动申请",
			Category:    "Network",
			Icon:        "🌐",
			Version:     "latest",
			Image:       "jc21/nginx-proxy-manager:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "HTTP", Default: 80},
				{Port: 443, Protocol: "tcp", Description: "HTTPS", Default: 443},
				{Port: 81, Protocol: "tcp", Description: "管理界面", Default: 8181},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/npm/data"},
				{ContainerPath: "/etc/letsencrypt", Description: "证书目录", Default: "/opt/nas/apps/npm/letsencrypt"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nginx-proxy-manager:
    image: jc21/nginx-proxy-manager:latest
    container_name: nginx-proxy-manager
    restart: unless-stopped
    ports:
      - "{{.HTTPPort}}:80"
      - "{{.HTTPSPort}}:443"
      - "{{.WebPort}}:81"
    volumes:
      - {{.DataDir}}:/data
      - {{.CertsDir}}:/etc/letsencrypt
`,
			Notes:   "默认登录: admin@example.com / changeme",
			Website: "https://nginxproxymanager.com",
			Source:  "https://github.com/NginxProxyManager/nginx-proxy-manager",
		},
		{
			ID:          "portainer",
			Name:        "portainer",
			DisplayName: "Portainer",
			Description: "Docker 容器管理界面，可视化管理容器",
			Category:    "Development",
			Icon:        "🐳",
			Version:     "latest",
			Image:       "portainer/portainer-ce:latest",
			Ports: []PortConfig{
				{Port: 9443, Protocol: "tcp", Description: "Web 界面", Default: 9443},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/portainer/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  portainer:
    image: portainer/portainer-ce:latest
    container_name: portainer
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9443"
    volumes:
      - {{.DataDir}}:/data
      - /var/run/docker.sock:/var/run/docker.sock
`,
			Notes:   "首次访问需要设置管理员密码",
			Website: "https://www.portainer.io",
			Source:  "https://github.com/portainer/portainer",
		},
		// ==================== 监控栈 ====================
		{
			ID:          "prometheus",
			Name:        "prometheus",
			DisplayName: "Prometheus",
			Description: "开源监控系统，时序数据库，支持多维度数据采集",
			Category:    "Monitoring",
			Icon:        "📊",
			Version:     "latest",
			Image:       "prom/prometheus:latest",
			Ports: []PortConfig{
				{Port: 9090, Protocol: "tcp", Description: "Web 界面", Default: 9090},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/prometheus", Description: "数据目录", Default: "/opt/nas/apps/prometheus/data"},
				{ContainerPath: "/etc/prometheus", Description: "配置目录", Default: "/opt/nas/apps/prometheus/config"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9090"
    volumes:
      - {{.DataDir}}:/prometheus
      - {{.ConfigDir}}:/etc/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=30d'
`,
			Notes:   "首次启动需要在配置目录创建 prometheus.yml 配置文件",
			Website: "https://prometheus.io",
			Source:  "https://github.com/prometheus/prometheus",
		},
		{
			ID:          "grafana",
			Name:        "grafana",
			DisplayName: "Grafana",
			Description: "开源可视化平台，支持多种数据源，创建监控仪表盘",
			Category:    "Monitoring",
			Icon:        "📈",
			Version:     "latest",
			Image:       "grafana/grafana:latest",
			Ports: []PortConfig{
				{Port: 3000, Protocol: "tcp", Description: "Web 界面", Default: 3001},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/lib/grafana", Description: "数据目录", Default: "/opt/nas/apps/grafana/data"},
				{ContainerPath: "/etc/grafana/provisioning", Description: "配置目录", Default: "/opt/nas/apps/grafana/provisioning"},
			},
			Environment: map[string]string{
				"GF_SECURITY_ADMIN_USER":     "admin",
				"GF_SECURITY_ADMIN_PASSWORD": "admin",
				"GF_INSTALL_PLUGINS":         "",
			},
			Compose: `version: '3'
services:
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3000"
    volumes:
      - {{.DataDir}}:/var/lib/grafana
      - {{.ProvisioningDir}}:/etc/grafana/provisioning
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD={{.AdminPassword}}
`,
			Notes:   "默认账号密码: admin/admin，首次登录后请修改密码",
			Website: "https://grafana.com",
			Source:  "https://github.com/grafana/grafana",
		},
		{
			ID:          "victoriametrics",
			Name:        "victoriametrics",
			DisplayName: "VictoriaMetrics",
			Description: "高性能时序数据库，兼容Prometheus，低内存占用",
			Category:    "Monitoring",
			Icon:        "⚡",
			Version:     "latest",
			Image:       "victoriametrics/victoria-metrics:latest",
			Ports: []PortConfig{
				{Port: 8428, Protocol: "tcp", Description: "Web 界面/API", Default: 8428},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/victoria-metrics-data", Description: "数据目录", Default: "/opt/nas/apps/victoriametrics/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  victoriametrics:
    image: victoriametrics/victoria-metrics:latest
    container_name: victoriametrics
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8428"
    volumes:
      - {{.DataDir}}:/victoria-metrics-data
    command:
      - '--storageDataPath=/victoria-metrics-data'
      - '--httpListenAddr=:8428'
      - '--retentionPeriod=12m'
`,
			Notes:   "兼容Prometheus API，可作为Prometheus替代品，内存占用更低",
			Website: "https://victoriametrics.com",
			Source:  "https://github.com/VictoriaMetrics/VictoriaMetrics",
		},
		{
			ID:          "loki",
			Name:        "loki",
			DisplayName: "Loki",
			Description: "轻量级日志聚合系统，Grafana生态，支持日志查询和分析",
			Category:    "Monitoring",
			Icon:        "📝",
			Version:     "latest",
			Image:       "grafana/loki:latest",
			Ports: []PortConfig{
				{Port: 3100, Protocol: "tcp", Description: "HTTP API", Default: 3100},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/loki", Description: "数据目录", Default: "/opt/nas/apps/loki/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  loki:
    image: grafana/loki:latest
    container_name: loki
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3100"
    volumes:
      - {{.DataDir}}:/loki
    command: -config.file=/etc/loki/local-config.yaml
`,
			Notes:   "需搭配Promtail或Grafana Agent采集日志，Grafana可直接查询",
			Website: "https://grafana.com/oss/loki/",
			Source:  "https://github.com/grafana/loki",
		},
		{
			ID:          "tempo",
			Name:        "tempo",
			DisplayName: "Tempo",
			Description: "分布式追踪后端，支持Jaeger/Zipkin/OpenTelemetry格式",
			Category:    "Monitoring",
			Icon:        "🔍",
			Version:     "latest",
			Image:       "grafana/tempo:latest",
			Ports: []PortConfig{
				{Port: 3200, Protocol: "tcp", Description: "HTTP API", Default: 3200},
				{Port: 4317, Protocol: "tcp", Description: "OTLP gRPC", Default: 4317},
				{Port: 4318, Protocol: "tcp", Description: "OTLP HTTP", Default: 4318},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/tmp/tempo", Description: "数据目录", Default: "/opt/nas/apps/tempo/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  tempo:
    image: grafana/tempo:latest
    container_name: tempo
    restart: unless-stopped
    ports:
      - "{{.HTTPPort}}:3200"
      - "{{.OTLPGRPCPort}}:4317"
      - "{{.OTLPHTTPPort}}:4318"
    volumes:
      - {{.DataDir}}:/tmp/tempo
    command: ["-config.file=/etc/tempo.yaml"]
`,
			Notes:   "需创建tempo.yaml配置文件，配合Grafana可视化追踪数据",
			Website: "https://grafana.com/oss/tempo/",
			Source:  "https://github.com/grafana/tempo",
		},
		// ==================== 实用工具 ====================
		{
			ID:          "uptimekuma",
			Name:        "uptimekuma",
			DisplayName: "Uptime Kuma",
			Description: "自托管监控工具，美观的状态页面，支持多种通知方式",
			Category:    "Monitoring",
			Icon:        "⬆️",
			Version:     "latest",
			Image:       "louislam/uptime-kuma:latest",
			Ports: []PortConfig{
				{Port: 3001, Protocol: "tcp", Description: "Web 界面", Default: 3002},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/app/data", Description: "数据目录", Default: "/opt/nas/apps/uptimekuma/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  uptime-kuma:
    image: louislam/uptime-kuma:latest
    container_name: uptime-kuma
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3001"
    volumes:
      - {{.DataDir}}:/app/data
`,
			Notes:   "首次访问需要创建管理员账户",
			Website: "https://uptime.kuma.pet",
			Source:  "https://github.com/louislam/uptime-kuma",
		},
		{
			ID:          "filebrowser",
			Name:        "filebrowser",
			DisplayName: "FileBrowser",
			Description: "轻量级Web文件管理器，支持文件上传、下载、分享",
			Category:    "Productivity",
			Icon:        "📁",
			Version:     "latest",
			Image:       "filebrowser/filebrowser:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8083},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/srv", Description: "文件根目录", Default: "/opt/nas/files"},
				{ContainerPath: "/database", Description: "数据库目录", Default: "/opt/nas/apps/filebrowser/db"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  filebrowser:
    image: filebrowser/filebrowser:latest
    container_name: filebrowser
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.FilesDir}}:/srv
      - {{.DbDir}}:/database
`,
			Notes:   "默认账号密码: admin/admin",
			Website: "https://filebrowser.org",
			Source:  "https://github.com/filebrowser/filebrowser",
		},
		{
			ID:          "codeserver",
			Name:        "codeserver",
			DisplayName: "Code Server",
			Description: "浏览器中的VS Code，远程开发环境",
			Category:    "Development",
			Icon:        "💻",
			Version:     "latest",
			Image:       "codercom/code-server:latest",
			Ports: []PortConfig{
				{Port: 8080, Protocol: "tcp", Description: "Web 界面", Default: 8084},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/home/coder/project", Description: "项目目录", Default: "/opt/nas/projects"},
				{ContainerPath: "/home/coder/.config", Description: "配置目录", Default: "/opt/nas/apps/codeserver/config"},
			},
			Environment: map[string]string{
				"PASSWORD": "changeme",
			},
			Compose: `version: '3'
services:
  code-server:
    image: codercom/code-server:latest
    container_name: code-server
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8080"
    volumes:
      - {{.ProjectsDir}}:/home/coder/project
      - {{.ConfigDir}}:/home/coder/.config
    environment:
      - PASSWORD={{.Password}}
`,
			Notes:   "通过PASSWORD环境变量设置访问密码",
			Website: "https://coder.com",
			Source:  "https://github.com/coder/code-server",
		},
		// ==================== 网络/VPN ====================
		{
			ID:          "wireguard",
			Name:        "wireguard",
			DisplayName: "WireGuard",
			Description: "高性能VPN服务器，简洁配置，快速连接",
			Category:    "Network",
			Icon:        "🔒",
			Version:     "latest",
			Image:       "linuxserver/wireguard:latest",
			Ports: []PortConfig{
				{Port: 51820, Protocol: "udp", Description: "VPN 端口", Default: 51820},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/wireguard/config"},
			},
			Environment: map[string]string{
				"PUID":            "1000",
				"PGID":            "1000",
				"TZ":              "Asia/Shanghai",
				"SERVERURL":       "auto",
				"SERVERPORT":      "51820",
				"PEERS":           "1",
				"PEERDNS":         "auto",
				"INTERNAL_SUBNET": "10.13.13.0",
			},
			Compose: `version: '3'
services:
  wireguard:
    image: linuxserver/wireguard:latest
    container_name: wireguard
    restart: unless-stopped
    privileged: true
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - SERVERURL={{.ServerURL}}
      - SERVERPORT=51820
      - PEERS={{.Peers}}
      - PEERDNS=auto
      - INTERNAL_SUBNET=10.13.13.0
    volumes:
      - {{.ConfigDir}}:/config
      - /lib/modules:/lib/modules
    ports:
      - "{{.VPNPort}}:51820/udp"
`,
			Notes:   "配置文件生成在/config目录，客户端配置在/config/peer_*目录",
			Website: "https://www.wireguard.com",
			Source:  "https://github.com/linuxserver/docker-wireguard",
		},
		{
			ID:          "duckdns",
			Name:        "duckdns",
			DisplayName: "DuckDNS",
			Description: "免费动态DNS服务，自动更新IP地址",
			Category:    "Network",
			Icon:        "🦆",
			Version:     "latest",
			Image:       "linuxserver/duckdns:latest",
			Ports:       []PortConfig{},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/duckdns/config"},
			},
			Environment: map[string]string{
				"PUID":       "1000",
				"PGID":       "1000",
				"TZ":         "Asia/Shanghai",
				"SUBDOMAINS": "your-subdomain",
				"TOKEN":      "your-token",
			},
			Compose: `version: '3'
services:
  duckdns:
    image: linuxserver/duckdns:latest
    container_name: duckdns
    restart: unless-stopped
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - SUBDOMAINS={{.Subdomains}}
      - TOKEN={{.Token}}
    volumes:
      - {{.ConfigDir}}:/config
`,
			Notes:   "需要在 duckdns.org 注册账户获取 token",
			Website: "https://www.duckdns.org",
			Source:  "https://github.com/linuxserver/docker-duckdns",
		},
		// ==================== 智能家居 ====================
		{
			ID:          "homebridge",
			Name:        "homebridge",
			DisplayName: "Homebridge",
			Description: "智能家居桥接，让非HomeKit设备接入Apple HomeKit",
			Category:    "Smart Home",
			Icon:        "🏠",
			Version:     "latest",
			Image:       "homebridge/homebridge:latest",
			Ports: []PortConfig{
				{Port: 8581, Protocol: "tcp", Description: "Web 界面", Default: 8581},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/homebridge", Description: "数据目录", Default: "/opt/nas/apps/homebridge/data"},
			},
			Environment: map[string]string{
				"TZ": "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  homebridge:
    image: homebridge/homebridge:latest
    container_name: homebridge
    restart: unless-stopped
    network_mode: host
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - {{.DataDir}}:/homebridge
`,
			Notes:   "首次访问设置账户，然后安装插件添加智能家居设备",
			Website: "https://homebridge.io",
			Source:  "https://github.com/homebridge/homebridge",
		},
		// ==================== 媒体服务 ====================
		{
			ID:          "plex",
			Name:        "plex",
			DisplayName: "Plex",
			Description: "功能强大的媒体服务器，支持转码、远程访问、多设备同步",
			Category:    "Media",
			Icon:        "🎥",
			Version:     "latest",
			Image:       "plexinc/pms-docker:latest",
			Ports: []PortConfig{
				{Port: 32400, Protocol: "tcp", Description: "Web 界面", Default: 32400},
				{Port: 1900, Protocol: "udp", Description: "DLNA", Default: 1900},
				{Port: 3005, Protocol: "tcp", Description: "Plex Companion", Default: 3005},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/plex/config"},
				{ContainerPath: "/transcode", Description: "转码目录", Default: "/opt/nas/apps/plex/transcode"},
				{ContainerPath: "/data", Description: "媒体目录", Default: "/opt/nas/media"},
			},
			Environment: map[string]string{
				"TZ":               "Asia/Shanghai",
				"PLEX_CLAIM":       "",
				"ALLOWED_NETWORKS": "172.16.0.0/12,192.168.0.0/16,10.0.0.0/8",
			},
			Compose: `version: '3'
services:
  plex:
    image: plexinc/pms-docker:latest
    container_name: plex
    restart: unless-stopped
    hostname: plex
    ports:
      - "{{.WebPort}}:32400"
      - "{{.DLNAPort}}:1900/udp"
      - "{{.CompanionPort}}:3005"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.TranscodeDir}}:/transcode
      - {{.MediaDir}}:/data
    environment:
      - TZ=Asia/Shanghai
      - PLEX_CLAIM={{.PlexClaim}}
      - ALLOWED_NETWORKS=172.16.0.0/12,192.168.0.0/16,10.0.0.0/8
`,
			Notes:   "首次访问需要登录Plex账户，PLEX_CLAIM可从plex.tv/claim获取",
			Website: "https://www.plex.tv",
			Source:  "https://github.com/plexinc/pms-docker",
		},
		// ==================== 数据库 ====================
		{
			ID:          "postgresql",
			Name:        "postgresql",
			DisplayName: "PostgreSQL",
			Description: "强大的开源关系型数据库，支持JSON、全文搜索",
			Category:    "Database",
			Icon:        "🐘",
			Version:     "16",
			Image:       "postgres:16",
			Ports: []PortConfig{
				{Port: 5432, Protocol: "tcp", Description: "数据库端口", Default: 5432},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/lib/postgresql/data", Description: "数据目录", Default: "/opt/nas/apps/postgresql/data"},
			},
			Environment: map[string]string{
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "changeme",
				"POSTGRES_DB":       "appdb",
			},
			Compose: `version: '3'
services:
  postgres:
    image: postgres:16
    container_name: postgres
    restart: unless-stopped
    ports:
      - "{{.DBPort}}:5432"
    volumes:
      - {{.DataDir}}:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER={{.DBUser}}
      - POSTGRES_PASSWORD={{.DBPassword}}
      - POSTGRES_DB={{.DBName}}
`,
			Notes:   "请修改默认密码，支持自定义用户名和数据库名",
			Website: "https://www.postgresql.org",
			Source:  "https://github.com/docker-library/postgres",
		},
		{
			ID:          "mysql",
			Name:        "mysql",
			DisplayName: "MySQL",
			Description: "流行的开源关系型数据库，广泛用于Web应用",
			Category:    "Database",
			Icon:        "🗄️",
			Version:     "8",
			Image:       "mysql:8",
			Ports: []PortConfig{
				{Port: 3306, Protocol: "tcp", Description: "数据库端口", Default: 3306},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/lib/mysql", Description: "数据目录", Default: "/opt/nas/apps/mysql/data"},
			},
			Environment: map[string]string{
				"MYSQL_ROOT_PASSWORD": "changeme",
				"MYSQL_DATABASE":      "appdb",
				"MYSQL_USER":          "appuser",
				"MYSQL_PASSWORD":      "changeme",
			},
			Compose: `version: '3'
services:
  mysql:
    image: mysql:8
    container_name: mysql
    restart: unless-stopped
    ports:
      - "{{.DBPort}}:3306"
    volumes:
      - {{.DataDir}}:/var/lib/mysql
    environment:
      - MYSQL_ROOT_PASSWORD={{.RootPassword}}
      - MYSQL_DATABASE={{.DBName}}
      - MYSQL_USER={{.DBUser}}
      - MYSQL_PASSWORD={{.DBPassword}}
`,
			Notes:   "请修改默认密码，支持创建普通用户",
			Website: "https://www.mysql.com",
			Source:  "https://github.com/docker-library/mysql",
		},
		{
			ID:          "redis",
			Name:        "redis",
			DisplayName: "Redis",
			Description: "高性能内存数据库，支持缓存、消息队列、发布订阅",
			Category:    "Database",
			Icon:        "⚡",
			Version:     "7",
			Image:       "redis:7-alpine",
			Ports: []PortConfig{
				{Port: 6379, Protocol: "tcp", Description: "Redis 端口", Default: 6379},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/redis/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  redis:
    image: redis:7-alpine
    container_name: redis
    restart: unless-stopped
    ports:
      - "{{.RedisPort}}:6379"
    volumes:
      - {{.DataDir}}:/data
    command: redis-server --appendonly yes
`,
			Notes:   "启用AOF持久化，重启后数据不丢失",
			Website: "https://redis.io",
			Source:  "https://github.com/docker-library/redis",
		},
		// ==================== 下载工具 ====================
		{
			ID:          "qbittorrent",
			Name:        "qbittorrent",
			DisplayName: "qBittorrent",
			Description: "功能丰富的BitTorrent客户端，WebUI管理界面",
			Category:    "Download",
			Icon:        "🔵",
			Version:     "latest",
			Image:       "linuxserver/qbittorrent:latest",
			Ports: []PortConfig{
				{Port: 8080, Protocol: "tcp", Description: "Web 界面", Default: 8085},
				{Port: 6881, Protocol: "tcp", Description: "BT 端口", Default: 6881},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/qbittorrent/config"},
				{ContainerPath: "/downloads", Description: "下载目录", Default: "/opt/nas/downloads"},
			},
			Environment: map[string]string{
				"PUID":       "1000",
				"PGID":       "1000",
				"TZ":         "Asia/Shanghai",
				"WEBUI_PORT": "8080",
			},
			Compose: `version: '3'
services:
  qbittorrent:
    image: linuxserver/qbittorrent:latest
    container_name: qbittorrent
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8080"
      - "{{.BTPort}}:6881"
      - "{{.BTPort}}:6881/udp"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.DownloadDir}}:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - WEBUI_PORT=8080
`,
			Notes:   "默认用户名: admin，密码: adminadmin",
			Website: "https://www.qbittorrent.org",
			Source:  "https://github.com/qbittorrent/qBittorrent",
		},
	}

	for _, t := range templates {
		s.templates[t.ID] = t
	}
}

// loadInstalled 加载已安装应用.
