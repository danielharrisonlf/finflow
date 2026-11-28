import { useQueryClient } from "@tanstack/react-query";
import { Cpu, ExternalLink, KeyRound, RefreshCw } from "lucide-react";
import { type FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import { AI_DEFAULTS, type AiStatus, pickChatModel, saveAiConfig, saveAiKey } from "../../lib/ai";
import { useAiModels } from "../../lib/queries";
import { isBrowserRuntime } from "../../runtime/client";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Field, Input } from "../ui/field";

function Code({ children }: { children: React.ReactNode }) {
  return <code className="rounded bg-app-soft px-1 text-xs text-app-text">{children}</code>;
}

/**
 * Configuração do Assistente: o modo local (Ollama, sem serviço pago) vem primeiro; a API do Claude
 * continua como alternativa. Chaves vão para o servidor local e nunca voltam para o navegador.
 */
export function AssistantSetupCard({
  status,
  onRecheck,
  rechecking,
}: {
  status?: AiStatus;
  onRecheck?: () => void;
  rechecking?: boolean;
}) {
  const queryClient = useQueryClient();
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  const [saving, setSaving] = useState<false | "local" | "claude">(false);

  // Ollama encontrado em localhost:11434 (nada configurado) ou Ollama configurado sem resposta.
  const ollamaFound =
    status?.reachable === true && (!status.provider || status.provider === "ollama");
  const installed = useAiModels("ollama", "", ollamaFound);
  const pendingCompatible = status?.provider === "openai_compatible" && !status.configured;

  const applyStatus = (next: AiStatus) => {
    queryClient.setQueryData(["ai", "status"], next);
    void queryClient.invalidateQueries({ queryKey: ["system", "info"] });
  };

  const chooseLocalOllama = async () => {
    setSaving("local");
    setLocalError(null);
    try {
      const model = pickChatModel(installed.data ?? []);
      applyStatus(await saveAiConfig({ provider: "ollama", model }));
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : "Não foi possível usar o Ollama");
    } finally {
      setSaving(false);
    }
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = apiKey.trim();
    if (!trimmed) {
      setError("Cole a chave da API");
      return;
    }
    setSaving("claude");
    setError(null);
    try {
      const next = await saveAiKey(trimmed);
      setApiKey("");
      applyStatus(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar a chave");
    } finally {
      setSaving(false);
    }
  };

  const models = installed.data ?? [];

  return (
    <Card className="p-5" data-testid="ai-setup">
      <div className="flex items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-income">
          <Cpu size={20} aria-hidden />
        </span>
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-app-text">Ative o Assistente</h2>
          <p className="mt-1 text-sm text-app-muted">
            Rode o assistente num modelo no seu próprio computador, sem serviço pago: as perguntas e
            os seus dados não saem daqui.
          </p>
        </div>
      </div>

      <section
        className="mt-4 rounded-(--radius-control) border border-emerald-500/30 bg-emerald-500/5 p-3.5"
        data-testid="ai-setup-local"
        aria-labelledby="ai-setup-local-title"
      >
        <h3 id="ai-setup-local-title" className="text-sm font-semibold text-app-text">
          Modelo local (recomendado)
        </h3>
        {pendingCompatible ? (
          <p className="mt-1 text-sm text-app-muted">
            O servidor compatível está escolhido, mas falta o modelo. Complete em{" "}
            <Link
              to="/configuracoes"
              className="font-medium text-income underline-offset-4 hover:underline"
            >
              Configurações → Assistente
            </Link>
            .
          </p>
        ) : ollamaFound ? (
          <>
            <p className="mt-1 text-sm text-app-muted">
              Encontramos o Ollama rodando neste computador.
              {models.length > 0
                ? ` Modelos instalados: ${models.map((m) => m.id).join(", ")}.`
                : installed.isLoading
                  ? ""
                  : ` Nenhum modelo instalado ainda — rode `}
              {!installed.isLoading && models.length === 0 ? (
                <>
                  <Code>ollama pull {AI_DEFAULTS.model}</Code>.
                </>
              ) : null}
            </p>
            <Button
              className="mt-3 w-full sm:w-auto"
              onClick={() => void chooseLocalOllama()}
              disabled={saving !== false}
            >
              {saving === "local" ? "Configurando…" : "Usar o Ollama deste computador"}
            </Button>
            {localError ? (
              <p role="alert" className="mt-2 text-sm text-expense">
                {localError}
              </p>
            ) : null}
          </>
        ) : (
          <>
            <ol className="mt-1.5 list-decimal space-y-1 pl-5 text-sm text-app-muted">
              <li>
                Instale o Ollama em{" "}
                <a
                  href="https://ollama.com/download"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-income underline-offset-4 hover:underline"
                >
                  ollama.com
                </a>
                .
              </li>
              <li>
                No terminal: <Code>ollama pull {AI_DEFAULTS.model}</Code>
              </li>
              <li>Volte aqui e clique em “Verificar de novo”.</li>
            </ol>
            {isBrowserRuntime ? (
              <p className="mt-2 text-xs text-app-muted">
                No navegador, o Ollama precisa liberar esta página: defina{" "}
                <Code>
                  OLLAMA_ORIGINS={typeof window === "undefined" ? "*" : window.location.origin}
                </Code>{" "}
                e reinicie-o.
              </p>
            ) : null}
            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center">
              {onRecheck ? (
                <Button variant="outline" onClick={onRecheck} disabled={rechecking}>
                  <RefreshCw
                    size={15}
                    aria-hidden
                    className={rechecking ? "animate-spin motion-reduce:animate-none" : ""}
                  />
                  Verificar de novo
                </Button>
              ) : null}
            </div>
          </>
        )}
        <p className="mt-3 text-xs text-app-muted">
          LM Studio, llama.cpp ou outra URL? Configure em{" "}
          <Link
            to="/configuracoes"
            className="font-medium text-income underline-offset-4 hover:underline"
          >
            Configurações → Assistente
          </Link>
          .
        </p>
      </section>

      <section className="mt-5" aria-labelledby="ai-setup-claude-title">
        <h3
          id="ai-setup-claude-title"
          className="flex items-center gap-1.5 text-sm font-semibold text-app-text"
        >
          <KeyRound size={15} aria-hidden className="text-app-muted" /> Ou use a API do Claude
        </h3>
        <p className="mt-1 text-sm text-app-muted">
          Com a sua própria chave. Ela fica salva só neste computador, no banco do FinFlow, e nunca
          é mostrada de volta na tela.
        </p>

        <form onSubmit={submit} className="mt-3 flex flex-col gap-3">
          <Field label="Chave da API do Claude" error={error ?? undefined}>
            <Input
              type="password"
              autoComplete="off"
              spellCheck={false}
              placeholder="sk-ant-…"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
          </Field>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Button
              type="submit"
              variant="outline"
              disabled={saving !== false}
              className="sm:w-auto"
            >
              {saving === "claude" ? "Salvando…" : "Salvar chave"}
            </Button>
            <a
              href="https://console.anthropic.com/"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 items-center gap-1.5 px-1 text-sm font-medium text-income underline-offset-4 hover:underline"
            >
              Criar uma chave no console.anthropic.com <ExternalLink size={14} aria-hidden />
            </a>
          </div>
        </form>
      </section>

      <details className="mt-4 rounded-(--radius-control) border border-app-border/80 bg-app-background/60 px-3.5 py-2.5 text-sm text-app-muted">
        <summary className="flex min-h-11 cursor-pointer items-center text-app-text">
          Prefere variável de ambiente?
        </summary>
        <p className="mt-1">Defina antes de iniciar o FinFlow e reinicie o servidor:</p>
        <pre className="mt-2 overflow-x-auto rounded bg-app-soft p-2.5 text-xs text-app-text">
          {`# modelo local
export FINFLOW_AI_PROVIDER=ollama
export FINFLOW_AI_MODEL=${AI_DEFAULTS.model}
# ou a API do Claude
export ANTHROPIC_API_KEY=sk-ant-...
finflow serve`}
        </pre>
        <p className="mt-2 text-xs">
          No Windows (PowerShell): <code>$env:FINFLOW_AI_PROVIDER="ollama"</code>
        </p>
      </details>
    </Card>
  );
}
