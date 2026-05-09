// Keymap (Phase 4 consolidated):
//
// Cross-screen contract: Enter=open, Esc=back, /=filter, r=refresh, ?=help (global), q=quit (global)
//
// Local bindings:
//   tab      — switch between list pane and YAML preview pane [DEVIATION: tab is normally global cycle;
//              here tab is captured by the rules screen for pane switching; global tab is not reached]
//   enter    — select rule and open YAML preview (compliant: enter=open)
//   e        — open selected rule in $EDITOR [DEVIATION: screen-specific, no contract conflict]
//   t        — toggle rule enabled/disabled [DEVIATION: screen-specific, no contract conflict]
//   r        — refresh rule list (compliant: r=refresh)

package screens

// Security constraints for $EDITOR invocation (MANDATORY — verified by test):
//
//  1. Temp directory: os.MkdirTemp(homeDir, "argus-rule-*") where homeDir
//     comes from os.UserHomeDir(). NEVER os.TempDir() or /tmp/ directly.
//  2. Dir permissions: 0700 (owner-only rwx).
//  3. File permissions: 0600 (owner-only rw). Use os.WriteFile(path, data, 0600).
//  4. Editor invocation: exec.Command(editorBin, tmpFile) directly.
//     NEVER exec.Command("sh", "-c", ...) or exec.Command("bash", "-c", ...).
//  5. editorBin resolved via exec.LookPath(os.Getenv("EDITOR")); fallback: vi → nano → notepad.
//  6. Always defer os.RemoveAll(tmpDir) regardless of editor exit code.
//  7. No Bearer token or file path in any log or status message.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// RulesLoadedMsg carries the list of detection rules.
type RulesLoadedMsg struct {
	Rules []api.Rule
	Err   error
}

// RuleSelectedMsg carries the full YAML for the selected rule.
type RuleSelectedMsg struct {
	Rule api.Rule
	Err  error
}

// RuleUpdatedMsg is returned after an $EDITOR session completes.
type RuleUpdatedMsg struct {
	RuleID  string
	NewYAML string
	Err     error
}

// RuleToggleResultMsg is returned after toggling a rule's enabled state.
type RuleToggleResultMsg struct {
	RuleID  string
	Enabled bool
	Err     error
}

// rulesPane identifies which pane has keyboard focus.
type rulesPane int

const (
	rulesPaneList    rulesPane = iota
	rulesPanePreview           // YAML viewport on right
)

// rulesConfirmAction identifies a pending destructive action.
type rulesConfirmAction int

const (
	rulesConfirmNone   rulesConfirmAction = iota
	rulesConfirmToggle                    // enable/disable toggle
)

var rulesTableColumns = []table.Column{
	{Title: "NAME", Width: 28},
	{Title: "SEV", Width: 7},
	{Title: "STATUS", Width: 10},
}

// RulesModel is the Bubbletea model for the Detection Rules screen.
//
// Layout:
//
//	┌─ Rules ──────────────────────────────────────────────────────────┐
//	│ [List pane 1/3]    │  [YAML preview pane 2/3]                   │
//	│ name     sev  ok   │  name: payment_rate_spike                  │
//	│ ...               │  description: …                            │
//	└───────────────────────────────────────────────────────────────────┘
//	 [↑↓] select  [e] edit  [t] toggle  [tab] switch pane  [?] help
type RulesModel struct {
	apiClient     *api.Client
	rules         []api.Rule
	selectedRule  *api.Rule
	table         components.Table
	preview       viewport.Model
	activePane    rulesPane
	loading       bool
	spinner       spinner.Model
	err           string
	confirmAction rulesConfirmAction
	confirmModal  components.Modal
	statusMsg     string // brief non-sensitive status ("Rule updated")
}

// NewRulesModel creates the rules screen.
func NewRulesModel(apiClient *api.Client) RulesModel {
	sp := spinner.New()
	sp.Style = theme.Subtle
	sp.Spinner = spinner.Dot

	vp := viewport.New(60, 30)

	return RulesModel{
		apiClient:  apiClient,
		table:      components.NewTable(rulesTableColumns, nil),
		preview:    vp,
		activePane: rulesPaneList,
		loading:    true,
		spinner:    sp,
	}
}

// Init fetches the rule list.
func (m RulesModel) Init() tea.Cmd {
	return tea.Batch(m.fetchRules(), m.spinner.Tick)
}

func (m RulesModel) fetchRules() tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg { return RulesLoadedMsg{} }
	}
	client := m.apiClient
	return func() tea.Msg {
		var resp struct {
			Rules []api.Rule `json:"rules"`
		}
		if err := client.Get(context.Background(), "/api/v1/rules", &resp); err != nil {
			return RulesLoadedMsg{Err: err}
		}
		return RulesLoadedMsg{Rules: resp.Rules}
	}
}

func (m RulesModel) toggleRule(rule api.Rule) tea.Cmd {
	if m.apiClient == nil {
		return func() tea.Msg {
			return RuleToggleResultMsg{RuleID: rule.ID, Enabled: !rule.Enabled}
		}
	}
	client := m.apiClient
	newEnabled := !rule.Enabled
	ruleID := rule.ID
	return func() tea.Msg {
		body := map[string]interface{}{"enabled": newEnabled}
		if err := client.Post(context.Background(), fmt.Sprintf("/api/v1/rules/%s/toggle", ruleID), body, nil); err != nil {
			return RuleToggleResultMsg{RuleID: ruleID, Err: err}
		}
		return RuleToggleResultMsg{RuleID: ruleID, Enabled: newEnabled}
	}
}

// editorCmd suspends bubbletea, opens the selected rule's YAML in $EDITOR,
// reads back the modified content, and emits a RuleUpdatedMsg.
//
// SECURITY INVARIANTS (verified by TestRulesModel_EditorSecurity_*):
//   - Temp dir created via os.MkdirTemp(homeDir, "argus-rule-*"), NOT os.TempDir()
//   - Dir mode 0700, file mode 0600
//   - Editor forked directly via exec.Command(editorBin, file), NOT via "sh -c"
//   - editorBin from os.Getenv("EDITOR"), fallback vi → nano → notepad
//   - os.RemoveAll deferred unconditionally
func editorCmd(rule api.Rule, apiClient *api.Client) tea.Cmd {
	return tea.ExecProcess(buildEditorCommand(rule), func(err error) tea.Msg {
		if err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: err}
		}
		// The file path was set up in buildEditorCommand; we need a closure to
		// communicate back. Use execEditorAndRead instead for the real path.
		return RuleUpdatedMsg{RuleID: rule.ID, NewYAML: ""}
	})
}

// launchEditor is the real editor entry point — creates a secure temp file,
// opens the editor, reads back the result, and cleans up.
//
// SECURITY: Uses os.MkdirTemp under homeDir (not os.TempDir), dir 0700, file 0600,
// exec.Command(editorBin, file) — never "sh -c".
func launchEditor(rule api.Rule, apiClient *api.Client) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: home dir: %w", err)}
		}

		// SECURITY CONSTRAINT 1 & 2: temp dir under home, mode 0700.
		tmpDir, err := os.MkdirTemp(homeDir, "argus-rule-*")
		if err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: create temp dir: %w", err)}
		}
		// SECURITY: always clean up, regardless of outcome.
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		tmpFile := tmpDir + string(os.PathSeparator) + "rule.yaml"

		// SECURITY CONSTRAINT 3: file mode 0600.
		if err := os.WriteFile(tmpFile, []byte(rule.YAML), 0600); err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: write temp file: %w", err)}
		}

		// SECURITY CONSTRAINT 3 & 5: resolve editor via LookPath.
		editorBin, lookErr := resolveEditor()
		if lookErr != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: no editor found: %w", lookErr)}
		}

		// SECURITY CONSTRAINT 4: exec.Command directly — NEVER "sh -c".
		cmd := exec.Command(editorBin, tmpFile) //nolint:gosec
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: editor exited with error: %w", err)}
		}

		// Read back the edited YAML.
		newYAML, err := os.ReadFile(tmpFile) //nolint:gosec
		if err != nil {
			return RuleUpdatedMsg{RuleID: rule.ID, Err: fmt.Errorf("rules: read temp file: %w", err)}
		}

		// Push update to API if client is available.
		if apiClient != nil {
			body := map[string]string{"yaml": string(newYAML)}
			if err := apiClient.Post(context.Background(), fmt.Sprintf("/api/v1/rules/%s", rule.ID), body, nil); err != nil {
				return RuleUpdatedMsg{RuleID: rule.ID, Err: err}
			}
		}

		return RuleUpdatedMsg{RuleID: rule.ID, NewYAML: string(newYAML)}
	}
}

// resolveEditor returns the absolute path of the editor binary.
// Priority: $EDITOR → vi → nano → notepad.
// SECURITY: uses exec.LookPath only — never exec "sh -c".
func resolveEditor() (string, error) {
	// Try EDITOR env var first.
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		if path, err := exec.LookPath(envEditor); err == nil {
			return path, nil
		}
	}
	// Fallback chain.
	for _, candidate := range []string{"vi", "nano", "notepad"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no editor found (set $EDITOR)")
}

// buildEditorCommand is used by the ExecProcess path (alt approach).
func buildEditorCommand(rule api.Rule) *exec.Cmd {
	// SECURITY: exec.Command directly with binary resolved from $EDITOR or fallback.
	editorBin, err := resolveEditor()
	if err != nil {
		editorBin = "vi" // safe fallback for ExecProcess path
	}

	// Note: in ExecProcess we can't pass the file path through cleanly because
	// the temp file must be created before the process starts. The real secure
	// path is launchEditor() above. This function returns a placeholder for
	// ExecProcess plumbing only.
	_ = rule
	cmd := exec.Command(editorBin) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Update implements tea.Model.
func (m RulesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.UpdateRules(msg)
	return updated, cmd
}

// UpdateRules is the type-safe update method.
func (m RulesModel) UpdateRules(msg tea.Msg) (RulesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.preview = viewport.New(msg.Width*2/3, msg.Height-6)
		if m.selectedRule != nil {
			m.preview.SetContent(m.selectedRule.YAML)
		}
		return m, nil

	case RulesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.rules = msg.Rules
		m.table = components.NewTable(rulesTableColumns, m.buildRows())
		return m, nil

	case RuleUpdatedMsg:
		if msg.Err != nil {
			m.statusMsg = "Editor error: " + msg.Err.Error()
			return m, nil
		}
		m.statusMsg = "Rule updated"
		// Update the YAML in the local cache.
		for i, r := range m.rules {
			if r.ID == msg.RuleID && msg.NewYAML != "" {
				m.rules[i].YAML = msg.NewYAML
				if m.selectedRule != nil && m.selectedRule.ID == msg.RuleID {
					cp := m.rules[i]
					m.selectedRule = &cp
					m.preview.SetContent(cp.YAML)
				}
			}
		}
		return m, nil

	case RuleToggleResultMsg:
		m.confirmAction = rulesConfirmNone
		if msg.Err != nil {
			m.statusMsg = "Toggle failed: " + msg.Err.Error()
			return m, nil
		}
		for i, r := range m.rules {
			if r.ID == msg.RuleID {
				m.rules[i].Enabled = msg.Enabled
			}
		}
		m.table = components.NewTable(rulesTableColumns, m.buildRows())
		m.statusMsg = "Rule toggled"
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to active pane.
	if m.activePane == rulesPanePreview {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return m, cmd
	}

	t, cmd := m.table.Update(msg)
	m.table = t
	return m, cmd
}

func (m RulesModel) handleKey(msg tea.KeyMsg) (RulesModel, tea.Cmd) {
	// Confirm modal active.
	if m.confirmAction != rulesConfirmNone {
		switch msg.String() {
		case "y", "Y":
			if m.selectedRule != nil && m.confirmAction == rulesConfirmToggle {
				rule := *m.selectedRule
				m.confirmAction = rulesConfirmNone
				return m, m.toggleRule(rule)
			}
			m.confirmAction = rulesConfirmNone
		default:
			m.confirmAction = rulesConfirmNone
		}
		return m, nil
	}

	switch msg.String() {
	case "tab":
		if m.activePane == rulesPaneList {
			m.activePane = rulesPanePreview
		} else {
			m.activePane = rulesPaneList
		}
		return m, nil

	case "enter":
		// Select rule from list — load full YAML into preview.
		if m.activePane == rulesPaneList {
			row := m.table.SelectedRow()
			if len(row) > 0 {
				for _, r := range m.rules {
					if r.Name == row[0] {
						cp := r
						m.selectedRule = &cp
						m.preview.SetContent(r.YAML)
						m.activePane = rulesPanePreview
						break
					}
				}
			}
		}
		return m, nil

	case "e":
		if m.selectedRule != nil {
			rule := *m.selectedRule
			client := m.apiClient
			return m, launchEditor(rule, client)
		}
		return m, nil

	case "t":
		if m.selectedRule != nil {
			statusLabel := "disable"
			if !m.selectedRule.Enabled {
				statusLabel = "enable"
			}
			m.confirmAction = rulesConfirmToggle
			m.confirmModal = components.NewModal(52, 7, "Toggle Rule",
				fmt.Sprintf("Confirm %s rule %q?\n\n[y] confirm  [any] cancel",
					statusLabel, truncStr(m.selectedRule.Name, 24)))
		}
		return m, nil

	case "r":
		m.loading = true
		m.err = ""
		m.statusMsg = ""
		return m, tea.Batch(m.fetchRules(), m.spinner.Tick)
	}

	// Delegate to active pane.
	if m.activePane == rulesPanePreview {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return m, cmd
	}

	t, cmd := m.table.Update(msg)
	m.table = t

	// Auto-update preview on row change.
	if m.selectedRule == nil && len(m.rules) > 0 {
		row := m.table.SelectedRow()
		if len(row) > 0 {
			for _, r := range m.rules {
				if r.Name == row[0] {
					cp := r
					m.selectedRule = &cp
					m.preview.SetContent(r.YAML)
					break
				}
			}
		}
	}

	return m, cmd
}

func (m RulesModel) buildRows() []table.Row {
	var rows []table.Row
	for _, r := range m.rules {
		status := "disabled"
		if r.Enabled {
			status = "enabled"
		}
		rows = append(rows, table.Row{truncStr(r.Name, 28), fmt.Sprintf("T%d", r.Tier), status})
	}
	return rows
}

// View renders the rules screen.
func (m RulesModel) View() string {
	var sb strings.Builder

	header := theme.Emphasis.Render("Detection Rules")
	if len(m.rules) > 0 {
		header += "  " + theme.Muted.Render(fmt.Sprintf("%d rules", len(m.rules)))
	}
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

	if len(m.rules) == 0 {
		sb.WriteString(theme.Subtle.Render("No rules found"))
		return theme.Panel.Render(sb.String())
	}

	// Two-pane layout.
	listView := m.table.View()
	previewContent := ""
	if m.selectedRule != nil {
		previewContent = theme.Emphasis.Render("YAML: "+truncStr(m.selectedRule.Name, 30)) +
			"\n\n" + theme.Code.Render(m.preview.View())
	} else {
		previewContent = theme.Subtle.Render("Select a rule to preview its YAML")
	}

	panes := listView + "\n\n" + previewContent

	// Confirm modal if active.
	if m.confirmAction != rulesConfirmNone {
		return m.confirmModal.Render(panes)
	}

	sb.WriteString(panes)
	sb.WriteString("\n")

	if m.statusMsg != "" {
		sb.WriteString(theme.Subtle.Render(m.statusMsg) + "\n")
	}

	sb.WriteString(theme.Muted.Render("[↑↓] select  [e] edit  [t] toggle  [tab] switch pane  [r] refresh  [?] help"))
	return sb.String()
}

// Title returns the screen name.
func (m RulesModel) Title() string { return "Rules" }

// KeyHelp returns local key bindings for the help overlay.
// Only screen-local bindings are returned; global bindings (q, ?, ctrl+c, esc) are
// rendered from keys/bindings.go and are NOT duplicated here.
// Note: tab is captured locally for pane switching (deviation from global tab=cycle).
func (m RulesModel) KeyHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch list/preview pane (local)")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select rule")),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit in $EDITOR")),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle enabled")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh rules")),
	}
}
