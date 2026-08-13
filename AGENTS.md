# Agent guide

Instructions for AI coding agents (and new contributors) working in this repository.

## What this repository is

A Vercel Services template: three services in one project, deployed together as one immutable deployment for each Git-triggered commit deployment.

| Path | Service | Language / runtime | Exposure |
| --- | --- | --- | --- |
| `apps/web/` | Storefront and preview inspector | Next.js (App Router, TypeScript) | Public: all routes except `/api/v1/checkout/*` |
| `services/checkout/` | Checkout API | Python, FastAPI (`main:app`) | Public: `/api/v1/checkout/*` via rewrite |
| `services/reservations/` | Reservations engine | Go, Gin, built from `Dockerfile.vercel` | Private: reachable only through the service binding |

`vercel.json` is the source of truth for service roots, the private binding, and public routing. If you add, rename, or re-route a service, change it there first.

## Invariants to preserve

- **The reservations service has no public route.** FastAPI reaches it through `RESERVATIONS_SERVICE_URL`, which Vercel injects from the `bindings` entry in `vercel.json`. It is not a secret and must never be added to `.env` files or Vercel project env vars by hand.
- **The public API is versioned.** Browser-facing checkout routes live under `/api/v1/checkout/`; the rewrite in `vercel.json` and the FastAPI route prefixes must stay in sync.
- **Successful backend responses report service identity** (service name, runtime, environment, branch, commit). Git metadata comes from `VERCEL_GIT_COMMIT_REF` and `VERCEL_GIT_COMMIT_SHA`. The services fall back to `local` in development and `CLI deployment` for hosted CLI deployments without Git metadata. Don't strip these fields; the preview inspector in `apps/web` renders them.
- **State is in-memory by design.** The Go service guards its catalog with a mutex inside one process. This is a deliberate demo simplification, documented in the README; don't "fix" it by adding a database unless asked.

## Commands

Use Node.js 24.x, Vercel CLI 59.0.0 or newer, Python 3.12 or newer, and Docker Desktop. Go 1.26 or newer is only required when editing or testing the reservations service.

```bash
npm install                 # root + apps/web workspace

# Run the full stack through the public rewrite and private binding
vercel dev -L               # serves everything at http://localhost:3000

# End-to-end smoke test against a running server
npm run smoke:services      # override target with BASE_URL=https://... ;
                            # protected deployments need VERCEL_AUTOMATION_BYPASS_SECRET

# Per-service checks
npm run check                                        # Next.js typecheck + build
python3 -m venv .venv && \
  .venv/bin/pip install -r services/checkout/requirements-dev.txt && \
  .venv/bin/pytest services/checkout                 # FastAPI tests
cd services/reservations && go test ./...            # Go tests
docker build -f services/reservations/Dockerfile.vercel services/reservations  # container build check
```

Run the checks for every service you touched; run the smoke test if you changed routing, the binding, or a request/response contract.

## Gotchas

- `npm run dev:web` runs only the Next.js workspace. Checkout calls will fail without the other services; use `vercel dev -L` for anything involving the API path.
- The Python virtualenvs (`.venv/`, `services/checkout/.venv/`) and `__pycache__/` are local artifacts and gitignored; never commit them.
- `GUIDE_OUTLINE.md`, `VERIFICATION.md`, and `draft-v2.md` are gitignored local working documents for the companion guide. They are not part of the published template; don't reference them from tracked files.
- Git deployments require the Vercel project's Framework Preset to be **Services**; another preset ignores the services configuration.
- Enable **Automatically expose System Environment Variables** when the preview inspector must show the Git branch and commit.
