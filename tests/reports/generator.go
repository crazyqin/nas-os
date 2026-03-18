// Package reports 提供 NAS-OS 测试报告生成
// 支持 JSON、HTML、Markdown 格式的测试报告
package reports

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"time"
)

// TestReport 测试报告
type TestReport struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	GeneratedAt time.Time      `json:"generated_at"`
	Duration    time.Duration  `json:"duration"`
	Summary     TestSummary    `json:"summary"`
	Modules     []ModuleReport `json:"modules"`
	Environment Environment    `json:"environment"`
}

// TestSummary 测试摘要
type TestSummary struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped"`
	Coverage float64 `json:"coverage"`
}

// ModuleReport 模块报告
type ModuleReport struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tests       []TestResult  `json:"tests"`
	Duration    time.Duration `json:"duration"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	Skipped     int           `json:"skipped"`
}

// TestResult 测试结果
type TestResult struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"` // passed, failed, skipped
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Environment 环境信息
type Environment struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
	Hostname  string `json:"hostname"`
}

// ReportGenerator 报告生成器
type ReportGenerator struct {
	outputDir string
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(outputDir string) *ReportGenerator {
	return &ReportGenerator{
		outputDir: outputDir,
	}
}

// Generate 生成测试报告
func (g *ReportGenerator) Generate(report *TestReport) error {
	// 确保输出目录存在
	if err := os.MkdirAll(g.outputDir, 0750); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 生成 JSON 报告
	if err := g.generateJSON(report); err != nil {
		return fmt.Errorf("生成 JSON 报告失败: %w", err)
	}

	// 生成 Markdown 报告
	if err := g.generateMarkdown(report); err != nil {
		return fmt.Errorf("生成 Markdown 报告失败: %w", err)
	}

	// 生成 HTML 报告
	if err := g.generateHTML(report); err != nil {
		return fmt.Errorf("生成 HTML 报告失败: %w", err)
	}

	return nil
}

// generateJSON 生成 JSON 报告
func (g *ReportGenerator) generateJSON(report *TestReport) error {
	path := filepath.Join(g.outputDir, "test-report.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// generateMarkdown 生成 Markdown 报告
func (g *ReportGenerator) generateMarkdown(report *TestReport) error {
	path := filepath.Join(g.outputDir, "test-report.md")

	var md string
	md += fmt.Sprintf("# %s 测试报告\n\n", report.Name)
	md += fmt.Sprintf("**版本**: %s\n\n", report.Version)
	md += fmt.Sprintf("**生成时间**: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	md += fmt.Sprintf("**执行时长**: %s\n\n", report.Duration)

	// 摘要
	md += "## 测试摘要\n\n"
	md += "| 指标 | 数值 |\n"
	md += "|------|------|\n"
	md += fmt.Sprintf("| 总测试数 | %d |\n", report.Summary.Total)
	md += fmt.Sprintf("| 通过 | %d |\n", report.Summary.Passed)
	md += fmt.Sprintf("| 失败 | %d |\n", report.Summary.Failed)
	md += fmt.Sprintf("| 跳过 | %d |\n", report.Summary.Skipped)
	md += fmt.Sprintf("| 覆盖率 | %.2f%% |\n", report.Summary.Coverage)
	md += "\n"

	// 模块详情
	md += "## 模块测试结果\n\n"
	for _, module := range report.Modules {
		md += fmt.Sprintf("### %s\n\n", module.Name)
		md += fmt.Sprintf("%s\n\n", module.Description)
		md += fmt.Sprintf("- 通过: %d\n", module.Passed)
		md += fmt.Sprintf("- 失败: %d\n", module.Failed)
		md += fmt.Sprintf("- 跳过: %d\n", module.Skipped)
		md += fmt.Sprintf("- 耗时: %s\n\n", module.Duration)

		if len(module.Tests) > 0 {
			md += "| 测试名称 | 状态 | 耗时 |\n"
			md += "|----------|------|------|\n"
			for _, test := range module.Tests {
				var status string
				switch test.Status {
				case "failed":
					status = "❌"
				case "skipped":
					status = "⏭️"
				default:
					status = "✅"
				}
				md += fmt.Sprintf("| %s | %s | %s |\n", test.Name, status, test.Duration)
			}
			md += "\n"
		}
	}

	// 环境信息
	md += "## 环境信息\n\n"
	md += fmt.Sprintf("- 操作系统: %s\n", report.Environment.OS)
	md += fmt.Sprintf("- 架构: %s\n", report.Environment.Arch)
	md += fmt.Sprintf("- Go 版本: %s\n", report.Environment.GoVersion)
	md += fmt.Sprintf("- 主机名: %s\n", report.Environment.Hostname)

	return os.WriteFile(path, []byte(md), 0644)
}

// generateHTML 生成 HTML 报告
func (g *ReportGenerator) generateHTML(report *TestReport) error {
	path := filepath.Join(g.outputDir, "test-report.html")

	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} 测试报告</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 40px 20px; text-align: center; border-radius: 10px; margin-bottom: 30px; }
        .header h1 { font-size: 2.5em; margin-bottom: 10px; }
        .header .meta { opacity: 0.9; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .summary-card { background: white; padding: 25px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); text-align: center; }
        .summary-card .number { font-size: 2.5em; font-weight: bold; color: #667eea; }
        .summary-card .label { color: #666; margin-top: 5px; }
        .summary-card.passed .number { color: #10b981; }
        .summary-card.failed .number { color: #ef4444; }
        .summary-card.skipped .number { color: #f59e0b; }
        .module { background: white; border-radius: 10px; margin-bottom: 20px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .module-header { background: #f8f9fa; padding: 20px; border-bottom: 1px solid #eee; display: flex; justify-content: space-between; align-items: center; }
        .module-header h3 { color: #333; }
        .module-stats { display: flex; gap: 15px; }
        .module-stats span { padding: 5px 10px; border-radius: 5px; font-size: 0.9em; }
        .module-stats .passed { background: #d1fae5; color: #065f46; }
        .module-stats .failed { background: #fee2e2; color: #991b1b; }
        .module-stats .skipped { background: #fef3c7; color: #92400e; }
        .module-body { padding: 20px; }
        .test-list { list-style: none; }
        .test-item { padding: 10px 15px; border-bottom: 1px solid #eee; display: flex; justify-content: space-between; align-items: center; }
        .test-item:last-child { border-bottom: none; }
        .test-item .name { font-weight: 500; }
        .test-item .status { padding: 3px 10px; border-radius: 3px; font-size: 0.85em; }
        .test-item .status.passed { background: #d1fae5; color: #065f46; }
        .test-item .status.failed { background: #fee2e2; color: #991b1b; }
        .test-item .status.skipped { background: #fef3c7; color: #92400e; }
        .environment { background: white; padding: 25px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .environment h3 { margin-bottom: 15px; color: #333; }
        .environment ul { list-style: none; }
        .environment li { padding: 8px 0; border-bottom: 1px solid #eee; }
        .environment li:last-child { border-bottom: none; }
        .progress-bar { height: 8px; background: #e5e7eb; border-radius: 4px; overflow: hidden; margin-top: 10px; }
        .progress-bar .fill { height: 100%; background: linear-gradient(90deg, #10b981 0%, #10b981 {{.Summary.PassedPercent}}%, #ef4444 {{.Summary.PassedPercent}}%, #ef4444 100%); }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Name}}</h1>
            <div class="meta">
                版本 {{.Version}} | 生成于 {{.GeneratedAt.Format "2006-01-02 15:04:05"}}
            </div>
        </div>

        <div class="summary">
            <div class="summary-card">
                <div class="number">{{.Summary.Total}}</div>
                <div class="label">总测试数</div>
            </div>
            <div class="summary-card passed">
                <div class="number">{{.Summary.Passed}}</div>
                <div class="label">通过</div>
            </div>
            <div class="summary-card failed">
                <div class="number">{{.Summary.Failed}}</div>
                <div class="label">失败</div>
            </div>
            <div class="summary-card skipped">
                <div class="number">{{.Summary.Skipped}}</div>
                <div class="label">跳过</div>
            </div>
        </div>

        {{range .Modules}}
        <div class="module">
            <div class="module-header">
                <h3>{{.Name}}</h3>
                <div class="module-stats">
                    <span class="passed">✅ {{.Passed}} 通过</span>
                    <span class="failed">❌ {{.Failed}} 失败</span>
                    <span class="skipped">⏭️ {{.Skipped}} 跳过</span>
                </div>
            </div>
            <div class="module-body">
                <p>{{.Description}}</p>
                {{if .Tests}}
                <ul class="test-list">
                    {{range .Tests}}
                    <li class="test-item">
                        <span class="name">{{.Name}}</span>
                        <span class="status {{.Status}}">{{if eq .Status "passed"}}✅{{else if eq .Status "failed"}}❌{{else}}⏭️{{end}} {{.Status}}</span>
                    </li>
                    {{end}}
                </ul>
                {{end}}
            </div>
        </div>
        {{end}}

        <div class="environment">
            <h3>🖥️ 环境信息</h3>
            <ul>
                <li><strong>操作系统:</strong> {{.Environment.OS}}</li>
                <li><strong>架构:</strong> {{.Environment.Arch}}</li>
                <li><strong>Go 版本:</strong> {{.Environment.GoVersion}}</li>
                <li><strong>主机名:</strong> {{.Environment.Hostname}}</li>
            </ul>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("关闭文件失败: %v", cerr)
		}
	}()

	return t.Execute(f, report)
}

// CreateSampleReport 创建示例报告
func CreateSampleReport() *TestReport {
	return &TestReport{
		Name:        "NAS-OS",
		Version:     "v1.0.0",
		GeneratedAt: time.Now(),
		Duration:    5 * time.Second,
		Summary: TestSummary{
			Total:    100,
			Passed:   95,
			Failed:   3,
			Skipped:  2,
			Coverage: 85.5,
		},
		Modules: []ModuleReport{
			{
				Name:        "存储管理",
				Description: "btrfs 存储管理模块测试",
				Tests: []TestResult{
					{Name: "TestCreateVolume", Status: "passed", Duration: 10 * time.Millisecond},
					{Name: "TestListVolumes", Status: "passed", Duration: 5 * time.Millisecond},
					{Name: "TestDeleteVolume", Status: "passed", Duration: 8 * time.Millisecond},
				},
				Passed:   25,
				Failed:   0,
				Skipped:  0,
				Duration: 500 * time.Millisecond,
			},
			{
				Name:        "用户认证",
				Description: "用户认证和授权模块测试",
				Tests: []TestResult{
					{Name: "TestLogin", Status: "passed", Duration: 15 * time.Millisecond},
					{Name: "TestLogout", Status: "passed", Duration: 5 * time.Millisecond},
					{Name: "TestInvalidCredentials", Status: "failed", Duration: 3 * time.Millisecond, Error: "unexpected status code"},
				},
				Passed:   20,
				Failed:   2,
				Skipped:  1,
				Duration: 300 * time.Millisecond,
			},
		},
		Environment: Environment{
			OS:        "linux",
			Arch:      "arm64",
			GoVersion: "go1.21.0",
			Hostname:  "nas-server",
		},
	}
}
