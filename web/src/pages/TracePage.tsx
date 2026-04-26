import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import type { TraceResponse, TraceSpan } from '../services/types'
import { getTrace } from '../services/api'
import { PayloadViewer } from '../components/trace/PayloadViewer'
import { SpanHud } from '../components/trace/SpanHud'

// Layer colors matching --color-layer-lN CSS tokens
const LAYER_COLORS: Record<number, string> = {
  1: '#EF4444',
  2: '#F97316',
  3: '#F59E0B',
  4: '#EAB308',
  5: '#84CC16',
  6: '#22C55E',
  7: '#14B8A6',
  8: '#06B6D4',
  9: '#3B82F6',
  10: '#F43F5E',
}

function GanttWaterfall({
  spans,
  selectedSpanId,
  onSelectSpan,
}: {
  spans: TraceSpan[]
  selectedSpanId: string | null
  onSelectSpan: (span: TraceSpan) => void
}) {
  const [hud, setHud] = useState<{ span: TraceSpan; x: number; y: number } | null>(null)
  const _containerRef = useRef<HTMLDivElement>(null)

  if (!spans.length) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          fontFamily: 'var(--font-mono)',
          fontSize: '12px',
          color: 'var(--color-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
        }}
      >
        NO SPANS
      </div>
    )
  }

  const minMs = Math.min(...spans.map((s) => s.start_ms))
  const maxMs = Math.max(...spans.map((s) => s.start_ms + s.duration_ms))
  const totalMs = maxMs - minMs || 1

  return (
    <div
      ref={_containerRef}
      style={{
        height: '100%',
        overflow: 'auto',
        fontFamily: 'var(--font-mono)',
        fontSize: '11px',
        color: 'var(--color-text)',
      }}
    >
      {spans.map((span) => {
        const left = ((span.start_ms - minMs) / totalMs) * 100
        const width = Math.max((span.duration_ms / totalMs) * 100, 0.5)
        const color = LAYER_COLORS[span.layer] ?? '#888'
        const isSelected = selectedSpanId === span.span_id

        return (
          <div
            key={span.span_id}
            style={{
              display: 'flex',
              alignItems: 'center',
              height: 'var(--row-height)',
              borderBottom: 'var(--border-stark)',
              cursor: 'pointer',
              background: isSelected ? 'var(--color-surface)' : 'transparent',
            }}
            onClick={() => onSelectSpan(span)}
            onMouseEnter={(e) => setHud({ span, x: e.clientX + 12, y: e.clientY + 4 })}
            onMouseMove={(e) => setHud({ span, x: e.clientX + 12, y: e.clientY + 4 })}
            onMouseLeave={() => setHud(null)}
          >
            {/* Layer label */}
            <div
              style={{
                width: '40px',
                flexShrink: 0,
                color,
                textAlign: 'right',
                paddingRight: '8px',
                fontWeight: 600,
                letterSpacing: '0.04em',
              }}
            >
              L{span.layer}
            </div>
            {/* Span name */}
            <div
              style={{
                width: '160px',
                flexShrink: 0,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                paddingRight: '8px',
                color: 'var(--color-muted)',
                fontSize: '10px',
              }}
            >
              {span.name}
            </div>
            {/* Bar track */}
            <div style={{ flex: 1, position: 'relative', height: '12px' }}>
              <div
                style={{
                  position: 'absolute',
                  left: `${left}%`,
                  width: `${width}%`,
                  height: '100%',
                  background: color,
                  opacity: isSelected ? 1 : 0.75,
                  outline: isSelected ? `1px solid ${color}` : 'none',
                }}
              />
            </div>
            {/* Duration */}
            <div
              style={{
                width: '60px',
                flexShrink: 0,
                textAlign: 'right',
                paddingRight: '12px',
                color: 'var(--color-muted)',
                fontSize: '10px',
              }}
            >
              {span.duration_ms}ms
            </div>
          </div>
        )
      })}

      {hud && <SpanHud span={hud.span} visible x={hud.x} y={hud.y} />}
    </div>
  )
}

export function TracePage() {
  const { traceId } = useParams<{ traceId?: string }>()
  const navigate = useNavigate()
  const [traceList, setTraceList] = useState<string[]>([])
  const [trace, setTrace] = useState<TraceResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<TraceSpan | null>(null)

  useEffect(() => {
    if (traceId) setTraceList([traceId])
  }, [traceId])

  useEffect(() => {
    if (!traceId) return
    setLoading(true)
    setError(null)
    setSelectedSpan(null)
    getTrace(traceId)
      .then((t) => {
        setTrace(t)
        if (t.spans.length > 0) setSelectedSpan(t.spans[0])
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load trace'))
      .finally(() => setLoading(false))
  }, [traceId])

  return (
    <div
      style={{
        display: 'flex',
        height: '100vh',
        background: 'var(--color-background)',
        color: 'var(--color-text)',
        fontFamily: 'var(--font-mono)',
        overflow: 'hidden',
      }}
    >
      {/* ── Trace list sidebar ── */}
      <div
        style={{
          width: '200px',
          flexShrink: 0,
          borderRight: 'var(--border-stark)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            padding: '8px 12px',
            borderBottom: 'var(--border-stark)',
            fontSize: '11px',
            textTransform: 'uppercase',
            color: 'var(--color-muted)',
            letterSpacing: '0.08em',
          }}
        >
          TRACES
        </div>
        <div style={{ flex: 1, overflow: 'auto' }}>
          {traceList.map((id) => (
            <div
              key={id}
              onClick={() => navigate(`/trace/${id}`)}
              style={{
                padding: '6px 12px',
                fontSize: '11px',
                cursor: 'pointer',
                borderBottom: 'var(--border-stark)',
                background: id === traceId ? 'var(--color-surface)' : 'transparent',
                color: id === traceId ? 'var(--color-primary)' : 'var(--color-text)',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {id}
            </div>
          ))}
          {!traceList.length && (
            <div
              style={{
                padding: '12px',
                fontSize: '11px',
                color: 'var(--color-muted)',
                textAlign: 'center',
              }}
            >
              No traces
            </div>
          )}
        </div>
      </div>

      {/* ── Main 50/50 vertical split ── */}
      {!traceId ? (
        <div
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            color: 'var(--color-muted)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
          }}
        >
          SELECT A TRACE
        </div>
      ) : loading ? (
        <div
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            color: 'var(--color-primary)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
          }}
        >
          LOADING_
        </div>
      ) : error ? (
        <div
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            color: 'var(--color-alert)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
          }}
        >
          {error}
        </div>
      ) : (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          {/* Top 50% — Payload viewer */}
          <div style={{ flex: 1, borderBottom: 'var(--border-stark)', overflow: 'hidden' }}>
            <PayloadViewer span={selectedSpan} />
          </div>

          {/* Bottom 50% — CSS Gantt waterfall */}
          <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
            <div
              style={{
                padding: '8px 12px',
                borderBottom: 'var(--border-stark)',
                fontSize: '11px',
                textTransform: 'uppercase',
                color: 'var(--color-muted)',
                letterSpacing: '0.08em',
                display: 'flex',
                gap: '16px',
                flexShrink: 0,
              }}
            >
              <span>TRACE {trace?.trace_id}</span>
              <span style={{ color: 'var(--color-text)' }}>
                {trace?.spans.length ?? 0} SPANS · {trace?.duration_ms ?? 0}MS
              </span>
            </div>
            <div style={{ flex: 1, overflow: 'hidden' }}>
              <GanttWaterfall
                spans={trace?.spans ?? []}
                selectedSpanId={selectedSpan?.span_id ?? null}
                onSelectSpan={setSelectedSpan}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
