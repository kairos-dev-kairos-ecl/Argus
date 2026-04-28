---
plan: 05-05
status: complete
completed: 2026-04-27
wave: 2
---

## Summary

Rebuilt TracePage (Screen 3). React Flow v11 directed graph replacing CSS Gantt bars. Custom SpanNode: 160x48px, layer-colored border (0px radius), 2px border + glow when selected. Edges from parent_span_id → span_id, ArrowClosed markerEnd. Node click loads span into PayloadViewer. Hover shows SpanHud tooltip. Auto-fitView on trace load via ReactFlowInstance ref. Installed reactflow@11.11.4.

## Key Files

### Created
- `web/src/components/trace/PayloadViewer.tsx` — monospace prompt/response display
- `web/src/components/trace/SpanHud.tsx` — fixed-position hover tooltip (TTFT/tokens/GPU)

### Modified
- `web/src/pages/TracePage.tsx` — Full rewrite with React Flow directed graph

## Self-Check: PASSED

All tasks complete. TypeScript clean. React Flow renders. Node click/hover functional.
