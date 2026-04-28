import { useAuthStore } from '../stores/auth';
import { getCsrfToken } from './csrf';

export interface AuditEntry {
  id: string;
  timestamp: string;
  action: string;
  actor_id: string | null;
  actor_email: string | null;
  resource_type: string;
  resource_id: string;
  ip_address: string;
  hash: string;
  metadata: Record<string, unknown>;
}

export interface AuditFilterState {
  actor?: string;
  ip?: string;
  layer?: number;
}

export async function fetchAuditEntries(
  filters: AuditFilterState = {},
  limit: number = 100,
  offset: number = 0
): Promise<{ entries: AuditEntry[]; total: number }> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  if (filters.actor) params.set('actor', filters.actor);
  if (filters.ip) params.set('ip', filters.ip);
  if (filters.layer != null) params.set('layer', String(filters.layer));

  const token = useAuthStore.getState().token;
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const csrf = getCsrfToken();
  if (csrf) headers['X-CSRF-Token'] = csrf;

  const res = await fetch(`/api/v1/audit?${params.toString()}`, {
    method: 'GET',
    credentials: 'include',
    headers,
  });

  if (!res.ok) throw new Error(`Audit fetch failed: ${res.status}`);
  return res.json();
}
