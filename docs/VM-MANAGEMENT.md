# 虚拟机管理模块 (v1.3.0)

## 概述

NAS-OS v1.3.0 新增虚拟机管理功能，基于 KVM/QEMU 和 libvirt，提供完整的虚拟机生命周期管理。

## 核心功能

### 1. VM 创建与管理
- 支持 Linux、Windows 及其他操作系统
- 自定义 CPU、内存、磁盘资源配置
- 桥接/NAT 网络模式选择
- ISO 镜像挂载

### 2. 镜像管理
- ISO 上传/下载
- 内置常用系统镜像（Ubuntu、Debian、CentOS、Windows 等）
- 镜像库管理

### 3. 资源配置
- CPU: 1-64 核心
- 内存：256MB-64GB
- 磁盘：1GB-1TB (qcow2 格式)
- 网络：桥接/桥接模式

### 4. 快照管理
- 创建 VM 快照
- 恢复到任意快照
- 快照删除

### 5. VNC 远程控制
- 内置 VNC 服务器
- Web 控制台访问
- 实时远程操作

### 6. 硬件直通 (可选)
- USB 设备直通
- PCIe 设备直通 (需要硬件支持)

### 7. 模板管理
- 预定义系统模板
- 自定义模板创建
- 快速部署

## API 接口

### 虚拟机管理

```bash
# 获取 VM 列表
GET /api/v1/vms

# 创建 VM
POST /api/v1/vms
{
  "name": "my-vm",
  "description": "测试虚拟机",
  "type": "linux",
  "cpu": 2,
  "memory": 2048,
  "diskSize": 20,
  "network": "bridge",
  "isoPath": "/mnt/isos/ubuntu.iso",
  "vncEnabled": true
}

# 获取 VM 详情
GET /api/v1/vms/{id}

# VM 操作 (启动/停止/重启/删除)
POST /api/v1/vms/{id}
{
  "action": "start|stop|restart|delete|vnc",
  "force": true
}

# 更新 VM
PUT /api/v1/vms/{id}

# 删除 VM
DELETE /api/v1/vms/{id}?force=true
```

### ISO 管理

```bash
# 获取 ISO 列表
GET /api/v1/vm-isos

# 获取 ISO 详情
GET /api/v1/vm-isos/{id}

# 下载 ISO
POST /api/v1/vm-isos/{id}
{
  "action": "download"
}

# 删除 ISO
DELETE /api/v1/vm-isos/{id}
```

### 快照管理

```bash
# 获取快照列表
GET /api/v1/vm-snapshots?vmId={vmId}

# 获取快照详情
GET /api/v1/vm-snapshots/{id}

# 创建快照
POST /api/v1/vm-snapshots
{
  "vmId": "vm-xxx",
  "name": "backup-1",
  "description": "系统备份"
}

# 恢复快照
POST /api/v1/vm-snapshots/{id}
{
  "action": "restore"
}

# 删除快照
DELETE /api/v1/vm-snapshots/{id}
```

### 模板管理

```bash
# 获取模板列表
GET /api/v1/vm-templates
```

### 硬件设备

```bash
# 获取 USB 设备列表
GET /api/v1/vm-usb-devices

# 获取 PCIe 设备列表
GET /api/v1/vm-pci-devices
```

## 内置模板

| 模板 ID | 名称 | CPU | 内存 | 磁盘 | 类型 |
|--------|------|-----|------|------|------|
| tpl-ubuntu-2204 | Ubuntu 22.04 LTS | 2 | 2GB | 20GB | Linux |
| tpl-debian-11 | Debian 11 | 2 | 2GB | 20GB | Linux |
| tpl-windows-11 | Windows 11 | 4 | 4GB | 60GB | Windows |
| tpl-windows-10 | Windows 10 | 2 | 4GB | 50GB | Windows |

## 内置 ISO 镜像

| ID | 名称 | 类型 | 下载链接 |
|----|------|------|----------|
| ubuntu-2204-lts | Ubuntu 22.04 LTS | Ubuntu | 官方源 |
| ubuntu-2004-lts | Ubuntu 20.04 LTS | Ubuntu | 官方源 |
| debian-11 | Debian 11 | Debian | 官方源 |
| debian-12 | Debian 12 | Debian | 官方源 |
| centos-stream-9 | CentOS Stream 9 | CentOS | 官方源 |
| almalinux-9 | AlmaLinux 9 | AlmaLinux | 官方源 |
| windows-11 | Windows 11 | Windows | 微软官网 |
| windows-10 | Windows 10 | Windows | 微软官网 |

## 存储结构

```
/mnt/
├── vms/                  # VM 存储目录
│   ├── vm-xxx/          # VM 实例目录
│   │   ├── disk.qcow2   # 磁盘镜像
│   │   └── config.json  # VM 配置
│   ├── snapshots/       # 快照目录
│   │   ├── snap-xxx/    # 快照数据
│   │   └── snap-xxx.json # 快照元数据
│   └── templates/       # 模板目录
└── isos/                # ISO 镜像目录
```

## 依赖要求

### 系统要求
- Linux 内核 4.0+
- KVM 支持 (Intel VT-x / AMD-V)
- QEMU 4.0+
- libvirt 6.0+ (可选，推荐使用)

### 安装依赖

```bash
# Ubuntu/Debian
sudo apt install qemu-kvm libvirt-daemon-system libvirt-clients bridge-utils virtinst

# CentOS/RHEL
sudo yum install qemu-kvm libvirt libvirt-python libvirt-client bridge-utils virt-install

# 启动 libvirt
sudo systemctl enable --now libvirtd

# 添加用户到 libvirt 组
sudo usermod -aG libvirt $USER
```

### 验证 KVM

```bash
# 检查 KVM 支持
ls -la /dev/kvm

# 检查 KVM 模块
lsmod | grep kvm

# 测试 libvirt
virsh -c qemu:///system list --all
```

## Web UI 使用

### 访问虚拟机管理页面

```
http://<nas-ip>:8080/vms.html
```

### 创建虚拟机步骤

1. 点击"创建虚拟机"按钮
2. 填写虚拟机名称和描述
3. 选择操作系统类型 (Linux/Windows/其他)
4. 配置 CPU、内存、磁盘大小
5. 选择 ISO 安装镜像 (可选)
6. 选择网络模式 (桥接/NAT)
7. 启用 VNC 远程控制
8. 点击"创建"

### 管理虚拟机

- **启动**: 点击"启动"按钮
- **停止**: 运行中 VM 可点击"停止"
- **重启**: 运行中 VM 可点击"重启"
- **控制台**: 点击"控制台"打开 VNC 远程桌面
- **删除**: 点击"删除"移除 VM (需先停止)

### ISO 管理

- 切换到"ISO 镜像"标签
- 点击"下载"获取内置镜像
- 点击"上传 ISO"上传本地镜像文件
- 删除不需要的镜像

## 开发说明

### 模块结构

```
internal/vm/
├── types.go      # 数据类型定义
├── manager.go    # VM 管理器核心逻辑
├── iso.go        # ISO 镜像管理
├── snapshot.go   # 快照管理
└── handlers.go   # HTTP API 处理器
```

### 集成到主程序

在 `internal/web/server.go` 中已自动集成 VM 模块：

```go
// 初始化 VM 管理器
vmMgr, err := vm.NewManager("/mnt/vms", vmLogger)
isoMgr, err := vm.NewISOManager("/mnt/isos", vmLogger)
snapshotMgr, err := vm.NewSnapshotManager("/mnt/vms", vmMgr, vmLogger)

// 注册 API 路由
vmHandler := vm.NewHandler(vmMgr, isoMgr, snapshotMgr, logger)
vmHandler.RegisterRoutes(api)
```

### 扩展开发

#### 添加新的 VM 配置选项

1. 在 `types.go` 中添加字段到 `VMConfig`
2. 在 `manager.go` 的 `generateLibvirtXML` 中使用新字段
3. 在 `vms.html` 中添加 UI 表单控件

#### 支持新的虚拟化技术

修改 `manager.go` 中的 `checkLibvirt()` 和相关 QEMU 调用：

```go
// 示例：添加对 bhyve 的支持
func (m *Manager) createBhyveVM(vm *VM) error {
    // bhyve 特定实现
}
```

## 故障排除

### VM 无法启动

1. 检查 KVM 支持：`kvm-ok`
2. 检查 libvirt 状态：`systemctl status libvirtd`
3. 查看日志：`journalctl -u libvirtd`

### VNC 连接失败

1. 确认 VNC 端口未被占用
2. 检查防火墙设置
3. 验证 VM 状态为运行中

### ISO 下载失败

1. 检查网络连接
2. 验证下载链接有效性
3. 检查磁盘空间

## 性能优化

### 磁盘性能

- 使用 virtio 驱动
- 启用磁盘缓存
- 使用 SSD 存储

### 网络性能

- 使用 virtio-net
- 配置 SR-IOV (如支持)
- 使用桥接模式

### 内存优化

- 启用内存气球 (virtio-balloon)
- 配置大页内存
- 使用内存超分 (谨慎使用)

## 安全建议

1. **隔离网络**: 为 VM 使用独立的 VLAN
2. **访问控制**: 限制 VNC 访问 IP
3. **快照备份**: 定期创建重要 VM 快照
4. **资源限制**: 设置 CPU/内存上限防止资源耗尽
5. **审计日志**: 启用 VM 操作日志记录

## 未来计划

- [ ] VM 克隆功能
- [ ] 批量操作
- [ ] VM 迁移 (live migration)
- [ ] 云初始化 (cloud-init)
- [ ] 自动快照计划
- [ ] VM 性能监控图表
- [ ] 容器与 VM 混合部署
- [ ] GPU 直通支持

## 参考资料

- [libvirt 文档](https://libvirt.org/)
- [QEMU 文档](https://www.qemu.org/documentation/)
- [KVM 文档](https://www.linux-kvm.org/page/Documentation)
- [noVNC](https://novnc.com/)
