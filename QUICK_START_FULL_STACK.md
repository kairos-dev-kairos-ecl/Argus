# 🚀 Quick Start: Full-Stack Integration Testing

**Phase 5 Complete** — All 6 plans executed, 13 commits, 0 build errors.

---

## 30-Second Setup

### Terminal 1: Start Backend
```bash
./argus.exe --config ./config.yml

# Or via Docker Compose:
docker-compose -f docker-compose-test.yml up -d
```

### Terminal 2: Start Frontend
```bash
cd web && npm run dev

# Opens: http://localhost:5173
```

---

## Test Flow (5 minutes)

1. **Login Page** (`http://localhost:5173/`)
   - Enter test user credentials (from Phase 2)
   - Click "Login"
   - Verify redirect to `/telemetry`

2. **Telemetry Dashboard** (`/telemetry`)
   - Signals should appear in list within 5 seconds
   - Sankey diagram shows layer flows
   - Check DevTools → Network → WS for WebSocket connection
   - New signals update in real-time

3. **Trace Flow** (`/trace` or click signal → "View Trace")
   - Trace loads with spans as React Flow nodes
   - Nodes colored by layer (L1-L10)
   - Click node → details panel shows metadata
   - Press ⌘K → search by layer name

4. **Hunting Console** (`/hunt`)
   - Enter SQL-like query in editor
   - Click "Execute"
   - Results display with status badges
   - Analytics panel shows latency histogram

5. **Audit Ledger** (`/audit`)
   - Alerts load from backend
   - Severity colors applied (red/yellow/gray)
   - Click alert → JSON payload expands
   - Pagination works for browsing history

6. **Error Handling**
   - Stop backend while dashboard is running
   - Verify "Backend offline" banner appears
   - Restart backend → "Retry" button works
   - UI recovers without page reload

---

## Key Integration Points

| Component | Backend Endpoint | Status |
|-----------|-----------------|--------|
| Login | `POST /api/auth/login` | ✅ |
| Token Refresh | `POST /api/auth/refresh` | ✅ |
| Health Check | `GET /health` | ✅ |
| Signals | `GET /api/v1/signals` | ✅ |
| Aggregations | `GET /api/v1/aggregations/*` | ✅ |
| Traces | `GET /api/v1/traces/{id}` | ✅ |
| Query | `POST /api/v1/query` | ✅ |
| Alerts | `GET /api/v1/alerts` | ✅ |
| WebSocket | `WS /ws/signals` | ✅ |

---

## Verify Builds

```bash
# Frontend
cd web && npm run build
# Expected: ✓ built in ~2s, 0 errors

# Backend (if compiled locally)
go build -o argus.exe ./cmd/argus/
# Expected: 0 errors
```

---

## DevTools Checklist

### Network Tab
- [ ] `POST /api/auth/login` returns 200 with token
- [ ] Subsequent requests have `Authorization: Bearer <token>` header
- [ ] `WS /ws/signals` shows 101 Switching Protocols

### Console Tab
- [ ] 0 errors (only warnings allowed)
- [ ] No TypeScript type errors
- [ ] WebSocket messages logged (if debug enabled)

### Application Tab
- [ ] localStorage has `auth_token` and/or `user` keys
- [ ] sessionStorage used as fallback if needed
- [ ] No sensitive data in cookies

---

## Troubleshooting

### Frontend won't load
```bash
# Clear cache
cd web && rm -rf node_modules dist && npm install && npm run dev
```

### Backend connection refused
```bash
# Check if backend is running
curl http://localhost:8080/health

# Check port 8080 is available
netstat -an | grep 8080
```

### WebSocket not connecting
```bash
# Open DevTools → Network → WS tab
# Look for /ws/signals connection attempts
# Check for 401/403 errors (auth issue)
```

### API returning 401 Unauthorized
```bash
# Token may be expired
# Clear localStorage and login again
# Check token in DevTools → Application → localStorage
```

---

## Test Results Template

```
Date: ________________
Tester: ________________
Backend: [ ] Local Binary [ ] Docker Compose
Frontend: [ ] Dev Server [ ] Production Build

Test Scenarios:
[ ] 1. Authentication (8/8 steps pass)
[ ] 2. Telemetry Dashboard (8/8 steps pass)
[ ] 3. Trace Flow (10/10 steps pass)
[ ] 4. Hunting Console (11/11 steps pass)
[ ] 5. Audit Ledger (8/8 steps pass)
[ ] 6. Error Handling (10/10 steps pass)

Build:
[ ] npm run build succeeds (0 errors)
[ ] TypeScript compilation passes
[ ] Production dist/ generated

Overall Result: [ ] PASS [ ] FAIL

Issues Found:
_________________________________
_________________________________

Notes:
_________________________________
_________________________________
```

---

## Full Test Guide

For detailed step-by-step testing with expected outputs:

📄 **`.planning/phases/05-dashboard-integration/FULL_STACK_TEST.md`**

- 6 complete test scenarios
- 50+ test steps with expected results
- Debugging guide
- Performance metrics
- Known limitations

---

## After Testing

### If ALL tests pass ✅
- Phase 5 is COMPLETE
- Move to Phase 6: UI Polish & Accessibility
- Document results in test template above

### If ANY test fails ❌
- Document failure in FAILURES.md
- Create bug fix plan
- Re-run tests after fix

---

## Key Files to Know

- `web/src/services/api.ts` — All API calls go through here
- `web/src/stores/auth.ts` — User session & token management
- `web/src/hooks/useSignalStream.ts` — WebSocket real-time updates
- `web/src/pages/TelemetryPage.tsx` — Dashboard
- `web/src/pages/TracePage.tsx` — Trace visualization
- `web/src/pages/QueryInterface.tsx` — Hunting console
- `web/src/pages/AuditLogPage.tsx` — Alert ledger

---

## Useful Commands

```bash
# Development
npm run dev          # Frontend with hot reload
npm run build        # Production build
npm run preview      # Preview production build locally

# Backend
./argus.exe --config ./config.yml  # Run local
docker-compose -f docker-compose-test.yml up -d  # Run in Docker

# Debugging
curl http://localhost:8080/health    # Backend health
curl http://localhost:8124/?query=SELECT%20COUNT%28*%29%20FROM%20signals  # Signal count
```

---

## You're Ready! 🎯

**Status:** Phase 5 Dashboard Integration — Complete & Ready for Testing

**Commits:** 13 ✓  
**Build:** 0 Errors ✓  
**Frontend:** Vite build passing ✓  
**Backend:** Binary ready ✓

👉 Start with Terminal 1 and Terminal 2 commands above, then follow the 5-minute test flow.

Questions? Check `FULL_STACK_TEST.md` for detailed debugging guide.

---

*Generated: 2026-04-20*  
*Phase: 05-dashboard-integration*  
*Status: READY FOR TESTING*
