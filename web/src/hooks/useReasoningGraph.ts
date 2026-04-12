import { useMemo } from 'react'
import type { Node, Edge } from 'reactflow'
import type { Span, Layer } from '../types/index'

/**
 * Node type for reasoning graph visualization
 */
export type ReasoningNodeType =
  | 'input'
  | 'retrieval'
  | 'safety_check'
  | 'inference'
  | 'tool_call'
  | 'output'
  | 'decision'
  | 'detection_hit'

/**
 * Determines node type based on span metadata
 */
function determineNodeType(span: Span, layer: Layer): ReasoningNodeType {
  // Infer from layer and message content
  const messageType = span.message.toLowerCase()

  // L9_LLM_API, L8_LLM_FRAMEWORK typically = Inference
  if (layer === 'L9_LLM_API' || layer === 'L8_LLM_FRAMEWORK') {
    if (messageType.includes('tool') || messageType.includes('function')) return 'tool_call'
    if (messageType.includes('safety')) return 'safety_check'
    if (messageType.includes('decision')) return 'decision'
    return 'inference'
  }

  // L10_SEMANTIC = Retrieval
  if (layer === 'L10_SEMANTIC') {
    if (messageType.includes('retrieval') || messageType.includes('embedding')) return 'retrieval'
    if (messageType.includes('grounding')) return 'retrieval'
  }

  // Detection hits
  if (messageType.includes('detection') || messageType.includes('threat')) return 'detection_hit'

  // Early layers: Input
  if (layer === 'L1_HARDWARE' || layer === 'L2_KERNEL' || layer === 'L3_RUNTIME') return 'input'

  // Default: infer from message
  if (messageType.includes('retriev')) return 'retrieval'
  if (messageType.includes('tool')) return 'tool_call'
  if (messageType.includes('output') || messageType.includes('response')) return 'output'
  if (messageType.includes('decision')) return 'decision'
  if (messageType.includes('safety')) return 'safety_check'

  return 'inference'
}

/**
 * Computes a confidence score for a span (0-1)
 * Used for node opacity and visual effects
 */
function computeConfidence(span: Span): number {
  // If span has error status, low confidence
  if (span.status === 'error') return 0.3

  // Default: medium-high confidence
  return 0.8
}

/**
 * Aggregates spans into graph nodes and edges
 * Returns React Flow nodes/edges ready for visualization
 */
export function useReasoningGraph(spans: Span[]) {
  return useMemo(() => {
    const nodes: Node[] = []
    const edges: Edge[] = []
    const nodeMap = new Map<string, Node>()

    // Create a node for each span
    spans.forEach((span, index) => {
      const nodeType = determineNodeType(span, span.layer)
      const confidence = computeConfidence(span)

      const node: Node = {
        id: span.signal_id,
        data: {
          label: span.message.substring(0, 30) + (span.message.length > 30 ? '...' : ''),
          nodeType,
          layer: span.layer,
          confidence,
          duration_ms: span.duration_ms,
          status: span.status,
          fullMessage: span.message,
        },
        position: { x: index * 150, y: 0 }, // Layout will be applied by Dagre
        type: nodeType,
      }

      nodes.push(node)
      nodeMap.set(span.signal_id, node)
    })

    // Create edges for parent-child relationships
    spans.forEach((span) => {
      if (span.parent_signal_id && nodeMap.has(span.parent_signal_id)) {
        const edge: Edge = {
          id: `${span.parent_signal_id}-${span.signal_id}`,
          source: span.parent_signal_id,
          target: span.signal_id,
          animated: false,
          data: {
            volume: 1, // Could be enhanced with actual data volume
            confidence: computeConfidence(span),
          },
        }
        edges.push(edge)
      }
    })

    return { nodes, edges }
  }, [spans])
}
