---
quick_task: 260508-n7t
phase: quick
plan: n7t
subsystem: tui
completed: "2026-05-08"
duration_seconds: 848
tasks_completed: 3
files_created: 26
files_modified: 2
tags: [tui, bubbletea, lipgloss, auth, security, websocket]
dependency_graph:
  requires: [260508-mt2]
  provides: [TUI-P2-ROOT, TUI-P2-THEME, TUI-P2-AUTH, TUI-P2-API, TUI-P2-WS, TUI-P2-COMPONENTS, TUI-P2-LOGIN]
  affects: [cmd/argus/tui]
tech_stack:
  added:
    - github.com/charmbracelet/bubbles v0.20.0
  patterns:
    - tea.Model with separate app.go/update.go/view.go
    - JWT-in-memory AuthState with mutex protection
    - 401 refresh-retry-once via flag (not recursion)
    - WS auth via Authorization header on Upgrade (never query param)
key_files:
  created:
    - cmd/argus/tui/theme/theme.go
    - cmd/argus/tui/theme/glyphs.go
    - cmd/argus/tui/keys/bindings.go
    - cmd/argus/tui/components/statusbar.go
    - cmd/argus/tui/components/table.go
    - cmd/argus/tui/components/viewport.go
    - cmd/argus/tui/components/badge.go
    - cmd/argus/tui/components/modal.go
    - cmd/argus/tui/components/spinner.go
    - cmd/argus/tui/auth/state.go
    - cmd/argus/tui/auth/client.go
    - cmd/argus/tui/api/types.go
    - cmd/argus/tui/api/client.go
    - cmd/argus/tui/api/ws.go
    - cmd/argus/tui/app.go
    - cmd/argus/tui/update.go
    - cmd/argus/tui/view.go
    - cmd/argus/tui/screens/login.go
    - cmd/argus/tui/screens/placeholder.go
    - cmd/argus/tui/theme/theme_test.go
    - cmd/argus/tui/keys/keys_test.go
    - cmd/argus/tui/components/statusbar_test.go
    - cmd/argus/tui/auth/client_test.go
    - cmd/argus/tui/api/client_test.go
    - cmd/argus/tui/api/ws_test.go
    - cmd/argus/tui/screens/login_test.go
  modified:
    - cmd/argus/tui/cmd.go
    - cmd/argus/main_test.go
    - go.mod
    - go.sum
decisions:
  - "bubbles v0.20.0 selected (matches bubbletea v0.27.1 compatibility)"
  - "JWT stored in private AuthState fields with no serialization methods"
  - "APIClient 401 retry uses sync flag approach, not recursion, to bound retries at exactly 1"
  - "WSClient reconnect capped at 5 attempts with exponential backoff + jitter"
  - "Phase 1 TestTUICmd_PrintsPhase2Message updated to TestTUICmd_IsWired (real TUI blocks on TTY)"
---

# Phase quick Plan n7t: TUI Phase 2 — Bubbletea Root Model + Lipgloss Theme Summary

**One-liner:** Bubbletea root model with brutalist Lipgloss theme, JWT-in-memory auth client, HTTP+WS API client with 401-refresh-retry, and a working login screen with MFA branch — Phase 3 operator screens can now plug in.

---

## What Was Built

### Task 1: Theme + Component Library + Keymap + go.mod bubbles dep

**go.mod:** Added `github.com/charmbracelet/bubbles v0.20.0` (compatible with bubbletea v0.27.1).

**`cmd/argus/tui/theme/theme.go`:** Brutalist dark theme constants matching CLAUDE.md design system tokens. Exported Lipgloss style values (`Background`, `Border`, `Title`, `Subtitle`, `Muted`, `Code`, `Header`, `StatusBar`, `Panel`, `PanelFocused`, `PanelError`, `Subtle`, `Emphasis`, `ErrorText`). Functions `LayerBadge(layer int)` and `SeverityBadge(sev string)` returning correctly-colored bold styles. Phase 3 `$EDITOR` security constraints documented in the header comment.

**`cmd/argus/tui/theme/glyphs.go`:** `Glyphs` struct with Unicode/ASCII constructors; `SetASCII(bool)` package-level toggle; `GetGlyphs()` accessor.

**`cmd/argus/tui/keys/bindings.go`:** `Bindings` struct with `New()` constructor. All global keys: `ctrl+c` (quit immediate), `q` (quit confirm), `?` (help), `tab`/`shift+tab` (screen cycle), `1-6` (direct screen select), `r` (refresh), `enter` (submit), `esc` (back). `Help() []key.Binding` for overlay rendering.

**Components (`cmd/argus/tui/components/`):**
- `statusbar.go`: `StatusBar{width}` with `Render(left, center, right)` producing exactly `width` visible runes
- `table.go`: bubbles/table wrapper with Argus header/selected styles
- `viewport.go`: bubbles/viewport with Panel border + scroll percentage
- `badge.go`: `Badge{style}` with `Render("[text]")`
- `modal.go`: centered `lipgloss.Place` overlay with PanelFocused border
- `spinner.go`: bubbles/spinner with Subtle style

**Tests:** 7 passing (LayerBadge non-empty, LayerBadge colors, SeverityBadge colors, Glyph toggle, StatusBar exact-width, Help non-empty, all keys present).

---

### Task 2: Auth State + Auth Client + HTTP API Client + WS Client

**`cmd/argus/tui/auth/state.go`:** `AuthState` with private fields (accessToken, refreshToken, expiresAt, email, role), `sync.RWMutex` protection, `Set()`, `Bearer()`, `RefreshToken()`, `ExpiresAt()`, `Email()`, `Role()`, `IsAuthenticated()`, `ClearOnLogout()`. No serialization methods; no JSON/YAML tags.

**`cmd/argus/tui/auth/client.go`:** `Login()`, `VerifyMFA()`, `RefreshTokens()`, `Logout()` against the 4 auth endpoints. `Logout()` uses `defer state.ClearOnLogout()` to always clear even on HTTP failure.

**`cmd/argus/tui/api/types.go`:** DTOs — `User`, `Signal`, `Alert`, `Rule`, `AuditEntry`, `WSMsg`, `APIError`.

**`cmd/argus/tui/api/client.go`:** `Client` with `Get()`, `Post()`, `Delete()` wrapping `do()`. `do()` injects `Authorization: Bearer <token>` from state, handles 401 by calling `RefreshTokens()` once (via flag, not recursion), then retries. If refresh fails, `ClearOnLogout()` and return `ErrUnauthenticated`. Token never appears in URL query params.

**`cmd/argus/tui/api/ws.go`:** `WSClient.Dial()` uses `gorilla/websocket.Dialer.DialContext()` with `http.Header{"Authorization": "Bearer ..."}`. `Messages() <-chan WSMsg` channel. Auto-reconnect with exponential backoff (500ms initial, 30s cap, ±20% jitter, max 5 attempts).

**Security tests (all pass):**
- `TestAPIClient_AuthHeaderNotQueryParam` — bearer in header, URL has no `token=`
- `TestAPIClient_401_RefreshAndRetry` — exactly 1 refresh + 1 retry
- `TestAPIClient_401_RefreshFail_ClearsState` — state cleared, `ErrUnauthenticated` returned
- `TestWSClient_AuthHeaderOnUpgrade` — `Authorization: Bearer` in Upgrade headers, no `?token=`
- `TestAuthState_ClearOnLogout_ZeroesFields` — all fields zeroed
- `TestAuthClient_NeverWritesTokenToDisk` — no files created in `~/.argus/` during Login

---

### Task 3: AppModel Root + Login Screen + Placeholder Screens + Help Overlay + cmd.go Wiring

**`cmd/argus/tui/app.go`:** `AppModel` root model with `Screen` enum (ScreenLogin, ScreenSignals, ScreenTrace, ScreenAlerts, ScreenRules, ScreenUsers, ScreenAudit). `New(cfg Config)` constructor wires auth state, auth client, API client, login screen, 6 placeholder screens. `Init()` returns login screen cursor blink.

**`cmd/argus/tui/update.go`:** Global key dispatch — `ctrl+c` quits immediately from any screen. On operator screens: `q` triggers quit confirm modal; `?` toggles help overlay; `tab`/`shift+tab` cycle screens; `1-6` switch directly; `esc` closes overlay. On login screen: all keys except `ctrl+c` forwarded to LoginModel. `LoginSuccessMsg` transitions to ScreenSignals and populates auth state.

**`cmd/argus/tui/view.go`:** Renders header (1 line) + activeScreen.View() + statusBar (1 line). Status bar shows `argus@localhost` / screen title / `anon` (unauthenticated) or `email@role` (authenticated). Help overlay uses `components.Modal` centered on the screen. Quit confirm overlay uses `components.Modal`.

**`cmd/argus/tui/screens/login.go`:** `LoginModel` with two `bubbles/textinput.Model` (email, password with `EchoPassword`/`•`). Tab cycles focus. Enter submits. MFA branch: `LoginMFAMsg` transitions to 6-digit OTP entry on the same screen. `LoginSuccessMsg` emitted on success. `ErrUnauthenticated` shown inline in `theme.ErrorText`.

**`cmd/argus/tui/screens/placeholder.go`:** `PlaceholderModel{Name}` renders a centered panel: `"{Name} — coming in Phase 3"`.

**`cmd/argus/tui/cmd.go`:** Replaced Phase 1 stub with `tea.NewProgram(New(cfg), tea.WithAltScreen()).Run()`. BaseURL from `viper.GetString("api.url")` with `http://localhost:8080` fallback.

**Login tests (all pass):** Tab cycles focus, Enter on filled form produces non-nil Cmd, raw password not visible in View(), MFA branch activated by `LoginMFAMsg`.

---

## Module Path

`github.com/argusxdr/argus/cmd/argus/tui/...`

---

## Test Results

```
go test ./cmd/argus/tui/... -timeout 60s
ok  github.com/argusxdr/argus/cmd/argus/tui/api        (5 tests)
ok  github.com/argusxdr/argus/cmd/argus/tui/auth       (7 tests)
ok  github.com/argusxdr/argus/cmd/argus/tui/components (1 test)
ok  github.com/argusxdr/argus/cmd/argus/tui/keys       (2 tests)
ok  github.com/argusxdr/argus/cmd/argus/tui/screens    (4 tests)
ok  github.com/argusxdr/argus/cmd/argus/tui/theme      (4 tests)

go test ./cmd/argus/... -timeout 30s
ok  github.com/argusxdr/argus/cmd/argus        (Phase 1 dispatch tests still pass)
ok  github.com/argusxdr/argus/cmd/argus/selector
```

Total: 23 new tests + all Phase 1 tests still green.

---

## Security Audit Results

All 5 constraints verified:

1. **No token disk writes:** `auth/state.go` has no `MarshalJSON`, `MarshalYAML`, YAML tags, or `os.WriteFile` calls. `TestAuthClient_NeverWritesTokenToDisk` passes.
2. **WS auth via header:** `ws.go` passes `http.Header{"Authorization": "Bearer ..."}` to `DialContext`. `TestWSClient_AuthHeaderOnUpgrade` verifies header is set and URL has no `token=`.
3. **Refresh clears state:** `ClearOnLogout()` is deferred in `Logout()`; called in `APIClient.do()` on refresh failure. `TestAPIClient_401_RefreshFail_ClearsState` passes.
4. **No sensitive data in logs:** `grep -rE 'zap\.(Info|Sugar).*[Tt]oken' cmd/argus/tui` returns no matches.
5. **Phase 3 $EDITOR constraints:** Documented in `theme/theme.go` header comment (os.MkdirTemp under home, 0700/0600, exec.Command not sh -c).

---

## Commits

| # | Hash | Message |
|---|------|---------|
| 1 | da8cd2b | feat(quick-260508-n7t): TUI Phase 2 task 1 — theme, glyphs, keys, component library |
| 2 | 49f1c97 | feat(quick-260508-n7t): TUI Phase 2 task 2 — auth state, auth client, API client, WS client |
| 3 | 463c48e | feat(quick-260508-n7t): TUI Phase 2 task 3 — AppModel root, login, placeholder screens, cmd wiring |

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Phase 1 TestTUICmd_PrintsPhase2Message incompatible with real TUI**
- **Found during:** Task 3 verification
- **Issue:** The Phase 1 test called `tui.Cmd.RunE(tui.Cmd, nil)` expecting the stub to print a message and return. Phase 2 replaces this with `tea.NewProgram(...).Run()` which blocks waiting for TTY input — hanging the test suite for 60s.
- **Fix:** Updated `TestTUICmd_PrintsPhase2Message` → `TestTUICmd_IsWired` which asserts `tui.Cmd.RunE != nil` and `tui.Cmd.Use == "tui"`. Real execution coverage is provided by the dispatch tests that use `stubRunE` to intercept `RunE` without launching the program.
- **Files modified:** `cmd/argus/main_test.go`
- **Commit:** 463c48e

**2. [Rule 3 - Blocking] lipgloss.WithWhitespaceStyle doesn't exist in v1.1.0**
- **Found during:** Task 1 build
- **Issue:** `WithWhitespaceStyle` not present in lipgloss v1.1.0 API. Used in modal.go overlay.
- **Fix:** Replaced with `WithWhitespaceForeground(lipgloss.Color(theme.ColorSecondary))`.
- **Files modified:** `cmd/argus/tui/components/modal.go`
- **Commit:** da8cd2b

**3. [Rule 3 - Blocking] Missing go.sum entry for github.com/atotto/clipboard**
- **Found during:** Task 3 (bubbles/textinput import)
- **Issue:** `go.sum` was missing the clipboard dep transitively required by `bubbles/textinput`.
- **Fix:** Ran `go mod tidy` to add the missing entries.
- **Commit:** 463c48e

---

## Phase 3 Readiness Checklist

- [x] Login screen functional: email/password textinputs, masked password, Tab cycle, Enter submit
- [x] MFA branch: `LoginMFAMsg` transitions to OTP entry on same screen
- [x] AuthState populated on login: `Bearer()`, `Email()`, `Role()` accessible
- [x] APIClient ready: `Get/Post/Delete` with bearer injection + 401 refresh-retry
- [x] WSClient ready: `Dial()` with Authorization header, `Messages()` channel, auto-reconnect
- [x] Theme constants: all Layer/Severity badges, all Lipgloss style exports
- [x] Component library: StatusBar, Table, Viewport, Badge, Modal, Spinner
- [x] Key bindings: all global keys defined, `Help()` slice for overlay
- [x] Screen routing: 1-6 keys, tab/shift+tab, Login → ScreenSignals on success
- [x] Help overlay: `?` toggle, modal over dimmed screen
- [x] Quit confirm: `q` modal, `y` quits, any other key cancels
- [x] Status bar: `argus@host` / screen title / `anon` or `email@role`
- [x] Phase 3 $EDITOR constraints documented in theme/theme.go
- [x] All TUI tests pass (23 tests)
- [x] All Phase 1 tests still pass (no regressions)
- [x] go build ./... PASS

## Self-Check: PASSED
