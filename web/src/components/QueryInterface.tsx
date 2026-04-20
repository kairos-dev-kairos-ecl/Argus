import React from 'react'
import { QueryEditor } from './QueryEditor'
import { QueryResults } from './QueryResults'
import { QueryAnalytics } from './QueryAnalytics'
import { useQuery } from '../hooks/useQuery'

/**
 * Combined query interface component.
 * Left side: SQL editor with syntax highlighting and autocomplete
 * Center: Query results with pagination and export
 * Right side: Analytics panel with latency distribution and metrics
 */
export const QueryInterface: React.FC = () => {
  const { sql, setSql, result, loading, error, execute } = useQuery()

  return (
    <div className="flex flex-col h-full gap-4 p-4">
      {/* Error banner */}
      {error && (
        <div className="p-3 bg-status-error/10 border border-status-error/30 rounded" role="alert">
          <div className="text-status-error text-sm font-semibold">Error</div>
          <div className="text-status-error/80 text-xs mt-1">{error}</div>
        </div>
      )}

      {/* Editor, results, and analytics three-way layout */}
      <div className="flex gap-4 flex-1 min-h-0">
        {/* Left: Query editor */}
        <div className="flex-1 min-w-0 max-w-md">
          <h2 className="text-lg font-semibold text-foreground mb-3">Query</h2>
          <QueryEditor
            sql={sql}
            onChange={setSql}
            onExecute={execute}
          />
        </div>

        {/* Center: Results */}
        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-semibold text-foreground mb-3">Results</h2>
          <QueryResults result={result} loading={loading} />
        </div>

        {/* Right: Analytics */}
        <div className="w-80 flex-shrink-0">
          <QueryAnalytics result={result} loading={loading} />
        </div>
      </div>
    </div>
  )
}
