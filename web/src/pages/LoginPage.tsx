import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/auth';
import { fetchCsrfToken } from '../services/csrf';
import { scrambleText } from '../lib/text-scramble';

/**
 * LoginPage — brutalist terminal-style authentication panel (Screen 1 spec).
 *
 * Layout:
 *   - Full-viewport dark background (var(--color-background))
 *   - Centred panel, max-width 800px, stark 1px border
 *   - JetBrains Mono / var(--font-mono) throughout
 *   - Headline scrambles with text-scramble effect while loading
 *   - Branches to 6-digit TOTP input when mfaPending is set
 *   - 429 responses show a Retry-After countdown on the submit button
 *   - All colours via CSS design tokens — no hardcoded hex values
 */
export function LoginPage() {
  const navigate = useNavigate();
  const { login, completeMfa, mfaPending, loading, error } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [headline, setHeadline] = useState('ARGUS XDR');
  const [retryAfter, setRetryAfter] = useState<number | null>(null);

  // Pre-fetch CSRF token so it is ready when the form is submitted
  useEffect(() => { fetchCsrfToken().catch(() => {}); }, []);

  // Scramble headline while loading; settle to context-appropriate title when done
  useEffect(() => {
    if (loading) {
      const cancel = scrambleText('AUTHENTICATING…', setHeadline);
      return cancel;
    } else {
      setHeadline(mfaPending ? 'MFA CHALLENGE' : 'ARGUS XDR');
    }
  }, [loading, mfaPending]);

  // Countdown timer for rate-limit feedback
  useEffect(() => {
    if (retryAfter === null || retryAfter <= 0) return;
    const id = setInterval(() => setRetryAfter(s => (s ?? 1) - 1), 1000);
    return () => clearInterval(id);
  }, [retryAfter]);

  const submitCreds = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await login(email, password);
      if (!useAuthStore.getState().mfaPending) navigate('/');
    } catch (err: any) {
      if (err?.status === 429) setRetryAfter(err?.details?.retry_after_seconds ?? 60);
    }
  };

  const submitMfa = async (e: React.FormEvent) => {
    e.preventDefault();
    await completeMfa(code);
    navigate('/');
  };

  return (
    <div style={{
      minHeight: '100vh',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '24px',
    }}>
      <div style={{
        width: '100%',
        maxWidth: '800px',
        border: 'var(--border-stark)',
        background: 'var(--color-surface)',
        padding: '32px',
      }}>
        {/* Headline — scrambles during auth, blinking cursor */}
        <div style={{
          fontFamily: 'var(--font-display)',
          fontSize: '32px',
          color: 'var(--color-primary)',
          marginBottom: '24px',
          letterSpacing: '0.02em',
        }}>
          {headline}<span className="animate-pulse">_</span>
        </div>

        {/* Credential form */}
        {!mfaPending ? (
          <form onSubmit={submitCreds} style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <label style={{ fontSize: '12px', textTransform: 'uppercase', color: 'var(--color-muted)' }}>
              EMAIL
            </label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
              autoComplete="email"
              style={{
                background: 'var(--color-background)',
                border: 'var(--border-stark)',
                color: 'var(--color-text)',
                fontFamily: 'var(--font-mono)',
                padding: '8px 12px',
              }}
            />

            <label style={{ fontSize: '12px', textTransform: 'uppercase', color: 'var(--color-muted)' }}>
              PASSWORD
            </label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              autoComplete="current-password"
              style={{
                background: 'var(--color-background)',
                border: 'var(--border-stark)',
                color: 'var(--color-text)',
                fontFamily: 'var(--font-mono)',
                padding: '8px 12px',
              }}
            />

            <button
              type="submit"
              disabled={loading || (retryAfter ?? 0) > 0}
              style={{
                marginTop: '12px',
                background: 'transparent',
                border: '1px solid var(--color-primary)',
                color: 'var(--color-primary)',
                padding: '10px 16px',
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                fontFamily: 'var(--font-mono)',
                cursor: 'pointer',
              }}
            >
              {retryAfter && retryAfter > 0
                ? `RATE LIMITED — RETRY IN ${retryAfter}S`
                : 'AUTHENTICATE'}
            </button>
          </form>
        ) : (
          /* MFA challenge form */
          <form onSubmit={submitMfa} style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <label style={{ fontSize: '12px', textTransform: 'uppercase', color: 'var(--color-muted)' }}>
              6-DIGIT CODE
            </label>
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              value={code}
              onChange={e => setCode(e.target.value.replace(/\D/g, ''))}
              required
              autoFocus
              autoComplete="one-time-code"
              style={{
                background: 'var(--color-background)',
                border: 'var(--border-stark)',
                color: 'var(--color-text)',
                fontFamily: 'var(--font-mono)',
                padding: '8px 12px',
                fontSize: '18px',
                letterSpacing: '0.5em',
              }}
            />

            <button
              type="submit"
              disabled={loading || code.length !== 6}
              style={{
                marginTop: '12px',
                background: 'transparent',
                border: '1px solid var(--color-primary)',
                color: 'var(--color-primary)',
                padding: '10px 16px',
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                fontFamily: 'var(--font-mono)',
                cursor: 'pointer',
              }}
            >
              VERIFY
            </button>
          </form>
        )}

        {/* Error display */}
        {error && (
          <div style={{
            marginTop: '16px',
            padding: '8px 12px',
            border: '1px solid var(--color-alert)',
            color: 'var(--color-alert)',
            fontSize: '12px',
          }}>
            ERROR: {error}
          </div>
        )}
      </div>
    </div>
  );
}
