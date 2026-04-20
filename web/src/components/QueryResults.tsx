import React, { useState, useMemo } from 'react'
import dayjs from 'dayjs'
import type { QueryResult } from '../types/index'
import Papa from 'papaparse'

interface QueryResultsProps {
  result: QueryResult | null
  loading: boolean
}

/**
 * Component displaying query results with expandable rows showing JSON payloads.
 * Features:
 * - Paginated table view (100 rows per page)
 * - Expandable rows to show full JSON payload
 * - CSV export
 * - Copy-to-clipboard for JSON
 */
export const QueryResults: React.FC<QueryResultsProps> = ({ result, loading }) => {
  const [page, setPage] = useState(1)
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set())
  const pageSize = 100

  const { paginatedRows, columns, totalPages } = useMemo(() => {
    if (!result || !result.rows || result.rows.length === 0) {
      return { paginatedRows: [], columns: [], totalPages: 0 }
    }

    const cols = Object.keys(result.rows[0] || {})
    const start = (page - 1) * pageSize
    const end = Math.min(start + pageSize, result.rows.length)
    const paginated = result.rows.slice(start, end)
    const total = Math.ceil(result.rows.length / pageSize)

    return { paginatedRows: paginated, columns: cols, totalPages: total }
  }, [result, page])

  const toggleRowExpanded = (idx: number) => {
    const newExpanded = new Set(expandedRows)
    if (newExpanded.has(idx)) {
      newExpanded.delete(idx)
    } else {
      newExpanded.add(idx)
    }
    setExpandedRows(newExpanded)
  }

  const handleExportCSV = () => {
    if (!result || !result.rows) return

    const csv = Papa.unparse(result.rows)
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    const url = URL.createObjectURL(blob)

    link.setAttribute('href', url)
    link.setAttribute('download', `query-results-${dayjs().format('YYYY-MM-DD-HHmmss')}.csv`)
    link.style.visibility = 'hidden'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const handleCopyJSON = (row: Record<string, any>) => {
    const json = JSON.stringify(row, null, 2)
    navigator.clipboard.writeText(json).catch((err) => {
      console.error('Failed to copy:', err)
    })
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full bg-muted-background rounded border border-border">
        <div className="text-muted-foreground">Executing query...</div>
      </div>
    )
  }

  if (!result) {
    return (
      <div className="flex items-center justify-center h-full bg-muted-background rounded border border-border">
        <div className="text-muted-foreground">No results yet</div>
      </div>
    )
  }

  if (result.rows.length === 0) {
    return (
      <div className="flex items-center justify-center h-full bg-muted-background rounded border border-border">
        <div className="text-muted-foreground">Query returned no rows</div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full bg-muted-background rounded border border-border overflow-hidden">
      {/* Header */}
      <div className="p-3 flex items-center justify-between bg-background border-b border-border">
        <span className="text-sm text-muted-foreground">
          {result.total} rows | {columns.length} columns | {result.execution_time_ms}ms
        </span>
        <button
          onClick={handleExportCSV}
          className="h-8 px-3 bg-primary/20 hover:bg-primary/30 text-foreground text-sm font-medium rounded transition-colors duration-200"
          aria-label="Export results as CSV"
        >
          Export CSV
        </button>
      </div>

      {/* Results as expandable cards */}
      <div className="flex-1 overflow-auto p-3 space-y-2">
        {paginatedRows.map((row, idx) => {
          const isExpanded = expandedRows.has(idx)
          const globalIdx = (page - 1) * pageSize + idx

          return (
            <div
              key={idx}
              className="border border-border rounded bg-background/50 hover:bg-background/80 transition-colors"
            >
              {/* Row header with expand button */}
              <button
                onClick={() => toggleRowExpanded(idx)}
                className="w-full text-left p-3 flex items-center gap-2 hover:bg-background/50 transition-colors"
              >
                <span className="text-foreground/60 text-sm font-mono">#{globalIdx + 1}</span>
                <span className="flex-1 text-foreground text-sm truncate">
                  {columns.slice(0, 3).map(col => {
                    const value = row[col]
                    let displayValue = ''

                    if (value === null || value === undefined) {
                      displayValue = 'NULL'
                    } else if (typeof value === 'object') {
                      displayValue = JSON.stringify(value).slice(0, 50)
                    } else {
                      displayValue = String(value).slice(0, 50)
                    }

                    return `${col}: ${displayValue}`
                  }).join(' | ')}
                </span>
                <span className={`text-foreground/60 text-xs transition-transform ${isExpanded ? 'rotate-180' : ''}`}>
                  ▼
                </span>
              </button>

              {/* Expanded details */}
              {isExpanded && (
                <div className="border-t border-border p-3 bg-background/30 space-y-2">
                  {/* Field list */}
                  <div className="space-y-1 max-h-48 overflow-auto">
                    {columns.map(col => {
                      const value = row[col]
                      let displayValue = ''

                      if (value === null || value === undefined) {
                        displayValue = 'NULL'
                      } else if (typeof value === 'object') {
                        displayValue = JSON.stringify(value)
                      } else {
                        displayValue = String(value)
                      }

                      return (
                        <div key={`${idx}-${col}`} className="text-xs">
                          <span className="text-foreground/70 font-mono">{col}:</span>
                          <code className="block bg-background/70 px-2 py-1 rounded mt-0.5 text-foreground/80 break-all">
                            {displayValue}
                          </code>
                        </div>
                      )
                    })}
                  </div>

                  {/* Copy JSON button */}
                  <button
                    onClick={() => handleCopyJSON(row)}
                    className="text-xs px-2 py-1 bg-primary/20 hover:bg-primary/30 text-foreground rounded transition-colors"
                  >
                    Copy JSON
                  </button>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Pagination */}
      <div className="p-3 flex items-center justify-center gap-3 bg-background border-t border-border">
        <button
          onClick={() => setPage(p => Math.max(1, p - 1))}
          disabled={page === 1}
          className="h-8 px-3 bg-primary/20 hover:bg-primary/30 disabled:bg-border disabled:text-muted-foreground text-foreground text-sm font-medium rounded transition-colors duration-200"
          aria-label="Previous page"
        >
          Previous
        </button>
        <span className="text-sm text-muted-foreground">
          Page {page} of {totalPages}
        </span>
        <button
          onClick={() => setPage(p => Math.min(totalPages, p + 1))}
          disabled={page >= totalPages}
          className="h-8 px-3 bg-primary/20 hover:bg-primary/30 disabled:bg-border disabled:text-muted-foreground text-foreground text-sm font-medium rounded transition-colors duration-200"
          aria-label="Next page"
        >
          Next
        </button>
      </div>
    </div>
  )
}
