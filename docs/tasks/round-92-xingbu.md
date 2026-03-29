# 第92轮刑部任务 - 勒索防护增强

## 背景
增强安全防护能力，对标企业级NAS安全特性。

## 任务要求

### 1. 熵值分析优化
- 文件: `internal/security/ransomware/entropy.go`
- 优化大文件流式计算
- 多线程并行分析
- 缓存已分析结果

### 2. 快速变更追踪增强
- 文件: `internal/security/ransomware/tracker.go`
- 降低误报率
- 支持白名单路径
- 基线学习模式

### 3. 自动快照保护
- 文件: `internal/security/ransomware/snapshot_protection.go`
- 检测到可疑活动时自动创建快照
- 快照命名规则: `ransomware-shield-{timestamp}`
- 保留策略配置

### 4. 核心功能
```go
type RansomwareShield interface {
    AnalyzeEntropy(path string) (float64, error)
    TrackChanges(path string, threshold ChangeThreshold) error
    CreateProtectionSnapshot(paths []string) error
    GetAlerts() ([]SecurityAlert, error)
}
```

## 交付物
- 熵值分析优化代码
- 变更追踪增强
- 自动快照保护实现
- 测试覆盖 >85%