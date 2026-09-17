# rip

A pack-opening simulator for Magic: The Gathering. Pick a set, rip a booster, watch the reveal.
Every pull is decided server-side before the animation plays, and every open is recorded so it
can be replayed from its seed.

> rip is unofficial Fan Content permitted under the Fan Content Policy. Not approved/endorsed by
> Wizards. Portions of the materials used are property of Wizards of the Coast. ©Wizards of the
> Coast LLC.

## Layout

```
/backend   Go API (net/http, pgx/v5, sqlc, goose)
/frontend  Next.js app (App Router, TypeScript, Tailwind)
/art       card images and generation prompts (placeholder assets only — cards render from Scryfall)
```

## Local setup

Requires Go 1.22+, Node 20+, pnpm, and a container runtime (OrbStack or Docker Desktop) for
Postgres.

**1. Database**

```bash
cd backend
cp .env.example .env
make dev          # starts Postgres via docker compose
make migrate-up   # applies the schema
```

**2. Import a set** (once — pulls booster structure from MTGJSON and card images from Scryfall)

```bash
set -a && source .env && set +a
go run ./cmd/import FDN
```

**3. Start the API** (from `backend/`, keep running)

```bash
make run   # :8080
```

**4. Start the frontend** (from `frontend/`, in a second terminal)

```bash
pnpm install
cp .env.example .env.local
pnpm dev   # :3000
```

Open [localhost:3000](http://localhost:3000), pick a set, rip a pack.

## API

See [`backend/openapi.yaml`](backend/openapi.yaml) for the full v1 API (sets, opening a pack,
fetching a past open).

## Card data

Booster structure comes from [MTGJSON](https://mtgjson.com) (weighted pack variants and print
sheets, as printed). Card images are loaded from [Scryfall](https://scryfall.com) and are never
cropped, altered, or watermarked. See `backend/cmd/import` for how a set is added.
