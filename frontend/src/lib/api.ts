import type { ApiError, PackOpen, SetSummary } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

class ApiRequestError extends Error {
  constructor(
    message: string,
    public code: string,
  ) {
    super(message);
    this.name = "ApiRequestError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiError | null;
    throw new ApiRequestError(
      body?.error.message ?? `request failed with status ${res.status}`,
      body?.error.code ?? "unknown",
    );
  }

  return res.json() as Promise<T>;
}

export function listSets(): Promise<{ sets: SetSummary[] }> {
  return request("/v1/sets");
}

export function openPack(setCode: string, boosterType?: string): Promise<PackOpen> {
  return request("/v1/packs/open", {
    method: "POST",
    body: JSON.stringify({ set_code: setCode, booster_type: boosterType }),
  });
}

/** SetSummary.pack_image_url is a path on this API, not a full URL. */
export function packImageSrc(path: string): string {
  return `${API_URL}${path}`;
}
