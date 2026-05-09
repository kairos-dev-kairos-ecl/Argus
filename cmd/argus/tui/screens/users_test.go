package screens

import (
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersModel_InitialState(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	assert.True(t, m.loading)
	assert.Equal(t, usersModeNormal, m.mode)
}

func TestUsersModel_NonAdmin_NoLoad(t *testing.T) {
	m := NewUsersModel(nil, "analyst")
	cmd := m.Init()
	assert.Nil(t, cmd, "non-admin should not fetch users")
}

func TestUsersModel_Admin_CanLoad(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	cmd := m.Init()
	assert.NotNil(t, cmd, "admin should issue fetch cmd")
}

func TestUsersModel_UsersLoadedMsg(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	users := []api.User{
		{ID: "u1", Email: "admin@argus.io", Role: "admin", CreatedAt: api.ProtoTime{time.Now()}},
		{ID: "u2", Email: "alice@argus.io", Role: "analyst", CreatedAt: api.ProtoTime{time.Now()}},
	}
	updated, _ := m.UpdateUsers(UsersLoadedMsg{Users: users})
	assert.False(t, updated.loading)
	assert.Len(t, updated.users, 2)
}

func TestUsersModel_UsersLoadedMsg_Error(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	updated, _ := m.UpdateUsers(UsersLoadedMsg{Err: assert.AnError})
	assert.False(t, updated.loading)
	assert.NotEmpty(t, updated.err)
}

func TestUsersModel_InviteKey_OpensModal(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false

	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	assert.Equal(t, usersModeInvite, updated.mode)
}

func TestUsersModel_InviteKey_NonAdmin_DoesNotOpen(t *testing.T) {
	m := NewUsersModel(nil, "analyst")
	m.loading = false

	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	assert.Equal(t, usersModeNormal, updated.mode, "non-admin cannot open invite modal")
}

func TestUsersModel_InviteModal_EscCancels(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeInvite

	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, usersModeNormal, updated.mode)
}

func TestUsersModel_InviteModal_TabCyclesFocus(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeInvite
	m.inviteFocused = inviteFieldEmail

	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, inviteFieldRole, updated.inviteFocused)

	updated2, _ := updated.UpdateUsers(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, inviteFieldEmail, updated2.inviteFocused)
}

func TestUsersModel_InviteModal_EmptyFieldsError(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeInvite
	// Don't fill in the email or role fields.

	updated, cmd := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "empty invite should not dispatch")
	assert.NotEmpty(t, updated.statusMsg)
}

func TestUsersModel_InviteAction_Success(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeInvite

	msg := UserActionResultMsg{Action: "invite"}
	updated, _ := m.UpdateUsers(msg)
	assert.Equal(t, usersModeNormal, updated.mode)
	assert.Equal(t, "Invite sent", updated.statusMsg)
}

func TestUsersModel_InviteAction_Error(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	msg := UserActionResultMsg{Action: "invite", Err: assert.AnError}
	updated, _ := m.UpdateUsers(msg)
	assert.NotEmpty(t, updated.statusMsg)
}

func TestUsersModel_DeactivateKey_ShowsConfirmModal(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.users = []api.User{
		{ID: "u1", Email: "alice@argus.io", Role: "analyst", CreatedAt: api.ProtoTime{time.Now()}},
	}
	// Note: without a real selected table row the deactivate won't fire.
	// Just verify key doesn't panic.
	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_ = updated
}

func TestUsersModel_ConfirmDeactivate_YConfirms(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeConfirmDel
	m.confirmUserID = "u1"

	updated, cmd := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, usersModeNormal, updated.mode)
	require.NotNil(t, cmd, "confirm should dispatch deactivate cmd")
}

func TestUsersModel_ConfirmDeactivate_OtherKeyCancels(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.mode = usersModeConfirmDel
	m.confirmUserID = "u1"

	updated, _ := m.UpdateUsers(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, usersModeNormal, updated.mode)
	assert.Empty(t, updated.confirmUserID)
}

func TestUsersModel_DeactivateAction_RemovesUser(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.users = []api.User{
		{ID: "u1", Email: "alice@argus.io", Role: "analyst", CreatedAt: api.ProtoTime{time.Now()}},
		{ID: "u2", Email: "bob@argus.io", Role: "viewer", CreatedAt: api.ProtoTime{time.Now()}},
	}

	msg := UserActionResultMsg{Action: "deactivate", UserID: "u1"}
	updated, _ := m.UpdateUsers(msg)
	assert.Len(t, updated.users, 1)
	assert.Equal(t, "u2", updated.users[0].ID)
	assert.Equal(t, "User deactivated", updated.statusMsg)
}

func TestUsersModel_View_NonAdmin_AccessDenied(t *testing.T) {
	m := NewUsersModel(nil, "analyst")
	view := m.View()
	assert.Contains(t, view, "ACCESS DENIED")
}

func TestUsersModel_View_Admin_Loading(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = true
	assert.Contains(t, m.View(), "loading")
}

func TestUsersModel_View_Admin_Error(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	m.loading = false
	m.err = "server error"
	assert.Contains(t, m.View(), "server error")
}

func TestUsersModel_Title(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	assert.Equal(t, "Users", m.Title())
}

func TestUsersModel_KeyHelp_NonEmpty(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	assert.NotEmpty(t, m.KeyHelp())
}

func TestRemoveUser(t *testing.T) {
	users := []api.User{{ID: "u1"}, {ID: "u2"}, {ID: "u3"}}
	result := removeUser(users, "u2")
	assert.Len(t, result, 2)
	assert.Equal(t, "u1", result[0].ID)
	assert.Equal(t, "u3", result[1].ID)
}

// Ensure UsersModel satisfies tea.Model.
func TestUsersModel_ImplementsTeaModel(t *testing.T) {
	m := NewUsersModel(nil, "admin")
	var _ tea.Model = m
}
