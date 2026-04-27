/**
 * PasswordChange
 *
 * Password change form with HIBP breach indicator:
 * - current_password, new_password, confirm_new_password
 * - Client validation: min 12 chars, new !== current, confirm === new
 * - On success: show HIBP breach count warning or success message
 */

import { useState } from 'react';
import { changePassword } from '../../services/iam-service';

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

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: '10px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
  marginBottom: '4px',
};

const inputStyle: React.CSSProperties = {
  border: 'var(--border-stark)',
  background: 'var(--color-background)',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  padding: '6px 10px',
  color: 'var(--color-text)',
  width: '300px',
  display: 'block',
  marginBottom: '12px',
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

export function PasswordChange() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [status, setStatus] = useState<'idle' | 'success' | 'hibp' | 'error'>('idle');
  const [hibpCount, setHibpCount] = useState(0);
  const [errorMsg, setErrorMsg] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const validationError = (() => {
    if (!current || !next || !confirm) return 'All fields required';
    if (next.length < 12) return 'New password must be at least 12 characters';
    if (next === current) return 'New password must differ from current';
    if (next !== confirm) return 'Passwords do not match';
    return null;
  })();

  const isValid = validationError === null;

  const handleSubmit = async () => {
    if (!isValid) return;
    setSubmitting(true);
    setErrorMsg('');
    setStatus('idle');
    try {
      const res = await changePassword(current, next);
      if (res.hibp_breached && res.hibp_breached > 0) {
        setHibpCount(res.hibp_breached);
        setStatus('hibp');
      } else if (res.changed) {
        setStatus('success');
      }
      setCurrent('');
      setNext('');
      setConfirm('');
    } catch (e) {
      setStatus('error');
      setErrorMsg(e instanceof Error ? e.message : 'Password change failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section style={sectionStyle}>
      <h2 style={titleStyle}>CHANGE PASSWORD</h2>

      <div>
        <label style={labelStyle}>CURRENT PASSWORD</label>
        <input
          type="password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          style={inputStyle}
          autoComplete="current-password"
        />

        <label style={labelStyle}>NEW PASSWORD (MIN 12 CHARS)</label>
        <input
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          style={inputStyle}
          autoComplete="new-password"
        />

        <label style={labelStyle}>CONFIRM NEW PASSWORD</label>
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          style={inputStyle}
          autoComplete="new-password"
        />

        {!isValid && (current || next || confirm) && (
          <div style={{ color: 'var(--color-muted)', fontSize: '11px', marginBottom: '8px' }}>
            {validationError}
          </div>
        )}

        <button
          style={btnPrimaryStyle}
          onClick={handleSubmit}
          disabled={!isValid || submitting}
        >
          {submitting ? 'UPDATING...' : 'SUBMIT'}
        </button>
      </div>

      {status === 'hibp' && (
        <div
          style={{
            color: 'var(--color-warning)',
            fontSize: '12px',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            marginTop: '12px',
          }}
        >
          PASSWORD FOUND IN {hibpCount} BREACH(ES) — CHANGE RECOMMENDED
        </div>
      )}

      {status === 'success' && (
        <div
          style={{
            color: 'var(--color-success)',
            fontSize: '12px',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            marginTop: '12px',
          }}
        >
          PASSWORD UPDATED
        </div>
      )}

      {status === 'error' && (
        <div
          style={{
            color: 'var(--color-alert)',
            fontSize: '12px',
            marginTop: '12px',
          }}
        >
          {errorMsg}
        </div>
      )}
    </section>
  );
}
