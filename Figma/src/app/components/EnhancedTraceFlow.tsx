/**
 * Enhanced Trace Flow - Onum/Falcon Style
 *
 * Rich entity tracing with React Flow-based pipeline visualization.
 * Features:
 * - Node-based pipeline builder with prompt lifecycle mapping
 * - Interactive correlation search via ⌘K/Ctrl+K command palette
 * - Entity encapsulation (What, How, Where, When)
 * - Right-side details panel for comprehensive entity information
 * - Real-time highlighting of matching nodes and connected edges
 *
 * @component
 */

import { useState, useCallback, useMemo, useEffect } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  Connection,
  Edge,
  Node,
  NodeProps,
  Handle,
  Position,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Command } from 'cmdk';
import { Search, X, ChevronRight } from 'lucide-react';

interface EntityData {
  label: string;
  type: 'prompt' | 'parsing' | 'sanitization' | 'tokenization' | 'inference' | 'tool' | 'vector' | 'api' | 'response';
  what: string;
  how: string;
  where: string;
  when: string;
  tokens?: number;
  latency?: number;
  model?: string;
  status?: 'success' | 'warning' | 'error';
}

const CustomNode = ({ data, selected }: NodeProps<EntityData>) => {
  const getNodeColor = (type: string, status?: string) => {
    if (status === 'error') return '#FF2A00';
    if (status === 'warning') return '#FFB300';

    switch (type) {
      case 'prompt': return '#9D4EDD';
      case 'parsing': return '#7B2CBF';
      case 'sanitization': return '#5A189A';
      case 'tokenization': return '#3C096C';
      case 'inference': return '#00F0FF';
      case 'vector': return '#FFB300';
      case 'tool': return '#FF2A00';
      case 'api': return '#10B981';
      case 'response': return '#00F0FF';
      default: return '#343A40';
    }
  };

  const color = getNodeColor(data.type, data.status);

  return (
    <div
      style={{
        background: '#111216',
        border: `2px solid ${selected ? color : 'rgba(52, 58, 64, 0.6)'}`,
        borderRadius: '8px',
        padding: '12px',
        minWidth: '200px',
        boxShadow: selected ? `0 0 20px ${color}40` : '0 2px 8px rgba(0,0,0,0.3)',
        transition: 'all 0.2s ease',
      }}
    >
      <Handle type="target" position={Position.Left} style={{ background: color }} />

      <div style={{ marginBottom: '8px', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <div
          style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: color,
            boxShadow: `0 0 8px ${color}`,
          }}
        />
        <div style={{ fontSize: '11px', fontWeight: 700, color, letterSpacing: '0.5px' }}>
          {data.type.toUpperCase()}
        </div>
      </div>

      <div style={{ fontSize: '13px', color: '#E9ECEF', fontWeight: 600, marginBottom: '8px' }}>
        {data.label}
      </div>

      {data.tokens && (
        <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>
          <span style={{ color: '#00F0FF' }}>{data.tokens}</span> tokens
        </div>
      )}

      {data.latency && (
        <div style={{ fontSize: '10px', color: '#343A40' }}>
          <span style={{ color: data.latency > 1000 ? '#FFB300' : '#E9ECEF' }}>{data.latency}ms</span>
        </div>
      )}

      <Handle type="source" position={Position.Right} style={{ background: color }} />
    </div>
  );
};

const DetailsPanel = ({ node, onClose }: { node: Node<EntityData> | null; onClose: () => void }) => {
  if (!node) return null;

  return (
    <div
      style={{
        position: 'absolute',
        top: 0,
        right: 0,
        width: '360px',
        height: '100%',
        background: '#050506',
        borderLeft: '1px solid #343A40',
        zIndex: 10,
        overflow: 'auto',
      }}
    >
      <div
        style={{
          padding: '16px',
          borderBottom: '1px solid #343A40',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <h3
          style={{
            fontFamily: 'var(--font-display)',
            fontSize: '14px',
            fontWeight: 600,
            color: '#E9ECEF',
            letterSpacing: '1px',
          }}
        >
          ENTITY DETAILS
        </h3>
        <button
          onClick={onClose}
          style={{
            background: 'transparent',
            border: 'none',
            color: '#343A40',
            cursor: 'pointer',
            padding: '4px',
          }}
        >
          <X size={16} />
        </button>
      </div>

      <div style={{ padding: '16px' }}>
        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>TYPE</div>
          <div style={{ fontSize: '13px', color: '#00F0FF', fontWeight: 600 }}>{node.data.type.toUpperCase()}</div>
        </div>

        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>WHAT</div>
          <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.what}</div>
        </div>

        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>HOW</div>
          <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.how}</div>
        </div>

        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>WHERE</div>
          <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.where}</div>
        </div>

        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>WHEN</div>
          <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.when}</div>
        </div>

        {node.data.model && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>MODEL</div>
            <div style={{ fontSize: '12px', color: '#00F0FF' }}>{node.data.model}</div>
          </div>
        )}

        {node.data.tokens && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>TOKENS</div>
            <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.tokens}</div>
          </div>
        )}

        {node.data.latency && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '4px' }}>LATENCY</div>
            <div style={{ fontSize: '12px', color: '#E9ECEF' }}>{node.data.latency}ms</div>
          </div>
        )}
      </div>
    </div>
  );
};

export function EnhancedTraceFlow() {
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedNode, setSelectedNode] = useState<Node<EntityData> | null>(null);
  const [highlightedNodeIds, setHighlightedNodeIds] = useState<Set<string>>(new Set());
  const [highlightedEdgeIds, setHighlightedEdgeIds] = useState<Set<string>>(new Set());

  const initialNodes: Node<EntityData>[] = useMemo(
    () => [
      {
        id: '1',
        type: 'custom',
        position: { x: 50, y: 250 },
        data: {
          label: 'User Query',
          type: 'prompt',
          what: 'Initial user request received',
          how: 'HTTP POST /v1/chat/completions',
          where: 'api-gateway-01',
          when: '2026-04-20T14:42:18.234Z',
        },
      },
      {
        id: '2',
        type: 'custom',
        position: { x: 300, y: 150 },
        data: {
          label: 'Prompt Parsing',
          type: 'parsing',
          what: 'Extract and structure user intent',
          how: 'Regex + NLP entity extraction',
          where: 'prompt-processor-svc',
          when: '2026-04-20T14:42:18.240Z',
          latency: 6,
        },
      },
      {
        id: '3',
        type: 'custom',
        position: { x: 300, y: 250 },
        data: {
          label: 'Sanitization',
          type: 'sanitization',
          what: 'Remove malicious payloads',
          how: 'PII detection + SQL injection filter',
          where: 'guardrails-svc',
          when: '2026-04-20T14:42:18.246Z',
          latency: 12,
        },
      },
      {
        id: '4',
        type: 'custom',
        position: { x: 300, y: 350 },
        data: {
          label: 'Tokenization',
          type: 'tokenization',
          what: 'Convert prompt to tokens',
          how: 'BPE tokenizer (cl100k_base)',
          where: 'tokenizer-svc',
          when: '2026-04-20T14:42:18.258Z',
          tokens: 632,
          latency: 8,
        },
      },
      {
        id: '5',
        type: 'custom',
        position: { x: 600, y: 50 },
        data: {
          label: 'Vector Search',
          type: 'vector',
          what: 'Semantic context retrieval',
          how: 'FAISS nearest neighbor search',
          where: 'vector-db-cluster-01',
          when: '2026-04-20T14:42:18.266Z',
          tokens: 128,
          latency: 45,
        },
      },
      {
        id: '6',
        type: 'custom',
        position: { x: 600, y: 250 },
        data: {
          label: 'GPT-4 Inference',
          type: 'inference',
          what: 'Generate completion',
          how: 'Transformer decoder (GPT-4)',
          where: 'gpu-cluster-01-node-07',
          when: '2026-04-20T14:42:18.311Z',
          model: 'gpt-4-turbo',
          tokens: 2456,
          latency: 1200,
          status: 'warning',
        },
      },
      {
        id: '7',
        type: 'custom',
        position: { x: 900, y: 250 },
        data: {
          label: 'Tool Execution',
          type: 'tool',
          what: 'Execute function call',
          how: 'Python function invocation',
          where: 'tool-executor-pod-42',
          when: '2026-04-20T14:42:19.511Z',
          latency: 300,
        },
      },
      {
        id: '8',
        type: 'custom',
        position: { x: 1200, y: 250 },
        data: {
          label: 'Response Format',
          type: 'response',
          what: 'Structure final output',
          how: 'JSON schema validation',
          where: 'api-gateway-01',
          when: '2026-04-20T14:42:19.811Z',
          latency: 10,
        },
      },
    ],
    []
  );

  const initialEdges: Edge[] = useMemo(
    () => [
      {
        id: 'e1-2',
        source: '1',
        target: '2',
        animated: true,
        style: { stroke: '#9D4EDD', strokeWidth: 2 },
      },
      {
        id: 'e1-3',
        source: '1',
        target: '3',
        animated: true,
        style: { stroke: '#7B2CBF', strokeWidth: 2 },
      },
      {
        id: 'e1-4',
        source: '1',
        target: '4',
        animated: true,
        style: { stroke: '#5A189A', strokeWidth: 2 },
      },
      {
        id: 'e4-5',
        source: '4',
        target: '5',
        animated: true,
        style: { stroke: '#FFB300', strokeWidth: 2 },
      },
      {
        id: 'e4-6',
        source: '4',
        target: '6',
        animated: true,
        style: { stroke: '#00F0FF', strokeWidth: 2 },
      },
      {
        id: 'e6-7',
        source: '6',
        target: '7',
        animated: true,
        style: { stroke: '#FF2A00', strokeWidth: 2 },
      },
      {
        id: 'e7-8',
        source: '7',
        target: '8',
        animated: true,
        style: { stroke: '#00F0FF', strokeWidth: 2 },
      },
    ],
    []
  );

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  const nodeTypes = useMemo(() => ({ custom: CustomNode }), []);

  const handleSearch = (query: string) => {
    setSearchQuery(query);

    if (!query.trim()) {
      setHighlightedNodeIds(new Set());
      setHighlightedEdgeIds(new Set());
      return;
    }

    const matchingNodeIds = new Set<string>();
    const connectedEdgeIds = new Set<string>();

    nodes.forEach((node) => {
      const searchableText = `${node.data.label} ${node.data.what} ${node.data.how} ${node.data.where} ${node.data.type} ${node.data.model || ''}`.toLowerCase();
      if (searchableText.includes(query.toLowerCase())) {
        matchingNodeIds.add(node.id);
      }
    });

    edges.forEach((edge) => {
      if (matchingNodeIds.has(edge.source) || matchingNodeIds.has(edge.target)) {
        connectedEdgeIds.add(edge.id);
      }
    });

    setHighlightedNodeIds(matchingNodeIds);
    setHighlightedEdgeIds(connectedEdgeIds);
  };

  const filteredNodes = useMemo(() => {
    if (highlightedNodeIds.size === 0) return nodes;

    return nodes.map((node) => ({
      ...node,
      style: {
        ...node.style,
        opacity: highlightedNodeIds.has(node.id) ? 1 : 0.2,
      },
    }));
  }, [nodes, highlightedNodeIds]);

  const filteredEdges = useMemo(() => {
    if (highlightedEdgeIds.size === 0) return edges;

    return edges.map((edge) => ({
      ...edge,
      style: {
        ...edge.style,
        opacity: highlightedEdgeIds.has(edge.id) ? 1 : 0.1,
      },
    }));
  }, [edges, highlightedEdgeIds]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setSearchOpen((prev) => !prev);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <div className="h-full flex flex-col" style={{ background: '#050506' }}>
      <div className="px-6 py-4 flex items-center justify-between" style={{ borderBottom: '1px solid #343A40' }}>
        <div>
          <h1
            style={{
              fontFamily: 'var(--font-display)',
              fontSize: '20px',
              fontWeight: 700,
              color: '#E9ECEF',
              letterSpacing: '1px',
            }}
          >
            RICH ENTITY TRACING
          </h1>
          <p style={{ fontSize: '11px', color: '#343A40', marginTop: '4px' }}>
            trace_id: d4f2b8a9-3c5e-4d2a-8b1f-9e7c6d5a4b3c
          </p>
        </div>

        <button
          onClick={() => setSearchOpen(!searchOpen)}
          style={{
            padding: '8px 16px',
            background: 'transparent',
            border: '1px solid #00F0FF',
            color: '#00F0FF',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontSize: '11px',
            fontWeight: 600,
          }}
        >
          <Search size={14} />
          CORRELATION SEARCH (⌘K)
        </button>
      </div>

      <div className="flex-1 relative">
        <ReactFlow
          nodes={filteredNodes}
          edges={filteredEdges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onNodeClick={(_, node) => setSelectedNode(node as Node<EntityData>)}
          nodeTypes={nodeTypes}
          fitView
          style={{ background: '#050506' }}
        >
          <Background color="#343A40" gap={16} />
          <Controls />
          <MiniMap
            style={{ background: '#111216', border: '1px solid #343A40' }}
            nodeColor={(node) => {
              const n = node as Node<EntityData>;
              return n.data.type === 'inference' ? '#00F0FF' : '#343A40';
            }}
          />
        </ReactFlow>

        {selectedNode && <DetailsPanel node={selectedNode} onClose={() => setSelectedNode(null)} />}

        {searchOpen && (
          <div
            style={{
              position: 'absolute',
              top: '20px',
              left: '50%',
              transform: 'translateX(-50%)',
              width: '500px',
              background: '#111216',
              border: '2px solid #00F0FF',
              boxShadow: '0 8px 32px rgba(0, 240, 255, 0.2)',
              zIndex: 100,
            }}
          >
            <Command>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  padding: '12px 16px',
                  borderBottom: '1px solid #343A40',
                }}
              >
                <Search size={16} style={{ color: '#343A40', marginRight: '8px' }} />
                <input
                  autoFocus
                  placeholder="Search entities, IPs, models, or strings..."
                  value={searchQuery}
                  onChange={(e) => handleSearch(e.target.value)}
                  style={{
                    flex: 1,
                    background: 'transparent',
                    border: 'none',
                    outline: 'none',
                    color: '#E9ECEF',
                    fontSize: '13px',
                  }}
                />
                <button
                  onClick={() => {
                    setSearchOpen(false);
                    setSearchQuery('');
                    setHighlightedNodeIds(new Set());
                    setHighlightedEdgeIds(new Set());
                  }}
                  style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: '#343A40' }}
                >
                  <X size={16} />
                </button>
              </div>

              {searchQuery && (
                <div style={{ padding: '8px' }}>
                  <div style={{ fontSize: '10px', color: '#343A40', padding: '8px', letterSpacing: '1px' }}>
                    MATCHING ENTITIES ({highlightedNodeIds.size})
                  </div>
                  {Array.from(highlightedNodeIds).map((nodeId) => {
                    const node = nodes.find((n) => n.id === nodeId);
                    if (!node) return null;

                    return (
                      <button
                        key={nodeId}
                        onClick={() => {
                          setSelectedNode(node as Node<EntityData>);
                          setSearchOpen(false);
                        }}
                        style={{
                          width: '100%',
                          padding: '12px',
                          background: 'transparent',
                          border: 'none',
                          textAlign: 'left',
                          cursor: 'pointer',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.background = '#1A1D24')}
                        onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                      >
                        <div>
                          <div style={{ fontSize: '12px', color: '#E9ECEF', fontWeight: 600 }}>
                            {node.data.label}
                          </div>
                          <div style={{ fontSize: '10px', color: '#343A40', marginTop: '2px' }}>
                            {node.data.type} · {node.data.where}
                          </div>
                        </div>
                        <ChevronRight size={14} style={{ color: '#343A40' }} />
                      </button>
                    );
                  })}
                </div>
              )}
            </Command>
          </div>
        )}
      </div>
    </div>
  );
}
