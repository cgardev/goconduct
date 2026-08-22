/** One second, in milliseconds. */
const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

/**
 * Formats an instant as a relative timestamp, such as `5 minutes ago`.
 *
 * A relative timestamp reads faster than an absolute one for a recent event,
 * which is what the analysis state always is. The absolute value stays
 * available as the `title` of the rendered `<time>` element, for a reader who
 * needs the exact instant.
 *
 * The unit grows with the distance, so the value never reports more precision
 * than it has: an analysis from two hours ago is not `120 minutes ago`.
 *
 * @param instant - When the event happened.
 * @param now - The instant to measure against.
 * @returns The distance in words, or `Now` under one minute.
 */
export function formatRelativeTime(instant: Date, now: Date): string {
  const elapsed = Math.max(0, now.getTime() - instant.getTime());

  if (elapsed < MINUTE) {
    return 'Now';
  }
  if (elapsed < HOUR) {
    return pluralize(Math.floor(elapsed / MINUTE), 'minute');
  }
  if (elapsed < DAY) {
    return pluralize(Math.floor(elapsed / HOUR), 'hour');
  }
  if (elapsed < WEEK) {
    return pluralize(Math.floor(elapsed / DAY), 'day');
  }
  if (elapsed < MONTH) {
    return pluralize(Math.floor(elapsed / WEEK), 'week');
  }
  if (elapsed < YEAR) {
    return pluralize(Math.floor(elapsed / MONTH), 'month');
  }
  return pluralize(Math.floor(elapsed / YEAR), 'year');
}

// pluralize writes the unit in full, because an abbreviated unit reads worse
// and translates worse than the whole word.
function pluralize(count: number, unit: string): string {
  return `${count} ${unit}${count === 1 ? '' : 's'} ago`;
}
