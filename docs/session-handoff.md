# Handoff da sessão

## 2026-10-30: bootstrap

O monorepo foi criado do zero. Tudo descrito em `docs/implementation-status.md` foi verificado localmente.

### Decisões e armadilhas encontradas

- **Prisma 7**: o `url` do datasource saiu do `schema.prisma` e foi para o `prisma.config.ts`. O runtime usa `@prisma/adapter-pg`, e o client é gerado em `apps/api/src/generated`, que é ignorado pelo Git e regenerado no `build` e no `typecheck`.
- **Prisma bloqueia `db push --accept-data-loss` quando roda via agente.** Os testes de integração usam `prisma migrate deploy`, que não destrói dados.
- **Portas**: o banco de desenvolvimento usa a **5435**, porque a 5433 já é usada pelo `mapbox-vision-postgres`. O banco de teste usa a 5434. A web usa a **5273** com `strictPort`, porque a 5173 é usada pelo dev server do Mapbox; o Playwright chegou a testar o app errado antes dessa mudança.
- **Biome e NestJS**: é preciso ativar `unsafeParameterDecoratorsEnabled` e desligar `useImportType`. Com `import type`, o `emitDecoratorMetadata` perde a referência das classes injetadas e a injeção de dependência quebra em tempo de execução.
- **Vitest com SWC**: use `module.type: "es6"`. Com `commonjs`, o import do próprio vitest quebra.
- **Zod 4 e React Hook Form**: `z.coerce` gera um tipo de entrada `unknown` e quebra o resolver. Nos formulários, mantenha os campos como string e converta no submit.

### Próximos passos sugeridos

1. Acompanhar a primeira execução da CI e corrigir a imagem do Playwright se a tag não existir.
2. Adicionar autenticação antes de qualquer deploy.
3. Criar telas de gestão de categorias e de edição de transação.
4. Validar o bot do Telegram com um token real.
