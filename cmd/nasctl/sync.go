package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// syncCmd 返回云同步命令组.
func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Cloud Drive Sync 管理",
		Long: `管理 Cloud Drive Sync 同步任务。

支持多云存储提供商（AWS S3、阿里云 OSS、Google Drive、OneDrive 等）的同步任务管理。

示例：
  nasctl sync list
  nasctl sync status <task-id>
  nasctl sync trigger <task-id>
  nasctl sync history <task-id>
  nasctl sync versions <file-path>`,
	}

	cmd.AddCommand(syncListCmd())
	cmd.AddCommand(syncCreateCmd())
	cmd.AddCommand(syncDeleteCmd())
	cmd.AddCommand(syncTriggerCmd())
	cmd.AddCommand(syncStatusCmd())
	cmd.AddCommand(syncHistoryCmd())
	cmd.AddCommand(syncVersionsCmd())
	cmd.AddCommand(syncPauseCmd())
	cmd.AddCommand(syncResumeCmd())
	cmd.AddCommand(syncCancelCmd())

	return cmd
}

// ==================== sync list ====================

func syncListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有同步任务",
		Long:  "列出所有已配置的 Cloud Drive Sync 同步任务及其状态。",
		Run:   runSyncList,
	}

	cmd.Flags().Bool("all", false, "显示所有任务（包括已禁用的）")

	return cmd
}

func runSyncList(cmd *cobra.Command, args []string) {
	resp, err := apiGet("/cloudsync/tasks")
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取任务列表失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data []struct {
			ID           string    `json:"id"`
			Name         string    `json:"name"`
			ProviderID   string    `json:"providerId"`
			Enabled      bool      `json:"enabled"`
			Direction    string    `json:"direction"`
			Mode         string    `json:"mode"`
			Status       string    `json:"status"`
			LastSync     time.Time `json:"lastSync"`
			ScheduleType string    `json:"scheduleType"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "解析响应失败: %v\n", err)
		os.Exit(1)
	}

	if outputFmt == OutputJSON {
		printSyncJSON(result.Data)
		return
	}

	if len(result.Data) == 0 {
		fmt.Println("没有同步任务。使用 nasctl sync create 创建新任务。")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDIRECTION\tMODE\tSCHEDULE\tSTATUS\tLAST SYNC")
	fmt.Fprintln(w, "--\t----\t---------\t----\t--------\t------\t----------")

	for _, task := range result.Data {
		lastSync := "-"
		if !task.LastSync.IsZero() {
			lastSync = task.LastSync.Format("2006-01-02 15:04")
		}

		schedule := task.ScheduleType
		if schedule == "" {
			schedule = "manual"
		}

		status := task.Status
		if !task.Enabled {
			status = "disabled"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			task.ID,
			truncate(task.Name, 20),
			task.Direction,
			task.Mode,
			schedule,
			status,
			lastSync,
		)
	}

	w.Flush()
}

// ==================== sync create ====================

func syncCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "创建同步任务",
		Long: `创建一个新的 Cloud Drive Sync 同步任务。

示例：
  nasctl sync create \
    --name "照片备份" \
    --provider provider_abc123 \
    --local /data/photos \
    --remote /backup/photos \
    --direction upload \
    --mode backup \
    --schedule cron \
    --cron "0 2 * * *"`,
		Run: runSyncCreate,
	}

	// 必填参数
	cmd.Flags().String("name", "", "任务名称（必填）")
	cmd.Flags().String("provider", "", "提供商 ID（必填）")
	cmd.Flags().String("local", "", "本地路径（必填）")
	cmd.Flags().String("remote", "", "远程路径（必填）")

	// 可选参数
	cmd.Flags().String("direction", "bidirect", "同步方向: upload/download/bidirect")
	cmd.Flags().String("mode", "sync", "同步模式: mirror/backup/sync/increment")
	cmd.Flags().String("schedule", "manual", "调度类型: manual/realtime/interval/cron")
	cmd.Flags().String("interval", "", "间隔时间（如 1h, 30m, 24h）")
	cmd.Flags().String("cron", "", "Cron 表达式（如 \"0 2 * * *\"）")
	cmd.Flags().StringSlice("include", nil, "包含文件模式（可重复）")
	cmd.Flags().StringSlice("exclude", nil, "排除文件模式（可重复）")
	cmd.Flags().Int64("max-file-size", 0, "最大文件大小（字节，0 表示不限制）")
	cmd.Flags().String("conflict", "newer", "冲突策略: skip/local/remote/newer/rename/ask")
	cmd.Flags().Bool("delete-remote", false, "本地删除时同步删除远程文件")
	cmd.Flags().Bool("delete-local", false, "远程删除时同步删除本地文件")
	cmd.Flags().Bool("preserve-modtime", true, "保留文件修改时间")
	cmd.Flags().Bool("checksum", false, "使用哈希校验（更准确但更慢）")
	cmd.Flags().Bool("encrypt", false, "加密传输")
	cmd.Flags().Int64("bandwidth", 0, "带宽限制（KB/s，0 表示不限制）")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("local")
	_ = cmd.MarkFlagRequired("remote")

	return cmd
}

func runSyncCreate(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	providerID, _ := cmd.Flags().GetString("provider")
	localPath, _ := cmd.Flags().GetString("local")
	remotePath, _ := cmd.Flags().GetString("remote")

	// 验证必填参数
	if name == "" || providerID == "" || localPath == "" || remotePath == "" {
		fmt.Fprintln(os.Stderr, "错误：name、provider、local、remote 为必填参数")
		os.Exit(1)
	}

	// 获取可选参数
	direction, _ := cmd.Flags().GetString("direction")
	mode, _ := cmd.Flags().GetString("mode")
	scheduleType, _ := cmd.Flags().GetString("schedule")
	cronExpr, _ := cmd.Flags().GetString("cron")
	intervalExpr, _ := cmd.Flags().GetString("interval")
	includePatterns, _ := cmd.Flags().GetStringSlice("include")
	excludePatterns, _ := cmd.Flags().GetStringSlice("exclude")
	maxFileSize, _ := cmd.Flags().GetInt64("max-file-size")
	conflictStrategy, _ := cmd.Flags().GetString("conflict")
	deleteRemote, _ := cmd.Flags().GetBool("delete-remote")
	deleteLocal, _ := cmd.Flags().GetBool("delete-local")
	preserveModTime, _ := cmd.Flags().GetBool("preserve-modtime")
	checksumVerify, _ := cmd.Flags().GetBool("checksum")
	encrypt, _ := cmd.Flags().GetBool("encrypt")
	bandwidth, _ := cmd.Flags().GetInt64("bandwidth")

	// 确定 scheduleExpr
	scheduleExpr := ""
	switch scheduleType {
	case "interval":
		scheduleExpr = intervalExpr
	case "cron":
		scheduleExpr = cronExpr
	}

	// 构建请求体
	body := map[string]interface{}{
		"name":             name,
		"providerId":       providerID,
		"localPath":        localPath,
		"remotePath":       remotePath,
		"direction":        direction,
		"mode":             mode,
		"scheduleType":     scheduleType,
		"scheduleExpr":     scheduleExpr,
		"conflictStrategy": conflictStrategy,
		"deleteRemote":     deleteRemote,
		"deleteLocal":      deleteLocal,
		"preserveModTime":  preserveModTime,
		"checksumVerify":   checksumVerify,
		"encrypt":          encrypt,
	}

	if len(includePatterns) > 0 {
		body["includePatterns"] = includePatterns
	}
	if len(excludePatterns) > 0 {
		body["excludePatterns"] = excludePatterns
	}
	if maxFileSize > 0 {
		body["maxFileSize"] = maxFileSize
	}
	if bandwidth > 0 {
		body["bandwidthLimit"] = bandwidth
	}

	resp, err := apiPost("/cloudsync/tasks", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建同步任务失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "解析响应失败: %v\n", err)
		os.Exit(1)
	}

	if code, ok := result["code"].(float64); ok && code != 0 {
		msg, _ := result["message"].(string)
		fmt.Fprintf(os.Stderr, "创建失败: %s\n", msg)
		os.Exit(1)
	}

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		taskID, _ := data["id"].(string)
		fmt.Printf("同步任务创建成功！\n")
		fmt.Printf("  ID:   %s\n", taskID)
		fmt.Printf("  名称: %s\n", name)
		fmt.Printf("  方向: %s\n", direction)
		fmt.Printf("  路径: %s → %s\n", localPath, remotePath)
		fmt.Printf("\n使用以下命令触发同步：\n")
		fmt.Printf("  nasctl sync trigger %s\n", taskID)
	}
}

// ==================== sync delete ====================

func syncDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <task-id>",
		Short: "删除同步任务",
		Long:  "删除指定的同步任务。删除任务不会删除已同步的文件。",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncDelete,
	}

	cmd.Flags().Bool("force", false, "跳过确认提示")

	return cmd
}

func runSyncDelete(cmd *cobra.Command, args []string) {
	taskID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("确认删除同步任务 %s？(y/N) ", taskID)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("已取消")
			return
		}
	}

	resp, err := apiDelete(fmt.Sprintf("/cloudsync/tasks/%s", taskID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "删除任务失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	fmt.Printf("同步任务 %s 已删除。\n", taskID)
}

// ==================== sync trigger ====================

func syncTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <task-id>",
		Short: "手动触发同步",
		Long:  "手动触发指定同步任务的执行。",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncTrigger,
	}

	cmd.Flags().Bool("wait", false, "等待同步完成后返回")
	cmd.Flags().Int("timeout", 0, "等待超时时间（秒，0 表示无限等待）")

	return cmd
}

func runSyncTrigger(cmd *cobra.Command, args []string) {
	taskID := args[0]

	resp, err := apiPost(fmt.Sprintf("/cloudsync/tasks/%s/run", taskID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "触发同步失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if code, ok := result["code"].(float64); ok && code != 0 {
		msg, _ := result["message"].(string)
		fmt.Fprintf(os.Stderr, "触发失败: %s\n", msg)
		os.Exit(1)
	}

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	fmt.Printf("同步任务 %s 已触发。\n", taskID)
	fmt.Printf("使用 nasctl sync status %s 查看进度。\n", taskID)

	// 如果指定 --wait，轮询等待完成
	wait, _ := cmd.Flags().GetBool("wait")
	if wait {
		timeout, _ := cmd.Flags().GetInt("timeout")
		pollSyncStatus(taskID, timeout)
	}
}

// ==================== sync status ====================

func syncStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [<task-id>]",
		Short: "查看同步状态",
		Long:  "查看指定任务的同步状态。不指定 task-id 时显示所有任务状态概览。",
		Args:  cobra.MaximumNArgs(1),
		Run:   runSyncStatus,
	}

	cmd.Flags().Bool("watch", false, "持续监视状态变化")
	cmd.Flags().Int("interval", 2, "监视刷新间隔（秒）")

	return cmd
}

func runSyncStatus(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		runSyncStatusAll()
		return
	}

	taskID := args[0]
	showTaskStatus(taskID)
}

func runSyncStatusAll() {
	resp, err := apiGet("/cloudsync/statuses")
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取状态失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]struct {
			TaskID      string  `json:"taskId"`
			Status      string  `json:"status"`
			Progress    float64 `json:"progress"`
			Speed       int64   `json:"speed"`
			CurrentFile string  `json:"currentFile"`
		} `json:"data"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	if outputFmt == OutputJSON {
		printSyncJSON(result.Data)
		return
	}

	if len(result.Data) == 0 {
		fmt.Println("没有运行中的同步任务。")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TASK ID\tSTATUS\tPROGRESS\tSPEED\tCURRENT FILE")
	fmt.Fprintln(w, "-------\t------\t--------\t-----\t-------------")

	for _, status := range result.Data {
		progress := fmt.Sprintf("%.1f%%", status.Progress)
		speed := formatSpeed(status.Speed)
		currentFile := truncate(status.CurrentFile, 30)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			status.TaskID,
			status.Status,
			progress,
			speed,
			currentFile,
		)
	}

	w.Flush()
}

func showTaskStatus(taskID string) {
	resp, err := apiGet(fmt.Sprintf("/cloudsync/tasks/%s/status", taskID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取状态失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TaskID           string  `json:"taskId"`
			Status           string  `json:"status"`
			StartTime        string  `json:"startTime"`
			EndTime          string  `json:"endTime"`
			TotalFiles       int64   `json:"totalFiles"`
			ProcessedFiles   int64   `json:"processedFiles"`
			TotalBytes       int64   `json:"totalBytes"`
			TransferredBytes int64   `json:"transferredBytes"`
			Speed            int64   `json:"speed"`
			Progress         float64 `json:"progress"`
			CurrentFile      string  `json:"currentFile"`
			CurrentAction    string  `json:"currentAction"`
			UploadedFiles    int64   `json:"uploadedFiles"`
			DownloadedFiles  int64   `json:"downloadedFiles"`
			SkippedFiles     int64   `json:"skippedFiles"`
			FailedFiles      int64   `json:"failedFiles"`
			DeletedFiles     int64   `json:"deletedFiles"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "解析响应失败: %v\n", err)
		os.Exit(1)
	}

	if outputFmt == OutputJSON {
		printSyncJSON(result.Data)
		return
	}

	s := result.Data

	fmt.Printf("任务 ID:  %s\n", s.TaskID)
	fmt.Printf("状态:     %s\n", formatStatus(s.Status))
	fmt.Printf("进度:     %s\n", formatProgress(s.Progress, s.ProcessedFiles, s.TotalFiles))
	fmt.Printf("传输:     %s / %s\n", humanBytesLocal(s.TransferredBytes), humanBytesLocal(s.TotalBytes))
	fmt.Printf("速度:     %s\n", formatSpeed(s.Speed))

	if s.CurrentFile != "" {
		fmt.Printf("当前文件: %s (%s)\n", s.CurrentFile, s.CurrentAction)
	}

	fmt.Println()
	fmt.Printf("上传: %d  下载: %d  跳过: %d  失败: %d  删除: %d\n",
		s.UploadedFiles, s.DownloadedFiles, s.SkippedFiles, s.FailedFiles, s.DeletedFiles)
}

// ==================== sync history ====================

func syncHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <task-id>",
		Short: "查看同步历史",
		Long:  "查看指定同步任务的执行历史记录。",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncHistory,
	}

	cmd.Flags().Int("limit", 20, "显示最近 N 条记录")

	return cmd
}

func runSyncHistory(cmd *cobra.Command, args []string) {
	taskID := args[0]

	resp, err := apiGet(fmt.Sprintf("/cloudsync/tasks/%s/status", taskID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取同步历史失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TaskID          string `json:"taskId"`
			Status          string `json:"status"`
			StartTime       string `json:"startTime"`
			EndTime         string `json:"endTime"`
			TotalFiles      int64  `json:"totalFiles"`
			UploadedFiles   int64  `json:"uploadedFiles"`
			DownloadedFiles int64  `json:"downloadedFiles"`
			FailedFiles     int64  `json:"failedFiles"`
			Conflicts       []struct {
				Path       string `json:"path"`
				Resolution string `json:"resolution"`
			} `json:"conflicts"`
			Errors []struct {
				Time   string `json:"time"`
				Path   string `json:"path"`
				Action string `json:"action"`
				Error  string `json:"error"`
			} `json:"errors"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "解析响应失败: %v\n", err)
		os.Exit(1)
	}

	if outputFmt == OutputJSON {
		printSyncJSON(result.Data)
		return
	}

	s := result.Data

	fmt.Printf("同步历史 - 任务 %s\n", taskID)
	fmt.Printf("══════════════════════════════════════\n\n")

	fmt.Printf("状态:       %s\n", formatStatus(s.Status))

	if s.StartTime != "" {
		fmt.Printf("开始时间:   %s\n", formatTime(s.StartTime))
	}
	if s.EndTime != "" {
		fmt.Printf("结束时间:   %s\n", formatTime(s.EndTime))
	}

	fmt.Printf("总文件数:   %d\n", s.TotalFiles)
	fmt.Printf("上传:       %d\n", s.UploadedFiles)
	fmt.Printf("下载:       %d\n", s.DownloadedFiles)
	fmt.Printf("失败:       %d\n", s.FailedFiles)

	// 显示冲突
	if len(s.Conflicts) > 0 {
		fmt.Printf("\n冲突文件 (%d):\n", len(s.Conflicts))
		for _, c := range s.Conflicts {
			fmt.Printf("  • %s → %s\n", c.Path, c.Resolution)
		}
	}

	// 显示错误
	if len(s.Errors) > 0 {
		fmt.Printf("\n错误记录 (%d):\n", len(s.Errors))
		for _, e := range s.Errors {
			fmt.Printf("  • [%s] %s (%s): %s\n", e.Time, e.Path, e.Action, e.Error)
		}
	}
}

// ==================== sync versions ====================

func syncVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions <file-path>",
		Short: "查看文件版本历史",
		Long:  "查看指定文件的版本历史记录。",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncVersions,
	}

	cmd.Flags().Int("limit", 10, "显示最近 N 个版本")

	return cmd
}

func runSyncVersions(cmd *cobra.Command, args []string) {
	filePath := args[0]

	// 查找包含此文件路径的所有任务状态
	resp, err := apiGet("/cloudsync/statuses")
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取状态失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var statusesResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&statusesResult)

	if outputFmt == OutputJSON {
		printSyncJSON(map[string]interface{}{
			"filePath": filePath,
			"message":  "版本历史功能需要配合版本管理后端使用",
		})
		return
	}

	fmt.Printf("文件版本历史: %s\n", filePath)
	fmt.Printf("══════════════════════════════════════\n\n")

	// 查询所有任务中的文件记录
	tasksResp, err := apiGet("/cloudsync/tasks")
	if err == nil {
		var tasksResult struct {
			Data []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				LocalPath  string `json:"localPath"`
				RemotePath string `json:"remotePath"`
			} `json:"data"`
		}
		json.NewDecoder(tasksResp.Body).Decode(&tasksResult)
		tasksResp.Body.Close()

		found := false
		for _, task := range tasksResult.Data {
			if strings.HasPrefix(filePath, task.LocalPath) || strings.HasPrefix(filePath, task.RemotePath) {
				fmt.Printf("关联任务: %s (%s)\n", task.Name, task.ID)
				fmt.Printf("  本地路径: %s\n", task.LocalPath)
				fmt.Printf("  远程路径: %s\n", task.RemotePath)
				found = true
			}
		}

		if !found {
			fmt.Println("未找到关联的同步任务。")
		}
	}

	fmt.Println()
	fmt.Println("提示：详细的版本历史需要开启 backup 模式并启用版本管理。")
}

// ==================== sync pause / resume / cancel ====================

func syncPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id>",
		Short: "暂停同步任务",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncPause,
	}
}

func runSyncPause(cmd *cobra.Command, args []string) {
	taskID := args[0]
	resp, err := apiPost(fmt.Sprintf("/cloudsync/tasks/%s/pause", taskID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "暂停失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	fmt.Printf("同步任务 %s 已暂停。\n", taskID)
}

func syncResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <task-id>",
		Short: "恢复同步任务",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncResume,
	}
}

func runSyncResume(cmd *cobra.Command, args []string) {
	taskID := args[0]
	resp, err := apiPost(fmt.Sprintf("/cloudsync/tasks/%s/resume", taskID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "恢复失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	fmt.Printf("同步任务 %s 已恢复。\n", taskID)
}

func syncCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <task-id>",
		Short: "取消同步任务",
		Args:  cobra.ExactArgs(1),
		Run:   runSyncCancel,
	}
}

func runSyncCancel(cmd *cobra.Command, args []string) {
	taskID := args[0]
	resp, err := apiPost(fmt.Sprintf("/cloudsync/tasks/%s/cancel", taskID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "取消失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if outputFmt == OutputJSON {
		printSyncJSON(result)
		return
	}

	fmt.Printf("同步任务 %s 已取消。\n", taskID)
}

// ==================== 辅助函数 ====================

// pollSyncStatus 轮询同步状态直到完成.
func pollSyncStatus(taskID string, timeout int) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(time.Duration(timeout) * time.Second)
	}

	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "等待超时")
			return
		}

		resp, err := apiGet(fmt.Sprintf("/cloudsync/tasks/%s/status", taskID))
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			Data struct {
				Status      float64 `json:"progress"`
				CurrentFile string  `json:"currentFile"`
			} `json:"data"`
		}

		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		fmt.Printf("\r进度: %.1f%% | 当前: %s", result.Data.Status, truncate(result.Data.CurrentFile, 40))

		status := ""
		// 简单判断是否完成
		if result.Data.Status >= 100.0 {
			fmt.Println("\n同步完成！")
			return
		}

		_ = status
		time.Sleep(2 * time.Second)
	}
}

// API 辅助函数.
func apiGet(path string) (*http.Response, error) {
	url := apiBaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req)
	return http.DefaultClient.Do(req)
}

func apiPost(path string, body interface{}) (*http.Response, error) {
	url := apiBaseURL + path
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	return http.DefaultClient.Do(req)
}

func apiDelete(path string) (*http.Response, error) {
	url := apiBaseURL + path
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req)
	return http.DefaultClient.Do(req)
}

func addAuthHeader(req *http.Request) {
	// 从配置或环境变量获取 token
	token := os.Getenv("NASCTL_TOKEN")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// 格式化辅助函数.
func formatStatus(status string) string {
	icons := map[string]string{
		"idle":      "⏸  空闲",
		"running":   "🔄 运行中",
		"paused":    "⏸  已暂停",
		"completed": "✅ 已完成",
		"failed":    "❌ 失败",
		"cancelled": "🚫 已取消",
	}
	if icon, ok := icons[status]; ok {
		return icon
	}
	return status
}

func formatProgress(pct float64, processed, total int64) string {
	if total == 0 {
		return "等待中..."
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", pct, processed, total)
}

func formatSpeed(kbps int64) string {
	if kbps == 0 {
		return "-"
	}
	if kbps >= 1024 {
		return fmt.Sprintf("%.1f MB/s", float64(kbps)/1024)
	}
	return fmt.Sprintf("%d KB/s", kbps)
}

func humanBytesLocal(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func formatTime(t string) string {
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}
	return parsed.Format("2006-01-02 15:04:05")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return "..." + s[len(s)-maxLen+3:]
	}
	return s[:maxLen]
}

func printSyncJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}
