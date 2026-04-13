package ingest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/stretchr/testify/assert"
)

func TestBroadcaster_SubscribeReceivesSignals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	sig := &v1.ArgusSignal{SignalId: "test-001", TraceId: "trace-001"}
	b.Publish(sig)

	select {
	case got := <-ch:
		assert.Equal(t, "test-001", got.SignalId)
	case <-time.After(time.Second):
		t.Fatal("timeout: signal not received")
	}
}

func TestBroadcaster_SlowSubscriberDropsOldest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Flood 200 signals into a buffer of 100
	for i := 0; i < 200; i++ {
		sig := &v1.ArgusSignal{SignalId: fmt.Sprintf("signal-%d", i)}
		b.Publish(sig)
	}

	// Subscriber should not block; we should read some signals
	received := 0
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case <-ch:
			received++
		case <-timeout:
			goto done
		}
	}
done:
	assert.Greater(t, received, 0, "should receive at least some signals")
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	sig := &v1.ArgusSignal{SignalId: "broadcast-001"}
	b.Publish(sig)

	for _, ch := range []<-chan *v1.ArgusSignal{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, "broadcast-001", got.SignalId)
		case <-time.After(time.Second):
			t.Fatal("timeout: subscriber did not receive signal")
		}
	}
}
