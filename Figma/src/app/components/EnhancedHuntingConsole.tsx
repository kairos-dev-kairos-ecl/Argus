/**
 * Enhanced Hunting Console - Onum/Falcon Style
 *
 * Advanced query interface with analytics panel and visualizations.
 * Features:
 * - SQL-like query editor with quick snippet insertion
 * - Real-time result rendering with expandable JSON payloads
 * - Right analytics panel with latency distribution charts
 * - Status-based color coding (success/warning/error)
 * - Top models breakdown and success rate metrics
 *
 * @component
 */

import { useState } from 'react';
import ReactECharts from 'echarts-for-react';
import { Play, ChevronRight, ChevronDown, Search, Filter } from 'lucide-react';

interface QueryResult {
  id: string;
  timestamp: string;
  model: string;
  tokens: number;
  ttft: number;
  latency: number;
  layer: string;
  payload: any;
  status: 'running' | 'success' | 'warning' | 'error';
}

const mockResults: QueryResult[] = [
  {
    id: '1',
    timestamp: '2026-04-16T14:42:18.234Z',
    model: 'gpt-4-turbo',
    tokens: 2456,
    ttft: 234,
    latency: 1200,
    layer: 'L5',
    status: 'success',
    payload: {
      request_id: 'req_d4f2b8a9',
      node: 'cluster-01-gpu-07',
      gpu_util: 89.4,
      vram_allocated: '8.2GB',
      prompt_tokens: 632,
      completion_tokens: 1824,
      temperature: 0.7,
      error: null,
    },
  },
  {
    id: '2',
    timestamp: '2026-04-16T14:41:52.891Z',
    model: 'gpt-4-turbo',
    tokens: 1834,
    ttft: 289,
    latency: 1842,
    layer: 'L5',
    status: 'warning',
    payload: {
      request_id: 'req_a3e1c7b5',
      node: 'cluster-01-gpu-04',
      gpu_util: 76.2,
      vram_allocated: '7.8GB',
      prompt_tokens: 512,
      completion_tokens: 1322,
      temperature: 0.7,
      error: null,
    },
  },
  {
    id: '3',
    timestamp: '2026-04-16T14:41:34.567Z',
    model: 'claude-3-opus',
    tokens: 3124,
    ttft: 412,
    latency: 2100,
    layer: 'L5',
    status: 'error',
    payload: {
      request_id: 'req_8b2f9c4d',
      node: 'cluster-02-gpu-02',
      gpu_util: 94.8,
      vram_allocated: '12.4GB',
      prompt_tokens: 824,
      completion_tokens: 2300,
      temperature: 0.9,
      error: 'CUDA_OUT_OF_MEMORY',
    },
  },
];

const querySnippets = [
  { label: 'High TTFT', query: 'model = "gpt-4-turbo" AND ttft > 200' },
  { label: 'Token Spike', query: 'tokens > 2000 AND layer = "L5"' },
  { label: 'GPU OOM', query: 'layer = "L1" AND error CONTAINS "CUDA_OUT_OF_MEMORY"' },
  { label: 'Slow RAG', query: 'layer = "L7" AND latency > 500' },
  { label: 'Failed Requests', query: 'status = "error"' },
];

export function EnhancedHuntingConsole() {
  const [query, setQuery] = useState('model = "gpt-4-turbo" AND ttft > 200');
  const [results, setResults] = useState<QueryResult[] | null>(null);
  const [isExecuting, setIsExecuting] = useState(false);
  const [expandedResults, setExpandedResults] = useState<Set<string>>(new Set());

  const handleExecute = () => {
    setIsExecuting(true);
    setTimeout(() => {
      setResults(mockResults);
      setIsExecuting(false);
    }, 1000);
  };

  const insertSnippet = (snippetQuery: string) => {
    setQuery(snippetQuery);
  };

  const toggleExpand = (id: string) => {
    const newExpanded = new Set(expandedResults);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    setExpandedResults(newExpanded);
  };

  const renderJsonTree = (obj: any, depth = 0): JSX.Element => {
    return (
      <div style={{ paddingLeft: depth > 0 ? '16px' : '0' }}>
        {Object.entries(obj).map(([key, value]) => (
          <div key={key} style={{ marginBottom: '4px' }}>
            {typeof value === 'object' && value !== null ? (
              <div>
                <span style={{ color: '#343A40' }}>{key}:</span>
                {renderJsonTree(value, depth + 1)}
              </div>
            ) : (
              <div>
                <span style={{ color: '#343A40' }}>{key}: </span>
                <span
                  style={{
                    color:
                      typeof value === 'number'
                        ? '#00F0FF'
                        : typeof value === 'string'
                        ? '#FFB300'
                        : value === null
                        ? '#343A40'
                        : '#E9ECEF',
                  }}
                >
                  {typeof value === 'string' ? `"${value}"` : String(value)}
                </span>
              </div>
            )}
          </div>
        ))}
      </div>
    );
  };

  const latencyDistributionOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#111216',
      borderColor: '#00F0FF',
      borderWidth: 1,
      textStyle: {
        color: '#E9ECEF',
        fontSize: 11,
      },
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '10%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: ['0-500ms', '500-1000ms', '1000-1500ms', '1500-2000ms', '2000ms+'],
      axisLine: {
        lineStyle: {
          color: '#343A40',
        },
      },
      axisLabel: {
        color: '#343A40',
        fontSize: 10,
      },
    },
    yAxis: {
      type: 'value',
      axisLine: {
        lineStyle: {
          color: '#343A40',
        },
      },
      axisLabel: {
        color: '#343A40',
        fontSize: 10,
      },
      splitLine: {
        lineStyle: {
          color: '#1A1D24',
        },
      },
    },
    series: [
      {
        name: 'Requests',
        type: 'bar',
        data: [
          { value: 245, itemStyle: { color: '#00F0FF' } },
          { value: 412, itemStyle: { color: '#00F0FF' } },
          { value: 189, itemStyle: { color: '#FFB300' } },
          { value: 78, itemStyle: { color: '#FF2A00' } },
          { value: 23, itemStyle: { color: '#FF2A00' } },
        ],
        barWidth: '60%',
      },
    ],
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success':
        return '#00F0FF';
      case 'warning':
        return '#FFB300';
      case 'error':
        return '#FF2A00';
      default:
        return '#343A40';
    }
  };

  return (
    <div className="h-full flex" style={{ background: '#050506', fontFamily: 'var(--font-primary)' }}>
      {/* Main Query & Results Panel */}
      <div className="flex-1 flex flex-col" style={{ minHeight: 0 }}>
        {/* Header */}
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
              AI TELEMETRY & HUNTING CONSOLE
            </h1>
            <p style={{ fontSize: '11px', color: '#343A40', marginTop: '4px' }}>
              Query AI/ML telemetry, logs, and payload data across all observability layers
            </p>
          </div>
        </div>

        {/* Query Snippets */}
        <div
          className="px-6 py-3 flex items-center gap-2 overflow-x-auto"
          style={{ borderBottom: '1px solid #343A40', background: '#111216' }}
        >
          <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px', marginRight: '8px' }}>
            QUICK QUERIES:
          </div>
          {querySnippets.map((snippet) => (
            <button
              key={snippet.label}
              onClick={() => insertSnippet(snippet.query)}
              className="px-3 py-1"
              style={{
                background: '#050506',
                border: '1px solid #343A40',
                color: '#E9ECEF',
                fontSize: '11px',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              {snippet.label}
            </button>
          ))}
        </div>

        <div className="flex-1 flex flex-col" style={{ minHeight: 0 }}>
          {/* Query Editor - 25% */}
          <div className="flex flex-col" style={{ height: '25%', borderBottom: '1px solid #343A40' }}>
            <div className="px-6 py-3 flex items-center justify-between" style={{ borderBottom: '1px solid #343A40' }}>
              <h2
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: '14px',
                  fontWeight: 600,
                  color: '#E9ECEF',
                  letterSpacing: '1px',
                }}
              >
                QUERY EDITOR
              </h2>

              <button
                onClick={handleExecute}
                disabled={isExecuting}
                className="px-6 py-2 flex items-center gap-2"
                style={{
                  background: isExecuting ? 'transparent' : '#00F0FF',
                  border: '1px solid #00F0FF',
                  color: isExecuting ? '#00F0FF' : '#050506',
                  fontFamily: 'var(--font-display)',
                  fontSize: '12px',
                  fontWeight: 600,
                  letterSpacing: '1px',
                  cursor: isExecuting ? 'default' : 'pointer',
                  opacity: isExecuting ? 0.6 : 1,
                }}
              >
                {isExecuting ? (
                  <>
                    <span className="animate-spin">⟳</span>
                    EXECUTING
                  </>
                ) : (
                  <>
                    <Play size={14} />
                    EXECUTE
                  </>
                )}
              </button>
            </div>

            <div className="flex-1 p-6">
              <textarea
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Enter query..."
                className="w-full h-full resize-none"
                style={{
                  background: '#050506',
                  border: '1px solid #343A40',
                  color: '#E9ECEF',
                  fontSize: '13px',
                  padding: '16px',
                  outline: 'none',
                  fontFamily: 'var(--font-primary)',
                }}
              />
            </div>
          </div>

          {/* Results Pane - 75% */}
          <div className="flex-1 flex flex-col" style={{ minHeight: 0 }}>
            <div className="px-6 py-3" style={{ borderBottom: '1px solid #343A40' }}>
              <h2
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: '14px',
                  fontWeight: 600,
                  color: '#E9ECEF',
                  letterSpacing: '1px',
                }}
              >
                RESULTS {results && `(${results.length})`}
              </h2>
            </div>

            <div className="flex-1 overflow-auto p-6">
              {!results ? (
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    height: '100%',
                    color: '#343A40',
                    fontSize: '11px',
                  }}
                >
                  Execute a query to see results_
                </div>
              ) : (
                <div className="space-y-4">
                  {results.map((result) => {
                    const isExpanded = expandedResults.has(result.id);

                    return (
                      <div
                        key={result.id}
                        style={{
                          background: '#111216',
                          border: '1px solid',
                          borderColor: getStatusColor(result.status),
                          boxShadow: `0 0 8px ${getStatusColor(result.status)}20`,
                        }}
                      >
                        {/* Result Header */}
                        <div
                          onClick={() => toggleExpand(result.id)}
                          className="p-4 flex items-center justify-between cursor-pointer"
                          style={{ borderBottom: isExpanded ? '1px solid #343A40' : 'none' }}
                        >
                          <div className="flex items-center gap-4">
                            {isExpanded ? (
                              <ChevronDown size={16} style={{ color: '#00F0FF' }} />
                            ) : (
                              <ChevronRight size={16} style={{ color: '#343A40' }} />
                            )}

                            <div
                              style={{
                                width: '8px',
                                height: '8px',
                                borderRadius: '50%',
                                background: getStatusColor(result.status),
                              }}
                            />

                            <div>
                              <div style={{ fontSize: '11px', color: '#343A40' }}>{result.timestamp}</div>
                              <div style={{ fontSize: '13px', color: '#E9ECEF', marginTop: '2px' }}>
                                {result.model} · {result.layer}
                              </div>
                            </div>
                          </div>

                          <div className="flex gap-6" style={{ fontSize: '11px' }}>
                            <div>
                              <span style={{ color: '#343A40' }}>TOKENS: </span>
                              <span style={{ color: '#00F0FF', fontWeight: 600 }}>{result.tokens}</span>
                            </div>
                            <div>
                              <span style={{ color: '#343A40' }}>TTFT: </span>
                              <span
                                style={{
                                  color: result.ttft > 300 ? '#FFB300' : '#E9ECEF',
                                  fontWeight: 600,
                                }}
                              >
                                {result.ttft}ms
                              </span>
                            </div>
                            <div>
                              <span style={{ color: '#343A40' }}>LATENCY: </span>
                              <span
                                style={{
                                  color: result.latency > 1500 ? '#FF2A00' : '#E9ECEF',
                                  fontWeight: 600,
                                }}
                              >
                                {result.latency}ms
                              </span>
                            </div>
                          </div>
                        </div>

                        {/* Expanded Payload */}
                        {isExpanded && (
                          <div className="p-4" style={{ fontSize: '11px' }}>
                            <div
                              style={{
                                color: '#343A40',
                                fontSize: '10px',
                                marginBottom: '12px',
                                letterSpacing: '1px',
                              }}
                            >
                              FULL PAYLOAD
                            </div>
                            {renderJsonTree(result.payload)}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Right Analytics Panel */}
      <div className="w-[380px] flex flex-col" style={{ borderLeft: '1px solid #343A40', background: '#111216' }}>
        <div className="px-4 py-3" style={{ borderBottom: '1px solid #343A40' }}>
          <h2
            style={{
              fontFamily: 'var(--font-display)',
              fontSize: '14px',
              fontWeight: 600,
              color: '#E9ECEF',
              letterSpacing: '1px',
            }}
          >
            ANALYTICS
          </h2>
        </div>

        <div className="flex-1 overflow-auto p-4">
          <div className="mb-6">
            <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '8px', letterSpacing: '1px' }}>
              LATENCY DISTRIBUTION
            </div>
            <div style={{ height: '180px', background: '#050506', border: '1px solid #343A40', padding: '12px' }}>
              <ReactECharts option={latencyDistributionOption} style={{ height: '100%', width: '100%' }} />
            </div>
          </div>

          <div className="space-y-3">
            <div style={{ background: '#050506', border: '1px solid #343A40', padding: '12px' }}>
              <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>TOTAL QUERIES</div>
              <div style={{ fontSize: '24px', color: '#00F0FF', fontWeight: 700, marginTop: '6px' }}>947</div>
            </div>

            <div style={{ background: '#050506', border: '1px solid #00F0FF', padding: '12px' }}>
              <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>SUCCESS RATE</div>
              <div style={{ fontSize: '24px', color: '#00F0FF', fontWeight: 700, marginTop: '6px' }}>94.2%</div>
            </div>

            <div style={{ background: '#050506', border: '1px solid #FFB300', padding: '12px' }}>
              <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>AVG LATENCY</div>
              <div style={{ fontSize: '24px', color: '#FFB300', fontWeight: 700, marginTop: '6px' }}>842ms</div>
            </div>

            <div style={{ background: '#050506', border: '1px solid #FF2A00', padding: '12px' }}>
              <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>ERROR RATE</div>
              <div style={{ fontSize: '24px', color: '#FF2A00', fontWeight: 700, marginTop: '6px' }}>5.8%</div>
            </div>
          </div>

          <div className="mt-6 pt-4" style={{ borderTop: '1px solid #343A40' }}>
            <div style={{ fontSize: '10px', color: '#343A40', marginBottom: '8px', letterSpacing: '1px' }}>
              TOP MODELS
            </div>
            <div className="space-y-2">
              {[
                { model: 'gpt-4-turbo', count: 547, color: '#00F0FF' },
                { model: 'claude-3-opus', count: 289, color: '#8B5CF6' },
                { model: 'llama-3-70b', count: 111, color: '#FFB300' },
              ].map((item) => (
                <div
                  key={item.model}
                  style={{
                    padding: '8px',
                    background: '#050506',
                    border: '1px solid #343A40',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                  }}
                >
                  <div style={{ fontSize: '11px', color: '#E9ECEF' }}>{item.model}</div>
                  <div style={{ fontSize: '11px', color: item.color, fontWeight: 600 }}>{item.count}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
