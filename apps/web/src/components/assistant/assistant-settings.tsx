import { useQueryClient } from "@tanstack/react-query";
import { Check, RefreshCw, ShieldCheck } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import {
  AI_DEFAULTS,
  AI_PROVIDER_LABEL,
  type AiModel,
  type AiProvider,
  type AiStatus,
  defaultBaseUrl,
  describeActiveAi,
  isLocalProvider,
  saveAiConfig,
  statusProvider,
} from "../../lib/ai";
import { ApiError } from "../../lib/api";
import { useAiModels, useAiStatus } from "../../lib/queries";
import { isBrowserRuntime } from "../../runtime/client";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Field, Input, Select } from "../ui/field";

const PROVIDERS: AiProvider[] = ["ollama", "openai_compatible", "claude"];

function Code({ children }: { children: React.ReactNode }) {
  return <code className="rounded bg-app-soft px-1 text-xs text-app-text">{children}</code>;
}

/** Instruções curtas de instalação por provedor. */
function SetupHelp({ provider }: { provider: AiProvider }) {
  if (provider === "ollama") {
    return (
      <ol className="list-decimal space-y-1 pl-5 text-xs text-app-muted" data-testid="ai-help">
        <li>
          Instale o Ollama em{" "}
          <a
            href="https://ollama.com/download"
            target="_blank"
            rel="noopener noreferrer"
            className="font-medium text-income underline-offset-4 hover:underline"
          >
            ollama.com
          </a>{" "}
          e deixe-o aberto.
        </li>
        <li>
          No terminal, baixe um modelo com ferramentas: <Code>ollama pull {AI_DEFAULTS.model}</Code>{" "}
          (alternativa: <Code>{AI_DEFAULTS.altModel}</Code>).
        </li>
        <li>
          Para ler comprovantes, um modelo de visão:{" "}
          <Code>ollama pull {AI_DEFAULTS.visionModel}</Code> (ou <Code>llava</Code>).
        </li>
        <li>Clique em “Testar conexão”, escolha o modelo e salve.</li>
        {isBrowserRuntime ? (
          <li>
            No navegador, o Ollama precisa liberar esta página: defina{" "}
            <Code>
              OLLAMA_ORIGINS={typeof window === "undefined" ? "*" : window.location.origin}
            </Code>{" "}
            e reinicie o Ollama.
          </li>
        ) : null}
      </ol>
    );
  }
  if (provider === "openai_compatible") {
    return (
      <ul className="list-disc space-y-1 pl-5 text-xs text-app-muted" data-testid="ai-help">
        <li>
          LM Studio: baixe um modelo com suporte a ferramentas (ex.: Qwen2.5 7B Instruct), abra
          Developer → “Start server” e use <Code>{AI_DEFAULTS.openaiUrl}</Code>.
        </li>
        <li>
          llama.cpp: <Code>llama-server -m modelo.gguf --jinja --port 8080</Code> e use{" "}
          <Code>http://localhost:8080/v1</Code>. vLLM e LocalAI também funcionam.
        </li>
        <li>A chave só é necessária se o servidor exigir.</li>
      </ul>
    );
  }
  return (
    <p className="text-xs text-app-muted" data-testid="ai-help">
      Crie uma chave em{" "}
      <a
        href="https://console.anthropic.com/"
        target="_blank"
        rel="noopener noreferrer"
        className="font-medium text-income underline-offset-4 hover:underline"
      >
        console.anthropic.com
      </a>
      . Ela fica salva só neste computador e nunca é mostrada de volta. Também dá para usar a
      variável <Code>ANTHROPIC_API_KEY</Code>.
    </p>
  );
}

/** Select de modelos vindo de /api/ai/models, com campo de texto quando a lista não está disponível. */
function ModelPicker({
  label,
  value,
  onChange,
  models,
  placeholder,
  optional,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  models: AiModel[] | undefined;
  placeholder: string;
  optional?: string;
}) {
  if (!models || models.length === 0) {
    return (
      <Field label={label}>
        <Input
          value={value}
          spellCheck={false}
          autoComplete="off"
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
        />
      </Field>
    );
  }
  const ids = models.map((m) => m.id);
  return (
    <Field label={label}>
      <Select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">{optional ?? "Escolha um modelo"}</option>
        {value && !ids.includes(value) ? (
          <option value={value}>{value} (não encontrado)</option>
        ) : null}
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.label}
            {m.tools === false ? " — sem ferramentas" : ""}
          </option>
        ))}
      </Select>
    </Field>
  );
}

interface FormState {
  provider: AiProvider;
  baseUrl: string;
  model: string;
  visionModel: string;
}

function initialForm(status: AiStatus | undefined): FormState {
  const provider = statusProvider(status) ?? "ollama";
  const local = isLocalProvider(provider);
  return {
    provider,
    baseUrl: local ? (status?.baseUrl ?? "") : "",
    model: local ? (status?.model ?? "") : "",
    visionModel: local ? (status?.visionModel ?? "") : "",
  };
}

/** Configurações → Assistente: provedor, URL, modelos, chave e teste de conexão (§3). */
export function AssistantSettingsSection() {
  const status = useAiStatus();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ tone: "ok" | "error"; text: string } | null>(null);
  const [testResult, setTestResult] = useState<{ tone: "ok" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (form === null && !status.isLoading) setForm(initialForm(status.data));
  }, [form, status.isLoading, status.data]);

  const provider = form?.provider ?? "ollama";
  const local = isLocalProvider(provider);
  const effectiveUrl = form?.baseUrl.trim() || defaultBaseUrl(provider);
  const models = useAiModels(provider, effectiveUrl, form !== null && local);

  if (form === null) {
    return <Card className="h-40 animate-pulse p-5 motion-reduce:animate-none" aria-busy="true" />;
  }

  const set = (patch: Partial<FormState>) => {
    setForm({ ...form, ...patch });
    setMessage(null);
    setTestResult(null);
  };

  const envLocked = status.data?.source === "ENV" && statusProvider(status.data) !== null;
  const savedClaudeKey =
    statusProvider(status.data) === "claude" && status.data?.source === "SETTINGS";

  const testConnection = async () => {
    setTestResult(null);
    const res = await models.refetch();
    if (res.error) {
      setTestResult({ tone: "error", text: res.error.message });
    } else {
      const n = res.data?.length ?? 0;
      setTestResult({
        tone: "ok",
        text:
          n === 0
            ? "Conectado, mas nenhum modelo encontrado. Baixe um modelo e teste de novo."
            : `Conectado: ${n} ${n === 1 ? "modelo encontrado" : "modelos encontrados"}.`,
      });
    }
  };

  const save = async (event: Pick<FormEvent, "preventDefault">, clearKey = false) => {
    event.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      const key = apiKey.trim();
      const next = await saveAiConfig({
        provider,
        ...(local
          ? {
              baseUrl: form.baseUrl.trim() || null,
              model: form.model.trim() || null,
              visionModel: form.visionModel.trim() || null,
            }
          : {}),
        ...(clearKey ? { apiKey: null } : key && provider !== "ollama" ? { apiKey: key } : {}),
      });
      setApiKey("");
      queryClient.setQueryData(["ai", "status"], next);
      void queryClient.invalidateQueries({ queryKey: ["system", "info"] });
      setMessage({
        tone: "ok",
        text: next.configured ? "Assistente salvo" : "Salvo — ainda falta completar a configuração",
      });
    } catch (e) {
      const issues =
        e instanceof ApiError && e.issues?.length ? e.issues.map((i) => i.message) : [];
      setMessage({
        tone: "error",
        text:
          issues.length > 0
            ? issues.join("; ")
            : e instanceof Error
              ? e.message
              : "Não foi possível salvar",
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      className="grid gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8"
      data-testid="ai-settings"
    >
      <div>
        <h2 className="text-sm font-semibold text-app-text">Assistente</h2>
        <p className="mt-1 text-sm text-app-muted">
          Escolha onde o assistente roda. Com um modelo local (Ollama ou servidor compatível), não é
          preciso serviço pago.
        </p>
        <p className="mt-3 flex gap-2 text-xs text-app-muted">
          <ShieldCheck size={15} aria-hidden className="mt-px shrink-0 text-income" />
          {local
            ? "Com um modelo local, as perguntas e os seus dados não saem deste computador."
            : "Com a API do Claude, as perguntas e os dados consultados vão para a Anthropic."}
        </p>
      </div>

      <form onSubmit={(e) => void save(e)} className="flex min-w-0 flex-col gap-4" noValidate>
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1.5 text-sm text-app-muted">Provedor</legend>
          {PROVIDERS.map((p) => (
            <label
              key={p}
              className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text has-[:checked]:border-emerald-500/60"
            >
              <input
                type="radio"
                name="ai-provider"
                value={p}
                checked={provider === p}
                onChange={() => {
                  // Voltar ao provedor salvo recupera os campos dele; outro provedor começa nos padrões.
                  const saved = initialForm(status.data);
                  set(
                    saved.provider === p
                      ? saved
                      : { provider: p, baseUrl: "", model: "", visionModel: "" },
                  );
                }}
                className="h-4 w-4 accent-emerald-500"
              />
              {AI_PROVIDER_LABEL[p]}
            </label>
          ))}
        </fieldset>

        {status.data?.source === "AUTO" ? (
          <p
            className="rounded-(--radius-control) border border-emerald-500/30 bg-emerald-500/5 px-3 py-2 text-xs text-app-muted"
            data-testid="ai-auto"
          >
            {describeActiveAi(status.data)}. Nada foi salvo: o FinFlow usa o Ollama encontrado em{" "}
            <Code>{status.data.baseUrl ?? AI_DEFAULTS.ollamaUrl}</Code> enquanto não houver outra
            escolha. Salve para fixar esta configuração.
          </p>
        ) : null}

        {envLocked ? (
          <p className="rounded-(--radius-control) border border-amber-400/40 bg-amber-400/5 px-3 py-2 text-xs text-app-muted">
            Há configuração por variáveis de ambiente (<Code>FINFLOW_AI_*</Code> ou{" "}
            <Code>ANTHROPIC_API_KEY</Code>); elas têm precedência sobre o que for salvo aqui.
          </p>
        ) : null}

        <SetupHelp provider={provider} />

        {local ? (
          <>
            <Field label="URL do servidor">
              <Input
                type="url"
                inputMode="url"
                spellCheck={false}
                autoComplete="off"
                placeholder={defaultBaseUrl(provider)}
                value={form.baseUrl}
                onChange={(e) => set({ baseUrl: e.target.value })}
              />
            </Field>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
              <div className="min-w-0 flex-1">
                <ModelPicker
                  label="Modelo"
                  value={form.model}
                  onChange={(model) => set({ model })}
                  models={models.data}
                  placeholder={
                    provider === "ollama" ? AI_DEFAULTS.model : "nome do modelo carregado"
                  }
                  optional={provider === "ollama" ? `Padrão (${AI_DEFAULTS.model})` : undefined}
                />
              </div>
              <Button
                type="button"
                variant="outline"
                onClick={() => void models.refetch()}
                disabled={models.isFetching}
                aria-label="Atualizar lista de modelos"
                className="sm:w-auto"
              >
                <RefreshCw
                  size={15}
                  aria-hidden
                  className={models.isFetching ? "animate-spin motion-reduce:animate-none" : ""}
                />
                <span className="sm:hidden">Atualizar modelos</span>
              </Button>
            </div>
            <ModelPicker
              label="Modelo de visão (comprovantes, opcional)"
              value={form.visionModel}
              onChange={(visionModel) => set({ visionModel })}
              models={models.data}
              placeholder={AI_DEFAULTS.visionModel}
              optional="Nenhum (sem leitura de comprovantes)"
            />
          </>
        ) : null}

        {provider !== "ollama" ? (
          <Field
            label={provider === "claude" ? "Chave da API do Claude" : "Chave da API (opcional)"}
          >
            <Input
              type="password"
              autoComplete="off"
              spellCheck={false}
              placeholder={
                provider === "claude"
                  ? savedClaudeKey
                    ? "Chave salva — deixe em branco para manter"
                    : "sk-ant-…"
                  : "deixe em branco para manter a salva"
              }
              value={apiKey}
              onChange={(e) => {
                setApiKey(e.target.value);
                setMessage(null);
              }}
            />
          </Field>
        ) : null}

        <div className="flex flex-wrap items-center gap-2">
          <Button type="submit" disabled={saving}>
            {saving ? "Salvando…" : "Salvar assistente"}
          </Button>
          {local ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => void testConnection()}
              disabled={models.isFetching}
            >
              {models.isFetching ? "Testando…" : "Testar conexão"}
            </Button>
          ) : null}
          {provider === "claude" && savedClaudeKey ? (
            <Button type="button" variant="danger" onClick={(e) => void save(e, true)}>
              Remover chave
            </Button>
          ) : null}
        </div>

        <div role="status" aria-live="polite" className="flex flex-col gap-1 text-sm">
          {testResult ? (
            <span
              data-testid="ai-test-result"
              className={testResult.tone === "ok" ? "text-income" : "text-expense"}
            >
              {testResult.text}
            </span>
          ) : null}
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
      </form>
    </Card>
  );
}
