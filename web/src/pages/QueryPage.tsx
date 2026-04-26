import { useState, useRef, useCallback } from 'react'
import { QueryEditor } from '../components/QueryEditor'
import * as api from '../services/api'
import type { QueryResponse } from '../services/types'

// Manual JSON coloring with <span> tags (no library per spec)
function colorJson(json: string): string {
  return json
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"([^"]+)":/g, '<span style="color:#00F0FF">"$1"</span>:')
    .replace(/: "([^"]*)"/g, ': <span style="color:#22C55E">"$1"</span>')
    .replace(/: (\d+(\.\d+)?)/g, ': <span style="color:#F97316">$1</span>')
    .replace(/: (true|false)/g, ': <span style="color:#EAB308">$1</span>')
    .replace(/: (null)/g, ': <span style="color:#343A40">$1</span>')
}

const DEFAULT_QUERY = `SELECT
  timestamp,
  layer,
  category,
  severity,
  signal_id
FROM signals
ORDER BY timestamp DESC
LIMIT 50`

export function QueryPage() {
  const [query, setQuery] = useState(DEFAULT_QUERY)
  const [result, setResult] = useState<QueryResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [retryAfter, setRetryAfter] = useState<number | null>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const startCountdown = useCallback((seconds: number) => {
    setRetryAfter(seconds)
    if (timerRef.current) clearInterval(timerRef.current)
    timerRef.current = setInterval(() => {
      setRetryAfter((prev) => {
        if (prev === null || prev <= 1) {
          clearInterval(timerRef.current!)
          return null
        }
        return prev - 1
      })
    }, 1000)
  }, [])

  const execute = useCallback(async () => {
    if (!query.trim() || loading || retryAfter !== null) return
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const res = await api.executeQuery({ query, sql: query })
      setResult(res)
    } catch (e: unknown) {
      const status = (e as { status?: number }).status
      if (status === 429) {
        const retryHeader = (e as { retryAfter?: number }).retryAfter ?? 30
        startCountdown(retryHeader)
        setError('RATE LIMITED')
      } else {
        setError(e instanceof Error ? e.message : 'Query failed')
      }
    } finally {
      setLoading(false)
    }
  }, [query, loading, retryAfter, startCountdown])

  const btnLabel = loading
    ? 'EXECUTING...'
    : retryAfter !== null
    ? `RETRY IN ${retryAfter}S`
    : 'EXECUTE'

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
      {/* ── Left 30% — SQL editor ── */}
      <div
        style={{
          flex: '0 0 30%',
          borderRight: 'var(--border-stark)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '8px 12px',
            borderBottom: 'var(--border-stark)',
            fontSize: '11px',
            textTransform: 'uppercase',
            color: 'var(--color-muted)',
            letterSpacing: '0.08em',
            flexShrink: 0,
          }}
        >
          SQL QUERY · CTRL+ENTER TO EXECUTE
        </div>

        {/* CodeMirror editor */}
        <div style={{ flex: 1, overflow: 'hidden', background: 'var(--color-background)' }}>
          <QueryEditor sql={query} onChange={setQuery} onExecute={execute} />
        </div>

        {/* Execute button */}
        <div
          style={{
            padding: '12px',
            borderTop: 'var(--border-stark)',
            flexShrink: 0,
          }}
        >
          <button
            onClick={execute}
            disabled={loading || retryAfter !== null}
            style={{
              width: '100%',
              padding: '10px',
              background: 'transparent',
              border: `1px solid ${retryAfter !== null ? 'var(--color-alert)' : 'var(--color-primary)'}`,
              color: retryAfter !== null ? 'var(--color-alert)' : 'var(--color-primary)',
              fontFamily: 'var(--font-mono)',
              fontSize: '12px',
              textTransform: 'uppercase',
              letterSpacing: '0.1em',
              cursor: loading || retryAfter !== null ? 'not-allowed' : 'pointer',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {btnLabel}
          </button>
        </div>
      </div>

      {/* ── Right 70% — Results ── */}
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '8px 12px',
            borderBottom: 'var(--border-stark)',
            fontSize: '11px',
            textTransform: 'uppercase',
            color: 'var(--color-muted)',
            letterSpacing: '0.08em',
            flexShrink: 0,
            display: 'flex',
            gap: '16px',
          }}
        >
          <span>RESULTS</span>
          {result && (
            <span style={{ color: 'var(--color-text)' }}>
              {result.total} ROWS · {result.execution_time_ms}MS
            </span>
          )}
          {error && (
            <span style={{ color: error === 'RATE LIMITED' ? 'var(--color-warning)' : 'var(--color-alert)' }}>
              {error}
            </span>
          )}
        </div>

        {/* Result body */}
        <div style={{ flex: 1, overflow: 'auto' }}>
          {loading && (
            <div
              style={{
                padding: '24px',
                textAlign: 'center',
                fontSize: '12px',
                color: 'var(--color-primary)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
            >
              EXECUTING...
            </div>
          )}

          {!loading && !result && !error && (
            <div
              style={{
                padding: '24px',
                textAlign: 'center',
                fontSize: '12px',
                color: 'var(--color-muted)',
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
              }}
            >
              RESULTS WILL APPEAR HERE
            </div>
          )}

          {!loading && result && (
            <pre
              style={{
                margin: 0,
                padding: '12px',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                color: 'var(--color-text)',
                lineHeight: 1.5,
              }}
              dangerouslySetInnerHTML={{
                __html: colorJson(JSON.stringify(result.rows, null, 2)),
              }}
            />
          )}
        </div>
      </div>
    </div>
  )
}
