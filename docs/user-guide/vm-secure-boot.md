# VM Secure Boot 用户指南

**适用版本**: nas-os v2.463.0+
**更新日期**: 2026-04-29

---

## 1. 什么是 Secure Boot？

Secure Boot（安全启动）是一项安全功能，确保虚拟机只运行经过签名验证的操作系统和驱动程序。它可以防止以下攻击：

- **Bootkit** — 篡改引导加载程序的恶意软件
- **Rootkit** — 植入内核级的隐藏恶意程序
- **内核模块篡改** — 未签名的内核驱动加载
- **固件攻击** — UEFI固件层面的篡改

### 工作原理

Secure Boot 建立一条从固件到操作系统的信任链：

```
UEFI 固件（签名验证）
    → Bootloader（签名验证）
        → OS Kernel（签名验证）
            → 内核模块（签名验证）
```

每一层都验证下一层的数字签名，确保整个启动过程未被篡改。TPM 2.0 模块负责安全存储密钥和验证记录。

---

## 2. 系统要求

| 要求 | 说明 |
|------|------|
| **nas-os版本** | v2.463.0 或更高 |
| **虚拟化引擎** | QEMU/KVM |
| **UEFI固件** | OVMF (EDK II) |
| **TPM** | 软件模拟 TPM 2.0（swtpm） |
| **Guest OS** | Windows 10/11、Ubuntu 20.04+、Fedora 36+、RHEL 8+ |

> **注意**: Secure Boot 需要 UEFI 引导模式，不支持传统 BIOS (SeaBIOS)。创建 VM 时请确保选择 OVMF 固件。

---

## 3. 启用 Secure Boot

### 3.1 新建 VM 时启用

1. 进入 **虚拟机** → **创建虚拟机**
2. 在 **固件类型** 中选择 **UEFI (OVMF)**
3. 勾选 **启用 Secure Boot**
4. 选择安全模式：
   - **strict** — 严格模式，未签名的组件将被阻止启动
   - **standard** — 标准模式，未签名组件会告警但允许启动
   - **audit** — 审计模式，仅记录不阻止（推荐初次使用）
5. 完成其他配置并创建 VM

### 3.2 为已有 VM 启用

1. 进入 **虚拟机** → 选择目标 VM → **设置**
2. 切换到 **安全** 选项卡
3. 开启 **Secure Boot**
4. 选择安全模式
5. 保存并重启 VM

> **重要**: 将 SeaBIOS VM 切换到 UEFI+Secure Boot 前，需要确认 Guest OS 支持 UEFI 引导。不支持的系统可能导致无法启动。

---

## 4. 安全模式详解

| 模式 | 签名验证 | 未签名行为 | 推荐场景 |
|------|----------|------------|----------|
| **strict** | ✅强制 | 🚫阻止启动 | 生产环境、安全合规 |
| **standard** | ✅验证 | ⚠️告警+允许 | 测试环境、过渡期 |
| **audit** | ✅记录 | ✅允许+记录日志 | 首次启用、调试 |

### 选择建议

- **首次启用**: 使用 `audit` 模式运行 1-2 周，确认所有组件兼容后再升级
- **正式环境**: 使用 `strict` 模式获得最高安全保护
- **开发测试**: 使用 `standard` 模式平衡安全和便利

---

## 5. TPM 管理

### 5.1 查看 TPM 状态

```bash
# 通过API查看
curl -s http://localhost:8080/api/v1/vm/{vm-id}/secureboot/status | jq

# 返回示例
{
  "enabled": true,
  "state": "strict",
  "last_boot_secure": true,
  "tpm_present": true,
  "key_count": 3
}
```

### 5.2 TPM 状态说明

| 字段 | 说明 |
|------|------|
| `enabled` | Secure Boot 是否启用 |
| `state` | 当前安全模式 |
| `last_boot_secure` | 上次启动是否通过安全验证 |
| `tpm_present` | TPM 2.0 模块是否就绪 |
| `key_count` | 已注册的签名密钥数量 |

---

## 6. 密钥管理

### 6.1 默认密钥

nas-os 自动管理以下默认密钥：

- **PK (Platform Key)** — 平台根密钥
- **KEK (Key Exchange Key)** — 密钥交换密钥
- **db (Signature Database)** — 签名数据库，存储允许的签名

### 6.2 添加自定义密钥

通过 API 添加自定义签名密钥：

```bash
curl -X POST http://localhost:8080/api/v1/vm/{vm-id}/secureboot/keys \
  -H "Content-Type: application/json" \
  -d '{
    "name": "custom-driver",
    "public_key": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
    "type": "db"
  }'
```

### 6.3 查看已注册密钥

```bash
curl -s http://localhost:8080/api/v1/vm/{vm-id}/secureboot/keys | jq
```

---

## 7. 故障排查

### 7.1 VM 无法启动（strict 模式）

**症状**: 开启 strict 模式后 VM 启动失败

**排查步骤**:

1. 切换到 `audit` 模式启动，查看日志
2. 检查 `/var/log/nas-os/secureboot.log` 中的签名验证失败记录
3. 确认 Guest OS 的 bootloader 和内核已正确签名
4. 如需自定义驱动，添加对应的签名密钥

```bash
# 查看安全启动日志
tail -100 /var/log/nas-os/secureboot.log | grep "VERIFICATION_FAILED"
```

### 7.2 上次启动不安全

**症状**: `last_boot_secure` 显示 `false`

**排查步骤**:

1. 检查是否有内核更新替换了未签名的内核
2. 确认第三方驱动模块是否已签名
3. 查看 audit 日志定位具体失败组件

### 7.3 TPM 模块未就绪

**症状**: `tpm_present` 显示 `false`

**排查步骤**:

1. 确认 swtpm 服务已启动
2. 检查 QEMU 版本是否支持 TPM passthrough
3. 重启虚拟化服务

```bash
# 检查swtpm状态
systemctl status swtpm

# 重启虚拟化服务
systemctl restart nas-virt
```

---

## 8. 最佳实践

### 8.1 部署建议

1. **渐进式启用**: audit → standard → strict，逐步提升安全级别
2. **密钥备份**: 定期导出 Secure Boot 密钥，防止数据丢失
3. **更新策略**: 内核更新前在 audit 模式验证兼容性
4. **日志监控**: 配置 Secure Boot 事件告警

### 8.2 安全合规

- 金融/医疗行业建议使用 strict 模式
- 配合 nas-os 的 WriteOnce 功能，保护 VM 配置不被篡改
- 定期审计 Secure Boot 日志，保留至少 90 天

### 8.3 性能影响

Secure Boot 对 VM 性能影响极小：

| 场景 | 影响 |
|------|------|
| 启动时间 | +2-5秒（签名验证） |
| 运行时性能 | 无影响 |
| 内存占用 | +16MB（TPM模拟） |

---

## 9. API 参考

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/vm/{id}/secureboot/status` | GET | 查看安全启动状态 |
| `/api/v1/vm/{id}/secureboot/config` | PUT | 更新安全启动配置 |
| `/api/v1/vm/{id}/secureboot/keys` | GET | 列出已注册密钥 |
| `/api/v1/vm/{id}/secureboot/keys` | POST | 添加签名密钥 |
| `/api/v1/vm/{id}/secureboot/keys/{key}` | DELETE | 删除签名密钥 |
| `/api/v1/vm/{id}/secureboot/log` | GET | 查看安全启动日志 |

---

## 10. 常见问题

**Q: Secure Boot 会影响已有 VM 的数据吗？**
A: 不会。Secure Boot 只验证启动过程，不修改磁盘数据。

**Q: 可以在运行中的 VM 上切换 Secure Boot 模式吗？**
A: 可以修改配置，但需要重启 VM 才能生效。

**Q: Linux 发行版都支持 Secure Boot 吗？**
A: 主流发行版（Ubuntu、Fedora、RHEL、Debian 11+、SUSE）均内置 Secure Boot 支持。部分小众发行版可能需要手动签名内核。

**Q: Secure Boot 和 nas-os 的 WriteOnce 功能可以同时使用吗？**
A: 可以。WriteOnce 保护数据不可变，Secure Boot 保护启动链完整，两者互补。

---

*刑部出品 | nas-os v2.463.0*
