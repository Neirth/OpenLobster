// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { formatChatTime } from './formatChatTime';

/**
 * We freeze time to a known instant so tests are deterministic.
 * All date arithmetic uses the same logic as the implementation:
 * "start of today" is midnight in the local timezone.
 */

// Fix "now" to 14:00 local time on a known date.
// We compute the frozen timestamp in local-time terms so the
// same-day / yesterday / same-week boundaries are predictable.
function makeLocalNoon(year: number, month: number /* 1-based */, day: number): Date {
  return new Date(year, month - 1, day, 14, 0, 0, 0);
}

const FIXED_NOW = makeLocalNoon(2026, 3, 22); // Sunday 22 Mar 2026, 14:00 local

function isoAt(date: Date): string {
  return date.toISOString();
}

/** Returns an ISO string for a point that is `ms` milliseconds before FIXED_NOW. */
function msAgo(ms: number): string {
  return isoAt(new Date(FIXED_NOW.getTime() - ms));
}

const ONE_HOUR = 3_600_000;
const ONE_DAY = 86_400_000;

describe('formatChatTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(FIXED_NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // ------------------------------------------------------------------ //
  // Same-day branch (both modes)                                        //
  // ------------------------------------------------------------------ //

  // Locale-agnostic time pattern: matches both 24h ("13:00") and 12h ("1:00 PM")
  const TIME_RE = /\d{1,2}:\d{2}/;

  it('returns a time string for a message from 1 hour ago (default mode)', () => {
    const result = formatChatTime(msAgo(ONE_HOUR));
    expect(result).toMatch(TIME_RE);
  });

  it('returns a time string for a message from 1 hour ago (withTime=true)', () => {
    const result = formatChatTime(msAgo(ONE_HOUR), true);
    expect(result).toMatch(TIME_RE);
  });

  it('returns a time string for a message sent at exactly midnight today', () => {
    const midnight = new Date(FIXED_NOW.getFullYear(), FIXED_NOW.getMonth(), FIXED_NOW.getDate(), 0, 0, 0, 0);
    const result = formatChatTime(isoAt(midnight));
    expect(result).toMatch(TIME_RE);
  });

  it('returns a time string for a message sent 1 minute ago', () => {
    const result = formatChatTime(msAgo(60_000));
    expect(result).toMatch(TIME_RE);
  });

  // ------------------------------------------------------------------ //
  // withTime=true (thread mode) — past messages                        //
  // ------------------------------------------------------------------ //

  it('withTime=true, yesterday: returns "date, HH:MM" (contains comma)', () => {
    // 1.5 days ago is clearly in "yesterday" or earlier
    const result = formatChatTime(msAgo(1.5 * ONE_DAY), true);
    expect(result).toContain(',');
  });

  it('withTime=true, same week: returns "date, HH:MM"', () => {
    const result = formatChatTime(msAgo(3 * ONE_DAY), true);
    expect(result).toContain(',');
  });

  it('withTime=true, older same year: no year in output', () => {
    // 30 days ago is still 2026 — FIXED_NOW is 22 Mar 2026
    const thirtyDaysAgo = new Date(FIXED_NOW.getTime() - 30 * ONE_DAY);
    const result = formatChatTime(isoAt(thirtyDaysAgo), true);
    expect(result).toContain(',');
    expect(result).not.toContain('2026');
  });

  it('withTime=true, different year: includes year', () => {
    const pastYear = new Date(2025, 0, 15, 10, 0, 0, 0); // 15 Jan 2025
    const result = formatChatTime(isoAt(pastYear), true);
    expect(result).toContain(',');
    expect(result).toContain('2025');
  });

  // ------------------------------------------------------------------ //
  // withTime=false (sidebar mode) — relative labels                    //
  // ------------------------------------------------------------------ //

  it('withTime=false, yesterday: returns non-empty relative label (no comma)', () => {
    // 30 hours ago is definitely yesterday
    const result = formatChatTime(msAgo(30 * ONE_HOUR), false);
    expect(result.length).toBeGreaterThan(0);
    expect(result).not.toContain(',');
  });

  it('withTime=false, 2 days ago: returns short weekday (no comma)', () => {
    const result = formatChatTime(msAgo(2 * ONE_DAY), false);
    expect(result.length).toBeGreaterThan(0);
    expect(result).not.toContain(',');
  });

  it('withTime=false, 5 days ago (within same-week window): short weekday', () => {
    // 5 days back is within the 6-day window
    const result = formatChatTime(msAgo(5 * ONE_DAY), false);
    expect(result.length).toBeGreaterThan(0);
    expect(result).not.toContain(',');
  });

  it('withTime=false, older than one week (same year): short date without year', () => {
    // 8 days ago is 14 Mar 2026 — same year, outside the same-week window
    const result = formatChatTime(msAgo(8 * ONE_DAY), false);
    expect(result.length).toBeGreaterThan(0);
    expect(result).not.toContain('2026');
  });

  it('withTime=false, different year: includes year in output', () => {
    const pastYear = new Date(2025, 0, 15, 10, 0, 0, 0); // 15 Jan 2025
    const result = formatChatTime(isoAt(pastYear), false);
    expect(result).toContain('2025');
  });

  // ------------------------------------------------------------------ //
  // Default parameter                                                   //
  // ------------------------------------------------------------------ //

  it('default (withTime omitted) behaves identically to withTime=false', () => {
    const ts = msAgo(8 * ONE_DAY);
    expect(formatChatTime(ts)).toBe(formatChatTime(ts, false));
  });

  // ------------------------------------------------------------------ //
  // Robustness                                                          //
  // ------------------------------------------------------------------ //

  it('does not throw for any of the date-range branches', () => {
    const timestamps = [
      msAgo(ONE_HOUR),           // same-day
      msAgo(30 * ONE_HOUR),      // yesterday
      msAgo(3 * ONE_DAY),        // same-week
      msAgo(8 * ONE_DAY),        // older, same year
      isoAt(new Date(2025, 0, 1, 0, 0, 0, 0)), // different year
    ];
    for (const ts of timestamps) {
      expect(() => formatChatTime(ts)).not.toThrow();
      expect(() => formatChatTime(ts, true)).not.toThrow();
    }
  });
});
