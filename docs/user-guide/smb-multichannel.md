# SMB Multichannel 用户指南

**版本**: v2.421.0  
**更新日期**: 2026-04-24

---

## 概述

SMB Multichannel（多通道SMB）是 SMB 3.0 协议的核心特性，允许客户端与服务器之间建立多个并行 TCP 连接，实现带宽聚合、故障冗余和负载均衡。

### 主要优势

- **带宽聚合**：多个网络通道并行传输，总带宽 = 各通道带宽之和
- **故障冗余**：单通道故障时自动切换，传输不中断
- **负载均衡**：自动分配数据流到各通道，避免单通道拥堵

### 适用场景

| 场景 | 说明 | 推荐配置 |
|------|------|----------|
| 视频剪辑工作室 | 4K/8K视频实时编辑 | 双10GbE或RDMA |
| 虚拟化环境 | VM镜像传输 | 双1GbE起步 |
| 备份服务器 | 大规模数据备份 | 双1GbE或双10GbE |
| 企业文件服务器 | 高并发读写 | 多网卡负载均衡 |

---

## 快速开始

### 性能提升预期

| 网卡配置 | 单通道速度 | 多通道速度 | 提升倍数 |
|----------|------------|------------|:--------:|
| 双 1GbE | 110 MB/s | 220 MB/s | 2x |
| 1GbE + 10GbE | 110 MB/s | 1.1 GB/s | 10x |
| 双 10GbE | 1.1 GB/s | 2.2 GB/s | 2x |

### 基本配置步骤

1. **准备网卡**：配置多个网卡在不同子网
2. **启用多通道**：Web界面或API配置
3. **验证工作状态**：Windows客户端检查连接

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

#### 2. 配置独立子网（推荐）

```
eth0: 192.168.1.100/24  → 子网 192.168.1.0/24
eth1: 192.168.2.100/24  → 子网 192.168.2.0/24
```

#### 3. 确认客户端可达

```bash
# 在客户端检查（Windows）
ping 192.168.1.100
ping 192.168.2.100
```

### 步骤二：启用 SMB Multichannel

#### 方式一：Web界面配置

1. 进入 **控制面板** → **服务** → **SMB**
2. 点击 **高级设置**
3. 启用 **多通道支持 (SMB Multichannel)**
4. 选择网卡绑定模式：
   - **自动模式**：系统自动检测可用网卡
   - **手动模式**：指定特定网卡参与多通道
5. 保存配置

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

重启服务：

```bash
systemctl restart smb
```

### 步骤三：验证多通道工作

#### Windows客户端验证

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

---

## 使用场景详解

### 场景一：视频剪辑工作室

**配置建议**：
- 硬件：双 10GbE 网卡 + NVMe SSD 存储
- NAS-OS：SMB Multichannel auto + RDMA支持（Phase2）
- 预期性能：单文件传输 2+ GB/s

### 场景二：企业文件服务器

**配置建议**：
- 硬件：4 x 1GbE（或双 10GbE）+ RAID 10
- NAS-OS：多通道 + 通道冗余 + 负载均衡
- 预期性能：多用户并发 400+ MB/s，单网卡故障不中断

### 场景三：家庭NAS

**配置建议**：
- 硬件：双 1GbE（主板自带 + PCIe扩展）
- NAS-OS：SMB Multichannel auto
- 预期性能：单用户大文件 220 MB/s

---

## 最佳实践

### 网络配置建议

| 配置项 | 推荐 | 避免 |
|--------|------|------|
| 子网配置 | 不同子网 | 同子网（路由冲突） |
| 网卡类型 | 独立物理网卡 | 单网卡多端口 |
| 网卡速度 | 同速网卡 | 混速（负载不均） |

### 存储配置配合

| 网卡配置 | 存储建议 |
|----------|----------|
| 双1GbE | HDD + SSD缓存 |
| 双10GbE | NVMe存储池 |
| RDMA | 全NVMe架构 |

### 客户端优化

#### Windows客户端

```powershell
# 启用SMB 3.0
Set-SmbClientConfiguration -EnableSecuritySignature $true

# 确认版本
Get-SmbClientConfiguration | Select-Object Smb2ProtocolVersion
```

#### macOS客户端

macOS 10.15+ 默认支持 SMB 3.0：

```bash
mount_smbfs //user@nas-server/share /Volumes/share
```

---

## 故障排查

### 问题一：多通道未生效

**症状**：配置后传输速度未提升

**排查步骤**：

1. 检查SMB配置：`cat /etc/samba/smb.conf | grep multichannel`
2. 检查网卡绑定：`ip addr show`
3. 检查客户端可达性：ping各网卡IP
4. Windows客户端检查：`Get-SmbConnection | Format-List`

**常见原因**：
- 网卡配置在同一子网
- 客户端不支持 SMB 3.0（Windows 7以下）
- NAT环境干扰

### 问题二：通道频繁切换

**症状**：传输过程中通道频繁变化

**排查**：

```bash
# 检查网卡稳定性
ip -s link show eth0
ip -s link show eth1
```

**解决方法**：
- 检查网线质量
- 检查交换机端口状态
- 使用手动模式固定网卡

### 问题三：RDMA不工作

**症状**：配置RDMA后无性能提升

**排查**：

```bash
# 检查网卡RDMA能力
rdma link show

# 检查NVMe-oF服务
systemctl status nvmeof
```

---

## 常见问题

### Q1: 哪些客户端支持多通道？

| 客户端 | SMB版本 | 多通道支持 |
|--------|:-------:|:----------:|
| Windows 8/10/11 | SMB 3.0+ | ✅ 完全支持 |
| Windows Server 2012+ | SMB 3.0+ | ✅ 完全支持 |
| Windows 7 | SMB 2.1 | ❌ 不支持 |
| macOS 10.15+ | SMB 3.0 | ⚠️ 部分支持 |
| Linux | SMB 3.0 | ⚠️ 需内核支持 |

### Q2: 多通道会增加资源占用吗？

- 内存：每个通道增加约 5-10MB
- CPU：略微增加（多TCP连接管理）
- 总体影响较小，性能提升远大于开销

### Q3: 无线网卡可以使用多通道吗？

Wi-Fi理论上支持，但实际效果不稳定。建议：
- 有线网卡用于多通道
- Wi-Fi作为备份通道

### Q4: NAT环境如何处理？

NAT可能导致多通道识别失败。解决方案：
1. 配置端口转发到所有网卡IP
2. 使用VPN穿透
3. 使用单通道模式

---

## API 参考

### 查询多通道状态

```bash
curl -X GET https://nas.local/api/v1/services/smb/multichannel/status \
  -H "Authorization: Bearer <token>"
```

### 更新多通道配置

```bash
curl -X PUT https://nas.local/api/v1/services/smb/config \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "multichannel": {
      "enabled": true,
      "mode": "manual",
      "channels": [
        {"interface": "eth0", "priority": 1},
        {"interface": "eth1", "priority": 2}
      ]
    }
  }'
```

---

## 相关文档

- [多通道SMB设计文档](../SMB_MULTICHANNEL_GUIDE.md)
- [SMB审计指南](smb-auditing.md)
- [网络配置最佳实践](../NETWORK_API.md)

---

*文档编制: 礼部文档组*
*最后更新: 2026-04-24*