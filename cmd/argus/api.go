package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/argusxdr/argus/internal/api/behaviour"
	"github.com/argusxdr/argus/internal/auth"
	"github.com/argusxdr/argus/internal/baseline"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/detection/loader"
	"github.com/argusxdr/argus/internal/detection/worker"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/kairos"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/argusxdr/argus/internal/resilience"
	"github.com/argusxdr/argus/internal/secrets"
	"github.com/argusxdr/argus/internal/storage"
	"github.com/argusxdr/argus/internal/trace"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// apiCmd represents the api subcommand (PLATFORM-02).
// Starts only the query API (GET /v1/signals) without gRPC, HTTP receivers, or OTLP bridge.
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the Argus query API only",
	Long:  `Start the Argus query API (GET /v1/signals) without ingest receivers.`,
	RunE:  runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)

	// HTTP server flags
	apiCmd.Flags().String("http-addr", "localhost:8080", "HTTP server address")
	viper.BindEnv("server.http.addr", "ARGUS_SERVER_HTTP_ADDR")
	viper.BindPFlag("server.http.addr", apiCmd.Flags().Lookup("http-addr"))

	// Database flags
	apiCmd.Flags().String("clickhouse-dsn", "localhost:9000", "ClickHouse address (host:port)")
	viper.BindEnv("database.clickhouse.dsn", "ARGUS_DATABASE_CLICKHOUSE_DSN")
	viper.BindPFlag("database.clickhouse.dsn", apiCmd.Flags().Lookup("clickhouse-dsn"))
	apiCmd.Flags().String("postgres-dsn", "", "PostgreSQL DSN (optional, enables alert persistence)")
	viper.BindEnv("database.postgres.dsn", "ARGUS_DATABASE_POSTGRES_DSN")
	viper.BindPFlag("database.postgres.dsn", apiCmd.Flags().Lookup("postgres-dsn"))
	apiCmd.Flags().String("rules-dir", "internal/rules/built-in", "Directory containing built-in YAML rules")
	viper.BindEnv("detection.rules_dir", "ARGUS_DETECTION_RULES_DIR")
	viper.BindPFlag("detection.rules_dir", apiCmd.Flags().Lookup("rules-dir"))
	apiCmd.Flags().String("redis-addr", "localhost:6379", "Redis address (host:port)")
	viper.BindEnv("redis.addr", "ARGUS_REDIS_ADDR")
	viper.BindPFlag("redis.addr", apiCmd.Flags().Lookup("redis-addr"))

	// Logging flags
	apiCmd.Flags().Bool("dev", false, "Enable development logging (more verbose)")
	viper.BindPFlag("logging.dev", apiCmd.Flags().Lookup("dev"))

	// Kairos flags (optional — leave empty to disable)
	apiCmd.Flags().String("kairos-endpoint", "", "Kairos policy engine endpoint (empty = disabled)")
	viper.BindEnv("kairos.endpoint", "ARGUS_KAIROS_ENDPOINT")
	viper.BindPFlag("kairos.endpoint", apiCmd.Flags().Lookup("kairos-endpoint"))
	apiCmd.Flags().Duration("kairos-timeout", 500*time.Millisecond, "Kairos evaluation timeout")
	viper.BindEnv("kairos.timeout", "ARGUS_KAIROS_TIMEOUT")
	viper.BindPFlag("kairos.timeout", apiCmd.Flags().Lookup("kairos-timeout"))
}

// runAPI starts the Argus query API subsystem only.
// Per P3 (Graceful Degradation): starts even when ClickHouse is unavailable.
// Queries return 503 when storage is down; health endpoint reports degraded state.
func runAPI(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logging
	log, err := newLogger(viper.GetBool("logging.dev"))
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Sync()

	// Initialize secrets store (optional but preferred)
	// If ARGUS_SECRETS_FILE is set or ./argus.key exists, load it.
	// Otherwise fall back to env-var-only mode.
	secretsFile := os.Getenv("ARGUS_SECRETS_FILE")
	if secretsFile == "" {
		if _, err := os.Stat("./argus.key"); err == nil {
			secretsFile = "./argus.key"
		}
	}
	if secretsFile != "" {
		if store, err := secrets.NewStore(secretsFile, nil); err == nil {
			secrets.SetStore(store)
			log.Info("secrets store initialized", zap.String("file", secretsFile))
		} else {
			log.Warn("secrets file configured but unloadable; falling back to env vars", zap.Error(err))
		}
	}

	log.Info("starting Argus query API subsystem")

	// Connect to ClickHouse — non-fatal (P3: graceful degradation)
	chDSN := viper.GetString("database.clickhouse.dsn")
	log.Info("connecting to ClickHouse", zap.String("dsn", maskDSN(chDSN)))

	var ch *storage.ClickHouse
	var chErr error
	ch, chErr = storage.NewClickHouse(ctx, chDSN)
	if chErr != nil {
		log.Warn("ClickHouse unavailable — starting in degraded mode; queries will return 503",
			zap.Error(chErr))
	} else {
		defer ch.Close()
		log.Info("ClickHouse connected")
	}

	// Connect to PostgreSQL (optional but needed for auth)
	pgDSN := viper.GetString("database.postgres.dsn")
	var pg *storage.Postgres
	var pgPool *pgxpool.Pool
	if pgDSN != "" {
		pg, err = storage.NewPostgres(ctx, pgDSN)
		if err != nil {
			log.Warn("PostgreSQL unavailable - detection alerts and auth will not persist", zap.Error(err))
		} else {
			defer pg.Close()
			log.Info("PostgreSQL connected")

			// Get the pool for migrations
			pgPool = pg.Pool

			// Run pending migrations so all tables exist before the server
			// starts accepting requests. Safe to call on every startup (idempotent).
			if migrateErr := storage.MigrateDB(ctx, pgPool, pgDSN, log); migrateErr != nil {
				log.Warn("database migration failed — some features may be unavailable",
					zap.Error(migrateErr))
			} else {
				log.Info("database migrations up to date")
			}
		}
	}

	// Auth subsystem (requires PostgreSQL)
	var authSvc *ingest.AuthService
	if pgPool != nil {
		privateKey, publicKey, keyErr := auth.LoadOrGenerateRSAKey()
		if keyErr != nil {
			log.Warn("failed to load/generate JWT key — auth disabled", zap.Error(keyErr))
		} else {
			userStore := auth.NewPgUserStore(pgPool)
			sessionStore := auth.NewPgSessionStore(pgPool)
			auditStore := auth.NewPgAuditStore(pgPool)
			auditLogger := auth.NewAuditLogger(auditStore)
			userSvc := auth.NewUserService(userStore)
			apiKeyStore := auth.NewPgAPIKeyStore(pgPool)
			tokenMgr := auth.NewTokenManager(auth.TokenConfig{
				PrivateKey: privateKey,
				PublicKey:  publicKey,
			})
			sessionMgr := auth.NewSessionManager(sessionStore, 0)
			authSvc = &ingest.AuthService{
				UserSvc:      userSvc,
				UserStore:    userStore,
				SessionMgr:   sessionMgr,
				TokenMgr:     tokenMgr,
				AuditLog:     auditLogger,
				SessionStore: sessionStore,
				APIKeyStore:  apiKeyStore,
			}
			log.Info("auth subsystem initialized")
		}
	}

	// Register metrics
	reg := prometheus.NewRegistry()
	httpMetrics := metrics.NewHTTP(reg)

	// Create query handler
	queryHandler := ingest.NewQueryHandler(ch, httpMetrics, log)
	ruleStore := engine.NewRuleStore()
	rulesDir := viper.GetString("detection.rules_dir")
	if rulesDir == "" {
		rulesDir = "internal/rules/built-in"
	}
	if builtInRules, loadErr := engine.LoadRulesFromDirectory(rulesDir); loadErr != nil {
		log.Warn("could not load built-in rules directory", zap.String("dir", rulesDir), zap.Error(loadErr))
	} else {
		for _, r := range builtInRules {
			ruleStore.Add(r)
		}
		log.Info("built-in rules loaded", zap.Int("count", ruleStore.Count()))
	}
	queryHandler.SetRuleStore(ruleStore)

	// Wire auth service if initialized
	if authSvc != nil {
		queryHandler.SetAuthService(authSvc)
	}

	// Notification adapter registry
	adapterRegistry := notify.NewAdapterRegistry(log)
	logAdapter := notify.NewLogAdapter(log)
	cbAdapter := notify.NewCircuitBreakerAdapter(logAdapter, notify.NewCircuitBreaker(nil))
	if regErr := adapterRegistry.Register(cbAdapter); regErr != nil {
		log.Warn("failed to register log adapter", zap.Error(regErr))
	}

	// Alert dispatcher (fixed worker pool)
	dispatcherCfg := notify.DefaultDispatcherConfig()
	dispatcherCfg.WorkerCount = 2
	alertDispatcher, dispErr := notify.NewAlertDispatcher(dispatcherCfg, adapterRegistry, log)
	if dispErr != nil {
		log.Warn("alert dispatcher unavailable", zap.Error(dispErr))
	} else {
		defer func() {
			shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()
			alertDispatcher.Shutdown(shutdownCtx2)
		}()
	}

	// Routing engine (requires PostgreSQL)
	var routingEngine *notify.RoutingEngine
	if pgPool != nil {
		routingEngine, err = notify.NewRoutingEngine(pgPool, log)
		if err != nil {
			log.Warn("routing engine unavailable", zap.Error(err))
			err = nil // non-fatal
		} else {
			routingEngine.Start()
			defer routingEngine.Stop()
		}
	}

	// Redis client for dedup + incident correlation
	redisAddr := viper.GetString("redis.addr")
	var redisClient *redis.Client
	if redisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
		if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
			log.Warn("Redis unavailable — dedup and incident correlation disabled", zap.Error(pingErr))
			redisClient = nil
		} else {
			defer redisClient.Close()
			log.Info("Redis connected", zap.String("addr", redisAddr))
		}
	}

	// Alert router: replaces PgAlertWriter — owns dedup, persist, route, dispatch
	alertRouter := ingest.NewAlertRouter(pgPool, redisClient, routingEngine, alertDispatcher, log)
	queryHandler.SetAlertRouter(alertRouter)
	if pgPool != nil {
		queryHandler.SetPool(pgPool)
	}

	// ── Detection engine wiring ──────────────────────────────────────────────
	// Construct the detection engine from the already-seeded ruleStore.
	detectionEngine := engine.New(ruleStore, nil)

	// Detection metrics — registered on the shared Prometheus registry.
	detMetrics := metrics.NewDetection(reg)

	// Circuit breaker (30s half-open cooldown).
	detBreaker := breaker.New(30 * time.Second)

	// Kairos evaluator — optional; nil when endpoint is not configured.
	var kairosEval worker.KairosEvaluator
	if kairosEndpoint := viper.GetString("kairos.endpoint"); kairosEndpoint != "" {
		kairosTimeout := viper.GetDuration("kairos.timeout")
		if kairosTimeout <= 0 {
			kairosTimeout = 500 * time.Millisecond
		}
		kairosClient := kairos.NewClient(kairosEndpoint, kairosTimeout, log)
		kairosEval = kairos.NewEvaluator(kairosClient, nil, log)
		log.Info("Kairos evaluator enabled", zap.String("endpoint", kairosEndpoint))
	}

	// Async detection worker — queue depth 10000, workers = GOMAXPROCS*2.
	asyncWorker := worker.New(
		detectionEngine,
		alertRouter,
		detBreaker,
		detMetrics,
		log,
		10000,
		runtime.GOMAXPROCS(0)*2,
	)
	if kairosEval != nil {
		asyncWorker.WithKairos(kairosEval)
	}
	asyncWorker.Start(ctx)
	defer asyncWorker.Shutdown()

	// DB-backed rule hot-reload (polls every 45s, short-circuits on unchanged version).
	// Only active when PostgreSQL is available; falls back to static YAML rules otherwise.
	if pgPool != nil {
		ruleReloader := loader.New(pgPool, ruleStore, 45*time.Second, log)
		go ruleReloader.Run(ctx)
	} else {
		log.Warn("PostgreSQL unavailable — DB rule hot-reload disabled; using static YAML rules only")
	}

	// Circuit breaker evaluation loop (5s cadence per D-19).
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				detBreaker.Evaluate(
					asyncWorker.QueueDepth(), 10000,
					asyncWorker.P99LatencyMs(), 500.0,
				)
			}
		}
	}()
	// ── End detection engine wiring ──────────────────────────────────────────

	// Build router
	httpAddr := viper.GetString("server.http.addr")
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// CORS middleware for development (allow localhost:5173)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from localhost:5173 (Vite dev server)
			origin := r.Header.Get("Origin")
			if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Auth middleware on protected routes
	if authSvc != nil {
		r.Use(func(next http.Handler) http.Handler {
			return auth.AuthMiddleware(auth.MiddlewareConfig{
				TokenManager: authSvc.TokenMgr,
				SessionStore: authSvc.SessionStore,
				AuditLogger:  authSvc.AuditLog,
				Logger:       log,
				ExcludedPaths: map[string]bool{
					// Health and observability (always public)
					"/health":    true,
					"/metrics":   true,
					// Setup status check + setup endpoint (public until first admin created)
					"/api/v1/setup/status": true,
					"/api/v1/setup":        true,
					// Unauthenticated auth entry points
					"/api/v1/auth/login":        true,
					"/api/v1/auth/refresh":      true,
					"/api/v1/auth/mfa/challenge": true,
					"/api/v1/auth/csrf-token":   true,
					// Signal ingest (protected by API key, not JWT)
					"/v1/signals":        true,
					"/v1/signals/stream": true,
					"/v1/schema/signals": true,
				},
				// Invite routes have path parameters — use prefix exclusion
				ExcludedPrefixes: []string{
					"/api/v1/invite/",
				},
			})(next)
		})
	}

	// Health endpoint — always available, reports component status
	r.Get("/health", makeHealthHandler(ch, pgPool, redisClient, log))

	// Prometheus metrics
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// Wire rate limiter to query handler if Redis is available
	if redisClient != nil {
		rl := resilience.NewRedisRateLimiter(redisClient)
		queryHandler.SetRateLimiter(rl)
		log.Info("rate limiter wired to query handler")
	}

	// Query API — returns 503 on individual endpoints when ClickHouse unavailable
	queryHandler.RegisterRoutes(r)

	// Phase 7: Behavioural traceability endpoints (requires ClickHouse + PostgreSQL)
	if ch != nil && pgPool != nil {
		reconstructor := trace.NewRunReconstructor(ch.Conn())
		timelineBuilder := trace.NewTimelineBuilder(ch.Conn())
		sessionStore := baseline.NewSessionProfileStore(redisClient, pgPool, log)
		behaviourHandler := behaviour.NewBehaviourHandler(
			ch.Conn(), pgPool, reconstructor, timelineBuilder, sessionStore, log,
		)

		// Start session baseline engine (async, 10-min cadence) — non-blocking
		sessionEngine := baseline.NewSessionBaselineEngine(ch.Conn(), pgPool, redisClient, log, nil)
		if err := sessionEngine.Start(ctx); err != nil {
			log.Warn("session baseline engine failed to start", zap.Error(err))
		}

		// Register routes under JWT auth (already applied globally) + analyst/admin role guard.
		r.Group(func(gr chi.Router) {
			gr.Use(auth.RequireRole("analyst", "admin"))
			behaviourHandler.RegisterRoutes(gr)
		})
		log.Info("phase 7 behaviour endpoints registered")
	}

	// Signal Broadcaster and WebSocket streaming (create first so HTTP receiver can use it)
	broadcaster := ingest.NewSignalBroadcaster()
	go broadcaster.Run(ctx)
	wsHandler := ingest.NewWebSocketHandler(broadcaster, log)
	wsHandler.RegisterRoutes(r)

	// HTTP Signal Ingest Receiver (add to query API for full functionality)
	if pg != nil && ch != nil {
		// Create ingest-specific metrics registry
		ingestReg := prometheus.NewRegistry()
		ingestMetrics := metrics.NewIngest(ingestReg)

		// Create ingest queue for signal batching
		queueCapacity := viper.GetInt("ingest.queue.capacity")
		if queueCapacity <= 0 {
			queueCapacity = 100000
		}
		queue := ingest.NewQueue(queueCapacity, ingestMetrics)

		// Create batch writer for ClickHouse persistence
		batchSize := viper.GetInt("storage.batch.size")
		if batchSize <= 0 {
			batchSize = 500
		}
		batchInterval := viper.GetDuration("storage.batch.interval")
		if batchInterval <= 0 {
			batchInterval = 2 * time.Second
		}
		storageMetrics := metrics.NewStorage(ingestReg)
		batchWriter, err := storage.NewBatchWriter(ctx, ch, batchSize, batchInterval, storageMetrics, log)
		if err != nil {
			log.Warn("failed to create batch writer for HTTP ingest", zap.Error(err))
		} else {
			// Start drain worker to process queued signals
			go storage.DrainWorker(ctx, queue, batchWriter, log)
		}

		// Create auth validator for API key validation
		// Support test mode (ARGUS_TEST_MODE=true) for validation harness
		var authValidator *ingest.AuthValidator
		if os.Getenv("ARGUS_TEST_MODE") == "true" {
			testKeys := map[string]string{
				"test-signal-api-key-validation-harness": "test-app",
			}
			authValidator = ingest.NewTestAuthValidator(log, testKeys)
			log.Info("auth validator in TEST MODE — using hardcoded test keys")
		} else {
			authValidator = ingest.NewAuthValidator(pg, log)
		}

		// Create HTTP receiver for signal ingest
		httpReceiver := ingest.NewHTTPReceiver(queue, authValidator, ingestMetrics, broadcaster, log)

		// Create API key store for signal ingest protection
		var apiKeyStore auth.ApiKeyStore
		if pgPool != nil {
			apiKeyStore = auth.NewPgAPIKeyStore(pgPool)
			httpReceiver.SetAPIKeyStore(apiKeyStore)

			// Create token bucket rate limiter for signal ingest (per API key)
			ingestLimiter := resilience.NewAPIKeyTokenBucketLimiter(10000, 100)

			// Mount API key middleware on signal ingest endpoints
			// Routes requiring signals:write scope
			r.Route("/v1", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(auth.APIKeyMiddleware(apiKeyStore, "signals:write"))
					r.Use(ingestLimiter.Middleware()) // Rate limit after auth
					r.Post("/signals", httpReceiver.HandlePostSignals)
				})
				// Route requiring signals:read scope
				r.Group(func(r chi.Router) {
					r.Use(auth.APIKeyMiddleware(apiKeyStore, "signals:read"))
					r.Get("/schema/signals", queryHandler.HandleGetSignalSchema)
				})
			})
		}

		// Register HTTP receiver routes (skips /v1/signals if apiKeyStore is set)
		httpReceiver.RegisterRoutes(r)

		// Register ingest metrics endpoint
		r.Handle("/metrics/ingest", promhttp.HandlerFor(ingestReg, promhttp.HandlerOpts{Registry: ingestReg}))
	}

	// Bind listener first so we know the port is available before logging
	ln, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("failed to bind %s: %w", httpAddr, err)
	}

	httpServer := &http.Server{
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Log AFTER bind succeeds — no more "listening" before the port is actually held
	log.Info("HTTP server listening", zap.String("addr", httpAddr))

	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Info("shutdown signal received, gracefully stopping")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
	log.Info("query API subsystem stopped")
	return nil
}

// makeHealthHandler returns a component-level health handler (Step 1.6 from build prompt).
// Always returns 200 — clients check "status" field for degraded/healthy.
func makeHealthHandler(ch *storage.ClickHouse, pgPool *pgxpool.Pool, redisClient *redis.Client, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		type componentHealth struct {
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms,omitempty"`
			Error     string `json:"error,omitempty"`
		}

		overall := "healthy"

		// ClickHouse check
		chComp := componentHealth{Status: "unknown"}
		if ch == nil {
			chComp = componentHealth{Status: "unhealthy", Error: "not configured"}
			overall = "degraded"
		} else {
			start := time.Now()
			if err := ch.Ping(ctx); err != nil {
				chComp = componentHealth{Status: "unhealthy", Error: err.Error()}
				overall = "degraded"
			} else {
				chComp = componentHealth{Status: "healthy", LatencyMs: time.Since(start).Milliseconds()}
			}
		}

		// PostgreSQL check
		pgComp := componentHealth{Status: "unknown"}
		if pgPool == nil {
			pgComp = componentHealth{Status: "unhealthy", Error: "not configured"}
			overall = "degraded"
		} else {
			start := time.Now()
			if err := pgPool.Ping(ctx); err != nil {
				pgComp = componentHealth{Status: "unhealthy", Error: err.Error()}
				overall = "degraded"
			} else {
				pgComp = componentHealth{Status: "healthy", LatencyMs: time.Since(start).Milliseconds()}
			}
		}

		// Redis check
		redisComp := componentHealth{Status: "unknown"}
		if redisClient == nil {
			redisComp = componentHealth{Status: "unhealthy", Error: "not configured"}
			overall = "degraded"
		} else {
			start := time.Now()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				redisComp = componentHealth{Status: "unhealthy", Error: err.Error()}
				overall = "degraded"
			} else {
				redisComp = componentHealth{Status: "healthy", LatencyMs: time.Since(start).Milliseconds()}
			}
		}

		type healthResponse struct {
			Status     string                     `json:"status"`
			Components map[string]componentHealth `json:"components"`
		}

		resp := healthResponse{
			Status: overall,
			Components: map[string]componentHealth{
				"clickhouse": chComp,
				"postgres":   pgComp,
				"redis":      redisComp,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
