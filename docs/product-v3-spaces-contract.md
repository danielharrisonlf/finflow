# FinFlow v3 fase 2 — Espaços (casa e empresa)

Complementa `docs/product-v3-contract.md` §17. Mesmas convenções. Inspirado nos planos "para a casa" e "para a
empresa" do Visor (espaço da casa em comum, espaço pessoal por pessoa, visão unificada, metas da casa; empresa com
contas PJ, MRR, ARR, churn, DRE, runway e projeção de 12 meses). Sem convites remotos nem nuvem: o FinFlow é local;
"membros" de um espaço são pessoas cadastradas para dividir despesas. O acesso de outra pessoa ao mesmo espaço é
pela rede local (`serve --lan`) com a senha.

## 1. Modelo

- Cada espaço é um banco **SQLite separado**. Arquivos: `<dataDir>/spaces/<slug>.db`, onde `dataDir` é a pasta do
  banco atual (`FINFLOW_DB_PATH`). O banco atual continua sendo o espaço `pessoal` (não é movido nem renomeado).
- Catálogo em `<dataDir>/spaces.json` (escrita atômica: arquivo temporário + rename):
  `{ "spaces": [{ "slug", "name", "kind": "PERSONAL"|"HOME"|"BUSINESS", "color", "createdAt", "file" }] }`.
  Sem catálogo = só `pessoal`.
- Em PostgreSQL (`DATABASE_URL`), cada espaço é um schema (`finflow_<slug>`) no mesmo banco; o `pessoal` é o schema
  configurado. Se isso complicar demais, o PostgreSQL pode ficar com um único espaço e a API responde 501 ao criar
  outro — documente a escolha.
- `slug`: `^[a-z0-9][a-z0-9-]{1,30}$`, único; `pessoal` é reservado e não pode ser apagado.

## 2. Seleção do espaço

- Cabeçalho `X-FinFlow-Space: <slug>` em qualquer rota `/api/*` (ausente = `pessoal`; slug inexistente = 404).
  Para downloads via link (`?token=`), aceitar também `?space=<slug>`.
- `internal/httpapi`: o `Server` passa a ter um registro de espaços (`*spaces.Registry`) que abre sob demanda e
  mantém em cache um `*service.Services` por espaço (abre o banco e roda `Migrate` + `Bootstrap` na primeira vez;
  fecha tudo no shutdown). Handlers trocam `s.svcs` por `s.svc(r)`. Rotas de auth, telegram e system não dependem
  de espaço (telegram usa sempre `pessoal`).
- CLI e MCP: flag global `--espaco <slug>` (e `FINFLOW_SPACE`); MCP HTTP respeita o cabeçalho.

## 3. API de espaços

```ts
Space { slug; name; kind: "PERSONAL" | "HOME" | "BUSINESS"; color: string; createdAt: string; isDefault: boolean }
GET    /api/spaces
POST   /api/spaces {name, kind, color?, slug?}   // slug derivado do nome se ausente; cria banco vazio com categorias
                                                 // padrão do tipo (BUSINESS: categorias de empresa, ver §5)
PATCH  /api/spaces/{slug} {name?, color?}
DELETE /api/spaces/{slug}?confirm=<slug>        // move o arquivo para <dataDir>/spaces/trash/<slug>-<timestamp>.db
                                                 // (não apaga de fato); 409 para "pessoal"
GET    /api/spaces/overview → { spaces: { slug; name; kind; balanceCents; netWorthCents; monthIncomeCents;
                                         monthOutCents; monthResultCents }[]; totals: {...mesmos campos} }   // visão unificada
```

## 4. Casa: membros e divisão de despesas

Tabelas no banco do espaço: `members(id, name, color, created_at)` e `transaction_splits(transaction_id FK CASCADE,
member_id FK CASCADE, share_bp INTEGER /* centésimos de % */, PRIMARY KEY(transaction_id, member_id))`,
`transactions.paid_by_member_id` (FK SET NULL).

```ts
Member { id; name; color }
GET/POST/PATCH/DELETE /api/members
Transaction.paidByMemberId: string | null; Transaction.splits: { memberId; sharePct: number }[]
CreateTransactionInput/UpdateTransactionInput: paidByMemberId?, splits?: { memberId; sharePct }[]  // soma = 100
CreateCardExpenseInput: paidByMemberId?, splits?
GET /api/members/balances?month&year →
  { members: { memberId; name; paidCents; owedCents; balanceCents }[];   // balance > 0: tem a receber
    settlements: { fromMemberId; toMemberId; amountCents }[] }           // mínimo de acertos (guloso)
POST /api/members/settle {fromMemberId, toMemberId, amountCents, date?, fromAccountId?, toAccountId?}
  → registra acerto: transação TRANSFER entre contas se as duas forem informadas, senão um registro de acerto
    (tabela `settlements(id, from_member_id, to_member_id, amount_cents, date)`) que abate os saldos
```
Despesa sem `splits` num espaço Casa = dividida igualmente entre todos os membros. Arredondamento: o resto em
centavos vai para quem pagou.

## 5. Empresa: métricas

Categorias padrão de empresa: Receitas (Vendas, Serviços, Assinaturas de clientes, Outras receitas), Deduções
(Impostos sobre vendas — FIXED), Custos (Fornecedores, Infraestrutura/cloud — CUTTABLE, Comissões), Despesas
operacionais (Pró-labore, Salários e encargos — FIXED; Aluguel, Contabilidade — FIXED; Software e ferramentas —
CUTTABLE; Marketing — VARIABLE; Tarifas bancárias — CUTTABLE), Financeiras (Juros, Empréstimos — FIXED).
Coluna `categories.dre_group`: `REVENUE | DEDUCTIONS | COSTS | OPEX | FINANCIAL | OTHER` (default por heurística
de nome; editável).

```ts
GET /api/business/metrics?month&year →
  { mrrCents;            // soma das recorrências INCOME ativas no fim do mês, normalizadas por mês
    arrCents;            // 12 × MRR
    newMrrCents; churnedMrrCents;
    churnPct: number | null;   // churnedMrr ÷ MRR no início do mês
    burnCents;           // média das saídas − entradas dos 3 meses anteriores (0 se positivo)
    runwayMonths: number | null;   // saldo disponível ÷ burn; null se burn = 0
    cashCents }
GET /api/business/dre?year → { months: { month; revenueCents; deductionsCents; netRevenueCents; costsCents;
                                           grossProfitCents; opexCents; operatingResultCents; financialCents;
                                           netResultCents; marginPct: number | null }[]; totals: {...} }
```
Uma recorrência INCOME "desativada no mês" = `active` virou false ou `endDate` caiu no mês (gravar
`deactivated_at` em `recurring` quando `active` muda para false).

## 6. Frontend

Seletor de espaço no topo da sidebar (e no "Mais" do mobile) com cor e tipo, "Novo espaço" (nome, tipo, cor) e
"Visão unificada" (`/espacos`: cards por espaço e totais). O espaço ativo fica em localStorage e vai no cabeçalho
`X-FinFlow-Space` de todas as chamadas (inclusive SSE do assistente e downloads via `?space=`); trocar de espaço
limpa o cache do TanStack Query. Espaço Casa: página `/divisao` (membros, saldos, acertos sugeridos, registrar
acerto) e campos "Quem pagou" + divisão nos formulários de transação (só nesse tipo de espaço). Espaço Empresa:
página `/empresa` (MRR, ARR, churn, runway, burn, DRE anual em tabela, gráfico de 12 meses) e grupo DRE nas
categorias. Metas da casa = metas do espaço Casa (nada novo). Na sidebar, cada tipo mostra só as páginas
relevantes (Empresa esconde Desejos e Reserva; Pessoal e Casa escondem Empresa).
