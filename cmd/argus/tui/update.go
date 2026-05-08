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
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case screens.LoginSuccessMsg:
		// Successful login: populate auth state and switch to Signals.
		m.authState = msg.State
		// Re-wire the API client with the new state.
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

// delegateUpdate sends the message to the currently active screen.
func (m *AppModel) delegateUpdate(msg tea.Msg) tea.Cmd {
	if m.current == ScreenLogin {
		updated, cmd := m.loginScreen.UpdateLogin(msg)
		m.loginScreen = updated
		return cmd
	}
	// Operator placeholder screens are stateless — no update needed.
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
