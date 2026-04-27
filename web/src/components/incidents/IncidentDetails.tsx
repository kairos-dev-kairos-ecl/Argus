import type { Incident } from '../../services/incidents-service';
import { KillChainMatrix } from './KillChainMatrix';
import { RuleYamlEditor } from './RuleYamlEditor';

interface IncidentDetailsProps {
  incident: Incident | null;
  counts: Record<string, number>;
  onCreateRule: (yaml: string, name: string) => Promise<void>;
}

export function IncidentDetails({ incident, counts, onCreateRule }: IncidentDetailsProps) {
  if (!incident) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          background: 'var(--color-background)',
          fontFamily: 'var(--font-mono)',
          fontSize: '11px',
          color: 'var(--color-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
        }}
      >
        SELECT AN INCIDENT
      </div>
    );
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        background: 'var(--color-background)',
        overflow: 'hidden',
      }}
    >
      {/* Section 1: Header */}
      <div
        style={{
          padding: '16px',
          borderBottom: 'var(--border-stark)',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            fontFamily: 'var(--font-display)',
            fontSize: '18px',
            color: 'var(--color-text)',
            marginBottom: '8px',
          }}
        >
          {incident.title}
        </div>
        <div
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            color: 'var(--color-muted)',
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
          }}
        >
          {incident.severity} SEV · L{incident.layer} · {incident.status} · {incident.signal_count} signals · {incident.assigned_to ?? 'unassigned'}
        </div>
      </div>

      {/* Section 2: Kill Chain Matrix */}
      <div style={{ flexShrink: 0 }}>
        <KillChainMatrix activeTactic={incident.atlas_tactic} counts={counts} />
      </div>

      {/* Section 3: YAML Rule Editor */}
      <RuleYamlEditor onSubmit={onCreateRule} />
    </div>
  );
}
