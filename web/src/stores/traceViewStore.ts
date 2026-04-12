import { create } from 'zustand'

/**
 * Shared state for timeline ↔ graph synchronization
 * When a span/node is selected in one view, highlight it in the other
 */
interface TraceViewState {
  selectedSpanId: string | null
  selectedNodeId: string | null
  setSelectedSpan: (spanId: string | null) => void
  setSelectedNode: (nodeId: string | null) => void
}

export const useTraceViewStore = create<TraceViewState>((set) => ({
  selectedSpanId: null,
  selectedNodeId: null,
  setSelectedSpan: (spanId: string | null) =>
    set({
      selectedSpanId: spanId,
      // When selecting a span, also select the node with same ID
      selectedNodeId: spanId,
    }),
  setSelectedNode: (nodeId: string | null) =>
    set({
      selectedNodeId: nodeId,
      // When selecting a node, also select the span with same ID
      selectedSpanId: nodeId,
    }),
}))
