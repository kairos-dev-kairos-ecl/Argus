package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FocusField identifies which login form field is focused.
type FocusField int

const (
	FocusEmail    FocusField = iota
	FocusPassword FocusField = iota
	FocusOTP      FocusField = iota
)

// LoginSuccessMsg is emitted when login completes successfully.
// The caller (AppModel) transitions to ScreenSignals when this is received.
type LoginSuccessMsg struct {
	State *auth.AuthState
}

// LoginMFAMsg is emitted by the auth command when mfa_required is returned.
// The login screen transitions to OTP entry mode when this is received.
type LoginMFAMsg struct {
	MFAToken string
}

// LoginModel is the Bubbletea model for the login screen.
// It supports email/password login and MFA OTP entry on the same screen.
//
// SECURITY: textinput contents are local-only and are never sent to logs or
// written to disk. The password field uses EchoPassword mode.
type LoginModel struct {
	emailInput    textinput.Model
	passwordInput textinput.Model
	otpInput      textinput.Model
	focused       FocusField
	mfaMode       bool
	mfaToken      string
	errorMsg      string
	authClient    *auth.Client
}

// NewLoginModel creates a new LoginModel. authClient may be nil for tests.
func NewLoginModel(authClient *auth.Client) LoginModel {
	email := textinput.New()
	email.Placeholder = "email@example.com"
	email.CharLimit = 256
	email.Focus()

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'
	password.CharLimit = 128

	otp := textinput.New()
	otp.Placeholder = "000000"
	otp.CharLimit = 6

	return LoginModel{
		emailInput:    email,
		passwordInput: password,
		otpInput:      otp,
		focused:       FocusEmail,
		authClient:    authClient,
	}
}

// Focused returns the currently focused field.
func (m LoginModel) Focused() FocusField {
	return m.focused
}

// MFAMode returns true when the screen is in OTP entry mode.
func (m LoginModel) MFAMode() bool {
	return m.mfaMode
}

// Init implements tea.Model. Starts cursor blinking on the email field.
func (m LoginModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateLogin(msg)
	return updated, cmd
}

// UpdateLogin is the type-safe update method used by tests.
func (m LoginModel) UpdateLogin(msg tea.Msg) (LoginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case LoginMFAMsg:
		m.mfaMode = true
		m.mfaToken = msg.MFAToken
		m.focused = FocusOTP
		m.errorMsg = ""
		m.emailInput.Blur()
		m.passwordInput.Blur()
		m.otpInput.Focus()
		return m, textinput.Blink

	case loginResultMsg:
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		if msg.mfaRequired {
			return m, func() tea.Msg {
				return LoginMFAMsg{MFAToken: msg.state.RefreshToken()}
			}
		}
		return m, func() tea.Msg {
			return LoginSuccessMsg{State: msg.state}
		}
	}

	// Forward key events to the active textinput.
	return m.updateActiveInput(msg)
}

func (m LoginModel) handleKey(msg tea.KeyMsg) (LoginModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		return m.cycleFocus()

	case tea.KeyEnter:
		if m.mfaMode {
			return m.submitMFA()
		}
		return m.submitLogin()
	}

	return m.updateActiveInput(msg)
}

func (m LoginModel) cycleFocus() (LoginModel, tea.Cmd) {
	if m.mfaMode {
		return m, nil
	}
	if m.focused == FocusEmail {
		m.focused = FocusPassword
		m.emailInput.Blur()
		m.passwordInput.Focus()
	} else {
		m.focused = FocusEmail
		m.passwordInput.Blur()
		m.emailInput.Focus()
	}
	return m, textinput.Blink
}

func (m LoginModel) submitLogin() (LoginModel, tea.Cmd) {
	email := m.emailInput.Value()
	password := m.passwordInput.Value()
	if email == "" || password == "" {
		m.errorMsg = "Email and password are required"
		return m, nil
	}
	m.errorMsg = ""

	if m.authClient == nil {
		// Test path: return a mock success.
		return m, func() tea.Msg {
			return loginResultMsg{state: auth.NewAuthState(), mfaRequired: false}
		}
	}

	client := m.authClient
	return m, func() tea.Msg {
		state, mfaRequired, err := client.Login(context.Background(), email, password)
		return loginResultMsg{state: state, mfaRequired: mfaRequired, err: err}
	}
}

func (m LoginModel) submitMFA() (LoginModel, tea.Cmd) {
	code := m.otpInput.Value()
	if len(code) != 6 {
		m.errorMsg = "OTP must be exactly 6 digits"
		return m, nil
	}
	m.errorMsg = ""

	if m.authClient == nil {
		return m, func() tea.Msg {
			return loginResultMsg{state: auth.NewAuthState(), mfaRequired: false}
		}
	}

	mfaToken := m.mfaToken
	client := m.authClient
	return m, func() tea.Msg {
		state, err := client.VerifyMFA(context.Background(), mfaToken, code)
		if err != nil {
			return loginResultMsg{err: err}
		}
		return loginResultMsg{state: state, mfaRequired: false}
	}
}

func (m LoginModel) updateActiveInput(msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmd tea.Cmd
	if m.mfaMode {
		m.otpInput, cmd = m.otpInput.Update(msg)
	} else if m.focused == FocusEmail {
		m.emailInput, cmd = m.emailInput.Update(msg)
	} else {
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

// View renders the login screen.
func (m LoginModel) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Title.Render("ARGUS XDR — Operator Login"))
	sb.WriteString("\n\n")

	if m.mfaMode {
		sb.WriteString(theme.Subtle.Render("Two-factor authentication required"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("OTP Code: %s\n", m.otpInput.View()))
	} else {
		sb.WriteString(fmt.Sprintf("Email:    %s\n", m.emailInput.View()))
		sb.WriteString(fmt.Sprintf("Password: %s\n", m.passwordInput.View()))
	}

	if m.errorMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(theme.ErrorText.Render("Error: " + m.errorMsg))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if m.mfaMode {
		sb.WriteString(theme.Muted.Render("[enter] verify OTP  [ctrl+c] quit"))
	} else {
		sb.WriteString(theme.Muted.Render("[tab] switch field  [enter] login  [ctrl+c] quit"))
	}

	return theme.Panel.Render(sb.String())
}

// Title returns the screen title.
func (m LoginModel) Title() string {
	return "Login"
}

// KeyHelp returns an empty slice; login bindings are shown inline.
func (m LoginModel) KeyHelp() []key.Binding {
	return []key.Binding{}
}

// loginResultMsg is an internal message carrying the login result.
type loginResultMsg struct {
	state       *auth.AuthState
	mfaRequired bool
	err         error
}
