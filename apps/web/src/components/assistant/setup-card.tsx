import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink, KeyRound } from "lucide-react";
import { type FormEvent, useState } from "react";
import { saveAiKey } from "../../lib/ai";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Field, Input } from "../ui/field";

/** Configuração do Assistente: a chave vai para o servidor local e nunca volta para o navegador. */
export function AssistantSetupCard() {
  const queryClient = useQueryClient();
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = apiKey.trim();
    if (!trimmed) {
      setError("Cole a chave da API");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const status = await saveAiKey(trimmed);
      setApiKey("");
      queryClient.setQueryData(["ai", "status"], status);
      void queryClient.invalidateQueries({ queryKey: ["system", "info"] });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar a chave");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="p-5" data-testid="ai-setup">
      <div className="flex items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-income">
          <KeyRound size={20} aria-hidden />
        </span>
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-app-text">Ative o Assistente</h2>
          <p className="mt-1 text-sm text-app-muted">
            O Assistente usa a API do Claude com a sua própria chave. Ela fica salva só neste
            computador, no banco do FinFlow, e nunca é mostrada de volta na tela.
          </p>
        </div>
      </div>

      <form onSubmit={submit} className="mt-4 flex flex-col gap-3">
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
          <Button type="submit" disabled={saving} className="sm:w-auto">
            {saving ? "Salvando…" : "Salvar chave"}
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

      <details className="mt-4 rounded-(--radius-control) border border-app-border/80 bg-app-background/60 px-3.5 py-2.5 text-sm text-app-muted">
        <summary className="flex min-h-11 cursor-pointer items-center text-app-text">
          Prefere variável de ambiente?
        </summary>
        <p className="mt-1">
          Defina <code className="rounded bg-app-soft px-1">ANTHROPIC_API_KEY</code> antes de
          iniciar o FinFlow (ou entre com{" "}
          <code className="rounded bg-app-soft px-1">ant auth login</code>) e reinicie o servidor:
        </p>
        <pre className="mt-2 overflow-x-auto rounded bg-app-soft p-2.5 text-xs text-app-text">
          {"export ANTHROPIC_API_KEY=sk-ant-...\nfinflow serve"}
        </pre>
        <p className="mt-2 text-xs">
          No Windows (PowerShell): <code>$env:ANTHROPIC_API_KEY="sk-ant-..."</code>
        </p>
      </details>
    </Card>
  );
}
