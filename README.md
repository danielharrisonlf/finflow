# FinFlow

Controle financeiro pessoal **gratuito, de código aberto e local-first**: contas, cartões com faturas e parcelas, projeção do saldo dia a dia, compromissos, limite mensal, metas e reserva de emergência, desejos, investimentos, dívidas, patrimônio e importação/exportação de planilhas, com um assistente opcional que usa a sua chave da API do Claude. Use direto no navegador, sem instalar nada, ou baixe um único programa para o computador. Nos dois casos os dados ficam com você (no navegador ou num arquivo SQLite local). Nada vai para a nuvem, exceto as perguntas que você fizer ao assistente.

## Usar no navegador

Abra **https://finflow-landing-umber.vercel.app/app/** no computador ou no celular. É o FinFlow completo, sem cadastro e sem baixar nada; para ter um ícone e abrir em tela cheia, use **Instalar app** (Chrome/Edge) ou **Adicionar à Tela de Início** (Safari).

- **Onde ficam os dados**: no próprio navegador, no aparelho em que você usa (armazenamento do site, OPFS). Nada vai para um servidor: a API do FinFlow roda dentro da página, compilada para WebAssembly, com o mesmo banco SQLite do programa. A única exceção são as perguntas ao assistente, que vão direto do navegador para a API do Claude com a sua chave.
- **Primeira vez**: escolha **Começar do zero**, **Explorar com dados de exemplo** ou **Restaurar um backup (.db)**.
- **Backup**: em **Configurações → Dados neste navegador**, faça um `.db` por espaço ou um `.zip` com todos. Limpar os dados de sites do navegador apaga o FinFlow, então mantenha um backup recente. A mesma tela pede **armazenamento persistente** (o navegador deixa de apagar os dados sozinho quando falta espaço).
- **Levar para o app instalado e vice-versa**: o formato do banco é o mesmo. Um backup `.db` do navegador abre no programa (substitua o `finflow.db`, veja [Onde ficam os seus dados](#onde-ficam-os-seus-dados)); em **Trazer dados do app instalado**, escolha o `finflow.db` do computador para usar no navegador.
- **Uma aba por vez**: o banco aceita uma conexão. Se o FinFlow já estiver aberto em outra aba, a nova mostra **Usar aqui**.
- **Offline**: depois do primeiro carregamento, o app abre sem internet (service worker). Quando sai uma versão nova, aparece **Nova versão disponível — Recarregar**.
- **O que fica só no programa instalado**: rede local com QR code, MCP (stdio/HTTP), bot do Telegram e prévia de links em Desejos (o navegador bloqueia ler outros sites).
- **Navegadores**: Chrome, Edge, Firefox e Safari atuais. Em janela anônima/privada o navegador pode não guardar os dados; nesse caso o app avisa.

## Baixar para o computador

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

- **Contas e cartões**: saldos, transferências, faturas com fechamento e vencimento, compras parceladas e a composição de cada fatura (parcelas, assinaturas, compras avulsas). Despesa e receita de mesmo valor entre contas suas viram transferência com um clique.
- **Projeção do saldo dia a dia** com faixas de cor (do folgado ao muito negativo), menor saldo nos próximos 30 dias e quanto dá para guardar até a próxima entrada.
- **Quanto posso gastar hoje e ritmo do mês**: o que sobra do limite dividido pelos dias restantes (ou o menor saldo projetado, sem limite), com o gasto acumulado comparado ao mês anterior.
- **Posso comprar?**: simula uma compra à vista ou parcelada no cartão sobre a projeção e responde “cabe”, “com cuidado” ou “melhor esperar”, com a data em que caberia.
- **Compromissos e assinaturas**: contas fixas, receitas recorrentes e parcelas dos próximos meses; cobranças repetidas são detectadas no histórico, com aviso de aumento de preço.
- **Alertas e dicas**: orçamento perto do limite, fatura ou conta vencendo, saldo que vai ficar negativo, reserva baixa, meta atrasada e outros, num sino com contador.
- **Tags**: marque lançamentos (viagem, reforma...) e veja o gasto por tag, além das categorias.
- **Fluxo de caixa**: mês a mês em regime de caixa (compra no cartão no mês do vencimento da fatura) ou de competência.
- **Fixo / Cortável / Variável**: cada categoria tem um tipo de gasto, com sugestões de onde dá para cortar.
- **Orçamentos, metas com modelos e reserva de emergência** (6 meses de custo de vida, ou 12 com dependentes).
- **Desejos**: lista de compras por prioridade, com prévia do link do produto e a data em que cada item cabe no saldo.
- **Investimentos**: posições por tipo e instituição, aportes, resgates, rentabilidade e alocação.
- **Dívidas e financiamentos**: parcelas (Price), saldo devedor, juros restantes e estratégia de quitação avalanche ou bola de neve.
- **Evolução do patrimônio**: contas + investimentos − dívidas no fim de cada mês.
- **11 calculadoras**: alugar ou comprar, carro ou app de transporte, juros compostos, pequenos gastos, 13º salário, reserva de emergência, independência financeira, quitar dívidas, Price × SAC, à vista ou parcelado, CDB × poupança.
- **Automações**: regras que categorizam (e marcam com tag) lançamentos e importações pela descrição.
- **Visão do ano**: entradas, saídas, performance e economia mês a mês.
- **Planilhas**: importa a planilha diária (Entrada · Saída · Diário · Saldo), Excel/CSV e extrato OFX; exporta uma pasta `.xlsx` anual com fórmulas vivas, saldo diário colorido e aba Economia.
- **Busca e privacidade**: paleta de comandos (Ctrl+K / ⌘K) e modo que oculta os valores na tela.
- **No celular, sem nuvem**: modo rede local e app instalável (PWA). Veja [No celular](#no-celular-rede-local--app).
- **Assistente “Pergunte ao FinFlow”** com a sua chave da API do Claude, inclusive leitura de foto de comprovante. Veja [Assistente](#assistente-pergunte-ao-finflow).
- **CLI, servidor MCP** (stdio e HTTP), **API pessoal** e **bot do Telegram**, no mesmo binário.

### No celular (rede local + app)

O FinFlow continua rodando no computador; o celular só acessa pela rede de casa. Nada passa por servidor externo.

1. Defina uma senha (obrigatória no modo rede local) e inicie com `--lan`:

   ```bash
   FINFLOW_PASSWORD='uma senha forte' finflow serve --lan    # ou FINFLOW_LAN=1
   ```

   No Windows (PowerShell): `$env:FINFLOW_PASSWORD = 'uma senha forte'; finflow serve --lan`. Sem `FINFLOW_PASSWORD`, o `--lan` se recusa a iniciar.
2. O terminal e **Configurações → API e IA** mostram os endereços da rede (`http://192.168.x.x:3333`) e um QR code. Abra no celular conectado ao mesmo Wi-Fi e entre com a senha.
3. Instale como app: no Android (Chrome), menu **⋮ → Instalar app**; no iPhone (Safari), **Compartilhar → Adicionar à Tela de Início**. O app guarda só a interface; os dados continuam no computador, que precisa estar ligado com o FinFlow aberto.

Sem `--lan`, o servidor escuta só em `127.0.0.1` (acessível apenas no próprio computador).

### Assistente (Pergunte ao FinFlow)

O assistente responde perguntas como “Posso fazer uma viagem?” consultando sua projeção, limites, compromissos e metas, e lê a foto (ou o PDF) de um comprovante para preencher um lançamento. Ele usa a API do Claude, da Anthropic, com **a sua própria chave**:

- defina `ANTHROPIC_API_KEY` antes de iniciar o FinFlow (`ANTHROPIC_API_KEY=sk-ant-... finflow serve`), ou
- cole a chave em **Configurações → API e IA**. Ela fica no seu banco local e nunca é devolvida à interface.

**Custo**: o FinFlow é gratuito; o uso da API é cobrado pela Anthropic na sua conta, conforme o volume de perguntas. Sem chave, todo o resto funciona, inclusive o simulador “Posso comprar?” e as calculadoras.

**Privacidade**: só as mensagens do assistente (com os dados que ele consultar para responder) e os comprovantes que você enviar vão para a API do Claude. Qualquer escrita (registrar despesa, criar compromisso, guardar numa meta, adicionar desejo) vira uma proposta que só é salva depois que você confirma na tela. O histórico da conversa fica no seu navegador.

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
| Binário único (`cmd/finflow`) | Go 1.26 (`net/http`, Cobra), API REST + Web embutida + CLI + MCP (stdio e HTTP) + assistente (API do Claude) |
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

### FinFlow no navegador (build e testes)

O app em `/app/` é o mesmo frontend com `VITE_FINFLOW_RUNTIME=browser`: `api.ts`, `ai.ts` e os downloads usam `runtimeFetch` (`apps/web/src/runtime/client.ts`), que fala com um worker onde roda `cmd/finflow-wasm` (Go → WebAssembly) sobre `@sqlite.org/sqlite-wasm`. Contrato em [docs/browser-runtime-contract.md](docs/browser-runtime-contract.md).

```bash
pnpm build:browser               # build:wasm + vite build --mode browser → apps/web/dist-browser (base /app/)
pnpm --filter @finflow/web preview:browser   # http://localhost:5274/app/
pnpm test:e2e:browser            # Playwright contra o dist-browser: primeira vez, exemplo, lançamento, xlsx, backup/restaurar, outra aba, offline
```

O build embutido no binário (`pnpm build:web`) não muda e não leva o runtime do navegador. Publicação do pacote público em [docs/release.md](docs/release.md#site-público-landing--app-no-navegador).

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

O contrato completo da v2 (modelos, insights, projeção, planilhas) está em [docs/product-v2-contract.md](docs/product-v2-contract.md); os recursos da v3 (tags, assinaturas, alertas, investimentos, dívidas, patrimônio, MCP via HTTP, rede local, assistente e calculadoras) em [docs/product-v3-contract.md](docs/product-v3-contract.md). As mudanças por versão estão no [CHANGELOG.md](CHANGELOG.md).

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
| Tags | `GET/POST /api/tags`, `PATCH/DELETE /api/tags/{id}`, `GET /api/insights/tags` |
| Assinaturas e alertas | `GET /api/insights/subscriptions` (+ `/{key}/track`, `/{key}/dismiss`), `GET /api/alerts`, `POST /api/alerts/{id}/dismiss` |
| Fluxo de caixa e “posso comprar?” | `GET /api/insights/cashflow?months=3&basis=CASH\|ACCRUAL`, `POST /api/insights/can-afford` |
| Investimentos | `GET/POST /api/investments`, `PATCH/DELETE /api/investments/{id}`, `POST .../{id}/value\|contribution\|redeem`, `GET /api/investments/summary` |
| Dívidas | `GET/POST /api/loans`, `PATCH/DELETE /api/loans/{id}`, `POST /api/loans/{id}/pay`, `GET /api/loans/strategy?extraCents=0` |
| Patrimônio | `GET /api/insights/net-worth?months=12` |
| Assistente | `GET /api/ai/status`, `PUT /api/ai/key`, `POST /api/ai/chat` (SSE), `POST /api/ai/receipt` |
| MCP e sistema | `POST /api/mcp`, `GET /api/system/info` |
| Espaços | `GET/POST /api/spaces`, `PATCH /api/spaces/{slug}`, `DELETE /api/spaces/{slug}?confirm=<slug>`, `GET /api/spaces/overview` |
| Casa (divisão) | `GET/POST /api/members`, `PATCH/DELETE /api/members/{id}`, `GET /api/members/balances?month&year`, `POST /api/members/settle` |
| Empresa | `GET /api/business/metrics?month&year`, `GET /api/business/dre?year` |

**Espaços** ([docs/product-v3-spaces-contract.md](docs/product-v3-spaces-contract.md)): cada espaço é um SQLite separado em `<pasta do banco>/spaces/<slug>.db`, listado em `<pasta do banco>/spaces.json`; o banco atual é o espaço `pessoal`. O cabeçalho `X-FinFlow-Space: <slug>` escolhe o espaço em qualquer rota `/api/*` (ausente = `pessoal`, inexistente = 404; em links com `?token=` vale `?space=<slug>`). Apagar um espaço move o arquivo para `spaces/trash/`. Com PostgreSQL (`DATABASE_URL`) há só o espaço `pessoal` e criar outro responde 501.

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
finflow assinaturas                        # assinaturas detectadas no histórico
finflow alertas                            # alertas e dicas do momento
finflow posso 2400 celular --parcelas 10 --cartao ultravioleta   # cabe na projeção?
finflow investimentos                      # posições, rentabilidade e alocação
finflow dividas                            # dívidas e estratégia de quitação
finflow patrimonio                         # evolução do patrimônio mês a mês
finflow serve --lan                        # acesso pela rede local (exige FINFLOW_PASSWORD)
finflow status
finflow espacos                            # espaços (pessoal, casa, empresa)
finflow espacos criar Casa --tipo casa     # novo espaço (casa, empresa ou pessoal)
finflow espacos visao                      # visão unificada de todos os espaços
finflow seed --espaco casa                 # demo da casa (cria o espaço "casa" se faltar); --espaco empresa idem
finflow --espaco casa divisao              # quem pagou, a parte de cada um e acertos sugeridos
finflow --espaco casa divisao acertar Ana Bruno 120,00 [--de-conta X --para-conta Y]
finflow --espaco empresa empresa           # MRR, ARR, churn, queima e runway
finflow --espaco empresa empresa dre       # DRE anual
```

A flag global `--espaco <slug>` (ou `FINFLOW_SPACE`) vale para todos os comandos, inclusive `finflow mcp`.

Contas, cartões, categorias e metas aceitam nome parcial sem diferenciar maiúsculas.

### Autenticação

Por padrão, em uso local sem `FINFLOW_PASSWORD`, a API roda em modo aberto e o `serve` escuta só em `127.0.0.1`. Para abrir na rede local, use `finflow serve --lan` (ou `FINFLOW_LAN=1`), que exige senha:

1. Defina `FINFLOW_PASSWORD` no `.env` com uma senha forte (e opcionalmente `FINFLOW_AUTH_SECRET` e `FINFLOW_API_TOKEN`, um token fixo para scripts, MCP via HTTP e a API pessoal).
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

Tools de leitura: `get_summary`, `get_cashflow`, `get_category_spending`, `get_upcoming_bills`, `list_accounts`, `list_cards`, `list_card_invoices`, `list_transactions`, `list_categories`, `list_budgets`, `list_goals`, `get_insights`, `get_projection`, `get_year_overview`, `list_recurring`, `list_tags`, `get_subscriptions`, `get_alerts`, `can_afford`, `get_cashflow_basis`, `list_investments`, `get_investment_summary`, `list_loans`, `get_debt_strategy`, `get_net_worth_history`, `list_members`, `get_member_balances`, `get_business_metrics`, `get_business_dre`. Escrita: `create_expense`, `create_income`, `create_card_expense`, `transfer`, `pay_invoice`, `set_budget`, `contribute_goal`, `create_recurring`, `export_transactions`. Os valores são em centavos.

#### MCP via HTTP

Com o `serve` rodando, as mesmas tools ficam em `POST /api/mcp` (Streamable HTTP, JSON-RPC 2.0), protegido pelo mesmo Bearer da API. Use como token o `FINFLOW_API_TOKEN` (recomendado) ou a `FINFLOW_PASSWORD`; em modo aberto, sem senha, o cabeçalho é dispensado. No Claude Code:

```bash
claude mcp add --transport http finflow http://localhost:3333/api/mcp --header "Authorization: Bearer <token>"
```

Para outro espaço, acrescente `--header "X-FinFlow-Space: <slug>"` (no stdio, `"args": ["mcp", "--espaco", "<slug>"]`). Outros clientes MCP com transporte HTTP usam a mesma URL e cabeçalho. **Configurações → API e IA** mostra os trechos prontos (stdio e HTTP) e a URL da API. O mesmo token dá acesso à **API pessoal** (`/api/*`, resumo abaixo) para scripts e automações.

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
  mcp/                servidor MCP JSON-RPC 2.0 (stdio e HTTP em /api/mcp)
  ai/                 assistente com a API do Claude (chave do usuário)
  webui/              embedding de apps/web/dist via go:embed
  testutil/           bancos isolados para testes (SQLite em memória + PostgreSQL opcional)
apps/web/             interface React 19 + Vite 8 + Tailwind v4 (e a landing em /inicio)
packages/contracts/   schemas Zod usados pelo frontend
scripts/              build, release (6 plataformas) e instaladores install.sh / install.ps1
.github/workflows/    CI e release por tag (v*)
```

## Licença

[MIT](LICENSE) — use, modifique e redistribua à vontade, mantendo o aviso de copyright.
