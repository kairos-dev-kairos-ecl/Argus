package selector

import tea "github.com/charmbracelet/bubbletea"

// Choice represents the user's interface selection.
type Choice string

const (
	// ChoiceWeb is returned when the user selects the web dashboard.
	ChoiceWeb Choice = "web"
	// ChoiceTUI is returned when the user selects the terminal UI.
	ChoiceTUI Choice = "tui"
	// ChoiceNone is returned when the user cancels (q / esc / ctrl+c).
	ChoiceNone Choice = ""
)

// Model is the Bubbletea model for the interface selector.
type Model struct {
	width  int
	height int
	choice Choice
}

// New returns a fresh selector model with no choice made.
func New() Model {
	return Model{}
}

// Init implements tea.Model. No I/O commands needed on startup.
func (m Model) Init() tea.Cmd {
	return nil
}

// Choice returns the user's selection after the program exits.
func (m Model) Choice() Choice {
	return m.choice
}
