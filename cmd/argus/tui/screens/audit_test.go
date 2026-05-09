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
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestAuditModel_InitialState(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	assert.True(t, m.loading)
	assert.Equal(t, auditFilterNone, m.activeFilter)
}

func TestAuditModel_NonAdmin_NoLoad(t *testing.T) {
	m := NewAuditModel(nil, "analyst")
	cmd := m.Init()
	assert.Nil(t, cmd, "non-admin should not fetch audit log")
}

func TestAuditModel_Admin_CanLoad(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	cmd := m.Init()
	assert.NotNil(t, cmd, "admin should issue fetch cmd")
}

func TestAuditModel_AuditLoadedMsg(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	email := "admin@argus.io"
	entries := []api.AuditEntry{
		{
			ID:           "e1",
			ActorEmail:   &email,
			Action:       "login_success",
			ResourceType: "auth",
			IPAddress:    "10.0.0.1",
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
		},
	}
	updated, _ := m.UpdateAudit(AuditLoadedMsg{Entries: entries})
	assert.False(t, updated.loading)
	assert.Len(t, updated.entries, 1)
	assert.Len(t, updated.filtered, 1)
}

func TestAuditModel_AuditLoadedMsg_Error(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	updated, _ := m.UpdateAudit(AuditLoadedMsg{Err: assert.AnError})
	assert.False(t, updated.loading)
	assert.NotEmpty(t, updated.err)
}

func TestAuditModel_FilterUser_KeyU(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false

	updated, _ := m.UpdateAudit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	assert.Equal(t, auditFilterUser, updated.activeFilter)
}

func TestAuditModel_FilterAction_KeyA(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false

	updated, _ := m.UpdateAudit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, auditFilterAction, updated.activeFilter)
}

func TestAuditModel_FilterActive_EscExits(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false
	m.activeFilter = auditFilterUser
	m.userFilter.Focus()

	updated, _ := m.UpdateAudit(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, auditFilterNone, updated.activeFilter)
}

func TestAuditModel_FilterActive_EnterExits(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false
	m.activeFilter = auditFilterAction

	updated, _ := m.UpdateAudit(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, auditFilterNone, updated.activeFilter)
}

func TestAuditModel_ApplyFilters_ByUser(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	admin := "admin@argus.io"
	alice := "alice@argus.io"
	m.entries = []api.AuditEntry{
		{ActorEmail: &admin, Action: "login"},
		{ActorEmail: &alice, Action: "login"},
	}
	m.userFilter.SetValue("alice")

	filtered := m.applyFilters()
	assert.Len(t, filtered, 1)
	assert.Equal(t, "alice@argus.io", filtered[0].ActorEmailStr())
}

func TestAuditModel_ApplyFilters_ByAction(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	email := "admin@argus.io"
	m.entries = []api.AuditEntry{
		{ActorEmail: &email, Action: "login_success"},
		{ActorEmail: &email, Action: "rule_updated"},
	}
	m.actionFilter.SetValue("rule")

	filtered := m.applyFilters()
	assert.Len(t, filtered, 1)
	assert.Equal(t, "rule_updated", filtered[0].Action)
}

func TestAuditModel_ApplyFilters_NoFilter_ReturnsAll(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	admin := "admin@argus.io"
	alice := "alice@argus.io"
	m.entries = []api.AuditEntry{
		{ActorEmail: &admin, Action: "login"},
		{ActorEmail: &alice, Action: "login"},
	}

	filtered := m.applyFilters()
	assert.Len(t, filtered, 2)
}

func TestAuditModel_RefreshKey_SetsLoading(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false

	updated, cmd := m.UpdateAudit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.NotNil(t, cmd)
	assert.True(t, updated.loading)
}

func TestAuditModel_View_NonAdmin_AccessDenied(t *testing.T) {
	m := NewAuditModel(nil, "analyst")
	view := m.View()
	assert.Contains(t, view, "ACCESS DENIED")
}

func TestAuditModel_View_Admin_Loading(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = true
	assert.Contains(t, m.View(), "loading")
}

func TestAuditModel_View_Admin_Error(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false
	m.err = "unauthorized"
	assert.Contains(t, m.View(), "unauthorized")
}

func TestAuditModel_View_Admin_ShowsEntries(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	m.loading = false
	email := "admin@argus.io"
	m.entries = []api.AuditEntry{
		{ActorEmail: &email, Action: "login_success", Timestamp: time.Now().UTC().Format(time.RFC3339)},
	}
	m.filtered = m.entries
	view := m.View()
	assert.Contains(t, view, "1 entries")
}

func TestAuditModel_Title(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	assert.Equal(t, "Audit", m.Title())
}

func TestAuditModel_KeyHelp_NonEmpty(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	assert.NotEmpty(t, m.KeyHelp())
}

// Ensure AuditModel satisfies tea.Model.
func TestAuditModel_ImplementsTeaModel(t *testing.T) {
	m := NewAuditModel(nil, "admin")
	var _ tea.Model = m
}
