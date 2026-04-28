import { useEffect, useState, useMemo } from 'react';
import { IncidentInbox } from '../components/incidents/IncidentInbox';
import { IncidentDetails } from '../components/incidents/IncidentDetails';
import { fetchIncidents, fetchAlertsAsIncidents, createRule, type Incident } from '../services/incidents-service';

export default function IncidentsPage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchIncidents()
      .then(list => list.length === 0 ? fetchAlertsAsIncidents() : list)
      .then(list => { if (!cancelled) setIncidents(list); })
      .catch(e => { if (!cancelled) setError((e as Error)?.message ?? 'Load failed'); });
    return () => { cancelled = true; };
  }, []);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    incidents.forEach(i => { c[i.atlas_tactic] = (c[i.atlas_tactic] ?? 0) + 1; });
    return c;
  }, [incidents]);

  const selected = incidents.find(i => i.id === selectedId) ?? null;

  const onCreateRule = async (yaml: string, name: string) => {
    await createRule({ name, yaml, severity: selected?.severity ?? 2 });
  };

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: '40% 60%',
      height: '100vh',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
    }}>
      {error && (
        <div style={{ gridColumn: '1 / -1', padding: '12px', color: 'var(--color-alert)', fontSize: '12px', textTransform: 'uppercase' }}>
          ERROR: {error}
        </div>
      )}
      <IncidentInbox incidents={incidents} selectedId={selectedId} onSelect={(i) => setSelectedId(i.id)} />
      <IncidentDetails incident={selected} counts={counts} onCreateRule={onCreateRule} />
    </div>
  );
}
