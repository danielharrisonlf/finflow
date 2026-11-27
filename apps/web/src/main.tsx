import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { AppShell } from "./components/layout/app-shell";
import { PrivacyRerender } from "./lib/privacy";
import "./styles.css";

const DashboardPage = lazy(() =>
  import("./pages/dashboard").then((m) => ({ default: m.DashboardPage })),
);
const TransactionsPage = lazy(() =>
  import("./pages/transactions").then((m) => ({ default: m.TransactionsPage })),
);
const AccountsPage = lazy(() =>
  import("./pages/accounts").then((m) => ({ default: m.AccountsPage })),
);
const BudgetsPage = lazy(() => import("./pages/budgets").then((m) => ({ default: m.BudgetsPage })));
const GoalsPage = lazy(() => import("./pages/goals").then((m) => ({ default: m.GoalsPage })));
const CategoriesPage = lazy(() =>
  import("./pages/categories").then((m) => ({ default: m.CategoriesPage })),
);
const CommitmentsPage = lazy(() =>
  import("./pages/commitments").then((m) => ({ default: m.CommitmentsPage })),
);
const ProjectionPage = lazy(() =>
  import("./pages/projection").then((m) => ({ default: m.ProjectionPage })),
);
const YearPage = lazy(() => import("./pages/year").then((m) => ({ default: m.YearPage })));
const SpreadsheetsPage = lazy(() =>
  import("./pages/spreadsheets").then((m) => ({ default: m.SpreadsheetsPage })),
);
const SettingsPage = lazy(() =>
  import("./pages/settings").then((m) => ({ default: m.SettingsPage })),
);
const CashflowPage = lazy(() =>
  import("./pages/cashflow").then((m) => ({ default: m.CashflowPage })),
);
const InvestmentsPage = lazy(() =>
  import("./pages/investments").then((m) => ({ default: m.InvestmentsPage })),
);
const LoansPage = lazy(() => import("./pages/loans").then((m) => ({ default: m.LoansPage })));
const AssistantPage = lazy(() =>
  import("./pages/assistant").then((m) => ({ default: m.AssistantPage })),
);
const CalculatorsPage = lazy(() =>
  import("./pages/calculators").then((m) => ({ default: m.CalculatorsPage })),
);
const LandingPage = lazy(() => import("./pages/landing").then((m) => ({ default: m.LandingPage })));

/** Página lazy que re-renderiza ao alternar o modo privacidade (valores mascarados). */
function page(element: React.ReactNode): React.ReactNode {
  return (
    <Suspense fallback={null}>
      <PrivacyRerender>{element}</PrivacyRerender>
    </Suspense>
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 15_000, retry: 1, refetchOnWindowFocus: false },
  },
});

const router = createBrowserRouter([
  { path: "/inicio", element: page(<LandingPage />) },
  {
    element: <AppShell />,
    children: [
      { path: "/", element: page(<DashboardPage />) },
      { path: "/extrato", element: page(<TransactionsPage />) },
      { path: "/contas", element: page(<AccountsPage />) },
      { path: "/orcamentos", element: page(<BudgetsPage />) },
      { path: "/metas", element: page(<GoalsPage />) },
      { path: "/categorias", element: page(<CategoriesPage />) },
      { path: "/compromissos", element: page(<CommitmentsPage />) },
      { path: "/ano", element: page(<YearPage />) },
      { path: "/fluxo", element: page(<CashflowPage />) },
      { path: "/projecao", element: page(<ProjectionPage />) },
      { path: "/investimentos", element: page(<InvestmentsPage />) },
      { path: "/dividas", element: page(<LoansPage />) },
      { path: "/planilhas", element: page(<SpreadsheetsPage />) },
      { path: "/assistente", element: page(<AssistantPage />) },
      { path: "/calculadoras", element: page(<CalculatorsPage />) },
      { path: "/configuracoes", element: page(<SettingsPage />) },
    ],
  },
]);

const root = document.getElementById("root");
if (!root) throw new Error("elemento #root não encontrado");

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);

// PWA (v3 §14): service worker só no build de produção, guardando apenas o shell estático.
if (import.meta.env.PROD && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {
      // sem service worker o app segue funcionando normalmente
    });
  });
}
