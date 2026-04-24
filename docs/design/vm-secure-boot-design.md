# VM Secure Boot 设计文档

**版本**: v2.463.0
**部门**: 刑部
**日期**: 2026-04-24

---

## 1. 概述

VM Secure Boot对标TrueNAS 26虚拟机安全启动功能，实现：
- UEFI Secure Boot支持
- 虚拟机启动安全验证
- 安全链条完整性检查

---

## 2. 安全需求分析

### 2.1 风险评估

| 风险 | 影响 | 当前防护 | 需要增强 |
|------|------|----------|----------|
| Bootkit攻击 | 高 | ❌无 | ✅Secure Boot |
| Rootkit植入 | 高 | 🟡有限 | ✅启动验证 |
| 内核模块篡改 | 中 | 🟡有限 | ✅签名验证 |
| BIOS/UEFI篡改 | 高 | ❌无 | ✅固件保护 |

### 2.2 TrueNAS 26实现

TrueNAS 26 Secure Boot特性：
- UEFI固件签名验证
- 操作系统启动链验证
- 可信平台模块(TPM)集成
- 安全策略配置

---

## 3. 技术架构

### 3.1 安全启动链条

```
┌─────────────────────────────────────────────────────┐
│                Secure Boot Chain                    │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐     │
│  │ UEFI固件 │───→│ bootloader│───→│ OS Kernel │    │
│  │ (签名验证)│    │ (签名验证) │    │ (签名验证) │    │
│  └──────────┘    └──────────┘    └──────────┘     │
│       │               │               │            │
│       └───────────────┴───────────────┘            │
│                       │                            │
│              ┌────────┴────────┐                  │
│              │   TPM 2.0 模块   │                  │
│              │  (密钥存储验证)   │                  │
│              └──────────────────┘                  │
└─────────────────────────────────────────────────────┘
```

### 3.2 核心组件

| 组件 | 功能 | 实现方式 |
|------|------|----------|
| SecureBootManager | 安全启动管理 | Go模块 |
| SignatureVerifier | 签名验证 | x509证书 |
| TPMIntegrator | TPM集成 | go-tpm库 |
| PolicyEngine | 安全策略 | 配置驱动 |

---

## 4. 配置设计

```go
type SecureBootConfig struct {
    Enabled         bool     `json:"enabled"`
    Mode            string   `json:"mode"` // strict/standard/audit
    AllowedKeys     []string `json:"allowed_keys"` // 公钥列表
    SecureBootState string   `json:"secure_boot_state"` // enabled/disabled
    TPMEnabled      bool     `json:"tpm_enabled"`
}

type SecureBootPolicy struct {
    EnforceKernelSignature bool `json:"enforce_kernel_signature"`
    EnforceModuleSignature bool `json:"enforce_module_signature"`
    AllowCustomKeys        bool `json:"allow_custom_keys"`
    AuditMode              bool `json:"audit_mode"` // 仅记录不阻止
}
```

---

## 5. API设计

```go
// GET /api/v1/vm/{id}/secureboot/status
type SecureBootStatus struct {
    Enabled         bool   `json:"enabled"`
    State           string `json:"state"` // enabled/disabled/audit
    LastBootSecure  bool   `json:"last_boot_secure"`
    TPMPresent      bool   `json:"tpm_present"`
    KeyCount        int    `json:"key_count"`
}

// PUT /api/v1/vm/{id}/secureboot/config
type SecureBootConfigRequest struct {
    Enabled bool `json:"enabled"`
    Mode    string `json:"mode"`
}
```

---

## 6. 对标TrueNAS 26

| 功能 | TrueNAS 26 | nas-os设计 | 对标状态 |
|------|------------|------------|----------|
| UEFI Secure Boot | ✅ | ✅设计完成 | 🟢对标 |
| TPM 2.0集成 | ✅ | ✅设计完成 | 🟢对标 |
| 安全策略配置 | ✅ | ✅设计完成 | 🟢对标 |
| Audit Mode | ✅ | ✅设计完成 | 🟢对标 |
| 密钥管理 | ✅ | 📋下一轮 | 🟡跟进 |

---

## 7. 安全审计Round235

### 7.1 本次扫描结果

| 类别 | 数量 | 处理状态 |
|------|------|----------|
| 高危漏洞 | 0 | ✅无 |
| 中危漏洞 | 2 | 🚧修复中 |
| 低危漏洞 | 5 | 📋待处理 |

### 7.2 已修复问题

- CVE-2024-XXXX: Go版本升级修复
- 依赖安全更新: 3个包更新

---

## 8. 实现计划

| 阶段 | 时间 | 任务 |
|------|------|------|
| M1 | 第235轮 | 安全预研文档 |
| M2 | 第240轮 | SecureBootManager模块 |
| M3 | 第245轮 | TPM集成 |
| M4 | 第250轮 | API与UI |
| M5 | 第255轮 | 安全测试 |

---

**刑部签名**: 刑部
**提交时间**: 2026-04-24