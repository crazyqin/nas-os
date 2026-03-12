#!/bin/bash

# 部署相册 Web UI 脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAS_OS_DIR="$(dirname "$SCRIPT_DIR")"
WEBUI_DIR="$NAS_OS_DIR/webui"
PHOTOS_UI_DIR="$WEBUI_DIR/photos"

echo "🚀 部署相册 Web UI..."

# 检查目录
if [ ! -d "$PHOTOS_UI_DIR" ]; then
    echo "❌ 相册 UI 目录不存在：$PHOTOS_UI_DIR"
    exit 1
fi

# 创建符号链接（如果使用 Docker）
if [ -d "/opt/nas/webui" ]; then
    echo "📦 检测到 Docker 部署模式..."
    ln -sf "$PHOTOS_UI_DIR" /opt/nas/webui/photos
    echo "✅ 已创建符号链接：/opt/nas/webui/photos -> $PHOTOS_UI_DIR"
fi

# 验证文件
if [ -f "$PHOTOS_UI_DIR/index.html" ]; then
    echo "✅ 相册 UI 文件验证通过"
    echo "📄 文件大小：$(du -h "$PHOTOS_UI_DIR/index.html" | cut -f1)"
else
    echo "❌ index.html 不存在"
    exit 1
fi

# 设置权限
chmod -R 755 "$PHOTOS_UI_DIR"
echo "✅ 权限设置完成"

echo ""
echo "🎉 部署完成！"
echo "📍 访问地址：http://localhost:8080/photos/"
echo ""
