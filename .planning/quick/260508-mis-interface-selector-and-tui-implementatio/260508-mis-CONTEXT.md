# Quick Task 260508-mis: Interface Selector + TUI Implementation Plan — Context

**Gathered:** 2026-05-08
**Status:** Ready for planning

<domain>
## Task Boundary

Design and plan two deliverables:
1. A CLI-based interface selector that shows when `argus` is run with no args — lets users choose between Web Dashboard and Terminal UI, displays advantages/disadvantages side-by-side, and remembers the choice.
2. A full phased implementation plan for the TUI using Bubbletea + Lipgloss (Go), integrated into the existing `cmd/argus/` Cobra command structure.

**No code changes in this task** — output is CONTEXT.md (this file) + PLAN.md only.

The GUI has 15 screens (from App.tsx):
- Dashboard, Signal Topology, Trace, Query, Kairos, Incidents, Alerts, Apps, Connectors, Rules, Config, Settings, Profile, Users (admin), Audit Log (admin)

</domain>

<decisions>
## Implementation Decisions

### Selector entry point
- **CLI binary** — `argus` with no args shows a terminal selector screen
- Launches web server or TUI from there
- `argus web` always bypasses selector and launches web
- `argus tui` always bypasses selector and launches TUI

### Preference persistence
- **Remember + skip** — saves preference to `~/.argus/config.yaml`
- Subsequent `argus` invocations skip the selector and go straight to the chosen interface
- `argus reset-ui` command clears the preference and re-shows the selector

### TUI v1 screen scope — Core 6
Cover the 90% operator workflow, not full GUI parity:
1. **Live signal feed** — streaming, filterable, WebSocket
2. **Trace investigation** — expand/collapse L1–L10 layers per trace
3. **Alerts queue** — priority-sorted, ack/resolve actions
4. **Detection rules** — viewer + `$EDITOR` handoff for editing
5. **Users / IAM** — list users, invite, role management
6. **Audit log** — scrollable read-only log

Screens deliberately excluded from v1 (GUI-only):
- Dashboard charts (ECharts/Sankey — no terminal equivalent)
- Signal Topology (Sankey diagram)
- Query interface (SQL editor — too complex for TUI v1)
- Kairos (policy evaluator UI)
- Connectors, Apps, Config, Settings, Profile

### Simultaneous mode
- **Exclusive choice** — selector picks one or the other
- `argus web` and `argus tui` are mutually exclusive entry points
- Simplifies the model; no need to coordinate two live processes

### Claude's Discretion
- Exact Bubbletea component breakdown (model/view split per screen)
- Keyboard shortcut layout
- How the comparison table is rendered in the selector (static vs dynamic)
- Phase sequencing for TUI build (how many phases, what order)
- Whether `argus reset-ui` is a subcommand or a flag on `argus`

</decisions>

<specifics>
## Specific Ideas from Conversation

**Selector screen layout (discussed):**
```
ARGUS XDR — Choose your interface

  ┌─────────────────────────┐  ┌─────────────────────────┐
  │  [G] WEB DASHBOARD      │  │  [T] TERMINAL (TUI)      │
  │                         │  │                          │
  │  ✓ Visual dashboards    │  │  ✓ Keyboard-driven       │
  │  ✓ Sankey flow charts   │  │  ✓ Works over SSH        │
  │  ✓ MITRE ATT&CK matrix  │  │  ✓ No browser needed     │
  │  ✓ Drag & drop rules    │  │  ✓ Lower latency         │
  │  ✓ Rich incident view   │  │  ✓ Scriptable/pipeable   │
  │                         │  │                          │
  │  ✗ Requires browser     │  │  ✗ No Sankey diagrams    │
  │  ✗ No SSH access        │  │  ✗ No MITRE grid view    │
  │  ✗ API binding overhead │  │  ✗ No drag & drop        │
  └─────────────────────────┘  └─────────────────────────┘

  [G] Launch Web Dashboard    [T] Launch Terminal UI

  Your choice will be remembered. Run 'argus reset-ui' to change it.
```

**TUI data layouts (discussed):**
- Live signals: colored table, layer badges (L1–L10), severity blocks (█)
- Trace: expand/collapse tree per layer with key fields inline
- Alerts: priority queue, [Enter] to open, [a]ck, [r]esolve, [/] to search
- Rules: YAML pager with syntax highlight, [e]dit hands off to $EDITOR
- Users: table + invite modal inline
- Audit: scrollable viewport, filter by user/action

**TUI tech stack:**
- Bubbletea (event loop + model/view/update)
- Lipgloss (styling, borders, colors matching brutalist dark theme)
- Bubbles (pre-built table, viewport, textinput, spinner components)
- Same Go binary as backend — new `cmd/argus/tui/` package

</specifics>

<canonical_refs>
## Canonical References

- `web/src/App.tsx` — definitive list of all 15 GUI screens
- `internal/auth/` — JWT/RBAC to reuse for TUI auth
- `web/src/services/iam-service.ts` — API endpoints the TUI will call
- Design System Conventions in CLAUDE.md — brutalist dark theme tokens apply to TUI too
- Bubbletea: https://github.com/charmbracelet/bubbletea
- Lipgloss: https://github.com/charmbracelet/lipgloss
- Bubbles: https://github.com/charmbracelet/bubbles

</canonical_refs>
