# 远程桌面网关 用户指南

> **版本**: v2.482.0+ | **适用版本**: NAS-OS v2.482.0 及以上

## 概述

NAS-OS 内置远程桌面网关，通过浏览器即可远程访问和控制局域网内的 Windows/Linux/macOS 主机。无需安装客户端、无需 VPN，直接在 Web 界面操作远程桌面。

## 核心特性

- **浏览器 RDP/VNC**：无需安装客户端，浏览器直接操作远程桌面
- **剪贴板同步**：本地与远程之间双向复制粘贴
- **文件传输**：拖拽上传/下载文件到远程主机
- **多显示器支持**：切换和查看远程主机的多个显示器
- **会话录制**：录制远程操作过程，用于审计或回放
- **连接管理**：保存和管理多个远程主机连接
- **权限控制**：按用户授权远程访问权限

## 支持的协议

| 协议 | 适用系统 | 说明 |
|------|----------|------|
| RDP | Windows | 远程桌面协议，Windows 原生支持 |
| VNC | Linux/macOS/Windows | 跨平台，适合 Linux 桌面 |
| SSH | Linux/macOS | 终端模式，适合命令行操作 |

## 配置步骤

### 1. 启用远程桌面网关

进入 **服务 → 远程桌面** 页面，启用网关服务：

```
网关端口：8443（HTTPS）
会话超时：30 分钟（可自定义）
录制存储：/data/recordings（可自定义）
```

### 2. 添加远程主机

```bash
# 通过 Web 界面添加
# 服务 → 远程桌面 → 添加主机

# 或通过 API
curl -X POST http://localhost:8080/api/v1/rdp/hosts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Windows工作站",
    "host": "192.168.1.100",
    "port": 3389,
    "protocol": "rdp",
    "username": "admin",
    "password_vault_ref": "win-workstation-pwd"
  }'
```

### 3. 配置目标主机

#### Windows（RDP）
1. 右键 **此电脑 → 属性 → 远程设置**
2. 启用 **允许远程连接到此计算机**
3. 确保防火墙放行 TCP 3389 端口

#### Linux（VNC）
```bash
# 安装 TigerVNC
sudo apt install tigervnc-standalone-server

# 启动 VNC 服务
vncserver :1 -geometry 1920x1080 -depth 24

# 确保防火墙放行 5901 端口
```

#### Linux/macOS（SSH）
```bash
# 确保 SSH 服务已启用
sudo systemctl enable ssh
sudo systemctl start ssh
```

### 4. 连接远程主机

1. 进入 **服务 → 远程桌面** 页面
2. 点击目标主机的 **连接** 按钮
3. 浏览器内打开远程桌面

### 5. 文件传输

在远程桌面会话中：
- **上传**：使用左侧文件面板拖拽文件到远程桌面
- **下载**：在远程桌面选择文件，右键 → 下载到本地

### 6. 会话录制

```bash
# 启用自动录制
curl -X PUT http://localhost:8080/api/v1/rdp/settings \
  -H "Content-Type: application/json" \
  -d '{"auto_record": true, "recording_path": "/data/recordings"}'

# 回放录制
# 服务 → 远程桌面 → 录制回放 → 选择录制文件
```

## 常见问题

### Q: 浏览器远程桌面流畅吗？
在局域网内延迟 < 20ms，体验接近本地操作。外网访问取决于带宽，建议 10Mbps 以上。

### Q: 支持多少个并发会话？
默认支持 10 个并发会话，可在设置中调整。受 NAS 内存限制。

### Q: 连接安全性如何？
- 网关强制 HTTPS (TLS 1.3)
- RDP/VNC 流量在 NAS 侧加密后通过 HTTPS 隧道传输
- 支持 MFA 二次认证

### Q: 可以外网访问吗？
可以。通过 NAS-OS 的内网穿透功能或 DDNS + 端口转发，从外网安全访问。

### Q: 剪贴板同步不工作？
1. 确认浏览器允许剪贴板权限
2. Chrome/Edge 需要 HTTPS 才能使用剪贴板 API
3. 检查是否在连接设置中禁用了剪贴板

### Q: 文件传输有大小限制吗？
默认单文件限制 2GB，可在设置中调整。建议大文件使用 SMB 共享传输。

## API 参考

### 主机管理

```bash
# 列出所有远程主机
curl http://localhost:8080/api/v1/rdp/hosts

# 获取主机详情
curl http://localhost:8080/api/v1/rdp/hosts/<host-id>

# 删除主机
curl -X DELETE http://localhost:8080/api/v1/rdp/hosts/<host-id>
```

### 会话管理

```bash
# 创建连接会话
curl -X POST http://localhost:8080/api/v1/rdp/sessions \
  -H "Content-Type: application/json" \
  -d '{"host_id": "<host-id>", "display": 0}'

# 获取会话状态
curl http://localhost:8080/api/v1/rdp/sessions/<session-id>

# 断开会话
curl -X DELETE http://localhost:8080/api/v1/rdp/sessions/<session-id>
```

### 文件传输

```bash
# 上传文件到远程主机
curl -X POST http://localhost:8080/api/v1/rdp/sessions/<session-id>/upload \
  -F "file=@local-file.txt" \
  -F "remote_path=C:/Users/Admin/Desktop/"

# 从远程主机下载文件
curl -o local-file.txt \
  http://localhost:8080/api/v1/rdp/sessions/<session-id>/download?path=C:/data/report.pdf
```

### 录制管理

```bash
# 列出录制文件
curl http://localhost:8080/api/v1/rdp/recordings

# 获取录制详情
curl http://localhost:8080/api/v1/rdp/recordings/<recording-id>

# 删除录制
curl -X DELETE http://localhost:8080/api/v1/rdp/recordings/<recording-id>
```

### 设置

```bash
# 获取网关设置
curl http://localhost:8080/api/v1/rdp/settings

# 更新网关设置
curl -X PUT http://localhost:8080/api/v1/rdp/settings \
  -H "Content-Type: application/json" \
  -d '{
    "max_sessions": 10,
    "session_timeout_min": 30,
    "auto_record": false,
    "clipboard_enabled": true,
    "file_transfer_enabled": true,
    "max_file_size_mb": 2048
  }'
```

---

## 相关指南

- [VPN Server](vpn-server-guide.md) — 配合 VPN 实现加密远程访问
- [NAT 穿透](natpierce.md) — 从外网访问 NAS 上的远程桌面
- [LXC 容器沙箱](lxc-sandbox-guide.md) — 在沙箱中测试远程连接
- [合规仪表盘](compliance-dashboard-guide.md) — 远程会话审计与合规检查
