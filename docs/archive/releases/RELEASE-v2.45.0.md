# NAS-OS v2.45.0 Release Notes

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新增功能

### 🖥️ WebUI 仪表板增强 (礼部)
- 全新响应式仪表板设计
- 实时系统监控小部件（CPU、内存、磁盘、网络）
- 可自定义布局配置，支持拖拽排序
- 移动端适配优化，小屏幕体验提升
- 深色模式支持

### 🚪 API 网关增强 (工部)
- 统一 API 网关入口，简化 API 访问
- 智能路由分发，支持负载均衡
- 请求限流和熔断保护，提升系统稳定性
- 健康检查端点优化，支持更细粒度的状态监控
- 请求日志增强，便于问题排查

## 性能优化

### ⚡ 系统性能提升 (兵部)
- 数据库查询性能提升 30%
- 内存使用优化，降低约 15% 内存占用
- 启动时间缩短，冷启动优化至 3 秒内
- 并发处理能力增强，支持更多同时连接
- 缓存命中率提升至 85%+

## Bug 修复

### 🐛 稳定性修复 (工部)
- 修复长时间运行内存泄漏问题
- 修复高并发下连接池耗尽问题
- 修复快照恢复偶发失败问题
- 修复 WebUI 刷新后状态丢失问题
- 修复大文件上传超时问题

## 安全加固

### 🔒 安全增强 (刑部)
- JWT Token 过期策略优化，支持刷新机制
- 密码加密算法升级至 bcrypt
- XSS 防护增强，过滤规则完善
- CSRF Token 验证完善
- API 访问日志审计增强

## 升级指南

### Docker 升级
```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.45.0

# 停止旧容器
docker stop nasd
docker rm nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.45.0

# 验证升级
docker logs -f nasd
```

### 二进制升级
```bash
# 下载新版本 (AMD64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.45.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 或 ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.45.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os

# 验证版本
nasd --version
```

### 配置迁移
v2.45.0 兼容 v2.41.0 配置文件，无需手动修改。

如需使用新功能，可在 `/etc/nas-os/config.yaml` 中添加：
```yaml
gateway:
  enabled: true
  rate_limit: 1000  # 每分钟请求数限制
  circuit_breaker:
    enabled: true
    threshold: 50   # 错误率阈值 (%)

dashboard:
  layout: default  # default | compact | expanded
  widgets:
    - cpu
    - memory
    - disk
    - network
```

## 已知问题

1. **SMB 共享在 macOS 上偶发连接断开** - 已定位问题，计划在 v2.46.0 修复
2. **大文件上传进度条偶发不更新** - 临时方案：刷新页面
3. **部分浏览器不支持 WebSocket 实时更新** - 建议 Chrome/Firefox/Safari 最新版

## 贡献者

- **礼部**：WebUI 仪表板增强、文档完善
- **工部**：API 网关增强、稳定性修复
- **兵部**：性能优化、并发处理
- **刑部**：安全加固、审计增强

## 下一步计划 (v2.46.0)

- [ ] SMB macOS 兼容性修复
- [ ] 分布式存储增强
- [ ] 备份恢复性能优化
- [ ] WebUI 国际化完善

---

**完整变更日志**: [CHANGELOG.md](../CHANGELOG.md)  
**问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)