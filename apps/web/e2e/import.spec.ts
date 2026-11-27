import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

const CSV =
  "Data;Histórico;Valor\n03/11/2026;IFOOD *RESTAURANTE;-45,90\n05/11/2026;SALARIO;9850,00\n";

test("importa CSV: preview com duplicadas desmarcadas e commit das linhas escolhidas", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/planilhas");

  await page.getByTestId("import-file").setInputFiles({
    name: "extrato.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(CSV, "utf8"),
  });

  const preview = page.getByTestId("import-preview");
  await expect(preview).toBeVisible();
  await expect(page.getByTestId("import-format")).toContainText("CSV");
  await expect(page.getByTestId("import-warnings")).toContainText("desmarcada");

  // upload foi multipart com o arquivo e a conta de destino
  const upload = captured.find((r) => r.url.includes("/import/preview"));
  expect(String(upload?.body)).toContain('name="file"');
  expect(String(upload?.body)).toContain('name="accountId"');

  const rows = preview.getByTestId("import-row");
  await expect(rows).toHaveCount(3);
  const duplicate = preview.locator('[data-duplicate="true"]');
  await expect(duplicate.getByText("Duplicado")).toBeVisible();
  await expect(duplicate.getByRole("checkbox")).not.toBeChecked();
  await expect(rows.first().getByRole("combobox")).toHaveValue(ids.catFood);

  await page.getByRole("button", { name: "Importar 2 lançamentos" }).click();
  await expect(page.getByTestId("import-result")).toContainText("2 lançamentos importados");

  const commit = captured.find((r) => r.url.includes("/import/commit"));
  expect(commit?.body).toMatchObject({
    accountId: ids.accountNubank,
    skipDuplicates: false,
    rows: [
      { amountCents: 4_590, kind: "EXPENSE", categoryId: ids.catFood, date: "2026-11-03" },
      { amountCents: 985_000, kind: "INCOME", categoryId: null },
    ],
  });
});

test("cards de exportação apontam para os downloads xlsx", async ({ page }) => {
  await mockApi(page);
  await page.goto("/planilhas");

  await expect(
    page.getByTestId("export-year").getByRole("link", { name: /Baixar \d{4}/ }),
  ).toHaveAttribute("href", /\/api\/spreadsheet\/export\.xlsx\?year=\d{4}/);
  await expect(page.getByTestId("export-template").getByRole("link")).toHaveAttribute(
    "href",
    /\/api\/spreadsheet\/template\.xlsx/,
  );
  await expect(page.getByTestId("export-transactions").getByRole("link")).toHaveAttribute(
    "href",
    /\/api\/transactions\/export\?.*format=xlsx/,
  );
});

test("extrato oferece exportação Excel com os filtros", async ({ page }) => {
  await mockApi(page);
  await page.goto("/extrato");
  await expect(page.getByRole("link", { name: /Excel \(\.xlsx\)/ })).toHaveAttribute(
    "href",
    /\/transactions\/export\?.*month=.*format=xlsx/,
  );
});
