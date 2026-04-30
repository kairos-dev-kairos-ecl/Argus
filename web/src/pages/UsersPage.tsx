/**
 * UsersPage — IAM Console (Screen 8)
 *
 * 20/80 sidebar layout:
 * Left (20%): vertical section nav (USERS, ROLES, MFA, SESSIONS, API KEYS, PASSWORD)
 * Right (80%): renders content for the active section
 */

import { useEffect, useState } from 'react';
import { fetchUsers, createInvite, type User } from '../services/iam-service';
import { useAuthStore } from '../stores/auth';
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

  // Invite form state
  const isAdmin = useAuthStore(s => s.isAdmin());
  const [showInviteForm, setShowInviteForm] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<'admin' | 'analyst' | 'viewer'>('analyst');
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [inviteURL, setInviteURL] = useState<string | null>(null);
  const [inviteCopied, setInviteCopied] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetchUsers()
      .then((res) => setUsers(res.users ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load users'))
      .finally(() => setLoading(false));
  }, []);

  const handleInviteSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteEmail) { setInviteError('Email required'); return; }
    setInviteLoading(true);
    setInviteError(null);
    setInviteURL(null);
    try {
      const res = await createInvite({ email: inviteEmail, role: inviteRole });
      setInviteURL(res.invite_url);
      setInviteEmail('');
    } catch (e: unknown) {
      setInviteError(e instanceof Error ? e.message : 'Failed to create invite');
    } finally {
      setInviteLoading(false);
    }
  };

  const handleCopyInvite = () => {
    if (inviteURL) {
      navigator.clipboard.writeText(inviteURL).then(() => {
        setInviteCopied(true);
        setTimeout(() => setInviteCopied(false), 2000);
      });
    }
  };

  const monoStyle: React.CSSProperties = { fontFamily: 'var(--font-mono)' };
  const inputSm: React.CSSProperties = {
    padding: '6px 8px',
    background: 'var(--color-background)',
    border: 'var(--border-stark)',
    color: 'var(--color-text)',
    fontFamily: 'var(--font-mono)',
    fontSize: '12px',
  };
  const btnSm = (active = true): React.CSSProperties => ({
    padding: '6px 12px',
    border: '1px solid var(--color-primary)',
    color: active ? 'var(--color-primary)' : 'var(--color-muted)',
    background: 'transparent',
    fontFamily: 'var(--font-mono)',
    fontSize: '11px',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.06em',
    cursor: active ? 'pointer' : 'not-allowed',
    opacity: active ? 1 : 0.5,
  });

  return (
    <div style={{ padding: '24px 0', fontFamily: 'var(--font-mono)' }}>
      {/* Header row with user count and admin-only INVITE USER button */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px 12px' }}>
        <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
          {loading ? 'LOADING...' : error ? `ERROR: ${error}` : `${users.length} USERS`}
        </div>
        {isAdmin && (
          <button
            onClick={() => { setShowInviteForm(v => !v); setInviteURL(null); setInviteError(null); }}
            style={btnSm()}
          >
            {showInviteForm ? 'CANCEL' : '+ INVITE USER'}
          </button>
        )}
      </div>

      {/* Inline invite form — admin only */}
      {isAdmin && showInviteForm && (
        <div style={{ margin: '0 24px 16px', padding: '16px', border: 'var(--border-stark)', background: 'var(--color-background)' }}>
          <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '12px' }}>
            SEND INVITE
          </div>
          <form onSubmit={handleInviteSubmit} style={{ display: 'flex', gap: '8px', alignItems: 'flex-end', flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '10px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', ...monoStyle }}>EMAIL</label>
              <input
                type="email"
                value={inviteEmail}
                onChange={e => setInviteEmail(e.target.value)}
                placeholder="user@example.com"
                required
                style={{ ...inputSm, width: '220px' }}
              />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '10px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', ...monoStyle }}>ROLE</label>
              <select
                value={inviteRole}
                onChange={e => setInviteRole(e.target.value as 'admin' | 'analyst' | 'viewer')}
                style={{ ...inputSm, width: '110px' }}
              >
                <option value="admin">ADMIN</option>
                <option value="analyst">ANALYST</option>
                <option value="viewer">VIEWER</option>
              </select>
            </div>
            <button type="submit" disabled={inviteLoading} style={btnSm(!inviteLoading)}>
              {inviteLoading ? 'SENDING...' : 'SEND INVITE'}
            </button>
          </form>

          {inviteError && (
            <div style={{ marginTop: '8px', fontSize: '11px', color: 'var(--color-alert)', textTransform: 'uppercase' }}>
              ERROR: {inviteError}
            </div>
          )}

          {inviteURL && (
            <div style={{ marginTop: '12px' }}>
              <div style={{ fontSize: '10px', color: 'var(--color-primary)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '4px' }}>
                INVITE SENT. SHARE THIS URL:
              </div>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <input
                  type="text"
                  value={inviteURL}
                  readOnly
                  style={{ ...inputSm, flex: 1, color: 'var(--color-muted)', fontSize: '11px' }}
                  onClick={e => (e.target as HTMLInputElement).select()}
                />
                <button onClick={handleCopyInvite} style={btnSm()}>
                  {inviteCopied ? 'COPIED!' : 'COPY'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

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
