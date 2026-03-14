# NAS-OS v2.41.0 Release Notes

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新增功能

### 📚 Swagger API 文档完善 (礼部)
- 生成完整的 OpenAPI/Swagger 文档
- 支持 `docs/swagger.json` 和 `docs/swagger.yaml`
- 自动生成 `docs/docs.go` 供 swaggo 使用
- API 文档覆盖所有主要模块

## 修复

### 🧪 测试修复 (兵部)
- 并发测试用例优化
- 存储成本测试修复
- 容量规划测试修复
- 备份/快照/缓存模块测试完善

## 优化

### 🚀 CI/CD 优化 (工部)
- Node.js 24 支持
- 缓存策略优化
- 构建并行化改进
- 测试超时配置优化

## 升级指南

### Docker 升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.41.0
docker stop nasd
docker rm nasd
# 使用新镜像启动
```

### 二进制升级
```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.41.0/nasd-linux-$(uname -m)
chmod +x nasd-linux-$(uname -m)
sudo mv nasd-linux-$(uname -m) /usr/local/bin/nasd
```

## 贡献者

- 礼部：Swagger API 文档完善
- 兵部：测试修复
- 工部：CI/CD 优化

---

**完整变更日志**: [CHANGELOG.md](../CHANGELOG.md)