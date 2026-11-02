import { useEffect } from "react";
import { LandingPage as FinFlowLanding } from "../components/landing/LandingPage";

export function LandingPage() {
  useEffect(() => {
    const previous = document.title;
    document.title = "FinFlow — O dinheiro organizado. A cabeça mais leve.";
    return () => {
      document.title = previous;
    };
  }, []);
  return <FinFlowLanding accountHref="/" />;
}
