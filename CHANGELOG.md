# Changelog

Todas as mudanças relevantes do FinFlow. O formato segue o [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e as versões seguem o [SemVer](https://semver.org/lang/pt-BR/). Os pacotes de cada versão ficam em [Releases](https://github.com/danielharrisonlf/finflow/releases).

## [Não lançado]

## [0.5.0] — 2026-11-28

Open source e local de ponta a ponta: banco próprio (SQLite) e agente próprio (modelo local), rodando pelo executável, pelo código-fonte ou pelo Docker ([docs/local-agent-contract.md](docs/local-agent-contract.md)).

### Adicionado

- **Assistente local**: o "Pergunte ao FinFlow" roda num modelo no seu computador via **Ollama** ou qualquer servidor compatível com a API da OpenAI (**LM Studio**, llama.cpp, vLLM, LocalAI), sem chave e sem enviar nada para fora; o Claude continua como opção. Mesmas ferramentas, propostas confirmadas pelo usuário e leitura de comprovante com um modelo de visão. Sem configuração, um Ollama em `FINFLOW_OLLAMA_URL` (padrão `http://localhost:11434`) é detectado e usado automaticamente. Configurações → **Assistente** escolhe provedor, URL e modelo (lista os modelos instalados) e testa a conexão; `GET /api/ai/models`, `PUT /api/ai/config` e variáveis `FINFLOW_AI_*`.
- **`finflow perguntar "…"`**: o assistente no terminal, local ou via `--api`; escritas só com `--confirmar` ou confirmação `s`.
- **`pnpm start`** para rodar a partir do código: `git clone … && cd finflow && pnpm install && pnpm start` confere Go 1.26+, Node 22.12+ e pnpm (Corepack) com mensagens e links de instalação, compila o web e o binário só quando os fontes mudaram e abre o FinFlow como no duplo clique. `pnpm start -- --seed` cria dados de exemplo; outras flags vão para `finflow serve` e comandos viram `finflow <comando>`.
- **Docker**: `Dockerfile` multi-stage (Node → Go com `CGO_ENABLED=0` → Alpine, usuário não-root, ~50 MB) e serviço `app` no `docker-compose.yaml` (`docker compose up -d`), com o banco no volume `finflow_data`, porta publicada só em `127.0.0.1:3333` e healthcheck em `/api/health`. `docker compose --profile ai up -d` sobe também um Ollama local, baixa o modelo (`FINFLOW_AI_MODEL`, padrão `qwen2.5:7b`) e o assistente detecta esse Ollama sozinho, sem travar a escolha em Configurações. O CI constrói a imagem e faz um smoke test (sem push).
- [`CONTRIBUTING.md`](CONTRIBUTING.md) com como rodar, testar e as convenções do projeto.

### Alterado

- Landing: **Baixar grátis** volta a ser o botão principal, **Código no GitHub** o secundário e **Experimentar no navegador** um link de texto; o topo destaca código aberto (MIT), dados num arquivo SQLite seu e assistente local (Ollama) ou Claude. A seção de download ganha as abas **Executável**, **Código-fonte** e **Docker**, e o FAQ apresenta primeiro o assistente local.
- README reorganizado em **Rodar no seu computador** (executável, código-fonte com `pnpm start` e Docker) e **Experimentar no navegador**.

## [0.4.0] — 2026-11-28

### Adicionado

- **FinFlow no navegador** em `/app/` do site público: o app completo sem instalar nada, no computador ou no celular, instalável como app (PWA) e com os dados guardados no próprio navegador ([docs/browser-runtime-contract.md](docs/browser-runtime-contract.md)). Tela de abertura com **Começar do zero**, **Explorar com dados de exemplo** e **Restaurar um backup (.db)**; aviso quando o FinFlow já está aberto em outra aba (**Usar aqui**); pedido de armazenamento persistente depois dos primeiros dados.
- **Configurações → Dados neste navegador**: status do armazenamento, backup `.db` por espaço ou `.zip` com todos, restauração, **Trazer dados do app instalado** (o `finflow.db` do computador) e **Apagar todos os dados**.
- Funciona offline depois do primeiro carregamento, com aviso **Nova versão disponível — Recarregar** quando sai uma versão nova.
- No programa instalado, Configurações aponta para o FinFlow no navegador.
- `pnpm build:browser`, `pnpm test:e2e:browser` e `apps/web/scripts/serve-public-package.mjs` para gerar, testar e servir localmente o pacote público com o app.

### Corrigido

- **Duplo clique no executável**: se a porta 3333 estiver ocupada por outro programa, o FinFlow usa a próxima livre e mostra o endereço; identifica um FinFlow já aberto por `/api/system/info` (e não por qualquer servidor com `/api/health`); grava `finflow-desktop.log` na pasta de configuração e mostra uma caixa de erro no Windows quando não consegue iniciar.

### Alterado

- Landing: o botão principal passa a ser **Usar no navegador**; **Baixar para o computador** fica como segunda opção, com novas perguntas no FAQ sobre onde ficam os dados no navegador e como levá-los para o app instalado.
- O pacote público (Vercel) inclui o app em `/app/` com fallback de SPA, `application/wasm`, cache imutável para arquivos com hash e `no-cache` para `index.html` e `sw.js`.

## [0.3.0] — 2026-11-27

Recursos de planejamento da v3, descritos em [docs/product-v3-contract.md](docs/product-v3-contract.md) e [docs/product-v3-spaces-contract.md](docs/product-v3-spaces-contract.md).

### Adicionado

- **Quanto posso gastar hoje** no painel, com gráfico do ritmo do mês (gasto acumulado contra o mês anterior) e gasto projetado até o fim do mês.
- **Posso comprar?**: simulador de compra à vista ou parcelada no cartão sobre a projeção de 365 dias, com veredito (cabe, com cuidado, melhor esperar), impacto no limite e data em que caberia.
- **Assinaturas detectadas** no histórico, com aviso de aumento de preço e opção de acompanhar como compromisso ou ignorar.
- **Alertas e dicas** num sino com contador: orçamento, ritmo, saldo negativo projetado, vencimentos, assinaturas, reserva baixa, meta atrasada, pendências antigas e lançamentos sem categoria.
- **Tags** em lançamentos (filtro, gasto por tag e regras de automação que aplicam tag).
- **Fluxo de caixa** mês a mês em regime de caixa ou de competência, com “para onde foi o dinheiro”.
- **Composição da fatura** (parcelas, assinaturas e compras avulsas) e **transferências inteligentes** entre contas próprias.
- **Prévia do link** em Desejos (título, preço e imagem do produto).
- **Investimentos**: posições, aportes, resgates, rentabilidade, alocação por tipo e instituição e histórico de 12 meses.
- **Dívidas e financiamentos**: parcelas Price, saldo devedor, juros restantes e estratégia de quitação avalanche, bola de neve ou só as parcelas; parcelas futuras entram na projeção e nos compromissos.
- **Evolução do patrimônio** mês a mês (contas, investimentos e dívidas).
- **11 calculadoras** em `/calculadoras`: alugar ou comprar, carro ou app de transporte, juros compostos, pequenos gastos, 13º salário, reserva de emergência, independência financeira, quitar dívidas, Price × SAC, à vista ou parcelado, CDB × poupança.
- **Modelos de meta** (casa própria, filho, período sabático, viagem, veículo, casamento, reserva), **paleta de comandos** (Ctrl+K / ⌘K) e **modo privacidade** que oculta valores.
- **No celular pela rede de casa**: `finflow serve --lan` (ou `FINFLOW_LAN=1`, exige `FINFLOW_PASSWORD`), QR code nas configurações e app instalável (PWA) que guarda só a interface.
- **Assistente “Pergunte ao FinFlow”** com a chave do próprio usuário para a API do Claude (`ANTHROPIC_API_KEY` ou Configurações): respostas sobre a projeção, leitura de foto ou PDF de comprovante, e escritas sempre como proposta confirmada pelo usuário.
- **MCP via HTTP** em `POST /api/mcp` (Bearer), dez novas tools MCP, `GET /api/system/info` e a página **API e IA** em Configurações.
- **Espaços**: Pessoal, Casa e Empresa, cada um num banco SQLite separado, com seletor no topo, visão unificada e categorias padrão por tipo. No espaço **Casa**, membros, "quem pagou" e divisão de despesas (igual ou por %), saldos de quem deve a quem e acertos. No espaço **Empresa**, MRR, ARR, churn, queima de caixa, runway e DRE anual, com grupo do DRE editável nas categorias. Com PostgreSQL, só o espaço Pessoal.
- CLI: `assinaturas`, `alertas`, `posso`, `investimentos`, `dividas`, `patrimonio`, `espacos`, `divisao`, `empresa` e a flag global `--espaco` (ou `FINFLOW_SPACE`); `seed --espaco casa|empresa` cria espaços de demonstração.
- Landing: seção “Pergunte ao FinFlow” com conversa de exemplo local, seção de patrimônio com estratégia de quitação e as 11 calculadoras, card “quanto dá para gastar hoje”, recursos atualizados, acesso pelo celular e novas perguntas frequentes.

### Alterado

- Sem `--lan`, `finflow serve` passa a escutar só em `127.0.0.1`.

## [0.2.0] — 2026-11-27

Primeira versão distribuída como binário único, gratuito e de código aberto (licença MIT).

### Adicionado

- **Binário único em Go** (`cmd/finflow`) com API REST, interface web embutida, CLI e servidor MCP (stdio). SQLite embutido por padrão (pure Go, sem CGO nem Docker); PostgreSQL opcional via `DATABASE_URL`. Dois cliques iniciam o servidor e abrem o navegador; `finflow version`.
- **Planejamento**: tipos de gasto Fixo / Cortável / Variável com taxonomia brasileira padrão, compromissos (contas e receitas recorrentes que viram lançamentos), limite mensal, faixas de saldo, reserva de emergência (6 ou 12 meses), desejos com a data em que cabem no saldo e regras de automação aplicadas a lançamentos, cartões, Telegram e importações.
- **Insights**: ritmo do mês, limite mensal, performance (entradas − fixo − diário), taxa de economia, patrimônio, cobertura da reserva, onde cortar, projeção diária e de 12 meses sobre o saldo líquido, visão do ano e compromissos.
- **Planilhas**: pasta `.xlsx` anual no formato da planilha diária com fórmulas vivas, modelo de importação, extrato em `.xlsx`, e importação com prévia e detecção de duplicados para planilha diária, Excel/CSV e OFX.
- **Interface web**: cards de insight no painel, Compromissos, Projeção com saldo diário colorido e plano de ação, Ano, Planilhas (assistente de importação e exportações), reserva e desejos em Metas, tipos de gasto e automações em Categorias, Configurações e navegação agrupada.
- CLI: `projecao`, `fixas`, `insights`, `importar`, `exportar --formato xlsx`. MCP com 24 tools.
- **Distribuição**: `scripts/release.mjs` gera pacotes para 6 plataformas com `checksums.txt`, workflow de release por tag e instaladores `install.sh` / `install.ps1` com conferência de SHA-256.
- **Landing**: download gratuito com detecção do sistema, comandos de instalação e seção de planilhas no lugar do diálogo de pré-lançamento.

### Alterado

- API NestJS, CLI Node e servidor MCP em Node substituídos pelo binário Go.

[Não lançado]: https://github.com/danielharrisonlf/finflow/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/danielharrisonlf/finflow/releases/tag/v0.5.0
[0.4.0]: https://github.com/danielharrisonlf/finflow/releases/tag/v0.4.0
[0.3.0]: https://github.com/danielharrisonlf/finflow/releases/tag/v0.3.0
[0.2.0]: https://github.com/danielharrisonlf/finflow/releases/tag/v0.2.0
