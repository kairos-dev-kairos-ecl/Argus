import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

interface AuditEvent {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  targetLayer: string;
  diff: any;
  hash: string;
}

const mockAuditEvents: AuditEvent[] = [
  {
    id: "1",
    timestamp: "2026-04-16 14:42:18.234 UTC",
    actor: "admin@argusxdr.ai",
    action: "MODEL_DEPLOYMENT",
    targetLayer: "L5",
    hash: "a7f3c2d9",
    diff: {
      before: { model: "gpt-4", version: "0613" },
      after: { model: "gpt-4-turbo", version: "2024-04-09" }
    }
  },
  {
    id: "2",
    timestamp: "2026-04-16 14:38:52.891 UTC",
    actor: "system.scheduler",
    action: "TOKEN_ROTATION",
    targetLayer: "L9",
    hash: "b9e5d1a3",
    diff: {
      before: { token_id: "tok_abc123", expires: "2026-04-16" },
      after: { token_id: "tok_xyz789", expires: "2026-05-16" }
    }
  },
  {
    id: "3",
    timestamp: "2026-04-16 14:35:23.567 UTC",
    actor: "mlops@argusxdr.ai",
    action: "VECTOR_INDEX_UPDATE",
    targetLayer: "L6",
    hash: "c1a8f4b7",
    diff: {
      before: { index_size: 1024000, dimensions: 1536 },
      after: { index_size: 2048000, dimensions: 1536 }
    }
  },
  {
    id: "4",
    timestamp: "2026-04-16 14:30:45.123 UTC",
    actor: "security@argusxdr.ai",
    action: "GUARDRAIL_CONFIG",
    targetLayer: "L8",
    hash: "d2b9e6c4",
    diff: {
      before: { pii_detection: "enabled", redaction_latency_ms: 85 },
      after: { pii_detection: "enabled", redaction_latency_ms: 120 }
    }
  },
  {
    id: "5",
    timestamp: "2026-04-16 14:25:12.789 UTC",
    actor: "admin@argusxdr.ai",
    action: "GPU_ALLOCATION",
    targetLayer: "L1",
    hash: "e3c7a5d8",
    diff: {
      before: { allocated_gpus: 8, vram_total: "64GB" },
      after: { allocated_gpus: 12, vram_total: "96GB" }
    }
  },
  {
    id: "6",
    timestamp: "2026-04-16 14:20:38.456 UTC",
    actor: "system.orchestrator",
    action: "CONTAINER_RESCHEDULE",
    targetLayer: "L3",
    hash: "f4d8b9a2",
    diff: {
      before: { node: "cluster-01-node-04", replicas: 3 },
      after: { node: "cluster-01-node-07", replicas: 5 }
    }
  }
];

export function AuditLedger() {
  const [expandedEvent, setExpandedEvent] = useState<string | null>(null);
  const [filterActor, setFilterActor] = useState("all");
  const [filterLayer, setFilterLayer] = useState("all");
  
  const toggleExpand = (id: string) => {
    setExpandedEvent(expandedEvent === id ? null : id);
  };
  
  const renderDiff = (diff: any) => {
    return (
      <div className="grid grid-cols-2 gap-4">
        <div>
          <div style={{ 
            fontSize: '10px', 
            color: 'var(--color-muted)', 
            marginBottom: '8px',
            letterSpacing: '1px'
          }}>
            BEFORE
          </div>
          <div className="p-3" style={{ 
            background: 'rgba(255, 42, 0, 0.05)',
            border: '1px solid rgba(255, 42, 0, 0.3)',
            fontSize: '11px'
          }}>
            {Object.entries(diff.before).map(([key, value]) => (
              <div key={key} style={{ marginBottom: '4px' }}>
                <span style={{ color: 'var(--color-muted)' }}>{key}: </span>
                <span style={{ color: 'var(--color-text)' }}>
                  {typeof value === 'string' ? `"${value}"` : String(value)}
                </span>
              </div>
            ))}
          </div>
        </div>
        
        <div>
          <div style={{ 
            fontSize: '10px', 
            color: 'var(--color-muted)', 
            marginBottom: '8px',
            letterSpacing: '1px'
          }}>
            AFTER
          </div>
          <div className="p-3" style={{ 
            background: 'rgba(0, 240, 255, 0.05)',
            border: '1px solid rgba(0, 240, 255, 0.3)',
            fontSize: '11px'
          }}>
            {Object.entries(diff.after).map(([key, value]) => (
              <div key={key} style={{ marginBottom: '4px' }}>
                <span style={{ color: 'var(--color-muted)' }}>{key}: </span>
                <span style={{ color: 'var(--color-text)' }}>
                  {typeof value === 'string' ? `"${value}"` : String(value)}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    );
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
          TRACEABILITY & ZERO-TRUST AUDIT LEDGER
        </h1>
        <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginTop: '4px' }}>
          Immutable cryptographic audit trail of all system actions and architectural modifications
        </p>
      </div>
      
      {/* Filters */}
      <div className="px-6 py-3 flex items-center gap-4" style={{ 
        borderBottom: 'var(--border-stark)',
        background: 'var(--color-surface)'
      }}>
        <div style={{ fontSize: '10px', color: 'var(--color-muted)', letterSpacing: '1px' }}>
          FILTERS:
        </div>
        
        <div>
          <select
            value={filterActor}
            onChange={(e) => setFilterActor(e.target.value)}
            style={{
              background: '#050506',
              border: '1px solid var(--color-muted)',
              color: 'var(--color-text)',
              fontSize: '11px',
              padding: '6px 12px',
              outline: 'none'
            }}
          >
            <option value="all">All Actors</option>
            <option value="admin">Admin Users</option>
            <option value="system">System Processes</option>
            <option value="mlops">MLOps Team</option>
          </select>
        </div>
        
        <div>
          <select
            value={filterLayer}
            onChange={(e) => setFilterLayer(e.target.value)}
            style={{
              background: '#050506',
              border: '1px solid var(--color-muted)',
              color: 'var(--color-text)',
              fontSize: '11px',
              padding: '6px 12px',
              outline: 'none'
            }}
          >
            <option value="all">All Layers</option>
            <option value="L1">L1 - Hardware/GPU</option>
            <option value="L5">L5 - Model/Inference</option>
            <option value="L6">L6 - Vector Storage</option>
            <option value="L8">L8 - Guardrails</option>
            <option value="L9">L9 - Logic/API</option>
          </select>
        </div>
        
        <div className="ml-auto flex items-center gap-2">
          <div style={{ fontSize: '11px', color: 'var(--color-muted)' }}>
            Total Events:
          </div>
          <div style={{ fontSize: '14px', color: 'var(--color-primary)', fontWeight: 600 }}>
            {mockAuditEvents.length}
          </div>
        </div>
      </div>
      
      {/* Audit Grid */}
      <div className="flex-1 overflow-auto">
        {/* Table Header */}
        <div className="sticky top-0 flex" style={{ 
          background: 'var(--color-surface)',
          borderBottom: 'var(--border-stark)',
          fontSize: '10px',
          color: 'var(--color-muted)',
          padding: '12px 0',
          letterSpacing: '1px'
        }}>
          <div style={{ width: '40px', paddingLeft: '24px' }}></div>
          <div style={{ width: '180px' }}>TIMESTAMP</div>
          <div style={{ width: '220px' }}>ACTOR/AGENT</div>
          <div style={{ width: '200px' }}>ACTION</div>
          <div style={{ width: '100px' }}>TARGET LAYER</div>
          <div className="flex-1">DIFF/PAYLOAD</div>
          <div style={{ width: '120px', paddingRight: '24px' }}>HASH</div>
        </div>
        
        {/* Table Rows */}
        <div>
          {mockAuditEvents.map((event, idx) => {
            const isExpanded = expandedEvent === event.id;
            
            return (
              <div key={event.id}>
                {/* Row Header */}
                <div
                  onClick={() => toggleExpand(event.id)}
                  className="flex cursor-pointer transition-colors"
                  style={{
                    background: idx % 2 === 0 ? '#050506' : '#0A0A0C',
                    borderBottom: isExpanded ? 'none' : '1px solid #0A0A0C',
                    fontSize: '11px',
                    padding: '12px 0',
                    color: 'var(--color-text)'
                  }}
                  onMouseEnter={(e) => e.currentTarget.style.background = '#1A1D24'}
                  onMouseLeave={(e) => e.currentTarget.style.background = idx % 2 === 0 ? '#050506' : '#0A0A0C'}
                >
                  <div style={{ width: '40px', paddingLeft: '24px' }}>
                    {isExpanded ? (
                      <ChevronDown size={14} style={{ color: 'var(--color-primary)' }} />
                    ) : (
                      <ChevronRight size={14} style={{ color: 'var(--color-muted)' }} />
                    )}
                  </div>
                  <div style={{ width: '180px', color: 'var(--color-muted)' }}>
                    {event.timestamp}
                  </div>
                  <div style={{ width: '220px' }}>
                    {event.actor}
                  </div>
                  <div style={{ width: '200px', color: 'var(--color-primary)' }}>
                    {event.action}
                  </div>
                  <div style={{ width: '100px' }}>
                    {event.targetLayer}
                  </div>
                  <div className="flex-1" style={{ color: 'var(--color-muted)' }}>
                    {isExpanded ? "See full diff below" : "Click to expand"}
                  </div>
                  <div 
                    className="group/hash"
                    style={{ 
                      width: '120px', 
                      paddingRight: '24px',
                      position: 'relative'
                    }}
                  >
                    <div className="inline-block px-2 py-1" style={{
                      background: 'var(--color-surface)',
                      border: '1px solid var(--color-muted)',
                      color: 'var(--color-text)',
                      fontSize: '10px',
                      fontFamily: 'var(--font-primary)'
                    }}>
                      {event.hash}
                    </div>
                    
                    {/* Full Hash Tooltip */}
                    <div 
                      className="hidden group-hover/hash:block absolute z-10"
                      style={{
                        top: '-40px',
                        right: '24px',
                        background: 'var(--color-surface)',
                        border: '1px solid var(--color-primary)',
                        padding: '8px',
                        fontSize: '10px',
                        whiteSpace: 'nowrap',
                        boxShadow: '0 0 12px rgba(0, 240, 255, 0.3)'
                      }}
                    >
                      <div style={{ color: 'var(--color-muted)' }}>FULL SHA-256</div>
                      <div style={{ color: 'var(--color-primary)' }}>
                        {event.hash}d1f7a4c9b2e5f8a3d6c9e2b5a8f1d4c7
                      </div>
                    </div>
                  </div>
                </div>
                
                {/* Expanded Diff */}
                {isExpanded && (
                  <div 
                    className="p-6"
                    style={{
                      background: '#0A0A0C',
                      borderBottom: '1px solid #0A0A0C'
                    }}
                  >
                    {renderDiff(event.diff)}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
      
      {/* Status Bar */}
      <div className="px-6 py-3 flex items-center justify-between" style={{ 
        borderTop: 'var(--border-stark)',
        background: 'var(--color-surface)'
      }}>
        <div style={{ fontSize: '11px', color: 'var(--color-muted)' }}>
          Showing 1-{mockAuditEvents.length} of {mockAuditEvents.length} events
        </div>
        
        <div className="flex gap-2">
          <button
            className="px-4 py-2"
            style={{
              background: 'transparent',
              border: '1px solid var(--color-muted)',
              color: 'var(--color-text)',
              fontSize: '11px',
              cursor: 'pointer'
            }}
          >
            PREVIOUS
          </button>
          <button
            className="px-4 py-2"
            style={{
              background: 'transparent',
              border: '1px solid var(--color-muted)',
              color: 'var(--color-text)',
              fontSize: '11px',
              cursor: 'pointer'
            }}
          >
            NEXT
          </button>
        </div>
      </div>
    </div>
  );
}
