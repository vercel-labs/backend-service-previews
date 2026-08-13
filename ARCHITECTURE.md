# Full-stack previews with Vercel Services

This document explains how three services in three languages become one deployment with one URL.

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

## Services

- **Next.js storefront** (`apps/web/`): collects a product, quantity, customer tier, and optional coupon, and displays the branch, commit, and runtime identity included in a successful quote so a reviewer can verify both backends belong to the same deployment.
- **FastAPI checkout** (`services/checkout/`): the policy boundary. Owns public API validation, the product catalog, and all pricing (tier discounts and coupons): it validates the request, asks the reservations engine for a final stock decision, and composes one stable public response.
- **Go reservations** (`services/reservations/`): the concurrency-sensitive inventory boundary, built from `Dockerfile.vercel`. Performs the final stock check, atomically creates a 15-minute hold, and returns the reservation ID and expiry, echoing the product and quantity it reserved so checkout can verify the contract.

## Routing: rewrites vs. bindings

Two different mechanisms connect the services, and the distinction matters:

- **Rewrites** route *public* traffic. `vercel.json` sends `/api/v1/checkout/(.*)` to the checkout service and everything else to the web service. Both are reachable from the internet.
- **Service bindings** provide *private* reachability. The checkout service declares a binding on the reservations service, and Vercel injects a deployment-scoped URL as `RESERVATIONS_SERVICE_URL`. The reservations service has no rewrite, so it has no public route at all; its only caller is FastAPI within the same deployment.

A binding is reachability, not authentication. If a private service handled sensitive operations in production, it would still need application-level authorization.

## Deployment lifecycle

1. A developer pushes a commit to a repository connected through the Vercel Git integration, for example on a pull-request branch.
2. Vercel reads `vercel.json`, detects the three service roots, and builds each one independently: Next.js and FastAPI with their framework builders, Go from its Dockerfile.
3. The builds are assembled into **one immutable deployment** with one URL.
4. The binding URL injected into FastAPI points at the reservations build *from the same deployment*: a preview's checkout service always talks to that preview's reservations service, never to production.
5. The Git integration adds a single preview URL to the pull request, where the whole cross-service change is reviewable.

Because a deployment is one unit, promotion and rollback are also atomic: rolling back restores a known combination of frontend and both backends.

## Deployment identity

Successful health, quote, and reservation responses include the service name, runtime, environment, branch, and shortened commit SHA:

```json
{
  "service": "reservations",
  "runtime": "Go + Gin reservation container",
  "environment": "preview",
  "branch": "feature/stock-holds",
  "commit": "a1b2c3d"
}
```

The values come from the system environment variables (`VERCEL_ENV`, `VERCEL_GIT_COMMIT_REF`, `VERCEL_GIT_COMMIT_SHA`) that Vercel exposes to Git-triggered deployments. The services use `local` when those Git values are unavailable under `vercel dev`. A direct CLI deployment without Git metadata uses `CLI deployment` instead. The storefront renders this metadata so "these responses came from this preview" is verifiable rather than assumed.

## What is isolated and what is not

Isolated per deployment:

- Code and public routing (each preview serves its own commit's rewrites)
- Service bindings (always resolve within the same deployment)

Not isolated:

- Environment variable values: Preview-scoped values are selected for each Preview Deployment, but all previews share them unless you configure branch-specific values or separate backing resources.
- External databases and third-party APIs: previews are not given cloned infrastructure. This template avoids the issue by keeping state in memory.
- Schema migrations: a preview sharing a database with production needs backward-compatible migration planning.

The in-memory catalog is a demo simplification: Vercel can run multiple container instances or replace one between requests, and instances do not share memory. Production inventory needs an atomic backing store (a Postgres transaction, a Redis operation, or equivalent) with TTL semantics for holds.

## Platform limits

This template also inherits the current limits and billing model of Vercel Services and container image functions:

- Vercel Services is in beta. Review the current availability and service limits before adopting it for a production workload.
- Calls over service bindings are billed as Service Requests. The bytes returned by a service are billed as Fast Origin Transfer.
- A container image function with no traffic scales down after five minutes in production and after 30 seconds in Preview. The next request may need to start a new instance.
- Secure Compute and Static IPs are not currently supported with custom container images. This rules out a containerized service when its upstream systems require private networking or allowlisted outbound IPs.

See the current [Vercel Services guide](https://vercel.com/kb/guide/vercel-services) and [container images documentation](https://vercel.com/docs/functions/container-images) before using this architecture in production.
