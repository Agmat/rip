"use client";

import { useEffect, useState } from "react";
import { listSets, openPack } from "@/lib/api";
import type { PackOpen, SetSummary } from "@/lib/types";
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

  function handleReset() {
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
        <button onClick={handleReset} className="rip-button">
          rip another
        </button>
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
      <button onClick={handleRip} disabled={tearing} className="rip-button">
        {tearing ? "ripping…" : "rip pack"}
      </button>
      {openError && <p className="text-sm text-red-400">Couldn&apos;t open the pack: {openError}</p>}
    </div>
  );
}
