import { createContext, useContext, useState, useEffect, useRef, useCallback, type ReactNode } from 'react';
import { createElement } from 'react';
import { PROVIDER_COLORS } from '@/lib/constants';
import { useLocation } from 'react-router-dom';

type ChameleonActivity = 'sleeping' | 'idle' | 'walking' | 'tongue' | 'excited' | 'petted' | 'startled';
type ChameleonMode = 'roaming' | 'home';

interface ChameleonState {
  x: number;
  y: number;
  facing: 'left' | 'right';
  mode: ChameleonMode;
  activity: ChameleonActivity;
  providerColor: string;
  isMoving: boolean;
}

interface ChameleonContextValue extends ChameleonState {
  toggleMode: () => void;
  feedData: (requestCount: number, proxyRunning: boolean, dominantProvider?: string) => void;
  pet: () => void;
  startle: () => void;
}

// Home position: on top of the terminal nest (left side, below header)
const HOME_X = 28;
const HOME_Y = 38;
const DEFAULT_COLOR = '#10b981';

function easeInOutCubic(t: number): number {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function dist(x1: number, y1: number, x2: number, y2: number): number {
  return Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2);
}

/**
 * Find perch points by scanning the DOM for elements with data-chameleon-perch.
 * Falls back to header-based perches. Returns ABSOLUTE page coordinates.
 */
function getPerchPoints(): { x: number; y: number }[] {
  const points: { x: number; y: number }[] = [];
  const scrollY = window.scrollY;

  // Header perches (always present)
  const header = document.querySelector('header');
  if (header) {
    const rect = header.getBoundingClientRect();
    const headerY = rect.top + scrollY + rect.height - 14; // sit on bottom edge of header
    points.push(
      { x: HOME_X, y: HOME_Y + scrollY },
      { x: rect.width * 0.35, y: headerY },
      { x: rect.width * 0.55, y: headerY },
      { x: rect.width * 0.75, y: headerY },
    );
  }

  // Scan for perch-able elements
  const perchElements = document.querySelectorAll('[data-chameleon-perch]');
  perchElements.forEach(el => {
    const rect = el.getBoundingClientRect();
    // Sit on top edge of the element
    points.push({
      x: rect.left + rect.width * 0.3 + Math.random() * rect.width * 0.4,
      y: rect.top + scrollY - 8,
    });
  });

  // If no perch elements found, add some based on main content area
  if (perchElements.length === 0) {
    const main = document.querySelector('main');
    if (main) {
      const rect = main.getBoundingClientRect();
      const baseY = rect.top + scrollY;
      points.push(
        { x: rect.left + 40, y: baseY + 60 },
        { x: rect.left + rect.width - 40, y: baseY + 60 },
        { x: rect.left + rect.width * 0.5, y: baseY + 200 },
        { x: rect.left + 40, y: baseY + 350 },
        { x: rect.left + rect.width - 40, y: baseY + 350 },
      );
    }
  }

  return points;
}

const ChameleonContext = createContext<ChameleonContextValue | null>(null);

export function useChameleon(): ChameleonContextValue {
  const ctx = useContext(ChameleonContext);
  if (!ctx) throw new Error('useChameleon must be used within ChameleonProvider');
  return ctx;
}

export function ChameleonProvider({ children }: { children: ReactNode }) {
  const location = useLocation();

  const [state, setState] = useState<ChameleonState>(() => {
    const savedMode = (typeof window !== 'undefined' && localStorage.getItem('chameleon-mode')) as ChameleonMode | null;
    return {
      x: HOME_X, y: HOME_Y,
      facing: 'right',
      mode: savedMode === 'roaming' ? 'roaming' : 'home',
      activity: 'idle',
      providerColor: DEFAULT_COLOR,
      isMoving: false,
    };
  });

  const animRef = useRef<number | null>(null);
  const stateRef = useRef(state);
  const prevRequestCount = useRef(0);
  const prevProxyRunning = useRef<boolean | null>(null);
  const idleTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const activityTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => { stateRef.current = state; }, [state]);
  useEffect(() => { localStorage.setItem('chameleon-mode', state.mode); }, [state.mode]);

  const stopAll = useCallback(() => {
    if (animRef.current) { cancelAnimationFrame(animRef.current); animRef.current = null; }
    if (idleTimeoutRef.current) { clearTimeout(idleTimeoutRef.current); idleTimeoutRef.current = null; }
  }, []);

  // Animate to a position using ABSOLUTE page coordinates
  const animateTo = useCallback((targetX: number, targetY: number, speed = 130, onDone?: () => void) => {
    if (animRef.current) cancelAnimationFrame(animRef.current);

    const startX = stateRef.current.x;
    const startY = stateRef.current.y;
    const d = dist(startX, startY, targetX, targetY);

    if (d < 5) { onDone?.(); return; }

    const facing = targetX >= startX ? 'right' : 'left';
    setState(prev => ({ ...prev, activity: 'walking', facing, isMoving: true }));

    const duration = Math.max(400, (d / speed) * 1000);

    // Movement: horizontal first, then vertical (like crawling along edges)
    const dx = targetX - startX;
    const dy = targetY - startY;
    const hRatio = Math.abs(dx) / (Math.abs(dx) + Math.abs(dy) + 0.001);

    const startTime = performance.now();

    const animate = (now: number) => {
      const progress = Math.min((now - startTime) / duration, 1);
      const eased = easeInOutCubic(progress);

      let cx: number, cy: number;
      if (eased < hRatio) {
        const hp = hRatio > 0 ? eased / hRatio : 1;
        cx = startX + dx * hp;
        cy = startY;
      } else {
        cx = startX + dx;
        const vp = hRatio < 1 ? (eased - hRatio) / (1 - hRatio) : 1;
        cy = startY + dy * vp;
      }

      // Flip facing at the corner if needed
      if (eased >= hRatio && eased < hRatio + 0.05 && Math.abs(dy) > 20) {
        setState(prev => ({ ...prev, facing: dy > 0 ? 'right' : 'left' }));
      }

      setState(prev => ({ ...prev, x: Math.round(cx), y: Math.round(cy) }));

      if (progress < 1) {
        animRef.current = requestAnimationFrame(animate);
      } else {
        animRef.current = null;
        setState(prev => ({
          ...prev, x: targetX, y: targetY,
          activity: prev.activity === 'walking' ? 'idle' : prev.activity,
          isMoving: false,
        }));
        onDone?.();
      }
    };

    animRef.current = requestAnimationFrame(animate);
  }, []);

  const scheduleNextRoam = useCallback(() => {
    if (idleTimeoutRef.current) clearTimeout(idleTimeoutRef.current);

    const delay = 3000 + Math.random() * 5000;
    idleTimeoutRef.current = setTimeout(() => {
      const s = stateRef.current;
      if (s.mode !== 'roaming' || s.activity === 'sleeping' || s.isMoving) return;

      const perches = getPerchPoints();
      const candidates = perches.filter(p => dist(p.x, p.y, s.x, s.y) > 80);
      if (candidates.length === 0) return;

      const target = candidates[Math.floor(Math.random() * candidates.length)];
      animateTo(target.x, target.y, 130, () => {
        if (stateRef.current.mode === 'roaming') scheduleNextRoam();
      });
    }, delay);
  }, [animateTo]);

  // Mode changes — animate home, don't teleport
  useEffect(() => {
    if (state.mode === 'roaming' && state.activity !== 'sleeping') {
      setTimeout(() => {
        if (stateRef.current.mode === 'roaming') scheduleNextRoam();
      }, 500);
    } else if (state.mode === 'home') {
      stopAll();
      // Animate to home at the header (absolute position includes scroll)
      const homeAbsY = HOME_Y; // header is always at top
      animateTo(HOME_X, homeAbsY, 200);
    }
    return () => { if (idleTimeoutRef.current) clearTimeout(idleTimeoutRef.current); };
  }, [state.mode, scheduleNextRoam, animateTo, stopAll]);

  // When sleeping, go home slowly
  const isSleeping = state.activity === 'sleeping';
  useEffect(() => {
    if (isSleeping) {
      stopAll();
      animateTo(HOME_X, HOME_Y, 60); // walk slowly home when sleepy
    }
  }, [isSleeping, stopAll, animateTo]);

  // On route change, go to a header perch first
  useEffect(() => {
    if (stateRef.current.mode === 'roaming' && stateRef.current.activity !== 'sleeping') {
      stopAll();
      const headerY = 18;
      const headerX = 200 + Math.random() * 300;
      animateTo(headerX, headerY, 180, () => {
        if (stateRef.current.mode === 'roaming') {
          // Wait a bit, then start roaming on the new page
          setTimeout(() => {
            if (stateRef.current.mode === 'roaming') scheduleNextRoam();
          }, 2000);
        }
      });
    }
  }, [location.pathname]); // eslint-disable-line react-hooks/exhaustive-deps

  const toggleMode = useCallback(() => {
    setState(prev => ({ ...prev, mode: prev.mode === 'roaming' ? 'home' : 'roaming' }));
  }, []);

  const setTempActivity = useCallback((activity: ChameleonActivity, durationMs: number, then: ChameleonActivity = 'idle') => {
    setState(prev => ({ ...prev, activity }));
    if (activityTimeoutRef.current) clearTimeout(activityTimeoutRef.current);
    activityTimeoutRef.current = setTimeout(() => {
      setState(prev => prev.activity === activity ? { ...prev, activity: then } : prev);
    }, durationMs);
  }, []);

  const pet = useCallback(() => {
    if (stateRef.current.activity === 'sleeping') return;
    setTempActivity('petted', 1500);
  }, [setTempActivity]);

  const startle = useCallback(() => {
    if (stateRef.current.activity === 'sleeping') return;
    setTempActivity('startled', 600);
    // Run away after startle
    setTimeout(() => {
      if (stateRef.current.mode === 'roaming' && !stateRef.current.isMoving) {
        const perches = getPerchPoints();
        const far = perches.filter(p => dist(p.x, p.y, stateRef.current.x, stateRef.current.y) > 150);
        const target = far.length > 0 ? far[Math.floor(Math.random() * far.length)] : perches[0];
        if (target) animateTo(target.x, target.y, 250); // run fast!
      }
    }, 600);
  }, [setTempActivity, animateTo]);

  const feedData = useCallback((requestCount: number, proxyRunning: boolean, dominantProvider?: string) => {
    // Tongue flick (only update counter if requestCount > 0 to avoid ProxyControl resetting it)
    if (requestCount > 0) {
      if (requestCount > prevRequestCount.current && prevRequestCount.current > 0) {
        if (stateRef.current.activity !== 'sleeping') {
          setTempActivity('tongue', 600);
        }
      }
      prevRequestCount.current = requestCount;
    }

    // Sleep/wake on proxy toggle
    if (prevProxyRunning.current !== null) {
      if (!proxyRunning && prevProxyRunning.current) {
        setState(prev => ({ ...prev, activity: 'sleeping' }));
      } else if (proxyRunning && !prevProxyRunning.current) {
        setState(prev => ({ ...prev, activity: 'idle' }));
        if (stateRef.current.mode === 'roaming') {
          setTimeout(() => scheduleNextRoam(), 2000);
        }
      }
    }
    prevProxyRunning.current = proxyRunning;

    // Provider color
    if (dominantProvider) {
      const color = PROVIDER_COLORS[dominantProvider.toLowerCase()] || DEFAULT_COLOR;
      setState(prev => prev.providerColor !== color ? { ...prev, providerColor: color } : prev);
    }
  }, [setTempActivity, scheduleNextRoam]);

  useEffect(() => {
    return () => {
      if (animRef.current) cancelAnimationFrame(animRef.current);
      if (idleTimeoutRef.current) clearTimeout(idleTimeoutRef.current);
      if (activityTimeoutRef.current) clearTimeout(activityTimeoutRef.current);
    };
  }, []);

  const value: ChameleonContextValue = { ...state, toggleMode, feedData, pet, startle };
  return createElement(ChameleonContext.Provider, { value }, children);
}

export type { ChameleonActivity, ChameleonMode, ChameleonState, ChameleonContextValue };
