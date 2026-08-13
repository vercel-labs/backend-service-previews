# Full-stack previews with Vercel Services

A template for full-stack previews with Vercel Services. It deploys a Next.js storefront, FastAPI checkout API, and containerized Go reservations service as one project.

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fvercel-labs%2Fbackend-service-previews)

```text
                        ┌────────────────────── one immutable deployment ─┐
                        │                                                 │
Browser ── /(.*) ───────┼─> Next.js storefront (apps/web)                 │
                        │                                                 │
Browser ── /api/v1/ ────┼─> FastAPI checkout (services/checkout)          │
           checkout/(.*)│         │                                       │
                        │         │ RESERVATIONS_SERVICE_URL              │
                        │         │ (private service binding)             │
                        │         ▼                                       │
                        │   Go reservations container                     │
                        │   (services/reservations, no public route)      │
                        └─────────────────────────────────────────────────┘
```

## Why this template

A pull request with a polished frontend preview is only half a preview if the page still calls the production backend or a shared staging API. When a change crosses service boundaries (a new field in the UI, the orchestrating API, and a downstream service), someone has to coordinate three deployments, align CORS rules, and decide what "roll back" means when only two of the three releases are healthy.

With Vercel Services, all three services build from the same commit into one immutable deployment. With the Git integration enabled, a pull request gets a single preview URL where the whole contract change is reviewable together, and rollback restores a known combination of frontend and backends. See [ARCHITECTURE.md](ARCHITECTURE.md) for how the pieces are wired.

## Project structure

```text
apps/web/                   Next.js storefront and preview inspector
services/checkout/          FastAPI checkout service (public API surface)
services/reservations/      Containerized Go reservations service (private)
scripts/smoke-services.mjs  End-to-end smoke test of the public request path
vercel.json                 Service definitions, private binding, and routing
```

## Run locally

Prerequisites:

- Node.js 24.x and npm
- Vercel CLI 59.0.0 or newer
- Python 3.12 or newer
- Docker Desktop (builds and runs the Go container)
- Go 1.26 or newer (only needed to edit or test the reservations service)

```bash
npm install
vercel dev -L
```

Open `http://localhost:3000`. `vercel dev -L` runs all three services and injects `RESERVATIONS_SERVICE_URL` into FastAPI. Use it to exercise the public rewrite and private binding together.

The CLI installs the FastAPI runtime dependencies from `services/checkout/requirements.txt` and builds the Go container automatically. Install `requirements-dev.txt` manually only when running the Python tests below.

With the development server running, verify the public and private request path from another terminal:

```bash
npm run smoke:services
```

The catalog and reservations are stored in memory for demonstration. Use a transactional database or Redis operation for production inventory.

## Run the checks

Each service has its own test suite:

```bash
# Next.js: typecheck and build
npm run check

# FastAPI: unit tests
python3 -m venv .venv
.venv/bin/pip install -r services/checkout/requirements-dev.txt
.venv/bin/pytest services/checkout

# Go: unit tests
cd services/reservations && go test ./...
```

## Deploy

1. Import the repository into Vercel (or use the button above).
2. Set the project's Framework Preset to **Services**.
3. Enable **Automatically expose System Environment Variables**.
4. Deploy from Git to create previews for pull requests.

Only Next.js and `/api/v1/checkout/*` are public. FastAPI reaches the private Go service through `RESERVATIONS_SERVICE_URL`, which Vercel injects from the binding in `vercel.json`; there is nothing to configure.

Deploy a preview or production release directly from the project root:

```bash
vercel deploy
vercel deploy --prod
```

The first deployment of a new CLI-created project initializes Production. After that, `vercel deploy` creates a Preview Deployment and `vercel deploy --prod` creates a Production Deployment. CLI deployments do not include Git branch or commit metadata.

### Try it: review a cross-service change

The point of this template is reviewing a change that spans all three services from one URL:

1. Open a branch and make a contract change. For example, add a product to the Next.js selector, define its catalog and price data in FastAPI, and define its inventory in Go.
2. Push the branch and open a pull request. Vercel builds all three services from that commit and posts one preview URL.
3. On the preview URL, request a quote for the new product and confirm its price and reservation.
4. Check the preview inspector on the page: both backends report the same branch and commit as the preview.
5. Compare against production, which does not contain the new product; nothing shipped until the PR merges.

The smoke test also runs against hosted deployments:

```bash
BASE_URL=https://<preview-url> npm run smoke:services
```

If the deployment is protected, set `VERCEL_AUTOMATION_BYPASS_SECRET` to a [Protection Bypass for Automation](https://vercel.com/docs/deployment-protection/methods-to-bypass-deployment-protection/protection-bypass-automation) secret.

## When to use this structure

Use one Services project when several components ship as one product, pull requests frequently change service contracts, and the backends can run as stateless request-driven services.

Prefer separate Vercel projects when services need independent domains or release lifecycles. Workloads that need persistent local disks, always-on workers, or state held inside a process need a different architecture: Services and container image functions are stateless.

## License

[MIT](LICENSE)
