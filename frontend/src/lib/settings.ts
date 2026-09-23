// User preferences, kept in localStorage like the session.
export type Settings = {
  dimEnabled: boolean;
  dimBelow: number; // EUR
};

export const DEFAULT_SETTINGS: Settings = { dimEnabled: false, dimBelow: 1 };

const KEY = "rip.settings";

// Unpriced cards are never dimmed: we don't know they're cheap.
export function isDimmed(s: Settings, price: number | null): boolean {
  return s.dimEnabled && price != null && price < s.dimBelow;
}

export function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? { ...DEFAULT_SETTINGS, ...JSON.parse(raw) } : DEFAULT_SETTINGS;
  } catch {
    return DEFAULT_SETTINGS;
  }
}

export function saveSettings(s: Settings) {
  try {
    localStorage.setItem(KEY, JSON.stringify(s));
  } catch {}
}
