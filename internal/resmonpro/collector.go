package resmonpro

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Collector 资源采集器.
type Collector struct {
	mu          sync.RWMutex
	procPath    string
	sysPath     string
	nvidiaSmi   string
	prevCPU     map[string]int64
	prevNetIO   map[string]NetIOCounters
	prevDiskIO  map[string]DiskIOCounters
	lastCollect time.Time
}

// NetIOCounters 网络 IO 计数器.
type NetIOCounters struct {
	BytesIn    int64
	BytesOut   int64
	PacketsIn  int64
	PacketsOut int64
}

// DiskIOCounters 磁盘 IO 计数器.
type DiskIOCounters struct {
	ReadSectors  int64
	WriteSectors int64
	IoTime       int64
	Timestamp    time.Time
}

// NewCollector 创建采集器.
func NewCollector() *Collector {
	return &Collector{
		procPath:   "/proc",
		sysPath:    "/sys",
		nvidiaSmi:  "nvidia-smi",
		prevCPU:    make(map[string]int64),
		prevNetIO:  make(map[string]NetIOCounters),
		prevDiskIO: make(map[string]DiskIOCounters),
	}
}

// CollectProcesses 采集进程信息.
func (c *Collector) CollectProcesses() ([]ProcessInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.procPath)
	if err != nil {
		return nil, fmt.Errorf("read proc failed: %w", err)
	}

	var processes []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		proc, err := c.collectProcess(pid)
		if err != nil {
			continue
		}
		processes = append(processes, proc)
	}

	return processes, nil
}

func (c *Collector) collectProcess(pid int) (ProcessInfo, error) {
	info := ProcessInfo{PID: pid}

	// 读取进程名
	commPath := filepath.Join(c.procPath, strconv.Itoa(pid), "comm")
	if data, err := os.ReadFile(commPath); err == nil {
		info.Name = strings.TrimSpace(string(data))
	}

	// 读取进程状态
	statusPath := filepath.Join(c.procPath, strconv.Itoa(pid), "status")
	if data, err := os.ReadFile(statusPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "State:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					info.Status = parts[1]
				}
			}
		}
	}

	// 读取内存使用
	statmPath := filepath.Join(c.procPath, strconv.Itoa(pid), "statm")
	if data, err := os.ReadFile(statmPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 2 {
			pages, _ := strconv.ParseInt(fields[1], 10, 64)
			info.Memory = float64(pages*int64(os.Getpagesize())) / 1024 / 1024
		}
	}

	return info, nil
}

// CollectGPU 采集 GPU 信息.
func (c *Collector) CollectGPU() ([]GPUInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 尝试 nvidia-smi
	gpus, err := c.collectNvidiaGPU()
	if err == nil && len(gpus) > 0 {
		return gpus, nil
	}

	// 尝试 rocm-smi (AMD)
	gpus, err = c.collectAMDGPU()
	if err == nil && len(gpus) > 0 {
		return gpus, nil
	}

	return []GPUInfo{}, nil
}

func (c *Collector) collectNvidiaGPU() ([]GPUInfo, error) {
	cmd := exec.Command(c.nvidiaSmi,
		"--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw,fan.speed",
		"--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var gpus []GPUInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) < 8 {
			continue
		}

		id, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		temp, _ := strconv.Atoi(strings.TrimSpace(fields[2]))
		util, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
		memUsed, _ := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
		memTotal, _ := strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64)
		power, _ := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)
		fan, _ := strconv.Atoi(strings.TrimSpace(fields[7]))

		gpus = append(gpus, GPUInfo{
			ID:        id,
			Name:      strings.TrimSpace(fields[1]),
			Temp:      temp,
			Util:      util,
			MemUsed:   memUsed,
			MemTotal:  memTotal,
			PowerDraw: power,
			FanSpeed:  fan,
		})
	}

	return gpus, nil
}

func (c *Collector) collectAMDGPU() ([]GPUInfo, error) {
	cmd := exec.Command("rocm-smi", "--showtemp", "--showuse", "--showmem", "--showpower", "--showfan", "--csv")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析 AMD GPU 输出
	var gpus []GPUInfo
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) >= 6 {
			gpu := GPUInfo{
				ID:   i - 1,
				Name: fmt.Sprintf("AMD GPU %d", i-1),
			}
			// 解析各字段
			if len(fields) > 1 {
				gpu.Temp, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
			}
			gpus = append(gpus, gpu)
		}
	}

	return gpus, nil
}

// CollectNetworkFlow 采集网络流量.
func (c *Collector) CollectNetworkFlow() ([]NetworkFlow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Open(filepath.Join(c.procPath, "net", "dev"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var flows []NetworkFlow
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue
		}

		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}

		bytesIn, _ := strconv.ParseInt(fields[0], 10, 64)
		packetsIn, _ := strconv.ParseInt(fields[1], 10, 64)
		bytesOut, _ := strconv.ParseInt(fields[8], 10, 64)
		packetsOut, _ := strconv.ParseInt(fields[9], 10, 64)

		flows = append(flows, NetworkFlow{
			Interface:  iface,
			BytesIn:    bytesIn,
			BytesOut:   bytesOut,
			PacketsIn:  packetsIn,
			PacketsOut: packetsOut,
		})
	}

	return flows, nil
}

// CollectDiskIO 采集磁盘 I/O.
func (c *Collector) CollectDiskIO() ([]DiskIOInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Open(filepath.Join(c.procPath, "diskstats"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var disks []DiskIOInfo
	now := time.Now()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		device := fields[2]
		// 只处理 sd* 和 nvme* 设备
		if !strings.HasPrefix(device, "sd") && !strings.HasPrefix(device, "nvme") {
			continue
		}

		readSectors, _ := strconv.ParseInt(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseInt(fields[9], 10, 64)
		ioTime, _ := strconv.ParseInt(fields[12], 10, 64)

		disk := DiskIOInfo{
			Device: device,
		}

		// 计算 IOPS 和吞吐量
		if prev, ok := c.prevDiskIO[device]; ok {
			duration := now.Sub(prev.Timestamp).Seconds()
			if duration > 0 {
				readDelta := readSectors - prev.ReadSectors
				writeDelta := writeSectors - prev.WriteSectors
				ioTimeDelta := ioTime - prev.IoTime

				disk.ReadIOPS = int64(float64(readDelta) / duration)
				disk.WriteIOPS = int64(float64(writeDelta) / duration)
				disk.ReadBps = int64(float64(readDelta*512) / duration) // 512 bytes per sector
				disk.WriteBps = int64(float64(writeDelta*512) / duration)
				disk.BusyPerc = float64(ioTimeDelta) / duration / 10 // ioTime in ms
			}
		}

		c.prevDiskIO[device] = DiskIOCounters{
			ReadSectors:  readSectors,
			WriteSectors: writeSectors,
			IoTime:       ioTime,
			Timestamp:    now,
		}

		disks = append(disks, disk)
	}

	return disks, nil
}

// DiagnoseBottlenecks 瓶颈诊断.
func (c *Collector) DiagnoseBottlenecks() ([]BottleneckDiagnosis, error) {
	var diagnoses []BottleneckDiagnosis

	// 检查 CPU
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 简单诊断逻辑
	procs, err := c.CollectProcesses()
	if err == nil {
		for _, p := range procs {
			if p.CPU > 80 {
				diagnoses = append(diagnoses, BottleneckDiagnosis{
					Component:  "cpu",
					Severity:   "warning",
					Issue:      fmt.Sprintf("Process %s (PID: %d) high CPU usage: %.1f%%", p.Name, p.PID, p.CPU),
					Suggestion: "Consider optimizing or limiting this process",
				})
			}
			if p.Memory > 1024 { // > 1GB
				diagnoses = append(diagnoses, BottleneckDiagnosis{
					Component:  "memory",
					Severity:   "warning",
					Issue:      fmt.Sprintf("Process %s (PID: %d) high memory usage: %.1f MB", p.Name, p.PID, p.Memory),
					Suggestion: "Check for memory leaks or optimize memory usage",
				})
			}
		}
	}

	// 检查 GPU
	gpus, err := c.CollectGPU()
	if err == nil {
		for _, gpu := range gpus {
			if gpu.Temp > 85 {
				diagnoses = append(diagnoses, BottleneckDiagnosis{
					Component:  "gpu",
					Severity:   "critical",
					Issue:      fmt.Sprintf("GPU %d (%s) temperature too high: %d°C", gpu.ID, gpu.Name, gpu.Temp),
					Suggestion: "Improve cooling or reduce GPU workload",
				})
			}
			if gpu.Util > 95 {
				diagnoses = append(diagnoses, BottleneckDiagnosis{
					Component:  "gpu",
					Severity:   "warning",
					Issue:      fmt.Sprintf("GPU %d (%s) utilization critical: %d%%", gpu.ID, gpu.Name, gpu.Util),
					Suggestion: "Consider distributing workload across multiple GPUs",
				})
			}
		}
	}

	// 检查磁盘
	disks, err := c.CollectDiskIO()
	if err == nil {
		for _, disk := range disks {
			if disk.BusyPerc > 90 {
				diagnoses = append(diagnoses, BottleneckDiagnosis{
					Component:  "disk",
					Severity:   "warning",
					Issue:      fmt.Sprintf("Disk %s busy: %.1f%%", disk.Device, disk.BusyPerc),
					Suggestion: "Consider using SSD or distributing I/O load",
				})
			}
		}
	}

	if len(diagnoses) == 0 {
		diagnoses = append(diagnoses, BottleneckDiagnosis{
			Component:  "system",
			Severity:   "info",
			Issue:      "No bottlenecks detected",
			Suggestion: "System is running normally",
		})
	}

	return diagnoses, nil
}
