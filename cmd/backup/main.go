package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"nas-os/internal/backup"
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
  backup incremental create --source /data --name mydata
  backup incremental list --name mydata
  backup cloud upload --config cloud.json --file backup.tar.gz
  backup encrypt --input backup.tar.gz --password "secret"
  backup restore full --backup mydata/latest --target /restore
`)
}

// ========== 增量备份 ==========

func handleIncremental(args []string) {
	if len(args) < 1 {
		fmt.Println("用法：backup incremental <create|list|space|delete> [options]")
		os.Exit(1)
	}

	subcmd := args[0]
	fs := flag.NewFlagSet("incremental", flag.ExitOnError)
	sourceDir := fs.String("source", "", "源目录")
	backupName := fs.String("name", "", "备份名称")
	baseDir := fs.String("dest", "/srv/backups", "备份根目录")

	fs.Parse(args[1:])

	ib := backup.NewIncrementalBackup(*baseDir)

	switch subcmd {
	case "create":
		if *sourceDir == "" || *backupName == "" {
			fmt.Println("错误：--source 和 --name 是必需的")
			os.Exit(1)
		}

		fmt.Printf("开始增量备份：%s -> %s/%s\n", *sourceDir, *baseDir, *backupName)
		result, err := ib.CreateBackup(*sourceDir, *backupName)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 备份完成\n")
		fmt.Printf("   路径：%s\n", result.BackupPath)
		fmt.Printf("   增量：%v\n", result.IsIncremental)
		fmt.Printf("   文件数：%d\n", result.TotalFiles)
		fmt.Printf("   耗时：%v\n", result.Duration)

	case "list":
		if *backupName == "" {
			fmt.Println("错误：--name 是必需的")
			os.Exit(1)
		}

		backups, err := ib.ListBackups(*backupName)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("备份列表 (%s):\n", *backupName)
		for _, b := range backups {
			fmt.Printf("  📦 %s\n", b.Name)
			if b.Metadata != nil {
				fmt.Printf("     时间：%s\n", b.Metadata.Timestamp)
				fmt.Printf("     文件：%d\n", b.Metadata.TotalFiles)
				fmt.Printf("     增量：%v\n", b.Metadata.IsIncremental)
			}
		}

	case "space":
		if *backupName == "" {
			fmt.Println("错误：--name 是必需的")
			os.Exit(1)
		}

		size, err := ib.GetSpaceUsage(*backupName)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("备份空间使用 (%s): %s\n", *backupName, formatSize(size))

	case "delete":
		timestamp := fs.String("timestamp", "", "备份时间戳")
		fs.Parse(args[1:])

		if *backupName == "" || *timestamp == "" {
			fmt.Println("错误：--name 和 --timestamp 是必需的")
			os.Exit(1)
		}

		if err := ib.DeleteBackup(*backupName, *timestamp); err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 已删除备份：%s/%s\n", *backupName, *timestamp)

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

	fs.Parse(args[1:])

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

	encryptor, err := backup.NewEncryptor(pwd)
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("加密：%s -> %s\n", *inputFile, *outputFile)
	if err := encryptor.EncryptFile(*inputFile, *outputFile); err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	// 写入校验和
	if err := backup.WriteChecksum(*outputFile); err != nil {
		fmt.Printf("警告：写入校验和失败：%v\n", err)
	}

	fmt.Printf("✅ 加密完成\n")
	fmt.Printf("   校验和：%s.sha256\n", *outputFile)
}

func handleDecrypt(args []string) {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	inputFile := fs.String("input", "", "输入文件")
	outputFile := fs.String("output", "", "输出文件")
	password := fs.String("password", "", "密码")
	passwordFile := fs.String("password-file", "", "密码文件")

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

	encryptor, err := backup.NewEncryptor(pwd)
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("解密：%s -> %s\n", *inputFile, *outputFile)
	if err := encryptor.DecryptFile(*inputFile, *outputFile); err != nil {
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

	rm := backup.NewRestoreManager("/srv/backups", "/srv/storage")

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
				fmt.Printf("     文件：%d\n", b.Metadata.TotalFiles)
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
	valid, err := backup.VerifyChecksum(*filePath)
	if err != nil {
		fmt.Printf("警告：校验和文件不存在，计算新校验和...\n")
		checksum, err := backup.VerifyIntegrity(*filePath)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SHA256: %s\n", checksum)
		if err := backup.WriteChecksum(*filePath); err != nil {
			fmt.Printf("警告：写入校验和失败：%v\n", err)
		}
	} else if valid {
		fmt.Printf("✅ 校验和验证通过\n")
	} else {
		fmt.Printf("❌ 校验和验证失败\n")
		os.Exit(1)
	}
}

// ========== 健康检查 ==========

func handleHealth(args []string) {
	manager := backup.NewManager("/etc/backup/config.json", "/srv/backups")
	if err := manager.Initialize(); err != nil {
		fmt.Printf("警告：初始化失败：%v\n", err)
	}

	result := manager.HealthCheck()

	fmt.Printf("备份系统健康检查\n")
	fmt.Printf("================\n\n")

	statusEmoji := "✅"
	if result.Status == "warning" {
		statusEmoji = "⚠️"
	} else if result.Status == "critical" {
		statusEmoji = "❌"
	}

	fmt.Printf("状态：%s %s\n\n", statusEmoji, result.Status)

	fmt.Println("检查项:")
	for _, check := range result.Checks {
		emoji := "✅"
		if check.Status == "fail" {
			emoji = "❌"
		} else if check.Status == "warn" {
			emoji = "⚠️"
		}
		fmt.Printf("  %s %s: %s\n", emoji, check.Name, check.Message)
	}

	if len(result.Recommendations) > 0 {
		fmt.Println("\n建议:")
		for _, rec := range result.Recommendations {
			fmt.Printf("  • %s\n", rec)
		}
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
