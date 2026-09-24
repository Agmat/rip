import Image from "next/image";
import { formatEUR } from "@/lib/money";
import type { CardPick, CardSummary } from "@/lib/types";
import { primaryImage } from "@/lib/types";

export const RARITY_COLOR: Record<CardSummary["rarity"], string> = {
  common: "var(--rarity-common)",
  uncommon: "var(--rarity-uncommon)",
  rare: "var(--rarity-rare)",
  mythic: "var(--rarity-mythic)",
  special: "var(--rarity-mythic)",
  bonus: "var(--rarity-mythic)",
};

// A hit is a card worth at least the pack on its own. Commons and uncommons
// glow rare gold: their grey reads as a border, not a highlight.
function hitColor(pick: CardPick, packPrice: number | null): string | null {
  if (packPrice == null || pick.price_eur == null || pick.price_eur < packPrice) return null;
  const { rarity } = pick.card;
  return rarity === "common" || rarity === "uncommon" ? "var(--rarity-rare)" : RARITY_COLOR[rarity];
}

export default function CardTile({
  pick,
  index,
  packPrice,
  dimmed,
  onZoom,
}: {
  pick: CardPick;
  index: number;
  packPrice: number | null;
  dimmed: boolean;
  onZoom: () => void;
}) {
  const src = primaryImage(pick.card.image_uris);
  const hit = hitColor(pick, packPrice);

  return (
    <div
      className="flex flex-col items-center gap-0.5 opacity-0 animate-[reveal_0.4s_ease-out_forwards]"
      // filter, not opacity: the reveal animation owns opacity.
      style={{ animationDelay: `${index * 80}ms`, filter: dimmed ? "opacity(0.5)" : undefined }}
    >
      <button
        onClick={onZoom}
        aria-label={`Zoom in on ${pick.card.name}`}
        className="card-zoom-trigger block w-full rounded-lg"
        style={hit ? { boxShadow: `0 0 0 2px ${hit}, 0 0 18px 2px ${hit}` } : undefined}
      >
        {src ? (
          <Image
            src={src}
            alt={pick.card.name}
            width={244}
            height={340}
            className="h-auto w-full rounded-lg"
            unoptimized
          />
        ) : (
          <div className="flex aspect-[244/340] w-full items-center justify-center rounded-lg bg-surface text-sm text-muted">
            no image
          </div>
        )}
      </button>
      <p className="w-full truncate text-center text-xs text-ink">{pick.card.name}</p>
      <p className="text-[11px]">
        <span style={{ color: RARITY_COLOR[pick.card.rarity] }}>
          {pick.card.rarity}
          {pick.foil ? " · foil" : ""}
        </span>{" "}
        <span
          className={hit ? "font-bold" : "text-muted"}
          style={hit ? { color: hit } : undefined}
        >
          · {pick.price_eur != null ? formatEUR(pick.price_eur) : "—"}
          {hit && <span className="sr-only"> (worth more than the pack)</span>}
        </span>
      </p>
    </div>
  );
}
