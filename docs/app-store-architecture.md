# NAS-OS 应用商店系统架构设计

## 1. 概述

NAS-OS 应用商店是一个类似 TrueNAS Apps 的应用生命周期管理系统，基于 Docker 容器技术，提供一键安装、配置、管理和更新功能。

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         NAS-OS 应用商店系统                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │   Web UI     │───▶│  API Layer   │───▶│  Core Engine │              │
│  │  (Frontend)  │    │  (REST API)  │    │  (Backend)   │              │
│  └──────────────┘    └──────────────┘    └──────────────┘              │
│         │                  │                    │                        │
│         │                  │                    │                        │
│         ▼                  ▼                    ▼                        │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │   Pages      │    │   Handlers   │    │   Managers   │              │
│  │  - Catalog   │    │  - Install   │    │  - AppStore  │              │
│  │  - Installed │    │  - Uninstall │    │  - Installer │              │
│  │  - Details   │    │  - Start     │    │  - Repository│              │
│  │              │    │  - Stop      │    │  - Catalog   │              │
│  └──────────────┘    └──────────────┘    └──────────────┘              │
│         │                  │                    │                        │
│         └──────────────────┼────────────────────┘                        │
│                            ▼                                             │
│                    ┌──────────────┐                                     │
│                    │  Docker API  │                                     │
│                    │  - Compose   │                                     │
│                    │  - Container │                                     │
│                    │  - Network   │                                     │
│                    └──────────────┘                                     │
│                            │                                             │
│                            ▼                                             │
│                    ┌──────────────┐                                     │
│                    │ Data Store   │                                     │
│                    │  - JSON      │                                     │
│                    │  - Templates │                                     │
│                    └──────────────┘                                     │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 核心模块划分

| 模块 | 路径 | 功能 |
|------|------|------|
| **应用模板管理** | `internal/apps/catalog.go` | 应用目录、模板定义、版本管理 |
| **应用生命周期** | `internal/apps/manager.go` | 安装/卸载/启动/停止/重启 |
| **应用安装器** | `internal/apps/installer.go` | Docker Compose生成、配置注入 |
| **应用仓库** | `internal/apps/repository.go` | 远程仓库同步、本地模板存储 |
| **API处理器** | `internal/docker/app_handlers.go` | REST API端点实现 |
| **Web界面** | `webui/pages/apps.html` | 用户交互界面 |

## 3. 数据模型

### 3.1 应用模板 (AppTemplate)

```go
type AppTemplate struct {
    // 基础信息
    ID              string            `json:"id"`              // 唯一标识
    Name            string            `json:"name"`            // 应用名称
    DisplayName     string            `json:"display_name"`    // 显示名称
    Icon            string            `json:"icon"`            // 图标URL
    Description     string            `json:"description"`     // 描述
    LongDescription string            `json:"long_description"`// 详细描述
    Category        string            `json:"category"`        // 分类
    Tags            []string          `json:"tags"`            // 标签
    Version         string            `json:"version"`         // 当前版本
    Website         string            `json:"website"`         // 官网
    SourceURL       string            `json:"source_url"`      // 源码地址
    
    // 容器配置
    Image           string            `json:"image"`           // Docker镜像
    Ports           []PortMapping     `json:"ports"`           // 端口映射
    Volumes         []VolumeMapping   `json:"volumes"`         // 卷挂载
    Environment     map[string]string `json:"environment"`     // 环境变量
    Networks        []string          `json:"networks"`        // 网络
    DependsOn       []string          `json:"depends_on"`      // 依赖应用
    
    // 资源限制
    CPULimit        string            `json:"cpu_limit"`       // CPU限制
    MemoryLimit     string            `json:"memory_limit"`    // 内存限制
    Privileged      bool              `json:"privileged"`      // 特权模式
    
    // 用户配置项
    ConfigSchema    []ConfigField     `json:"config_schema"`   // 配置表单定义
    
    // 元数据
    Maintainer      string            `json:"maintainer"`      // 维护者
    License         string            `json:"license"`         // 许可证
    Rating          float64           `json:"rating"`          // 评分
    Downloads       int64             `json:"downloads"`       // 下载次数
    CreatedAt       time.Time         `json:"created_at"`      // 创建时间
    UpdatedAt       time.Time         `json:"updated_at"`      // 更新时间
}
```

### 3.2 已安装应用 (InstalledApp)

```go
type InstalledApp struct {
    // 基础信息
    ID              string            `json:"id"`              // 安装实例ID
    TemplateID      string            `json:"template_id"`     // 模板ID
    Name            string            `json:"name"`            // 实例名称（用户自定义）
    InstallPath     string            `json:"install_path"`    // 安装路径
    
    // 状态
    Status          AppStatus         `json:"status"`          // 运行状态
    ContainerID     string            `json:"container_id"`    // 容器ID
    StartedAt       time.Time         `json:"started_at"`      // 启动时间
    
    // 用户配置
    Config          map[string]string `json:"config"`          // 用户配置值
    CustomPorts     []PortMapping     `json:"custom_ports"`    // 自定义端口
    CustomVolumes   []VolumeMapping   `json:"custom_volumes"`  // 自定义卷
    
    // 资源使用
    CPUUsage        float64           `json:"cpu_usage"`       // CPU使用率
    MemoryUsage     int64             `json:"memory_usage"`    // 内存使用
    NetworkRx       int64             `json:"network_rx"`      // 网络接收
    NetworkTx       int64             `json:"network_tx"`      // 网络发送
    
    // 元数据
    InstalledAt     time.Time         `json:"installed_at"`    // 安装时间
    UpdatedAt       time.Time         `json:"updated_at"`      // 更新时间
    Version         string            `json:"version"`         // 安装版本
}
```

### 3.3 应用状态

```go
type AppStatus string

const (
    AppStatusNotInstalled AppStatus = "not_installed"
    AppStatusInstalling   AppStatus = "installing"
    AppStatusRunning      AppStatus = "running"
    AppStatusStopped      AppStatus = "stopped"
    AppStatusStarting     AppStatus = "starting"
    AppStatusStopping     AppStatus = "stopping"
    AppStatusError        AppStatus = "error"
    AppStatusUpdating     AppStatus = "updating"
)
```

### 3.4 配置字段定义

```go
type ConfigField struct {
    Name        string      `json:"name"`         // 字段名
    Label       string      `json:"label"`        // 显示标签
    Type        FieldType   `json:"type"`         // 字段类型
    Required    bool        `json:"required"`     // 是否必填
    Default     string      `json:"default"`      // 默认值
    Description string      `json:"description"`  // 描述
    Options     []string    `json:"options"`      // 选项列表（用于select）
    Min         int         `json:"min"`          // 最小值（用于number）
    Max         int         `json:"max"`          // 最大值（用于number）
    Validation  string      `json:"validation"`   // 验证规则
}

type FieldType string

const (
    FieldTypeText     FieldType = "text"
    FieldTypeNumber   FieldType = "number"
    FieldTypePassword FieldType = "password"
    FieldTypeSelect   FieldType = "select"
    FieldTypeBoolean  FieldType = "boolean"
    FieldTypePath     FieldType = "path"
    FieldTypePort     FieldType = "port"
)
```

## 4. API 设计

### 4.1 REST API 端点

#### 应用目录 API

| 端点 | 方法 | 功能 | 参数 |
|------|------|------|------|
| `/api/v1/apps/catalog` | GET | 获取应用目录 | `?category=&search=&limit=` |
| `/api/v1/apps/catalog/:id` | GET | 获取模板详情 | - |
| `/api/v1/apps/categories` | GET | 获取分类列表 | - |
| `/api/v1/apps/search` | GET | 搜索应用 | `?q=` |

#### 已安装应用 API

| 端点 | 方法 | 功能 | 参数 |
|------|------|------|------|
| `/api/v1/apps/installed` | GET | 列出已安装应用 | - |
| `/api/v1/apps/installed/:id` | GET | 获取应用详情 | - |
| `/api/v1/apps/installed/:id/status` | GET | 获取运行状态 | - |
| `/api/v1/apps/installed/:id/stats` | GET | 获取资源统计 | - |
| `/api/v1/apps/installed/:id/logs` | GET | 获取应用日志 | `?lines=&follow=` |

#### 生命周期 API

| 端点 | 方法 | 功能 | 请求体 |
|------|------|------|------|
| `/api/v1/apps/install/:template_id` | POST | 安装应用 | `{name, config, ports, volumes}` |
| `/api/v1/apps/installed/:id/start` | POST | 启动应用 | - |
| `/api/v1/apps/installed/:id/stop` | POST | 停止应用 | - |
| `/api/v1/apps/installed/:id/restart` | POST | 重启应用 | - |
| `/api/v1/apps/installed/:id` | DELETE | 卸载应用 | `{keep_data: bool}` |
| `/api/v1/apps/installed/:id/update` | POST | 更新应用 | `{version}` |
| `/api/v1/apps/installed/:id/config` | PUT | 更新配置 | `{config}` |

### 4.2 API 请求/响应示例

#### 安装应用请求

```json
{
  "name": "my-nextcloud",
  "config": {
    "admin_user": "admin",
    "admin_password": "secret123",
    "db_type": "mysql"
  },
  "ports": [
    {"container_port": 80, "host_port": 8080}
  ],
  "volumes": [
    {"container_path": "/data", "host_path": "/mnt/storage/nextcloud"}
  ]
}
```

#### 安装应用响应

```json
{
  "id": "app-nextcloud-001",
  "template_id": "nextcloud",
  "name": "my-nextcloud",
  "status": "installing",
  "install_path": "/opt/nas/apps/my-nextcloud",
  "installed_at": "2024-01-15T10:30:00Z",
  "message": "应用正在安装中，请稍候..."
}
```

## 5. Docker Compose 生成

### 5.1 模板生成流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ AppTemplate │───▶│ ConfigMerge │───▶│ComposeGen   │───▶│ DockerFile  │
│   (基础)    │    │  (配置合并) │    │  (YAML生成) │    │  (写入)     │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
      │                   │                   │                  │
      ▼                   ▼                   ▼                  ▼
 ┌─────────┐       ┌─────────┐        ┌──────────────┐    ┌──────────┐
 │ Ports   │       │ UserCfg │        │ compose.yaml │    │ /opt/nas │
 │ Volumes │       │ Ports   │        │              │    │ /apps/   │
 │ EnvVars │       │ Volumes │        │              │    │ <name>/  │
 └─────────┘       └─────────┘        └──────────────┘    └──────────┘
```

### 5.2 Compose 模板示例

```yaml
# /opt/nas/apps/<app-name>/compose.yaml
version: "3.8"
services:
  <app-name>:
    image: <image>:<version>
    container_name: nas-<app-name>
    restart: unless-stopped
    ports:
      - "<host_port>:<container_port>"
    volumes:
      - "<host_path>:<container_path>"
    environment:
      - KEY=<value>
    networks:
      - nas-network
    labels:
      - "nas-os.managed=true"
      - "nas-os.template=<template_id>"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80"]
      interval: 30s
      timeout: 10s
      retries: 3

networks:
  nas-network:
    external: true
```

### 5.3 安装目录结构

```
/opt/nas/apps/<app-name>/
├── compose.yaml          # Docker Compose 配置
├── config.json           # 用户配置记录
├── .env                  # 环境变量文件
├── data/                 # 应用数据（可选保留）
├── logs/                 # 应用日志
└── backup/               # 备份目录
```

## 6. 仓库系统

### 6.1 仓库架构

```
┌──────────────────────────────────────────────────────────────────┐
│                        应用仓库系统                                │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐ │
│  │ Official    │         │ Community   │         │ Local       │ │
│  │ Repository  │         │ Repository  │         │ Repository  │ │
│  │ (官方仓库)  │         │ (社区仓库)  │         │ (本地仓库)  │ │
│  └─────────────┘         └─────────────┘         └─────────────┘ │
│        │                       │                       │         │
│        │                       │                       │         │
│        ▼                       ▼                       ▼         │
│  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐ │
│  │ HTTPS       │         │ GitHub      │         │ /opt/nas/   │ │
│  │ api.nas-os  │         │ Raw URL     │         │ templates/  │ │
│  │ .com/repo   │         │             │         │             │ │
│  └─────────────┘         └─────────────┘         └─────────────┘ │
│                                                                   │
│        └──────────────────────┬──────────────────────┘           │
│                               ▼                                   │
│                    ┌─────────────────────┐                       │
│                    │   Repository Sync   │                       │
│                    │   (仓库同步器)      │                       │
│                    │   - 定时拉取        │                       │
│                    │   - 版本检查        │                       │
│                    │   - 缓存管理        │                       │
│                    └─────────────────────┘                       │
│                               │                                   │
│                               ▼                                   │
│                    ┌─────────────────────┐                       │
│                    │   Local Catalog     │                       │
│                    │   (本地目录缓存)    │                       │
│                    └─────────────────────┘                       │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 6.2 仓库配置

```go
type RepositoryConfig struct {
    URL         string        `json:"url"`          // 仓库URL
    Name        string        `json:"name"`         // 仓库名称
    Type        RepoType      `json:"type"`         // 仓库类型
    Enabled     bool          `json:"enabled"`      // 是否启用
    Priority    int           `json:"priority"`     // 优先级
    SyncInterval time.Duration `json:"sync_interval"` // 同步间隔
    Auth        *RepoAuth     `json:"auth"`         // 认证信息
}

type RepoType string

const (
    RepoTypeOfficial   RepoType = "official"    // 官方仓库
    RepoTypeCommunity  RepoType = "community"   // 社区仓库
    RepoTypeLocal      RepoType = "local"       // 本地仓库
    RepoTypeCustom     RepoType = "custom"      // 自定义仓库
)
```

### 6.3 仓库同步流程

1. **定时同步**: 每6小时检查远程仓库更新
2. **增量同步**: 只下载新增/更新的模板
3. **签名验证**: 验证模板签名（官方仓库）
4. **缓存清理**: 清理过期模板缓存

## 7. 安全与权限

### 7.1 权限控制

```
┌────────────────────────────────────────────────────────────┐
│                      应用权限模型                           │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Role: admin                                               │
│  └── apps:*:*              // 完全权限                     │
│                                                            │
│  Role: operator                                            │
│  └── apps:install:*        // 安装权限                     │
│  └── apps:start:*          // 启动/停止权限                 │
│  └── apps:config:*         // 配置权限                     │
│                                                            │
│  Role: viewer                                              │
│  └── apps:list:*           // 查看权限                     │
│  └── apps:status:*         // 状态查询权限                 │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 7.2 安全检查

- **镜像验证**: 只允许来自可信源的镜像
- **端口冲突检查**: 阿止端口占用
- **路径隔离**: 应用数据隔离存储
- **资源限制**: 强制设置资源上限
- **网络隔离**: 默认使用独立网络

## 8. 更新机制

### 8.1 更新流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                        应用更新流程                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. 版本检查                                                         │
│     ├── 检查远程仓库最新版本                                          │
│     ├── 对比本地安装版本                                              │
│     └── 记录可更新应用                                                │
│                                                                      │
│  2. 更新通知                                                         │
│     ├── Web界面提示                                                  │
│     ├── 邮件/通知推送（可选）                                          │
│     └── 记录更新日志                                                  │
│                                                                      │
│  3. 用户确认                                                         │
│     ├── 选择更新版本                                                  │
│     ├── 确认数据备份                                                  │
│     └── 授权更新操作                                                  │
│                                                                      │
│  4. 执行更新                                                         │
│     ├── 拉取新镜像                                                   │
│     ├── 停止旧容器                                                   │
│     ├── 启动新容器                                                   │
│     └── 验证健康状态                                                  │
│                                                                      │
│  5. 回滚机制                                                         │
│     ├── 失败自动回滚                                                 │
│     ├── 保留旧镜像备份                                                │
│     └── 记录更新历史                                                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 8.2 更新策略

```go
type UpdatePolicy struct {
    AutoUpdate      bool          `json:"auto_update"`      // 自动更新
    UpdateChannel   UpdateChannel `json:"update_channel"`   // 更新通道
    BackupBefore    bool          `json:"backup_before"`    // 更新前备份
    RollbackOnFail  bool          `json:"rollback_on_fail"` // 失败回滚
    CheckInterval   time.Duration `json:"check_interval"`   // 检查间隔
}

type UpdateChannel string

const (
    UpdateChannelStable    UpdateChannel = "stable"     // 稳定版
    UpdateChannelBeta      UpdateChannel = "beta"       // 测试版
    UpdateChannelNightly   UpdateChannel = "nightly"    // 每日构建
)
```

## 9. 监控与日志

### 9.1 应用监控

- **CPU使用率**: 实时监控容器CPU
- **内存使用**: 内存占用统计
- **网络流量**: 入站/出站流量
- **磁盘IO**: 读写统计
- **健康检查**: 定期健康探测

### 9.2 日志管理

```
/opt/nas/apps/<app-name>/logs/
├── stdout.log        # 标准输出日志
├── stderr.log        # 错误日志
├── events.log        # 事件日志
└── history.log       # 操作历史
```

## 10. 预置应用分类

| 分类 | 应用示例 |
|------|----------|
| **生产力** | Nextcloud, Syncthing, OwnCloud |
| **媒体** | Jellyfin, Plex, Emby, Immich |
| **智能家居** | Home Assistant, openHAB |
| **网络** | Pi-hole, Nginx Proxy Manager, AdGuard |
| **下载** | Transmission, qBittorrent, aria2 |
| **开发** | Gitea, GitLab, Jenkins |
| **安全** | Vaultwarden, Keycloak |
| **数据库** | PostgreSQL, MySQL, MongoDB, Redis |
| **监控** | Grafana, Prometheus, Uptime Kuma |

## 11. 技术选型

| 层级 | 技术 | 说明 |
|------|------|------|
| **前端** | HTML/CSS/JS | 无框架，轻量实现 |
| **API** | Go + Gin | 高性能REST框架 |
| **容器** | Docker + Compose | 容器编排 |
| **存储** | JSON文件 | 简单持久化 |
| **网络** | Docker Network | 容器网络隔离 |

## 12. 未来扩展

### 12.1 短期计划

1. **应用备份/恢复**: 数据备份到指定存储
2. **自定义模板导入**: 用户上传compose.yaml
3. **应用配置编辑**: 运行时配置修改
4. **应用日志查看**: 实时日志流

### 12.2 中期计划

1. **多仓库支持**: 添加第三方仓库
2. **应用依赖管理**: 自动安装依赖应用
3. **应用模板开发工具**: 模板创建辅助
4. **应用评分系统**: 用户评价机制

### 12.3 长期计划

1. **Kubernetes支持**: 迁移到K8s编排
2. **应用市场商业化**: 付费应用支持
3. **多云部署**: 支持云端应用部署
4. **应用安全扫描**: 自动安全检查

## 13. 文件结构

```
nas-os/
├── internal/
│   ├── apps/
│   │   ├── catalog.go           # 应用目录管理
│   │   ├── catalog_test.go      # 目录测试
│   │   ├── manager.go           # 生命周期管理
│   │   ├── installer.go         # 安装器
│   │   ├── repository.go        # 仓库管理
│   │   ├── service.go           # 服务接口
│   │   └── service_test.go      # 服务测试
│   ├── docker/
│   │   ├── appstore.go          # 应用商店核心
│   │   ├── app_handlers.go      # API处理器
│   │   └── manager.go           # Docker管理
│   └── web/
│       └── server.go            # Web服务器路由
├── webui/
│   └── pages/
│       └── apps.html            # 应用商店UI
├── docs/
│   ├── app-store-architecture.md # 本文档
│   └── app-store.md             # 功能说明
└── configs/
    └── app-templates/           # 内置模板
        ├── nextcloud.json
        ├── jellyfin.json
        └── ...
```

## 14. 参考资料

- [TrueNAS Apps Architecture](https://www.truenas.com/docs/core/coreapps/)
- [Docker Compose Specification](https://github.com/compose-spec/compose-spec)
- [Portainer Templates](https://portainer.io/templates)
- [Helm Chart Design](https://helm.sh/docs/topics/charts/)