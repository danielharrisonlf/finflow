import { useEffect } from "react";
import { LandingPage as FinFlowLanding } from "../components/landing/LandingPage";
import { HOSTED_APP_URL } from "../lib/browser-runtime";
import { isBrowserRuntime } from "../runtime/client";

export function LandingPage() {
  useEffect(() => {
    const previous = document.title;
    document.title = "FinFlow — Seu dinheiro em ordem, no seu computador.";
    return () => {
      document.title = previous;
    };
  }, []);
  // modo navegador: o app mora em /app/ (BASE_URL); no binário, "/" e o app no navegador é o
  // publicado junto com a landing
  return (
    <FinFlowLanding
      accountHref={import.meta.env.BASE_URL}
      appHref={isBrowserRuntime ? import.meta.env.BASE_URL : HOSTED_APP_URL}
    />
  );
}
