package zfsenhanced

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CreateSnapshot 创建快照
func (pm *PoolManager) CreateSnapshot(ctx context.Context, dataset, snapshotName string, recursive bool) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	args := []string{"snapshot"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// DeleteSnapshot 删除快照
func (pm *PoolManager) DeleteSnapshot(ctx context.Context, dataset, snapshotName string, recursive bool) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	args := []string{"destroy"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// ListSnapshots 列出快照
func (pm *PoolManager) ListSnapshots(ctx context.Context, poolName string) ([]SnapshotInfo, error) {
	cmd := exec.CommandContext(ctx, "zfs", "list", "-t", "snapshot", "-o", "name,used,refer,creation", "-H", "-p", "-r", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var snapshots []SnapshotInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// 解析 name@snapshot 格式
		parts := strings.SplitN(fields[0], "@", 2)
		if len(parts) != 2 {
			continue
		}

		snap := SnapshotInfo{
			Name:         fields[0],
			PoolName:     poolName,
			Dataset:      parts[0],
			SnapshotName: parts[1],
			UsedBytes:    parseSize(fields[1]),
			ReferBytes:   parseSize(fields[2]),
		}

		// 解析创建时间
		if ts, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
			snap.CreatedAt = time.Unix(ts, 0)
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// RollbackSnapshot 回滚到快照
func (pm *PoolManager) RollbackSnapshot(ctx context.Context, dataset, snapshotName string, force bool) error {
	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	args := []string{"rollback"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rollback snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// CloneSnapshot 克隆快照
func (pm *PoolManager) CloneSnapshot(ctx context.Context, dataset, snapshotName, targetDataset string) error {
	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	cmd := exec.CommandContext(ctx, "zfs", "clone", fullName, targetDataset)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// CreateSnapshotPolicy 创建快照策略
func (pm *PoolManager) CreateSnapshotPolicy(ctx context.Context, policy SnapshotPolicy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.snapshotPolicies[policy.ID]; exists {
		return fmt.Errorf("snapshot policy %s already exists", policy.ID)
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	pm.snapshotPolicies[policy.ID] = &policy
	return nil
}

// UpdateSnapshotPolicy 更新快照策略
func (pm *PoolManager) UpdateSnapshotPolicy(ctx context.Context, policy SnapshotPolicy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.snapshotPolicies[policy.ID]; !exists {
		return fmt.Errorf("snapshot policy %s not found", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	pm.snapshotPolicies[policy.ID] = &policy
	return nil
}

// DeleteSnapshotPolicy 删除快照策略
func (pm *PoolManager) DeleteSnapshotPolicy(ctx context.Context, policyID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.snapshotPolicies[policyID]; !exists {
		return fmt.Errorf("snapshot policy %s not found", policyID)
	}

	delete(pm.snapshotPolicies, policyID)
	return nil
}

// GetSnapshotPolicy 获取快照策略
func (pm *PoolManager) GetSnapshotPolicy(policyID string) (*SnapshotPolicy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policy, exists := pm.snapshotPolicies[policyID]
	if !exists {
		return nil, fmt.Errorf("snapshot policy %s not found", policyID)
	}

	return policy, nil
}

// ListSnapshotPolicies 列出快照策略
func (pm *PoolManager) ListSnapshotPolicies() []SnapshotPolicy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policies := make([]SnapshotPolicy, 0, len(pm.snapshotPolicies))
	for _, p := range pm.snapshotPolicies {
		policies = append(policies, *p)
	}

	return policies
}

// ExecuteSnapshotPolicy 执行快照策略
func (pm *PoolManager) ExecuteSnapshotPolicy(ctx context.Context, policyID string) error {
	pm.mu.RLock()
	policy, exists := pm.snapshotPolicies[policyID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("snapshot policy %s not found", policyID)
	}

	if !policy.Enabled {
		return fmt.Errorf("snapshot policy %s is disabled", policyID)
	}

	// 生成快照名称
	snapshotName := fmt.Sprintf("%s%s", policy.Prefix, time.Now().Format("20060102-150405"))

	// 创建快照
	if err := pm.CreateSnapshot(ctx, policy.Dataset, snapshotName, policy.Recursive); err != nil {
		return err
	}

	// 更新最后快照时间
	pm.mu.Lock()
	policy.LastSnapshot = time.Now()
	pm.mu.Unlock()

	// 清理过期快照
	if policy.AutoDestroy {
		if err := pm.cleanupExpiredSnapshots(ctx, policy); err != nil {
			return fmt.Errorf("snapshot created but cleanup failed: %w", err)
		}
	}

	return nil
}

// cleanupExpiredSnapshots 清理过期快照
func (pm *PoolManager) cleanupExpiredSnapshots(ctx context.Context, policy *SnapshotPolicy) error {
	snapshots, err := pm.ListSnapshots(ctx, policy.PoolName)
	if err != nil {
		return err
	}

	// 过滤策略相关的快照
	var policySnapshots []SnapshotInfo
	for _, snap := range snapshots {
		if snap.Dataset == policy.Dataset && strings.HasPrefix(snap.SnapshotName, policy.Prefix) {
			policySnapshots = append(policySnapshots, snap)
		}
	}

	// 按创建时间排序
	sort.Slice(policySnapshots, func(i, j int) bool {
		return policySnapshots[i].CreatedAt.After(policySnapshots[j].CreatedAt)
	})

	// 删除超过保留数量的快照
	if policy.MaxSnapshots > 0 && len(policySnapshots) > policy.MaxSnapshots {
		for _, snap := range policySnapshots[policy.MaxSnapshots:] {
			if err := pm.DeleteSnapshot(ctx, policy.Dataset, snap.SnapshotName, false); err != nil {
				// 记录错误但继续
				continue
			}
		}
	}

	// 删除超过保留天数的快照
	if policy.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -policy.RetentionDays)
		for _, snap := range policySnapshots {
			if snap.CreatedAt.Before(cutoff) {
				if err := pm.DeleteSnapshot(ctx, policy.Dataset, snap.SnapshotName, false); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// GetSnapshotDiff 获取两个快照之间的差异
func (pm *PoolManager) GetSnapshotDiff(ctx context.Context, snapshot1, snapshot2 string) (string, error) {
	cmd := exec.CommandContext(ctx, "zfs", "diff", snapshot1, snapshot2)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot diff: %w", err)
	}

	return string(output), nil
}

// GetSnapshotSpaceUsage 获取快照空间使用情况
func (pm *PoolManager) GetSnapshotSpaceUsage(ctx context.Context, poolName string) (map[string]int64, error) {
	cmd := exec.CommandContext(ctx, "zfs", "list", "-t", "snapshot", "-o", "name,used", "-H", "-p", "-r", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot space usage: %w", err)
	}

	usage := make(map[string]int64)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			usage[fields[0]] = parseSize(fields[1])
		}
	}

	return usage, nil
}

// HoldSnapshot 保留快照（防止删除）
func (pm *PoolManager) HoldSnapshot(ctx context.Context, dataset, snapshotName, tag string) error {
	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	cmd := exec.CommandContext(ctx, "zfs", "hold", tag, fullName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to hold snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// ReleaseSnapshot 释放快照保留
func (pm *PoolManager) ReleaseSnapshot(ctx context.Context, dataset, snapshotName, tag string) error {
	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	cmd := exec.CommandContext(ctx, "zfs", "release", tag, fullName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to release snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// RenameSnapshot 重命名快照
func (pm *PoolManager) RenameSnapshot(ctx context.Context, dataset, oldName, newName string) error {
	oldFullName := fmt.Sprintf("%s@%s", dataset, oldName)
	newFullName := fmt.Sprintf("%s@%s", dataset, newName)

	cmd := exec.CommandContext(ctx, "zfs", "rename", oldFullName, newFullName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rename snapshot: %w, output: %s", err, string(output))
	}

	return nil
}

// SendSnapshot 发送快照到stdout（用于备份/复制）
func (pm *PoolManager) SendSnapshot(ctx context.Context, dataset, snapshotName string, incremental string) ([]byte, error) {
	fullName := fmt.Sprintf("%s@%s", dataset, snapshotName)

	args := []string{"send"}
	if incremental != "" {
		args = append(args, "-i", incremental)
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to send snapshot: %w", err)
	}

	return output, nil
}

// ReceiveSnapshot 接收快照
func (pm *PoolManager) ReceiveSnapshot(ctx context.Context, targetDataset string, data []byte) error {
	cmd := exec.CommandContext(ctx, "zfs", "receive", targetDataset)
	cmd.Stdin = strings.NewReader(string(data))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to receive snapshot: %w, output: %s", err, string(output))
	}

	return nil
}
