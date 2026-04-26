import type { TraceSpan } from '../../services/types'

interface PayloadViewerProps {
  span: TraceSpan | null
}

export function PayloadViewer({ span }: PayloadViewerProps) {
  if (!span) {
    return (
      <div
        style={{
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily: 'var(--font-mono)',
          background: 'var(--color-background)',
          color: 'var(--color-muted)',
          fontSize: '12px',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
        }}
      >
        SELECT A SPAN TO VIEW PAYLOAD
      </div>
    )
  }

  return (
    <div
      style={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        fontFamily: 'var(--font-mono)',
        background: 'var(--color-background)',
        color: 'var(--color-text)',
        borderBottom: 'var(--border-stark)',
        overflow: 'hidden',
      }}
    >
      {/* Header strip */}
      <div
        style={{
          padding: '8px 12px',
          fontSize: '11px',
          textTransform: 'uppercase',
          color: 'var(--color-muted)',
          borderBottom: 'var(--border-stark)',
          flexShrink: 0,
          letterSpacing: '0.08em',
        }}
      >
        SPAN {span.span_id} · L{span.layer} · {span.duration_ms}MS
      </div>

      {/* PROMPT section */}
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'auto',
          borderBottom: 'var(--border-stark)',
        }}
      >
        <div
          style={{
            padding: '6px 12px',
            fontSize: '11px',
            textTransform: 'uppercase',
            color: 'var(--color-muted)',
            borderBottom: 'var(--border-stark)',
            flexShrink: 0,
            letterSpacing: '0.08em',
          }}
        >
          PROMPT
        </div>
        <pre
          style={{
            margin: 0,
            padding: '12px',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            color: 'var(--color-text)',
            flex: 1,
            overflow: 'auto',
          }}
        >
          {span.prompt ?? '—'}
        </pre>
      </div>

      {/* RESPONSE section */}
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'auto',
        }}
      >
        <div
          style={{
            padding: '6px 12px',
            fontSize: '11px',
            textTransform: 'uppercase',
            color: 'var(--color-muted)',
            borderBottom: 'var(--border-stark)',
            flexShrink: 0,
            letterSpacing: '0.08em',
          }}
        >
          RESPONSE
        </div>
        <pre
          style={{
            margin: 0,
            padding: '12px',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            color: 'var(--color-text)',
            flex: 1,
            overflow: 'auto',
          }}
        >
          {span.response ?? '—'}
        </pre>
      </div>
    </div>
  )
}
