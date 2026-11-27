# Landing B2C e temas

> O ajuste visual após o feedback do usuário, descrito ao final, é a versão atual: copy original restaurada e hero com painel compacto. A landing em `/inicio` continua comunicando pré-lançamento. Os registros anteriores são históricos.

## Refatoração visual

A abertura usa “Seu dinheiro em ordem. Mais espaço para viver.”, seguida de um dashboard amplo com números e controles legíveis. A interface não é comprimida por regras de altura do notebook. Cartões e orçamento precedem a experiência móvel; celular e Telegram ficam na mesma narrativa. O assistente ocupa uma superfície distinta, e a chamada final usa uma pilha vertical acessível no celular. Os temas, dados ilustrativos e interações locais foram preservados.

Referências de composição consultadas: [Copilot Money](https://www.copilot.money/), [Monarch](https://www.monarch.com/) e [Origin](https://useorigin.com/). Capturas de referência e comparações antes/depois estão em `.qa/` no repositório público, ignoradas pelo Git e pela publicação.

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

## Reformulação de pré-lançamento — 3 de novembro de 2026

Escopo implementado: landing integrada em `apps/web`, acessível em `/inicio`. O painel autenticado continua na rota `/`, com o acesso e as regras de negócio preservados. Nenhuma dependência de produção foi adicionada.

### Diagnóstico e decisões

- O painel anterior reduzia textos a 7–9px no mobile, escondia os orçamentos e usava curvas sem escala monetária. A nova composição reorganiza os blocos e mantém limites disponíveis por expansão.
- O hero apresenta “Seu dinheiro mais organizado. Sua vida mais tranquila.” e “FinFlow · disponível em breve”. As chamadas de demonstração gratuita foram substituídas por navegação para seções reais. Não há cadastro ou lista de espera sem implementação.
- A hierarquia segue perguntas práticas: entender gastos, escolher limites, antecipar parcelas e planejar objetivos. Como funciona, celular, calculadora, conversa ilustrativa e dúvidas complementam a apresentação. Recursos de integração não têm disponibilidade prometida no lançamento.
- A direção preserva verde, tipografia Geist local e temas claro/escuro. O CSS da landing foi reorganizado com estilos mobile-first e ampliações em breakpoints, sem esconder overflow para disfarçar problemas.
- Referências consultadas no UItopic: [dashboard financeiro editorial](https://uitopic.com/design/editorial-finance-dashboard-with-card-system), [indicadores financeiros em painéis](https://uitopic.com/design/portfolio-dashboard-with-card-based-kpi-panels) e [hero com painel financeiro](https://uitopic.com/design/wealth-management-landing-hero-with-embedded-dashboard-card). Foram usadas como inspiração de hierarquia e apresentação de dados.

### Gráficos e comportamento

`FinanceChart.tsx` desenha a evolução de saldo e barras de entradas/saídas a partir dos mesmos valores em centavos que alimentam os indicadores. Cada período tem suas entradas, despesas e saldo inicial. O gasto fictício atualiza totais e série; desfazer restaura ambos. A escala mostra reais e as datas são controles de toque/teclado, com leitura textual e tabela expandida para consulta.

A distribuição por categoria usa barras horizontais com valores explícitos. Os limites têm percentuais e mensagens de estado; no painel são medidores semânticos. A calculadora mantém aritmética em centavos e funciona localmente, sem juros ou rendimentos presumidos.

Foram preservados período, gasto/desfazer, download CSV com BOM, telefone, exemplos de conversa, FAQ e preferências de tema. Extrato e Metas agora mostram telas próprias na prévia do telefone. A despesa rápida usa uma conta, coerente com a redução de saldo. Nenhum dado da prévia é enviado à API.

O menu mantém Escape, restauração de rolagem e percurso de foco. A rodada WebKit identificou que o Safari pode pular links na navegação por Tab conforme as preferências do navegador; o menu agora percorre explicitamente seus controles nos dois sentidos. Os nomes acessíveis dos botões também correspondem aos textos visíveis.

### Validação reproduzível

```powershell
pnpm lint
pnpm --filter @finflow/web typecheck
pnpm --filter @finflow/web build
pnpm --filter @finflow/web test:e2e
pnpm --filter @finflow/web exec playwright test --config=playwright.landing.config.ts --project=ios-webkit --workers=2
```

- Suíte padrão: 48 testes aprovados entre desktop Chromium e perfil Android/Pixel 7, dos quais 16 são da landing.
- Rodada adicional: 8 testes aprovados com WebKit e perfil iPhone 13.
- Larguras verificadas em ambos os temas: 320, 375, 390, 430, 768, 1024 e 1440px. Sem rolagem horizontal. A tabela do gráfico e os limites também foram abertos a 320px.
- Axe WCAG 2.1 AA: sem violações nos estados auditados, nos dois temas e modos de gráfico. Asserções conferem alvos de 44px, menu por teclado, dados e CSV, simulador e interações preservadas.
- Evidências locais: `apps/web/test-results/screenshots/`, com 14 capturas de largura/tema e duas do painel expandido a 320px. Esses arquivos são ignorados pelo Git.
- Lint, typecheck do frontend, build e `git diff --check` aprovados. Revisão independente sem defeitos materiais pendentes.

As verificações de Android/iOS usam emulação de navegador; não substituem validação em aparelhos físicos. Não houve publicação nem teste de backend, pois o escopo desta mudança é a landing e seus exemplos locais.

O repositório irmão `finflow-landing` já contém alterações próprias e não foi modificado nem sobrescrito por sincronização nesta tarefa. Compare as versões antes de usar `sync-app.mjs`, para preservar esta reformulação. As mudanças locais anteriores do workspace foram mantidas como base.

Semantica não está disponível nesta sessão. O histórico de decisões e evidências foi registrado neste arquivo; não foi persistido em memória externa.

## Correção após revisão visual do usuário

O usuário rejeitou a primeira reformulação: o hero ficou desproporcional, o painel ultrapassava a dobra e o gráfico mostrava controles demais. A aprovação funcional anterior não representava aceitação visual do produto.

Correções implementadas:

- Headline original restaurada: “Seu dinheiro em ordem. Mais espaço para viver.” O subtítulo, os textos de recursos e os títulos de celular, privacidade e fechamento também retomam a linguagem anterior, mantendo o pré-lançamento.
- Título limitado a 52px no desktop e 36px nas menores telas. Duas linhas no desktop e até quatro no mobile.
- Uma superfície para o painel, sem a moldura verde adicional. Saldo, entradas/saídas e curva aparecem diretamente. A altura inicial caiu de aproximadamente 1.093px para 525px.
- As abas, datas interativas, tabela, limites, lançamentos, simulação e download CSV continuam funcionais dentro de “Explorar exemplo”. A expansão cresce naturalmente, sem cortar conteúdo.
- O gráfico inicial tem uma única série, escala em reais, datas discretas e um marcador final. Não repete saldo em uma segunda linha de resultado nem apresenta abas no primeiro contato.
- Os dois cards extras de categoria e plano foram retirados; os cards originais de cartões e orçamento foram preservados.

Foi acrescentado um teste específico da dobra em 1366×768, 1536×864, 1920×900 e 1536×718 (aproximação do espaço útil com zoom de 125%). O teste exige painel fechado entre 480 e 540px, CTA e nota completos no viewport e título de até três linhas no desktop. As medições aguardam o React e as fontes antes da captura.

A prévia foi aberta no navegador integrado do Orca em `http://localhost:5273/inicio`. Na janela real de 1272×706 CSS px, o painel mediu 524px e a nota terminou em 685px, sem rolagem horizontal. A revisão visual independente aceitou a composição desktop escura nessa captura.

Validação da correção: 18/18 testes da landing em Chromium (desktop e Android) e 9/9 no perfil iPhone/WebKit aprovados. Os sete tamanhos solicitados, os dois temas, o conteúdo expandido a 320px e os quatro cenários de dobra foram verificados. Axe sem violações nos estados auditados. Não foi repetida a suíte completa do aplicativo nesta correção, que ficou restrita à landing. Capturas de dobra em `apps/web/test-results/screenshots/fold-*.png`.

## Redeploy em produção — 3 de novembro de 2026

Publicado a pedido do usuário em [finflow-landing-umber.vercel.app](https://finflow-landing-umber.vercel.app/). Deployment `dpl_BvpTfQWHQHeMqbx6PAVoLt8eQc5c`, estado `READY`, ambiente `production`. [Inspeção na Vercel](https://vercel.com/daniels-projects-f8239060/finflow-landing/BvpTfQWHQHeMqbx6PAVoLt8eQc5c).

A fonte publicada é a landing corrigida deste monorepo. O repositório irmão contém uma versão local diferente e foi preservado. O script `apps/web/scripts/build-public-landing.mjs` cria um pacote isolado em `apps/web/test-results/vercel-landing`, copia os nove componentes aprovados e a fonte local, gera hashes de origem em `manifest.json` e compila somente a landing. A entrada pública abre o diálogo de pré-lançamento para acesso à conta, sem apontar ao painel privado. Não inclui roteador do aplicativo, API, arquivos de ambiente ou source maps.

Reprodução a partir da raiz do monorepo:

```powershell
node apps/web/scripts/build-public-landing.mjs
vercel deploy --prebuilt --prod --yes --cwd apps/web/test-results/vercel-landing
node apps/web/scripts/verify-public-landing.mjs https://finflow-landing-umber.vercel.app/
```

O vínculo Vercel existente é lido de `../finflow-landing/.vercel/project.json`. O pacote usa a Build Output API v3, com conteúdo estático, os cabeçalhos existentes e suporte a `/inicio`.

Antes do deploy e no domínio de produção, as verificações passaram em Chromium e WebKit: HTTP 200 sem autenticação, copy original, dois temas, sete larguras, painel na dobra, axe AA, diálogo de acesso, gasto/desfazer, troca de mês, CSV, calculadora e assets. Nenhuma chamada à API. Capturas da versão pública em `apps/web/test-results/public-verification/`. O pacote teve revisão independente dos hashes e do conteúdo a publicar.
