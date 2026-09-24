"use client";

import { useEffect, useState, type ReactNode } from "react";
import { listSets, openPack } from "@/lib/api";
import { formatEUR, formatSignedEUR } from "@/lib/money";
import {
  addPack,
  EMPTY_SESSION,
  loadHistory,
  loadSession,
  pushHistory,
  saveHistory,
  saveSession,
  type HistoryEntry,
  type Session,
} from "@/lib/session";
import { isDimmed, loadSettings, saveSettings, type Settings } from "@/lib/settings";
import type { PackOpen, Pricing, SetSummary } from "@/lib/types";
import CardTile from "./CardTile";
import CardZoom from "./CardZoom";
import HistoryDrawer from "./HistoryDrawer";
import Pack from "./Pack";
import SettingsModal from "./SettingsModal";

// Minimum time the tear animation gets to play, even if the API answers
// faster - so opening never feels like an instant cut.
const TEAR_MS = 650;

function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// When the pack total reveals: alongside the last card, so both land together.
function revealDelayMs(pack: PackOpen) {
  return (pack.cards.length - 1) * 80;
}

export default function PackOpener() {
  const [sets, setSets] = useState<SetSummary[] | null>(null);
  const [setsError, setSetsError] = useState<string | null>(null);
  const [index, setIndex] = useState(0);
  const [pack, setPack] = useState<PackOpen | null>(null);
  const [tearing, setTearing] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);
  const [zoomed, setZoomed] = useState<number | null>(null);
  // Only rendered once sets have loaded (client-side), so reading storage in
  // the initializer can't cause a hydration mismatch.
  const [session, setSession] = useState<Session>(loadSession);
  const [history, setHistory] = useState<HistoryEntry[]>(loadHistory);
  const [settings, setSettings] = useState<Settings>(loadSettings);

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
      // Persist now so a reload mid-reveal doesn't lose the pack; the tracker
      // picks it up when PackValue's reveal starts, so it can't spoil the result.
      const next = addPack(loadSession(), result.pricing);
      saveSession(next);
      saveHistory(pushHistory(loadHistory(), { n: next.packs, pack: result }));
    } catch (err) {
      setOpenError(err instanceof Error ? err.message : "failed to open pack");
    } finally {
      setTearing(false);
    }
  }

  function handleResetSession() {
    saveSession(EMPTY_SESSION);
    setSession(EMPTY_SESSION);
    // Pack numbers restart with the session, so the old ones would clash.
    saveHistory([]);
    setHistory([]);
  }

  function handleSettingsChange(next: Settings) {
    saveSettings(next);
    setSettings(next);
  }

  const sessionBar = (
    <div className="fixed top-2 right-6 z-10 flex items-center gap-2">
      <SessionTracker session={session} onReset={handleResetSession} />
      <HistoryDrawer history={history} currentId={pack?.open_id ?? null} />
      <SettingsModal settings={settings} onChange={handleSettingsChange} />
    </div>
  );

  function handleChangePack() {
    setPack(null);
    setOpenError(null);
  }

  if (pack) {
    return (
      <div className="flex flex-col items-center gap-4">
        {sessionBar}
        <div className="reveal-grid grid w-full grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {pack.cards.map((pick, i) => (
            <CardTile
              key={`${pick.slot}-${pick.card.id}`}
              pick={pick}
              index={i}
              packPrice={pack.pricing.pack_price_eur}
              dimmed={isDimmed(settings, pick.price_eur)}
              onZoom={() => setZoomed(i)}
            />
          ))}
        </div>
        {zoomed != null && (
          <CardZoom
            cards={pack.cards}
            index={zoomed}
            kicker={selectedSet.name}
            packTotal={pack.pricing.total_value_eur}
            onIndex={setZoomed}
            onClose={() => setZoomed(null)}
          />
        )}
        {/* Keyed so "rip another" remounts it and the reveal replays; the
            animation start is also what updates the tracker, so both land together. */}
        <PackValue
          key={pack.open_id}
          pricing={pack.pricing}
          revealDelayMs={revealDelayMs(pack)}
          onReveal={() => {
            setSession(loadSession());
            setHistory(loadHistory());
          }}
        />
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
      {sessionBar}
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

// Shown with the last card (same reveal animation, delayed to the end of
// the stagger) so the total lands as the punchline rather than a spoiler.
function PackValue({
  pricing,
  revealDelayMs,
  onReveal,
}: {
  pricing: Pricing;
  revealDelayMs: number;
  onReveal: () => void;
}) {
  const { pack_price_eur: packPrice, total_value_eur: total, unpriced_cards: unpriced } = pricing;

  let summary: ReactNode;
  if (packPrice != null) {
    const delta = total - packPrice;
    summary = (
      <>
        Pulled {formatEUR(total)} from a {formatEUR(packPrice)} pack{" "}
        <span className={delta >= 0 ? "text-green-400" : "text-red-400"}>
          ({delta >= 0 ? "+" : "−"}
          {formatEUR(Math.abs(delta))})
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
      // animationstart fires after the delay, i.e. the moment the total appears.
      onAnimationStart={onReveal}
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

// Running P/L since the last reset: a pill in the top bar, details in a
// native popover (light dismiss + Esc for free). Before the first pack it
// stays neutral and the popover explains what will show up.
function SessionTracker({ session, onReset }: { session: Session; onReset: () => void }) {
  const { packs, unpriced, spent, pulled } = session;
  const empty = packs === 0;
  const net = pulled - spent;
  const priced = packs - unpriced;
  const tone = empty ? "text-ink" : net >= 0 ? "text-gain" : "text-loss";
  const packLabel = `${packs} pack${packs === 1 ? "" : "s"}`;

  return (
    <>
      <button popoverTarget="session-pop" className="session-pill">
        <span className="text-muted">{packLabel}</span>
        <span className="session-pill-sep" />
        <span className={`font-medium ${tone}`}>{empty ? formatEUR(0) : formatSignedEUR(net)}</span>
        <svg className="session-chev" viewBox="0 0 12 12" aria-hidden="true">
          <path d="M3 7.5 6 4.5l3 3" fill="none" stroke="currentColor" strokeWidth="1.5" />
        </svg>
      </button>
      <div id="session-pop" popover="auto" className="session-pop">
        <p className="session-kicker">Session · {packLabel}</p>
        {empty ? (
          <div className="session-empty">
            <span className="session-empty-icon">
              <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
                <path
                  d="M4.5 2.5h7v11h-7zM4.5 5h7"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.25"
                  strokeLinejoin="round"
                />
              </svg>
            </span>
            <p className="display text-xl">Nothing ripped yet</p>
            <p className="text-sm text-muted">Open a pack and your spent, pulled and net totals will show up here.</p>
          </div>
        ) : (
          <>
            <dl className="session-rows">
              <dt>Spent</dt>
              <dd>{formatEUR(spent)}</dd>
              <dt>Pulled</dt>
              <dd>{formatEUR(pulled)}</dd>
            </dl>
            <div className="session-net">
              <span>Net</span>
              <span className={`flex items-baseline gap-2 ${tone}`}>
                {spent > 0 && (
                  <span className="text-xs">
                    {net >= 0 ? "+" : "−"}
                    {Math.abs((net / spent) * 100).toFixed(1)}%
                  </span>
                )}
                <span className="display text-2xl">{formatSignedEUR(net)}</span>
              </span>
            </div>
            {priced > 0 && (
              <p className="flex justify-between text-xs text-muted">
                <span>Avg per pack</span>
                <span>{formatSignedEUR(net / priced)}</span>
              </p>
            )}
            {unpriced > 0 && (
              <p className="text-xs text-muted">
                {unpriced} unpriced pack{unpriced === 1 ? "" : "s"} excluded
              </p>
            )}
            <button
              onClick={(e) => {
                onReset();
                e.currentTarget.closest<HTMLElement>("[popover]")?.hidePopover();
              }}
              className="session-reset"
            >
              <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
                <path
                  d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9M2.5 2.5v3h3"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              Start new session
            </button>
          </>
        )}
      </div>
    </>
  );
}
