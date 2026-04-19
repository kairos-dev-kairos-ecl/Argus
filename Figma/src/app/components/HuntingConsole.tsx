import { useState } from "react";
import { Play, ChevronRight, ChevronDown } from "lucide-react";

interface QueryResult {
  id: string;
  timestamp: string;
  model: string;
  tokens: number;
  ttft: number;
  latency: number;
  layer: string;
  payload: any;
}

const mockResults: QueryResult[] = [
  {
    id: "1",
    timestamp: "2026-04-16T14:42:18.234Z",
    model: "gpt-4-turbo",
    tokens: 2456,
    ttft: 234,
    latency: 1200,
    layer: "L5",
    payload: {
      request_id: "req_d4f2b8a9",
      node: "cluster-01-gpu-07",
      gpu_util: 89.4,
      vram_allocated: "8.2GB",
      prompt_tokens: 632,
      completion_tokens: 1824,
      temperature: 0.7,
      error: null
    }
  },
  {
    id: "2",
    timestamp: "2026-04-16T14:41:52.891Z",
    model: "gpt-4-turbo",
    tokens: 1834,
    ttft: 189,
    latency: 890,
    layer: "L5",
    payload: {
      request_id: "req_a3e1c7b5",
      node: "cluster-01-gpu-04",
      gpu_util: 76.2,
      vram_allocated: "7.8GB",
      prompt_tokens: 512,
      completion_tokens: 1322,
      temperature: 0.7,
      error: null
    }
  }
];

const querySnippets = [
  { label: "High TTFT", query: 'model = "gpt-4-turbo" AND ttft > 200' },
  { label: "Token Spike", query: 'tokens > 2000 AND layer = "L5"' },
  { label: "GPU OOM", query: 'layer = "L1" AND error CONTAINS "CUDA_OUT_OF_MEMORY"' },
  { label: "Slow RAG", query: 'layer = "L7" AND latency > 500' }
];

export function HuntingConsole() {
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
                <span style={{ color: 'var(--color-muted)' }}>{key}:</span>
                {renderJsonTree(value, depth + 1)}
              </div>
            ) : (
              <div>
                <span style={{ color: 'var(--color-muted)' }}>{key}: </span>
                <span style={{ 
                  color: typeof value === 'number' ? 'var(--color-primary)' : 
                        typeof value === 'string' ? 'var(--color-warning)' : 
                        value === null ? 'var(--color-muted)' : 'var(--color-text)'
                }}>
                  {typeof value === 'string' ? `"${value}"` : String(value)}
                </span>
              </div>
            )}
          </div>
        ))}
      </div>
    );
  };
  
  return (
    <div className="h-full flex flex-col" style={{ 
      background: 'var(--color-background)',
      fontFamily: 'var(--font-primary)'
    }}>
      {/* Header */}
      <div className="px-6 py-4 flex items-center justify-between" style={{ borderBottom: 'var(--border-stark)' }}>
        <div>
          <h1 style={{
            fontFamily: 'var(--font-display)',
            fontSize: '20px',
            fontWeight: 700,
            color: 'var(--color-text)',
            letterSpacing: '1px'
          }}>
            AI TELEMETRY & HUNTING CONSOLE
          </h1>
          <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginTop: '4px' }}>
            Query AI/ML telemetry, logs, and payload data across all observability layers
          </p>
        </div>
      </div>
      
      {/* Query Snippets */}
      <div className="px-6 py-3 flex items-center gap-2" style={{ borderBottom: 'var(--border-stark)', background: 'var(--color-surface)' }}>
        <div style={{ fontSize: '10px', color: 'var(--color-muted)', letterSpacing: '1px', marginRight: '8px' }}>
          QUICK QUERIES:
        </div>
        {querySnippets.map((snippet) => (
          <button
            key={snippet.label}
            onClick={() => insertSnippet(snippet.query)}
            className="px-3 py-1"
            style={{
              background: 'var(--color-surface)',
              border: '1px solid var(--color-muted)',
              color: 'var(--color-text)',
              fontSize: '11px',
              cursor: 'pointer'
            }}
          >
            {snippet.label}
          </button>
        ))}
      </div>
      
      <div className="flex-1 flex flex-col" style={{ minHeight: 0 }}>
        {/* Query Editor - 30% */}
        <div className="flex flex-col" style={{ height: '30%', borderBottom: 'var(--border-stark)' }}>
          <div className="px-6 py-3 flex items-center justify-between" style={{ borderBottom: 'var(--border-stark)' }}>
            <h2 style={{
              fontFamily: 'var(--font-display)',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text)',
              letterSpacing: '1px'
            }}>
              QUERY EDITOR
            </h2>
            
            <button
              onClick={handleExecute}
              disabled={isExecuting}
              className="px-6 py-2 flex items-center gap-2"
              style={{
                background: isExecuting ? 'transparent' : 'var(--color-primary)',
                border: '1px solid var(--color-primary)',
                color: isExecuting ? 'var(--color-primary)' : '#050506',
                fontFamily: 'var(--font-display)',
                fontSize: '12px',
                fontWeight: 600,
                letterSpacing: '1px',
                cursor: isExecuting ? 'default' : 'pointer',
                opacity: isExecuting ? 0.6 : 1
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
                border: '1px solid var(--color-muted)',
                color: 'var(--color-text)',
                fontSize: '13px',
                padding: '16px',
                outline: 'none',
                fontFamily: 'var(--font-primary)'
              }}
            />
          </div>
        </div>
        
        {/* Results Pane - 70% */}
        <div className="flex-1 flex flex-col" style={{ minHeight: 0 }}>
          <div className="px-6 py-3" style={{ borderBottom: 'var(--border-stark)' }}>
            <h2 style={{
              fontFamily: 'var(--font-display)',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text)',
              letterSpacing: '1px'
            }}>
              RESULTS {results && `(${results.length})`}
            </h2>
          </div>
          
          <div className="flex-1 overflow-auto p-6">
            {!results ? (
              <div style={{ 
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%',
                color: 'var(--color-muted)',
                fontSize: '11px'
              }}>
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
                        background: '#050506',
                        border: '1px solid var(--color-muted)'
                      }}
                    >
                      {/* Result Header */}
                      <div
                        onClick={() => toggleExpand(result.id)}
                        className="p-4 flex items-center justify-between cursor-pointer"
                        style={{ borderBottom: isExpanded ? 'var(--border-stark)' : 'none' }}
                      >
                        <div className="flex items-center gap-4">
                          {isExpanded ? (
                            <ChevronDown size={16} style={{ color: 'var(--color-primary)' }} />
                          ) : (
                            <ChevronRight size={16} style={{ color: 'var(--color-muted)' }} />
                          )}
                          
                          <div>
                            <div style={{ fontSize: '11px', color: 'var(--color-muted)' }}>
                              {result.timestamp}
                            </div>
                            <div style={{ fontSize: '13px', color: 'var(--color-text)', marginTop: '2px' }}>
                              {result.model} · {result.layer}
                            </div>
                          </div>
                        </div>
                        
                        <div className="flex gap-6" style={{ fontSize: '11px' }}>
                          <div>
                            <span style={{ color: 'var(--color-muted)' }}>TOKENS: </span>
                            <span style={{ color: 'var(--color-primary)' }}>{result.tokens}</span>
                          </div>
                          <div>
                            <span style={{ color: 'var(--color-muted)' }}>TTFT: </span>
                            <span style={{ color: 'var(--color-warning)' }}>{result.ttft}ms</span>
                          </div>
                          <div>
                            <span style={{ color: 'var(--color-muted)' }}>LATENCY: </span>
                            <span style={{ color: 'var(--color-text)' }}>{result.latency}ms</span>
                          </div>
                        </div>
                      </div>
                      
                      {/* Expanded Payload */}
                      {isExpanded && (
                        <div className="p-4" style={{ fontSize: '11px' }}>
                          <div style={{ 
                            color: 'var(--color-muted)', 
                            fontSize: '10px', 
                            marginBottom: '12px',
                            letterSpacing: '1px'
                          }}>
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
  );
}
