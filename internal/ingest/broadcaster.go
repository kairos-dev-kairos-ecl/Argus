package ingest

import (
	"context"
	"sync"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

const subscriberBufferSize = 100 // per P2: per-client send buffer

// SignalBroadcaster fans out signals from the pipeline to all WebSocket subscribers.
// Per P2: slow subscribers get their oldest signal dropped (never block ingest).
type SignalBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *v1.ArgusSignal]struct{}
	publish     chan *v1.ArgusSignal
}

// NewSignalBroadcaster creates a new broadcaster. Call Run(ctx) in a goroutine.
func NewSignalBroadcaster() *SignalBroadcaster {
	return &SignalBroadcaster{
		subscribers: make(map[chan *v1.ArgusSignal]struct{}),
		publish:     make(chan *v1.ArgusSignal, 1000),
	}
}

// Run processes the publish channel, fanning out to all subscribers.
// Exits when ctx is cancelled.
func (b *SignalBroadcaster) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-b.publish:
			b.fan(sig)
		}
	}
}

// fan delivers sig to all current subscribers.
// If a subscriber's buffer is full, drop the oldest signal (non-blocking send).
func (b *SignalBroadcaster) fan(sig *v1.ArgusSignal) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- sig:
			// delivered
		default:
			// buffer full — drain one to make room (drop oldest), then try again
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- sig:
			default:
			}
		}
	}
}

// Publish sends a signal to all subscribers asynchronously.
// Never blocks — drops if publish channel is full.
func (b *SignalBroadcaster) Publish(sig *v1.ArgusSignal) {
	select {
	case b.publish <- sig:
	default:
		// publish buffer full — drop (P2: never block ingest hot path)
	}
}

// Subscribe creates a new per-client buffered channel.
func (b *SignalBroadcaster) Subscribe() chan *v1.ArgusSignal {
	ch := make(chan *v1.ArgusSignal, subscriberBufferSize)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *SignalBroadcaster) Unsubscribe(ch chan *v1.ArgusSignal) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

// SubscriberCount returns current subscriber count (for metrics).
func (b *SignalBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
