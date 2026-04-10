package connections

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
)

// ToolExecutor is a minimal interface representing a registered tool that can be
// invoked via MCP. The real project exposes a richer interface in internal/tools,
// but for the local MCP implementation we only require a name and an Execute method.
// This keeps MCP decoupled from external dependencies while matching the
// expectations described in the task.
type ToolExecutor interface {
	Name() string
	// Execute receives raw JSON parameters and returns a serializable result or an error.
	Execute(params json.RawMessage) (interface{}, error)
}

// MCPRequest represents a JSON-RPC 2.0 request.
type MCPRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Id      json.RawMessage `json:"id"`
}

// MCPResponse represents a JSON-RPC 2.0 response.
type MCPResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	Id      json.RawMessage `json:"id"`
}

// MCPError represents a JSON-RPC error object.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPNotificationHandler is a callback type used for simple event subscriptions.
type MCPNotificationHandler func(event string, payload interface{})

// MCPServer is a lightweight HTTP server that exposes a small JSON-RPC API
// for interacting with registered tools.
// It intentionally uses only the standard library to keep the runtime lightweight
// and dependency-free.
type MCPServer struct {
	mu             sync.RWMutex
	tools          map[string]ToolExecutor
	notify         []MCPNotificationHandler
	listener       net.Listener
	server         *http.Server
	port           int
	started        bool
	startCtx       context.Context
	startCtxCancel func()
}

// NewMCPServer creates a new MCPServer instance ready to register tools.
func NewMCPServer() *MCPServer {
	return &MCPServer{tools: make(map[string]ToolExecutor)}
}

// Start launches the MCP HTTP server on the given port. It accepts JSON-RPC
// requests and routes them to registered tools.
func (m *MCPServer) Start(ctx context.Context, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		// Already started
		return nil
	}
	m.port = port
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", m.handleRPC)
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("mcp: failed to listen on port %d: %w", port, err)
	}
	m.listener = ln
	m.server = &http.Server{Handler: mux}
	m.started = true
	// Save context so Stop can gracefully shut down.
	c, cancel := context.WithCancel(ctx)
	m.startCtx = c
	m.startCtxCancel = cancel

	go func() {
		// Run the HTTP server until the context is canceled.
		_ = m.server.Serve(ln)
	}()
	// Watch the provided context for cancellation to gracefully stop the server.
	go func() {
		<-ctx.Done()
		_ = m.Stop()
	}()
	return nil
}

// Stop gracefully stops the MCP server.
func (m *MCPServer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil
	}
	if m.server != nil {
		_ = m.server.Shutdown(context.Background())
	}
	if m.listener != nil {
		_ = m.listener.Close()
	}
	m.started = false
	if m.startCtxCancel != nil {
		m.startCtxCancel()
	}
	return nil
}

// RegisterTool registers a tool by name so it can be invoked via MCP.
func (m *MCPServer) RegisterTool(name string, handler ToolExecutor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[name] = handler
}

// ListTools returns the list of registered tool names.
func (m *MCPServer) ListTools() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.tools))
	for k := range m.tools {
		names = append(names, k)
	}
	return names
}

// RegisterNotification registers a notification handler for MCP events.
func (m *MCPServer) RegisterNotification(handler MCPNotificationHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notify = append(m.notify, handler)
}

// handleRPC is the HTTP handler for JSON-RPC requests on the MCP server.
func (m *MCPServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req MCPRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: nil, Error: &MCPError{Code: -32600, Message: "Invalid Request"}})
		return
	}

	switch req.Method {
	case "tools/list":
		res := m.ListTools()
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Result: res})
		return
	case "tools/call":
		// Params should contain {"name": "toolName", "params": ...}
		var payload struct {
			Name   string          `json:"name"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(req.Params, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Error: &MCPError{Code: -32602, Message: "Invalid params"}})
			return
		}
		m.mu.RLock()
		tool, ok := m.tools[payload.Name]
		m.mu.RUnlock()
		if !ok {
			_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Error: &MCPError{Code: -32602, Message: "Tool not found"}})
			return
		}
		result, err := tool.Execute(payload.Params)
		if err != nil {
			_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Error: &MCPError{Code: -32603, Message: err.Error()}})
			return
		}
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Result: result})
		return
	case "resources/list":
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Result: []string{}})
		return
	case "prompts/list":
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Result: []string{}})
		return
	default:
		_ = json.NewEncoder(w).Encode(MCPResponse{Jsonrpc: "2.0", Id: req.Id, Error: &MCPError{Code: -32601, Message: "Method not found"}})
		return
	}
}

// internal helper to broadcast notifications to all subscribers
func (m *MCPServer) notifyAll(event string, payload interface{}) {
	m.mu.RLock()
	handlers := append([]MCPNotificationHandler{}, m.notify...)
	m.mu.RUnlock()
	for _, h := range handlers {
		go h(event, payload)
	}
}
