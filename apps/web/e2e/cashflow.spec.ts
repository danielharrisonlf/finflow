import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test("fluxo de caixa alterna caixa/competência e o período", async ({ page }) => {
  await mockApi(page);
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/api/insights/cashflow")) requests.push(request.url());
  });
  await page.goto("/fluxo");

  await expect(page.getByRole("heading", { level: 1, name: "Fluxo de caixa" })).toBeVisible();
  await expect(page.getByTestId("cashflow-basis")).toHaveAttribute("data-basis", "CASH");
  await expect(page.getByTestId("cashflow-chart").locator("svg")).toBeVisible();
  await expect(page.getByTestId("cashflow-months")).toContainText("Nov 2026");
  await expect(page.getByTestId("cashflow-where")).toContainText("Moradia");
  expect(requests.some((u) => u.includes("basis=CASH") && u.includes("months=3"))).toBe(true);

  await page.getByRole("button", { name: "Competência" }).click();
  await expect(page.getByTestId("cashflow-basis")).toHaveAttribute("data-basis", "ACCRUAL");
  await expect(page.getByTestId("cashflow-where")).toContainText("Sem categoria");
  expect(requests.some((u) => u.includes("basis=ACCRUAL"))).toBe(true);

  await page.getByRole("button", { name: "6 meses" }).click();
  await expect.poll(() => requests.some((u) => u.includes("months=6"))).toBe(true);
});
