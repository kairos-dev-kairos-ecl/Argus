import { useState, useEffect } from 'react'
import { getTrace } from '../services/api'
import type { Span, Layer } from '../types/index'
import type { TraceResponse } from '../services/types'
import { colors } from '../lib/design-tokens'

/**
 * React Flow Node type for trace visualization
 */
export interface TraceFlowNode {
  id: string
  data: {
    label: string
    span: Span
    layer: Layer
    status: 'ok' | 'error'
  }
  position: { x: number; y: number }
  style?: Record<string, unknown>
}

/**
 * React Flow Edge type for trace relationships
 */
export interface TraceFlowEdge {
  id: string
  source: string
  target: string
  animated: boolean
}

/**
 * Hook for fetching and visualizing trace data as React Flow nodes/edges.
 *
 * Returns:
 * - trace: Trace | null — The complete trace object with spans and detections
 * - nodes: TraceFlowNode[] — React Flow nodes computed from spans
 * - edges: TraceFlowEdge[] — React Flow edges showing parent-child relationships
 * - loading: boolean — True while fetching trace
 * - error: string | null — Error message if fetch failed
 * - selectedNode: string | null — ID of currently selected node (span signal_id)
 * - setSelectedNode: (nodeId: string | null) => void — Update selection
 *
 * Usage:
 * ```
 * const { trace, nodes, edges, loading, error, selectedNode, setSelectedNode } = useTraceData(traceId)
 * ```
 */
export function useTraceData(traceId: string) {
  const [trace, setTrace] = useState<TraceResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [nodes, setNodes] = useState<TraceFlowNode[]>([])
  const [edges, setEdges] = useState<TraceFlowEdge[]>([])

  useEffect(() => {
    const fetchAndTransform = async () => {
      try {
        setLoading(true)
        setError(null)

        // Fetch trace from API
        const traceData = await getTrace(traceId)
        setTrace(traceData)

        // Transform spans into React Flow nodes
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const computedNodes = transformSpansToNodes(traceData.spans as any)
        setNodes(computedNodes)

        // Transform span relationships into edges
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const computedEdges = transformSpansToEdges(traceData.spans as any)
        setEdges(computedEdges)
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : typeof err === 'object' && err !== null && 'message' in err
              ? String((err as any).message)
              : 'Failed to load trace'

        // Check for specific error types
        if (message.includes('404')) {
          setError('Trace not found')
        } else if (message.includes('Failed to load trace')) {
          setError('Failed to load trace')
        } else if (message.includes('Network')) {
          setError('Network error - backend may be offline')
        } else {
          setError(message)
        }

        setTrace(null)
        setNodes([])
        setEdges([])
      } finally {
        setLoading(false)
      }
    }

    if (traceId && traceId.trim()) {
      fetchAndTransform()
    } else {
      setLoading(false)
      setTrace(null)
      setNodes([])
      setEdges([])
    }
  }, [traceId])

  return {
    trace,
    nodes,
    edges,
    loading,
    error,
    selectedNode,
    setSelectedNode,
  }
}

/**
 * Transform spans into React Flow nodes with layer colors and positioning.
 *
 * Nodes are positioned vertically by layer (L1 at top, L10 at bottom).
 * Each node is styled with the layer's designated color.
 */
function transformSpansToNodes(spans: Span[]): TraceFlowNode[] {
  const layerMap: Record<Layer, number> = {
    L1_HARDWARE: 0,
    L2_MODEL_WEIGHTS: 1,
    L3_TOKENIZER: 2,
    L4_TRANSFORMER: 3,
    L5_OUTPUT_DECODING: 4,
    L6_SAFETY: 5,
    L7_RAG_RETRIEVAL: 6,
    L8_AGENTS: 7,
    L9_API_GATEWAY: 8,
    L10_APPLICATION: 9,
  }

  const layerColors: Record<Layer, string> = {
    L1_HARDWARE: colors.layer.l1,
    L2_MODEL_WEIGHTS: colors.layer.l2,
    L3_TOKENIZER: colors.layer.l3,
    L4_TRANSFORMER: colors.layer.l4,
    L5_OUTPUT_DECODING: colors.layer.l5,
    L6_SAFETY: colors.layer.l6,
    L7_RAG_RETRIEVAL: colors.layer.l7,
    L8_AGENTS: colors.layer.l8,
    L9_API_GATEWAY: colors.layer.l9,
    L10_APPLICATION: colors.layer.l10,
  }

  // Group spans by layer for positioning
  const spansByLayer: Map<Layer, Span[]> = new Map()
  spans.forEach((span) => {
    if (!spansByLayer.has(span.layer)) {
      spansByLayer.set(span.layer, [])
    }
    spansByLayer.get(span.layer)!.push(span)
  })

  // Create nodes with vertical positioning
  const nodes: TraceFlowNode[] = []
  const layerOrder = Object.keys(layerMap) as Layer[]

  layerOrder.forEach((layer) => {
    const spansInLayer = spansByLayer.get(layer) || []
    const yPosition = layerMap[layer] * 120 // 120px vertical spacing per layer

    spansInLayer.forEach((span, index) => {
      const xPosition = index * 250 // 250px horizontal spacing for siblings

      const node: TraceFlowNode = {
        id: span.signal_id,
        data: {
          label: `${layer}`,
          span,
          layer: span.layer,
          status: span.status,
        },
        position: { x: xPosition, y: yPosition },
        style: {
          background: layerColors[layer],
          color: '#FFFFFF',
          border: `2px solid ${layerColors[layer]}`,
          borderRadius: '8px',
          padding: '12px 16px',
          fontSize: '12px',
          fontWeight: 600,
          minWidth: '120px',
          textAlign: 'center',
          cursor: 'pointer',
          transition: 'all 0.2s ease',
          opacity: span.status === 'error' ? 0.7 : 1,
          filter: span.status === 'error' ? 'brightness(0.9)' : 'none',
        },
      }

      nodes.push(node)
    })
  })

  return nodes
}

/**
 * Transform span parent-child relationships into React Flow edges.
 *
 * For each span with a parent_signal_id, create an edge from parent to child.
 */
function transformSpansToEdges(spans: Span[]): TraceFlowEdge[] {
  const edges: TraceFlowEdge[] = []
  const spanIds = new Set(spans.map((s) => s.signal_id))

  spans.forEach((span) => {
    if (span.parent_signal_id && spanIds.has(span.parent_signal_id)) {
      const edge: TraceFlowEdge = {
        id: `${span.parent_signal_id}-to-${span.signal_id}`,
        source: span.parent_signal_id,
        target: span.signal_id,
        animated: false,
      }
      edges.push(edge)
    }
  })

  return edges
}
