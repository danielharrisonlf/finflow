# Como publicar uma release

Cada release publica seis pacotes com nomes fixos (sem versão no nome) e um `checksums.txt`:

```text
finflow-windows-amd64.zip    finflow-windows-arm64.zip
finflow-darwin-amd64.tar.gz  finflow-darwin-arm64.tar.gz
finflow-linux-amd64.tar.gz   finflow-linux-arm64.tar.gz
checksums.txt
```

Por isso `https://github.com/danielharrisonlf/finflow/releases/latest/download/<arquivo>` sempre aponta para a última release estável. A landing, o README e os scripts `scripts/install.sh` / `scripts/install.ps1` dependem desses nomes: não os altere sem atualizar os três.

Cada pacote contém o binário (`finflow` ou `finflow.exe`) e um `LEIA-ME.txt` com como abrir, onde ficam os dados e como parar.

## Caminho recomendado: tag + workflow

1. Confira que `main` está verde no CI.
2. No [`CHANGELOG.md`](../CHANGELOG.md), renomeie a seção “Não lançado” para a versão e a data (por exemplo `## [0.3.0] — 2026-12-10`), abra uma nova “Não lançado” vazia e atualize os links de comparação no fim do arquivo.
3. Crie e envie a tag:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

4. O workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) roda `node scripts/release.mjs v0.2.0` e cria a GitHub Release com `gh release create` (notas geradas automaticamente). Se a release já existir, os arquivos são substituídos (`gh release upload --clobber`).

Tags com hífen (`v0.3.0-rc.1`) viram **pre-release** e não entram em `releases/latest`.

## Caminho manual: build local + gh

```bash
pnpm install
pnpm release v0.2.0                       # = node scripts/release.mjs v0.2.0
gh release create v0.2.0 dist/release/* --title "FinFlow v0.2.0" --generate-notes
```

Sem versão explícita, o script usa `FINFLOW_VERSION`, a tag do GitHub Actions ou `git describe --tags --always --dirty`. Com `--skip-web`, reaproveita o web já embutido em `internal/webui/dist` (útil para repetir só a compilação Go).

## O que o script faz

1. `pnpm build:web` uma vez (contratos, web e cópia para `internal/webui/dist`, embutido via `go:embed`).
2. Para cada plataforma: `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -trimpath -ldflags "-s -w -X main.version=<v>"`. O SQLite é pure Go, então não há toolchain C.
3. Empacota em `.zip` (Windows) ou `.tar.gz` (permissão 0755 no binário), escritos em Node puro, sem `zip`/`tar` externos.
4. Grava `dist/release/checksums.txt` no formato do `sha256sum -c`.

`dist/` é ignorado pelo Git.

## Conferência rápida

```bash
cd dist/release && sha256sum -c checksums.txt
tar -tzf finflow-linux-amd64.tar.gz          # finflow, LEIA-ME.txt
```

No Windows, extraia o `.zip`, rode `finflow.exe version` e dê dois cliques em `finflow.exe`: o servidor sobe e o navegador abre em http://localhost:3333.

Os instaladores aceitam `FINFLOW_DOWNLOAD_URL` para testar contra uma pasta local servida por HTTP:

```bash
(cd dist/release && python -m http.server 8765) &
FINFLOW_DOWNLOAD_URL=http://localhost:8765 FINFLOW_INSTALL_DIR=/tmp/ff sh scripts/install.sh
```

## Site público (landing + app no navegador)

O site da Vercel (`finflow-landing-umber.vercel.app`) publica a landing em `/` e o FinFlow no navegador em `/app/` ([docs/browser-runtime-contract.md](browser-runtime-contract.md) §5). Ele não depende das releases do GitHub: refaça e publique sempre que o app ou a landing mudarem.

```bash
pnpm install
pnpm build:browser                                   # wasm + app → apps/web/dist-browser
node apps/web/scripts/build-public-landing.mjs       # pacote em apps/web/test-results/vercel-landing
node apps/web/scripts/serve-public-package.mjs 4180  # serve o pacote com as mesmas rotas da Vercel
node apps/web/scripts/verify-public-landing.mjs http://localhost:4180/   # em outro terminal
```

`build-public-landing.mjs` exige o projeto Vercel vinculado em `../finflow-landing/.vercel/project.json` e o `dist-browser` completo (`index.html`, `sw.js`, `manifest.webmanifest` e `runtime/`). Ele monta `.vercel/output` (Build Output API v3):

- `static/` com a landing e `static/app/` com o app;
- `config.json` com fallback de SPA (`/app/*` → `/app/index.html`, exceto `assets/`, `runtime/`, `icons/` e `fonts/`), `/app` → `/app/` (308), `Content-Type: application/wasm`, cache imutável em `/assets/*` e `/app/assets/*`, `no-cache` no `index.html`, no `sw.js` e nas rotas do app, e revalidação em `/app/runtime/*`.

A verificação confere a landing (temas, larguras, acessibilidade, CTAs **Baixar grátis**, **Código no GitHub** e **Experimentar no navegador**, as abas Executável/Código-fonte/Docker da seção de download, FAQ) e o app: `/app/` carrega, o runtime sobe, os dados de exemplo são criados e nenhuma requisição sai do próprio site; também confere o fallback de SPA, o MIME do `.wasm` e o manifest com `start_url`/`scope` `/app/`. O Chromium é obrigatório; no WebKit o app é reportado como `runtime-unsupported` se o motor não tiver o OPFS que o runtime usa.

Para publicar, a partir do pacote gerado:

```bash
cd apps/web/test-results/vercel-landing
vercel deploy --prebuilt            # prévia; confira e rode a verificação contra a URL da prévia
vercel deploy --prebuilt --prod     # produção
node ../../scripts/verify-public-landing.mjs https://finflow-landing-umber.vercel.app/
```

Cada build do app gera um `sw.js` com versão nova; quem estiver com o app aberto vê **Nova versão disponível — Recarregar** e os dados (no navegador da pessoa) não são tocados.

## Código-fonte e Docker

Não há artefato extra a publicar para quem roda a partir do código: `pnpm start` ([scripts/start.mjs](../scripts/start.mjs)) compila o web e o binário na máquina da pessoa, e `docker compose up -d` constrói a imagem localmente a partir do [`Dockerfile`](../Dockerfile). Antes de marcar a tag, confira os dois caminhos:

```bash
# clone limpo, como quem chega pelo README (use uma porta e um banco descartáveis)
git clone https://github.com/danielharrisonlf/finflow /tmp/ff && cd /tmp/ff
pnpm install && FINFLOW_NO_BROWSER=1 API_PORT=3411 FINFLOW_DB_PATH=/tmp/ff.db pnpm start -- --seed
curl -s http://127.0.0.1:3411/api/system/info

# imagem e compose (o job `docker` do CI faz o mesmo build e um smoke test, sem push)
docker build --build-arg FINFLOW_VERSION=v0.5.0 -t finflow:local .
FINFLOW_PORT=3412 docker compose -p finflow-check up -d app && docker compose -p finflow-check ps
docker compose -p finflow-check down -v
docker compose --profile ai config -q      # valida o profile do Ollama sem baixar o modelo
```

A imagem não é publicada em registry; quem usa Docker constrói a partir do repositório. As tags das imagens base (`node`, `golang`, `alpine`, `ollama/ollama`) ficam fixas no `Dockerfile` e no `docker-compose.yaml`: atualize-as de propósito, junto com o `go.mod` e o `engines` do `package.json`.

## Avisos para quem baixa

- Os binários não são assinados. O Windows SmartScreen pede **Mais informações → Executar assim mesmo**; no macOS, `xattr -d com.apple.quarantine finflow` libera o binário (o `install.sh` já faz isso).
- O repositório precisa ser público para que os links `releases/latest/download` e `raw.githubusercontent.com` funcionem sem autenticação.
