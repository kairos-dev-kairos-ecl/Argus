import type { Incident } from '../../services/incidents-service';

interface IncidentInboxProps {
  incidents: Incident[];
  selectedId: string | null;
  onSelect: (i: Incident) => void;
}

function severityLabel(severity: number): string {
  if (severity >= 4) return 'C';
  if (severity === 3) return 'H';
  if (severity === 2) return 'M';
  return 'L';
}

function severityBg(severity: number): string {
  if (severity >= 4) return 'var(--color-alert)';
  if (severity === 3) return 'var(--color-warning)';
  if (severity === 2) return '#3B82F6'; // info blue — no local token; CSS var fallback below
  return 'var(--color-muted)';
}

export function IncidentInbox({ incidents, selectedId, onSelect }: IncidentInboxProps) {
  return (
    <div
      style={{
        borderRight: 'var(--border-stark)',
        background: 'var(--color-background)',
        overflowY: 'auto',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: '8px 12px',
          borderBottom: 'var(--border-stark)',
          background: 'var(--color-surface)',
          fontSize: '11px',
          textTransform: 'uppercase',
          color: 'var(--color-muted)',
          fontFamily: 'var(--font-mono)',
          letterSpacing: '0.05em',
          flexShrink: 0,
        }}
      >
        INCIDENT INBOX · {incidents.length}
      </div>

      {/* Rows */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {incidents.length === 0 && (
          <div
            style={{
              padding: '16px 12px',
              fontFamily: 'var(--font-mono)',
              fontSize: '11px',
              color: 'var(--color-muted)',
              textTransform: 'uppercase',
            }}
          >
            NO INCIDENTS
          </div>
        )}
        {incidents.map((incident) => {
          const isSelected = incident.id === selectedId;
          return (
            <div
              key={incident.id}
              onClick={() => onSelect(incident)}
              style={{
                height: '20px',
                lineHeight: '20px',
                display: 'flex',
                gap: '8px',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                padding: '0 12px',
                cursor: 'pointer',
                borderBottom: 'var(--border-stark)',
                background: isSelected ? 'var(--color-surface)' : 'transparent',
                borderLeft: isSelected ? '2px solid var(--color-primary)' : '2px solid transparent',
                alignItems: 'center',
                overflow: 'hidden',
              }}
            >
              {/* Severity chip */}
              <span
                style={{
                  width: '40px',
                  minWidth: '40px',
                  background: severityBg(incident.severity),
                  color: 'var(--color-background)',
                  textAlign: 'center',
                  fontSize: '10px',
                  textTransform: 'uppercase',
                  fontWeight: 700,
                  lineHeight: '16px',
                  flexShrink: 0,
                }}
              >
                {severityLabel(incident.severity)}
              </span>

              {/* Time */}
              <span
                style={{
                  width: '80px',
                  minWidth: '80px',
                  color: 'var(--color-muted)',
                  flexShrink: 0,
                  overflow: 'hidden',
                }}
              >
                {(incident.created_at ?? '').slice(11, 19)}
              </span>

              {/* Title */}
              <span
                style={{
                  flex: 1,
                  color: 'var(--color-text)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {incident.title}
              </span>

              {/* ATLAS badge */}
              <span
                style={{
                  width: '140px',
                  minWidth: '140px',
                  color: 'var(--color-primary)',
                  fontSize: '10px',
                  textTransform: 'uppercase',
                  textAlign: 'right',
                  flexShrink: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {incident.atlas_tactic}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
