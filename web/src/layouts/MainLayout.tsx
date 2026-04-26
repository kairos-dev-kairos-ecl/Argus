import type { ReactNode } from 'react';
import { SideNav } from '../components/SideNav';

interface Props { children: ReactNode; }

export function MainLayout({ children }: Props) {
  return (
    <div style={{
      display: 'flex',
      minHeight: '100vh',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
    }}>
      <SideNav />
      <main style={{ flex: 1, minWidth: 0, overflow: 'auto' }}>
        {children}
      </main>
    </div>
  );
}
