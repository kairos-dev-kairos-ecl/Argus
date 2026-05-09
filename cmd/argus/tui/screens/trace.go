// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings:
//   up/k     — move cursor up (navigation, no contract conflict)
//   down/j   — move cursor down
//   enter/space — expand/collapse layer node (compliant: enter=open/confirm)
//   b        — back to signals [DEVIATION: Esc also works via global contract; b is an alias for familiarity]
//   r        — reload trace (compliant: r=refresh)
package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// TraceLoadedMsg carries the signals for a trace ID.
type TraceLoadedMsg struct {
	TraceID string
	Signals []api.Signal
	Err     error
}

// TraceGoBackMsg is emitted when the user presses [b] to return to signals.
type TraceGoBackMsg struct{}

// traceNode is one entry in the flat renderable tree.
type traceNode struct {
	layer     int
	category  string
	signalID  string
	message   string
	anomaly   float64
	isLayer   bool // true = layer heading row
	isCategory bool // true = category sub-heading
	expanded  bool
}

// TraceModel is the Bubbletea model for the Trace Investigation screen.
//
// Layout:
//
//	┌─ Trace abc12345  ─ 14 signals across L1-L10 ───────────────────┐
//	│ ▼ [L1] Hardware           2 signals                            │
//	│     gpu_id=A100-3, util=87%                                    │
//	│ ▶ [L2] Network            3 signals                            │
//	│ ▼ [L7] Inference Layer    5 signals                            │
//	└─────────────────────────────────────────────────────────────────┘
//	 [↑↓/j/k] navigate  [enter/space] expand  [b] back  [?] help
type TraceModel struct {
	apiClient *api.Client
	traceID   string
	signals   []api.Signal
	nodes     []traceNode // flat renderable list
	cursor    int         // index into nodes[]
	loading   bool
	spinner   spinner.Model
	err       string
}

// NewTraceModel creates the trace screen. apiClient may be nil in tests.
func NewTraceModel(apiClient *api.Client) TraceModel {
	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	return TraceModel{
		apiClient: apiClient,
		loading:   false,
		spinner:   sp,
	}
}

// SetTrace sets the active trace ID and loads its signals.
func (m TraceModel) SetTrace(traceID string) (TraceModel, tea.Cmd) {
	m.traceID = traceID
	m.signals = nil
	m.nodes = nil
	m.cursor = 0
	m.loading = true
	m.err = ""
	return m, tea.Batch(m.fetchTrace(traceID), m.spinner.Tick)
}

func (m TraceModel) fetchTrace(traceID string) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg {
			return TraceLoadedMsg{TraceID: traceID, Signals: nil}
		}
	}
	client := m.apiClient
	return func() tea.Msg {
		// GET /api/v1/traces/{traceId} returns TraceResponse{spans:[SpanView]}
		// SpanView.layer is the proto enum string ("L1_HARDWARE" etc.), not int.
		path := fmt.Sprintf("/api/v1/traces/%s", traceID)
		var tr api.TraceResponse
		if err := client.Get(context.Background(), path, &tr); err != nil {
			return TraceLoadedMsg{TraceID: traceID, Err: err}
		}
		// Convert spans → []api.Signal so buildTraceTree can consume them.
		signals := make([]api.Signal, 0, len(tr.Spans))
		for _, sp := range tr.Spans {
			signals = append(signals, api.Signal{
				ID:       sp.SignalID,
				TraceID:  tr.TraceID,
				Layer:    api.SpanLayerInt[sp.Layer], // "" → 0 for unknowns
				Category: sp.Message,
			})
		}
		return TraceLoadedMsg{TraceID: traceID, Signals: signals}
	}
}

// Init implements tea.Model. Nothing to do until SetTrace is called.
func (m TraceModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m TraceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateTrace(msg)
	return updated, cmd
}

// UpdateTrace is the type-safe update method.
func (m TraceModel) UpdateTrace(msg tea.Msg) (TraceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case TraceLoadedMsg:
		if msg.TraceID != m.traceID {
			return m, nil // stale response
		}
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.signals = msg.Signals
		m.nodes = buildTraceTree(msg.Signals)
		m.cursor = 0
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m TraceModel) handleKey(msg tea.KeyMsg) (TraceModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.nodes)-1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor < len(m.nodes) {
			m.nodes[m.cursor].expanded = !m.nodes[m.cursor].expanded
		}
	case "b", "esc":
		return m, func() tea.Msg { return TraceGoBackMsg{} }
	case "r":
		if m.traceID != "" {
			return m.SetTrace(m.traceID)
		}
	}
	return m, nil
}

// buildTraceTree converts a flat signal list into an expandable tree.
// Structure: layer nodes (collapsed by default) → signals under each layer.
func buildTraceTree(signals []api.Signal) []traceNode {
	// Group signals by layer.
	type layerGroup struct {
		layer   int
		signals []api.Signal
	}
	seen := map[int]bool{}
	order := []int{}
	byLayer := map[int][]api.Signal{}

	for _, s := range signals {
		if !seen[s.Layer] {
			seen[s.Layer] = true
			order = append(order, s.Layer)
		}
		byLayer[s.Layer] = append(byLayer[s.Layer], s)
	}

	var nodes []traceNode
	for _, layer := range order {
		// Layer heading node (collapsed by default).
		nodes = append(nodes, traceNode{
			layer:    layer,
			isLayer:  true,
			expanded: false,
		})
		// Signal detail nodes (only rendered when parent is expanded).
		for _, s := range byLayer[layer] {
			nodes = append(nodes, traceNode{
				layer:    layer,
				signalID: s.ID,
				message:  s.Category, // Category doubles as message (SpanView.Message = category)
				anomaly:  s.AnomalyScore(),
				category: s.Category,
			})
		}
	}
	return nodes
}

// visibleNodes returns the nodes that should be rendered (respecting expansion).
func (m TraceModel) visibleNodes() []traceNode {
	var visible []traceNode
	var currentLayerExpanded bool

	for _, n := range m.nodes {
		if n.isLayer {
			currentLayerExpanded = n.expanded
			visible = append(visible, n)
		} else {
			// Signal detail — only show if its layer heading is expanded.
			if currentLayerExpanded {
				visible = append(visible, n)
			}
		}
	}
	return visible
}

// View renders the trace screen.
func (m TraceModel) View() string {
	var sb strings.Builder

	if m.traceID == "" {
		sb.WriteString(theme.Subtle.Render("No trace selected — press [1] to return to signals"))
		return theme.Panel.Render(sb.String())
	}

	header := theme.Emphasis.Render("Trace") + "  " + theme.Muted.Render(shortID(m.traceID))
	if len(m.signals) > 0 {
		header += "  " + theme.Muted.Render(fmt.Sprintf("%d signals", len(m.signals)))
	}
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString(m.spinner.View() + " loading trace…")
		return theme.Panel.Render(sb.String())
	}

	if m.err != "" {
		sb.WriteString(theme.ErrorText.Render("Error: " + m.err))
		sb.WriteString("\n")
		sb.WriteString(theme.Muted.Render("[r] retry  [b] back"))
		return theme.Panel.Render(sb.String())
	}

	if len(m.signals) == 0 {
		sb.WriteString(theme.Subtle.Render("No signals found for this trace"))
		sb.WriteString("\n")
		sb.WriteString(theme.Muted.Render("[b] back"))
		return theme.Panel.Render(sb.String())
	}

	// Render tree.
	visible := m.visibleNodes()
	// Map visible cursor back to actual cursor.
	visibleCursor := m.cursorInVisible(visible)

	for i, n := range visible {
		selected := i == visibleCursor
		line := m.renderNode(n, selected)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(theme.Muted.Render("[↑↓/j/k] navigate  [enter/space] expand  [b] back  [?] help"))

	return theme.Panel.Render(sb.String())
}

// renderNode renders a single tree node line.
func (m TraceModel) renderNode(n traceNode, selected bool) string {
	var sb strings.Builder

	if n.isLayer {
		arrow := "▶"
		if n.expanded {
			arrow = "▼"
		}
		layerStyle := theme.LayerBadge(n.layer)
		layerLabel := layerStyle.Render(fmt.Sprintf("[L%d]", n.layer))
		count := len(m.signalsForLayer(n.layer))

		line := fmt.Sprintf(" %s %s  %d signals", arrow, layerLabel, count)
		if selected {
			line = theme.Emphasis.Render(line)
		} else {
			line = theme.Subtle.Render(" "+arrow) + " " + layerLabel + theme.Subtle.Render(fmt.Sprintf("  %d signals", count))
		}
		sb.WriteString(line)
	} else {
		// Signal detail line.
		sev := abbreviateSeverity(n.category)
		anomaly := ""
		if n.anomaly > 0 {
			anomaly = fmt.Sprintf(" score=%.2f", n.anomaly)
		}
		msg := truncStr(n.message, 60)
		line := fmt.Sprintf("     [%s]%s  %s", sev, anomaly, msg)
		if selected {
			line = theme.Emphasis.Render(line)
		} else {
			line = theme.Muted.Render(line)
		}
		sb.WriteString(line)
	}

	return sb.String()
}

// signalsForLayer returns all signals in the given layer.
func (m TraceModel) signalsForLayer(layer int) []api.Signal {
	var result []api.Signal
	for _, s := range m.signals {
		if s.Layer == layer {
			result = append(result, s)
		}
	}
	return result
}

// cursorInVisible maps m.cursor (index into m.nodes) to an index in visible.
func (m TraceModel) cursorInVisible(visible []traceNode) int {
	if m.cursor >= len(m.nodes) {
		return len(visible) - 1
	}
	target := m.nodes[m.cursor]
	for i, v := range visible {
		if v.layer == target.layer && v.isLayer == target.isLayer &&
			v.signalID == target.signalID {
			return i
		}
	}
	return 0
}

// Title returns the screen name.
func (m TraceModel) Title() string { return "Trace" }

// KeyHelp returns local key bindings.
func (m TraceModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter/space", "expand layer")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back to signals")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload trace")),
	}
}
