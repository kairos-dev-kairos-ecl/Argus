// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings (admin-only):
//   u        — activate user email filter input [DEVIATION: subordinate to /; 'u' is screen-specific shortcut]
//   a        — activate action filter input [DEVIATION: subordinate to /; 'a' is screen-specific shortcut]
//   /        — toggle between user filter input and no-filter (compliant: /=filter)
//   r        — refresh audit log (compliant: r=refresh)
//   up/down  — scroll viewport (navigation, no contract conflict)
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
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// AuditLoadedMsg carries audit log entries.
type AuditLoadedMsg struct {
	Entries []api.AuditEntry
	Err     error
}

// auditFilterField identifies which filter input is active.
type auditFilterField int

const (
	auditFilterNone   auditFilterField = iota
	auditFilterUser                    // filter by user email
	auditFilterAction                  // filter by action string
)

// AuditModel is the Bubbletea model for the Audit Log screen.
//
// Layout:
//
//	┌─ Audit Log ──────────────────────────────────────────────────────┐
//	│ filter user: [___]  action: [___]                               │
//	│ ─────────────────────────────────────────────────────────────── │
//	│ 10:42:01  admin@argus.io   login_success         ip=10.0.0.5   │
//	│ 10:38:14  alice@argus.io   rule_updated          rule_id=r_…   │
//	└───────────────────────────────────────────────────────────────────┘
//	 [↑↓] scroll  [/u] filter user  [/a] filter action  [r] refresh
//
// Admin-only: non-admin users see "ACCESS DENIED".
type AuditModel struct {
	apiClient    *api.Client
	currentRole  string
	entries      []api.AuditEntry
	filtered     []api.AuditEntry
	viewport     viewport.Model
	userFilter   textinput.Model
	actionFilter textinput.Model
	activeFilter auditFilterField
	loading      bool
	spinner      spinner.Model
	err          string
	width        int
	height       int
}

// NewAuditModel creates the audit log screen.
func NewAuditModel(apiClient *api.Client, currentRole string) AuditModel {
	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	uf := textinput.New()
	uf.Placeholder = "user@example.com"
	uf.CharLimit = 128
	uf.Width = 24

	af := textinput.New()
	af.Placeholder = "login_success…"
	af.CharLimit = 64
	af.Width = 20

	vp := viewport.New(80, 30)

	return AuditModel{
		apiClient:    apiClient,
		currentRole:  currentRole,
		viewport:     vp,
		userFilter:   uf,
		actionFilter: af,
		loading:      true,
		spinner:      sp,
	}
}

func (m AuditModel) isAdmin() bool {
	return strings.ToLower(m.currentRole) == "admin"
}

// Init fetches audit entries if admin.
func (m AuditModel) Init() tea.Cmd {
	if !m.isAdmin() {
		return nil
	}
	return tea.Batch(m.fetchAudit(), m.spinner.Tick)
}

func (m AuditModel) fetchAudit() tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return AuditLoadedMsg{} }
	}
	client := m.apiClient
	userFilter := m.userFilter.Value()
	actionFilter := m.actionFilter.Value()
	return func() tea.Msg {
		path := "/api/v1/audit?limit=500"
		if userFilter != "" {
			path += "&user=" + userFilter
		}
		if actionFilter != "" {
			path += "&action=" + actionFilter
		}
		var entries []api.AuditEntry
		if err := client.Get(context.Background(), path, &entries); err != nil {
			return AuditLoadedMsg{Err: err}
		}
		return AuditLoadedMsg{Entries: entries}
	}
}

// Update implements tea.Model.
func (m AuditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateAudit(msg)
	return updated, cmd
}

// UpdateAudit is the type-safe update method.
func (m AuditModel) UpdateAudit(msg tea.Msg) (AuditModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width-4, msg.Height-10)
		if len(m.filtered) > 0 {
			m.viewport.SetContent(m.renderEntries())
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case AuditLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.entries = msg.Entries
		m.filtered = m.applyFilters()
		m.viewport.SetContent(m.renderEntries())
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to active filter input.
	if m.activeFilter == auditFilterUser {
		var cmd tea.Cmd
		m.userFilter, cmd = m.userFilter.Update(msg)
		m.filtered = m.applyFilters()
		m.viewport.SetContent(m.renderEntries())
		return m, cmd
	}
	if m.activeFilter == auditFilterAction {
		var cmd tea.Cmd
		m.actionFilter, cmd = m.actionFilter.Update(msg)
		m.filtered = m.applyFilters()
		m.viewport.SetContent(m.renderEntries())
		return m, cmd
	}

	// Delegate to viewport for scroll.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m AuditModel) handleKey(msg tea.KeyMsg) (AuditModel, tea.Cmd) {
	// If filter input is active, esc or enter exits it.
	if m.activeFilter != auditFilterNone {
		switch msg.String() {
		case "esc", "enter":
			m.activeFilter = auditFilterNone
			m.userFilter.Blur()
			m.actionFilter.Blur()
			return m, nil
		}
		if m.activeFilter == auditFilterUser {
			var cmd tea.Cmd
			m.userFilter, cmd = m.userFilter.Update(msg)
			m.filtered = m.applyFilters()
			m.viewport.SetContent(m.renderEntries())
			return m, cmd
		}
		var cmd tea.Cmd
		m.actionFilter, cmd = m.actionFilter.Update(msg)
		m.filtered = m.applyFilters()
		m.viewport.SetContent(m.renderEntries())
		return m, cmd
	}

	switch msg.String() {
	case "u":
		m.activeFilter = auditFilterUser
		m.userFilter.Focus()
		m.actionFilter.Blur()
		return m, textinput.Blink

	case "a":
		m.activeFilter = auditFilterAction
		m.actionFilter.Focus()
		m.userFilter.Blur()
		return m, textinput.Blink

	case "/":
		// Toggle between user/action filter.
		if m.activeFilter == auditFilterNone {
			m.activeFilter = auditFilterUser
			m.userFilter.Focus()
			return m, textinput.Blink
		}
		m.activeFilter = auditFilterNone
		m.userFilter.Blur()
		m.actionFilter.Blur()
		return m, nil

	case "r":
		m.loading = true
		m.err = ""
		return m, tea.Batch(m.fetchAudit(), m.spinner.Tick)
	}

	// Delegate to viewport.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m AuditModel) applyFilters() []api.AuditEntry {
	userText := strings.ToLower(m.userFilter.Value())
	actionText := strings.ToLower(m.actionFilter.Value())

	if userText == "" && actionText == "" {
		return m.entries
	}

	var result []api.AuditEntry
	for _, e := range m.entries {
		if userText != "" && !strings.Contains(strings.ToLower(e.UserEmail), userText) {
			continue
		}
		if actionText != "" && !strings.Contains(strings.ToLower(e.Action), actionText) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (m AuditModel) renderEntries() string {
	if len(m.filtered) == 0 {
		return theme.Subtle.Render("No audit entries match the current filter")
	}

	var sb strings.Builder
	for _, e := range m.filtered {
		ts := e.CreatedAt.Format("15:04:05")
		user := truncStr(e.UserEmail, 22)
		action := truncStr(e.Action, 20)
		resource := truncStr(e.Resource, 30)

		line := fmt.Sprintf("%-10s  %-22s  %-20s  %s", ts, user, action, resource)
		sb.WriteString(theme.Muted.Render(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

// View renders the audit log screen.
func (m AuditModel) View() string {
	if !m.isAdmin() {
		msg := "ACCESS DENIED — admin role required to view audit log"
		return theme.Panel.Render(theme.ErrorText.Render(msg))
	}

	var sb strings.Builder

	header := theme.Emphasis.Render("Audit Log") + "  " +
		theme.Muted.Render(fmt.Sprintf("%d entries", len(m.filtered)))
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

	// Filter row.
	userLabel := "user: "
	if m.activeFilter == auditFilterUser {
		userLabel = theme.Emphasis.Render("user: ") + m.userFilter.View()
	} else {
		uv := m.userFilter.Value()
		if uv != "" {
			userLabel = "user: " + theme.Code.Render(uv)
		} else {
			userLabel = theme.Muted.Render("user: [u]")
		}
	}

	actionLabel := "action: "
	if m.activeFilter == auditFilterAction {
		actionLabel = theme.Emphasis.Render("action: ") + m.actionFilter.View()
	} else {
		av := m.actionFilter.Value()
		if av != "" {
			actionLabel = "action: " + theme.Code.Render(av)
		} else {
			actionLabel = theme.Muted.Render("action: [a]")
		}
	}

	sb.WriteString(userLabel + "   " + actionLabel)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 60))
	sb.WriteString("\n\n")

	// Viewport.
	vpView := components.NewViewport(m.width-4, m.height-12)
	vpView.SetContent(m.renderEntries())
	sb.WriteString(vpView.View())

	sb.WriteString("\n")
	sb.WriteString(theme.Muted.Render("[↑↓] scroll  [u] filter user  [a] filter action  [r] refresh  [?] help"))

	return sb.String()
}

// Title returns the screen name.
func (m AuditModel) Title() string { return "Audit" }

// KeyHelp returns local key bindings.
func (m AuditModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "filter by user")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "filter by action")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
