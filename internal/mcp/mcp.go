package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/gatra-io/gatra/internal/errors"
)

// Inspector parses and unwraps MCP JSON-RPC 2.0 tool execution payloads.
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  RequestParams `json:"params"`
	ID      interface{}   `json:"id"`
}

type RequestParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// IsMCPPayload returns true if the incoming JSON body conforms to an MCP tool execution request.
func IsMCPPayload(body []byte) bool {
	jsonrpc := gjson.GetBytes(body, "jsonrpc").String()
	method := gjson.GetBytes(body, "method").String()
	return jsonrpc == "2.0" && method == "tools/call"
}

// UnpackMCP extracts the target tool name and inner arguments JSON object from an MCP request.
func UnpackMCP(body []byte) (toolName string, argsJSON []byte, err error) {
	if !IsMCPPayload(body) {
		return "", nil, fmt.Errorf("%w: body is not a valid MCP tools/call request", errors.ErrInvalidConfiguration)
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return "", nil, fmt.Errorf("%w: failed to parse MCP JSON-RPC payload: %v", errors.ErrInvalidConfiguration, err)
	}

	if req.Params.Name == "" {
		return "", nil, fmt.Errorf("%w: MCP tool request missing params.name", errors.ErrInvalidConfiguration)
	}

	if len(req.Params.Arguments) == 0 {
		return req.Params.Name, []byte("{}"), nil
	}

	return req.Params.Name, req.Params.Arguments, nil
}