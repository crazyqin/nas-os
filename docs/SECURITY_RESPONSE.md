# 安全响应流程

## 概述
本文档定义 NAS-OS 项目的安全事件响应流程，确保及时发现、评估和修复安全漏洞。

## 安全扫描工具

### 1. 代码安全扫描 (Gosec)
```bash
# 安装
go install github.com/securego/gosec/v2/cmd/gosec@latest

# 运行扫描
gosec -fmt=text ./...
gosec -fmt=sarif -out=gosec-report.sarif ./...
```

### 2. 依赖漏洞检查 (govulncheck)
```bash
# 安装
go install golang.org/x/vuln/cmd/govulncheck@latest

# 运行检查
govulncheck ./...
```

## 漏洞分级

| 等级 | 响应时间 | 说明 |
|------|----------|------|
| 🔴 高危 | 24 小时内 | 路径遍历、命令注入、认证绕过 |
| 🟡 中危 | 7 天内 | 弱加密、信息泄露、配置问题 |
| 🟢 低危 | 30 天内 | 代码质量、最佳实践偏离 |

## 当前发现的问题

### 代码安全问题 (Gosec)

1. **G703 - 路径遍历风险** (高危)
   - 位置：`internal/backup/manager.go:648`
   - 问题：`os.WriteFile` 使用未清理的路径
   - 修复：添加路径验证和清理

2. **G122 - 符号链接 TOCTOU 风险** (高危)
   - 位置：`internal/backup/manager.go:644`
   - 问题：`filepath.Walk` 回调中使用竞争敏感路径
   - 修复：使用 `os.Root` 等安全 API

3. **G401 - 弱加密原语** (中危)
   - 位置：`internal/network/ddns_providers.go:559`
   - 问题：使用 SHA1
   - 修复：升级为 SHA256 或 SHA3

4. **G204 - 命令注入风险** (中危)
   - 位置：`pkg/btrfs/btrfs.go` (多处)
   - 问题：`exec.Command` 使用变量参数
   - 修复：参数验证和清理

### 依赖漏洞 (govulncheck)

1. **GO-2026-4601** - net/url IPv6 解析问题
   - 修复：升级 Go 至 1.26.1+

2. **GO-2026-4600** - crypto/x509 证书验证 panic
   - 修复：升级 Go 至 1.26.1+

3. **GO-2026-4599** - crypto/x509 邮箱约束问题
   - 修复：升级 Go 至 1.26.1+

## 响应流程

### 发现阶段
1. 定期运行安全扫描（建议每周）
2. CI/CD 集成自动扫描
3. 接收外部安全报告

### 评估阶段
1. 确认漏洞真实性
2. 评估影响范围
3. 确定优先级

### 修复阶段
1. 创建修复分支
2. 实施修复
3. 添加测试用例
4. 代码审查

### 验证阶段
1. 重新运行安全扫描
2. 回归测试
3. 发布安全公告

### 发布阶段
1. 创建安全补丁版本
2. 更新依赖
3. 通知用户

## 自动化通知

使用 `scripts/security-check.sh` 脚本进行定期扫描和通知：

```bash
#!/bin/bash
# 运行安全检查并发送通知
./scripts/security-check.sh --notify
```

## 联系方式

发现安全漏洞请报告：
- GitHub Issues (敏感问题使用 Private 报告)
- 邮件：security@example.com

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-03-11 | 初始版本 |
