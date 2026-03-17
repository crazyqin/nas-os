package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Volume Docker 存储卷
type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	MountPoint string            `json:"mountPoint"`
	Created    time.Time         `json:"created"`
	Size       uint64            `json:"size"`
	SizeHuman  string            `json:"sizeHuman"`
	Labels     map[string]string `json:"labels"`
	Scope      string            `json:"scope"`
	Options    map[string]string `json:"options"`
	Containers []string          `json:"containers"` // 使用该卷的容器
}

// VolumeConfig 卷创建配置
type VolumeConfig struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver"` // "local", "nfs", "cifs"
	Labels   map[string]string `json:"labels"`
	Options  map[string]string `json:"options"`  // 驱动特定选项
	HostPath string            `json:"hostPath"` // 本地路径（bind mount）
}

// VolumeBackup 卷备份信息
type VolumeBackup struct {
	Name       string    `json:"name"`
	VolumeName string    `json:"volumeName"`
	BackupPath string    `json:"backupPath"`
	Size       uint64    `json:"size"`
	SizeHuman  string    `json:"sizeHuman"`
	Created    time.Time `json:"created"`
	Checksum   string    `json:"checksum"`
	Compressed bool      `json:"compressed"`
}

// VolumeManager 卷管理器
type VolumeManager struct {
	manager *Manager
}

// NewVolumeManager 创建卷管理器
func NewVolumeManager(mgr *Manager) *VolumeManager {
	return &VolumeManager{
		manager: mgr,
	}
}

// ListVolumes 列出所有卷
func (vm *VolumeManager) ListVolumes() ([]*Volume, error) {
	cmd := exec.Command("docker", "volume", "ls", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出卷：%w", err)
	}

	var volumes []*Volume
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			Driver string `json:"Driver"`
			Name   string `json:"Name"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// 获取详细信息
		volume, err := vm.GetVolume(raw.Name)
		if err != nil {
			volume = &Volume{
				Name:   raw.Name,
				Driver: raw.Driver,
			}
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

// GetVolume 获取卷详情
func (vm *VolumeManager) GetVolume(name string) (*Volume, error) {
	cmd := exec.Command("docker", "volume", "inspect", "--format", "{{json .}}", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取卷信息：%w", err)
	}

	var raw struct {
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Mountpoint string            `json:"Mountpoint"`
		CreatedAt  time.Time         `json:"CreatedAt"`
		Labels     map[string]string `json:"Labels"`
		Scope      string            `json:"Scope"`
		Options    map[string]string `json:"Options"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	volume := &Volume{
		Name:       raw.Name,
		Driver:     raw.Driver,
		MountPoint: raw.Mountpoint,
		Created:    raw.CreatedAt,
		Labels:     raw.Labels,
		Scope:      raw.Scope,
		Options:    raw.Options,
		Containers: make([]string, 0),
	}

	// 计算卷大小
	if size, err := vm.getVolumeSize(raw.Mountpoint); err == nil {
		volume.Size = size
		volume.SizeHuman = formatSize(size)
	}

	// 查找使用该卷的容器
	containers, err := vm.manager.ListContainers(true)
	if err == nil {
		for _, c := range containers {
			for _, v := range c.Volumes {
				if v.Source == raw.Name || v.Source == raw.Mountpoint {
					volume.Containers = append(volume.Containers, c.Name)
				}
			}
		}
	}

	return volume, nil
}

// CreateVolume 创建卷
func (vm *VolumeManager) CreateVolume(config *VolumeConfig) (*Volume, error) {
	args := []string{"volume", "create"}

	// 驱动
	if config.Driver != "" {
		args = append(args, "--driver", config.Driver)
	}

	// 标签
	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// 驱动选项
	for k, v := range config.Options {
		args = append(args, "--opt", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, config.Name)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建卷失败：%w, %s", err, string(output))
	}

	volumeName := strings.TrimSpace(string(output))
	return vm.GetVolume(volumeName)
}

// RemoveVolume 删除卷
func (vm *VolumeManager) RemoveVolume(name string, force bool) error {
	args := []string{"volume", "rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除卷失败：%w, %s", err, string(output))
	}
	return nil
}

// BackupVolume 备份卷
func (vm *VolumeManager) BackupVolume(volumeName, backupPath string, compress bool) (*VolumeBackup, error) {
	// 获取卷信息
	_, err := vm.GetVolume(volumeName)
	if err != nil {
		return nil, err
	}

	// 确保备份目录存在
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败：%w", err)
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s", volumeName, timestamp)
	var backupFile string

	if compress {
		backupFile = filepath.Join(backupPath, backupName+".tar.gz")
	} else {
		backupFile = filepath.Join(backupPath, backupName+".tar")
	}

	// 使用 docker run 临时容器备份卷
	tarCmd := "tar cf - -C /volume ."
	if compress {
		tarCmd = "tar czf - -C /volume ."
	}

	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/volume", volumeName),
		"-v", fmt.Sprintf("%s:/backup", backupPath),
		"alpine",
		"sh", "-c", fmt.Sprintf("%s > /backup/%s", tarCmd, filepath.Base(backupFile)),
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("备份卷失败：%w, %s", err, string(output))
	}

	// 获取备份文件大小
	size, err := vm.getFileSize(backupFile)
	if err != nil {
		size = 0
	}

	// 计算校验和
	checksum, err := vm.calculateChecksum(backupFile)
	if err != nil {
		checksum = ""
	}

	backup := &VolumeBackup{
		Name:       backupName,
		VolumeName: volumeName,
		BackupPath: backupFile,
		Size:       size,
		SizeHuman:  formatSize(size),
		Created:    time.Now(),
		Checksum:   checksum,
		Compressed: compress,
	}

	return backup, nil
}

// RestoreVolume 从备份恢复卷
func (vm *VolumeManager) RestoreVolume(backupPath, volumeName string) error {
	// 检查备份文件是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在：%s", backupPath)
	}

	// 确保卷存在
	_, err := vm.GetVolume(volumeName)
	if err != nil {
		// 卷不存在，创建它
		_, err = vm.CreateVolume(&VolumeConfig{Name: volumeName})
		if err != nil {
			return fmt.Errorf("创建卷失败：%w", err)
		}
	}

	// 判断是否压缩
	compressed := strings.HasSuffix(backupPath, ".gz")

	// 使用临时容器恢复
	tarCmd := "tar xf - -C /volume"
	if compressed {
		tarCmd = "tar xzf - -C /volume"
	}

	args := []string{
		"run", "--rm",
		"-i",
		"-v", fmt.Sprintf("%s:/volume", volumeName),
		"alpine",
		"sh", "-c", tarCmd,
	}

	cmd := exec.Command("docker", args...)
	inputFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("打开备份文件失败：%w", err)
	}
	defer func() { _ = inputFile.Close() }()

	cmd.Stdin = inputFile
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("恢复卷失败：%w, %s", err, string(output))
	}

	return nil
}

// PruneVolumes 清理未使用的卷
func (vm *VolumeManager) PruneVolumes() (uint64, error) {
	cmd := exec.Command("docker", "volume", "prune", "-f")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("清理卷失败：%w", err)
	}

	// 解析回收的空间
	var reclaimed uint64
	outputStr := string(output)
	if strings.Contains(outputStr, "Total reclaimed space:") {
		parts := strings.Split(outputStr, "Total reclaimed space:")
		if len(parts) > 1 {
			sizeStr := strings.TrimSpace(parts[1])
			reclaimed = parseSize(sizeStr)
		}
	}

	return reclaimed, nil
}

// getVolumeSize 获取卷大小
func (vm *VolumeManager) getVolumeSize(path string) (uint64, error) {
	cmd := exec.Command("du", "-sb", path)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		var size uint64
		if _, err := fmt.Sscanf(parts[0], "%d", &size); err != nil {
			return 0, fmt.Errorf("无法解析大小")
		}
		return size, nil
	}

	return 0, fmt.Errorf("无法解析大小")
}

// getFileSize 获取文件大小
func (vm *VolumeManager) getFileSize(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return uint64(info.Size()), nil
}

// calculateChecksum 计算文件校验和
func (vm *VolumeManager) calculateChecksum(path string) (string, error) {
	cmd := exec.Command("sha256sum", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("无法计算校验和")
}

// ListBackups 列出卷备份
func (vm *VolumeManager) ListBackups(backupDir string) ([]*VolumeBackup, error) {
	var backups []*VolumeBackup

	files, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasSuffix(name, ".tar") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}

		path := filepath.Join(backupDir, name)
		info, err := file.Info()
		if err != nil {
			continue
		}

		// 从文件名提取卷名
		volumeName := strings.TrimSuffix(name, ".tar.gz")
		volumeName = strings.TrimSuffix(volumeName, ".tar")
		parts := strings.Split(volumeName, "_")
		if len(parts) > 0 {
			volumeName = parts[0]
		}

		backups = append(backups, &VolumeBackup{
			Name:       name,
			VolumeName: volumeName,
			BackupPath: path,
			Size:       uint64(info.Size()),
			SizeHuman:  formatSize(uint64(info.Size())),
			Created:    info.ModTime(),
			Compressed: strings.HasSuffix(name, ".gz"),
		})
	}

	return backups, nil
}
