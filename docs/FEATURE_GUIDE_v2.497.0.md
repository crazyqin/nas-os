# NAS-OS v2.497.0 新功能指南

> 版本：v2.497.0
> 发布日期：2026-05-27
> 本文档介绍 v2.497.0 新增功能的配置和使用方法。

---

## 目录

1. [PXE 网络启动](#1-pxe-网络启动)
2. [反向代理管理](#2-反向代理管理)
3. [存储空间分析](#3-存储空间分析)
4. [快照验证测试](#4-快照验证测试)

---

## 1. PXE 网络启动

### 功能简介

PXE（Preboot Execution Environment）网络启动允许客户端计算机通过网络从 NAS-OS 服务器加载操作系统，无需本地存储设备。适用于：

- 批量装机部署
- 无盘工作站
- 网络克隆
- 系统恢复环境

### 配置步骤

#### 1.1 启用 PXE 服务

```bash
# 通过 Web UI
系统设置 → 网络服务 → PXE 启动 → 启用

# 通过 API
curl -X POST http://localhost:8080/api/v1/pxe/enable \
  -H "Authorization: Bearer <token>" \
  -d '{"enabled": true}'
```

#### 1.2 TFTP 服务配置

TFTP（Trivial File Transfer Protocol）用于传输启动文件：

```yaml
# /etc/nas-os/pxe/tftp.yaml
tftp:
  enabled: true
  port: 69
  root_dir: /data/pxe/tftp
  block_size: 1468
  timeout: 5
  retries: 3
```

**目录结构：**
```
/data/pxe/tftp/
├── pxelinux.0          # PXE 引导程序
├── ldlinux.c32         # 引导加载器
├── menu.c32            # 菜单模块
├── pxelinux.cfg/
│   └── default         # 默认启动配置
└── images/
    ├── ubuntu-24.04/
    │   ├── vmlinuz
    │   └── initrd
    └── winpe/
        └── boot.wim
```

#### 1.3 DHCP 配置

如果使用 NAS-OS 内置 DHCP：

```yaml
# /etc/nas-os/pxe/dhcp.yaml
dhcp:
  enabled: true
  interface: eth0
  range_start: 192.168.1.100
  range_end: 192.168.1.200
  subnet_mask: 255.255.255.0
  gateway: 192.168.1.1
  dns_servers:
    - 8.8.8.8
    - 114.114.114.114
  lease_time: 24h
  
  # PXE 特定配置
  pxe:
    next_server: 192.168.1.10  # NAS-OS 服务器 IP
    filename: "pxelinux.0"
```

如果使用外部 DHCP 服务器（如路由器），需要配置 DHCP 选项：

```
Option 66 (TFTP Server): 192.168.1.10
Option 67 (Boot File): pxelinux.0
```

#### 1.4 添加启动镜像

**方法一：Web UI 上传**

1. 导航到 `PXE 管理 → 镜像管理`
2. 点击 `上传镜像`
3. 选择 ISO 文件或指定镜像 URL
4. 系统自动提取并配置

**方法二：命令行**

```bash
# 下载 Ubuntu 24.04 网络安装镜像
nasctl pxe image add \
  --name "Ubuntu 24.04" \
  --url "http://releases.ubuntu.com/24.04/ubuntu-24.04-live-server-amd64.iso" \
  --type "linux"

# 从本地 ISO 添加
nasctl pxe image add \
  --name "WinPE" \
  --path "/data/iso/winpe.iso" \
  --type "windows"
```

**方法三：API**

```bash
curl -X POST http://localhost:8080/api/v1/pxe/images \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ubuntu 24.04",
    "url": "http://releases.ubuntu.com/24.04/ubuntu-24.04-live-server-amd64.iso",
    "type": "linux",
    "description": "Ubuntu 24.04 LTS Server"
  }'
```

#### 1.5 启动菜单配置

编辑 `/data/pxe/tftp/pxelinux.cfg/default`：

```
DEFAULT menu.c32
PROMPT 0
TIMEOUT 300
ONTIMEOUT ubuntu-24.04

MENU TITLE NAS-OS PXE Boot Menu

LABEL ubuntu-24.04
  MENU LABEL Ubuntu 24.04 LTS
  KERNEL images/ubuntu-24.04/vmlinuz
  INITRD images/ubuntu-24.04/initrd
  APPEND ip=dhcp url=http://192.168.1.10/iso/ubuntu-24.04-live-server-amd64.iso

LABEL winpe
  MENU LABEL Windows PE
  KERNEL memdisk
  APPEND iso initrd=images/winpe/boot.wim raw

LABEL local
  MENU LABEL Boot from local disk
  LOCALBOOT 0
```

### 客户端启动

1. 开机按 F12（或指定键）进入启动菜单
2. 选择 "Network Boot" 或 "PXE Boot"
3. 自动获取 IP 并连接到 PXE 服务器
4. 从启动菜单选择要安装的系统
5. 按照提示完成安装

---

## 2. 反向代理管理

### 功能简介

内置反向代理服务器，支持：

- 可视化规则管理
- Let's Encrypt 自动 SSL 证书
- 多域名支持
- 访问日志和统计

### 2.1 创建代理规则

**Web UI 操作：**

1. 导航到 `网络服务 → 反向代理`
2. 点击 `添加代理规则`
3. 填写配置：
   - **域名**：`app.example.com`
   - **目标地址**：`http://localhost:3000`
   - **启用 SSL**：✅
4. 点击保存

**API 操作：**

```bash
curl -X POST http://localhost:8080/api/v1/proxy/rules \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "app.example.com",
    "target": "http://localhost:3000",
    "ssl": {
      "enabled": true,
      "provider": "letsencrypt",
      "auto_renew": true
    },
    "headers": {
      "X-Real-IP": "$remote_addr",
      "X-Forwarded-For": "$proxy_add_x_forwarded_for"
    },
    "websocket": true,
    "rate_limit": {
      "enabled": true,
      "requests_per_second": 100
    }
  }'
```

### 2.2 SSL 证书申请

**自动申请（Let's Encrypt）：**

```bash
# 系统会自动为配置的域名申请证书
# 需要域名已解析到 NAS-OS 服务器 IP

# 查看证书状态
nasctl proxy cert list

# 手动触发证书申请
nasctl proxy cert request --domain app.example.com

# 手动续期
nasctl proxy cert renew --domain app.example.com
```

**手动上传证书：**

```bash
nasctl proxy cert upload \
  --domain app.example.com \
  --cert /path/to/cert.pem \
  --key /path/to/key.pem
```

**证书配置：**

```yaml
# /etc/nas-os/proxy/ssl.yaml
ssl:
  provider: letsencrypt  # letsencrypt | manual
  email: admin@example.com
  auto_renew: true
  renew_days_before: 30
  
  # 可选：使用 DNS 验证（支持通配符证书）
  dns_challenge:
    enabled: true
    provider: cloudflare
    api_token: ${CLOUDFLARE_API_TOKEN}
```

### 2.3 高级配置

**WebSocket 支持：**

代理规则默认支持 WebSocket，无需额外配置。

**负载均衡：**

```json
{
  "domain": "api.example.com",
  "target": [
    {"url": "http://192.168.1.20:8080", "weight": 3},
    {"url": "http://192.168.1.21:8080", "weight": 2},
    {"url": "http://192.168.1.22:8080", "weight": 1}
  ],
  "load_balancer": {
    "method": "weighted_round_robin",
    "health_check": {
      "enabled": true,
      "path": "/health",
      "interval": 30
    }
  }
}
```

**访问控制：**

```json
{
  "domain": "internal.example.com",
  "target": "http://localhost:3000",
  "access_control": {
    "allow": ["192.168.1.0/24", "10.0.0.0/8"],
    "deny": ["192.168.1.100"],
    "basic_auth": {
      "enabled": true,
      "users": [
        {"username": "admin", "password_hash": "..."}
      ]
    }
  }
}
```

**自定义错误页面：**

```json
{
  "error_pages": {
    "404": "/data/proxy/errors/404.html",
    "502": "/data/proxy/errors/502.html",
    "503": "/data/proxy/errors/maintenance.html"
  }
}
```

### 2.4 监控和日志

```bash
# 查看代理统计
nasctl proxy stats

# 实时访问日志
nasctl proxy logs --follow

# 按域名过滤
nasctl proxy logs --domain app.example.com --follow
```

---

## 3. 存储空间分析

### 功能简介

存储空间分析工具提供：

- Treemap 可视化展示
- 大文件快速定位
- 目录空间占用分析
- 历史趋势对比
- 智能清理建议

### 3.1 扫描配置

**Web UI 操作：**

1. 导航到 `存储管理 → 空间分析`
2. 选择要分析的卷/目录
3. 点击 `开始扫描`

**API 操作：**

```bash
# 启动扫描任务
curl -X POST http://localhost:8080/api/v1/storage/analysis/scan \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data",
    "scan_type": "full",
    "options": {
      "follow_symlinks": false,
      "exclude": [
        "/data/.snapshots",
        "/data/tmp"
      ],
      "min_file_size": "1MB"
    },
    "schedule": {
      "enabled": true,
      "cron": "0 3 * * 0"
    }
  }'
```

**命令行：**

```bash
# 手动扫描
nasctl storage analysis scan --path /data

# 增量扫描（只分析变化部分）
nasctl storage analysis scan --path /data --incremental

# 查看扫描进度
nasctl storage analysis status
```

### 3.2 Treemap 查看

扫描完成后，可通过 Treemap 可视化查看空间占用：

**Web UI：**

1. 导航到 `存储管理 → 空间分析 → Treemap 视图`
2. 使用鼠标缩放和平移
3. 点击目录查看详细信息
4. 右键菜单可执行操作（删除、移动、压缩）

**API：**

```bash
# 获取 Treemap 数据
curl http://localhost:8080/api/v1/storage/analysis/treemap?path=/data \
  -H "Authorization: Bearer <token>"

# 返回格式
{
  "name": "data",
  "size": 1073741824000,
  "children": [
    {
      "name": "documents",
      "size": 536870912000,
      "percentage": 50.0,
      "children": [...]
    },
    {
      "name": "media",
      "size": 322122547200,
      "percentage": 30.0,
      "children": [...]
    }
  ]
}
```

### 3.3 大文件查找

```bash
# 查找最大的 100 个文件
nasctl storage analysis large-files --path /data --limit 100

# 按类型过滤
nasctl storage analysis large-files --path /data --type video --limit 50

# 按时间范围
nasctl storage analysis large-files --path /data --older-than 90d --limit 50
```

**API：**

```bash
curl -X GET "http://localhost:8080/api/v1/storage/analysis/large-files?path=/data&limit=100&min_size=100MB" \
  -H "Authorization: Bearer <token>"

# 返回示例
{
  "files": [
    {
      "path": "/data/media/movie.mkv",
      "size": 4294967296,
      "modified": "2026-01-15T10:30:00Z",
      "type": "video"
    },
    ...
  ],
  "total_size": 107374182400,
  "count": 100
}
```

### 3.4 智能清理建议

系统会自动分析并提供清理建议：

```bash
# 获取清理建议
nasctl storage analysis cleanup-suggestions --path /data

# 返回内容
# - 重复文件
# - 临时文件
# - 旧日志文件
# - 缓存文件
# - 大文件未访问超过 X 天
```

**API：**

```bash
curl http://localhost:8080/api/v1/storage/analysis/cleanup?path=/data \
  -H "Authorization: Bearer <token>"

# 返回示例
{
  "suggestions": [
    {
      "type": "duplicate",
      "description": "发现 50 个重复文件",
      "potential_savings": "2.5 GB",
      "files": [...]
    },
    {
      "type": "old_logs",
      "description": "30 天前的日志文件",
      "potential_savings": "500 MB",
      "files": [...]
    }
  ],
  "total_potential_savings": "3 GB"
}
```

### 3.5 历史趋势

```bash
# 查看空间使用趋势
nasctl storage analysis trend --path /data --period 30d

# 导出报告
nasctl storage analysis report --path /data --format pdf --output /tmp/report.pdf
```

---

## 4. 快照验证测试

### 功能简介

快照验证测试确保备份数据真正可用：

- 自动化验证策略
- 定期数据完整性检查
- 自动修复损坏数据
- 验证报告生成
- 告警通知

### 4.1 创建验证策略

**Web UI 操作：**

1. 导航到 `存储管理 → 快照验证`
2. 点击 `创建验证策略`
3. 配置策略参数
4. 保存并启用

**API 操作：**

```bash
curl -X POST http://localhost:8080/api/v1/snapshot/verification/policies \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-verify",
    "description": "每日快照验证",
    "volumes": ["/data/documents", "/data/photos"],
    "schedule": {
      "cron": "0 4 * * *",
      "timezone": "Asia/Shanghai"
    },
    "verification": {
      "type": "full",
      "checksum": "blake2b",
      "test_restore": true,
      "restore_path": "/tmp/verify-test"
    },
    "retention": {
      "keep_results": 30
    },
    "alerts": {
      "on_failure": true,
      "channels": ["email", "webhook"],
      "webhook_url": "https://hooks.example.com/alert"
    }
  }'
```

**命令行：**

```bash
# 创建验证策略
nasctl snapshot verify policy create \
  --name "daily-verify" \
  --volumes "/data/documents,/data/photos" \
  --schedule "0 4 * * *" \
  --test-restore \
  --alert-on-failure

# 列出所有策略
nasctl snapshot verify policy list

# 查看策略详情
nasctl snapshot verify policy show daily-verify
```

### 4.2 手动验证测试

```bash
# 验证特定快照
nasctl snapshot verify run \
  --volume /data/documents \
  --snapshot 2026-05-27-03-00 \
  --type full \
  --checksum blake2b

# 验证最新快照
nasctl snapshot verify run \
  --volume /data/documents \
  --latest

# 验证并测试恢复
nasctl snapshot verify run \
  --volume /data/documents \
  --latest \
  --test-restore \
  --restore-path /tmp/verify-test
```

**API：**

```bash
curl -X POST http://localhost:8080/api/v1/snapshot/verification/run \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "volume": "/data/documents",
    "snapshot": "2026-05-27-03-00",
    "options": {
      "type": "full",
      "checksum": "blake2b",
      "test_restore": true,
      "restore_path": "/tmp/verify-test",
      "compare_metadata": true,
      "verify_permissions": true
    }
  }'
```

### 4.3 查看验证结果

```bash
# 查看验证历史
nasctl snapshot verify history --volume /data/documents

# 查看特定验证结果
nasctl snapshot verify result show <verification-id>

# 导出验证报告
nasctl snapshot verify report \
  --volume /data/documents \
  --format pdf \
  --output /tmp/verification-report.pdf
```

**API：**

```bash
curl http://localhost:8080/api/v1/snapshot/verification/results?volume=/data/documents \
  -H "Authorization: Bearer <token>"

# 返回示例
{
  "results": [
    {
      "id": "ver-20260527-001",
      "snapshot": "2026-05-27-03-00",
      "volume": "/data/documents",
      "status": "passed",
      "started_at": "2026-05-27T04:00:00Z",
      "completed_at": "2026-05-27T04:15:00Z",
      "duration": "15m",
      "checks": {
        "total_files": 15000,
        "verified_files": 15000,
        "corrupted_files": 0,
        "missing_files": 0
      },
      "checksum": {
        "algorithm": "blake2b",
        "match": true
      },
      "restore_test": {
        "status": "success",
        "restore_time": "2m 30s"
      }
    }
  ]
}
```

### 4.4 自动修复

当验证发现数据损坏时，系统可自动修复：

```yaml
# 在验证策略中启用自动修复
auto_repair:
  enabled: true
  strategy: "rebuild_from_mirror"  # rebuild_from_mirror | restore_from_backup | scrub
  
  # 修复前通知
  notify_before_repair: true
  approval_required: false
  
  # 修复后验证
  verify_after_repair: true
```

**手动触发修复：**

```bash
# 修复特定快照
nasctl snapshot verify repair \
  --volume /data/documents \
  --snapshot 2026-05-27-03-00 \
  --strategy rebuild_from_mirror

# 查看修复状态
nasctl snapshot verify repair status <repair-id>
```

### 4.5 告警配置

```yaml
# /etc/nas-os/verification/alerts.yaml
alerts:
  channels:
    email:
      enabled: true
      smtp_host: smtp.example.com
      smtp_port: 587
      username: alert@example.com
      password: ${SMTP_PASSWORD}
      recipients:
        - admin@example.com
    
    webhook:
      enabled: true
      url: https://hooks.example.com/verification
      method: POST
      headers:
        Authorization: Bearer ${WEBHOOK_TOKEN}
    
    slack:
      enabled: true
      webhook_url: https://hooks.slack.com/services/xxx
  
  rules:
    - name: "verification_failed"
      condition: "status == 'failed'"
      severity: "critical"
      channels: ["email", "slack"]
    
    - name: "corruption_detected"
      condition: "corrupted_files > 0"
      severity: "warning"
      channels: ["email"]
    
    - name: "restore_test_failed"
      condition: "restore_test.status == 'failed'"
      severity: "critical"
      channels: ["email", "webhook"]
```

---

## 常见问题

### PXE 启动

**Q: 客户端无法获取 IP 地址？**
A: 检查 DHCP 服务是否运行，确认网络连接正常，尝试手动设置 DHCP 选项 66/67。

**Q: 启动过程中提示 "TFTP timeout"？**
A: 检查 TFTP 服务状态，确认防火墙允许 UDP 69 端口，验证文件路径正确。

### 反向代理

**Q: SSL 证书申请失败？**
A: 确认域名已正确解析到服务器 IP，检查 80 端口是否开放，查看 Let's Encrypt 日志。

**Q: WebSocket 连接失败？**
A: 确认代理规则中启用了 WebSocket 支持，检查目标服务是否支持 WebSocket。

### 存储空间分析

**Q: 扫描速度很慢？**
A: 尝试增量扫描模式，排除不必要的目录，调整 `min_file_size` 过滤小文件。

**Q: Treemap 显示不准确？**
A: 重新执行完整扫描，确认文件系统权限允许读取所有目录。

### 快照验证

**Q: 验证任务超时？**
A: 调整验证策略，减少同时验证的卷数量，或使用增量验证模式。

**Q: 自动修复失败？**
A: 检查存储空间是否充足，确认镜像卷健康状态，尝试手动修复。

---

## API 参考

完整的 API 文档请访问：

- Swagger UI: `http://<your-nas-ip>:8080/swagger/`
- OpenAPI 规范: [docs/api.yaml](api.yaml)

---

## 相关文档

- [竞品分析 2026 Q2](COMPETITIVE_ANALYSIS_2026Q2.md)
- [产品路线图](ROADMAP.md)
- [API 文档](api.yaml)
- [用户指南](user-guide/)
