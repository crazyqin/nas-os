# VM Secure Boot 设计文档

## 版本：v2.219.0
## 日期：2026-04-11
## 作者：兵部

---

## 一、概述

本文档设计 VM Secure Boot 功能架构，对标 TrueNAS 25.10 新特性。

### 1.1 设计目标

1. **安全启动支持**：为虚拟机提供 UEFI Secure Boot 能力
2. **操作系统兼容**：支持 Windows 11、Linux 等需要 Secure Boot 的操作系统
3. **密钥管理**：提供安全密钥管理机制
4. **易于配置**：用户可通过 UI/API 简单启用/禁用 Secure Boot

### 1.2 技术背景

UEFI Secure Boot 是一种安全标准，确保设备只使用可信的操作系统启动加载程序。在虚拟化环境中，QEMU/libvirt 提供以下支持：

- **OVMF (Open Virtual Machine Firmware)**：开源 UEFI 固件实现
- **Secure Boot 密钥**：PK (Platform Key)、KEK (Key Exchange Key)、db (Authorized Signature Database)、dbx (Forbidden Signature Database)

---

## 二、架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          NAS-OS VM Secure Boot                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌───────────────────┐   │
│  │      Web UI         │  │     REST API        │  │   WebSocket       │   │
│  │   Secure Boot 配置   │  │   /api/vm/boot     │  │   状态推送        │   │
│  └──────────┬──────────┘  └──────────┬──────────┘  └─────────┬─────────┘   │
│             │                        │                       │             │
│             └────────────────────────┼───────────────────────┘             │
│                                      │                                     │
│  ┌───────────────────────────────────┴─────────────────────────────────┐   │
│  │                      Secure Boot Manager                             │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐   │   │
│  │  │ Key Manager   │  │ OVMF Config   │  │ Boot State Tracker   │   │   │
│  │  │ 密钥管理       │  │ 固件配置       │  │ 启动状态跟踪         │   │   │
│  │  └───────────────┘  └───────────────┘  └───────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                     │
│  ┌───────────────────────────────────┴─────────────────────────────────┐   │
│  │                      QEMU/Libvirt Layer                              │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐   │   │
│  │  │ Libvirt Domain│  │ OVMF Firmware │  │ QEMU Command Line    │   │   │
│  │  │ XML 配置       │  │ /usr/share/OVMF│ │ -machine q35         │   │   │
│  │  └───────────────┘  └───────────────┘  └───────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 模块说明

#### 2.2.1 Secure Boot Manager（安全启动管理器）

核心组件，负责：

- 管理虚拟机的 Secure Boot 配置
- 处理密钥导入/导出
- 验证操作系统签名
- 跟踪启动状态

#### 2.2.2 Key Manager（密钥管理器）

管理 Secure Boot 四类密钥：

| 密钥类型 | 说明 | 数量 |
|---------|------|-----|
| PK (Platform Key) | 平台密钥，最高级别信任根 | 1 |
| KEK (Key Exchange Key) | 密钥交换密钥，用于更新 db/dbx | 1-多个 |
| db (Signature Database) | 授权签名数据库，包含可信 OS 签名 | 多个 |
| dbx (Forbidden Signature Database) | 禁止签名数据库，包含被撤销的签名 | 多个 |

#### 2.2.3 OVMF Config（固件配置）

管理 OVMF 固件：

- 使用预设 Secure Boot OVMF 固件
- 或使用自定义密钥的 OVMF 固件
- 支持固件更新和回滚

---

## 三、API 设计

### 3.1 Secure Boot 配置 API

```go
// SecureBootConfig Secure Boot 配置
type SecureBootConfig struct {
    // 是否启用 Secure Boot
    Enabled bool `json:"enabled"`
    
    // 固件模式：uefi 或 legacy
    FirmwareMode string `json:"firmwareMode"`
    
    // Secure Boot 模式
    // - "default": 使用预设 Microsoft 密钥
    // - "custom": 使用用户自定义密钥
    // - "disabled": 禁用 Secure Boot
    Mode string `json:"mode"`
    
    // 自定义密钥配置（仅 custom 模式）
    CustomKeys *CustomKeyConfig `json:"customKeys,omitempty"`
    
    // 是否允许无签名 OS 启动（仅测试用途）
    AllowUnsigned bool `json:"allowUnsigned"`
}

// CustomKeyConfig 自定义密钥配置
type CustomKeyConfig struct {
    // Platform Key (PK)
    PK KeyEntry `json:"pk"`
    
    // Key Exchange Keys (KEK)
    KEK []KeyEntry `json:"kek"`
    
    // Authorized Signatures (db)
    DB []KeyEntry `json:"db"`
    
    // Forbidden Signatures (dbx)
    DBX []KeyEntry `json:"dbx"`
}

// KeyEntry 密钥条目
type KeyEntry struct {
    // 密钥名称
    Name string `json:"name"`
    
    // 密钥类型：x509 或 sha256
    Type string `json:"type"`
    
    // 密钥数据（Base64 编码）
    Data string `json:"data"`
    
    // 密钥来源
    Source string `json:"source"`
}

// SecureBootState Secure Boot 状态
type SecureBootState struct {
    // VM ID
    VMID string `json:"vmId"`
    
    // Secure Boot 是否启用
    Enabled bool `json:"enabled"`
    
    // 当前模式
    Mode string `json:"mode"`
    
    // 启动验证状态
    // - "verified": 通过验证，正常启动
    // - "failed": 验证失败，启动被阻止
    // - "pending": 待验证
    // - "unknown": 状态未知
    VerificationStatus string `json:"verificationStatus"`
    
    // 最后验证时间
    LastVerified *time.Time `json:"lastVerified,omitempty"`
    
    // 验证失败的操作系统信息
    FailedOSInfo *FailedOSInfo `json:"failedOsInfo,omitempty"`
    
    // 当前加载的密钥信息
    LoadedKeys LoadedKeysInfo `json:"loadedKeys"`
}

// FailedOSInfo 验证失败的操作系统信息
type FailedOSInfo struct {
    OSName string    `json:"osName"`
    FailedAt time.Time `json:"failedAt"`
    Reason  string    `json:"reason"`
}

// LoadedKeysInfo 已加载密钥信息
type LoadedKeysInfo struct {
    PK  KeySummary   `json:"pk"`
    KEK []KeySummary `json:"kek"`
    DB  []KeySummary `json:"db"`
    DBX []KeySummary `json:"dbx"`
}

// KeySummary 密钥摘要
type KeySummary struct {
    Name      string    `json:"name"`
    Owner     string    `json:"owner"`
    AddedAt   time.Time `json:"addedAt"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}
```

### 3.2 REST API 接口

| 接口 | 方法 | 说明 |
|-----|------|------|
| `/api/v1/vm/{vmId}/secureboot` | GET | 获取 VM Secure Boot 配置 |
| `/api/v1/vm/{vmId}/secureboot` | PUT | 更新 VM Secure Boot 配置 |
| `/api/v1/vm/{vmId}/secureboot/state` | GET | 获取 VM Secure Boot 状态 |
| `/api/v1/secureboot/keys` | GET | 列出系统可用密钥 |
| `/api/v1/secureboot/keys` | POST | 导入自定义密钥 |
| `/api/v1/secureboot/keys/{keyId}` | DELETE | 删除密钥 |
| `/api/v1/secureboot/firmware` | GET | 获取可用固件列表 |

---

## 四、QEMU 配置

### 4.1 Libvirt Domain XML 配置

启用 Secure Boot 的虚拟机配置示例：

```xml
<domain type='kvm'>
  <name>secure-vm</name>
  <os>
    <type arch='x86_64' machine='q35'>hvm</type>
    <loader readonly='yes' secure='yes' type='pflash'>
      /usr/share/OVMF/OVMF_CODE.secure.fd
    </loader>
    <nvram template='/usr/share/OVMF/OVMF_VARS.fd'>
      /var/lib/libvirt/nvram/secure-vm_VARS.fd
    </nvram>
  </os>
  <features>
    <acpi/>
    <apic/>
    <smm state='on'/>  <!-- SMM 必需 for Secure Boot -->
  </features>
  <!-- 其他配置 -->
</domain>
```

### 4.2 关键配置说明

| 配置项 | 说明 | 要求 |
|-------|------|-----|
| `machine='q35'` | 使用 Q35 机器类型 | Secure Boot 要求 |
| `<smm state='on'/>` | 启用 SMM (System Management Mode) | Secure Boot 必需 |
| `<loader secure='yes'>` | 标记固件为 Secure Boot 模式 | 必需 |
| OVMF_CODE.secure.fd | Secure Boot 固件代码 | 包含 Microsoft 密钥 |
| OVMF_VARS.fd | 变量存储模板 | 用于存储自定义密钥 |

### 4.3 QEMU 命令行参数

直接使用 QEMU 启动 Secure Boot VM：

```bash
qemu-system-x86_64 \
  -machine q35,accel=kvm,smm=on \
  -cpu host \
  -m 4096 \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE.secure.fd \
  -drive if=pflash,format=raw,file=/var/lib/libvirt/nvram/vm_VARS.fd \
  -drive file=/var/lib/libvirt/images/vm.qcow2,format=qcow2 \
  -netdev user,id=net0 -device virtio-net-pci,netdev=net0 \
  -vnc :0
```

---

## 五、密钥管理设计

### 5.1 密钥存储

```
/var/lib/nas-os/secureboot/
├── keys/
│   ├── pk/
│   │   └── default.pk          # 默认平台密钥
│   │   └── custom-{vmid}.pk    # VM 自定义密钥
│   ├── kek/
│   │   ├── microsoft.kek       # Microsoft KEK
│   │   └── custom-{vmid}.kek   # 自定义 KEK
│   ├── db/
│   │   ├── microsoft.db        # Microsoft 授权签名
│   │   ├── linux.db            # Linux 发行版签名
│   │   └── custom-{vmid}.db    # 自定义签名
│   └── dbx/
│       ├── revoked.dbx         # 已撤销签名列表
│       └── custom-{vmid}.dbx   # 自定义撤销列表
├── firmware/
│   ├── OVMF_CODE.secure.fd     # Secure Boot 固件
│   ├── OVMF_CODE.fd            # 标准 UEFI 固件
│   └── OVMF_VARS.fd            # 变量模板
└── nvram/
    ├── {vmid}_VARS.fd          # VM NVRAM 存储
    └── backups/
        └── {vmid}_VARS.backup.fd
```

### 5.2 密钥导入流程

```
┌──────────────┐
│ 用户上传密钥  │
└───────┬──────┘
        │
        ▼
┌──────────────┐
│ 验证密钥格式  │  ──► 失败：返回错误
└───────┬──────┘
        │ 成功
        ▼
┌──────────────┐
│ 解析密钥内容  │  (X509/SHA256)
└───────┬──────┘
        │
        ▼
┌──────────────┐
│ 存储密钥文件  │
└───────┬──────┘
        │
        ▼
┌──────────────┐
│ 生成 NVRAM   │  (更新 OVMF_VARS)
└───────┬──────┘
        │
        ▼
┌──────────────┐
│ 更新 VM 配置  │
└──────────────┘
```

### 5.3 密钥类型支持

| 类型 | 格式 | 说明 |
|-----|------|------|
| X.509 | DER/PEM | 标准证书格式 |
| SHA256 | Hash | 直接使用哈希签名 |
| EFI Signature List | .esl | UEFI 标准签名列表格式 |

---

## 六、操作系统兼容性

### 6.1 支持的操作系统

| 操作系统 | Secure Boot 支持 | 推荐配置 |
|---------|----------------|---------|
| Windows 11 | 强制要求 | 默认 Microsoft 密钥 |
| Windows 10 | 可选 | 默认 Microsoft 密钥 |
| Ubuntu 22.04+ | 支持 | 默认 Microsoft 密钥（包含 Ubuntu 签名） |
| Fedora | 支持 | 默认 Microsoft 密钥（包含 Fedora 签名） |
| RHEL/CentOS 9 | 支持 | 默认 Microsoft 密钥（包含 RedHat 签名） |
| Arch Linux | 需自定义签名 | 自定义密钥模式 |
| FreeBSD | 不支持 | 禁用 Secure Boot |

### 6.2 Linux Secure Boot 支持

Linux 发行版通过以下方式支持 Secure Boot：

1. ** shim 加载器**：第一阶段加载器，由 Microsoft 签名
2. **GRUB/MOK**：第二阶段加载器，使用 Machine Owner Key (MOK)
3. **内核签名**：内核模块可能需要签名

用户可使用 `mokutil` 工具管理 MOK：

```bash
# 查看 Secure Boot 状态
mokutil --sb-state

# 导入自定义 MOK
mokutil --import custom.der

# 删除 MOK
mokutil --delete custom.der
```

---

## 七、安全考虑

### 7.1 安全威胁分析

| 威胁 | 风险等级 | 缓解措施 |
|-----|---------|---------|
| 恶意操作系统启动 | 高 | Secure Boot 验证阻止 |
| 密钥泄露 | 高 | 密钥加密存储、访问控制 |
| 固件篡改 | 中 | 固件完整性校验 |
| VM 配置篡改 | 中 | 配置访问控制、审计日志 |
| 回滚攻击 | 中 | 固件版本控制、dbx 更新 |

### 7.2 安全策略

1. **密钥存储加密**：使用 AES-256 加密密钥文件
2. **访问控制**：密钥管理仅限管理员角色
3. **审计日志**：记录所有密钥操作
4. **固件校验**：启动前校验固件完整性
5. **最小权限**：VM 仅加载所需密钥

### 7.3 合规性

Secure Boot 设计符合：

- **UEFI 2.3.1 规范**
- **NIST SP 800-147**：BIOS 保护指南
- **TCG PC Client Platform TPM Profile (PTP)**

---

## 八、实现计划

### 8.1 Phase 1：基础支持（P0）

- [ ] OVMF 固件安装和配置
- [ ] Libvirt Domain XML Secure Boot 配置
- [ ] VM 配置 API（启用/禁用 Secure Boot）
- [ ] 默认 Microsoft 密钥模式

### 8.2 Phase 2：自定义密钥（P1）

- [ ] 自定义密钥导入 API
- [ ] 密钥管理 UI
- [ ] 密钥加密存储
- [ ] KEK/db/dbx 管理

### 8.3 Phase 3：高级功能（P2）

- [ ] 启动状态监控和告警
- [ ] 密钥轮换机制
- [ ] 批量 VM Secure Boot 配置
- [ ] 与 TPM 集成

### 8.4 依赖条件

```bash
# 安装 OVMF 固件（Ubuntu/Debian）
apt-get install ovmf

# 安装 OVMF 固件（RHEL/CentOS/Fedora）
dnf install edk2-ovmf

# 安装密钥管理工具
apt-get install efitools
```

---

## 九、测试计划

### 9.1 功能测试

| 测试场景 | 验证点 |
|---------|-------|
| 启用 Secure Boot 启动 Windows 11 | 正常启动 |
| 启用 Secure Boot 启动 Ubuntu | 正常启动 |
| 启用 Secure Boot 启动未签名 OS | 启动被阻止 |
| 禁用 Secure Boot 启动任意 OS | 正常启动 |
| 自定义密钥导入 | 密钥正确加载 |
| 密钥删除 | 配置正确更新 |
| VM 迁移后 Secure Boot | 配置保留 |

### 9.2 安全测试

| 测试场景 | 验证点 |
|---------|-------|
| 恶意固件替换 | 启动失败或告警 |
| 密钥未授权访问 | 访问被拒绝 |
| 配置篡改 | 审计日志记录 |
| VM 快照回滚 | Secure Boot 状态一致 |

---

## 十、参考文档

1. [UEFI Secure Boot Specification](https://uefi.org/specifications)
2. [QEMU OVMF Documentation](https://github.com/tianocore/tianocore.github.io/wiki/OVMF)
3. [Libvirt Secure Boot Configuration](https://libvirt.org/formatdomain.html#elementsOSBIOS)
4. [Microsoft Secure Boot Key Management](https://docs.microsoft.com/en-us/windows-hardware/design/device-experiences/oem-secure-boot)
5. [Linux Secure Boot Implementation](https://ubuntu.com/blog/how-ubuntu-secure-boot-works)

---

## 十一、总结

本设计文档规划了 NAS-OS VM Secure Boot 功能的完整架构：

1. **架构清晰**：分层设计，易于扩展
2. **API 完整**：支持配置管理、密钥导入、状态查询
3. **兼容性好**：支持主流操作系统
4. **安全可靠**：密钥加密存储、访问控制、审计日志

后续开发应遵循分阶段实施计划，优先完成 P0 基础支持功能。