import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

// Valores conferidos com as funções puras de src/lib/calculators.

test.beforeEach(async ({ page }) => {
  await mockApi(page);
  await page.goto("/calculadoras");
  await expect(page.getByRole("heading", { level: 1, name: "Calculadoras" })).toBeVisible();
});

test("juros compostos: R$ 1.000 a 1% a.m.", async ({ page }) => {
  const calc = page.getByTestId("calc-compound");
  await expect(calc.getByTestId("compound-final")).toContainText("R$ 1.126,83");
  await calc.getByLabel("Prazo (meses)").fill("24");
  await expect(calc.getByTestId("compound-final")).toContainText("R$ 1.269,73");
  await calc.getByLabel("Aporte mensal").fill("100,00");
  await calc.getByLabel("Prazo (meses)").fill("12");
  await expect(calc.getByTestId("compound-final")).toContainText("R$ 2.395,08");
});

test("Price × SAC: 1ª e última parcela e juros totais", async ({ page }) => {
  const calc = page.getByTestId("calc-price-sac");
  await expect(calc.getByTestId("price-first")).toContainText("R$ 8.884,88");
  await expect(calc.getByTestId("price-last")).toContainText("R$ 8.884,88");
  await expect(calc.getByTestId("price-interest")).toContainText("R$ 6.618,55");
  await expect(calc.getByTestId("sac-first")).toContainText("R$ 9.333,33");
  await expect(calc.getByTestId("sac-last")).toContainText("R$ 8.416,67");
  await expect(calc.getByTestId("sac-interest")).toContainText("R$ 6.500,00");

  await calc.getByText("Ver tabela de amortização").click();
  const table = calc.getByTestId("amortization-table");
  await expect(table.locator("tbody tr")).toHaveCount(12);
  await calc.getByRole("button", { name: "SAC", exact: true }).click();
  await expect(table.locator("tbody tr").first()).toContainText("R$ 9.333,33");
});

test("CDB × poupança: IR regressivo pelo prazo", async ({ page }) => {
  const calc = page.getByTestId("calc-cdb");
  await expect(calc.getByTestId("cdb-ir-rate")).toContainText("17,5%");
  await expect(calc.getByTestId("cdb-net")).toContainText("R$ 1.229,25");
  await expect(calc.getByTestId("savings-gain")).toContainText("R$ 744,24");
  await expect(calc.getByTestId("cdb-verdict")).toContainText("O CDB rende");

  const days = calc.getByLabel("Prazo (dias corridos)");
  await days.fill("180");
  await expect(calc.getByTestId("cdb-ir-rate")).toContainText("22,5%");
  await days.fill("181");
  await expect(calc.getByTestId("cdb-ir-rate")).toContainText("20%");
  await days.fill("721");
  await expect(calc.getByTestId("cdb-ir-rate")).toContainText("15%");
});

test("13º salário com as tabelas de referência", async ({ page }) => {
  const calc = page.getByTestId("calc-13");
  await expect(calc.getByText("valores de referência — confira a tabela vigente")).toBeVisible();
  await expect(calc.getByTestId("thirteenth-inss")).toContainText("R$ 253,41");
  await expect(calc.getByTestId("thirteenth-irrf")).toContainText("R$ 23,83");
  await expect(calc.getByTestId("thirteenth-net")).toContainText("R$ 2.722,76");
  await expect(calc.getByTestId("thirteenth-first")).toContainText("R$ 1.500,00");
  await expect(calc.getByTestId("thirteenth-second")).toContainText("R$ 1.222,76");

  await calc.getByLabel("Salário bruto mensal").fill("6.000,00");
  await calc.getByLabel("Dependentes").fill("1");
  await expect(calc.getByTestId("thirteenth-inss")).toContainText("R$ 649,60");
  await expect(calc.getByTestId("thirteenth-irrf")).toContainText("R$ 510,49");
  await expect(calc.getByTestId("thirteenth-net")).toContainText("R$ 4.839,91");
});

test("pequenos gastos e quitar dívidas", async ({ page }) => {
  await expect(page.getByTestId("small-year")).toContainText("R$ 2.080,00");
  const debt = page.getByTestId("calc-debt");
  await expect(debt.getByTestId("debt-base")).toContainText("2 anos e 7 meses");
  await expect(debt.getByTestId("debt-extra")).toContainText("1 ano e 7 meses");
  await expect(debt.getByTestId("debt-saved")).toContainText("R$ 2.246,21");
});

test("reserva, independência financeira e à vista × parcelado", async ({ page }) => {
  await expect(page.getByTestId("emergency-target")).toContainText("R$ 24.000,00");
  await expect(page.getByTestId("fire-target")).toContainText("R$ 1.500.000,00");
  const cash = page.getByTestId("calc-cash-installments");
  await expect(cash.getByTestId("cash-pv")).toContainText("R$ 949,73");
  await expect(cash.getByTestId("cash-verdict")).toContainText("mesmo");
});

test("calculadoras não rolam na horizontal em 320px", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await page.goto("/calculadoras");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
