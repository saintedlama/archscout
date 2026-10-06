package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/saintedlama/archscout/codegraph/sqlite"
)

// Server implements a Model Context Protocol (MCP) JSON-RPC 2.0 server over stdio.
type Server struct {
	store *sqlite.Store
}

// NewServer creates a new MCP Server backed by an ArchScout SQLite store.
func NewServer(store *sqlite.Store) *Server {
	return &Server{store: store}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type schemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callToolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError"`
}

// Serve reads JSON-RPC requests from r and writes responses to w until EOF or context cancellation.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// Allow large requests if necessary
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			errResp := response{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "Parse error: " + err.Error()},
			}
			_ = sendResponse(w, errResp)
			continue
		}

		// Handle notifications (no ID)
		if req.ID == nil {
			continue
		}

		resp := s.handleRequest(ctx, req)
		if err := sendResponse(w, resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}

	return scanner.Err()
}

func sendResponse(w io.Writer, resp response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func (s *Server) handleRequest(ctx context.Context, req request) response {
	switch req.Method {
	case "initialize":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "archscout",
					"version": "0.1.0",
				},
			},
		}

	case "ping":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": getTools(),
			},
		}

	case "tools/call":
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "Invalid params: " + err.Error()},
			}
		}

		res := s.callTool(ctx, callParams.Name, callParams.Arguments)
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  res,
		}

	default:
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("Method %q not found", req.Method)},
		}
	}
}

func getTools() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "search_symbols",
			Description: "Full-text search across symbols, functions, types, and packages in the codebase graph.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"query": {Type: "string", Description: "Symbol name, keyword, or prefix to search for"},
					"limit": {Type: "integer", Description: "Maximum number of results to return (default 10)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "trace_callers",
			Description: "Trace incoming callers to a function or method up to a given depth using recursive graph traversal in SQL.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"symbol": {Type: "string", Description: "Fully-qualified name or node ID of the function/method (e.g. 'example.com/pkg.ProcessOrder')"},
					"depth":  {Type: "integer", Description: "Maximum search depth (default 3)"},
				},
				Required: []string{"symbol"},
			},
		},
		{
			Name:        "trace_dependencies",
			Description: "Trace outgoing calls, imports, and package dependencies from a symbol or package up to a given depth.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"symbol": {Type: "string", Description: "Node ID or package path of the starting entity"},
					"depth":  {Type: "integer", Description: "Maximum search depth (default 3)"},
				},
				Required: []string{"symbol"},
			},
		},
		{
			Name:        "get_symbol_info",
			Description: "Retrieve detailed metadata for a symbol (kind, name, QName, package, source file, line, and column).",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"symbol": {Type: "string", Description: "Exact node ID or qualified name of the symbol"},
				},
				Required: []string{"symbol"},
			},
		},
		{
			Name:        "get_implementers",
			Description: "Retrieve concrete types that implement a specified Go interface.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"interface": {Type: "string", Description: "Node ID or fully-qualified name of the interface"},
				},
				Required: []string{"interface"},
			},
		},
	}
}

func (s *Server) callTool(ctx context.Context, name string, rawArgs json.RawMessage) callToolResult {
	switch name {
	case "search_symbols":
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return makeErrorResult("invalid arguments: " + err.Error())
		}
		if args.Limit <= 0 {
			args.Limit = 10
		}
		nodes, err := s.store.SearchFTS(ctx, args.Query, args.Limit)
		if err != nil {
			return makeErrorResult("search failed: " + err.Error())
		}
		return makeJSONResult(nodes)

	case "trace_callers":
		var args struct {
			Symbol string `json:"symbol"`
			Depth  int    `json:"depth"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return makeErrorResult("invalid arguments: " + err.Error())
		}
		if args.Depth <= 0 {
			args.Depth = 3
		}
		callers, err := s.store.TraceCallers(ctx, args.Symbol, args.Depth)
		if err != nil {
			return makeErrorResult("trace callers failed: " + err.Error())
		}
		return makeJSONResult(map[string]any{
			"target":  args.Symbol,
			"depth":   args.Depth,
			"callers": callers,
		})

	case "trace_dependencies":
		var args struct {
			Symbol string `json:"symbol"`
			Depth  int    `json:"depth"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return makeErrorResult("invalid arguments: " + err.Error())
		}
		if args.Depth <= 0 {
			args.Depth = 3
		}
		deps, err := s.store.TraceDependencies(ctx, args.Symbol, args.Depth)
		if err != nil {
			return makeErrorResult("trace dependencies failed: " + err.Error())
		}
		return makeJSONResult(map[string]any{
			"source":       args.Symbol,
			"depth":        args.Depth,
			"dependencies": deps,
		})

	case "get_symbol_info":
		var args struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return makeErrorResult("invalid arguments: " + err.Error())
		}
		node, err := s.store.GetNode(ctx, args.Symbol)
		if err != nil {
			return makeErrorResult("get symbol failed: " + err.Error())
		}
		if node == nil {
			// Try FTS fallback if not found by exact ID
			candidates, _ := s.store.SearchFTS(ctx, args.Symbol, 1)
			if len(candidates) > 0 {
				node = &candidates[0]
			} else {
				return makeErrorResult(fmt.Sprintf("symbol %q not found", args.Symbol))
			}
		}
		return makeJSONResult(node)

	case "get_implementers":
		var args struct {
			Interface string `json:"interface"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return makeErrorResult("invalid arguments: " + err.Error())
		}
		implementers, err := s.store.GetImplementers(ctx, args.Interface)
		if err != nil {
			return makeErrorResult("get implementers failed: " + err.Error())
		}
		return makeJSONResult(map[string]any{
			"interface":    args.Interface,
			"implementers": implementers,
		})

	default:
		return makeErrorResult(fmt.Sprintf("unknown tool: %q", name))
	}
}

func makeJSONResult(v any) callToolResult {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return makeErrorResult("formatting result: " + err.Error())
	}
	return callToolResult{
		Content: []textContent{
			{
				Type: "text",
				Text: string(bytes),
			},
		},
		IsError: false,
	}
}

func makeErrorResult(msg string) callToolResult {
	return callToolResult{
		Content: []textContent{
			{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}
