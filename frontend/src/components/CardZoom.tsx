"use client";

import Image from "next/image";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { formatEUR } from "@/lib/money";
import type { CardPick } from "@/lib/types";
import { primaryImage } from "@/lib/types";
import { RARITY_COLOR } from "./CardTile";

// One card up close, over the pack. Native <dialog>: top layer (so the
// tile's dim filter never reaches it), Esc and focus trapping for free.
// ←/→ step through the pack without closing: the current card slides out
// against the arrow, then the next one slides in from its side.
export default function CardZoom({
  cards,
  index,
  kicker,
  packTotal,
  onIndex,
  onClose,
}: {
  cards: CardPick[];
  index: number;
  kicker: string;
  packTotal: number;
  onIndex: (i: number) => void;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const pick = cards[index];
  const src = primaryImage(pick.card.image_uris);
  const n = cards.length;
  // Direction of the last step (0 on open: plain fade in).
  const [dir, setDir] = useState(0);
  const [leaving, setLeaving] = useState(false);

  function step(by: number) {
    if (leaving) return;
    setDir(by);
    setLeaving(true);
  }

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  return (
    <dialog
      ref={dialog}
      onClose={onClose}
      // Click on the backdrop (the dialog itself, outside the content) closes it.
      onClick={(e) => e.target === e.currentTarget && dialog.current?.close()}
      onKeyDown={(e) => {
        if (e.key === "ArrowLeft") step(-1);
        if (e.key === "ArrowRight") step(1);
      }}
      className="card-zoom"
    >
      <button onClick={() => step(-1)} aria-label="Previous card" className="card-zoom-arrow">
        ‹
      </button>
      <div
        // Keyed so each card remounts and plays its enter animation.
        key={index}
        className={`card-zoom-body ${leaving ? "leave" : "enter"}`}
        style={{ "--dir": dir } as CSSProperties}
        onAnimationEnd={(e) => {
          if (e.target !== e.currentTarget || !leaving) return;
          onIndex((index + dir + n) % n);
          setLeaving(false);
        }}
      >
        {src ? (
          <Image
            src={src}
            alt={pick.card.name}
            width={488}
            height={680}
            className="card-zoom-img"
            unoptimized
          />
        ) : (
          <div className="card-zoom-img flex items-center justify-center bg-surface text-muted">no image</div>
        )}
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-muted">{kicker}</p>
          <h2 className="display text-3xl sm:text-4xl">{pick.card.name}</h2>
          <p className="capitalize" style={{ color: RARITY_COLOR[pick.card.rarity] }}>
            {pick.card.rarity}
            {pick.foil ? " · foil" : ""}
          </p>
          <p className="display mt-2 text-5xl">{pick.price_eur != null ? formatEUR(pick.price_eur) : "—"}</p>
          <p className="text-sm text-muted">
            {pick.price_eur == null
              ? "unpriced on Cardmarket"
              : packTotal > 0 && `${Math.round((pick.price_eur / packTotal) * 100)}% of this pack's value`}
          </p>
          <form method="dialog" className="mt-2">
            <button className="card-zoom-back">back to pack</button>
          </form>
        </div>
      </div>
      <button onClick={() => step(1)} aria-label="Next card" className="card-zoom-arrow">
        ›
      </button>
    </dialog>
  );
}
