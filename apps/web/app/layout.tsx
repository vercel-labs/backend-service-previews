import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Backend Preview / Checkout Lab",
  description: "Review a Next.js checkout, FastAPI orchestrator, and containerized Go reservation engine in one preview.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
