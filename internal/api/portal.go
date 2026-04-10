package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zclaw/zclaw/internal/agents"
	"github.com/zclaw/zclaw/internal/providers"
	"github.com/zclaw/zclaw/internal/telemetry"
)

// Server startup time for uptime reporting.
var serverStartTime = time.Now().UTC()

// DashboardStats describes high level fleet metrics.
type DashboardStats struct {
	AgentsTotal       int                       `json:"agents_total"`
	AgentsActive      int                       `json:"agents_active"`
	AgentsPaused      int                       `json:"agents_paused"`
	TasksPending      int                       `json:"tasks_pending"`
	TasksRunning      int                       `json:"tasks_running"`
	TasksCompleted24h int                       `json:"tasks_completed_24h"`
	TasksFailed24h    int                       `json:"tasks_failed_24h"`
	WorkersActive     int                       `json:"workers_active"`
	BrowserSessions   int                       `json:"browser_sessions"`
	QueueDepth        int                       `json:"queue_depth"`
	ProviderHealth    map[string]ProviderHealth `json:"provider_health"`
	Uptime            string                    `json:"uptime"`
}

// ProviderHealth contains health metrics for a given provider.
type ProviderHealth struct {
	ProviderID  string        `json:"provider_id"`
	Status      string        `json:"status"` // healthy/degraded/down
	LastRequest time.Time     `json:"last_request"`
	ErrorRate   float64       `json:"error_rate"`
	AvgLatency  time.Duration `json:"avg_latency"`
}

// TaskSummary provides a compact representation of a task.
type TaskSummary struct {
	ID          string     `json:"id"`
	State       string     `json:"state"`
	Input       string     `json:"input"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// AgentDetail enriches an Agent with recent activity for dashboards.
type AgentDetail struct {
	Agent         agents.Agent  `json:"agent"`
	ActiveTask    *TaskSummary  `json:"active_task,omitempty"`
	RecentTasks   []TaskSummary `json:"recent_tasks"`
	WorkspaceSize int64         `json:"workspace_size"`
}

// FleetOverview aggregates dashboard data for a fleet view.
type FleetOverview struct {
	DashboardStats DashboardStats         `json:"stats"`
	TopAgents      []AgentDetail          `json:"top_agents"`
	RecentErrors   []telemetry.AuditEntry `json:"recent_errors"`
}

// PortalHandler provides HTTP handlers for the fleet dashboard API.
type PortalHandler struct {
	agentRepo  AgentRepository
	scheduler  SchedulerInterface
	audit      telemetry.AuditLogger
	browserMgr BrowserSessionManager
	provReg    ProvidersRegistryInterface
	// metrics and health can be added later if needed
}

// AgentRepository abstracts the agent store access used by the portal.
type AgentRepository interface {
	List(ctx context.Context, filter agents.AgentFilter) (*agents.AgentList, error)
	Get(ctx context.Context, id agents.AgentID) (*agents.Agent, error)
	Count(ctx context.Context, state agents.AgentState) (int, error)
	TransitionState(ctx context.Context, id agents.AgentID, newState agents.AgentState) (*agents.Agent, error)
}

// SchedulerInterface surfaces scheduler telemetry used by the portal.
type SchedulerInterface interface {
	QueueDepth() int
	ActiveWorkers() int
}

// BrowserSessionManager abstracts browser session counts.
type BrowserSessionManager interface {
	ActiveCount() int
}

// ProvidersRegistryInterface abstracts provider registry for health checks.
type ProvidersRegistryInterface interface {
	List(ctx context.Context) []providers.ProviderID
}

// NewPortalHandler constructs a new PortalHandler with required dependencies.
func NewPortalHandler(agentRepo AgentRepository, sched SchedulerInterface, audit telemetry.AuditLogger, browserMgr BrowserSessionManager, provReg ProvidersRegistryInterface) *PortalHandler {
	return &PortalHandler{
		agentRepo:  agentRepo,
		scheduler:  sched,
		audit:      audit,
		browserMgr: browserMgr,
		provReg:    provReg,
	}
}

// HandleDashboard returns the fleet overview in a single payload.
func (h *PortalHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Stats
	total, _ := h.agentRepo.Count(ctx, "")
	active, _ := h.agentRepo.Count(ctx, agents.StateActive)
	paused, _ := h.agentRepo.Count(ctx, agents.StatePaused)
	// Minimal placeholder: pending/running/completed/failed are derived elsewhere; keep zero for now
	stats := DashboardStats{
		AgentsTotal:       total,
		AgentsActive:      active,
		AgentsPaused:      paused,
		TasksPending:      0,
		TasksRunning:      0,
		TasksCompleted24h: 0,
		TasksFailed24h:    0,
		WorkersActive:     h.scheduler.ActiveWorkers(),
		BrowserSessions:   h.browserMgr.ActiveCount(),
		QueueDepth:        h.scheduler.QueueDepth(),
		ProviderHealth:    map[string]ProviderHealth{},
		Uptime:            time.Since(serverStartTime).Round(time.Second).String(),
	}

	// Providers health (best-effort placeholder)
	for _, pid := range h.provReg.List(ctx) {
		ph := ProviderHealth{ProviderID: string(pid), Status: "healthy", LastRequest: time.Now().UTC(), ErrorRate: 0, AvgLatency: 0}
		stats.ProviderHealth[string(pid)] = ph
	}

	// Top agents (simple listing with workspace size estimation)
	top := []AgentDetail{}
	if lst, err := h.agentRepo.List(ctx, agents.AgentFilter{Limit: 5, OrderBy: "created_at"}); err == nil {
		for _, a := range lst.Agents {
			// Estimate workspace size by directory file size (if accessible)
			size := int64(0)
			if a.WorkspacePath != "" {
				if fi, err := os.Stat(a.WorkspacePath); err == nil {
					if fi.IsDir() {
						_ = filepath.Walk(a.WorkspacePath, func(_ string, info os.FileInfo, err error) error {
							if err != nil {
								return nil
							}
							if !info.IsDir() {
								size += info.Size()
							}
							return nil
						})
					}
				}
			}
			top = append(top, AgentDetail{Agent: a, ActiveTask: nil, RecentTasks: []TaskSummary{}, WorkspaceSize: size})
		}
	}

	// Recent errors from audit log
	recErrs, _ := h.audit.Query(ctx, telemetry.AuditFilter{Limit: 100})

	resp := FleetOverview{DashboardStats: stats, TopAgents: top, RecentErrors: recErrs}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleDashboardStats returns only the stats payload.
func (h *PortalHandler) HandleDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	total, _ := h.agentRepo.Count(ctx, "")
	active, _ := h.agentRepo.Count(ctx, agents.StateActive)
	paused, _ := h.agentRepo.Count(ctx, agents.StatePaused)
	stats := DashboardStats{AgentsTotal: total, AgentsActive: active, AgentsPaused: paused, QueueDepth: h.scheduler.QueueDepth(), WorkersActive: h.scheduler.ActiveWorkers(), BrowserSessions: h.browserMgr.ActiveCount(), Uptime: time.Since(serverStartTime).Round(time.Second).String()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleAgentDetail returns detailed view for a single agent.
func (h *PortalHandler) HandleAgentDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Path: /api/v1/dashboard/agents/{id}/detail
	id := extractPathParam(r.URL.Path, "/api/v1/dashboard/agents/", "/detail")
	if id == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	a, err := h.agentRepo.Get(ctx, agents.AgentID(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Workspace size estimation similar to HandleDashboard
	size := int64(0)
	if a.WorkspacePath != "" {
		if fi, err := os.Stat(a.WorkspacePath); err == nil && fi.IsDir() {
			_ = filepath.Walk(a.WorkspacePath, func(_ string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					size += info.Size()
				}
				return nil
			})
		}
	}
	detail := AgentDetail{Agent: *a, ActiveTask: nil, RecentTasks: []TaskSummary{}, WorkspaceSize: size}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

// HandleProvidersHealth returns health snippets for all registered providers.
func (h *PortalHandler) HandleProvidersHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := map[string]ProviderHealth{}
	for _, pid := range h.provReg.List(ctx) {
		resp[string(pid)] = ProviderHealth{ProviderID: string(pid), Status: "healthy", LastRequest: time.Now().UTC(), ErrorRate: 0, AvgLatency: 0}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleWorkers returns the current worker pool status.
func (h *PortalHandler) HandleWorkers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"workers_active": h.scheduler.ActiveWorkers()})
}

// HandleQueue returns queue depth information.
func (h *PortalHandler) HandleQueue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"queue_depth": h.scheduler.QueueDepth()})
}

// HandleActivity returns recent audit activity.
func (h *PortalHandler) HandleActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entries, err := h.audit.Query(ctx, telemetry.AuditFilter{Limit: 100})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleErrors returns recent error events from audit logs.
func (h *PortalHandler) HandleErrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entries, err := h.audit.Query(ctx, telemetry.AuditFilter{Limit: 100})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Simple filter on action containing 'error' or 'failed'
	var errs []telemetry.AuditEntry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Action), "error") || strings.Contains(strings.ToLower(e.Action), "fail") {
			errs = append(errs, e)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(errs)
}

// RestartAgent transitions an agent to Active state.
func (h *PortalHandler) RestartAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := extractPathParam(r.URL.Path, "/api/v1/dashboard/agents/", "/restart")
	if id == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	a, err := h.agentRepo.TransitionState(ctx, agents.AgentID(id), agents.StateActive)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// KillAgentTask cancels the current task for an agent (best-effort).
func (h *PortalHandler) KillAgentTask(w http.ResponseWriter, r *http.Request) {
	// Since we don't have a task repository wired in here, respond with 202 Accepted.
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"status":"cancel_requested"}`)
}

// HandleAgentRouter dispatches agent-related subroutes under /api/v1/dashboard/agents/{id}/...
func (h *PortalHandler) HandleAgentRouter(w http.ResponseWriter, r *http.Request) {
	// Expected patterns:
	// GET  /api/v1/dashboard/agents/{id}/detail
	// POST /api/v1/dashboard/agents/{id}/restart
	// POST /api/v1/dashboard/agents/{id}/kill-task
	// We'll route based on the path after the base.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/dashboard/agents/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	if len(parts) >= 2 {
		switch parts[1] {
		case "detail":
			if r.Method == http.MethodGet {
				h.HandleAgentDetail(w, r)
				return
			}
		case "restart":
			if r.Method == http.MethodPost {
				h.RestartAgent(w, r)
				return
			}
		case "kill-task":
			if r.Method == http.MethodPost {
				h.KillAgentTask(w, r)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// ExportAgents returns all agents as a JSON array.
func (h *PortalHandler) HandleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Paginate to avoid huge allocations; start with a reasonable limit and loop if needed.
	var all []agents.Agent
	limit := 100
	offset := 0
	for {
		lst, err := h.agentRepo.List(ctx, agents.AgentFilter{Limit: limit, Offset: offset, OrderBy: "id"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if lst == nil || len(lst.Agents) == 0 {
			break
		}
		all = append(all, lst.Agents...)
		if len(lst.Agents) < limit {
			break
		}
		offset += limit
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(all)
}

// extractPathParam is a tiny helper to pull the first path segment after base, given a trailing suffix.
func extractPathParam(fullPath string, base string, suffix string) string {
	p := strings.TrimPrefix(fullPath, base)
	if suffix != "" {
		p = strings.TrimSuffix(p, suffix)
	}
	// Now p should be either "<id>" or "<id>/detail" etc. Normalize.
	parts := strings.Split(p, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
