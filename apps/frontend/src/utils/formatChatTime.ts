// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Returns an idiomatic label for a message timestamp.
 *
 * Thread mode (withTime=true) — always includes the clock time:
 *   - Same day  → "13:13"
 *   - Other day → "21 mar, 13:13"  (+ year when different)
 *
 * Sidebar mode (withTime=false) — relative label, no clock time:
 *   - Same day  → "13:13"
 *   - Yesterday → "ayer" / "yesterday" / locale equivalent
 *   - Same week → short weekday, e.g. "lun."
 *   - Older     → short date, e.g. "12 mar"  (+ year when different)
 */
export function formatChatTime(isoString: string, withTime = false): string {
  const date = new Date(isoString);
  const now = new Date();
  const loc = (typeof navigator !== 'undefined' ? navigator.language : undefined) ?? 'en';
  const timeStr = date.toLocaleTimeString(loc, { hour: '2-digit', minute: '2-digit' });

  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());

  if (date >= startOfToday) {
    return timeStr;
  }

  if (withTime) {
    // Thread: date + time in a single compact string — no awkward relative label.
    const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' };
    if (date.getFullYear() !== now.getFullYear()) opts.year = 'numeric';
    return `${date.toLocaleDateString(loc, opts)}, ${timeStr}`;
  }

  // Sidebar: relative label without time.
  const startOfYesterday = new Date(startOfToday.getTime() - 86_400_000);
  const startOfWeek = new Date(startOfToday.getTime() - 6 * 86_400_000);

  if (date >= startOfYesterday) {
    return new Intl.RelativeTimeFormat(loc, { numeric: 'auto' }).format(-1, 'day');
  }

  if (date >= startOfWeek) {
    return date.toLocaleDateString(loc, { weekday: 'short' });
  }

  const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' };
  if (date.getFullYear() !== now.getFullYear()) opts.year = 'numeric';
  return date.toLocaleDateString(loc, opts);
}
