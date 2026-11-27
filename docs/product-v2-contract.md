# FinFlow v2 — contrato de produto e API

Fonte de verdade para a rodada "local-first + planilhas + planejamento". Backend (Go), frontend (React) e
landing seguem este documento. Dinheiro sempre em **centavos inteiros** (`int64` / `number`), datas
`YYYY-MM-DD`, "hoje" segue `FINFLOW_TZ`. Respostas JSON em camelCase. Erros seguem o padrão já existente
em `internal/httpapi` / `internal/service/errors.go`.

## Inspirações (o que portamos e por quê)

- **Método de fluxo de caixa diário (planilha Entrada · Saída · Diário · Saldo)**: separar *Saídas*
  (custo de vida fixo, recorrente) de *Gastos diários* (variáveis); **Performance = Entradas − (Saídas + Diário)**;
  **Economia** mensal (quanto foi guardado) e **taxa de economia = Economia ÷ Entradas**; saldo projetado dia a dia
  colorido por faixas (verde escuro > R$ 2.000, verde claro R$ 1.000–2.000, amarelo R$ 0–1.000, vermelho claro
  R$ 0 a −R$ 500, vermelho < −R$ 500); **menor saldo nos próximos 30 dias** como alerta; guardar dinheiro quando o
  saldo projetado fica positivo até a próxima entrada; **reserva de emergência = 6 meses de custo de vida (12 com
  dependentes)**; repor a reserva com **+10%**. Na UI pública não citamos marcas/autores de terceiros: o formato é
  chamado de "planilha diária (Entrada · Saída · Diário · Saldo)".
- **Visor Finance**: ritmo do mês (gasto até hoje vs. mesmo período do mês passado), limite mensal de gastos,
  tipo de gasto **Fixo / Cortável / Variável** com "onde dá para cortar", compromissos (contas fixas + parcelas,
  próximos 6 meses, a pagar / a receber), fluxo de caixa (receitas, contas fixas, parcelas, dia a dia), projeção de
  12 meses (saldo atual + receitas recorrentes − recorrências − parcelas − gastos avulsos), patrimônio, reserva de
  emergência, **desejos** (lista de compras priorizada com "quando cabe no saldo") e **automações** (regras de
  categorização).

## 1. Mudanças de modelo

### 1.1 Categoria — tipo de gasto

`spendType: "FIXED" | "CUTTABLE" | "VARIABLE" | null` (Fixo / Cortável / Variável). Obrigatório só em `EXPENSE`
(default `VARIABLE`); `null` em `INCOME`. Coluna `categories.spend_type` (migração aditiva, `ALTER TABLE ... ADD
COLUMN` idempotente nos dois dialetos). Subcategoria sem valor próprio herda do pai.

Mapeamento para o método da planilha: `FIXED` + `CUTTABLE` → **Saídas**; `VARIABLE` (e despesa sem categoria) →
**Gastos diários (Diário)**.

`Category`, `CreateCategoryInput`, `UpdateCategoryInput` ganham `spendType`.

Categorias padrão (seed e bancos vazios no boot): taxonomia brasileira inspirada no Visor com subcategorias e
`spendType` coerente — p.ex. Moradia (Aluguel FIXED, IPTU FIXED, Condomínio FIXED), Utilidades (Energia, Água, Gás
FIXED), Serviços (Internet, Celular FIXED; TV CUTTABLE), Serviços digitais (Streaming, Música, Jogos CUTTABLE),
Alimentação (Supermercado, Restaurantes, Delivery VARIABLE), Transporte, Automotivo, Saúde (Seguro saúde FIXED,
Farmácia VARIABLE), Educação (FIXED), Lazer, Compras, Viagens, Seguros (FIXED), Empréstimos e financiamentos (FIXED),
Impostos, Tarifas bancárias (CUTTABLE), Doações, Pet, Outros. Receitas: Salário, Freelance, Rendimentos, Reembolsos,
Outras receitas. Nunca duplicar categorias já existentes (UNIQUE(name, kind)); bancos existentes só ganham
`spend_type` preenchido por heurística de nome quando estiver NULL.

### 1.2 Transação — vínculo com recorrência

`transactions.recurring_id` (nullable, FK `recurring(id) ON DELETE SET NULL`). `Transaction.recurringId`.

### 1.3 Meta — reserva de emergência

`goals.kind: "GOAL" | "RESERVE"` (default `GOAL`). No máximo uma `RESERVE` não arquivada (409 se tentar criar
outra). `Goal.kind`, `CreateGoalInput.kind?`.

## 2. Novos recursos

### 2.1 Recorrências (Compromissos: contas fixas e receitas recorrentes)

```ts
Recurring {
  id: string; description: string;
  kind: "INCOME" | "EXPENSE";
  amountCents: number;          // > 0
  dayOfMonth: number;           // 1..31 (meses curtos → último dia)
  accountId: string | null; accountName: string | null;
  categoryId: string | null; categoryName: string | null; categoryIcon: string | null; categoryColor: string | null;
  startDate: string;            // primeira ocorrência possível
  endDate: string | null;
  active: boolean;
  nextDate: string | null;      // próxima ocorrência ainda não lançada (>= hoje ou atrasada do mês corrente)
}
GET    /api/recurring
POST   /api/recurring                {description, kind, amountCents, dayOfMonth, accountId?, categoryId?, startDate?, endDate?}
PATCH  /api/recurring/{id}           (campos opcionais + active)
DELETE /api/recurring/{id}
POST   /api/recurring/{id}/post      {date?, amountCents?, accountId?}  → Transaction
```

`post` lança a ocorrência do mês da `date` (default: data da ocorrência do mês corrente) como transação
`INCOME`/`EXPENSE` confirmada com `recurring_id`. Uma ocorrência do mês M é **realizada** se existe transação com
esse `recurring_id` e data em M — realizadas não entram na projeção (sem dupla contagem). `accountId` obrigatório
para lançar (do corpo ou da recorrência).

### 2.2 Configurações

```ts
Settings {
  monthlyLimitCents: number | null;     // limite mensal de gastos (Visor)
  reserveMonths: number;                // default 6
  hasDependents: boolean;               // se true, meta sugerida usa 12 meses
  reserveRefillRatePct: number;         // default 10
  balanceBands: { darkGreen: number; lightGreen: number; yellow: number; lightRed: number }
                                        // defaults 200000, 100000, 0, -50000 (limites inferiores de cada faixa)
  spendPlan: { fixedPct: number; cuttablePct: number; variablePct: number } | null;  // "Seu plano"
}
GET   /api/settings
PATCH /api/settings   (merge parcial)
```

Tabela `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL)` com JSON por chave.

### 2.3 Regras de categorização (Automações)

```ts
CategoryRule { id; pattern: string; categoryId: string; categoryName: string; priority: number }
GET /api/rules · POST /api/rules {pattern, categoryId, priority?} · PATCH /api/rules/{id} · DELETE /api/rules/{id}
POST /api/rules/apply {onlyUncategorized?: boolean = true} → {updated: number}
```

`pattern` casa por substring, sem diferenciar maiúsculas nem acentos. Maior `priority` vence; empate → padrão mais
longo. A categoria só é aplicada se o `kind` da categoria combina com a transação (INCOME ↔ categoria INCOME;
EXPENSE/CARD_EXPENSE ↔ EXPENSE). Aplicada automaticamente quando `categoryId` não é enviado em: criar transação,
compra no cartão, despesa via Telegram e importação. Serviço exporta
`func (s *Services) SuggestCategory(ctx, description string, kind domain.CategoryKind) (*string, error)`.

### 2.4 Desejos

```ts
WishItem {
  id; name: string; priceCents: number; url: string | null; notes: string | null;
  priority: number;                       // menor = mais prioritário; ordem da lista
  status: "LISTED" | "BOUGHT" | "ARCHIVED";
  affordableOn: string | null;            // 1ª data (YYYY-MM-DD) em que o saldo projetado comporta este desejo
                                          // somado aos desejos LISTED de maior prioridade e permanece >= 0 até o fim
                                          // do horizonte de 365 dias; null se não couber no horizonte
}
GET /api/wishlist · POST /api/wishlist · PATCH /api/wishlist/{id} · DELETE /api/wishlist/{id}
```

## 3. Insights

Definições comuns (mês M):
- **Entradas** = `INCOME` confirmadas no mês (transferências nunca contam).
- **Gasto** = `EXPENSE` + `CARD_EXPENSE` do mês por data da compra (competência, igual aos orçamentos), incluindo
  pendentes. `INVOICE_PAYMENT` não conta (evita dupla contagem).
- **Saídas (fixed)** = gasto em categorias `FIXED` + `CUTTABLE`. **Diário (variable)** = `VARIABLE` + sem categoria.
- **Economia (saved)** = transferências de contas líquidas (corrente, carteira, salário) que **entram** em contas
  `SAVINGS`/`INVESTMENT` no mês. Movimentação entre duas contas de reserva não conta. Aportes em metas só contam
  quando não existe nenhuma conta de reserva ativa (senão o mesmo dinheiro seria contado duas vezes).
- **Saldo disponível (líquido)** = contas ativas exceto `SAVINGS`/`INVESTMENT` — é a base da projeção, dos
  compromissos e da planilha diária exportada ("a conta corrente é passagem; o cofre fica à parte").
- **Performance** = Entradas − (Saídas + Diário).

### 3.1 `GET /api/insights/overview?month&year`

```ts
{
  month; year;
  pace: { currentCents; previousSamePeriodCents; diffCents; dayOfMonth; status: "BELOW" | "SAME" | "ABOVE" };
        // gasto do dia 1 até hoje (ou fim do mês se M passado) vs. mesmo intervalo de dias do mês anterior;
        // SAME se |diff| <= 5% do anterior (ou ambos 0)
  monthlyLimit: { limitCents: number | null; spentCents; remainingCents: number | null;
                  dailyAllowanceCents: number | null; percent: number | null };
        // dailyAllowance = remaining ÷ dias restantes incluindo hoje (só mês corrente; >= 0)
  performance: { incomeCents; fixedCents; variableCents; totalOutCents; savedCents; performanceCents;
                 savingsRatePct: number | null; previousPerformanceCents;
                 verdict: "SAVING" | "POSITIVE" | "TIGHT" | "NEGATIVE_SAVED" | "NEGATIVE"; message: string };
        // NEGATIVE: performance<0 (gastos acima das entradas — alarme, mesmo que tenha guardado)
        // NEGATIVE_SAVED: performance>=0 mas guardou mais do que sobrou (usou saldo anterior — tudo bem)
        // SAVING: guardou e ainda sobrou · TIGHT: 0<=performance<5% das entradas · POSITIVE: resto
        // message em pt-BR explicando o veredito
  spendTypes: { spendType: "FIXED" | "CUTTABLE" | "VARIABLE"; totalCents; percent;
                categories: { categoryId: string | null; name; icon: string | null; color: string | null; totalCents }[] }[];
  plan: { fixedPct; cuttablePct; variablePct } | null;   // de settings.spendPlan
  netWorthCents: number;   // soma dos saldos das contas ativas − faturas não pagas (abertas/fechadas)
  reserve: { goalId: string | null; avgMonthlyCostCents; suggestedTargetCents; targetCents; savedCents;
             monthsCovered: number; status: "NONE" | "BUILDING" | "PROTECTED" };
        // avgMonthlyCost = média de (Saídas + Diário) dos 3 últimos meses completos com algum gasto
        // suggestedTarget = avg × (hasDependents ? 12 : reserveMonths); target = goal.target ou suggested
        // monthsCovered = saved ÷ avg (0 se avg = 0); PROTECTED quando saved >= target
  cutSuggestions: { kind: "CANCEL" | "REDUCE" | "ABOVE_AVERAGE"; categoryId: string | null; name; currentCents; averageCents }[];
        // CANCEL: categorias CUTTABLE com gasto no mês · ABOVE_AVERAGE: VARIABLE > 120% da média dos 3 meses anteriores
        // REDUCE: as 3 maiores VARIABLE do mês (sem repetir as anteriores)
}
```

### 3.2 `GET /api/insights/projection?days=90`

Simulação diária a partir de hoje (horizonte interno de 365 dias; `days` limita só a lista retornada, máx. 365).

```ts
{
  startDate; startBalanceCents;           // saldo disponível: contas ativas exceto SAVINGS/INVESTMENT
  limitConfigured: boolean;
  days: { date; inCents; outCents; balanceCents;
          band: "DARK_GREEN" | "LIGHT_GREEN" | "YELLOW" | "LIGHT_RED" | "RED";
          events: { kind: "RECURRING" | "INVOICE" | "PENDING" | "DAILY_SPEND"; description; amountCents }[] }[];
          // amountCents com sinal (+ entra, − sai)
  lowest: { date; balanceCents };         // menor saldo nos próximos 30 dias
  nextIncomeDate: string | null;          // próxima entrada prevista (recorrência INCOME ou pendente INCOME)
  canSaveNowCents: number;                // max(0, menor saldo entre hoje e nextIncomeDate (ou 30 dias))
  months: { month; year; incomeCents; recurringOutCents; installmentsCents; invoicesCents; dailySpendCents; endBalanceCents }[];  // 12
}
```

Eventos: transações `PENDING` de conta com data >= hoje (atrasadas entram hoje); ocorrências de recorrências ativas
ainda não realizadas (a do mês corrente entra hoje se o dia já passou); faturas não pagas no `dueDate` (total da
fatura; parcelas futuras já estão nas faturas futuras — `installmentsCents` é a parte da fatura vinda de parcelas,
informativo); gasto diário estimado quando há limite mensal: `orçamentoVariável(M) = max(0, limite − recorrências
EXPENSE de M − parcelas de cartão de M)`; no mês corrente `max(0, orçamentoVariável − Diário já gasto) ÷ dias
restantes incluindo hoje`; meses futuros `orçamentoVariável ÷ dias do mês`.

### 3.3 `GET /api/insights/year?year=2026`

```ts
{ year;
  months: { month; incomeCents; fixedCents; variableCents; totalOutCents; performanceCents; savedCents;
            savingsRatePct: number | null; endBalanceCents }[];   // 12; endBalance = saldo total no fim do mês
  totals: { incomeCents; fixedCents; variableCents; totalOutCents; performanceCents; savedCents; savingsRatePct: number | null } }
```

### 3.4 `GET /api/insights/commitments?month&year`

```ts
{ months: { month; year; recurringExpenseCents; recurringIncomeCents; installmentsCents }[];   // 6 a partir de M
  items: { kind: "RECURRING" | "INSTALLMENT"; direction: "PAY" | "RECEIVE"; description; dueDate; amountCents;
           recurringId: string | null; realized: boolean; installmentNumber: number | null;
           installmentTotal: number | null; cardName: string | null }[];   // do mês M, ordenado por dueDate
  totals: { payCents; receiveCents; balanceAfterCents } }   // balanceAfter = saldo atual + receber − pagar ainda não realizados
```

Parcelas = lançamentos `CARD_EXPENSE` com `installment_total > 1` cuja fatura vence em M (`dueDate` da fatura).

## 4. Planilhas (import/export)

Pacote puro `internal/spreadsheet` (sem banco) + serviço `internal/service/imports.go` + handlers em
`internal/httpapi/spreadsheet.go`. Biblioteca: `github.com/xuri/excelize/v2`. Limite de corpo de 10 MiB nestas rotas.

### 4.1 Exportar

- `GET /api/spreadsheet/export.xlsx?year=2026` — pasta de trabalho com:
  1. aba `<ano>`: layout diário em 12 blocos (Data · Entrada · Saída · Diário · Saldo; 31 linhas por mês, dias
     inexistentes em cinza), **fórmulas vivas** de saldo encadeado entre meses, totais (ENTRADAS/SAÍDAS/DIÁRIO,
     Saída total, Performance), cores condicionais das faixas de saldo (de `settings.balanceBands`), painéis
     congelados. Primeiro dia de janeiro inclui o saldo de abertura (saldo total no fim do ano anterior) como Entrada
     anotada. Compras no cartão entram no **vencimento da fatura** (regime de caixa); pagamento de fatura não entra
     de novo. Transferências para contas SAVINGS/INVESTMENT entram como Saída e na aba Economia.
  2. aba `Economia`: jan–dez × Entradas (fórmula ligada à aba do ano) · Economia · % (com `IFERROR`), TOTAL.
  3. aba `Lançamentos`: tabela com Data, Descrição, Tipo, Categoria, Tipo de gasto, Conta/Cartão, Valor (R$),
     Status, Parcela — com filtro automático.
  4. aba `Categorias`: matriz categoria × mês com totais.
- `GET /api/transactions/export?format=xlsx` — mesmos filtros do extrato, uma aba `Lançamentos`.
- `GET /api/spreadsheet/template.xlsx` — modelo de importação: aba `Lançamentos` (Data · Descrição · Valor · Tipo ·
  Categoria · Conta, com 3 exemplos e validação de lista em Tipo) + aba `Como usar`.

### 4.2 Importar

```ts
POST /api/import/preview   multipart/form-data: file (xlsx|xls? não|csv|ofx), mapping? (JSON), sheet? (string)
→ { format: "DAILY_SHEET" | "TABLE" | "CSV" | "OFX";
    sheets: string[]; sheet: string | null;
    columns: string[];                               // cabeçalhos detectados (TABLE/CSV)
    mapping: { date: number | null; description: number | null; amount: number | null; debit: number | null;
               credit: number | null; type: number | null; category: number | null; account: number | null };
    rows: { index: number; date: string; description: string; amountCents: number; kind: "INCOME" | "EXPENSE";
            categoryName: string | null; suggestedCategoryId: string | null; accountName: string | null;
            duplicate: boolean; error: string | null }[];
    warnings: string[] }

POST /api/import/commit    { accountId: string; skipDuplicates?: boolean = true;
                             rows: { date; description; amountCents; kind; categoryId?: string | null; status?: "CONFIRMED" | "PENDING" }[] }
→ { created: number; skipped: number }
```

- **DAILY_SHEET**: abas com nome `^\d{4}$`; blocos achados pelo cabeçalho `Data|Entrada|Saída|Diário|Saldo`
  (sem acento/maiúsculas) com fallback de coluna `1 + 6·(mês−1)`; mês pelo título do bloco. Cada célula não vazia
  vira uma transação agregada do dia: Entrada → INCOME "Entrada (planilha)", Saída → EXPENSE "Saída (planilha)"
  (sugere categoria FIXED/"Custo de vida"), Diário → EXPENSE "Gasto diário (planilha)". Valores lidos do cache de
  fórmulas; negativos invertem o tipo; dias impossíveis geram warning. Saldo recalculado e comparado ao `Saldo`
  da planilha (divergência → warning).
- **TABLE/CSV**: detecção de cabeçalho por sinônimos pt-BR/en (data, descrição/histórico/lançamento, valor, débito,
  crédito, tipo, categoria, conta); CSV com `;`, `,` ou tab, UTF-8 (com/sem BOM) ou Windows-1252; datas `dd/mm/aaaa`,
  `dd/mm/aa`, `aaaa-mm-dd` e seriais do Excel; valores `R$ 1.234,56`, `1,234.56`, `-10`, `(10,00)`, `10 D`/`10 C`.
  Sem coluna de tipo: valor negativo → EXPENSE, positivo → INCOME (ou débito/crédito separados).
- **OFX**: `STMTTRN` (TRNAMT, DTPOSTED, MEMO/NAME).
- **duplicate**: já existe transação na mesma conta (se `accountId` enviado no preview), mesma data, mesmo valor e
  descrição normalizada igual — ou repetida dentro do próprio arquivo.
- `suggestedCategoryId`: nome de categoria igual (sem acento) → regra de automação (`SuggestCategory`).
- Commit cria transações na conta escolhida, aplica regras quando `categoryId` ausente, tudo numa transação de banco.

## 5. CLI e MCP

CLI: `finflow importar <arquivo> --conta <nome> [--simular]`, `finflow exportar --formato xlsx [--ano 2026]`,
`finflow projecao [--dias 30]`, `finflow fixas` (lista recorrências), `finflow insights [--mes --ano]`.
MCP: `get_insights`, `get_projection`, `get_year_overview`, `list_recurring`, `create_recurring`.

## 6. Frontend (rotas)

Navegação agrupada: **Organização** (Visão geral `/`, Transações `/extrato`, Compromissos `/compromissos`,
Ano `/ano`) · **Controle** (Contas e cartões `/contas`, Categorias `/categorias`, Orçamentos `/orcamentos`,
Metas e desejos `/metas`) · **Visão estratégica** (Projeção `/projecao`, Planilhas `/planilhas`) · Configurações
`/configuracoes`. Rotas atuais preservadas.
