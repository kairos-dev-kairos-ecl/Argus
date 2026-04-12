package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/argusxdr/argus/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Logging flags
	apiCmd.Flags().Bool("dev", false, "Enable development logging (more verbose)")
	viper.BindPFlag("logging.dev", apiCmd.Flags().Lookup("dev"))
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

	// Register metrics
	reg := prometheus.NewRegistry()
	httpMetrics := metrics.NewHTTP(reg)

	// Build router
	httpAddr := viper.GetString("server.http.addr")
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health endpoint — always available, reports component status
	r.Get("/health", makeHealthHandler(ch, log))

	// Prometheus metrics
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// Query API — returns 503 on individual endpoints when ClickHouse unavailable
	queryHandler := ingest.NewQueryHandler(ch, httpMetrics, log)
	queryHandler.RegisterRoutes(r)

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
func makeHealthHandler(ch *storage.ClickHouse, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		chStatus := `"healthy"`
		if ch == nil {
			chStatus = `"unhealthy"`
		} else {
			pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := ch.Ping(pingCtx); err != nil {
				chStatus = fmt.Sprintf(`"unhealthy: %s"`, err.Error())
			}
		}

		overall := "healthy"
		if chStatus != `"healthy"` {
			overall = "degraded"
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":%q,"components":{"clickhouse":{"status":%s}}}`,
			overall, chStatus)
	}
}
