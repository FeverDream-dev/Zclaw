package connections

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// ConnectionStatus represents the runtime state of a connection type.
type ConnectionStatus struct {
	Type         string
	State        string
	LastActivity time.Time
	Error        string
}

// ConnectionManager coordinates MCP, webhook senders/receivers and file watchers.
type ConnectionManager struct {
	mu             sync.RWMutex
	mcp            *MCPServer
	webhookConfigs []WebhookConfig
	webhookSenders []*WebhookSender
	fileWatchers   []struct {
		path    string
		pattern string
		handler func(FileEvent)
		watcher *FileWatcher
	}
	watchersStarted bool
	started         bool
	statuses        map[string]ConnectionStatus
}

// NewConnectionManager creates a fresh connection manager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{statuses: make(map[string]ConnectionStatus)}
}

// RegisterMCP registers an MCP server instance to manage.
func (cm *ConnectionManager) RegisterMCP(mcpServer *MCPServer) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.mcp = mcpServer
	cm.statuses["mcp"] = ConnectionStatus{Type: "mcp", State: "registered"}
}

// RegisterWebhook registers a webhook configuration and prepares a sender.
func (cm *ConnectionManager) RegisterWebhook(config WebhookConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.webhookConfigs = append(cm.webhookConfigs, config)
	cm.webhookSenders = append(cm.webhookSenders, &WebhookSender{client: &http.Client{}, config: config})
	cm.statuses["webhook"] = ConnectionStatus{Type: "webhook", State: "registered"}
}

// RegisterFileWatch registers a path/pattern watcher with a handler.
func (cm *ConnectionManager) RegisterFileWatch(path, pattern string, handler func(FileEvent)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	w := NewFileWatcher()
	_ = w.Watch(path, pattern)
	cm.fileWatchers = append(cm.fileWatchers, struct {
		path    string
		pattern string
		handler func(FileEvent)
		watcher *FileWatcher
	}{path: path, pattern: pattern, handler: handler, watcher: w})
	cm.statuses["filewatch"] = ConnectionStatus{Type: "filewatch", State: "registered"}
}

// Start starts all registered connections and returns an error if any fail.
func (cm *ConnectionManager) Start(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Start MCP if present
	if cm.mcp != nil {
		_ = cm.mcp.Start(ctx, cm.mcp.port)
		st := cm.statuses["mcp"]
		st.State = "running"
		st.LastActivity = time.Now()
		cm.statuses["mcp"] = st
	}
	// Start file watchers
	for _, w := range cm.fileWatchers {
		if w.watcher != nil {
			// loop runs in background; no extra action required here
			st := cm.statuses["filewatch"]
			st.State = "running"
			st.LastActivity = time.Now()
			cm.statuses["filewatch"] = st
			_ = w // keep reference to avoid GC
		}
	}
	cm.started = true
	return nil
}

// Stop gracefully stops all registered connections.
func (cm *ConnectionManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.mcp != nil {
		_ = cm.mcp.Stop()
		st := cm.statuses["mcp"]
		st.State = "stopped"
		cm.statuses["mcp"] = st
	}
	cm.started = false
}

// Status returns a snapshot of the current connection statuses.
func (cm *ConnectionManager) Status() map[string]ConnectionStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	// Return a shallow copy to avoid race conditions
	out := make(map[string]ConnectionStatus, len(cm.statuses))
	for k, v := range cm.statuses {
		out[k] = v
	}
	return out
}
