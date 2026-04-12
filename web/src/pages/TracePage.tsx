import React, { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useTrace } from '../hooks/useTrace'
import { useRealtimeTrace } from '../hooks/useRealtimeTrace'
import { useTraceViewStore } from '../stores/traceViewStore'
import { TraceTimeline } from '../components/TraceTimeline'
import { ReasoningGraph } from '../components/ReasoningGraph'
import { SpanDetail } from '../components/SpanDetail'

type ViewMode = 'timeline' | 'graph' | 'split'

/**
 * Page component for viewing detailed trace information by trace_id.
 * Routes: /trace/:traceId
 * Displays timeline of spans across layers and detail panel for selected spans.
 * Also includes reasoning graph view and grounding information.
 */
export const TracePage: React.FC = () => {
  const { traceId } = useParams<{ traceId: string }>()
  const { trace, loading, error, selectedSpanID, setSelectedSpanID } = useTrace(traceId || '')
  const [viewMode, setViewMode] = useState<ViewMode>('timeline')
  const { selectedSpanId } = useTraceViewStore()

  // Real-time trace updates
  useRealtimeTrace(
    traceId || '',
    (newSpan) => {
      // In a real app, we'd append the new span to the trace
      // For now, this is a hook for future streaming implementation
      console.log('New span received:', newSpan)
    },
    (error) => {
      console.error('WebSocket error:', error)
    }
  )

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-muted-foreground">Loading trace...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-status-error text-center">
          <div className="font-semibold mb-2">Error loading trace</div>
          <div className="text-sm">{error}</div>
        </div>
      </div>
    )
  }

  if (!trace) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-muted-foreground">Trace not found</div>
      </div>
    )
  }

  // Determine selected span (from props or store)
  const currentSelectedSpanID = selectedSpanId || selectedSpanID
  const selectedSpan = trace.spans.find((s) => s.signal_id === currentSelectedSpanID) || null

  const tabs: Array<{ id: ViewMode; label: string }> = [
    { id: 'timeline', label: 'Timeline' },
    { id: 'graph', label: 'Reasoning Graph' },
    { id: 'split', label: 'Split View' },
  ]

  return (
    <div className="flex flex-col h-full gap-4 p-4">
      {/* Tab navigation */}
      <div className="flex gap-2 border-b border-border">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setViewMode(tab.id)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              viewMode === tab.id
                ? 'text-foreground border-b-2 border-primary -mb-[2px]'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Timeline view */}
      {viewMode === 'timeline' && (
        <div className="flex h-full gap-4">
          <div className="flex-1 flex flex-col">
            <h2 className="text-lg font-semibold text-foreground mb-3">Trace Timeline</h2>
            <TraceTimeline
              trace={trace}
              selectedSpanID={currentSelectedSpanID}
              onSelectSpan={setSelectedSpanID}
            />
          </div>

          <div className="w-80">
            <h2 className="text-lg font-semibold text-foreground mb-3">Span Details</h2>
            <SpanDetail span={selectedSpan} detections={trace.detections} />
          </div>
        </div>
      )}

      {/* Graph view */}
      {viewMode === 'graph' && (
        <div className="flex h-full gap-4">
          <div className="flex-1">
            <h2 className="text-lg font-semibold text-foreground mb-3">Reasoning Graph</h2>
            <div className="h-full border border-border rounded-md overflow-hidden">
              <ReasoningGraph
                trace={trace}
                selectedNodeId={currentSelectedSpanID || undefined}
                onSelectNode={setSelectedSpanID}
              />
            </div>
          </div>

          <div className="w-80">
            <h2 className="text-lg font-semibold text-foreground mb-3">Node Details</h2>
            <SpanDetail span={selectedSpan} detections={trace.detections} />
          </div>
        </div>
      )}

      {/* Split view: timeline top, graph bottom */}
      {viewMode === 'split' && (
        <div className="flex flex-col h-full gap-4">
          <div className="flex-1 flex flex-col">
            <h2 className="text-lg font-semibold text-foreground mb-3">Timeline</h2>
            <div className="flex-1 flex gap-4">
              <div className="flex-1">
                <TraceTimeline
                  trace={trace}
                  selectedSpanID={currentSelectedSpanID}
                  onSelectSpan={setSelectedSpanID}
                />
              </div>
              <div className="w-72">
                <SpanDetail span={selectedSpan} detections={trace.detections} />
              </div>
            </div>
          </div>

          <div className="flex-1 flex flex-col border-t border-border pt-4">
            <h2 className="text-lg font-semibold text-foreground mb-3">Reasoning Graph</h2>
            <div className="flex-1 border border-border rounded-md overflow-hidden">
              <ReasoningGraph
                trace={trace}
                selectedNodeId={currentSelectedSpanID || undefined}
                onSelectNode={setSelectedSpanID}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
