/**
 * UsersPage — IAM Console (Screen 8)
 *
 * 20/80 sidebar layout:
 * Left (20%): vertical section nav (USERS, ROLES, MFA, SESSIONS, API KEYS, PASSWORD)
 * Right (80%): renders content for the active section
 */

import { useEffect, useState } from 'react';
import { fetchUsers, type User } from '../services/iam-service';
import { RolePermissionMatrix } from '../components/iam/RolePermissionMatrix';
import { MfaEnrollment, ActiveSessions, ApiKeys, PasswordChange } from './SettingsPages';

type Section = 'USERS' | 'ROLES' | 'MFA' | 'SESSIONS' | 'API KEYS' | 'PASSWORD';

const SECTIONS: Section[] = ['USERS', 'ROLES', 'MFA', 'SESSIONS', 'API KEYS', 'PASSWORD'];

const containerStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: '20% 80%',
  height: '100vh',
  background: 'var(--color-background)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
};

const sidebarStyle: React.CSSProperties = {
  borderRight: 'var(--border-stark)',
  background: 'var(--color-surface)',
  overflowY: 'auto',
};

const sidebarItemBase: React.CSSProperties = {
  padding: '12px 16px',
  fontSize: '12px',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
  cursor: 'pointer',
  borderBottom: 'var(--border-stark)',
  userSelect: 'none',
};

function sidebarItemStyle(active: boolean): React.CSSProperties {
  return {
    ...sidebarItemBase,
    borderLeft: active ? '2px solid var(--color-primary)' : '2px solid transparent',
    color: active ? 'var(--color-primary)' : 'var(--color-text)',
    background: active ? 'var(--color-background)' : 'transparent',
  };
}

const mainStyle: React.CSSProperties = {
  overflowY: 'auto',
};

// --- Users data grid ---

const headerRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  fontSize: '10px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
  borderBottom: 'var(--border-stark)',
  height: '20px',
  padding: '0 24px',
};

const dataRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  fontSize: '12px',
  borderBottom: 'var(--border-stark)',
  height: '20px',
  padding: '0 24px',
};

function formatDate(s: string) {
  try {
    return new Date(s).toISOString().slice(0, 10);
  } catch {
    return s;
  }
}

function UsersGrid() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    fetchUsers()
      .then((res) => setUsers(res.users ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load users'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ padding: '24px 0', fontFamily: 'var(--font-mono)' }}>
      <div
        style={{
          padding: '0 24px 12px',
          fontSize: '11px',
          color: 'var(--color-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
        }}
      >
        {loading ? 'LOADING...' : error ? `ERROR: ${error}` : `${users.length} USERS`}
      </div>

      <div style={{ borderTop: 'var(--border-stark)' }}>
        <div style={headerRowStyle}>
          <span style={{ width: '240px', flexShrink: 0 }}>EMAIL</span>
          <span style={{ width: '180px', flexShrink: 0 }}>DISPLAY NAME</span>
          <span style={{ width: '100px', flexShrink: 0 }}>ROLE</span>
          <span style={{ width: '60px', flexShrink: 0 }}>MFA</span>
          <span style={{ width: '100px', flexShrink: 0 }}>STATUS</span>
          <span style={{ flex: 1 }}>CREATED</span>
        </div>

        {users.map((u) => (
          <div key={u.id} style={dataRowStyle}>
            <span
              style={{
                width: '240px',
                flexShrink: 0,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {u.email}
            </span>
            <span
              style={{
                width: '180px',
                flexShrink: 0,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {u.display_name}
            </span>
            <span style={{ width: '100px', flexShrink: 0 }}>{u.role}</span>
            <span
              style={{
                width: '60px',
                flexShrink: 0,
                color: u.mfa_enabled ? 'var(--color-primary)' : 'var(--color-muted)',
              }}
            >
              {u.mfa_enabled ? 'ON' : 'OFF'}
            </span>
            <span style={{ width: '100px', flexShrink: 0 }}>{u.status}</span>
            <span style={{ flex: 1 }}>{formatDate(u.created_at)}</span>
          </div>
        ))}

        {!loading && users.length === 0 && (
          <div
            style={{
              padding: '8px 24px',
              color: 'var(--color-muted)',
              fontSize: '12px',
            }}
          >
            No users found.
          </div>
        )}
      </div>
    </div>
  );
}

// --- Main component ---

export function UsersPage() {
  const [section, setSection] = useState<Section>('USERS');

  return (
    <div style={containerStyle}>
      {/* Left sidebar — 20% */}
      <nav style={sidebarStyle} aria-label="IAM Console navigation">
        {SECTIONS.map((s) => (
          <div
            key={s}
            style={sidebarItemStyle(section === s)}
            onClick={() => setSection(s)}
            role="button"
            aria-pressed={section === s}
            tabIndex={0}
            onKeyDown={(e) => e.key === 'Enter' && setSection(s)}
          >
            {s}
          </div>
        ))}
      </nav>

      {/* Right pane — 80% */}
      <main style={mainStyle}>
        {section === 'USERS' && <UsersGrid />}
        {section === 'ROLES' && <RolePermissionMatrix />}
        {section === 'MFA' && <MfaEnrollment />}
        {section === 'SESSIONS' && <ActiveSessions />}
        {section === 'API KEYS' && <ApiKeys />}
        {section === 'PASSWORD' && <PasswordChange />}
      </main>
    </div>
  );
}
