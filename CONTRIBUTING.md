# Como contribuir com o FinFlow

Obrigado pelo interesse! O FinFlow é gratuito, de código aberto ([MIT](LICENSE)) e local: os dados de quem usa ficam num arquivo SQLite no próprio computador (ou no navegador, em `/app/`). Toda contribuição precisa manter isso: nada de telemetria, contas ou serviços de terceiros obrigatórios.

## Preparar o ambiente

Requisitos: [Go 1.26+](https://go.dev/dl/), [Node 22.12+](https://nodejs.org/pt/download) e pnpm pelo Corepack (`corepack enable`). Docker é opcional (PostgreSQL de teste e imagem do app).

```bash
git clone https://github.com/danielharrisonlf/finflow
cd finflow
pnpm install
pnpm start -- --seed      # compila o que mudou, cria dados de exemplo e abre o FinFlow
```

Para não misturar com os seus dados reais, use um banco separado: `FINFLOW_DB_PATH=./dev.db pnpm start -- --seed` (ou `cp .env.example .env`, que já aponta para `finflow.db` na raiz do projeto).

### Desenvolvimento com hot-reload

```bash
pnpm dev:api              # API Go em http://localhost:3333 (reinicie depois de mudar Go)
pnpm dev:web              # Vite em http://localhost:5273 (chama a API em :3333)
```

- Landing pública: `http://localhost:5273/inicio` (fonte em `apps/web/src/components/landing/`).
- App no navegador (WebAssembly): `pnpm build:browser` e `pnpm --filter @finflow/web preview:browser` (http://localhost:5274/app/).
- Assistente local: com o [Ollama](https://ollama.com) rodando (`ollama pull qwen2.5:7b`), defina `FINFLOW_AI_PROVIDER=ollama` ou escolha em **Configurações → Assistente**.

## Testes e verificações

Rode o que corresponde ao que você mudou antes de abrir o PR. O CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) roda tudo.

| Mudou | Rode |
| --- | --- |
| Go (`cmd/`, `internal/`) | `gofmt -l cmd internal` (vazio), `go vet ./...`, `go test ./...` |
| Web, contratos ou scripts | `pnpm lint`, `pnpm typecheck`, `pnpm build:web` |
| Telas do app | `pnpm test:e2e` (Playwright, desktop + Pixel 7) |
| Landing | `pnpm --filter @finflow/web exec playwright test e2e/landing-themes.spec.ts` e o mesmo com `--config=playwright.landing.config.ts` (Android + iOS) |
| App no navegador | `pnpm build:browser && pnpm test:e2e:browser` |
| Dockerfile ou compose | `docker build -t finflow:local .` e `docker compose config -q` |

Testes em PostgreSQL (opcional): `docker compose --profile test up -d db_test` e `FINFLOW_TEST_DATABASE_URL="postgres://finflow:finflow@localhost:5434/finflow_test?sslmode=disable" go test ./...`.

## Convenções

- **Idioma**: interface, mensagens de erro, CLI, documentação e comentários em português do Brasil. Identificadores de código em inglês. Termos de produto seguem o que já existe (lançamento, compromisso, fatura, espaço).
- **Go**: `gofmt`, sem dependências novas sem necessidade (o binário é único, pure Go, `CGO_ENABLED=0`). Regras de negócio em `internal/service`, SQL em `internal/store`, HTTP em `internal/httpapi`; dinheiro sempre em centavos (`int64`), datas `YYYY-MM-DD` no fuso `FINFLOW_TZ`.
- **Web**: React + TypeScript estrito, Biome (`pnpm lint:fix` formata), Tailwind v4 e componentes Radix já usados no projeto. Acessibilidade AA nos dois temas, alvos de toque de 44px e nada de rolagem horizontal em 320px.
- **API**: mudou um contrato? Atualize `packages/contracts`, o README (seção API) e, se houver, o documento em `docs/`.
- **Escritas do assistente** são sempre propostas confirmadas pelo usuário; não crie ferramentas que gravem direto.
- **Commits**: mensagens curtas no imperativo, com prefixo do tipo e escopo quando ajudar (`feat(web): …`, `fix(cli): …`, `docs: …`, `test: …`).
- **CHANGELOG**: descreva mudanças visíveis para quem usa em [`CHANGELOG.md`](CHANGELOG.md), na seção “Não lançado”.

## Pull requests

1. Crie um branch a partir de `main`.
2. Mantenha o PR focado; explique o porquê e como testar.
3. Inclua testes para regras novas ou bugs corrigidos e capturas de tela para mudanças visuais.
4. Garanta o CI verde.

Releases e o site público seguem [docs/release.md](docs/release.md).
