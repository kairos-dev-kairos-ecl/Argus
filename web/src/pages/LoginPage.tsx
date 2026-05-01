import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuthStore } from '../stores/auth';
import { fetchCsrfToken } from '../services/csrf';
import { scrambleText } from '../lib/text-scramble';
import { checkSetupStatus } from '../services/iam-service';
import backdropUrl from '../assets/backdrop.png';

/**
 * LoginPage — brutalist terminal-style authentication panel.
 *
 * Navigation flow:
 *   - On mount: checks /api/v1/setup/status. If needs_setup → shows "FIRST RUN" banner + setup link.
 *   - Bottom links: "First time setup →" (shown when needs_setup) | "Have an invite? →"
 *   - MFA challenge shown after successful credential submission.
 *   - 429 responses show a Retry-After countdown.
 */
export function LoginPage() {
  const navigate = useNavigate();
  const { login, completeMfa, mfaPending, loading, error } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [headline, setHeadline] = useState('ARGUS XDR');
  const [retryAfter, setRetryAfter] = useState<number | null>(null);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [inviteToken, setInviteToken] = useState('');
  const [showInviteInput, setShowInviteInput] = useState(false);

  // On mount: check if instance needs first-run setup
  useEffect(() => {
    checkSetupStatus()
      .then(({ needs_setup }) => {
        if (needs_setup) {
          setNeedsSetup(true);
          // Auto-redirect only on first check — makes the flow obvious
          navigate('/setup', { replace: true });
        }
      })
      .catch(() => {}); // fail silently — don't block login if endpoint errors
  }, [navigate]);

  // Pre-fetch CSRF token so it is ready when the form is submitted
  useEffect(() => { fetchCsrfToken().catch(() => {}); }, []);

  // Scramble headline while loading
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

  const goToInvite = () => {
    const t = inviteToken.trim();
    if (t) navigate(`/accept-invite?token=${encodeURIComponent(t)}`);
  };

  const inputStyle: React.CSSProperties = {
    background: 'var(--color-background)',
    border: 'var(--border-stark)',
    color: 'var(--color-text)',
    fontFamily: 'var(--font-mono)',
    padding: '8px 12px',
    width: '100%',
    boxSizing: 'border-box',
    outline: 'none',
    fontSize: '13px',
  };

  const btnPrimaryStyle: React.CSSProperties = {
    marginTop: '12px',
    background: 'transparent',
    border: '1px solid var(--color-primary)',
    color: 'var(--color-primary)',
    padding: '10px 16px',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    fontFamily: 'var(--font-mono)',
    cursor: 'pointer',
    fontSize: '12px',
    width: '100%',
  };

  return (
    <div style={{
      minHeight: '100vh',
      position: 'relative',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '24px',
    }}>
      {/* Backdrop image with dark overlay */}
      <div style={{
        position: 'fixed',
        inset: 0,
        backgroundImage: `url(${backdropUrl})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center right',
        zIndex: 0,
      }} />
      <div style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(10, 10, 11, 0.82)',
        zIndex: 1,
      }} />

      {/* Panel */}
      <div style={{
        position: 'relative',
        zIndex: 2,
        width: '100%',
        maxWidth: '440px',
        border: 'var(--border-stark)',
        background: 'rgba(10, 10, 11, 0.92)',
        backdropFilter: 'blur(12px)',
        padding: '36px 32px 28px',
      }}>
        {/* First-run banner */}
        {needsSetup && (
          <div style={{
            marginBottom: '20px',
            padding: '8px 12px',
            border: '1px solid var(--color-primary)',
            background: 'rgba(0,200,150,0.06)',
            fontSize: '11px',
            color: 'var(--color-primary)',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
          }}>
            FIRST RUN DETECTED —{' '}
            <Link to="/setup" style={{ color: 'var(--color-primary)', textDecoration: 'underline' }}>
              SET UP YOUR INSTANCE →
            </Link>
          </div>
        )}

        {/* Headline */}
        <div style={{
          fontFamily: 'var(--font-display)',
          fontSize: '28px',
          color: 'var(--color-primary)',
          marginBottom: '28px',
          letterSpacing: '0.02em',
        }}>
          {headline}<span className="animate-pulse">_</span>
        </div>

        {/* Credential form */}
        {!mfaPending ? (
          <form onSubmit={submitCreds} style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <label style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)', letterSpacing: '0.06em' }}>
              EMAIL
            </label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
              autoComplete="email"
              style={inputStyle}
            />

            <label style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)', letterSpacing: '0.06em', marginTop: '8px' }}>
              PASSWORD
            </label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              autoComplete="current-password"
              style={inputStyle}
            />

            <button
              type="submit"
              disabled={loading || (retryAfter ?? 0) > 0}
              style={{ ...btnPrimaryStyle, opacity: loading || (retryAfter ?? 0) > 0 ? 0.5 : 1 }}
            >
              {retryAfter && retryAfter > 0
                ? `RATE LIMITED — RETRY IN ${retryAfter}S`
                : 'AUTHENTICATE'}
            </button>
          </form>
        ) : (
          /* MFA challenge form */
          <form onSubmit={submitMfa} style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <label style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)', letterSpacing: '0.06em' }}>
              6-DIGIT AUTHENTICATOR CODE
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
              style={{ ...inputStyle, fontSize: '20px', letterSpacing: '0.5em', textAlign: 'center' }}
            />
            <button
              type="submit"
              disabled={loading || code.length !== 6}
              style={{ ...btnPrimaryStyle, opacity: loading || code.length !== 6 ? 0.5 : 1 }}
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

        {/* ── Navigation footer ── */}
        <div style={{
          marginTop: '28px',
          paddingTop: '20px',
          borderTop: 'var(--border-stark)',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
        }}>
          {/* Invite code entry */}
          {showInviteInput ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label style={{ fontSize: '11px', textTransform: 'uppercase', color: 'var(--color-muted)', letterSpacing: '0.06em' }}>
                PASTE INVITE TOKEN
              </label>
              <div style={{ display: 'flex', gap: '8px' }}>
                <input
                  type="text"
                  value={inviteToken}
                  onChange={e => setInviteToken(e.target.value)}
                  placeholder="argus-inv-..."
                  style={{ ...inputStyle, flexGrow: 1 }}
                  autoFocus
                />
                <button
                  onClick={goToInvite}
                  disabled={!inviteToken.trim()}
                  style={{
                    padding: '8px 14px',
                    border: '1px solid var(--color-primary)',
                    color: 'var(--color-primary)',
                    background: 'transparent',
                    fontFamily: 'var(--font-mono)',
                    fontSize: '11px',
                    cursor: inviteToken.trim() ? 'pointer' : 'not-allowed',
                    opacity: inviteToken.trim() ? 1 : 0.4,
                    textTransform: 'uppercase',
                    whiteSpace: 'nowrap',
                  }}
                >
                  GO →
                </button>
              </div>
              <button
                onClick={() => { setShowInviteInput(false); setInviteToken(''); }}
                style={{ background: 'none', border: 'none', color: 'var(--color-muted)', fontSize: '11px', cursor: 'pointer', textAlign: 'left', padding: 0, fontFamily: 'var(--font-mono)' }}
              >
                ← CANCEL
              </button>
            </div>
          ) : (
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Link
                to="/setup"
                style={{ fontSize: '11px', color: 'var(--color-muted)', textDecoration: 'none', textTransform: 'uppercase', letterSpacing: '0.06em' }}
              >
                FIRST TIME SETUP →
              </Link>
              <button
                onClick={() => setShowInviteInput(true)}
                style={{ background: 'none', border: 'none', color: 'var(--color-muted)', fontSize: '11px', cursor: 'pointer', fontFamily: 'var(--font-mono)', textTransform: 'uppercase', letterSpacing: '0.06em', padding: 0 }}
              >
                HAVE AN INVITE →
              </button>
            </div>
          )}
        </div>
      </div>

      <style>{`@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }`}</style>
    </div>
  );
}
