package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HTTPReceiver implements HTTP handlers for signal ingestion.
type HTTPReceiver struct {
	queue   *Queue
	auth    *AuthValidator
	metrics *metrics.Ingest
	log     *zap.Logger
}

// NewHTTPReceiver creates a new HTTP ingest receiver.
func NewHTTPReceiver(queue *Queue, auth *AuthValidator, metrics *metrics.Ingest, log *zap.Logger) *HTTPReceiver {
	if log == nil {
		log = zap.NewNop()
	}
	return &HTTPReceiver{
		queue:   queue,
		auth:    auth,
		metrics: metrics,
		log:     log,
	}
}

// RegisterRoutes mounts HTTP receiver routes onto the chi mux.
// All routes are protected by auth middleware.
func (r *HTTPReceiver) RegisterRoutes(mux *chi.Mux) {
	// Wrap routes with auth middleware
	handler := r.auth.HTTPMiddleware()(http.HandlerFunc(r.handlePostSignals))
	mux.Post("/v1/signals", http.HandlerFunc(handler.ServeHTTP))
}

// handlePostSignals handles POST /v1/signals.
//
// Request body (Content-Type: application/json):
//   Single signal:  { "signal_id": "...", "layer": "L5_OUTPUT_DECODING", ... }
//   Array:          [{ "signal_id": "..." }, { "signal_id": "..." }]
//
// Response (201 Created):
//   { "accepted": 2, "rejected": 0 }
//
// Response (207 Multi-Status) if partial acceptance:
//   { "accepted": 1, "rejected": 1, "errors": [{"index": 1, "error": "queue full"}] }
//
// Response (429 Too Many Requests) if all signals rejected:
//   { "error": "ingest queue full", "retry_after": 1 }
//
// Body size limit: 4MB (chi middleware r.Body = http.MaxBytesReader(w, r.Body, 4<<20))
func (r *HTTPReceiver) handlePostSignals(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	startTime := time.Now()

	appID, ok := AppIDFromContext(ctx)
	if !ok {
		appID = "unknown"
	}

	// Enforce 4MB body size limit
	req.Body = http.MaxBytesReader(w, req.Body, 4<<20) // 4MB

	// Read request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			r.log.Warn("request body too large", zap.String("app_id", appID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge) // 413
			fmt.Fprintf(w, `{"error":"request body too large"}`)
			return
		}
		r.log.Error("failed to read request body", zap.String("app_id", appID), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"failed to read request body"}`)
		return
	}

	// Peek at first byte to detect array vs single object
	trimmed := bytes.TrimSpace(bodyBytes)
	if len(trimmed) == 0 {
		r.log.Warn("empty request body", zap.String("app_id", appID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"empty request body"}`)
		return
	}

	var signals []*v1.ArgusSignal
	var acceptedCount, rejectedCount int
	var errors []map[string]interface{}

	isBatch := trimmed[0] == '['

	if isBatch {
		// Parse as array
		var rawSignals []json.RawMessage
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		if err := decoder.Decode(&rawSignals); err != nil {
			r.log.Warn("malformed JSON array", zap.String("app_id", appID), zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid request body"}`)
			return
		}

		signals = make([]*v1.ArgusSignal, 0, len(rawSignals))
		for i, raw := range rawSignals {
			sig := &v1.ArgusSignal{}
			if err := protojson.Unmarshal(raw, sig); err != nil {
				r.log.Warn("failed to unmarshal signal", zap.Int("index", i), zap.String("app_id", appID), zap.Error(err))
				rejectedCount++
				errors = append(errors, map[string]interface{}{
					"index": i,
					"error": "invalid signal JSON",
				})
				continue
			}

			// Validate required fields
			if sig.SignalId == "" || sig.TraceId == "" {
				r.log.Warn("missing required fields", zap.Int("index", i), zap.String("app_id", appID))
				rejectedCount++
				errors = append(errors, map[string]interface{}{
					"index": i,
					"error": "missing required fields (signal_id, trace_id)",
				})
				continue
			}

			// Ensure ingested_at is set
			if sig.IngestedAt == nil {
				sig.IngestedAt = timestamppb.Now()
			}

			signals = append(signals, sig)
		}
	} else {
		// Parse as single object
		sig := &v1.ArgusSignal{}
		if err := protojson.Unmarshal(bodyBytes, sig); err != nil {
			r.log.Warn("malformed JSON object", zap.String("app_id", appID), zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid request body"}`)
			return
		}

		// Validate required fields
		if sig.SignalId == "" || sig.TraceId == "" {
			r.log.Warn("missing required fields", zap.String("app_id", appID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing required fields (signal_id, trace_id)"}`)
			return
		}

		// Ensure ingested_at is set
		if sig.IngestedAt == nil {
			sig.IngestedAt = timestamppb.Now()
		}

		signals = append(signals, sig)
	}

	// Enqueue signals
	for i, sig := range signals {
		err := r.queue.Enqueue(sig)
		if err != nil {
			if err == ErrQueueFull {
				r.log.Warn("queue full, signal rejected", zap.String("signal_id", sig.SignalId), zap.String("app_id", appID))
				rejectedCount++
				if isBatch {
					errors = append(errors, map[string]interface{}{
						"index": i,
						"error": "queue full",
					})
				}
			} else {
				r.log.Error("enqueue error", zap.String("signal_id", sig.SignalId), zap.String("app_id", appID), zap.Error(err))
				rejectedCount++
				if isBatch {
					errors = append(errors, map[string]interface{}{
						"index": i,
						"error": fmt.Sprintf("enqueue error: %v", err),
					})
				}
			}
		} else {
			acceptedCount++
			if r.metrics != nil && r.metrics.SignalsReceived != nil {
				r.metrics.SignalsReceived.WithLabelValues("http").Inc()
			}
		}
	}

	// Build response
	w.Header().Set("Content-Type", "application/json")

	// Determine response status
	totalSignals := len(signals)
	if acceptedCount == totalSignals {
		// All signals accepted
		w.WriteHeader(http.StatusCreated) // 201
		fmt.Fprintf(w, `{"accepted":%d,"rejected":%d}`, acceptedCount, rejectedCount)
	} else if rejectedCount == totalSignals {
		// All signals rejected
		w.WriteHeader(http.StatusTooManyRequests) // 429
		fmt.Fprintf(w, `{"error":"ingest queue full","retry_after":1}`)
	} else {
		// Partial acceptance
		w.WriteHeader(http.StatusMultiStatus) // 207
		respBody := map[string]interface{}{
			"accepted": acceptedCount,
			"rejected": rejectedCount,
		}
		if len(errors) > 0 {
			respBody["errors"] = errors
		}
		respJSON, _ := json.Marshal(respBody)
		w.Write(respJSON)
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if r.metrics != nil && r.metrics.RequestDuration != nil {
		status := "ok"
		if rejectedCount > 0 {
			status = "partial"
		}
		if rejectedCount == totalSignals && totalSignals > 0 {
			status = "error"
		}
		r.metrics.RequestDuration.WithLabelValues("http", status).Observe(duration)
	}

	r.log.Debug("http signal ingestion",
		zap.String("app_id", appID),
		zap.Int("accepted", acceptedCount),
		zap.Int("rejected", rejectedCount),
		zap.Float64("duration_seconds", duration),
	)
}
