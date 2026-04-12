package pipeline

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Processor is the interface that all pipeline stages must implement.
// Each processor receives a signal, performs its operation, and returns the modified signal.
// If a processor encounters an error or wishes to skip a signal, it returns (nil, error).
// Processors must not modify the input signal; if changes are needed, return a new instance.
type Processor interface {
	// Process applies the processor's logic to a signal.
	// Returns the (possibly modified) signal and nil on success.
	// Returns (nil, error) if validation/processing fails; the signal is skipped.
	// Returning (nil, nil) is treated as a normal skip (e.g., for filtering).
	Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error)

	// Name returns the processor's name for logging and metrics.
	Name() string
}

// ErrPipelineFull is returned by Chain.Enqueue when the input channel is at capacity.
var ErrPipelineFull = errors.New("pipeline input channel full")

// ErrChainShutdown is returned when attempting to enqueue after shutdown.
var ErrChainShutdown = errors.New("pipeline chain is shutdown")

// Chain orchestrates a sequence of Processors, feeding signals through them serially.
// It manages buffered channels between the caller and the first processor, and from the last
// processor to the caller's output channel. Backpressure naturally propagates: if downstream
// processors are slow, channels fill and block upstream processing.
//
// The Chain is not thread-safe by design; it is meant to be created once and used for the
// lifetime of the application. Start() must be called exactly once before Enqueue().
type Chain struct {
	processors []Processor
	inputCh   chan *v1.ArgusSignal
	outputCh  chan *v1.ArgusSignal
	logger    *zap.Logger
	done      chan struct{} // closed to signal shutdown
	shutdown  bool          // tracks if Shutdown has been called
}

// NewChain creates a new processor chain with the given sequence of processors.
// Buffer size defaults to 1000 signals per channel (tunable via config).
// Processors execute in order; output of one becomes input to the next.
func NewChain(processors ...Processor) *Chain {
	return NewChainWithBufferSize(1000, processors...)
}

// NewChainWithBufferSize creates a new processor chain with a custom buffer size.
func NewChainWithBufferSize(bufferSize int, processors ...Processor) *Chain {
	if len(processors) == 0 {
		// Allow empty chain (will just pass signals through)
	}
	return &Chain{
		processors: processors,
		inputCh:   make(chan *v1.ArgusSignal, bufferSize),
		outputCh:  make(chan *v1.ArgusSignal, bufferSize),
		logger:    zap.L(),
		done:      make(chan struct{}),
		shutdown:  false,
	}
}

// SetLogger sets the logger for the chain (optional; defaults to zap.L()).
func (c *Chain) SetLogger(logger *zap.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

// Start launches the main processing goroutine.
// This goroutine reads signals from inputCh, feeds them through each processor in order,
// and sends results (or skipped signals) to outputCh.
// Start must be called exactly once and should be called before Enqueue.
// The goroutine exits when inputCh is closed or ctx is cancelled.
func (c *Chain) Start(ctx context.Context) {
	go func() {
		defer close(c.outputCh)
		for {
			select {
			case sig, ok := <-c.inputCh:
				if !ok {
					// inputCh closed, drain and exit
					return
				}
				c.processSignal(ctx, sig)
			case <-ctx.Done():
				// Context cancelled, exit
				return
			case <-c.done:
				// Explicit shutdown signal, drain remaining signals then exit
				for sig := range c.inputCh {
					c.processSignal(ctx, sig)
				}
				return
			}
		}
	}()
}

// processSignal feeds a single signal through the entire processor chain.
// If any processor returns an error or filters the signal, it is logged and not forwarded.
// If the signal makes it through all processors, it is sent to outputCh.
func (c *Chain) processSignal(ctx context.Context, sig *v1.ArgusSignal) {
	if sig == nil {
		return // Skip nil signals
	}

	current := sig
	var err error

	// Feed through each processor in order
	for _, proc := range c.processors {
		if current == nil {
			// Processor filtered the signal (returned nil, nil), skip rest
			break
		}

		current, err = proc.Process(ctx, current)
		if err != nil {
			// Processor returned an error; log and skip this signal
			c.logger.Error("processor failed",
				zap.String("signal_id", sig.SignalId),
				zap.String("processor", proc.Name()),
				zap.Error(err),
			)
			current = nil
			break
		}
	}

	// If signal survived the chain, send it to output
	if current != nil {
		select {
		case c.outputCh <- current:
			// Successfully enqueued to output
		case <-ctx.Done():
			// Context cancelled while trying to send
		case <-c.done:
			// Shutdown in progress
		}
	}
}

// Enqueue attempts to add a signal to the pipeline for processing.
// Non-blocking: returns immediately with an error if the input channel is full.
// Returns ErrPipelineFull if the buffer is at capacity.
// Returns ErrChainShutdown if Shutdown() has been called.
func (c *Chain) Enqueue(sig *v1.ArgusSignal) error {
	if c.shutdown {
		return ErrChainShutdown
	}

	if sig == nil {
		return errors.New("signal cannot be nil")
	}

	select {
	case c.inputCh <- sig:
		return nil
	default:
		return ErrPipelineFull
	}
}

// Results returns a read-only channel for receiving processed signals.
// The channel is closed when Start() completes or Shutdown() is called.
func (c *Chain) Results() <-chan *v1.ArgusSignal {
	return c.outputCh
}

// InputDepth returns the current number of signals waiting in the input buffer.
// Useful for monitoring backpressure.
func (c *Chain) InputDepth() int {
	return len(c.inputCh)
}

// OutputDepth returns the current number of signals waiting in the output buffer.
func (c *Chain) OutputDepth() int {
	return len(c.outputCh)
}

// Shutdown gracefully closes the pipeline.
// Signals shutdown intent, closes the input channel, and waits for pending signals to drain.
// Subsequent calls to Enqueue will return ErrChainShutdown.
func (c *Chain) Shutdown(ctx context.Context) error {
	c.shutdown = true
	close(c.done)
	// Close inputCh to signal no more signals will be added
	close(c.inputCh)

	// Wait for outputCh to close (indicating all signals have been processed)
	// or ctx to expire (timeout shutdown)
	select {
	case <-c.outputCh:
		// outputCh was closed by the processing goroutine
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	}
}
