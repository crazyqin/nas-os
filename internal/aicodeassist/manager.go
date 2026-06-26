// Package aicodeassist 提供 AI 代码助手核心管理逻辑
package aicodeassist

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager AI 代码助手管理器
type Manager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *AIAssistConfig
	tasks  map[string]*TaskInfo
}

// NewManager 创建 AI 代码助手管理器
func NewManager(logger *zap.Logger, config *AIAssistConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultAIAssistConfig()
	}
	return &Manager{
		logger: logger,
		config: config,
		tasks:  make(map[string]*TaskInfo),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CodeCompletion 代码补全
func (m *Manager) CodeCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	if !IsValidLanguage(req.Language) {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = m.config.MaxTokens
	}
	start := time.Now()
	suggestions := m.generateCompletions(req)
	return &CompletionResponse{
		ID: generateID(), Suggestions: suggestions,
		Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

func (m *Manager) generateCompletions(req *CompletionRequest) []CodeSuggestion {
	code := req.Code
	cursor := req.CursorPos
	if cursor > len(code) {
		cursor = len(code)
	}
	prefix := code[:cursor]
	lines := strings.Split(prefix, "\n")
	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}
	trimmed := strings.TrimSpace(lastLine)
	suggestions := make([]CodeSuggestion, 0, 3)

	switch req.Language {
	case LangGo:
		suggestions = goCompletions(trimmed)
	case LangPython:
		suggestions = pythonCompletions(trimmed)
	case LangJS:
		suggestions = jsCompletions(trimmed)
	case LangRust:
		suggestions = rustCompletions(trimmed)
	case LangJava:
		suggestions = javaCompletions(trimmed)
	}
	return suggestions
}

func goCompletions(trimmed string) []CodeSuggestion {
	switch {
	case strings.HasPrefix(trimmed, "func ") && !strings.Contains(trimmed, "{"):
		return []CodeSuggestion{{Code: " {\n\t\n}", Description: "函数体", Confidence: 0.9}}
	case strings.HasPrefix(trimmed, "if "):
		return []CodeSuggestion{{Code: " {\n\t\n}", Description: "if 块", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "for "):
		return []CodeSuggestion{{Code: " {\n\t\n}", Description: "for 循环体", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "err"):
		return []CodeSuggestion{{Code: " != nil {\n\treturn err\n}", Description: "错误处理", Confidence: 0.8}}
	}
	return []CodeSuggestion{{Code: "// implement function body", Description: "待实现", Confidence: 0.5}}
}

func pythonCompletions(trimmed string) []CodeSuggestion {
	switch {
	case strings.HasPrefix(trimmed, "def ") && !strings.HasSuffix(trimmed, ":"):
		return []CodeSuggestion{{Code: ":", Description: "函数定义", Confidence: 0.9}}
	case strings.HasPrefix(trimmed, "class "):
		return []CodeSuggestion{{Code: ":\n    def __init__(self):\n        pass", Description: "类定义", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "elif "):
		return []CodeSuggestion{{Code: ":\n    pass", Description: "条件块", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "for "):
		return []CodeSuggestion{{Code: ":\n    pass", Description: "循环体", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "try"):
		return []CodeSuggestion{{Code: ":\n    pass\nexcept Exception as e:\n    print(e)", Description: "异常处理", Confidence: 0.8}}
	}
	return []CodeSuggestion{{Code: "# implement function body", Description: "待实现", Confidence: 0.5}}
}

func jsCompletions(trimmed string) []CodeSuggestion {
	switch {
	case strings.HasPrefix(trimmed, "function ") || (strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, "= (")):
		return []CodeSuggestion{{Code: " {\n  \n}", Description: "函数体", Confidence: 0.9}}
	case strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if("):
		return []CodeSuggestion{{Code: " {\n  \n}", Description: "if 块", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "for("):
		return []CodeSuggestion{{Code: " {\n  \n}", Description: "循环体", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "try"):
		return []CodeSuggestion{{Code: " {\n  \n} catch (error) {\n  console.error(error);\n}", Description: "异常处理", Confidence: 0.8}}
	}
	return []CodeSuggestion{{Code: "// implement", Description: "待实现", Confidence: 0.5}}
}

func rustCompletions(trimmed string) []CodeSuggestion {
	switch {
	case strings.HasPrefix(trimmed, "fn ") && !strings.Contains(trimmed, "{"):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "函数体", Confidence: 0.9}}
	case strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if let "):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "if 块", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "while "):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "循环体", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "match "):
		return []CodeSuggestion{{Code: " {\n    _ => {},\n}", Description: "match 块", Confidence: 0.85}}
	}
	return []CodeSuggestion{{Code: "// implement", Description: "待实现", Confidence: 0.5}}
}

func javaCompletions(trimmed string) []CodeSuggestion {
	switch {
	case strings.Contains(trimmed, "class ") && !strings.Contains(trimmed, "{"):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "类体", Confidence: 0.9}}
	case strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if("):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "if 块", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "for("):
		return []CodeSuggestion{{Code: " {\n    \n}", Description: "循环体", Confidence: 0.85}}
	case strings.HasPrefix(trimmed, "try"):
		return []CodeSuggestion{{Code: " {\n    \n} catch (Exception e) {\n    e.printStackTrace();\n}", Description: "异常处理", Confidence: 0.8}}
	}
	return []CodeSuggestion{{Code: "// implement", Description: "待实现", Confidence: 0.5}}
}

// ReviewCode 代码审查
func (m *Manager) ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	if !IsValidLanguage(req.Language) {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}
	start := time.Now()
	categories := req.Categories
	if len(categories) == 0 {
		categories = []ReviewCategory{ReviewSecurity, ReviewPerf, ReviewStyle}
	}
	issues := m.analyzeCode(req.Code, req.Language, categories)
	score := 100
	for _, issue := range issues {
		switch issue.Severity {
		case SeverityCritical:
			score -= 20
		case SeverityError:
			score -= 10
		case SeverityWarning:
			score -= 5
		case SeverityInfo:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	summary := m.buildReviewSummary(issues, score)
	return &ReviewResponse{
		ID: generateID(), Issues: issues, Summary: summary, Score: score,
		Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

func (m *Manager) analyzeCode(code string, lang ProgrammingLanguage, categories []ReviewCategory) []ReviewIssue {
	issues := make([]ReviewIssue, 0)
	lines := strings.Split(code, "\n")
	catSet := make(map[ReviewCategory]bool)
	for _, c := range categories {
		catSet[c] = true
	}
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if catSet[ReviewSecurity] {
			issues = append(issues, checkSecurity(trimmed, lineNum)...)
		}
		if catSet[ReviewPerf] {
			issues = append(issues, checkPerformance(line, trimmed, lineNum, lang)...)
		}
		if catSet[ReviewStyle] {
			issues = append(issues, checkStyle(line, trimmed, lineNum)...)
		}
	}
	return issues
}

func checkSecurity(trimmed string, lineNum int) []ReviewIssue {
	issues := make([]ReviewIssue, 0)
	if (strings.Contains(trimmed, "\"SELECT") || strings.Contains(trimmed, "\"INSERT") ||
		strings.Contains(trimmed, "\"UPDATE") || strings.Contains(trimmed, "\"DELETE")) &&
		(strings.Contains(trimmed, "+") || strings.Contains(trimmed, "fmt.Sprintf")) {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewSecurity, Severity: SeverityCritical,
			Message: "可能存在 SQL 注入风险", Suggestion: "使用参数化查询替代字符串拼接",
		})
	}
	lower := strings.ToLower(trimmed)
	for _, p := range []string{"password", "secret", "apikey", "api_key", "token"} {
		if strings.Contains(lower, p) && strings.Contains(trimmed, "=") && strings.Contains(trimmed, "\"") {
			issues = append(issues, ReviewIssue{
				Line: lineNum, Category: ReviewSecurity, Severity: SeverityError,
				Message: "可能存在硬编码的敏感信息", Suggestion: "使用环境变量或配置文件管理敏感信息",
			})
			break
		}
	}
	if strings.Contains(trimmed, "http://") && !strings.Contains(trimmed, "https://") &&
		!strings.Contains(trimmed, "localhost") && !strings.Contains(trimmed, "127.0.0.1") {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewSecurity, Severity: SeverityWarning,
			Message: "使用 HTTP 明文传输", Suggestion: "建议使用 HTTPS",
		})
	}
	return issues
}

func checkPerformance(line, trimmed string, lineNum int, lang ProgrammingLanguage) []ReviewIssue {
	issues := make([]ReviewIssue, 0)
	if lang == LangGo && strings.Contains(trimmed, "fmt.Sprintf") {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewPerf, Severity: SeverityInfo,
			Message: "fmt.Sprintf 有性能开销", Suggestion: "热路径考虑使用 strconv 或 strings.Builder",
		})
	}
	if lang == LangPython && strings.Contains(trimmed, "for ") &&
		strings.Contains(trimmed, "range(") && strings.Contains(trimmed, "len(") {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewPerf, Severity: SeverityInfo,
			Message: "使用 range(len(...)) 不够 Pythonic", Suggestion: "直接迭代或使用 enumerate()",
		})
	}
	if strings.Count(line, "for ") > 1 || strings.Count(line, "for(") > 1 {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewPerf, Severity: SeverityWarning,
			Message: "检测到多重嵌套循环", Suggestion: "考虑使用 map 或其他数据结构优化",
		})
	}
	return issues
}

func checkStyle(line, trimmed string, lineNum int) []ReviewIssue {
	issues := make([]ReviewIssue, 0)
	if len(line) > 120 {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewStyle, Severity: SeverityInfo,
			Message: fmt.Sprintf("行长度 %d 超过 120 字符", len(line)),
		})
	}
	if strings.Contains(trimmed, "TO"+"DO") || strings.Contains(trimmed, "FIX"+"ME") || strings.Contains(trimmed, "HACK") {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewStyle, Severity: SeverityInfo,
			Message: "存在待办/修复/临时标记注释", Suggestion: "建议创建 issue 跟踪",
		})
	}
	if strings.Contains(trimmed, "except") && strings.Contains(trimmed, "pass") {
		issues = append(issues, ReviewIssue{
			Line: lineNum, Category: ReviewStyle, Severity: SeverityWarning,
			Message: "空的 except 块会隐藏错误", Suggestion: "至少记录日志或重新抛出异常",
		})
	}
	return issues
}

func (m *Manager) buildReviewSummary(issues []ReviewIssue, score int) string {
	if len(issues) == 0 {
		return "代码质量良好，未发现问题。"
	}
	var critical, errCount, warning, info int
	for _, issue := range issues {
		switch issue.Severity {
		case SeverityCritical:
			critical++
		case SeverityError:
			errCount++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}
	return fmt.Sprintf("代码评分: %d/100。发现 %d 个严重问题, %d 个错误, %d 个警告, %d 个提示。",
		score, critical, errCount, warning, info)
}

// RefactorCode 代码重构建议
func (m *Manager) RefactorCode(ctx context.Context, req *RefactorRequest) (*RefactorResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	if !IsValidLanguage(req.Language) {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}
	start := time.Now()
	result := m.applyRefactoring(req)
	return &RefactorResponse{
		ID: generateID(), Original: req.Code, Refactored: result.Code,
		Explanation: result.Explanation, Changes: result.Changes,
		Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

type refactoringResult struct {
	Code        string
	Explanation string
	Changes     []Change
}

func (m *Manager) applyRefactoring(req *RefactorRequest) refactoringResult {
	switch req.Type {
	case RefactorExtractFunc:
		return m.extractFunction(req)
	case RefactorSimplify:
		return m.simplifyCode(req)
	case RefactorDRY:
		return m.dryRefactor(req)
	case RefactorErrorHandle:
		return m.errorHandleRefactor(req)
	default:
		return refactoringResult{Code: req.Code, Explanation: fmt.Sprintf("不支持的重构类型: %s", req.Type)}
	}
}

func (m *Manager) extractFunction(req *RefactorRequest) refactoringResult {
	lines := strings.Split(req.Code, "\n")
	if len(lines) <= 3 {
		return refactoringResult{Code: req.Code, Explanation: "代码太短，无需提取函数"}
	}
	funcName := "extractedFunc"
	if req.Target != "" {
		funcName = req.Target
	}
	switch req.Language {
	case LangGo:
		return refactoringResult{
			Code:        fmt.Sprintf("func %s() {\n%s\n}\n\n%s()", funcName, strings.Join(lines, "\n"), funcName),
			Explanation: fmt.Sprintf("将代码块提取为函数 %s", funcName),
			Changes:     []Change{{Type: "extract", Description: fmt.Sprintf("提取为函数 %s", funcName)}},
		}
	case LangPython:
		indented := make([]string, 0, len(lines))
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				indented = append(indented, "    "+l)
			}
		}
		return refactoringResult{
			Code:        fmt.Sprintf("def %s():\n%s\n\n%s()", funcName, strings.Join(indented, "\n"), funcName),
			Explanation: fmt.Sprintf("将代码块提取为函数 %s", funcName),
			Changes:     []Change{{Type: "extract", Description: fmt.Sprintf("提取为函数 %s", funcName)}},
		}
	default:
		return refactoringResult{
			Code:        fmt.Sprintf("// 提取的函数: %s\n%s", funcName, req.Code),
			Explanation: fmt.Sprintf("建议将代码块提取为函数 %s", funcName),
			Changes:     []Change{{Type: "extract", Description: fmt.Sprintf("提取为函数 %s", funcName)}},
		}
	}
}

func (m *Manager) simplifyCode(req *RefactorRequest) refactoringResult {
	changes := make([]Change, 0)
	if strings.Count(req.Code, "if") > 1 {
		changes = append(changes, Change{Type: "simplify", Description: "建议使用 early return 减少嵌套层级"})
	}
	if req.Language == LangGo && strings.Contains(req.Code, "if err != nil") {
		changes = append(changes, Change{Type: "simplify", Description: "可以使用 errors.Is/As 进行更精确的错误匹配"})
	}
	if len(changes) == 0 {
		return refactoringResult{Code: req.Code, Explanation: "代码结构已经比较简单"}
	}
	return refactoringResult{Code: req.Code, Explanation: "简化建议", Changes: changes}
}

type duplicateInfo struct{ Line, Count int }

func (m *Manager) dryRefactor(req *RefactorRequest) refactoringResult {
	lines := strings.Split(req.Code, "\n")
	counts := make(map[string]int)
	lineNums := make(map[string]int)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			continue
		}
		if _, exists := lineNums[trimmed]; !exists {
			lineNums[trimmed] = i + 1
		}
		counts[trimmed]++
	}
	changes := make([]Change, 0)
	seen := make(map[string]bool)
	for line, count := range counts {
		if count >= 2 && !seen[line] {
			seen[line] = true
			changes = append(changes, Change{
				Type: "dry", Description: fmt.Sprintf("第 %d 行出现 %d 次重复", lineNums[line], count),
			})
		}
	}
	if len(changes) == 0 {
		return refactoringResult{Code: req.Code, Explanation: "未发现明显的重复代码"}
	}
	return refactoringResult{Code: req.Code, Explanation: "发现重复代码", Changes: changes}
}

func (m *Manager) errorHandleRefactor(req *RefactorRequest) refactoringResult {
	changes := make([]Change, 0)
	switch req.Language {
	case LangGo:
		if strings.Contains(req.Code, "_") && strings.Contains(req.Code, "err") {
			changes = append(changes, Change{Type: "error_handling", Description: "不应忽略错误"})
		}
		if strings.Contains(req.Code, "panic(") {
			changes = append(changes, Change{Type: "error_handling", Description: "避免使用 panic，建议返回 error"})
		}
	case LangPython:
		if strings.Contains(req.Code, "except:") && !strings.Contains(req.Code, "except Exception") {
			changes = append(changes, Change{Type: "error_handling", Description: "避免使用裸 except"})
		}
	case LangJava:
		if strings.Contains(req.Code, "catch (Exception") {
			changes = append(changes, Change{Type: "error_handling", Description: "建议捕获特定异常类型"})
		}
	}
	if len(changes) == 0 {
		return refactoringResult{Code: req.Code, Explanation: "错误处理看起来没有明显问题"}
	}
	return refactoringResult{Code: req.Code, Explanation: "错误处理改进建议", Changes: changes}
}

// GenerateTests 测试用例生成
func (m *Manager) GenerateTests(ctx context.Context, req *TestGenRequest) (*TestGenResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	if !IsValidLanguage(req.Language) {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}
	start := time.Now()
	framework := req.Framework
	if framework == "" {
		framework = m.defaultTestFramework(req.Language)
	}
	testCode, testCases := m.generateTestCode(req.Code, req.Language, framework)
	return &TestGenResponse{
		ID: generateID(), TestCode: testCode, Framework: framework, TestCases: testCases,
		Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

func (m *Manager) defaultTestFramework(lang ProgrammingLanguage) string {
	switch lang {
	case LangGo:
		return "testing"
	case LangPython:
		return "pytest"
	case LangJS:
		return "jest"
	case LangRust:
		return "cargo test"
	case LangJava:
		return "junit"
	}
	return "unknown"
}

func (m *Manager) generateTestCode(code string, lang ProgrammingLanguage, framework string) (string, []TestCase) {
	funcNames := extractFunctionNames(code, lang)
	if len(funcNames) == 0 {
		funcNames = []string{"Function"}
	}
	testCases := make([]TestCase, 0)
	var sb strings.Builder
	for _, name := range funcNames {
		testName := "Test" + strings.Title(name)
		errorTestName := testName + "_Error"
		sb.WriteString(fmt.Sprintf("// %s - %s\nfunc %s(t *testing.T) {\n\t// arrange, act, assert\n}\n\n", name, framework, testName))
		sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n\t// arrange error case, act, assert\n}\n\n", errorTestName))
		testCases = append(testCases,
			TestCase{Name: testName, Description: fmt.Sprintf("测试 %s", name)},
			TestCase{Name: errorTestName, Description: fmt.Sprintf("测试 %s 错误处理", name)},
		)
	}
	return sb.String(), testCases
}

func extractFunctionNames(code string, lang ProgrammingLanguage) []string {
	names := make([]string, 0)
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		switch lang {
		case LangGo:
			if strings.HasPrefix(trimmed, "func ") {
				parts := strings.SplitN(trimmed[5:], "(", 2)
				if n := strings.TrimSpace(parts[0]); n != "" {
					names = append(names, n)
				}
			}
		case LangPython:
			if strings.HasPrefix(trimmed, "def ") {
				parts := strings.SplitN(trimmed[4:], "(", 2)
				if n := strings.TrimSpace(parts[0]); n != "" {
					names = append(names, n)
				}
			}
		case LangJS:
			if strings.HasPrefix(trimmed, "function ") {
				parts := strings.SplitN(trimmed[9:], "(", 2)
				if n := strings.TrimSpace(parts[0]); n != "" {
					names = append(names, n)
				}
			}
		case LangRust:
			if idx := strings.Index(trimmed, "fn "); idx >= 0 {
				parts := strings.SplitN(trimmed[idx+3:], "(", 2)
				if n := strings.TrimSpace(parts[0]); n != "" {
					names = append(names, n)
				}
			}
		case LangJava:
			if (strings.Contains(trimmed, "public ") || strings.Contains(trimmed, "private ") || strings.Contains(trimmed, "protected ")) &&
				strings.Contains(trimmed, "(") && !strings.Contains(trimmed, "class ") {
				if parts := strings.Split(trimmed, "("); len(parts) > 0 {
					fields := strings.Fields(parts[0])
					if len(fields) > 0 {
						names = append(names, fields[len(fields)-1])
					}
				}
			}
		}
	}
	return names
}

// ExplainCode 代码解释
func (m *Manager) ExplainCode(ctx context.Context, req *ExplainRequest) (*ExplainResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	if !IsValidLanguage(req.Language) {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}
	start := time.Now()
	lines := strings.Split(req.Code, "\n")
	lineCount := len(lines)
	funcCount := len(extractFunctionNames(req.Code, req.Language))
	complexity := "low"
	if lineCount > 50 || funcCount > 5 {
		complexity = "medium"
	}
	if lineCount > 200 || funcCount > 15 {
		complexity = "high"
	}
	summary := fmt.Sprintf("共 %d 行代码，包含 %d 个函数", lineCount, funcCount)
	detail := req.Detail
	if detail == "" {
		detail = "normal"
	}
	explanation := fmt.Sprintf("这段 %s 代码 %s。", req.Language, summary)
	if detail == "detailed" {
		explanation += fmt.Sprintf(" 复杂度: %s。", complexity)
	}
	return &ExplainResponse{
		ID: generateID(), Explanation: explanation, Summary: summary, Complexity: complexity,
		Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

// AnalyzeGitDiff Git diff 分析
func (m *Manager) AnalyzeGitDiff(ctx context.Context, req *GitDiffRequest) (*GitDiffResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	start := time.Now()
	changes := make([]DiffChange, 0)
	risks := make([]string, 0)
	currentFile := ""
	added, deleted := 0, 0
	for _, line := range strings.Split(req.Diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				currentFile = strings.TrimPrefix(parts[3], "b/")
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}
	if currentFile != "" {
		changeType := "modified"
		if deleted > 0 && added == 0 {
			changeType = "deleted"
		} else if added > 0 && deleted == 0 {
			changeType = "added"
		}
		changes = append(changes, DiffChange{
			File: currentFile, Type: changeType,
			Summary: fmt.Sprintf("+%d -%d lines", added, deleted),
		})
	}
	if added+deleted > 500 {
		risks = append(risks, "大型变更，建议拆分为更小的提交")
	}
	return &GitDiffResponse{
		ID: generateID(), Summary: fmt.Sprintf("分析了 %d 个文件变更", len(changes)),
		Changes: changes, Risks: risks, Duration: time.Since(start), CreatedAt: time.Now(),
	}, nil
}

// GenerateCommitMessage 生成 commit message
func (m *Manager) GenerateCommitMessage(ctx context.Context, req *CommitMsgRequest) (*CommitMsgResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI code assistant is disabled")
	}
	added, deleted, files := 0, 0, 0
	for _, line := range strings.Split(req.Diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			files++
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}
	subject := fmt.Sprintf("update: %d file(s) changed, +%d -%d", files, added, deleted)
	if req.Style == "conventional" {
		subject = "chore: " + subject
	}
	return &CommitMsgResponse{
		ID: generateID(), Message: subject, Subject: subject,
		CreatedAt: time.Now(),
	}, nil
}
