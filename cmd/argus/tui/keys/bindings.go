// Package keys defines the global key bindings for the Argus TUI.
package keys

import "github.com/charmbracelet/bubbles/key"

// Bindings holds all global key bindings for the TUI. Individual screens may
// define additional local bindings.
type Bindings struct {
	// Quit exits the application immediately (ctrl+c) or with confirmation (q).
	Quit key.Binding
	// Help_ toggles the help overlay. Named Help_ to avoid conflict with method.
	Help_ key.Binding
	// NextScreen cycles to the next screen (tab).
	NextScreen key.Binding
	// PrevScreen cycles to the previous screen (shift+tab).
	PrevScreen key.Binding
	// Screen1–Screen6 jump directly to each operator screen.
	Screen1 key.Binding
	Screen2 key.Binding
	Screen3 key.Binding
	Screen4 key.Binding
	Screen5 key.Binding
	Screen6 key.Binding
	// Refresh reloads the current screen's data (r).
	Refresh key.Binding
	// Submit confirms a form or selection (enter).
	Submit key.Binding
	// Back cancels or goes back (esc).
	Back key.Binding
}

// New returns a Bindings populated with the default key assignments.
func New() Bindings {
	return Bindings{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit immediately"),
		),
		Help_: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		NextScreen: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next screen"),
		),
		PrevScreen: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev screen"),
		),
		Screen1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "signals"),
		),
		Screen2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "traces"),
		),
		Screen3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "alerts"),
		),
		Screen4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "rules"),
		),
		Screen5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "users"),
		),
		Screen6: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "audit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back / cancel"),
		),
	}
}

// Help returns all global key bindings as a flat slice for the help overlay.
func (b Bindings) Help() []key.Binding {
	return []key.Binding{
		b.Quit,
		b.Help_,
		b.NextScreen,
		b.PrevScreen,
		b.Screen1,
		b.Screen2,
		b.Screen3,
		b.Screen4,
		b.Screen5,
		b.Screen6,
		b.Refresh,
		b.Submit,
		b.Back,
	}
}

// Section groups a named set of key bindings for the sectioned help overlay.
type Section struct {
	// Name is the human-readable section heading shown in the help overlay.
	Name string
	// Bindings is the ordered list of bindings within this section.
	Bindings []key.Binding
}

// Sections returns bindings grouped by section for the help overlay.
// The Global section contains all 13 global bindings. Per-screen sections start
// empty — the view layer populates them from each screen's KeyHelp() at render time.
// Sections are returned in a fixed order:
//
//	Global, Navigation, Signals, Trace, Alerts, Rules, Users, Audit
func (b Bindings) Sections() []Section {
	return []Section{
		{
			Name: "Global",
			Bindings: []key.Binding{
				b.Quit,
				b.Help_,
				b.NextScreen,
				b.PrevScreen,
				b.Screen1,
				b.Screen2,
				b.Screen3,
				b.Screen4,
				b.Screen5,
				b.Screen6,
				b.Refresh,
				b.Submit,
				b.Back,
			},
		},
		{Name: "Navigation", Bindings: nil},
		{Name: "Signals", Bindings: nil},
		{Name: "Trace", Bindings: nil},
		{Name: "Alerts", Bindings: nil},
		{Name: "Rules", Bindings: nil},
		{Name: "Users", Bindings: nil},
		{Name: "Audit", Bindings: nil},
	}
}
