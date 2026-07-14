package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/konono/aw/v4/internal/messaging"
)

// Server implements a minimal MCP server over stdin/stdout.
type Server struct {
	dbPath    string
	agentName string
	teamName  string
}

// NewServer creates a new MCP server.
func NewServer(dbPath, agentName, teamName string) *Server {
	return &Server{dbPath: dbPath, agentName: agentName, teamName: teamName}
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run reads JSON-RPC requests from stdin and writes responses to stdout.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
			}
			writeResponse(writer, resp)
			continue
		}

		resp := s.handle(req)
		writeResponse(writer, resp)
	}

	return scanner.Err()
}

func writeResponse(w *bufio.Writer, resp jsonrpcResponse) {
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}

func (s *Server) handle(req jsonrpcRequest) jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}
	}
}

func (s *Server) handleInitialize(req jsonrpcRequest) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "aw-msg",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req jsonrpcRequest) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": toolDefinitions(),
		},
	}
}

func (s *Server) handleToolsCall(req jsonrpcRequest) jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "invalid params")
	}

	store, err := messaging.OpenStore(s.dbPath)
	if err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			},
		}
	}
	defer func() { _ = store.Close() }()

	result, err := s.callTool(store, params.Name, params.Arguments)
	if err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			},
		}
	}

	text, _ := json.Marshal(result)
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": string(text)},
			},
		},
	}
}

func errorResponse(id json.RawMessage, code int, msg string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonrpcError{Code: code, Message: msg},
	}
}

// RunStdio is the entry point for `aw internal-mcp-msg`.
func RunStdio(dbPath, agentName, teamName string) error {
	srv := NewServer(dbPath, agentName, teamName)

	os.Stderr = os.NewFile(0, os.DevNull)

	return srv.Run()
}
