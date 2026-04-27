import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import ReactFlow, {
  Controls,
  Handle,
  Position,
  useNodesState,
  useEdgesState,
  MarkerType,
} from 'reactflow'
import type { Node, Edge, NodeTypes, ReactFlowInstance, NodeProps } from 'reactflow'
import 'reactflow/dist/style.css'
import type { TraceResponse, TraceSpan } from '../services/types'
import { getTrace, getSignals } from '../services/api'
import { PayloadViewer } from '../components/trace/PayloadViewer'
import { SpanHud } from '../components/trace/SpanHud'

// ── Layer colors ──────────────────────────────────────────────────────────────

const LAYER_COLORS: Record<number, string> = {
  1: '#EF4444', 2: '#F97316', 3: '#F59E0B', 4: '#EAB308',
  5: '#84CC16', 6: '#22C55E', 7: '#14B8A6', 8: '#06B6D4',
  9: '#3B82F6', 10: '#F43F5E',
}

// ── Custom node component ─────────────────────────────────────────────────────

interface SpanNodeData {
  span: TraceSpan
}

function SpanNode({ data, selected }: NodeProps<SpanNodeData>) {
  const { span } = data
  const color = LAYER_COLORS[span.layer] ?? '#888'

  return (
    <>
      <Handle
        type="target"
        position={Position.Left}
        style={{ background: color, border: 'none', width: 6, height: 6 }}
      />
      <div
        style={{
          width: '160px',
          height: '48px',
          background: 'var(--color-surface, #111216)',
          border: `${selected ? 2 : 1}px solid ${color}`,
          boxShadow: selected ? `0 0 8px ${color}60` : 'none',
          borderRadius: '0px',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          padding: '0 10px',
          cursor: 'pointer',
          fontFamily: "'JetBrains Mono', monospace",
          userSelect: 'none',
        }}
      >
        <div style={{ fontSize: '10px', fontWeight: 700, color, letterSpacing: '0.04em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          L{span.layer} · {span.name}
        </div>
        <div style={{ fontSize: '9px', color: '#6c757d', marginTop: '3px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {span.duration_ms}ms
          {span.ttft_ms != null ? ` · TTFT ${span.ttft_ms}ms` : ''}
          {span.tokens != null ? ` · ${span.tokens}tok` : ''}
        </div>
      </div>
      <Handle
        type="source"
        position={Position.Right}
        style={{ background: color, border: 'none', width: 6, height: 6 }}
      />
    </>
  )
}

// nodeTypes must be stable (defined outside component)
const nodeTypes: NodeTypes = { spanNode: SpanNode }

// ── Graph builder ─────────────────────────────────────────────────────────────

function buildFlowGraph(spans: TraceSpan[]): { nodes: Node[]; edges: Edge[] } {
  if (!spans.length) return { nodes: [], edges: [] }

  // Group by layer for vertical positioning
  const layerGroups: Record<number, number> = {}
  const layerCounts: Record<number, number> = {}
  for (const span of spans) {
    if (!(span.layer in layerCounts)) layerCounts[span.layer] = 0
    layerGroups[span.span_id as unknown as number] = layerCounts[span.layer]
    layerCounts[span.layer]++
  }

  // Build index for y positions
  const layerIdx: Record<string, number> = {}
  const seenLayers: Record<number, number> = {}
  for (const span of spans) {
    if (!(span.layer in seenLayers)) seenLayers[span.layer] = 0
    layerIdx[span.span_id] = seenLayers[span.layer]++
  }

  const nodes: Node<SpanNodeData>[] = spans.map((span) => ({
    id: span.span_id,
    type: 'spanNode',
    position: {
      x: (span.layer - 1) * 220,
      y: (layerIdx[span.span_id] ?? 0) * 64,
    },
    data: { span },
  }))

  // Build set of valid span IDs for edge validation
  const spanIds = new Set(spans.map((s) => s.span_id))

  const edges: Edge[] = spans
    .filter((s) => s.parent_span_id && spanIds.has(s.parent_span_id))
    .map((span) => ({
      id: `e-${span.parent_span_id}-${span.span_id}`,
      source: span.parent_span_id!,
      target: span.span_id,
      style: { stroke: '#343A40', strokeWidth: 1 },
      markerEnd: { type: MarkerType.ArrowClosed, color: '#343A40', width: 12, height: 12 },
    }))

  return { nodes, edges }
}

// ── TracePage ─────────────────────────────────────────────────────────────────

export function TracePage() {
  const { traceId } = useParams<{ traceId?: string }>()
  const navigate = useNavigate()

  const [traceList, setTraceList] = useState<string[]>([])
  const [traceSearch, setTraceSearch] = useState('')
  const [trace, setTrace] = useState<TraceResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<TraceSpan | null>(null)
  const [hud, setHud] = useState<{ span: TraceSpan; x: number; y: number } | null>(null)

  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])
  const flowInstanceRef = useRef<ReactFlowInstance | null>(null)

  // Populate trace list from recent signals on mount
  useEffect(() => {
    getSignals({ limit: 500 })
      .then((res) => {
        const ids: string[] = []
        const seen = new Set<string>()
        for (const sig of (res.signals ?? [])) {
          const tid = (sig as any).trace_id as string | undefined
          if (tid && !seen.has(tid)) {
            seen.add(tid)
            ids.push(tid)
          }
        }
        setTraceList(ids)
      })
      .catch(() => { /* signals unavailable — start with empty list */ })
  }, [])

  // Add current URL trace to the list if not already present
  useEffect(() => {
    if (traceId) setTraceList((prev) => prev.includes(traceId) ? prev : [traceId, ...prev])
  }, [traceId])

  useEffect(() => {
    if (!traceId) return
    setLoading(true)
    setError(null)
    setSelectedSpan(null)
    setHud(null)
    getTrace(traceId)
      .then((t) => {
        setTrace(t)
        const { nodes: n, edges: e } = buildFlowGraph(t.spans)
        setNodes(n)
        setEdges(e)
        if (t.spans.length > 0) setSelectedSpan(t.spans[0])
        // Fit view after layout
        setTimeout(() => flowInstanceRef.current?.fitView({ padding: 0.15 }), 50)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load trace'))
      .finally(() => setLoading(false))
  }, [traceId, setNodes, setEdges])

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node<SpanNodeData>) => {
      setSelectedSpan(node.data.span)
    },
    []
  )

  const handleNodeMouseEnter = useCallback(
    (event: React.MouseEvent, node: Node<SpanNodeData>) => {
      setHud({ span: node.data.span, x: event.clientX + 12, y: event.clientY + 4 })
    },
    []
  )

  const handleNodeMouseLeave = useCallback(() => {
    setHud(null)
  }, [])

  // Keep React Flow selection in sync with selectedSpan
  useEffect(() => {
    if (!selectedSpan) return
    setNodes((nds) =>
      nds.map((n) => ({ ...n, selected: n.id === selectedSpan.span_id }))
    )
  }, [selectedSpan, setNodes])

  return (
    <div style={{
      display: 'flex', height: '100vh',
      background: 'var(--color-background)', color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)', overflow: 'hidden',
    }}>
      {/* ── Trace list sidebar ── */}
      <div style={{
        width: '200px', flexShrink: 0, borderRight: 'var(--border-stark)',
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        <div style={{
          padding: '8px 12px', borderBottom: 'var(--border-stark)',
          fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)',
          letterSpacing: '0.08em',
        }}>
          TRACES ({traceList.length})
        </div>
        <div style={{ padding: '6px 8px', borderBottom: 'var(--border-stark)' }}>
          <input
            type="text"
            placeholder="search trace id..."
            value={traceSearch}
            onChange={(e) => setTraceSearch(e.target.value)}
            style={{
              width: '100%',
              background: 'var(--color-surface)',
              border: 'var(--border-stark)',
              color: 'var(--color-text)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              padding: '4px 6px',
              boxSizing: 'border-box',
            }}
          />
        </div>
        <div style={{ flex: 1, overflow: 'auto' }}>
          {traceList.filter((id) => !traceSearch || id.toLowerCase().includes(traceSearch.toLowerCase())).map((id) => (
            <div
              key={id}
              onClick={() => navigate(`/trace/${id}`)}
              style={{
                padding: '6px 12px', fontSize: '11px', cursor: 'pointer',
                borderBottom: 'var(--border-stark)',
                background: id === traceId ? 'var(--color-surface)' : 'transparent',
                color: id === traceId ? 'var(--color-primary)' : 'var(--color-text)',
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}
            >
              {id}
            </div>
          ))}
          {traceList.length === 0 && (
            <div style={{ padding: '12px', fontSize: '11px', color: 'var(--color-muted)', textAlign: 'center' }}>
              No traces
            </div>
          )}
        </div>
      </div>

      {/* ── Main panel ── */}
      {!traceId ? (
        <div style={{
          flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '12px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em',
        }}>
          SELECT A TRACE
        </div>
      ) : loading ? (
        <div style={{
          flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '12px', color: 'var(--color-primary)', textTransform: 'uppercase', letterSpacing: '0.08em',
        }}>
          LOADING_
        </div>
      ) : error ? (
        <div style={{
          flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '12px', color: 'var(--color-alert)', textTransform: 'uppercase', letterSpacing: '0.08em',
        }}>
          {error}
        </div>
      ) : (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          {/* Top 50% — PayloadViewer */}
          <div style={{ flex: 1, borderBottom: 'var(--border-stark)', overflow: 'hidden' }}>
            <PayloadViewer span={selectedSpan} />
          </div>

          {/* Bottom 50% — React Flow graph */}
          <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
            {/* Panel header */}
            <div style={{
              padding: '6px 12px', borderBottom: 'var(--border-stark)',
              fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)',
              letterSpacing: '0.08em', display: 'flex', gap: '16px',
              flexShrink: 0, alignItems: 'center',
            }}>
              <span>MLOPS LIFECYCLE</span>
              {trace && (
                <span style={{ color: 'var(--color-text)' }}>
                  {trace.spans.length} SPANS · {trace.duration_ms}MS
                </span>
              )}
              {trace && (
                <span style={{ color: 'var(--color-muted)', fontSize: '10px' }}>
                  {trace.trace_id}
                </span>
              )}
            </div>

            {/* React Flow */}
            <div style={{ flex: 1, background: '#050506', position: 'relative' }}>
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                nodeTypes={nodeTypes}
                onNodeClick={handleNodeClick}
                onNodeMouseEnter={handleNodeMouseEnter}
                onNodeMouseLeave={handleNodeMouseLeave}
                onInit={(instance) => { flowInstanceRef.current = instance }}
                fitView
                fitViewOptions={{ padding: 0.15 }}
                attributionPosition="bottom-left"
                style={{ background: '#050506' }}
                minZoom={0.2}
                maxZoom={2}
              >
                <Controls
                  style={{
                    background: 'var(--color-surface)',
                    border: 'var(--border-stark)',
                    borderRadius: '0px',
                  }}
                />
              </ReactFlow>

              {/* Span HUD tooltip */}
              {hud && <SpanHud span={hud.span} visible x={hud.x} y={hud.y} />}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
