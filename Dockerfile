# FinFlow em contêiner (docs/local-agent-contract.md §4): sem Go nem Node na máquina.
#
#   docker build -t finflow .
#   docker compose up -d                   # http://localhost:3333, banco no volume finflow_data
#   ANTHROPIC_API_KEY=sk-ant-... docker compose up -d   # com o assistente (opcional, API do Claude)
#
# Estágios: web (Node + pnpm via Corepack) → binário Go com o web embutido (CGO_ENABLED=0, SQLite
# pure Go) → imagem mínima (Alpine, usuário não-root). O Alpine traz o `wget` do BusyBox, usado no
# healthcheck em /api/health.

# ───────────────────────── 1. web ─────────────────────────
FROM node:22.23.0-bookworm-slim AS web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    CI=true
WORKDIR /src
RUN corepack enable
# Só os manifestos primeiro, para o cache das dependências sobreviver a mudanças no código.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/contracts/package.json packages/contracts/
RUN pnpm install --frozen-lockfile --filter "@finflow/web..."
COPY packages/contracts packages/contracts
COPY apps/web apps/web
RUN pnpm --filter @finflow/contracts build && pnpm --filter @finflow/web build

# ───────────────────────── 2. binário ─────────────────────────
FROM golang:1.26.3-alpine3.23 AS go
ARG FINFLOW_VERSION=docker
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
# O web compilado vai para onde o go:embed espera (o mesmo que scripts/sync-webui.mjs faz).
COPY --from=web /src/apps/web/dist internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${FINFLOW_VERSION}" \
      -o /out/finflow ./cmd/finflow

# ───────────────────────── 3. runtime ─────────────────────────
FROM alpine:3.23.4
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 10001 finflow \
 && adduser -S -D -H -u 10001 -G finflow -h /data finflow \
 && mkdir -p /data \
 && chown finflow:finflow /data
COPY --from=go /out/finflow /usr/local/bin/finflow

# API_HOST=0.0.0.0 só dentro do contêiner; quem decide o que fica exposto é o `ports:` do compose
# (por padrão 127.0.0.1:3333, só esta máquina).
ENV FINFLOW_DB_PATH=/data/finflow.db \
    API_HOST=0.0.0.0 \
    API_PORT=3333 \
    HOME=/data
VOLUME ["/data"]
EXPOSE 3333
USER finflow:finflow
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${API_PORT}/api/health" || exit 1
ENTRYPOINT ["finflow"]
CMD ["serve"]
