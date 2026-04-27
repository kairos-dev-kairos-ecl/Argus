---
phase: 05-dashboard-integration
plan: "08"
subsystem: frontend/incidents
tags: [incidents, mitre-atlas, kill-chain, codemirror, yaml, screen-6]
dependency_graph:
  requires: ["05-01", "05-03", "05-06"]
  provides: ["incidents-screen", "atlas-kill-chain-matrix", "rule-yaml-editor"]
  affects: ["web/src/pages/IncidentsPage.tsx"]
tech_stack:
  added: ["@codemirror/lang-yaml"]
  patterns: ["CodeMirror-yaml-extension", "404-fallback-pattern", "40/60-css-grid"]
key_files:
  created:
    - web/src/services/incidents-service.ts
    - web/src/components/incidents/KillChainMatrix.tsx
    - web/src/components/incidents/RuleYamlEditor.tsx
    - web/src/components/incidents/IncidentInbox.tsx
    - web/src/components/incidents/IncidentDetails.tsx
  modified:
    - web/src/pages/IncidentsPage.tsx
decisions:
  - "404-fallback: fetchIncidents returns [] on 404, caller chains to fetchAlertsAsIncidents"
  - "ATLAS tactic grid uses repeat(auto-fit, minmax(120px,1fr)) — responsive without JS"
  - "IncidentInbox severity chips use inline style severityBg() for mapping 1-4 → token colors"
  - "Removed unused React imports to satisfy verbatimModuleSyntax + TS6133 errors"
metrics:
  duration_minutes: 8
  tasks_completed: 2
  files_created: 5
  files_modified: 1
  completed_date: "2026-04-27"
---

# Phase 5 Plan 8: Incidents MITRE ATLAS Screen (Screen 6) Summary

Rebuilt the Incidents / MITRE ATLAS triage screen from the old scaffolded `IncidentsPage` into a brutalist 40/60 split console with a dense incident inbox, kill chain matrix, and CodeMirror YAML rule editor — all token-driven and zero hex color literals.

---

## What Was Built

### incidents-service.ts

New service providing three exports:

- `fetchIncidents()` — hits `GET /api/v1/incidents?limit=200`; returns `[]` on 404 (endpoint not yet implemented backend-side)
- `fetchAlertsAsIncidents()` — hits `GET /api/v1/alerts?limit=200`; maps alert shape to `Incident` interface
- `createRule(body)` — `POST /api/v1/rules` with JSON body; throws on non-2xx

All three functions inject `Authorization: Bearer {token}` and `X-CSRF-Token` via `authedHeaders()`, matching the pattern in `api.ts`.

**404 fallback chain** (in IncidentsPage):
```
fetchIncidents() → returns [] on 404 → chain to fetchAlertsAsIncidents()
```

### KillChainMatrix.tsx

CSS grid table rendering all 13 MITRE ATLAS tactics in order:
`Reconnaissance, ResourceDevelopment, InitialAccess, MLModelAccess, Execution, Persistence, PrivilegeEscalation, DefenseEvasion, Discovery, Collection, MLAttackStaging, Exfiltration, Impact`

- `gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))'`
- Active tactic: `1px solid var(--color-primary)` border + primary color text
- Non-zero count: `2px solid var(--color-alert)` top border
- Zero hex literals — all via CSS token variables

### RuleYamlEditor.tsx

CodeMirror 6 component with YAML language mode:
- `yaml()` extension from `@codemirror/lang-yaml` (installed as part of this plan)
- `oneDark` theme + custom `EditorView.theme()` overrides for token background/font
- Default YAML template: `detect_prompt_injection` rule
- Footer: name input + SUBMIT button (disabled when name or yaml empty)
- SUBMIT button: `1px solid var(--color-primary)` when enabled, `var(--border-stark)` when disabled

### IncidentInbox.tsx

Dense row list component:
- Each row: `height: '20px'`, `lineHeight: '20px'`, flexbox layout
- Severity chip (40px): C/H/M/L labels with color derived from severity 1-4
- Time column (80px): `created_at.slice(11, 19)` → HH:MM:SS
- Title: flex-1, ellipsis overflow
- ATLAS badge (140px): right-aligned, `var(--color-primary)`, 10px uppercase
- Selected row: `background: var(--color-surface)`, `borderLeft: 2px solid var(--color-primary)`

### IncidentDetails.tsx

Right pane with three stacked sections:
1. **Header**: title in `var(--font-display)` 18px + meta line (severity/layer/status/signals/assigned)
2. **KillChainMatrix**: passes `activeTactic` and `counts` props
3. **RuleYamlEditor**: flex-1, fills remaining vertical space, `onSubmit` wired to `createRule`

Empty state: centered `SELECT AN INCIDENT` in muted uppercase monospace.

### IncidentsPage.tsx (rebuilt)

- `display: grid, gridTemplateColumns: '40% 60%'` layout
- `useEffect` with cancellation guard fetches incidents on mount
- `useMemo` aggregates `counts` (tactic → incident count) from loaded list
- Error banner: `var(--color-alert)` uppercase strip at top of grid when fetch fails

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Unused React imports caused TypeScript build errors**
- **Found during:** Build verification (Task 2)
- **Issue:** `import React from 'react'` in components under `verbatimModuleSyntax: true` caused TS6133 errors since JSX transform doesn't require React in scope
- **Fix:** Removed `React` import from IncidentInbox.tsx, IncidentDetails.tsx, RuleYamlEditor.tsx; linter auto-fixed KillChainMatrix.tsx to `import type React`
- **Files modified:** `IncidentInbox.tsx`, `IncidentDetails.tsx`, `RuleYamlEditor.tsx`
- **Commit:** 2db4f8a

**2. [Rule 3 - Missing dependency] @codemirror/lang-yaml not installed**
- **Found during:** Pre-execution check
- **Issue:** Plan references `@codemirror/lang-yaml` but package was absent from node_modules
- **Fix:** `cd web && npm install @codemirror/lang-yaml --save`
- **Files modified:** `web/package.json`, `web/package-lock.json`
- **Commit:** 1aa51cd

---

## Known Stubs

None — all data fetching wired to real endpoints with fallback; no hardcoded placeholder data.

---

## Self-Check

Checking created files exist:
- `web/src/services/incidents-service.ts` — FOUND
- `web/src/components/incidents/KillChainMatrix.tsx` — FOUND
- `web/src/components/incidents/RuleYamlEditor.tsx` — FOUND
- `web/src/components/incidents/IncidentInbox.tsx` — FOUND
- `web/src/components/incidents/IncidentDetails.tsx` — FOUND
- `web/src/pages/IncidentsPage.tsx` — FOUND (rebuilt)

Checking commits exist:
- `1aa51cd` — Task 1 (incidents-service + KillChainMatrix + RuleYamlEditor)
- `2db4f8a` — Task 2 (IncidentInbox + IncidentDetails + IncidentsPage)

Build: `cd web && npm run build` — exits 0 (verified).

## Self-Check: PASSED
