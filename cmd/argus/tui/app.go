// Package tui provides the Bubbletea application for the Argus terminal UI.
package tui

import (
	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/keys"
	"github.com/argusxdr/argus/cmd/argus/tui/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen identifies which operator screen is currently active.
type Screen int

const (
	ScreenLogin   Screen = iota
	ScreenSignals        // 1 — Live signal feed
	ScreenTrace          // 2 — Trace investigation
	ScreenAlerts         // 3 — Alerts queue
	ScreenRules          // 4 — Detection rules
	ScreenUsers          // 5 — Users / IAM
	ScreenAudit          // 6 — Audit log
)

// screenNames maps Screen values to their human-readable names.
var screenNames = map[Screen]string{
	ScreenLogin:   "Login",
	ScreenSignals: "Signals",
	ScreenTrace:   "Trace",
	ScreenAlerts:  "Alerts",
	ScreenRules:   "Rules",
	ScreenUsers:   "Users",
	ScreenAudit:   "Audit",
}

// Config holds startup configuration for the TUI application.
type Config struct {
	// BaseURL is the base URL of the Argus API server.
	BaseURL string
}

// AppModel is the root Bubbletea model. It acts as a screen router, managing
// which operator screen is active and dispatching global key events.
type AppModel struct {
	// Current active screen.
	current Screen

	// Auth state and clients.
	authState  *auth.AuthState
	authClient *auth.Client
	apiClient  *api.Client

	// Key bindings.
	keys keys.Bindings

	// Dimensions.
	width  int
	height int

	// Overlay state.
	helpVisible  bool
	quitConfirm  bool

	// Login screen (always resident — handles first boot and re-login).
	loginScreen screens.LoginModel

	// Placeholder screens for the 6 operator screens (Phase 3 will replace these).
	placeholders map[Screen]*screens.PlaceholderModel

	// Status bar.
	statusBar components.StatusBar
}

// New creates a new AppModel with the given configuration.
// This function is safe to call in tests without spinning up a tea.Program.
func New(cfg Config) *AppModel {
	authState := auth.NewAuthState()
	authClient := auth.NewClient(cfg.BaseURL)
	apiClient := api.NewClient(cfg.BaseURL, authState, authClient)

	placeholders := map[Screen]*screens.PlaceholderModel{
		ScreenSignals: screens.NewPlaceholder("Signals"),
		ScreenTrace:   screens.NewPlaceholder("Trace"),
		ScreenAlerts:  screens.NewPlaceholder("Alerts"),
		ScreenRules:   screens.NewPlaceholder("Rules"),
		ScreenUsers:   screens.NewPlaceholder("Users"),
		ScreenAudit:   screens.NewPlaceholder("Audit"),
	}

	return &AppModel{
		current:      ScreenLogin,
		authState:    authState,
		authClient:   authClient,
		apiClient:    apiClient,
		keys:         keys.New(),
		loginScreen:  screens.NewLoginModel(authClient),
		placeholders: placeholders,
		statusBar:    components.NewStatusBar(80), // default; updated on WindowSizeMsg
	}
}

// Init implements tea.Model. Returns the login screen's init command (cursor blink).
func (m *AppModel) Init() tea.Cmd {
	return m.loginScreen.Init()
}

// screenTitle returns the title of the currently active screen.
func (m *AppModel) screenTitle() string {
	if name, ok := screenNames[m.current]; ok {
		return name
	}
	return "Unknown"
}

// authStatus returns the status bar right section showing the current auth state.
func (m *AppModel) authStatus() string {
	if !m.authState.IsAuthenticated() {
		return "anon"
	}
	return m.authState.Email() + "@" + m.authState.Role()
}

// screenCount returns the total number of operator screens (excluding login).
var operatorScreens = []Screen{
	ScreenSignals, ScreenTrace, ScreenAlerts, ScreenRules, ScreenUsers, ScreenAudit,
}
