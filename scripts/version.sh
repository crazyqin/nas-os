#!/bin/bash
# 版本同步脚本
# 从 VERSION 文件读取版本号，同步到各处

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$PROJECT_ROOT/VERSION"

# 读取版本号（去掉 v 前缀）
VERSION=$(cat "$VERSION_FILE" | tr -d 'v' | tr -d '\n')
echo "📋 当前版本: $VERSION"

# 同步 internal/version/version.go
VERSION_GO="$PROJECT_ROOT/internal/version/version.go"
if [ -f "$VERSION_GO" ]; then
    # 更新 Version 变量
    sed -i "s/Version   = \"[^\"]*\"/Version   = \"$VERSION\"/" "$VERSION_GO"
    # 更新 BuildDate
    BUILD_DATE=$(date +%Y-%m-%d)
    sed -i "s/BuildDate = \"[^\"]*\"/BuildDate = \"$BUILD_DATE\"/" "$VERSION_GO"
    echo "✅ 已同步 version.go"
fi

# 同步 cmd/nasd/main.go Swagger 注释
MAIN_GO="$PROJECT_ROOT/cmd/nasd/main.go"
if [ -f "$MAIN_GO" ]; then
    sed -i "s/@version .*/@version $VERSION/" "$MAIN_GO"
    echo "✅ 已同步 main.go Swagger 注释"
fi

# 同步 README.md 版本信息
README="$PROJECT_ROOT/README.md"
if [ -f "$README" ]; then
    # 更新最新版本行
    sed -i "s/\*\*最新版本\*\*: v[0-9.]* / **最新版本**: v$VERSION /" "$README"
    # 更新 Docker badge
    sed -i "s/docker\/v[0-9.]*/docker\/v$VERSION/" "$README"
    # 更新下载链接
    sed -i "s/releases\/download\/v[0-9.]*/releases\/download\/v$VERSION/" "$README"
    sed -i "s/nasd-linux-[a-z0-9]*$/nasd-linux-\1/g" "$README" 2>/dev/null || true
    echo "✅ 已同步 README.md"
fi

# 同步 CHANGELOG.md（如果未发布）
CHANGELOG="$PROJECT_ROOT/CHANGELOG.md"
if [ -f "$CHANGELOG" ]; then
    if ! grep -q "## \[v$VERSION\]" "$CHANGELOG" && ! grep -q "## v$VERSION" "$CHANGELOG"; then
        echo "⚠️ CHANGELOG.md 未包含 v$VERSION，请手动添加"
    else
        echo "✅ CHANGELOG.md 已包含 v$VERSION"
    fi
fi

echo ""
echo "🎉 版本同步完成: v$VERSION"