import { useState, useEffect, useCallback, useRef } from 'react'
import ECharts from 'echarts-for-react'
import type { ArgusSignal } from '../types'
import { TimeScrubber } from '../components/dashboard/TimeScrubber'
import type { TimeRange } from '../components/dashboard/TimeScrubber'
import { FilterSidebar } from '../components/dashboard/FilterSidebar'
import { createSignalSocket } from '../services/websocket'
import * as api from '../services/api'

// ── Layer config ─────────────────────────────────────────────────────────────

const LAYER_COLORS: Record<number, string> = {
  1: '#EF4444', 2: '#F97316', 3: '#F59E0B', 4: '#EAB308',
  5: '#84CC16', 6: '#22C55E', 7: '#14B8A6', 8: '#06B6D4',
  9: '#3B82F6', 10: '#F43F5E',
}

const LAYER_LABELS: Record<number, string> = {
  1: 'L1·HW',     2: 'L2·Wgts',   3: 'L3·Tok',  4: 'L4·Infer',
  5: 'L5·Decode', 6: 'L6·Safety', 7: 'L7·RAG',  8: 'L8·Agent',
  9: 'L9·GW',     10: 'L10·App',
}

const SEVERITY_COLORS: Record<number, string> = {
  1: '#343A40', 2: '#EAB308', 3: '#F97316', 4: '#EF4444', 5: '#FF2A00',
}

// ── Sankey option builder ─────────────────────────────────────────────────────

function buildSankeyOption(signals: ArgusSignal[], selectedLayer: number | null) {
  // Count signals per layer
  const counts: Record<number, number> = {}
  for (let i = 1; i <= 10; i++) counts[i] = 0
  for (const s of signals) {
    if (s.layer >= 1 && s.layer <= 10) counts[s.layer]++
  }

  const total = Object.values(counts).reduce((a, b) => a + b, 0)
  // When there are no signals, show skeleton at equal weight 5
  const skeleton = total === 0

  const nodes = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((l) => ({
    name: LAYER_LABELS[l],
    itemStyle: {
      color: LAYER_COLORS[l],
      opacity: selectedLayer === null || selectedLayer === l ? 1 : 0.25,
    },
    label: {
      color: '#E9ECEF',
      fontFamily: "'JetBrains Mono', monospace",
      fontSize: 11,
      opacity: selectedLayer === null || selectedLayer === l ? 1 : 0.4,
    },
  }))

  const links = []
  for (let l = 1; l <= 9; l++) {
    links.push({
      source: LAYER_LABELS[l],
      target: LAYER_LABELS[l + 1],
      value: skeleton ? 5 : Math.max(counts[l], 1),
      lineStyle: {
        color: 'gradient',
        opacity: selectedLayer === null || selectedLayer === l || selectedLayer === l + 1
          ? 0.45 : 0.1,
      },
    })
  }

  return {
    backgroundColor: '#050506',
    series: [
      {
        type: 'sankey',
        layout: 'none',
        left: '4%',
        right: '4%',
        top: '8%',
        bottom: '8%',
        nodeAlign: 'left',
        orient: 'horizontal',
        nodeGap: 16,
        nodeWidth: 14,
        emphasis: { focus: 'adjacency' },
        data: nodes,
        links,
        label: {
          color: '#E9ECEF',
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: 11,
        },
        lineStyle: {
          color: 'gradient',
          curveness: 0.5,
          opacity: 0.45,
        },
        itemStyle: { borderWidth: 0 },
      },
    ],
  }
}

// ── Anomaly horizon grid ──────────────────────────────────────────────────────

function AnomalyHorizonGrid({
  signals,
  selectedLayer,
  onSelectLayer,
}: {
  signals: ArgusSignal[]
  selectedLayer: number | null
  onSelectLayer: (l: number | null) => void
}) {
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
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', fontFamily: 'var(--font-mono)', overflow: 'auto' }}>
      <div style={{
        padding: '8px 12px', borderBottom: 'var(--border-stark)',
        fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)',
        letterSpacing: '0.08em', flexShrink: 0,
      }}>
        ANOMALY HORIZON
      </div>

      {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((layer) => {
        const { count, maxSev } = layerData[layer]
        const color = LAYER_COLORS[layer]
        const sevColor = count > 0 ? (SEVERITY_COLORS[maxSev] ?? color) : 'var(--color-surface)'
        const isCrit = maxSev >= 4 && count > 0
        const isSelected = selectedLayer === layer

        return (
          <div
            key={layer}
            onClick={() => onSelectLayer(isSelected ? null : layer)}
            style={{
              display: 'flex', alignItems: 'center', height: '36px',
              borderBottom: 'var(--border-stark)', padding: '0 12px', cursor: 'pointer',
              background: isSelected
                ? `rgba(${hexToRgb(color)},0.12)`
                : isCrit
                ? 'rgba(239,68,68,0.06)'
                : 'transparent',
              outline: isSelected ? `1px solid ${color}` : 'none',
              flexShrink: 0,
            }}
          >
            <div style={{ width: '28px', fontSize: '11px', fontWeight: 700, color, letterSpacing: '0.04em', flexShrink: 0 }}>
              L{layer}
            </div>
            <div style={{ flex: 1, height: '8px', background: 'var(--color-surface)', position: 'relative', margin: '0 8px' }}>
              {count > 0 && (
                <div style={{
                  position: 'absolute', left: 0, top: 0, height: '100%',
                  width: `${Math.min(100, (count / 20) * 100)}%`,
                  background: sevColor, opacity: 0.85,
                }} />
              )}
            </div>
            <div style={{ width: '36px', textAlign: 'right', fontSize: '11px', color: count > 0 ? 'var(--color-text)' : 'var(--color-muted)', flexShrink: 0 }}>
              {count > 0 ? count : '—'}
            </div>
            {isCrit && (
              <div style={{
                marginLeft: '8px', padding: '1px 5px', fontSize: '10px',
                border: '1px solid var(--color-alert)', color: 'var(--color-alert)',
                letterSpacing: '0.05em', flexShrink: 0,
              }}>
                CRIT
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

/** Convert hex color to "r,g,b" string for rgba() */
function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `${r},${g},${b}`
}

// ── Filter state ──────────────────────────────────────────────────────────────

interface FilterState {
  layer?: number
  severity?: number
  category?: string
}

// ── DashboardPage ─────────────────────────────────────────────────────────────

/** Convert preset TimeRange to ISO start timestamp (now - N). */
function presetToStart(range: TimeRange): string | undefined {
  const now = Date.now()
  const offsets: Record<string, number> = {
    '5m':  5  * 60 * 1000,
    '15m': 15 * 60 * 1000,
    '1h':  60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
  }
  if (range in offsets) return new Date(now - offsets[range]).toISOString()
  return undefined
}

export function DashboardPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>('15m')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo] = useState('')
  const [filters, setFilters] = useState<FilterState>({})
  const [signals, setSignals] = useState<ArgusSignal[]>([])
  const [wsStatus, setWsStatus] = useState<'connecting' | 'open' | 'closed' | 'error'>('connecting')
  const [selectedLayer, setSelectedLayer] = useState<number | null>(null)
  const cleanupRef = useRef<(() => void) | null>(null)

  // Compute start/end for the current time range
  const timeStart = timeRange === 'custom'
    ? (customFrom ? new Date(customFrom).toISOString() : undefined)
    : presetToStart(timeRange)
  const timeEnd = timeRange === 'custom'
    ? (customTo ? new Date(customTo).toISOString() : undefined)
    : new Date().toISOString()

  // Fetch historical signals on mount / time range / filter change
  useEffect(() => {
    // Skip custom fetch if from/to aren't both set
    if (timeRange === 'custom' && (!customFrom || !customTo)) return
    api.getSignals({
      limit: 200,
      layer: filters.layer,
      severity: filters.severity,
      category: filters.category,
      start: timeStart,
      end: timeEnd,
    }).then((res) => {
      setSignals(res.signals ?? [])
    }).catch(() => { /* ignore */ })
  }, [timeRange, customFrom, customTo, filters, timeStart, timeEnd])

  // WebSocket live feed
  const handleSignal = useCallback((signal: ArgusSignal) => {
    setSignals((prev) => [signal, ...prev].slice(0, 1000))
  }, [])

  useEffect(() => {
    cleanupRef.current?.()
    cleanupRef.current = createSignalSocket({ onSignal: handleSignal, onStatus: setWsStatus })
    return () => { cleanupRef.current?.() }
  }, [handleSignal])

  // Apply client-side filters for Sankey + anomaly panel
  const filtered = signals.filter((s) => {
    if (filters.layer && s.layer !== filters.layer) return false
    if (filters.severity && s.severity < filters.severity) return false
    if (filters.category && s.category !== filters.category) return false
    return true
  })

  // Sync filter sidebar layer selection with Sankey highlight
  const handleFilterChange = useCallback((f: FilterState) => {
    setFilters(f)
    if (f.layer !== undefined) setSelectedLayer(f.layer ?? null)
  }, [])

  const handleLayerSelect = useCallback((l: number | null) => {
    setSelectedLayer(l)
    setFilters((prev) => ({ ...prev, layer: l ?? undefined }))
  }, [])

  // ECharts node click handler
  const sankeyEvents = {
    click: (params: { dataType?: string; name?: string }) => {
      if (params.dataType === 'node' && params.name) {
        const match = params.name.match(/^L(\d+)/)
        if (match) {
          const l = parseInt(match[1])
          handleLayerSelect(selectedLayer === l ? null : l)
        }
      }
    },
  }

  const wsColor =
    wsStatus === 'open' ? '#22C55E' :
    wsStatus === 'connecting' ? '#EAB308' : '#EF4444'

  const sankeyOption = buildSankeyOption(filtered, selectedLayer)

  return (
    <div style={{
      display: 'flex', flexDirection: 'column', height: '100vh',
      background: 'var(--color-background)', color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)', overflow: 'hidden',
    }}>
      {/* ── Top bar ── */}
      <div style={{ display: 'flex', alignItems: 'center', borderBottom: 'var(--border-stark)', flexShrink: 0 }}>
        <TimeScrubber
          value={timeRange}
          onChange={setTimeRange}
          customFrom={customFrom}
          customTo={customTo}
          onCustomChange={(from, to) => { setCustomFrom(from); setCustomTo(to) }}
        />
        <div style={{ flex: 1 }} />
        <div style={{ padding: '0 16px', fontSize: '11px', color: wsColor, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
          ● {wsStatus.toUpperCase()}
        </div>
      </div>

      {/* ── Body ── */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* Filter sidebar */}
        <FilterSidebar filters={filters} onChange={handleFilterChange} />

        {/* 70% — Sankey diagram */}
        <div style={{
          flex: '0 0 70%', borderRight: 'var(--border-stark)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
          background: '#050506',
        }}>
          {/* Sankey header */}
          <div style={{
            padding: '6px 12px', borderBottom: 'var(--border-stark)',
            fontSize: '10px', color: 'var(--color-muted)',
            textTransform: 'uppercase', letterSpacing: '0.08em',
            display: 'flex', alignItems: 'center', gap: '16px', flexShrink: 0,
          }}>
            <span>SIGNAL FLOW  L1 → L10</span>
            <span style={{ color: 'var(--color-text)' }}>{filtered.length} SIGNALS</span>
            {selectedLayer !== null && (
              <span style={{ color: LAYER_COLORS[selectedLayer] }}>
                LAYER {selectedLayer} SELECTED
              </span>
            )}
            {selectedLayer !== null && (
              <span
                onClick={() => handleLayerSelect(null)}
                style={{ cursor: 'pointer', color: 'var(--color-muted)', textDecoration: 'underline' }}
              >
                CLEAR
              </span>
            )}
          </div>

          {/* ECharts Sankey */}
          <div style={{ flex: 1, overflow: 'hidden' }}>
            <ECharts
              option={sankeyOption}
              style={{ height: '100%', width: '100%' }}
              onEvents={sankeyEvents}
              opts={{ renderer: 'canvas' }}
            />
          </div>
        </div>

        {/* 30% — Anomaly horizon */}
        <div style={{ flex: '0 0 30%', overflow: 'hidden' }}>
          <AnomalyHorizonGrid
            signals={signals}
            selectedLayer={selectedLayer}
            onSelectLayer={handleLayerSelect}
          />
        </div>
      </div>
    </div>
  )
}
