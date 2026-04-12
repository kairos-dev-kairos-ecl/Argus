import React, { useCallback, useEffect, useState } from 'react'
import ReactFlow, {
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  MiniMap,
  Panel,
} from 'reactflow'
import type { Node, Edge } from 'reactflow'
import * as dagre from 'dagre'
import 'reactflow/dist/style.css'
import type { Trace } from '../types/index'
import { useReasoningGraph } from '../hooks/useReasoningGraph'
import { useTraceViewStore } from '../stores/traceViewStore'
import {
  InputNode,
  RetrievalNode,
  SafetyCheckNode,
  InferenceNode,
  ToolCallNode,
  OutputNode,
  DecisionNode,
  DetectionHitNode,
  NodeDetailPanel,
} from './ReasoningGraph/CustomNodeRenderer'
import { colors } from '../lib/design-tokens'

const nodeTypes = {
  input: InputNode,
  retrieval: RetrievalNode,
  safety_check: SafetyCheckNode,
  inference: InferenceNode,
  tool_call: ToolCallNode,
  output: OutputNode,
  decision: DecisionNode,
  detection_hit: DetectionHitNode,
}

interface ReasoningGraphProps {
  trace: Trace
  selectedNodeId?: string
  onSelectNode?: (nodeId: string | null) => void
}

/**
 * Applies Dagre layout to nodes and edges
 */
function getLayoutedElements(nodes: Node[], edges: Edge[]) {
  const g = new dagre.graphlib.Graph({ compound: true })
  g.setGraph({ rankdir: 'LR', nodesep: 50, ranksep: 100 })
  g.setDefaultEdgeLabel(() => ({}))

  nodes.forEach((node) => {
    g.setNode(node.id, { width: 180, height: 60 })
  })

  edges.forEach((edge) => {
    g.setEdge(edge.source, edge.target)
  })

  dagre.layout(g)

  const layoutedNodes = nodes.map((node) => {
    const nodeWithPosition = g.node(node.id)
    return {
      ...node,
      position: {
        x: nodeWithPosition.x - 90,
        y: nodeWithPosition.y - 30,
      },
    }
  })

  return { nodes: layoutedNodes, edges }
}

/**
 * ReasoningGraph: Interactive DAG visualization of inference trace
 * Shows node types, confidence, connections, and real-time growth
 */
export const ReasoningGraph: React.FC<ReasoningGraphProps> = ({
  trace,
  selectedNodeId,
  onSelectNode,
}) => {
  const { nodes: initialNodes, edges: initialEdges } = useReasoningGraph(trace.spans)
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)
  const [isCollapsed, setIsCollapsed] = useState(false)
  const { selectedNodeId: storeSelectedNodeId, setSelectedNode: setStoreSelectedNode } =
    useTraceViewStore()

  // Apply layout when data changes
  useEffect(() => {
    // Check if we need to collapse large traces
    if (initialNodes.length > 100) {
      setIsCollapsed(true)
    }

    const { nodes: layoutedNodes, edges: layoutedEdges } =
      getLayoutedElements(initialNodes, initialEdges)

    setNodes(layoutedNodes)
    setEdges(layoutedEdges)
  }, [initialNodes, initialEdges, setNodes, setEdges])

  // Sync selected node from store
  useEffect(() => {
    const nodeToSelect = storeSelectedNodeId || selectedNodeId
    if (nodeToSelect) {
      const node = nodes.find((n) => n.id === nodeToSelect)
      if (node) {
        setSelectedNode(node)
      }
    }
  }, [storeSelectedNodeId, selectedNodeId, nodes])

  const handleNodeClick = useCallback(
    (event: React.MouseEvent, node: Node) => {
      event.preventDefault()
      setSelectedNode(node)
      // Update store to sync with timeline
      setStoreSelectedNode(node.id)
      onSelectNode?.(node.id)
    },
    [onSelectNode, setStoreSelectedNode]
  )

  return (
    <div className="w-full h-full flex">
      <ReactFlow
        nodes={nodes.map((node) => ({
          ...node,
          selected: node.id === selectedNode?.id,
        }))}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        onNodeClick={handleNodeClick}
        fitView
      >
        <Background color={colors.border} gap={16} />
        <Controls />
        <MiniMap
          nodeColor={(node: Node) => {
            const nodeType = node.data?.nodeType
            const confidenceMap: Record<string, string> = {
              input: colors.layer.l4,
              retrieval: colors.layer.l4,
              safety_check: colors.layer.l3,
              inference: colors.layer.l7,
              tool_call: colors.layer.l6,
              output: colors.layer.l4,
              decision: colors.layer.l3,
              detection_hit: colors.status.error,
            }
            return confidenceMap[nodeType] || colors.primary
          }}
          style={{
            backgroundColor: colors.muted_background,
            border: `1px solid ${colors.border}`,
          }}
        />

        <Panel position="top-left" className="bg-background border border-border rounded-md p-2">
          <div className="text-sm text-foreground">
            <div className="font-semibold mb-2">Trace Graph</div>
            <div className="text-xs text-muted-foreground">
              {nodes.length} nodes
              {isCollapsed && (
                <button
                  onClick={() => setIsCollapsed(false)}
                  className="ml-2 text-primary hover:underline"
                >
                  Expand
                </button>
              )}
            </div>
          </div>
        </Panel>
      </ReactFlow>

      {/* Node detail panel */}
      {selectedNode && (
        <NodeDetailPanel
          node={selectedNode}
          span={trace.spans.find((s) => s.signal_id === selectedNode.id) || null}
          onClose={() => {
            setSelectedNode(null)
            setStoreSelectedNode(null)
            onSelectNode?.(null)
          }}
        />
      )}
    </div>
  )
}
