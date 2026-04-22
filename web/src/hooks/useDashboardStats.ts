import { useEffect, useState, useCallback } from 'react'
import { useAuthStore } from '../stores/auth'

/**
 * useDashboardStats
 *
 * Fetches live aggregations from ClickHouse via POST /api/v1/query.
 * Runs 4 queries in parallel on mount and every 60s.
 * Falls back to empty state on error so the UI never hard-crashes.
 */

export interface HourlyBucket {
  hour: string   // "HH:00"
  count: number
}

export interface SeverityBucket {
  severity: number
  label: string
  count: number
  color: string
}

export interface LayerBucket {
  layer: number
  name: string
  count: number
  color: string
}

export interface LatencyBucket {
  hour: string
  p99_ms: number
}

export interface DashboardKPIs {
  signalsLastHour: number
  signalsTrend: number   // % change vs previous hour
  openIncidents: number
  detectionP99: number   // ms
  activeApps: number
}

export interface DashboardStats {
  hourly: HourlyBucket[]
  severity: SeverityBucket[]
  byLayer: LayerBucket[]
  latency: LatencyBucket[]
  kpis: DashboardKPIs
  loading: boolean
  error: string | null
  refetch: () => void
}

const SEVERITY_META: Record<number, { label: string; color: string }> = {
  1: { label: 'Info',     color: '#6B7280' },
  2: { label: 'Low',      color: '#3B82F6' },
  3: { label: 'Medium',   color: '#EAB308' },
  4: { label: 'High',     color: '#F97316' },
  5: { label: 'Critical', color: '#EF4444' },
}

const LAYER_META: Record<number, { name: string; color: string }> = {
  1:  { name: 'L1 Hardware',       color: '#EF4444' },
  2:  { name: 'L2 Weights',        color: '#F97316' },
  3:  { name: 'L3 Tokenizer',      color: '#F59E0B' },
  4:  { name: 'L4 Transformer',    color: '#22C55E' },
  5:  { name: 'L5 Output',         color: '#10B981' },
  6:  { name: 'L6 Safety',         color: '#14B8A6' },
  7:  { name: 'L7 RAG',            color: '#3B82F6' },
  8:  { name: 'L8 Agents',         color: '#8B5CF6' },
  9:  { name: 'L9 Gateway',        color: '#EC4899' },
  10: { name: 'L10 Application',   color: '#F43F5E' },
}

async function runQuery(sql: string, token: string): Promise<any[]> {
  const res = await fetch('/api/v1/query', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    credentials: 'include',
    body: JSON.stringify({ sql }),
  })
  if (!res.ok) throw new Error(`Query failed: ${res.status}`)
  const data = await res.json()
  return data.rows ?? []
}

export function useDashboardStats(): DashboardStats {
  const token = useAuthStore(s => s.token) ?? ''

  const [hourly, setHourly]     = useState<HourlyBucket[]>([])
  const [severity, setSeverity] = useState<SeverityBucket[]>([])
  const [byLayer, setByLayer]   = useState<LayerBucket[]>([])
  const [latency, setLatency]   = useState<LatencyBucket[]>([])
  const [kpis, setKpis]         = useState<DashboardKPIs>({
    signalsLastHour: 0, signalsTrend: 0,
    openIncidents: 0, detectionP99: 0, activeApps: 0,
  })
  const [loading, setLoading] = useState(true)
  const [error, setError]     = useState<string | null>(null)

  const fetch24hHourly = `
    SELECT toStartOfHour(timestamp) AS hr, count() AS cnt
    FROM signals
    WHERE timestamp >= now() - INTERVAL 25 HOUR
    GROUP BY hr ORDER BY hr ASC`

  const fetchSeverity = `
    SELECT severity, count() AS cnt
    FROM signals
    WHERE timestamp >= now() - INTERVAL 1 HOUR
    GROUP BY severity ORDER BY severity`

  const fetchByLayer = `
    SELECT layer, count() AS cnt
    FROM signals
    WHERE timestamp >= now() - INTERVAL 24 HOUR
    GROUP BY layer ORDER BY layer`

  const fetchLatency = `
    SELECT toStartOfHour(timestamp) AS hr,
           quantile(0.99)(duration_ms) AS p99
    FROM signals
    WHERE timestamp >= now() - INTERVAL 25 HOUR
      AND duration_ms IS NOT NULL
    GROUP BY hr ORDER BY hr ASC`

  const fetchKPIs = `
    SELECT
      countIf(timestamp >= now() - INTERVAL 1 HOUR)  AS last_hour,
      countIf(timestamp >= now() - INTERVAL 2 HOUR AND timestamp < now() - INTERVAL 1 HOUR) AS prev_hour,
      countDistinctIf(app_id, timestamp >= now() - INTERVAL 24 HOUR) AS apps
    FROM signals`

  const fetchP99 = `
    SELECT quantile(0.99)(duration_ms) AS p99
    FROM signals
    WHERE timestamp >= now() - INTERVAL 1 HOUR
      AND duration_ms IS NOT NULL`

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [hourlyRows, sevRows, layerRows, latencyRows, kpiRows, p99Rows] =
        await Promise.all([
          runQuery(fetch24hHourly, token),
          runQuery(fetchSeverity, token),
          runQuery(fetchByLayer, token),
          runQuery(fetchLatency, token),
          runQuery(fetchKPIs, token),
          runQuery(fetchP99, token),
        ])

      // Hourly signal rate — fill missing hours with 0
      const hourMap = new Map<string, number>()
      hourlyRows.forEach((r: any) => {
        const d = new Date(r.hr)
        const label = `${d.getUTCHours().toString().padStart(2, '0')}:00`
        hourMap.set(label, Number(r.cnt))
      })
      const hourlyFilled: HourlyBucket[] = Array.from({ length: 25 }, (_, i) => {
        const d = new Date(Date.now() - (24 - i) * 3_600_000)
        const label = `${d.getUTCHours().toString().padStart(2, '0')}:00`
        return { hour: label, count: hourMap.get(label) ?? 0 }
      })
      setHourly(hourlyFilled)

      // Severity
      const sevBuckets: SeverityBucket[] = sevRows.map((r: any) => ({
        severity: Number(r.severity),
        label: SEVERITY_META[Number(r.severity)]?.label ?? `Sev ${r.severity}`,
        count: Number(r.cnt),
        color: SEVERITY_META[Number(r.severity)]?.color ?? '#6B7280',
      }))
      setSeverity(sevBuckets)

      // By layer
      const layerBuckets: LayerBucket[] = layerRows.map((r: any) => ({
        layer: Number(r.layer),
        name: LAYER_META[Number(r.layer)]?.name ?? `L${r.layer}`,
        count: Number(r.cnt),
        color: LAYER_META[Number(r.layer)]?.color ?? '#6B7280',
      }))
      setByLayer(layerBuckets)

      // Latency
      const latencyFilled: LatencyBucket[] = Array.from({ length: 25 }, (_, i) => {
        const d = new Date(Date.now() - (24 - i) * 3_600_000)
        const label = `${d.getUTCHours().toString().padStart(2, '0')}:00`
        const match = latencyRows.find((r: any) => {
          const rd = new Date(r.hr)
          return rd.getUTCHours() === d.getUTCHours()
        })
        return { hour: label, p99_ms: match ? Math.round(Number(match.p99)) : 0 }
      })
      setLatency(latencyFilled)

      // KPIs
      const kr = kpiRows[0] ?? {}
      const lastHour = Number(kr.last_hour ?? 0)
      const prevHour = Number(kr.prev_hour ?? 0)
      const trend = prevHour > 0 ? Math.round(((lastHour - prevHour) / prevHour) * 100) : 0
      const p99 = p99Rows[0] ? Math.round(Number(p99Rows[0].p99)) : 0

      setKpis({
        signalsLastHour: lastHour,
        signalsTrend: trend,
        openIncidents: 0, // will come from /api/v1/incidents when wired
        detectionP99: p99,
        activeApps: Number(kr.apps ?? 0),
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard stats')
    } finally {
      setLoading(false)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  useEffect(() => {
    load()
    const interval = setInterval(load, 60_000)
    return () => clearInterval(interval)
  }, [load])

  return { hourly, severity, byLayer, latency, kpis, loading, error, refetch: load }
}
