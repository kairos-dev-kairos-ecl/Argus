import type { TraceSpan } from '../../services/types'

interface SpanHudProps {
  span: TraceSpan
  visible: boolean
  x: number
  y: number
}

export function SpanHud({ span, visible, x, y }: SpanHudProps) {
  if (!visible) return null

  const rows: Array<{ label: string; value: string }> = [
    { label: 'LAYER', value: `L${span.layer}` },
    { label: 'DURATION', value: `${span.duration_ms}MS` },
    { label: 'TTFT', value: span.ttft_ms != null ? `${span.ttft_ms}MS` : '—' },
    { label: 'TOKENS', value: span.tokens != null ? String(span.tokens) : '—' },
    { label: 'GPU', value: span.gpu_util != null ? `${(span.gpu_util * 100).toFixed(0)}%` : '—' },
  ]

  return (
    <div
      style={{
        position: 'fixed',
        top: y,
        left: x,
        pointerEvents: 'none',
        zIndex: 50,
        background: 'var(--color-surface)',
        border: 'var(--border-stark)',
        padding: '8px 12px',
        fontFamily: 'var(--font-mono)',
        fontSize: '11px',
        color: 'var(--color-text)',
      }}
    >
      {rows.map(({ label, value }) => (
        <div
          key={label}
          style={{
            display: 'flex',
            gap: '8px',
            lineHeight: '1.6',
          }}
        >
          <span
            style={{
              width: '80px',
              textTransform: 'uppercase',
              color: 'var(--color-muted)',
              flexShrink: 0,
              letterSpacing: '0.06em',
            }}
          >
            {label}
          </span>
          <span style={{ color: 'var(--color-text)' }}>{value}</span>
        </div>
      ))}
    </div>
  )
}
