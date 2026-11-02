# Handoff da sessão

## 2026-11-02: autenticação, categorias/subcategorias, edição de transações e otimização do bundle

### O que foi entregue nesta sessão

- **Autenticação (`AuthModule` + `LoginPage`)**:
  - `GET /api/auth/status` e `POST /api/auth/login` na API, com tokens de sessão assinados via HMAC-SHA256 (30 dias) e `AuthGuard` global.
  - Quando `FINFLOW_PASSWORD` está definida no `.env`, todas as rotas (exceto `/api/auth/status`, `/api/auth/login` e `/api/telegram/webhook`) exigem Bearer token (ou `?token=` na exportação CSV/OFX). Sem `FINFLOW_PASSWORD`, a API continua aberta em dev local.
  - Tela de login na web (`apps/web/src/pages/login.tsx`), botão de sair na sidebar/barra mobile, e suporte a `--token` / `FINFLOW_TOKEN` / `FINFLOW_PASSWORD` no `@finflow/api-client`, CLI e MCP.
- **Gestão de categorias e subcategorias na web (`/categorias`)**:
  - Nova página `apps/web/src/pages/categories.tsx` com listagem separada por Despesas e Receitas, criação, edição (`PATCH /api/categories/:id`) e exclusão (`DELETE /api/categories/:id`), seletor visual de 21 ícones Lucide, paleta de cores e vínculo de subcategoria (`parentId`).
  - Exibição hierárquica (`Pai › Subcategoria`) nos seletores de categoria em lançamentos, extrato e orçamentos.
- **Edição de transações na web**:
  - Botão de edição em cada linha do extrato (`apps/web/src/pages/transactions.tsx`) abrindo `EditTransactionSheet` (`PATCH /api/transactions/:id`), respeitando a invariante de domínio (transações simples editam valor e conta; compras no cartão, transferências e pagamentos de fatura editam descrição, categoria, data, status e observações).
- **Redução do bundle do Dashboard**:
  - Gráficos `CashflowChart` e `CategoryDonut` migrados para SVG nativo responsivo, reduzindo o chunk `dashboard` de ~400 KB para **10,6 KB** (3,6 KB gzip).

## 2026-10-30: bootstrap

O monorepo foi criado do zero. Tudo descrito em `docs/implementation-status.md` foi verificado localmente.

### Decisões e armadilhas encontradas

- **Prisma 7**: o `url` do datasource saiu do `schema.prisma` e foi para o `prisma.config.ts`. O runtime usa `@prisma/adapter-pg`, e o client é gerado em `apps/api/src/generated`, que é ignorado pelo Git e regenerado no `build` e no `typecheck`.
- **Prisma bloqueia `db push --accept-data-loss` quando roda via agente.** Os testes de integração usam `prisma migrate deploy`, que não destrói dados.
- **Portas**: o banco de desenvolvimento usa a **5435**, porque a 5433 já é usada pelo `mapbox-vision-postgres`. O banco de teste usa a 5434. A web usa a **5273** com `strictPort`, porque a 5173 é usada pelo dev server do Mapbox; o Playwright chegou a testar o app errado antes dessa mudança.
- **Biome e NestJS**: é preciso ativar `unsafeParameterDecoratorsEnabled` e desligar `useImportType`. Com `import type`, o `emitDecoratorMetadata` perde a referência das classes injetadas e a injeção de dependência quebra em tempo de execução.
- **Vitest com SWC**: use `module.type: "es6"`. Com `commonjs`, o import do próprio vitest quebra.
- **Zod 4 e React Hook Form**: `z.coerce` gera um tipo de entrada `unknown` e quebra o resolver. Nos formulários, mantenha os campos como string e converta no submit.
