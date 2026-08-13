import { CheckoutDemo } from "../components/checkout-demo";

export default function Home() {
  return (
    <main>
      <header className="topbar">
        <a href="https://vercel.com/docs/services" className="brand">
          Transit Supply
        </a>
      </header>

      <section className="intro">
        <p className="kicker">Backend preview / Stock reservation</p>
        <h1>
          Checkout that
          <br />
          <em>holds the stock.</em>
        </h1>
        <p className="summary">
          Reserve mock inventory for 15 minutes, then inspect which version of the Python orchestrator
          and Go reservation engine answered. Every preview keeps all three on the same commit.
        </p>
      </section>

      <CheckoutDemo />

      <footer>
        <span>One preview URL</span>
        <b>→</b>
        <span>Next.js + FastAPI</span>
        <b>→</b>
        <span>Private Go container</span>
      </footer>
    </main>
  );
}
