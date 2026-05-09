package screens

import (
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTraceModel_InitialState(t *testing.T) {
	m := NewTraceModel(nil)
	assert.Empty(t, m.traceID)
	assert.False(t, m.loading)
}

func TestTraceModel_SetTrace_StartsLoading(t *testing.T) {
	m := NewTraceModel(nil)
	updated, cmd := m.SetTrace("trace-abc")
	assert.True(t, updated.loading)
	assert.Equal(t, "trace-abc", updated.traceID)
	assert.NotNil(t, cmd)
}

func TestTraceModel_TraceLoadedMsg_BuildsTree(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	now := time.Now()

	msg := TraceLoadedMsg{
		TraceID: "trace-abc",
		Signals: []api.Signal{
			{ID: "s1", Layer: 1, Category: "gpu_util", Message: "high GPU usage", Timestamp: api.ProtoTime{now}},
			{ID: "s2", Layer: 7, Category: "prompt_injection", Message: "injection detected", Timestamp: api.ProtoTime{now}},
			{ID: "s3", Layer: 7, Category: "baseline_drift", Message: "drift detected", Timestamp: api.ProtoTime{now}},
		},
	}

	updated, _ := m.UpdateTrace(msg)
	assert.False(t, updated.loading)
	assert.Len(t, updated.signals, 3)
	// Tree should have 2 layer nodes + 3 signal nodes = 5 nodes.
	assert.Len(t, updated.nodes, 5)
}

func TestTraceModel_TraceLoadedMsg_Stale_Ignored(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-new"

	msg := TraceLoadedMsg{
		TraceID: "trace-old",
		Signals: []api.Signal{{ID: "s1", Layer: 1}},
	}

	updated, _ := m.UpdateTrace(msg)
	assert.Nil(t, updated.signals, "stale trace response should be ignored")
}

func TestTraceModel_TraceLoadedMsg_Error(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	msg := TraceLoadedMsg{TraceID: "trace-abc", Err: assert.AnError}
	updated, _ := m.UpdateTrace(msg)
	assert.False(t, updated.loading)
	assert.NotEmpty(t, updated.err)
}

func TestTraceModel_NavigationKeys(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	m.signals = []api.Signal{
		{ID: "s1", Layer: 1, Timestamp: api.ProtoTime{time.Now()}},
		{ID: "s2", Layer: 2, Timestamp: api.ProtoTime{time.Now()}},
	}
	m.nodes = buildTraceTree(m.signals)

	// Move down.
	updated, _ := m.UpdateTrace(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, updated.cursor)

	// Move up.
	updated2, _ := updated.UpdateTrace(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, updated2.cursor)
}

func TestTraceModel_ExpandCollapse(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	m.signals = []api.Signal{
		{ID: "s1", Layer: 1, Category: "hw", Timestamp: api.ProtoTime{time.Now()}},
	}
	m.nodes = buildTraceTree(m.signals)
	m.cursor = 0

	assert.False(t, m.nodes[0].expanded)

	updated, _ := m.UpdateTrace(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.nodes[0].expanded)

	updated2, _ := updated.UpdateTrace(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated2.nodes[0].expanded)
}

func TestTraceModel_BackKey_EmitsGoBack(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"

	_, cmd := m.UpdateTrace(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(TraceGoBackMsg)
	assert.True(t, ok, "b key should emit TraceGoBackMsg")
}

func TestTraceModel_View_NoTrace(t *testing.T) {
	m := NewTraceModel(nil)
	view := m.View()
	assert.Contains(t, view, "No trace selected")
}

func TestTraceModel_View_Loading(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	m.loading = true
	view := m.View()
	assert.Contains(t, view, "loading trace")
}

func TestTraceModel_View_Error(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	m.err = "not found"
	view := m.View()
	assert.Contains(t, view, "not found")
}

func TestTraceModel_View_ShowsSignals(t *testing.T) {
	m := NewTraceModel(nil)
	m.traceID = "trace-abc"
	m.signals = []api.Signal{
		{ID: "s1", Layer: 1, Category: "hw", Message: "high GPU", Timestamp: api.ProtoTime{time.Now()}},
	}
	m.nodes = buildTraceTree(m.signals)
	view := m.View()
	assert.Contains(t, view, "L1")
	assert.Contains(t, view, "1 signals")
}

func TestTraceModel_Title(t *testing.T) {
	m := NewTraceModel(nil)
	assert.Equal(t, "Trace", m.Title())
}

func TestTraceModel_KeyHelp_NonEmpty(t *testing.T) {
	m := NewTraceModel(nil)
	assert.NotEmpty(t, m.KeyHelp())
}

func TestBuildTraceTree_GroupsByLayer(t *testing.T) {
	signals := []api.Signal{
		{ID: "s1", Layer: 1, Category: "hw", Timestamp: api.ProtoTime{time.Now()}},
		{ID: "s2", Layer: 1, Category: "gpu", Timestamp: api.ProtoTime{time.Now()}},
		{ID: "s3", Layer: 7, Category: "infer", Timestamp: api.ProtoTime{time.Now()}},
	}

	nodes := buildTraceTree(signals)

	// Should have: 1 layer-1 heading + 2 signals + 1 layer-7 heading + 1 signal = 5 total.
	assert.Len(t, nodes, 5)
	assert.True(t, nodes[0].isLayer)
	assert.Equal(t, 1, nodes[0].layer)
	assert.False(t, nodes[0].expanded)
	assert.Equal(t, "s1", nodes[1].signalID)
	assert.Equal(t, "s2", nodes[2].signalID)
	assert.True(t, nodes[3].isLayer)
	assert.Equal(t, 7, nodes[3].layer)
}

func TestTraceModel_VisibleNodes_HidesWhenCollapsed(t *testing.T) {
	signals := []api.Signal{
		{ID: "s1", Layer: 1, Category: "hw", Timestamp: api.ProtoTime{time.Now()}},
		{ID: "s2", Layer: 1, Category: "gpu", Timestamp: api.ProtoTime{time.Now()}},
	}
	m := NewTraceModel(nil)
	m.nodes = buildTraceTree(signals)
	// Layer node is collapsed by default.
	visible := m.visibleNodes()
	// Only the layer heading should be visible (signals hidden when collapsed).
	assert.Len(t, visible, 1)
	assert.True(t, visible[0].isLayer)
}

func TestTraceModel_VisibleNodes_ShowsWhenExpanded(t *testing.T) {
	signals := []api.Signal{
		{ID: "s1", Layer: 1, Category: "hw", Timestamp: api.ProtoTime{time.Now()}},
	}
	m := NewTraceModel(nil)
	m.nodes = buildTraceTree(signals)
	m.nodes[0].expanded = true // expand the layer

	visible := m.visibleNodes()
	assert.Len(t, visible, 2) // heading + 1 signal
}
