import React from 'react'
import { Handle, Position } from 'reactflow'
import { colors } from '../../lib/design-tokens'

interface CustomNodeProps {
  data: {
    label: string
    nodeType: string
    layer: string
    confidence: number
    duration_ms: number
    status: 'ok' | 'error'
    fullMessage: string
  }
  isConnecting?: boolean
  selected?: boolean
}

const nodeStyles = (confidence: number, selected?: boolean) => ({
  opacity: 0.3 + confidence * 0.7, // Range 0.3-1.0
  borderWidth: selected ? '2px' : '1px',
  borderColor: selected ? colors.primary : colors.border,
  transition: 'all 200ms ease-out',
})

const nodeGlow = (confidence: number) =>
  confidence > 0.7
    ? `0 0 12px rgba(59, 130, 246, ${confidence})`
    : `0 0 6px rgba(100, 100, 100, ${confidence * 0.5})`

/**
 * Input Node: rounded rectangle, blue
 * Represents input data or prompts
 */
export const InputNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l4,
        boxShadow: nodeGlow(data.confidence),
        borderRadius: '8px',
        padding: '8px 12px',
        minWidth: '140px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex items-center gap-2">
        <span className="text-lg">📥</span>
        <div className="flex-1">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Retrieval Node: diamond shape, green
 * Represents RAG retrieval from vector database
 */
export const RetrievalNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l4,
        boxShadow: nodeGlow(data.confidence),
        clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)',
        width: '120px',
        height: '60px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        padding: '4px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex flex-col items-center justify-center">
        <span className="text-lg">📚</span>
        <div className="text-xs">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Safety Check Node: hexagon, orange
 * Represents safety/content filtering passes
 */
export const SafetyCheckNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l3,
        boxShadow: nodeGlow(data.confidence),
        clipPath: 'polygon(50% 0%, 93.3% 25%, 93.3% 75%, 50% 100%, 6.7% 75%, 6.7% 25%)',
        width: '120px',
        height: '70px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '4px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex flex-col items-center justify-center">
        <span className="text-lg">🛡️</span>
        <div className="text-xs">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Inference Node: rectangle, purple
 * Represents LLM inference/forward pass
 */
export const InferenceNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l7,
        boxShadow: nodeGlow(data.confidence),
        borderRadius: '4px',
        padding: '8px 12px',
        minWidth: '140px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex items-center gap-2">
        <span className="text-lg">🧠</span>
        <div className="flex-1">{data.label}</div>
      </div>
      <div className="text-xs mt-1 text-opacity-70">{data.duration_ms}ms</div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Tool Call Node: parallelogram, teal
 * Represents function/tool invocations
 */
export const ToolCallNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l6,
        boxShadow: nodeGlow(data.confidence),
        clipPath: 'polygon(20% 0%, 100% 0%, 80% 100%, 0% 100%)',
        padding: '8px 12px',
        minWidth: '140px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex items-center gap-2">
        <span className="text-lg">🔧</span>
        <div className="flex-1">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Output Node: rounded rectangle, blue
 * Represents final model output/response
 */
export const OutputNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l4,
        boxShadow: nodeGlow(data.confidence),
        borderRadius: '8px',
        padding: '8px 12px',
        minWidth: '140px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex items-center gap-2">
        <span className="text-lg">📤</span>
        <div className="flex-1">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Decision Node: octagon, gold
 * Represents branching logic or route selection
 */
export const DecisionNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: colors.layer.l3,
        boxShadow: nodeGlow(data.confidence),
        clipPath: 'polygon(30% 0%, 70% 0%, 100% 30%, 100% 70%, 70% 100%, 30% 100%, 0% 70%, 0% 30%)',
        width: '120px',
        height: '80px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '4px',
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex flex-col items-center justify-center">
        <span className="text-lg">⚡</span>
        <div className="text-xs text-center">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/**
 * Detection Hit Node: circle, red
 * Represents security detection/alert trigger
 */
export const DetectionHitNode: React.FC<CustomNodeProps> = ({ data, selected }) => {
  const isError = data.status === 'error'
  return (
    <div
      style={{
        ...nodeStyles(data.confidence, selected),
        backgroundColor: isError ? colors.status.error : colors.status.warning,
        boxShadow: nodeGlow(data.confidence),
        borderRadius: '50%',
        width: '80px',
        height: '80px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '4px',
        animation: isError ? 'pulse 2s infinite' : undefined,
      }}
      className="text-xs font-medium text-white"
      title={data.fullMessage}
    >
      <div className="flex flex-col items-center justify-center">
        <span className="text-2xl">⚠️</span>
        <div className="text-xs text-center mt-1">{data.label}</div>
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.7; }
        }
      `}</style>
    </div>
  )
}

/**
 * Node Detail Panel: shows metadata for selected node
 */
interface NodeDetailPanelProps {
  node: any
  span: any
  onClose: () => void
}

export const NodeDetailPanel: React.FC<NodeDetailPanelProps> = ({ node, span, onClose }) => {
  return (
    <div
      className="w-80 bg-muted-background border border-border rounded-lg p-4 shadow-lg"
      style={{
        position: 'absolute',
        right: '16px',
        top: '16px',
        zIndex: 1000,
        maxHeight: 'calc(100% - 32px)',
        overflowY: 'auto',
      }}
    >
      <div className="flex justify-between items-center mb-3">
        <h3 className="text-sm font-semibold text-foreground">Node Details</h3>
        <button
          onClick={onClose}
          className="text-muted-foreground hover:text-foreground text-lg leading-none"
        >
          ✕
        </button>
      </div>

      <div className="space-y-2 text-xs">
        <div>
          <div className="text-muted-foreground">Type</div>
          <div className="text-foreground font-mono">{node.data.nodeType}</div>
        </div>

        <div>
          <div className="text-muted-foreground">Layer</div>
          <div className="text-foreground font-mono">{node.data.layer}</div>
        </div>

        <div>
          <div className="text-muted-foreground">Confidence</div>
          <div className="text-foreground">{(node.data.confidence * 100).toFixed(0)}%</div>
        </div>

        {span && (
          <>
            <div>
              <div className="text-muted-foreground">Duration</div>
              <div className="text-foreground">{span.duration_ms}ms</div>
            </div>

            <div>
              <div className="text-muted-foreground">Status</div>
              <div className={span.status === 'ok' ? 'text-status-success' : 'text-status-error'}>
                {span.status.toUpperCase()}
              </div>
            </div>

            <div>
              <div className="text-muted-foreground">Message</div>
              <div className="text-foreground break-words">{span.message}</div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
