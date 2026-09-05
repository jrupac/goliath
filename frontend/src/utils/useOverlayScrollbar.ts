import React, { RefObject, useCallback, useEffect, useRef } from 'react';

/* Draws a scroll container's scrollbar as an overlay element instead of using
   the native one.

   This exists so the article title bar can be full-bleed. A native scrollbar
   is laid out between the border box and the padding box, and a scroller clips
   its contents to that padding box, so nothing inside the scroller can paint
   over the scrollbar and nothing outside it can paint there without covering
   the scrollbar entirely. A header that spans the full width therefore has to
   have the scrollbar drawn on top of it, which means drawing it ourselves.

   Only the *painted* bar is replaced. The scroller stays a real scroller, so
   the wheel, keyboard, touch scrolling, find-in-page and scrollIntoView are
   all still handled natively; this just mirrors scrollTop and writes it back
   when the thumb is dragged.

   The track starts below the sticky header rather than at the top of the
   scroller. That keeps the thumb from being drawn over the frosted header,
   and it describes the article better besides: the scroller does extend up
   behind the header, but that part of it is occluded, so the region the thumb
   spans is the region the reader can actually see. */

/** Shortest the thumb may get, so it stays grabbable on very long articles. */
const MIN_THUMB_HEIGHT = 24;

export interface OverlayScrollbar {
  railRef: RefObject<HTMLDivElement>;
  thumbRef: RefObject<HTMLDivElement>;
  /** Redraws the thumb from the scroller's current geometry. */
  sync: () => void;
  onRailPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
  onThumbPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
}

export function useOverlayScrollbar(
  scrollRef: RefObject<HTMLElement | null>,
  contentRef: RefObject<HTMLElement | null>,
  headerRef: RefObject<HTMLElement | null>
): OverlayScrollbar {
  const railRef = useRef<HTMLDivElement>(null);
  const thumbRef = useRef<HTMLDivElement>(null);
  const frameRef = useRef(0);

  // Written straight to the DOM rather than through state: this runs on every
  // scroll event, and re-rendering the article on each one would be far more
  // work than the two style writes it takes to move the thumb.
  const sync = useCallback(() => {
    const scroller = scrollRef.current;
    const rail = railRef.current;
    const thumb = thumbRef.current;
    if (!scroller || !rail || !thumb) {
      return;
    }

    const { scrollHeight, clientHeight, scrollTop } = scroller;
    const overflow = scrollHeight - clientHeight;
    // The rail spans the whole scroller — it has to, since its height is what
    // is being measured here and a zero-height rail could never show itself
    // again — but the track is only the part below the header.
    const inset = headerRef.current?.offsetHeight ?? 0;
    const trackHeight = rail.clientHeight - inset;

    // Nothing to scroll, so there is nothing to indicate. Hiding the rail is
    // what keeps short articles from showing a full-height thumb.
    if (overflow <= 0 || trackHeight <= 0) {
      rail.hidden = true;
      return;
    }
    rail.hidden = false;

    // The minimum keeps the thumb grabbable on a long article; the maximum
    // keeps it inside a track that a tall header has left shorter than that
    // minimum.
    const height = Math.min(
      trackHeight,
      Math.max(MIN_THUMB_HEIGHT, (clientHeight / scrollHeight) * trackHeight)
    );
    // Clamped because overscroll (rubber-banding, or a momentum fling) reports
    // a scrollTop outside [0, overflow].
    const progress = Math.min(1, Math.max(0, scrollTop / overflow));
    thumb.style.height = `${height}px`;
    thumb.style.transform = `translateY(${inset + (trackHeight - height) * progress}px)`;
  }, [scrollRef, headerRef]);

  // Coalesced to one update per frame: a fast scroll delivers events faster
  // than the page can paint them.
  const schedule = useCallback(() => {
    if (frameRef.current) {
      return;
    }
    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = 0;
      sync();
    });
  }, [sync]);

  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) {
      return;
    }

    sync();
    scroller.addEventListener('scroll', schedule, { passive: true });

    // The scroller resizes with the window; the content resizes as images load
    // and when reader mode is toggled; the header resizes as each article's
    // title reflows. All three change where the thumb belongs.
    const observer =
      typeof ResizeObserver === 'undefined'
        ? null
        : new ResizeObserver(schedule);
    observer?.observe(scroller);
    if (contentRef.current) {
      observer?.observe(contentRef.current);
    }
    if (headerRef.current) {
      observer?.observe(headerRef.current);
    }

    return () => {
      scroller.removeEventListener('scroll', schedule);
      observer?.disconnect();
      if (frameRef.current) {
        cancelAnimationFrame(frameRef.current);
      }
    };
  }, [scrollRef, contentRef, headerRef, schedule, sync]);

  const onThumbPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const scroller = scrollRef.current;
      const rail = railRef.current;
      const thumb = thumbRef.current;
      if (!scroller || !rail || !thumb) {
        return;
      }

      const overflow = scroller.scrollHeight - scroller.clientHeight;
      const inset = headerRef.current?.offsetHeight ?? 0;
      const travel = rail.clientHeight - inset - thumb.offsetHeight;
      if (overflow <= 0 || travel <= 0) {
        return;
      }

      // Read once, at grab time: the mapping from pointer travel to scroll
      // offset has to stay fixed for the whole drag, or the content would
      // slide out from under the cursor if it grew mid-drag.
      const startY = event.clientY;
      const startScrollTop = scroller.scrollTop;

      event.preventDefault();
      thumb.setPointerCapture(event.pointerId);
      thumb.classList.add('GoliathArticleScrollbarThumbDragging');

      const onMove = (moveEvent: PointerEvent) => {
        scroller.scrollTop =
          startScrollTop + ((moveEvent.clientY - startY) / travel) * overflow;
      };
      const onRelease = (releaseEvent: PointerEvent) => {
        thumb.releasePointerCapture(releaseEvent.pointerId);
        thumb.removeEventListener('pointermove', onMove);
        thumb.removeEventListener('pointerup', onRelease);
        thumb.removeEventListener('pointercancel', onRelease);
        thumb.classList.remove('GoliathArticleScrollbarThumbDragging');
      };

      thumb.addEventListener('pointermove', onMove);
      thumb.addEventListener('pointerup', onRelease);
      thumb.addEventListener('pointercancel', onRelease);
    },
    [scrollRef, headerRef]
  );

  const onRailPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const scroller = scrollRef.current;
      const rail = railRef.current;
      const thumb = thumbRef.current;
      // A press that landed on the thumb is a drag, handled above.
      if (
        !scroller ||
        !rail ||
        !thumb ||
        event.target !== event.currentTarget
      ) {
        return;
      }

      // The strip of rail alongside the header is not part of the track, so a
      // press there is not a press on anything.
      const inset = headerRef.current?.offsetHeight ?? 0;
      if (event.clientY < rail.getBoundingClientRect().top + inset) {
        return;
      }

      const above = event.clientY < thumb.getBoundingClientRect().top;
      scroller.scrollBy({
        top: above ? -scroller.clientHeight : scroller.clientHeight,
        behavior: 'smooth',
      });
    },
    [scrollRef, headerRef]
  );

  return { railRef, thumbRef, sync, onRailPointerDown, onThumbPointerDown };
}
