import { RefObject, useEffect, useRef, useState } from 'react';
import { flushSync } from 'react-dom';

/* Touch gestures for a layout that shows one pane at a time.

   Where both panes are on screen the reader moves between them by looking.
   Where only one is, every move costs a trip to a control, and the two moves
   that are made most -- back to where this came from, and on to the next
   article -- are the ones a hand can make directly.

   Both gestures carry the pane itself and show what is behind or below it,
   so that the move is one continuous motion: the finger starts it, the
   release finishes it along the same path, and what arrives is what was
   already in view. Nothing is swapped in at the moment of release.

   Sideways is the hierarchy: carrying the pane to the right uncovers what is
   behind it, which is the list behind an article and the feed tree behind the
   list. It tracks the finger exactly, because a gesture that uncovers
   something should feel like a door rather than like elastic.

   Up and down are the sequence, and they are an overscroll rather than a
   swipe: the pull only starts once the article has nothing left to scroll in
   that direction, so it can never take a scroll away from the reader. It is
   damped, which is what makes the reader mean it, and what gives them the
   moment in which the article below is readable before they commit to it.

   The gesture writes two custom properties on the stage and nothing else;
   which elements move, and how far, is the stylesheet's business.

   Touch events rather than pointer events, because a pointer stream is
   cancelled the moment the browser claims the gesture for scrolling -- which
   is exactly the gesture the pull continues from. */

/** Finger travel, past the slop, that commits a pull to the next article. */
const PULL_COMMIT_PX = 120;

/** Share of the pane's width that commits the sideways gesture. */
const BACK_COMMIT_FRACTION = 0.33;

/** Scale of the pull's resistance: the travel at which it gives half. */
const PULL_RESISTANCE_PX = 420;

/** Movement before the gesture decides which way it is going. */
const AXIS_SLOP_PX = 10;

/** How much more sideways than vertical a sideways gesture has to be. */
const AXIS_BIAS = 1.5;

/** Class the stage wears while it carries through or returns. */
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
  // Where what the gesture uncovers is not behind this pane but over it --
  // the sidebar, which slides in rather than being revealed -- the caller
  // takes the distance and moves it itself, and this pane stays put.
  onCarry?: (offset: number) => void;
  onCarryEnd?: (committed: boolean) => void;
}

export interface SwipeNavigateState {
  /** Which way a pull is going, and so which article is shown below it. */
  direction: PeekDirection;
  /** Whether letting go now would go there. */
  armed: boolean;
  /** Whether the pane is being carried sideways, and so whatever is behind
      it has to be rendered. */
  sliding: boolean;
}

const IDLE: SwipeNavigateState = {
  direction: 0,
  armed: false,
  sliding: false,
};

const damp = (distance: number): number => {
  const sign = distance < 0 ? -1 : 1;
  const travelled = Math.abs(distance);
  return sign * (travelled / (1 + travelled / PULL_RESISTANCE_PX));
};

/** Damped travel at which a pull commits. */
const pullCommitOffset = (): number => damp(PULL_COMMIT_PX);

export function useSwipeNavigate(
  scrollRef: RefObject<HTMLElement | null>,
  stageRef: RefObject<HTMLElement | null>,
  handlers: SwipeNavigateHandlers
): SwipeNavigateState {
  const [state, setState] = useState<SwipeNavigateState>(IDLE);

  // Read inside the listeners, so that a changed callback does not mean
  // tearing the listeners down and putting them back mid-gesture.
  const latest = useRef(handlers);
  useEffect(() => {
    latest.current = handlers;
  });

  const { enabled } = handlers;

  useEffect(() => {
    const scroller = scrollRef.current;
    const stage = stageRef.current;
    if (!scroller || !stage || !enabled) {
      return;
    }

    let axis: null | 'x' | 'y' | 'scroll' = null;
    let startX = 0;
    let startY = 0;
    let originY = 0;
    let startTop = 0;
    let carry = 0;
    let pull = 0;
    let settleTimer: ReturnType<typeof setTimeout> | undefined;
    let reported: SwipeNavigateState = IDLE;

    const report = (next: SwipeNavigateState): void => {
      if (
        reported.direction !== next.direction ||
        reported.armed !== next.armed ||
        reported.sliding !== next.sliding
      ) {
        reported = next;
        setState(next);
      }
    };

    // Written straight to the DOM: this runs on every touch move, and the
    // stage holds a whole article's markup.
    const place = (x: number, y: number): void => {
      stage.style.setProperty('--swipe-x', `${x}px`);
      stage.style.setProperty('--swipe-y', `${y}px`);
    };

    const setSettling = (on: boolean): void => {
      stage.classList.toggle(SETTLING_CLASS, on);
    };

    const rest = (): void => {
      axis = null;
      carry = 0;
      pull = 0;
      place(0, 0);
      report(IDLE);
    };

    /** Carries the pane the rest of the way, then takes the move. */
    const carryThrough = (x: number, y: number, move: () => void): void => {
      setSettling(true);
      place(x, y);
      clearTimeout(settleTimer);
      settleTimer = setTimeout(() => {
        // As far as the screen is concerned the move and the pane coming
        // back to rest are one change. Let either land on its own and the
        // pane that is leaving is painted back in place for a frame.
        flushSync(move);
        setSettling(false);
        rest();
      }, SETTLE_MS);
    };

    const release = (): void => {
      if (axis === 'x') {
        const committed = carry >= stage.clientWidth * BACK_COMMIT_FRACTION;
        if (latest.current.onCarry) {
          latest.current.onCarryEnd?.(committed);
          rest();
          return;
        }
        if (committed) {
          carryThrough(stage.clientWidth, 0, () => latest.current.onBack?.());
          return;
        }
      }
      if (axis === 'y' && Math.abs(pull) >= pullCommitOffset()) {
        const forward = pull < 0;
        carryThrough(
          0,
          forward ? -stage.clientHeight : stage.clientHeight,
          () =>
            forward ? latest.current.onNext?.() : latest.current.onPrevious?.()
        );
        return;
      }

      if (axis === 'x' || axis === 'y') {
        setSettling(true);
        clearTimeout(settleTimer);
        settleTimer = setTimeout(() => setSettling(false), SETTLE_MS);
      }
      rest();
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
        // A gesture that has already scrolled the article is a scroll and not
        // a way out of it -- except at the end of the travel, where it can go
        // on into a pull.
        const sideways =
          scroller.scrollTop === startTop &&
          dx > AXIS_SLOP_PX &&
          dx > Math.abs(dy) * AXIS_BIAS;
        if (sideways && (latest.current.onBack || latest.current.onCarry)) {
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
        // The pane is marked pan-y, so the browser is not panning sideways;
        // this stops a long drag becoming a selection or a page gesture.
        event.preventDefault();
        carry = Math.max(0, dx);
        if (latest.current.onCarry) {
          latest.current.onCarry(carry);
        } else {
          place(carry, 0);
        }
        report({
          direction: 0,
          armed: carry >= stage.clientWidth * BACK_COMMIT_FRACTION,
          sliding: true,
        });
      } else if (axis === 'y') {
        pull = damp(touch.clientY - originY);
        place(0, pull);
        report({
          direction: pull < 0 ? 1 : -1,
          armed: Math.abs(pull) >= pullCommitOffset(),
          sliding: false,
        });
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
      stage.style.removeProperty('--swipe-x');
      stage.style.removeProperty('--swipe-y');
    };
  }, [scrollRef, stageRef, enabled]);

  return state;
}
