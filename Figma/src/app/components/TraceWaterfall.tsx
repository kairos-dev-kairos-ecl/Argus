import { useState } from "react";

interface Span {
  id: string;
  name: string;
  model?: string;
  start: number;
  duration: number;
  type: "llm" | "vector" | "api" | "tool";
  tokens?: number;
  ttft?: number;
  gpuMemory?: string;
}

const mockSpans: Span[] = [
  { id: "1", name: "user_query", type: "api", start: 0, duration: 5 },
  { id: "2", name: "vector_search", type: "vector", start: 5, duration: 45, tokens: 128 },
  { id: "3", name: "gpt-4-turbo", model: "gpt-4-turbo", type: "llm", start: 50, duration: 1200, tokens: 2456, ttft: 234, gpuMemory: "8.2GB" },
  { id: "4", name: "tool_execution", type: "tool", start: 1250, duration: 300 },
  { id: "5", name: "gpt-4-turbo", model: "gpt-4-turbo", type: "llm", start: 1550, duration: 890, tokens: 1234, ttft: 189, gpuMemory: "8.4GB" },
  { id: "6", name: "response_format", type: "api", start: 2440, duration: 10 },
];

const mockPayload = {
  prompt: "Analyze the recent security incidents in the production cluster and provide a detailed breakdown of GPU utilization patterns during the last 24 hours.",
  response: "Based on the telemetry data, I've identified 3 critical security incidents:\n\n1. CUDA Out-of-Memory error on node-07 at 08:42 UTC\n2. Abnormal token generation spike (4.2x baseline) at 14:15 UTC\n3. Unauthorized API access attempt from 10.0.2.45\n\nGPU Utilization Analysis:\n- Average: 76.4%\n- Peak: 94.2% (node-03, 16:30 UTC)\n- Thermal throttling events: 2",
  metadata: {
    total_tokens: 2456,
    completion_tokens: 1824,
    prompt_tokens: 632,
    ttft_ms: 234,
    generation_latency_ms: 1200
  }
};

export function TraceWaterfall() {
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(mockSpans[2]);
  const [hoveredSpan, setHoveredSpan] = useState<Span | null>(null);
  
  const maxTime = Math.max(...mockSpans.map(s => s.start + s.duration));
  
  const getSpanColor = (type: string) => {
    switch (type) {
      case "llm": return "#00F0FF";
      case "vector": return "#FFB300";
      case "api": return "#E9ECEF";
      case "tool": return "#FF2A00";
      default: return "#343A40";
    }
  };
  
  return (
    <div className="h-full flex flex-col" style={{ 
      background: 'var(--color-background)',
      fontFamily: 'var(--font-primary)'
    }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: 'var(--border-stark)' }}>
        <h1 style={{
          fontFamily: 'var(--font-display)',
          fontSize: '20px',
          fontWeight: 700,
          color: 'var(--color-text)',
          letterSpacing: '1px'
        }}>
          AGENT & INFERENCE TRACE WATERFALL
        </h1>
        <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginTop: '4px' }}>
          trace_id: d4f2b8a9-3c5e-4d2a-8b1f-9e7c6d5a4b3c
        </p>
      </div>
      
      <div className="flex-1 flex" style={{ minHeight: 0 }}>
        {/* Payload Viewer - Top Pane (50%) */}
        <div className="w-1/2 flex flex-col" style={{ borderRight: 'var(--border-stark)' }}>
          <div className="p-4" style={{ borderBottom: 'var(--border-stark)' }}>
            <h2 style={{
              fontFamily: 'var(--font-display)',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text)',
              letterSpacing: '1px'
            }}>
              PAYLOAD VIEWER
            </h2>
          </div>
          
          <div className="flex-1 overflow-auto p-4" style={{ background: '#050506' }}>
            {selectedSpan && selectedSpan.type === "llm" ? (
              <div style={{ fontSize: '11px' }}>
                <div className="mb-4">
                  <div style={{ 
                    color: 'var(--color-muted)', 
                    fontSize: '10px', 
                    marginBottom: '8px',
                    letterSpacing: '1px'
                  }}>
                    PROMPT [{mockPayload.metadata.prompt_tokens} tokens]
                  </div>
                  <div style={{ 
                    color: 'var(--color-text)',
                    lineHeight: '1.6',
                    padding: '12px',
                    background: 'var(--color-surface)',
                    border: '1px solid var(--color-muted)'
                  }}>
                    {mockPayload.prompt}
                  </div>
                </div>
                
                <div className="mb-4">
                  <div style={{ 
                    color: 'var(--color-muted)', 
                    fontSize: '10px', 
                    marginBottom: '8px',
                    letterSpacing: '1px'
                  }}>
                    RESPONSE [{mockPayload.metadata.completion_tokens} tokens]
                  </div>
                  <div style={{ 
                    color: 'var(--color-text)',
                    lineHeight: '1.6',
                    padding: '12px',
                    background: 'var(--color-surface)',
                    border: '1px solid var(--color-muted)',
                    whiteSpace: 'pre-wrap'
                  }}>
                    {mockPayload.response}
                  </div>
                </div>
                
                <div>
                  <div style={{ 
                    color: 'var(--color-muted)', 
                    fontSize: '10px', 
                    marginBottom: '8px',
                    letterSpacing: '1px'
                  }}>
                    METADATA
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="p-3" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-muted)' }}>
                      <div style={{ color: 'var(--color-muted)', fontSize: '10px' }}>TOTAL TOKENS</div>
                      <div style={{ color: 'var(--color-primary)', fontSize: '16px', fontWeight: 600, marginTop: '4px' }}>
                        {mockPayload.metadata.total_tokens}
                      </div>
                    </div>
                    <div className="p-3" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-muted)' }}>
                      <div style={{ color: 'var(--color-muted)', fontSize: '10px' }}>TTFT</div>
                      <div style={{ color: 'var(--color-primary)', fontSize: '16px', fontWeight: 600, marginTop: '4px' }}>
                        {mockPayload.metadata.ttft_ms}ms
                      </div>
                    </div>
                    <div className="p-3" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-muted)' }}>
                      <div style={{ color: 'var(--color-muted)', fontSize: '10px' }}>LATENCY</div>
                      <div style={{ color: 'var(--color-warning)', fontSize: '16px', fontWeight: 600, marginTop: '4px' }}>
                        {mockPayload.metadata.generation_latency_ms}ms
                      </div>
                    </div>
                    <div className="p-3" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-muted)' }}>
                      <div style={{ color: 'var(--color-muted)', fontSize: '10px' }}>GPU MEMORY</div>
                      <div style={{ color: 'var(--color-text)', fontSize: '16px', fontWeight: 600, marginTop: '4px' }}>
                        {selectedSpan.gpuMemory}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            ) : (
              <div style={{ 
                color: 'var(--color-muted)', 
                fontSize: '11px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%'
              }}>
                Select an LLM span to view payload details
              </div>
            )}
          </div>
        </div>
        
        {/* Trace Waterfall - Bottom Pane (50%) */}
        <div className="w-1/2 flex flex-col">
          <div className="p-4" style={{ borderBottom: 'var(--border-stark)' }}>
            <h2 style={{
              fontFamily: 'var(--font-display)',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text)',
              letterSpacing: '1px'
            }}>
              EXECUTION TIMELINE
            </h2>
          </div>
          
          <div className="flex-1 overflow-auto p-4">
            {/* Timeline */}
            <div className="mb-4 flex items-center" style={{ fontSize: '10px', color: 'var(--color-muted)' }}>
              <div style={{ width: '200px' }}>SPAN</div>
              <div className="flex-1 flex justify-between px-2">
                <span>0ms</span>
                <span>500ms</span>
                <span>1000ms</span>
                <span>1500ms</span>
                <span>2000ms</span>
                <span>2500ms</span>
              </div>
            </div>
            
            {/* Spans */}
            <div className="space-y-2">
              {mockSpans.map((span) => (
                <div 
                  key={span.id}
                  className="flex items-center group"
                  onMouseEnter={() => setHoveredSpan(span)}
                  onMouseLeave={() => setHoveredSpan(null)}
                  onClick={() => setSelectedSpan(span)}
                  style={{ cursor: 'pointer' }}
                >
                  {/* Span Name */}
                  <div style={{ 
                    width: '200px', 
                    fontSize: '11px',
                    color: selectedSpan?.id === span.id ? 'var(--color-primary)' : 'var(--color-text)',
                    paddingRight: '8px',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap'
                  }}>
                    {span.name}
                  </div>
                  
                  {/* Span Bar */}
                  <div className="flex-1 relative" style={{ height: '20px' }}>
                    <div
                      style={{
                        position: 'absolute',
                        left: `${(span.start / maxTime) * 100}%`,
                        width: `${(span.duration / maxTime) * 100}%`,
                        height: '16px',
                        background: getSpanColor(span.type),
                        opacity: selectedSpan?.id === span.id ? 1 : 0.7,
                        borderLeft: '2px solid white',
                        display: 'flex',
                        alignItems: 'center',
                        paddingLeft: '4px',
                        fontSize: '10px',
                        color: span.type === 'api' ? '#050506' : '#050506',
                        fontWeight: 600
                      }}
                    >
                      {span.model || span.name}
                    </div>
                    
                    {/* Hover Tooltip */}
                    {hoveredSpan?.id === span.id && (
                      <div
                        style={{
                          position: 'absolute',
                          left: `${(span.start / maxTime) * 100}%`,
                          top: '-80px',
                          width: '200px',
                          background: 'var(--color-surface)',
                          border: '1px solid var(--color-primary)',
                          padding: '8px',
                          fontSize: '10px',
                          zIndex: 10,
                          boxShadow: '0 0 12px rgba(0, 240, 255, 0.3)'
                        }}
                      >
                        <div style={{ color: 'var(--color-muted)' }}>DURATION</div>
                        <div style={{ color: 'var(--color-primary)', fontWeight: 600 }}>{span.duration}ms</div>
                        {span.tokens && (
                          <>
                            <div style={{ color: 'var(--color-muted)', marginTop: '4px' }}>TOKENS</div>
                            <div style={{ color: 'var(--color-text)' }}>{span.tokens}</div>
                          </>
                        )}
                        {span.ttft && (
                          <>
                            <div style={{ color: 'var(--color-muted)', marginTop: '4px' }}>TTFT</div>
                            <div style={{ color: 'var(--color-text)' }}>{span.ttft}ms</div>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                  
                  {/* Duration Label */}
                  <div style={{ 
                    width: '80px', 
                    textAlign: 'right',
                    fontSize: '11px',
                    color: 'var(--color-muted)',
                    paddingLeft: '8px'
                  }}>
                    {span.duration}ms
                  </div>
                </div>
              ))}
            </div>
            
            {/* Legend */}
            <div className="mt-6 pt-4" style={{ borderTop: 'var(--border-stark)' }}>
              <div style={{ fontSize: '10px', color: 'var(--color-muted)', marginBottom: '8px' }}>SPAN TYPES</div>
              <div className="flex gap-4">
                {[
                  { type: "llm", label: "LLM Inference" },
                  { type: "vector", label: "Vector Search" },
                  { type: "api", label: "API Call" },
                  { type: "tool", label: "Tool Execution" }
                ].map(({ type, label }) => (
                  <div key={type} className="flex items-center gap-2">
                    <div style={{ 
                      width: '12px', 
                      height: '12px', 
                      background: getSpanColor(type),
                      border: '1px solid white'
                    }} />
                    <span style={{ fontSize: '11px', color: 'var(--color-text)' }}>{label}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
