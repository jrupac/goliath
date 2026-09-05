import React, { useRef } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { useOverlayScrollbar } from '../useOverlayScrollbar';

// happy-dom does no layout, so every geometry property the hook reads is 0
// unless it is stubbed. These tests drive the arithmetic and the event wiring;
// that the rail lands in the right place on screen is not something a DOM
// stub can tell us.
const stubGeometry = (el: Element, props: Record<string, number>) => {
  for (const [name, value] of Object.entries(props)) {
    Object.defineProperty(el, name, {
      value,
      configurable: true,
      writable: true,
    });
  }
};

const Harness: React.FC = () => {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const { railRef, thumbRef, sync, onRailPointerDown, onThumbPointerDown } =
    useOverlayScrollbar(scrollRef, contentRef);

  return (
    <div>
      <div data-testid="scroller" ref={scrollRef}>
        <div data-testid="content" ref={contentRef} />
      </div>
      <div
        data-testid="rail"
        ref={railRef}
        onPointerDown={onRailPointerDown}
        hidden
      >
        <div
          data-testid="thumb"
          ref={thumbRef}
          onPointerDown={onThumbPointerDown}
        />
      </div>
      <button onClick={sync}>sync</button>
    </div>
  );
};

interface Harnessed {
  scroller: HTMLElement;
  rail: HTMLElement;
  thumb: HTMLElement;
  sync: () => void;
}

// A 2000px article in a 500px window, giving 1500px of overflow and a
// 500 * 500/2000 = 125px thumb with 375px of travel.
const setup = (
  scrollerGeometry: Record<string, number> = {
    scrollHeight: 2000,
    clientHeight: 500,
    scrollTop: 0,
  },
  trackHeight = 500
): Harnessed => {
  render(<Harness />);
  const scroller = screen.getByTestId('scroller');
  const rail = screen.getByTestId('rail');
  const thumb = screen.getByTestId('thumb');

  stubGeometry(scroller, scrollerGeometry);
  stubGeometry(rail, { clientHeight: trackHeight });
  scroller.scrollBy = vi.fn();
  thumb.setPointerCapture = vi.fn();
  thumb.releasePointerCapture = vi.fn();

  const sync = () => fireEvent.click(screen.getByText('sync'));
  sync();
  return { scroller, rail, thumb, sync };
};

describe('useOverlayScrollbar', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('hides the rail when the content fits', () => {
    const { rail } = setup({
      scrollHeight: 400,
      clientHeight: 500,
      scrollTop: 0,
    });

    expect(rail.hidden).toBe(true);
  });

  it('shows a rail that starts hidden once there is something to scroll', () => {
    // The rail renders hidden, and CSS keeps its box so that this measurement
    // still works — see .GoliathArticleScrollbar[hidden] in App.css. Without
    // that it would measure zero here and stay hidden forever.
    const { rail } = setup();

    expect(rail.hidden).toBe(false);
  });

  it('sizes the thumb in proportion to the visible fraction', () => {
    const { rail, thumb } = setup();

    expect(rail.hidden).toBe(false);
    expect(thumb.style.height).toBe('125px');
    expect(thumb.style.transform).toBe('translateY(0px)');
  });

  it('moves the thumb to the end of the track at the bottom of the article', () => {
    const { scroller, thumb, sync } = setup();

    stubGeometry(scroller, { scrollTop: 1500 });
    sync();

    // 375px of travel: the full track less the thumb's own height.
    expect(thumb.style.transform).toBe('translateY(375px)');
  });

  it('clamps the thumb to a grabbable minimum on very long articles', () => {
    const { thumb } = setup({
      scrollHeight: 100000,
      clientHeight: 500,
      scrollTop: 0,
    });

    // The proportional height would be 2.5px.
    expect(thumb.style.height).toBe('24px');
  });

  it('keeps the thumb in the track when overscrolled past the end', () => {
    const { scroller, thumb, sync } = setup();

    stubGeometry(scroller, { scrollTop: 1800 });
    sync();

    expect(thumb.style.transform).toBe('translateY(375px)');
  });

  it('scrolls the container when the thumb is dragged', () => {
    const { scroller, thumb } = setup();
    stubGeometry(thumb, { offsetHeight: 125 });

    fireEvent.pointerDown(thumb, { clientY: 0, pointerId: 1 });
    fireEvent.pointerMove(thumb, { clientY: 75, pointerId: 1 });

    // A fifth of the 375px of travel is a fifth of the 1500px of overflow.
    expect(scroller.scrollTop).toBe(300);
  });

  it('stops following the pointer once the drag is released', () => {
    const { scroller, thumb } = setup();
    stubGeometry(thumb, { offsetHeight: 125 });

    fireEvent.pointerDown(thumb, { clientY: 0, pointerId: 1 });
    fireEvent.pointerMove(thumb, { clientY: 75, pointerId: 1 });
    fireEvent.pointerUp(thumb, { pointerId: 1 });
    fireEvent.pointerMove(thumb, { clientY: 375, pointerId: 1 });

    expect(scroller.scrollTop).toBe(300);
  });

  it('pages towards a press on the bare rail', () => {
    const { scroller, rail, thumb } = setup();
    thumb.getBoundingClientRect = vi.fn(() => ({ top: 100 }) as DOMRect);

    fireEvent.pointerDown(rail, { clientY: 400 });
    expect(scroller.scrollBy).toHaveBeenCalledWith({
      top: 500,
      behavior: 'smooth',
    });

    fireEvent.pointerDown(rail, { clientY: 20 });
    expect(scroller.scrollBy).toHaveBeenCalledWith({
      top: -500,
      behavior: 'smooth',
    });
  });

  it('redraws the thumb when the container is scrolled', async () => {
    const { scroller, thumb } = setup();

    stubGeometry(scroller, { scrollTop: 750 });
    fireEvent.scroll(scroller);

    // The scroll handler defers to the next frame.
    await waitFor(() =>
      expect(thumb.style.transform).toBe('translateY(187.5px)')
    );
  });
});
