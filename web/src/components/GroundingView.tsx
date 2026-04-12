import React, { useState, useMemo } from 'react'
import { colors } from '../lib/design-tokens'

/**
 * Retrieved document chunk for grounding
 */
export interface GroundingChunk {
  id: string
  text: string
  source: string
  similarity_score: number // 0-1
  matched_segments?: string[] // Segments in output that cite this chunk
}

interface GroundingViewProps {
  chunks: GroundingChunk[]
  output: string
}

/**
 * Finds overlap between output and chunk text (lexical matching)
 */
function findGroundedSegments(output: string, chunk: string): string[] {
  const segments: string[] = []
  const words = chunk.split(/\s+/)

  // Find contiguous sequences of 3+ words from chunk in output
  for (let i = 0; i < words.length - 2; i++) {
    const phrase = words.slice(i, i + 3).join(' ')
    if (output.toLowerCase().includes(phrase.toLowerCase())) {
      segments.push(phrase)
    }
  }

  return [...new Set(segments)] // Deduplicate
}

/**
 * Highlights text in output, coloring grounded vs ungrounded segments
 */
function highlightGroundedText(output: string, chunks: GroundingChunk[]): React.ReactNode[] {
  const elements: React.ReactNode[] = []
  let lastIndex = 0
  const groundedRanges: Array<{ start: number; end: number; chunkId: string }> = []

  // Build list of grounded ranges
  chunks.forEach((chunk) => {
    const segments = findGroundedSegments(output, chunk.text)
    segments.forEach((segment) => {
      const index = output.toLowerCase().indexOf(segment.toLowerCase(), lastIndex)
      if (index !== -1) {
        groundedRanges.push({
          start: index,
          end: index + segment.length,
          chunkId: chunk.id,
        })
      }
    })
  })

  // Sort ranges by start position
  groundedRanges.sort((a, b) => a.start - b.start)

  // Merge overlapping ranges
  const mergedRanges = groundedRanges.reduce(
    (acc, range) => {
      const last = acc[acc.length - 1]
      if (last && last.end >= range.start) {
        last.end = Math.max(last.end, range.end)
      } else {
        acc.push(range)
      }
      return acc
    },
    [] as typeof groundedRanges
  )

  // Build highlighted output
  let currentIndex = 0
  mergedRanges.forEach(({ start, end }, idx) => {
    if (currentIndex < start) {
      // Ungrounded segment (yellow highlight)
      elements.push(
        <span
          key={`ungrounded-${idx}`}
          style={{
            backgroundColor: colors.status.warning,
            opacity: 0.2,
            padding: '1px 2px',
            borderRadius: '2px',
          }}
        >
          {output.substring(currentIndex, start)}
        </span>
      )
    }

    // Grounded segment
    elements.push(
      <span
        key={`grounded-${idx}`}
        style={{
          backgroundColor: colors.status.success,
          opacity: 0.15,
          padding: '1px 2px',
          borderRadius: '2px',
          borderLeft: `2px solid ${colors.status.success}`,
        }}
      >
        {output.substring(start, end)}
      </span>
    )

    currentIndex = end
  })

  // Remaining ungrounded
  if (currentIndex < output.length) {
    elements.push(
      <span
        key="ungrounded-end"
        style={{
          backgroundColor: colors.status.warning,
          opacity: 0.2,
          padding: '1px 2px',
          borderRadius: '2px',
        }}
      >
        {output.substring(currentIndex)}
      </span>
    )
  }

  return elements.length > 0 ? elements : [output]
}

/**
 * GroundingView: Shows RAG context and how output is grounded in retrieved chunks
 * Left: chunks ranked by relevance, Right: output with segment highlighting
 */
export const GroundingView: React.FC<GroundingViewProps> = ({ chunks, output }) => {
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null)
  const sortedChunks = useMemo(
    () => [...chunks].sort((a, b) => b.similarity_score - a.similarity_score),
    [chunks]
  )

  const groundedElements = useMemo(() => highlightGroundedText(output, chunks), [output, chunks])

  const ungroundedCount = useMemo(() => {
    const allSegments = groundedElements.filter((el) => {
      if (typeof el !== 'string' && React.isValidElement(el)) {
        const span = el as React.ReactElement<any>
        return span.props?.style?.backgroundColor === colors.status.warning
      }
      return false
    })
    return allSegments.length
  }, [groundedElements])

  return (
    <div className="flex gap-4 h-full">
      {/* Left: Retrieved chunks */}
      <div className="w-1/2 flex flex-col border-r border-border">
        <div className="text-sm font-semibold text-foreground mb-3 flex items-center justify-between">
          <span>Retrieved Chunks ({chunks.length})</span>
          <span className="text-xs text-muted-foreground font-normal">
            Ranked by relevance
          </span>
        </div>

        <div className="flex-1 overflow-y-auto space-y-2">
          {sortedChunks.map((chunk) => (
            <div
              key={chunk.id}
              onClick={() => setSelectedChunkId(chunk.id)}
              className={`p-2 rounded-md cursor-pointer transition-colors ${
                selectedChunkId === chunk.id
                  ? 'bg-primary bg-opacity-20 border border-primary'
                  : 'bg-muted-background border border-border hover:bg-opacity-50'
              }`}
            >
              <div className="flex items-start justify-between gap-2 mb-1">
                <span className="text-xs font-mono text-muted-foreground truncate">
                  {chunk.source}
                </span>
                <span className="text-xs font-semibold text-foreground">
                  {(chunk.similarity_score * 100).toFixed(0)}%
                </span>
              </div>
              <div className="text-xs text-foreground leading-tight line-clamp-3">
                {chunk.text}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Right: Model output with grounding highlight */}
      <div className="w-1/2 flex flex-col">
        <div className="text-sm font-semibold text-foreground mb-3 flex items-center justify-between">
          <span>Model Output</span>
          <span className="text-xs text-muted-foreground font-normal">
            {ungroundedCount > 0 ? (
              <>
                <span className="text-status-warning">
                  {ungroundedCount} ungrounded segments
                </span>
                {' (potential hallucination)'}
              </>
            ) : (
              <span className="text-status-success">Fully grounded</span>
            )}
          </span>
        </div>

        <div className="flex-1 overflow-y-auto bg-muted-background p-3 rounded-md border border-border">
          <div className="text-sm leading-relaxed text-foreground">
            {groundedElements}
          </div>
        </div>

        {/* Legend */}
        <div className="mt-3 flex gap-4 text-xs text-muted-foreground">
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded"
              style={{
                backgroundColor: colors.status.success,
                opacity: 0.3,
              }}
            />
            <span>Grounded in retrieval</span>
          </div>
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded"
              style={{
                backgroundColor: colors.status.warning,
                opacity: 0.3,
              }}
            />
            <span>Ungrounded (potential hallucination)</span>
          </div>
        </div>
      </div>
    </div>
  )
}
