// Package mcpagent 提供增强的MCP协议合规支持。
// 实现MCP tool list/get/call协议方法，支持tool input schema验证。
package mcpagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// MCPProtocol MCP协议处理器.
type MCPProtocol struct {
	agent  *MCPAgent
	logger *slog.Logger
}

// NewMCPProtocol 创建MCP协议处理器.
func NewMCPProtocol(agent *MCPAgent) *MCPProtocol {
	return &MCPProtocol{
		agent:  agent,
		logger: agent.logger,
	}
}

// MCPRequest MCP请求.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse MCP响应.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError MCP错误.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP标准错误码.
const (
	MCPErrorParse          = -32700
	MCPErrorInvalidRequest = -32600
	MCPErrorMethodNotFound = -32601
	MCPErrorInvalidParams  = -32602
	MCPErrorInternal       = -32603
	MCPErrorToolNotFound   = -32000
	MCPErrorToolExecFailed = -32001
	MCPErrorSchemaInvalid  = -32002
)

// ToolsListParams tools/list请求参数.
type ToolsListParams struct {
	Cursor string `json:"cursor,omitempty"` // 分页游标
}

// ToolsListResult tools/list响应结果.
type ToolsListResult struct {
	Tools      []MCPToolInfo `json:"tools"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// MCPToolInfo MCP工具信息.
type MCPToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsGetParams tools/get请求参数.
type ToolsGetParams struct {
	Name string `json:"name"`
}

// ToolsGetResult tools/get响应结果.
type ToolsGetResult struct {
	Tool MCPToolInfo `json:"tool"`
}

// ToolsCallParams tools/call请求参数.
type ToolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolsCallResult tools/call响应结果.
type ToolsCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent MCP内容块.
type MCPContent struct {
	Type string `json:"type"` // text, image, resource
	Text string `json:"text,omitempty"`
}

// HandleRequest 处理MCP请求.
func (p *MCPProtocol) HandleRequest(ctx context.Context, rawRequest []byte) ([]byte, error) {
	var request MCPRequest
	if err := json.Unmarshal(rawRequest, &request); err != nil {
		return p.createErrorResponse(nil, MCPErrorParse, "Parse error", nil)
	}

	// 验证JSONRPC版本
	if request.JSONRPC != "2.0" {
		return p.createErrorResponse(request.ID, MCPErrorInvalidRequest, "Invalid JSON-RPC version", nil)
	}

	// 路由到对应的处理方法
	switch request.Method {
	case "tools/list":
		return p.handleToolsList(ctx, request.ID, request.Params)
	case "tools/get":
		return p.handleToolsGet(ctx, request.ID, request.Params)
	case "tools/call":
		return p.handleToolsCall(ctx, request.ID, request.Params)
	default:
		return p.createErrorResponse(request.ID, MCPErrorMethodNotFound,
			fmt.Sprintf("Method not found: %s", request.Method), nil)
	}
}

// handleToolsList 处理tools/list请求.
func (p *MCPProtocol) handleToolsList(ctx context.Context, id interface{}, params json.RawMessage) ([]byte, error) {
	var listParams ToolsListParams
	if params != nil {
		if err := json.Unmarshal(params, &listParams); err != nil {
			return p.createErrorResponse(id, MCPErrorInvalidParams, "Invalid params", err.Error())
		}
	}

	// 获取所有工具
	tools := p.agent.GetTools()

	// 转换为MCP工具信息格式
	mcpTools := make([]MCPToolInfo, 0, len(tools))
	for _, tool := range tools {
		if !tool.Enabled {
			continue
		}

		schemaJSON, err := json.Marshal(tool.InputSchema)
		if err != nil {
			p.logger.Warn("Failed to marshal tool schema", "tool", tool.Name, "error", err)
			schemaJSON = []byte(`{"type":"object","properties":{}}`)
		}

		mcpTools = append(mcpTools, MCPToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaJSON,
		})
	}

	result := ToolsListResult{
		Tools: mcpTools,
	}

	return p.createSuccessResponse(id, result)
}

// handleToolsGet 处理tools/get请求.
func (p *MCPProtocol) handleToolsGet(ctx context.Context, id interface{}, params json.RawMessage) ([]byte, error) {
	var getParams ToolsGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return p.createErrorResponse(id, MCPErrorInvalidParams, "Invalid params", err.Error())
	}

	if getParams.Name == "" {
		return p.createErrorResponse(id, MCPErrorInvalidParams, "Tool name is required", nil)
	}

	// 查找工具
	tool, exists := p.agent.tools[getParams.Name]
	if !exists {
		return p.createErrorResponse(id, MCPErrorToolNotFound,
			fmt.Sprintf("Tool not found: %s", getParams.Name), nil)
	}

	schemaJSON, err := json.Marshal(tool.InputSchema)
	if err != nil {
		schemaJSON = []byte(`{"type":"object","properties":{}}`)
	}

	result := ToolsGetResult{
		Tool: MCPToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaJSON,
		},
	}

	return p.createSuccessResponse(id, result)
}

// handleToolsCall 处理tools/call请求.
func (p *MCPProtocol) handleToolsCall(ctx context.Context, id interface{}, params json.RawMessage) ([]byte, error) {
	var callParams ToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return p.createErrorResponse(id, MCPErrorInvalidParams, "Invalid params", err.Error())
	}

	if callParams.Name == "" {
		return p.createErrorResponse(id, MCPErrorInvalidParams, "Tool name is required", nil)
	}

	// 查找工具
	tool, exists := p.agent.tools[callParams.Name]
	if !exists {
		return p.createErrorResponse(id, MCPErrorToolNotFound,
			fmt.Sprintf("Tool not found: %s", callParams.Name), nil)
	}

	// 验证input schema
	if err := p.validateInputSchema(tool, callParams.Arguments); err != nil {
		return p.createErrorResponse(id, MCPErrorSchemaInvalid,
			"Input validation failed", err.Error())
	}

	// 执行工具
	start := time.Now()
	result, err := tool.Handler(ctx, callParams.Arguments)
	duration := time.Since(start)

	if err != nil {
		p.logger.Error("Tool execution failed",
			"tool", callParams.Name,
			"duration", duration,
			"error", err)

		return p.createSuccessResponse(id, ToolsCallResult{
			Content: []MCPContent{
				{Type: "text", Text: fmt.Sprintf("Tool execution failed: %v", err)},
			},
			IsError: true,
		})
	}

	// 转换结果为MCP格式
	contents := p.convertToolResult(result)

	p.logger.Info("Tool executed successfully",
		"tool", callParams.Name,
		"duration", duration)

	return p.createSuccessResponse(id, ToolsCallResult{
		Content: contents,
	})
}

// validateInputSchema 验证输入参数是否符合schema.
func (p *MCPProtocol) validateInputSchema(tool *NASTool, args map[string]interface{}) error {
	if tool.InputSchema == nil {
		return nil
	}

	// 提取required字段
	required, ok := tool.InputSchema["required"].([]interface{})
	if !ok {
		return nil
	}

	// 检查必需字段
	for _, field := range required {
		fieldName, ok := field.(string)
		if !ok {
			continue
		}

		if _, exists := args[fieldName]; !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}
	}

	// 验证字段类型
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	for key, value := range args {
		propSchema, exists := properties[key]
		if !exists {
			// 允许额外属性
			continue
		}

		propMap, ok := propSchema.(map[string]interface{})
		if !ok {
			continue
		}

		expectedType, _ := propMap["type"].(string)
		if expectedType == "" {
			continue
		}

		// 类型检查
		if !p.checkType(value, expectedType) {
			return fmt.Errorf("field '%s' expected type '%s', got %T", key, expectedType, value)
		}

		// 枚举检查
		if enum, ok := propMap["enum"].([]interface{}); ok {
			found := false
			for _, allowed := range enum {
				if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", allowed) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field '%s' value '%v' not in allowed values: %v", key, value, enum)
			}
		}
	}

	return nil
}

// checkType 检查值是否匹配预期类型.
func (p *MCPProtocol) checkType(value interface{}, expectedType string) bool {
	if value == nil {
		return true // null值通常被允许
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float32, float64:
			// JSON数字通常是float64
			return true
		default:
			return false
		}
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true
	}
}

// convertToolResult 转换工具结果为MCP内容格式.
func (p *MCPProtocol) convertToolResult(result *ToolResult) []MCPContent {
	var contents []MCPContent

	if result == nil {
		return []MCPContent{
			{Type: "text", Text: "No result"},
		}
	}

	// 添加消息
	if result.Message != "" {
		contents = append(contents, MCPContent{
			Type: "text",
			Text: result.Message,
		})
	}

	// 添加可视化数据
	if result.Visual != nil {
		visualText := fmt.Sprintf("[%s] %s\n%s", result.Visual.Type, result.Visual.Title, result.Visual.Data)
		contents = append(contents, MCPContent{
			Type: "text",
			Text: visualText,
		})
	}

	// 添加数据
	if len(result.Data) > 0 {
		dataJSON, err := json.MarshalIndent(result.Data, "", "  ")
		if err == nil {
			contents = append(contents, MCPContent{
				Type: "text",
				Text: string(dataJSON),
			})
		}
	}

	// 添加建议
	if len(result.Suggestions) > 0 {
		suggestionsText := "Suggestions:\n" + strings.Join(result.Suggestions, "\n")
		contents = append(contents, MCPContent{
			Type: "text",
			Text: suggestionsText,
		})
	}

	if len(contents) == 0 {
		contents = append(contents, MCPContent{
			Type: "text",
			Text: "Tool executed successfully",
		})
	}

	return contents
}

// createSuccessResponse 创建成功响应.
func (p *MCPProtocol) createSuccessResponse(id interface{}, result interface{}) ([]byte, error) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(response)
}

// createErrorResponse 创建错误响应.
func (p *MCPProtocol) createErrorResponse(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(response)
}

// RegisterMCPTool 注册MCP工具 (带schema验证).
func (a *MCPAgent) RegisterMCPTool(name, description string, category ToolCategory,
	inputSchema map[string]interface{}, handler ToolHandler, permission PermissionLevel) error {

	// 验证schema格式
	if err := validateToolSchema(inputSchema); err != nil {
		return fmt.Errorf("invalid tool schema: %w", err)
	}

	tool := &NASTool{
		Name:        name,
		Description: description,
		Category:    category,
		InputSchema: inputSchema,
		Handler:     handler,
		Enabled:     true,
		Permission:  permission,
	}

	a.RegisterTool(tool)
	return nil
}

// validateToolSchema 验证工具schema格式.
func validateToolSchema(schema map[string]interface{}) error {
	if schema == nil {
		return nil
	}

	// 检查type字段
	schemaType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("schema missing 'type' field")
	}

	if schemaType != "object" {
		return fmt.Errorf("schema type must be 'object', got '%s'", schemaType)
	}

	// 检查properties字段
	properties, ok := schema["properties"]
	if !ok {
		return nil // properties是可选的
	}

	// 验证properties是map类型
	if _, ok := properties.(map[string]interface{}); !ok {
		return fmt.Errorf("'properties' must be an object")
	}

	// 检查required字段
	if required, ok := schema["required"]; ok {
		reqSlice, ok := required.([]interface{})
		if !ok {
			return fmt.Errorf("'required' must be an array")
		}

		// 验证required中的字段都在properties中定义
		propsMap := properties.(map[string]interface{})
		for _, field := range reqSlice {
			fieldName, ok := field.(string)
			if !ok {
				return fmt.Errorf("'required' array must contain strings")
			}
			if _, exists := propsMap[fieldName]; !exists {
				return fmt.Errorf("required field '%s' not defined in properties", fieldName)
			}
		}
	}

	return nil
}

// GetMCPProtocol 获取MCP协议处理器.
func (a *MCPAgent) GetMCPProtocol() *MCPProtocol {
	return NewMCPProtocol(a)
}
