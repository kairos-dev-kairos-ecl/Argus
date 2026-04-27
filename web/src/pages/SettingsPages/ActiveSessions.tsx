/**
 * ActiveSessions
 *
 * Lists all active sessions for the current user.
 * - Per row: DEVICE, IP, CREATED, LAST SEEN, REVOKE button
 * - Current session: shows CURRENT chip, no revoke button
 * - REVOKE ALL OTHER SESSIONS: mass revoke
 */

import { useEffect, useState } from 'react';
import { fetchSessions, revokeSession, revokeAllOther } from '../../services/iam-service';
import type { Session } from '../../services/iam-service';

const sectionStyle: React.CSSProperties = {
  padding: '24px',
  fontFamily: 'var(--font-mono)',
  color: 'var(--color-text)',
};

const titleStyle: React.CSSProperties = {
  fontFamily: 'var(--font-display)',
  fontSize: '20px',
  color: 'var(--color-primary)',
  textTransform: 'uppercase',
  marginBottom: '16px',
};

const headerRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  fontSize: '10px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
  borderBottom: 'var(--border-stark)',
  height: '20px',
};

const dataRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  fontSize: '12px',
  borderBottom: 'var(--border-stark)',
  height: '20px',
};

const revokeBtn: React.CSSProperties = {
  border: '1px solid var(--color-alert)',
  color: 'var(--color-alert)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '10px',
  padding: '2px 6px',
  cursor: 'pointer',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
};

const revokeAllBtn: React.CSSProperties = {
  border: '1px solid var(--color-alert)',
  color: 'var(--color-alert)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  padding: '6px 10px',
  cursor: 'pointer',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
  marginBottom: '16px',
};

const currentChip: React.CSSProperties = {
  fontSize: '9px',
  color: 'var(--color-primary)',
  border: '1px solid var(--color-primary)',
  padding: '1px 4px',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
};

function formatDate(s: string) {
  try {
    return new Date(s).toISOString().slice(0, 16).replace('T', ' ');
  } catch {
    return s;
  }
}

export function ActiveSessions() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetchSessions();
      setSessions(res.sessions ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load sessions');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleRevoke = async (id: string) => {
    try {
      await revokeSession(id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Revoke failed');
    }
  };

  const handleRevokeAll = async () => {
    try {
      await revokeAllOther();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Revoke all failed');
    }
  };

  return (
    <section style={sectionStyle}>
      <h2 style={titleStyle}>ACTIVE SESSIONS</h2>

      <button style={revokeAllBtn} onClick={handleRevokeAll}>
        REVOKE ALL OTHER SESSIONS
      </button>

      {error && (
        <div style={{ color: 'var(--color-alert)', fontSize: '12px', marginBottom: '8px' }}>
          {error}
        </div>
      )}

      {loading && sessions.length === 0 ? (
        <div style={{ color: 'var(--color-muted)', fontSize: '12px' }}>Loading sessions...</div>
      ) : (
        <div style={{ borderTop: 'var(--border-stark)' }}>
          {/* Header */}
          <div style={headerRowStyle}>
            <span style={{ width: '180px', flexShrink: 0 }}>DEVICE</span>
            <span style={{ width: '140px', flexShrink: 0 }}>IP</span>
            <span style={{ width: '180px', flexShrink: 0 }}>CREATED</span>
            <span style={{ width: '180px', flexShrink: 0 }}>LAST SEEN</span>
            <span style={{ flex: 1 }}>ACTIONS</span>
          </div>

          {sessions.map((s) => (
            <div key={s.id} style={dataRowStyle}>
              <span style={{ width: '180px', flexShrink: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {s.device || '—'}
              </span>
              <span style={{ width: '140px', flexShrink: 0 }}>{s.ip_address || '—'}</span>
              <span style={{ width: '180px', flexShrink: 0 }}>{formatDate(s.created_at)}</span>
              <span style={{ width: '180px', flexShrink: 0 }}>{formatDate(s.last_seen_at)}</span>
              <span style={{ flex: 1 }}>
                {s.current ? (
                  <span style={currentChip}>CURRENT</span>
                ) : (
                  <button style={revokeBtn} onClick={() => handleRevoke(s.id)}>
                    REVOKE
                  </button>
                )}
              </span>
            </div>
          ))}

          {sessions.length === 0 && (
            <div style={{ color: 'var(--color-muted)', fontSize: '12px', padding: '8px 0' }}>
              No active sessions found.
            </div>
          )}
        </div>
      )}
    </section>
  );
}
