import { expect, test } from "@playwright/test";
import { fixtures, mockApi } from "./mocks";

test("dashboard mostra ritmo, limite, performance, tipos de gasto, reserva e cortes", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");

  await expect(page.getByTestId("pace-card")).toContainText(
    "15% a mais que no mesmo período do mês passado",
  );
  await expect(page.getByTestId("limit-card")).toContainText("R$ 171,87 por dia");
  await expect(page.getByTestId("networth")).toContainText("11.611,87");

  const verdict = page.getByTestId("performance-verdict");
  await expect(verdict).toHaveAttribute("data-verdict", "NEGATIVE");
  await expect(verdict).toHaveAttribute("role", "alert");
  await expect(verdict).toContainText("Atenção");

  const spend = page.getByTestId("spendtype-card");
  await expect(spend).toContainText("Fixo");
  await expect(spend).toContainText("Cortável");
  await expect(spend).toContainText("Plano 50%");

  await expect(page.getByTestId("cut-card")).toContainText("Delivery da semana");
  await expect(page.getByTestId("cut-card")).toContainText("Acima da média");
});

test("performance negativa com economia é mostrada como tranquilizadora", async ({ page }) => {
  await mockApi(page);
  await page.route("**/api/insights/overview*", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        ...fixtures.overview,
        performance: {
          ...fixtures.overview.performance,
          savedCents: 300_000,
          verdict: "NEGATIVE_SAVED",
          message: "Você guardou R$ 3.000,00, por isso a performance ficou negativa.",
        },
      }),
    }),
  );
  await page.goto("/");

  const verdict = page.getByTestId("performance-verdict");
  await expect(verdict).toHaveAttribute("data-verdict", "NEGATIVE_SAVED");
  await expect(verdict).not.toHaveAttribute("role", "alert");
  await expect(verdict).toContainText("Tudo certo");
});

test("card da reserva cria a meta RESERVE com o valor sugerido", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/");

  await page
    .getByTestId("reserve-card")
    .getByRole("button", { name: "Criar minha reserva" })
    .click();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/goals")))
    .toBeDefined();
  const req = captured.find((r) => r.method === "POST" && r.url.endsWith("/api/goals"));
  expect(req?.body).toMatchObject({ kind: "RESERVE", targetCents: 2_520_000 });
});
