"use client";

import { useEffect, useState } from "react";
import { listSets, openPack } from "@/lib/api";
import type { PackOpen, SetSummary } from "@/lib/types";
import CardTile from "./CardTile";

export default function PackOpener() {
  const [sets, setSets] = useState<SetSummary[] | null>(null);
  const [selectedSet, setSelectedSet] = useState<string>("");
  const [pack, setPack] = useState<PackOpen | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listSets()
      .then((res) => {
        setSets(res.sets);
        if (res.sets.length > 0) setSelectedSet(res.sets[0].code);
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  async function handleRip() {
    setLoading(true);
    setError(null);
    try {
      const result = await openPack(selectedSet);
      setPack(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to open pack");
    } finally {
      setLoading(false);
    }
  }

  if (error) {
    return <p className="text-red-400">{error}</p>;
  }

  if (!sets) {
    return <p className="text-neutral-400">loading sets…</p>;
  }

  if (!pack) {
    return (
      <div className="flex flex-col items-center gap-4">
        <select
          value={selectedSet}
          onChange={(e) => setSelectedSet(e.target.value)}
          className="rounded border border-neutral-700 bg-neutral-900 px-3 py-2 text-neutral-100"
        >
          {sets.map((s) => (
            <option key={s.code} value={s.code}>
              {s.name}
            </option>
          ))}
        </select>
        <button
          onClick={handleRip}
          disabled={loading || !selectedSet}
          className="rounded bg-neutral-100 px-6 py-3 font-medium text-neutral-900 disabled:opacity-50"
        >
          {loading ? "ripping…" : "rip pack"}
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-5">
        {pack.cards.map((pick, i) => (
          <CardTile key={`${pick.slot}-${pick.card.id}`} pick={pick} index={i} />
        ))}
      </div>
      <button
        onClick={() => setPack(null)}
        disabled={loading}
        className="rounded bg-neutral-100 px-6 py-3 font-medium text-neutral-900 disabled:opacity-50"
      >
        rip another
      </button>
    </div>
  );
}
