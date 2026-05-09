package screens

import (
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertsModel_InitialState(t *testing.T) {
	m := NewAlertsModel(nil)
	assert.True(t, m.loading)
	assert.Equal(t, alertsActionNone, m.confirmAction)
}

func TestAlertsModel_AlertsLoadedMsg(t *testing.T) {
	m := NewAlertsModel(nil)
	msg := AlertsLoadedMsg{
		Alerts: []api.Alert{
			{ID: "a1", Title: "rule1", Severity: 4, Status: "open",
				Category: "test alert", FirstSeenAt: time.Now()},
		},
	}

	updated, _ := m.UpdateAlerts(msg)
	assert.False(t, updated.loading)
	assert.Len(t, updated.alerts, 1)
	assert.Len(t, updated.filtered, 1)
}

func TestAlertsModel_AlertsLoadedMsg_Error(t *testing.T) {
	m := NewAlertsModel(nil)
	msg := AlertsLoadedMsg{Err: assert.AnError}
	updated, _ := m.UpdateAlerts(msg)
	assert.False(t, updated.loading)
	assert.NotEmpty(t, updated.err)
}

func TestAlertsModel_AckKey_NoSelectedRow_NoModal(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.alerts = []api.Alert{
		{ID: "alrt-001", Severity: 4, Status: "open", FirstSeenAt: time.Now()},
	}
	m.filtered = m.alerts

	// We don't have a selected row without a real table, so just test key dispatch.
	updated, _ := m.UpdateAlerts(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// Without a selected row from table, confirmAlertID will be "" so no modal is shown.
	// This tests the key was dispatched (no panic).
	assert.Equal(t, alertsActionNone, updated.confirmAction)
}

func TestAlertsModel_ConfirmModal_YConfirmsAction(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.confirmAction = alertsActionAck
	m.confirmAlertID = "alrt-001"

	updated, cmd := m.UpdateAlerts(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, alertsActionNone, updated.confirmAction, "action should be cleared after confirm")
	require.NotNil(t, cmd, "confirm should return a cmd to call API")
}

func TestAlertsModel_ConfirmModal_OtherKeyCancels(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.confirmAction = alertsActionAck
	m.confirmAlertID = "alrt-001"

	updated, _ := m.UpdateAlerts(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, alertsActionNone, updated.confirmAction, "non-y should cancel")
	assert.Empty(t, updated.confirmAlertID)
}

func TestAlertsModel_ConfirmModal_Resolve_YConfirmsAction(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.confirmAction = alertsActionResolve
	m.confirmAlertID = "alrt-002"

	updated, cmd := m.UpdateAlerts(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, alertsActionNone, updated.confirmAction)
	require.NotNil(t, cmd)
}

func TestAlertsModel_AlertActionResult_RemovesAlert(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.alerts = []api.Alert{
		{ID: "a1", Severity: 4, Status: "open", FirstSeenAt: time.Now()},
		{ID: "a2", Severity: 2, Status: "open", FirstSeenAt: time.Now()},
	}
	m.filtered = m.alerts

	msg := AlertActionResultMsg{AlertID: "a1", Action: "ack"}
	updated, _ := m.UpdateAlerts(msg)
	assert.Len(t, updated.alerts, 1)
	assert.Equal(t, "a2", updated.alerts[0].ID)
}

func TestAlertsModel_AlertActionResult_Error(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	msg := AlertActionResultMsg{AlertID: "a1", Action: "ack", Err: assert.AnError}
	updated, _ := m.UpdateAlerts(msg)
	assert.NotEmpty(t, updated.err)
}

func TestAlertsModel_FilterMode_EnterEscExits(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.filterMode = true
	m.filter.Focus()

	updated, _ := m.UpdateAlerts(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, updated.filterMode)
}

func TestAlertsModel_View_Loading(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = true
	view := m.View()
	assert.Contains(t, view, "loading")
}

func TestAlertsModel_View_Error(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	m.err = "server error"
	view := m.View()
	assert.Contains(t, view, "server error")
}

func TestAlertsModel_View_NoAlerts(t *testing.T) {
	m := NewAlertsModel(nil)
	m.loading = false
	view := m.View()
	assert.Contains(t, view, "No alerts")
}

func TestAlertsModel_Title(t *testing.T) {
	m := NewAlertsModel(nil)
	assert.Equal(t, "Alerts", m.Title())
}

func TestAlertsModel_KeyHelp_NonEmpty(t *testing.T) {
	m := NewAlertsModel(nil)
	assert.NotEmpty(t, m.KeyHelp())
}

func TestCriticalCount(t *testing.T) {
	alerts := []api.Alert{
		{Severity: 5}, // CRITICAL
		{Severity: 4}, // HIGH
		{Severity: 5}, // CRITICAL
	}
	assert.Equal(t, 2, criticalCount(alerts))
}

func TestRemoveAlert(t *testing.T) {
	alerts := []api.Alert{{ID: "a1"}, {ID: "a2"}, {ID: "a3"}}
	result := removeAlert(alerts, "a2")
	assert.Len(t, result, 2)
	assert.Equal(t, "a1", result[0].ID)
	assert.Equal(t, "a3", result[1].ID)
}

// Ensure the model satisfies tea.Model.
func TestAlertsModel_ImplementsTeaModel(t *testing.T) {
	m := NewAlertsModel(nil)
	var _ tea.Model = m
}
