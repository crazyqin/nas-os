# NAS-OS v2.44.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable (稳定版)

---

## 本次更新

### 🧪 测试修复

- 修复 `internal/monitor/alerting_test.go` 语法错误
- 告警规则测试用例优化，提升测试稳定性

### 📚 文档完善

- **用户快速入门指南** - 更新至 v2.44.0
- **API 使用示例文档** - 完善示例代码，更新下载链接
- **常见问题 FAQ** - 新增 8 个实用 FAQ：
  - Q6: 如何启用双重认证 (MFA)
  - Q7: 如何配置定时快照
  - Q8: 如何扩展存储卷
  - Q9: 如何迁移数据到新 NAS
  - Q10: 如何查看系统日志
  - Q11: 忘记管理员密码怎么办
  - Q12: 如何配置邮件告警
- 所有文档版本号统一更新至 v2.44.0

---

## 升级指南

### 从 v2.43.0 升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.44.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os
```

### Docker 升级

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.44.0

# 重启容器
docker-compose down
docker-compose up -d
```

---

## 下载

| 平台 | 文件 | 校验和 |
|------|------|--------|
| Linux AMD64 | nasd-linux-amd64 | SHA256 |
| Linux ARM64 | nasd-linux-arm64 | SHA256 |
| Linux ARMv7 | nasd-linux-armv7 | SHA256 |
| Docker | ghcr.io/crazyqin/nas-os:v2.44.0 | - |

---

## 贡献者

感谢所有参与本次版本开发的贡献者！

- 兵部：测试修复
- 礼部：文档完善

---

## 下一步计划

- 持续优化测试覆盖率
- 完善 API 文档
- 性能优化

---

*NAS-OS 团队*