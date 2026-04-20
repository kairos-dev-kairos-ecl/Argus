import React, { useMemo } from 'react'
import ECharts from 'echarts-for-react'
import type { QueryResult } from '../types/index'

interface QueryAnalyticsProps {
  result: QueryResult | null
  loading: boolean
}

/**
 * Analytics panel for query results.
 * Displays:
 * - Latency bar chart
 * - Row count and execution metrics
 * - Status indicator
 */
export const QueryAnalytics: React.FC<QueryAnalyticsProps> = ({ result, loading }) => {
  // Mock latency distribution data (in real implementation, collect from multiple queries)
  const latencyData = useMemo(() => {
    if (!result) {
      return {
        series: [0, 0, 0, 0, 0],
        executionTime: 0,
        avgLatency: 0,
      }
    }

    const execTime = result.execution_time_ms || 0

    // Simplified: show current execution time in a histogram bucket
    const buckets = [0, 0, 0, 0, 0]
    if (execTime < 50) {
      buckets[0]++
    } else if (execTime < 100) {
      buckets[1]++
    } else if (execTime < 200) {
      buckets[2]++
    } else if (execTime < 500) {
      buckets[3]++
    } else {
      buckets[4]++
    }

    return {
      series: buckets,
      executionTime: execTime,
      avgLatency: execTime,
    }
  }, [result])

  const chartOption = {
    responsive: true,
    color: ['#3B82F6'],
    grid: {
      left: 30,
      right: 10,
      top: 20,
      bottom: 30,
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: ['<50ms', '50-100ms', '100-200ms', '200-500ms', '>500ms'],
      axisLabel: {
        fontSize: 11,
        color: '#A0A0A0',
      },
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        fontSize: 11,
        color: '#A0A0A0',
      },
    },
    series: [
      {
        data: latencyData.series,
        type: 'bar',
        itemStyle: {
          color: '#3B82F6',
        },
        emphasis: {
          itemStyle: {
            color: '#60A5FA',
          },
        },
      },
    ],
  }

  if (loading) {
    return (
      <div className="flex flex-col h-full bg-muted-background rounded border border-border p-4 space-y-4">
        <h2 className="text-lg font-semibold text-foreground">Analytics</h2>
        <div className="flex items-center justify-center flex-1 text-muted-foreground">
          Loading...
        </div>
      </div>
    )
  }

  if (!result) {
    return (
      <div className="flex flex-col h-full bg-muted-background rounded border border-border p-4 space-y-4">
        <h2 className="text-lg font-semibold text-foreground">Analytics</h2>
        <div className="flex items-center justify-center flex-1 text-muted-foreground">
          Run a query to see analytics
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full bg-muted-background rounded border border-border p-4 space-y-4 overflow-hidden">
      <h2 className="text-lg font-semibold text-foreground">Analytics</h2>

      {/* Execution metrics */}
      <div className="space-y-2">
        <div className="flex justify-between items-center">
          <span className="text-sm text-muted-foreground">Execution Time</span>
          <span className="text-sm font-mono text-foreground">
            {latencyData.executionTime}ms
          </span>
        </div>
        <div className="flex justify-between items-center">
          <span className="text-sm text-muted-foreground">Row Count</span>
          <span className="text-sm font-mono text-foreground">
            {result.total?.toLocaleString() ?? 0}
          </span>
        </div>
        {result.cursor && (
          <div className="flex justify-between items-center">
            <span className="text-sm text-muted-foreground">Has Next Page</span>
            <span className="text-sm font-mono text-foreground">Yes</span>
          </div>
        )}
      </div>

      {/* Latency distribution chart */}
      <div className="border-t border-border pt-4">
        <h3 className="text-sm font-semibold text-foreground mb-3">Latency Distribution</h3>
        <div className="bg-background/50 rounded h-32">
          <ECharts
            option={chartOption}
            style={{ height: '100%', width: '100%' }}
            notMerge={true}
            lazyUpdate={true}
          />
        </div>
      </div>

      {/* Status indicator */}
      <div className="border-t border-border pt-4">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-status-success"></div>
          <span className="text-sm text-foreground">Query succeeded</span>
        </div>
      </div>
    </div>
  )
}
