import assert from "node:assert/strict";

const baseUrl = (process.env.BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");
const bypassSecret = process.env.VERCEL_AUTOMATION_BYPASS_SECRET;

async function request(path, init) {
  const headers = new Headers(init?.headers);
  if (bypassSecret) {
    headers.set("x-vercel-protection-bypass", bypassSecret);
  }

  const response = await fetch(`${baseUrl}${path}`, { ...init, headers });
  const body = await response.text();
  return { response, body };
}

const home = await request("/");
assert.equal(home.response.status, 200, "Next.js should serve the public UI");
assert.match(home.body, /Backend Preview \/ Checkout Lab/);

const quote = await request("/api/v1/checkout/quote", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    product_id: "field-notes",
    quantity: 2,
    customer_tier: "vip",
    coupon: "PREVIEW10",
  }),
});
assert.equal(quote.response.status, 200, "FastAPI should return a reservation-backed quote");

const payload = JSON.parse(quote.body);
assert.equal(payload.total_cents, 2754);
assert.match(payload.reservation_id, /^res_[a-f0-9]+$/);
assert.equal(payload.reservation_status, "reserved");
assert.equal(payload.reservation_ttl_seconds, 900);
assert.ok(new Date(payload.reserved_until).getTime() > Date.now());
assert.deepEqual(
  payload.services.map(({ service }) => service),
  ["checkout", "reservations"],
  "The response should prove the FastAPI-to-Go call succeeded",
);

const privateRoute = await request("/internal/health");
assert.equal(privateRoute.response.status, 404, "Go must not have a public route");

console.log(`Verified Next.js -> FastAPI -> Go through ${baseUrl}`);
