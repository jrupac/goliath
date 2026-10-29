// Dates are rendered in English whatever the browser is set to, so that what
// a timestamp reads as does not depend on where it is being read. Formatters
// are built once: constructing one is far more expensive than using it, and
// these run for every article in a list.
const DATE_LOCALE = 'en-US';

const fullFormat = new Intl.DateTimeFormat(DATE_LOCALE, {
  dateStyle: 'full',
  timeStyle: 'medium',
});
const thisYearFormat = new Intl.DateTimeFormat(DATE_LOCALE, {
  weekday: 'short',
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
  hour12: true,
});
const otherYearFormat = new Intl.DateTimeFormat(DATE_LOCALE, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
});

// Wording a span as a phrase -- which unit to pluralize, where "ago" goes,
// and that a single day back reads as "yesterday" -- belongs to the language,
// so it is left to the platform. What is left here is choosing which unit to
// ask for and how many of it, which no formatter decides.
const relativeFormat = new Intl.RelativeTimeFormat(DATE_LOCALE, {
  numeric: 'auto',
});

const SECOND_MS = 1000;
const MINUTE_MS = 60 * SECOND_MS;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

// Counts at which a span stops being reported in one unit and rounds up to
// the next, so that a timestamp reads as the unit a reader would reach for:
// most of an hour is better said in hours than as fifty-odd minutes. They
// also keep rounding from reporting a unit's own ceiling back to itself, as
// "60 minutes ago" or "24 hours ago". These are deliberately not the
// conversion factors between the units.
const ROUND_UP_AT_SECONDS = 45;
const ROUND_UP_AT_MINUTES = 45;
const ROUND_UP_AT_HOURS = 22;

function formatRelative(elapsedMs: number): string {
  // Each unit is rounded from the elapsed time itself rather than from the
  // unit below it, since rounding twice can carry a value across a boundary
  // and report the wrong count.
  const seconds = Math.round(elapsedMs / SECOND_MS);
  if (seconds < ROUND_UP_AT_SECONDS) {
    return relativeFormat.format(-seconds, 'second');
  }
  const minutes = Math.round(elapsedMs / MINUTE_MS);
  if (minutes < ROUND_UP_AT_MINUTES) {
    return relativeFormat.format(-Math.max(minutes, 1), 'minute');
  }
  const hours = Math.round(elapsedMs / HOUR_MS);
  if (hours < ROUND_UP_AT_HOURS) {
    return relativeFormat.format(-Math.max(hours, 1), 'hour');
  }
  return relativeFormat.format(-1, 'day');
}

// What a feed supplies as a timestamp may not be a usable date at all, and
// arrives here as an invalid Date. Formatting one throws, which would take
// down the article being rendered, so it is reported in place instead.
const INVALID_DATE = 'Invalid date';

function isValidDate(date: Date): boolean {
  return !Number.isNaN(date.getTime());
}

/** Spells a timestamp out in full, for somewhere a reader has to ask. */
export function formatFull(date: Date): string {
  if (!isValidDate(date)) {
    return INVALID_DATE;
  }
  return fullFormat.format(date);
}

/**
 * Renders a timestamp the way a reader scanning a list wants it: how long
 * ago, while that is still a useful thing to say, and a calendar date once
 * it is not.
 */
export function formatFriendly(date: Date): string {
  if (!isValidDate(date)) {
    return INVALID_DATE;
  }
  const elapsedMs = Date.now() - date.getTime();
  // A date in the future, which a publisher can supply, gets an absolute
  // rendering rather than a relative one counting backwards.
  if (elapsedMs > 0 && elapsedMs < DAY_MS) {
    return formatRelative(elapsedMs);
  }
  if (date.getFullYear() !== new Date().getFullYear()) {
    return otherYearFormat.format(date);
  }
  return thisYearFormat.format(date);
}
