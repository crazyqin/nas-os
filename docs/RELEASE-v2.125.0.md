# NAS-OS v2.125.0 Release Notes

**发布日期**: 2026-03-16

## 概述

v2.125.0 版本修复了 Docker 健康检查工具构建失败的问题。

## 更新内容

### 工部 - Docker 构建修复
- 修复 Docker 健康检查工具构建失败问题
- 使用 heredoc 语法替代 echo 转义
- 解决 shell 转义字符导致的构建错误

### 礼部 - 文档版本同步
- 版本号同步至 v2.125.0
- README.md 版本信息更新
- CHANGELOG.md 版本记录更新

## 修复内容

### Dockerfile 健康检查脚本语法修复
- 使用 heredoc 语法 (`<<'EOF'`) 替代 `echo -e` 转义
- 避免了 shell 转义字符在不同环境下的兼容性问题
- 构建流程稳定性提升

## 升级说明

### Docker 部署升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.125.0
docker stop nasd
docker rm nasd
# 重新运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.125.0
```

### 二进制升级
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.125.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.125.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

## 贡献者

感谢所有贡献者的付出！

---

**完整更新日志**: 查看 [CHANGELOG.md](../CHANGELOG.md)