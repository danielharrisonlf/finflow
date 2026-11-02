# Landing B2C e temas

A landing foi integrada em `/inicio`, fora de `AppShell`, sem alterar a entrada `/` do painel ou seu controle de autenticação. A versão pública continua em https://finflow-landing-umber.vercel.app/ no repositório independente `finflow-landing`.

## Conteúdo e acesso

O conteúdo apresenta contas, faturas, orçamentos, metas e mensagens para o público geral. A seção **Conecte seu agente de IA via CLI ou MCP** demonstra conversas, sem terminal técnico. Todas as interações de marketing são simulações locais e identificadas como tal.

Na rota integrada, **Acessar minha conta** leva a `/`. A landing pública só leva a um app externo quando `VITE_APP_URL` está configurada com um endereço verificado; sem ele, um diálogo explica a disponibilidade e oferece a demonstração.

## Sistema de temas

`index.html` resolve `finflow-theme` antes do React, usa a preferência do sistema quando não há escolha explícita e aplica `.light` ou `.dark`, `data-theme` e `color-scheme`. O componente `ThemeToggle` oferece alternância e retorno à preferência do sistema. Ele acompanha mudanças do sistema e de outras abas e mantém a sessão utilizável quando `localStorage` está bloqueado.

`src/components/landing/theme.css` define tokens `--ff-*` para superfícies, textos, bordas, entradas, saídas e alertas. `styles.css` expõe tokens Tailwind `app-*` e as cores contábeis. Os componentes do painel, formulários, login e gráficos usam esses tokens em ambos os temas. As regras visuais da landing são limitadas a `.finflow-landing`.

## Fonte canônica

Os oito arquivos em `src/components/landing/` vêm do repositório irmão `finflow-landing/src/components/landing/`. Edite a fonte canônica e rode `node scripts/sync-app.mjs` nesse repositório para copiar os componentes, a fonte local com licença OFL e a inicialização do tema. O script não altera as regras de negócio ou a autenticação.

## Validação

Os comandos padrão `pnpm typecheck`, `pnpm build` e `pnpm lint` passaram no monorepo durante a implementação. Os testes de navegador ficam em `apps/web/e2e/landing-themes.spec.ts` e no repositório público. Cobrem a rota pública, manutenção do acesso existente, temas e interações. Evidências visuais são gravadas localmente em `test-results/`.

Resultado final em 2 de novembro de 2026: 44/44 testes na suíte completa do frontend do app (incluindo 12 novos cenários da landing e temas) e 9/9 na landing pública. O axe não reportou violações WCAG 2.1 AA nos estados verificados em ambos os temas após concluir as transições. A revisão independente foi aceita sem defeitos materiais pendentes. Screenshots de 390 e 1440 px foram inspecionados.

As alterações preexistentes do workspace foram preservadas; este trabalho não publica o monorepo privado. Semantica não está disponível nesta sessão, por isso as decisões foram registradas aqui.
