package behaviour

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runsLoadedMsg:
		m.Loading = false
		m.Err = msg.Err
		m.Runs = msg.Runs
		return m, nil
	case graphLoadedMsg:
		m.Loading = false
		m.Err = msg.Err
		if msg.Graph != nil {
			switch msg.Slot {
			case 0:
				m.Selected = msg.Graph
				m.CurrentView = ViewRunDetail
			case 1:
				m.CompareA = msg.Graph
			case 2:
				m.CompareB = msg.Graph
				m.CurrentView = ViewRunCompare
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.CurrentView = ViewRunList
			return m, nil
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil
		case "down", "j":
			if m.Cursor < len(m.Runs)-1 {
				m.Cursor++
			}
			return m, nil
		case "enter":
			if m.CurrentView == ViewRunList && len(m.Runs) > 0 {
				m.Loading = true
				return m, fetchGraph(m.Client, m.BaseURL, m.Token, m.Runs[m.Cursor].TraceID, 0)
			}
		case "a":
			if m.CurrentView == ViewRunList && len(m.Runs) > 0 {
				return m, fetchGraph(m.Client, m.BaseURL, m.Token, m.Runs[m.Cursor].TraceID, 1)
			}
		case "b":
			if m.CurrentView == ViewRunList && len(m.Runs) > 0 {
				return m, fetchGraph(m.Client, m.BaseURL, m.Token, m.Runs[m.Cursor].TraceID, 2)
			}
		case "c":
			m.CurrentView = ViewRunCompare
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.CurrentView {
	case ViewRunDetail:
		return renderRunDetail(m)
	case ViewRunCompare:
		return renderRunCompare(m)
	default:
		return renderRunList(m)
	}
}
