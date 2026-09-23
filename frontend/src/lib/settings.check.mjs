// Run: node src/lib/settings.check.mjs
import assert from "node:assert/strict";
import { DEFAULT_SETTINGS, isDimmed } from "./settings.ts";

const on = { ...DEFAULT_SETTINGS, dimEnabled: true };
assert.equal(isDimmed(DEFAULT_SETTINGS, 0.1), false, "off by default");
assert.equal(isDimmed(on, 0.99), true);
assert.equal(isDimmed(on, 1), false, "threshold itself is not dimmed");
assert.equal(isDimmed(on, null), false, "unpriced never dimmed");
console.log("settings ok");
