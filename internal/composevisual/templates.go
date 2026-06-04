package composevisual

import "time"

// initTemplates 初始化预设模板
func (m *Manager) initTemplates() {
	now := time.Now()

	// 1. WordPress
	m.templates["wordpress"] = &ComposeTemplate{
		ID:          "wordpress",
		Name:        "WordPress",
		Description: "经典博客/CMS平台，含MySQL和phpMyAdmin",
		Category:    "cms",
		Icon:        "wordpress",
		Tags:        []string{"cms", "blog", "php", "mysql"},
		Rating:      4.8,
		Downloads:   15000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		EnvExample: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root_password",
			"MYSQL_DATABASE":      "wordpress",
			"MYSQL_USER":          "wp_user",
			"MYSQL_PASSWORD":      "wp_password",
		},
		Services: map[string]*ServiceNode{
			"wordpress": {
				Name: "wordpress", Image: "wordpress:6.4-php8.2-apache",
				Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
				Volumes: []VolumeMapping{{Type: "volume", Source: "wp_data", Target: "/var/www/html"}},
				Environment: map[string]string{
					"WORDPRESS_DB_HOST":     "db:3306",
					"WORDPRESS_DB_USER":     "${MYSQL_USER}",
					"WORDPRESS_DB_PASSWORD": "${MYSQL_PASSWORD}",
					"WORDPRESS_DB_NAME":     "${MYSQL_DATABASE}",
				},
				DependsOn:   []string{"db"},
				Restart:     "unless-stopped",
				Resources:   SuggestResources("wordpress"),
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:80/"}, Interval: "30s", Timeout: "10s", Retries: 3, StartPeriod: "30s"},
			},
			"db": {
				Name: "db", Image: "mysql:8.0",
				Volumes: []VolumeMapping{{Type: "volume", Source: "db_data", Target: "/var/lib/mysql"}},
				Environment: map[string]string{
					"MYSQL_ROOT_PASSWORD": "${MYSQL_ROOT_PASSWORD}",
					"MYSQL_DATABASE":      "${MYSQL_DATABASE}",
					"MYSQL_USER":          "${MYSQL_USER}",
					"MYSQL_PASSWORD":      "${MYSQL_PASSWORD}",
				},
				Restart:     "unless-stopped",
				Resources:   SuggestResources("mysql"),
				HealthCheck: &HealthCheck{Test: []string{"CMD", "mysqladmin", "ping", "-h", "localhost"}, Interval: "10s", Timeout: "5s", Retries: 5, StartPeriod: "30s"},
			},
			"phpmyadmin": {
				Name: "phpmyadmin", Image: "phpmyadmin:latest",
				Ports:     []PortMapping{{HostPort: 8081, ContainerPort: 80, Protocol: "tcp"}},
				DependsOn: []string{"db"},
				Environment: map[string]string{
					"PMA_HOST":     "db",
					"PMA_PORT":     "3306",
					"MYSQL_ROOT_PASSWORD": "${MYSQL_ROOT_PASSWORD}",
				},
				Restart: "unless-stopped",
			},
		},
		Volumes: map[string]*VolumeConfig{
			"wp_data":  {Driver: "local"},
			"db_data":  {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 2. NextCloud
	m.templates["nextcloud"] = &ComposeTemplate{
		ID:          "nextcloud",
		Name:        "NextCloud",
		Description: "私有云盘/文件同步平台",
		Category:    "storage",
		Icon:        "nextcloud",
		Tags:        []string{"cloud", "storage", "sync", "sharing"},
		Rating:      4.7,
		Downloads:   12000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		EnvExample: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root_password",
			"MYSQL_DATABASE":      "nextcloud",
			"MYSQL_USER":          "nc_user",
			"MYSQL_PASSWORD":      "nc_password",
		},
		Services: map[string]*ServiceNode{
			"nextcloud": {
				Name: "nextcloud", Image: "nextcloud:28-apache",
				Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "volume", Source: "nc_data", Target: "/var/www/html"},
					{Type: "bind", Source: "/mnt/storage/nextcloud", Target: "/var/www/html/data"},
				},
				DependsOn: []string{"db", "redis"},
				Environment: map[string]string{
					"MYSQL_HOST":            "db:3306",
					"MYSQL_DATABASE":        "${MYSQL_DATABASE}",
					"MYSQL_USER":            "${MYSQL_USER}",
					"MYSQL_PASSWORD":        "${MYSQL_PASSWORD}",
					"REDIS_HOST":            "redis",
					"NEXTCLOUD_ADMIN_USER":  "admin",
					"NEXTCLOUD_ADMIN_PASSWORD": "admin_password",
				},
				Restart: "unless-stopped",
			},
			"db": {
				Name: "db", Image: "mariadb:10.11",
				Volumes: []VolumeMapping{{Type: "volume", Source: "nc_db", Target: "/var/lib/mysql"}},
				Environment: map[string]string{
					"MYSQL_ROOT_PASSWORD": "${MYSQL_ROOT_PASSWORD}",
					"MYSQL_DATABASE":      "${MYSQL_DATABASE}",
					"MYSQL_USER":          "${MYSQL_USER}",
					"MYSQL_PASSWORD":      "${MYSQL_PASSWORD}",
				},
				Restart:   "unless-stopped",
				Resources: SuggestResources("mariadb"),
			},
			"redis": {
				Name: "redis", Image: "redis:7-alpine",
				Restart: "unless-stopped",
				Resources: &ResourceLimits{CPUs: "0.5", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.1", Memory: "64M"}},
			},
		},
		Volumes: map[string]*VolumeConfig{
			"nc_data": {Driver: "local"},
			"nc_db":   {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 3. GitLab CE
	m.templates["gitlab"] = &ComposeTemplate{
		ID:          "gitlab",
		Name:        "GitLab CE",
		Description: "自托管Git仓库和CI/CD平台",
		Category:    "devops",
		Icon:        "gitlab",
		Tags:        []string{"git", "ci-cd", "devops", "code"},
		Rating:      4.6,
		Downloads:   8000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		EnvExample: map[string]string{
			"GITLAB_OMNIBUS_CONFIG": "external_url 'http://gitlab.local'",
		},
		Services: map[string]*ServiceNode{
			"gitlab": {
				Name: "gitlab", Image: "gitlab/gitlab-ce:16.8.0-ce.0",
				Ports: []PortMapping{
					{HostPort: 8929, ContainerPort: 8929, Protocol: "tcp"},
					{HostPort: 2222, ContainerPort: 22, Protocol: "tcp"},
				},
				Volumes: []VolumeMapping{
					{Type: "volume", Source: "gitlab_config", Target: "/etc/gitlab"},
					{Type: "volume", Source: "gitlab_logs", Target: "/var/log/gitlab"},
					{Type: "volume", Source: "gitlab_data", Target: "/var/opt/gitlab"},
				},
				Environment: map[string]string{"GITLAB_OMNIBUS_CONFIG": "${GITLAB_OMNIBUS_CONFIG}"},
				DependsOn:   []string{"redis", "postgres"},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "4.0", Memory: "8G", Reservations: &ResourceReservation{CPUs: "2.0", Memory: "4G"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:8929/-/readiness"}, Interval: "60s", Timeout: "10s", Retries: 5, StartPeriod: "120s"},
			},
			"redis": {
				Name: "redis", Image: "redis:7-alpine",
				Volumes:   []VolumeMapping{{Type: "volume", Source: "gitlab_redis", Target: "/data"}},
				Restart:   "unless-stopped",
				Resources: &ResourceLimits{CPUs: "0.5", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.1", Memory: "64M"}},
			},
			"postgres": {
				Name: "postgres", Image: "postgres:15-alpine",
				Volumes: []VolumeMapping{{Type: "volume", Source: "gitlab_pg", Target: "/var/lib/postgresql/data"}},
				Environment: map[string]string{
					"POSTGRES_DB":       "gitlabhq_production",
					"POSTGRES_USER":     "gitlab",
					"POSTGRES_PASSWORD": "gitlab_password",
				},
				Restart:   "unless-stopped",
				Resources: SuggestResources("postgres"),
			},
		},
		Volumes: map[string]*VolumeConfig{
			"gitlab_config": {Driver: "local"},
			"gitlab_logs":   {Driver: "local"},
			"gitlab_data":   {Driver: "local"},
			"gitlab_redis":  {Driver: "local"},
			"gitlab_pg":     {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 4. Home Assistant
	m.templates["homeassistant"] = &ComposeTemplate{
		ID:          "homeassistant",
		Name:        "Home Assistant",
		Description: "智能家居自动化平台",
		Category:    "smart_home",
		Icon:        "home-assistant",
		Tags:        []string{"smart-home", "iot", "automation", "hass"},
		Rating:      4.9,
		Downloads:   18000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"homeassistant": {
				Name: "homeassistant", Image: "ghcr.io/home-assistant/home-assistant:stable",
				Ports: []PortMapping{{HostPort: 8123, ContainerPort: 8123, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "/opt/homeassistant", Target: "/config"},
					{Type: "bind", Source: "/etc/localtime", Target: "/etc/localtime", ReadOnly: true},
				},
				Restart:     "unless-stopped",
				Networks:    []string{"host"},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:8123/"}, Interval: "30s", Timeout: "10s", Retries: 3, StartPeriod: "60s"},
			},
			"mqtt": {
				Name: "mqtt", Image: "eclipse-mosquitto:2",
				Ports: []PortMapping{{HostPort: 1883, ContainerPort: 1883, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "volume", Source: "mqtt_config", Target: "/mosquitto/config"},
					{Type: "volume", Source: "mqtt_data", Target: "/mosquitto/data"},
					{Type: "volume", Source: "mqtt_log", Target: "/mosquitto/log"},
				},
				Restart: "unless-stopped",
			},
			"zigbee2mqtt": {
				Name: "zigbee2mqtt", Image: "koenkk/zigbee2mqtt",
				Ports: []PortMapping{{HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "/opt/zigbee2mqtt/data", Target: "/app/data"},
					{Type: "bind", Source: "/run/udev", Target: "/run/udev", ReadOnly: true},
				},
				DependsOn:   []string{"mqtt"},
				Restart:     "unless-stopped",
				Environment: map[string]string{"TZ": "Asia/Shanghai"},
			},
		},
		Volumes: map[string]*VolumeConfig{
			"mqtt_config": {Driver: "local"},
			"mqtt_data":   {Driver: "local"},
			"mqtt_log":    {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 5. Jellyfin
	m.templates["jellyfin"] = &ComposeTemplate{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "开源媒体服务器，支持视频/音乐/图片",
		Category:    "media",
		Icon:        "jellyfin",
		Tags:        []string{"media", "streaming", "video", "music"},
		Rating:      4.7,
		Downloads:   14000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"jellyfin": {
				Name: "jellyfin", Image: "jellyfin/jellyfin:latest",
				Ports: []PortMapping{
					{HostPort: 8096, ContainerPort: 8096, Protocol: "tcp"},
					{HostPort: 7359, ContainerPort: 7359, Protocol: "udp"},
				},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "/opt/jellyfin/config", Target: "/config"},
					{Type: "bind", Source: "/opt/jellyfin/cache", Target: "/cache"},
					{Type: "bind", Source: "/mnt/media/movies", Target: "/media/movies"},
					{Type: "bind", Source: "/mnt/media/tvshows", Target: "/media/tvshows"},
					{Type: "bind", Source: "/mnt/media/music", Target: "/media/music"},
				},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "4.0", Memory: "4G", Reservations: &ResourceReservation{CPUs: "1.0", Memory: "1G"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:8096/health"}, Interval: "30s", Timeout: "10s", Retries: 3, StartPeriod: "60s"},
			},
		},
		Volumes: map[string]*VolumeConfig{},
		CreatedAt: now,
	}

	// 6. Nginx + MySQL + PHP (LNMP)
	m.templates["lnmp"] = &ComposeTemplate{
		ID:          "lnmp",
		Name:        "LNMP Stack",
		Description: "Nginx + MySQL + PHP 经典Web开发环境",
		Category:    "web",
		Icon:        "lnmp",
		Tags:        []string{"web", "nginx", "php", "mysql", "lnmp"},
		Rating:      4.5,
		Downloads:   10000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		EnvExample: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root_password",
			"MYSQL_DATABASE":      "app_db",
		},
		Services: map[string]*ServiceNode{
			"nginx": {
				Name: "nginx", Image: "nginx:1.25-alpine",
				Ports: []PortMapping{{HostPort: 80, ContainerPort: 80, Protocol: "tcp"}, {HostPort: 443, ContainerPort: 443, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "./nginx/conf.d", Target: "/etc/nginx/conf.d"},
					{Type: "bind", Source: "./www", Target: "/var/www/html"},
				},
				DependsOn: []string{"php"},
				Restart:   "unless-stopped",
			},
			"php": {
				Name: "php", Image: "php:8.2-fpm-alpine",
				Volumes: []VolumeMapping{{Type: "bind", Source: "./www", Target: "/var/www/html"}},
				DependsOn: []string{"db"},
				Restart:   "unless-stopped",
				Resources: SuggestResources("php"),
			},
			"db": {
				Name: "db", Image: "mysql:8.0",
				Volumes: []VolumeMapping{{Type: "volume", Source: "mysql_data", Target: "/var/lib/mysql"}},
				Environment: map[string]string{
					"MYSQL_ROOT_PASSWORD": "${MYSQL_ROOT_PASSWORD}",
					"MYSQL_DATABASE":      "${MYSQL_DATABASE}",
				},
				Restart:     "unless-stopped",
				Resources:   SuggestResources("mysql"),
				HealthCheck: &HealthCheck{Test: []string{"CMD", "mysqladmin", "ping", "-h", "localhost"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
		Volumes: map[string]*VolumeConfig{"mysql_data": {Driver: "local"}},
		CreatedAt: now,
	}

	// 7. Portainer
	m.templates["portainer"] = &ComposeTemplate{
		ID:          "portainer",
		Name:        "Portainer CE",
		Description: "Docker容器管理Web界面",
		Category:    "devops",
		Icon:        "portainer",
		Tags:        []string{"docker", "management", "devops", "ui"},
		Rating:      4.6,
		Downloads:   20000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"portainer": {
				Name: "portainer", Image: "portainer/portainer-ce:latest",
				Ports: []PortMapping{{HostPort: 9000, ContainerPort: 9000, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
					{Type: "volume", Source: "portainer_data", Target: "/data"},
				},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "0.5", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.1", Memory: "64M"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:9000/"}, Interval: "30s", Timeout: "5s", Retries: 3},
			},
		},
		Volumes: map[string]*VolumeConfig{"portainer_data": {Driver: "local"}},
		CreatedAt: now,
	}

	// 8. Grafana + Prometheus + Node Exporter
	m.templates["monitoring"] = &ComposeTemplate{
		ID:          "monitoring",
		Name:        "Grafana + Prometheus",
		Description: "完整监控方案：Prometheus采集 + Grafana展示",
		Category:    "devops",
		Icon:        "grafana",
		Tags:        []string{"monitoring", "metrics", "grafana", "prometheus"},
		Rating:      4.8,
		Downloads:   16000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"grafana": {
				Name: "grafana", Image: "grafana/grafana:latest",
				Ports:     []PortMapping{{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"}},
				Volumes:   []VolumeMapping{{Type: "volume", Source: "grafana_data", Target: "/var/lib/grafana"}},
				DependsOn: []string{"prometheus"},
				Environment: map[string]string{
					"GF_SECURITY_ADMIN_PASSWORD": "admin",
				},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "1.0", Memory: "512M", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "128M"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:3000/api/health"}, Interval: "30s", Timeout: "5s", Retries: 3},
			},
			"prometheus": {
				Name: "prometheus", Image: "prom/prometheus:latest",
				Ports: []PortMapping{{HostPort: 9090, ContainerPort: 9090, Protocol: "tcp"}},
				Volumes: []VolumeMapping{
					{Type: "bind", Source: "./prometheus/prometheus.yml", Target: "/etc/prometheus/prometheus.yml"},
					{Type: "volume", Source: "prometheus_data", Target: "/prometheus"},
				},
				Restart:   "unless-stopped",
				Resources: SuggestResources("prometheus"),
			},
			"node-exporter": {
				Name: "node-exporter", Image: "prom/node-exporter:latest",
				Ports:   []PortMapping{{HostPort: 9100, ContainerPort: 9100, Protocol: "tcp"}},
				Volumes: []VolumeMapping{{Type: "bind", Source: "/proc", Target: "/host/proc", ReadOnly: true}, {Type: "bind", Source: "/sys", Target: "/host/sys", ReadOnly: true}},
				Restart: "unless-stopped",
			},
		},
		Volumes: map[string]*VolumeConfig{
			"grafana_data":    {Driver: "local"},
			"prometheus_data": {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 9. MinIO
	m.templates["minio"] = &ComposeTemplate{
		ID:          "minio",
		Name:        "MinIO",
		Description: "高性能S3兼容对象存储",
		Category:    "storage",
		Icon:        "minio",
		Tags:        []string{"s3", "object-storage", "storage", "minio"},
		Rating:      4.7,
		Downloads:   11000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		EnvExample: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Services: map[string]*ServiceNode{
			"minio": {
				Name: "minio", Image: "minio/minio:latest",
				Ports: []PortMapping{
					{HostPort: 9000, ContainerPort: 9000, Protocol: "tcp"},
					{HostPort: 9001, ContainerPort: 9001, Protocol: "tcp"},
				},
				Volumes: []VolumeMapping{{Type: "volume", Source: "minio_data", Target: "/data"}},
				Environment: map[string]string{
					"MINIO_ROOT_USER":     "${MINIO_ROOT_USER}",
					"MINIO_ROOT_PASSWORD": "${MINIO_ROOT_PASSWORD}",
				},
				Command:     []string{"server", "/data", "--console-address", ":9001"},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "2.0", Memory: "2G", Reservations: &ResourceReservation{CPUs: "0.5", Memory: "512M"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:9000/minio/health/live"}, Interval: "30s", Timeout: "10s", Retries: 3},
			},
		},
		Volumes: map[string]*VolumeConfig{"minio_data": {Driver: "local"}},
		CreatedAt: now,
	}

	// 10. Nginx Proxy Manager
	m.templates["nginx-proxy-manager"] = &ComposeTemplate{
		ID:          "nginx-proxy-manager",
		Name:        "Nginx Proxy Manager",
		Description: "可视化Nginx反向代理管理，自带SSL证书",
		Category:    "network",
		Icon:        "nginx",
		Tags:        []string{"nginx", "proxy", "ssl", "reverse-proxy"},
		Rating:      4.6,
		Downloads:   13000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"npm": {
				Name: "npm", Image: "jc21/nginx-proxy-manager:latest",
				Ports: []PortMapping{
					{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
					{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
					{HostPort: 81, ContainerPort: 81, Protocol: "tcp"},
				},
				Volumes: []VolumeMapping{
					{Type: "volume", Source: "npm_data", Target: "/data"},
					{Type: "volume", Source: "npm_letsencrypt", Target: "/etc/letsencrypt"},
				},
				DependsOn:   []string{"db"},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "1.0", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "64M"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:81/"}, Interval: "30s", Timeout: "10s", Retries: 3},
			},
			"db": {
				Name: "db", Image: "mariadb:10.11",
				Volumes: []VolumeMapping{{Type: "volume", Source: "npm_db", Target: "/var/lib/mysql"}},
				Environment: map[string]string{
					"MYSQL_ROOT_PASSWORD": "npm_root_pw",
					"MYSQL_DATABASE":      "npm",
					"MYSQL_USER":          "npm",
					"MYSQL_PASSWORD":      "npm_password",
				},
				Restart: "unless-stopped",
			},
		},
		Volumes: map[string]*VolumeConfig{
			"npm_data":        {Driver: "local"},
			"npm_letsencrypt": {Driver: "local"},
			"npm_db":          {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 11. Gitea
	m.templates["gitea"] = &ComposeTemplate{
		ID:          "gitea",
		Name:        "Gitea",
		Description: "轻量级自托管Git服务",
		Category:    "devops",
		Icon:        "gitea",
		Tags:        []string{"git", "code", "devops", "lightweight"},
		Rating:      4.5,
		Downloads:   9000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"gitea": {
				Name: "gitea", Image: "gitea/gitea:latest",
				Ports: []PortMapping{
					{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
					{HostPort: 2222, ContainerPort: 22, Protocol: "tcp"},
				},
				Volumes: []VolumeMapping{
					{Type: "volume", Source: "gitea_data", Target: "/data"},
					{Type: "bind", Source: "/etc/timezone", Target: "/etc/timezone", ReadOnly: true},
				},
				DependsOn:   []string{"db"},
				Environment: map[string]string{"GITEA__database__DB_TYPE": "postgres", "GITEA__database__HOST": "db:5432"},
				Restart:     "unless-stopped",
			},
			"db": {
				Name: "db", Image: "postgres:15-alpine",
				Volumes: []VolumeMapping{{Type: "volume", Source: "gitea_db", Target: "/var/lib/postgresql/data"}},
				Environment: map[string]string{
					"POSTGRES_DB":       "gitea",
					"POSTGRES_USER":     "gitea",
					"POSTGRES_PASSWORD": "gitea_password",
				},
				Restart: "unless-stopped",
			},
		},
		Volumes: map[string]*VolumeConfig{
			"gitea_data": {Driver: "local"},
			"gitea_db":   {Driver: "local"},
		},
		CreatedAt: now,
	}

	// 12. Uptime Kuma
	m.templates["uptimekuma"] = &ComposeTemplate{
		ID:          "uptimekuma",
		Name:        "Uptime Kuma",
		Description: "自托管监控工具，美观UI + 多渠道通知",
		Category:    "devops",
		Icon:        "uptime-kuma",
		Tags:        []string{"monitoring", "uptime", "status", "notifications"},
		Rating:      4.8,
		Downloads:   14000,
		Author:      "ComposeVisual",
		Version:     "1.0.0",
		Services: map[string]*ServiceNode{
			"uptimekuma": {
				Name: "uptimekuma", Image: "louislam/uptime-kuma:latest",
				Ports:   []PortMapping{{HostPort: 3001, ContainerPort: 3001, Protocol: "tcp"}},
				Volumes: []VolumeMapping{{Type: "volume", Source: "uptimekuma_data", Target: "/app/data"}},
				Restart:     "unless-stopped",
				Resources:   &ResourceLimits{CPUs: "0.5", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.1", Memory: "64M"}},
				HealthCheck: &HealthCheck{Test: []string{"CMD", "curl", "-f", "http://localhost:3001/"}, Interval: "30s", Timeout: "10s", Retries: 3, StartPeriod: "30s"},
			},
		},
		Volumes: map[string]*VolumeConfig{"uptimekuma_data": {Driver: "local"}},
		CreatedAt: now,
	}
}
