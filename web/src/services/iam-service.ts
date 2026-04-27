/**
 * IAM Service Layer
 *
 * Provides typed async functions for IAM and user management endpoints:
 * - fetchUsers: List all users
 * - fetchSessions: List active sessions
 * - revokeSession: Revoke a specific session
 * - revokeAllOther: Revoke all sessions except current
 * - fetchApiKeys: List API keys (apps)
 * - createApiKey: Create a new API key (shown ONCE)
 * - revokeApiKey: Revoke an API key
 * - changePassword: Change password with HIBP breach check
 */

import { useAuthStore } from '../stores/auth';
import { getCsrfToken, fetchCsrfToken } from './csrf';

export interface Session {
  id: string;
  device: string;
  ip_address: string;
  created_at: string;
  last_seen_at: string;
  current?: boolean;
}

export interface App {
  id: string;
  name: string;
  created_at: string;
  last_used_at: string | null;
  scopes: string[];
}

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: 'admin' | 'analyst' | 'viewer';
  status: string;
  mfa_enabled: boolean;
  created_at: string;
}

function headers(method: string): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const t = useAuthStore.getState().token;
  if (t) h['Authorization'] = `Bearer ${t}`;
  if (method !== 'GET') {
    const c = getCsrfToken();
    if (c) h['X-CSRF-Token'] = c;
  }
  return h;
}

async function call<T>(url: string, method: string = 'GET', body?: unknown): Promise<T> {
  if (method !== 'GET' && !getCsrfToken()) await fetchCsrfToken();
  const res = await fetch(url, {
    method,
    credentials: 'include',
    headers: headers(method),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error((err as any).message ?? `${method} ${url} failed: ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const fetchUsers     = ()                 => call<{ users: User[] }>('/api/v1/users');
export const fetchSessions  = ()                 => call<{ sessions: Session[] }>('/api/v1/auth/sessions');
export const revokeSession  = (id: string)       => call<void>(`/api/v1/auth/sessions/${id}`, 'DELETE');
export const revokeAllOther = ()                 => call<void>('/api/v1/auth/sessions', 'DELETE');
export const fetchApiKeys   = ()                 => call<{ apps: App[] }>('/api/v1/apps');
export const createApiKey   = (name: string)     => call<{ id: string; key: string; name: string }>('/api/v1/apps', 'POST', { name });
export const revokeApiKey   = (id: string)       => call<void>(`/api/v1/apps/${id}`, 'DELETE');
export const changePassword = (current: string, next: string) =>
  call<{ changed: true; hibp_breached?: number }>('/api/v1/auth/password', 'POST', { current_password: current, new_password: next });
