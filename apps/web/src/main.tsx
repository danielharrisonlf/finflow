import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { AppShell } from "./components/layout/app-shell";
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

function page(element: React.ReactNode): React.ReactNode {
  return <Suspense fallback={null}>{element}</Suspense>;
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 15_000, retry: 1, refetchOnWindowFocus: false },
  },
});

const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { path: "/", element: page(<DashboardPage />) },
      { path: "/extrato", element: page(<TransactionsPage />) },
      { path: "/contas", element: page(<AccountsPage />) },
      { path: "/orcamentos", element: page(<BudgetsPage />) },
      { path: "/metas", element: page(<GoalsPage />) },
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
