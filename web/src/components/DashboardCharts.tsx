import React from 'react'
import ECharts from 'echarts-for-react'
import { useDashboardStats } from '../hooks/useDashboardStats'
import type { HourlyBucket, SeverityBucket, LayerBucket, LatencyBucket } from '../hooks/useDashboardStats'

/**
 * DashboardCharts — Onum/Falcon-style live analytics panels.
 *
 * All data sourced from POST /api/v1/query → ClickHouse.
 * Refreshes every 60s via useDashboardStats.
 *
 * Visual language: Onum dark — near-black bg, coloured accent icons,
 * gradient area charts, dense but airy layout.
 */

// ─── Shared ECharts theme ────────────────────────────────────────────────────
const T = {
  text:   '#A0A0A0',
  dim:    '#606068',
  line:   '#1E1E26',
  bg:     'transparent',
  white:  '#F0F0F5',
}

// ─── Skeleton shimmer ────────────────────────────────────────────────────────
const Shimmer: React.FC<{ h?: number | string; w?: string }> = ({ h = 20, w = '100%' }) => (
  <div
    className="animate-pulse rounded bg-border"
    style={{ height: h, width: w }}
  />
)

// ─── KPI stat card (Onum style) ──────────────────────────────────────────────
interface KpiCardProps {
  label: string
  value: string | number
  sub?: string
  subColor?: string
  iconBg: string
  icon: React.ReactNode
  loading?: boolean
}

const KpiCard: React.FC<KpiCardProps> = ({
  label, value, sub, subColor = T.text, iconBg, icon, loading
}) => (
  <div className="flex items-center gap-4 bg-muted-background rounded-xl border border-border px-5 py-4">
    <div
      className="w-10 h-10 rounded-lg flex items-center justify-center text-white text-lg flex-shrink-0"
      style={{ background: iconBg }}
    >
      {icon}
    </div>
    <div className="min-w-0">
      <div className="text-xs text-muted-foreground uppercase tracking-widest mb-0.5">{label}</div>
      {loading
        ? <Shimmer h={28} w="80px" />
        : <div className="text-2xl font-bold text-foreground leading-tight">{value}</div>
      }
      {sub && !loading && (
        <div className="text-xs mt-0.5" style={{ color: subColor }}>{sub}</div>
      )}
    </div>
  </div>
)

// ─── Signal rate (24 h area) ─────────────────────────────────────────────────
const SignalRateChart: React.FC<{ data: HourlyBucket[]; loading: boolean }> = ({ data, loading }) => {
  const option = {
    backgroundColor: T.bg,
    grid: { left: 0, right: 0, top: 12, bottom: 28, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#13131A',
      borderColor: '#2A2A35',
      textStyle: { color: T.white, fontSize: 12 },
      formatter: (p: any[]) => `<b>${p[0].name}</b><br/>${p[0].value.toLocaleString()} signals`,
    },
    xAxis: {
      type: 'category',
      data: data.map(d => d.hour),
      axisLine:  { lineStyle: { color: T.line } },
      axisLabel: { color: T.dim, fontSize: 10, interval: 4 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLine:  { show: false },
      axisTick:  { show: false },
      axisLabel: { color: T.dim, fontSize: 10 },
      splitLine: { lineStyle: { color: T.line, type: 'dashed' } },
    },
    series: [{
      type: 'line',
      data: data.map(d => d.count),
      smooth: 0.4,
      symbol: 'none',
      lineStyle: { color: '#6366F1', width: 2 },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(99,102,241,0.45)' },
            { offset: 1, color: 'rgba(99,102,241,0.03)' },
          ],
        },
      },
    }],
  }

  return (
    <div className="bg-muted-background rounded-xl border border-border p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground">Signal Throughput</span>
        <span className="text-xs text-muted-foreground px-2 py-0.5 rounded-full border border-border">Last 24 h</span>
      </div>
      {loading
        ? <div className="flex flex-col gap-2"><Shimmer h={8} /><Shimmer h={140} /></div>
        : <ECharts option={option} style={{ height: 150 }} notMerge lazyUpdate />
      }
    </div>
  )
}

// ─── Severity donut ──────────────────────────────────────────────────────────
const SeverityDonut: React.FC<{ data: SeverityBucket[]; loading: boolean }> = ({ data, loading }) => {
  const total = data.reduce((s, d) => s + d.count, 0)

  const option = {
    backgroundColor: T.bg,
    tooltip: {
      backgroundColor: '#13131A',
      borderColor: '#2A2A35',
      textStyle: { color: T.white, fontSize: 12 },
      formatter: (p: any) => `${p.name}: <b>${p.value}</b> (${p.percent}%)`,
    },
    legend: {
      orient: 'vertical',
      right: 4,
      top: 'middle',
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 8,
      textStyle: { color: T.text, fontSize: 11 },
    },
    series: [{
      type: 'pie',
      radius: ['54%', '80%'],
      center: ['36%', '50%'],
      data: data.length > 0
        ? data.map(d => ({
            name: d.label,
            value: d.count,
            itemStyle: { color: d.color },
          }))
        : [{ name: 'No data', value: 1, itemStyle: { color: T.line } }],
      label: {
        show: true,
        position: 'center',
        formatter: () => total > 0 ? `${total.toLocaleString()}\nsignals` : '',
        color: T.white,
        fontSize: 12,
        lineHeight: 18,
        fontWeight: 'bold',
      },
      labelLine: { show: false },
      emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.5)' } },
    }],
  }

  return (
    <div className="bg-muted-background rounded-xl border border-border p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground">Severity Mix</span>
        <span className="text-xs text-muted-foreground px-2 py-0.5 rounded-full border border-border">Last 1 h</span>
      </div>
      {loading
        ? <div className="flex items-center gap-4"><Shimmer h={120} w="120px" /><div className="flex flex-col gap-2 flex-1">{[1,2,3,4,5].map(i=><Shimmer key={i} h={14} />)}</div></div>
        : <ECharts option={option} style={{ height: 150 }} notMerge lazyUpdate />
      }
    </div>
  )
}

// ─── Layer bar chart ─────────────────────────────────────────────────────────
const LayerBarChart: React.FC<{ data: LayerBucket[]; loading: boolean }> = ({ data, loading }) => {
  const sorted = [...data].sort((a, b) => a.count - b.count)

  const option = {
    backgroundColor: T.bg,
    grid: { left: 0, right: 36, top: 4, bottom: 4, containLabel: true },
    tooltip: {
      backgroundColor: '#13131A',
      borderColor: '#2A2A35',
      textStyle: { color: T.white, fontSize: 12 },
      formatter: (p: any) => `${p.name}: <b>${p.value.toLocaleString()}</b>`,
    },
    xAxis: {
      type: 'value',
      axisLine:  { show: false },
      axisTick:  { show: false },
      axisLabel: { color: T.dim, fontSize: 10 },
      splitLine: { lineStyle: { color: T.line, type: 'dashed' } },
    },
    yAxis: {
      type: 'category',
      data: sorted.map(d => d.name),
      axisLine:  { lineStyle: { color: T.line } },
      axisLabel: { color: T.text, fontSize: 10 },
      splitLine: { show: false },
    },
    series: [{
      type: 'bar',
      data: sorted.map(d => ({
        value: d.count,
        itemStyle: { color: d.color, borderRadius: [0, 4, 4, 0], opacity: 0.9 },
      })),
      barMaxWidth: 12,
      label: {
        show: true, position: 'right',
        color: T.dim, fontSize: 10,
        formatter: (p: any) => p.value.toLocaleString(),
      },
      emphasis: { itemStyle: { opacity: 1 } },
    }],
  }

  return (
    <div className="bg-muted-background rounded-xl border border-border p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground">Signals by Layer</span>
        <span className="text-xs text-muted-foreground px-2 py-0.5 rounded-full border border-border">Last 24 h</span>
      </div>
      {loading
        ? <div className="flex flex-col gap-2">{[...Array(8)].map((_,i)=><Shimmer key={i} h={14} />)}</div>
        : <ECharts option={option} style={{ height: 240 }} notMerge lazyUpdate />
      }
    </div>
  )
}

// ─── Latency sparkline ────────────────────────────────────────────────────────
const LatencyChart: React.FC<{ data: LatencyBucket[]; loading: boolean }> = ({ data, loading }) => {
  const maxVal = Math.max(...data.map(d => d.p99_ms), 120)
  const slaMs  = 100

  const option = {
    backgroundColor: T.bg,
    grid: { left: 0, right: 0, top: 12, bottom: 28, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#13131A',
      borderColor: '#2A2A35',
      textStyle: { color: T.white, fontSize: 12 },
      formatter: (p: any[]) => `<b>${p[1]?.name}</b><br/>p99: <b>${p[1]?.value ?? 0} ms</b>`,
    },
    xAxis: {
      type: 'category',
      data: data.map(d => d.hour),
      axisLine:  { lineStyle: { color: T.line } },
      axisLabel: { color: T.dim, fontSize: 10, interval: 4 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: Math.ceil(maxVal * 1.2),
      axisLine:  { show: false },
      axisTick:  { show: false },
      axisLabel: { color: T.dim, fontSize: 10, formatter: '{value}ms' },
      splitLine: { lineStyle: { color: T.line, type: 'dashed' } },
    },
    series: [
      // SLA reference line
      {
        type: 'line',
        data: Array(data.length).fill(slaMs),
        symbol: 'none',
        silent: true,
        lineStyle: { color: '#EF4444', type: 'dashed', width: 1, opacity: 0.5 },
        tooltip: { show: false },
      },
      // Actual p99
      {
        type: 'line',
        name: 'p99',
        data: data.map(d => d.p99_ms),
        smooth: 0.4,
        symbol: 'none',
        lineStyle: { color: '#22C55E', width: 2 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(34,197,94,0.35)' },
              { offset: 1, color: 'rgba(34,197,94,0.02)' },
            ],
          },
        },
      },
    ],
  }

  return (
    <div className="bg-muted-background rounded-xl border border-border p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground">Detection Latency p99</span>
        <span className="text-xs text-muted-foreground">
          <span className="inline-block w-3 border-t border-dashed border-red-500 mr-1 align-middle" />
          100 ms SLA
        </span>
      </div>
      {loading
        ? <div className="flex flex-col gap-2"><Shimmer h={8} /><Shimmer h={140} /></div>
        : <ECharts option={option} style={{ height: 150 }} notMerge lazyUpdate />
      }
    </div>
  )
}

// ─── Main export ─────────────────────────────────────────────────────────────

export const DashboardCharts: React.FC = () => {
  const { hourly, severity, byLayer, latency, kpis, loading } = useDashboardStats()

  const trendSign  = kpis.signalsTrend >= 0 ? '+' : ''
  const trendColor = kpis.signalsTrend >= 0 ? '#22C55E' : '#EF4444'
  const p99Color   = kpis.detectionP99 <= 100 ? '#22C55E' : '#EF4444'
  const p99Sub     = kpis.detectionP99 <= 100
    ? `${kpis.detectionP99} ms — within SLA ✓`
    : `${kpis.detectionP99} ms — SLA breach ✗`

  return (
    <div className="flex flex-col gap-4">

      {/* KPI row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard
          label="Signals / hr"
          value={loading ? '—' : kpis.signalsLastHour.toLocaleString()}
          sub={loading ? undefined : `${trendSign}${kpis.signalsTrend}% vs prev hour`}
          subColor={trendColor}
          iconBg="linear-gradient(135deg,#6366F1,#8B5CF6)"
          icon="⚡"
          loading={loading}
        />
        <KpiCard
          label="Open Incidents"
          value={kpis.openIncidents}
          sub="via /api/v1/incidents"
          iconBg="linear-gradient(135deg,#EF4444,#F97316)"
          icon="🚨"
          loading={loading}
        />
        <KpiCard
          label="Detection p99"
          value={loading ? '—' : `${kpis.detectionP99} ms`}
          sub={loading ? undefined : p99Sub}
          subColor={p99Color}
          iconBg="linear-gradient(135deg,#10B981,#14B8A6)"
          icon="⏱"
          loading={loading}
        />
        <KpiCard
          label="Active Apps"
          value={loading ? '—' : kpis.activeApps}
          sub="distinct app_id last 24 h"
          iconBg="linear-gradient(135deg,#EC4899,#F43F5E)"
          icon="🗂"
          loading={loading}
        />
      </div>

      {/* Row 1: throughput (wide) + severity donut */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="md:col-span-2">
          <SignalRateChart data={hourly} loading={loading} />
        </div>
        <SeverityDonut data={severity} loading={loading} />
      </div>

      {/* Row 2: latency (wide) + layer bar */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="md:col-span-2">
          <LatencyChart data={latency} loading={loading} />
        </div>
        <LayerBarChart data={byLayer} loading={loading} />
      </div>

    </div>
  )
}
