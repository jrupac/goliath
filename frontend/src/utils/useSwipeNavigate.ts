import { RefObject, useEffect, useRef, useState } from 'react';

/* Touch gestures for a layout that shows one pane at a time.

   Where both panes are on screen the reader moves between them by looking.
   Where only one is, every move costs a trip to a control, and the two moves
   that are made most -- back to where this came from, and on to the next
   article -- are the ones a hand can make directly.

   Sideways is the hierarchy: carrying the pane to the right uncovers what is
   behind it, which is the list behind an article and the feed tree behind the
   list. It tracks the finger exactly, because a gesture that uncovers
   something should feel like a door rather than like elastic, and it commits
   past a third of the pane.

   Up and down are the sequence, and they are an overscroll rather than a
   swipe: the pull only starts once the article has nothing left to scroll in
   that direction, so it can never take a scroll away from the reader. It is
   damped, so that the reader has to mean it and has a moment to see what is
   coming before it arrives. What that moment shows is the caller's business;
   this only reports how far the pull has got.

   Touch events rather than pointer events, because a pointer stream is
   cancelled the moment the browser claims the gesture for scrolling -- which
   is exactly the gesture the pull continues from. */

/** Finger travel that commits a pull to the next or previous article. */
const PULL_COMMIT_PX = 90;

/** Share of the pane's width that commits the sideways gesture. */
const BACK_COMMIT_FRACTION = 0.33;

/** Scale of the pull's resistance: the travel at which it gives half. */
const PULL_RESISTANCE_PX = 260;

/** How far the peek opens, however hard the pull. */
const PEEK_MAX_PX = 56;

/** How much of the pull the peek takes, up to that ceiling. */
const PEEK_GAIN = 1.15;

/** Movement before the gesture decides which way it is going. */
const AXIS_SLOP_PX = 10;

/** How much more sideways than vertical a sideways gesture has to be. */
const AXIS_BIAS = 1.5;

/** Class the pane and the peek wear while they return or carry through. */
const SETTLING_CLASS = 'GoliathSwipeSettling';

/** How long that takes, which the class's transition has to agree with. */
const SETTLE_MS = 260;

/** 1 while pulling towards the next article, -1 towards the previous. */
export type PeekDirection = 0 | 1 | -1;

export interface SwipeNavigateHandlers {
  /** Only a layout showing one pane at a time has anywhere to swipe to. */
  enabled: boolean;
  onBack?: () => void;
  onNext?: () => void;
  onPrevious?: () => void;
}

export interface SwipeNavigateState {
  direction: PeekDirection;
  /** Whether letting go now would commit the pull. */
  armed: boolean;
}

const damp = (distance: number): number => {
  const sign = distance < 0 ? -1 : 1;
  const travelled = Math.abs(distance);
  return sign * (travelled / (1 + travelled / PULL_RESISTANCE_PX));
};

export function useSwipeNavigate(
  scrollRef: RefObject<HTMLElement | null>,
  paneRef: RefObject<HTMLElement | null>,
  peekRef: RefObject<HTMLElement | null>,
  handlers: SwipeNavigateHandlers
): SwipeNavigateState {
  const [state, setState] = useState<SwipeNavigateState>({
    direction: 0,
    armed: false,
  });

  // Read inside the listeners, so that a changed callback does not mean
  // tearing the listeners down and putting them back mid-gesture.
  const latest = useRef(handlers);
  useEffect(() => {
    latest.current = handlers;
  });

  const { enabled } = handlers;

  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller || !enabled) {
      return;
    }
    // Held for as long as the listeners are, so that the cleanup puts back
    // the same elements the gesture was moving.
    const pane = paneRef.current;
    const peek = peekRef.current;

    let axis: null | 'x' | 'y' | 'scroll' = null;
    let startX = 0;
    let startY = 0;
    let originY = 0;
    let startTop = 0;
    let carry = 0;
    let pull = 0;
    let settleTimer: ReturnType<typeof setTimeout> | undefined;
    // What the caller has been told. Held here rather than read back out of
    // React, so that the comparison costs nothing on a move that changes
    // neither -- which is almost all of them.
    let reported: SwipeNavigateState = { direction: 0, armed: false };

    const report = (direction: PeekDirection, armed: boolean): void => {
      if (reported.direction !== direction || reported.armed !== armed) {
        reported = { direction, armed };
        setState(reported);
      }
    };

    // Written straight to the DOM: this runs on every touch move, and the
    // pane holds a whole article's markup.
    const paint = (): void => {
      if (pane) {
        pane.style.transform = carry === 0 ? '' : `translateX(${carry}px)`;
      }
      if (peek) {
        peek.style.height = `${Math.min(PEEK_MAX_PX, Math.abs(pull) * PEEK_GAIN)}px`;
      }
    };

    const setSettling = (on: boolean): void => {
      pane?.classList.toggle(SETTLING_CLASS, on);
      peek?.classList.toggle(SETTLING_CLASS, on);
    };

    const release = (): void => {
      const committed =
        axis === 'x'
          ? carry >= (pane?.clientWidth ?? 0) * BACK_COMMIT_FRACTION
          : axis === 'y' && Math.abs(pull) >= damp(PULL_COMMIT_PX);

      if (committed) {
        if (axis === 'x') {
          latest.current.onBack?.();
        } else if (pull < 0) {
          latest.current.onNext?.();
        } else {
          latest.current.onPrevious?.();
        }
      }

      axis = null;
      carry = 0;
      pull = 0;
      setSettling(true);
      paint();
      report(0, false);
      clearTimeout(settleTimer);
      settleTimer = setTimeout(() => setSettling(false), SETTLE_MS);
    };

    const onTouchStart = (event: TouchEvent): void => {
      if (event.touches.length !== 1) {
        axis = 'scroll';
        return;
      }
      const touch = event.touches[0];
      axis = null;
      startX = touch.clientX;
      startY = touch.clientY;
      startTop = scroller.scrollTop;
      carry = 0;
      pull = 0;
      setSettling(false);
    };

    const onTouchMove = (event: TouchEvent): void => {
      if (axis === 'scroll' || event.touches.length !== 1) {
        return;
      }
      const touch = event.touches[0];
      const dx = touch.clientX - startX;
      const dy = touch.clientY - startY;

      if (axis === null) {
        // A gesture that has already scrolled the article is a scroll, and
        // stays one until the finger lifts -- except at the end of the
        // travel, where it can go on into a pull.
        const sideways =
          scroller.scrollTop === startTop &&
          dx > AXIS_SLOP_PX &&
          dx > Math.abs(dy) * AXIS_BIAS;
        if (sideways && latest.current.onBack) {
          axis = 'x';
        } else if (Math.abs(dy) > AXIS_SLOP_PX && Math.abs(dy) > Math.abs(dx)) {
          const { scrollTop, scrollHeight, clientHeight } = scroller;
          const atEnd = scrollTop + clientHeight >= scrollHeight - 1;
          const atStart = scrollTop <= 0;
          if (dy < 0 && atEnd && latest.current.onNext) {
            axis = 'y';
          } else if (dy > 0 && atStart && latest.current.onPrevious) {
            axis = 'y';
          }
          // Measured from here rather than from where the finger landed, so
          // that a pull which follows a scroll starts from nothing.
          originY = touch.clientY;
        }
      }

      if (axis === 'x') {
        // The browser is not panning sideways -- the pane is marked pan-y --
        // so this only has to stop a long drag turning into a text
        // selection or a page gesture.
        event.preventDefault();
        carry = Math.max(0, dx);
        paint();
        const width = pane?.clientWidth ?? 0;
        report(0, carry >= width * BACK_COMMIT_FRACTION);
      } else if (axis === 'y') {
        pull = damp(touch.clientY - originY);
        paint();
        report(
          pull < 0 ? 1 : -1,
          Math.abs(pull) >= Math.abs(damp(PULL_COMMIT_PX))
        );
      }
    };

    scroller.addEventListener('touchstart', onTouchStart, { passive: true });
    scroller.addEventListener('touchmove', onTouchMove, { passive: false });
    scroller.addEventListener('touchend', release);
    scroller.addEventListener('touchcancel', release);

    return () => {
      scroller.removeEventListener('touchstart', onTouchStart);
      scroller.removeEventListener('touchmove', onTouchMove);
      scroller.removeEventListener('touchend', release);
      scroller.removeEventListener('touchcancel', release);
      clearTimeout(settleTimer);
      setSettling(false);
      if (pane) {
        pane.style.transform = '';
      }
      if (peek) {
        peek.style.height = '';
      }
    };
  }, [scrollRef, paneRef, peekRef, enabled]);

  return state;
}
