// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// jsonKey 格式化 JSON 键名.
func jsonKey(key string) string {
	return "\"" + key + "\""
}

// ConvertImage 转换虚拟磁盘镜像格式.
// 使用 qemu-img convert 进行格式转换.
// sourcePath: 源文件路径.
// targetPath: 目标文件路径.
// targetFormat: 目标格式.
// progressCallback: 进度回调函数（0-100）.
func ConvertImage(ctx context.Context, sourcePath, targetPath string, targetFormat DiskFormat, progressCallback func(float64)) error {
	// 确保目标目录存在.
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 使用 qemu-img convert 进行转换.
	srcFormat, err := DetectFormat(sourcePath)
	if err != nil {
		return fmt.Errorf("检测源格式失败: %w", err)
	}

	args := []string{
		"convert",
		"-f", string(srcFormat),
		"-O", string(targetFormat),
		"-p", // 显示进度
		sourcePath,
		targetPath,
	}

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "qemu-img", args...)

	// 捕获 stderr 获取进度信息.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 qemu-img 失败: %w", err)
	}

	// 读取进度.
	if progressCallback != nil {
		go readQemuProgress(stderr, progressCallback)
	}

	if err := cmd.Wait(); err != nil {
		// 清理失败的输出文件.
		os.Remove(targetPath) //nolint:errcheck
		return fmt.Errorf("格式转换失败: %w", err)
	}

	return nil
}

// readQemuProgress 读取 qemu-img 进度信息.
func readQemuProgress(stderr interface{ Read([]byte) (int, error) }, callback func(float64)) {
	buf := make([]byte, 256)
	var accumulated []byte

	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			accumulated = append(accumulated, buf[:n]...)
			line := string(accumulated)

			// qemu-img 输出格式: (xx.xx/100%)
			if idx := strings.Index(line, "("); idx >= 0 {
				endIdx := strings.Index(line[idx:], "%)")
				if endIdx > 0 {
					progressStr := line[idx+1 : idx+endIdx]
					if p, parseErr := strconv.ParseFloat(progressStr, 64); parseErr == nil {
						callback(p)
					}
					accumulated = nil
				}
			}

			// 检查换行符.
			if strings.Contains(string(buf[:n]), "\n") {
				accumulated = nil
			}
		}

		if err != nil {
			break
		}
	}
}

// GetQemuImageInfo 使用 qemu-img info 获取镜像信息.
func GetQemuImageInfo(path string) (format DiskFormat, virtualSize int64, err error) {
	//nolint:gosec
	cmd := exec.Command("qemu-img", "info", "--output=json", path)
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		return "", 0, fmt.Errorf("获取镜像信息失败: %w", cmdErr)
	}

	// 简单解析 JSON 输出.
	outputStr := string(output)

	// 提取 format.
	formatKey := jsonKey("format")
	if fmtIdx := strings.Index(outputStr, formatKey); fmtIdx >= 0 {
		afterFormat := outputStr[fmtIdx:]
		if colonIdx := strings.Index(afterFormat, ":"); colonIdx >= 0 {
			afterColon := strings.TrimSpace(afterFormat[colonIdx+1:])
			// 去掉引号.
			if strings.HasPrefix(afterColon, "\"") {
				endQuote := strings.Index(afterColon[1:], "\"")
				if endQuote >= 0 {
					format = DiskFormat(afterColon[1 : endQuote+1])
				}
			}
		}
	}

	// 提取 virtual-size.
	vsKey := jsonKey("virtual-size")
	if vsIdx := strings.Index(outputStr, vsKey); vsIdx >= 0 {
		afterVS := outputStr[vsIdx:]
		if colonIdx := strings.Index(afterVS, ":"); colonIdx >= 0 {
			afterColon := strings.TrimSpace(afterVS[colonIdx+1:])
			// 找到数字结尾.
			endIdx := 0
			for i, c := range afterColon {
				if c >= '0' && c <= '9' {
					endIdx = i + 1
				} else {
					break
				}
			}
			if endIdx > 0 {
				virtualSize, _ = strconv.ParseInt(afterColon[:endIdx], 10, 64)
			}
		}
	}

	if format == "" {
		format = FormatRAW
	}

	return format, virtualSize, nil
}

// GetVirtualSize 获取虚拟磁盘大小.
func GetVirtualSize(path string) (int64, error) {
	_, size, err := GetQemuImageInfo(path)
	if err != nil {
		// 如果 qemu-img 不可用，回退到文件大小.
		info, statErr := os.Stat(path)
		if statErr != nil {
			return 0, statErr
		}
		return info.Size(), nil
	}
	return size, nil
}
