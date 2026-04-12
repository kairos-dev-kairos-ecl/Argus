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

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/argusxdr/argus/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ingestCmd represents the ingest subcommand (PLATFORM-02).
// Starts only ingest subsystems (gRPC receiver, HTTP receiver, OTLP bridge, queue, batch writer).
// Does NOT start the query API (GET /v1/signals).
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Start the Argus ingest subsystem only",
	Long:  `Start the Argus ingest receiver (gRPC, HTTP, OTLP) without the query API.`,
	RunE:  runIngest,
}

func init() {
	rootCmd.AddCommand(ingestCmd)

	// gRPC server flags
	ingestCmd.Flags().String("grpc-addr", "localhost:5001", "gRPC server address")
	viper.BindPFlag("server.grpc.addr", ingestCmd.Flags().Lookup("grpc-addr"))

	// HTTP server flags (for receivers and metrics only)
	ingestCmd.Flags().String("http-addr", "localhost:8080", "HTTP server address")
	viper.BindEnv("server.http.addr", "ARGUS_SERVER_HTTP_ADDR")
	viper.BindPFlag("server.http.addr", ingestCmd.Flags().Lookup("http-addr"))

	// Database flags
	ingestCmd.Flags().String("postgres-dsn", "postgres://postgres:password@localhost:5432/argus", "PostgreSQL DSN")
	viper.BindPFlag("database.postgres.dsn", ingestCmd.Flags().Lookup("postgres-dsn"))

	ingestCmd.Flags().String("clickhouse-dsn", "localhost:9000", "ClickHouse address (host:port)")
	viper.BindEnv("database.clickhouse.dsn", "ARGUS_DATABASE_CLICKHOUSE_DSN")
	viper.BindPFlag("database.clickhouse.dsn", ingestCmd.Flags().Lookup("clickhouse-dsn"))

	// Queue flags
	ingestCmd.Flags().Int("queue-capacity", 100000, "Ingest queue capacity")
	viper.BindPFlag("ingest.queue.capacity", ingestCmd.Flags().Lookup("queue-capacity"))

	// Batch writer flags
	ingestCmd.Flags().Int("batch-size", 500, "ClickHouse batch writer size")
	viper.BindPFlag("storage.batch.size", ingestCmd.Flags().Lookup("batch-size"))

	ingestCmd.Flags().Duration("batch-interval", 2*time.Second, "ClickHouse batch flush interval")
	viper.BindPFlag("storage.batch.interval", ingestCmd.Flags().Lookup("batch-interval"))

	// Logging flags
	ingestCmd.Flags().Bool("dev", false, "Enable development logging (more verbose)")
	viper.BindPFlag("logging.dev", ingestCmd.Flags().Lookup("dev"))
}

// runIngest starts the Argus ingest subsystem only.
func runIngest(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logging
	log, err := newLogger(viper.GetBool("logging.dev"))
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Sync()

	log.Info("starting Argus ingest subsystem")

	// Connect to PostgreSQL
	pgDSN := viper.GetString("database.postgres.dsn")
	log.Info("connecting to PostgreSQL", zap.String("dsn", maskDSN(pgDSN)))
	pg, err := storage.NewPostgres(ctx, pgDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgresql: %w", err)
	}
	defer pg.Close()
	log.Info("postgresql connected")

	// Connect to ClickHouse
	chDSN := viper.GetString("database.clickhouse.dsn")
	log.Info("connecting to ClickHouse", zap.String("dsn", maskDSN(chDSN)))
	ch, err := storage.NewClickHouse(ctx, chDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to clickhouse: %w", err)
	}
	defer ch.Close()
	log.Info("clickhouse connected")

	// Register metrics
	reg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(reg)

	// Create ingest queue
	queueCapacity := viper.GetInt("ingest.queue.capacity")
	queue := ingest.NewQueue(queueCapacity, ingestMetrics)
	log.Info("ingest queue created", zap.Int("capacity", queueCapacity))

	// Create batch writer
	batchSize := viper.GetInt("storage.batch.size")
	batchInterval := viper.GetDuration("storage.batch.interval")
	storageMetrics := metrics.NewStorage(reg)
	batchWriter, err := storage.NewBatchWriter(ctx, ch, batchSize, batchInterval, storageMetrics, log)
	if err != nil {
		return fmt.Errorf("failed to create batch writer: %w", err)
	}
	log.Info("batch writer created", zap.Int("batch_size", batchSize), zap.Duration("flush_interval", batchInterval))

	// Start drain worker (pulls from queue, writes to ClickHouse)
	go storage.DrainWorker(ctx, queue, batchWriter, log)
	log.Info("drain worker started")

	// Create auth validator
	authValidator := ingest.NewAuthValidator(pg, log)
	log.Info("auth validator created")

	// Create gRPC receiver
	grpcReceiver := ingest.NewGRPCReceiver(queue, ingestMetrics, log)

	// Create and start gRPC server
	grpcAddr := viper.GetString("server.grpc.addr")
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024),
		grpc.MaxSendMsgSize(10*1024*1024),
		grpc.StreamInterceptor(authValidator.GRPCStreamInterceptor()),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 15 * time.Minute,
			Time:              5 * time.Minute,
			Timeout:           1 * time.Minute,
		}),
	)
	v1.RegisterIngestServiceServer(grpcServer, grpcReceiver)

	grpcLn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC address %s: %w", grpcAddr, err)
	}
	log.Info("gRPC server listening", zap.String("addr", grpcAddr))

	go func() {
		if err := grpcServer.Serve(grpcLn); err != nil {
			log.Error("grpc server error", zap.Error(err))
		}
	}()

	// Create and start HTTP server (receivers and metrics only, NO query API)
	httpAddr := viper.GetString("server.http.addr")
	httpRouter := chi.NewRouter()

	// Create HTTP receiver (requires auth)
	httpReceiver := ingest.NewHTTPReceiver(queue, authValidator, ingestMetrics, log)
	httpReceiver.RegisterRoutes(httpRouter)

	// Create OTLP receiver (no auth required for OTLP)
	otlpReceiver := ingest.NewOTLPReceiver(queue, ingestMetrics, log)
	otlpReceiver.RegisterRoutes(httpRouter)

	// Prometheus metrics endpoint
	httpRouter.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// Health check endpoint
	httpRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Info("HTTP server listening", zap.String("addr", httpAddr))

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info("shutdown signal received, gracefully stopping")

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Graceful shutdown sequence
	httpServer.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()
	batchWriter.Close()
	queue.Close()

	log.Info("ingest subsystem stopped")
	return nil
}
