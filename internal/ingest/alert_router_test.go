package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockRoutingEngine struct {
	targets []string
}

func (m *mockRoutingEngine) Evaluate(_ *notify.EvaluationContext) []string {
	return m.targets
}

type captureDispatcher struct {
	jobs []*notify.DispatchJob
}

func (c *captureDispatcher) Dispatch(job *notify.DispatchJob) error {
	c.jobs = append(c.jobs, job)
	return nil
}

func TestAlertRouterFingerprint(t *testing.T) {
	ruleID := "t1-001"
	appID := "app-abc"
	expected := func() string {
		h := sha256.Sum256([]byte(ruleID + ":" + appID))
		return fmt.Sprintf("%x", h[:])
	}()
	assert.Equal(t, expected, computeRouterFingerprint(ruleID, appID))
}

func TestAlertRouterWriteAlertNilPoolNoop(t *testing.T) {
	router := &AlertRouter{
		pool:       nil,
		dispatcher: &captureDispatcher{},
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-001", Severity: 3, Action: engine.Action{Title: "test"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-1",
			Source:   &v1.Source{AppId: "app-abc"},
			Layer:    v1.Layer_L10_APPLICATION,
			Category: "anomaly",
		},
	}

	err := router.WriteAlert(context.Background(), match)
	assert.NoError(t, err)
}

func TestAlertRouterWriteAlertDispatches(t *testing.T) {
	cap := &captureDispatcher{}
	router := &AlertRouter{
		pool:       nil,
		dispatcher: cap,
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-002", Severity: 4, Action: engine.Action{Title: "alert", Description: "desc"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-2",
			TraceId:  "trace-1",
			Source:   &v1.Source{AppId: "app-1"},
			Layer:    v1.Layer_L6_SAFETY,
			Category: "safety.classifier",
		},
	}

	err := router.WriteAlert(context.Background(), match)
	require.NoError(t, err)
	require.Len(t, cap.jobs, 1)
	assert.Equal(t, []string{"log"}, cap.jobs[0].Targets)
	assert.Equal(t, "alert", cap.jobs[0].Notification.Title)
	assert.NotEqual(t, uuid.Nil, cap.jobs[0].AlertID)
}
