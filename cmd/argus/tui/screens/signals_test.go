package screens

import (
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalsModel_Init(t *testing.T) {
	m := NewSignalsModel(nil)
	cmd := m.Init()
	assert.NotNil(t, cmd, "Init should return a batch command")
}

func TestSignalsModel_InitialState(t *testing.T) {
	m := NewSignalsModel(nil)
	assert.True(t, m.loading, "should start in loading state")
	assert.False(t, m.paused)
	assert.False(t, m.filterMode)
}

func TestSignalsModel_SignalLoadedMsg_PopulatesTable(t *testing.T) {
	m := NewSignalsModel(nil)
	now := time.Now()

	msg := SignalLoadedMsg{
		Signals: []api.Signal{
			{ID: "s1", TraceID: "trace-abc123",
				Source:    api.SignalSource{AppID: "qwen-prod"},
				Layer:     7,
				Category:  "prompt_injection",
				Severity:  4,
				Timestamp: api.ProtoTime{now}},
		},
	}

	updated, _ := m.UpdateSignals(msg)
	assert.False(t, updated.loading)
	assert.Len(t, updated.signals, 1)
}

func TestSignalsModel_SignalLoadedMsg_Error(t *testing.T) {
	m := NewSignalsModel(nil)
	msg := SignalLoadedMsg{Err: assert.AnError}
	updated, _ := m.UpdateSignals(msg)
	assert.False(t, updated.loading)
	assert.NotEmpty(t, updated.err)
}

func TestSignalsModel_FilterKey_EntersFilterMode(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false

	updated, _ := m.UpdateSignals(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, updated.filterMode, "pressing / should enable filter mode")
}

func TestSignalsModel_PauseKey_TogglesPause(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false

	assert.False(t, m.paused)
	updated, _ := m.UpdateSignals(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.True(t, updated.paused)
	updated2, _ := updated.UpdateSignals(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.False(t, updated2.paused)
}

func TestSignalsModel_SignalReceived_AppendedWhenNotPaused(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	m.paused = false

	sig := api.Signal{ID: "s1", Layer: 3, Timestamp: api.ProtoTime{time.Now()}}
	updated, _ := m.UpdateSignals(SignalReceivedMsg{Signal: sig})
	assert.Len(t, updated.signals, 1)
}

func TestSignalsModel_SignalReceived_IgnoredWhenPaused(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	m.paused = true

	sig := api.Signal{ID: "s1", Layer: 3, Timestamp: api.ProtoTime{time.Now()}}
	updated, _ := m.UpdateSignals(SignalReceivedMsg{Signal: sig})
	assert.Empty(t, updated.signals)
}

func TestSignalsModel_MaxSignalRows(t *testing.T) {
	signals := make([]api.Signal, maxSignalRows)
	for i := range signals {
		signals[i] = api.Signal{ID: "s", Layer: 1, Timestamp: api.ProtoTime{time.Now()}}
	}
	newSig := api.Signal{ID: "new", Layer: 2, Timestamp: api.ProtoTime{time.Now()}}
	result := appendSignal(signals, newSig)
	assert.Len(t, result, maxSignalRows, "should cap at maxSignalRows")
	assert.Equal(t, "new", result[0].ID, "newest signal should be first")
}

func TestSignalsModel_EnterOnRow_NoTable_NoPanic(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	m.signals = []api.Signal{
		{ID: "s1", TraceID: "trace-abc123xyz", Layer: 7, Timestamp: api.ProtoTime{time.Now()}},
	}

	// Verify enter key dispatches without panic (table has no selected row in test).
	_, cmd := m.UpdateSignals(tea.KeyMsg{Type: tea.KeyEnter})
	_ = cmd
}

func TestSignalsModel_View_ShowsLoadingSpinner(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = true
	view := m.View()
	assert.Contains(t, view, "loading")
}

func TestSignalsModel_View_ShowsError(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	m.err = "connection refused"
	view := m.View()
	assert.Contains(t, view, "connection refused")
}

func TestSignalsModel_View_ShowsNoSignalsMsg(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	view := m.View()
	assert.Contains(t, view, "No signals")
}

func TestSignalsModel_Title(t *testing.T) {
	m := NewSignalsModel(nil)
	assert.Equal(t, "Signals", m.Title())
}

func TestSignalsModel_KeyHelp_NonEmpty(t *testing.T) {
	m := NewSignalsModel(nil)
	assert.NotEmpty(t, m.KeyHelp())
}

func TestAbbreviateSeverity(t *testing.T) {
	cases := map[string]string{
		"CRITICAL": "CRIT",
		"HIGH":     "HI",
		"MEDIUM":   "MD",
		"LOW":      "LO",
	}
	for input, want := range cases {
		assert.Equal(t, want, abbreviateSeverity(input))
	}
}

func TestShortID(t *testing.T) {
	assert.Equal(t, "abc", shortID("abc"))
	assert.Equal(t, "abcdefgh…", shortID("abcdefghij"))
}

func TestTruncStr(t *testing.T) {
	assert.Equal(t, "hello", truncStr("hello", 10))
	assert.Equal(t, "hell…", truncStr("hello world", 5))
}

func TestSignalsModel_RefreshKey_SetsLoading(t *testing.T) {
	m := NewSignalsModel(nil)
	m.loading = false
	m.err = "previous error"

	updated, cmd := m.UpdateSignals(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd, "refresh should return a cmd")
	assert.True(t, updated.loading)
	assert.Empty(t, updated.err)
}
