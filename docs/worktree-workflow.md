# Argus XDR — Git Worktree Workflow

This document describes the intended git branching model and worktree structure for ongoing development of Argus XDR. It is a design document, not a script — follow the conventions here when doing any sustained feature work.

---

## Branch Model

Two long-lived branches:

| Branch | Worktree path | Purpose |
|--------|--------------|---------|
| `main` | repo root (`C:/Users/Drupad/ArgusXDR`) | Clean public release branch. Always buildable. No in-progress work. |
| `dev`  | `C:/Users/Drupad/ArgusXDR-dev` (or `.worktrees/dev`) | Active development. Features branch from here, PRs target here. |

`main` is the face of the project to the public. It should never contain work-in-progress commits, debug artifacts, or half-built features. The rule: **if you wouldn't put it in a release note, it doesn't go directly to `main`.**

`dev` is where real work happens. It accumulates completed feature branches, hotfixes, and documentation updates. It runs CI on every push. When a meaningful milestone is complete and CI is green, `dev` is merged to `main` via a reviewed PR.

---

## Worktree Setup

Git worktrees let you check out multiple branches simultaneously on disk without cloning the repo twice. The `main` worktree lives at the repo root; the `dev` worktree is a separate directory linked to the same `.git` object store.

```bash
# One-time setup: create the dev worktree
git worktree add ../ArgusXDR-dev dev

# Verify both are registered
git worktree list
# /path/to/ArgusXDR       <sha>  [main]
# /path/to/ArgusXDR-dev   <sha>  [dev]
```

After this, `main` and `dev` are independent working directories sharing a single git history. You can build, run, and edit them independently.

---

## Day-to-Day Development Flow

### Starting a new feature

All feature work originates from `dev`, never from `main`.

```
cd ../ArgusXDR-dev            # enter the dev worktree
git pull origin dev           # make sure you're current
git checkout -b feature/my-thing
# ... do the work ...
git push origin feature/my-thing
```

Open a PR targeting `dev` (not `main`). Get it reviewed. Merge it to `dev` when CI is green.

### What goes into `dev`

- New features (completed, tested)
- Bug fixes
- Documentation updates
- Performance work
- Dependency upgrades

### What never goes directly into `dev` without a PR

- Large architectural changes — always branch and PR even if solo
- Anything touching `proto/` — proto changes are breaking changes, treat them with care
- Auth/security changes — require an explicit review checkpoint

---

## Merging `dev` → `main`

`main` is updated only when a logical milestone is complete. The criteria:

1. CI is fully green on `dev`
2. All UAT tests for the milestone pass
3. CHANGELOG entry written
4. No known blockers remain

The merge is a **non-fast-forward merge commit** so the milestone boundary is visible in the graph:

```bash
cd /path/to/ArgusXDR          # main worktree
git fetch origin
git merge --no-ff origin/dev -m "chore: merge dev → main for v1.x milestone"
git push origin main
```

Then open a GitHub release pointing at the new `main` HEAD.

---

## PR Hygiene

**Title format:** `<type>: <short description>` where type is one of:
- `feat` — new capability
- `fix` — bug fix
- `chore` — tooling, deps, refactoring (no behaviour change)
- `docs` — documentation only
- `perf` — performance improvement
- `test` — tests only

**Body must include:**
- What changed and why (not just what)
- How it was tested (manual steps or test reference)
- Any follow-up items noted as `TODO:` comments or linked issues

**Review checklist before merging to `dev`:**
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes (no skipped tests without explanation)
- [ ] No secrets or credentials in diff
- [ ] Proto changes are backward compatible (or migration notes added)
- [ ] Signals produced by the change are documented (layer, category, fields)

**Review checklist before merging to `main`:**
- All of the above, plus:
- [ ] CHANGELOG entry present
- [ ] Version tag planned
- [ ] Helm chart values updated if new config was introduced

---

## What Lives in Each Worktree

### `main` (repo root)

The clean release. Contains:
- All source code at the last release state
- `README.md`, `LICENSE`, `CONTRIBUTING.md`
- `docs/` — stable, published documentation
- `proto/`, `gen/`, `cmd/`, `internal/`, `web/`, `sdk/`
- `docker-compose.yml`, `Dockerfile`, `Makefile`
- `migrations/` — all applied migrations

Does **not** contain:
- `Figma/` design tooling (excluded via `.gitignore`)
- `graphify-out/` graph cache (excluded via `.gitignore`)
- `.claude/`, `.planning/` GSD workflow artifacts (excluded via `.gitignore`)
- Any `*.local.*` config files with credentials

### `dev` worktree

Same source layout as `main`, plus whatever is being actively built. The `dev` branch may have commits that are ahead of `main` by days or weeks — that's expected.

---

## Worktree Cleanup

When a feature branch is fully merged to `dev`:

```bash
# Delete the remote branch
git push origin --delete feature/my-thing

# Delete the local branch (from either worktree)
git branch -d feature/my-thing
```

When a worktree is no longer needed (e.g., an ad-hoc debugging worktree):

```bash
git worktree remove /path/to/worktree
git worktree prune
```

**Do not leave stale worktrees registered.** `git worktree list` should only show `main` and `dev` under normal conditions. Additional worktrees for specific purposes (e.g., a hotfix worktree) should be created and removed within the same working session where possible.

---

## Stale Branch Policy

A branch is stale if:
- It has been merged to `dev` or `main`, or
- It has had no commits in 30 days and is not linked to an open issue

Stale branches are deleted without ceremony. If something was valuable in a stale branch, cherry-pick the commit — don't keep the branch alive as an archive.

To audit:
```bash
# Branches merged into dev
git branch --merged origin/dev | grep -v "^\*\|main\|dev"

# Branches with no activity in 30 days
git for-each-ref --sort=committerdate refs/remotes/origin --format='%(committerdate:short) %(refname:short)' | head -20
```

---

## CI Integration Notes

CI (GitHub Actions) runs on:
- Every push to `dev`
- Every PR opened against `dev` or `main`
- Every push to `main`

The pipeline: lint → build → unit tests → integration tests (docker compose up) → image build.

CI must be green before any PR is merged. The `main` branch has a branch protection rule: no direct pushes, all merges require a passing status check.

---

## Summary

| Rule | Short form |
|------|-----------|
| Feature work | Branch from `dev`, PR back to `dev` |
| Hotfixes | Branch from `main`, PR to both `main` and `dev` |
| Release | Merge `dev` → `main` via non-ff merge, tag |
| Cleanup | Delete merged branches within the same week |
| Stale worktrees | Remove same session or within one day |
| Secrets | Never in any branch, ever — use `argus.yaml.local` (gitignored) |
