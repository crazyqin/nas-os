# Security Module - NAS-OS 安全加固模块

## 核心功能

1. **防火墙管理**: 端口规则/IP 黑白名单/地理位置限制
2. **失败登录保护**: 自动封禁 IP/账户锁定
3. **双因素认证**: TOTP/短信/邮件验证
4. **安全审计**: 登录日志/操作日志/异常检测
5. **文件加密**: 加密文件夹/加密共享
6. **安全基线**: 安全配置检查/漏洞扫描

## 目录结构

```
security/
├── firewall.go        # 防火墙管理
├── fail2ban.go        # 失败登录保护
├── audit.go           # 安全审计日志
├── baseline.go        # 安全基线检查
├── encryption.go      # 文件加密
├── handlers.go        # HTTP 处理器
├── types.go           # 类型定义
└── manager.go         # 安全管理器
```

## API 端点

### 防火墙
- `GET /api/security/firewall/rules` - 获取防火墙规则
- `POST /api/security/firewall/rules` - 添加防火墙规则
- `DELETE /api/security/firewall/rules/:id` - 删除防火墙规则
- `GET /api/security/firewall/ipblacklist` - 获取 IP 黑名单
- `POST /api/security/firewall/ipblacklist` - 添加 IP 到黑名单
- `DELETE /api/security/firewall/ipblacklist/:ip` - 从黑名单移除 IP

### 失败登录保护
- `GET /api/security/fail2ban/status` - 获取状态
- `POST /api/security/fail2ban/config` - 更新配置
- `GET /api/security/fail2ban/banned` - 获取被封禁的 IP
- `POST /api/security/fail2ban/unban/:ip` - 解封 IP

### 安全审计
- `GET /api/security/audit/logs` - 获取审计日志
- `GET /api/security/audit/login-logs` - 获取登录日志
- `GET /api/security/audit/alerts` - 获取安全告警

### 安全基线
- `GET /api/security/baseline/check` - 执行基线检查
- `GET /api/security/baseline/report` - 获取检查报告
