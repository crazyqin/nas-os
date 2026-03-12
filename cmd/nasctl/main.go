// Package main nasctl - NAS-OS 命令行管理工具
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// 全局配置
var (
	configFile string
	outputFmt  string
	verbose    bool
	quiet      bool
)

// 输出格式常量
const (
	OutputText = "text"
	OutputJSON = "json"
)

// API 配置
var apiBaseURL = "http://localhost:8080/api/v1"

func main() {
	rootCmd := &cobra.Command{
		Use:   "nasctl",
		Short: "NAS-OS 命令行管理工具",
		Long: `nasctl 是 NAS-OS 的命令行管理工具，提供卷管理、共享管理、快照管理等功能。

完整文档：https://github.com/nas-os/nas-os/docs/NASCTL-CLI.md`,
		Version: "1.0.0",
	}

	// 全局标志
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "/etc/nas-os/config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "text", "输出格式 (text/json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "静默模式")

	// 添加子命令
	rootCmd.AddCommand(volumeCmd())
	rootCmd.AddCommand(subvolumeCmd())
	rootCmd.AddCommand(snapshotCmd())
	rootCmd.AddCommand(shareCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(restartCmd())
	rootCmd.AddCommand(logsCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ========== 卷管理命令 ==========

func volumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "卷管理",
		Long:  "管理 Btrfs 存储卷（创建、删除、查看）",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出所有卷",
		Run:   runVolumeList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "显示卷详情",
		Args:  cobra.ExactArgs(1),
		Run:   runVolumeShow,
	})

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "创建卷",
		Args:  cobra.ExactArgs(1),
		Run:   runVolumeCreate,
	}
	createCmd.Flags().StringSlice("devices", nil, "设备列表（逗号分隔）")
	createCmd.Flags().String("raid", "single", "RAID 级别 (single/raid0/raid1/raid5/raid6/raid10)")
	cmd.AddCommand(createCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "删除卷",
		Args:  cobra.ExactArgs(1),
		Run:   runVolumeDelete,
	}
	deleteCmd.Flags().Bool("force", false, "强制删除")
	cmd.AddCommand(deleteCmd)

	return cmd
}

func runVolumeList(cmd *cobra.Command, args []string) {
	volumes, err := apiListVolumes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printVolumes(volumes)
}

func runVolumeShow(cmd *cobra.Command, args []string) {
	name := args[0]
	volume, err := apiGetVolume(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printVolume(*volume)
}

func runVolumeCreate(cmd *cobra.Command, args []string) {
	name := args[0]
	devices, _ := cmd.Flags().GetStringSlice("devices")
	raid, _ := cmd.Flags().GetString("raid")

	if len(devices) == 0 {
		fmt.Fprintln(os.Stderr, "错误：--devices 参数必填")
		os.Exit(1)
	}

	volume, err := apiCreateVolume(name, devices, raid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 卷 %s 创建成功\n", volume.Name)
	printVolume(*volume)
}

func runVolumeDelete(cmd *cobra.Command, args []string) {
	name := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("⚠️  警告：删除卷 %s 将导致数据丢失！\n", name)
		fmt.Print("确认删除？(y/N): ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("已取消")
			return
		}
	}

	if err := apiDeleteVolume(name, force); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 卷 %s 已删除\n", name)
}

// ========== 子卷管理命令 ==========

func subvolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subvolume",
		Short: "子卷管理",
		Long:  "管理 Btrfs 子卷（创建、删除、查看）",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <volume>",
		Short: "列出子卷",
		Args:  cobra.ExactArgs(1),
		Run:   runSubvolumeList,
	})

	createCmd := &cobra.Command{
		Use:   "create <path>",
		Short: "创建子卷",
		Args:  cobra.ExactArgs(1),
		Run:   runSubvolumeCreate,
	}
	cmd.AddCommand(createCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <path>",
		Short: "删除子卷",
		Args:  cobra.ExactArgs(1),
		Run:   runSubvolumeDelete,
	}
	deleteCmd.Flags().Bool("force", false, "强制删除")
	cmd.AddCommand(deleteCmd)

	return cmd
}

func runSubvolumeList(cmd *cobra.Command, args []string) {
	volumeName := args[0]
	subvolumes, err := apiListSubvolumes(volumeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printSubvolumes(subvolumes)
}

func runSubvolumeCreate(cmd *cobra.Command, args []string) {
	path := args[0]
	subvolume, err := apiCreateSubvolume(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 子卷 %s 创建成功\n", subvolume.Name)
	printSubvolume(*subvolume)
}

func runSubvolumeDelete(cmd *cobra.Command, args []string) {
	path := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if err := apiDeleteSubvolume(path, force); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 子卷 %s 已删除\n", path)
}

// ========== 快照管理命令 ==========

func snapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "快照管理",
		Long:  "管理 Btrfs 快照（创建、恢复、删除、查看）",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <volume>",
		Short: "列出快照",
		Args:  cobra.ExactArgs(1),
		Run:   runSnapshotList,
	})

	createCmd := &cobra.Command{
		Use:   "create <subvolume>",
		Short: "创建快照",
		Args:  cobra.ExactArgs(1),
		Run:   runSnapshotCreate,
	}
	createCmd.Flags().String("name", "", "快照名称")
	createCmd.Flags().Bool("readonly", true, "只读快照")
	cmd.AddCommand(createCmd)

	restoreCmd := &cobra.Command{
		Use:   "restore <snapshot>",
		Short: "恢复快照",
		Args:  cobra.ExactArgs(1),
		Run:   runSnapshotRestore,
	}
	restoreCmd.Flags().String("target", "", "恢复目标路径")
	cmd.AddCommand(restoreCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <snapshot>",
		Short: "删除快照",
		Args:  cobra.ExactArgs(1),
		Run:   runSnapshotDelete,
	}
	cmd.AddCommand(deleteCmd)

	return cmd
}

func runSnapshotList(cmd *cobra.Command, args []string) {
	volumeName := args[0]
	snapshots, err := apiListSnapshots(volumeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printSnapshots(snapshots)
}

func runSnapshotCreate(cmd *cobra.Command, args []string) {
	subvolume := args[0]
	name, _ := cmd.Flags().GetString("name")
	readonly, _ := cmd.Flags().GetBool("readonly")

	snapshot, err := apiCreateSnapshot(subvolume, name, readonly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 快照 %s 创建成功\n", snapshot.Name)
	printSnapshot(*snapshot)
}

func runSnapshotRestore(cmd *cobra.Command, args []string) {
	snapshot := args[0]
	target, _ := cmd.Flags().GetString("target")

	if target == "" {
		fmt.Fprintln(os.Stderr, "错误：--target 参数必填")
		os.Exit(1)
	}

	if err := apiRestoreSnapshot(snapshot, target); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 快照 %s 已恢复到 %s\n", snapshot, target)
}

func runSnapshotDelete(cmd *cobra.Command, args []string) {
	snapshot := args[0]

	if err := apiDeleteSnapshot(snapshot); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 快照 %s 已删除\n", snapshot)
}

// ========== 共享管理命令 ==========

func shareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share",
		Short: "共享管理",
		Long:  "管理 SMB/NFS 共享（创建、删除、查看）",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出所有共享",
		Run:   runShareList,
	})

	createCmd := &cobra.Command{
		Use:   "create <type> <name>",
		Short: "创建共享",
		Args:  cobra.ExactArgs(2),
		Run:   runShareCreate,
	}
	createCmd.Flags().String("path", "", "共享路径")
	createCmd.Flags().Bool("guest-ok", false, "允许访客访问 (SMB)")
	createCmd.Flags().StringSlice("users", nil, "授权用户列表 (SMB)")
	createCmd.Flags().String("network", "", "允许的网络 (NFS, e.g. 192.168.1.0/24)")
	cmd.AddCommand(createCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "删除共享",
		Args:  cobra.ExactArgs(1),
		Run:   runShareDelete,
	}
	cmd.AddCommand(deleteCmd)

	return cmd
}

func runShareList(cmd *cobra.Command, args []string) {
	shares, err := apiListShares()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printShares(shares)
}

func runShareCreate(cmd *cobra.Command, args []string) {
	shareType := args[0]
	name := args[1]

	if shareType != "smb" && shareType != "nfs" {
		fmt.Fprintln(os.Stderr, "错误：共享类型必须是 smb 或 nfs")
		os.Exit(1)
	}

	path, _ := cmd.Flags().GetString("path")
	if path == "" {
		fmt.Fprintln(os.Stderr, "错误：--path 参数必填")
		os.Exit(1)
	}

	var err error
	if shareType == "smb" {
		guestOk, _ := cmd.Flags().GetBool("guest-ok")
		users, _ := cmd.Flags().GetStringSlice("users")
		_, err = apiCreateSMBShare(name, path, guestOk, users)
	} else {
		network, _ := cmd.Flags().GetString("network")
		_, err = apiCreateNFSExport(name, path, network)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ %s 共享 %s 创建成功\n", shareType, name)
}

func runShareDelete(cmd *cobra.Command, args []string) {
	name := args[0]

	if err := apiDeleteShare(name); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 共享 %s 已删除\n", name)
}

// ========== 系统命令 ==========

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "查看系统状态",
		Long:  "显示系统运行状态、存储健康、服务状态等信息",
		Run:   runStatus,
	}

	cmd.Flags().Bool("verbose", false, "详细信息")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")

	status, err := apiGetStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	printStatus(*status, verbose)
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "重启 NAS-OS 服务",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🔄 正在重启 NAS-OS 服务...")
			if err := apiRestart(); err != nil {
				fmt.Fprintf(os.Stderr, "错误：%v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ 服务已重启")
		},
	}
}

func logsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "查看日志",
		Long:  "查看 NAS-OS 系统日志",
		Run:   runLogs,
	}

	cmd.Flags().BoolP("follow", "f", false, "实时跟踪日志")
	cmd.Flags().Int("tail", 100, "显示最后 N 行")
	cmd.Flags().String("level", "info", "日志级别 (debug/info/warn/error)")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) {
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetInt("tail")
	level, _ := cmd.Flags().GetString("level")

	if follow {
		fmt.Println("📋 实时日志模式 (Ctrl+C 退出)...")
		if err := apiStreamLogs(level); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			os.Exit(1)
		}
	} else {
		logs, err := apiGetLogs(tail, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			os.Exit(1)
		}
		for _, log := range logs {
			fmt.Println(log)
		}
	}
}

// ========== 输出辅助函数 ==========

func printVolumes(volumes []Volume) {
	if outputFmt == OutputJSON {
		printJSON(volumes)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tUSED\tFREE\tPROFILE\tSTATUS")
	for _, v := range volumes {
		status := "✓"
		if !v.Status.Healthy {
			status = "✗"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			v.Name,
			formatSize(v.Size),
			formatSize(v.Used),
			formatSize(v.Free),
			v.DataProfile,
			status,
		)
	}
	w.Flush()
}

func printVolume(v Volume) {
	if outputFmt == OutputJSON {
		printJSON(v)
		return
	}

	fmt.Printf("名称：     %s\n", v.Name)
	fmt.Printf("UUID:     %s\n", v.UUID)
	fmt.Printf("设备：    %v\n", v.Devices)
	fmt.Printf("大小：    %s\n", formatSize(v.Size))
	fmt.Printf("已用：    %s (%.1f%%)\n", formatSize(v.Used), float64(v.Used)*100/float64(v.Size))
	fmt.Printf("可用：    %s\n", formatSize(v.Free))
	fmt.Printf("数据配置： %s\n", v.DataProfile)
	fmt.Printf("元数据：  %s\n", v.MetaProfile)
	fmt.Printf("挂载点：  %s\n", v.MountPoint)
	fmt.Printf("子卷数：  %d\n", len(v.Subvolumes))
	fmt.Printf("健康状态： %v\n", v.Status.Healthy)
}

func printSubvolumes(subvolumes []SubVolume) {
	if outputFmt == OutputJSON {
		printJSON(subvolumes)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPATH\tREADONLY")
	for _, sv := range subvolumes {
		ro := "false"
		if sv.ReadOnly {
			ro = "true"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", sv.ID, sv.Name, sv.Path, ro)
	}
	w.Flush()
}

func printSubvolume(sv SubVolume) {
	if outputFmt == OutputJSON {
		printJSON(sv)
		return
	}

	fmt.Printf("ID:       %d\n", sv.ID)
	fmt.Printf("名称：    %s\n", sv.Name)
	fmt.Printf("路径：    %s\n", sv.Path)
	fmt.Printf("只读：    %v\n", sv.ReadOnly)
	fmt.Printf("UUID:     %s\n", sv.UUID)
	fmt.Printf("快照数：  %d\n", len(sv.Snapshots))
}

func printSnapshots(snapshots []Snapshot) {
	if outputFmt == OutputJSON {
		printJSON(snapshots)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSOURCE\tREADONLY\tCREATED")
	for _, s := range snapshots {
		ro := "false"
		if s.ReadOnly {
			ro = "true"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Source, ro, s.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
}

func printSnapshot(s Snapshot) {
	if outputFmt == OutputJSON {
		printJSON(s)
		return
	}

	fmt.Printf("名称：    %s\n", s.Name)
	fmt.Printf("源：      %s\n", s.Source)
	fmt.Printf("只读：    %v\n", s.ReadOnly)
	fmt.Printf("创建时间：%s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("大小：    %s\n", formatSize(s.Size))
}

func printShares(shares []Share) {
	if outputFmt == OutputJSON {
		printJSON(shares)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tNAME\tPATH")
	for _, s := range shares {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Type, s.Name, s.Path)
	}
	w.Flush()
}

func printStatus(status Status, verbose bool) {
	if outputFmt == OutputJSON {
		printJSON(status)
		return
	}

	fmt.Println("📊 NAS-OS 系统状态")
	fmt.Println("================")
	fmt.Printf("版本：    %s\n", status.Version)
	fmt.Printf("运行时间：%s\n", status.Uptime)
	fmt.Printf("主机名：  %s\n", status.Hostname)
	fmt.Println()

	fmt.Println("存储服务:")
	fmt.Printf("  SMB: %v\n", statusBool(status.SMBRunning))
	fmt.Printf("  NFS: %v\n", statusBool(status.NFSRunning))
	fmt.Println()

	if verbose {
		fmt.Println("存储卷:")
		for _, v := range status.Volumes {
			fmt.Printf("  - %s: %s / %s (%.1f%%)\n",
				v.Name, formatSize(v.Used), formatSize(v.Size),
				float64(v.Used)*100/float64(v.Size))
		}
	}
}

func statusBool(b bool) string {
	if b {
		return "✓ 运行中"
	}
	return "✗ 未运行"
}

func formatSize(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func printJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(v)
}

// printYAML 保留用于未来需要 YAML 输出的场景
// func printYAML(v interface{}) {
// 	encoder := yaml.NewEncoder(os.Stdout)
// 	_ = encoder.Encode(v)
// }

// ========== 数据结构 ==========

type Volume struct {
	Name        string       `json:"name"`
	UUID        string       `json:"uuid"`
	Devices     []string     `json:"devices"`
	Size        uint64       `json:"size"`
	Used        uint64       `json:"used"`
	Free        uint64       `json:"free"`
	DataProfile string       `json:"dataProfile"`
	MetaProfile string       `json:"metaProfile"`
	MountPoint  string       `json:"mountPoint"`
	Subvolumes  []SubVolume  `json:"subvolumes"`
	Status      VolumeStatus `json:"status"`
}

type VolumeStatus struct {
	Healthy bool `json:"healthy"`
}

type SubVolume struct {
	ID        uint64     `json:"id"`
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	ParentID  uint64     `json:"parentId"`
	ReadOnly  bool       `json:"readOnly"`
	UUID      string     `json:"uuid"`
	Size      uint64     `json:"size"`
	Snapshots []Snapshot `json:"snapshots"`
}

type Snapshot struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Source     string    `json:"source"`
	SourceUUID string    `json:"sourceUuid"`
	ReadOnly   bool      `json:"readOnly"`
	CreatedAt  time.Time `json:"createdAt"`
	Size       uint64    `json:"size"`
}

type Share struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Status struct {
	Version    string   `json:"version"`
	Uptime     string   `json:"uptime"`
	Hostname   string   `json:"hostname"`
	SMBRunning bool     `json:"smbRunning"`
	NFSRunning bool     `json:"nfsRunning"`
	Volumes    []Volume `json:"volumes"`
}

// ========== API 调用函数 ==========

// apiRequest 通用 API 请求函数
func apiRequest(method, path string, body interface{}) ([]byte, error) {
	var req *http.Request
	var err error

	url := apiBaseURL + path

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, url, bytes.NewReader(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API 请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// API 响应结构
type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func apiListVolumes() ([]Volume, error) {
	respBody, err := apiRequest("GET", "/volumes", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var volumes []Volume
	if err := json.Unmarshal(resp.Data, &volumes); err != nil {
		return nil, err
	}

	return volumes, nil
}

func apiGetVolume(name string) (*Volume, error) {
	respBody, err := apiRequest("GET", "/volumes/"+name, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var volume Volume
	if err := json.Unmarshal(resp.Data, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

func apiCreateVolume(name string, devices []string, raid string) (*Volume, error) {
	body := map[string]interface{}{
		"name":    name,
		"devices": devices,
		"profile": raid,
	}

	respBody, err := apiRequest("POST", "/volumes", body)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var volume Volume
	if err := json.Unmarshal(resp.Data, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

func apiDeleteVolume(name string, force bool) error {
	_, err := apiRequest("DELETE", "/volumes/"+name+"?force="+fmt.Sprintf("%v", force), nil)
	return err
}

func apiListSubvolumes(volumeName string) ([]SubVolume, error) {
	respBody, err := apiRequest("GET", "/volumes/"+volumeName+"/subvolumes", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var subvolumes []SubVolume
	if err := json.Unmarshal(resp.Data, &subvolumes); err != nil {
		return nil, err
	}

	return subvolumes, nil
}

func apiCreateSubvolume(path string) (*SubVolume, error) {
	body := map[string]string{"path": path}
	respBody, err := apiRequest("POST", "/subvolumes", body)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var subvolume SubVolume
	if err := json.Unmarshal(resp.Data, &subvolume); err != nil {
		return nil, err
	}

	return &subvolume, nil
}

func apiDeleteSubvolume(path string, force bool) error {
	_, err := apiRequest("DELETE", "/subvolumes/"+path+"?force="+fmt.Sprintf("%v", force), nil)
	return err
}

func apiListSnapshots(volumeName string) ([]Snapshot, error) {
	respBody, err := apiRequest("GET", "/volumes/"+volumeName+"/snapshots", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var snapshots []Snapshot
	if err := json.Unmarshal(resp.Data, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

func apiCreateSnapshot(subvolume, name string, readonly bool) (*Snapshot, error) {
	body := map[string]interface{}{
		"subvolume": subvolume,
		"name":      name,
		"readonly":  readonly,
	}
	respBody, err := apiRequest("POST", "/snapshots", body)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(resp.Data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func apiRestoreSnapshot(snapshot, target string) error {
	body := map[string]string{"target": target}
	_, err := apiRequest("POST", "/snapshots/"+snapshot+"/restore", body)
	return err
}

func apiDeleteSnapshot(snapshot string) error {
	_, err := apiRequest("DELETE", "/snapshots/"+snapshot, nil)
	return err
}

func apiListShares() ([]Share, error) {
	respBody, err := apiRequest("GET", "/shares", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var shares []Share
	if err := json.Unmarshal(resp.Data, &shares); err != nil {
		return nil, err
	}

	return shares, nil
}

func apiCreateSMBShare(name, path string, guestOk bool, users []string) (*Share, error) {
	body := map[string]interface{}{
		"name":     name,
		"path":     path,
		"guest_ok": guestOk,
		"users":    users,
	}
	respBody, err := apiRequest("POST", "/shares/smb", body)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var share Share
	if err := json.Unmarshal(resp.Data, &share); err != nil {
		return nil, err
	}

	return &share, nil
}

func apiCreateNFSExport(name, path, network string) (*Share, error) {
	body := map[string]interface{}{
		"name":    name,
		"path":    path,
		"network": network,
	}
	respBody, err := apiRequest("POST", "/shares/nfs", body)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var share Share
	if err := json.Unmarshal(resp.Data, &share); err != nil {
		return nil, err
	}

	return &share, nil
}

func apiDeleteShare(name string) error {
	_, err := apiRequest("DELETE", "/shares/"+name, nil)
	return err
}

func apiGetStatus() (*Status, error) {
	respBody, err := apiRequest("GET", "/status", nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var status Status
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func apiRestart() error {
	_, err := apiRequest("POST", "/system/restart", nil)
	return err
}

func apiGetLogs(tail int, level string) ([]string, error) {
	respBody, err := apiRequest("GET", "/logs?tail="+fmt.Sprintf("%d", tail)+"&level="+level, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	var logs []string
	if err := json.Unmarshal(resp.Data, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func apiStreamLogs(level string) error {
	// 简化实现：轮询日志
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		logs, err := apiGetLogs(10, level)
		if err != nil {
			return err
		}
		for _, log := range logs {
			fmt.Println(log)
		}
	}
	return nil
}
