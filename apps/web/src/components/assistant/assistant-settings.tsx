import { useQueryClient } from "@tanstack/react-query";
import { Check, ExternalLink } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import { type AiStatus, ANTHROPIC_CONSOLE_URL, saveAiKey } from "../../lib/ai";
import { ApiError } from "../../lib/api";
import { useAiStatus } from "../../lib/queries";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Field, Input } from "../ui/field";
import { Badge } from "../ui/segmented";

/** Âncora da seção: o estado desativado do Assistente aponta para /configuracoes#assistente. */
export const ASSISTANT_SETTINGS_ANCHOR = "assistente";

/**
 * Configurações → Assistente (IA): o único lugar para ativar o Assistente. Ele usa a API do Claude
 * com a chave do próprio usuário; sem chave o recurso fica desativado e some da navegação. A chave vai
 * para o servidor local e nunca volta para o navegador.
 */
export function AssistantSettingsSection() {
  const status = useAiStatus();
  const queryClient = useQueryClient();
  const { hash } = useLocation();
  const [apiKey, setApiKey] = useState("");
  const [saving, setSaving] = useState<false | "save" | "remove">(false);
  const [message, setMessage] = useState<{ tone: "ok" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (hash === `#${ASSISTANT_SETTINGS_ANCHOR}` && !status.isLoading) {
      document
        .getElementById(ASSISTANT_SETTINGS_ANCHOR)
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [hash, status.isLoading]);

  const configured = status.data?.configured === true;
  const fromEnv = configured && status.data?.source === "ENV";
  const savedKey = configured && status.data?.source === "SETTINGS";

  const apply = (next: AiStatus) => {
    queryClient.setQueryData(["ai", "status"], next);
    void queryClient.invalidateQueries({ queryKey: ["system", "info"] });
  };

  const fail = (e: unknown, fallback: string) => {
    const issues = e instanceof ApiError && e.issues?.length ? e.issues.map((i) => i.message) : [];
    setMessage({
      tone: "error",
      text: issues.length > 0 ? issues.join("; ") : e instanceof Error ? e.message : fallback,
    });
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = apiKey.trim();
    if (!trimmed) {
      setMessage({ tone: "error", text: "Cole a chave da API do Claude" });
      return;
    }
    setSaving("save");
    setMessage(null);
    try {
      const next = await saveAiKey(trimmed);
      setApiKey("");
      apply(next);
      setMessage({ tone: "ok", text: "Chave salva. O Assistente está ativo." });
    } catch (e) {
      fail(e, "Não foi possível salvar a chave");
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    setSaving("remove");
    setMessage(null);
    try {
      const next = await saveAiKey(null);
      apply(next);
      setMessage({
        tone: "ok",
        text: next.configured ? "Chave removida." : "Chave removida. O Assistente foi desativado.",
      });
    } catch (e) {
      fail(e, "Não foi possível remover a chave");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      id={ASSISTANT_SETTINGS_ANCHOR}
      className="grid scroll-mt-6 gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8"
      data-testid="ai-settings"
    >
      <div>
        <h2 className="text-sm font-semibold text-app-text">Assistente (IA)</h2>
        <p className="mt-1 text-sm text-app-muted">
          Opcional. O Assistente usa a API do Claude com a sua chave: as perguntas e os dados
          consultados para respondê-las vão para a API da Anthropic, e o uso é cobrado pela
          Anthropic na sua conta. Todo o resto do FinFlow funciona sem ele.
        </p>
      </div>

      <div className="flex min-w-0 flex-col gap-4">
        <p className="flex flex-wrap items-center gap-2 text-sm" data-testid="ai-settings-status">
          <span className="text-app-muted">Status:</span>
          {status.isLoading ? (
            <span className="text-app-muted">verificando…</span>
          ) : (
            <Badge tone={configured ? "income" : "neutral"}>
              {configured ? "Ativo" : "Desativado"}
            </Badge>
          )}
          {fromEnv ? (
            <span className="text-app-muted">chave da variável de ambiente</span>
          ) : savedKey ? (
            <span className="text-app-muted">chave salva neste FinFlow</span>
          ) : null}
        </p>

        {status.error ? (
          <p role="alert" className="text-sm text-expense">
            Não foi possível verificar o Assistente: {status.error.message}
          </p>
        ) : null}

        {fromEnv ? (
          <p
            className="rounded-(--radius-control) border border-app-border bg-app-background/60 px-3.5 py-2.5 text-sm text-app-muted"
            data-testid="ai-env-note"
          >
            A chave vem da variável de ambiente{" "}
            <code className="rounded bg-app-soft px-1 text-xs text-app-text">
              ANTHROPIC_API_KEY
            </code>{" "}
            e não pode ser removida por aqui. Para desativar o Assistente, remova a variável e
            reinicie o FinFlow.
          </p>
        ) : (
          <form onSubmit={(e) => void save(e)} className="flex flex-col gap-3" noValidate>
            <Field label="Chave da API do Claude">
              <Input
                type="password"
                autoComplete="off"
                spellCheck={false}
                placeholder={savedKey ? "Chave salva — cole outra para trocar" : "sk-ant-…"}
                value={apiKey}
                onChange={(e) => {
                  setApiKey(e.target.value);
                  setMessage(null);
                }}
              />
            </Field>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="submit" disabled={saving !== false}>
                {saving === "save" ? "Salvando…" : "Salvar"}
              </Button>
              {savedKey ? (
                <Button
                  type="button"
                  variant="danger"
                  onClick={() => void remove()}
                  disabled={saving !== false}
                >
                  {saving === "remove" ? "Removendo…" : "Remover chave"}
                </Button>
              ) : null}
            </div>
          </form>
        )}

        <div role="status" aria-live="polite" className="text-sm">
          {message ? (
            <span
              className={
                message.tone === "ok" ? "flex items-center gap-1.5 text-income" : "text-expense"
              }
            >
              {message.tone === "ok" ? <Check size={15} aria-hidden /> : null}
              {message.text}
            </span>
          ) : null}
        </div>

        <a
          href={ANTHROPIC_CONSOLE_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex min-h-11 items-center gap-1.5 self-start text-sm font-medium text-income underline-offset-4 hover:underline"
        >
          Criar uma chave no console.anthropic.com <ExternalLink size={14} aria-hidden />
        </a>
      </div>
    </Card>
  );
}
