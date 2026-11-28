# Versões verificadas em 28/11/2026

Política de versões: [.claude/skills/finflow-release/SKILL.md](../.claude/skills/finflow-release/SKILL.md)
(SemVer com a convenção pré-1.0, a mesma do PC Remote). Os lockfiles (`go.sum`, `pnpm-lock.yaml`) são a referência
exata de todas as dependências transitivas; as versões abaixo foram usadas nas builds e testes desta entrega.

| Componente | Versão |
|---|---|
| FinFlow (binário, app no navegador, imagem Docker) | 0.6.0 |
| Go (toolchain / `go.mod`) | 1.26.1 / 1.26.1 |
| Node / pnpm | 24.21.0 (CI: 22.x; mínimo 22.12) / 10.11.1 |
| SQLite desktop: modernc.org/sqlite (pure Go, sem CGO) | 1.63.0 |
| SQLite navegador: @sqlite.org/sqlite-wasm (VFS opfs-sahpool) | 3.54.1-build1 |
| PostgreSQL opcional: pgx / imagem de teste | 5.13.0 / postgres:16-alpine |
| Planilhas: excelize | 2.11.1 |
| Assistente: anthropic-sdk-go (modelo `claude-opus-5-5`) | 2.15.0 |
| CLI: cobra | 1.10.2 |
| Web: React / Vite / TypeScript / Tailwind | 19.2.8 / 8.3.4 / 6.0.3 / 4.3.3 |
| Web: TanStack Query / Zod / Recharts | 5.105.0 / 4.3.6 / 2.15.4 |
| Testes de interface: Playwright | 1.64.1 |
| Docker: build web / build Go / runtime | node:22.23.0-bookworm-slim / golang:1.26.3-alpine3.23 / alpine:3.23.4 |

Binários sem assinatura de código (Authenticode/notarização): o Windows SmartScreen e o Gatekeeper do macOS avisam
na primeira execução.

## Compatibilidade entre versões

O banco é um arquivo SQLite. Toda versão abre e migra sozinha o banco das anteriores (migrações aditivas); o mesmo
arquivo serve de backup entre o programa instalado e o app no navegador.

| Versão | Banco e backups | Configuração e integrações |
|---|---|---|
| 0.6.0 | Abre bancos e backups `.db`/`.zip` de 0.2.0 a 0.5.0 sem ação manual | **Incompatível com 0.5.0 no assistente:** provedores locais (Ollama e servidores compatíveis), `PUT /api/ai/config`, `GET /api/ai/models`, variáveis `FINFLOW_AI_*`/`FINFLOW_OLLAMA_URL` e o profile `ai` do Docker foram removidos. O assistente usa só a chave do Claude (`ANTHROPIC_API_KEY` ou Configurações); sem chave, fica desativado. Um provedor local salvo na 0.5.0 é ignorado |
| 0.5.0 | Abre bancos de 0.2.0 a 0.4.0 | Assistente com Ollama/servidores compatíveis ou Claude; `docker compose --profile ai` |
| 0.4.0 | Abre bancos de 0.2.0 e 0.3.0; app no navegador lê backups `.db` do programa instalado | Duplo clique escolhe outra porta se a 3333 estiver ocupada |
| 0.3.0 | Abre bancos de 0.2.0 | `serve` passa a escutar só em 127.0.0.1 (use `--lan` para a rede local) |
| 0.2.0 | Primeira versão distribuída | — |

Um banco aberto por uma versão nova continua abrindo numa versão anterior da mesma família (`0.6.x`), porque as
migrações só acrescentam tabelas e colunas. Voltar para uma família anterior não é suportado: guarde um backup antes
de atualizar entre MINORs.
