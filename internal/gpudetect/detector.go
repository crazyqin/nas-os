package gpudetect

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// GPUType represents the type of GPU.
type GPUType string

const (
	GPUNvidia  GPUType = "nvidia"
	GPUAMD     GPUType = "amd"
	GPUIntel   GPUType = "intel"
	GPUUnknown GPUType = "unknown"
)

// GPUInfo contains information about a detected GPU.
type GPUInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       GPUType `json:"type"`
	VRAMMB     int     `json:"vram_mb"`
	TempC      int     `json:"temp_c"`
	UtilPct    int     `json:"util_pct"`
	MemUsedMB  int     `json:"mem_used_mb"`
	MemFreeMB  int     `json:"mem_free_mb"`
	Driver     string  `json:"driver"`
	BusID      string  `json:"bus_id"`
	CUDACores  int     `json:"cuda_cores,omitempty"`
	ComputeCap string  `json:"compute_cap,omitempty"`
}

// Detector handles GPU detection and monitoring.
type Detector struct {
	mu       sync.RWMutex
	gpus     []GPUInfo
	interval int // monitoring interval in seconds
}

// NewDetector creates a new GPU detector.
func NewDetector(interval int) *Detector {
	return &Detector{
		interval: interval,
	}
}

// DetectAll discovers all available GPUs.
func (d *Detector) DetectAll(ctx context.Context) ([]GPUInfo, error) {
	var allGPUs []GPUInfo

	// Detect NVIDIA GPUs
	nvidiaGPUs, err := d.detectNvidia(ctx)
	if err == nil {
		allGPUs = append(allGPUs, nvidiaGPUs...)
	}

	// Detect AMD GPUs
	amdGPUs, err := d.detectAMD(ctx)
	if err == nil {
		allGPUs = append(allGPUs, amdGPUs...)
	}

	// Detect Intel GPUs
	intelGPUs, err := d.detectIntel(ctx)
	if err == nil {
		allGPUs = append(allGPUs, intelGPUs...)
	}

	d.mu.Lock()
	d.gpus = allGPUs
	d.mu.Unlock()

	return allGPUs, nil
}

// GetGPUs returns cached GPU information.
func (d *Detector) GetGPUs() []GPUInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.gpus
}

// detectNVIDIA discovers NVIDIA GPUs using nvidia-smi.
func (d *Detector) detectNvidia(ctx context.Context) ([]GPUInfo, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,memory.total,memory.used,memory.free,temperature.gpu,utilization.gpu,driver_version,pci.bus_id",
		"--format=csv,noheader,nounits")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi not available: %w", err)
	}

	var gpus []GPUInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 9 {
			continue
		}

		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		id := fields[0]
		name := fields[1]
		vramMB, _ := strconv.Atoi(fields[2])
		memUsedMB, _ := strconv.Atoi(fields[3])
		memFreeMB, _ := strconv.Atoi(fields[4])
		tempC, _ := strconv.Atoi(fields[5])
		utilPct, _ := strconv.Atoi(fields[6])
		driver := fields[7]
		busID := fields[8]

		// Extract compute capability if available
		computeCap := ""
		cudaCores := 0

		gpu := GPUInfo{
			ID:         id,
			Name:       name,
			Type:       GPUNvidia,
			VRAMMB:     vramMB,
			TempC:      tempC,
			UtilPct:    utilPct,
			MemUsedMB:  memUsedMB,
			MemFreeMB:  memFreeMB,
			Driver:     driver,
			BusID:      busID,
			CUDACores:  cudaCores,
			ComputeCap: computeCap,
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// detectAMD discovers AMD GPUs using rocm-smi.
func (d *Detector) detectAMD(ctx context.Context) ([]GPUInfo, error) {
	cmd := exec.CommandContext(ctx, "rocm-smi",
		"--showid", "--showproductname", "--showmeminfo", "vram",
		"--showtemp", "--showuse", "--csv")

	out, err := cmd.Output()
	if err != nil {
		// Try alternative detection via sysfs
		return d.detectAMDSysfs(ctx)
	}

	var gpus []GPUInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	if len(lines) < 2 {
		return nil, fmt.Errorf("no AMD GPU data found")
	}

	// Parse CSV header and data
	header := strings.Split(lines[0], ",")

	for _, line := range lines[1:] {
		fields := strings.Split(line, ",")
		if len(fields) < len(header) {
			continue
		}

		gpu := GPUInfo{
			Type: GPUAMD,
		}

		for i, h := range header {
			h = strings.TrimSpace(h)
			val := strings.TrimSpace(fields[i])

			switch {
			case strings.Contains(h, "device"):
				gpu.ID = val
			case strings.Contains(h, "card") && strings.Contains(h, "series"):
				gpu.Name = val
			case strings.Contains(h, "Temperature"):
				gpu.TempC, _ = strconv.Atoi(val)
			case strings.Contains(h, "GPU use") || strings.Contains(h, "use"):
				gpu.UtilPct, _ = strconv.Atoi(strings.TrimSuffix(val, "%"))
			}
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// detectAMDSysfs detects AMD GPUs via sysfs.
func (d *Detector) detectAMDSysfs(ctx context.Context) ([]GPUInfo, error) {
	// Try reading from /sys/class/drm/card*/
	// This is a simplified implementation
	return nil, fmt.Errorf("AMD GPU sysfs detection not implemented")
}

// detectIntel discovers Intel GPUs.
func (d *Detector) detectIntel(ctx context.Context) ([]GPUInfo, error) {
	// Check for Intel GPU via lspci
	cmd := exec.CommandContext(ctx, "lspci", "-nn")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lspci not available: %w", err)
	}

	re := regexp.MustCompile(`(?i)Intel.*(?:HD|UHD|Iris|Arc).*Graphics`)

	var gpus []GPUInfo
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if re.MatchString(line) {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				continue
			}

			busID := parts[0]
			name := strings.TrimSpace(parts[1])

			gpu := GPUInfo{
				ID:    busID,
				Name:  name,
				Type:  GPUIntel,
				BusID: busID,
			}

			gpus = append(gpus, gpu)
		}
	}

	return gpus, nil
}

// UpdateStats updates runtime statistics for all detected GPUs.
func (d *Detector) UpdateStats(ctx context.Context) error {
	d.mu.RLock()
	gpus := d.gpus
	d.mu.RUnlock()

	for i := range gpus {
		switch gpus[i].Type {
		case GPUNvidia:
			d.updateNvidiaStats(ctx, &gpus[i])
		case GPUAMD:
			d.updateAMDStats(ctx, &gpus[i])
		}
	}

	d.mu.Lock()
	d.gpus = gpus
	d.mu.Unlock()

	return nil
}

// updateNvidiaStats updates NVIDIA GPU statistics.
func (d *Detector) updateNvidiaStats(ctx context.Context, gpu *GPUInfo) {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"-i", gpu.ID,
		"--query-gpu=memory.used,memory.free,temperature.gpu,utilization.gpu",
		"--format=csv,noheader,nounits")

	out, err := cmd.Output()
	if err != nil {
		return
	}

	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(fields) >= 4 {
		gpu.MemUsedMB, _ = strconv.Atoi(strings.TrimSpace(fields[0]))
		gpu.MemFreeMB, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
		gpu.TempC, _ = strconv.Atoi(strings.TrimSpace(fields[2]))
		gpu.UtilPct, _ = strconv.Atoi(strings.TrimSpace(fields[3]))
	}
}

// updateAMDStats updates AMD GPU statistics.
func (d *Detector) updateAMDStats(ctx context.Context, gpu *GPUInfo) {
	cmd := exec.CommandContext(ctx, "rocm-smi",
		"-d", gpu.ID,
		"--showtemp", "--showuse", "--showmeminfo", "vram",
		"--csv")

	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 2 {
		// Parse AMD stats
		// Simplified implementation
	}
}

// GetBackendForGPU returns the appropriate compute backend for a GPU.
func GetBackendForGPU(gpu GPUInfo) string {
	switch gpu.Type {
	case GPUNvidia:
		if gpu.ComputeCap >= "6.0" {
			return "cuda"
		}
		return "cuda-legacy"
	case GPUAMD:
		return "rocm"
	case GPUIntel:
		return "opencl"
	default:
		return "cpu"
	}
}
