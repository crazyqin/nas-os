# SensitiveMask - 敏感信息保护模块

参考群晖DSM 7.3 AI Console的去敏感机制设计，为NAS-OS项目提供敏感信息检测和脱敏功能。

## 功能特性

- **敏感信息检测**：自动检测手机号、身份证、银行卡、邮箱、护照等敏感数据
- **数据脱敏**：支持多种脱敏策略（部分脱敏、完全脱敏、哈希脱敏等）
- **策略管理**：可配置的检测和脱敏策略
- **审计日志**：完整的操作审计记录
- **云服务保护**：在数据发送到云AI服务前自动处理敏感信息

## 支持的敏感信息类型

| 类型 | 说明 | 风险等级 |
|------|------|---------|
| `TypePhoneNumber` | 中国手机号 | 中等 |
| `TypeIDCard` | 中国身份证号 | 高 |
| `TypeBankCard` | 银行卡号 | 高 |
| `TypeEmail` | 邮箱地址 | 中等 |
| `TypePassport` | 护照号码 | 高 |
| `TypeCreditCard` | 信用卡号 | 高 |
| `TypeAPIKey` | API密钥 | 严重 |
| `TypePassword` | 密码相关 | 严重 |
| `TypeIPv4` | IPv4地址 | 低 |

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "nas-os/internal/security/sensitivemask"
)

func main() {
    // 快速检测
    text := "用户手机号：13812345678，邮箱：test@example.com"
    matches := sensitivemask.QuickDetect(text)
    for _, m := range matches {
        fmt.Printf("检测到 %s: %s\n", m.Type, m.Value)
    }

    // 快速脱敏
    masked, _ := sensitivemask.QuickMask(text)
    fmt.Println("脱敏后:", masked)
    // 输出: 用户手机号：138****5678，邮箱：tes*****@example.com

    // 检查是否包含敏感信息
    if sensitivemask.HasSensitive(text) {
        fmt.Println("文本包含敏感信息")
    }
}
```

### 使用ServiceGuard保护云AI服务

```go
// 创建ServiceGuard
guard := sensitivemask.NewServiceGuard(sensitivemask.ServiceGuardConfig{
    EnableAudit: true,
})

// 注册云AI服务
guard.RegisterService(sensitivemask.ServiceConfig{
    Name:    "openai-api",
    Enabled: true,
    PolicyID: "default",
})

// 处理发送到云服务的数据
ctx := context.Background()
result, err := guard.ProcessData(ctx, "openai-api", userInput, userID)
if err != nil {
    // 处理错误
}

if result.Blocked {
    // 数据被阻止传输
    fmt.Println("阻止原因:", result.BlockReason)
} else {
    // 使用脱敏后的数据
    safeText := result.ProcessedText
    // 发送到云AI服务...
}
```

### 自定义策略

```go
// 创建策略管理器
pm := sensitivemask.NewPolicyManager("/path/to/storage")

// 创建自定义策略
policy, _ := pm.CreatePolicy(
    "strict-policy",
    "严格模式策略",
    sensitivemask.DetectorConfig{
        EnabledTypes: map[sensitivemask.SensitiveType]bool{
            sensitivemask.TypePhoneNumber: true,
            sensitivemask.TypeIDCard:     true,
            sensitivemask.TypeEmail:      true,
        },
        MinConfidence: 0.95,
        CheckChecksum: true,
    },
    sensitivemask.MaskerConfig{
        Strategies: map[sensitivemask.SensitiveType]sensitivemask.MaskStrategy{
            sensitivemask.TypePhoneNumber: sensitivemask.MaskStrategyFull,
            sensitivemask.TypeIDCard:      sensitivemask.MaskStrategyFull,
        },
        DefaultMask: "*",
    },
    sensitivemask.PolicyActions{
        BlockTransmission: true,
        MaskBeforeSend:    true,
        LogDetection:      true,
        AlertOnHighRisk:   true,
    },
)

// 设为活动策略
pm.SetActivePolicy(policy.ID)
```

## 脱敏策略说明

| 策略 | 说明 | 示例 |
|------|------|------|
| `MaskStrategyNone` | 不脱敏 | 13812345678 |
| `MaskStrategyPartial` | 部分脱敏 | 138****5678 |
| `MaskStrategyFull` | 完全脱敏 | *********** |
| `MaskStrategyHash` | 哈希脱敏 | HASH:a1b2c3d4e5f6 |
| `MaskStrategyRemove` | 移除 | [REMOVED] |

## 身份证校验

本模块实现了中国身份证18位校验码验证：

```go
// 有效身份证号示例（校验码正确）
validID := "110105199003070009"  // 北京朝阳区，1990年3月7日

// 校验码计算规则：
// 前17位加权求和，权重为 [7,9,10,5,8,4,2,1,6,3,7,9,10,5,8,4,2]
// 余数对照表：0-1, 1-0, 2-X, 3-9, 4-8, 5-7, 6-6, 7-5, 8-4, 9-3, 10-2
```

## 银行卡校验

使用Luhn算法验证银行卡号：

```go
// 有效银行卡号示例（通过Luhn校验）
validCard := "6222021234567894"  // 银联卡号
```

## 风险等级

| 等级 | 说明 | 处理建议 |
|------|------|---------|
| `RiskLevelLow` | 低风险 | 可部分显示 |
| `RiskLevelMedium` | 中风险 | 需完全脱敏 |
| `RiskLevelHigh` | 高风险 | 必须阻止传输 |
| `RiskLevelCritical` | 严重 | 立即告警 |

## 审计日志

```go
// 获取审计日志
logs := guard.GetAuditLogger().GetLogs(sensitivemask.AuditFilter{
    StartTime:   &startTime,
    EndTime:     &endTime,
    OnlyBlocked: true,  // 只查看被阻止的操作
})

for _, log := range logs {
    fmt.Printf("[%s] 用户 %s 尝试向 %s 发送敏感数据\n",
        log.Timestamp, log.UserID, log.ServiceName)
}
```

## 设计参考

本模块设计参考了群晖DSM 7.3 AI Console的去敏感机制：

1. **自动检测**：在数据发送到云算力服务商前自动标记敏感信息
2. **智能脱敏**：根据敏感程度选择合适的脱敏策略
3. **审计追踪**：记录所有敏感数据处理操作
4. **策略配置**：支持灵活的策略配置满足不同场景需求

## 测试

```bash
# 运行测试
go test -v ./internal/security/sensitivemask/...

# 运行基准测试
go test -bench=. ./internal/security/sensitivemask/...
```

## 许可证

MIT License