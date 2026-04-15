# Cloud Drive Sync - 管理员配置指南

> 文档版本：v1.0 | 面向系统管理员和运维人员

---

## 目录

1. [架构概述](#1-架构概述)
2. [系统要求](#2-系统要求)
3. [安装与配置](#3-安装与配置)
4. [安全配置](#4-安全配置)
5. [性能调优](#5-性能调优)
6. [监控与日志](#6-监控与日志)
7. [备份与恢复](#7-备份与恢复)
8. [故障排查](#8-故障排查)
9. [API 参考](#9-api-参考)

---

## 1. 架构概述

### 1.1 组件架构

```
┌─────────────────────────────────────────────────────────────┐
│                     NAS-OS Cloud Drive Sync                    │
├──────────────┬──────────────────┬──────────────────────────┤
│   Web UI     │     nas-cli       │      HTTP API             │
│  (Gin/HTML)  │  (Cobra CLI)      │   (Gin REST)             │
├──────────────┴────────┬──────────┴──────────────────────────┤
│                     Manager (cloud sync)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Provider    │  │   Scheduler  │  │  SyncEngine      │  │
│  │  Registry    │  │  (Cron/Int.) │  │  (Delta Sync)     │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                   Provider Implementations                    │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐    │
│  │  S3    │ │WebDAV  │ │Google  │ │OneDrive│ │中国网盘 │    │
│  │(AWS/Ali│ │        │ │ Drive  │ │        │ │115/百度 │    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘    │
├─────────────────────────────────────────────────────────────┤
│               OAuth2 / Token Management                      │
├─────────────────────────────────────────────────────────────┤
│               Realtime Sync (inotify/fanotify)              │
├─────────────────────────────────────────────────────────────┤
│               Resumable Upload (断点续传)                    │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 数据流

```
本地文件系统 ──→ SyncEngine ──→ Provider ──→ 云存储
                       │
                       ├── Delta计算（比较本地索引 vs 远程索引）
                       ├── 冲突检测（双向同步时）
                       └── 断点记录（大文件分片上传）
```

---

## 2. 系统要求

### 2.1 最低要求

| 组件 | 要求 |
|------|------|
| CPU | 2 核心 |
| 内存 | 512 MB |
| 磁盘 | 50 MB（程序本身）|
| 网络 | 100 Mbps |

### 2.2 推荐配置

| 场景 | 内存 | 说明 |
|------|------|------|
| 个人使用（1-3个任务）| 1 GB | 日常照片/文档备份 |
| 家庭使用（5-10个任务）| 2 GB | 多个设备的混合同步 |
| 小型办公（10+任务）| 4 GB | 并行同步，建议配合 SSD |

### 2.3 依赖

- **Go 1.21+**（编译依赖）
- **inotify-tools**（Linux 实时同步，`apt install inotify-tools`）
- **OpenSSL**（HTTPS/加密传输）

---

## 3. 安装与配置

### 3.1 配置文件位置

```
/etc/nas-os/cloudsync.yaml
```

### 3.2 配置示例

```yaml
# Cloud Drive Sync 配置
cloudsync:
  # 配置存储路径
  config_path: /var/lib/nas-os/cloudsync/config.json
  
  # 数据目录（版本历史、断点记录）
  data_dir: /var/lib/nas-os/cloudsync/data
  
  # 最大并发同步任务数
  max_concurrent_tasks: 5
  
  # 默认超时（秒）
  default_timeout: 300
  
  # 最大重试次数
  max_retries: 3
  
  # 调度器
  scheduler:
    enabled: true
    max_cron_tasks: 20
  
  # 实时同步
  realtime:
    enabled: true
    debounce_ms: 1000  # 防抖延迟（毫秒）
    max_watch_paths: 10
  
  # 断点续传
  resumable:
    enabled: true
    chunk_size: 5242880  # 5MB 分片大小
    record_ttl: 604800   # 记录保留时间（7天）
  
  # 带宽限制
  default_bandwidth_limit: 0  # 0 表示不限制
  
  # 日志
  log:
    level: info           # debug/info/warn/error
    file: /var/log/nas-os/cloudsync.log
    max_size: 100         # MB
    max_backups: 5
  
  # 安全
  security:
    # 敏感字段加密密钥（请使用随机字符串，建议 32 字节）
    encryption_key: "CHANGE_ME_TO_RANDOM_STRING"
    # 禁止在日志中输出文件路径
    redact_paths: true
```

### 3.3 首次启动

```bash
# 启动服务
systemctl start nas-os-cloudsync

# 设置开机自启
systemctl enable nas-os-cloudsync

# 查看状态
systemctl status nas-os-cloudsync
```

---

## 4. 安全配置

### 4.1 密钥管理

**CRITICAL**：所有云存储的 Access Key / Secret Key 必须加密存储。

```bash
# 生成随机加密密钥（32字节 hex）
openssl rand -hex 32
```

在配置文件中设置 `encryption_key`，系统会自动对敏感字段进行 AES-256-GCM 加密。

> ⚠️ **警告**：更换 `encryption_key` 后，已存储的密钥将无法解密。请先导出旧数据再更换。

### 4.2 TLS 配置

WebDAV 和 OAuth2 回调必须使用 HTTPS：

```yaml
server:
  # 强制 HTTPS
  force_https: true
  # 证书路径
  cert_file: /etc/nas-os/tls/server.crt
  key_file: /etc/nas-os/tls/server.key
```

### 4.3 访问控制

通过 NAS-OS 的 RBAC 系统控制用户权限：

| 权限 | 说明 |
|------|------|
| `cloudsync:read` | 查看任务和状态 |
| `cloudsync:write` | 创建/修改任务 |
| `cloudsync:execute` | 触发/暂停同步 |
| `cloudsync:admin` | 管理提供商配置 |

### 4.4 网络隔离

生产环境建议：

```yaml
# 仅允许内网访问 API
bind_address: 127.0.0.1

# 通过反向代理（如 Nginx）暴露 API
# Nginx 配置示例：
# location /api/v1/cloudsync {
#     proxy_pass http://127.0.0.1:8080/api/v1/cloudsync;
#     # 添加认证 header
# }
```

### 4.5 敏感信息过滤

日志默认会对以下内容进行脱敏：

- `accessKey`、`secretKey`、`accessToken`、`refreshToken`
- `Authorization` header
- 文件路径（可选）

```yaml
security:
  redact_paths: true
  redact_tokens: true
```

---

## 5. 性能调优

### 5.1 并发控制

```yaml
cloudsync:
  # 单个任务的并发文件数
  files_per_task: 10
  
  # 全局最大并发任务数
  max_concurrent_tasks: 5
  
  # 大文件阈值（MB）
  large_file_threshold: 100
```

### 5.2 分片上传

对于大文件（如视频、虚拟机镜像），系统自动使用分片上传：

```yaml
resumable:
  # 分片大小（字节），默认 5MB
  chunk_size: 5242880
  
  # 同时上传的分片数
  parallel_chunks: 4
```

### 5.3 缓存策略

```yaml
cache:
  # 远程文件索引缓存（内存）
  index_cache_size: 1000
  
  # 哈希计算缓存
  hash_cache_enabled: true
  hash_cache_ttl: 3600  # 秒
```

### 5.4 Delta Sync（增量同步）

同步引擎通过以下方式减少传输量：

1. **时间戳比较**：修改时间相同则跳过
2. **文件大小比较**：大小相同则跳过
3. **哈希校验**（可选）：计算 SHA-256 确认内容一致

建议开启 `checksumVerify` 以太字节级别的准确性：

```yaml
cloudsync:
  default_checksum_verify: false  # 关闭可提升性能
```

### 5.5 性能瓶颈排查

```bash
# 查看当前同步性能
nas-cli sync status <task-id> --verbose

# 观察 goroutine 数量
curl http://localhost:8080/api/v1/cloudsync/debug/pprof/goroutine?debug=1
```

---

## 6. 监控与日志

### 6.1 Prometheus 指标

Cloud Drive Sync 暴露以下 Prometheus 指标：

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `nas_cloudsync_tasks_total` | Gauge | 任务总数/运行中/已完成 |
| `nas_cloudsync_files_synced_total` | Counter | 累计同步文件数 |
| `nas_cloudsync_bytes_transferred_total` | Counter | 累计传输字节数 |
| `nas_cloudsync_task_duration_seconds` | Histogram | 任务执行耗时 |
| `nas_cloudsync_errors_total` | Counter | 错误总数（按类型标签）|
| `nas_cloudsync_bandwidth_bytes_per_second` | Gauge | 当前带宽使用 |
| `nas_cloudsync_realtime_watches` | Gauge | 实时监控路径数 |

访问：`GET /metrics`

### 6.2 日志配置

```yaml
log:
  level: info
  format: json          # JSON 格式便于日志收集
  file: /var/log/nas-os/cloudsync.log
  max_size: 100         # MB
  max_backups: 5
  
  # Syslog（可选）
  syslog:
    enabled: false
    facility: local0
    tag: nas-os-cloudsync
```

### 6.3 日志级别说明

| 级别 | 适用场景 |
|------|---------|
| `debug` | 调试问题，排查 Bug |
| `info` | 正常运行信息（**默认**）|
| `warn` | 警告信息（如 token 即将过期）|
| `error` | 错误信息（需要关注）|

### 6.4 日志分析

```bash
# 统计错误类型
grep "level=error" /var/log/nas-os/cloudsync.log | \
  jq '.fields.error_type' | sort | uniq -c

# 查看特定任务日志
grep "task_id=task_abc123" /var/log/nas-os/cloudsync.log

# 分析同步性能
grep "sync completed" /var/log/nas-os/cloudsync.log | \
  jq '.fields.duration, .fields.files_count'
```

---

## 7. 备份与恢复

### 7.1 配置备份

Cloud Drive Sync 的配置存储在 `config_path` 指向的 JSON 文件中：

```bash
# 备份配置
cp /var/lib/nas-os/cloudsync/config.json /backup/cloudsync-config.json

# 备份完整数据目录
tar -czf /backup/cloudsync-data-$(date +%Y%m%d).tar.gz \
  /var/lib/nas-os/cloudsync/data/
```

### 7.2 恢复配置

```bash
# 停止服务
systemctl stop nas-os-cloudsync

# 恢复配置
cp /backup/cloudsync-config.json /var/lib/nas-os/cloudsync/config.json

# 重新启动
systemctl start nas-os-cloudsync
```

### 7.3 迁移到新系统

1. 在新系统安装 NAS-OS
2. 复制 `config.json` 和 `data/` 目录
3. 重新启动服务
4. 验证任务状态

> ⚠️ **注意**：Access Token 已在旧系统中加密，换机器后需要重新授权 OAuth2 提供商。

---

## 8. 故障排查

### 8.1 常见错误及处理

| 错误信息 | 原因 | 处理方式 |
|---------|------|---------|
| `connection refused` | 服务未启动 | `systemctl start nas-os-cloudsync` |
| `provider not found` | 提供商 ID 不存在 | 检查 `nas-cli provider list` |
| `access token expired` | OAuth2 token 过期 | 重新授权提供商 |
| `insufficient quota` | 云存储配额不足 | 清理云端空间或升级套餐 |
| `file not found` | 远程文件不存在 | 检查路径是否正确 |
| `upload timeout` | 网络超时 | 增加 `timeout` 配置 |

### 8.2 调试模式

```bash
# 以 debug 级别重启服务
SYSTEM_LOG_LEVEL=debug systemctl restart nas-os-cloudsync

# 观察实时日志
journalctl -u nas-os-cloudsync -f

# CLI 测试连接
nas-cli provider test <provider-id>
```

### 8.3 服务无法启动

```bash
# 检查配置文件语法
python3 -c "import json; json.load(open('/var/lib/nas-os/cloudsync/config.json'))"

# 检查端口占用
ss -tlnp | grep 8080

# 查看启动错误
journalctl -u nas-os-cloudsync -n 50
```

### 8.4 同步卡住

```bash
# 强制取消卡住的任务
nas-cli sync cancel <task-id>

# 检查是否有死锁（goroutine 泄露）
curl http://localhost:8080/api/v1/cloudsync/debug/pprof/goroutine?debug=1

# 重启服务
systemctl restart nas-os-cloudsync
```

---

## 9. API 参考

### 9.1 端点概览

```
POST   /api/v1/cloudsync/providers              # 创建提供商
GET    /api/v1/cloudsync/providers              # 列出提供商
GET    /api/v1/cloudsync/providers/:id          # 获取提供商详情
PUT    /api/v1/cloudsync/providers/:id         # 更新提供商
DELETE /api/v1/cloudsync/providers/:id         # 删除提供商
POST   /api/v1/cloudsync/providers/:id/test    # 测试连接

POST   /api/v1/cloudsync/tasks                  # 创建同步任务
GET    /api/v1/cloudsync/tasks                  # 列出同步任务
GET    /api/v1/cloudsync/tasks/:id             # 获取任务详情
PUT    /api/v1/cloudsync/tasks/:id             # 更新任务
DELETE /api/v1/cloudsync/tasks/:id             # 删除任务

POST   /api/v1/cloudsync/tasks/:id/run         # 触发同步
POST   /api/v1/cloudsync/tasks/:id/pause       # 暂停
POST   /api/v1/cloudsync/tasks/:id/resume       # 恢复
POST   /api/v1/cloudsync/tasks/:id/cancel      # 取消
GET    /api/v1/cloudsync/tasks/:id/status      # 获取状态

GET    /api/v1/cloudsync/statuses              # 所有任务状态
GET    /api/v1/cloudsync/stats                 # 统计信息

# OAuth2
GET    /api/v1/cloudsync/oauth2/auth-url/:providerType  # 获取授权 URL
POST   /api/v1/cloudsync/oauth2/callback       # OAuth2 回调

# 实时同步
GET    /api/v1/cloudsync/realtime/status       # 实时同步状态
POST   /api/v1/cloudsync/realtime/start       # 启动
POST   /api/v1/cloudsync/realtime/stop        # 停止

# 断点续传
GET    /api/v1/cloudsync/resumable/status      # 续传状态
GET    /api/v1/cloudsync/resumable/pending    # 待恢复上传
POST   /api/v1/cloudsync/resumable/resume/:fileId  # 恢复上传
```

### 9.2 请求/响应格式

**标准成功响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

**标准错误响应：**
```json
{
  "code": 500,
  "message": "错误描述"
}
```

### 9.3 API 认证

API 通过 NAS-OS 的统一认证中间件：

- **Session Cookie**：Web UI 使用
- **Bearer Token**：CLI 和外部集成使用

获取 Token：
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"..."}'
```

使用 Token：
```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/cloudsync/tasks
```

---

## 附录：完整配置字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `config_path` | string | — | 配置文件路径 |
| `data_dir` | string | — | 数据目录 |
| `max_concurrent_tasks` | int | 5 | 最大并发任务数 |
| `default_timeout` | int | 300 | 默认超时（秒）|
| `max_retries` | int | 3 | 最大重试次数 |
| `default_bandwidth_limit` | int64 | 0 | 默认带宽限制（KB/s）|
| `log.level` | string | info | 日志级别 |
| `log.file` | string | — | 日志文件路径 |
| `security.encryption_key` | string | — | 加密密钥 |
| `security.redact_paths` | bool | true | 日志路径脱敏 |
| `security.redact_tokens` | bool | true | 日志令牌脱敏 |
