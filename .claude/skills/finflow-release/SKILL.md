---
name: finflow-release
description: Política de versionamento e roteiro de release do FinFlow (binário Go com web embutida, CLI e MCP; app no navegador em /app/; imagem Docker; landing pública). Use antes de qualquer release, bump de versão, tag, publicação no GitHub ou deploy da landing, e quando alguém perguntar "que versão é essa?", "isso é major, minor ou patch?", "posso publicar?", "como lanço a próxima versão?". Também ao mexer no schema do banco, nas rotas da API, nas tools do MCP, nos comandos da CLI, nas variáveis de ambiente, no armazenamento do navegador ou nos nomes dos assets de release, porque isso decide o tipo do bump.
---

# Versionamento e release do FinFlow

Mesma política da skill `pc-remote-release` (PC Remote): **Semantic Versioning 2.0.0** com a convenção pré-1.0 de
**Cargo/release-please**, mensagens no formato **Conventional Commits** e notas no estilo **Keep a Changelog**, em
pt-BR. Fontes e o porquê de cada regra em [references/fundamentos.md](references/fundamentos.md).

## 1. Leia a versão como `MAJOR.MINOR.PATCH`

Enquanto o projeto estiver em `0.y.z` (desenvolvimento inicial), cada posição "desce" uma casa:

| Mudança | Antes da 1.0.0 (`0.y.z`) | A partir da 1.0.0 | Commits que a indicam |
|---|---|---|---|
| **Incompatível** (quebra) | MINOR: `0.6.3 → 0.7.0` | MAJOR: `1.4.2 → 2.0.0` | `feat!:`, `fix!:` ou rodapé `BREAKING CHANGE:` |
| **Funcionalidade nova compatível** | PATCH: `0.6.3 → 0.6.4` | MINOR: `1.4.2 → 1.5.0` | `feat:` |
| **Correção compatível** | PATCH: `0.6.3 → 0.6.4` | PATCH: `1.4.2 → 1.4.3` | `fix:`, `perf:`, `revert:` |
| Sem efeito para o usuário | não lança | não lança | `docs:`, `test:`, `refactor:`, `chore:`, `ci:`, `build:`, `style:` |

Vários commits na mesma release: vale o **maior** efeito. Um único `feat!` faz da release inteira uma quebra.

Em `0.y.z` o número do meio só sobe quando algo **obriga** o usuário a agir de forma incompatível. O MINOR vira um
aviso claro: "leia as notas antes de atualizar".

## 2. O que é "incompatível" no FinFlow

A "API pública" do FinFlow é tudo de que o usuário, um script ou outra peça já instalada depende. Quebra é qualquer
item abaixo:

1. **Banco de dados** (`internal/db/schema.go`, `internal/store`): uma versão nova que não abre ou não migra sozinha
   um banco de versão anterior; migração que apaga ou reinterpreta dados; mudança de unidade (dinheiro sempre em
   centavos inteiros, datas `YYYY-MM-DD`); backup `.db` que deixa de abrir no desktop ou no navegador. Coluna ou
   tabela **nova e aditiva** (`ADD COLUMN`/`CREATE TABLE IF NOT EXISTS`, que a versão anterior ignora) **não** é quebra.
2. **API REST** (`/api/*`, `internal/httpapi`): remover ou renomear rota, campo ou valor de enum; mudar semântica,
   unidade ou código de status; exigir campo antes opcional; mudar autenticação. Campo **novo** em resposta ou campo
   **opcional** novo em entrada não é quebra. A CLI remota (`--api`), o MCP via HTTP e scripts de terceiros dependem dela.
3. **MCP** (`internal/mcp`): remover ou renomear tool, tornar argumento obrigatório, mudar o formato do resultado.
4. **CLI** (`internal/cli`): remover ou renomear comando ou flag, mudar o significado de uma flag ou o código de saída.
5. **Configuração e dados salvos**: variável de ambiente ou chave do `.env` removida ou renomeada; valor salvo em
   `settings` que a versão nova deixa de ler; mudança de padrão que quebra uma instalação existente (porta, bind,
   local do banco).
6. **Armazenamento do navegador** (`apps/web/src/runtime/**`, `internal/db/browser.go`): caminho dos arquivos no
   OPFS (`/finflow/*.db`), catálogo de espaços ou formato do backup `.zip` que a versão nova não reconhece.
7. **Contrato de distribuição** (ver §5): padrão da tag, nomes dos assets, `checksums.txt`, URLs usadas pela landing
   e pelos instaladores, serviços e volumes do `docker-compose.yaml`, endereço `/app/`.
8. **Remoção** de uma funcionalidade que o usuário usa.

Se não está na lista e não muda o comportamento esperado, é PATCH. Na dúvida entre quebra e compatível, trate como
**quebra**.

## 3. Uma versão para todas as superfícies

O binário (desktop, CLI, MCP), o app no navegador (`finflow.wasm`, compilado do mesmo commit), a imagem Docker e a
landing que aponta para os downloads saem **juntos e com o mesmo número**. Não existe versão separada do app no
navegador: o `/app/` publicado é sempre o da última release estável.

## 4. Onde a versão mora

- `package.json` da raiz, `apps/web/package.json` e `packages/contracts/package.json`: campo `version` = `X.Y.Z`.
  O `scripts/build-wasm.mjs` lê a versão da raiz quando não recebe argumento.
- Binário: `-ldflags "-X main.version=vX.Y.Z"` (`scripts/release.mjs <tag>`; o workflow usa a tag). Builds locais
  sem tag mostram `dev`.
- `CHANGELOG.md` (entrada `## [X.Y.Z] — AAAA-MM-DD` e links no fim), `docs/releases/X.Y.Z.md` (notas da release no
  GitHub) e `docs/VERSIONS.md` (versões verificadas e compatibilidade).
- **Tag:** `vX.Y.Z`, no commit de merge na `main`. Versões de teste: `vX.Y.Z-rc.N`, publicadas como pré-release.

## 5. Contrato de distribuição (não quebre)

A landing, os instaladores e as instruções do README procuram as versões assim. Mudar qualquer item é quebra (§2.7)
e exige plano de migração.

| Quem procura | O que precisa existir |
|---|---|
| Landing e README (botão Baixar) | Release **Latest** (não pré-release, não rascunho) com `finflow-windows-amd64.zip`, `finflow-windows-arm64.zip`, `finflow-darwin-amd64.tar.gz`, `finflow-darwin-arm64.tar.gz`, `finflow-linux-amd64.tar.gz`, `finflow-linux-arm64.tar.gz` em `.../releases/latest/download/` |
| `scripts/install.sh` / `install.ps1` | os mesmos assets + `checksums.txt` no formato do `sha256sum` (`<hash>  <arquivo>`), lidos de `raw.githubusercontent.com/.../main/scripts/` |
| Docker | `docker compose up -d` com o serviço `app` e o volume `finflow_data` (o banco do usuário mora nele) |
| App no navegador | `/app/` na landing pública; banco em `/finflow/*.db` no OPFS; backups `.db` e `.zip` (com `spaces.json`) restauráveis |
| Quem atualiza o binário à mão | o arquivo do banco no lugar de sempre (`%AppData%\finflow\finflow.db`, `~/.config/finflow/`, `~/Library/Application Support/finflow/`) aberto e migrado sem ação manual |

Versões em teste ficam invisíveis para quem segue `releases/latest`: tag `vX.Y.Z-rc.N`, que o workflow marca como
pré-release (tags com `-`).

## 6. Roteiro de release

1. **Descubra o bump:** `node .claude/skills/finflow-release/scripts/suggest-version.mjs --fetch` na raiz do repo. O
   script lista os commits desde a última tag por tipo, sugere a versão e avisa quando arquivos sensíveis do §2
   mudaram. Revise o §2 manualmente nesses avisos: o script lê só as mensagens dos commits.
2. **Explique ao usuário antes de aplicar**, no formato do §8, e espere a confirmação se houver quebra.
3. **Commit de release** (branch `release/X.Y.Z` ou dentro do PR da funcionalidade): `chore(release): X.Y.Z` com os
   arquivos do §4 — versões dos três `package.json`, entrada no `CHANGELOG.md`, `docs/releases/X.Y.Z.md` e
   `docs/VERSIONS.md`.
4. **Testes:** `gofmt -l cmd internal` vazio, `go vet ./...`, `GOOS=js GOARCH=wasm go vet ./cmd/finflow-wasm/
   ./internal/...`, `go test ./...` (e em PostgreSQL com `FINFLOW_TEST_DATABASE_URL`), `go mod tidy -diff`,
   staticcheck, `pnpm lint`, `pnpm typecheck`, `pnpm test:e2e`, `pnpm build:browser` + `pnpm test:e2e:browser`,
   suítes da landing e `docker compose config -q`.
5. **PR → CI verde → merge na `main`** (sem force-push na `main`).
6. **Tag** `vX.Y.Z` no commit de merge e push da tag. O workflow `release.yml` gera os 6 pacotes + `checksums.txt` e
   publica a release como **Latest** com as notas de `docs/releases/X.Y.Z.md`.
7. **Landing e app no navegador:** `pnpm build:browser`, `node apps/web/scripts/build-public-landing.mjs`,
   `vercel deploy --prebuilt --prod --yes --cwd apps/web/test-results/vercel-landing` (gere o pacote logo antes do
   deploy: o Playwright apaga `test-results/`).
8. **Confira pelo caminho público**, sem login:
   - `api.github.com/repos/danielharrisonlf/finflow/releases/latest` mostra a tag nova;
   - `.../releases/latest/download/checksums.txt` responde 200 e o `.zip` baixado tem o hash publicado;
   - o binário baixado responde `finflow version` com `vX.Y.Z` e sobe pelo duplo clique;
   - `node apps/web/scripts/verify-public-landing.mjs https://finflow-landing-umber.vercel.app/` passa (landing e `/app/`).

## 7. Nunca faça

- **Não baixe nem reaproveite números.** Uma versão publicada é imutável (SemVer, regra 3). Depois de `0.6.0` vem
  `0.6.1`, nunca `0.5.x` de novo.
- **Não apague, mova ou recrie uma tag publicada.** Errou? Lance a próxima PATCH com a correção.
- **Não troque o arquivo de uma versão já publicada** por bytes diferentes com o mesmo número (o `checksums.txt`
  publicado deixaria de bater).
- **Não publique como pré-release** uma versão que o botão Baixar precisa ver, nem como Latest uma versão em teste.
- **Não mude** nomes de asset, padrão de tag, formato do `checksums.txt`, serviços/volumes do Docker ou o caminho do
  banco sem tratar como quebra.
- **Não lance a 1.0.0 por empolgação**: siga os critérios do §9.

## 8. Como apresentar a proposta ao usuário

```
FinFlow: 0.6.0 → 0.6.1  (PATCH, compatível)
  • feat: <resumo>             → funcionalidade compatível
  • fix: <resumo>              → correção
  Banco: migração aditiva automática; backups .db anteriores continuam abrindo.
  API/MCP/CLI: sem remoções.
  Atualização: baixar o novo executável (ou git pull + pnpm start, ou docker compose up -d --build); dados preservados.
```

Diga explicitamente quando houver quebra: o que o usuário precisa fazer (ajustar `.env`, refazer uma configuração,
atualizar um script) e por que o MINOR subiu.

## 9. Quando ir para a 1.0.0

SemVer: "se o software já é usado em produção, provavelmente já deveria ser 1.0.0". Aqui, os critérios são:

- schema do banco declarado estável, com migrações testadas a partir de cada versão publicada;
- API REST, tools do MCP e comandos da CLI documentados como contrato estável;
- binários assinados (Authenticode no Windows, notarização no macOS);
- uso real por algum tempo com backup e restauração exercitados (desktop e navegador);
- atualização de uma versão para a seguinte validada nos três caminhos (executável, código-fonte e Docker).

Ao lançar a 1.0.0, a tabela do §1 passa a usar a coluna "A partir da 1.0.0", e os contratos do §2 e do §5 só quebram
em MAJOR.
