package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ExtendedManager 扩展的 VM 管理器（包含 API 所需的方法）.
type ExtendedManager struct {
	*Manager
	snapshotManager *SnapshotManager
}

// NewExtendedManager 创建扩展 VM 管理器.
func NewExtendedManager(storagePath string, logger *zap.Logger) (*ExtendedManager, error) {
	mgr, err := NewManager(storagePath, logger)
	if err != nil {
		return nil, err
	}

	snapMgr, err := NewSnapshotManager(storagePath, mgr, logger)
	if err != nil {
		logger.Warn("创建快照管理器失败", zap.Error(err))
	}

	return &ExtendedManager{
		Manager:         mgr,
		snapshotManager: snapMgr,
	}, nil
}

// ListSnapshots 列出 VM 快照.
func (m *ExtendedManager) ListSnapshots(vmID string) []*Snapshot {
	if m.snapshotManager == nil {
		return nil
	}
	return m.snapshotManager.ListSnapshots(vmID)
}

// CreateSnapshot 创建快照.
func (m *ExtendedManager) CreateSnapshot(ctx context.Context, vmID, name, description string) (*Snapshot, error) {
	if m.snapshotManager == nil {
		return nil, fmt.Errorf("快照管理器未初始化")
	}
	return m.snapshotManager.CreateSnapshot(ctx, vmID, name, description)
}

// GetSnapshot 获取快照详情.
func (m *ExtendedManager) GetSnapshot(vmID, snapshotID string) (*Snapshot, error) {
	if m.snapshotManager == nil {
		return nil, fmt.Errorf("快照管理器未初始化")
	}
	return m.snapshotManager.GetSnapshot(snapshotID)
}

// DeleteSnapshot 删除快照.
func (m *ExtendedManager) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error {
	if m.snapshotManager == nil {
		return fmt.Errorf("快照管理器未初始化")
	}
	return m.snapshotManager.DeleteSnapshot(ctx, snapshotID)
}

// RestoreSnapshot 恢复快照.
func (m *ExtendedManager) RestoreSnapshot(ctx context.Context, vmID, snapshotID string) error {
	if m.snapshotManager == nil {
		return fmt.Errorf("快照管理器未初始化")
	}
	return m.snapshotManager.RestoreSnapshot(ctx, snapshotID)
}

// CreateTemplate 创建自定义模板.
func (m *Manager) CreateTemplate(name, description string, vmType Type, cpu int, memory, diskSize uint64, network, os string, tags map[string]string) (*Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证名称
	for _, r := range name {
		if !isSafeChar(r) {
			return nil, fmt.Errorf("模板名称只能包含字母、数字、下划线和连字符")
		}
	}

	// 检查名称重复
	for _, tpl := range m.templates {
		if tpl.Name == name {
			return nil, fmt.Errorf("模板名称 %s 已存在", name)
		}
	}

	templateID := "tpl-" + uuid.New().String()[:8]
	now := time.Now()

	tpl := &Template{
		ID:          templateID,
		Name:        name,
		Description: description,
		Type:        vmType,
		CPU:         cpu,
		Memory:      memory,
		DiskSize:    diskSize,
		Network:     network,
		OS:          os,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        tags,
	}

	// 验证配置
	if cpu < 1 {
		return nil, fmt.Errorf("CPU 核心数至少为 1")
	}
	if memory < 256 {
		return nil, fmt.Errorf("内存至少为 256MB")
	}
	if diskSize < 1 {
		return nil, fmt.Errorf("磁盘大小至少为 1GB")
	}

	m.templates[templateID] = tpl

	// 保存模板配置
	if err := m.saveTemplate(tpl); err != nil {
		delete(m.templates, templateID)
		return nil, err
	}

	m.logger.Info("模板创建成功", zap.String("templateId", templateID), zap.String("name", name))
	return tpl, nil
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, exists := m.templates[templateID]
	if !exists {
		return fmt.Errorf("模板 %s 不存在", templateID)
	}
	_ = tpl // 用于检查存在性

	// 不允许删除内置模板
	if strings.HasPrefix(templateID, "tpl-ubuntu") ||
		strings.HasPrefix(templateID, "tpl-debian") ||
		strings.HasPrefix(templateID, "tpl-windows") {
		return fmt.Errorf("内置模板不允许删除")
	}

	// 删除模板文件
	templatePath := filepath.Join(m.storagePath, "templates", templateID+".json")
	_ = os.Remove(templatePath)

	delete(m.templates, templateID)

	m.logger.Info("模板删除成功", zap.String("templateId", templateID))
	return nil
}

// saveTemplate 保存模板配置.
func (m *Manager) saveTemplate(tpl *Template) error {
	templateDir := filepath.Join(m.storagePath, "templates")
	if err := os.MkdirAll(templateDir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return err
	}

	templatePath := filepath.Join(templateDir, tpl.ID+".json")
	return os.WriteFile(templatePath, data, 0640)
}

// ListISOs 列出 ISO 镜像.
func (m *Manager) ListISOs() []*ISOImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var isos []*ISOImage

	// 遍历 ISO 目录
	if _, err := os.Stat(m.isoPath); err == nil {
		files, err := os.ReadDir(m.isoPath)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(file.Name(), ".iso") {
					info, err := file.Info()
					if err != nil {
						continue
					}

					isoID := "iso-" + strings.ReplaceAll(file.Name(), ".iso", "")
					isos = append(isos, &ISOImage{
						ID:         isoID,
						Name:       file.Name(),
						Path:       filepath.Join(m.isoPath, file.Name()),
						Size:       uint64(info.Size()),
						CreatedAt:  info.ModTime(),
						UpdatedAt:  info.ModTime(),
						IsUploaded: true,
						OS:         detectOSFromISOName(file.Name()),
					})
				}
			}
		}
	}

	return isos
}

// GetISO 获取 ISO 详情.
func (m *Manager) GetISO(isoID string) (*ISOImage, error) {
	isos := m.ListISOs()
	for _, iso := range isos {
		if iso.ID == isoID {
			return iso, nil
		}
	}
	return nil, fmt.Errorf("ISO %s 不存在", isoID)
}

// DownloadISO 从 URL 下载 ISO.
func (m *Manager) DownloadISO(ctx context.Context, name, url string) (*ISOImage, error) {
	// 验证名称
	if !strings.HasSuffix(name, ".iso") {
		name += ".iso"
	}

	isoPath := filepath.Join(m.isoPath, name)

	// 使用 wget 或 curl 下载
	cmd := exec.CommandContext(ctx, "wget", "-O", isoPath, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("下载 ISO 失败：%w, %s", err, string(output))
	}

	// 获取文件信息
	info, err := os.Stat(isoPath)
	if err != nil {
		return nil, err
	}

	isoID := "iso-" + strings.ReplaceAll(name, ".iso", "")
	iso := &ISOImage{
		ID:         isoID,
		Name:       name,
		Path:       isoPath,
		Size:       uint64(info.Size()),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		IsUploaded: false,
		URL:        url,
		OS:         detectOSFromISOName(name),
	}

	m.logger.Info("ISO 下载成功", zap.String("isoId", isoID), zap.String("name", name))
	return iso, nil
}

// DeleteISO 删除 ISO.
func (m *Manager) DeleteISO(isoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	iso, err := m.GetISO(isoID)
	if err != nil {
		return err
	}

	// 检查是否有 VM 正在使用
	for _, vm := range m.vms {
		if vm.ISOPath == iso.Path {
			return fmt.Errorf("ISO 正被 VM %s 使用", vm.Name)
		}
	}

	// 删除文件
	if err := os.Remove(iso.Path); err != nil {
		return fmt.Errorf("删除 ISO 文件失败：%w", err)
	}

	m.logger.Info("ISO 删除成功", zap.String("isoId", isoID))
	return nil
}

// GetUploadInfo 获取上传信息.
func (m *Manager) GetUploadInfo(name string) map[string]interface{} {
	if !strings.HasSuffix(name, ".iso") {
		name += ".iso"
	}

	return map[string]interface{}{
		"uploadPath": filepath.Join(m.isoPath, name),
		"method":     "POST",
		"maxSize":    "10GB",
		"supported":  []string{"iso"},
	}
}

// detectOSFromISOName 从 ISO 名称检测操作系统.
func detectOSFromISOName(name string) string {
	name = strings.ToLower(name)

	if strings.Contains(name, "ubuntu") {
		if strings.Contains(name, "22.04") || strings.Contains(name, "2204") {
			return "ubuntu-2204"
		}
		if strings.Contains(name, "24.04") || strings.Contains(name, "2404") {
			return "ubuntu-2404"
		}
		return "ubuntu"
	}

	if strings.Contains(name, "debian") {
		if strings.Contains(name, "11") {
			return "debian-11"
		}
		if strings.Contains(name, "12") {
			return "debian-12"
		}
		return "debian"
	}

	if strings.Contains(name, "windows") {
		if strings.Contains(name, "11") {
			return "windows-11"
		}
		if strings.Contains(name, "10") {
			return "windows-10"
		}
		return "windows"
	}

	if strings.Contains(name, "centos") || strings.Contains(name, "rhel") {
		if strings.Contains(name, "8") || strings.Contains(name, "9") {
			return "rhel"
		}
		return "centos"
	}

	if strings.Contains(name, "alpine") {
		return "alpine"
	}

	return "other"
}

// CloneVM 从模板克隆 VM.
func (m *Manager) CloneVM(ctx context.Context, templateID, name string, overrides map[string]interface{}) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	// 验证名称
	for _, r := range name {
		if !isSafeChar(r) {
			return nil, fmt.Errorf("VM 名称只能包含字母、数字、下划线和连字符")
		}
	}

	// 检查名称重复
	for _, vm := range m.vms {
		if vm.Name == name {
			return nil, fmt.Errorf("VM 名称 %s 已存在", name)
		}
	}

	// 构建配置
	config := Config{
		Name:        name,
		Description: fmt.Sprintf("从模板 %s 克隆", tpl.Name),
		Type:        tpl.Type,
		CPU:         tpl.CPU,
		Memory:      tpl.Memory,
		DiskSize:    tpl.DiskSize,
		Network:     tpl.Network,
		VNCEnabled:  true,
		Tags:        tpl.Tags,
	}

	// 应用覆盖配置
	if cpu, ok := overrides["cpu"].(int); ok && cpu > 0 {
		config.CPU = cpu
	}
	if mem, ok := overrides["memory"].(uint64); ok && mem >= 256 {
		config.Memory = mem
	}
	if disk, ok := overrides["diskSize"].(uint64); ok && disk >= 1 {
		config.DiskSize = disk
	}
	if network, ok := overrides["network"].(string); ok {
		config.Network = network
	}
	if iso, ok := overrides["isoPath"].(string); ok {
		config.ISOPath = iso
	}

	// 解锁后调用 CreateVM（CreateVM 会自己加锁）
	m.mu.Unlock()
	vm, err := m.CreateVM(ctx, config)
	m.mu.Lock()

	if err != nil {
		return nil, err
	}

	// 添加克隆标记
	if vm.Tags == nil {
		vm.Tags = make(map[string]string)
	}
	vm.Tags["cloned_from"] = templateID
	vm.Tags["template_name"] = tpl.Name

	m.logger.Info("VM 克隆成功", zap.String("vmId", vm.ID), zap.String("templateId", templateID))
	return vm, nil
}

// MigrateVM 迁移 VM 到另一主机（离线迁移）.
func (m *Manager) MigrateVM(ctx context.Context, vmID, targetHost string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == StatusRunning {
		return fmt.Errorf("运行中的 VM 需要先停止才能迁移")
	}

	// 验证目标主机
	for _, r := range targetHost {
		if !isSafeChar(r) && r != '.' && r != ':' {
			return fmt.Errorf("目标主机地址包含不安全字符")
		}
	}

	// 如果 libvirt 可用，使用 virsh migrate
	if m.libvirtAvailable {
		// #nosec G204 -- vm.Name and targetHost validated
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "migrate", vm.Name, "qemu+tcp://"+targetHost+"/system", "--offline")
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("迁移失败", zap.Error(err), zap.String("output", string(output)))
			return fmt.Errorf("迁移失败：%w", err)
		}
	}

	m.logger.Info("VM 迁移成功", zap.String("vmId", vmID), zap.String("targetHost", targetHost))
	return nil
}

// ExportVM 导出 VM 为模板.
func (m *Manager) ExportVM(ctx context.Context, vmID, templateName string) (*Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == StatusRunning {
		return nil, fmt.Errorf("运行中的 VM 无法导出")
	}

	// 验证模板名称
	for _, r := range templateName {
		if !isSafeChar(r) {
			return nil, fmt.Errorf("模板名称只能包含字母、数字、下划线和连字符")
		}
	}

	m.mu.Unlock()
	tpl, err := m.CreateTemplate(templateName, fmt.Sprintf("从 VM %s 导出", vm.Name), vm.Type, vm.CPU, vm.Memory, vm.DiskSize, vm.Network, vm.Tags["os"], vm.Tags)
	m.mu.Lock()

	if err != nil {
		return nil, err
	}

	m.logger.Info("VM 导出为模板成功", zap.String("vmId", vmID), zap.String("templateId", tpl.ID))
	return tpl, nil
}

// BackupVM 备份 VM.
func (m *Manager) BackupVM(ctx context.Context, vmID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return "", fmt.Errorf("VM %s 不存在", vmID)
	}
	_ = vm // 用于检查存在性

	backupDir := filepath.Join(m.storagePath, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return "", err
	}

	backupID := "backup-" + vmID + "-" + time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, backupID+".tar.gz")

	// 使用 tar 打包 VM 目录
	vmDir := filepath.Join(m.storagePath, vmID)
	// #nosec G204 -- backupPath and vmDir are internally generated
	cmd := exec.CommandContext(ctx, "tar", "-czf", backupPath, "-C", filepath.Dir(vmDir), filepath.Base(vmDir))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("备份失败：%w, %s", err, string(output))
	}

	m.logger.Info("VM 备份成功", zap.String("vmId", vmID), zap.String("backupPath", backupPath))
	return backupPath, nil
}

// RestoreVMBackup 从备份恢复 VM.
func (m *Manager) RestoreVMBackup(ctx context.Context, backupPath string) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证备份文件
	if _, err := os.Stat(backupPath); err != nil {
		return nil, fmt.Errorf("备份文件不存在")
	}

	// 解压到临时目录
	tempDir := filepath.Join(m.storagePath, "restore-temp-"+time.Now().Format("20060102-150405"))
	defer os.RemoveAll(tempDir)

	// #nosec G204 -- paths are internally generated
	cmd := exec.CommandContext(ctx, "tar", "-xzf", backupPath, "-C", tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("解压失败：%w, %s", err, string(output))
	}

	// 查找 VM 目录
	files, err := os.ReadDir(tempDir)
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("无法找到 VM 数据")
	}

	vmDir := filepath.Join(tempDir, files[0].Name())
	configPath := filepath.Join(vmDir, "config.json")

	// 加载配置
	vm, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// 生成新的 ID（避免冲突）
	newID := "vm-" + uuid.New().String()[:8]
	vm.ID = newID
	vm.CreatedAt = time.Now()
	vm.UpdatedAt = time.Now()
	vm.Status = StatusStopped

	// 移动到正式目录
	newDir := filepath.Join(m.storagePath, newID)
	if err := os.Rename(vmDir, newDir); err != nil {
		return nil, fmt.Errorf("移动 VM 目录失败：%w", err)
	}

	// 更新磁盘路径
	vm.DiskPath = filepath.Join(newDir, "disk.qcow2")

	// 保存配置
	if err := m.saveConfig(vm); err != nil {
		return nil, err
	}

	m.vms[newID] = vm

	m.logger.Info("VM 从备份恢复成功", zap.String("vmId", newID), zap.String("backupPath", backupPath))
	return vm, nil
}
