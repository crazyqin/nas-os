# NVMe over Fabrics 使用指南

**功能版本**: v2.320.0 | **文档创建**: 2026-03-30 | **部门**: 礼部

---

## 一、概念介绍

### 1.1 什么是NVMe-oF

NVMe over Fabrics (NVMe-oF) 是一种高性能存储网络协议，通过标准网络（TCP或RDMA）远程访问NVMe存储设备，性能接近本地NVMe。

### 1.2 核心优势

| 优势 | 说明 |
|------|------|
| **性能卓越** | 存储网络性能提升10倍，接近本地NVMe |
| **低延迟** | 微秒级延迟，适合高性能应用 |
| **高吞吐** | 支持多队列并行，充分利用NVMe性能 |
| **远程访问** | 跨服务器访问存储，实现存储分离 |
| **灵活部署** | 支持TCP（通用）和RDMA（高性能）两种传输模式 |

### 1.3 适用场景

| 场景 | 说明 |
|------|------|
| **虚拟化平台** | 为虚拟机提供高性能存储 |
| **数据库服务器** | 降低数据库I/O延迟 |
| **高性能计算** | HPC集群共享存储 |
| **容器存储** | Kubernetes持久化存储后端 |
| **存储分离架构** | 计算节点与存储节点分离 |

### 1.4 竞品对标

对标TrueNAS 25.04 NVMe-oF实现：

| 特性 | TrueNAS | NAS-OS |
|------|---------|--------|
| NVMe-oF Target | ✅ | ✅ |
| NVMe-oF Initiator | ✅ | ✅ |
| TCP传输模式 | ✅ | ✅ |
| RDMA传输模式 | ✅ | ✅ |
| 多路径支持 | ✅ | ✅ |
| WebUI配置 | ✅ | ✅ |
| 性能监控 | ✅ | ✅ |

---

## 二、架构说明

### 2.1 Target与Initiator

NVMe-oF采用Target-Initiator架构：

| 角色 | 说明 | NAS-OS支持 |
|------|------|------------|
| **Target** | 服务端，将本地NVMe设备导出为网络存储 | ✅ |
| **Initiator** | 客户端，连接远程NVMe-oF Target | ✅ |

### 2.2 传输模式

| 模式 | 网络要求 | 性能 | 适用场景 |
|------|----------|------|----------|
| **TCP** | 标准以太网 | 良好 | 通用部署，无需特殊硬件 |
| **RDMA** | InfiniBand/RoCE | 最佳 | 高性能场景，需要RDMA网卡 |

### 2.3 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    NVMe-oF Architecture                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────┐         网络传输          ┌─────────────────┐│
│  │  Initiator主机   │◄──────────────────────► │  Target NAS     ││
│  │                  │   TCP / RDMA            │                 ││
│  ├─────────────────┤                          ├─────────────────┤│
│  │  nvme-cli       │                          │  nvmet内核模块   ││
│  │  nvme内核模块    │                          │                 ││
│  ├─────────────────┤                          ├─────────────────┤│
│  │  应用程序        │                          │  NVMe设备       ││
│  │  /dev/nvmeXnY   │                          │  /dev/nvme0n1   ││
│  └─────────────────┘                          └─────────────────┘│
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 三、Target配置（服务端）

### 3.1 准备工作

#### 硬件要求

- NVMe SSD设备（推荐企业级）
- 网络接口：以太网（TCP模式）或RDMA网卡（RDMA模式）

#### 软件要求

- NAS-OS v2.320.0+
- 内核模块：`nvmet`, `nvme-loop`（TCP）或 `nvme-rdma`（RDMA）

### 3.2 WebUI配置

通过WebUI「存储管理」→「NVMe-oF」→「Target」配置：

#### 创建Target

1. 点击「新建Target」
2. 选择NVMe设备（如 `/dev/nvme0n1`）
3. 设置Target名称（NQN）
4. 选择传输模式（TCP/RDMA）
5. 配置监听地址和端口
6. 点击「保存并启用」

#### 配置参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| **Target NQN** | NVMe Qualified Name，唯一标识 | `nqn.2026-01.nas-os:target1` |
| **设备路径** | NVMe设备路径 | `/dev/nvme0n1` |
| **传输模式** | TCP或RDMA | TCP |
| **监听地址** | 监听IP地址 | `0.0.0.0`（所有接口） |
| **监听端口** | 监听端口 | `4420` |
| **允许主机** | 允许连接的主机列表 | 所有主机 |

### 3.3 命令行配置

使用nvme-cli工具配置：

```bash
# 加载内核模块
modprobe nvmet
modprobe nvme-loop    # TCP模式
# modprobe nvme-rdma  # RDMA模式

# 创建Target
nvmecli create-target --nqn=nqn.2026-01.nas-os:target1

# 添加Namespace（存储设备）
nvmecli create-namespace --device=/dev/nvme0n1 --target=nqn.2026-01.nas-os:target1

# 创建端口
nvmecli create-port --trtype=tcp --traddr=192.168.1.100 --trsvcid=4420

# 绑定Target到端口
nvmecli bind-target --nqn=nqn.2026-01.nas-os:target1 --port=1
```

### 3.4 RDMA模式配置

RDMA模式需要额外配置：

#### 网卡配置

```bash
# 检查RDMA网卡
ibv_devinfo

# 配置IP地址（RoCE）
ip addr add 192.168.100.100/24 dev roce0
```

#### RDMA Target创建

```bash
# 创建RDMA端口
nvmecli create-port --trtype=rdma --traddr=192.168.100.100 --trsvcid=4420

# 绑定Target
nvmecli bind-target --nqn=nqn.2026-01.nas-os:rdma-target --port=1
```

### 3.5 访问控制

#### 主机白名单

限制允许连接的主机：

```bash
# 允许特定主机连接
nvmecli allow-host --nqn=nqn.2026-01.nas-os:target1 --host=nqn.2026-01.client:*

# 禁止其他主机
nvmecli disallow-all-hosts --nqn=nqn.2026-01.nas-os:target1
```

---

## 四、Initiator配置（客户端）

### 4.1 准备工作

#### 硬件要求

- 网络接口：以太网或RDMA网卡
- 与Target网络可达

#### 软件要求

- Linux系统（内核5.0+）
- nvme-cli工具
- 内核模块：`nvme`, `nvme-core`

### 4.2 WebUI配置

在作为Initiator的NAS-OS上：

通过WebUI「存储管理」→「NVMe-oF」→「Initiator」配置：

#### 连接Target

1. 点击「新建连接」
2. 输入Target地址（IP或主机名）
3. 输入Target NQN
4. 选择传输模式（TCP/RDMA）
5. 输入端口（默认4420）
6. 点击「连接」

#### 连接参数说明

| 参数 | 说明 |
|------|------|
| **Target地址** | Target服务端IP地址 |
| **Target NQN** | Target的唯一标识名称 |
| **传输模式** | TCP或RDMA |
| **端口** | Target监听端口 |
| **连接名称** | 本地连接标识（可选） |

### 4.3 命令行配置

```bash
# 加载内核模块
modprobe nvme
modprobe nvme-core
modprobe nvme-tcp    # TCP模式
# modprobe nvme-rdma # RDMA模式

# 发现Target
nvme discover --transport=tcp --traddr=192.168.1.100 --trsvcid=4420

# 连接Target
nvme connect --transport=tcp --traddr=192.168.1.100 --trsvcid=4420 --nqn=nqn.2026-01.nas-os:target1

# 查看连接设备
nvme list

# 输出示例：
# Node          SN               Model
# /dev/nvme1n1  NASOS_TARGET1    NAS-OS NVMe-oF Target
```

### 4.4 使用NVMe-oF设备

连接成功后，设备出现在 `/dev/nvmeXnY`：

```bash
# 查看设备信息
nvme id-ctrl /dev/nvme1n1

# 创建文件系统
mkfs.ext4 /dev/nvme1n1

# 挂载使用
mount /dev/nvme1n1 /mnt/nvme-storage

# 使用LVM管理
pvcreate /dev/nvme1n1
vgcreate nvmevg /dev/nvme1n1
lvcreate -L 100G -n nvmevol nvmevg
```

### 4.5 断开连接

```bash
# 断开指定连接
nvme disconnect --nqn=nqn.2026-01.nas-os:target1

# 断开所有连接
nvme disconnect-all
```

---

## 五、多路径配置（高可用）

### 5.1 多路径架构

多路径提供故障切换和负载均衡：

```
┌─────────────────┐
│  Initiator      │
│  多路径DM        │
│  /dev/dm-0      │
├─────────────────┤
│  path1: nvme1n1 │──► Target A (192.168.1.100)
│  path2: nvme2n1 │──► Target B (192.168.1.101)
└─────────────────┘
```

### 5.2 配置多路径

#### 安装multipath工具

```bash
# Ubuntu/Debian
apt install multipath-tools

# CentOS/RHEL
yum install device-mapper-multipath
```

#### 配置multipath.conf

```bash
# /etc/multipath.conf
defaults {
    user_friendly_names yes
    find_multipaths yes
}

blacklist {
    # 排除本地NVMe设备
    devnode "^nvme0"
}

multipaths {
    multipath {
        wwid "NVMe.NQN.2026-01.nas-os:target1"
        alias "nas-nvme-shared"
    }
}
```

#### 启用多路径

```bash
# 启动multipath服务
systemctl start multipathd
systemctl enable multipathd

# 连接多个Target路径
nvme connect --transport=tcp --traddr=192.168.1.100 --nqn=nqn.2026-01.nas-os:target1
nvme connect --transport=tcp --traddr=192.168.1.101 --nqn=nqn.2026-01.nas-os:target1

# 查看多路径状态
multipath -ll
```

### 5.3 故障切换测试

```bash
# 模拟路径故障
nvme disconnect --traddr=192.168.1.100

# 查看多路径状态（应自动切换）
multipath -ll

# 重新连接恢复路径
nvme connect --transport=tcp --traddr=192.168.1.100 --nqn=nqn.2026-01.nas-os:target1
```

---

## 六、性能优化建议

### 6.1 网络优化

#### TCP模式优化

| 优化项 | 建议 |
|--------|------|
| **MTU** | 使用9000（Jumbo Frame） |
| **网络带宽** | 至少10GbE，推荐25GbE+ |
| **CPU亲和性** | 绑定NVMe-oF线程到专用CPU核心 |
| **TCP参数** | 调整窗口大小和缓冲区 |

```bash
# 设置MTU
ip link set eth0 mtu 9000

# TCP参数优化
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216
sysctl -w net.ipv4.tcp_rmem="4096 87380 16777216"
sysctl -w net.ipv4.tcp_wmem="4096 65536 16777216"
```

#### RDMA模式优化

| 优化项 | 建议 |
|--------|------|
| **RDMA网卡** | 使用InfiniBand或RoCEv2 |
| **带宽** | 至少40Gb/s，推荐100Gb/s |
| **队列深度** | 增加SQ和CQ深度 |
| **CPU亲和性** | 绑定RDMA队列到专用核心 |

### 6.2 NVMe参数优化

#### 队列配置

```bash
# 增加IO队列数量
nvme connect ... --nr-io-queues=16

# 增加队列深度
nvme connect ... --queue-size=128
```

#### 内核参数

```bash
# 增加NVMe超时时间（网络环境需更长）
echo 60 > /sys/class/nvme/nvme1/timeout

# 启用NVMe特性
echo 1 > /sys/module/nvme_core/parameters/iopolicy
```

### 6.3 性能基准测试

#### 使用fio测试

```bash
# 安装fio
apt install fio

# 随机读写测试
fio --filename=/dev/nvme1n1 --direct=1 --rw=randread --bs=4k --ioengine=libaio --iodepth=64 --numjobs=4 --runtime=60 --group_reporting --name=nvme-test

# 吞吐量测试
fio --filename=/dev/nvme1n1 --direct=1 --rw=read --bs=1M --ioengine=libaio --iodepth=32 --numjobs=2 --runtime=60 --group_reporting --name=nvme-throughput
```

#### 性能对比

| 测试场景 | 本地NVMe | NVMe-oF TCP | NVMe-oF RDMA |
|----------|----------|-------------|--------------|
| 4K随机读IOPS | 500K | 350K | 450K |
| 4K随机写IOPS | 400K | 280K | 380K |
| 顺序读吞吐 | 3GB/s | 2.2GB/s | 2.8GB/s |
| 延迟 | 10μs | 50μs | 20μs |

### 6.4 性能监控

#### WebUI监控

「存储管理」→「NVMe-oF」→「监控」提供：

- 连接状态实时显示
- IOPS和吞吐量图表
- 延迟统计
- 错误计数

#### 命令行监控

```bash
# 查看NVMe设备状态
nvme smart-log /dev/nvme1n1

# 查看连接统计
nvme get-stats /dev/nvme1n1

# 查看性能计数器
cat /sys/class/nvme/nvme1n1/device/statistics/*
```

---

## 七、故障排查

### 7.1 常见问题

#### 连接失败

| 原因 | 解决方案 |
|------|----------|
| Target未启动 | 检查Target服务状态 |
| 网络不通 | ping测试网络连通性 |
| 端口错误 | 确认Target监听端口 |
| NQN错误 | 确认Target NQN名称 |

```bash
# 发现Target测试
nvme discover --transport=tcp --traddr=192.168.1.100

# 检查Target状态
nvmecli list-targets
```

#### 性能不佳

| 原因 | 解决方案 |
|------|----------|
| 网络带宽不足 | 升级到10GbE+网络 |
| 队列数过少 | 增加IO队列数 |
| MTU设置不当 | 使用Jumbo Frame |
| CPU负载高 | 绑定专用CPU核心 |

#### RDMA连接问题

| 原因 | 解决方案 |
|------|----------|
| RDMA网卡未配置 | 检查ibv_devinfo |
| 驱动未加载 | modprobe nvme-rdma |
| IP地址配置错误 | 检查RDMA接口IP |

```bash
# 检查RDMA状态
ibv_devinfo
rping -s -a 192.168.100.100 -v  # RDMA ping测试
```

### 7.2 日志查看

```bash
# 内核日志
dmesg | grep nvme

# NVMe-oF日志
journalctl -u nvmet

# 连接日志
nvme list
```

---

## 八、安全建议

### 8.1 网络隔离

- 使用独立网络传输NVMe-oF
- 配置防火墙规则限制访问
- RDMA网络与普通网络分离

### 8.2 访问控制

- 配置Target主机白名单
- 使用复杂NQN标识
- 定期审查连接日志

### 8.3 数据加密

NVMe-oF传输默认不加密。如需加密：

- 使用IPSec加密网络传输
- 在应用层使用加密文件系统
- 网络层配置TLS（需额外支持）

---

## 九、相关文档

| 文档 | 路径 | 说明 |
|------|------|------|
| 路线图 | `docs/roadmap-v2.283.0.md` | NVMe-oF功能规划 |
| 存储架构 | `docs/ARCHITECTURE_NEW_FEATURES.md` | 存储系统架构 |
| 竞品分析 | `docs/COMPETITOR_ANALYSIS.md` | TrueNAS NVMe-oF对比 |

---

## 十、参考命令速查

### Target命令

```bash
nvmecli create-target --nqn=<NQN>              # 创建Target
nvmecli create-namespace --device=<DEV>        # 添加设备
nvmecli create-port --trtype=<TYPE>            # 创建端口
nvmecli bind-target --nqn=<NQN> --port=<PORT>  # 绑定端口
nvmecli list-targets                           # 列出Targets
nvmecli delete-target --nqn=<NQN>              # 删除Target
```

### Initiator命令

```bash
nvme discover -t <transport> -a <address>      # 发现Target
nvme connect -t <transport> -a <address> -n <nqn>  # 连接
nvme list                                      # 列出设备
nvme disconnect -n <nqn>                       # 断开连接
nvme id-ctrl <device>                          # 查看设备信息
nvme smart-log <device>                        # SMART信息
```

---

## 更新记录

| 版本 | 日期 | 更新内容 |
|------|------|----------|
| v1.0 | 2026-03-30 | 礼部创建用户指南 |

---

**文档维护**: 礼部 | **技术支持**: 兵部存储组