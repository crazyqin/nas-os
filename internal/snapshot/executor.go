package snapshot

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// SnapshotExecutor 快照执行器
type SnapshotExecutor struct {
	storageMgr StorageManager
}

// NewSnapshotExecutor 创建执行器
func NewSnapshotExecutor(storageMgr StorageManager) *SnapshotExecutor {
	return &SnapshotExecutor{
		storageMgr: storageMgr,
	}
}

// Execute 执行快照创建
func (e *SnapshotExecutor) Execute(policy *Policy) (string, error) {
	// 生成快照名称
	snapshotName := e.generateSnapshotName(policy)

	// 执行前置脚本
	if policy.Scripts != nil && policy.Scripts.PreSnapshotScript != "" {
		if err := e.runScript(policy.Scripts.PreSnapshotScript, policy.Scripts.TimeoutSeconds); err != nil {
			if !policy.Scripts.ContinueOnFailure {
				return "", fmt.Errorf("前置脚本执行失败: %w", err)
			}
			// 记录错误但继续
			fmt.Printf("警告: 前置脚本执行失败: %v\n", err)
		}
	}

	// 创建快照
	_, err := e.storageMgr.CreateSnapshot(
		policy.VolumeName,
		policy.SubvolumeName,
		policy.SnapshotDir+"/"+snapshotName,
		policy.ReadOnly,
	)

	// 执行后置脚本
	if policy.Scripts != nil && policy.Scripts.PostSnapshotScript != "" {
		postErr := e.runScript(policy.Scripts.PostSnapshotScript, policy.Scripts.TimeoutSeconds)
		if postErr != nil {
			fmt.Printf("警告: 后置脚本执行失败: %v\n", postErr)
		}
	}

	if err != nil {
		return "", fmt.Errorf("创建快照失败: %w", err)
	}

	return snapshotName, nil
}

// generateSnapshotName 生成快照名称
func (e *SnapshotExecutor) generateSnapshotName(policy *Policy) string {
	timestamp := time.Now().Format("20060102-150405")

	name := ""
	if policy.SnapshotPrefix != "" {
		name = policy.SnapshotPrefix + "-"
	}
	name += timestamp

	// 如果是定时快照，添加策略标识
	if policy.Type == PolicyTypeScheduled {
		name += "-" + policy.ID[:8]
	}

	return name
}

// runScript 执行脚本
func (e *SnapshotExecutor) runScript(script string, timeoutSeconds int) error {
	timeout := 300 // 默认 5 分钟
	if timeoutSeconds > 0 {
		timeout = timeoutSeconds
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = append(cmd.Env,
		"SNAPSHOT_TIMESTAMP="+time.Now().Format(time.RFC3339),
		"SNAPSHOT_SCRIPT_TYPE=snapshot",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("脚本执行失败: %w, output: %s", err, string(output))
	}

	return nil
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	SnapshotName string    `json:"snapshotName"`
	VolumeName   string    `json:"volumeName"`
	SubvolName   string    `json:"subvolName"`
	Path         string    `json:"path"`
	CreatedAt    time.Time `json:"createdAt"`
	Size         int64     `json:"size"`
	ReadOnly     bool      `json:"readOnly"`
	PreScriptOK  bool      `json:"preScriptOk"`
	PostScriptOK bool      `json:"postScriptOk"`
	Error        string    `json:"error,omitempty"`
}
