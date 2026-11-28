# FinFlow local de ponta a ponta — rodar a partir do código e assistente opcional

O FinFlow é **open source e local**: baixar o executável ou clonar o repositório e rodar no próprio computador,
com **banco próprio** (SQLite local). A versão no navegador continua como forma de experimentar.

> **Nota (v0.6.0): provedores locais removidos.** A v0.5.0 permitia rodar o assistente em modelos locais (Ollama
> e servidores compatíveis com a API da OpenAI, como LM Studio e llama.cpp), com detecção automática de um Ollama,
> variáveis `FINFLOW_AI_*` / `FINFLOW_OLLAMA_URL`, `PUT /api/ai/config`, `GET /api/ai/models` e o profile `ai` do
> Docker. Por decisão de produto, tudo isso foi removido: o assistente usa **só a API do Claude com a chave do
> próprio usuário**; sem chave, o recurso fica desativado. O histórico está no [CHANGELOG](../CHANGELOG.md).

## 1. Assistente: só a API do Claude

- O assistente é opcional. Ele usa a API do Claude (modelo `claude-opus-5-5`) com a chave do próprio usuário:
  `ANTHROPIC_API_KEY` (ou a cadeia padrão de credenciais do SDK: `ANTHROPIC_AUTH_TOKEN`, perfil do
  `ant auth login`) ou uma chave salva em Configurações (`settings.anthropicApiKey`, nunca devolvida pela API).
  O ambiente tem precedência sobre a chave salva.
- As perguntas e os dados consultados para respondê-las vão para a API da Anthropic; o uso é cobrado pela
  Anthropic na conta do usuário. Todo o resto do FinFlow funciona sem chave.
- Escritas nunca acontecem pelo modelo: as ferramentas de escrita só geram propostas que o usuário confirma.

## 2. API

```ts
GET /api/ai/status → { configured: boolean; source: "ENV" | "SETTINGS" | null; model: string | null;
                       receiptSupported: boolean }   // receiptSupported = configured; a chave nunca volta
PUT /api/ai/key { apiKey: string | null }           // null ou "" remove a chave salva → status
POST /api/ai/chat (SSE: text/tool/proposal/done/error)
POST /api/ai/receipt (multipart, campo file)
```

Sem chave, `/api/ai/chat` e `/api/ai/receipt` respondem **412** com a mensagem em pt-BR dizendo como ativar.
`/api/system/info` continua com `aiConfigured`.

## 3. Interfaces

- **Sem chave o recurso fica desativado e não é anunciado**: o item "Assistente" some da barra lateral, da barra
  inferior/"Mais" e da paleta de comandos; não há anexo de comprovante nem atalhos para o assistente. Quem abrir
  `/assistente` vê um aviso curto apontando para Configurações.
- **Configurações → Assistente (IA)**: o único lugar para ativar. Explicação curta, campo da chave (senha) +
  Salvar, "Remover chave", status ("Ativo" / "Desativado"), aviso quando a chave vem do ambiente (não dá para
  remover pela interface) e link para criar a chave em console.anthropic.com.
- **CLI**: `finflow perguntar "…"` usa o mesmo agente; sem chave, avisa em pt-BR que o assistente está desativado
  e como ativar (código de saída diferente de 0). Propostas só são gravadas com `--confirmar` ou s/N.
- O "Posso comprar?" (Projeção e Desejos) e as calculadoras não usam IA e funcionam sempre.

## 4. Rodar a partir do código

- `git clone … && cd finflow && pnpm install && pnpm start` → compila o web, embute no binário, compila o Go e abre
  o FinFlow (modo duplo clique). `pnpm start -- --seed` com dados de exemplo. Requisitos verificados com mensagens
  claras: Go ≥ 1.26, Node ≥ 22.12, pnpm (via Corepack).
- **Docker** (sem Go/Node na máquina): `docker compose up -d` sobe o FinFlow em `http://localhost:3333` com o banco
  num volume; para o assistente, passe `ANTHROPIC_API_KEY` no ambiente (ou salve a chave em Configurações).
  `Dockerfile` multi-stage (Node → Go → imagem mínima, usuário não-root, `CGO_ENABLED=0`), healthcheck em
  `/api/health`, porta e dados configuráveis. O Postgres continua opcional.
- Landing e README: **Baixar** (executável) e **Código no GitHub** como ações principais; "Experimentar no navegador"
  como terceira opção; destaque para código aberto e dados num arquivo seu.
