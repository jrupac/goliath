import { useEffect, useRef, useState } from 'react';

/** Share of the distance still to go that the number covers in one frame. */
const FRAME_SHARE = 0.15;

/** The frame that share is quoted for, so a faster display walks no faster. */
const FRAME_MS = 1000 / 60;

const prefersReducedMotion = (): boolean =>
  typeof window !== 'undefined' &&
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/**
 * Walks a displayed number to a new value rather than replacing it.
 *
 * A count that moves says a batch arrived without anything being added to the
 * chrome to say so, which is the whole point of it: the badge it replaces had
 * to appear and then disappear, and pushed its neighbours aside both times.
 *
 * Each frame closes a share of whatever distance is left, so the walk has no
 * fixed length and no destination of its own: a small change is over almost
 * at once, a whole feed marked read takes about a second, and a target that
 * keeps moving is simply followed. That last case is the common one -- read
 * at any speed, the count descends a step or two behind rather than freezing
 * until the reader stops.
 *
 * An increase that no refresh accounts for is taken as given. The first one
 * is the count arriving rather than growing, and there is nothing to watch in
 * an article being marked unread. A reader who has asked for less motion gets
 * every new value directly.
 */
export function useCountTo(value: number, trigger: number): number {
  const [shown, setShown] = useState(value);
  // Carries the walk's position between frames and between walks, unrounded,
  // so that a long approach is not quantised into steps of its own.
  const positionRef = useRef(value);
  const triggerRef = useRef(trigger);

  useEffect(() => {
    const from = positionRef.current;
    const announced = trigger !== triggerRef.current;
    triggerRef.current = trigger;

    const settle = (): void => {
      positionRef.current = value;
      setShown(value);
    };

    if (
      (!announced && value >= from) ||
      from === value ||
      prefersReducedMotion() ||
      typeof requestAnimationFrame !== 'function'
    ) {
      settle();
      return;
    }

    let frame = 0;
    let last = 0;
    const step = (now: number): void => {
      // The first frame is stamped with a time that can precede the render
      // that asked for it, so the walk starts its clock here rather than
      // measuring back to one taken outside the frame loop.
      if (last === 0) {
        last = now;
        frame = requestAnimationFrame(step);
        return;
      }
      const elapsed = Math.max(0, now - last);
      last = now;

      const current = positionRef.current;
      const closed = 1 - Math.pow(1 - FRAME_SHARE, elapsed / FRAME_MS);
      const next = current + (value - current) * closed;
      if (Math.abs(value - next) < 1) {
        settle();
        frame = 0;
        return;
      }
      positionRef.current = next;
      setShown(Math.round(next));
      frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);

    return () => {
      if (frame) {
        cancelAnimationFrame(frame);
      }
    };
  }, [value, trigger]);

  return shown;
}
