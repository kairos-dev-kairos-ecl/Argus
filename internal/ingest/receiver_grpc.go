package ingest

import (
	"fmt"
	"io"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCReceiver implements IngestService (generated from service.proto).
type GRPCReceiver struct {
	queue   *Queue
	metrics *metrics.Ingest
	log     *zap.Logger
	v1.UnimplementedIngestServiceServer
}

// NewGRPCReceiver creates a new gRPC ingest receiver.
func NewGRPCReceiver(queue *Queue, metrics *metrics.Ingest, log *zap.Logger) *GRPCReceiver {
	if log == nil {
		log = zap.NewNop()
	}
	return &GRPCReceiver{
		queue:   queue,
		metrics: metrics,
		log:     log,
	}
}

// StreamSignals implements IngestService.StreamSignals.
// Client sends a stream of ArgusSignal; server reads until EOF, enqueues each,
// then closes stream with IngestResponse.
//
// Flow:
//   1. Loop: stream.Recv() → *ArgusSignal (blocks until message arrives or EOF)
//   2. For each signal: call queue.Enqueue(signal)
//      - On success: increment accepted counter
//      - On queue full (ErrQueueFull): increment rejected counter, log at WARN
//   3. On EOF: return stream.SendAndClose(&IngestResponse{...}) with final counts
//   4. On other errors: return status.Errorf(codes.Internal, ...)
//
// Do NOT validate signal content here (that is the pipeline validator's job in Phase 3).
// Do NOT write to ClickHouse here (queue workers do that).
func (r *GRPCReceiver) StreamSignals(stream v1.IngestService_StreamSignalsServer) error {
	ctx := stream.Context()
	appID, ok := AppIDFromContext(ctx)
	if !ok {
		// This should not happen if auth interceptor is properly configured
		appID = "unknown"
	}

	batchID := uuid.New().String()
	startTime := time.Now()

	var acceptedCount int32 = 0
	var rejectedCount int32 = 0
	var rejections []*v1.RejectionDetail

	r.log.Info("stream started", zap.String("batch_id", batchID), zap.String("app_id", appID))

	// Read signals from stream until EOF
	for {
		signal, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Expected end of stream
				break
			}
			// Unexpected error during receive
			r.log.Error("stream receive error", zap.String("batch_id", batchID), zap.Error(err))
			return status.Errorf(codes.Internal, "failed to receive signal: %v", err)
		}

		// Validate signal has minimum required fields (signal_id, trace_id)
		if signal == nil {
			rejectedCount++
			rejections = append(rejections, &v1.RejectionDetail{
				SignalId: "unknown",
				Reason:   "signal is nil",
			})
			continue
		}

		if signal.SignalId == "" {
			rejectedCount++
			rejections = append(rejections, &v1.RejectionDetail{
				SignalId: "unknown",
				Reason:   "signal_id is empty",
			})
			continue
		}

		if signal.TraceId == "" {
			rejectedCount++
			rejections = append(rejections, &v1.RejectionDetail{
				SignalId: signal.SignalId,
				Reason:   "trace_id is empty",
			})
			continue
		}

		// Ensure signal has ingested_at timestamp
		if signal.IngestedAt == nil {
			signal.IngestedAt = timestampNow()
		}

		// Attempt to enqueue the signal
		err = r.queue.Enqueue(signal)
		if err != nil {
			if err == ErrQueueFull {
				r.log.Warn("queue full, signal rejected", zap.String("signal_id", signal.SignalId))
				rejectedCount++
				rejections = append(rejections, &v1.RejectionDetail{
					SignalId: signal.SignalId,
					Reason:   "ingest queue full",
				})
				// Return ResourceExhausted status to client
				return status.Errorf(codes.ResourceExhausted, "ingest queue full")
			}
			// Unexpected error during enqueue
			r.log.Error("enqueue error", zap.String("signal_id", signal.SignalId), zap.Error(err))
			rejectedCount++
			rejections = append(rejections, &v1.RejectionDetail{
				SignalId: signal.SignalId,
				Reason:   fmt.Sprintf("enqueue error: %v", err),
			})
			continue
		}

		acceptedCount++
		if r.metrics != nil && r.metrics.SignalsReceived != nil {
			r.metrics.SignalsReceived.WithLabelValues("grpc").Inc()
		}
	}

	// Build response
	response := &v1.IngestResponse{
		BatchId:      batchID,
		AcceptedCount: acceptedCount,
		RejectedCount: rejectedCount,
		Rejections:   rejections,
	}

	// Record request duration metric
	duration := time.Since(startTime).Seconds()
	if r.metrics != nil && r.metrics.RequestDuration != nil {
		status := "ok"
		if rejectedCount > 0 {
			status = "partial"
		}
		r.metrics.RequestDuration.WithLabelValues("grpc", status).Observe(duration)
	}

	r.log.Info("stream complete",
		zap.String("batch_id", batchID),
		zap.Int32("accepted", acceptedCount),
		zap.Int32("rejected", rejectedCount),
		zap.Float64("duration_seconds", duration),
	)

	// Send response and close stream
	return stream.SendAndClose(response)
}

// timestampNow returns the current time as a protobuf Timestamp.
func timestampNow() *timestamppb.Timestamp {
	return timestamppb.Now()
}

// Compile-time check to ensure GRPCReceiver implements IngestServiceServer.
var _ v1.IngestServiceServer = (*GRPCReceiver)(nil)
