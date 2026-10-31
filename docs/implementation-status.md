# Estado da implementação

Atualizado em 2026-10-30.

## Verificado localmente (Windows 11, Node 24, Docker)

| Verificação | Resultado |
| --- | --- |
| `pnpm lint` (Biome) | limpo |
| `pnpm typecheck` | 8/8 tarefas |
| `pnpm build` | 6/6 pacotes |
| `pnpm test` | API 50/50, CLI 3/3 |
| `pnpm test:integration` | 11/11 contra o Postgres `finflow_test` |
| `pnpm test:e2e` | 26/26 (13 cenários × desktop Chrome e Pixel 7) |
| Migration `init` + seed | aplicados no banco de desenvolvimento |
| Smoke test da API | summary, contas, cartões, orçamentos, metas, vencimentos e exportação CSV responderam com dados coerentes |
| Smoke test da CLI | `saldo`, `resumo` e `cartoes` funcionaram contra a API real |
| GitHub Actions (run 41158389037) | os dois jobs passaram no Ubuntu 24.04: checks com integração em Postgres e Playwright no container v1.64.1 |
| Smoke test do MCP | handshake stdio, `tools/list` (19 tools) e `get_summary` funcionaram contra a API real |

## Funcionalidades

- Contas (5 tipos), saldo derivado, arquivamento e exclusão protegida.
- Transferência atômica entre contas; excluir uma perna remove o par.
- Cartões com ciclo de fatura, compras parceladas em até 48x, pagamento atômico de fatura, limite usado e disponível.
- Extrato com filtros de período, conta, cartão, categoria, tipo, status e busca, paginado.
- Confirmação de pendências com um clique.
- Exportação em CSV (formato do Excel em português) e OFX 1.02.
- Orçamentos com status OK, WARNING e DANGER.
- Metas com aportes, resgates e cálculo do aporte mensal necessário.
- Dashboard com saldo, métricas do mês, fluxo de caixa, distribuição por categoria e vencimentos dos próximos 7 dias.
- Web responsiva: sidebar no desktop; no mobile, navegação inferior, botão flutuante e painel inferior de lançamento com 4 abas.
- CLI com 14 comandos.
- Servidor MCP com 19 tools.
- Bot do Telegram: comandos, lançamento rápido e alertas de orçamento.

## Não verificado ou pendente

- **Entrega real no Telegram**: depende de token de bot e URL pública. Só há cobertura com a Bot API simulada.
- **Autenticação**: o MVP é de uso pessoal e não tem login. Não exponha a API publicamente sem antes adicionar autenticação.
- **Categorias pela interface**: a API tem CRUD completo, mas a web só lista as categorias; a criação acontece pelo seed ou pela API.
- **Edição de transação na web**: a API suporta (`PATCH /transactions/:id`), mas a interface só cria, confirma e exclui.
- **Subcategorias**: o schema tem `parentId`, mas nenhuma tela usa esse campo.
- **Bundle**: o chunk do dashboard tem cerca de 400 KB por causa do Recharts. Ele já é carregado sob demanda, mas ainda pode ser reduzido.
