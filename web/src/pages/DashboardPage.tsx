import { useState, useEffect, useRef, useCallback } from 'react'
import type { ArgusSignal } from '../types'
import { TimeScrubber } from '../components/dashboard/TimeScrubber'
import type { TimeRange } from '../components/dashboard/TimeScrubber'
import { FilterSidebar } from '../components/dashboard/FilterSidebar'
import { createSignalSocket } from '../services/websocket'
import * as api from '../services/api'

// Layer colors matching CSS tokens
const LAYER_COLORS: Record<number, string> = {
  1: '#EF4444', 2: '#F97316', 3: '#F59E0B', 4: '#EAB308',
  5: '#84CC16', 6: '#22C55E', 7: '#14B8A6', 8: '#06B6D4',
  9: '#3B82F6', 10: '#F43F5E',
}

const SEVERITY_COLORS: Record<number, string> = {
  1: '#343A40', 2: '#EAB308', 3: '#F97316', 4: '#EF4444', 5: '#FF2A00',
}

function fmtTs(ts: ArgusSignal['timestamp']): string {
  if (typeof ts === 'string') return ts.slice(11, 23)
  if (ts && typeof ts === 'object' && 'seconds' in ts) {
    return new Date(ts.seconds * 1000).toISOString().slice(11, 23)
  }
  return '—'
}

// Anomaly horizon grid: 10 rows (L1-L10), each showing severity heat
function AnomalyHorizonGrid({ signals }: { signals: ArgusSignal[] }) {
  // Build per-layer severity counts
  const layerData: Record<number, { count: number; maxSev: number }> = {}
  for (let i = 1; i <= 10; i++) layerData[i] = { count: 0, maxSev: 0 }
  for (const s of signals) {
    const l = s.layer
    if (l >= 1 && l <= 10) {
      layerData[l].count++
      if (s.severity > layerData[l].maxSev) layerData[l].maxSev = s.severity
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        fontFamily: 'var(--font-mono)',
        overflow: 'auto',
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
        ANOMALY HORIZON
      </div>

      {/* Layer rows */}
      {[1,2,3,4,5,6,7,8,9,10].map((layer) => {
        const { count, maxSev } = layerData[layer]
        const color = LAYER_COLORS[layer]
        const sevColor = count > 0 ? SEVERITY_COLORS[maxSev] ?? color : 'var(--color-surface)'
        const isCrit = maxSev >= 4 && count > 0

        return (
          <div
            key={layer}
            style={{
              display: 'flex',
              alignItems: 'center',
              height: '36px',
              borderBottom: 'var(--border-stark)',
              padding: '0 12px',
              background: isCrit ? 'rgba(239,68,68,0.06)' : 'transparent',
              flexShrink: 0,
            }}
          >
            {/* Layer badge */}
            <div
              style={{
                width: '28px',
                fontSize: '11px',
                fontWeight: 700,
                color,
                letterSpacing: '0.04em',
                flexShrink: 0,
              }}
            >
              L{layer}
            </div>

            {/* Heat bar */}
            <div
              style={{
                flex: 1,
                height: '8px',
                background: 'var(--color-surface)',
                position: 'relative',
                margin: '0 8px',
              }}
            >
              {count > 0 && (
                <div
                  style={{
                    position: 'absolute',
                    left: 0,
                    top: 0,
                    height: '100%',
                    width: `${Math.min(100, (count / 20) * 100)}%`,
                    background: sevColor,
                    opacity: 0.85,
                  }}
                />
              )}
            </div>

            {/* Count */}
            <div
              style={{
                width: '40px',
                textAlign: 'right',
                fontSize: '11px',
                color: count > 0 ? 'var(--color-text)' : 'var(--color-muted)',
                flexShrink: 0,
              }}
            >
              {count > 0 ? count : '—'}
            </div>

            {/* CRIT badge */}
            {isCrit && (
              <div
                style={{
                  marginLeft: '8px',
                  padding: '1px 5px',
                  fontSize: '10px',
                  border: '1px solid var(--color-alert)',
                  color: 'var(--color-alert)',
                  letterSpacing: '0.05em',
                  flexShrink: 0,
                }}
              >
                CRIT
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

// Dense signal row (20px height)
function SignalRow({ signal }: { signal: ArgusSignal }) {
  const color = LAYER_COLORS[signal.layer] ?? '#888'
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '90px 28px 40px 60px 1fr',
        alignItems: 'center',
        height: 'var(--row-height)',
        borderBottom: '1px solid rgba(52,58,64,0.4)',
        fontSize: '11px',
        fontFamily: 'var(--font-mono)',
        padding: '0 8px',
        color: 'var(--color-text)',
        flexShrink: 0,
      }}
    >
      <span style={{ color: 'var(--color-muted)' }}>{fmtTs(signal.timestamp)}</span>
      <span style={{ color, fontWeight: 600 }}>L{signal.layer}</span>
      <span
        style={{
          color: SEVERITY_COLORS[signal.severity] ?? 'var(--color-muted)',
          fontWeight: signal.severity >= 4 ? 700 : 400,
        }}
      >
        S{signal.severity}
      </span>
      <span
        style={{
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          color: 'var(--color-muted)',
          fontSize: '10px',
        }}
      >
        {signal.category}
      </span>
      <span
        style={{
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          fontSize: '10px',
          color: 'var(--color-muted)',
        }}
      >
        {signal.signal_id.slice(0, 26)}
      </span>
    </div>
  )
}

interface FilterState {
  layer?: number
  severity?: number
  category?: string
}

export function DashboardPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>('15m')
  const [filters, setFilters] = useState<FilterState>({})
  const [signals, setSignals] = useState<ArgusSignal[]>([])
  const [wsStatus, setWsStatus] = useState<'connecting' | 'open' | 'closed' | 'error'>('connecting')
  const scrollRef = useRef<HTMLDivElement>(null)
  const cleanupRef = useRef<(() => void) | null>(null)

  // Fetch historical signals on mount / time range / filter change
  useEffect(() => {
    api.getSignals({
      limit: 100,
      layer: filters.layer,
      severity: filters.severity,
      category: filters.category,
    }).then((res) => {
      setSignals(res.signals ?? [])
    }).catch(() => {/* ignore */})
  }, [timeRange, filters])

  // WebSocket live feed
  const handleSignal = useCallback((signal: ArgusSignal) => {
    setSignals((prev) => {
      const next = [signal, ...prev]
      return next.slice(0, 500) // cap at 500
    })
  }, [])

  useEffect(() => {
    cleanupRef.current?.()
    cleanupRef.current = createSignalSocket({
      onSignal: handleSignal,
      onStatus: setWsStatus,
    })
    return () => {
      cleanupRef.current?.()
    }
  }, [handleSignal])

  // Filter signals for display
  const filtered = signals.filter((s) => {
    if (filters.layer && s.layer !== filters.layer) return false
    if (filters.severity && s.severity < filters.severity) return false
    if (filters.category && s.category !== filters.category) return false
    return true
  })

  const wsColor =
    wsStatus === 'open' ? 'var(--color-success)' :
    wsStatus === 'connecting' ? 'var(--color-warning)' : 'var(--color-alert)'

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        background: 'var(--color-background)',
        color: 'var(--color-text)',
        fontFamily: 'var(--font-mono)',
        overflow: 'hidden',
      }}
    >
      {/* ── Top bar: Time scrubber + WS status ── */}
      <div style={{ display: 'flex', alignItems: 'center', borderBottom: 'var(--border-stark)', flexShrink: 0 }}>
        <TimeScrubber value={timeRange} onChange={setTimeRange} />
        <div style={{ flex: 1 }} />
        <div
          style={{
            padding: '0 16px',
            fontSize: '11px',
            color: wsColor,
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
          }}
        >
          ● {wsStatus.toUpperCase()}
        </div>
      </div>

      {/* ── Body: Filter sidebar + 70/30 split ── */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* Filter sidebar */}
        <FilterSidebar filters={filters} onChange={setFilters} />

        {/* 70% signal log */}
        <div
          style={{
            flex: '0 0 70%',
            borderRight: 'var(--border-stark)',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
          }}
        >
          {/* Column header */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '90px 28px 40px 60px 1fr',
              height: '24px',
              alignItems: 'center',
              padding: '0 8px',
              borderBottom: 'var(--border-stark)',
              fontSize: '10px',
              color: 'var(--color-muted)',
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              flexShrink: 0,
            }}
          >
            <span>TIME</span>
            <span>LYR</span>
            <span>SEV</span>
            <span>CAT</span>
            <span>SIGNAL_ID</span>
          </div>

          {/* Signal rows */}
          <div
            ref={scrollRef}
            style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}
          >
            {filtered.map((s) => (
              <SignalRow key={s.signal_id} signal={s} />
            ))}
            {!filtered.length && (
              <div
                style={{
                  padding: '24px',
                  textAlign: 'center',
                  fontSize: '12px',
                  color: 'var(--color-muted)',
                  textTransform: 'uppercase',
                }}
              >
                NO SIGNALS
              </div>
            )}
          </div>
        </div>

        {/* 30% anomaly horizon */}
        <div style={{ flex: '0 0 30%', overflow: 'hidden' }}>
          <AnomalyHorizonGrid signals={signals} />
        </div>
      </div>
    </div>
  )
}
