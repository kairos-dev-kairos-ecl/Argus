package selector

import tea "github.com/charmbracelet/bubbletea"

// Update implements tea.Model. Handles window sizing and key input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "g", "G":
			m.choice = ChoiceWeb
			return m, tea.Quit
		case "t", "T":
			m.choice = ChoiceTUI
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.choice = ChoiceNone
			return m, tea.Quit
		}
	}

	return m, nil
}
