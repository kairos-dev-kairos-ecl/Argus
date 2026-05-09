// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings:
//   a        — acknowledge selected alert
//   r        — resolve selected alert [DEVIATION: contract has r=refresh; alerts uses r=resolve
//              as the primary operator action. Capital-R refresh removed in Phase 4 to eliminate conflict.]
//   /        — open search filter
package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// AlertsLoadedMsg carries the alert list from the REST API.
type AlertsLoadedMsg struct {
	Alerts []api.Alert
	Err    error
}

// AlertActionResultMsg carries the result of an ack/resolve API call.
type AlertActionResultMsg struct {
	AlertID string
	Action  string // "ack" | "resolve"
	Err     error
}

// alertsAction is which destructive action is pending confirmation.
type alertsAction int

const (
	alertsActionNone    alertsAction = iota
	alertsActionAck                  // ack alert
	alertsActionResolve              // resolve alert
)

var alertTableColumns = []table.Column{
	{Title: "ID", Width: 12},
	{Title: "SEV", Width: 7},
	{Title: "CREATED", Width: 12},
	{Title: "APP", Width: 14},
	{Title: "SUMMARY", Width: 34},
	{Title: "STATUS", Width: 10},
}

// AlertsModel is the Bubbletea model for the Alerts Queue screen.
//
// Layout:
//
//	┌─ Alerts (12 open · 3 critical) ──────────────────────────────────┐
//	│ ID         SEV   CREATED       APP          SUMMARY     STATUS   │
//	│ alrt_001   CRIT  10:30:22      qwen-prod    …           open     │
//	└───────────────────────────────────────────────────────────────────┘
//	 [enter] details  [a] ack  [r] resolve  [/] search  [?] help
type AlertsModel struct {
	apiClient   *api.Client
	alerts      []api.Alert
	filtered    []api.Alert // after filter applied
	table       components.Table
	filter      textinput.Model
	filterMode  bool
	loading     bool
	spinner     spinner.Model
	err         string
	// Confirm modal state.
	confirmAction alertsAction
	confirmAlertID string
	confirmModal  components.Modal
}

// NewAlertsModel creates the alerts screen.
func NewAlertsModel(apiClient *api.Client) AlertsModel {
	f := textinput.New()
	f.Placeholder = "search alerts…"
	f.CharLimit = 128

	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	return AlertsModel{
		apiClient: apiClient,
		table:     components.NewTable(alertTableColumns, nil),
		filter:    f,
		loading:   true,
		spinner:   sp,
	}
}

// Init fetches the initial alert list.
func (m AlertsModel) Init() tea.Cmd {
	return tea.Batch(m.fetchAlerts(), m.spinner.Tick)
}

func (m AlertsModel) fetchAlerts() tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return AlertsLoadedMsg{} }
	}
	client := m.apiClient
	return func() tea.Msg {
		var resp struct {
			Alerts []api.Alert `json:"alerts"`
		}
		if err := client.Get(context.Background(), "/api/v1/alerts?limit=200&status=open", &resp); err != nil {
			return AlertsLoadedMsg{Err: err}
		}
		return AlertsLoadedMsg{Alerts: resp.Alerts}
	}
}

func (m AlertsModel) ackAlert(alertID string) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return AlertActionResultMsg{AlertID: alertID, Action: "ack"} }
	}
	client := m.apiClient
	return func() tea.Msg {
		err := client.Post(context.Background(), fmt.Sprintf("/api/v1/alerts/%s/ack", alertID), nil, nil)
		return AlertActionResultMsg{AlertID: alertID, Action: "ack", Err: err}
	}
}

func (m AlertsModel) resolveAlert(alertID string) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return AlertActionResultMsg{AlertID: alertID, Action: "resolve"} }
	}
	client := m.apiClient
	return func() tea.Msg {
		err := client.Post(context.Background(), fmt.Sprintf("/api/v1/alerts/%s/resolve", alertID), nil, nil)
		return AlertActionResultMsg{AlertID: alertID, Action: "resolve", Err: err}
	}
}

// Update implements tea.Model.
func (m AlertsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateAlerts(msg)
	return updated, cmd
}

// UpdateAlerts is the type-safe update method.
func (m AlertsModel) UpdateAlerts(msg tea.Msg) (AlertsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case AlertsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.alerts = msg.Alerts
		m.filtered = m.applyFilter()
		m.table = components.NewTable(alertTableColumns, m.buildRows())
		return m, nil

	case AlertActionResultMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		// Optimistic remove from list on ack/resolve.
		m.alerts = removeAlert(m.alerts, msg.AlertID)
		m.filtered = m.applyFilter()
		m.table = components.NewTable(alertTableColumns, m.buildRows())
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.filterMode {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.filtered = m.applyFilter()
		m.table = components.NewTable(alertTableColumns, m.buildRows())
		return m, cmd
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m AlertsModel) handleKey(msg tea.KeyMsg) (AlertsModel, tea.Cmd) {
	// Confirm modal active — intercept y/n.
	if m.confirmAction != alertsActionNone {
		switch msg.String() {
		case "y", "Y":
			action := m.confirmAction
			alertID := m.confirmAlertID
			m.confirmAction = alertsActionNone
			m.confirmAlertID = ""
			if action == alertsActionAck {
				return m, m.ackAlert(alertID)
			}
			return m, m.resolveAlert(alertID)
		default:
			m.confirmAction = alertsActionNone
			m.confirmAlertID = ""
			return m, nil
		}
	}

	if m.filterMode {
		switch msg.String() {
		case "esc", "enter":
			m.filterMode = false
			m.filter.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.filtered = m.applyFilter()
		m.table = components.NewTable(alertTableColumns, m.buildRows())
		return m, cmd
	}

	switch msg.String() {
	case "/":
		m.filterMode = true
		m.filter.Focus()
		return m, textinput.Blink

	case "a":
		id := m.selectedAlertID()
		if id != "" {
			m.confirmAction = alertsActionAck
			m.confirmAlertID = id
			m.confirmModal = components.NewModal(50, 7, "Acknowledge Alert",
				fmt.Sprintf("Acknowledge alert %s?\n\n[y] confirm  [any] cancel", shortID(id)))
		}
		return m, nil

	case "r":
		id := m.selectedAlertID()
		if id != "" {
			m.confirmAction = alertsActionResolve
			m.confirmAlertID = id
			m.confirmModal = components.NewModal(50, 7, "Resolve Alert",
				fmt.Sprintf("Resolve alert %s?\n\n[y] confirm  [any] cancel", shortID(id)))
		}
		return m, nil

	// Note: capital 'R' for refresh was removed in Phase 4 (key conflict with r=resolve).
	// Use the global 'r' key behaviour (documented in keys/bindings.go) or press '/' to filter.
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m AlertsModel) selectedAlertID() string {
	row := m.table.SelectedRow()
	if len(row) > 0 {
		return row[0]
	}
	return ""
}

func (m AlertsModel) applyFilter() []api.Alert {
	text := strings.ToLower(m.filter.Value())
	if text == "" {
		return m.alerts
	}
	var result []api.Alert
	for _, a := range m.alerts {
		if strings.Contains(strings.ToLower(a.Title+a.Category+a.Status), text) {
			result = append(result, a)
		}
	}
	return result
}

func (m AlertsModel) buildRows() []table.Row {
	var rows []table.Row
	for _, a := range m.filtered {
		sev := abbreviateSeverityInt(a.Severity)
		created := a.FirstSeenAt.Format("15:04:05")
		app := truncStr(a.AppID, 14)
		summary := truncStr(a.Title, 34)
		rows = append(rows, table.Row{a.ID, sev, created, app, summary, a.Status})
	}
	return rows
}

func removeAlert(alerts []api.Alert, id string) []api.Alert {
	result := make([]api.Alert, 0, len(alerts))
	for _, a := range alerts {
		if a.ID != id {
			result = append(result, a)
		}
	}
	return result
}

// criticalCount returns the number of critical alerts.
// Critical severity = 5 per proto Severity enum (SEVERITY_CRITICAL).
func criticalCount(alerts []api.Alert) int {
	count := 0
	for _, a := range alerts {
		if a.Severity == 5 {
			count++
		}
	}
	return count
}

// View renders the alerts screen.
func (m AlertsModel) View() string {
	var sb strings.Builder

	crit := criticalCount(m.alerts)
	header := theme.Emphasis.Render("Alerts") + "  " +
		theme.Muted.Render(fmt.Sprintf("%d open · %d critical", len(m.alerts), crit))
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString(m.spinner.View() + " loading…")
		return theme.Panel.Render(sb.String())
	}

	if m.err != "" {
		sb.WriteString(theme.ErrorText.Render("Error: " + m.err))
		sb.WriteString("\n")
		sb.WriteString(theme.Muted.Render("[R] retry"))
		return theme.Panel.Render(sb.String())
	}

	if m.filterMode || m.filter.Value() != "" {
		sb.WriteString(theme.Muted.Render("search: ") + m.filter.View())
		sb.WriteString("\n\n")
	}

	if len(m.filtered) == 0 {
		sb.WriteString(theme.Subtle.Render("No alerts — all clear"))
		return theme.Panel.Render(sb.String())
	}

	tableView := m.table.View()

	// Render confirm modal if active.
	if m.confirmAction != alertsActionNone {
		return m.confirmModal.Render(tableView)
	}

	sb.WriteString(tableView)
	sb.WriteString("\n")
	sb.WriteString(theme.Muted.Render("[a] ack  [r] resolve  [/] search  [?] help"))

	return sb.String()
}

// Title returns the screen name.
func (m AlertsModel) Title() string { return "Alerts" }

// KeyHelp returns local key bindings for the help overlay.
// Note: only screen-local bindings are listed here; global bindings (q, ?, 1-6, tab, esc, ctrl+c)
// are rendered from keys/bindings.go and are NOT duplicated here.
func (m AlertsModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "acknowledge alert")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resolve alert (deviation: r=resolve, not refresh)")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search filter")),
	}
}
