import { Bot, Check, Copy, Plug, Smartphone } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useAiStatus, useSystemInfo } from "../lib/queries";
import { Card } from "./ui/card";
import { LoadingRows } from "./ui/empty";
import { QrCode } from "./ui/qr-code";
import { Badge } from "./ui/segmented";

const MCP_STDIO_SNIPPET = JSON.stringify(
  { mcpServers: { finflow: { command: "/caminho/para/finflow", args: ["mcp"] } } },
  null,
  2,
);

/** Botão de copiar com retorno "Copiado" por alguns segundos. */
function CopyButton({ text, label }: { text: string; label: string }) {
  const [state, setState] = useState<"idle" | "done" | "error">("idle");
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);

  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(text);
      setState("done");
    } catch {
      setState("error");
    }
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setState("idle"), 2000);
  };

  return (
    <button
      type="button"
      onClick={() => void copy()}
      aria-label={label}
      className="inline-flex h-9 shrink-0 cursor-pointer items-center gap-1.5 rounded-(--radius-control) border border-app-border px-2.5 text-xs text-app-muted transition-colors hover:border-app-muted hover:text-app-text"
    >
      {state === "done" ? (
        <Check size={13} aria-hidden className="text-income" />
      ) : (
        <Copy size={13} aria-hidden />
      )}
      <span aria-live="polite">
        {state === "done" ? "Copiado" : state === "error" ? "Não foi possível copiar" : "Copiar"}
      </span>
    </button>
  );
}

function Snippet({ code, copyLabel }: { code: string; copyLabel: string }) {
  return (
    <div className="relative min-w-0">
      <pre className="overflow-x-auto rounded-(--radius-control) border border-app-border bg-app-background/70 p-3 pr-24 text-xs leading-relaxed text-app-text">
        <code>{code}</code>
      </pre>
      <div className="absolute top-2 right-2">
        <CopyButton text={code} label={copyLabel} />
      </div>
    </div>
  );
}

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 sm:flex-row sm:items-baseline sm:gap-3">
      <dt className="shrink-0 text-xs text-app-muted sm:w-32">{label}</dt>
      <dd className="min-w-0 text-sm break-all text-app-text">{children}</dd>
    </div>
  );
}

function SubHeading({ icon: Icon, children }: { icon: typeof Plug; children: React.ReactNode }) {
  return (
    <h3 className="flex items-center gap-1.5 text-sm font-medium text-app-text">
      <Icon size={15} aria-hidden className="text-app-muted" /> {children}
    </h3>
  );
}

/** Configurações → "API e IA" (v3 §13–15): dados do sistema, MCP, rede local e assistente. */
export function ApiAiSection() {
  const system = useSystemInfo();
  const ai = useAiStatus();
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const apiBase = `${origin}/api`;
  const info = system.data;
  const mcpUrl = `${origin}${info?.mcpHttpPath ?? "/api/mcp"}`;
  const httpSnippet = [
    `POST ${mcpUrl}`,
    "Content-Type: application/json",
    ...(info?.authRequired === false ? [] : ["Authorization: Bearer <seu token>"]),
    "",
    '{"jsonrpc":"2.0","id":1,"method":"tools/list"}',
  ].join("\n");
  const lanUrl = info?.lanUrls[0];

  return (
    <Card
      className="grid gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8"
      data-testid="api-ai-section"
    >
      <div>
        <h2 className="text-sm font-semibold text-app-text">API e IA</h2>
        <p className="mt-1 text-sm text-app-muted">
          Use seus dados em assistentes compatíveis com MCP, acesse a API pessoal e abra o FinFlow
          no celular pela rede local.
        </p>
      </div>

      <div className="flex min-w-0 flex-col gap-5">
        {system.isLoading ? (
          <LoadingRows rows={3} />
        ) : !info ? (
          <p className="rounded-(--radius-control) border border-app-border bg-app-background/60 px-3.5 py-2.5 text-sm text-app-muted">
            Informações do sistema indisponíveis no momento. Atualize o FinFlow ou tente de novo
            mais tarde.
          </p>
        ) : (
          <dl className="flex flex-col gap-2" data-testid="system-info">
            <InfoRow label="Versão">
              <span className="tnum">{info.version}</span>
            </InfoRow>
            <InfoRow label="Banco de dados">
              {info.dbDialect === "postgres" ? "PostgreSQL" : "SQLite"}
            </InfoRow>
            {info.dataPath ? (
              <InfoRow label="Arquivo de dados">
                <span className="flex flex-wrap items-center gap-2">
                  <code className="text-xs">{info.dataPath}</code>
                  <CopyButton text={info.dataPath} label="Copiar caminho do arquivo de dados" />
                </span>
              </InfoRow>
            ) : null}
            <InfoRow label="URL da API">
              <code className="text-xs">{apiBase}</code>
            </InfoRow>
          </dl>
        )}

        <section className="flex min-w-0 flex-col gap-2">
          <SubHeading icon={Plug}>MCP no computador (stdio)</SubHeading>
          <p className="text-xs text-app-muted">
            Adicione ao arquivo de configuração do seu assistente compatível com MCP, trocando o
            caminho pelo local do executável do FinFlow.
          </p>
          <Snippet code={MCP_STDIO_SNIPPET} copyLabel="Copiar configuração MCP stdio" />
        </section>

        <section className="flex min-w-0 flex-col gap-2">
          <SubHeading icon={Plug}>MCP por HTTP</SubHeading>
          <p className="text-xs text-app-muted">
            Endpoint JSON-RPC com as mesmas ferramentas.{" "}
            {info?.authRequired === false
              ? "Sem senha configurada, não é preciso enviar cabeçalho de autorização."
              : "Com senha ativada, envie o token obtido no login no cabeçalho Authorization."}
          </p>
          <Snippet code={httpSnippet} copyLabel="Copiar exemplo de MCP por HTTP" />
        </section>

        <section className="flex min-w-0 flex-col gap-2" data-testid="lan-status">
          <SubHeading icon={Smartphone}>Celular na rede local</SubHeading>
          {info?.lanEnabled && lanUrl ? (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <QrCode value={lanUrl} label={`QR code para abrir ${lanUrl}`} />
              <div className="min-w-0">
                <p className="text-sm text-app-text">
                  Aponte a câmera do celular para o código ou abra:
                </p>
                <ul className="mt-1 space-y-0.5">
                  {info.lanUrls.map((url) => (
                    <li key={url}>
                      <code className="text-sm break-all text-income">{url}</code>
                    </li>
                  ))}
                </ul>
                <p className="mt-2 text-xs text-app-muted">
                  O celular precisa estar no mesmo Wi-Fi. No navegador, use “Adicionar à tela
                  inicial” para instalar como app.
                </p>
              </div>
            </div>
          ) : (
            <p className="text-sm text-app-muted">
              Modo rede local desligado. Para abrir no celular, inicie com{" "}
              <code className="text-xs text-app-text">finflow serve --lan</code> (ou{" "}
              <code className="text-xs text-app-text">FINFLOW_LAN=1</code>). Por segurança, é
              preciso definir uma senha (<code className="text-xs">FINFLOW_PASSWORD</code>).
            </p>
          )}
        </section>

        <section className="flex min-w-0 flex-col gap-2" data-testid="ai-status">
          <SubHeading icon={Bot}>Assistente</SubHeading>
          {ai.isLoading ? (
            <p className="text-sm text-app-muted">Verificando…</p>
          ) : !ai.data ? (
            <p className="text-sm text-app-muted">Status do assistente indisponível.</p>
          ) : (
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={ai.data.configured ? "income" : "warn"}>
                {ai.data.configured ? "Chave configurada" : "Chave não configurada"}
              </Badge>
              <span className="text-sm text-app-muted">
                {ai.data.configured
                  ? ai.data.source === "ENV"
                    ? "via variável de ambiente"
                    : "salva nas configurações"
                  : "o assistente precisa de uma chave de API"}
                {ai.data.model ? ` · modelo ${ai.data.model}` : ""}
              </span>
            </div>
          )}
          <Link
            to="/assistente"
            className="inline-flex min-h-11 items-center self-start text-sm font-medium text-income underline-offset-4 hover:underline"
          >
            Abrir o Assistente
          </Link>
        </section>
      </div>
    </Card>
  );
}
