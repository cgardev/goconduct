import { formatRelativeTime } from './relative-time';

const NOW = new Date('2026-08-22T12:00:00.000Z');

function ago(milliseconds: number): Date {
  return new Date(NOW.getTime() - milliseconds);
}

describe('formatRelativeTime', () => {
  it('reports anything under one minute as now', () => {
    expect(formatRelativeTime(ago(0), NOW)).toBe('Now');
    expect(formatRelativeTime(ago(59_000), NOW)).toBe('Now');
  });

  it('writes the singular unit without a plural mark', () => {
    expect(formatRelativeTime(ago(60_000), NOW)).toBe('1 minute ago');
    expect(formatRelativeTime(ago(3_600_000), NOW)).toBe('1 hour ago');
  });

  it('grows the unit with the distance', () => {
    expect(formatRelativeTime(ago(5 * 60_000), NOW)).toBe('5 minutes ago');
    expect(formatRelativeTime(ago(3 * 3_600_000), NOW)).toBe('3 hours ago');
    expect(formatRelativeTime(ago(2 * 86_400_000), NOW)).toBe('2 days ago');
    expect(formatRelativeTime(ago(14 * 86_400_000), NOW)).toBe('2 weeks ago');
    expect(formatRelativeTime(ago(90 * 86_400_000), NOW)).toBe('3 months ago');
    expect(formatRelativeTime(ago(800 * 86_400_000), NOW)).toBe('2 years ago');
  });

  /**
   * A clock that runs behind the server would otherwise report a future
   * instant as a negative distance, which reads as a defect of the console.
   */
  it('reports an instant in the future as now', () => {
    expect(formatRelativeTime(ago(-5 * 60_000), NOW)).toBe('Now');
  });
});
