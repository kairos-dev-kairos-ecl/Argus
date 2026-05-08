package components

import (
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Table wraps bubbles/table with Argus theme styles.
type Table struct {
	inner table.Model
}

// NewTable creates a themed table with the given columns and rows.
func NewTable(columns []table.Column, rows []table.Row) Table {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.ColorText)).
		Background(lipgloss.Color(theme.ColorBorder)).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(theme.ColorBorder)).
		BorderBottom(true)
	s.Selected = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.ColorBg)).
		Background(lipgloss.Color(theme.ColorAccent))
	t.SetStyles(s)

	return Table{inner: t}
}

// SetRows updates the table data.
func (t *Table) SetRows(rows []table.Row) {
	t.inner.SetRows(rows)
}

// View renders the table.
func (t Table) View() string {
	return theme.Panel.Render(t.inner.View())
}

// SelectedRow returns the currently highlighted row.
func (t Table) SelectedRow() table.Row {
	return t.inner.SelectedRow()
}
