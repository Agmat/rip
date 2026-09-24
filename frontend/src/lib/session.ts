import type { PackOpen, Pricing } from "./types";

// Running P/L across every pack ripped since the user last hit reset.
// spent/pulled only cover packs with a Cardmarket pack price, so the net
// stays honest; packs without one are counted in `unpriced` instead.
export type Session = {
  packs: number;
  unpriced: number;
  spent: number;
  pulled: number;
};

export const EMPTY_SESSION: Session = { packs: 0, unpriced: 0, spent: 0, pulled: 0 };

const KEY = "rip.session";

// Round to cents on every add so float drift doesn't creep into the totals.
const cents = (n: number) => Math.round(n * 100) / 100;

export function addPack(s: Session, p: Pricing): Session {
  if (p.pack_price_eur == null) return { ...s, packs: s.packs + 1, unpriced: s.unpriced + 1 };
  return {
    ...s,
    packs: s.packs + 1,
    spent: cents(s.spent + p.pack_price_eur),
    pulled: cents(s.pulled + p.total_value_eur),
  };
}

// Storage can be missing (SSR) or throw (private mode, blocked site data):
// fall back to an empty session rather than breaking the page.
export function loadSession(): Session {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? { ...EMPTY_SESSION, ...JSON.parse(raw) } : EMPTY_SESSION;
  } catch {
    return EMPTY_SESSION;
  }
}

export function saveSession(s: Session) {
  try {
    localStorage.setItem(KEY, JSON.stringify(s));
  } catch {}
}

// The last few packs ripped, newest first, for the history drawer. `n` is the
// pack's number within the session ("Pack #39").
export type HistoryEntry = { n: number; pack: PackOpen };

export const HISTORY_SIZE = 10;

const HISTORY_KEY = "rip.history";

export function pushHistory(h: HistoryEntry[], e: HistoryEntry): HistoryEntry[] {
  return [e, ...h].slice(0, HISTORY_SIZE);
}

export function loadHistory(): HistoryEntry[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

export function saveHistory(h: HistoryEntry[]) {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(h));
  } catch {}
}
