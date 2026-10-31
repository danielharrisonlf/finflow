# FinFlow

Controle financeiro pessoal: contas, cartões de crédito com faturas e parcelamentos, extrato com exportação, orçamentos por categoria e metas. Uma API alimenta quatro interfaces: web (desktop e mobile), CLI, servidor MCP para agentes de IA e bot do Telegram.

## Stack

| Camada | Tecnologia |
| --- | --- |
| Monorepo | pnpm 10 workspaces + Turborepo, Biome, TypeScript 6 |
| API (`apps/api`) | NestJS 12, Prisma 7 (adapter `pg`), PostgreSQL 16, Zod 4, Swagger |
| Web (`apps/web`) | React 19, Vite 8, Tailwind v4, Radix UI, TanStack Query, React Hook Form, Recharts |
| CLI (`apps/cli`) | Commander, picocolors |
| MCP (`apps/mcp`) | `@modelcontextprotocol/sdk` (stdio) |
| Contratos (`packages/contracts`) | Schemas Zod compartilhados entre API, web, CLI e MCP |
| Client (`packages/api-client`) | Client HTTP tipado usado pela CLI e pelo MCP |
| Testes | Vitest (unidade + integração com Postgres real), Playwright (desktop + mobile) |

## Como rodar

Requisitos: Node 22.12+, pnpm 10.11, Docker.

```bash
cp .env.example .env          # ajuste se precisar; nunca versione o .env
pnpm install
docker compose up -d          # Postgres dev (porta 5435) e teste (porta 5434)
pnpm db:migrate               # aplica as migrations
pnpm db:seed                  # dados de exemplo: 4 contas, 2 cartões, 3 meses de transações
pnpm build
pnpm dev                      # API em :3333 e web em :5273
```

- Web: http://localhost:5273
- API: http://localhost:3333/api
- Swagger: http://localhost:3333/docs

## Verificação

```bash
pnpm lint               # Biome (somente leitura)
pnpm typecheck
pnpm build
pnpm test               # unidade: API (50) + CLI (3)
pnpm test:integration   # API contra o banco finflow_test (11 cenários)
pnpm test:e2e           # Playwright, 13 cenários × desktop e Pixel 7
```

O banco de integração precisa ter nome terminado em `_test`; o setup recusa qualquer outro. As migrations são aplicadas com `prisma migrate deploy`, que não apaga dados.

Os testes Playwright interceptam a API com fixtures determinísticas (`apps/web/e2e/mocks.ts`). Eles validam a interface, não a integração com o backend; essa parte é coberta por `test:integration`.

## Regras de domínio

- Dinheiro trafega sempre em **centavos inteiros**. Datas de domínio são `YYYY-MM-DD` em UTC.
- **Saldo da conta** = saldo inicial + soma das transações confirmadas. Pendentes não afetam o saldo.
- **Transferência** cria duas pernas com o mesmo `transferGroupId` numa transação de banco. Excluir uma perna exclui o par.
- **Compra no cartão**: entra na fatura cujo fechamento é posterior à data da compra; no dia do fechamento já cai na próxima. Uma compra em N parcelas cria N lançamentos, um em cada fatura, e o resto da divisão vai para as primeiras parcelas.
- **Fatura**: o status `PAID` é persistido. `OPEN` e `CLOSED` são derivados da data de fechamento. O pagamento debita a conta e marca a fatura numa única transação; excluir o pagamento reabre a fatura.
- **Orçamento**: verde abaixo de 70%, amarelo de 70% a 90%, vermelho acima de 90%. Conta despesas em conta e no cartão do mês, inclusive as pendentes.
- **Meta**: aporte mensal necessário = restante ÷ meses até o prazo, arredondado para cima. O resgate não pode deixar o saldo guardado negativo.

## Exportação

`GET /api/transactions/export?format=csv|ofx` aceita os mesmos filtros do extrato.

- **CSV**: separador `;`, vírgula decimal e BOM UTF-8, para abrir direto no Excel em português.
- **OFX 1.02**: aceito por planilhas e apps financeiros.

Também disponível no botão do extrato, em `finflow exportar` e na tool MCP `export_transactions`.

## CLI

```bash
pnpm build && pnpm cli -- --help
pnpm cli saldo
pnpm cli resumo --mes 10 --ano 2026
pnpm cli extrato --busca mercado
pnpm cli despesa 45,90 almoço --categoria alimentação
pnpm cli receita 3200 freelance --conta nubank
pnpm cli transferir 500 --de nubank --para xp
pnpm cli compra 3500 notebook --cartao ultravioleta --parcelas 10
pnpm cli faturas ultravioleta --pagar 11/2026 --conta nubank
pnpm cli orcamentos
pnpm cli metas
pnpm cli guardar 200 --meta reserva
pnpm cli exportar --formato ofx --saida extrato.ofx
pnpm cli status
```

Contas, cartões, categorias e metas aceitam nome parcial sem diferenciar maiúsculas. Use `--api <url>` ou `FINFLOW_API_URL` para apontar para outra API.

## Servidor MCP

Com a API rodando, registre no seu agente:

```json
{
  "mcpServers": {
    "finflow": {
      "command": "node",
      "args": ["C:/Users/daniellima/code/projetos/finflow/apps/mcp/dist/index.js"],
      "env": { "FINFLOW_API_URL": "http://localhost:3333" }
    }
  }
}
```

No Claude Code: `claude mcp add finflow -- node <caminho>/apps/mcp/dist/index.js`.

**19 tools.** Leitura: `get_summary`, `get_cashflow`, `get_category_spending`, `get_upcoming_bills`, `list_accounts`, `list_cards`, `list_card_invoices`, `list_transactions`, `list_categories`, `list_budgets`, `list_goals`. Escrita: `create_expense`, `create_income`, `create_card_expense`, `transfer`, `pay_invoice`, `set_budget`, `contribute_goal`, `export_transactions`. Os valores são em centavos.

## Bot do Telegram

1. Crie o bot com o [@BotFather](https://t.me/BotFather) e coloque o token em `TELEGRAM_BOT_TOKEN`.
2. Defina `TELEGRAM_WEBHOOK_SECRET` com um valor aleatório.
3. Exponha a API com HTTPS (por exemplo, com um túnel) e registre o webhook:
   ```bash
   curl "https://api.telegram.org/bot<TOKEN>/setWebhook?url=https://<host>/api/telegram/webhook&secret_token=<SECRET>"
   ```
4. Mande qualquer mensagem ao bot. Ele responde com o seu chat id; coloque esse valor em `TELEGRAM_CHAT_ID` e reinicie a API.

Comandos: `/saldo`, `/resumo`, `/extrato`, `/ajuda`. Para lançar uma despesa rápida, envie algo como `45,90 almoço #alimentacao`. O bot só responde ao chat vinculado e ignora os demais.

Quando um gasto faz um orçamento passar de 70% ou de 90%, o bot avisa. O aviso é enviado só no momento em que o limite é cruzado.

> **Estado:** o webhook, o parser e os alertas estão cobertos por testes com a Bot API simulada. A entrega real no Telegram ainda **não foi verificada**, porque depende de um token de bot e de uma URL pública. Sem token, o módulo fica inativo e o resto do app funciona normalmente.

## Estrutura

```text
apps/
  api/        NestJS: módulos accounts, categories, cards, transactions, budgets,
              goals, dashboard, notifications, telegram
  web/        React: dashboard, extrato, contas e cartões, orçamentos, metas
  cli/        finflow (binário)
  mcp/        finflow-mcp (binário stdio)
packages/
  contracts/  schemas Zod e tipos
  api-client/ client HTTP tipado
```

As camadas são separadas assim: controllers só mapeiam HTTP e validam com Zod; services têm as regras de negócio; repositories fazem as consultas Prisma. O projeto não usa `any` explícito; o Biome bloqueia.
