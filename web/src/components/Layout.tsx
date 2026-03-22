import { NavLink, Outlet } from 'react-router-dom';
import { useChameleon } from '@/hooks/useChameleon';
import { ProxyControl } from './ProxyControl';
import { cn } from '@/lib/utils';

const navItems = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/requests', label: 'Requests' },
  { to: '/analytics', label: 'Analytics' },
];

export function Layout() {
  const { mode, toggleMode, activity, providerColor } = useChameleon();
  const isSleeping = activity === 'sleeping';
  const isHome = mode === 'home';
  const nestColor = providerColor || '#10b981';

  return (
    <div className="min-h-screen bg-background text-foreground">
      <a href="#main-content" className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:p-3 focus:bg-secondary focus:text-foreground">
        Skip to content
      </a>
      <header className="border-b border-[var(--color-separator)] px-6 py-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-lg font-bold font-mono tracking-tight">llm<span className="text-[var(--accent-provider)]">.</span>log</span>
          <button
            onClick={toggleMode}
            title={isHome ? 'Let chameleon roam' : 'Call chameleon home'}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors px-1.5 py-0.5 rounded"
          >
            {isHome ? '🌿 roam' : '🏠 home'}
          </button>
        </div>
        <nav className="flex gap-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  'px-2 sm:px-3 py-1.5 text-xs sm:text-sm font-medium transition-colors',
                  isActive
                    ? 'text-foreground border-b-2 border-[var(--accent-provider)]'
                    : 'text-muted-foreground hover:text-foreground'
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <ProxyControl />
      </header>

      {/* Chameleon home — pixel terminal/pod (z-index BELOW chameleon) */}
      <div
        style={{
          position: 'absolute',
          top: 52,
          left: 18,
          zIndex: 30,
          pointerEvents: 'none',
          opacity: isHome ? 1 : 0.12,
          transition: 'opacity 0.6s ease',
        }}
      >
        <svg width="52" height="32" viewBox="0 0 52 32" style={{ imageRendering: 'pixelated' as const }}>
          {/* Terminal frame */}
          <rect x="2" y="4" width="48" height="26" rx="3" fill="#1e293b" stroke="#334155" strokeWidth="1.5" />
          {/* Screen area */}
          <rect x="5" y="7" width="42" height="18" rx="1.5" fill="#0f172a" />
          {/* Screen glow */}
          <rect x="5" y="7" width="42" height="18" rx="1.5" fill={nestColor} opacity={isHome ? 0.08 : 0.03}>
            {isHome && <animate attributeName="opacity" values="0.05;0.12;0.05" dur="3s" repeatCount="indefinite" />}
          </rect>
          {/* Prompt lines */}
          <rect x="8" y="10" width="14" height="1.5" rx="0.5" fill={nestColor} opacity="0.3" />
          <rect x="8" y="14" width="20" height="1.5" rx="0.5" fill={nestColor} opacity="0.2" />
          <rect x="8" y="18" width="8" height="1.5" rx="0.5" fill={nestColor} opacity="0.25" />
          {/* Blinking cursor */}
          {isHome && (
            <rect x="17" y="18" width="2" height="2" fill={nestColor} opacity="0.6">
              <animate attributeName="opacity" values="0.6;0;0.6" dur="1.2s" repeatCount="indefinite" />
            </rect>
          )}
          {/* Status LEDs */}
          <circle cx="42" cy="10" r="1.2" fill={isHome ? nestColor : '#475569'}>
            {isHome && !isSleeping && <animate attributeName="opacity" values="1;0.4;1" dur="2s" repeatCount="indefinite" />}
          </circle>
          <circle cx="42" cy="14" r="1.2" fill={isSleeping ? '#fbbf24' : '#475569'} opacity={isSleeping ? 0.6 : 0.3} />
          {/* Stand */}
          <rect x="16" y="28" width="20" height="3" rx="1" fill="#334155" />
          <rect x="22" y="26" width="8" height="3" fill="#334155" />
        </svg>
      </div>

      <main id="main-content" className="p-6 lg:px-8 max-w-[1400px] mx-auto">
        <Outlet />
      </main>
    </div>
  );
}
