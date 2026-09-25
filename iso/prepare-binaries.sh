#!/usr/bin/env bash
# 预构建 nasd / nasctl 二进制，供 ISO 打包使用。
# 用法: ./iso/prepare-binaries.sh <amd64|arm64>
# 产物: iso-bin/<arch>/{nasd,nasctl}
set -euo pipefail

ARCH="${1:?用法: prepare-binaries.sh <amd64|arm64>}"
case "$ARCH" in
  amd64|arm64) ;;
  *) echo "不支持的架构: $ARCH（支持 amd64 / arm64）" >&2; exit 1 ;;
esac

cd "$(dirname "$0")/.."
VERSION="$(tr -d '[:space:]' < VERSION)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

OUT="iso-bin/$ARCH"
mkdir -p "$OUT"

LDFLAGS="-s -w -X nas-os/internal/version.Version=$VERSION -X nas-os/internal/version.Commit=$COMMIT -X nas-os/internal/version.BuildTime=$BUILD_TIME"

echo ">>> 构建 $ARCH 二进制 (Core, version=$VERSION commit=$COMMIT)"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
  go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/nasd" ./cmd/nasd
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
  go build -trimpath -ldflags "-s -w -X nas-os/internal/version.Version=$VERSION" -o "$OUT/nasctl" ./cmd/nasctl

ls -lh "$OUT"
echo ">>> 二进制就绪: $OUT"
