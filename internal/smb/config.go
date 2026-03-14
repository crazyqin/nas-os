package smb

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// logger 全局日志记录器
var logger *zap.SugaredLogger

// SetLogger 设置日志记录器
func SetLogger(l *zap.SugaredLogger) {
	logger = l
}

// logInfo 记录信息日志
func logInfo(msg string, fields ...interface{}) {
	if logger != nil {
		logger.Infow(msg, fields...)
	}
}

// logError 记录错误日志
func logError(msg string, err error, fields ...interface{}) {
	if logger != nil {
		allFields := append(fields, "error", err)
		logger.Errorw(msg, allFields...)
	}
}

// ConfigParser SMB 配置解析器
type ConfigParser struct {
	configPath string
}

// NewConfigParser 创建配置解析器
func NewConfigParser(configPath string) *ConfigParser {
	return &ConfigParser{configPath: configPath}
}

// ParseSmbConf 解析 smb.conf 文件
func (p *ConfigParser) ParseSmbConf() (map[string]*Share, *Config, error) {
	file, err := os.Open(p.configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	shares := make(map[string]*Share)
	config := &Config{}
	currentSection := ""
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// 解析 section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(line[1 : len(line)-1])
			if currentSection != "global" {
				shares[currentSection] = &Share{
					Name:          currentSection,
					Browseable:    true,
					ReadOnly:      false,
					GuestOK:       false,
					CreateMask:    "0644",
					DirectoryMask: "0755",
				}
			}
			continue
		}

		// 解析键值对
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if currentSection == "global" {
			p.parseGlobalConfig(config, key, value)
		} else if share, exists := shares[currentSection]; exists {
			p.parseShareConfig(share, key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	return shares, config, nil
}

// parseGlobalConfig 解析全局配置
func (p *ConfigParser) parseGlobalConfig(config *Config, key, value string) {
	switch key {
	case "workgroup":
		config.Workgroup = value
	case "server string":
		config.ServerString = value
	case "server role":
		config.ServerRole = value
	case "security":
		config.Security = value
	case "log level":
		config.LogLevel = value
	case "max log size":
		config.MaxLogSize = value
	case "load printers":
		config.LoadPrinters = (value == "yes")
	case "printing":
		config.Printing = value
	case "printcap name":
		config.PrintcapName = value
	case "guest account":
		config.GuestAccount = value
	case "map to guest":
		config.MapToGuest = value
	case "usershare allow guests":
		config.UsershareAllowGuests = (value == "yes")
	case "usershare max shares":
		config.UsershareMaxShares = value
	case "usershare owner only":
		config.UsershareOwnerOnly = (value == "yes")
	case "socket options":
		config.SocketOptions = value
	case "dns proxy":
		config.DNSProxy = (value == "yes")
	case "bind interfaces only":
		config.BindInterfacesOnly = (value == "yes")
	case "interfaces":
		config.Interfaces = strings.Split(value, ",")
		for i, v := range config.Interfaces {
			config.Interfaces[i] = strings.TrimSpace(v)
		}
	case "host msdfs":
		config.HostMSDFS = (value == "yes")
	case "passdb backend":
		config.PassdbBackend = value
	case "obey pam restrictions":
		config.ObeyPAMRestrictions = (value == "yes")
	case "unix password sync":
		config.UnixPasswordSync = (value == "yes")
	case "passwd program":
		config.PasswdProgram = value
	case "passwd chat":
		config.PasswdChat = value
	case "pam password change":
		config.PAMPasswordChange = (value == "yes")
	case "username map":
		config.UsernameMap = value
	case "log file":
		config.LogFile = value
	case "min protocol":
		config.MinProtocol = value
	case "max protocol":
		config.MaxProtocol = value
	case "server min protocol":
		config.ServerMinProtocol = value
	case "client min protocol":
		config.ClientMinProtocol = value
	case "client max protocol":
		config.ClientMaxProtocol = value
	}
}

// parseShareConfig 解析共享配置
func (p *ConfigParser) parseShareConfig(share *Share, key, value string) {
	switch key {
	case "path":
		share.Path = value
	case "comment":
		share.Comment = value
	case "browseable":
		share.Browseable = (value == "yes")
	case "read only":
		share.ReadOnly = (value == "yes")
	case "writable", "writeable":
		share.ReadOnly = (value != "yes")
	case "guest ok":
		share.GuestOK = (value == "yes")
	case "public":
		share.GuestOK = (value == "yes")
	case "valid users":
		share.ValidUsers = parseUserList(value)
	case "write list":
		share.WriteList = parseUserList(value)
	case "read list":
		share.Users = parseUserList(value)
	case "create mask", "create mode":
		share.CreateMask = value
	case "directory mask", "directory mode":
		share.DirectoryMask = value
	case "force user":
		share.ForceUser = value
	case "force group":
		share.ForceGroup = value
	case "veto files":
		share.VetoFiles = parseVetoFiles(value)
	case "hide files":
		share.HideFiles = parseVetoFiles(value)
	case "store dos attributes":
		share.StoreDOSAttributes = (value == "yes")
	case "ea support":
		share.EASupport = (value == "yes")
	case "acl group control":
		share.ACLGroupControl = (value == "yes")
	case "inherit acls":
		share.InheritACLs = (value == "yes")
	case "inherit owner":
		share.InheritOwner = value
	case "inherit permissions":
		share.InheritPermissions = (value == "yes")
	case "follow symlinks":
		share.FollowSymlinks = (value == "yes")
	case "wide links":
		share.WideLinks = (value == "yes")
	case "available":
		share.Available = (value == "yes")
	case "printable":
		share.Printable = (value == "yes")
	}
}

// parseUserList 解析用户列表
func parseUserList(value string) []string {
	if value == "" {
		return nil
	}

	var users []string
	for _, part := range strings.Split(value, ",") {
		user := strings.TrimSpace(part)
		if user != "" && user != "@" && !strings.HasPrefix(user, "@") {
			users = append(users, user)
		}
	}
	return users
}

// parseVetoFiles 解析隐藏文件列表
func parseVetoFiles(value string) []string {
	if value == "" {
		return nil
	}

	// 格式: /file1/file2/file3/
	if strings.HasPrefix(value, "/") && strings.HasSuffix(value, "/") {
		value = value[1 : len(value)-1]
	}

	var files []string
	for _, f := range strings.Split(value, "/") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

// GenerateSmbConf 生成 smb.conf 内容
func GenerateSmbConf(config *Config, shares map[string]*Share) string {
	var sb strings.Builder

	// 生成全局配置
	sb.WriteString("[global]\n")

	if config.Workgroup != "" {
		sb.WriteString(fmt.Sprintf("    workgroup = %s\n", config.Workgroup))
	}
	if config.ServerString != "" {
		sb.WriteString(fmt.Sprintf("    server string = %s\n", config.ServerString))
	}
	if config.ServerRole != "" {
		sb.WriteString(fmt.Sprintf("    server role = %s\n", config.ServerRole))
	}
	if config.Security != "" {
		sb.WriteString(fmt.Sprintf("    security = %s\n", config.Security))
	}
	if config.LogLevel != "" {
		sb.WriteString(fmt.Sprintf("    log level = %s\n", config.LogLevel))
	}
	if config.MaxLogSize != "" {
		sb.WriteString(fmt.Sprintf("    max log size = %s\n", config.MaxLogSize))
	}
	if config.LogFile != "" {
		sb.WriteString(fmt.Sprintf("    log file = %s\n", config.LogFile))
	}

	// 默认配置
	sb.WriteString("    map to guest = Bad User\n")
	sb.WriteString("    dns proxy = no\n")

	if config.MinProtocol != "" {
		sb.WriteString(fmt.Sprintf("    min protocol = %s\n", config.MinProtocol))
	}
	if config.MaxProtocol != "" {
		sb.WriteString(fmt.Sprintf("    max protocol = %s\n", config.MaxProtocol))
	}

	if len(config.Interfaces) > 0 {
		sb.WriteString(fmt.Sprintf("    interfaces = %s\n", strings.Join(config.Interfaces, ", ")))
	}

	if config.PassdbBackend != "" {
		sb.WriteString(fmt.Sprintf("    passdb backend = %s\n", config.PassdbBackend))
	}

	// 打印机配置（默认禁用）
	if !config.LoadPrinters {
		sb.WriteString("    load printers = no\n")
		sb.WriteString("    printing = bsd\n")
		sb.WriteString("    printcap name = /dev/null\n")
	}

	sb.WriteString("\n")

	// 生成共享配置
	for _, share := range shares {
		sb.WriteString(fmt.Sprintf("[%s]\n", share.Name))
		sb.WriteString(fmt.Sprintf("    path = %s\n", share.Path))

		if share.Comment != "" {
			sb.WriteString(fmt.Sprintf("    comment = %s\n", share.Comment))
		}

		sb.WriteString(fmt.Sprintf("    browseable = %s\n", boolToYesNo(share.Browseable)))
		sb.WriteString(fmt.Sprintf("    read only = %s\n", boolToYesNo(share.ReadOnly)))
		sb.WriteString(fmt.Sprintf("    guest ok = %s\n", boolToYesNo(share.GuestOK)))

		if len(share.ValidUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    valid users = %s\n", strings.Join(share.ValidUsers, ", ")))
		}
		if len(share.WriteList) > 0 {
			sb.WriteString(fmt.Sprintf("    write list = %s\n", strings.Join(share.WriteList, ", ")))
		}
		if len(share.Users) > 0 {
			sb.WriteString(fmt.Sprintf("    read list = %s\n", strings.Join(share.Users, ", ")))
		}

		if share.CreateMask != "" {
			sb.WriteString(fmt.Sprintf("    create mask = %s\n", share.CreateMask))
		} else {
			sb.WriteString("    create mask = 0644\n")
		}

		if share.DirectoryMask != "" {
			sb.WriteString(fmt.Sprintf("    directory mask = %s\n", share.DirectoryMask))
		} else {
			sb.WriteString("    directory mask = 0755\n")
		}

		if share.ForceUser != "" {
			sb.WriteString(fmt.Sprintf("    force user = %s\n", share.ForceUser))
		}
		if share.ForceGroup != "" {
			sb.WriteString(fmt.Sprintf("    force group = %s\n", share.ForceGroup))
		}

		if len(share.VetoFiles) > 0 {
			sb.WriteString(fmt.Sprintf("    veto files = /%s/\n", strings.Join(share.VetoFiles, "/")))
		}

		// 额外选项
		if share.StoreDOSAttributes {
			sb.WriteString("    store dos attributes = yes\n")
		}
		if share.EASupport {
			sb.WriteString("    ea support = yes\n")
		}
		if share.FollowSymlinks {
			sb.WriteString("    follow symlinks = yes\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// boolToYesNo 将布尔值转换为 yes/no 字符串
func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// WriteSmbConf 写入 smb.conf 文件
func WriteSmbConf(configPath string, config *Config, shares map[string]*Share) error {
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	content := GenerateSmbConf(config, shares)

	// 写入临时文件
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 原子性重命名
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名配置文件失败: %w", err)
	}

	logInfo("SMB配置文件已保存", "path", configPath)
	return nil
}

// BackupSmbConf 备份 smb.conf 文件
func BackupSmbConf(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	// 创建备份
	backupPath := configPath + ".bak"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	logInfo("SMB配置文件已备份", "path", backupPath)
	return nil
}

// ValidateShareConfig 验证共享配置
func ValidateShareConfig(share *Share) error {
	if share.Name == "" {
		return fmt.Errorf("共享名称不能为空")
	}
	if share.Path == "" {
		return fmt.Errorf("共享路径不能为空")
	}

	// 验证路径格式
	if !filepath.IsAbs(share.Path) {
		return fmt.Errorf("共享路径必须是绝对路径: %s", share.Path)
	}

	// 验证 create mask 格式
	if share.CreateMask != "" {
		if err := validateOctalMask(share.CreateMask); err != nil {
			return fmt.Errorf("无效的 create mask: %w", err)
		}
	}

	// 验证 directory mask 格式
	if share.DirectoryMask != "" {
		if err := validateOctalMask(share.DirectoryMask); err != nil {
			return fmt.Errorf("无效的 directory mask: %w", err)
		}
	}

	return nil
}

// validateOctalMask 验证八进制权限掩码
func validateOctalMask(mask string) error {
	if len(mask) != 4 {
		return fmt.Errorf("掩码长度必须为4位")
	}

	for _, c := range mask {
		if c < '0' || c > '7' {
			return fmt.Errorf("掩码必须为八进制数字")
		}
	}

	return nil
}

// ValidateConfig 验证全局配置
func ValidateConfig(config *Config) error {
	// 验证 workgroup
	if config.Workgroup != "" && len(config.Workgroup) > 15 {
		return fmt.Errorf("workgroup 名称不能超过15个字符")
	}

	// 验证协议版本
	validProtocols := map[string]bool{
		"":        true,
		"SMB1":    true,
		"SMB2":    true,
		"SMB3":    true,
		"NT1":     true,
		"LANMAN1": true,
		"LANMAN2": true,
	}

	if !validProtocols[config.MinProtocol] {
		return fmt.Errorf("无效的最小协议版本: %s", config.MinProtocol)
	}

	if !validProtocols[config.MaxProtocol] {
		return fmt.Errorf("无效的最大协议版本: %s", config.MaxProtocol)
	}

	return nil
}
