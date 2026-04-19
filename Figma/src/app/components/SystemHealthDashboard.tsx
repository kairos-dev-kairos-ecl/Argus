import { useState } from "react";
import { AlertTriangle, TrendingUp, Activity } from "lucide-react";

interface TelemetryEvent {
  id: string;
  timestamp: string;
  layer: string;
  severity: "critical" | "warning" | "info";
  message: string;
  tokens?: number;
  latency?: number;
}

interface LayerMetric {
  layer: string;
  name: string;
  status: "critical" | "warning" | "healthy";
  value: string;
  trend: number;
}

const mockTelemetry: TelemetryEvent[] = [
  { id: "1", timestamp: "14:42:18.234", layer: "L5", severity: "critical", message: "TTFT degradation detected: gpt-4-turbo", tokens: 2456, latency: 1842 },
  { id: "2", timestamp: "14:42:15.891", layer: "L1", severity: "warning", message: "GPU thermal throttling: node-07", latency: 245 },
  { id: "3", timestamp: "14:42:12.567", layer: "L7", severity: "info", message: "Multi-agent orchestration complete", tokens: 1234 },
  { id: "4", timestamp: "14:42:09.123", layer: "L3", severity: "info", message: "Container rescheduled: inference-pod-42" },
  { id: "5", timestamp: "14:42:05.789", layer: "L6", severity: "warning", message: "Vector search latency spike: +340ms" },
  { id: "6", timestamp: "14:42:02.456", layer: "L9", severity: "info", message: "API gateway routing updated" },
  { id: "7", timestamp: "14:41:58.123", layer: "L2", severity: "critical", message: "NVLink bandwidth saturation: 94.2%" },
  { id: "8", timestamp: "14:41:54.890", layer: "L8", severity: "warning", message: "PII redaction overhead: +120ms" },
  { id: "9", timestamp: "14:41:51.567", layer: "L4", severity: "info", message: "Batch processing queue cleared" },
  { id: "10", timestamp: "14:41:48.234", layer: "L10", severity: "info", message: "User session completed: 2.4s total" },
];

const layerMetrics: LayerMetric[] = [
  { layer: "L1", name: "Hardware/GPU", status: "warning", value: "89% util", trend: 12 },
  { layer: "L2", name: "Network", status: "critical", value: "94% bw", trend: 24 },
  { layer: "L3", name: "Infrastructure", status: "healthy", value: "nominal", trend: -3 },
  { layer: "L4", name: "Data Pipeline", status: "healthy", value: "2.4k/s", trend: 5 },
  { layer: "L5", name: "Model/Inference", status: "critical", value: "842ms TTFT", trend: 45 },
  { layer: "L6", name: "Vector Storage", status: "warning", value: "340ms", trend: 18 },
  { layer: "L7", name: "Agentic/RAG", status: "healthy", value: "nominal", trend: -2 },
  { layer: "L8", name: "Guardrails", status: "warning", value: "120ms", trend: 8 },
  { layer: "L9", name: "Logic/API", status: "healthy", value: "98.7% up", trend: 0 },
  { layer: "L10", name: "Application", status: "healthy", value: "2.4s avg", trend: -5 },
];

export function SystemHealthDashboard() {
  const [selectedEvent, setSelectedEvent] = useState<TelemetryEvent | null>(null);
  const [timeRange, setTimeRange] = useState("15m");
  
  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case "critical": return "var(--color-accent)";
      case "warning": return "var(--color-warning)";
      default: return "var(--color-text)";
    }
  };
  
  const getStatusColor = (status: string) => {
    switch (status) {
      case "critical": return "var(--color-accent)";
      case "warning": return "var(--color-warning)";
      default: return "var(--color-primary)";
    }
  };
  
  return (
    <div className="h-full flex flex-col" style={{ 
      background: 'var(--color-background)',
      fontFamily: 'var(--font-primary)'
    }}>
      {/* Global Time-Scrubber */}
      <div className="px-6 py-3 flex items-center justify-between" style={{ 
        borderBottom: 'var(--border-stark)',
        background: 'var(--color-surface)'
      }}>
        <div>
          <h1 style={{
            fontFamily: 'var(--font-display)',
            fontSize: '20px',
            fontWeight: 700,
            color: 'var(--color-text)',
            letterSpacing: '1px'
          }}>
            AI SYSTEM HEALTH & LAYERS
          </h1>
        </div>
        
        <div className="flex items-center gap-4">
          <div style={{ fontSize: '11px', color: 'var(--color-muted)' }}>
            <span style={{ color: 'var(--color-text)' }}>2026-04-16</span> 14:27:00 - 14:42:00 UTC
          </div>
          
          <div className="flex gap-1">
            {["5m", "15m", "1h", "6h", "24h"].map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className="px-3 py-1"
                style={{
                  background: timeRange === range ? 'var(--color-primary)' : 'transparent',
                  color: timeRange === range ? '#050506' : 'var(--color-text)',
                  border: '1px solid',
                  borderColor: timeRange === range ? 'var(--color-primary)' : 'var(--color-muted)',
                  fontSize: '11px',
                  cursor: 'pointer',
                  fontWeight: 600
                }}
              >
                {range}
              </button>
            ))}
          </div>
        </div>
      </div>
      
      <div className="flex-1 flex" style={{ minHeight: 0 }}>
        {/* Left Sidebar Filters */}
        <div className="w-60 flex flex-col p-4" style={{ borderRight: 'var(--border-stark)' }}>
          <div className="mb-6">
            <div style={{ 
              fontSize: '10px', 
              color: 'var(--color-muted)', 
              marginBottom: '8px',
              letterSpacing: '1px'
            }}>
              FILTERS
            </div>
            
            <div className="space-y-2">
              <div>
                <label style={{ fontSize: '11px', color: 'var(--color-text)', display: 'block', marginBottom: '4px' }}>
                  Severity
                </label>
                <select 
                  className="w-full px-3 py-2"
                  style={{
                    background: '#050506',
                    border: '1px solid var(--color-muted)',
                    color: 'var(--color-text)',
                    fontSize: '11px',
                    outline: 'none'
                  }}
                >
                  <option>All</option>
                  <option>Critical</option>
                  <option>Warning</option>
                  <option>Info</option>
                </select>
              </div>
              
              <div>
                <label style={{ fontSize: '11px', color: 'var(--color-text)', display: 'block', marginBottom: '4px' }}>
                  Layer
                </label>
                <select 
                  className="w-full px-3 py-2"
                  style={{
                    background: '#050506',
                    border: '1px solid var(--color-muted)',
                    color: 'var(--color-text)',
                    fontSize: '11px',
                    outline: 'none'
                  }}
                >
                  <option>All Layers</option>
                  <option>L1 - Hardware/GPU</option>
                  <option>L5 - Model/Inference</option>
                  <option>L7 - Agentic/RAG</option>
                </select>
              </div>
              
              <div>
                <label style={{ fontSize: '11px', color: 'var(--color-text)', display: 'block', marginBottom: '4px' }}>
                  Model
                </label>
                <select 
                  className="w-full px-3 py-2"
                  style={{
                    background: '#050506',
                    border: '1px solid var(--color-muted)',
                    color: 'var(--color-text)',
                    fontSize: '11px',
                    outline: 'none'
                  }}
                >
                  <option>All Models</option>
                  <option>gpt-4-turbo</option>
                  <option>claude-3-opus</option>
                  <option>llama-3-70b</option>
                </select>
              </div>
            </div>
          </div>
          
          {/* Stats */}
          <div className="pt-4" style={{ borderTop: 'var(--border-stark)' }}>
            <div style={{ fontSize: '10px', color: 'var(--color-muted)', marginBottom: '8px' }}>
              LIVE METRICS
            </div>
            <div className="space-y-2">
              <div className="p-2" style={{ background: '#050506', border: '1px solid var(--color-muted)' }}>
                <div style={{ fontSize: '10px', color: 'var(--color-muted)' }}>Events/sec</div>
                <div style={{ fontSize: '16px', color: 'var(--color-primary)', fontWeight: 600 }}>2.4k</div>
              </div>
              <div className="p-2" style={{ background: '#050506', border: '1px solid var(--color-accent)' }}>
                <div style={{ fontSize: '10px', color: 'var(--color-muted)' }}>Critical</div>
                <div style={{ fontSize: '16px', color: 'var(--color-accent)', fontWeight: 600 }}>2</div>
              </div>
              <div className="p-2" style={{ background: '#050506', border: '1px solid var(--color-warning)' }}>
                <div style={{ fontSize: '10px', color: 'var(--color-muted)' }}>Warnings</div>
                <div style={{ fontSize: '16px', color: 'var(--color-warning)', fontWeight: 600 }}>3</div>
              </div>
            </div>
          </div>
        </div>
        
        {/* Main Content Area */}
        <div className="flex-1 flex" style={{ minHeight: 0 }}>
          {/* Telemetry Stream - 70% */}
          <div className="flex-1 flex flex-col">
            <div className="px-4 py-3" style={{ borderBottom: 'var(--border-stark)' }}>
              <h2 style={{
                fontFamily: 'var(--font-display)',
                fontSize: '14px',
                fontWeight: 600,
                color: 'var(--color-text)',
                letterSpacing: '1px'
              }}>
                TELEMETRY STREAM
              </h2>
            </div>
            
            <div className="flex-1 overflow-auto">
              {/* Table Header */}
              <div className="sticky top-0 flex" style={{ 
                background: 'var(--color-surface)',
                borderBottom: 'var(--border-stark)',
                fontSize: '10px',
                color: 'var(--color-muted)',
                padding: '8px 0'
              }}>
                <div style={{ width: '100px', paddingLeft: '16px' }}>TIME</div>
                <div style={{ width: '60px' }}>LAYER</div>
                <div style={{ width: '80px' }}>SEVERITY</div>
                <div className="flex-1">MESSAGE</div>
                <div style={{ width: '80px', textAlign: 'right', paddingRight: '16px' }}>LATENCY</div>
              </div>
              
              {/* Table Rows */}
              <div>
                {mockTelemetry.map((event, idx) => (
                  <div
                    key={event.id}
                    onClick={() => setSelectedEvent(event)}
                    className="flex transition-colors cursor-crosshair"
                    style={{
                      background: idx % 2 === 0 ? '#050506' : '#0A0A0C',
                      borderBottom: '1px solid #0A0A0C',
                      fontSize: '11px',
                      padding: '8px 0',
                      color: 'var(--color-text)'
                    }}
                    onMouseEnter={(e) => e.currentTarget.style.background = '#1A1D24'}
                    onMouseLeave={(e) => e.currentTarget.style.background = idx % 2 === 0 ? '#050506' : '#0A0A0C'}
                  >
                    <div style={{ width: '100px', paddingLeft: '16px', color: 'var(--color-muted)' }}>
                      {event.timestamp}
                    </div>
                    <div style={{ 
                      width: '60px',
                      color: getStatusColor(layerMetrics.find(l => l.layer === event.layer)?.status || 'healthy')
                    }}>
                      {event.layer}
                    </div>
                    <div style={{ width: '80px' }}>
                      {event.severity === "critical" && (
                        <span className="px-2 py-1" style={{
                          background: 'transparent',
                          color: 'var(--color-accent)',
                          border: '1px solid var(--color-accent)',
                          fontSize: '10px',
                          fontWeight: 600
                        }}>
                          CRITICAL
                        </span>
                      )}
                      {event.severity === "warning" && (
                        <span className="px-2 py-1" style={{
                          background: 'transparent',
                          color: 'var(--color-warning)',
                          border: '1px solid var(--color-warning)',
                          fontSize: '10px',
                          fontWeight: 600
                        }}>
                          WARNING
                        </span>
                      )}
                      {event.severity === "info" && (
                        <span style={{ color: 'var(--color-muted)', fontSize: '10px' }}>INFO</span>
                      )}
                    </div>
                    <div className="flex-1" style={{ color: getSeverityColor(event.severity) }}>
                      {event.message}
                    </div>
                    <div style={{ width: '80px', textAlign: 'right', paddingRight: '16px', color: 'var(--color-muted)' }}>
                      {event.latency ? `${event.latency}ms` : '—'}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
          
          {/* L1-L10 Horizon Grid - 30% */}
          <div className="w-[30%] flex flex-col" style={{ borderLeft: 'var(--border-stark)' }}>
            <div className="px-4 py-3" style={{ borderBottom: 'var(--border-stark)' }}>
              <h2 style={{
                fontFamily: 'var(--font-display)',
                fontSize: '14px',
                fontWeight: 600,
                color: 'var(--color-text)',
                letterSpacing: '1px'
              }}>
                L1-L10 HORIZON
              </h2>
            </div>
            
            <div className="flex-1 overflow-auto p-4">
              <div className="space-y-3">
                {layerMetrics.map((metric) => (
                  <div
                    key={metric.layer}
                    className="p-3"
                    style={{
                      background: '#050506',
                      border: '1px solid',
                      borderColor: metric.status === "healthy" ? 'var(--color-muted)' : getStatusColor(metric.status)
                    }}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div>
                        <div style={{ 
                          fontSize: '10px', 
                          color: 'var(--color-muted)',
                          letterSpacing: '1px'
                        }}>
                          {metric.layer}
                        </div>
                        <div style={{ 
                          fontSize: '11px', 
                          color: 'var(--color-text)',
                          marginTop: '2px'
                        }}>
                          {metric.name}
                        </div>
                      </div>
                      
                      {metric.status !== "healthy" && (
                        metric.status === "critical" ? (
                          <AlertTriangle size={14} style={{ color: 'var(--color-accent)' }} />
                        ) : (
                          <Activity size={14} style={{ color: 'var(--color-warning)' }} />
                        )
                      )}
                    </div>
                    
                    <div className="flex items-end justify-between">
                      <div style={{ 
                        fontSize: '16px', 
                        fontWeight: 600,
                        color: getStatusColor(metric.status)
                      }}>
                        {metric.value}
                      </div>
                      
                      <div className="flex items-center gap-1" style={{
                        fontSize: '10px',
                        color: metric.trend > 0 ? 'var(--color-accent)' : 'var(--color-primary)'
                      }}>
                        <TrendingUp 
                          size={12} 
                          style={{ 
                            transform: metric.trend < 0 ? 'rotate(180deg)' : 'none'
                          }} 
                        />
                        {Math.abs(metric.trend)}%
                      </div>
                    </div>
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
