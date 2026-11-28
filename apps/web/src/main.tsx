import { MutationCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { RuntimeGate } from "./components/browser/runtime-gate";
import { AppShell } from "./components/layout/app-shell";
import { Toaster } from "./components/ui/toaster";
import { askPersistentStorageOnce } from "./lib/browser-runtime";
import { PrivacyRerender } from "./lib/privacy";
import { registerServiceWorker } from "./lib/pwa";
import { subscribeSpace } from "./lib/space";
import { isBrowserRuntime } from "./runtime/client";
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
const SpacesPage = lazy(() => import("./pages/spaces").then((m) => ({ default: m.SpacesPage })));
const SplitPage = lazy(() => import("./pages/split").then((m) => ({ default: m.SplitPage })));
const BusinessPage = lazy(() =>
  import("./pages/business").then((m) => ({ default: m.BusinessPage })),
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
  // modo navegador: na primeira gravação de dados reais, pede armazenamento persistente
  mutationCache: isBrowserRuntime
    ? new MutationCache({ onSuccess: () => askPersistentStorageOnce() })
    : undefined,
  defaultOptions: {
    queries: { staleTime: 15_000, retry: 1, refetchOnWindowFocus: false },
  },
});

// Trocar de espaço (inclusive a volta automática para o pessoal) descarta tudo o que veio do
// espaço anterior; o AppShell remonta com a nova chave e refaz as consultas com o novo cabeçalho.
subscribeSpace(() => {
  void queryClient.cancelQueries();
  queryClient.clear();
});

const router = createBrowserRouter(
  [
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
        { path: "/espacos", element: page(<SpacesPage />) },
        { path: "/divisao", element: page(<SplitPage />) },
        { path: "/empresa", element: page(<BusinessPage />) },
        { path: "/configuracoes", element: page(<SettingsPage />) },
      ],
    },
  ],
  // modo navegador: o app mora em /app/ no site público
  { basename: isBrowserRuntime ? "/app" : undefined },
);

const root = document.getElementById("root");
if (!root) throw new Error("elemento #root não encontrado");

const app = <RouterProvider router={router} />;

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {isBrowserRuntime ? <RuntimeGate>{app}</RuntimeGate> : app}
      <Toaster />
    </QueryClientProvider>
  </StrictMode>,
);

// PWA: service worker só no build de produção (lib/pwa: servidor e navegador).
registerServiceWorker();
