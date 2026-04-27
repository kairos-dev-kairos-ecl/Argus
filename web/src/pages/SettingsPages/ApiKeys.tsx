/**
 * ApiKeys
 *
 * API key management:
 * - Create new key (shown ONCE with copy button)
 * - List existing keys (name, created, last used, revoke)
 * - Revoke key per row
 */

import { useEffect, useState } from 'react';
import { fetchApiKeys, createApiKey, revokeApiKey } from '../../services/iam-service';
import type { APIKey } from '../../services/iam-service';

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

const inputStyle: React.CSSProperties = {
  border: 'var(--border-stark)',
  background: 'var(--color-background)',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  padding: '6px 10px',
  color: 'var(--color-text)',
  width: '240px',
};

const btnPrimaryStyle: React.CSSProperties = {
  border: '1px solid var(--color-primary)',
  color: 'var(--color-primary)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  padding: '6px 10px',
  cursor: 'pointer',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
};

const revokeBtn: React.CSSProperties = {
  border: 'var(--border-stark)',
  color: 'var(--color-muted)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '10px',
  padding: '2px 6px',
  cursor: 'pointer',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
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

function formatDate(s: string | null) {
  if (!s) return '—';
  try {
    return new Date(s).toISOString().slice(0, 16).replace('T', ' ');
  } catch {
    return s;
  }
}

export function ApiKeys() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<{ id: string; key: string; prefix: string; name: string } | null>(null);
  const [keyCopied, setKeyCopied] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetchApiKeys();
      setKeys(Array.isArray(res) ? res : []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load API keys');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!newKeyName.trim()) return;
    setError(null);
    try {
      const result = await createApiKey(newKeyName.trim());
      setCreatedKey({ id: result.id, key: result.key, prefix: result.prefix, name: result.name });
      setNewKeyName('');
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create API key');
    }
  };

  const handleRevoke = async (id: string) => {
    setError(null);
    try {
      await revokeApiKey(id);
      if (createdKey?.id === id) setCreatedKey(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to revoke API key');
    }
  };

  const handleCopyKey = () => {
    if (createdKey?.key) {
      navigator.clipboard.writeText(createdKey.key).then(() => {
        setKeyCopied(true);
        setTimeout(() => setKeyCopied(false), 2000);
      });
    }
  };

  return (
    <section style={sectionStyle}>
      <h2 style={titleStyle}>API KEYS</h2>

      {/* Create form */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
        <input
          type="text"
          placeholder="key name..."
          value={newKeyName}
          onChange={(e) => setNewKeyName(e.target.value)}
          style={inputStyle}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <button style={btnPrimaryStyle} onClick={handleCreate} disabled={!newKeyName.trim()}>
          CREATE
        </button>
      </div>

      {/* One-time key reveal */}
      {createdKey && (
        <div
          style={{
            background: 'var(--color-surface)',
            border: '1px solid var(--color-primary)',
            padding: '12px',
            fontFamily: 'var(--font-mono)',
            marginBottom: '16px',
          }}
        >
          <div
            style={{
              fontSize: '11px',
              color: 'var(--color-warning)',
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
              marginBottom: '8px',
            }}
          >
            THIS KEY WILL NOT BE SHOWN AGAIN
          </div>
          <div style={{ fontSize: '12px', color: 'var(--color-text)', wordBreak: 'break-all', marginBottom: '8px' }}>
            {createdKey.key}
          </div>
          <button style={btnPrimaryStyle} onClick={handleCopyKey}>
            {keyCopied ? 'COPIED' : 'COPY'}
          </button>
        </div>
      )}

      {error && (
        <div style={{ color: 'var(--color-alert)', fontSize: '12px', marginBottom: '8px' }}>
          {error}
        </div>
      )}

      {/* Keys list */}
      {loading && keys.length === 0 ? (
        <div style={{ color: 'var(--color-muted)', fontSize: '12px' }}>Loading API keys...</div>
      ) : (
        <div style={{ borderTop: 'var(--border-stark)' }}>
          <div style={headerRowStyle}>
            <span style={{ flex: 2 }}>NAME</span>
            <span style={{ width: '120px', flexShrink: 0 }}>PREFIX</span>
            <span style={{ width: '160px', flexShrink: 0 }}>CREATED</span>
            <span style={{ width: '160px', flexShrink: 0 }}>LAST USED</span>
            <span style={{ width: '80px', flexShrink: 0 }}>REVOKE</span>
          </div>

          {keys.filter((k) => !k.revoked_at).map((key) => (
            <div key={key.id} style={dataRowStyle}>
              <span style={{ flex: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {key.name}
              </span>
              <span style={{ width: '120px', flexShrink: 0, fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-muted)' }}>
                {key.prefix}…
              </span>
              <span style={{ width: '160px', flexShrink: 0 }}>{formatDate(key.created_at)}</span>
              <span style={{ width: '160px', flexShrink: 0 }}>{formatDate(key.last_used_at)}</span>
              <span style={{ width: '80px', flexShrink: 0 }}>
                <button style={revokeBtn} onClick={() => handleRevoke(key.id)}>
                  REVOKE
                </button>
              </span>
            </div>
          ))}

          {keys.filter((k) => !k.revoked_at).length === 0 && (
            <div style={{ color: 'var(--color-muted)', fontSize: '12px', padding: '8px 0' }}>
              No active API keys.
            </div>
          )}
        </div>
      )}
    </section>
  );
}
