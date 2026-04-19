import { Link, Outlet, useLocation } from "react-router";
import { Activity, Shield, BarChart3, Search, FileText } from "lucide-react";

export function Layout() {
  const location = useLocation();
  
  const navItems = [
    { path: "/", label: "ONBOARDING", icon: Shield },
    { path: "/trace", label: "TRACE", icon: Activity },
    { path: "/dashboard", label: "HEALTH", icon: BarChart3 },
    { path: "/hunt", label: "HUNT", icon: Search },
    { path: "/audit", label: "AUDIT", icon: FileText },
  ];
  
  return (
    <div className="flex h-screen overflow-hidden" style={{ 
      fontFamily: 'var(--font-primary)',
      background: 'var(--color-background)'
    }}>
      {/* Left Sidebar Navigation */}
      <aside className="w-60 flex flex-col" style={{ 
        background: 'var(--color-surface)',
        borderRight: 'var(--border-stark)'
      }}>
        {/* Logo */}
        <div className="p-6" style={{ borderBottom: 'var(--border-stark)' }}>
          <h1 className="uppercase tracking-wider" style={{
            fontFamily: 'var(--font-display)',
            fontSize: '20px',
            fontWeight: 700,
            color: 'var(--color-primary)',
            letterSpacing: '2px'
          }}>
            ARGUS<span style={{ color: 'var(--color-text)' }}>XDR</span>
          </h1>
          <p className="mt-1" style={{ 
            fontSize: '10px', 
            color: 'var(--color-muted)',
            letterSpacing: '1px'
          }}>
            TACTICAL HUB v2.0
          </p>
        </div>
        
        {/* Navigation */}
        <nav className="flex-1 p-4">
          <div className="space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = location.pathname === item.path;
              
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className="flex items-center gap-3 px-4 py-3 transition-colors"
                  style={{
                    background: isActive ? 'var(--color-primary)' : 'transparent',
                    color: isActive ? '#050506' : 'var(--color-text)',
                    border: '1px solid',
                    borderColor: isActive ? 'var(--color-primary)' : 'var(--color-muted)',
                    fontFamily: 'var(--font-display)',
                    fontSize: '12px',
                    fontWeight: 600,
                    letterSpacing: '1px',
                    cursor: 'pointer'
                  }}
                >
                  <Icon size={16} />
                  {item.label}
                </Link>
              );
            })}
          </div>
        </nav>
        
        {/* Footer */}
        <div className="p-4" style={{ borderTop: 'var(--border-stark)' }}>
          <div style={{ fontSize: '10px', color: 'var(--color-muted)' }}>
            <div>STATUS: <span style={{ color: 'var(--color-primary)' }}>ONLINE</span></div>
            <div className="mt-1">NODE: cluster-01.us-west</div>
          </div>
        </div>
      </aside>
      
      {/* Main Content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
