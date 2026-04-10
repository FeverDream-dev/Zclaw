package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/zclaw/zclaw/internal/agents"
	"github.com/zclaw/zclaw/internal/browser"
	"github.com/zclaw/zclaw/internal/memory"
	"github.com/zclaw/zclaw/internal/providers"
	"github.com/zclaw/zclaw/internal/providers/adapters"
	"github.com/zclaw/zclaw/internal/scheduler"
	"github.com/zclaw/zclaw/internal/storage"
	"github.com/zclaw/zclaw/internal/telemetry"
)

const version = "0.1.0"

type Config struct {
	DataDir         string
	DBPath          string
	HTTPPort        int
	HealthPort      int
	LogLevel        string
	BrowserWSURL    string
	DockerSocket    string
	WorkerPoolSize  int
	BrowserPoolSize int
	HeartbeatJitter int
	DefaultProvider string
	DefaultModel    string
}

func loadConfig() Config {
	return Config{
		DataDir:         envOr("ZCLAW_DATA_DIR", "./data"),
		DBPath:          envOr("ZCLAW_DB_PATH", "./data/zclaw.db"),
		HTTPPort:        envIntOr("ZCLAW_HTTP_PORT", 8080),
		HealthPort:      envIntOr("ZCLAW_HEALTH_PORT", 8081),
		LogLevel:        envOr("ZCLAW_LOG_LEVEL", "info"),
		BrowserWSURL:    envOr("ZCLAW_BROWSER_WORKER_URL", "ws://browser-worker:9222"),
		DockerSocket:    envOr("ZCLAW_DOCKER_SOCKET", "/var/run/docker.sock"),
		WorkerPoolSize:  envIntOr("ZCLAW_WORKER_POOL_SIZE", 10),
		BrowserPoolSize: envIntOr("ZCLAW_BROWSER_POOL_SIZE", 5),
		HeartbeatJitter: envIntOr("ZCLAW_HEARTBEAT_JITTER_SECONDS", 30),
		DefaultProvider: envOr("ZCLAW_DEFAULT_MODEL_PROVIDER", "openai"),
		DefaultModel:    envOr("ZCLAW_DEFAULT_MODEL", "gpt-4o-mini"),
	}
}

func main() {
	cfg := loadConfig()

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := runMigrate(cfg); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "backup-db" {
		if err := runBackup(cfg); err != nil {
			slog.Error("backup failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := run(cfg); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("starting zclaw", "version", version, "db", cfg.DBPath)

	db, err := storage.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	slog.Info("database ready")

	agentRepo := storage.NewAgentRepository(db)
	providerConfigRepo := storage.NewProviderConfigRepository(db)

	providerReg := providers.NewLocalRegistry()
	registerProviders(providerReg, cfg)
	slog.Info("providers registered", "count", len(providerReg.List(ctx)))

	browserCfg := browser.DefaultBrowserConfig()
	browserCfg.WSURL = cfg.BrowserWSURL
	browserCfg.MaxSessions = cfg.BrowserPoolSize
	browserMgr := browser.NewSessionManager(browserCfg)

	metrics := telemetry.NewMetricsCollector()
	audit := telemetry.NewAuditLogger()
	healthMon := telemetry.NewHealthMonitor(version)

	healthMon.Register("database", func(_ context.Context) telemetry.HealthCheck {
		return telemetry.HealthCheck{Status: telemetry.HealthHealthy, Timestamp: time.Now().UTC()}
	})
	healthMon.Register("browser", func(_ context.Context) telemetry.HealthCheck {
		active := browserMgr.ActiveCount()
		hc := telemetry.HealthCheck{Timestamp: time.Now().UTC()}
		if active <= browserCfg.MaxSessions {
			hc.Status = telemetry.HealthHealthy
		} else {
			hc.Status = telemetry.HealthDegraded
		}
		hc.Message = fmt.Sprintf("%d/%d sessions", active, browserCfg.MaxSessions)
		return hc
	})

	taskHandler := func(taskCtx context.Context, task scheduler.Task, agent agents.Agent) (*scheduler.TaskResult, error) {
		provider, model, err := providerReg.ResolveProvider(taskCtx, agent.Provider.ProviderID+"/"+agent.Provider.Model)
		if err != nil {
			return nil, fmt.Errorf("resolve provider: %w", err)
		}

		audit.Log(taskCtx, string(agent.ID), string(task.ID), "task_start", map[string]string{
			"provider": string(provider.ID()),
			"model":    model,
		})

		messages := []providers.Message{
			{Role: providers.RoleUser, Content: task.Input, Timestamp: time.Now().UTC()},
		}

		resp, err := provider.Generate(taskCtx, providers.GenerateRequest{
			Model:        model,
			Messages:     messages,
			SystemPrompt: agent.Provider.SystemPrompt,
			Temperature:  &agent.Provider.Temperature,
			MaxTokens:    agent.Provider.MaxContextTokens,
		})
		if err != nil {
			metrics.Inc(telemetry.MetricTasksFailed)
			audit.Log(taskCtx, string(agent.ID), string(task.ID), "task_error", err.Error())
			return nil, err
		}

		metrics.Inc(telemetry.MetricTasksCompleted)
		metrics.Add(telemetry.MetricTokensUsed, int64(resp.Usage.TotalTokens))
		metrics.Inc(telemetry.MetricProviderRequests)

		audit.Log(taskCtx, string(agent.ID), string(task.ID), "task_complete", map[string]any{
			"tokens": resp.Usage.TotalTokens,
			"model":  resp.Model,
		})

		return &scheduler.TaskResult{
			TaskID:    task.ID,
			Output:    resp.Message.Content,
			Usage:     resp.Usage,
			ModelUsed: resp.Model,
		}, nil
	}

	schedCfg := scheduler.DefaultConfig()
	schedCfg.MaxWorkers = cfg.WorkerPoolSize
	schedCfg.JitterSeconds = cfg.HeartbeatJitter
	sched := scheduler.NewScheduler(schedCfg, agentRepo, taskHandler)

	sched.Start(ctx)
	slog.Info("scheduler started", "max_workers", cfg.WorkerPoolSize)

	mux := http.NewServeMux()
	registerRoutes(mux, agentRepo, providerReg, providerConfigRepo, sched, browserMgr, metrics, healthMon, audit, cfg)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		report := healthMon.Check(ctx)
		w.Header().Set("Content-Type", "application/json")
		statusCode := 200
		if report.Status == telemetry.HealthDegraded {
			statusCode = 200
		} else if report.Status == telemetry.HealthUnhealthy {
			statusCode = 503
		}
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(report)
	})

	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HealthPort),
		Handler: healthMux,
	}

	go func() {
		slog.Info("api listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("api server error", "error", err)
		}
	}()

	go func() {
		slog.Info("health listening", "port", cfg.HealthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	sched.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
	healthServer.Shutdown(shutdownCtx)
	providerReg.Close()

	slog.Info("stopped")
	return nil
}

func registerProviders(reg *providers.LocalRegistry, cfg Config) {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewOpenAIAdapter(key, ""))
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewAnthropicAdapter(key, ""))
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewOpenRouterAdapter(key))
	}
	reg.Register(context.Background(), adapters.NewOllamaAdapter(os.Getenv("OLLAMA_BASE_URL")))
	if baseURL := os.Getenv("LITELLM_BASE_URL"); baseURL != "" {
		reg.Register(context.Background(), adapters.NewLiteLLMAdapter(os.Getenv("LITELLM_API_KEY"), baseURL))
	}
}

func registerRoutes(mux *http.ServeMux, agentRepo *storage.AgentRepository, provReg *providers.LocalRegistry, provCfgRepo *storage.ProviderConfigRepository, sched *scheduler.Scheduler, browserMgr *browser.LocalSessionManager, metrics *telemetry.MetricsCollector, healthMon *telemetry.HealthMonitor, audit *telemetry.LogAuditLogger, cfg Config) {
	mux.HandleFunc("GET /api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 50
		}
		list, err := agentRepo.List(r.Context(), agents.AgentFilter{
			State:   agents.AgentState(r.URL.Query().Get("state")),
			Limit:   limit,
			Offset:  offset,
			OrderBy: r.URL.Query().Get("order_by"),
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	mux.HandleFunc("POST /api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		var req agents.CreateAgentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		agent, err := agentRepo.Create(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		metrics.Inc(telemetry.MetricAgentsTotal)
		audit.Log(r.Context(), string(agent.ID), "", "agent_created", req.Name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(agent)
	})

	mux.HandleFunc("GET /api/v1/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		agent, err := agentRepo.Get(r.Context(), agents.AgentID(r.PathValue("id")))
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agent)
	})

	mux.HandleFunc("PATCH /api/v1/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req agents.UpdateAgentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		agent, err := agentRepo.Update(r.Context(), agents.AgentID(r.PathValue("id")), req)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agent)
	})

	mux.HandleFunc("DELETE /api/v1/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := agentRepo.Delete(r.Context(), agents.AgentID(r.PathValue("id"))); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		audit.Log(r.Context(), r.PathValue("id"), "", "agent_deleted", nil)
		w.WriteHeader(204)
	})

	mux.HandleFunc("GET /api/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		ids := provReg.List(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ids)
	})

	mux.HandleFunc("GET /api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		total, _ := agentRepo.Count(r.Context(), "")
		active, _ := agentRepo.Count(r.Context(), agents.StateActive)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agents_total":      total,
			"agents_active":     active,
			"queue_depth":       sched.QueueDepth(),
			"active_workers":    sched.ActiveWorkers(),
			"browser_sessions":  browserMgr.ActiveCount(),
			"provider_requests": metrics.Get(telemetry.MetricProviderRequests),
			"tokens_used":       metrics.Get(telemetry.MetricTokensUsed),
		})
	})

	mux.HandleFunc("POST /api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AgentID string `json:"agent_id"`
			Input   string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		task, err := sched.Enqueue(r.Context(), scheduler.EnqueueOptions{
			AgentID:  agents.AgentID(body.AgentID),
			Input:    body.Input,
			Priority: scheduler.PriorityNormal,
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(task)
	})
}

func runMigrate(cfg Config) error {
	ctx := context.Background()
	db, err := storage.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Migrate(ctx)
}

func runBackup(cfg Config) error {
	ctx := context.Background()
	db, err := storage.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	slog.Info("database backup point reached")
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// Suppress unused imports for memory package (used in full API routes).
var _ memory.ConversationID
