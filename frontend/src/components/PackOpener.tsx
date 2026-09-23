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
      <div className="flex flex-col items-center gap-4">
        <div className="reveal-grid grid w-full grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {pack.cards.map((pick, i) => (
            <CardTile key={`${pick.slot}-${pick.card.id}`} pick={pick} index={i} />
          ))}
        </div>
        <PackValue pricing={pack.pricing} revealDelayMs={pack.cards.length * 80 + 400} />
        <div className="flex items-center gap-3">
          <button onClick={handleRip} disabled={tearing} className="rip-button">
            {tearing ? "ripping…" : "rip another"}
          </button>
          <button onClick={handleChangePack} className="ghost-button">
            change pack
          </button>
        </div>
        {openError && <p className="text-sm text-red-400">Couldn&apos;t open the pack: {openError}</p>}
      </div>
    );
  }

  return (
    <div className="flex w-full flex-col items-center gap-5">
      <PackRack sets={sets} index={index} onSelect={setIndex} tearing={tearing} />
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
        <h2 className="display min-w-48 text-center text-xl">{selectedSet.name}</h2>
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

// How many packs to show on each side of the selected one. Beyond this
// they're faded out by the rack's edge mask anyway, so don't mount them.
const RACK_REACH = 3;

// The packs standing in a row like a display rack: neighbors recede in 3D
// and dim, the selected one faces you and carries the tilt/tear behavior.
// Arrow buttons and ←/→ keys are the accessible path; clicking a neighbor
// is a shortcut, so those slots stay out of the tab order.
function PackRack({
  sets,
  index,
  onSelect,
  tearing,
}: {
  sets: SetSummary[];
  index: number;
  onSelect: (i: number) => void;
  tearing: boolean;
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Don't swap the pack out from under a tear that's mid-animation.
      if (tearing) return;
      if (e.key === "ArrowLeft") onSelect((index - 1 + sets.length) % sets.length);
      if (e.key === "ArrowRight") onSelect((index + 1) % sets.length);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [index, sets.length, onSelect, tearing]);

  return (
    <div className="rack">
      {sets.map((set, i) => {
        // Offset wrapped to the shortest way round, so the rack reads as a
        // ring and both sides stay filled at the first and last set.
        const n = sets.length;
        const d = ((((i - index) % n) + n + Math.floor(n / 2)) % n) - Math.floor(n / 2);
        if (Math.abs(d) > RACK_REACH) return null;
        const selected = d === 0;
        return (
          <div
            key={set.code}
            className={`rack-slot${selected ? " selected" : ""}`}
            style={{ "--d": d, "--ad": Math.abs(d) } as React.CSSProperties}
            onClick={selected ? undefined : () => onSelect(i)}
            aria-hidden={!selected}
          >
            <Pack set={set} tearing={selected && tearing} />
          </div>
        );
      })}
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
