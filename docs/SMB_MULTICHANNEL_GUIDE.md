# SMB Multichannel 用户指南

> 版本: v2.421.0 | 编制: 礼部 | 日期: 2026-04-07

本指南介绍如何配置和使用 SMB Multichannel 功能，实现多网卡并行传输，大幅提升文件传输性能。

---

## 目录

- [功能概述](#功能概述)
- [工作原理](#工作原理)
- [配置步骤](#配置步骤)
- [使用场景](#使用场景)
- [性能对比](#性能对比)
- [最佳实践](#最佳实践)
- [故障排查](#故障排查)
- [常见问题](#常见问题)

---

## 功能概述

### 什么是 SMB Multichannel？

SMB Multichannel（多通道SMB）是 SMB 3.0 协议的核心特性，允许客户端与服务器之间建立多个并行TCP连接，实现：

- **带宽聚合**：多个网络通道并行传输，总带宽 = 各通道带宽之和
- **故障冗余**：单通道故障时自动切换到其他通道，传输不中断
- **负载均衡**：自动分配数据流到各通道，避免单通道拥堵

### 性能提升预期

| 网卡配置 | 单通道速度 | 多通道速度 | 提升倍数 |
|----------|------------|------------|:--------:|
| 双 1GbE | 110 MB/s | 220 MB/s | 2x |
| 1GbE + 10GbE | 110 MB/s | 1.1 GB/s | 10x |
| 双 10GbE | 1.1 GB/s | 2.2 GB/s | 2x |
| RDMA (NVMe-oF) | 1.1 GB/s | 3+ GB/s | 3x+ |

### 适用场景

| 场景 | 典型用途 | 推荐配置 |
|------|----------|----------|
| 视频剪辑工作室 | 大文件实时编辑 | 双10GbE或RDMA |
| 虚拟化环境 | VM镜像传输 | 双1GbE起步 |
| 备份服务器 | 大规模数据备份 | 双1GbE或双10GbE |
| 家庭NAS | 多用户并发访问 | 双1GbE |
| 企业文件服务器 | 高并发读写 | 多网卡负载均衡 |

---

## 工作原理

### 架构示意

```
┌─────────────────────────────────────────────────────────┐
│                    NAS-OS SMB服务                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────┐    ┌─────────────────┐            │
│  │   网卡1 (eth0)  │    │   网卡2 (eth1)  │            │
│  │   192.168.1.100 │    │   192.168.2.100 │            │
│  │   1 Gbps        │    │   1 Gbps        │            │
│  └─┬───────────────┘    └─┬───────────────┘            │
│    │                      │                             │
│    └──────────┬───────────┘                             │
│               │                                         │
│        ┌──────▼──────┐                                  │
│        │ SMB Server  │                                  │
│        │ Multichannel│                                  │
│        │   Manager   │                                  │
│        └──────┬──────┘                                  │
│               │                                         │
│  ┌────────────▼────────────┐                           │
│  │    Windows Client       │                           │
│  │                         │                           │
│  │  Channel 1 ── eth0      │  1 Gbps                   │
│  │  Channel 2 ── eth1      │  1 Gbps                   │
│  │                         │                           │
│  │  总带宽: 2 Gbps         │                           │
│  └─────────────────────────┘                           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 通道建立流程

1. **网卡发现**：服务器检测可用网卡及其IP地址
2. **能力协商**：客户端连接时协商多通道能力
3. **通道建立**：客户端为每个网卡IP建立独立TCP连接
4. **数据分发**：传输数据自动分发到各通道

### 网络要求

| 要求 | 说明 |
|------|------|
| 独立IP地址 | 每个网卡需要独立的IP地址 |
| 不同子网 | 建议每个网卡配置在不同子网（避免路由冲突） |
| 路由可达 | 各网卡IP从客户端均可达 |
| 无NAT环境 | NAT可能导致多通道识别失败 |

---

## 配置步骤

### 步骤一：准备网卡配置

#### 1. 检查网卡状态

```bash
# 查看所有网卡
ip addr show

# 输出示例
eth0: 192.168.1.100/24  (UP, 1Gbps)
eth1: 192.168.2.100/24  (UP, 1Gbps)
```

#### 2. 配置独立子网

建议配置：

```
eth0: 192.168.1.100/24  → 子网 192.168.1.0/24
eth1: 192.168.2.100/24  → 子网 192.168.2.0/24
```

#### 3. 确认客户端路由

客户端需要能够访问所有网卡IP：

```bash
# 在客户端检查（Windows）
ping 192.168.1.100
ping 192.168.2.100

# 路由表检查
route print
```

### 步骤二：启用 SMB Multichannel

#### 方式一：Web界面配置

```
1. 进入 控制面板 → 服务 → SMB
2. 点击 高级设置
3. 启用 多通道支持 (SMB Multichannel)
4. 选择网卡绑定模式：
   - 自动模式：系统自动检测可用网卡
   - 手动模式：指定特定网卡参与多通道
5. 保存配置
```

#### 方式二：API配置

```bash
curl -X PUT https://nas.local/api/v1/services/smb/config \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "multichannel": {
      "enabled": true,
      "mode": "auto",
      "interfaces": ["eth0", "eth1"]
    }
  }'
```

#### 方式三：手动配置 smb.conf

编辑 `/etc/samba/smb.conf`：

```ini
[global]
    # 启用多通道
    server multi channel support = yes
    
    # 绑定网卡
    interfaces = eth0 eth1
    bind interfaces only = yes
    
    # 性能优化
    aio read size = 1
    aio write size = 1
    max mux = 50
    socket options = TCP_NODELAY SO_RCVBUF=65536 SO_SNDBUF=65536
```

重启SMB服务：

```bash
systemctl restart smb
```

### 步骤三：验证多通道工作

#### Windows客户端验证

1. 打开 PowerShell，运行：

```powershell
# 查看SMB连接详情
Get-SmbConnection

# 查看多通道状态
Get-SmbMultichannelConnection -SmbServer NAS_IP
```

输出示例（成功）：

```
ServerName    ClientIP        ServerIP        ChannelCount
nas-server    192.168.1.50    192.168.1.100   2
              192.168.2.50    192.168.2.100
```

#### Linux客户端验证

```bash
# 使用 smbclient 测试
smbclient -L nas-server -U user

# 查看连接状态
mount.cifs //nas-server/share /mnt -o vers=3.0,multichannel
```

---

## 使用场景

### 场景一：视频剪辑工作室

**需求**：
- 4K视频实时编辑
- 大文件（50GB+）快速传输
- 多编辑师并发工作

**推荐配置**：
```
硬件：
  - 双 10GbE 网卡
  - NVMe SSD 存储
  - 支持 RDMA 的网络交换机

NAS-OS 配置：
  - SMB Multichannel: auto
  - RDMA 支持: enabled (Phase2)
  - 存储: NVMe 存储池

预期性能：
  - 单文件传输: 2+ GB/s
  - 多用户并发: 10GbE x 用户数
```

### 场景二：企业文件服务器

**需求**：
- 100+ 用户并发访问
- 文档实时协作
- 高可用要求

**推荐配置**：
```
硬件：
  - 4 x 1GbE 网卡（或双 10GbE）
  - RAID 10 存储

NAS-OS 配置：
  - SMB Multichannel: auto
  - 通道冗余: enabled
  - 负载均衡: enabled

预期性能：
  - 单用户传输: 110 MB/s (单网卡)
  - 多用户并发: 400+ MB/s (4网卡聚合)
  - 故障切换: 单网卡故障不中断
```

### 场景三：家庭NAS

**需求**：
- 家庭成员并发访问
- 媒体文件流媒体播放
- 成本敏感

**推荐配置**：
```
硬件：
  - 双 1GbE 网卡（主板自带 + PCIe扩展）

NAS-OS 配置：
  - SMB Multichannel: auto
  - 存储: HDD 存储池 + SSD 缓存

预期性能：
  - 单用户大文件: 220 MB/s
  - 多用户并发: 2x 提升
```

---

## 性能对比

### 实测数据（参考）

#### 双 1GbE 配置

| 测试项目 | 单通道 | 多通道 | 提升 |
|----------|:------:|:------:|:----:|
| 10GB文件传输 | 90秒 | 45秒 | 2x |
| 100并发读写 | 50 MB/s | 100 MB/s | 2x |
| 单网卡故障恢复 | 连接中断 | 自动切换 | ✅ |

#### 双 10GbE 配置

| 测试项目 | 单通道 | 多通道 | 提升 |
|----------|:------:|:------:|:----:|
| 50GB视频传输 | 45秒 | 22秒 | 2x |
| 4K视频实时编辑 | 卡顿 |流畅 | ✅ |
| VM镜像传输(100GB) | 90秒 | 45秒 | 2x |

### 性能监控

```bash
# 查看SMB多通道实时状态
curl https://nas.local/api/v1/services/smb/multichannel/status

# 输出示例
{
  "channels": [
    {"interface": "eth0", "ip": "192.168.1.100", "speed": "1Gbps", "active_connections": 5},
    {"interface": "eth1", "ip": "192.168.2.100", "speed": "1Gbps", "active_connections": 3}
  ],
  "total_bandwidth": "2Gbps",
  "throughput": {"read": "180MB/s", "write": "150MB/s"}
}
```

---

## 最佳实践

### 1. 网络配置建议

#### 推荐配置

```
┌──────────────────────────────────────────┐
│  网卡配置最佳实践                         │
├──────────────────────────────────────────┤
│                                          │
│  ✅ 不同子网                             │
│     eth0: 192.168.1.x                   │
│     eth1: 192.168.2.x                   │
│                                          │
│  ✅ 独立物理网卡                         │
│     避免单网卡多端口                     │
│                                          │
│  ✅ 同速网卡                             │
│     推荐使用相同速度网卡                 │
│                                          │
│  ❌ 避免同子网                           │
│     可能导致路由冲突                     │
│                                          │
└──────────────────────────────────────────┘
```

#### 不推荐配置

```
❌ 同子网配置（可能失败）：
   eth0: 192.168.1.100
   eth1: 192.168.1.101
   → 客户端无法区分通道

❌ 单网卡多端口：
   使用单网卡的两个端口
   → 无法实现真正并行传输
```

### 2. 存储配置配合

| 网卡配置 | 存储建议 | 说明 |
|----------|----------|------|
| 双1GbE | HDD + SSD缓存 | 存储速度匹配网络 |
| 双10GbE | NVMe存储池 | 发挥网络带宽优势 |
| RDMA | 全NVMe架构 | 高性能场景必选 |

### 3. 客户端优化

#### Windows客户端设置

```powershell
# 启用SMB 3.0
Set-SmbClientConfiguration -EnableSecuritySignature $true

# 查看多通道设置
Get-SmbClientConfiguration | Select-Object EnableMultichannel

# 确认版本
Get-SmbClientConfiguration | Select-Object Smb2ProtocolVersion
```

#### macOS客户端

macOS 10.15+ 默认支持 SMB 3.0：

```bash
# 指定SMB版本挂载
mount_smbfs //user@nas-server/share /Volumes/share
```

### 4. 故障切换测试

定期测试故障切换能力：

```bash
# 模拟单网卡故障
ip link set eth0 down

# 检查传输是否继续
# Windows: Get-SmbMultichannelConnection
# 传输应自动切换到 eth1

# 恢复网卡
ip link set eth0 up

# 等待通道重新建立
# 通常 5-10 秒自动恢复
```

---

## 故障排查

### 问题一：多通道未生效

**症状**：
- 配置后传输速度未提升
- `Get-SmbMultichannelConnection` 显示单通道

**排查步骤**：

```bash
# 1. 检查SMB配置
cat /etc/samba/smb.conf | grep multichannel

# 2. 检查网卡绑定
ip addr show | grep -A2 eth

# 3. 检查客户端可达性
ping 192.168.1.100
ping 192.168.2.100

# 4. Windows客户端检查
Get-SmbConnection | Format-List
```

**常见原因**：
- 网卡配置在同一子网
- 客户端不支持 SMB 3.0
- NAT环境干扰

### 问题二：通道频繁切换

**症状**：
- 传输过程中通道频繁变化
- 性能不稳定

**排查**：

```bash
# 检查网卡稳定性
ip -s link show eth0
ip -s link show eth1

# 检查错误计数
# 如果 errors/packets 比例高，检查物理连接
```

**解决方法**：
- 检查网线质量
- 检查交换机端口状态
- 固定绑定特定网卡

### 问题三：RDMA不工作

**症状**：
- 配置RDMA后无性能提升

**排查**：

```bash
# 检查网卡RDMA能力
rdma link show

# 检查NVMe-oF服务
systemctl status nvmeof
```

**注意**：
- RDMA需要专用网络交换机支持
- 目前NAS-OS RDMA支持为Phase2开发中

---

## 常见问题

### Q1: 所有客户端都支持多通道吗？

**支持列表**：

| 客户端 | SMB版本 | 多通道支持 | 备注 |
|--------|:-------:|:----------:|------|
| Windows 8/10/11 | SMB 3.0+ | ✅ | 完全支持 |
| Windows Server 2012+ | SMB 3.0+ | ✅ | 完全支持 |
| Windows 7 | SMB 2.1 | ❌ | 不支持 |
| macOS 10.15+ | SMB 3.0 | ⚠️ | 部分支持 |
| Linux | SMB 3.0 | ⚠️ | 需内核支持 |

### Q2: 多通道会增加内存/CPU占用吗？

**回答**：
- 内存：每个通道增加约 5-10MB 内存占用
- CPU：略微增加（多TCP连接管理）
- 总体影响较小，性能提升远大于开销

### Q3: 可以只使用部分网卡吗？

**回答**：可以。使用手动模式指定网卡：

```json
{
  "multichannel": {
    "mode": "manual",
    "channels": [
      {"interface": "eth0", "priority": 1},
      {"interface": "eth1", "priority": 2}
    ]
  }
}
```

### Q4: 无线网卡可以使用多通道吗？

**回答**：
- Wi-Fi理论上支持，但实际效果不稳定
- 建议：
  - 有线网卡用于多通道
  - Wi-Fi作为备份通道

### Q5: NAT环境如何处理？

**回答**：
- NAT环境可能导致多通道识别失败
- 解决方案：
  1. 配置端口转发到所有网卡IP
  2. 或使用VPN穿透
  3. 或使用单通道模式

---

## 相关文档

- [多通道SMB设计文档](../features/MULTICHANNEL_SMB.md)
- [SMB服务配置指南](./API.md#smb)
- [竞品对比分析](./COMPETITOR_ANALYSIS.md)
- [网络配置最佳实践](./BEST_PRACTICES.md#network)

---

## 参考资料

- [Microsoft SMB Multichannel Documentation](https://docs.microsoft.com/en-us/windows-server/storage/file-server/smb-multichannel)
- [Samba SMB Multichannel Wiki](https://wiki.samba.org/index.php/SMB_Multichannel)
- [SMB 3.0 Protocol Specification](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2)

---

*文档编制: 礼部文档组*
*反馈渠道: docs@nas-os.local*
*最后更新: 2026-04-07*