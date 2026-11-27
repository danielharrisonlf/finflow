# FinFlow

Controle financeiro pessoal **gratuito, de código aberto e local-first**: contas, cartões com faturas e parcelas, projeção do saldo dia a dia, compromissos, limite mensal, metas e reserva de emergência, desejos e importação/exportação de planilhas. Um único programa roda no seu computador e abre no navegador. Seus dados ficam num arquivo SQLite local. Nada vai para a nuvem.

## Baixar e usar

Baixe o arquivo do seu sistema (sempre a versão mais recente):

| Sistema | Download |
| --- | --- |
| Windows (64 bits) | [finflow-windows-amd64.zip](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-windows-amd64.zip) |
| Windows (ARM) | [finflow-windows-arm64.zip](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-windows-arm64.zip) |
| macOS (Apple Silicon, M1 ou mais novo) | [finflow-darwin-arm64.tar.gz](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-darwin-arm64.tar.gz) |
| macOS (Intel) | [finflow-darwin-amd64.tar.gz](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-darwin-amd64.tar.gz) |
| Linux (x64) | [finflow-linux-amd64.tar.gz](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-linux-amd64.tar.gz) |
| Linux (ARM64) | [finflow-linux-arm64.tar.gz](https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-linux-arm64.tar.gz) |

Os hashes SHA-256 estão em [`checksums.txt`](https://github.com/danielharrisonlf/finflow/releases/latest/download/checksums.txt). Todas as versões ficam em [Releases](https://github.com/danielharrisonlf/finflow/releases).

### Instalar pelo terminal

macOS e Linux (instala em `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.sh | sh
```

Windows, no PowerShell (instala em `%LOCALAPPDATA%\FinFlow` e adiciona ao PATH do usuário):

```powershell
irm https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.ps1 | iex
```

Os dois scripts conferem o SHA-256 e aceitam `FINFLOW_VERSION=v0.2.0` para fixar uma versão e `FINFLOW_INSTALL_DIR` para outro destino. No Windows, `FINFLOW_NO_PATH=1` não altera o PATH.

### Primeira execução

- **Windows**: extraia o `.zip` e dê dois cliques em `finflow.exe`. O servidor sobe e o navegador abre http://localhost:3333. Se aparecer “O Windows protegeu o computador”, clique em **Mais informações → Executar assim mesmo** (o binário ainda não é assinado).
- **macOS**: extraia, libere o binário baixado com `xattr -d com.apple.quarantine finflow` e rode `./finflow`.
- **Linux**: extraia e rode `./finflow`.

Sem argumentos, o `finflow` inicia o servidor e abre o navegador (`FINFLOW_NO_BROWSER=1` desativa a abertura). Se já houver um FinFlow rodando na porta, ele só abre o navegador. Para conhecer o app com dados de exemplo:

```bash
finflow serve --seed      # cria contas, cartões e 3 meses de lançamentos se o banco estiver vazio
```

Para parar, feche a janela do FinFlow ou pressione `Ctrl+C` no terminal. `finflow version` mostra a versão instalada.

### Onde ficam os seus dados

Num único arquivo SQLite, na pasta de configuração do usuário:

| Sistema | Arquivo |
| --- | --- |
| Windows | `%AppData%\finflow\finflow.db` |
| macOS | `~/Library/Application Support/finflow/finflow.db` |
| Linux | `~/.config/finflow/finflow.db` |

Para fazer backup, copie o arquivo com o FinFlow fechado. Para usar outro local, defina `FINFLOW_DB_PATH` (veja a tabela completa em [Onde fica o banco SQLite](#onde-fica-o-banco-sqlite)). `serve` e `mcp` mostram no terminal o caminho do banco em uso.

## O que o FinFlow faz

- **Contas e cartões**: saldos, transferências, faturas com fechamento e vencimento, compras parceladas.
- **Projeção do saldo dia a dia** com faixas de cor (do folgado ao muito negativo), menor saldo nos próximos 30 dias e quanto dá para guardar até a próxima entrada.
- **Compromissos**: contas fixas, receitas recorrentes e parcelas dos próximos meses, a pagar e a receber.
- **Limite mensal e ritmo do mês**: quanto ainda cabe por dia e a comparação com o mesmo período do mês anterior.
- **Fixo / Cortável / Variável**: cada categoria tem um tipo de gasto, com sugestões de onde dá para cortar.
- **Orçamentos, metas e reserva de emergência** (6 meses de custo de vida, ou 12 com dependentes).
- **Desejos**: lista de compras por prioridade com a data em que cada item cabe no saldo.
- **Automações**: regras que categorizam lançamentos e importações pela descrição.
- **Visão do ano**: entradas, saídas, performance e economia mês a mês.
- **Planilhas**: importa a planilha diária (Entrada · Saída · Diário · Saldo), Excel/CSV e extrato OFX; exporta uma pasta `.xlsx` anual com fórmulas vivas, saldo diário colorido e aba Economia.
- **CLI, servidor MCP** para assistentes de IA e **bot do Telegram**, no mesmo binário.

## Compilar do código-fonte

Requisitos: Go 1.26+, Node 22.12+ e pnpm 10.11.

```bash
git clone https://github.com/danielharrisonlf/finflow.git
cd finflow
pnpm install
pnpm build                # compila o web, embute via go:embed e gera ./finflow(.exe)
./finflow serve --seed    # API + interface web em http://localhost:3333
```

O `.env` é opcional (`cp .env.example .env`); o SQLite funciona sem configuração. Para gerar os pacotes das 6 plataformas como na release, use `pnpm release` (veja [docs/release.md](docs/release.md)).

---

## Desenvolvimento

### Stack

| Camada | Tecnologia |
| --- | --- |
| Binário único (`cmd/finflow`) | Go 1.26 (`net/http`, Cobra), API REST + Web embutida + CLI + MCP (stdio) |
| Banco de dados (`internal/db`) | **SQLite** embutido (`modernc.org/sqlite`, pure Go, zero CGO/Docker) por padrão; **PostgreSQL** opcional (`pgx/v5`) via `DATABASE_URL` |
| Planilhas (`internal/spreadsheet`) | `excelize/v2` para `.xlsx`; leitores de CSV e OFX |
| Web (`apps/web`) | React 19, Vite 8, Tailwind v4, Radix UI, TanStack Query, React Hook Form |
| Contratos web (`packages/contracts`) | Schemas Zod e tipos compartilhados no frontend |
| Testes | `go test ./...` (domínio, API, CLI e MCP), Playwright (desktop + mobile) |

### Frontend com hot-reload

```bash
pnpm dev:api              # API Go em http://localhost:3333
pnpm dev:web              # Vite dev server em http://localhost:5273
```

A landing pública fica em `/inicio` (fonte em `apps/web/src/components/landing/`) e é publicada à parte; veja [docs/landing-themes.md](docs/landing-themes.md).

### Usar PostgreSQL (opcional)

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
| não definido | `os.UserConfigDir()/finflow/finflow.db` (`%AppData%\finflow` no Windows, `~/Library/Application Support/finflow` no macOS, `~/.config/finflow` no Linux) |

Assim, `finflow mcp` iniciado por um host MCP em outro diretório não cria um banco vazio por engano.

### Fuso horário

"Hoje" e o mês corrente seguem `America/Sao_Paulo` por padrão (sobrescreva com `FINFLOW_TZ`, por exemplo `FINFLOW_TZ=UTC`). As datas gravadas continuam `YYYY-MM-DD`, e os timestamps internos ficam em UTC.

### Verificação

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
pnpm release              # pacotes das 6 plataformas em dist/release/
```

### Regras de domínio

- Dinheiro trafega sempre em **centavos inteiros**. Datas de domínio são `YYYY-MM-DD`; "hoje" segue `FINFLOW_TZ` (padrão `America/Sao_Paulo`).
- **Saldo da conta** = saldo inicial + soma das transações confirmadas. Pendentes não afetam o saldo.
- **Transferência** cria duas pernas com o mesmo `transferGroupId` numa transação de banco. Excluir uma perna exclui o par.
- **Compra no cartão**: entra na fatura cujo fechamento é posterior à data da compra; no dia do fechamento já cai na próxima. Uma compra em N parcelas cria N lançamentos, um em cada fatura, e o resto da divisão vai para as primeiras parcelas.
- **Fatura**: o status `PAID` é persistido. `OPEN` e `CLOSED` são derivados da data de fechamento. O pagamento debita a conta e marca a fatura numa única transação; excluir o pagamento reabre a fatura.
- **Orçamento**: verde abaixo de 70%, amarelo de 70% a 90%, vermelho acima de 90%. Conta despesas em conta e no cartão do mês, inclusive as pendentes.
- **Meta**: aporte mensal necessário = restante ÷ meses até o prazo, arredondado para cima. O resgate não pode deixar o saldo guardado negativo. Uma meta pode ser a **reserva de emergência** (no máximo uma ativa).
- **Tipo de gasto**: categorias de despesa são `FIXED`, `CUTTABLE` ou `VARIABLE` (subcategoria sem valor herda do pai). Fixo + Cortável formam as **Saídas**; Variável e despesas sem categoria formam o **Diário**. **Performance** = Entradas − (Saídas + Diário).
- **Faixas de saldo** (padrão, configuráveis): verde escuro ≥ R$ 2.000, verde claro ≥ R$ 1.000, amarelo ≥ R$ 0, vermelho claro ≥ −R$ 500, vermelho abaixo disso.

O contrato completo da v2 (modelos, insights, projeção, planilhas) está em [docs/product-v2-contract.md](docs/product-v2-contract.md).

### API (resumo)

Além de contas, cartões, faturas, transações, categorias, orçamentos e metas em `/api/*`:

| Recurso | Rotas |
| --- | --- |
| Recorrências (compromissos) | `GET/POST /api/recurring`, `PATCH/DELETE /api/recurring/{id}`, `POST /api/recurring/{id}/post` |
| Configurações | `GET/PATCH /api/settings` (limite mensal, reserva, faixas de saldo, plano de gastos) |
| Automações | `GET/POST /api/rules`, `PATCH/DELETE /api/rules/{id}`, `POST /api/rules/apply` |
| Desejos | `GET/POST /api/wishlist`, `PATCH/DELETE /api/wishlist/{id}` |
| Insights | `GET /api/insights/overview`, `/projection?days=90`, `/year?year=2026`, `/commitments` |
| Planilhas | `GET /api/spreadsheet/export.xlsx?year=2026`, `GET /api/spreadsheet/template.xlsx`, `POST /api/import/preview`, `POST /api/import/commit` |
| Exportação do extrato | `GET /api/transactions/export?format=csv\|ofx\|xlsx` (mesmos filtros do extrato) |

- **CSV**: separador `;`, vírgula decimal e BOM UTF-8, para abrir direto no Excel em português.
- **OFX 1.02**: aceito por planilhas e apps financeiros.
- **XLSX**: aba `Lançamentos` com filtro automático; a exportação anual inclui as abas do ano, Economia, Lançamentos e Categorias.

### CLI

A CLI acessa o banco local **diretamente** (sem precisar manter o servidor aberto). Para apontar para uma API remota, use `--api <url>` (ou `FINFLOW_API_URL`) e `--token <token>` (ou `FINFLOW_TOKEN` / `FINFLOW_PASSWORD`).

```bash
finflow                    # sem argumentos: inicia o servidor e abre o navegador
finflow --help
finflow version
finflow serve [--port 3333] [--seed]
finflow seed
finflow saldo
finflow resumo --mes 10 --ano 2026
finflow extrato --busca mercado
finflow despesa 45,90 almoço --categoria alimentação
finflow receita 3200 freelance --conta nubank
finflow transferir 500 --de nubank --para xp
finflow compra 3500 notebook --cartao ultravioleta --parcelas 10
finflow cartoes
finflow faturas ultravioleta --pagar 11/2026 --conta nubank
finflow orcamentos
finflow metas
finflow guardar 200 --meta reserva
finflow exportar --formato ofx --saida extrato.ofx
finflow exportar --formato xlsx --ano 2026
finflow importar planilha.xlsx --conta nubank [--simular]
finflow projecao [--dias 30]
finflow fixas
finflow insights [--mes 11 --ano 2026]
finflow status
```

Contas, cartões, categorias e metas aceitam nome parcial sem diferenciar maiúsculas.

### Autenticação

Por padrão, em uso local sem `FINFLOW_PASSWORD`, a API roda em modo aberto. O `serve` escuta em todas as interfaces; antes de expor a API na rede:

1. Defina `FINFLOW_PASSWORD` no `.env` com uma senha forte (e opcionalmente `FINFLOW_AUTH_SECRET` e `FINFLOW_API_TOKEN`). Para aceitar só conexões locais, use `finflow serve --host 127.0.0.1` (ou `API_HOST`).
2. Na interface web, uma tela de login passa a ser exigida automaticamente e armazena um token de sessão assinado (HMAC-SHA256, válido por 30 dias).
3. Na CLI e no MCP remotos, defina `FINFLOW_TOKEN` (ou `FINFLOW_PASSWORD`) ou passe `--token`.
4. O webhook do Telegram (`/api/telegram/webhook`) continua acessível mediante validação do cabeçalho `x-telegram-bot-api-secret-token`.

### Servidor MCP

O servidor MCP roda como subcomando do mesmo binário (`finflow mcp`) e funciona direto no SQLite local (ou apontando para `FINFLOW_API_URL`). Use o caminho do binário instalado:

```json
{
  "mcpServers": {
    "finflow": {
      "command": "/caminho/para/finflow",
      "args": ["mcp"]
    }
  }
}
```

No Windows, algo como `"C:/Users/<você>/AppData/Local/FinFlow/finflow.exe"` (destino do `install.ps1`); no macOS/Linux, `~/.local/bin/finflow` expandido para o caminho absoluto.

Tools de leitura: `get_summary`, `get_cashflow`, `get_category_spending`, `get_upcoming_bills`, `list_accounts`, `list_cards`, `list_card_invoices`, `list_transactions`, `list_categories`, `list_budgets`, `list_goals`, `get_insights`, `get_projection`, `get_year_overview`, `list_recurring`. Escrita: `create_expense`, `create_income`, `create_card_expense`, `transfer`, `pay_invoice`, `set_budget`, `contribute_goal`, `create_recurring`, `export_transactions`. Os valores são em centavos.

### Bot do Telegram

1. Crie o bot com o [@BotFather](https://t.me/BotFather) e coloque o token em `TELEGRAM_BOT_TOKEN`.
2. Defina `TELEGRAM_WEBHOOK_SECRET` com um valor aleatório.
3. Exponha a API com HTTPS e registre o webhook:
   ```bash
   curl "https://api.telegram.org/bot<TOKEN>/setWebhook?url=https://<host>/api/telegram/webhook&secret_token=<SECRET>"
   ```
4. Mande qualquer mensagem ao bot. Ele responde com o seu chat id; coloque esse valor em `TELEGRAM_CHAT_ID` e reinicie o servidor.

Comandos: `/saldo`, `/resumo`, `/extrato`, `/ajuda`. Para lançar uma despesa rápida, envie algo como `45,90 almoço #alimentacao`.

### Estrutura

```text
cmd/finflow/          entrypoint do binário único (modo duplo clique, version, serve, seed, mcp, CLI)
internal/
  config/             carregamento de .env e variáveis de ambiente
  domain/             contratos JSON, datas e fuso, centavos, faturas, orçamentos, metas, insights, CSV/OFX
  db/                 driver SQLite (padrão) e PostgreSQL (opcional) + migrations automáticas
  store/              repositórios SQL e seed idempotente
  service/            regras de negócio, insights, importação, auth HMAC, notificações e bot Telegram
  spreadsheet/        leitura e escrita de planilhas (xlsx, CSV, OFX), sem banco
  httpapi/            rotas /api/*, CORS, auth, recover, limite de corpo, timeouts e SPA embutida
  client/             backend unificado (local in-process ou HTTP remoto)
  cli/                comandos Cobra e renderização de tabelas/barras no terminal
  mcp/                servidor MCP JSON-RPC 2.0 sobre stdio
  webui/              embedding de apps/web/dist via go:embed
  testutil/           bancos isolados para testes (SQLite em memória + PostgreSQL opcional)
apps/web/             interface React 19 + Vite 8 + Tailwind v4 (e a landing em /inicio)
packages/contracts/   schemas Zod usados pelo frontend
scripts/              build, release (6 plataformas) e instaladores install.sh / install.ps1
.github/workflows/    CI e release por tag (v*)
```
