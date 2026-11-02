# Estado da implementação

Atualizado em 2026-11-02.

## Verificado localmente (Windows 11, Node 24, Docker)

| Verificação | Resultado |
| --- | --- |
| `pnpm lint` (Biome) | limpo (133 arquivos) |
| `pnpm typecheck` | 8/8 tarefas |
| `pnpm build` | 6/6 pacotes (chunk `dashboard` reduzido de ~400 KB para ~10,6 KB com SVG nativo) |
| `pnpm test` | API 53/53, CLI 3/3 (56 testes unitários) |
| `pnpm test:integration` | 13/13 contra o Postgres `finflow_test` (incluindo subcategorias, edição de transação e autenticação) |
| `pnpm test:e2e` | 32/32 (16 cenários × desktop Chrome e Pixel 7, incluindo `/categorias`, edição de transação e login) |
| Migration `init` + seed | aplicados no banco de desenvolvimento |
| Smoke test da API | summary, contas, cartões, categorias, orçamentos, metas, vencimentos, autenticação e exportação CSV responderam com dados coerentes |
| Smoke test da CLI | `saldo`, `resumo`, `cartoes` e `status` funcionaram contra a API real |
| GitHub Actions (run 41158389037) | os dois jobs passaram no Ubuntu 24.04: checks com integração em Postgres e Playwright no container v1.64.1 |
| Smoke test do MCP | handshake stdio, `tools/list` (19 tools) e `get_summary` funcionaram contra a API real |

## Funcionalidades

- Contas (5 tipos), saldo derivado, arquivamento e exclusão protegida.
- Transferência atômica entre contas; excluir uma perna remove o par.
- Cartões com ciclo de fatura, compras parceladas em até 48x, pagamento atômico de fatura, limite usado e disponível.
- Extrato com filtros de período, conta, cartão, categoria, tipo, status e busca, paginado.
- Criação, edição (`PATCH /transactions/:id`), confirmação com um clique e exclusão de transações pela web.
- Gestão completa de categorias e subcategorias (`parentId`) pela interface web (`/categorias`), com seletor de ícones e cores.
- Exportação em CSV (formato do Excel em português) e OFX 1.02.
- Orçamentos com status OK, WARNING e DANGER.
- Metas com aportes, resgates e cálculo do aporte mensal necessário.
- Dashboard com saldo, métricas do mês, fluxo de caixa e distribuição por categoria em SVG leve (~10,6 KB) e vencimentos dos próximos 7 dias.
- Autenticação (`AuthModule` + `AuthGuard` global + tela de login na web + suporte a Bearer token na CLI e no MCP) ativada ao definir `FINFLOW_PASSWORD`.
- Web responsiva: sidebar no desktop; no mobile, barra superior com atalho para categorias, navegação inferior, botão flutuante e painel inferior de lançamento com 4 abas.
- CLI com 14 comandos e suporte a `--token` / `FINFLOW_TOKEN`.
- Servidor MCP com 19 tools.
- Bot do Telegram: comandos, lançamento rápido e alertas de orçamento.

## Pendente (depende de credencial externa)

- **Entrega real no Telegram**: depende de criar o token no `@BotFather`, preencher `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID` no `.env` e expor a API numa URL HTTPS pública (passo a passo no `README.md`). A integração está coberta por testes unitários e de integração com a Bot API simulada.
