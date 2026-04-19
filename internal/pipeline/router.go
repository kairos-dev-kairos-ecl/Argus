// Package pipeline - router.go
//
// Deprecated: Router has been removed from the pipeline chain.
// WorkerPool is the sole writer for processed signals (prevents double-write).
// This file is retained as a tombstone. Do not add back Router to the chain.
//
// History: Router was originally a final-stage processor that wrote signals to
// storage. This caused a double-write bug: WorkerPool also calls storage.Write
// after the chain completes. Resolution (Phase 04, Plan 01): Router removed,
// WorkerPool.processSignal is the single writer.
package pipeline

import (
	"context"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Writer is an interface for persisting signals to storage.
// Implemented by storage backends (ClickHouse, etc.).
type Writer interface {
	// Write persists a signal to storage.
	// Returns nil on success, error on failure.
	Write(ctx context.Context, sig *v1.ArgusSignal) error
}
