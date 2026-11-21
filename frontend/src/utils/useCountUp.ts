import { useEffect, useRef, useState } from 'react';

/** How long the number takes to walk from the old value to the new one. */
const COUNT_UP_MS = 600;

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
 * Only a change of `trigger` animates. The value itself moves every time an
 * article is read, and tweening each of those would leave the number
 * permanently in motion; a change that arrives without a new trigger is taken
 * at once. A reader who has asked for less motion gets the new value
 * directly.
 */
export function useCountUp(value: number, trigger: number): number {
  const [shown, setShown] = useState(value);
  const shownRef = useRef(value);
  const triggerRef = useRef(trigger);

  useEffect(() => {
    const from = shownRef.current;
    const animate = trigger !== triggerRef.current;
    triggerRef.current = trigger;

    const settle = (): void => {
      shownRef.current = value;
      setShown(value);
    };

    if (
      !animate ||
      from === value ||
      prefersReducedMotion() ||
      typeof requestAnimationFrame !== 'function'
    ) {
      settle();
      return;
    }

    let frame = 0;
    const start = performance.now();
    const step = (now: number): void => {
      const progress = Math.min(1, (now - start) / COUNT_UP_MS);
      const eased = 1 - Math.pow(1 - progress, 3);
      const next = Math.round(from + (value - from) * eased);
      shownRef.current = next;
      setShown(next);
      if (progress < 1) {
        frame = requestAnimationFrame(step);
      } else {
        frame = 0;
      }
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
