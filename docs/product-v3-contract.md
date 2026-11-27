# FinFlow v3 — contrato (recursos restantes do Visor, sem WhatsApp e Open Finance)

Continua o `docs/product-v2-contract.md` (mesmas convenções: centavos `int64`, datas `YYYY-MM-DD`, `FINFLOW_TZ`,
camelCase, erros de `internal/service/errors.go`, migrações aditivas idempotentes nos dois dialetos, testes em
cada camada). Nada de marcas de terceiros na UI. Fonte: landing, tabela de planos e calculadoras do Visor
(27/11/2026). Fora de escopo por decisão do usuário: WhatsApp e Open Finance.

| Recurso do Visor | No FinFlow | Seção |
|---|---|---|
| Tags e gastos por tag | tags em transações, filtro e relatório | 1 |
| Detecção de assinaturas | heurística local sobre o histórico | 2 |
| Dicas personalizadas / alertas | feed de alertas + sino | 3 |
| Ritmo de gastos (gráfico) e "quanto pode gastar hoje" | série acumulada + `spendToday` | 4 |
| Fluxo de caixa (regime de caixa) | página com competência/caixa | 5 |
| "Entender minhas faturas" | composição da fatura | 6 |
| Transferências inteligentes | casar despesa/receita entre contas próprias | 7 |
| "Posso comprar?" | simulador sobre a projeção | 8 |
| Desejos com link do produto | prévia do link (og:title/preço) | 9 |
| Investimentos | posições, rentabilidade, alocação | 10 |
| Empréstimos e financiamentos | dívidas com Price, quitação avalanche/bola de neve | 11 |
| Evolução do patrimônio | série mensal | 12 |
| Usar seus dados no Claude/ChatGPT, API pessoal | MCP via HTTP + página "API e IA" | 13 |
| App no celular + sincronização | modo rede local (LAN) + PWA instalável | 14 |
| Pergunte ao Visor (chat, foto e arquivo) | Assistente com Claude (chave do usuário) | 15 |
| Calculadoras (11) | `/calculadoras` | 16 |
| Metas com modelos (casa, filho, sabático...) | modelos de meta | 16 |
| Busca ⌘K, ocultar valores | paleta de comandos, modo privacidade | 16 |
| Espaços (casa/empresa), visão unificada, metas da casa, divisão | espaços = bancos SQLite separados | 17 (fase 2) |
| Visor Business: MRR, ARR, churn, DRE, runway | métricas do espaço empresa | 17 (fase 2) |

## 1. Tags

Tabelas `tags(id, name UNIQUE, color, created_at)` e `transaction_tags(transaction_id FK CASCADE, tag_id FK CASCADE,
PK(transaction_id, tag_id))`.

```ts
Tag { id; name; color: string; usageCount: number }
GET /api/tags · POST /api/tags {name, color?} · PATCH /api/tags/{id} · DELETE /api/tags/{id}
Transaction.tags: { id; name; color }[]          // sempre presente (array vazio)
CreateTransactionInput.tagIds?: string[] · UpdateTransactionInput.tagIds?: string[] (substitui o conjunto)
CreateCardExpenseInput.tagIds?: string[]          // aplicado a todas as parcelas
GET /api/transactions?tagId=...                   // filtro
GET /api/insights/tags?month&year → { tagId; name; color; totalCents; count }[]   // gasto (EXPENSE+CARD_EXPENSE) por tag
CategoryRule.tagId?: string | null                // regra pode também aplicar uma tag
```

## 2. Assinaturas detectadas

`GET /api/insights/subscriptions` →
```ts
{ items: { key: string; description: string; averageCents; lastCents; lastDate; occurrences: number;
           cadence: "WEEKLY" | "MONTHLY" | "YEARLY"; nextExpectedDate: string;
           priceChangePct: number | null;          // último vs. média anterior, se |Δ| >= 5%
           categoryId: string | null; accountId: string | null; cardId: string | null;
           recurringId: string | null }[];         // já acompanhada como recorrência
  monthlyTotalCents: number }                      // soma normalizada para mês (semanal×52/12, anual÷12)
POST /api/insights/subscriptions/{key}/track   → Recurring (EXPENSE, isSubscription=true, dia do último lançamento)
POST /api/insights/subscriptions/{key}/dismiss → 204 (chave guardada em settings; some da lista)
```
Heurística (pura em `domain`, com testes): últimos 13 meses de `EXPENSE` + `CARD_EXPENSE` sem parcelamento
(`installment_total` nulo ou 1); agrupa por descrição normalizada (minúsculas, sem acento, sem dígitos, datas,
`*`, códigos e sufixos de parcela; espaços colapsados); MONTHLY = ≥3 ocorrências com mediana dos intervalos
25–35 dias; WEEKLY = ≥4 com 6–8 dias; YEARLY = ≥2 com 330–400 dias; variação de valor ≤ 20% da mediana; a última
ocorrência não pode ser mais antiga que 1,5 ciclo. `Recurring.isSubscription: boolean` (coluna nova, default false).

## 3. Alertas e dicas

`GET /api/alerts` → `Alert[]` ordenados por severidade e data; `POST /api/alerts/{id}/dismiss` → 204.
```ts
Alert { id: string;            // determinístico: "<kind>:<ref>:<YYYY-MM>" (dispensar vale até mudar)
        kind: "BUDGET_WARNING" | "BUDGET_DANGER" | "LIMIT_PACE" | "PACE_ABOVE" | "NEGATIVE_PROJECTION"
            | "BILL_DUE" | "INVOICE_DUE" | "LOAN_DUE" | "SUBSCRIPTION_PRICE_UP" | "NEW_SUBSCRIPTION"
            | "CATEGORY_ABOVE_AVERAGE" | "RESERVE_LOW" | "GOAL_BEHIND" | "PENDING_OLD" | "UNCATEGORIZED"
            | "INVESTMENT_STALE";
        severity: "INFO" | "WARNING" | "DANGER"; title: string; message: string;   // pt-BR, com valores
        href: string;            // rota do app para agir
        date: string }
```
Regras: orçamento ≥70% (WARNING) / ≥90% (DANGER); LIMIT_PACE quando gasto ÷ dia × dias do mês > limite;
PACE_ABOVE quando `pace.status == ABOVE` e diff > 10%; NEGATIVE_PROJECTION quando `lowest < 0` em 30 dias
(DANGER); BILL_DUE/INVOICE_DUE/LOAN_DUE com vencimento em até 3 dias e não realizado; SUBSCRIPTION_PRICE_UP com
`priceChangePct >= 5`; NEW_SUBSCRIPTION para assinatura detectada com `occurrences == 3` e sem recorrência;
CATEGORY_ABOVE_AVERAGE = `cutSuggestions` ABOVE_AVERAGE; RESERVE_LOW quando a reserva cobre < 3 meses;
GOAL_BEHIND quando os aportes dos últimos 30 dias < `monthlyNeededCents`; PENDING_OLD pendentes com mais de
7 dias; UNCATEGORIZED ≥ 5 lançamentos do mês sem categoria; INVESTMENT_STALE valor não atualizado há 45 dias.
Dispensados ficam em `settings` (chave `dismissedAlerts`, mapa id → data; limpa entradas com mais de 90 dias).

## 4. Ritmo e "quanto pode gastar hoje"

`InsightsOverview.pace` ganha:
```ts
series: { day: number; currentCents: number | null; previousCents: number }[];  // acumulado; current null após hoje
projectedMonthCents: number;   // gasto até hoje ÷ dia × dias do mês (mês corrente; senão o total)
```
`InsightsOverview.spendToday: { cents: number; basis: "LIMIT" | "BALANCE" }` — com limite mensal:
`dailyAllowanceCents`; sem limite: `max(0, menor saldo projetado até o fim do mês) ÷ dias restantes (incl. hoje)`.

## 5. Fluxo de caixa

`GET /api/insights/cashflow?months=3&basis=CASH|ACCRUAL` (default `CASH`, `months` 1–12, termina no mês corrente
ou em `month/year` se enviados) →
```ts
{ basis; months: { month; year; incomeCents; fixedBillsCents; installmentsCents; dailyCents; totalOutCents;
                   resultCents; resultChangePct: number | null }[];    // resultChange vs. mês anterior
  whereItWent: { categoryId: string | null; name; icon: string | null; color: string | null; totalCents; percent }[] }  // último mês
```
CASH: compra no cartão conta no mês do **vencimento da fatura**; ACCRUAL: data da compra. `fixedBillsCents` =
despesas com `recurring_id` ou `loan_id`; `installmentsCents` = lançamentos com `installment_total > 1`;
`dailyCents` = o resto. Pagamento de fatura e transferências nunca contam.

## 6. Composição da fatura

`GET /api/cards/{id}/invoices/{invoiceId}/breakdown` →
```ts
{ purchasesCents; creditsCents; paymentsCents; totalCents;          // compras (+), estornos (valores positivos), pago
  composition: { installmentsCents; subscriptionsCents; oneOffCents } }   // assinatura = descrição casada com §2
```

## 7. Transferências inteligentes

`GET /api/transactions/transfer-candidates` →
`{ expense: Transaction; income: Transaction; daysApart: number }[]` — `EXPENSE` na conta A e `INCOME` na conta
B ≠ A, mesmo valor absoluto, datas a até 3 dias, ambas sem `transfer_group_id`, `recurring_id` e `loan_id`;
cada transação aparece no máximo em um par (menor distância de datas vence).
`POST /api/transactions/merge-transfer {expenseId, incomeId}` → `{transferGroupId}`: converte as duas pernas
em `TRANSFER` (mesmo grupo, categoria nula, descrições mantidas), numa transação de banco.

## 8. "Posso comprar?"

`POST /api/insights/can-afford {amountCents, date?, installments?: 1..48, cardId?, description?}` →
```ts
{ verdict: "YES" | "CAREFUL" | "NO";
  lowestBalanceAfterCents; lowestDate;              // menor saldo projetado (365 dias) com a compra
  limit: { limitCents; spentAfterCents; percentAfter } | null;   // impacto no limite do mês da compra
  reserveMonthsAfter: number | null;                // se a compra sair da reserva (quando saldo disponível não cobre)
  fitsOn: string | null;                            // 1ª data em que caberia sem saldo negativo (se verdict NO)
  message: string }                                 // pt-BR
```
Sem `cardId`: saída única do saldo disponível na `date` (default hoje). Com `cardId`: parcelas entram nas faturas
seguindo o ciclo do cartão (reutilizar o cálculo de fatura) e saem no vencimento. NO quando o menor saldo fica
< 0; CAREFUL quando fica abaixo da faixa amarela (`balanceBands.yellow`) ou estoura o limite mensal; YES no resto.

## 9. Prévia de link de desejo

`POST /api/wishlist/preview-link {url}` → `{ title: string | null; priceCents: number | null; imageUrl: string | null }`.
Só `http`/`https`; resolve o host e recusa loopback, link-local e redes privadas; timeout 5 s; lê no máximo 1 MiB;
segue até 3 redirecionamentos revalidando o host; extrai `og:title`/`<title>`, `product:price:amount` /
`og:price:amount` / JSON-LD `offers.price`. `WishItem.imageUrl: string | null` (coluna nova).

## 10. Investimentos

Tabelas `investments(id, account_id FK accounts NULL, name, kind, institution, invested_cents, current_cents,
notes, archived, value_updated_at, created_at, updated_at)` e `investment_snapshots(id, investment_id FK CASCADE,
date, value_cents, invested_cents)`.
```ts
InvestmentKind = "FIXED_INCOME" | "TREASURY" | "STOCKS" | "FII" | "FUNDS" | "CRYPTO" | "PENSION" | "OTHER"
Investment { id; name; kind; institution: string | null; accountId: string | null; accountName: string | null;
             investedCents; currentCents; returnCents; returnPct: number | null; valueUpdatedAt: string | null;
             notes: string | null; archived: boolean }
GET /api/investments · POST · PATCH /{id} · DELETE /{id}
POST /api/investments/{id}/value        {currentCents, date?}            // atualiza valor e grava snapshot
POST /api/investments/{id}/contribution {amountCents, date?, fromAccountId?}
     // soma em invested e current (grava snapshot). Com fromAccountId: cria TRANSFER da conta de origem para a
     // conta INVESTMENT vinculada; se a posição não tiver conta vinculada → 400 pedindo para vincular.
POST /api/investments/{id}/redeem       {amountCents, date?, toAccountId?}   // inverso
GET  /api/investments/summary → { totalInvestedCents; totalCurrentCents; returnCents; returnPct: number | null;
       byKind: { kind; currentCents; percent }[]; byInstitution: { institution; currentCents; percent }[];
       history: { month; year; currentCents; investedCents }[] }   // 12 meses a partir dos snapshots
```
Patrimônio: para cada conta `INVESTMENT` com posições vinculadas, usa a soma do `current` das posições em vez do
saldo contábil da conta; posições sem conta somam direto.

## 11. Empréstimos e financiamentos

Tabela `loans(id, name, lender, kind, principal_cents, monthly_rate_bp INTEGER /* centésimos de % ao mês */,
installment_cents, total_installments, paid_installments, first_due_date, account_id NULL, category_id NULL,
active, notes, created_at, updated_at)`; `transactions.loan_id` (FK SET NULL).
```ts
LoanKind = "PERSONAL" | "PAYROLL" | "VEHICLE" | "MORTGAGE" | "STUDENT" | "CARD_DEBT" | "OVERDRAFT" | "OTHER"
Loan { id; name; lender: string | null; kind; principalCents; monthlyRatePct: number; installmentCents;
       totalInstallments; paidInstallments; remainingInstallments; firstDueDate; nextDueDate: string | null;
       outstandingCents;              // valor presente das parcelas restantes (Price) com a taxa; sem taxa = soma
       totalInterestRemainingCents;   // parcelas restantes − outstanding
       accountId: string | null; categoryId: string | null; active: boolean; notes: string | null }
GET /api/loans · POST · PATCH /{id} · DELETE /{id}
POST /api/loans/{id}/pay {date?, amountCents?, accountId?} → Transaction   // EXPENSE com loan_id; paid += 1
GET  /api/loans/strategy?extraCents=0 →
  { avalanche: Plan; snowball: Plan; minimumOnly: Plan }
  Plan { order: string[]; months: number; totalInterestCents: number; payoffDate: string;
         perLoan: { loanId; payoffDate; interestCents }[] }
```
Simulação mês a mês: todas pagam a parcela; o extra (e as parcelas liberadas de dívidas quitadas) vai para a de
maior taxa (avalanche) ou menor saldo (bola de neve). Parcelas futuras entram na projeção (`events.kind` "LOAN"),
nos compromissos (`items.kind` "LOAN", `months[].loansCents`) e no patrimônio como dívida. Categoria padrão nova:
"Empréstimos e financiamentos" (FIXED) quando `categoryId` vazio.

## 12. Evolução do patrimônio

`GET /api/insights/net-worth?months=12` → `{ month; year; cashCents; investmentsCents; debtsCents; netWorthCents }[]`
(fim de cada mês). `cashCents` = contas não-INVESTMENT; `investmentsCents` = snapshots mais recentes até a data
(ou saldo contábil de contas INVESTMENT sem posições); `debtsCents` = faturas não pagas com vencimento até a data
+ saldo devedor de empréstimos naquela data.

## 13. MCP via HTTP e API pessoal

- `POST /api/mcp`: transporte Streamable HTTP (JSON-RPC 2.0; respostas `application/json`; `GET` → 405), mesmas
  tools do `finflow mcp`, protegido por `requireAuth` (Bearer). Reutiliza o dispatcher de `internal/mcp`.
- Novas tools MCP: `list_tags`, `get_subscriptions`, `get_alerts`, `can_afford`, `get_cashflow_basis`,
  `list_investments`, `get_investment_summary`, `list_loans`, `get_debt_strategy`, `get_net_worth_history`.
- `GET /api/system/info` → `{ version; dbDialect: "sqlite" | "postgres"; dataPath: string | null; authRequired;
  lanEnabled; lanUrls: string[]; aiConfigured: boolean; mcpHttpPath: "/api/mcp" }`.

## 14. Celular: rede local + PWA

- `finflow serve --lan` (ou `FINFLOW_LAN=1`) escuta em `0.0.0.0`; sem `--lan` continua só em `127.0.0.1`.
  Com `--lan` e sem `FINFLOW_PASSWORD`, recusa iniciar com mensagem explicando. `lanUrls` lista
  `http://<ip-privado>:<porta>` das interfaces ativas. Configurações mostram QR code para abrir no celular.
- PWA: `manifest.webmanifest` (nome, ícones 192/512 maskable, `display: standalone`, tema dos dois modos),
  service worker que guarda só o shell estático (nunca `/api/*`), registro só em produção.

## 15. Assistente (Pergunte ao FinFlow)

Usa a API do Claude com a chave do próprio usuário (SDK oficial em Go, Messages API; o loop de ferramentas roda
no binário). Modelo `claude-opus-5-5`, pensamento adaptativo (omitir `thinking`), `output_config.effort` explícito,
streaming, `fallbacks: "default"` (beta `server-side-fallback-2026-07-01`), tratar `stop_reason: "refusal"`,
cache de prompt no system + tools.

- Chave: `ANTHROPIC_API_KEY` (ou credenciais do `ant auth login`) ou salva em Configurações (tabela `settings`,
  nunca devolvida ao frontend). `GET /api/ai/status` → `{ configured; source: "ENV" | "SETTINGS" | null; model }`;
  `PUT /api/ai/key {apiKey: string | null}` → status.
- `POST /api/ai/chat` (SSE) `{ messages: { role: "user" | "assistant"; content: string }[] }` → eventos
  `text` `{delta}`, `tool` `{name, label}` (rótulo pt-BR, ex.: "Consultando sua projeção"), `proposal`
  `{id, kind: "CREATE_TRANSACTION" | "CREATE_RECURRING" | "CONTRIBUTE_GOAL" | "ADD_WISH", summary, payload}`,
  `done` `{stopReason}`, `error` `{message}`. Ferramentas de leitura = as tools de leitura do MCP + `can_afford`;
  **ferramentas de escrita só geram `proposal`** — o usuário confirma na UI, que chama o endpoint normal.
  Histórico mantido no cliente (localStorage).
- `POST /api/ai/receipt` (multipart, imagem JPEG/PNG/WebP/HEIC→recusar, ou PDF; até 10 MiB) → `{ description;
  amountCents; date; kind: "EXPENSE" | "INCOME"; suggestedCategoryId: string | null; merchant: string | null;
  installments: number | null; confidence: "HIGH" | "MEDIUM" | "LOW" }` via structured outputs. A UI abre o
  formulário de nova transação preenchido. Arquivo .xlsx/.csv/.ofx enviado no chat → encaminha para `/planilhas`.
- Sem chave: a página mostra o simulador "Posso comprar?" (§8) e como configurar.

## 16. Frontend

Rotas novas: `/fluxo` (§5), `/investimentos` (§10), `/dividas` (§11), `/assistente` (§15), `/calculadoras` (§16).
Navegação: Organização (+ Fluxo de caixa), Controle (+ Investimentos, Dívidas), Visão estratégica (+ Assistente,
Calculadoras). Também: sino de alertas no topo (§3) com contador; tags no extrato (chips, filtro, edição) e aba
Tags em Categorias; assinaturas detectadas em Compromissos ("Acompanhar" / "Ignorar"); gráfico de ritmo e
"Você pode gastar R$ X hoje" no dashboard; composição da fatura em Contas e cartões; sugestões de transferência
no extrato; prévia de link em Desejos; evolução do patrimônio no dashboard; "API e IA" em Configurações (status
da chave, snippet MCP stdio e HTTP, URL da API, QR da rede local); modelos de meta (Casa própria, Filho, Período
sabático, Viagem, Veículo, Casamento, Reserva); paleta ⌘K/Ctrl+K (navegar, buscar lançamentos, ações rápidas);
modo privacidade (olho) que mascara valores monetários e persiste por navegador.

Calculadoras (cálculo no cliente, centavos inteiros onde houver dinheiro, premissas editáveis e explicadas):
aluguel × compra (financiar vs. alugar e investir a diferença), carro × app de transporte, juros compostos,
simulador de pequenos gastos (valor × frequência → ano/10 anos/investido), 13º salário (INSS e IRRF em tabelas
editáveis, marcadas "valores de referência — confira a tabela vigente"), reserva de emergência (tamanho e prazo),
independência financeira (regra de retirada editável, padrão 4% a.a.), quitar dívidas (prazo e economia com
extra), financiamento Price × SAC, à vista × parcelado (com rendimento do dinheiro), CDB × poupança (IR
regressivo 22,5/20/17,5/15%; poupança 0,5% a.m. + TR com Selic > 8,5%, senão 70% da Selic + TR).

## 17. Fase 2 — Espaços e empresa (entregue depois da fase 1)

Espaços = bancos SQLite separados (`<dataDir>/spaces/<slug>.db`; o atual vira "Pessoal"). Cabeçalho
`X-FinFlow-Space` escolhe o espaço por requisição; tipos `PERSONAL | HOME | BUSINESS`. Visão unificada soma
saldos, patrimônio e resultado de todos os espaços. Espaço Casa: membros (nomes) e divisão de despesas
(quem pagou, % de cada) com saldo "quem deve a quem" e acerto por transferência. Espaço Empresa: DRE mensal,
runway (saldo ÷ queima média de 3 meses), MRR = receitas recorrentes ativas, ARR = 12 × MRR, churn = receitas
recorrentes desativadas no mês ÷ ativas no início. Em PostgreSQL, espaços = schemas. Detalhado num contrato próprio.
