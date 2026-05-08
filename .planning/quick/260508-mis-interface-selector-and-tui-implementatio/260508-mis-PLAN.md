# Quick Task 260508-mis: Interface Selector + TUI — Implementation Plan

**Status:** Blueprint (no code in this task)
**Type:** Phased implementation plan
**Audience:** Developer executing the build over multiple sessions
**Companion doc:** `260508-mis-CONTEXT.md` (decisions + scope)

---

## Executive Summary

Two deliverables, sequenced as four phases:

| Phase | Deliverable | Depends on |
|-------|-------------|------------|
| **1** | Interface selector + Cobra restructure + preference file | nothing |
| **2** | TUI foundation (root model, theme, auth, API client, components) | Phase 1 (so `argus tui` is wired) |
| **3** | Six core operator screens | Phase 2 |
| **4** | UX polish (help overlay, keyboard map, SSH/no-unicode fallback) | Phase 3 |

**Total scope:** ~25–35 new Go files, 0 schema changes, 0 backend changes. Reuses existing REST API and JWT auth.

**Anti-goals:**
- No GUI parity (Sankey, ECharts, Kairos UI deliberately excluded)
- No new backend endpoints (consume what frontend already uses)
- No persistent TUI state on disk (JWT in memory, preference file is the only on-disk artifact)

---

## Repository Layout — Target State

```
cmd/argus/
├── main.go                  [MODIFIED] — root command rewires to selector when no subcommand
├── selector/                [NEW] — interface selector mini-Bubbletea app
│   ├── model.go
│   ├── view.go
│   ├── update.go
│   └── prefs.go             — read/write ~/.argus/config.yaml
├── tui/                     [NEW] — Cobra subcommand entrypoint for TUI
│   └── cmd.go
├── web/                     [NEW or refactored] — `argus web` subcommand (extracted from current api/server)
│   └── cmd.go
└── reset_ui.go              [NEW] — `argus reset-ui` subcommand

internal/tui/                [NEW] — TUI implementation lives here, not in cmd/
├── app.go                   — root Bubbletea model (screen router)
├── theme/
│   ├── theme.go             — Lipgloss styles matching brutalist tokens
│   └── tokens.go            — color/spacing constants
├── auth/
│   └── login.go             — login screen model
├── api/
│   ├── client.go            — HTTP client (wraps net/http with JWT injection)
│   ├── ws.go                — WebSocket signal stream
│   └── types.go             — DTOs matching web/src/types
├── components/              — reusable Bubbles wrappers
│   ├── statusbar.go
│   ├── helpfooter.go
│   ├── table.go             — themed wrapper around bubbles/table
│   ├── viewport.go
│   ├── modal.go
│   └── badge.go             — layer/severity colored badges
├── screens/
│   ├── signals.go           — Live signal feed
│   ├── trace.go             — Trace investigation tree
│   ├── alerts.go            — Alerts queue
│   ├── rules.go             — Detection rules viewer
│   ├── users.go             — Users / IAM
│   └── audit.go             — Audit log
└── keys/
    └── bindings.go          — keymap struct (used by help overlay)

go.mod                       [MODIFIED] — add bubbletea, lipgloss, bubbles, gopkg.in/yaml.v3
```

---

## Phase 1 — Interface Selector & CLI Restructure

**Goal:** Running `argus` with no args shows a 2-pane comparison screen and remembers the choice. `argus web`, `argus tui`, and `argus reset-ui` work as direct entry points.

### 1.1 Cobra command restructure

**File: `cmd/argus/main.go` (MODIFIED)**

Currently the root command likely binds directly to api/server logic. Restructure:

- Root command (`argus` with no args): `RunE` checks for saved preference. If found → dispatch to web or tui. If not found → launch selector. After selector picks, save and dispatch.
- Subcommands stay registered but root-level dispatch behaves like a router.
- Existing `api`, `ingest`, `server`, `rules`, `users`, `doctor` subcommands remain untouched — they're operator/admin tools, not the user-facing UI.

**New subcommand: `cmd/argus/web/cmd.go`**
- Wraps whatever currently launches the web server (likely the existing `server` or `api` command's web-serving path).
- Decision: either `argus web` is an alias of `argus server` OR a thin wrapper that calls the same underlying entry function. Prefer a thin wrapper so semantics stay clean (web = "launch the dashboard for me", server = "run the backend daemon").

**New subcommand: `cmd/argus/tui/cmd.go`**
- Initializes TUI program: `tea.NewProgram(internal/tui.NewApp(cfg), tea.WithAltScreen())`.
- Reads connection config (API base URL) from same Viper config the rest of `cmd/argus` uses.

**New subcommand: `cmd/argus/reset_ui.go`**
- `argus reset-ui` → deletes the `ui:` key (or whole file if only that key exists) from `~/.argus/config.yaml`.
- Decision: subcommand, not a flag. Keeps root flag surface clean and is more discoverable in `argus --help`.

### 1.2 Preference file

**File: `cmd/argus/selector/prefs.go` (NEW)**

```
~/.argus/config.yaml structure:
ui: web | tui            # the UI preference (only key needed for now)
```

Functions:
- `LoadUIPref() (string, error)` — returns "web", "tui", or "" if unset. Errors only on permission/parse failure, not missing file.
- `SaveUIPref(choice string) error` — writes atomically (tmp file + rename). Creates `~/.argus/` dir with 0700 if missing.
- `ClearUIPref() error` — used by `argus reset-ui`.

**Design notes:**
- Use `os.UserHomeDir()` not `$HOME` directly (cross-platform).
- File mode 0600 (contains nothing sensitive yet but future-proofs for API URL/tokens).
- Use `gopkg.in/yaml.v3` (already in Go ecosystem, no new dep beyond what TUI brings).
- This is a **separate file** from the server's Viper config. It is per-user UI state, not deployment config.

### 1.3 Selector screen (mini Bubbletea app)

**Files: `cmd/argus/selector/{model,view,update}.go` (NEW)**

A self-contained tea.Model with one job: display the comparison and capture a key.

**Layout (from CONTEXT.md `<specifics>`):**
- Title bar: "ARGUS XDR — Choose your interface"
- Two side-by-side bordered panels (Lipgloss `Border()` with `JoinHorizontal`)
- Each panel: heading + ✓ pros list + ✗ cons list
- Hint line under panels: `[G] Launch Web Dashboard    [T] Launch Terminal UI`
- Footer: "Your choice will be remembered. Run 'argus reset-ui' to change it."

**Update logic:**
- `g`, `G` → return choice "web" via tea.Quit + sentinel
- `t`, `T` → return choice "tui" via tea.Quit + sentinel
- `q`, `ctrl+c`, `esc` → exit without saving

**Returning the choice:**
The selector is launched from `argus` root command's `RunE`. After `tea.Program.Run()` returns, root command reads the final model's `choice` field, calls `SaveUIPref`, then dispatches to the chosen subcommand's `RunE` (or re-execs in worst case — prefer in-process call).

**Design decisions deferred to implementation:**
- Whether the comparison content is hardcoded in `view.go` or loaded from a constants file (recommend: hardcoded, ~30 lines, low churn).
- Width handling for narrow terminals: if `tea.WindowSizeMsg.Width < 80`, stack panels vertically instead of side-by-side.

### 1.4 Integration points

- **Cobra:** `os.Args` length check in root `RunE` to detect "no subcommand" path.
- **Viper:** none — preference file is independent of server config.
- **Logging:** selector should not log to the same destination as the server. Stderr only for fatal errors; stdout is owned by Bubbletea.

### 1.5 Phase 1 acceptance

1. `argus` with empty `~/.argus/config.yaml` shows selector → key picks UI → file written → chosen UI launches.
2. `argus` with existing pref skips selector and launches saved UI.
3. `argus web` and `argus tui` always launch directly (never read pref).
4. `argus reset-ui` deletes pref; next `argus` shows selector again.
5. Width < 80 cols: panels stack vertically, no truncation.

---

## Phase 2 — TUI Foundation

**Goal:** Boot a Bubbletea app that authenticates against the existing backend, shows a placeholder home screen, and has a working theme + reusable component layer. No real screens yet — those come in Phase 3.

### 2.1 Dependencies

Add to `go.mod`:
- `github.com/charmbracelet/bubbletea` v0.25+
- `github.com/charmbracelet/lipgloss` v0.10+
- `github.com/charmbracelet/bubbles` v0.18+
- `gopkg.in/yaml.v3` (if not already present from selector phase)

`go mod tidy` after adding.

### 2.2 Root app model — `internal/tui/app.go`

The root model is a screen router. It owns:
- `currentScreen` — enum (login, signals, trace, alerts, rules, users, audit, help)
- `auth` — JWT + user info (in-memory only, never persisted)
- `apiClient` — shared HTTP client
- `theme` — pre-built Lipgloss styles
- `keys` — global keymap
- `width`, `height` — from WindowSizeMsg
- `statusBar` — reusable component instance
- `helpVisible` — bool for `?` overlay

**Screen contract:**

```go
type Screen interface {
    tea.Model                 // Init, Update, View
    Title() string            // shown in header
    KeyHelp() []key.Binding   // shown in help footer / ? overlay
}
```

Each screen in Phase 3 implements this interface. Root model's `Update` delegates to the active screen's `Update` for non-global keys, intercepts global keys (q to quit, ? for help, 1-6 to switch screens) before delegating.

**Screen routing:**
- Number keys `1`–`6` → switch screen (1=signals, 2=trace, 3=alerts, 4=rules, 5=users, 6=audit)
- Tab / Shift-Tab → cycle screens
- Each screen is lazy-instantiated; first activation triggers initial data fetch via tea.Cmd.

### 2.3 Theme — `internal/tui/theme/`

Match the brutalist dark palette from CLAUDE.md:

| Token | Hex | Lipgloss role |
|-------|-----|---------------|
| Background | #0A0A0B | base canvas |
| Border | #2A2A2F | panel borders |
| Text primary | #FFFFFF | body text |
| Text secondary | #A0A0A0 | dimmed labels |
| Success | #22C55E | resolved/OK badges |
| Warning | #EAB308 | medium severity |
| Error | #EF4444 | high severity / critical |
| Info | #3B82F6 | informational |

Layer color ramp L1→L10 (matches GUI):
L1=#EF4444, L2=#F97316, L3=#EAB308, L4=#84CC16, L5=#22C55E, L6=#14B8A6, L7=#06B6D4, L8=#3B82F6, L9=#8B5CF6, L10=#F43F5E.

**Style structs to expose:**
- `theme.Header`, `theme.StatusBar`, `theme.Panel`, `theme.PanelActive`, `theme.Help`
- `theme.LayerBadge(layer int) lipgloss.Style`
- `theme.SeverityBadge(sev string) lipgloss.Style`
- `theme.Subtle`, `theme.Emphasis`, `theme.Error`

**Decision:** Keep theme construction in `init()` so styles are built once. Re-derive only on terminal resize if width-dependent.

**Unicode policy:** Theme file declares two glyph sets — `glyphs.Unicode` (✓ ✗ █ ▶ ▼) and `glyphs.ASCII` (+ x # > v). Phase 4 wires the toggle.

### 2.4 Auth flow — `internal/tui/auth/login.go`

**Login screen model:**
- `bubbles/textinput` for email
- `bubbles/textinput` for password (masked)
- Submit on Enter → POST `/api/v1/auth/login` (existing endpoint used by web frontend)
- Response: JWT access token + refresh token. Both held in memory only.

**Token lifecycle:**
- Access token attached as `Authorization: Bearer <token>` to every API call.
- On 401 from any endpoint, the API client triggers refresh via `/api/v1/auth/refresh` using the refresh token.
- If refresh fails → app returns to login screen and clears in-memory tokens.
- App quit → tokens evaporate. No persistence to disk (security decision, matches "JWT stored in memory" from prompt).

**MFA handling (Phase 6 backend already supports TOTP):**
- If login response indicates MFA required, push an MFA prompt screen with another textinput for the 6-digit code.
- POST to `/api/v1/auth/mfa/verify` with the code → finalize session.

**First-run / setup detection:**
- Before showing login, GET `/api/v1/auth/setup-status` (existing endpoint).
- If `setup_required: true`, show a friendly "Run `argus web` to complete first-time setup, then come back here" message and quit cleanly. (TUI v1 does not implement the setup wizard.)

### 2.5 API client — `internal/tui/api/client.go`

A thin wrapper over `net/http` with:
- Configurable base URL (from CLI flag `--api-url` or Viper, default `http://localhost:8080`)
- Auto-injection of `Authorization` header from auth state
- 401 → refresh → retry once interceptor
- Configurable timeout (10s default)
- Single shared `*http.Client` (connection pooling)
- `context.Context` support on every method (so screens can cancel inflight requests on screen change)

**Endpoint methods to expose (mirror `web/src/services/iam-service.ts` and friends):**
- `Login(email, pass)`, `RefreshToken()`, `Logout()`
- `ListSignals(filter SignalFilter) ([]Signal, error)`
- `GetTrace(traceID string) (Trace, error)`
- `ListAlerts(filter AlertFilter) ([]Alert, error)`
- `AckAlert(id string)`, `ResolveAlert(id string)`
- `ListRules() ([]Rule, error)`, `GetRule(id string) (Rule, error)`, `UpdateRule(id, yaml string) error`
- `ListUsers()`, `InviteUser(email, role string)`, `UpdateUserRole(id, role string)`
- `ListAuditLog(filter AuditFilter) ([]AuditEntry, error)`

**DTOs in `api/types.go`:** Lift from `web/src/types/*.ts`. Strict types, json tags matching the existing API contract.

### 2.6 WebSocket client — `internal/tui/api/ws.go`

For live signal feed.

- Use `nhooyr.io/websocket` or `github.com/gorilla/websocket` (latter likely already in repo for backend WS server — check and reuse if so).
- Connect to `/api/v1/signals/stream` (the same endpoint the web frontend uses).
- JWT passed via query param or first message frame depending on existing protocol — verify against backend's WS handler.
- Emits `tea.Cmd`-compatible messages: `SignalReceivedMsg{Signal}` pumped into Bubbletea's `Send()` channel.
- Auto-reconnect with exponential backoff, capped at 30s.
- Polling fallback: if WS connection fails 3 times, the signals screen falls back to polling `GET /api/v1/signals?since=<lastTimestamp>` every 2s. Status bar shows "FALLBACK POLLING" indicator.

### 2.7 Reusable components — `internal/tui/components/`

**`statusbar.go`** — bottom bar showing: connected user, current screen name, signal connection status (live / polling / disconnected), key hints.

**`helpfooter.go`** — single-line key hints rendered by current screen via the `Screen.KeyHelp()` contract.

**`table.go`** — wraps `bubbles/table` with theme styles, sticky header, row truncation rules, and a `SetRows` helper that handles empty state ("No data — press R to refresh").

**`viewport.go`** — wraps `bubbles/viewport` with theme + scrollbar indicator + filter input bar at top.

**`modal.go`** — overlay modal for invite forms, confirmations. Centers content, dims background by rendering a darkened version of the screen behind it.

**`badge.go`** — small inline pill: `theme.LayerBadge(3)` returns `[L3]` styled with the L3 color. Severity equivalent: `[HI]`, `[MD]`, `[LO]`.

### 2.8 Phase 2 acceptance

1. `argus tui` boots, shows login screen.
2. Valid credentials → home screen with status bar showing username + "WS: connected".
3. Wrong password → inline error, screen stays on login.
4. Resize terminal → all components reflow without panic.
5. `q` from any screen quits cleanly.
6. Theme renders correctly on iTerm2, Windows Terminal, and SSH session over `ssh -t`.

---

## Phase 3 — Core Operator Screens

**Goal:** Six functional screens, each consuming the existing REST/WS API. Number keys 1–6 navigate between them.

### 3.1 Screen 1 — Live Signal Feed (`screens/signals.go`)

**Layout:**
```
┌─ Live Signals ─────────────────────────── 1,247 signals · WS connected ─┐
│ TIME      LAYER  SEVERITY  TRACE         APP        CATEGORY            │
│ 10:42:13  [L7]   ████ HI   abc123…       qwen-prod  prompt_injection    │
│ 10:42:12  [L4]   ██   LO   def456…       qwen-prod  baseline_drift      │
│ ...                                                                       │
└──────────────────────────────────────────────────────────────────────────┘
 [/] filter  [Enter] open trace  [p] pause stream  [r] refresh  [?] help
```

**Behavior:**
- WS messages append to top of table (newest first), bounded to 1000 in-memory rows (older drop off).
- `/` opens an inline filter input (textinput component) that filters by app, layer, or category as you type.
- `Enter` on a row → push trace screen, passing trace_id.
- `p` toggles WS pause (stops appending; existing rows stay).
- Color-coded layer badges via `theme.LayerBadge`.
- Severity rendered as block characters proportional to score, colored by severity.

**Data:** `WSStream` for new signals. `apiClient.ListSignals()` for initial load (last 100).

### 3.2 Screen 2 — Trace Investigation (`screens/trace.go`)

**Layout:**
```
┌─ Trace abc123…  ─ 14 signals across L1-L10 ──────────────────────────────┐
│ ▼ [L1] Hardware           2 signals                                       │
│     gpu_id=A100-3, util=87%                                               │
│ ▶ [L2] Network            3 signals                                       │
│ ▼ [L7] Inference Layer    5 signals                                       │
│     ▼ prompt_injection    score=0.91                                      │
│         input: "Ignore previous instructions and..."                      │
│ ▶ [L9] Output Layer       2 signals                                       │
└──────────────────────────────────────────────────────────────────────────┘
 [↑↓] navigate  [Enter/Space] expand  [j/k] vim-style  [b] back  [?] help
```

**Behavior:**
- Custom tree model (Bubbletea idiomatic — no off-the-shelf bubble for trees).
- Nodes: layer → category → signal. Each level expands/collapses.
- `b` returns to signals screen, preserves trace_id in history (for re-entry).
- Colored by layer. Severity badges next to category counts.

**Data:** `apiClient.GetTrace(traceID)` returns full trace (signals grouped by layer in DTO).

### 3.3 Screen 3 — Alerts Queue (`screens/alerts.go`)

**Layout:**
```
┌─ Alerts (12 open · 3 critical) ──────────────────────────────────────────┐
│ ID         SEVERITY  CREATED       APP        SUMMARY                    │
│ alrt_001   ████ CRIT 10:30:22      qwen-prod  Prompt injection rate spike│
│ alrt_002   ███  HI   10:25:01      llama-dev  Baseline drift L7          │
│ ...                                                                       │
└──────────────────────────────────────────────────────────────────────────┘
 [Enter] details  [a] ack  [r] resolve  [/] search  [f] filter  [?] help
```

**Behavior:**
- Default sort: severity desc, then created desc.
- `a` → POST ack endpoint, optimistic UI update.
- `r` → POST resolve endpoint with optional comment via inline modal.
- `Enter` → expand inline panel below row showing rule that fired, related signals, suggested actions.

**Data:** `apiClient.ListAlerts(filter)`, refresh every 15s, also pushed via WS if backend supports it (verify; if not, polling only).

### 3.4 Screen 4 — Detection Rules (`screens/rules.go`)

**Layout:** Two-pane.
- Left (1/3 width): rule list (name, layer, status enabled/disabled).
- Right (2/3 width): YAML viewport showing selected rule body, syntax-highlighted.

**Behavior:**
- `↑↓` on left list → updates right pane.
- `e` → write selected rule's YAML to `os.TempDir()`, suspend Bubbletea via `tea.ExecProcess`, launch `$EDITOR` (fallback chain: `$VISUAL` → `$EDITOR` → `vim` → `nano` → `notepad` on Windows). On editor exit, read tmp file, diff vs. original, POST update via API. Show confirmation.
- `t` → toggle rule enabled/disabled.
- `n` → not implemented in v1 (rule creation is GUI-only); stub with a flash message.

**YAML highlighting:** Use `github.com/alecthomas/chroma/v2` (popular, supports YAML, Lipgloss-friendly via 256-color terminal output). Falls back to plain text if terminal can't render.

**Data:** `apiClient.ListRules()`, `apiClient.GetRule(id)`, `apiClient.UpdateRule(id, yaml)`.

### 3.5 Screen 5 — Users / IAM (`screens/users.go`)

**Layout:** Table of users (email, role, last_login, MFA enabled, status).

**Behavior:**
- `i` → open invite modal: textinput for email + role selector (admin / analyst / viewer). Submit → POST invite.
- `Enter` on row → expand inline detail panel; `r` cycles role; `d` deactivates user (with confirmation modal).
- Admin-only screen. If logged-in user lacks admin role, screen renders "ACCESS DENIED — admin role required" and number key 5 is ignored.

**Data:** `apiClient.ListUsers()`, `InviteUser()`, `UpdateUserRole()`.

### 3.6 Screen 6 — Audit Log (`screens/audit.go`)

**Layout:**
```
┌─ Audit Log ──────────────────────────────────────────────────────────────┐
│ filter: user=               action=               last 24h               │
│ ────────────────────────────────────────────────────────────────────────│
│ 10:42:01  admin@argus.io   login_success         ip=10.0.0.5            │
│ 10:38:14  alice@argus.io   rule_updated          rule_id=r_payment_rate │
│ ...                                                                       │
└──────────────────────────────────────────────────────────────────────────┘
 [↑↓] scroll  [/] filter  [u] filter user  [a] filter action  [b] back     │
```

**Behavior:**
- Read-only viewport.
- Filter inputs at top (3 fields: user, action, time window).
- Pagination: load more on scroll-to-bottom.
- Admin-only (same RBAC check as Users screen).

**Data:** `apiClient.ListAuditLog(filter)`.

### 3.7 Phase 3 acceptance

1. All six screens reachable via number keys 1–6.
2. Live signals stream populates within 2s of connection.
3. Trace screen renders 50+ signal trace without scroll glitches.
4. Alert ack/resolve persists (verify in web GUI after action).
5. Rule editor round-trips YAML through `$EDITOR` correctly on Linux/Mac/Windows.
6. RBAC blocks non-admins from screens 5 and 6.

---

## Phase 4 — UX Polish

**Goal:** Make the TUI feel as deliberate as the brutalist web GUI. Help discoverable, keyboard everything, works over SSH and ASCII-only terminals.

### 4.1 Global help overlay

**File: `internal/tui/help.go` (NEW)**

- `?` from any screen → modal overlay listing global keys + current screen's `KeyHelp()`.
- Two columns: shortcut | description.
- `Esc` or `?` again → close.
- Auto-generated content: pulls from each screen's `KeyHelp()` method, so adding a binding in a screen automatically updates the help.

### 4.2 Keymap consolidation

**File: `internal/tui/keys/bindings.go` (NEW)**

Centralize all keys via `bubbles/key`:
- Global: `Quit`, `Help`, `NextScreen`, `PrevScreen`, `Screen1`–`Screen6`, `Refresh`
- Per-screen bindings live in screen file but use the same `key.Binding` type for consistency.

Benefits: single place to retune shortcuts; `key.Help()` formatting is uniform.

### 4.3 SSH / no-unicode fallback

**File: `internal/tui/theme/glyphs.go` (NEW)**

Two glyph sets:
- `Unicode` — ✓ ✗ █ ▶ ▼ │ ─ ┌ ┐ └ ┘
- `ASCII` — `+` `x` `#` `>` `v` `|` `-` `+`

Toggle via:
- CLI flag `argus tui --no-unicode`
- Auto-detect: check `LANG`/`LC_ALL` for UTF-8 hint; if missing, default to ASCII.
- Runtime toggle `Ctrl+U` (rebuilds theme styles).

Lipgloss border styles: switch from `lipgloss.NormalBorder()` to `lipgloss.ASCIIBorder()`.

### 4.4 Quit & exit handling

- `q` from any non-input context → confirmation modal "Quit Argus TUI? [y/N]".
- `Ctrl+C` → immediate exit (no prompt, standard convention).
- On quit: cancel any inflight HTTP requests, close WS connection cleanly, `tea.Quit`.

### 4.5 Status bar polish

Status bar always shows:
- Left: `argus@<api-host>` `user@<email>`
- Center: current screen name
- Right: connection state (`● WS` green / `⟳ POLL` yellow / `✗ OFFLINE` red), pending request indicator (small spinner)

Reuse `bubbles/spinner` for the pending indicator.

### 4.6 Accessibility

- All interactive elements navigable via Tab + Enter (no mouse required).
- 4.5:1 contrast verified against #0A0A0B background — re-check secondary text (#A0A0A0) which is borderline; bump to #B5B5B5 if needed.
- No flashing / animated content (matches "no decorative animations" rule).

### 4.7 Phase 4 acceptance

1. `?` opens help on every screen with screen-correct content.
2. `argus tui --no-unicode` over SSH on a basic xterm renders without box-drawing artifacts.
3. `Ctrl+C` exits cleanly with no goroutine leaks (verify with `go test -run TestTUIShutdown`).
4. All actions across all screens reachable from keyboard alone.

---

## Cross-cutting Concerns

### Testing strategy

Unit tests:
- `cmd/argus/selector/prefs_test.go` — file roundtrip
- `internal/tui/api/client_test.go` — mock backend, verify auth header injection + 401 retry
- `internal/tui/screens/*_test.go` — Bubbletea models tested via `teatest` (charmbracelet/x/exp/teatest)

Manual / integration:
- Per-phase acceptance checklists above.
- SSH compatibility: test against `ssh user@vm argus tui` from a tmux session.

### Observability

- The TUI is a CLI tool; logs go to `~/.argus/tui.log` with daily rotation when `ARGUS_TUI_DEBUG=1`.
- Default: silent. Errors surface in status bar or modal.

### Performance budget

- Startup to login screen: < 200ms.
- Screen switch: < 50ms (lazy-instantiate screens, but cache after first activation).
- WS signal append: < 16ms per message (60fps perception threshold).
- Memory: < 50MB resident with 1000 signals buffered.

### Out of scope (do not build in v1)

- Multi-tab / split-pane layouts within TUI
- Mouse support
- Saved filter presets
- Keybinding customization via config
- Local SQLite cache for offline mode
- Themes beyond the brutalist palette

These are noted here so future "scope creep" PRs have a written reference for "v2-or-later".

---

## Phase Dependency Graph

```
Phase 1 (Selector + Cobra restructure)
   │
   └─► Phase 2 (TUI foundation) — depends on `argus tui` subcommand existing
          │
          └─► Phase 3 (Core 6 screens) — depends on app/theme/api/auth
                 │
                 └─► Phase 4 (Polish) — depends on screens existing to add help/keymap
```

Each phase is independently testable and shippable. Phase 1 alone delivers value (better UX for `argus` CLI even if TUI is a stub). Phase 2 alone gives a working "TUI shell" with login. Phase 3 makes it useful. Phase 4 makes it pleasant.

---

## Open Questions for Implementation Time

1. **Existing WS endpoint format** — what's the exact wire format and auth handshake? (Verify in `internal/api/ws/` or wherever the backend WS lives before writing `internal/tui/api/ws.go`.)
2. **Rule YAML schema** — pull a real rule fixture before designing the YAML pager / editor flow.
3. **Audit log filters** — which fields does the existing `/api/v1/audit` endpoint actually support filtering on? Mirror those exactly.
4. **Setup wizard equivalence** — confirmed scoped out of TUI v1; first-run users are routed to `argus web`.
5. **`argus web` extraction** — does the current `argus server` cleanly separate the web-serve code path from the API-serve code path, or do they share a single HTTP server? If shared, `argus web` is just an alias of `argus server` for now and the rename is cosmetic.

These are not blockers — they're pre-flight checks before each phase.

---

## Files Summary (Net-New Count)

| Phase | New files | Modified files |
|-------|-----------|----------------|
| 1 | 6 | 1 (`cmd/argus/main.go`) |
| 2 | ~14 | 1 (`go.mod`) |
| 3 | 6 | 0 |
| 4 | 3 | minor edits across screens |
| **Total** | **~29 new** | **~2 modified** |

Backend: zero changes. Frontend: zero changes. This is a pure additive client.
