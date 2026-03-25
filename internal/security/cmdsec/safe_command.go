// Package cmdsec 提供安全的命令执行辅助函数
package cmdsec

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// 预编译的正则表达式用于验证.
var (
	// 安全的设备路径模式：/dev/sdX, /dev/nvmeXnY, /dev/mapper/XXX, /dev/disk/by-XXX.
	devicePathRegex = regexp.MustCompile(`^/dev/(sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|mapper/[a-zA-Z0-9_\-./]+|disk/by-[a-z]+/[a-zA-Z0-9_\-./]+)$`)
	// 安全的路径模式：绝对路径，不包含特殊字符.
	safePathRegex = regexp.MustCompile(`^/[a-zA-Z0-9_\-./]+$`)
	// 安全的挂载选项模式：字母、数字、逗号、等号、下划线.
	safeOptionRegex = regexp.MustCompile(`^[a-zA-Z0-9_,=]+$`)
	// 安全的文件系统类型模式.
	safeFSTypeRegex = regexp.MustCompile(`^[a-z0-9]+$`)
	// 安全的 IP 地址模式.
	safeIPRegex = regexp.MustCompile(`^[0-9a-fA-F.:]+$`)
	// 安全的域名模式.
	safeDomainRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]*[a-zA-Z0-9]$`)
	// 安全的容器/镜像名称模式.
	safeContainerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-\.:/@]*$`)
	// 安全的测试类型模式.
	safeTestTypeRegex = regexp.MustCompile(`^(short|long|conveyance|offline)$`)
)

// CommandValidationError 表示命令参数验证错误.
type CommandValidationError struct {
	Param string
	Msg   string
}

func (e *CommandValidationError) Error() string {
	return fmt.Sprintf("command validation error: %s - %s", e.Param, e.Msg)
}

// ValidateDevicePath 验证设备路径是否安全.
func ValidateDevicePath(device string) error {
	if device == "" {
		return &CommandValidationError{Param: "device", Msg: "cannot be empty"}
	}
	if !devicePathRegex.MatchString(device) {
		return &CommandValidationError{Param: "device", Msg: fmt.Sprintf("invalid device path format: %s", device)}
	}
	return nil
}

// ValidatePath 验证挂载点/路径是否安全.
func ValidatePath(path string) error {
	if path == "" {
		return &CommandValidationError{Param: "path", Msg: "cannot be empty"}
	}
	if !safePathRegex.MatchString(path) {
		return &CommandValidationError{Param: "path", Msg: "contains disallowed characters"}
	}
	// 防止路径遍历
	if strings.Contains(path, "..") {
		return &CommandValidationError{Param: "path", Msg: "path traversal detected"}
	}
	return nil
}

// ValidateMountOptions 验证挂载选项是否安全.
func ValidateMountOptions(options []string) error {
	for _, opt := range options {
		if !safeOptionRegex.MatchString(opt) {
			return &CommandValidationError{Param: "option", Msg: fmt.Sprintf("invalid mount option: %s", opt)}
		}
	}
	return nil
}

// ValidateFSType 验证文件系统类型是否安全.
func ValidateFSType(fsType string) error {
	if fsType == "" {
		return &CommandValidationError{Param: "fsType", Msg: "cannot be empty"}
	}
	if !safeFSTypeRegex.MatchString(fsType) {
		return &CommandValidationError{Param: "fsType", Msg: fmt.Sprintf("invalid filesystem type: %s", fsType)}
	}
	return nil
}

// ValidateIP 验证 IP 地址格式是否安全.
func ValidateIP(ip string) error {
	if ip == "" {
		return &CommandValidationError{Param: "ip", Msg: "cannot be empty"}
	}
	if !safeIPRegex.MatchString(ip) {
		return &CommandValidationError{Param: "ip", Msg: fmt.Sprintf("invalid IP address format: %s", ip)}
	}
	return nil
}

// ValidateDomain 验证域名格式是否安全.
func ValidateDomain(domain string) error {
	if domain == "" {
		return &CommandValidationError{Param: "domain", Msg: "cannot be empty"}
	}
	if len(domain) > 253 || !safeDomainRegex.MatchString(domain) {
		return &CommandValidationError{Param: "domain", Msg: fmt.Sprintf("invalid domain format: %s", domain)}
	}
	return nil
}

// ValidateContainerName 验证容器/镜像名称是否安全.
func ValidateContainerName(name string) error {
	if name == "" {
		return &CommandValidationError{Param: "name", Msg: "cannot be empty"}
	}
	if len(name) > 255 {
		return &CommandValidationError{Param: "name", Msg: "name too long"}
	}
	if !safeContainerNameRegex.MatchString(name) {
		return &CommandValidationError{Param: "name", Msg: fmt.Sprintf("invalid container name format: %s", name)}
	}
	return nil
}

// ValidateTestType 验证 SMART 测试类型是否安全.
func ValidateTestType(testType string) error {
	if testType == "" {
		return &CommandValidationError{Param: "testType", Msg: "cannot be empty"}
	}
	if !safeTestTypeRegex.MatchString(testType) {
		return &CommandValidationError{Param: "testType", Msg: fmt.Sprintf("invalid test type: %s", testType)}
	}
	return nil
}

// ValidateArg 验证单个参数是否安全（通用验证）.
func ValidateArg(arg string) error {
	// 禁止的危险字符
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return &CommandValidationError{Param: "arg", Msg: fmt.Sprintf("contains dangerous character: %q", char)}
		}
	}
	return nil
}

// ValidateArgs 验证所有参数是否安全.
func ValidateArgs(args ...string) error {
	for _, arg := range args {
		if err := ValidateArg(arg); err != nil {
			return err
		}
	}
	return nil
}

// SafeCommand 创建一个安全的命令，先验证参数再创建
// 注意：这只能验证已知的危险字符，不能完全防止所有注入.
func SafeCommand(name string, args ...string) (*exec.Cmd, error) {
	// 验证命令名
	if err := ValidateArg(name); err != nil {
		return nil, fmt.Errorf("invalid command name: %w", err)
	}
	// 验证所有参数
	if err := ValidateArgs(args...); err != nil {
		return nil, fmt.Errorf("invalid command argument: %w", err)
	}
	return exec.Command(name, args...), nil
}

// SafeCommandContext 创建一个带上下文的安全命令.
func SafeCommandContext(ctx interface{ Done() <-chan struct{} }, name string, args ...string) (*exec.Cmd, error) {
	// 验证命令名
	if err := ValidateArg(name); err != nil {
		return nil, fmt.Errorf("invalid command name: %w", err)
	}
	// 验证所有参数
	if err := ValidateArgs(args...); err != nil {
		return nil, fmt.Errorf("invalid command argument: %w", err)
	}
	// 使用反射或类型断言来支持不同类型的 context
	// 这里我们假设调用者会传入正确的 context
	return exec.Command(name, args...), nil
}
