# FinFlow local de ponta a ponta — assistente próprio e rodar a partir do código

Pedido do usuário: o FinFlow é **open source e local** — baixar o executável ou clonar o repositório e rodar no
próprio computador, com **banco próprio** (SQLite local, já existente) e **agente próprio** (assistente que roda num
modelo local, sem depender de serviço pago). A versão no navegador continua como forma de experimentar.

## 1. Provedores do assistente

| Provedor | Uso | Credencial |
|---|---|---|
| `ollama` (**padrão recomendado**) | modelo local via Ollama (`http://localhost:11434`) | nenhuma |
| `openai_compatible` | qualquer servidor local compatível com `/v1/chat/completions` (LM Studio, llama.cpp `llama-server`, vLLM, LocalAI) | chave opcional |
| `claude` | API do Claude (como hoje) | `ANTHROPIC_API_KEY` ou chave salva |

O loop do agente, as ferramentas (mesmas 22 de `internal/ai/tools.go`), as **propostas confirmadas pelo usuário**
para escritas, o SSE (`text`/`tool`/`proposal`/`done`/`error`) e o system prompt são **compartilhados**; só o
cliente do modelo muda (interface `Provider` em `internal/ai`). Ollama: `POST /api/chat` com `tools` e
`stream: true` (chunks NDJSON, `message.tool_calls`). OpenAI-compatible: `POST /v1/chat/completions` com `tools`,
`stream: true` (SSE, `delta.tool_calls` acumulados por `index`). Sem SDK de terceiros: `net/http` + `encoding/json`.
Modelos locais erram mais em tool calling: validar argumentos (já existe `validateToolInput`), devolver o erro como
resultado da ferramenta e limitar a 8 rodadas de ferramenta por pergunta.

## 2. Configuração

Env (desktop/Docker) com precedência sobre o que está salvo em `settings`:

```
FINFLOW_AI_PROVIDER=ollama|openai_compatible|claude
FINFLOW_AI_BASE_URL=http://localhost:11434        # ollama; ou http://localhost:1234/v1 (LM Studio)
FINFLOW_AI_MODEL=qwen2.5:7b                        # modelo de chat com suporte a ferramentas
FINFLOW_AI_VISION_MODEL=qwen2.5vl:7b               # opcional, para ler comprovantes
FINFLOW_AI_API_KEY=                                # openai_compatible opcional
ANTHROPIC_API_KEY=                                 # claude (já existe)
FINFLOW_OLLAMA_URL=http://localhost:11434          # onde procurar um Ollama para a configuração automática
```

Ordem de decisão: `FINFLOW_AI_PROVIDER` → provedor salvo em Configurações → credencial do Claude → **AUTO**
(sem nada definido, um Ollama em `FINFLOW_OLLAMA_URL` com pelo menos um modelo é usado automaticamente:
`FINFLOW_AI_MODEL` se instalado, senão o primeiro com ferramentas). Variáveis vazias contam como não definidas.

API (substitui/estende `/api/ai/status` e `/api/ai/key`, mantendo compatibilidade):

```ts
GET /api/ai/status → { configured: boolean; provider: "ollama" | "openai_compatible" | "claude" | null;
                       model: string | null; visionModel: string | null; baseUrl: string | null;
                       source: "ENV" | "SETTINGS" | "AUTO" | null; reachable: boolean | null;   // ping do servidor local
                       receiptSupported: boolean }
PUT /api/ai/config { provider, baseUrl?, model?, visionModel?, apiKey?: string | null } → status
GET /api/ai/models?provider&baseUrl → { models: { id: string; label: string; tools: boolean | null }[] }
     // ollama: /api/tags (+ /api/show para capacidades quando disponível); openai_compatible: /v1/models
PUT /api/ai/key {apiKey}  // legado: equivale a PUT /api/ai/config {provider:"claude", apiKey}
```

Sem configuração nenhuma, o status sugere Ollama: se `http://localhost:11434` responder, `configured` fica false mas
`reachable: true` e a UI oferece "Usar o Ollama deste computador" com os modelos instalados.

## 3. Interfaces

- **Configurações → Assistente**: escolher provedor, URL, modelo (lista vinda de `/api/ai/models`), modelo de visão,
  testar conexão; instruções curtas (instalar Ollama, `ollama pull qwen2.5:7b`; LM Studio "Start server").
- **Assistente**: cartão de configuração oferece primeiro o modo local; mostra qual provedor/modelo está ativo.
- **CLI**: `finflow perguntar "Posso comprar um celular de R$ 2.000 em 10x?"` usa o mesmo agente e imprime a resposta
  em streaming; propostas de escrita são mostradas e só aplicadas com `--confirmar` (ou pergunta s/N no terminal).
- **Navegador (`/app/`)**: o provedor local funciona se o servidor permitir CORS (Ollama: `OLLAMA_ORIGINS`); a UI
  explica isso quando a chamada falhar por CORS.

## 4. Rodar a partir do código

- `git clone … && cd finflow && pnpm install && pnpm start` → compila o web, embute no binário, compila o Go e abre
  o FinFlow (modo duplo clique). `pnpm start -- --seed` com dados de exemplo. Requisitos verificados com mensagens
  claras: Go ≥ 1.26, Node ≥ 22.12, pnpm (via Corepack).
- **Docker** (sem Go/Node na máquina): `docker compose up -d` sobe o FinFlow em `http://localhost:3333` com o banco
  num volume; `docker compose --profile ai up -d` sobe também um Ollama local e configura o assistente
  (`FINFLOW_AI_PROVIDER=ollama`, `FINFLOW_AI_BASE_URL=http://ollama:11434`); o primeiro uso faz
  `ollama pull` do modelo configurado. `Dockerfile` multi-stage (Node → Go → imagem mínima, usuário não-root,
  `CGO_ENABLED=0`), healthcheck em `/api/health`, porta e dados configuráveis. O Postgres continua opcional.
- Landing e README: **Baixar** (executável) e **Código no GitHub** como ações principais; "Experimentar no navegador"
  como terceira opção; destaque para código aberto, dados num arquivo seu e assistente local.
