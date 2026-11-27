import { useEffect } from "react";
import { LandingPage as FinFlowLanding } from "../components/landing/LandingPage";

export function LandingPage() {
  useEffect(() => {
    const previous = document.title;
    document.title = "FinFlow — Seu dinheiro em ordem, no seu computador.";
    return () => {
      document.title = previous;
    };
  }, []);
  return <FinFlowLanding accountHref="/" />;
}
