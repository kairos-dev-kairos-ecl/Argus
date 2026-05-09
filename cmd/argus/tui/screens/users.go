// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings (admin-only):
//   i        — open invite user modal [DEVIATION: screen-specific, no contract conflict]
//   d        — deactivate selected user [DEVIATION: screen-specific, no contract conflict]
//   r        — refresh user list (compliant: r=refresh)
//
// In invite modal:
//   tab      — switch between email/role fields [DEVIATION: tab captured locally for field focus]
//   enter    — submit invite (compliant: enter=confirm)
//   esc      — cancel invite (compliant: esc=back)
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

// UsersLoadedMsg carries the list of users.
type UsersLoadedMsg struct {
	Users []api.User
	Err   error
}

// UserActionResultMsg carries the result of an IAM action.
type UserActionResultMsg struct {
	Action string // "invite" | "deactivate" | "role"
	UserID string
	Err    error
}

// usersMode identifies the current interaction mode.
type usersMode int

const (
	usersModeNormal     usersMode = iota
	usersModeInvite               // invite modal open
	usersModeConfirmDel           // confirm deactivate
)

// inviteField identifies which invite form field is focused.
type inviteField int

const (
	inviteFieldEmail inviteField = iota
	inviteFieldRole
)

var usersTableColumns = []table.Column{
	{Title: "EMAIL", Width: 28},
	{Title: "ROLE", Width: 10},
	{Title: "STATUS", Width: 10},
	{Title: "NAME", Width: 20},
}

// UsersModel is the Bubbletea model for the Users / IAM screen.
//
// Layout:
//
//	┌─ Users / IAM ──────────────────────────────────────────────────────┐
//	│ EMAIL                     ROLE    MFA   CREATED                   │
//	│ admin@argus.io            admin   yes   2026-01-01                │
//	│ alice@argus.io            analyst no    2026-02-15                │
//	└────────────────────────────────────────────────────────────────────┘
//	 [i] invite  [d] deactivate  [r] refresh  [?] help
//
// Admin-only: non-admin users see "ACCESS DENIED" and cannot use this screen.
type UsersModel struct {
	apiClient    *api.Client
	currentRole  string // role of the logged-in operator
	users        []api.User
	table        components.Table
	loading      bool
	spinner      spinner.Model
	err          string
	mode         usersMode
	// Invite form.
	inviteEmail  textinput.Model
	inviteRole   textinput.Model
	inviteFocused inviteField
	// Confirm deactivate.
	confirmUserID string
	confirmModal  components.Modal
	statusMsg    string
}

// NewUsersModel creates the users screen.
func NewUsersModel(apiClient *api.Client, currentRole string) UsersModel {
	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	emailInput := textinput.New()
	emailInput.Placeholder = "user@example.com"
	emailInput.CharLimit = 256

	roleInput := textinput.New()
	roleInput.Placeholder = "admin | analyst | viewer"
	roleInput.CharLimit = 32

	return UsersModel{
		apiClient:   apiClient,
		currentRole: currentRole,
		table:       components.NewTable(usersTableColumns, nil),
		loading:     true,
		spinner:     sp,
		inviteEmail: emailInput,
		inviteRole:  roleInput,
	}
}

// Init fetches the user list if admin.
func (m UsersModel) Init() tea.Cmd {
	if !m.isAdmin() {
		return nil
	}
	return tea.Batch(m.fetchUsers(), m.spinner.Tick)
}

func (m UsersModel) isAdmin() bool {
	return strings.ToLower(m.currentRole) == "admin"
}

func (m UsersModel) fetchUsers() tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return UsersLoadedMsg{} }
	}
	client := m.apiClient
	return func() tea.Msg {
		var resp struct {
			Users []api.User `json:"users"`
		}
		if err := client.Get(context.Background(), "/api/v1/users", &resp); err != nil {
			return UsersLoadedMsg{Err: err}
		}
		return UsersLoadedMsg{Users: resp.Users}
	}
}

func (m UsersModel) sendInvite(email, role string) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg {
			return UserActionResultMsg{Action: "invite"}
		}
	}
	client := m.apiClient
	return func() tea.Msg {
		body := map[string]string{"email": email, "role": role}
		if err := client.Post(context.Background(), "/api/v1/users/invite", body, nil); err != nil {
			return UserActionResultMsg{Action: "invite", Err: err}
		}
		return UserActionResultMsg{Action: "invite"}
	}
}

func (m UsersModel) deactivateUser(userID string) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg {
			return UserActionResultMsg{Action: "deactivate", UserID: userID}
		}
	}
	client := m.apiClient
	return func() tea.Msg {
		if err := client.Delete(context.Background(), fmt.Sprintf("/api/v1/users/%s", userID)); err != nil {
			return UserActionResultMsg{Action: "deactivate", UserID: userID, Err: err}
		}
		return UserActionResultMsg{Action: "deactivate", UserID: userID}
	}
}

// Update implements tea.Model.
func (m UsersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateUsers(msg)
	return updated, cmd
}

// UpdateUsers is the type-safe update method.
func (m UsersModel) UpdateUsers(msg tea.Msg) (UsersModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case UsersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.users = msg.Users
		m.table = components.NewTable(usersTableColumns, m.buildRows())
		return m, nil

	case UserActionResultMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Action + " failed: " + msg.Err.Error()
			return m, nil
		}
		switch msg.Action {
		case "invite":
			m.statusMsg = "Invite sent"
			m.mode = usersModeNormal
			// Refresh list.
			return m, m.fetchUsers()
		case "deactivate":
			m.statusMsg = "User deactivated"
			m.mode = usersModeNormal
			m.users = removeUser(m.users, msg.UserID)
			m.table = components.NewTable(usersTableColumns, m.buildRows())
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to invite form inputs when in invite mode.
	if m.mode == usersModeInvite {
		if m.inviteFocused == inviteFieldEmail {
			var cmd tea.Cmd
			m.inviteEmail, cmd = m.inviteEmail.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.inviteRole, cmd = m.inviteRole.Update(msg)
		return m, cmd
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m UsersModel) handleKey(msg tea.KeyMsg) (UsersModel, tea.Cmd) {
	// Confirm deactivate modal.
	if m.mode == usersModeConfirmDel {
		switch msg.String() {
		case "y", "Y":
			userID := m.confirmUserID
			m.mode = usersModeNormal
			m.confirmUserID = ""
			return m, m.deactivateUser(userID)
		default:
			m.mode = usersModeNormal
			m.confirmUserID = ""
			return m, nil
		}
	}

	// Invite modal.
	if m.mode == usersModeInvite {
		switch msg.String() {
		case "esc":
			m.mode = usersModeNormal
			m.inviteEmail.Blur()
			m.inviteRole.Blur()
			return m, nil

		case "tab":
			if m.inviteFocused == inviteFieldEmail {
				m.inviteFocused = inviteFieldRole
				m.inviteEmail.Blur()
				m.inviteRole.Focus()
			} else {
				m.inviteFocused = inviteFieldEmail
				m.inviteRole.Blur()
				m.inviteEmail.Focus()
			}
			return m, textinput.Blink

		case "enter":
			email := m.inviteEmail.Value()
			role := m.inviteRole.Value()
			if email == "" || role == "" {
				m.statusMsg = "Email and role are required"
				return m, nil
			}
			m.statusMsg = ""
			m.inviteEmail.SetValue("")
			m.inviteRole.SetValue("")
			m.inviteEmail.Blur()
			m.inviteRole.Blur()
			return m, m.sendInvite(email, role)
		}

		// Forward key events to the active input.
		if m.inviteFocused == inviteFieldEmail {
			var cmd tea.Cmd
			m.inviteEmail, cmd = m.inviteEmail.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.inviteRole, cmd = m.inviteRole.Update(msg)
		return m, cmd
	}

	// Normal mode.
	switch msg.String() {
	case "i":
		if m.isAdmin() {
			m.mode = usersModeInvite
			m.inviteFocused = inviteFieldEmail
			m.inviteEmail.Focus()
			m.inviteRole.Blur()
			return m, textinput.Blink
		}
		return m, nil

	case "d":
		if m.isAdmin() {
			row := m.table.SelectedRow()
			if len(row) > 0 {
				// Find the user ID for this email.
				email := row[0]
				for _, u := range m.users {
					if u.Email == email {
						m.mode = usersModeConfirmDel
						m.confirmUserID = u.ID
						m.confirmModal = components.NewModal(52, 7, "Deactivate User",
							fmt.Sprintf("Deactivate user %q?\nThis action cannot be undone.\n\n[y] confirm  [any] cancel",
								truncStr(email, 30)))
						break
					}
				}
			}
		}
		return m, nil

	case "r":
		m.loading = true
		m.err = ""
		m.statusMsg = ""
		return m, tea.Batch(m.fetchUsers(), m.spinner.Tick)
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m UsersModel) buildRows() []table.Row {
	var rows []table.Row
	for _, u := range m.users {
		status := u.Status
		if status == "" {
			status = "active"
		}
		rows = append(rows, table.Row{u.Email, u.Role, status, truncStr(u.DisplayName, 20)})
	}
	return rows
}

func removeUser(users []api.User, id string) []api.User {
	result := make([]api.User, 0, len(users))
	for _, u := range users {
		if u.ID != id {
			result = append(result, u)
		}
	}
	return result
}

// View renders the users screen.
func (m UsersModel) View() string {
	if !m.isAdmin() {
		msg := "ACCESS DENIED — admin role required to manage users"
		return theme.Panel.Render(theme.ErrorText.Render(msg))
	}

	var sb strings.Builder

	header := theme.Emphasis.Render("Users / IAM") + "  " +
		theme.Muted.Render(fmt.Sprintf("%d users", len(m.users)))
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

	if m.mode == usersModeInvite {
		inviteContent := theme.Emphasis.Render("Invite User") + "\n\n" +
			"Email: " + m.inviteEmail.View() + "\n" +
			"Role:  " + m.inviteRole.View() + "\n\n" +
			theme.Muted.Render("[tab] switch field  [enter] send invite  [esc] cancel")
		modal := components.NewModal(60, 12, "Invite User", inviteContent)
		return modal.Render(m.table.View())
	}

	tableView := m.table.View()

	if m.mode == usersModeConfirmDel {
		return m.confirmModal.Render(tableView)
	}

	sb.WriteString(tableView)
	sb.WriteString("\n")

	if m.statusMsg != "" {
		sb.WriteString(theme.Subtle.Render(m.statusMsg) + "\n")
	}

	sb.WriteString(theme.Muted.Render("[i] invite  [d] deactivate  [r] refresh  [?] help"))
	return sb.String()
}

// Title returns the screen name.
func (m UsersModel) Title() string { return "Users" }

// KeyHelp returns local key bindings.
func (m UsersModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "invite user")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "deactivate user")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
