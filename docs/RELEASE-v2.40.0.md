# NAS-OS v2.40.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新增功能

### 🛡️ 安全审计系统 (刑部)
- 新增 `internal/auth/security_audit.go` 安全审计模块
- 9 项安全检查项：
  - 密码策略强度验证
  - 会话管理安全性
  - 权限隔离检查
  - 敏感数据加密
  - API 限流配置
  - 审计日志完整性
  - 输入验证规范
  - 错误信息脱敏
  - 依赖安全扫描
- 完整的安全审计测试覆盖

### 📊 配额管理优化 (户部)
- 配额管理模块分析 (13,914 行代码)
- 成本计算逻辑验证
- 资源效率评分分析
- 5 项优化建议

## Bug 修复

### 🔧 并发安全修复 (兵部)
- 修复 `websocket_enhanced.go` closeOnce 并发问题
- 修复 `response.go` NoContent() 返回状态码
- 修复 `validator_test.go` 测试用例
- 修复 `ldap/ad.go` 负数解析问题
- 修复 `capacity_planning_test.go` 测试期望

## 改进优化

### 🚀 CI/CD 优化 (工部)
- 超时配置从 20m 增至 30m
- 测试并行化 (`-parallel 4`)
- Dockerfile 健康检查修复 (wget→curl)
- Makefile 测试参数优化

### 📚 文档更新 (礼部)
- 所有文档版本号同步至 v2.40.0
- README.md 版本信息更新
- docs/ 文档索引更新
- API 文档版本更新

## 升级说明

### Docker 用户

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.40.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.40.0
```

### 二进制用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.40.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 完整变更日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

**六部联建** - 兵部、工部、礼部、刑部、户部、吏部协同完成