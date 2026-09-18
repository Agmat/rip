"use client";

import { useEffect, useState, type ReactNode } from "react";
import { listSets, openPack } from "@/lib/api";
import { formatEUR } from "@/lib/money";
import type { PackOpen, Pricing, SetSummary } from "@/lib/types";
import CardTile from "./CardTile";
import Pack from "./Pack";

// Minimum time the tear animation gets to play, even if the API answers
// faster - so opening never feels like an instant cut.
const TEAR_MS = 650;

function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export default function PackOpener() {
  const [sets, setSets] = useState<SetSummary[] | null>(null);
  const [setsError, setSetsError] = useState<string | null>(null);
  const [index, setIndex] = useState(0);
  const [pack, setPack] = useState<PackOpen | null>(null);
  const [tearing, setTearing] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);

  useEffect(() => {
    listSets()
      .then((res) => setSets(res.sets))
      .catch((err: Error) => setSetsError(err.message));
  }, []);

  if (setsError) {
    return <p className="text-red-400">{setsError}</p>;
  }

  if (!sets) {
    return <p className="text-muted">loading sets…</p>;
  }

  if (sets.length === 0) {
    return <p className="text-muted">no sets imported yet</p>;
  }

  const selectedSet = sets[index];

  async function handleRip() {
    setOpenError(null);
    setTearing(true);
    try {
      const [result] = await Promise.all([openPack(selectedSet.code), wait(TEAR_MS)]);
      setPack(result);
    } catch (err) {
      setOpenError(err instanceof Error ? err.message : "failed to open pack");
    } finally {
      setTearing(false);
    }
  }

  function handleChangePack() {
    setPack(null);
    setOpenError(null);
  }

  if (pack) {
    return (
      <div className="flex flex-col items-center gap-6">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-5">
          {pack.cards.map((pick, i) => (
            <CardTile key={`${pick.slot}-${pick.card.id}`} pick={pick} index={i} />
          ))}
        </div>
        <PackValue pricing={pack.pricing} revealDelayMs={pack.cards.length * 80 + 400} />
        <div className="flex items-center gap-3">
          <button onClick={handleRip} disabled={tearing} className="rip-button">
            {tearing ? "ripping…" : "rip another"}
          </button>
          <button onClick={handleChangePack} className="pack-arrow text-sm">
            change pack
          </button>
        </div>
        {openError && <p className="text-sm text-red-400">Couldn&apos;t open the pack: {openError}</p>}
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-5">
      <div className="flex items-center gap-4">
        {sets.length > 1 && (
          <button
            onClick={() => setIndex((i) => (i - 1 + sets.length) % sets.length)}
            aria-label="Previous set"
            className="pack-arrow"
          >
            ‹
          </button>
        )}
        <Pack set={selectedSet} tearing={tearing} />
        {sets.length > 1 && (
          <button
            onClick={() => setIndex((i) => (i + 1) % sets.length)}
            aria-label="Next set"
            className="pack-arrow"
          >
            ›
          </button>
        )}
      </div>
      {selectedSet.pack_price_eur != null && (
        <p className="text-sm text-muted">{formatEUR(selectedSet.pack_price_eur)} on Cardmarket</p>
      )}
      <button onClick={handleRip} disabled={tearing} className="rip-button">
        {tearing ? "ripping…" : "rip pack"}
      </button>
      {openError && <p className="text-sm text-red-400">Couldn&apos;t open the pack: {openError}</p>}
    </div>
  );
}

// Shown once the last card has revealed (same reveal animation, delayed past
// the stagger) so the total lands as the punchline rather than a spoiler.
function PackValue({ pricing, revealDelayMs }: { pricing: Pricing; revealDelayMs: number }) {
  const { pack_price_eur: packPrice, total_value_eur: total, unpriced_cards: unpriced } = pricing;

  let summary: ReactNode;
  if (packPrice != null) {
    const delta = total - packPrice;
    summary = (
      <>
        Pulled {formatEUR(total)} from a {formatEUR(packPrice)} pack{" "}
        <span className={delta >= 0 ? "text-green-400" : "text-red-400"}>
          ({delta >= 0 ? "+" : "−"}{formatEUR(Math.abs(delta))})
        </span>
      </>
    );
  } else {
    summary = <>Pack value: {formatEUR(total)}</>;
  }

  return (
    <div
      className="text-center opacity-0 animate-[reveal_0.4s_ease-out_forwards]"
      style={{ animationDelay: `${revealDelayMs}ms` }}
    >
      <p className="text-ink">{summary}</p>
      {unpriced > 0 && (
        <p className="text-xs text-muted">
          {unpriced} card{unpriced === 1 ? "" : "s"} unpriced on Cardmarket
        </p>
      )}
    </div>
  );
}
