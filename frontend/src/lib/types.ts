// Mirrors backend/openapi.yaml. Keep in sync by hand for now - a generated
// client is worth adding once the API stops changing shape every commit.

export type SetSummary = {
  code: string;
  name: string;
  booster_types: string[];
};

export type SingleFaceImages = {
  image_uris: {
    small?: string;
    normal?: string;
    large?: string;
    png?: string;
    art_crop?: string;
    border_crop?: string;
  };
};

export type DoubleFaceImages = {
  faces: {
    name: string;
    image_uris: SingleFaceImages["image_uris"];
  }[];
};

export type CardImageURIs = SingleFaceImages | DoubleFaceImages;

export type CardSummary = {
  id: string;
  name: string;
  rarity: "common" | "uncommon" | "rare" | "mythic" | "special" | "bonus";
  collector_number: string;
  set_code: string;
  image_uris: CardImageURIs;
};

export type CardPick = {
  slot: number;
  sheet_name: string;
  foil: boolean;
  card: CardSummary;
};

export type PackOpen = {
  open_id: string;
  set_code: string;
  booster_type: string;
  config_version: number;
  created_at: string;
  cards: CardPick[];
};

export type ApiError = {
  error: {
    code: string;
    message: string;
  };
};

/** The card's primary display image - front face for a double-faced card. */
export function primaryImage(uris: CardImageURIs): string | undefined {
  if ("faces" in uris) {
    return uris.faces[0]?.image_uris.normal;
  }
  return uris.image_uris.normal;
}
