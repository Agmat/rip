"use client";

import { useRef, useState } from "react";
import Image from "next/image";
import { formatEUR, formatSignedEUR } from "@/lib/money";
import type { HistoryEntry } from "@/lib/session";
import { primaryImage, type CardPick } from "@/lib/types";
import CardTile from "./CardTile";
import CardZoom from "./CardZoom";

const TOP_CARDS = 3;

function ago(iso: string, now: number) {
  const min = Math.floor((now - Date.parse(iso)) / 60_000);
  if (min < 1) return "just now";
  if (min < 60) return `${min} min ago`;
  if (min < 60 * 24) return `${Math.floor(min / 60)} h ago`;
  return `${Math.floor(min / (60 * 24))} d ago`;
}

// Rare and up keep their rarity color, like a hit; the rest stay quiet.
function priceColor(pick: CardPick) {
  const { rarity } = pick.card;
  if (rarity === "common" || rarity === "uncommon") return undefined;
  return rarity === "rare" ? "var(--rarity-rare)" : "var(--rarity-mythic)";
}

// Last few packs in a right-hand drawer. Native popover: light dismiss and
// Esc for free, and it doesn't cover the reveal with a backdrop.
export default function HistoryDrawer({
  history,
  currentId,
}: {
  history: HistoryEntry[];
  currentId: string | null;
}) {
  // Stamped on open so "3 min ago" is fresh each time without a render-time clock.
  const [now, setNow] = useState(0);
  // Index into history of the pack open in the viewer.
  const [viewing, setViewing] = useState<number | null>(null);
  const viewer = useRef<HTMLDialogElement>(null);

  function openViewer(i: number) {
    document.getElementById("history-drawer")?.hidePopover();
    setViewing(i);
    viewer.current?.showModal();
  }
  const priced = history.filter((e) => e.pack.pricing.pack_price_eur != null);
  const spent = priced.reduce((s, e) => s + (e.pack.pricing.pack_price_eur ?? 0), 0);
  const pulled = priced.reduce((s, e) => s + e.pack.pricing.total_value_eur, 0);
  const net = pulled - spent;

  return (
    <>
      <button popoverTarget="history-drawer" className="ghost-button">
        history
      </button>
      <div
        id="history-drawer"
        popover="auto"
        className="history-drawer"
        onToggle={(e) => e.newState === "open" && setNow(Date.now())}
      >
        <header className="history-head">
          <div>
            <h2 className="display text-xl">
              Last {history.length || ""} pull{history.length === 1 ? "" : "s"}
            </h2>
            {priced.length > 0 && (
              <p className="text-xs text-muted">
                spent {formatEUR(spent)} · pulled {formatEUR(pulled)} ·{" "}
                <span className={net >= 0 ? "text-gain" : "text-loss"}>{formatSignedEUR(net)}</span>
              </p>
            )}
          </div>
          <button
            popoverTarget="history-drawer"
            popoverTargetAction="hide"
            aria-label="Close history"
            className="history-close"
          >
            <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
              <path d="M3 3l10 10M13 3 3 13" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
        </header>
        {history.length === 0 ? (
          <p className="px-5 py-8 text-center text-sm text-muted">Packs you rip will show up here.</p>
        ) : (
          <ol className="history-list">
            {history.map((entry, i) => (
              <HistoryItem
                key={entry.pack.open_id}
                entry={entry}
                now={now}
                current={entry.pack.open_id === currentId}
                onView={() => openViewer(i)}
              />
            ))}
          </ol>
        )}
      </div>
      <dialog
        ref={viewer}
        className="pack-viewer"
        aria-label="Pack viewer"
        onClose={() => setViewing(null)}
      >
        {viewing != null && history[viewing] && (
          // Keyed per pack so switching packs replays the reveal and drops the zoom.
          <PackViewer
            key={history[viewing].pack.open_id}
            history={history}
            index={viewing}
            now={now}
            onNavigate={setViewing}
            onBack={() => {
              viewer.current?.close();
              document.getElementById("history-drawer")?.showPopover();
            }}
            onClose={() => viewer.current?.close()}
          />
        )}
      </dialog>
    </>
  );
}

function HistoryItem({
  entry,
  now,
  current,
  onView,
}: {
  entry: HistoryEntry;
  now: number;
  current: boolean;
  onView: () => void;
}) {
  const { cards, pricing, created_at } = entry.pack;
  const { pack_price_eur: packPrice, total_value_eur: total } = pricing;
  const top = [...cards].sort((a, b) => (b.price_eur ?? -1) - (a.price_eur ?? -1)).slice(0, TOP_CARDS);

  return (
    // The whole card opens the viewer; the "View all" button is the keyboard
    // path, and its click just bubbles up here.
    <li
      className={`history-item${current ? " current" : ""}`}
      aria-current={current || undefined}
      onClick={onView}
    >
      <div className="flex items-baseline justify-between">
        <p>
          <span className="display text-base">Pack #{entry.n}</span>{" "}
          <span className="text-[11px] text-muted">{ago(created_at, now)}</span>
        </p>
        {packPrice != null && (
          <span className={`text-sm font-medium ${total >= packPrice ? "text-gain" : "text-loss"}`}>
            {formatSignedEUR(total - packPrice)}
          </span>
        )}
      </div>
      <p className="text-xs text-muted">
        {packPrice != null
          ? `${formatEUR(total)} from a ${formatEUR(packPrice)} pack`
          : `${formatEUR(total)} pulled`}
      </p>
      <ul className="flex flex-col gap-1.5 py-1">
        {top.map((pick) => {
          const src = primaryImage(pick.card.image_uris);
          return (
            <li key={`${pick.slot}-${pick.card.id}`} className="flex items-center gap-2.5 text-sm">
              {src ? (
                <Image src={src} alt="" width={18} height={25} className="history-thumb" unoptimized />
              ) : (
                <span className="history-thumb bg-surface" />
              )}
              <span className="flex-1 truncate">{pick.card.name}</span>
              <span className="text-xs text-muted" style={{ color: priceColor(pick) }}>
                {pick.price_eur != null ? formatEUR(pick.price_eur) : "—"}
              </span>
            </li>
          );
        })}
      </ul>
      <button className="history-view">
        View all {cards.length} cards →
      </button>
    </li>
  );
}

// One past pack, full screen, cards sorted by value. Prev/next walk the
// history in drawer order (newest first).
function PackViewer({
  history,
  index,
  now,
  onNavigate,
  onBack,
  onClose,
}: {
  history: HistoryEntry[];
  index: number;
  now: number;
  onNavigate: (i: number) => void;
  onBack: () => void;
  onClose: () => void;
}) {
  const { n, pack } = history[index];
  const { pack_price_eur: packPrice, total_value_eur: total } = pack.pricing;
  const cards = [...pack.cards].sort((a, b) => (b.price_eur ?? -1) - (a.price_eur ?? -1));
  const [zoomed, setZoomed] = useState<number | null>(null);

  return (
    <div className="flex h-full flex-col">
      <header className="viewer-head">
        <button onClick={onBack} className="viewer-btn gap-2 px-4">
          <Chevron dir="left" />
          history
        </button>
        <div className="min-w-0 flex-1">
          <p>
            <span className="display text-2xl">Pack #{n}</span>{" "}
            <span className="text-sm text-muted">{ago(pack.created_at, now)}</span>
          </p>
          <p className="text-sm text-muted">
            {packPrice != null ? (
              <>
                Pulled {formatEUR(total)} from a {formatEUR(packPrice)} pack{" "}
                <span className={`font-semibold ${total >= packPrice ? "text-gain" : "text-loss"}`}>
                  ({formatSignedEUR(total - packPrice)})
                </span>
              </>
            ) : (
              <>Pulled {formatEUR(total)}</>
            )}
          </p>
        </div>
        <p className="hidden text-xs text-muted sm:block">
          Sorted by value · {index + 1} of {history.length}
        </p>
        <button
          onClick={() => onNavigate(index - 1)}
          disabled={index === 0}
          aria-label="Newer pack"
          className="viewer-btn w-11"
        >
          <Chevron dir="left" />
        </button>
        <button
          onClick={() => onNavigate(index + 1)}
          disabled={index === history.length - 1}
          aria-label="Older pack"
          className="viewer-btn w-11"
        >
          <Chevron dir="right" />
        </button>
        <button onClick={onClose} aria-label="Close" className="history-close ml-2">
          <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
            <path d="M3 3l10 10M13 3 3 13" stroke="currentColor" strokeWidth="1.5" />
          </svg>
        </button>
      </header>
      <div className="flex-1 overflow-y-auto">
        <div className="viewer-grid grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {cards.map((pick, i) => (
            <CardTile
              key={`${pick.slot}-${pick.card.id}`}
              pick={pick}
              index={i}
              packPrice={packPrice}
              dimmed={false}
              onZoom={() => setZoomed(i)}
            />
          ))}
        </div>
      </div>
      {zoomed != null && (
        <CardZoom
          cards={cards}
          index={zoomed}
          kicker={`Pack #${n}`}
          packTotal={total}
          onIndex={setZoomed}
          onClose={() => setZoomed(null)}
        />
      )}
    </div>
  );
}

function Chevron({ dir }: { dir: "left" | "right" }) {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
      <path
        d={dir === "left" ? "M10 3 5 8l5 5" : "M6 3l5 5-5 5"}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
      />
    </svg>
  );
}
