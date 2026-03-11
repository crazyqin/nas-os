# 定时安全检查设置

## 概述
配置定时安全检查，自动扫描代码和依赖漏洞，并发送通知。

## 快速开始

### 1. 设置执行权限
```bash
cd /home/mrafter/clawd/nas-os
chmod +x scripts/security-check.sh scripts/security-cron.sh
```

### 2. 测试运行
```bash
./scripts/security-check.sh --notify
```

### 3. 配置 Cron

编辑 crontab：
```bash
crontab -e
```

添加以下条目（每周一上午 9 点运行）：
```cron
# 每周一 9:00 运行安全检查
0 9 * * 1 /home/mrafter/clawd/nas-os/scripts/security-cron.sh >> /home/mrafter/clawd/nas-os/logs/security-cron.log 2>&1
```

### 4. 可选：每日检查
```cron
# 每日 9:00 运行快速检查
0 9 * * * /home/mrafter/clawd/nas-os/scripts/security-cron.sh >> /home/mrafter/clawd/nas-os/logs/security-cron.log 2>&1
```

## 通知配置

### Discord Webhook
在 `security-cron.sh` 中添加：
```bash
DISCORD_WEBHOOK="your-webhook-url"
curl -X POST "$DISCORD_WEBHOOK" \
  -H "Content-Type: application/json" \
  -d "{\"content\": \"安全检查完成：$(cat $SUMMARY_FILE)\"}"
```

### 邮件通知
```bash
# 需要配置 mail 命令
mail -s "安全检查报告" admin@example.com < "$SUMMARY_FILE"
```

## 日志查看

```bash
# 查看最新日志
tail -f /home/mrafter/clawd/nas-os/logs/security-check.log

# 查看历史报告
ls -la /home/mrafter/clawd/nas-os/reports/security/
```

## 报告位置

所有安全报告保存在：
```
/home/mrafter/clawd/nas-os/reports/security/
├── gosec_YYYYMMDD_HHMMSS.txt
├── gosec_YYYYMMDD_HHMMSS.sarif
├── vulncheck_YYYYMMDD_HHMMSS.txt
└── security_summary_YYYYMMDD_HHMMSS.md
```

## 故障排除

### 工具未找到
```bash
# 安装 gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# 安装 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
```

### 权限问题
```bash
chmod +x scripts/*.sh
```

### 查看 cron 日志
```bash
# Ubuntu/Debian
grep CRON /var/log/syslog

# CentOS/RHEL
grep CRON /var/log/cron
```
