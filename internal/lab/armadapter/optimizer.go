package armadapter

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== ARM 优化引擎 ==========

// Optimizer ARM 优化建议引擎.
type Optimizer struct {
	mu sync.RWMutex
}

// NewOptimizer 创建优化引擎.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// GenerateProfile 生成优化配置档案.
func (o *Optimizer) GenerateProfile(info *ARMHardwareInfo) (*OptProfile, error) {
	if info == nil {
		return nil, fmt.Errorf("hardware info is nil")
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	profile := &OptProfile{
		DeviceName:  info.SoCModel,
		ArchType:    info.ArchType,
		GeneratedAt: time.Now(),
	}

	var opts []Optimization

	// CPU 调度优化
	opts = append(opts, o.generateCPUOpts(info)...)

	// 内存优化
	opts = append(opts, o.generateMemoryOpts(info)...)

	// 存储优化
	opts = append(opts, o.generateStorageOpts(info)...)

	// 网络优化
	opts = append(opts, o.generateNetworkOpts(info)...)

	// 功耗优化
	opts = append(opts, o.generatePowerOpts(info)...)

	// 内核参数优化
	opts = append(opts, o.generateKernelOpts(info)...)

	profile.Optimizations = opts

	log.Printf("[ARM适配] 生成优化档案: device=%s arch=%s opts=%d",
		profile.DeviceName, profile.ArchType, len(profile.Optimizations))

	return profile, nil
}

// generateCPUOpts 生成 CPU 调度优化建议.
func (o *Optimizer) generateCPUOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// 调度器优化
	if info.CPUCores >= 4 {
		opts = append(opts, Optimization{
			Category:    OptCPU,
			Priority:    OptPriorityHigh,
			Title:       "CPU 调度器优化",
			Description: "使用 EAS (Energy Aware Scheduling) 调度器，充分利用 big.LITTLE 架构",
			Parameter:   "kernel.sched_energy_aware",
			Value:       "1",
			Reason:      fmt.Sprintf("检测到 %d 核 CPU，启用能效感知调度", info.CPUCores),
		})
	}

	// big.LITTLE 优化
	if info.BigCores > 0 && info.LittleCores > 0 {
		opts = append(opts, Optimization{
			Category:    OptCPU,
			Priority:    OptPriorityHigh,
			Title:       "big.LITTLE 调度优化",
			Description: "配置 CPU 亲和性，将关键任务绑定到大核",
			Parameter:   "sched_setaffinity (建议配置)",
			Value:       fmt.Sprintf("大核: 0-%d, 小核: %d-%d", info.BigCores-1, info.BigCores, info.CPUCores-1),
			Reason:      fmt.Sprintf("检测到 big.LITTLE 架构 (%d 大核 + %d 小核)", info.BigCores, info.LittleCores),
		})
	}

	// CPU 频率策略
	if info.MaxFreqMHz > 0 {
		policy := "ondemand"
		if info.MaxFreqMHz >= 2000 {
			policy = "schedutil"
		}
		opts = append(opts, Optimization{
			Category:    OptCPU,
			Priority:    OptPriorityMedium,
			Title:       "CPU 频率调节策略",
			Description: "使用 schedutil 调频策略，根据调度器负载自动调整频率",
			Parameter:   "cpufreq/scaling_governor",
			Value:       policy,
			Reason:      fmt.Sprintf("最大频率 %dMHz，推荐使用 %s 策略", info.MaxFreqMHz, policy),
		})
	}

	// CPU 缓存优化
	if info.ArchType == ArchARM64 {
		opts = append(opts, Optimization{
			Category:    OptCPU,
			Priority:    OptPriorityMedium,
			Title:       "缓存行预取优化",
			Description: "调整 L2 缓存预取策略，提升顺序读取性能",
			Parameter:   "kernel.sched_min_granularity_ns",
			Value:       "1000000",
			Reason:      "ARM64 架构支持 L2 缓存预取优化",
		})
	}

	return opts
}

// generateMemoryOpts 生成内存优化建议.
func (o *Optimizer) generateMemoryOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// 虚拟内存优化
	if info.MemoryMB <= 2048 {
		opts = append(opts, Optimization{
			Category:    OptMemory,
			Priority:    OptPriorityHigh,
			Title:       "Swap 策略优化",
			Description: "降低 swappiness，减少不必要的 swap 操作",
			Parameter:   "vm.swappiness",
			Value:       "10",
			Reason:      fmt.Sprintf("内存 %dMB，需要减少 swap 频率", info.MemoryMB),
		})

		opts = append(opts, Optimization{
			Category:    OptMemory,
			Priority:    OptPriorityHigh,
			Title:       "内存压缩 (zram)",
			Description: "启用 zram 内存压缩，等效增加可用内存",
			Parameter:   "zram",
			Value:       fmt.Sprintf("size=%dMB", info.MemoryMB/2),
			Reason:      "低内存设备推荐使用 zram 压缩",
		})
	}

	// 页表优化
	if info.ArchType == ArchARM64 {
		opts = append(opts, Optimization{
			Category:    OptMemory,
			Priority:    OptPriorityMedium,
			Title:       "大页内存支持",
			Description: "启用透明大页 (THP)，减少 TLB miss",
			Parameter:   "vm.transparent_hugepage",
			Value:       "madvise",
			Reason:      "ARM64 支持 4KB/16KB/64KB 页大小，大页可提升性能",
		})
	}

	// 脏页写回策略
	opts = append(opts, Optimization{
		Category:    OptMemory,
		Priority:    OptPriorityMedium,
		Title:       "脏页写回策略",
		Description: "调整脏页写回参数，平衡写入性能和数据安全性",
		Parameter:   "vm.dirty_ratio",
		Value:       "20",
		Reason:      "ARM 设备 I/O 能力有限，需要优化脏页策略",
	})

	opts = append(opts, Optimization{
		Category:    OptMemory,
		Priority:    OptPriorityLow,
		Title:       "脏页后台写回",
		Description: "设置后台脏页写回阈值",
		Parameter:   "vm.dirty_background_ratio",
		Value:       "5",
		Reason:      "避免大量脏页集中写入造成 I/O 卡顿",
	})

	return opts
}

// generateStorageOpts 生成存储优化建议.
func (o *Optimizer) generateStorageOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// I/O 调度器
	if info.HasPCIe {
		opts = append(opts, Optimization{
			Category:    OptStorage,
			Priority:    OptPriorityHigh,
			Title:       "NVMe I/O 调度器",
			Description: "NVMe 设备使用 none (noop) 调度器，减少调度开销",
			Parameter:   "queue/scheduler",
			Value:       "none",
			Reason:      "检测到 PCIe 接口，NVMe 设备自带调度",
		})
	} else if info.HasSATA {
		opts = append(opts, Optimization{
			Category:    OptStorage,
			Priority:    OptPriorityHigh,
			Title:       "SATA I/O 调度器",
			Description: "SATA 设备使用 mq-deadline 调度器",
			Parameter:   "queue/scheduler",
			Value:       "mq-deadline",
			Reason:      "SATA 设备推荐使用 mq-deadline 减少延迟",
		})
	} else {
		opts = append(opts, Optimization{
			Category:    OptStorage,
			Priority:    OptPriorityMedium,
			Title:       "eMMC/SD I/O 调度器",
			Description: "使用 bfq 调度器优化 eMMC/SD 卡 I/O",
			Parameter:   "queue/scheduler",
			Value:       "bfq",
			Reason:      "eMMC/SD 设备使用 bfq 调度器改善公平性",
		})
	}

	// 文件系统优化
	opts = append(opts, Optimization{
		Category:    OptStorage,
		Priority:    OptPriorityHigh,
		Title:       "文件系统挂载选项",
		Description: "使用 noatime 减少元数据写入，延长 eMMC/SD 寿命",
		Parameter:   "mount options",
		Value:       "noatime,nodiratime",
		Reason:      "减少不必要的元数据写入，降低存储磨损",
	})

	// 读写队列深度
	if info.MemoryMB >= 2048 {
		opts = append(opts, Optimization{
			Category:    OptStorage,
			Priority:    OptPriorityMedium,
			Title:       "读写队列深度",
			Description: "增加读写队列深度，提升并发 I/O 性能",
			Parameter:   "queue/nr_requests",
			Value:       "128",
			Reason:      fmt.Sprintf("内存 %dMB 足够支持更深的 I/O 队列", info.MemoryMB),
		})
	} else {
		opts = append(opts, Optimization{
			Category:    OptStorage,
			Priority:    OptPriorityMedium,
			Title:       "读写队列深度",
			Description: "适中队列深度，避免内存压力",
			Parameter:   "queue/nr_requests",
			Value:       "64",
			Reason:      fmt.Sprintf("内存 %dMB 有限，使用适中队列深度", info.MemoryMB),
		})
	}

	// TRIM/DISCARD 支持
	opts = append(opts, Optimization{
		Category:    OptStorage,
		Priority:    OptPriorityLow,
		Title:       "TRIM/DISCARD 支持",
		Description: "定期执行 TRIM 命令，维护 SSD 性能",
		Parameter:   "fstrim",
		Value:       "weekly cron",
		Reason:      "维护 SSD/eMMC 写入性能和寿命",
	})

	return opts
}

// generateNetworkOpts 生成网络优化建议.
func (o *Optimizer) generateNetworkOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// 网络缓冲区
	if info.HasGbE {
		opts = append(opts, Optimization{
			Category:    OptNetwork,
			Priority:    OptPriorityHigh,
			Title:       "网络缓冲区优化",
			Description: "增大 TCP 缓冲区，提升千兆网络吞吐量",
			Parameter:   "net.core.rmem_max",
			Value:       "16777216",
			Reason:      "千兆网络需要更大缓冲区以获得最佳吞吐量",
		})

		opts = append(opts, Optimization{
			Category:    OptNetwork,
			Priority:    OptPriorityHigh,
			Title:       "TCP 发送缓冲区",
			Description: "增大 TCP 发送缓冲区",
			Parameter:   "net.core.wmem_max",
			Value:       "16777216",
			Reason:      "配合接收缓冲区，实现双向高速传输",
		})
	}

	if info.Has2_5GbE {
		opts = append(opts, Optimization{
			Category:    OptNetwork,
			Priority:    OptPriorityHigh,
			Title:       "2.5G 网络优化",
			Description: "增大网络队列长度和 NAPI 轮询预算",
			Parameter:   "net.core.netdev_budget",
			Value:       "600",
			Reason:      "2.5G 网络需要更大的处理预算",
		})
	}

	// TCP 优化
	opts = append(opts, Optimization{
		Category:    OptNetwork,
		Priority:    OptPriorityMedium,
		Title:       "TCP BBR 拥塞控制",
		Description: "使用 BBR 拥塞控制算法，提升长距离传输性能",
		Parameter:   "net.ipv4.tcp_congestion_control",
		Value:       "bbr",
		Reason:      "BBR 在 ARM 设备上表现优于传统 Cubic",
	})

	// 网络中断合并
	opts = append(opts, Optimization{
		Category:    OptNetwork,
		Priority:    OptPriorityMedium,
		Title:       "网络中断合并",
		Description: "启用网络中断合并，减少 CPU 中断开销",
		Parameter:   "ethtool -C eth0",
		Value:       "rx-usecs 50 tx-usecs 50",
		Reason:      "ARM CPU 处理中断开销较大，合并可降低负载",
	})

	// TCP Fast Open
	opts = append(opts, Optimization{
		Category:    OptNetwork,
		Priority:    OptPriorityLow,
		Title:       "TCP Fast Open",
		Description: "启用 TCP Fast Open，减少连接建立延迟",
		Parameter:   "net.ipv4.tcp_fastopen",
		Value:       "3",
		Reason:      "减少 SMB/NFS 等协议的连接建立开销",
	})

	return opts
}

// generatePowerOpts 生成功耗优化建议.
func (o *Optimizer) generatePowerOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// CPU 空闲状态
	opts = append(opts, Optimization{
		Category:    OptPower,
		Priority:    OptPriorityHigh,
		Title:       "CPU 空闲状态",
		Description: "启用深度 C-state，降低空闲功耗",
		Parameter:   "cpuidle/off=0",
		Value:       "enabled",
		Reason:      "ARM 设备支持多级 C-state，可显著降低待机功耗",
	})

	// 硬盘休眠
	if info.HasSATA {
		opts = append(opts, Optimization{
			Category:    OptPower,
			Priority:    OptPriorityMedium,
			Title:       "硬盘休眠策略",
			Description: "无访问时自动休眠硬盘，降低功耗和噪音",
			Parameter:   "hdparm -S",
			Value:       "120 (10 分钟)",
			Reason:      "SATA 硬盘支持 APM 和休眠，可节省 3-5W/盘",
		})
	}

	// USB 自动挂起
	opts = append(opts, Optimization{
		Category:    OptPower,
		Priority:    OptPriorityLow,
		Title:       "USB 自动挂起",
		Description: "空闲 USB 设备自动挂起，降低功耗",
		Parameter:   "usbcore.autosuspend",
		Value:       "2",
		Reason:      "USB 设备自动挂起可降低约 0.5W 功耗",
	})

	// GPU 电源管理
	if info.SoC == SoCRockchip {
		opts = append(opts, Optimization{
			Category:    OptPower,
			Priority:    OptPriorityLow,
			Title:       "GPU 电源管理",
			Description: "NAS 场景关闭或降频 GPU，节省功耗",
			Parameter:   "GPU power management",
			Value:       "min_freq / disabled",
			Reason:      "NAS 通常无显示需求，GPU 功耗浪费",
		})
	}

	return opts
}

// generateKernelOpts 生成内核参数优化建议.
func (o *Optimizer) generateKernelOpts(info *ARMHardwareInfo) []Optimization {
	var opts []Optimization

	// 文件描述符限制
	opts = append(opts, Optimization{
		Category:    OptKernel,
		Priority:    OptPriorityHigh,
		Title:       "文件描述符限制",
		Description: "增加系统最大文件描述符数，支持更多并发连接",
		Parameter:   "fs.file-max",
		Value:       "2097152",
		Reason:      "NAS 设备需要处理大量文件和网络连接",
	})

	// inotify 限制
	opts = append(opts, Optimization{
		Category:    OptKernel,
		Priority:    OptPriorityMedium,
		Title:       "inotify 监控限制",
		Description: "增加 inotify 监控数量，支持文件实时同步",
		Parameter:   "fs.inotify.max_user_watches",
		Value:       "524288",
		Reason:      "文件同步和索引需要大量 inotify watch",
	})

	// 网络连接追踪
	opts = append(opts, Optimization{
		Category:    OptKernel,
		Priority:    OptPriorityMedium,
		Title:       "连接追踪表大小",
		Description: "增大连接追踪表，支持更多并发网络连接",
		Parameter:   "net.netfilter.nf_conntrack_max",
		Value:       "262144",
		Reason:      "NAS 作为网关或代理时需要大量连接追踪",
	})

	// 内存 overcommit
	if info.MemoryMB <= 2048 {
		opts = append(opts, Optimization{
			Category:    OptKernel,
			Priority:    OptPriorityMedium,
			Title:       "内存 overcommit 策略",
			Description: "使用启发式 overcommit，避免 OOM Killer",
			Parameter:   "vm.overcommit_memory",
			Value:       "0",
			Reason:      "低内存设备需要更谨慎的内存分配策略",
		})
	}

	// 定时器精度
	opts = append(opts, Optimization{
		Category:    OptKernel,
		Priority:    OptPriorityLow,
		Title:       "定时器精度",
		Description: "使用 250Hz 定时器频率，平衡精度和开销",
		Parameter:   "CONFIG_HZ",
		Value:       "250",
		Reason:      "ARM 设备推荐 250Hz，减少定时器中断开销",
	})

	// CRC32 硬件加速
	if hasFeature(info.Features, FeatureCRC32) {
		opts = append(opts, Optimization{
			Category:    OptKernel,
			Priority:    OptPriorityMedium,
			Title:       "CRC32 硬件加速",
			Description: "启用 CRC32 硬件指令加速内核校验",
			Parameter:   "CONFIG_ARM64_CRC32",
			Value:       "y",
			Reason:      "检测到 CRC32 硬件指令，可加速 ZFS/Btrfs 校验",
		})
	}

	// AES 硬件加速
	if hasFeature(info.Features, FeatureAES) {
		opts = append(opts, Optimization{
			Category:    OptKernel,
			Priority:    OptPriorityMedium,
			Title:       "AES 硬件加速",
			Description: "启用 AES 硬件指令加速加密操作",
			Parameter:   "CONFIG_ARM64_CRYPTO",
			Value:       "y",
			Reason:      "检测到 AES 硬件指令，可加速磁盘加密和 VPN",
		})
	}

	return opts
}

// hasFeature 检查是否具有指定特性.
func hasFeature(features []CPUFeature, target CPUFeature) bool {
	for _, f := range features {
		if f == target {
			return true
		}
	}
	return false
}
