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
    fireEvent.keyDown(window, { key: 'g' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's' });

    expect(mockGoSaved).toHaveBeenCalledTimes(1);
    expect(mockToggleSave).not.toHaveBeenCalled();

    // Press 's' alone
    mockGoSaved.mockClear();
    mockToggleSave.mockClear();

    vi.advanceTimersByTime(2000); // clear any history window
    fireEvent.keyDown(window, { key: 's' });

    expect(mockToggleSave).toHaveBeenCalledTimes(1);
    expect(mockGoSaved).not.toHaveBeenCalled();
  });

  it('handles dual sequences "g u" and "u g" with standalone "u" and "g"', () => {
    const mockGoUnread = vi.fn();
    const mockGoG = vi.fn();
    const mockU = vi.fn();
    const mockG = vi.fn();

    keybindRegistry.register('test', {
      'g u': mockGoUnread,
      'u g': mockGoG,
      u: mockU,
      g: mockG,
    });

    // Pressing 'g u g u' sequence
    // 1. 'g'
    fireEvent.keyDown(window, { key: 'g' });
    expect(mockG).toHaveBeenCalledTimes(1);
    expect(mockGoUnread).not.toHaveBeenCalled();
    expect(mockGoG).not.toHaveBeenCalled();
    expect(mockU).not.toHaveBeenCalled();

    mockG.mockClear();

    // 2. 'u' (triggers 'g u', blocks standalone 'u')
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'u' });
    expect(mockGoUnread).toHaveBeenCalledTimes(1);
    expect(mockU).not.toHaveBeenCalled();
    expect(mockG).not.toHaveBeenCalled();
    expect(mockGoG).not.toHaveBeenCalled();

    mockGoUnread.mockClear();

    // 3. 'g' (triggers 'u g', blocks standalone 'g')
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'g' });
    expect(mockGoG).toHaveBeenCalledTimes(1);
    expect(mockG).not.toHaveBeenCalled();
    expect(mockU).not.toHaveBeenCalled();
    expect(mockGoUnread).not.toHaveBeenCalled();

    mockGoG.mockClear();

    // 4. 'u' (triggers 'g u', blocks standalone 'u')
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'u' });
    expect(mockGoUnread).toHaveBeenCalledTimes(1);
    expect(mockU).not.toHaveBeenCalled();
    expect(mockG).not.toHaveBeenCalled();
    expect(mockGoG).not.toHaveBeenCalled();
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
    fireEvent.keyDown(window, { key: 'g' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'o' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's' });

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

    // Press 'g' once
    fireEvent.keyDown(window, { key: 'g' });
    expect(mockG).toHaveBeenCalledTimes(1);
    expect(mockGG).not.toHaveBeenCalled();

    mockG.mockClear();

    // Press 'g' again quickly
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 'g' });
    expect(mockGG).toHaveBeenCalledTimes(1);
    expect(mockG).not.toHaveBeenCalled();
  });
});
