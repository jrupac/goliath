import { useRef } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { useSwipeNavigate } from '../useSwipeNavigate';

// The hook reads only `touches` off the event and may call preventDefault, so
// the tests hand it plain events carrying that one property. A constructed
// TouchEvent needs a Touch constructor, which the test DOM does not have.
const touch = (type: string, x: number, y: number): Event => {
  const event = new Event(type, { bubbles: true, cancelable: true });
  const points = type === 'touchend' ? [] : [{ clientX: x, clientY: y }];
  Object.defineProperty(event, 'touches', { value: points });
  return event;
};

const stub = (el: Element, props: Record<string, number>) => {
  for (const [name, value] of Object.entries(props)) {
    Object.defineProperty(el, name, { value, configurable: true });
  }
};

interface HarnessProps {
  enabled?: boolean;
  onBack?: () => void;
  onNext?: () => void;
  onPrevious?: () => void;
}

const Harness = ({ enabled = true, ...handlers }: HarnessProps) => {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const paneRef = useRef<HTMLDivElement | null>(null);
  const peekRef = useRef<HTMLDivElement | null>(null);
  const { direction, armed } = useSwipeNavigate(scrollRef, paneRef, peekRef, {
    enabled,
    ...handlers,
  });

  return (
    <div data-testid="pane" ref={paneRef}>
      <div data-testid="scroller" ref={scrollRef} />
      <div data-testid="peek" ref={peekRef} />
      <span data-testid="state">{`${direction}:${armed}`}</span>
    </div>
  );
};

// An article twice the height of its pane, parked at the bottom, in a pane
// 300px wide. A pull up from here is an overscroll and so is a gesture.
const setup = (
  handlers: HarnessProps,
  scroll: Record<string, number> = {
    scrollTop: 400,
    scrollHeight: 800,
    clientHeight: 400,
  }
) => {
  render(<Harness {...handlers} />);
  const scroller = screen.getByTestId('scroller');
  stub(scroller, scroll);
  stub(screen.getByTestId('pane'), { clientWidth: 300 });
  return {
    scroller,
    pane: screen.getByTestId('pane'),
    peek: screen.getByTestId('peek'),
    state: () => screen.getByTestId('state').textContent,
  };
};

const drag = (
  el: Element,
  from: [number, number],
  points: [number, number][]
) => {
  act(() => {
    el.dispatchEvent(touch('touchstart', from[0], from[1]));
    for (const [x, y] of points) {
      el.dispatchEvent(touch('touchmove', x, y));
    }
    el.dispatchEvent(touch('touchend', 0, 0));
  });
};

describe('useSwipeNavigate', () => {
  it('goes on to the next article when the pull runs past the end', () => {
    const onNext = vi.fn();
    const { scroller } = setup({ onNext });

    drag(
      scroller,
      [100, 300],
      [
        [100, 280],
        [100, 200],
        [100, 120],
      ]
    );

    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it('leaves a short pull alone', () => {
    const onNext = vi.fn();
    const { scroller, peek } = setup({ onNext });

    drag(
      scroller,
      [100, 300],
      [
        [100, 280],
        [100, 265],
      ]
    );

    expect(onNext).not.toHaveBeenCalled();
    expect(peek.style.height).toBe('0px');
  });

  it('goes back to the previous article from the top of the article', () => {
    const onPrevious = vi.fn();
    const onNext = vi.fn();
    const { scroller } = setup(
      { onNext, onPrevious },
      { scrollTop: 0, scrollHeight: 800, clientHeight: 400 }
    );

    drag(
      scroller,
      [100, 200],
      [
        [100, 220],
        [100, 300],
        [100, 380],
      ]
    );

    expect(onPrevious).toHaveBeenCalledTimes(1);
    expect(onNext).not.toHaveBeenCalled();
  });

  it('does not pull where the article still has somewhere to scroll', () => {
    const onNext = vi.fn();
    const onPrevious = vi.fn();
    const { scroller } = setup(
      { onNext, onPrevious },
      { scrollTop: 100, scrollHeight: 800, clientHeight: 400 }
    );

    drag(
      scroller,
      [100, 300],
      [
        [100, 260],
        [100, 150],
        [100, 60],
      ]
    );

    expect(onNext).not.toHaveBeenCalled();
    expect(onPrevious).not.toHaveBeenCalled();
  });

  it('carries the pane sideways and goes back past a third of it', () => {
    const onBack = vi.fn();
    const { scroller, pane } = setup({ onBack });

    drag(
      scroller,
      [20, 300],
      [
        [50, 300],
        [120, 302],
        [180, 304],
      ]
    );

    expect(onBack).toHaveBeenCalledTimes(1);
    // The pane is put back for whatever is rendered next.
    expect(pane.style.transform).toBe('');
  });

  it('keeps a sideways gesture that does not reach the third', () => {
    const onBack = vi.fn();
    const { scroller } = setup({ onBack });

    drag(
      scroller,
      [20, 300],
      [
        [50, 300],
        [80, 302],
      ]
    );

    expect(onBack).not.toHaveBeenCalled();
  });

  it('reads a mostly vertical drag as a scroll, not as going back', () => {
    const onBack = vi.fn();
    const { scroller } = setup(
      { onBack },
      { scrollTop: 100, scrollHeight: 800, clientHeight: 400 }
    );

    drag(
      scroller,
      [20, 300],
      [
        [40, 260],
        [120, 150],
        [200, 60],
      ]
    );

    expect(onBack).not.toHaveBeenCalled();
  });

  it('does nothing at all where the layout shows both panes', () => {
    const onBack = vi.fn();
    const onNext = vi.fn();
    const { scroller } = setup({ enabled: false, onBack, onNext });

    drag(
      scroller,
      [20, 300],
      [
        [120, 300],
        [260, 300],
      ]
    );
    drag(
      scroller,
      [100, 300],
      [
        [100, 200],
        [100, 100],
      ]
    );

    expect(onBack).not.toHaveBeenCalled();
    expect(onNext).not.toHaveBeenCalled();
  });
});
