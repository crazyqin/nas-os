package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// isSafeString 检查字符串是否只包含安全字符
func isSafeString(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ' ') {
			return false
		}
	}
	return true
}

// sanitizeDescription 清理描述字符串，移除危险字符
func sanitizeDescription(s string) string {
	// 移除可能的命令注入字符
	replacer := strings.NewReplacer(
		";", "",
		"|", "",
		"&", "",
		"$", "",
		"`", "",
		"(", "",
		")", "",
		"<", "",
		">", "",
		"\n", " ",
		"\r", " ",
		"\x00", "",
	)
	return replacer.Replace(s)
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	mu               sync.RWMutex
	storagePath      string
	snapshots        map[string]*VMSnapshot
	vmManager        *Manager
	logger           *zap.Logger
	libvirtAvailable bool
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(storagePath string, vmManager *Manager, logger *zap.Logger) (*SnapshotManager, error) {
	if storagePath == "" {
		storagePath = DefaultVMStoragePath
	}

	snapshotPath := filepath.Join(storagePath, "snapshots")
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败：%w", err)
	}

	m := &SnapshotManager{
		storagePath:      storagePath,
		snapshots:        make(map[string]*VMSnapshot),
		vmManager:        vmManager,
		logger:           logger,
		libvirtAvailable: vmManager.libvirtAvailable,
	}

	// 加载现有快照
	if err := m.loadSnapshots(); err != nil {
		logger.Warn("加载快照失败", zap.Error(err))
	}

	return m, nil
}

// loadSnapshots 加载现有快照
func (m *SnapshotManager) loadSnapshots() error {
	snapshotPath := filepath.Join(m.storagePath, "snapshots")

	files, err := os.ReadDir(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(snapshotPath, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var snapshot VMSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		m.snapshots[snapshot.ID] = &snapshot
	}

	return nil
}

// CreateSnapshot 创建虚拟机快照
func (m *SnapshotManager) CreateSnapshot(ctx context.Context, vmID string, name, description string) (*VMSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取 VM 信息
	vm, err := m.vmManager.GetVM(vmID)
	if err != nil {
		return nil, err
	}

	// 输入验证：防止命令注入
	if !isSafeString(name) {
		return nil, fmt.Errorf("快照名称包含不安全的字符")
	}

	// 清理描述字符串
	description = sanitizeDescription(description)

	// 检查 VM 状态（运行中也可以创建快照，但可能需要 quiesce）
	if vm.Status == VMStatusCreating || vm.Status == VMStatusDeleting {
		return nil, fmt.Errorf("VM 当前状态无法创建快照")
	}

	snapshotID := "snap-" + uuid.New().String()[:8]
	now := time.Now()

	snapshot := &VMSnapshot{
		ID:          snapshotID,
		VMID:        vmID,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		Status:      "creating",
	}

	// 创建快照目录
	snapshotDir := filepath.Join(m.storagePath, "snapshots", snapshotID)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败：%w", err)
	}

	// 使用 libvirt 创建快照
	if m.libvirtAvailable {
		snapshotName := fmt.Sprintf("%s_%s", vm.Name, snapshotID)
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "snapshot-create-as", vm.Name, snapshotName, "--description", description)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("libvirt 快照创建失败", zap.Error(err), zap.String("output", string(output)))
			// 继续尝试手动方式
		}
	}

	// 复制磁盘文件（简单实现，生产环境应使用 qcow2 内部快照）
	if vm.DiskPath != "" {
		snapshotDiskPath := filepath.Join(snapshotDir, "disk.qcow2")
		cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-f", "qcow2", "-O", "qcow2", vm.DiskPath, snapshotDiskPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("磁盘快照创建失败", zap.Error(err), zap.String("output", string(output)))
			// 不返回错误，继续保存快照元数据
		} else {
			// 获取快照大小
			info, err := os.Stat(snapshotDiskPath)
			if err == nil {
				snapshot.Size = uint64(info.Size())
			}
		}
	}

	// 保存 VM 配置
	vmConfigPath := filepath.Join(m.storagePath, vmID, "config.json")
	if _, err := os.Stat(vmConfigPath); err == nil {
		snapshotConfigPath := filepath.Join(snapshotDir, "vm-config.json")
		cmd := exec.CommandContext(ctx, "cp", vmConfigPath, snapshotConfigPath)
		cmd.Run()
	}

	// 保存快照元数据
	snapshot.Status = "ready"
	if err := m.saveSnapshot(snapshot); err != nil {
		os.RemoveAll(snapshotDir)
		return nil, fmt.Errorf("保存快照元数据失败：%w", err)
	}

	m.snapshots[snapshotID] = snapshot

	m.logger.Info("快照创建成功", zap.String("snapshotId", snapshotID), zap.String("vmId", vmID))

	return snapshot, nil
}

// ListSnapshots 获取 VM 的所有快照
func (m *SnapshotManager) ListSnapshots(vmID string) []*VMSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]*VMSnapshot, 0)
	for _, snapshot := range m.snapshots {
		if snapshot.VMID == vmID {
			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots
}

// GetSnapshot 获取快照信息
func (m *SnapshotManager) GetSnapshot(snapshotID string) (*VMSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	return snapshot, nil
}

// RestoreSnapshot 恢复虚拟机快照
func (m *SnapshotManager) RestoreSnapshot(ctx context.Context, snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	if snapshot.Status != "ready" {
		return fmt.Errorf("快照状态不可恢复")
	}

	// 获取 VM 信息
	vm, err := m.vmManager.GetVM(snapshot.VMID)
	if err != nil {
		return err
	}

	// 如果 VM 在运行，先停止
	if vm.Status == VMStatusRunning {
		if err := m.vmManager.StopVM(ctx, snapshot.VMID, true); err != nil {
			return fmt.Errorf("停止 VM 失败：%w", err)
		}
	}

	snapshot.Status = "restoring"
	m.saveSnapshot(snapshot)

	// 恢复磁盘文件
	snapshotDiskPath := filepath.Join(m.storagePath, "snapshots", snapshotID, "disk.qcow2")
	if _, err := os.Stat(snapshotDiskPath); err == nil {
		cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-f", "qcow2", "-O", "qcow2", snapshotDiskPath, vm.DiskPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("磁盘恢复失败", zap.Error(err), zap.String("output", string(output)))
			snapshot.Status = "ready"
			m.saveSnapshot(snapshot)
			return fmt.Errorf("恢复磁盘失败：%w", err)
		}
	}

	// 使用 libvirt 恢复
	if m.libvirtAvailable {
		snapshotName := fmt.Sprintf("%s_%s", vm.Name, snapshotID)
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "snapshot-revert", vm.Name, snapshotName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("libvirt 快照恢复失败", zap.Error(err), zap.String("output", string(output)))
		}
	}

	snapshot.Status = "ready"
	m.saveSnapshot(snapshot)

	m.logger.Info("快照恢复成功", zap.String("snapshotId", snapshotID), zap.String("vmId", snapshot.VMID))

	return nil
}

// DeleteSnapshot 删除快照
func (m *SnapshotManager) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	// 获取 VM 信息
	vm, err := m.vmManager.GetVM(snapshot.VMID)
	if err == nil && m.libvirtAvailable {
		// 删除 libvirt 快照
		snapshotName := fmt.Sprintf("%s_%s", vm.Name, snapshotID)
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "snapshot-delete", vm.Name, snapshotName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warn("libvirt 快照删除失败", zap.Error(err), zap.String("output", string(output)))
		}
	}

	// 删除快照目录
	snapshotDir := filepath.Join(m.storagePath, "snapshots", snapshotID)
	if err := os.RemoveAll(snapshotDir); err != nil {
		m.logger.Warn("删除快照目录失败", zap.Error(err))
	}

	// 删除快照元数据文件
	snapshotFile := filepath.Join(m.storagePath, "snapshots", snapshotID+".json")
	os.Remove(snapshotFile)

	delete(m.snapshots, snapshotID)

	m.logger.Info("快照删除成功", zap.String("snapshotId", snapshotID))

	return nil
}

// saveSnapshot 保存快照元数据
func (m *SnapshotManager) saveSnapshot(snapshot *VMSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	snapshotFile := filepath.Join(m.storagePath, "snapshots", snapshot.ID+".json")
	return os.WriteFile(snapshotFile, data, 0644)
}
