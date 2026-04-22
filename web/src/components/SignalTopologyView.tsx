/**
 * SignalTopologyView — Onum/Falcon signal pipeline visualisation
 *
 * Design ref: Figma/stitch_argusxdr_tactical_hub_prd/DESIGN.md
 * Visual ref: Figma/image_for_ref/image (10).png  (Onum pipelines page)
 *
 * Sankey layout — strictly left → right, matching Onum's collector→pipeline→sink pattern:
 *
 *   [Apps]  ──▶  [L1 Hardware]  ──▶  [L2 Weights]  ──▶  …  ──▶  [L10 Application]  ──▶  [Observed]
 *
 * Node widths = signal volume at that stage.
 * Link thickness = volume flowing between consecutive stages.
 * Apps feed into whichever layers they instrument — multiple parallel coloured streams.
 *
 * No severity here — this is the observability pipeline view.
 * Severity lives on the Overview dashboard (XDR perspective).
 *
 * Data: POST /api/v1/query → ClickHouse, refreshes every 60 s.
 */

import React, { useEffect, useState, useCallback, useMemo } from 'react'
import ECharts from 'echarts-for-react'
import { useAuthStore } from '../stores/auth'

// ─── Design tokens (DESIGN.md) ───────────────────────────────────────────────
const C = {
  bg:      '#0e0e12',
  surface: '#131317',
  hi:      '#1f1f25',
  border:  'rgba(72,71,76,0.15)',
  text:    '#E9ECEF',
  sub:     '#868e96',
  dim:     '#495057',
  cyan:    '#8ff5ff',
  purple:  '#d575ff',
  pink:    '#ff51fa',
  amber:   '#FFB300',
  red:     '#FF2A00',
  green:   '#10B981',
}

// L1→L10 pipeline colours (cool→warm mirrors data-flow heat)
const LAYER_COLORS = [
  '#3B82F6', // L1  Hardware      — blue
  '#6366F1', // L2  Weights       — indigo
  '#8B5CF6', // L3  Tokenizer     — violet
  '#A855F7', // L4  Transformer   — purple
  '#D946EF', // L5  Output        — fuchsia
  '#EC4899', // L6  Safety        — pink
  '#F43F5E', // L7  RAG           — rose
  '#FF5A1F', // L8  Agents        — orange
  '#F59E0B', // L9  Gateway       — amber
  '#10B981', // L10 Application   — emerald
]

const LAYER_LABELS = [
  'L1 Hardware',
  'L2 Weights',
  'L3 Tokenizer',
  'L4 Transformer',
  'L5 Output',
  'L6 Safety',
  'L7 RAG',
  'L8 Agents',
  'L9 Gateway',
  'L10 App',
]

// Distinct colours for app source nodes
const APP_COLORS = ['#8ff5ff', '#d575ff', '#ff51fa', '#FFB300', '#10B981', '#F97316']

// ─── Types ────────────────────────────────────────────────────────────────────
interface TsPoint  { t: string; v: number }
interface KpiData  { signalsHr: number; apps: number; avgLatency: number; busyLayer: string; trend: number }

// ─── Query helper ─────────────────────────────────────────────────────────────
async function runQuery(sql: string, token: string): Promise<any[]> {
  const r = await fetch('/api/v1/query', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    credentials: 'include',
    body: JSON.stringify({ sql }),
  })
  if (!r.ok) throw new Error(`Query ${r.status}`)
  const d = await r.json()
  return d.rows ?? []
}

// ─── Hook ─────────────────────────────────────────────────────────────────────
interface TopologyData {
  sankeyOption: object
  eps:          TsPoint[]
  latency:      TsPoint[]
  throughput:   TsPoint[]
  kpis:         KpiData
  loading:      boolean
  error:        string | null
  refetch:      () => void
}

function useTopologyData(): TopologyData {
  const token = useAuthStore(s => s.token) ?? ''

  const [sankeyOption, setSankeyOption] = useState<object>({})
  const [eps,       setEps]      = useState<TsPoint[]>([])
  const [latency,   setLatency]  = useState<TsPoint[]>([])
  const [throughput, setThru]    = useState<TsPoint[]>([])
  const [kpis,      setKpis]     = useState<KpiData>({ signalsHr:0, apps:0, avgLatency:0, busyLayer:'—', trend:0 })
  const [loading,   setLoading]  = useState(true)
  const [error,     setError]    = useState<string|null>(null)

  const load = useCallback(async () => {
    setLoading(true); setError(null)
    try {
      const [appLayerRows, epsRows, latRows, thruRows, kpiRows, p99Row, busyRow] = await Promise.all([
        // Per-app, per-layer signal counts (feeds Sankey)
        runQuery(`
          SELECT app_id, layer, count() AS cnt
          FROM signals
          WHERE timestamp >= now() - INTERVAL 24 HOUR
          GROUP BY app_id, layer
          ORDER BY app_id, layer`, token),

        // 5-min EPS buckets last 2.5 h
        runQuery(`
          SELECT toStartOfInterval(timestamp, INTERVAL 5 MINUTE) AS t, count() AS cnt
          FROM signals
          WHERE timestamp >= now() - INTERVAL 150 MINUTE
          GROUP BY t ORDER BY t ASC`, token),

        // p99 latency per 5-min
        runQuery(`
          SELECT toStartOfInterval(timestamp, INTERVAL 5 MINUTE) AS t,
                 quantile(0.99)(duration_ms) AS p99
          FROM signals
          WHERE timestamp >= now() - INTERVAL 150 MINUTE AND duration_ms IS NOT NULL
          GROUP BY t ORDER BY t ASC`, token),

        // Throughput (signal count) per 5-min
        runQuery(`
          SELECT toStartOfInterval(timestamp, INTERVAL 5 MINUTE) AS t, count() AS cnt
          FROM signals
          WHERE timestamp >= now() - INTERVAL 150 MINUTE
          GROUP BY t ORDER BY t ASC`, token),

        // KPIs
        runQuery(`
          SELECT
            countIf(timestamp >= now() - INTERVAL 1 HOUR)  AS hr,
            countIf(timestamp >= now() - INTERVAL 2 HOUR
                    AND timestamp < now() - INTERVAL 1 HOUR) AS prev_hr,
            countDistinctIf(app_id, timestamp >= now() - INTERVAL 24 HOUR) AS apps
          FROM signals`, token),

        // Average p99
        runQuery(`
          SELECT quantile(0.99)(duration_ms) AS p99
          FROM signals
          WHERE timestamp >= now() - INTERVAL 1 HOUR AND duration_ms IS NOT NULL`, token),

        // Busiest layer in last 1 h
        runQuery(`
          SELECT layer, count() AS cnt
          FROM signals
          WHERE timestamp >= now() - INTERVAL 1 HOUR
          GROUP BY layer ORDER BY cnt DESC LIMIT 1`, token),
      ])

      // ── Build Sankey ────────────────────────────────────────────────────────
      //
      // Layout (left → right, matching Onum pipeline):
      //   depth 0  : App source nodes
      //   depth 1-10: L1-L10 pipeline stages
      //   depth 11 : "Observed" terminal sink
      //
      // Links:
      //   1. App → first layer the app has signals in  (entry point into pipeline)
      //   2. L(n) → L(n+1) aggregated across all apps (shows pipeline flow)
      //   3. L10  → "Observed"

      // Build layer totals and app→layer map
      const layerTotal = new Map<number, number>()   // layer → total signals
      const appLayer   = new Map<string, Map<number, number>>()  // app → (layer → count)
      const allApps    = new Set<string>()

      for (const r of appLayerRows) {
        const app   = String(r.app_id)
        const layer = Number(r.layer)
        const cnt   = Number(r.cnt)
        allApps.add(app)
        layerTotal.set(layer, (layerTotal.get(layer) ?? 0) + cnt)
        if (!appLayer.has(app)) appLayer.set(app, new Map())
        appLayer.get(app)!.set(layer, cnt)
      }

      if (layerTotal.size === 0) {
        setSankeyOption({})
        return
      }

      // Collect layers present in data (sorted)
      const presentLayers = Array.from(layerTotal.keys()).sort((a, b) => a - b)
      const minLayer = presentLayers[0]

      // Assign app colours
      const appList   = Array.from(allApps)
      const appColorMap = new Map(appList.map((a, i) => [a, APP_COLORS[i % APP_COLORS.length]]))

      // Nodes
      const nodes: any[] = []

      // App nodes (depth 0)
      appList.forEach(app => {
        nodes.push({
          name:      app,
          depth:     0,
          itemStyle: { color: appColorMap.get(app) },
          label:     { color: C.text, fontSize: 10 },
        })
      })

      // Layer nodes (depth = layer number)
      presentLayers.forEach(l => {
        const idx   = l - 1
        nodes.push({
          name:      LAYER_LABELS[idx] ?? `L${l}`,
          depth:     l,
          itemStyle: { color: LAYER_COLORS[idx] ?? '#6B7280' },
          label:     { color: C.text, fontSize: 10 },
        })
      })

      // Terminal "Observed" node (depth = max_layer + 1)
      const maxLayer = presentLayers[presentLayers.length - 1]
      nodes.push({
        name:      'Observed',
        depth:     maxLayer + 1,
        itemStyle: { color: C.green },
        label:     { color: C.green, fontSize: 11, fontWeight: 'bold' },
      })

      // Links
      const links: any[] = []

      // App → first layer in pipeline (entry point)
      appList.forEach(app => {
        const layerMap = appLayer.get(app)!
        // Connect app to the first layer it has signals in
        const firstLayer = Math.min(...Array.from(layerMap.keys()))
        const firstLabel = LAYER_LABELS[firstLayer - 1] ?? `L${firstLayer}`
        const val = layerMap.get(firstLayer) ?? 1
        links.push({ source: app, target: firstLabel, value: val })
      })

      // L(n) → L(n+1) — pipeline chain across all apps
      for (let i = 0; i < presentLayers.length - 1; i++) {
        const src    = presentLayers[i]
        const tgt    = presentLayers[i + 1]
        const srcLbl = LAYER_LABELS[src - 1] ?? `L${src}`
        const tgtLbl = LAYER_LABELS[tgt - 1] ?? `L${tgt}`
        // Use the source layer's total as flow volume
        // (signals that were observed at this stage continue downstream)
        const val = layerTotal.get(src) ?? 1
        links.push({ source: srcLbl, target: tgtLbl, value: val })
      }

      // Last layer → Observed
      const lastLabel = LAYER_LABELS[maxLayer - 1] ?? `L${maxLayer}`
      links.push({ source: lastLabel, target: 'Observed', value: layerTotal.get(maxLayer) ?? 1 })

      setSankeyOption({
        backgroundColor: 'transparent',
        tooltip: {
          trigger: 'item',
          backgroundColor: '#111216',
          borderColor: C.cyan,
          borderWidth: 1,
          textStyle: { color: C.text, fontSize: 11, fontFamily: 'JetBrains Mono, monospace' },
          formatter: (p: any) => {
            if (p.data?.source) {
              return `<b>${p.data.source}</b> → <b>${p.data.target}</b><br/>`
                + `<span style="color:${C.cyan}">${Number(p.data.value).toLocaleString()}</span> signals`
            }
            return `<b>${p.name}</b><br/>`
              + `<span style="color:${C.cyan}">${Number(p.value ?? 0).toLocaleString()}</span> signals`
          },
        },
        series: [{
          type:     'sankey',
          layout:   'none',          // required when using depth
          orient:   'horizontal',
          nodeAlign: 'left',
          top:      '10%',
          bottom:   '10%',
          left:     '5%',
          right:    '8%',
          emphasis: { focus: 'adjacency' },
          nodeGap:  16,
          nodeWidth: 16,
          draggable: false,
          lineStyle: {
            color:     'gradient',
            curveness: 0.5,
            opacity:   0.45,
          },
          data:  nodes,
          links,
          label: {
            position: 'right',
            color:    C.text,
            fontSize: 10,
            fontFamily: 'Inter, sans-serif',
          },
          levels: presentLayers.map((l) => ({
            depth: l,
            lineStyle: { color: LAYER_COLORS[l - 1] ?? '#6B7280', opacity: 0.4 },
          })),
        }],
      })

      // ── Time series ─────────────────────────────────────────────────────────
      const fmt = (row: any) => {
        const d = new Date(row.t)
        return `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}`
      }
      setEps(epsRows.map((r: any) => ({ t: fmt(r), v: Number(r.cnt) })))
      setLatency(latRows.map((r: any) => ({ t: fmt(r), v: Math.round(Number(r.p99)) })))
      setThru(thruRows.map((r: any) => ({ t: fmt(r), v: Number(r.cnt) })))

      // ── KPIs ────────────────────────────────────────────────────────────────
      const kr  = kpiRows[0] ?? {}
      const hr  = Number(kr.hr ?? 0)
      const prev = Number(kr.prev_hr ?? 0)
      const bl  = busyRow[0]
      const busyLayerName = bl
        ? (LAYER_LABELS[Number(bl.layer) - 1] ?? `L${bl.layer}`)
        : '—'

      setKpis({
        signalsHr:  hr,
        apps:       Number(kr.apps ?? 0),
        avgLatency: p99Row[0] ? Math.round(Number(p99Row[0].p99)) : 0,
        busyLayer:  busyLayerName,
        trend:      prev > 0 ? Math.round(((hr - prev) / prev) * 100) : 0,
      })

    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load topology')
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    load()
    const id = setInterval(load, 60_000)
    return () => clearInterval(id)
  }, [load])

  return { sankeyOption, eps, latency, throughput, kpis, loading, error, refetch: load }
}

// ─── Hex → rgba ───────────────────────────────────────────────────────────────
function hexToRgba(hex: string, a: number) {
  const h = hex.replace('#', '')
  return `rgba(${parseInt(h.slice(0,2),16)},${parseInt(h.slice(2,4),16)},${parseInt(h.slice(4,6),16)},${a})`
}

// ─── Area chart option ────────────────────────────────────────────────────────
function areaOption(data: TsPoint[], line: string, unit = '') {
  return {
    backgroundColor: 'transparent',
    grid: { left: 0, right: 0, top: 8, bottom: 24, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#111216',
      borderColor: line,
      borderWidth: 1,
      textStyle: { color: C.text, fontSize: 11, fontFamily: 'JetBrains Mono, monospace' },
      formatter: (p: any[]) => `<b>${p[0].name}</b><br/><span style="color:${line}">${p[0].value}${unit}</span>`,
    },
    xAxis: {
      type: 'category',
      data: data.map(d => d.t),
      axisLine:  { lineStyle: { color: C.dim } },
      axisLabel: { color: C.dim, fontSize: 9, interval: 4 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false }, axisTick: { show: false },
      axisLabel: { color: C.dim, fontSize: 9 },
      splitLine: { lineStyle: { color: '#1A1D24' } },
    },
    series: [{
      type: 'line',
      data: data.map(d => d.v),
      smooth: true, symbol: 'none',
      lineStyle: { color: line, width: 1.5 },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: hexToRgba(line, 0.4) },
            { offset: 1, color: hexToRgba(line, 0.02) },
          ],
        },
      },
    }],
  }
}

// ─── Shared sub-components ────────────────────────────────────────────────────
const Shimmer: React.FC<{ h: number }> = ({ h }) => (
  <div className="animate-pulse rounded" style={{ height: h, background: C.hi }} />
)

const Chip: React.FC<{ label: string; value: string; accent: string; sub?: string }> = ({ label, value, accent, sub }) => (
  <div style={{ background: C.surface, border: `1px solid ${C.border}`, padding: '12px 16px', boxShadow: 'inset 0 1px 0 rgba(252,248,254,0.06)' }}>
    <div style={{ fontSize: 10, color: C.sub, letterSpacing: '1.2px', marginBottom: 6 }}>{label}</div>
    <div style={{ fontSize: 22, fontWeight: 700, color: accent, fontFamily: 'Space Grotesk, sans-serif' }}>{value}</div>
    {sub && <div style={{ fontSize: 10, color: C.dim, marginTop: 4 }}>{sub}</div>}
  </div>
)

const SparkPanel: React.FC<{ title: string; avg: string; avgColor: string; badge?: string; badgeColor?: string; option: object; loading: boolean }> =
  ({ title, avg, avgColor, badge, badgeColor, option, loading }) => (
  <div>
    <div className="flex items-center justify-between mb-1.5">
      <span style={{ fontSize: 11, fontWeight: 600, color: C.text, letterSpacing: '1px', fontFamily: 'Space Grotesk, sans-serif' }}>{title}</span>
      <div style={{ fontSize: 10, color: C.sub, fontFamily: 'JetBrains Mono, monospace' }}>
        AVG <span style={{ color: avgColor, fontWeight: 600 }}>{avg}</span>
        {badge && <span style={{ color: badgeColor, marginLeft: 8 }}>{badge}</span>}
      </div>
    </div>
    <div style={{ height: 175, background: C.surface, border: `1px solid ${C.border}`, padding: 10, boxShadow: 'inset 0 1px 0 rgba(252,248,254,0.06)' }}>
      {loading ? <Shimmer h={155} /> : <ECharts option={option} style={{ height: '100%', width: '100%' }} notMerge lazyUpdate />}
    </div>
  </div>
)

// ─── Main export ──────────────────────────────────────────────────────────────
export const SignalTopologyView: React.FC = () => {
  const { sankeyOption, eps, latency, throughput, kpis, loading, error, refetch } = useTopologyData()

  const epsOpt   = useMemo(() => areaOption(eps,        C.cyan,   ''), [eps])
  const latOpt   = useMemo(() => areaOption(latency,    C.amber,  'ms'), [latency])
  const thruOpt  = useMemo(() => areaOption(throughput, C.purple, ''), [throughput])

  const avgEps  = eps.length     ? Math.round(eps.reduce((s,d) => s+d.v,0)     / eps.length)     : 0
  const avgLat  = latency.length ? Math.round(latency.reduce((s,d) => s+d.v,0) / latency.length) : 0
  const avgThru = throughput.length ? Math.round(throughput.reduce((s,d) => s+d.v,0) / throughput.length) : 0
  const trendTxt  = kpis.trend >= 0 ? `+${kpis.trend}%` : `${kpis.trend}%`
  const trendColor = kpis.trend >= 0 ? C.cyan : C.red

  const hasSankey = !loading && (sankeyOption as any)?.series?.length > 0

  return (
    <div style={{ background: C.bg, fontFamily: 'Inter, sans-serif', minHeight: '100%' }} className="p-6 flex flex-col gap-6">

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 style={{ fontFamily: 'Space Grotesk, sans-serif', fontSize: 18, fontWeight: 700, color: C.text, letterSpacing: '1px' }}>
            SIGNAL PIPELINE
          </h1>
          <p style={{ fontSize: 10, color: C.sub, marginTop: 3 }}>
            LLM system layers L1 → L10 · signal volume flow · 24 h window
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div style={{ fontSize: 10, color: C.dim, padding: '4px 10px', border: `1px solid ${C.border}` }}>📅 LAST 24 H</div>
          <button
            onClick={refetch}
            style={{ fontSize: 10, color: C.cyan, padding: '4px 12px', border: `1px solid ${C.cyan}`, background: 'transparent', cursor: 'pointer' }}
          >
            ↺ REFRESH
          </button>
        </div>
      </div>

      {error && (
        <div style={{ background: 'rgba(255,42,0,0.08)', border: `1px solid ${C.red}`, padding: '10px 14px', fontSize: 11, color: C.red }}>
          {error}
        </div>
      )}

      {/* KPI strip */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <Chip label="SIGNALS / HR"   value={loading ? '—' : kpis.signalsHr.toLocaleString()} accent={C.cyan}   sub={loading ? undefined : trendTxt} />
        <Chip label="ACTIVE APPS"    value={loading ? '—' : String(kpis.apps)}                accent={C.purple} sub="distinct app_id 24 h" />
        <Chip label="LATENCY P99"    value={loading ? '—' : `${kpis.avgLatency} ms`}          accent={C.amber}  sub={kpis.avgLatency <= 100 ? '✓ within SLA' : '✗ SLA breach'} />
        <Chip label="BUSIEST LAYER"  value={loading ? '—' : kpis.busyLayer}                   accent={C.green}  sub="highest volume 1 h" />
      </div>

      {/* ── Sankey — Left to right: Apps → L1 → L2 → … → L10 → Observed ────── */}
      <div>
        <div className="flex items-center gap-3 mb-2">
          <h2 style={{ fontFamily: 'Space Grotesk, sans-serif', fontSize: 12, fontWeight: 600, color: C.text, letterSpacing: '1px' }}>
            PIPELINE SIGNAL FLOW
          </h2>
          <span style={{ fontSize: 10, color: C.sub }}>
            Ingestion sources → LLM layers → Observed output
          </span>
        </div>

        {/* Layer legend */}
        <div className="flex flex-wrap gap-3 mb-3">
          {LAYER_LABELS.map((lbl, i) => (
            <div key={lbl} className="flex items-center gap-1">
              <div style={{ width: 8, height: 8, borderRadius: 2, background: LAYER_COLORS[i] }} />
              <span style={{ fontSize: 9, color: C.sub, fontFamily: 'JetBrains Mono, monospace' }}>{lbl}</span>
            </div>
          ))}
        </div>

        <div style={{ height: 420, background: C.surface, border: `1px solid ${C.border}`, padding: '20px 16px', boxShadow: 'inset 0 1px 0 rgba(252,248,254,0.06)' }}>
          {loading && <Shimmer h={380} />}
          {!loading && !hasSankey && (
            <div className="flex items-center justify-center h-full" style={{ color: C.sub, fontSize: 12 }}>
              No signal data in the last 24 h
            </div>
          )}
          {!loading && hasSankey && (
            <ECharts option={sankeyOption} style={{ height: '100%', width: '100%' }} notMerge lazyUpdate />
          )}
        </div>
      </div>

      {/* ── 3-column sparklines ─────────────────────────────────────────────── */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <SparkPanel
          title="EVENTS / 5 MIN"
          avg={avgEps.toLocaleString()}
          avgColor={C.cyan}
          badge={trendTxt}
          badgeColor={trendColor}
          option={epsOpt}
          loading={loading}
        />
        <SparkPanel
          title="LATENCY P99"
          avg={`${avgLat} ms`}
          avgColor={C.amber}
          badge={avgLat <= 100 ? '✓ SLA' : '✗ SLA'}
          badgeColor={avgLat <= 100 ? C.green : C.red}
          option={latOpt}
          loading={loading}
        />
        <SparkPanel
          title="THROUGHPUT"
          avg={avgThru.toLocaleString()}
          avgColor={C.purple}
          option={thruOpt}
          loading={loading}
        />
      </div>

    </div>
  )
}
