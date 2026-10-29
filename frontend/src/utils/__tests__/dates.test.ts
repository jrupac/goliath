import { describe, expect, it } from 'vitest';
import { formatFriendly, formatFull } from '../dates';

describe('formatFriendly', () => {
  const ago = (ms: number) => new Date(Date.now() - ms);
  const SECOND = 1000;
  const MINUTE = 60 * SECOND;
  const HOUR = 60 * MINUTE;
  const DAY = 24 * HOUR;

  it('formats dates within 24 hours relatively', () => {
    expect(formatFriendly(ago(2 * HOUR))).toBe('2 hours ago');
  });

  it.each([
    [10 * SECOND, '10 seconds ago'],
    [44 * SECOND, '44 seconds ago'],
    [45 * SECOND, '1 minute ago'],
    [44 * MINUTE, '44 minutes ago'],
    [45 * MINUTE, '1 hour ago'],
    [21 * HOUR, '21 hours ago'],
    [22 * HOUR, 'yesterday'],
  ])('rounds %d ms up to the unit a reader would use', (elapsed, expected) => {
    expect(formatFriendly(ago(elapsed))).toBe(expected);
  });

  it('gives a date from another year its year and no time', () => {
    const date = new Date();
    date.setFullYear(date.getFullYear() - 2);
    expect(formatFriendly(date)).toMatch(/^[A-Z][a-z]{2} \d{1,2}, \d{4}$/);
  });

  it('gives a date from this year its weekday and time', () => {
    // Stepping back from today can cross into last year, which is formatted
    // differently, so pick whichever direction stays inside this year.
    let date = ago(2 * DAY);
    if (date.getFullYear() !== new Date().getFullYear()) {
      date = new Date(Date.now() + 2 * DAY);
    }
    const formatted = formatFriendly(date);
    expect(formatted).toContain(',');
    expect(formatted).toMatch(/(AM|PM)$/);
  });

  it('renders a future date absolutely rather than counting backwards', () => {
    const formatted = formatFriendly(new Date(Date.now() + HOUR));
    expect(formatted).not.toMatch(/ago|in /);
  });

  it('reports a date it cannot read instead of throwing', () => {
    expect(formatFriendly(new Date(NaN))).toBe('Invalid date');
  });

  it('counts a unit from the elapsed time, not from the unit below it', () => {
    // Just under ten and a half hours: rounding to minutes first carries it
    // over the half-hour and reports an hour too many.
    expect(formatFriendly(ago(10.4958 * HOUR))).toBe('10 hours ago');
  });

  it.each([
    [59.7 * MINUTE, '1 hour ago'],
    [23.9 * HOUR, 'yesterday'],
  ])('rounds %d ms up rather than to a unit ceiling', (elapsed, expected) => {
    // Rounding within the unit would reach its own ceiling and say "60
    // minutes ago" or "24 hours ago", which is what the thresholds prevent.
    expect(formatFriendly(ago(elapsed))).toBe(expected);
  });
});

describe('formatFull', () => {
  it('spells a timestamp out in full', () => {
    expect(formatFull(new Date(2026, 9, 26, 9, 7, 5))).toBe(
      'Monday, October 26, 2026 at 9:07:05 AM'
    );
  });

  it('reports a date it cannot read instead of throwing', () => {
    expect(formatFull(new Date(NaN))).toBe('Invalid date');
  });
});
