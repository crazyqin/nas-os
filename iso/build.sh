#!/bin/bash
# NAS-OS 裸机安装 ISO 构建（在 debian:bookworm 容器内以 root 运行）。
#
# 用法（通常由 CI / Makefile 调用，不直接手工跑）:
#   docker run --rm -v "$(pwd)":/src debian:bookworm bash /src/iso/build.sh <amd64|arm64>
# 前置: iso-bin/<arch>/{nasd,nasctl} 已由 iso/prepare-binaries.sh 生成。
# 产物: dist/nas-os-<version>-<arch>.iso (+ .sha256)
#
# 设计:
# - Debian bookworm live 系统（live-boot + squashfs），amd64 支持 BIOS+UEFI 双启动，
#   arm64 支持 UEFI（EFI fallback 路径，无需 NVRAM 条目）。
# - 控制台自动登录 root，显示横幅；nasd 在 live 模式下即已运行（试用模式）。
# - 裸机安装: 运行 nasos-install 把 live 根文件系统复制到目标盘（全程离线）。
set -euo pipefail

ARCH="${1:?用法: build.sh <amd64|arm64>}"
SRC="${SRC:-/src}"
case "$ARCH" in
  amd64|arm64) ;;
  *) echo "不支持的架构: $ARCH" >&2; exit 1 ;;
esac

if [ ! -x "$SRC/iso-bin/$ARCH/nasd" ]; then
  echo "错误: 找不到 $SRC/iso-bin/$ARCH/nasd，请先运行 iso/prepare-binaries.sh $ARCH" >&2
  exit 1
fi

VERSION="$(tr -d '[:space:]' < "$SRC/VERSION")"
MIRROR="${NASOS_MIRROR:-http://deb.debian.org/debian}"   # 中国大陆可设 https://mirrors.tuna.tsinghua.edu.cn/debian
WORK="/build/isowork"

export DEBIAN_FRONTEND=noninteractive
export LC_ALL=C.UTF-8

echo ">>> [1/6] 安装 live-build 工具链（含 recommends: xorriso/mtools 等）"
apt-get update -qq
apt-get install -y -qq live-build rsync >/dev/null
echo "live-build 版本: $(dpkg-query -W -f='${Version}' live-build)"
# 参考：合法 bootloader 值随 live-build 版本/架构而异，输出校验源位置便于排障
grep -rn "not a valid bootloader" /usr/share/live/build/ 2>/dev/null | head -2 || true

echo ">>> [2/6] 准备构建目录（live-build lb build 要求 root，容器内直接 root 运行）"
rm -rf "$WORK"; mkdir -p "$WORK"
cd "$WORK"

echo ">>> [3/6] lb config ($ARCH, bookworm)"
case "$ARCH" in
  amd64)
    # 已验证合法值：syslinux(BIOS) + grub(UEFI via hybrid)
    BOOTLOADER_CANDIDATES=("syslinux grub")
    BOOTAPPEND="hostname=nasos console=tty0 console=ttyS0,115200n8"
    ;;
  arm64)
    # 无 BIOS：grub-efi；live-build 各版本命名有差异，多候选重试
    BOOTLOADER_CANDIDATES=("grub-efi" "grub")
    BOOTAPPEND="hostname=nasos console=ttyAMA0,115200n8"
    ;;
esac

CONFIGURED=0
for BL in "${BOOTLOADER_CANDIDATES[@]}"; do
  rm -rf "$WORK/config"
  if lb config \
    --architecture "$ARCH" \
    --distribution bookworm \
    --archive-areas 'main non-free-firmware' \
    --binary-images iso-hybrid \
    --bootloaders "$BL" \
    --debian-installer none \
    --security true \
    --updates true \
    --mirror-bootstrap "$MIRROR" \
    --mirror-chroot "$MIRROR" \
    --mirror-binary "$MIRROR" \
    --bootappend-live "$BOOTAPPEND" 2>&1; then
    echo ">>> lb config 成功 (bootloaders='$BL')"
    CONFIGURED=1
    break
  else
    echo ">>> lb config 失败 (bootloaders='$BL')，尝试下一候选…" >&2
  fi
done
[ "$CONFIGURED" = 1 ] || { echo "错误: 所有 bootloader 候选均失败" >&2; exit 1; }

echo ">>> [4/6] 叠加 NAS-OS 内容（二进制 / WebUI / 配置 / 安装器 / 服务）"
INC="$WORK/config/includes.chroot"
mkdir -p "$INC"
# 静态 rootfs（systemd 单元、网络配置、控制台横幅、nasos-install 安装器）
cp -a "$SRC/iso/live-config/rootfs/." "$INC/"
# nasd / nasctl 二进制
mkdir -p "$INC/usr/local/bin"
install -m 0755 "$SRC/iso-bin/$ARCH/nasd"  "$INC/usr/local/bin/nasd"
install -m 0755 "$SRC/iso-bin/$ARCH/nasctl" "$INC/usr/local/bin/nasctl"
# WebUI 静态文件（nasd 会自动探测 /usr/share/nas-os/webui）
mkdir -p "$INC/usr/share/nas-os"
cp -a "$SRC/webui" "$INC/usr/share/nas-os/webui"
# 主配置：裸机场景监听 0.0.0.0（原 default.yaml 是 127.0.0.1，面向容器/本地）
mkdir -p "$INC/etc/nas-os"
sed 's/^\([[:space:]]*host:[[:space:]]*\)127\.0\.0\.1[[:space:]]*$/\10.0.0.0/' \
  "$SRC/configs/default.yaml" > "$INC/etc/nas-os/config.yaml"

# 包列表（公共 + 按架构 grub 清单）
mkdir -p "$WORK/config/package-lists"
cp "$SRC/iso/live-config/package-lists/nas-os.list.chroot" "$WORK/config/package-lists/"
cp "$SRC/iso/live-config/package-lists/grub-$ARCH.list.chroot" "$WORK/config/package-lists/grub.list.chroot"
# chroot 钩子
mkdir -p "$WORK/config/hooks"
cp "$SRC/iso/live-config/hooks/"*.hook.chroot "$WORK/config/hooks/"
chmod 0755 "$WORK/config/hooks/"*.hook.chroot

echo ">>> [5/6] lb build（chroot 组装 + squashfs + ISO，arm64 交叉模拟下约 30-60 分钟）"
lb build 2>&1 | tee "$WORK/lb-build.log"

LIVE_ISO="$(ls "$WORK"/live-image-*.hybrid.iso 2>/dev/null || true)"
if [ -z "$LIVE_ISO" ]; then
  echo "错误: lb build 未产出 ISO。构建日志尾部：" >&2
  tail -n 100 "$WORK"/lb-build.log 2>/dev/null || true
  exit 1
fi

echo ">>> [6/6] 产出 dist/ 制品"
mkdir -p "$SRC/dist"
FINAL_ISO="$SRC/dist/nas-os-$VERSION-$ARCH.iso"
cp "$LIVE_ISO" "$FINAL_ISO"
( cd "$SRC/dist" && sha256sum "nas-os-$VERSION-$ARCH.iso" > "nas-os-$VERSION-$ARCH.iso.sha256" )
ls -lh "$FINAL_ISO"
echo ">>> 完成: $FINAL_ISO"
