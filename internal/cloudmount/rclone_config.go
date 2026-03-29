// Package cloudmount - rclone 配置生成
// 为各种云盘生成 rclone.conf 配置文件
package cloudmount

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// generateRcloneConf 生成 rclone 配置文件.
func (m *Manager) generateRcloneConf() error {
	var lines []string

	// 添加注释
	lines = append(lines, "# rclone configuration for NAS-OS cloudmount")
	lines = append(lines, "# Generated automatically by cloudmount manager")
	lines = append(lines, "# Do not edit manually - use NAS-OS API instead")
	lines = append(lines, "")

	// 添加每个挂载的配置
	for _, mount := range m.mounts {
		remoteConfig := m.generateRemoteConfig(mount.Config)
		lines = append(lines, remoteConfig)
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(m.rcloneConf, []byte(content), 0600)
}

// generateRemoteConfig 生成单个远程存储配置.
func (m *Manager) generateRemoteConfig(cfg *CloudMountConfig) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("[%s]", cfg.Name))

	switch cfg.ProviderType {
	case ProviderAliyunPan:
		lines = append(lines, m.generateAliyunConfig(cfg))
	case ProviderBaiduPan:
		lines = append(lines, m.generateBaiduConfig(cfg))
	case Provider115:
		lines = append(lines, m.generate115Config(cfg))
	case ProviderQuark:
		lines = append(lines, m.generateQuarkConfig(cfg))
	case ProviderGoogle:
		lines = append(lines, m.generateGoogleConfig(cfg))
	case ProviderOneDrive:
		lines = append(lines, m.generateOneDriveConfig(cfg))
	case ProviderWebDAV:
		lines = append(lines, m.generateWebDAVConfig(cfg))
	case ProviderS3:
		lines = append(lines, m.generateS3Config(cfg))
	default:
		lines = append(lines, fmt.Sprintf("type = %s", cfg.ProviderType))
	}

	return strings.Join(lines, "\n")
}

// generateAliyunConfig 生成阿里云盘配置.
// 注意：阿里云盘需要使用 aliauth 或 OAuth2 配置
func (m *Manager) generateAliyunConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = aliyundrive",
	}

	// 使用 refresh_token 认证
	if cfg.RefreshToken != "" {
		lines = append(lines, fmt.Sprintf("refresh_token = %s", cfg.RefreshToken))
	}

	// 使用 drive_id（可选）
	if cfg.DriveID != "" {
		lines = append(lines, fmt.Sprintf("drive_id = %s", cfg.DriveID))
	}

	// 根目录（可选）
	if cfg.RemotePath != "" && cfg.RemotePath != "/" {
		lines = append(lines, fmt.Sprintf("root_folder_id = %s", m.getAliyunFolderID(cfg.RemotePath)))
	}

	return lines
}

// getAliyunFolderID 获取阿里云盘文件夹ID（需要调用API）.
func (m *Manager) getAliyunFolderID(path string) string {
	// TODO: 调用阿里云盘API获取文件夹ID
	// 目前返回空，使用根目录
	return ""
}

// generateBaiduConfig 生成百度网盘配置.
// 注意：百度网盘需要使用第三方 rclone backend
func (m *Manager) generateBaiduConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = baidu",
	}

	// 使用 refresh_token 认证
	if cfg.RefreshToken != "" {
		lines = append(lines, fmt.Sprintf("refresh_token = %s", cfg.RefreshToken))
	}

	return lines
}

// generate115Config 生成115网盘配置.
func (m *Manager) generate115Config(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = 115",
	}

	// 使用 cookies 认证（115网盘需要 cookies）
	if cfg.AccessToken != "" {
		lines = append(lines, fmt.Sprintf("cookies = %s", cfg.AccessToken))
	}

	return lines
}

// generateQuarkConfig 生成夸克网盘配置.
// 注意：夸克需要使用第三方实现
func (m *Manager) generateQuarkConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = quark",
	}

	// 使用 cookies 或 token 认证
	if cfg.AccessToken != "" {
		lines = append(lines, fmt.Sprintf("cookie = %s", cfg.AccessToken))
	}

	return lines
}

// generateGoogleConfig 生成 Google Drive 配置.
func (m *Manager) generateGoogleConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = drive",
	}

	// OAuth2 配置
	if cfg.RefreshToken != "" {
		lines = append(lines, fmt.Sprintf("refresh_token = %s", cfg.RefreshToken))
	}

	// 客户端ID（可选）
	if cfg.UserID != "" {
		lines = append(lines, fmt.Sprintf("client_id = %s", cfg.UserID))
	}

	// 根目录（可选）
	if cfg.RemotePath != "" && cfg.RemotePath != "/" {
		lines = append(lines, fmt.Sprintf("root_folder_id = %s", cfg.RemotePath))
	}

	return lines
}

// generateOneDriveConfig 生成 OneDrive 配置.
func (m *Manager) generateOneDriveConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = onedrive",
	}

	// OAuth2 配置
	if cfg.RefreshToken != "" {
		lines = append(lines, fmt.Sprintf("refresh_token = %s", cfg.RefreshToken))
	}

	// Tenant ID（可选）
	if cfg.DriveID != "" {
		lines = append(lines, fmt.Sprintf("tenant_id = %s", cfg.DriveID))
	}

	// Drive ID（可选）
	if cfg.UserID != "" {
		lines = append(lines, fmt.Sprintf("drive_id = %s", cfg.UserID))
	}

	return lines
}

// generateWebDAVConfig 生成 WebDAV 配置.
func (m *Manager) generateWebDAVConfig(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = webdav",
	}

	// URL
	if cfg.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("url = %s", cfg.Endpoint))
	}

	// 用户名密码
	if cfg.AccessKey != "" {
		lines = append(lines, fmt.Sprintf("user = %s", cfg.AccessKey))
	}
	if cfg.SecretKey != "" {
		lines = append(lines, fmt.Sprintf("pass = %s", cfg.SecretKey))
	}

	// 跳过 TLS 验证
	if cfg.Insecure {
		lines = append(lines, "tls_insecure_skip_verify = true")
	}

	return lines
}

// generateS3Config 生成 S3 配置.
func (m *Manager) generateS3Config(cfg *CloudMountConfig) []string {
	lines := []string{
		"type = s3",
	}

	// Provider（可选）
	if cfg.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("endpoint = %s", cfg.Endpoint))
	}

	// Region
	if cfg.Region != "" {
		lines = append(lines, fmt.Sprintf("region = %s", cfg.Region))
	}

	// Bucket
	if cfg.Bucket != "" {
		lines = append(lines, fmt.Sprintf("bucket = %s", cfg.Bucket))
	}

	// Access Key
	if cfg.AccessKey != "" {
		lines = append(lines, fmt.Sprintf("access_key_id = %s", cfg.AccessKey))
	}

	// Secret Key
	if cfg.SecretKey != "" {
		lines = append(lines, fmt.Sprintf("secret_access_key = %s", cfg.SecretKey))
	}

	return lines
}

// fetchRcloneAbout 获取存储使用情况.
func (m *Manager) fetchRcloneAbout(instance *MountInstance) (*MountStats, error) {
	// 使用 rclone about 命令获取存储信息
	cmdArgs := []string{
		"about",
		fmt.Sprintf("%s:", instance.Config.Name),
		"--json",
	}

	cmd := m.newRcloneCmd(cmdArgs)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取存储信息失败: %w", err)
	}

	// 解析 JSON 输出
	stats := &MountStats{}
	if err := parseAboutOutput(output, stats); err != nil {
		return nil, err
	}

	stats.UpdatedAt = instance.MountedAt
	return stats, nil
}

// parseAboutOutput 解析 rclone about 输出.
func parseAboutOutput(output []byte, stats *MountStats) error {
	// rclone about 输出格式:
	// {
	//   "total": 1099511627776,
	//   "used": 6442450944,
	//   "free": 1093067172832
	// }
	// 简化解析，使用正则或 JSON 解析

	var aboutData struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
		Free  int64 `json:"free"`
	}

	if err := parseJSON(output, &aboutData); err != nil {
		return err
	}

	stats.TotalSize = aboutData.Total
	stats.UsedSize = aboutData.Used
	stats.FreeSize = aboutData.Free
	if stats.TotalSize > 0 {
		stats.UsedPercent = float64(stats.UsedSize) / float64(stats.TotalSize) * 100
	}

	return nil
}

// parseJSON 简化 JSON 解析.
func parseJSON(data []byte, v interface{}) error {
	// 使用标准库
	import "encoding/json"
	return json.Unmarshal(data, v)
}