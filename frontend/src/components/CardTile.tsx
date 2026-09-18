import Image from "next/image";
import { formatEUR } from "@/lib/money";
import type { CardPick, CardSummary } from "@/lib/types";
import { primaryImage } from "@/lib/types";

const RARITY_COLOR: Record<CardSummary["rarity"], string> = {
  common: "var(--rarity-common)",
  uncommon: "var(--rarity-uncommon)",
  rare: "var(--rarity-rare)",
  mythic: "var(--rarity-mythic)",
  special: "var(--rarity-mythic)",
  bonus: "var(--rarity-mythic)",
};

export default function CardTile({ pick, index }: { pick: CardPick; index: number }) {
  const src = primaryImage(pick.card.image_uris);

  return (
    <div
      className="flex flex-col items-center gap-0.5 opacity-0 animate-[reveal_0.4s_ease-out_forwards]"
      style={{ animationDelay: `${index * 80}ms` }}
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
      <p className="w-full truncate text-center text-xs text-ink">{pick.card.name}</p>
      <p className="text-[11px]">
        <span style={{ color: RARITY_COLOR[pick.card.rarity] }}>
          {pick.card.rarity}
          {pick.foil ? " · foil" : ""}
        </span>{" "}
        <span className="text-muted">
          · {pick.price_eur != null ? formatEUR(pick.price_eur) : "—"}
        </span>
      </p>
    </div>
  );
}
