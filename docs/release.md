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
2. Crie e envie a tag:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. O workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) roda `node scripts/release.mjs v0.2.0` e cria a GitHub Release com `gh release create` (notas geradas automaticamente). Se a release já existir, os arquivos são substituídos (`gh release upload --clobber`).

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

## Avisos para quem baixa

- Os binários não são assinados. O Windows SmartScreen pede **Mais informações → Executar assim mesmo**; no macOS, `xattr -d com.apple.quarantine finflow` libera o binário (o `install.sh` já faz isso).
- O repositório precisa ser público para que os links `releases/latest/download` e `raw.githubusercontent.com` funcionem sem autenticação.
