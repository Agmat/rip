import Image from "next/image";
import type { CardPick } from "@/lib/types";
import { primaryImage } from "@/lib/types";

export default function CardTile({ pick, index }: { pick: CardPick; index: number }) {
  const src = primaryImage(pick.card.image_uris);

  return (
    <div
      className="flex flex-col items-center gap-1 opacity-0 animate-[reveal_0.4s_ease-out_forwards]"
      style={{ animationDelay: `${index * 80}ms` }}
    >
      {src ? (
        <Image
          src={src}
          alt={pick.card.name}
          width={244}
          height={340}
          className="rounded-lg"
          unoptimized
        />
      ) : (
        <div className="flex h-[340px] w-[244px] items-center justify-center rounded-lg bg-neutral-800 text-sm text-neutral-400">
          no image
        </div>
      )}
      <p className="text-sm text-neutral-200">{pick.card.name}</p>
      <p className="text-xs uppercase text-neutral-400">
        {pick.card.rarity}
        {pick.foil ? " · foil" : ""}
      </p>
    </div>
  );
}
