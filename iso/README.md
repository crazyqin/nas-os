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
   设置主机名与 root 密码。GPT 分区：amd64 额外保留 1M BIOS GRUB 分区，512M ESP + btrfs root，复制 live 根文件系统、
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
amd64、arm64 使用同架构 runner 原生构建，再执行 Live 引导冒烟和安装后磁盘验收。

安装验收覆盖 amd64 BIOS、amd64 UEFI、arm64 UEFI：在禁止访问外网的 QEMU 客体中，
自动回答安装器提示，运行 ISO 内真实安装器，将系统装入新建的 12G 虚拟盘。
移除 ISO 后检查 btrfs 根文件系统、服务和 WebUI，完成管理员首登改密，
再冷启动验证新密码仍然有效。系统根卷不作为可删除的数据卷管理。
临时 SSH 公钥仅用于测试；工件保存诊断日志与结果，不上传虚拟磁盘或私钥。

`ISO installed disk acceptance` 可手动复测已有构建工件，需要提供 ISO Build 运行编号；
工作流会检查工件对应的系统源码一致，避免旧 ISO 验证新代码。
QEMU 结果不代替具体 NAS 硬件、USB/Ventoy、固件和网卡兼容性验收。

### ARM 完整安装验收的运行环境

构建和 Live 冒烟可使用 GitHub-hosted runner；ARM 完整安装 gate 要求原生 ARM64
和实际可用的 KVM。`ubuntu-24.04-arm` 只保证 CPU 架构，不能保证嵌套虚拟化。
2026-10-01 的运行 `36802104374` / `36802104332` 实际均为 TCG，安装 30 分钟后
在 Live 包 purge 触发的第二次 initramfs 生成阶段超时。迁移 runner 标签不能视为
安装验收已通过。

两个工作流的 ARM 安装矩阵读取仓库变量 `ISO_ARM64_KVM_RUNNER`，值是 JSON
runner 标签数组。例如专用自托管 ARM Linux 主机：

```json
["self-hosted", "Linux", "ARM64", "nas-os-kvm"]
```

主机应为 ARM64 裸机，或已确认向客体提供 ARM KVM 的虚拟机；至少 4 核、8 GiB
内存和 30 GiB 空闲磁盘。runner 用户需可读写 `/dev/kvm`（通常加入 `kvm` 组后
重启 runner 服务），预装 Python 3、Git、GitHub CLI、OpenSSH 客户端，以及支持
无密码 sudo 的 apt 环境。工作流安装 QEMU 和 AAVMF。`/dev/kvm` 存在不够：验收
先启动暂停的同架构 QEMU，并通过 QMP `query-kvm` 确认 `enabled=true`；正式安装
及两次冷启动均使用显式 `-accel kvm`，初始化失败不会回退 TCG。

当前触发范围是维护者的分支 push / 手动运行；自托管主机应专用于该仓库，采用
一次任务后销毁的 ephemeral runner，且不承载业务数据或其他凭据。不要把此 job
改为自动执行外部 fork 的代码。checkout 不保留 Git 凭据，工件仍仅含日志和结果。

变量未设置时会在默认 hosted ARM runner 上明确失败；缺少 KVM 是环境阻塞，
不会跳过 ARM、使用 `continue-on-error` 或把 Live 冒烟当作完整安装通过。
接好 KVM 主机后手动运行 `ISO installed disk acceptance`，`iso_run_id` 填入
当前系统源码对应的 ISO Build 运行编号，仍检查源码一致性和 ISO SHA256。

诊断复现可以直接在无 KVM 主机上省略 `--require-kvm` 运行 `iso/e2e_install.py`；
这是性能调查，不替代 gate。尚未配置 KVM runner 时，独立验收工作流另跑
`ARM TCG performance diagnostics (not acceptance)`：对真实安装器采样最多 15 分钟，
结果的 `purpose=diagnostic`，工件名独立；完整 ARM job 仍要求 KVM，不能因诊断
通过而变绿。完整安装上限仍为 1800 秒。`*-timeline.log` 是每段串口
数据的 UTC 接收时间和累计秒数（JSON Lines）；串口每分钟附加客体进程的状态、
CPU 累计时间、等待位置及 initrd 文件大小/修改时间。用这些变化区分压缩计算、
磁盘等待与没有进展，不能只根据最后一行推断死锁。`result.json` 保存实际加速器、
工件系统源码和验收脚本提交，KVM 探针输出保存为 `kvm-probe.log`。

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
