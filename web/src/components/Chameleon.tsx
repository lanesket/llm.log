import { useState, useEffect, useRef, useCallback } from 'react';
import { useChameleon } from '@/hooks/useChameleon';
import { lighten, darken } from '@/lib/utils';

const PREFERS_REDUCED_MOTION =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

interface TrailDot {
  id: number;
  x: number;
  y: number;
  opacity: number;
  size: number;
}

/**
 * Pixel-art chameleon pet — side view, 48px, CSS-drawn.
 * Realistic chameleon features: casque (helmet head), zygodactyl feet (fused toes),
 * prehensile spiral tail, independently moving eye, color-changing body.
 */
function Chameleon() {
  const { x, y, facing, activity, providerColor, isMoving, pet, startle } = useChameleon();
  const [showTongue, setShowTongue] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [clickBurst, setClickBurst] = useState(false);
  const [trail, setTrail] = useState<TrailDot[]>([]);
  const trailIdRef = useRef(0);
  const lastTrailPos = useRef({ x: 0, y: 0 });
  const tongueTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const isSleeping = activity === 'sleeping';
  const isPetted = activity === 'petted';
  const isStartled = activity === 'startled';
  const isExcited = activity === 'excited';
  const isTongue = activity === 'tongue';
  const noMotion = PREFERS_REDUCED_MOTION;

  useEffect(() => {
    if (isTongue) {
      setShowTongue(true);
      if (tongueTimer.current) clearTimeout(tongueTimer.current);
      tongueTimer.current = setTimeout(() => setShowTongue(false), 700);
    }
    return () => { if (tongueTimer.current) clearTimeout(tongueTimer.current); };
  }, [isTongue]);

  // Trail — bright colored dots that fade and shrink
  useEffect(() => {
    if (!isMoving || noMotion) return;
    const interval = setInterval(() => {
      if (Math.abs(x - lastTrailPos.current.x) + Math.abs(y - lastTrailPos.current.y) < 12) return;
      lastTrailPos.current = { x, y };
      setTrail(prev => [...prev.slice(-8), {
        id: trailIdRef.current++,
        x: x + 24 + (Math.random() - 0.5) * 8,
        y: y + 38 + Math.random() * 4,
        opacity: 0.6,
        size: 5 + Math.random() * 3,
      }]);
    }, 70);
    return () => clearInterval(interval);
  }, [isMoving, x, y]);

  const hasTrail = trail.length > 0;
  useEffect(() => {
    if (!hasTrail) return;
    const timer = setInterval(() => {
      setTrail(prev => prev.map(d => ({ ...d, opacity: d.opacity - 0.025, size: d.size * 0.96 })).filter(d => d.opacity > 0));
    }, 50);
    return () => clearInterval(timer);
  }, [hasTrail]);

  const burstTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => { if (burstTimerRef.current) clearTimeout(burstTimerRef.current); }, []);

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (isSleeping) return;
    pet();
    setClickBurst(true);
    if (burstTimerRef.current) clearTimeout(burstTimerRef.current);
    burstTimerRef.current = setTimeout(() => setClickBurst(false), 700);
  }, [isSleeping, pet]);

  const handleRightClick = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (isSleeping) return;
    startle();
  }, [isSleeping, startle]);

  // Colors — the chameleon changes to match the dominant provider
  const baseColor = providerColor || '#10b981';
  const sleepColor = '#4b5563';
  const c = isSleeping ? sleepColor : baseColor;
  const cLight = lighten(c, 0.25);
  const cDark = darken(c, 0.2);
  const cBelly = lighten(c, 0.4);

  const scaleX = facing === 'left' ? -1 : 1;

  // Size
  const W = 48;
  const H = 44;

  return (
    <>
      {/* Trail sparkles */}
      {trail.map(dot => (
        <div key={dot.id} style={{
          position: 'absolute', left: dot.x, top: dot.y,
          width: dot.size, height: dot.size, borderRadius: '50%',
          background: `radial-gradient(circle, ${baseColor}cc, ${baseColor}44)`,
          opacity: dot.opacity, pointerEvents: 'none', zIndex: 39,
          boxShadow: `0 0 ${dot.size}px ${baseColor}66`,
        }} />
      ))}

      <div style={{
        position: 'absolute', top: y, left: x,
        zIndex: 40, pointerEvents: 'none',
        transition: isMoving ? 'none' : 'top 0.2s ease-out, left 0.2s ease-out',
      }}>
        {/* Click ring */}
        {clickBurst && <div style={{
          position: 'absolute', top: 4, left: 4, width: 40, height: 40,
          borderRadius: '50%', border: `2px solid ${baseColor}`,
          animation: 'chameleon-click-ring 0.7s ease-out forwards',
          pointerEvents: 'none',
        }} />}

        {/* Hover glow */}
        {isHovered && !isSleeping && <div style={{
          position: 'absolute', top: 0, left: 0, width: W, height: H,
          borderRadius: '50%',
          background: `radial-gradient(ellipse, ${baseColor}18, transparent 70%)`,
          animation: 'chameleon-glow-pulse 2s ease-in-out infinite',
          pointerEvents: 'none',
        }} />}

        {/* Hit area */}
        <div style={{
          position: 'absolute', width: W + 16, height: H + 16, top: -8, left: -8,
          cursor: isSleeping ? 'default' : 'pointer',
          pointerEvents: 'auto', borderRadius: 8,
        }}
          role="button"
          aria-label={isSleeping ? 'Chameleon (sleeping)' : 'Pet the chameleon'}
          tabIndex={-1}
          onClick={handleClick}
          onContextMenu={handleRightClick}
          onMouseEnter={() => setIsHovered(true)}
          onMouseLeave={() => setIsHovered(false)}
          title={isSleeping ? 'Sleeping... (proxy off)' : 'Click to pet! Right-click to spook!'}
        />

        {/* ═══ THE CHAMELEON ═══ */}
        <div style={{
          position: 'relative', width: W, height: H,
          transform: `scaleX(${scaleX})`,
          transition: 'transform 0.3s ease',
          filter: isHovered && !isSleeping ? 'brightness(1.12)' : undefined,
        }}>

          {/* ── Tail: prehensile spiral (3 curving segments) ── */}
          <svg style={{ position: 'absolute', top: 6, left: -8, width: 20, height: 28, overflow: 'visible' }} viewBox="0 0 20 28">
            <path
              d="M18,8 C18,4 14,2 11,4 C8,6 6,10 8,13 C10,16 14,14 14,11 C14,9 12,8 11,9"
              fill="none"
              stroke={c}
              strokeWidth="3.5"
              strokeLinecap="round"
              style={{ transition: 'stroke 0.5s' }}
            />
          </svg>

          {/* ── Body ── */}
          <div style={{
            position: 'absolute', width: 26, height: 20, top: 10, left: 8,
            borderRadius: '55% 45% 50% 50%',
            background: `linear-gradient(180deg, ${c} 0%, ${cLight} 40%, ${cBelly} 100%)`,
            transition: 'background 0.5s',
            animation: noMotion ? 'none'
              : isSleeping ? 'chameleon-breathe 3s ease-in-out infinite'
              : isExcited ? 'chameleon-bounce 0.4s ease-in-out infinite'
              : isPetted ? 'chameleon-sprite-wiggle 0.35s ease-in-out infinite'
              : isStartled ? 'chameleon-sprite-jump 0.4s ease-out 1'
              : isMoving ? 'chameleon-body-bob 0.3s ease-in-out infinite'
              : undefined,
          }} />

          {/* Dorsal ridge (row of bumps along back) */}
          {[12, 17, 22, 27].map((lx, i) => (
            <div key={i} style={{
              position: 'absolute', width: 4, height: 3, top: 9, left: lx,
              borderRadius: '50% 50% 0 0',
              backgroundColor: cDark, transition: 'background-color 0.5s',
            }} />
          ))}

          {/* ── Head with casque (helmet) ── */}
          <div style={{
            position: 'absolute', width: 16, height: 15, top: 7, left: 26,
            borderRadius: '35% 55% 50% 40%',
            backgroundColor: c, transition: 'background-color 0.5s',
          }} />

          {/* Casque (raised helmet crest) */}
          <div style={{
            position: 'absolute', width: 10, height: 7, top: 2, left: 27,
            borderRadius: '40% 60% 10% 10%',
            backgroundColor: cDark, opacity: 0.9,
            transition: 'background-color 0.5s',
          }} />

          {/* Jaw / chin */}
          <div style={{
            position: 'absolute', width: 10, height: 4, top: 19, left: 30,
            borderRadius: '0 0 50% 40%',
            backgroundColor: cLight, transition: 'background-color 0.5s',
          }} />

          {/* Mouth line */}
          <div style={{
            position: 'absolute', width: 7, height: 1.5, top: 18, left: 34,
            backgroundColor: cDark, borderRadius: 1,
            transition: 'background-color 0.5s',
          }} />

          {/* ── Eye (large, turret-like, independently moving) ── */}
          <div style={{
            position: 'absolute',
            width: isStartled ? 10 : 8, height: isStartled ? 10 : 8,
            top: isStartled ? 9 : 10, left: isStartled ? 31 : 32,
            borderRadius: '50%', backgroundColor: '#e5e7eb',
            border: `1.5px solid ${cDark}`,
            overflow: 'hidden', transition: 'all 0.15s',
            boxShadow: isHovered ? `0 0 6px ${baseColor}44` : undefined,
          }}>
            {isSleeping ? (
              // Closed eye — peaceful line
              <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <div style={{ width: 5, height: 2, backgroundColor: '#374151', borderRadius: 2 }} />
              </div>
            ) : isPetted ? (
              // Happy squint
              <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <div style={{ width: 5, height: 3, borderBottom: '2.5px solid #374151', borderRadius: '0 0 50% 50%' }} />
              </div>
            ) : (
              // Pupil — vertical slit (like real chameleons!)
              <div style={{
                position: 'absolute', width: 3, height: 5,
                top: 1.5, left: 2.5, borderRadius: '40%',
                backgroundColor: '#1a1a2e',
                animation: noMotion ? 'none'
                  : isExcited ? 'chameleon-eye-independent-a 1.2s ease-in-out infinite'
                  : isStartled ? 'none'
                  : isHovered ? 'none'
                  : 'chameleon-eye-wander 4s ease-in-out infinite',
                transform: isHovered ? 'translate(1px, -1px)' : isStartled ? 'translate(2px, 0)' : undefined,
                transition: 'transform 0.15s',
              }} />
            )}
          </div>

          {/* Blush when petted */}
          {isPetted && <div style={{
            position: 'absolute', width: 5, height: 4, top: 17, left: 35,
            borderRadius: '50%', backgroundColor: '#fb7185', opacity: 0.45,
          }} />}

          {/* ── Legs: zygodactyl (fused toes, like real chameleons) ── */}
          {/* Front leg pair */}
          <ChameleonLeg x={28} color={c} darkColor={cDark} isMoving={isMoving} front />
          {/* Back leg pair */}
          <ChameleonLeg x={12} color={c} darkColor={cDark} isMoving={isMoving} front={false} />

          {/* ── Tongue ── */}
          {(isTongue || showTongue) && (
            <div style={{
              position: 'absolute', height: 2.5, top: 17, left: 40,
              width: 22,
              backgroundColor: '#f87171',
              borderRadius: '0 3px 3px 0',
              transformOrigin: 'left',
              animation: 'chameleon-tongue 0.7s ease-out forwards',
            }}>
              {/* Tongue tip (sticky bulb) */}
              <div style={{
                position: 'absolute', right: -2, top: -2,
                width: 6, height: 6, borderRadius: '50%',
                backgroundColor: '#ef4444',
              }} />
            </div>
          )}
        </div>

        {/* ═══ EFFECTS (outside scaleX container so they don't flip) ═══ */}

        {/* Zzz */}
        {isSleeping && (
          <div style={{ position: 'absolute', top: -8, left: 34, pointerEvents: 'none' }}>
            {['z', 'z', 'z'].map((ch, i) => (
              <span key={i} style={{
                position: 'absolute',
                top: i * -9, left: i * 5,
                fontSize: 8 + i * 2,
                color: '#9ca3af',
                fontFamily: 'monospace', fontWeight: 'bold',
                animation: `chameleon-zzz-${i + 1} 2.5s ease-in-out infinite ${i * 0.4}s`,
                opacity: i === 0 ? 1 : 0,
              }}>{ch}</span>
            ))}
          </div>
        )}

        {/* Hearts */}
        {isPetted && (
          <div style={{ position: 'absolute', top: -14, left: 12, pointerEvents: 'none' }}>
            <span style={{ position: 'absolute', fontSize: 12, animation: 'chameleon-heart 1s ease-out forwards' }}>❤️</span>
            <span style={{ position: 'absolute', left: 18, top: 4, fontSize: 9, animation: 'chameleon-heart 1s ease-out forwards 0.2s', opacity: 0 }}>💚</span>
            <span style={{ position: 'absolute', left: -6, top: 2, fontSize: 8, animation: 'chameleon-heart 1s ease-out forwards 0.4s', opacity: 0 }}>✨</span>
          </div>
        )}

        {/* Startled */}
        {isStartled && (
          <div style={{ position: 'absolute', top: -8, left: 20, fontSize: 14, animation: 'chameleon-startle-emoji 0.5s ease-out forwards', pointerEvents: 'none' }}>❗</div>
        )}
      </div>
    </>
  );
}

/** Chameleon leg — zygodactyl: two fused "toes" gripping */
function ChameleonLeg({ x, color, darkColor, isMoving, front }: {
  x: number; color: string; darkColor: string; isMoving: boolean; front: boolean;
}) {
  const animName = front ? 'chameleon-leg-front' : 'chameleon-leg-back';
  return (
    <div style={{
      position: 'absolute', top: 27, left: x, width: 6, height: 10,
      transformOrigin: 'top center',
      animation: isMoving && !PREFERS_REDUCED_MOTION ? `${animName} 0.35s ease-in-out infinite` : undefined,
    }}>
      {/* Upper leg */}
      <div style={{
        width: 4, height: 6, backgroundColor: color,
        borderRadius: '2px 2px 0 0',
        margin: '0 auto',
        transition: 'background-color 0.5s',
      }} />
      {/* Foot — two-toed grip */}
      <div style={{ position: 'relative', width: 6, height: 4 }}>
        <div style={{
          position: 'absolute', left: 0, width: 3, height: 3,
          backgroundColor: darkColor, borderRadius: '0 0 0 50%',
          transition: 'background-color 0.5s',
        }} />
        <div style={{
          position: 'absolute', right: 0, width: 3, height: 3,
          backgroundColor: darkColor, borderRadius: '0 0 50% 0',
          transition: 'background-color 0.5s',
        }} />
      </div>
    </div>
  );
}

export { Chameleon };
