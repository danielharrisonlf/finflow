# FinFlow

Controle financeiro pessoal: contas, cartões de crédito com faturas e parcelamentos, extrato com exportação, orçamentos por categoria e metas. Um **único binário em Go** unifica API HTTP, interface web embutida (`go:embed`), CLI, servidor MCP para agentes de IA e bot do Telegram — com **SQLite embutido por padrão** (zero Docker) e suporte opcional a PostgreSQL via `DATABASE_URL`.

## Stack

| Camada | Tecnologia |
| --- | --- |
| Binário único (`cmd/finflow`) | Go 1.26 (`net/http`, Cobra), API REST + Web embutida + CLI + MCP (stdio) |
| Banco de dados (`internal/db`) | **SQLite** embutido (`modernc.org/sqlite`, pure Go, zero CGO/Docker) por padrão; **PostgreSQL** opcional (`pgx/v5`) via `DATABASE_URL` |
| Web (`apps/web`) | React 19, Vite 8, Tailwind v4, Radix UI, TanStack Query, React Hook Form |
| Contratos web (`packages/contracts`) | Schemas Zod e tipos compartilhados no frontend |
| Testes | `go test ./...` (domínio, API, CLI e MCP), Playwright (desktop + mobile) |

## Como rodar localmente (sem Docker)

Requisitos para compilar: Go 1.26+, Node 22.12+, pnpm 10.11. Depois de compilado, o binário `./finflow` (ou `finflow.exe`) roda sozinho.

```bash
cp .env.example .env      # opcional — SQLite (finflow.db) já funciona sem configuração
pnpm install
pnpm build                # compila o web, embute via go:embed e gera ./finflow(.exe)
./finflow seed            # dados de exemplo: 4 contas, 2 cartões, 3 meses de transações
./finflow serve           # sobe API + interface web em http://localhost:3333
```

Ou em um único passo (criando dados de exemplo automaticamente se o banco estiver vazio):

```bash
./finflow serve --seed
```

### Desenvolvimento do frontend com hot-reload

```bash
pnpm dev:api              # API Go em http://localhost:3333
pnpm dev:web              # Vite dev server em http://localhost:5273
```

### Usar PostgreSQL (opcional)

Para rodar com PostgreSQL em vez do SQLite padrão:

```bash
docker compose --profile postgres up -d db
```

E defina `DATABASE_URL="postgresql://finflow:finflow@localhost:5435/finflow?schema=public"` no `.env`. O schema é aplicado automaticamente no boot. O parâmetro `schema` (formato do Prisma) é traduzido para `search_path`, e a sessão roda sempre em UTC.

### Onde fica o banco SQLite

| `FINFLOW_DB_PATH` | Arquivo usado |
|---|---|
| caminho absoluto | o próprio caminho |
| relativo, definido no `.env` | relativo à pasta do `.env` (o `.env.example` usa `finflow.db` → raiz do projeto) |
| relativo, definido no ambiente | relativo ao diretório atual |
| não definido | `os.UserConfigDir()/finflow/finflow.db` (`%AppData%inflow` no Windows, `~/.config/finflow` no Linux) |

Assim, `finflow mcp` iniciado por um host MCP em outro diretório não cria um banco vazio por engano. `serve` e `mcp` mostram no stderr o caminho absoluto do banco em uso.

### Fuso horário

"Hoje" e o mês corrente seguem `America/Sao_Paulo` por padrão (sobrescreva com `FINFLOW_TZ`, por exemplo `FINFLOW_TZ=UTC`). As datas gravadas continuam `YYYY-MM-DD`, e os timestamps internos ficam em UTC.

## Verificação

```bash
gofmt -l cmd internal     # deve sair vazio
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...
go test ./...             # unidade + integração (store, service, API, CLI, MCP) em SQLite

# A mesma suíte também em PostgreSQL (cada teste usa um schema descartável):
docker compose --profile test up -d db_test
FINFLOW_TEST_DATABASE_URL="postgres://finflow:finflow@localhost:5434/finflow_test?sslmode=disable" go test ./...
```

O CI roda tudo isso com `-race` e um serviço PostgreSQL, além de `go mod tidy -diff`.

```bash
pnpm lint                 # Biome (frontend/contratos/scripts)
pnpm typecheck            # TypeScript (web/contracts)
pnpm build                # build completo (web + binário único Go)
pnpm test:e2e             # Playwright (desktop + Pixel 7)
```

## Regras de domínio

- Dinheiro trafega sempre em **centavos inteiros**. Datas de domínio são `YYYY-MM-DD`; "hoje" segue `FINFLOW_TZ` (padrão `America/Sao_Paulo`).
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

Também disponível no botão do extrato, em `./finflow exportar` e na tool MCP `export_transactions`.

## CLI

A CLI acessa o banco local **diretamente** (sem precisar manter o servidor aberto). Para apontar para uma API remota, use `--api <url>` (ou `FINFLOW_API_URL`) e `--token <token>` (ou `FINFLOW_TOKEN` / `FINFLOW_PASSWORD`).

```bash
./finflow --help
./finflow serve [--port 3333] [--seed]
./finflow seed
./finflow saldo
./finflow resumo --mes 10 --ano 2026
./finflow extrato --busca mercado
./finflow despesa 45,90 almoço --categoria alimentação
./finflow receita 3200 freelance --conta nubank
./finflow transferir 500 --de nubank --para xp
./finflow compra 3500 notebook --cartao ultravioleta --parcelas 10
./finflow cartoes
./finflow faturas ultravioleta --pagar 11/2026 --conta nubank
./finflow orcamentos
./finflow metas
./finflow guardar 200 --meta reserva
./finflow exportar --formato ofx --saida extrato.ofx
./finflow status
```

Contas, cartões, categorias e metas aceitam nome parcial sem diferenciar maiúsculas.

## Autenticação

Por padrão, em uso local sem `FINFLOW_PASSWORD`, a API roda em modo aberto. Antes de expor a API na rede:

1. Defina `FINFLOW_PASSWORD` no `.env` com uma senha forte (e opcionalmente `FINFLOW_AUTH_SECRET` e `FINFLOW_API_TOKEN`).
2. Na interface web, uma tela de login passa a ser exigida automaticamente e armazena um token de sessão assinado (HMAC-SHA256, válido por 30 dias).
3. Na CLI e no MCP remotos, defina `FINFLOW_TOKEN` (ou `FINFLOW_PASSWORD`) ou passe `--token`.
4. O webhook do Telegram (`/api/telegram/webhook`) continua acessível mediante validação do cabeçalho `x-telegram-bot-api-secret-token`.

## Servidor MCP

O servidor MCP roda como subcomando do mesmo binário (`finflow mcp`) e funciona direto no SQLite local (ou apontando para `FINFLOW_API_URL`):

```json
{
  "mcpServers": {
    "finflow": {
      "command": "C:/Users/daniellima/code/projetos/finflow/finflow.exe",
      "args": ["mcp"]
    }
  }
}
```

**19 tools.** Leitura: `get_summary`, `get_cashflow`, `get_category_spending`, `get_upcoming_bills`, `list_accounts`, `list_cards`, `list_card_invoices`, `list_transactions`, `list_categories`, `list_budgets`, `list_goals`. Escrita: `create_expense`, `create_income`, `create_card_expense`, `transfer`, `pay_invoice`, `set_budget`, `contribute_goal`, `export_transactions`. Os valores são em centavos.

## Bot do Telegram

1. Crie o bot com o [@BotFather](https://t.me/BotFather) e coloque o token em `TELEGRAM_BOT_TOKEN`.
2. Defina `TELEGRAM_WEBHOOK_SECRET` com um valor aleatório.
3. Exponha a API com HTTPS e registre o webhook:
   ```bash
   curl "https://api.telegram.org/bot<TOKEN>/setWebhook?url=https://<host>/api/telegram/webhook&secret_token=<SECRET>"
   ```
4. Mande qualquer mensagem ao bot. Ele responde com o seu chat id; coloque esse valor em `TELEGRAM_CHAT_ID` e reinicie o servidor.

Comandos: `/saldo`, `/resumo`, `/extrato`, `/ajuda`. Para lançar uma despesa rápida, envie algo como `45,90 almoço #alimentacao`.

## Estrutura

```text
cmd/finflow/          entrypoint do binário único (serve, seed, mcp, CLI)
internal/
  config/             carregamento de .env e variáveis de ambiente
  domain/             contratos JSON, datas e fuso, centavos, faturas, orçamentos, metas, CSV/OFX
  db/                 driver SQLite (padrão) e PostgreSQL (opcional) + migrations automáticas
  store/              repositórios SQL e seed idempotente
  service/            regras de negócio, auth HMAC, notificações e bot Telegram
  httpapi/            rotas /api/*, CORS, auth, recover, limite de corpo (1 MiB), timeouts e SPA embutida
  client/             backend unificado (local in-process ou HTTP remoto)
  cli/                comandos Cobra e renderização de tabelas/barras no terminal
  mcp/                servidor MCP JSON-RPC 2.0 sobre stdio (19 tools)
  webui/              embedding de apps/web/dist via go:embed
  testutil/           bancos isolados para testes (SQLite em memória + PostgreSQL opcional)
apps/web/             interface React 19 + Vite 8 + Tailwind v4
packages/contracts/   schemas Zod usados pelo frontend
```
