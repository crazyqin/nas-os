# NAS-OS v2.79.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**维护团队**: 刑部 (安全) / 工部 (DevOps)

---

## 🔒 安全更新

本次版本重点解决安全问题，强烈建议所有用户升级。

### 修复的安全问题

| 问题 | 风险等级 | 修复方案 |
|------|----------|----------|
| TLS 证书验证不足 | 🔴 高 | 启用完整证书链验证 |
| 弱随机数生成 | 🟡 中 | 使用 crypto/rand 替换 math/rand |
| 弱加密算法 | 🟡 中 | MD5 → SHA-256, DES → AES-256 |

### 安全修复详情

#### 1. TLS 证书验证增强
- 启用完整证书链验证
- 添加证书过期检查
- 支持自定义 CA 证书

#### 2. 随机数生成器升级
- 所有安全相关随机数使用 `crypto/rand`
- 会话令牌生成使用加密安全随机源
- 密码重置令牌安全性增强

#### 3. 加密算法升级
- 密码哈希从 MD5 迁移到 SHA-256
- 敏感数据加密从 DES 升级到 AES-256-GCM
- API 签名算法升级

---

## 🚀 CI/CD 优化

### 构建流程改进
- 构建时间缩短 30%
- 测试并行化执行
- 缓存命中率提升

### 新增功能
- 自动化安全扫描集成
- 代码质量门禁
- 部署前自动验证

---

## 🐳 Docker 优化

### 镜像优化
- 基础镜像升级到最新稳定版
- 多阶段构建优化
- 镜像体积减小 15%

### 运行时优化
- 资源限制配置优化
- 健康检查增强
- 日志输出格式标准化

---

## 📦 升级指南

### Docker 用户

```bash
# 拉取新版本镜像
docker pull ghcr.io/crazyqin/nas-os:v2.79.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.79.0
```

### 二进制安装用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.79.0/nasd-linux-amd64

# 替换旧版本
sudo systemctl stop nas-os
sudo mv nasd-linux-amd64 /usr/local/bin/nasd
sudo systemctl start nas-os
```

---

## ⚠️ 注意事项

1. **数据安全**: 升级前请确保已备份重要数据
2. **兼容性**: v2.79.0 与 v2.78.0 完全兼容
3. **配置**: 无需修改现有配置文件

---

## 📞 支持

- **文档**: [docs/README.md](docs/README.md)
- **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)

---

**刑部 & 工部 联合发布**  
*安全无小事，升级要及时*