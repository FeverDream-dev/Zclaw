package telemetry

import (
	"context"
	"testing"
)

func TestMetricsCollector(t *testing.T) {
	mc := NewMetricsCollector()

	mc.Inc(MetricAgentsTotal)
	mc.Inc(MetricAgentsTotal)
	if mc.Get(MetricAgentsTotal) != 2 {
		t.Errorf("expected 2, got %d", mc.Get(MetricAgentsTotal))
	}

	mc.Set(MetricQueueDepth, 42)
	if mc.Get(MetricQueueDepth) != 42 {
		t.Errorf("expected 42, got %d", mc.Get(MetricQueueDepth))
	}

	mc.Add(MetricTokensUsed, 100)
	mc.Add(MetricTokensUsed, 50)
	if mc.Get(MetricTokensUsed) != 150 {
		t.Errorf("expected 150, got %d", mc.Get(MetricTokensUsed))
	}
}

func TestMetricsSnapshot(t *testing.T) {
	mc := NewMetricsCollector()
	mc.Inc(MetricAgentsTotal)
	snapshot := mc.Snapshot()
	if len(snapshot) == 0 {
		t.Error("expected non-empty snapshot")
	}
}

func TestHealthMonitor(t *testing.T) {
	hm := NewHealthMonitor("test-version")

	hm.Register("test-check", func(_ context.Context) HealthCheck {
		return HealthCheck{Status: HealthHealthy}
	})

	report := hm.Check(context.Background())
	if report.Status != HealthHealthy {
		t.Errorf("expected healthy, got %s", report.Status)
	}
	if report.Version != "test-version" {
		t.Errorf("expected test-version, got %s", report.Version)
	}
	if len(report.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(report.Checks))
	}
}

func TestHealthMonitorDegraded(t *testing.T) {
	hm := NewHealthMonitor("test")
	hm.Register("ok", func(_ context.Context) HealthCheck {
		return HealthCheck{Status: HealthHealthy}
	})
	hm.Register("degraded", func(_ context.Context) HealthCheck {
		return HealthCheck{Status: HealthDegraded}
	})
	report := hm.Check(context.Background())
	if report.Status != HealthDegraded {
		t.Errorf("expected degraded, got %s", report.Status)
	}
}

func TestAuditLogger(t *testing.T) {
	al := NewAuditLogger()
	ctx := context.Background()

	al.Log(ctx, "agent-1", "task-1", "test_action", map[string]string{"key": "value"})

	entries, err := al.Query(ctx, AuditFilter{AgentID: "agent-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != "test_action" {
		t.Errorf("expected test_action, got %s", entries[0].Action)
	}

	entries, _ = al.Query(ctx, AuditFilter{AgentID: "nonexistent"})
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nonexistent agent")
	}
}
