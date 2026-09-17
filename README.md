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
/frontend  Next.js app (later phase)
/art       card images and generation prompts (placeholder assets only — cards render from Scryfall)
```

## Local setup

Requires Go 1.22+ and a container runtime (OrbStack or Docker Desktop) for Postgres.

```bash
cp backend/.env.example backend/.env
make dev   # starts Postgres via docker compose
make run   # starts the API on :8080
```

```bash
curl localhost:8080/healthz
```

## Card data

Booster structure comes from [MTGJSON](https://mtgjson.com) (weighted pack variants and print
sheets, as printed). Card images are loaded from [Scryfall](https://scryfall.com) and are never
cropped, altered, or watermarked. See `backend/cmd/import` for how a set is added.
