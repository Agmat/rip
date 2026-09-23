// Run: node src/lib/session.check.mjs
import assert from "node:assert/strict";
import { addPack, EMPTY_SESSION } from "./session.ts";

let s = EMPTY_SESSION;
s = addPack(s, { pack_price_eur: 4.5, total_value_eur: 1.1, unpriced_cards: 0, priced_at: null });
s = addPack(s, { pack_price_eur: 4.5, total_value_eur: 10.2, unpriced_cards: 0, priced_at: null });
s = addPack(s, { pack_price_eur: null, total_value_eur: 99, unpriced_cards: 0, priced_at: null });
assert.deepEqual(s, { packs: 3, unpriced: 1, spent: 9, pulled: 11.3 });
assert.equal(EMPTY_SESSION.packs, 0, "addPack must not mutate");
console.log("session ok");
