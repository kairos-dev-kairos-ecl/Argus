import { useAuthStore } from '../stores/auth';
import { getCsrfToken } from './csrf';

export interface Incident {
  id: string;
  created_at: string;
  severity: number;
  status: 'open' | 'triaging' | 'closed';
  title: string;
  atlas_tactic: string;
  layer: number;
  signal_count: number;
  assigned_to: string | null;
}

function authedHeaders(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const t = useAuthStore.getState().token;
  if (t) h['Authorization'] = `Bearer ${t}`;
  const c = getCsrfToken();
  if (c) h['X-CSRF-Token'] = c;
  return h;
}

export async function fetchIncidents(): Promise<Incident[]> {
  const res = await fetch('/api/v1/incidents?limit=200', { credentials: 'include', headers: authedHeaders() });
  if (res.status === 404) return [];  // endpoint not yet implemented — caller should fall back to alerts
  if (!res.ok) throw new Error(`Incidents fetch failed: ${res.status}`);
  const data = await res.json();
  return data.incidents ?? [];
}

export async function fetchAlertsAsIncidents(): Promise<Incident[]> {
  const res = await fetch('/api/v1/alerts?limit=200', { credentials: 'include', headers: authedHeaders() });
  if (!res.ok) throw new Error(`Alerts fetch failed: ${res.status}`);
  const data = await res.json();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (data.alerts ?? []).map((a: any) => ({
    id: a.id,
    created_at: a.created_at ?? a.timestamp,
    severity: a.severity ?? 2,
    status: a.status ?? 'open',
    title: a.title ?? a.rule_name ?? 'UNTITLED',
    atlas_tactic: a.atlas_tactic ?? 'Discovery',
    layer: a.layer ?? 0,
    signal_count: a.signal_count ?? 1,
    assigned_to: a.assigned_to ?? null,
  }));
}

export async function createRule(body: { name: string; yaml: string; severity?: number }): Promise<{ rule_id: string }> {
  const res = await fetch('/api/v1/rules', {
    method: 'POST',
    credentials: 'include',
    headers: authedHeaders(),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    throw new Error((err as any).message ?? `Rule creation failed: ${res.status}`);
  }
  return res.json();
}
