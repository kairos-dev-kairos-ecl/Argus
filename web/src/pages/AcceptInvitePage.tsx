import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { getInvite, acceptInvite } from '../services/iam-service'

const inputStyle: React.CSSProperties = {
  width: '100%',
  marginTop: '6px',
  padding: '8px 10px',
  background: 'var(--color-background)',
  border: 'var(--border-stark)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '13px',
  boxSizing: 'border-box',
  outline: 'none',
}

const labelStyle: React.CSSProperties = {
  fontSize: '11px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.06em',
  display: 'block',
  marginTop: '12px',
}

const readonlyStyle: React.CSSProperties = {
  ...inputStyle,
  color: 'var(--color-muted)',
  cursor: 'default',
}

export function AcceptInvitePage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const { setAccessToken, setUser } = useAuthStore()

  const [validating, setValidating] = useState(true)
  const [invite, setInvite] = useState<{ email: string; role: string } | null>(null)
  const [invalidReason, setInvalidReason] = useState<string | null>(null)

  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!token) {
      setInvalidReason('No token provided.')
      setValidating(false)
      return
    }
    getInvite(token)
      .then(res => {
        if (!res.valid) {
          setInvalidReason(res.reason ?? 'Invalid or expired invite link.')
        } else {
          setInvite({ email: res.email, role: res.role })
        }
      })
      .catch(() => {
        setInvalidReason('Failed to validate invite link.')
      })
      .finally(() => setValidating(false))
  }, [token])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!displayName.trim()) { setError('Display name required'); return }
    if (password.length < 8) { setError('Password must be at least 8 characters'); return }
    if (password !== confirmPassword) { setError('Passwords do not match'); return }

    setSubmitting(true)
    setError(null)
    try {
      const res = await acceptInvite(token, { display_name: displayName, password })

      if (res.access_token) {
        setAccessToken(res.access_token)
        try {
          const payload = JSON.parse(atob(res.access_token.split('.')[1]))
          setUser({
            id: payload.sub,
            email: payload.email,
            display_name: payload.name,
            role: payload.role,
            permissions: payload.permissions ?? [],
            status: 'active',
            created_at: new Date(payload.iat * 1000).toISOString(),
          } as any)
        } catch {
          // JWT decode failed — proceed anyway
        }
      }

      navigate('/')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to accept invite')
    } finally {
      setSubmitting(false)
    }
  }

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
        maxWidth: '520px',
        border: 'var(--border-stark)',
        background: 'var(--color-surface)',
        padding: '32px',
      }}>
        <div style={{
          fontFamily: 'var(--font-display)',
          fontSize: '24px',
          color: 'var(--color-primary)',
          marginBottom: '24px',
          letterSpacing: '0.02em',
        }}>
          ACCEPT INVITE<span style={{ animation: 'blink 1s steps(1) infinite' }}>_</span>
        </div>

        {validating && (
          <div style={{ color: 'var(--color-muted)', fontSize: '12px', textTransform: 'uppercase' }}>
            VALIDATING INVITE...
          </div>
        )}

        {!validating && invalidReason && (
          <div>
            <div style={{ padding: '12px', border: '1px solid var(--color-alert)', color: 'var(--color-alert)', fontSize: '12px', marginBottom: '20px' }}>
              {invalidReason}
            </div>
            <Link
              to="/login"
              style={{ color: 'var(--color-primary)', fontSize: '12px', textDecoration: 'none', textTransform: 'uppercase', letterSpacing: '0.06em' }}
            >
              ← BACK TO LOGIN
            </Link>
          </div>
        )}

        {!validating && invite && (
          <form onSubmit={handleSubmit}>
            <div>
              <label style={labelStyle}>EMAIL (PRE-FILLED)</label>
              <input type="email" value={invite.email} readOnly style={readonlyStyle} />
            </div>
            <div>
              <label style={labelStyle}>ROLE</label>
              <input type="text" value={invite.role.toUpperCase()} readOnly style={readonlyStyle} />
            </div>
            <div>
              <label style={labelStyle}>DISPLAY NAME</label>
              <input
                type="text"
                value={displayName}
                onChange={e => setDisplayName(e.target.value)}
                placeholder="Your name"
                required
                style={inputStyle}
              />
            </div>
            <div>
              <label style={labelStyle}>PASSWORD</label>
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                required
                autoComplete="new-password"
                style={inputStyle}
              />
            </div>
            <div>
              <label style={labelStyle}>CONFIRM PASSWORD</label>
              <input
                type="password"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                required
                autoComplete="new-password"
                style={inputStyle}
              />
            </div>

            {error && (
              <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>
                ERROR: {error}
              </div>
            )}

            <button
              type="submit"
              disabled={submitting}
              style={{
                marginTop: '20px',
                padding: '10px 20px',
                border: '1px solid var(--color-primary)',
                color: submitting ? 'var(--color-muted)' : 'var(--color-primary)',
                background: 'transparent',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                cursor: submitting ? 'not-allowed' : 'pointer',
                opacity: submitting ? 0.5 : 1,
              }}
            >
              {submitting ? 'CREATING ACCOUNT...' : 'CREATE ACCOUNT'}
            </button>
          </form>
        )}
      </div>

      <style>{`@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }`}</style>
    </div>
  )
}
