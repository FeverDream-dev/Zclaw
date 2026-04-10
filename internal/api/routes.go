package api

import (
	"context"
	"net/http"

	"github.com/zclaw/zclaw/internal/agents"
	"github.com/zclaw/zclaw/internal/providers"
	"github.com/zclaw/zclaw/internal/telemetry"
)

// PortalDeps groups dependencies required by the portal routes.
type PortalDeps struct {
	AgentRepo  AgentRepository
	Scheduler  SchedulerInterface
	Audit      telemetry.AuditLogger
	BrowserMgr BrowserSessionManager
	ProvReg    ProvidersRegistryInterface
}

// Interfaces to adapt concrete implementations to the portal.
type PortalHandlerAgentRepo interface {
	List(ctx context.Context, filter agents.AgentFilter) (*agents.AgentList, error)
	Get(ctx context.Context, id agents.AgentID) (*agents.Agent, error)
	Count(ctx context.Context, state agents.AgentState) (int, error)
	TransitionState(ctx context.Context, id agents.AgentID, newState agents.AgentState) (*agents.Agent, error)
}

type PortalHandlerScheduler interface {
	QueueDepth() int
	ActiveWorkers() int
}

type PortalHandlerAudit interface {
	Query(ctx context.Context, filter telemetry.AuditFilter) ([]telemetry.AuditEntry, error)
}

type PortalHandlerBrowser interface {
	ActiveCount() int
}

type PortalHandlerProviders interface {
	List(ctx context.Context) []providers.ProviderID
}

// RegisterPortalRoutes wires up the API endpoints for the portal dashboard.
func RegisterPortalRoutes(mux *http.ServeMux, deps PortalDeps) {
	h := NewPortalHandler(deps.AgentRepo, deps.Scheduler, deps.Audit, deps.BrowserMgr, deps.ProvReg)

	// Dashboard fleet overview
	mux.HandleFunc("/api/v1/dashboard", h.HandleDashboard)

	// Stats only
	mux.HandleFunc("/api/v1/dashboard/stats", h.HandleDashboardStats)

	// Agent detail and actions under /agents/{id}/... (handled in a small path parser)
	mux.HandleFunc("/api/v1/dashboard/agents/", h.HandleAgentRouter)

	// Providers health
	mux.HandleFunc("/api/v1/dashboard/providers/health", h.HandleProvidersHealth)

	// Workers
	mux.HandleFunc("/api/v1/dashboard/workers", h.HandleWorkers)

	// Queue
	mux.HandleFunc("/api/v1/dashboard/queue", h.HandleQueue)

	// Activity
	mux.HandleFunc("/api/v1/dashboard/activity", h.HandleActivity)

	// Errors
	mux.HandleFunc("/api/v1/dashboard/errors", h.HandleErrors)

	// Export
	mux.HandleFunc("/api/v1/dashboard/export", h.HandleExport)
}
