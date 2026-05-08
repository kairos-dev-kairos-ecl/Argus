package tui

import (
	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/screens"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
)

// Update implements tea.Model. It handles global key events and delegates to
// the active screen's update method.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusBar = components.NewStatusBar(msg.Width)
		// Forward resize to all screens so they can reflow.
		var cmds []tea.Cmd
		cmds = append(cmds, m.delegateUpdate(msg))
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case screens.LoginSuccessMsg:
		// Successful login: populate auth state and switch to Signals.
		m.authState = msg.State
		// Re-create role-gated screens now that we know the role.
		role := m.authState.Role()
		m.usersScreen = screens.NewUsersModel(m.apiClient, role)
		m.auditScreen = screens.NewAuditModel(m.apiClient, role)
		m.current = ScreenSignals
		// Kick off init for the signals screen (initial data fetch + WS).
		return m, m.signalsScreen.Init()

	case screens.SignalOpenTraceMsg:
		// Signals screen emitted this — switch to trace screen and load the trace.
		var cmd tea.Cmd
		m.traceScreen, cmd = m.traceScreen.SetTrace(msg.TraceID)
		m.current = ScreenTrace
		return m, cmd

	case screens.TraceGoBackMsg:
		// Trace screen navigated back — return to signals.
		m.current = ScreenSignals
		return m, nil
	}

	// Delegate to the active screen.
	return m, m.delegateUpdate(msg)
}

// handleKey dispatches global key events before delegating to the active screen.
func (m *AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c quits immediately from any screen.
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	// On the login screen: only ctrl+c is global; all other keys go to the form.
	if m.current == ScreenLogin {
		return m, m.delegateUpdate(msg)
	}

	// --- Operator screen global keys ---

	// If quit confirm is active: handle y/n.
	if m.quitConfirm {
		switch msg.String() {
		case "y", "Y":
			return m, tea.Quit
		default:
			m.quitConfirm = false
			return m, nil
		}
	}

	// ? — toggle help overlay.
	if key.Matches(msg, m.keys.Help_) {
		m.helpVisible = !m.helpVisible
		return m, nil
	}

	// Esc — close help or pass through.
	if key.Matches(msg, m.keys.Back) {
		if m.helpVisible {
			m.helpVisible = false
			return m, nil
		}
		return m, m.delegateUpdate(msg)
	}

	// q — quit with confirmation (only on operator screens, not textinput-focused).
	if msg.String() == "q" {
		m.quitConfirm = true
		return m, nil
	}

	// Number keys 1–6 switch screens directly.
	switch {
	case key.Matches(msg, m.keys.Screen1):
		m.current = ScreenSignals
		return m, nil
	case key.Matches(msg, m.keys.Screen2):
		m.current = ScreenTrace
		return m, nil
	case key.Matches(msg, m.keys.Screen3):
		m.current = ScreenAlerts
		return m, nil
	case key.Matches(msg, m.keys.Screen4):
		m.current = ScreenRules
		return m, nil
	case key.Matches(msg, m.keys.Screen5):
		m.current = ScreenUsers
		return m, nil
	case key.Matches(msg, m.keys.Screen6):
		m.current = ScreenAudit
		return m, nil
	}

	// Tab / Shift+Tab — cycle screens.
	if key.Matches(msg, m.keys.NextScreen) {
		m.current = nextScreen(m.current)
		return m, nil
	}
	if key.Matches(msg, m.keys.PrevScreen) {
		m.current = prevScreen(m.current)
		return m, nil
	}

	return m, m.delegateUpdate(msg)
}

// delegateUpdate sends the message to the currently active screen and stores
// the updated screen model back in the AppModel.
func (m *AppModel) delegateUpdate(msg tea.Msg) tea.Cmd {
	switch m.current {
	case ScreenLogin:
		updated, cmd := m.loginScreen.UpdateLogin(msg)
		m.loginScreen = updated
		return cmd

	case ScreenSignals:
		updated, cmd := m.signalsScreen.UpdateSignals(msg)
		m.signalsScreen = updated
		return cmd

	case ScreenTrace:
		updated, cmd := m.traceScreen.UpdateTrace(msg)
		m.traceScreen = updated
		return cmd

	case ScreenAlerts:
		updated, cmd := m.alertsScreen.UpdateAlerts(msg)
		m.alertsScreen = updated
		return cmd

	case ScreenRules:
		updated, cmd := m.rulesScreen.UpdateRules(msg)
		m.rulesScreen = updated
		return cmd

	case ScreenUsers:
		updated, cmd := m.usersScreen.UpdateUsers(msg)
		m.usersScreen = updated
		return cmd

	case ScreenAudit:
		updated, cmd := m.auditScreen.UpdateAudit(msg)
		m.auditScreen = updated
		return cmd
	}

	return nil
}

// nextScreen cycles to the next operator screen (wraps around).
func nextScreen(current Screen) Screen {
	for i, s := range operatorScreens {
		if s == current {
			return operatorScreens[(i+1)%len(operatorScreens)]
		}
	}
	return ScreenSignals
}

// prevScreen cycles to the previous operator screen (wraps around).
func prevScreen(current Screen) Screen {
	for i, s := range operatorScreens {
		if s == current {
			idx := (i - 1 + len(operatorScreens)) % len(operatorScreens)
			return operatorScreens[idx]
		}
	}
	return ScreenSignals
}
