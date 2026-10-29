import { keybindRegistry } from '../keybindRegistry';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent } from '@testing-library/react';

describe('KeybindRegistry Collision Handling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    // Unregister test handlers to clean up window event listeners
    keybindRegistry.unregister('test');
    vi.useRealTimers();
  });

  it('handles standard sequence "g s" and standalone "s" correctly', () => {
    const mockGoSaved = vi.fn();
    const mockToggleSave = vi.fn();

    keybindRegistry.register('test', {
      'g s': mockGoSaved,
      s: mockToggleSave,
    });

    // Press 'g' then 's' quickly
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's', code: 'KeyS' });

    expect(mockGoSaved).toHaveBeenCalledTimes(1);
    expect(mockToggleSave).not.toHaveBeenCalled();

    // Press 's' alone
    mockGoSaved.mockClear();
    mockToggleSave.mockClear();

    vi.advanceTimersByTime(2000); // clear any history window
    fireEvent.keyDown(window, { key: 's', code: 'KeyS' });

    expect(mockToggleSave).toHaveBeenCalledTimes(1);
    expect(mockGoSaved).not.toHaveBeenCalled();
  });

  it('handles sequences sharing a prefix alongside their standalone suffixes', () => {
    const mockGoUnread = vi.fn();
    const mockGoSaved = vi.fn();
    const mockU = vi.fn();
    const mockS = vi.fn();

    keybindRegistry.register('test', {
      'g u': mockGoUnread,
      'g s': mockGoSaved,
      u: mockU,
      s: mockS,
    });

    // A suffix key on its own is unambiguous and fires.
    fireEvent.keyDown(window, { key: 'u', code: 'KeyU' });
    expect(mockU).toHaveBeenCalledTimes(1);
    expect(mockGoUnread).not.toHaveBeenCalled();
    mockU.mockClear();

    // Reached through the shared prefix, the sequence wins and the standalone
    // suffix stays silent.
    vi.advanceTimersByTime(2000);
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'u', code: 'KeyU' });
    expect(mockGoUnread).toHaveBeenCalledTimes(1);
    expect(mockU).not.toHaveBeenCalled();
    mockGoUnread.mockClear();

    // The other branch of the same prefix behaves the same way.
    vi.advanceTimersByTime(2000);
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's', code: 'KeyS' });
    expect(mockGoSaved).toHaveBeenCalledTimes(1);
    expect(mockS).not.toHaveBeenCalled();
    expect(mockGoUnread).not.toHaveBeenCalled();
  });

  it('handles sequences with more than 2 keys (e.g., "g o s" and suffix "o s" and standalone "s")', () => {
    const mockGoOS = vi.fn();
    const mockOS = vi.fn();
    const mockS = vi.fn();

    keybindRegistry.register('test', {
      'g o s': mockGoOS,
      'o s': mockOS,
      s: mockS,
    });

    // Press 'g' -> 'o' -> 's' quickly
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'o', code: 'KeyO' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's', code: 'KeyS' });

    expect(mockGoOS).toHaveBeenCalledTimes(1);
    expect(mockOS).not.toHaveBeenCalled();
    expect(mockS).not.toHaveBeenCalled();
  });

  it('handles repeated keys in sequence like "g g" vs standalone "g"', () => {
    const mockGG = vi.fn();
    const mockG = vi.fn();

    keybindRegistry.register('test', {
      'g g': mockGG,
      g: mockG,
    });

    // The first press is ambiguous between the two, so neither fires.
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    expect(mockG).not.toHaveBeenCalled();
    expect(mockGG).not.toHaveBeenCalled();

    // Press 'g' again quickly
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    expect(mockGG).toHaveBeenCalledTimes(1);
    expect(mockG).not.toHaveBeenCalled();
  });

  it('never fires a key that is the opening press of a longer sequence', () => {
    const mockGG = vi.fn();
    const mockG = vi.fn();

    keybindRegistry.register('test', {
      'g g': mockGG,
      g: mockG,
    });

    // Every press of 'g' opens 'g g' afresh, so the ambiguity returns rather
    // than lapsing: waiting out the sequence timeout does not release the
    // standalone binding. A key meant to act on its own cannot also be the
    // first press of a sequence.
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(2000);
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    vi.advanceTimersByTime(2000);
    fireEvent.keyDown(window, { key: 'g', code: 'KeyG' });
    expect(mockG).not.toHaveBeenCalled();
    expect(mockGG).not.toHaveBeenCalled();
  });

  it('keeps firing while a key is held down', () => {
    const mockNext = vi.fn();

    keybindRegistry.register('test', { j: mockNext });

    fireEvent.keyDown(window, { key: 'j', code: 'KeyJ' });
    fireEvent.keyDown(window, { key: 'j', code: 'KeyJ', repeat: true });
    fireEvent.keyDown(window, { key: 'j', code: 'KeyJ', repeat: true });

    expect(mockNext).toHaveBeenCalledTimes(3);
  });

  it('ignores a keystroke typed into a form field', () => {
    const mockToggle = vi.fn();

    keybindRegistry.register('test', { t: mockToggle });

    const input = document.createElement('input');
    document.body.appendChild(input);
    try {
      fireEvent.keyDown(input, { key: 't', code: 'KeyT' });
      expect(mockToggle).not.toHaveBeenCalled();
    } finally {
      input.remove();
    }
  });
});
