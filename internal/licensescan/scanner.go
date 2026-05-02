package licensescan

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Scanner 许可证扫描器.
type Scanner struct {
	policy *Policy
}

// NewScanner 创建许可证扫描器.
func NewScanner(policy *Policy) *Scanner {
	return &Scanner{policy: policy}
}

// SetPolicy 更新合规策略.
func (s *Scanner) SetPolicy(policy *Policy) {
	s.policy = policy
}

// GetPolicy 获取当前策略.
func (s *Scanner) GetPolicy() *Policy {
	return s.policy
}

// ScanDockerImage 扫描Docker镜像的许可证信息.
// 通过 docker inspect 获取镜像标签中的许可证信息.
func (s *Scanner) ScanDockerImage(imageRef string) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{
		ID:        fmt.Sprintf("docker-%d", start.UnixNano()),
		ScanType:  ScanTypeDocker,
		Target:    imageRef,
		Status:    StatusRunning,
		StartedAt: start,
	}

	licenses, err := s.extractDockerLicenses(imageRef)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		return result, err
	}

	result.Licenses = make([]License, 0, len(licenses))
	for _, lic := range licenses {
		cat := ClassifyLicense(lic.Name)
		compliance := GetComplianceStatus(lic.Name, s.policy)
		result.Licenses = append(result.Licenses, License{
			Name:       lic.Name,
			SPDXID:     lic.Name,
			Category:   cat,
			Compliance: compliance,
			Source:     imageRef,
			Version:    lic.Version,
			URL:        lic.URL,
		})
	}

	result.Status = StatusComplete
	result.FinishedAt = time.Now()
	result.Summary = buildSummary(result.Licenses)
	result.Violations = findViolations(result.Licenses)

	return result, nil
}

// ScanGoMod 扫描Go模块依赖的许可证.
// 解析go.mod文件，通过已知许可证数据库匹配.
func (s *Scanner) ScanGoMod(goModPath string) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{
		ID:        fmt.Sprintf("gomod-%d", start.UnixNano()),
		ScanType:  ScanTypeGoMod,
		Target:    goModPath,
		Status:    StatusRunning,
		StartedAt: start,
	}

	// 解析go.mod获取依赖列表
	deps, err := parseGoMod(goModPath)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		return result, err
	}

	// 尝试读取go.sum获取更多许可证信息
	sumPath := strings.TrimSuffix(goModPath, ".mod") + ".sum"
	sumLicenses := parseGoSum(sumPath)

	result.Licenses = make([]License, 0, len(deps))
	for _, dep := range deps {
		licName := lookupGoLicense(dep.Path, dep.Version, sumLicenses)
		cat := ClassifyLicense(licName)
		compliance := GetComplianceStatus(licName, s.policy)
		result.Licenses = append(result.Licenses, License{
			Name:       licName,
			SPDXID:     licName,
			Category:   cat,
			Compliance: compliance,
			Source:     dep.Path,
			Version:    dep.Version,
		})
	}

	result.Status = StatusComplete
	result.FinishedAt = time.Now()
	result.Summary = buildSummary(result.Licenses)
	result.Violations = findViolations(result.Licenses)
	result.Summary.TotalPackages = len(deps)

	return result, nil
}

// ScanFull 执行全量扫描（Docker + Go mod）.
func (s *Scanner) ScanFull(dockerImages []string, goModPaths []string) []ScanResult {
	var results []ScanResult

	for _, img := range dockerImages {
		r, err := s.ScanDockerImage(img)
		if err != nil {
			r = &ScanResult{
				ID:        fmt.Sprintf("docker-err-%d", time.Now().UnixNano()),
				ScanType:  ScanTypeDocker,
				Target:    img,
				Status:    StatusFailed,
				Error:     err.Error(),
				StartedAt: time.Now(),
			}
			r.FinishedAt = time.Now()
		}
		results = append(results, *r)
	}

	for _, modPath := range goModPaths {
		r, err := s.ScanGoMod(modPath)
		if err != nil {
			r = &ScanResult{
				ID:        fmt.Sprintf("gomod-err-%d", time.Now().UnixNano()),
				ScanType:  ScanTypeGoMod,
				Target:    modPath,
				Status:    StatusFailed,
				Error:     err.Error(),
				StartedAt: time.Now(),
			}
			r.FinishedAt = time.Now()
		}
		results = append(results, *r)
	}

	return results
}

// ========== 内部实现 ==========

// dockerLicenseInfo Docker镜像许可证信息.
type dockerLicenseInfo struct {
	Name    string
	Version string
	URL     string
}

// extractDockerLicenses 从Docker镜像提取许可证信息.
func (s *Scanner) extractDockerLicenses(imageRef string) ([]dockerLicenseInfo, error) {
	// 尝试通过docker inspect获取LABEL信息
	cmd := exec.Command("docker", "inspect", "--format", "{{json .Config.Labels}}", imageRef)
	out, err := cmd.Output()
	if err != nil {
		// 如果docker不可用，返回空结果（不报错，标记为unknown）
		return []dockerLicenseInfo{
			{Name: "unknown", Version: "", URL: ""},
		}, nil
	}

	var licenses []dockerLicenseInfo
	labels := string(out)

	// 解析常见的许可证标签
	licenseKeys := []string{
		"org.opencontainers.image.licenses",
		"org.label-schema.license",
		"license",
		"Licenses",
	}

	for _, key := range licenseKeys {
		if val := extractLabel(labels, key); val != "" {
			// 可能包含多个许可证，用逗号或分号分隔
			for _, l := range splitLicenses(val) {
				l = strings.TrimSpace(l)
				if l != "" {
					licenses = append(licenses, dockerLicenseInfo{Name: l})
				}
			}
		}
	}

	if len(licenses) == 0 {
		licenses = append(licenses, dockerLicenseInfo{Name: "unknown"})
	}

	return licenses, nil
}

// extractLabel 从JSON标签字符串中提取指定key的值.
func extractLabel(jsonStr, key string) string {
	// 简单解析，查找 "key":"value" 模式
	search := `"` + key + `":`
	idx := strings.Index(jsonStr, search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	// 跳过空格
	for start < len(jsonStr) && (jsonStr[start] == ' ' || jsonStr[start] == '\t') {
		start++
	}
	if start >= len(jsonStr) || jsonStr[start] != '"' {
		return ""
	}
	start++ // 跳过开头引号
	end := start
	for end < len(jsonStr) && jsonStr[end] != '"' {
		if jsonStr[end] == '\\' {
			end++ // 跳过转义字符
		}
		end++
	}
	return jsonStr[start:end]
}

// splitLicenses 分割多个许可证字符串.
func splitLicenses(s string) []string {
	// 尝试多种分隔符
	for _, sep := range []string{",", ";", " AND ", " and ", " OR ", " or ", "\n"} {
		if strings.Contains(s, sep) {
			parts := strings.Split(s, sep)
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
			return result
		}
	}
	return []string{s}
}

// goModule Go模块依赖信息.
type goModule struct {
	Path    string
	Version string
}

// parseGoMod 解析go.mod文件，提取依赖模块列表.
func parseGoMod(goModPath string) ([]goModule, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开go.mod: %w", err)
	}
	defer f.Close()

	var modules []goModule
	scanner := bufio.NewScanner(f)
	inRequire := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过注释
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		// 检测require块开始
		if strings.HasPrefix(line, "require") {
			if strings.Contains(line, "(") {
				inRequire = true
				continue
			}
			// 单行require
			mod := parseRequireLine(line)
			if mod != nil {
				modules = append(modules, *mod)
			}
			continue
		}

		// require块结束
		if inRequire && line == ")" {
			inRequire = false
			continue
		}

		// require块内的行
		if inRequire {
			mod := parseRequireLine("require " + line)
			if mod != nil {
				modules = append(modules, *mod)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取go.mod失败: %w", err)
	}

	return modules, nil
}

// parseRequireLine 解析单行require声明.
func parseRequireLine(line string) *goModule {
	// 格式: require module version
	// 或:   module version
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil
	}

	// 跳过require关键字
	start := 0
	if fields[0] == "require" {
		start = 1
	}

	if start+1 >= len(fields) {
		return nil
	}

	path := fields[start]
	version := fields[start+1]

	// 跳过// indirect注释
	if strings.Contains(version, "//") {
		version = strings.Split(version, "//")[0]
		version = strings.TrimSpace(version)
	}

	return &goModule{
		Path:    path,
		Version: version,
	}
}

// parseGoSum 解析go.sum文件，提取模块-许可证映射.
// 返回 module@version -> license 映射.
func parseGoSum(sumPath string) map[string]string {
	result := make(map[string]string)
	f, err := os.Open(sumPath)
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// go.sum格式: module version hash
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := fields[0] + "@" + fields[1]
			// go.sum本身不包含许可证信息，但记录模块存在
			if _, exists := result[key]; !exists {
				result[key] = ""
			}
		}
	}
	return result
}

// lookupGoLicense 查询Go模块的许可证.
// 使用已知模块许可证数据库和启发式规则.
func lookupGoLicense(modulePath, version string, sumLicenses map[string]string) string {
	// 已知模块许可证数据库
	knownLicenses := map[string]string{
		"github.com/gin-gonic/gin":          "MIT",
		"github.com/gorilla/mux":            "BSD-3-Clause",
		"github.com/gorilla/websocket":      "BSD-2-Clause",
		"github.com/google/uuid":            "BSD-3-Clause",
		"github.com/sirupsen/logrus":        "MIT",
		"github.com/spf13/cobra":            "Apache-2.0",
		"github.com/spf13/viper":            "MIT",
		"github.com/stretchr/testify":       "MIT",
		"github.com/go-redis/redis":         "BSD-2-Clause",
		"github.com/go-redis/redis/v8":      "BSD-2-Clause",
		"github.com/jmoiron/sqlx":           "MIT",
		"github.com/lib/pq":                 "MIT",
		"github.com/mattn/go-sqlite3":       "MIT",
		"github.com/dgrijalva/jwt-go":       "MIT",
		"github.com/golang-jwt/jwt":         "MIT",
		"github.com/pkg/errors":             "BSD-2-Clause",
		"github.com/rs/zerolog":             "MIT",
		"go.uber.org/zap":                   "MIT",
		"golang.org/x/":                     "BSD-3-Clause",
		"google.golang.org/grpc":            "Apache-2.0",
		"google.golang.org/protobuf":        "BSD-3-Clause",
		"gopkg.in/yaml.v3":                  "Apache-2.0",
		"gopkg.in/yaml.v2":                  "Apache-2.0",
		"github.com/fsnotify/fsnotify":      "BSD-3-Clause",
		"github.com/prometheus/client_golang": "Apache-2.0",
		"github.com/docker/docker":          "Apache-2.0",
		"github.com/docker/cli":             "Apache-2.0",
		"github.com/aws/aws-sdk-go":         "Apache-2.0",
		"github.com/aws/aws-sdk-go-v2":      "Apache-2.0",
		"cloud.google.com/go":               "Apache-2.0",
		"github.com/blevesearch/bleve":      "Apache-2.0",
		"github.com/disintegration/imaging": "MIT",
		"github.com/go-playground/validator": "MIT",
		"github.com/go-ldap/ldap":           "MIT",
	}

	// 精确匹配
	if lic, ok := knownLicenses[modulePath]; ok {
		return lic
	}

	// 前缀匹配
	for prefix, lic := range knownLicenses {
		if strings.HasPrefix(modulePath, prefix) {
			return lic
		}
	}

	// 尝试读取本地模块缓存中的LICENSE文件
	if lic := readModuleLicense(modulePath, version); lic != "" {
		return lic
	}

	// 根据域名启发式判断
	domain := extractDomain(modulePath)
	switch domain {
	case "golang.org", "go.googlesource.com":
		return "BSD-3-Clause"
	case "google.golang.org":
		return "Apache-2.0"
	case "gopkg.in":
		return "Apache-2.0"
	case "k8s.io", "sigs.k8s.io":
		return "Apache-2.0"
	case "go.uber.org":
		return "MIT"
	case "bazil.org":
		return "MIT"
	}

	return "unknown"
}

// extractDomain 从模块路径中提取域名.
func extractDomain(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return modulePath
}

// readModuleLicense 尝试从Go模块缓存中读取LICENSE文件.
func readModuleLicense(modulePath, version string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	gomodCache := filepath.Join(home, "go", "pkg", "mod")
	modDir := filepath.Join(gomodCache, modulePath+"@"+version)

	// 检查常见许可证文件名
	licenseFiles := []string{
		"LICENSE", "LICENSE.md", "LICENSE.txt",
		"LICENCE", "LICENCE.md", "LICENCE.txt",
		"license", "license.md", "license.txt",
		"COPYING", "COPYING.md", "COPYING.txt",
	}

	for _, name := range licenseFiles {
		licPath := filepath.Join(modDir, name)
		data, err := os.ReadFile(licPath)
		if err != nil {
			continue
		}
		return identifyLicenseFromContent(string(data))
	}

	return ""
}

// identifyLicenseFromContent 从许可证文件内容识别许可证类型.
func identifyLicenseFromContent(content string) string {
	lower := strings.ToLower(content)

	// 检查关键短语
	switch {
	case strings.Contains(lower, "apache license") && strings.Contains(lower, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(lower, "mit license") || strings.HasPrefix(lower, "mit license"):
		return "MIT"
	case strings.Contains(lower, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(lower, "affero") && strings.Contains(lower, "general public license"):
		return "AGPL-3.0"
	case strings.Contains(lower, "gnu lesser general public license") && strings.Contains(lower, "version 3"):
		return "LGPL-3.0"
	case strings.Contains(lower, "gnu lesser general public license") && strings.Contains(lower, "version 2"):
		return "LGPL-2.1"
	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 3"):
		return "GPL-3.0"
	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 2"):
		return "GPL-2.0"
	case strings.Contains(lower, "mozilla public license") && strings.Contains(lower, "2.0"):
		return "MPL-2.0"
	case strings.Contains(lower, "bsd 3-clause") || strings.Contains(lower, "redistribution and use in source and binary"):
		return "BSD-3-Clause"
	case strings.Contains(lower, "bsd 2-clause"):
		return "BSD-2-Clause"
	case strings.Contains(lower, "isc license") || strings.Contains(lower, "permission to use, copy, modify, and/or distribute"):
		return "ISC"
	case strings.Contains(lower, "the unlicense") || strings.Contains(lower, "this is free and unencumbered"):
		return "Unlicense"
	}

	return ""
}

// buildSummary 构建扫描摘要.
func buildSummary(licenses []License) ScanSummary {
	summary := ScanSummary{TotalLicenses: len(licenses)}
	for _, lic := range licenses {
		switch lic.Compliance {
		case ComplianceAllowed:
			summary.Allowed++
		case ComplianceDenied:
			summary.Denied++
		case ComplianceReview:
			summary.ReviewRequired++
		case ComplianceUnknown:
			summary.Unknown++
		}
	}
	return summary
}

// findViolations 从扫描结果中提取违规项.
func findViolations(licenses []License) []Violation {
	var violations []Violation
	for _, lic := range licenses {
		switch lic.Compliance {
		case ComplianceDenied:
			violations = append(violations, Violation{
				LicenseName: lic.Name,
				Source:      lic.Source,
				ListType:    ListBlacklist,
				Severity:    SeverityHigh,
				Message:     fmt.Sprintf("许可证 %s 在黑名单中，禁止使用 (来源: %s)", lic.Name, lic.Source),
			})
		case ComplianceReview:
			violations = append(violations, Violation{
				LicenseName: lic.Name,
				Source:      lic.Source,
				ListType:    ListGraylist,
				Severity:    SeverityMedium,
				Message:     fmt.Sprintf("许可证 %s 需要人工审批 (来源: %s)", lic.Name, lic.Source),
			})
		case ComplianceUnknown:
			violations = append(violations, Violation{
				LicenseName: lic.Name,
				Source:      lic.Source,
				ListType:    "",
				Severity:    SeverityLow,
				Message:     fmt.Sprintf("许可证 %s 无法识别 (来源: %s)", lic.Name, lic.Source),
			})
		}
	}
	return violations
}
