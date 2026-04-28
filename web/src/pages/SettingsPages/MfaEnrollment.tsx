/**
 * MfaEnrollment
 *
 * Phase 6 MFA enrollment flow:
 * 1. BEGIN ENROLLMENT button calls mfaEnroll() → returns secret, qr_code_data_url, backup_codes
 * 2. Show QR code + manual key + backup codes
 * 3. User enters 6-digit code → mfaVerify(code) → shows success
 */

import { useState } from 'react';
import { mfaEnroll, mfaVerify } from '../../services/auth-service';

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

const inputStyle: React.CSSProperties = {
  border: 'var(--border-stark)',
  background: 'var(--color-background)',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  padding: '6px 10px',
  color: 'var(--color-text)',
  width: '120px',
};

export function MfaEnrollment() {
  const [enrolling, setEnrolling] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [qrUrl, setQrUrl] = useState<string | null>(null);
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [code, setCode] = useState('');
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [errorMsg, setErrorMsg] = useState('');
  const [copied, setCopied] = useState(false);

  const handleEnroll = async () => {
    setEnrolling(true);
    setErrorMsg('');
    try {
      const res = await mfaEnroll();
      setSecret(res.secret);
      setQrUrl(res.qr_code_data_url);
      setBackupCodes(res.backup_codes);
    } catch (e) {
      setErrorMsg(e instanceof Error ? e.message : 'Enrollment failed');
    } finally {
      setEnrolling(false);
    }
  };

  const handleVerify = async () => {
    setErrorMsg('');
    try {
      await mfaVerify(code);
      setStatus('success');
    } catch (e) {
      setStatus('error');
      setErrorMsg(e instanceof Error ? e.message : 'Verification failed');
    }
  };

  const handleCopy = () => {
    if (secret) {
      navigator.clipboard.writeText(secret).then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      });
    }
  };

  return (
    <section style={sectionStyle}>
      <h2 style={titleStyle}>MULTI-FACTOR AUTHENTICATION</h2>

      {!secret && status !== 'success' && (
        <button style={btnPrimaryStyle} onClick={handleEnroll} disabled={enrolling}>
          {enrolling ? 'ENROLLING...' : 'BEGIN ENROLLMENT'}
        </button>
      )}

      {errorMsg && (
        <div style={{ color: 'var(--color-alert)', fontSize: '12px', marginTop: '8px' }}>
          {errorMsg}
        </div>
      )}

      {secret && status !== 'success' && (
        <div style={{ marginTop: '16px' }}>
          {/* 3-column layout: QR | Manual Key | Backup Codes */}
          <div style={{ display: 'grid', gridTemplateColumns: '200px 1fr 1fr', gap: '24px', alignItems: 'start' }}>
            {/* QR Code */}
            <div>
              <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', marginBottom: '8px' }}>
                SCAN QR CODE
              </div>
              {qrUrl && (
                <img
                  src={qrUrl}
                  alt="MFA QR Code"
                  style={{ background: 'var(--color-text)', padding: '8px', width: '180px', height: '180px' }}
                />
              )}
            </div>

            {/* Manual Key */}
            <div>
              <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', marginBottom: '8px' }}>
                MANUAL ENTRY KEY
              </div>
              <code
                style={{
                  display: 'block',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  color: 'var(--color-text)',
                  background: 'var(--color-surface)',
                  border: 'var(--border-stark)',
                  padding: '8px',
                  wordBreak: 'break-all',
                  marginBottom: '8px',
                }}
              >
                {secret}
              </code>
              <button style={btnPrimaryStyle} onClick={handleCopy}>
                {copied ? 'COPIED' : 'COPY KEY'}
              </button>
            </div>

            {/* Backup Codes */}
            <div>
              <div
                style={{
                  fontSize: '11px',
                  color: 'var(--color-warning)',
                  textTransform: 'uppercase',
                  marginBottom: '8px',
                  fontWeight: '600',
                }}
              >
                STORE THESE BACKUP CODES
              </div>
              <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                {backupCodes.map((bc, i) => (
                  <li
                    key={i}
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '12px',
                      color: 'var(--color-text)',
                      lineHeight: '1.8',
                    }}
                  >
                    {bc}
                  </li>
                ))}
              </ul>
            </div>
          </div>

          {/* Verify */}
          <div style={{ marginTop: '24px', display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ fontSize: '12px', color: 'var(--color-muted)', textTransform: 'uppercase' }}>
              ENTER 6-DIGIT CODE:
            </div>
            <input
              type="text"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              style={inputStyle}
              placeholder="000000"
            />
            <button style={btnPrimaryStyle} onClick={handleVerify} disabled={code.length !== 6}>
              VERIFY
            </button>
          </div>

          {status === 'error' && (
            <div style={{ color: 'var(--color-alert)', fontSize: '12px', marginTop: '8px' }}>
              {errorMsg || 'Verification failed. Try again.'}
            </div>
          )}
        </div>
      )}

      {status === 'success' && (
        <div
          style={{
            color: 'var(--color-success)',
            fontSize: '14px',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            marginTop: '16px',
          }}
        >
          MFA ENABLED
        </div>
      )}
    </section>
  );
}
