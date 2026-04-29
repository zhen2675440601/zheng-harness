package plugin

import "encoding/json"

const jsonRPCVersion = "2.0"

const (
	methodInitialize = "initialize"
	methodToolInfo   = "tool.info"
	methodToolExecute = "tool.execute"
	methodShutdown   = "shutdown"
)

// JSONRPCRequest 表示一条 JSON-RPC 2.0 请求消息。
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse 表示一条 JSON-RPC 2.0 响应消息。
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError 表示 JSON-RPC 2.0 错误对象。
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
