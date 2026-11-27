import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

test("lista, totais e comparação das estratégias de quitação", async ({ page }) => {
  await mockApi(page);
  await page.goto("/dividas");

  const totals = page.getByTestId("loans-totals");
  await expect(totals).toContainText("29.300,00");
  await expect(totals).toContainText("1.300,00");
  await expect(totals).toContainText("7.660,00");

  const list = page.getByTestId("loans-list");
  await expect(list).toContainText("Financiamento do carro");
  await expect(list).toContainText("Veículo");
  await expect(list).toContainText("1,49% a.m.");
  await expect(list).toContainText("12 de 48 parcelas");

  const strategy = page.getByTestId("debt-strategy");
  const avalanche = strategy.getByTestId("plan-avalanche");
  await expect(avalanche).toContainText("Avalanche");
  await expect(avalanche).toContainText("36 meses");
  await expect(avalanche).toContainText("6.900,00");
  await expect(avalanche).toContainText("Menos juros");
  await expect(avalanche).toContainText("Empréstimo pessoal");
  await expect(strategy.getByTestId("plan-snowball")).toContainText("Bola de neve");
  await expect(strategy.getByTestId("plan-minimumOnly")).toContainText("7.660,00");

  const refetch = page.waitForRequest((r) =>
    r.url().includes("/api/loans/strategy?extraCents=100000"),
  );
  await strategy.getByLabel("Extra por mês").fill("1.000,00");
  await refetch;
  await expect(avalanche).toContainText("31 meses");
  await expect(avalanche).toContainText("5.900,00");
  await expect(avalanche).toContainText("Janeiro 2029");
});

test("pagar parcela registra o pagamento na conta da dívida", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/dividas");

  await page.getByRole("button", { name: "Pagar parcela de Financiamento do carro" }).click();
  const form = page.getByTestId("form-pay-loan");
  await expect(form.getByLabel("Valor")).toHaveValue("890,00");
  await form.getByRole("button", { name: "Registrar pagamento" }).click();

  const find = () =>
    captured.find((r) => r.method === "POST" && r.url.includes(`/loans/${ids.loanCar}/pay`));
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({
    amountCents: 89_000,
    accountId: ids.accountNubank,
    date: "2026-11-15",
  });
});

test("nova dívida calcula a parcela Price e envia taxa e centavos", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/dividas");

  await page.getByRole("button", { name: "Nova dívida" }).click();
  const form = page.getByTestId("form-loan");
  await form.getByLabel("Nome", { exact: true }).fill("Empréstimo da reforma");
  await form.getByLabel("Valor emprestado").fill("10.000,00");
  await form.getByLabel("Taxa de juros (% ao mês)").fill("2");
  await form.getByLabel("Nº de parcelas").fill("24");
  await form.getByRole("button", { name: "Calcular parcela (Price)" }).click();
  await expect(form.getByLabel("Valor da parcela")).toHaveValue("528,71");
  await form.getByLabel("Primeira parcela vence em").fill("2026-12-10");
  await form.getByRole("button", { name: "Cadastrar dívida" }).click();

  const find = () => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/loans"));
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({
    name: "Empréstimo da reforma",
    kind: "PERSONAL",
    principalCents: 1_000_000,
    monthlyRatePct: 2,
    installmentCents: 52_871,
    totalInstallments: 24,
    paidInstallments: 0,
    firstDueDate: "2026-12-10",
  });
});
