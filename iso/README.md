# NAS-OS 裸机安装 ISO

Debian bookworm live 系统打包 nasd，插盘即用：**Live 试用 → `nasos-install` 离线装入硬盘 → 浏览器初始化**。

> 对应 Issue #22（"几时制作 ISO 文件"）。v1 形态为 live + 控制台安装器；
> Docker 与 `install.sh` 安装路径不受影响。

## 产物

| 文件 | 架构 | 启动方式 |
|------|------|----------|
| `dist/nas-os-<version>-amd64.iso` | x86_64 | BIOS（isolinux/grub-pc）+ UEFI（EFI fallback 路径） |
| `dist/nas-os-<version>-arm64.iso` | aarch64 | 仅 UEFI（EFI fallback 路径） |

## 使用流程

1. **启动**：ISO 写入 U 盘（`dd` 或 Rufus/Ventoy）或虚拟机光驱直接引导。
2. **Live 试用**：控制台自动登录 root，nasd 已在运行 —— 同网段浏览器访问
   `http://<设备IP>:8080`（或 `http://nasos.local:8080`，mDNS）。此模式不写盘，断电即失。
3. **安装**：控制台运行 `nasos-install`，选择目标磁盘（安装介质所在盘自动排除），
   设置主机名与 root 密码。GPT 分区：512M ESP + btrfs root，复制 live 根文件系统、
   装 GRUB、清理 live 组件（`live-boot` 等）并重生成 initramfs。**全程离线**。
4. **初始化**：重启后浏览器访问 `http://<设备IP>:8080`，用户名 `admin`，
   初始密码在 **新系统** 的 `/etc/nas-os/.admin_password`（控制台 `cat` 查看），
   首次登录强制改密。之后的数据盘/RAID/共享在 WebUI 里配置。

## 构建

```bash
make iso               # amd64（需本地 docker）
make iso-arm64         # arm64（自动注册 binfmt）
# 产物: dist/nas-os-*.iso + .sha256
```

CI：GitHub Actions `ISO Build` workflow（手动触发 `workflow_dispatch`），
amd64 产物附带 QEMU 引导冒烟（轮询 `/api/v1/system/health`）。

## 系统构成

- **基础**：Debian bookworm（minbase + 自定义包列表，含 `non-free-firmware` 常见网卡固件）
- **nasd**：Core 静态二进制 `/usr/local/bin/nasd` + WebUI `/usr/share/nas-os/webui`
- **运行时依赖**：对齐 `Dockerfile.full`（btrfs-progs / samba / nfs-kernel-server /
  smartmontools / nvme-cli / hdparm / rsync / zstd / iptables …）
- **网络**：systemd-networkd 全网口 DHCP + systemd-resolved；avahi mDNS（`nasos.local`）
- **远程管理**：openssh-server（root 密码由安装器设置；live 模式下密码锁定）
- **服务**：`nas-os.service`（systemd，`Restart=on-failure`）

源码位置：`iso/build.sh`（容器内编排 live-build）、`iso/live-config/`（包列表/钩子/rootfs 叠加层，
其中 `usr/sbin/nasos-install` 为安装器）。

## v1 限制

- **单盘安装**：安装器只写一块系统盘；数据盘/多盘池安装后在 WebUI 存储 manager 里配置
- **Secure Boot 需关闭**（未内置 shim 签名链）
- arm64 仅 UEFI（无 u-boot/SD 卡启动支持；树莓派类 SBC 不适用）
- 安装器界面语言为中文；WebUI 自身多语言不受影响
- 无 RAID/btrfs 多盘布局选项（v1 后续按需求排期）
