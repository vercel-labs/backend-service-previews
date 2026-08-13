"use client";

import { FormEvent, useEffect, useRef, useState, useTransition } from "react";

type ServiceIdentity = {
  service: string;
  runtime: string;
  environment: string;
  branch: string;
  commit: string;
};

type Quote = {
  quote_id: string;
  reservation_id: string;
  reservation_status: "reserved";
  reserved_until: string;
  reservation_ttl_seconds: number;
  product: string;
  quantity: number;
  currency: string;
  subtotal_cents: number;
  tier_discount_cents: number;
  coupon_discount_cents: number;
  total_cents: number;
  services: ServiceIdentity[];
};

type CheckoutFailure = {
  message: string;
  suggestedQuantity?: number;
};

type ErrorDetail = {
  message?: string;
  suggested_quantity?: number;
};

const PRODUCTS = [
  { id: "field-notes", label: "Field Notes", price: "$18" },
  { id: "desk-lamp", label: "Signal Lamp", price: "$84" },
  { id: "travel-mug", label: "Transit Mug", price: "$32" },
];

function money(cents: number, currency: string) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
  }).format(cents / 100);
}

function secondsUntil(timestamp: string) {
  return Math.max(0, Math.ceil((new Date(timestamp).getTime() - Date.now()) / 1000));
}

function countdown(seconds: number) {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, "0");
  const remainder = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${remainder}`;
}

function ServiceStamp({ service }: { service: ServiceIdentity }) {
  return (
    <article className="service-stamp">
      <div className="stamp-heading">
        <span className="online-dot" />
        <strong>{service.service}</strong>
        <small>{service.environment}</small>
      </div>
      <p>{service.runtime}</p>
      <dl>
        <div>
          <dt>Branch</dt>
          <dd>{service.branch}</dd>
        </div>
        <div>
          <dt>Commit</dt>
          <dd>{service.commit}</dd>
        </div>
      </dl>
    </article>
  );
}

export function CheckoutDemo() {
  const formRef = useRef<HTMLFormElement>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [remainingSeconds, setRemainingSeconds] = useState(0);
  const [failure, setFailure] = useState<CheckoutFailure | null>(null);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    if (!quote) {
      return;
    }

    const updateCountdown = () => setRemainingSeconds(secondsUntil(quote.reserved_until));
    updateCountdown();
    const timer = window.setInterval(updateCountdown, 1000);
    return () => window.clearInterval(timer);
  }, [quote]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);

    startTransition(async () => {
      setFailure(null);

      try {
        const response = await fetch("/api/v1/checkout/quote", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            product_id: form.get("product"),
            quantity: Number(form.get("quantity")),
            customer_tier: form.get("tier"),
            coupon: form.get("coupon"),
          }),
        });

        const body = (await response.json()) as Quote | { detail?: string | ErrorDetail };

        if (!response.ok) {
          const detail = "detail" in body ? body.detail : undefined;
          if (detail && typeof detail === "object") {
            setQuote(null);
            setFailure({
              message: detail.message ?? "The reservation could not be created.",
              suggestedQuantity: detail.suggested_quantity,
            });
            return;
          }
          throw new Error(detail || "The reservation could not be created.");
        }

        setQuote(body as Quote);
      } catch (requestError) {
        setQuote(null);
        setFailure({
          message: requestError instanceof Error ? requestError.message : "The backend preview is unavailable.",
        });
      }
    });
  }

  function acceptSuggestedQuantity() {
    const form = formRef.current;
    const quantity = form?.elements.namedItem("quantity");
    if (!form || !(quantity instanceof HTMLInputElement) || !failure?.suggestedQuantity) {
      return;
    }

    quantity.value = String(failure.suggestedQuantity);
    form.requestSubmit();
  }

  return (
    <section className="workspace">
      <form ref={formRef} className="checkout-card" onSubmit={submit}>
        <div className="section-number">01 / Request</div>
        <h2>Hold stock for checkout</h2>

        <label>
          Product
          <select name="product" defaultValue="desk-lamp">
            {PRODUCTS.map((product) => (
              <option key={product.id} value={product.id}>
                {product.label} — {product.price}
              </option>
            ))}
          </select>
        </label>

        <div className="field-row">
          <label>
            Customer
            <select name="tier" defaultValue="vip">
              <option value="standard">Standard</option>
              <option value="vip">VIP</option>
            </select>
          </label>
          <label>
            Quantity
            <input name="quantity" type="number" defaultValue="2" />
          </label>
        </div>

        <label>
          Preview coupon
          <input name="coupon" defaultValue="PREVIEW10" spellCheck="false" />
        </label>

        <button type="submit" disabled={isPending}>
          {isPending ? "Checking final stock…" : "Reserve for 15 minutes"}
          <span>↗</span>
        </button>

      </form>

      <div className="result-card" aria-live="polite">
        <div className="section-number">02 / Inspect</div>
        {failure ? (
          <div className="empty-result checkout-failure">
            <div role="alert">
              <h2>We couldn’t hold this cart</h2>
              <p>{failure.message}</p>
            </div>
            {failure.suggestedQuantity ? (
              <button type="button" onClick={acceptSuggestedQuantity} disabled={isPending}>
                Accept {failure.suggestedQuantity} items &amp; continue to payment
                <span>→</span>
              </button>
            ) : null}
          </div>
        ) : quote ? (
          <>
            <div className="receipt-title">
              <div>
                <span>Checkout {quote.quote_id}</span>
                <h2>{quote.product}</h2>
              </div>
              <strong>{money(quote.total_cents, quote.currency)}</strong>
            </div>

            <div className="reservation-banner">
              <div>
                <span>Stock held</span>
                <strong>{quote.reservation_id}</strong>
              </div>
              <time dateTime={quote.reserved_until}>{countdown(remainingSeconds)}</time>
            </div>

            <dl className="receipt-lines">
              <div>
                <dt>Subtotal · {quote.quantity} items</dt>
                <dd>{money(quote.subtotal_cents, quote.currency)}</dd>
              </div>
              <div>
                <dt>Customer tier</dt>
                <dd>−{money(quote.tier_discount_cents, quote.currency)}</dd>
              </div>
              <div>
                <dt>PREVIEW10 coupon</dt>
                <dd>−{money(quote.coupon_discount_cents, quote.currency)}</dd>
              </div>
            </dl>

            <div className="service-list">
              {quote.services.map((service) => (
                <ServiceStamp key={service.service} service={service} />
              ))}
            </div>
          </>
        ) : (
          <div className="empty-result">
            <div className="empty-orbit" aria-hidden="true">
              <i />
              <i />
            </div>
            <h2>No request yet</h2>
            <p>The reservation will carry deployment identity from FastAPI and the private Go container.</p>
          </div>
        )}
      </div>
    </section>
  );
}
