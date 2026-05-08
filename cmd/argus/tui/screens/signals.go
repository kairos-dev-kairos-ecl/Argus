// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings (compliant with contract):
//   /        — open filter
//   enter    — open trace for selected signal
//   p        — pause/resume live stream (screen-specific, no contract conflict)
//   r        — refresh signal list (compliant: r=refresh)
package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- messages ---

// SignalLoadedMsg carries the initial signal list from the REST API.
type SignalLoadedMsg struct {
	Signals []api.Signal
	Err     error
}

// SignalReceivedMsg is emitted by the WS client on each incoming signal.
type SignalReceivedMsg struct {
	Signal api.Signal
}

// SignalPollMsg triggers a polling refresh when WS is unavailable.
type SignalPollMsg struct{}

// SignalOpenTraceMsg is emitted when the user presses Enter on a signal row.
// The root AppModel handles this by switching to the Trace screen.
type SignalOpenTraceMsg struct {
	TraceID string
}

// maxSignalRows is the upper bound on in-memory signal rows (newest-first).
const maxSignalRows = 1000

// SignalsModel is the Bubbletea model for the Live Signal Feed screen.
//
// Layout:
//
//	┌─ Live Signals ──────────────────── 1,247 signals · WS connected ─┐
//	│ TIME      LAYER  SEV   TRACE        APP         CATEGORY          │
//	│ 10:42:13  [L7]   HI    abc123…      qwen-prod   prompt_injection  │
//	└──────────────────────────────────────────────────────────────────┘
//	 [/] filter  [Enter] open trace  [p] pause  [r] refresh  [?] help
type SignalsModel struct {
	apiClient  *api.Client
	wsClient   *api.WSClient
	signals    []api.Signal // newest first, capped at maxSignalRows
	table      components.Table
	filter     textinput.Model
	filterMode bool
	paused     bool
	wsLive     bool
	loading    bool
	spinner    spinner.Model
	err        string
	width      int
	height     int
}

// signalTableColumns defines the column widths for the signal table.
var signalTableColumns = []table.Column{
	{Title: "TIME", Width: 10},
	{Title: "LAYER", Width: 7},
	{Title: "SEV", Width: 7},
	{Title: "TRACE", Width: 12},
	{Title: "APP", Width: 14},
	{Title: "CATEGORY", Width: 22},
}

// NewSignalsModel creates the signals screen. apiClient may be nil in tests.
func NewSignalsModel(apiClient *api.Client) SignalsModel {
	f := textinput.New()
	f.Placeholder = "filter by app, layer, category…"
	f.CharLimit = 128

	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	t := components.NewTable(signalTableColumns, nil)

	return SignalsModel{
		apiClient: apiClient,
		table:     t,
		filter:    f,
		loading:   true,
		spinner:   sp,
	}
}

// Init loads the initial signal list and starts the spinner.
func (m SignalsModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchSignals(),
		m.spinner.Tick,
	)
}

// fetchSignals issues a GET /api/v1/signals?limit=100 returning a SignalLoadedMsg.
func (m SignalsModel) fetchSignals() tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg {
			return SignalLoadedMsg{Signals: nil}
		}
	}
	client := m.apiClient
	return func() tea.Msg {
		var signals []api.Signal
		if err := client.Get(context.Background(), "/api/v1/signals?limit=100&order=desc", &signals); err != nil {
			return SignalLoadedMsg{Err: err}
		}
		return SignalLoadedMsg{Signals: signals}
	}
}

// pollCmd schedules a polling tick after 2 seconds (used when WS is unavailable).
func pollCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return SignalPollMsg{}
	})
}

// Update handles messages.
func (m SignalsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateSignals(msg)
	return updated, cmd
}

// UpdateSignals is the type-safe update method used by AppModel and tests.
func (m SignalsModel) UpdateSignals(msg tea.Msg) (SignalsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = components.NewTable(signalTableColumns, m.buildRows())
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case SignalLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.signals = msg.Signals
		m.table = components.NewTable(signalTableColumns, m.buildRows())
		return m, nil

	case SignalReceivedMsg:
		if !m.paused {
			m.signals = appendSignal(m.signals, msg.Signal)
			m.table = components.NewTable(signalTableColumns, m.buildRows())
		}
		return m, nil

	case SignalPollMsg:
		if !m.wsLive {
			return m, tea.Batch(m.fetchSignals(), pollCmd())
		}
		return m, nil
	}

	// Delegate to filter input when active.
	if m.filterMode {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.table = components.NewTable(signalTableColumns, m.buildRows())
		return m, cmd
	}

	// Delegate to table.
	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m SignalsModel) handleKey(msg tea.KeyMsg) (SignalsModel, tea.Cmd) {
	if m.filterMode {
		switch msg.String() {
		case "esc", "enter":
			m.filterMode = false
			m.filter.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.table = components.NewTable(signalTableColumns, m.buildRows())
		return m, cmd
	}

	switch msg.String() {
	case "/":
		m.filterMode = true
		m.filter.Focus()
		return m, textinput.Blink

	case "enter":
		row := m.table.SelectedRow()
		if len(row) > 3 {
			traceID := row[3]
			if traceID != "" && traceID != "—" {
				return m, func() tea.Msg {
					return SignalOpenTraceMsg{TraceID: traceID}
				}
			}
		}
		return m, nil

	case "p":
		m.paused = !m.paused
		return m, nil

	case "r":
		m.loading = true
		m.err = ""
		return m, tea.Batch(m.fetchSignals(), m.spinner.Tick)
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

// appendSignal prepends s to signals and trims to maxSignalRows.
func appendSignal(signals []api.Signal, s api.Signal) []api.Signal {
	result := make([]api.Signal, 0, len(signals)+1)
	result = append(result, s)
	result = append(result, signals...)
	if len(result) > maxSignalRows {
		result = result[:maxSignalRows]
	}
	return result
}

// buildRows converts the filtered signal list to table rows.
func (m SignalsModel) buildRows() []table.Row {
	filterText := strings.ToLower(m.filter.Value())
	var rows []table.Row

	for _, s := range m.signals {
		if filterText != "" {
			haystack := strings.ToLower(s.AppID + " " + s.Category + fmt.Sprintf("l%d", s.Layer))
			if !strings.Contains(haystack, filterText) {
				continue
			}
		}

		ts := s.Timestamp.Format("15:04:05")
		layer := fmt.Sprintf("[L%d]", s.Layer)
		sev := abbreviateSeverity(s.Severity)
		traceShort := shortID(s.TraceID)
		appShort := truncStr(s.AppID, 14)
		cat := truncStr(s.Category, 22)

		rows = append(rows, table.Row{ts, layer, sev, traceShort, appShort, cat})
	}
	return rows
}

// View renders the signals screen.
func (m SignalsModel) View() string {
	var sb strings.Builder

	// Header info line.
	connStatus := "WS connected"
	if !m.wsLive {
		connStatus = "POLLING"
	}
	if m.paused {
		connStatus += " · PAUSED"
	}
	count := fmt.Sprintf("%d signals · %s", len(m.signals), connStatus)
	header := theme.Emphasis.Render("Live Signals") + "  " + theme.Muted.Render(count)
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString(m.spinner.View() + " loading…")
		return theme.Panel.Render(sb.String())
	}

	if m.err != "" {
		sb.WriteString(theme.ErrorText.Render("Error: " + m.err))
		sb.WriteString("\n")
		sb.WriteString(theme.Muted.Render("[r] retry"))
		return theme.Panel.Render(sb.String())
	}

	if m.filterMode || m.filter.Value() != "" {
		label := theme.Muted.Render("filter: ")
		sb.WriteString(label + m.filter.View())
		sb.WriteString("\n\n")
	}

	if len(m.signals) == 0 {
		sb.WriteString(theme.Subtle.Render("No signals — press [r] to refresh or wait for WS stream"))
		return theme.Panel.Render(sb.String())
	}

	sb.WriteString(m.table.View())
	sb.WriteString("\n")

	hints := "[/] filter  [enter] open trace  [p] pause  [r] refresh  [?] help"
	sb.WriteString(theme.Muted.Render(hints))

	return sb.String()
}

// Title returns the screen name.
func (m SignalsModel) Title() string { return "Signals" }

// KeyHelp returns local key bindings for the help overlay.
// Only screen-local bindings are returned; global bindings (q, ?, 1-6, tab, esc, ctrl+c)
// are rendered from keys/bindings.go and are NOT duplicated here.
func (m SignalsModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter signals")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open trace")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pause/resume stream")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh signals")),
	}
}

// --- helpers ---

func abbreviateSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return "CRIT"
	case "HIGH":
		return "HI"
	case "MEDIUM":
		return "MD"
	case "LOW":
		return "LO"
	default:
		if len(s) > 4 {
			return s[:4]
		}
		return s
	}
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
