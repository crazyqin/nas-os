package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"nas-os/internal/backup"

	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "incremental":
		handleIncremental(os.Args[2:])
	case "cloud":
		handleCloud(os.Args[2:])
	case "encrypt":
		handleEncrypt(os.Args[2:])
	case "decrypt":
		handleDecrypt(os.Args[2:])
	case "restore":
		handleRestore(os.Args[2:])
	case "verify":
		handleVerify(os.Args[2:])
	case "health":
		handleHealth(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令：%s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`备份增强工具

用法：backup <command> [options]

命令:
  incremental  增量备份管理
  cloud        云端备份操作
  encrypt      加密备份
  decrypt      解密备份
  restore      恢复备份
  verify       验证备份
  health       健康检查

示例:
  backup incremental create --source /data --dest /backup
  backup incremental list
  backup cloud upload --config cloud.json --file backup.tar.gz
  backup encrypt --input backup.tar.gz --password "secret"
  backup restore full --backup mydata --target /restore
`)
}

// ========== 增量备份 ==========

func handleIncremental(args []string) {
	if len(args) < 1 {
		fmt.Println("用法：backup incremental <create|list|delete|stats> [options]")
		os.Exit(1)
	}

	subcmd := args[0]
	fs := flag.NewFlagSet("incremental", flag.ExitOnError)
	sourceDir := fs.String("source", "", "源目录")
	destDir := fs.String("dest", "/srv/backups", "备份目录")
	chunkDir := fs.String("chunks", "/srv/chunks", "块存储目录")
	snapshotID := fs.String("id", "", "快照ID")

	_ = fs.Parse(args[1:])

	logger := zap.NewNop()
	config := &backup.BackupConfig{
		BackupPath: *destDir,
		ChunkPath:  *chunkDir,
	}
	ib := backup.NewIncrementalBackup(config, logger)

	switch subcmd {
	case "create":
		if *sourceDir == "" {
			fmt.Println("错误：--source 是必需的")
			os.Exit(1)
		}

		fmt.Printf("开始增量备份：%s -> %s\n", *sourceDir, *destDir)
		snapshot, err := ib.CreateSnapshot(context.TODO(), *sourceDir, *destDir, backup.SnapshotTypeInc)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 备份完成\n")
		fmt.Printf("   快照ID：%s\n", snapshot.ID)
		fmt.Printf("   类型：%s\n", snapshot.Type)
		fmt.Printf("   文件数：%d\n", len(snapshot.Files))
		fmt.Printf("   大小：%s\n", formatSize(snapshot.Size))
		fmt.Printf("   耗时：%v\n", snapshot.Duration)

	case "list":
		snapshots := ib.ListSnapshots()
		fmt.Printf("快照列表:\n")
		for _, s := range snapshots {
			fmt.Printf("  📦 %s\n", s.ID)
			fmt.Printf("     时间：%s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("     类型：%s\n", s.Type)
			fmt.Printf("     文件：%d\n", len(s.Files))
			fmt.Printf("     大小：%s\n", formatSize(s.Size))
			fmt.Printf("     状态：%s\n", s.Status)
		}

	case "delete":
		if *snapshotID == "" {
			fmt.Println("错误：--id 是必需的")
			os.Exit(1)
		}

		if err := ib.DeleteSnapshot(*snapshotID); err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 已删除快照：%s\n", *snapshotID)

	case "stats":
		stats := ib.GetStats()
		fmt.Printf("备份统计:\n")
		fmt.Printf("  快照总数：%v\n", stats["total_snapshots"])
		fmt.Printf("  总大小：%v\n", stats["total_size"])
		fmt.Printf("  活动任务：%v\n", stats["active_jobs"])

	default:
		fmt.Printf("未知子命令：%s\n", subcmd)
		os.Exit(1)
	}
}

// ========== 云端备份 ==========

func handleCloud(args []string) {
	if len(args) < 1 {
		fmt.Println("用法：backup cloud <upload|download|list|verify|delete> [options]")
		os.Exit(1)
	}

	subcmd := args[0]
	fs := flag.NewFlagSet("cloud", flag.ExitOnError)
	configFile := fs.String("config", "", "云端配置文件")
	localFile := fs.String("file", "", "本地文件路径")
	remotePath := fs.String("remote", "", "云端路径")

	_ = fs.Parse(args[1:])

	if *configFile == "" {
		fmt.Println("错误：--config 是必需的")
		os.Exit(1)
	}

	cfgData, err := os.ReadFile(*configFile)
	if err != nil {
		fmt.Printf("错误：读取配置文件失败：%v\n", err)
		os.Exit(1)
	}

	var cfg backup.CloudConfig
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		fmt.Printf("错误：解析配置文件失败：%v\n", err)
		os.Exit(1)
	}

	cb, err := backup.NewCloudBackup(cfg)
	if err != nil {
		fmt.Printf("错误：创建云端客户端失败：%v\n", err)
		os.Exit(1)
	}

	switch subcmd {
	case "upload":
		if *localFile == "" || *remotePath == "" {
			fmt.Println("错误：--file 和 --remote 是必需的")
			os.Exit(1)
		}

		fmt.Printf("上传：%s -> %s\n", *localFile, *remotePath)
		result, err := cb.UploadBackup(*localFile, *remotePath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 上传完成\n")
		fmt.Printf("   云端路径：%s\n", result.RemotePath)
		fmt.Printf("   大小：%s\n", formatSize(result.Size))
		fmt.Printf("   耗时：%v\n", result.Duration)

	case "download":
		if *remotePath == "" || *localFile == "" {
			fmt.Println("错误：--remote 和 --file 是必需的")
			os.Exit(1)
		}

		fmt.Printf("下载：%s -> %s\n", *remotePath, *localFile)
		result, err := cb.DownloadBackup(*remotePath, *localFile)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 下载完成\n")
		fmt.Printf("   本地路径：%s\n", result.LocalPath)
		fmt.Printf("   大小：%s\n", formatSize(result.Size))
		fmt.Printf("   耗时：%v\n", result.Duration)

	case "list":
		prefix := fs.String("prefix", "", "路径前缀")
		fs.Parse(args[1:])

		backups, err := cb.ListBackups(*prefix)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("云端备份列表:\n")
		for _, b := range backups {
			fmt.Printf("  📦 %s\n", b.Name)
			fmt.Printf("     路径：%s\n", b.Path)
			fmt.Printf("     大小：%s\n", formatSize(b.Size))
			fmt.Printf("     时间：%v\n", b.CreatedAt)
		}

	case "verify":
		if *remotePath == "" {
			fmt.Println("错误：--remote 是必需的")
			os.Exit(1)
		}

		fmt.Printf("验证：%s\n", *remotePath)
		valid, err := cb.VerifyBackup(*remotePath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		if valid {
			fmt.Printf("✅ 备份验证通过\n")
		} else {
			fmt.Printf("❌ 备份验证失败\n")
			os.Exit(1)
		}

	case "delete":
		if *remotePath == "" {
			fmt.Println("错误：--remote 是必需的")
			os.Exit(1)
		}

		fmt.Printf("删除：%s\n", *remotePath)
		if err := cb.DeleteBackup(*remotePath); err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 已删除\n")

	default:
		fmt.Printf("未知子命令：%s\n", subcmd)
		os.Exit(1)
	}
}

// ========== 加密/解密 ==========

func handleEncrypt(args []string) {
	fs := flag.NewFlagSet("encrypt", flag.ExitOnError)
	inputFile := fs.String("input", "", "输入文件")
	outputFile := fs.String("output", "", "输出文件")
	password := fs.String("password", "", "密码")
	passwordFile := fs.String("password-file", "", "密码文件")
	keyPath := fs.String("key-path", "/srv/keys", "密钥存储路径")

	fs.Parse(args)

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("错误：--input 和 --output 是必需的")
		os.Exit(1)
	}

	var pwd string
	if *password != "" {
		pwd = *password
	} else if *passwordFile != "" {
		data, err := os.ReadFile(*passwordFile)
		if err != nil {
			fmt.Printf("错误：读取密码文件失败：%v\n", err)
			os.Exit(1)
		}
		pwd = string(data)
	} else {
		fmt.Println("错误：需要 --password 或 --password-file")
		os.Exit(1)
	}

	logger := zap.NewNop()
	emConfig := &backup.EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       *keyPath,
		SaltLength:    16,
		Iterations:    100000,
	}
	em := backup.NewEncryptionManager(emConfig, logger)

	key, err := em.GenerateKey(pwd)
	if err != nil {
		fmt.Printf("错误：生成密钥失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("加密：%s -> %s\n", *inputFile, *outputFile)
	if err := em.EncryptFile(*inputFile, *outputFile, key.ID); err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	// 写入校验和
	if err := writeChecksum(*outputFile); err != nil {
		fmt.Printf("警告：写入校验和失败：%v\n", err)
	}

	fmt.Printf("✅ 加密完成\n")
	fmt.Printf("   密钥ID：%s\n", key.ID)
	fmt.Printf("   校验和：%s.sha256\n", *outputFile)
}

func handleDecrypt(args []string) {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	inputFile := fs.String("input", "", "输入文件")
	outputFile := fs.String("output", "", "输出文件")
	_ = fs.String("password", "", "密码（未使用）")
	_ = fs.String("password-file", "", "密码文件（未使用）")
	keyPath := fs.String("key-path", "/srv/keys", "密钥存储路径")
	keyID := fs.String("key-id", "", "密钥ID")

	fs.Parse(args)

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("错误：--input 和 --output 是必需的")
		os.Exit(1)
	}

	if *keyID == "" {
		fmt.Println("错误：需要 --key-id")
		os.Exit(1)
	}

	logger := zap.NewNop()
	emConfig := &backup.EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       *keyPath,
		SaltLength:    16,
		Iterations:    100000,
	}
	em := backup.NewEncryptionManager(emConfig, logger)

	fmt.Printf("解密：%s -> %s\n", *inputFile, *outputFile)
	if err := em.DecryptFile(*inputFile, *outputFile, *keyID); err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 解密完成\n")
}

// ========== 恢复 ==========

func handleRestore(args []string) {
	if len(args) < 1 {
		fmt.Println("用法：backup restore <full|file|quick|list|preview> [options]")
		os.Exit(1)
	}

	subcmd := args[0]
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	backupID := fs.String("backup", "", "备份 ID 或路径")
	targetPath := fs.String("target", "", "恢复目标路径")
	overwrite := fs.Bool("overwrite", false, "覆盖现有文件")
	verify := fs.Bool("verify", false, "恢复后验证")
	filePath := fs.String("file", "", "单文件恢复路径")
	fs.Bool("dry-run", false, "预览模式")

	fs.Parse(args[1:])

	logger := zap.NewNop()
	emConfig := &backup.EncryptionManagerConfig{
		Enabled:   false,
		KeyPath:   "/srv/keys",
		Algorithm: "aes-256-gcm",
	}
	rm := backup.NewRestoreManager("/srv/backups", "/srv/storage", backup.NewEncryptionManager(emConfig, logger))

	switch subcmd {
	case "list":
		backups, err := rm.ListAllBackups()
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Println("可用备份:")
		for _, b := range backups {
			fmt.Printf("  📦 %s\n", b.Name)
			if b.Metadata != nil {
				fmt.Printf("     时间：%s\n", b.Metadata.Timestamp)
				fmt.Printf("     文件：%d\n", b.Metadata.FileCount)
				fmt.Printf("     大小：%s\n", formatSize(b.Metadata.Size))
			}
		}

	case "preview":
		if *backupID == "" {
			fmt.Println("错误：--backup 是必需的")
			os.Exit(1)
		}

		result, err := rm.Restore(backup.RestoreOptionsExtended{
			BackupID: *backupID,
			DryRun:   true,
		})
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("预览恢复 (%s):\n", *backupID)
		fmt.Printf("   将恢复 %d 个文件\n", result.TotalFiles)
		fmt.Printf("   总大小：%s\n", formatSize(result.TotalSize))

	case "full":
		if *backupID == "" || *targetPath == "" {
			fmt.Println("错误：--backup 和 --target 是必需的")
			os.Exit(1)
		}

		fmt.Printf("恢复：%s -> %s\n", *backupID, *targetPath)
		result, err := rm.Restore(backup.RestoreOptionsExtended{
			BackupID:    *backupID,
			TargetPath:  *targetPath,
			Overwrite:   *overwrite,
			VerifyAfter: *verify,
		})
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 恢复完成\n")
		fmt.Printf("   文件数：%d\n", result.RestoredFiles)
		fmt.Printf("   大小：%s\n", formatSize(result.TotalSize))
		fmt.Printf("   耗时：%v\n", result.Duration)

	case "file":
		if *backupID == "" || *filePath == "" || *targetPath == "" {
			fmt.Println("错误：--backup, --file, --target 是必需的")
			os.Exit(1)
		}

		fmt.Printf("恢复文件：%s/%s -> %s\n", *backupID, *filePath, *targetPath)
		if _, err := rm.RestoreSingleFile(*backupID, *filePath, *targetPath); err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 文件恢复完成\n")

	case "quick":
		if *backupID == "" || *targetPath == "" {
			fmt.Println("错误：--backup 和 --target 是必需的")
			os.Exit(1)
		}

		fmt.Printf("快速恢复：%s -> %s\n", *backupID, *targetPath)
		result, err := rm.QuickRestore(*backupID, *targetPath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 快速恢复完成\n")
		fmt.Printf("   文件数：%d\n", result.RestoredFiles)
		fmt.Printf("   耗时：%v\n", result.Duration)

	default:
		fmt.Printf("未知子命令：%s\n", subcmd)
		os.Exit(1)
	}
}

// ========== 验证 ==========

func handleVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	filePath := fs.String("file", "", "文件路径")

	fs.Parse(args)

	if *filePath == "" {
		fmt.Println("错误：--file 是必需的")
		os.Exit(1)
	}

	fmt.Printf("验证：%s\n", *filePath)

	// 验证校验和
	checksumFile := *filePath + ".sha256"
	if _, err := os.Stat(checksumFile); err == nil {
		// 读取存储的校验和
		storedChecksum, err := os.ReadFile(checksumFile)
		if err != nil {
			fmt.Printf("错误：读取校验和文件失败：%v\n", err)
			os.Exit(1)
		}

		// 计算当前校验和
		currentChecksum, err := calculateChecksum(*filePath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		if hex.EncodeToString(currentChecksum) == string(storedChecksum)[:64] {
			fmt.Printf("✅ 校验和验证通过\n")
		} else {
			fmt.Printf("❌ 校验和验证失败\n")
			os.Exit(1)
		}
	} else {
		fmt.Printf("校验和文件不存在，计算新校验和...\n")
		checksum, err := calculateChecksum(*filePath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SHA256: %s\n", hex.EncodeToString(checksum))
		if err := writeChecksum(*filePath); err != nil {
			fmt.Printf("警告：写入校验和失败：%v\n", err)
		}
	}
}

// ========== 健康检查 ==========

func handleHealth(args []string) {
	manager := backup.NewManager("/etc/backup/config.json", "/srv/backups")
	if err := manager.Initialize(); err != nil {
		fmt.Printf("警告：初始化失败：%v\n", err)
	}

	result, err := manager.HealthCheck()
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("备份系统健康检查\n")
	fmt.Printf("================\n\n")

	statusEmoji := "✅"
	if result.Status == "warning" {
		statusEmoji = "⚠️"
	} else if result.Status == "critical" {
		statusEmoji = "❌"
	}

	fmt.Printf("状态：%s %s\n\n", statusEmoji, result.Status)

	fmt.Println("详情:")
	for k, v := range result.Details {
		fmt.Printf("  %s: %v\n", k, v)
	}
}

// ========== 辅助函数 ==========

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
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

func calculateChecksum(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func writeChecksum(filePath string) error {
	checksum, err := calculateChecksum(filePath)
	if err != nil {
		return err
	}

	checksumFile := filePath + ".sha256"
	return os.WriteFile(checksumFile, []byte(hex.EncodeToString(checksum)+"  "+filepath.Base(filePath)+"\n"), 0644)
}

// BackupResult 用于命令行输出的备份结果
type BackupResult struct {
	BackupPath    string        `json:"backupPath"`
	IsIncremental bool          `json:"isIncremental"`
	TotalFiles    int           `json:"totalFiles"`
	Duration      time.Duration `json:"duration"`
}
