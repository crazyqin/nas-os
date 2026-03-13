# iSCSI 目标使用指南

**版本**: v2.2.0  
**更新日期**: 2026-03-21

---

## 📖 概述

iSCSI (Internet Small Computer System Interface) 是一种基于 IP 网络的存储协议，允许服务器通过 TCP/IP 网络访问块级存储设备。NAS-OS 提供完整的 iSCSI Target 功能，适用于虚拟化环境、数据库存储和高性能应用场景。

### 特性

- ✅ 标准兼容的 iSCSI Target 实现
- ✅ LUN (Logical Unit Number) 管理
- ✅ CHAP 认证支持
- ✅ IQN 自动生成和管理
- ✅ Initiator 访问控制列表
- ✅ 连接会话监控
- ✅ WebUI 管理界面

---

## 🚀 快速开始

### 前提条件

1. Linux 内核支持 `target_core_user` 模块
2. 安装 `targetcli` 工具包
3. NAS-OS v2.2.0+

```bash
# 检查内核模块
lsmod | grep target_core

# 安装 targetcli（如未安装）
sudo apt install targetcli-fb
```

### 创建第一个 iSCSI 目标

#### 方式一：WebUI

1. 登录 NAS-OS Web 管理界面
2. 导航到 **存储** → **iSCSI**
3. 点击 **创建目标**
4. 填写配置信息：
   - 目标名称：`target1`
   - LUN 编号：`0`
   - 存储路径：`/data/iscsi/lun0.img`
   - 容量大小：`100 GB`
5. 点击 **创建**

#### 方式二：CLI

```bash
# 创建 iSCSI 目标
nasctl iscsi create \
  --name target1 \
  --lun 0 \
  --path /data/iscsi/lun0.img \
  --size 100G
```

#### 方式三：API

```bash
curl -X POST http://localhost:8080/api/v1/iscsi/targets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "target1",
    "lun": 0,
    "path": "/data/iscsi/lun0.img",
    "size": 107374182400
  }'
```

---

## 📋 详细配置

### 目标配置参数

| 参数 | 说明 | 必填 | 默认值 |
|------|------|------|--------|
| `name` | 目标名称 | 是 | - |
| `iqn` | IQN 标识符 | 否 | 自动生成 |
| `lun` | LUN 编号 | 是 | - |
| `path` | 存储文件路径 | 是 | - |
| `size` | 容量（字节） | 是 | - |
| `chap_user` | CHAP 用户名 | 否 | - |
| `chap_password` | CHAP 密码 | 否 | - |
| `allowed_initiators` | 允许的 Initiator IQN 列表 | 否 | 全部允许 |

### IQN 命名规范

IQN 格式：`iqn.yyyy-mm.reverse-domain-name:identifier`

示例：
```
iqn.2026-03.com.nas-os:target1
iqn.2026-03.com.nas-os:vm-storage
```

NAS-OS 自动生成 IQN，也可手动指定。

---

## 🔐 安全配置

### CHAP 认证

启用 CHAP (Challenge-Handshake Authentication Protocol) 认证增强安全性。

```bash
# 创建带 CHAP 认证的目标
nasctl iscsi create \
  --name secure-target \
  --lun 0 \
  --path /data/iscsi/secure.img \
  --size 200G \
  --chap-user admin \
  --chap-password SecurePass123
```

### Initiator 访问控制

限制允许连接的 Initiator 列表：

```bash
curl -X PUT http://localhost:8080/api/v1/iscsi/targets/target1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "allowed_initiators": [
      "iqn.2026-03.com.vmware:esxi-host1",
      "iqn.2026-03.com.vmware:esxi-host2"
    ]
  }'
```

---

## 📊 监控与管理

### 查看目标状态

```bash
# CLI
nasctl iscsi list
nasctl iscsi show target1

# API
curl http://localhost:8080/api/v1/iscsi/targets \
  -H "Authorization: Bearer $TOKEN"
```

### 监控连接会话

```bash
# 查看活跃会话
curl http://localhost:8080/api/v1/iscsi/targets/target1/sessions \
  -H "Authorization: Bearer $TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "data": [
    {
      "session_id": "sid-001",
      "initiator_iqn": "iqn.2026-03.com.vmware:esxi-host1",
      "connected_at": "2026-03-21T10:00:00Z",
      "bytes_read": 1073741824,
      "bytes_written": 2147483648,
      "io_operations": 5000
    }
  ]
}
```

---

## 🖥️ Initiator 配置

### Linux (open-iscsi)

```bash
# 安装 open-iscsi
sudo apt install open-iscsi

# 发现目标
sudo iscsiadm -m discovery -t st -p <NAS_IP>:3260

# 登录目标
sudo iscsiadm -m node -T iqn.2026-03.com.nas-os:target1 -p <NAS_IP>:3260 -l

# 查看已连接的磁盘
lsblk
```

### VMware ESXi

1. 导航到 **存储** → **存储适配器**
2. 选择 **iSCSI 软件适配器**
3. 点击 **属性** → **动态发现**
4. 添加 iSCSI 服务器地址
5. 扫描新的存储设备

### Windows Server

1. 打开 **iSCSI 发起程序**
2. 在 **发现** 选项卡添加目标门户
3. 在 **目标** 选项卡连接目标
4. 在 **磁盘管理** 中初始化新磁盘

---

## 🔄 日常维护

### 扩容 LUN

```bash
# 扩容到 200GB
curl -X PUT http://localhost:8080/api/v1/iscsi/targets/target1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"size": 214748364800}'
```

> ⚠️ 注意：扩容后需要在 Initiator 端扩展文件系统。

### 删除目标

```bash
# 确保无活跃连接
nasctl iscsi show target1

# 删除目标
nasctl iscsi delete target1
```

---

## ⚠️ 注意事项

1. **性能考虑**: iSCSI 性能受网络延迟影响，建议使用万兆网络
2. **冗余配置**: 重要数据建议配置多路径 (MPIO)
3. **安全性**: 生产环境强烈建议启用 CHAP 认证
4. **备份**: iSCSI LUN 数据需要定期备份

---

## 🔧 故障排查

### 目标无法连接

```bash
# 检查服务状态
systemctl status nas-os

# 检查端口监听
netstat -tlnp | grep 3260

# 查看日志
journalctl -u nas-os -f
```

### Initiator 无法发现目标

1. 检查防火墙是否允许 3260 端口
2. 确认 iSCSI Target 服务已启动
3. 验证网络连通性

### 性能问题

1. 检查网络带宽利用率
2. 检查存储后端性能
3. 考虑启用 Jumbo Frame

---

## 📚 相关文档

- [API 文档 - iSCSI 模块](API_GUIDE.md#iscsi)
- [快照策略配置指南](SNAPSHOT_POLICY_GUIDE.md)
- [性能监控配置指南](PERFORMANCE_MONITORING_GUIDE.md)

---

**最后更新**: 2026-03-21  
**维护团队**: NAS-OS 吏部