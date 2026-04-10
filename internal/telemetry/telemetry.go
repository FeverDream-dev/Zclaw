package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Duration  string       `json:"duration,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

type HealthReport struct {
	Status    HealthStatus  `json:"status"`
	Checks    []HealthCheck `json:"checks"`
	Timestamp time.Time     `json:"timestamp"`
	Version   string        `json:"version"`
	Uptime    string        `json:"uptime"`
}

type MetricName string

const (
	MetricAgentsTotal      MetricName = "agents_total"
	MetricAgentsActive     MetricName = "agents_active"
	MetricTasksPending     MetricName = "tasks_pending"
	MetricTasksRunning     MetricName = "tasks_running"
	MetricTasksCompleted   MetricName = "tasks_completed"
	MetricTasksFailed      MetricName = "tasks_failed"
	MetricWorkersActive    MetricName = "workers_active"
	MetricBrowserSessions  MetricName = "browser_sessions"
	MetricQueueDepth       MetricName = "queue_depth"
	MetricProviderRequests MetricName = "provider_requests"
	MetricProviderErrors   MetricName = "provider_errors"
	MetricTokensUsed       MetricName = "tokens_used"
)

type Metric struct {
	Name      MetricName        `json:"name"`
	Value     int64             `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

type AuditEntry struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Action    string    `json:"action"`
	Details   any       `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type HealthChecker func(ctx context.Context) HealthCheck

type MetricsCollector struct {
	counters map[MetricName]*atomic.Int64
	gauges   map[MetricName]*atomic.Int64
	mu       sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		counters: make(map[MetricName]*atomic.Int64),
		gauges:   make(map[MetricName]*atomic.Int64),
	}
	for _, name := range []MetricName{
		MetricAgentsTotal, MetricAgentsActive,
		MetricTasksPending, MetricTasksRunning,
		MetricTasksCompleted, MetricTasksFailed,
		MetricWorkersActive, MetricBrowserSessions,
		MetricQueueDepth, MetricProviderRequests,
		MetricProviderErrors, MetricTokensUsed,
	} {
		mc.counters[name] = &atomic.Int64{}
		mc.gauges[name] = &atomic.Int64{}
	}
	return mc
}

func (mc *MetricsCollector) Inc(name MetricName) {
	if c, ok := mc.counters[name]; ok {
		c.Add(1)
	}
}

func (mc *MetricsCollector) Add(name MetricName, delta int64) {
	if c, ok := mc.counters[name]; ok {
		c.Add(delta)
	}
}

func (mc *MetricsCollector) Set(name MetricName, value int64) {
	if g, ok := mc.gauges[name]; ok {
		g.Store(value)
	}
}

func (mc *MetricsCollector) Get(name MetricName) int64 {
	if g, ok := mc.gauges[name]; ok {
		return g.Load()
	}
	if c, ok := mc.counters[name]; ok {
		return c.Load()
	}
	return 0
}

func (mc *MetricsCollector) Snapshot() []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	now := time.Now().UTC()
	var metrics []Metric
	for name, c := range mc.counters {
		metrics = append(metrics, Metric{
			Name:      name,
			Value:     c.Load(),
			Timestamp: now,
		})
	}
	return metrics
}

func (mc *MetricsCollector) SnapshotJSON() []byte {
	metrics := mc.Snapshot()
	data, _ := json.Marshal(metrics)
	return data
}

type HealthMonitor struct {
	checkers map[string]HealthChecker
	started  time.Time
	version  string
}

func NewHealthMonitor(version string) *HealthMonitor {
	return &HealthMonitor{
		checkers: make(map[string]HealthChecker),
		started:  time.Now().UTC(),
		version:  version,
	}
}

func (hm *HealthMonitor) Register(name string, checker HealthChecker) {
	hm.checkers[name] = checker
}

func (hm *HealthMonitor) Check(ctx context.Context) *HealthReport {
	report := &HealthReport{
		Timestamp: time.Now().UTC(),
		Version:   hm.version,
		Uptime:    time.Since(hm.started).Round(time.Second).String(),
		Status:    HealthHealthy,
	}

	for name, checker := range hm.checkers {
		hc := checker(ctx)
		hc.Name = name
		hc.Timestamp = time.Now().UTC()
		report.Checks = append(report.Checks, hc)

		if hc.Status == HealthUnhealthy {
			report.Status = HealthUnhealthy
		} else if hc.Status == HealthDegraded && report.Status != HealthUnhealthy {
			report.Status = HealthDegraded
		}
	}

	return report
}

type AuditLogger interface {
	Log(ctx context.Context, agentID, taskID, action string, details any) error
	Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
}

type AuditFilter struct {
	AgentID string    `json:"agent_id,omitempty"`
	TaskID  string    `json:"task_id,omitempty"`
	Action  string    `json:"action,omitempty"`
	From    time.Time `json:"from,omitempty"`
	To      time.Time `json:"to,omitempty"`
	Limit   int       `json:"limit,omitempty"`
}

type LogAuditLogger struct {
	mu  sync.Mutex
	log []AuditEntry
}

func NewAuditLogger() *LogAuditLogger {
	return &LogAuditLogger{
		log: make([]AuditEntry, 0),
	}
}

func (l *LogAuditLogger) Log(ctx context.Context, agentID, taskID, action string, details any) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := AuditEntry{
		ID:        time.Now().UnixMilli(),
		AgentID:   agentID,
		TaskID:    taskID,
		Action:    action,
		Details:   details,
		CreatedAt: time.Now().UTC(),
	}
	l.log = append(l.log, entry)
	return nil
}

func (l *LogAuditLogger) Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []AuditEntry
	for _, entry := range l.log {
		if filter.AgentID != "" && entry.AgentID != filter.AgentID {
			continue
		}
		if filter.TaskID != "" && entry.TaskID != filter.TaskID {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		result = append(result, entry)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (l *LogAuditLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, _ := json.MarshalIndent(l.log, "", "  ")
	return string(data)
}

type fmtDetails struct {
	agentID string
	taskID  string
	action  string
	details any
}

func (d fmtDetails) String() string {
	return fmt.Sprintf("agent=%s task=%s action=%s details=%v", d.agentID, d.taskID, d.action, d.details)
}
