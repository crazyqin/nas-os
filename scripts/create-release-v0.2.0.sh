#!/bin/bash
# NAS-OS v0.2.0 GitHub Release 创建脚本
# 使用方法：./create-release.sh

set -e

VERSION="v0.2.0"
TAG_NAME="v0.2.0"
RELEASE_NAME="NAS-OS v0.2.0 Alpha"
BODY_FILE="docs/RELEASE-v0.2.0.md"

echo "🚀 创建 GitHub Release $VERSION..."

# 检查 gh CLI 是否安装
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) 未安装"
    echo "请安装：https://cli.github.com/"
    echo "或手动在 GitHub 上创建 Release: https://github.com/nas-os/nasd/releases/new"
    exit 1
fi

# 检查是否已登录
if ! gh auth status &> /dev/null; then
    echo "❌ 未登录 GitHub"
    echo "请运行：gh auth login"
    exit 1
fi

# 创建 Git 标签
echo "📌 创建 Git 标签 $TAG_NAME..."
git tag -a "$TAG_NAME" -m "Release $RELEASE_NAME"

# 推送到远程
echo "📤 推送标签到 GitHub..."
git push origin "$TAG_NAME"

# 准备发布说明
echo "📝 准备发布说明..."
RELEASE_BODY=$(cat <<EOF
**发布日期**: 2026-03-10
**版本类型**: Alpha (早期预览)

## ✨ 新功能

- ✅ SMB/CIFS 文件共享支持
- ✅ NFS 文件共享支持
- ✅ 配置持久化系统 (YAML)
- ✅ 配置热重载功能
- ✅ 共享权限管理
- ✅ Web UI 存储管理页面
- ✅ Web UI 共享配置页面
- ✅ btrfs 完整功能 (balance/scrub)

## 📦 下载

### 二进制文件
- **Linux amd64**: nasd-linux-amd64 (22MB)
- **Linux arm64**: nasd-linux-arm64 (20MB)
- **Linux armv7**: nasd-linux-armv7 (21MB)

### Docker
\`\`\`bash
docker pull nas-os/nasd:v0.2.0
\`\`\`

## 📋 快速开始

\`\`\`bash
# 下载 (根据你的架构)
wget https://github.com/nas-os/nasd/releases/download/$VERSION/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 运行
sudo nasd
\`\`\`

访问 http://localhost:8080

## 📖 完整文档

- [发布说明](docs/RELEASE-v0.2.0.md)
- [安装指南](docs/INSTALL.md)
- [变更日志](docs/CHANGELOG.md)

## ⚠️ 注意事项

Alpha 版本不适合生产环境使用。数据可能丢失，功能可能变更。

---

*发布团队：吏部*
EOF
)

# 创建 Release
echo "🎉 创建 GitHub Release..."
gh release create "$TAG_NAME" \
    --title "$RELEASE_NAME" \
    --notes "$RELEASE_BODY" \
    --draft \
    nasd-linux-amd64 \
    nasd-linux-arm64 \
    nasd-linux-armv7

echo ""
echo "✅ Release 创建成功!"
echo "🔗 查看 Release: https://github.com/nas-os/nasd/releases/$TAG_NAME"
echo ""
echo "⚠️  注意：Release 已创建为草稿 (Draft)"
echo "   请检查后在 GitHub 上发布：https://github.com/nas-os/nasd/releases"
