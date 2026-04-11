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
	"github.com/zclaw/zclaw/internal/api"
	"github.com/zclaw/zclaw/internal/auth"
	"github.com/zclaw/zclaw/internal/browser"
	"github.com/zclaw/zclaw/internal/config"
	"github.com/zclaw/zclaw/internal/connections"
	"github.com/zclaw/zclaw/internal/providers"
	"github.com/zclaw/zclaw/internal/providers/adapters"
	"github.com/zclaw/zclaw/internal/scheduler"
	"github.com/zclaw/zclaw/internal/storage"
	"github.com/zclaw/zclaw/internal/telemetry"
	"github.com/zclaw/zclaw/internal/tools"
)

const version = config.Version

func main() {
	cfg := config.Load()

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

func run(cfg config.Config) error {
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

	toolReg := tools.NewLocalToolRegistry()
	registerBuiltinTools(toolReg)
	slog.Info("tools registered", "count", len(toolReg.List()))

	connMgr := connections.NewConnectionManager()
	if envOr("ZCLAW_MCP_ENABLED", "") == "true" {
		mcpServer := connections.NewMCPServer()
		connMgr.RegisterMCP(mcpServer)
	}
	slog.Info("connection manager initialized")

	subAgentRepo := storage.NewSubAgentRepository(db)
	templateRepo := storage.NewTemplateRepository(db)
	_ = subAgentRepo
	_ = templateRepo
	slog.Info("sub-agent system initialized")

	// Auth initialization.
	var authService auth.AuthService
	if cfg.AuthEnabled {
		apiKeyRepo := storage.NewAPIKeyRepository(db)
		userRepo := storage.NewUserRepository(db)
		authStore := storage.NewAuthStore(apiKeyRepo, userRepo)
		authService = auth.NewAuthService(authStore, cfg.JWTSecret)

		if cfg.AdminAPIKey != "" {
			if err := bootstrapAdmin(ctx, db, cfg); err != nil {
				slog.Warn("admin bootstrap skipped", "error", err)
			}
		}
		slog.Info("auth enabled", "admin_configured", cfg.AdminAPIKey != "")
	} else {
		slog.Info("auth disabled — all requests run as admin")
	}

	mux := http.NewServeMux()
	registerRoutes(mux, agentRepo, providerReg, providerConfigRepo, sched, browserMgr, metrics, healthMon, audit, cfg)

	api.RegisterPortalRoutes(mux, api.PortalDeps{
		AgentRepo:  agentRepo,
		Scheduler:  sched,
		Audit:      audit,
		BrowserMgr: browserMgr,
		ProvReg:    providerReg,
	})
	slog.Info("dashboard portal routes registered")

	registerToolRoutes(mux, toolReg)
	registerConnectionRoutes(mux, connMgr)
	registerSubAgentRoutes(mux, subAgentRepo)

	var handler http.Handler = mux
	handler = api.RecoveryMiddleware(handler)
	handler = api.CorsMiddleware(handler)
	handler = api.LoggingMiddleware(handler)
	handler = api.RateLimitMiddleware(api.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst))(handler)
	handler = api.AuthMiddleware(api.AuthMiddlewareConfig{
		AuthService:  authService,
		AuthEnabled:  cfg.AuthEnabled,
		AdminAPIKey:  cfg.AdminAPIKey,
		APIKeyPrefix: cfg.APIKeyPrefix,
		PublicPaths:  map[string]bool{"/health": true},
	})(handler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      handler,
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

func bootstrapAdmin(ctx context.Context, db *storage.DB, cfg config.Config) error {
	tenantRepo := storage.NewTenantRepository(db)
	userRepo := storage.NewUserRepository(db)
	apiKeyRepo := storage.NewAPIKeyRepository(db)

	tenant, err := tenantRepo.GetBySlug(ctx, "default")
	if err != nil {
		tenant, err = tenantRepo.Create(ctx, "Default", "default", 300)
		if err != nil {
			return fmt.Errorf("create default tenant: %w", err)
		}
		slog.Info("bootstrapped default tenant", "id", tenant.ID)
	}

	users, err := userRepo.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	adminEmail := "admin@zclaw.local"
	var adminUser *auth.User
	for _, u := range users {
		if u.Role == auth.RoleAdmin {
			adminUser = &u
			break
		}
	}

	if adminUser == nil {
		adminUser, err = userRepo.Create(ctx, tenant.ID, adminEmail, "Admin", string(auth.RoleAdmin))
		if err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
		slog.Info("bootstrapped admin user", "id", adminUser.ID)
	}

	existingKeys, err := apiKeyRepo.ListByUser(ctx, adminUser.ID)
	if err != nil {
		return fmt.Errorf("list admin keys: %w", err)
	}

	for _, k := range existingKeys {
		if k.Name == "admin-bootstrap" {
			return nil
		}
	}

	plainKey, hash, err := auth.GenerateAPIKey(cfg.APIKeyPrefix)
	if err != nil {
		return fmt.Errorf("generate admin key: %w", err)
	}

	_, err = apiKeyRepo.Create(ctx, adminUser.ID, tenant.ID, "admin-bootstrap",
		plainKey[:8], hash, string(auth.RoleAdmin), nil)
	if err != nil {
		return fmt.Errorf("store admin key: %w", err)
	}

	slog.Info("bootstrapped admin API key (bootstrap key, not the env var)",
		"prefix", plainKey[:8],
		"tenant", string(tenant.ID))
	_ = plainKey
	return nil
}

func registerProviders(reg *providers.LocalRegistry, cfg config.Config) {
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
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewGroqAdapter(key))
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewGeminiAdapter(key))
	}
	if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewMistralAdapter(key))
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewXAIAdapter(key))
	}
	if key := os.Getenv("AWS_ACCESS_KEY_ID"); key != "" {
		reg.Register(context.Background(), adapters.NewBedrockAdapter(
			envOr("AWS_BEDROCK_BASE_URL", ""),
			key,
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
		))
	}
	if key := os.Getenv("QWEN_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewQwenAdapter(key))
	}
	if key := os.Getenv("FIREWORKS_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewFireworksAdapter(key))
	}
	if accountID := os.Getenv("CF_ACCOUNT_ID"); accountID != "" {
		reg.Register(context.Background(), adapters.NewCloudflareAIGatewayAdapter(
			accountID,
			os.Getenv("CF_GATEWAY_ID"),
			os.Getenv("CF_API_KEY"),
		))
	}
	if key := os.Getenv("OLLAMA_CLOUD_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewOllamaCloudAdapter(key))
	}
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewZAIAdapter(key))
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewDeepSeekAdapter(key))
	}
	if key := os.Getenv("TOGETHER_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewTogetherAdapter(key))
	}
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewMiniMaxAdapter(key))
	}
	if key := os.Getenv("MOONSHOT_API_KEY"); key != "" {
		reg.Register(context.Background(), adapters.NewMoonshotAdapter(key))
	}
	if token := os.Getenv("VERTEX_AI_ACCESS_TOKEN"); token != "" {
		reg.Register(context.Background(), adapters.NewVertexAIAdapter(
			token,
			envOr("VERTEX_AI_PROJECT_ID", ""),
			envOr("VERTEX_AI_REGION", "us-central1"),
		))
	}
}

func registerRoutes(mux *http.ServeMux, agentRepo *storage.AgentRepository, provReg *providers.LocalRegistry, provCfgRepo *storage.ProviderConfigRepository, sched *scheduler.Scheduler, browserMgr *browser.LocalSessionManager, metrics *telemetry.MetricsCollector, healthMon *telemetry.HealthMonitor, audit *telemetry.LogAuditLogger, cfg config.Config) {
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

func runMigrate(cfg config.Config) error {
	ctx := context.Background()
	db, err := storage.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Migrate(ctx)
}

func runBackup(cfg config.Config) error {
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

func registerBuiltinTools(reg *tools.LocalToolRegistry) {
	builtins := []tools.ToolExecutor{
		tools.NewWebFetchTool(),
		tools.NewFileReadTool(),
		tools.NewFileWriteTool(),
		tools.NewShellExecTool(),
		tools.NewHTTPRequestTool(),
		tools.NewJSONParseTool(),
		tools.NewWaitTool(),
		tools.NewPythonExecTool(),
		tools.NewJavaScriptExecTool(),
		tools.NewGoEvalTool(),
		tools.NewCSVReadTool(),
		tools.NewTextSearchTool(),
		tools.NewTextReplaceTool(),
		tools.NewBase64EncodeTool(),
		tools.NewBase64DecodeTool(),
		tools.NewHashTool(),
		tools.NewListFilesTool(),
		tools.NewDiskUsageTool(),
		tools.NewEnvTool(),
		tools.NewTimestampTool(),
	}
	for _, t := range builtins {
		if err := reg.Register(t); err != nil {
			slog.Warn("tool registration failed", "tool", t.Spec().ID, "error", err)
		}
	}
}

func registerToolRoutes(mux *http.ServeMux, reg *tools.LocalToolRegistry) {
	mux.HandleFunc("GET /api/v1/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reg.List())
	})

	mux.HandleFunc("GET /api/v1/tools/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, ok := reg.Get(id)
		if !ok {
			http.Error(w, "tool not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(t.Spec())
	})

	mux.HandleFunc("POST /api/v1/tools/{id}/execute", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var params map[string]any
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result, err := reg.Execute(r.Context(), id, params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
}

func registerConnectionRoutes(mux *http.ServeMux, cm *connections.ConnectionManager) {
	mux.HandleFunc("GET /api/v1/connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cm.Status())
	})
}

func registerSubAgentRoutes(mux *http.ServeMux, repo *storage.SubAgentRepository) {
	mux.HandleFunc("POST /api/v1/subagents", func(w http.ResponseWriter, r *http.Request) {
		var req agents.SubAgentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sa, err := repo.Spawn(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sa)
	})

	mux.HandleFunc("GET /api/v1/subagents/{id}", func(w http.ResponseWriter, r *http.Request) {
		sa, err := repo.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sa)
	})

	mux.HandleFunc("GET /api/v1/subagents/parent/{parentId}", func(w http.ResponseWriter, r *http.Request) {
		list, err := repo.ListByParent(r.Context(), r.PathValue("parentId"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	mux.HandleFunc("POST /api/v1/subagents/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if err := repo.Cancel(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		templates, err := repo.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(templates)
	})

	mux.HandleFunc("POST /api/v1/templates/{name}/instantiate", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ParentID string `json:"parent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		agent, err := repo.InstantiateTemplate(r.Context(), agents.AgentID(body.ParentID), r.PathValue("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(agent)
	})
}
