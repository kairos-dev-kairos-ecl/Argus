/**
 * Enhanced Telemetry Dashboard - Onum/Falcon Style
 *
 * Advanced data flow visualization with Sankey diagrams and time-series charts.
 * Features:
 * - Macro vs. Micro scope toggle (org-wide vs. individual user)
 * - Sankey diagrams with glowing gradients (blue → purple → pink → amber)
 * - Smooth area-fill line charts for EPS, latency, throughput
 * - Real-time metrics with color-coded statuses
 * - Recent trace history with latency indicators
 *
 * @component
 */

import { useState, useMemo } from 'react';
import ReactECharts from 'echarts-for-react';
import { BarChart3, TrendingUp, Users, User } from 'lucide-react';

type ScopeMode = 'org' | 'user';

interface TelemetryMetric {
  timestamp: number;
  eps: number;
  latency: number;
  throughput: number;
  errors: number;
}

const generateMockTelemetry = (count: number): TelemetryMetric[] => {
  const now = Date.now();
  return Array.from({ length: count }, (_, i) => ({
    timestamp: now - (count - i) * 5000,
    eps: 5000 + Math.random() * 1000 - 500,
    latency: 450 + Math.random() * 200,
    throughput: 5 + Math.random() * 2,
    errors: Math.random() * 10,
  }));
};

export function EnhancedTelemetryDashboard() {
  const [scope, setScope] = useState<ScopeMode>('org');
  const [selectedUser, setSelectedUser] = useState('user_d4f2b8a9');

  const telemetryData = useMemo(() => generateMockTelemetry(60), []);

  const sankeyOption = {
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
    series: [
      {
        type: 'sankey',
        layout: 'none',
        emphasis: {
          focus: 'adjacency',
        },
        lineStyle: {
          color: 'gradient',
          curveness: 0.5,
        },
        data: [
          { name: 'Collector 1', itemStyle: { color: '#3B82F6' } },
          { name: 'Collector 2', itemStyle: { color: '#60A5FA' } },
          { name: 'default', itemStyle: { color: '#8B5CF6' } },
          { name: 'cisco.asa', itemStyle: { color: '#A78BFA' } },
          { name: 'Pipelines', itemStyle: { color: '#EC4899' } },
          { name: 'QRadar', itemStyle: { color: '#FBBF24' } },
          { name: 'Data Sinks', itemStyle: { color: '#34D399' } },
        ],
        links: [
          { source: 'Collector 1', target: 'default', value: 470.07 },
          { source: 'Collector 1', target: 'cisco.asa', value: 100.41 },
          { source: 'Collector 2', target: 'default', value: 180.23 },
          { source: 'default', target: 'Pipelines', value: 650.3 },
          { source: 'cisco.asa', target: 'Pipelines', value: 100.41 },
          { source: 'Pipelines', target: 'QRadar', value: 650.71 },
          { source: 'Pipelines', target: 'Data Sinks', value: 100 },
          { source: 'QRadar', target: 'Data Sinks', value: 650.71 },
        ],
        label: {
          color: '#E9ECEF',
          fontSize: 11,
        },
      },
    ],
  };

  const epsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
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
      boundaryGap: false,
      data: telemetryData.map((d) => new Date(d.timestamp).toLocaleTimeString()),
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
        name: 'EPS',
        type: 'line',
        smooth: true,
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(0, 240, 255, 0.4)',
              },
              {
                offset: 1,
                color: 'rgba(0, 240, 255, 0.0)',
              },
            ],
          },
        },
        lineStyle: {
          color: '#00F0FF',
          width: 2,
        },
        itemStyle: {
          color: '#00F0FF',
        },
        data: telemetryData.map((d) => d.eps.toFixed(0)),
      },
    ],
  };

  const latencyOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#111216',
      borderColor: '#FFB300',
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
      boundaryGap: false,
      data: telemetryData.map((d) => new Date(d.timestamp).toLocaleTimeString()),
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
        formatter: '{value}ms',
      },
      splitLine: {
        lineStyle: {
          color: '#1A1D24',
        },
      },
    },
    series: [
      {
        name: 'Latency',
        type: 'line',
        smooth: true,
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(255, 179, 0, 0.4)',
              },
              {
                offset: 1,
                color: 'rgba(255, 179, 0, 0.0)',
              },
            ],
          },
        },
        lineStyle: {
          color: '#FFB300',
          width: 2,
        },
        itemStyle: {
          color: '#FFB300',
        },
        data: telemetryData.map((d) => d.latency.toFixed(0)),
      },
    ],
  };

  const throughputOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#111216',
      borderColor: '#8B5CF6',
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
      boundaryGap: false,
      data: telemetryData.map((d) => new Date(d.timestamp).toLocaleTimeString()),
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
        formatter: '{value} MB/s',
      },
      splitLine: {
        lineStyle: {
          color: '#1A1D24',
        },
      },
    },
    series: [
      {
        name: 'Throughput',
        type: 'line',
        smooth: true,
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(139, 92, 246, 0.4)',
              },
              {
                offset: 1,
                color: 'rgba(139, 92, 246, 0.0)',
              },
            ],
          },
        },
        lineStyle: {
          color: '#8B5CF6',
          width: 2,
        },
        itemStyle: {
          color: '#8B5CF6',
        },
        data: telemetryData.map((d) => d.throughput.toFixed(2)),
      },
    ],
  };

  return (
    <div className="h-full flex flex-col" style={{ background: '#050506', fontFamily: 'var(--font-primary)' }}>
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
            TELEMETRY & SIGNAL ANALYTICS
          </h1>
          <p style={{ fontSize: '11px', color: '#343A40', marginTop: '4px' }}>
            Real-time data flow visualization with Onum/Falcon aesthetic
          </p>
        </div>

        <div className="flex items-center gap-4">
          <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>SCOPE</div>
          <div className="flex gap-1">
            <button
              onClick={() => setScope('org')}
              style={{
                padding: '8px 16px',
                background: scope === 'org' ? '#00F0FF' : 'transparent',
                color: scope === 'org' ? '#050506' : '#E9ECEF',
                border: '1px solid',
                borderColor: scope === 'org' ? '#00F0FF' : '#343A40',
                fontSize: '11px',
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Users size={14} />
              ORG-WIDE
            </button>
            <button
              onClick={() => setScope('user')}
              style={{
                padding: '8px 16px',
                background: scope === 'user' ? '#00F0FF' : 'transparent',
                color: scope === 'user' ? '#050506' : '#E9ECEF',
                border: '1px solid',
                borderColor: scope === 'user' ? '#00F0FF' : '#343A40',
                fontSize: '11px',
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <User size={14} />
              USER
            </button>
          </div>

          {scope === 'user' && (
            <select
              value={selectedUser}
              onChange={(e) => setSelectedUser(e.target.value)}
              style={{
                padding: '8px 12px',
                background: '#111216',
                border: '1px solid #343A40',
                color: '#E9ECEF',
                fontSize: '11px',
                outline: 'none',
              }}
            >
              <option value="user_d4f2b8a9">user_d4f2b8a9</option>
              <option value="user_a3e1c7b5">user_a3e1c7b5</option>
              <option value="user_8b2f9c4d">user_8b2f9c4d</option>
            </select>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {scope === 'org' && (
          <div className="p-6">
            <div className="mb-6">
              <div className="mb-3">
                <h2
                  style={{
                    fontFamily: 'var(--font-display)',
                    fontSize: '14px',
                    fontWeight: 600,
                    color: '#E9ECEF',
                    letterSpacing: '1px',
                  }}
                >
                  DATA FLOW SANKEY
                </h2>
                <p style={{ fontSize: '10px', color: '#343A40', marginTop: '2px' }}>
                  Aggregated data volumes and flow paths across collectors, pipelines, and sinks
                </p>
              </div>
              <div style={{ height: '400px', background: '#111216', border: '1px solid #343A40', padding: '16px' }}>
                <ReactECharts option={sankeyOption} style={{ height: '100%', width: '100%' }} />
              </div>
            </div>

            <div className="grid grid-cols-3 gap-4 mb-6">
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: '#E9ECEF',
                      letterSpacing: '1px',
                    }}
                  >
                    EVENTS/SEC
                  </h3>
                  <div style={{ fontSize: '10px', color: '#343A40' }}>
                    AVG <span style={{ color: '#00F0FF', fontWeight: 600 }}>5.394K</span>{' '}
                    <span style={{ color: '#FF2A00' }}>-0.345%</span>
                  </div>
                </div>
                <div style={{ height: '200px', background: '#111216', border: '1px solid #343A40', padding: '12px' }}>
                  <ReactECharts option={epsOption} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>

              <div>
                <div className="mb-2 flex items-center justify-between">
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: '#E9ECEF',
                      letterSpacing: '1px',
                    }}
                  >
                    LATENCY
                  </h3>
                  <div style={{ fontSize: '10px', color: '#343A40' }}>
                    AVG <span style={{ color: '#FFB300', fontWeight: 600 }}>485ms</span>{' '}
                    <span style={{ color: '#FF2A00' }}>+12.4%</span>
                  </div>
                </div>
                <div style={{ height: '200px', background: '#111216', border: '1px solid #343A40', padding: '12px' }}>
                  <ReactECharts option={latencyOption} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>

              <div>
                <div className="mb-2 flex items-center justify-between">
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: '#E9ECEF',
                      letterSpacing: '1px',
                    }}
                  >
                    THROUGHPUT
                  </h3>
                  <div style={{ fontSize: '10px', color: '#343A40' }}>
                    AVG <span style={{ color: '#8B5CF6', fontWeight: 600 }}>5.12 MB/s</span>{' '}
                    <span style={{ color: '#00F0FF' }}>-0.345%</span>
                  </div>
                </div>
                <div style={{ height: '200px', background: '#111216', border: '1px solid #343A40', padding: '12px' }}>
                  <ReactECharts option={throughputOption} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-4 gap-4">
              <div style={{ background: '#111216', border: '1px solid #343A40', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>LISTENERS</div>
                <div style={{ fontSize: '24px', color: '#3B82F6', fontWeight: 700, marginTop: '8px' }}>50.69K</div>
                <div style={{ fontSize: '10px', color: '#343A40', marginTop: '4px' }}>EPS</div>
              </div>
              <div style={{ background: '#111216', border: '1px solid #343A40', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>CLUSTERS</div>
                <div style={{ fontSize: '24px', color: '#8B5CF6', fontWeight: 700, marginTop: '8px' }}>20.36K</div>
                <div style={{ fontSize: '10px', color: '#343A40', marginTop: '4px' }}>EPS</div>
              </div>
              <div style={{ background: '#111216', border: '1px solid #343A40', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>DATA SINKS</div>
                <div style={{ fontSize: '24px', color: '#EC4899', fontWeight: 700, marginTop: '8px' }}>413.55GB</div>
                <div style={{ fontSize: '10px', color: '#343A40', marginTop: '4px' }}>TOTAL</div>
              </div>
              <div style={{ background: '#111216', border: '1px solid #343A40', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>QRADAR</div>
                <div style={{ fontSize: '24px', color: '#FBBF24', fontWeight: 700, marginTop: '8px' }}>71.51GB</div>
                <div style={{ fontSize: '10px', color: '#343A40', marginTop: '4px' }}>STORED</div>
              </div>
            </div>
          </div>
        )}

        {scope === 'user' && (
          <div className="p-6">
            <div className="mb-6">
              <h2
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: '14px',
                  fontWeight: 600,
                  color: '#E9ECEF',
                  letterSpacing: '1px',
                  marginBottom: '4px',
                }}
              >
                USER JOURNEY: {selectedUser}
              </h2>
              <p style={{ fontSize: '10px', color: '#343A40' }}>
                End-to-end trace execution path and individual telemetry metrics
              </p>
            </div>

            <div className="grid grid-cols-2 gap-4 mb-6">
              <div>
                <div className="mb-2">
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: '#E9ECEF',
                      letterSpacing: '1px',
                    }}
                  >
                    USER EPS
                  </h3>
                </div>
                <div style={{ height: '250px', background: '#111216', border: '1px solid #343A40', padding: '12px' }}>
                  <ReactECharts option={epsOption} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>

              <div>
                <div className="mb-2">
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: '#E9ECEF',
                      letterSpacing: '1px',
                    }}
                  >
                    USER LATENCY
                  </h3>
                </div>
                <div style={{ height: '250px', background: '#111216', border: '1px solid #343A40', padding: '12px' }}>
                  <ReactECharts option={latencyOption} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div style={{ background: '#111216', border: '1px solid #00F0FF', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>TOTAL REQUESTS</div>
                <div style={{ fontSize: '28px', color: '#00F0FF', fontWeight: 700, marginTop: '8px' }}>1,247</div>
              </div>
              <div style={{ background: '#111216', border: '1px solid #FFB300', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>AVG LATENCY</div>
                <div style={{ fontSize: '28px', color: '#FFB300', fontWeight: 700, marginTop: '8px' }}>542ms</div>
              </div>
              <div style={{ background: '#111216', border: '1px solid #FF2A00', padding: '16px' }}>
                <div style={{ fontSize: '10px', color: '#343A40', letterSpacing: '1px' }}>ERROR RATE</div>
                <div style={{ fontSize: '28px', color: '#FF2A00', fontWeight: 700, marginTop: '8px' }}>0.8%</div>
              </div>
            </div>

            <div className="mt-6 p-4" style={{ background: '#111216', border: '1px solid #343A40' }}>
              <h3
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: '#E9ECEF',
                  letterSpacing: '1px',
                  marginBottom: '12px',
                }}
              >
                RECENT TRACES
              </h3>
              <div className="space-y-2">
                {[
                  { id: 'd4f2b8a9', time: '14:42:18', status: 'success', latency: 1200 },
                  { id: 'a3e1c7b5', time: '14:41:52', status: 'success', latency: 890 },
                  { id: '8b2f9c4d', time: '14:41:34', status: 'warning', latency: 1842 },
                ].map((trace) => (
                  <div
                    key={trace.id}
                    style={{
                      padding: '12px',
                      background: '#050506',
                      border: '1px solid #343A40',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                    }}
                  >
                    <div>
                      <div style={{ fontSize: '11px', color: '#E9ECEF', fontWeight: 600 }}>trace_{trace.id}</div>
                      <div style={{ fontSize: '10px', color: '#343A40', marginTop: '2px' }}>{trace.time} UTC</div>
                    </div>
                    <div className="flex items-center gap-4">
                      <div style={{ fontSize: '11px', color: '#343A40' }}>
                        <span
                          style={{
                            color: trace.status === 'success' ? '#00F0FF' : '#FFB300',
                            fontWeight: 600,
                          }}
                        >
                          {trace.latency}ms
                        </span>
                      </div>
                      <div
                        style={{
                          width: '8px',
                          height: '8px',
                          borderRadius: '50%',
                          background: trace.status === 'success' ? '#00F0FF' : '#FFB300',
                        }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
