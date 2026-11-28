import { useQueryClient } from "@tanstack/react-query";
import { ArrowUp, Paperclip, PowerOff, RotateCcw, Sparkles, Square } from "lucide-react";
import {
  type ChangeEvent,
  type FormEvent,
  type KeyboardEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import { Link, useNavigate } from "react-router-dom";
import { ASSISTANT_SETTINGS_ANCHOR } from "../components/assistant/assistant-settings";
import { ChatMessage } from "../components/assistant/chat-message";
import {
  type NewTransactionInitialValues,
  NewTransactionSheet,
} from "../components/new-transaction-sheet";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import {
  type AiEvent,
  type AiReceipt,
  clearHistory,
  extractReceipt,
  isSpreadsheetFile,
  loadHistory,
  MAX_RECEIPT_BYTES,
  newId,
  RECEIPT_ACCEPT,
  type StoredProposal,
  saveAiKey,
  saveHistory,
  streamChat,
  toChatPayload,
  type UiMessage,
} from "../lib/ai";
import { ApiError } from "../lib/api";
import { formatBRL, formatDate } from "../lib/format";
import { useAiStatus } from "../lib/queries";

export const ASSISTANT_SUGGESTIONS = [
  "Posso jantar fora esse mês?",
  "Posso fazer uma viagem de R$ 3.000 em março?",
  "Posso comprar um celular de R$ 5.000 em 10x?",
  "Onde estou gastando mais?",
];

const ATTACH_ACCEPT = `${RECEIPT_ACCEPT},.xlsx,.xls,.csv,.ofx`;

function applyEvent(message: UiMessage, event: AiEvent): UiMessage {
  switch (event.type) {
    case "text":
      return { ...message, content: message.content + event.delta };
    case "tool":
      return {
        ...message,
        tools: [...(message.tools ?? []), { name: event.name, label: event.label }],
      };
    case "proposal":
      return {
        ...message,
        proposals: [...(message.proposals ?? []), { ...event.proposal, status: "pending" }],
      };
    case "error":
      return { ...message, error: event.message };
    case "done":
      // Recusa: a resposta parcial é descartada (fica só a mensagem de erro amigável).
      return {
        ...message,
        stopReason: event.stopReason,
        content: event.stopReason === "refusal" ? "" : message.content,
      };
  }
}

function receiptToInitial(r: AiReceipt): NewTransactionInitialValues {
  const tab =
    r.kind === "INCOME" ? "income" : r.installments && r.installments > 1 ? "card" : "expense";
  return {
    tab,
    amountCents: r.amountCents > 0 ? r.amountCents : undefined,
    description: r.description,
    date: r.date,
    categoryId: r.suggestedCategoryId,
    installments: r.installments,
  };
}

function receiptSummary(r: AiReceipt): string {
  const parts = [`Li o comprovante: **${r.description}**`];
  if (r.amountCents > 0) parts.push(`${formatBRL(r.amountCents)} em ${formatDate(r.date)}`);
  if (r.installments && r.installments > 1) parts.push(`em ${r.installments}x`);
  let text = `${parts.join(" — ")}.\nAbri o formulário de lançamento já preenchido para você conferir e salvar.`;
  if (r.confidence === "LOW")
    text += "\n*Não consegui ler tudo com segurança — confira os campos.*";
  return text;
}

export function AssistantPage() {
  const status = useAiStatus();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [messages, setMessages] = useState<UiMessage[]>(() =>
    loadHistory().map((m) => ({ ...m, streaming: false })),
  );
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState<false | "chat" | "receipt">(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [sheetInitial, setSheetInitial] = useState<NewTransactionInitialValues | undefined>();
  const abortRef = useRef<AbortController | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    saveHistory(messages);
  }, [messages]);

  useEffect(() => () => abortRef.current?.abort(), []);

  const lastContentLength = messages[messages.length - 1]?.content.length ?? 0;
  // biome-ignore lint/correctness/useExhaustiveDependencies: rola quando chega texto novo ou mensagem nova
  useEffect(() => {
    if (busy) endRef.current?.scrollIntoView?.({ block: "end" });
  }, [messages.length, lastContentLength, busy]);

  const updateMessage = (id: string, fn: (m: UiMessage) => UiMessage) =>
    setMessages((list) => list.map((m) => (m.id === id ? fn(m) : m)));

  const send = async (text: string) => {
    const question = text.trim();
    if (!question || busy) return;
    const userMsg: UiMessage = { id: newId(), role: "user", content: question };
    const assistantId = newId();
    const history = [...messages, userMsg];
    setMessages([...history, { id: assistantId, role: "assistant", content: "", streaming: true }]);
    setInput("");
    setBusy("chat");
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      await streamChat(
        toChatPayload(history),
        (event) => updateMessage(assistantId, (m) => applyEvent(m, event)),
        controller.signal,
      );
    } catch (e) {
      if (controller.signal.aborted) {
        updateMessage(assistantId, (m) => ({
          ...m,
          error: m.content ? undefined : "Resposta interrompida.",
        }));
      } else {
        if (e instanceof ApiError && e.status === 412) void status.refetch();
        const message = e instanceof Error ? e.message : "Não foi possível falar com o assistente.";
        updateMessage(assistantId, (m) => ({ ...m, error: message }));
      }
    } finally {
      updateMessage(assistantId, (m) => ({ ...m, streaming: false }));
      abortRef.current = null;
      setBusy(false);
      textareaRef.current?.focus();
    }
  };

  const onProposalStatus = (
    messageId: string,
    proposalId: string,
    next: StoredProposal["status"],
  ) =>
    updateMessage(messageId, (m) => ({
      ...m,
      proposals: m.proposals?.map((p) => (p.id === proposalId ? { ...p, status: next } : p)),
    }));

  const onFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (isSpreadsheetFile(file.name)) {
      navigate("/planilhas");
      return;
    }
    const userMsg: UiMessage = {
      id: newId(),
      role: "user",
      content: `Comprovante: ${file.name}`,
      local: true,
    };
    const replyId = newId();
    if (file.size > MAX_RECEIPT_BYTES) {
      setMessages((list) => [
        ...list,
        userMsg,
        {
          id: replyId,
          role: "assistant",
          content: "",
          local: true,
          error: "Arquivo maior que 10 MiB.",
        },
      ]);
      return;
    }
    setMessages((list) => [
      ...list,
      userMsg,
      {
        id: replyId,
        role: "assistant",
        content: "",
        local: true,
        streaming: true,
        tools: [{ name: "receipt", label: "Lendo seu comprovante" }],
      },
    ]);
    setBusy("receipt");
    try {
      const receipt = await extractReceipt(file);
      updateMessage(replyId, (m) => ({ ...m, content: receiptSummary(receipt) }));
      setSheetInitial(receiptToInitial(receipt));
      setSheetOpen(true);
    } catch (e) {
      const message = e instanceof Error ? e.message : "Não foi possível ler o comprovante.";
      updateMessage(replyId, (m) => ({ ...m, error: message }));
    } finally {
      updateMessage(replyId, (m) => ({ ...m, streaming: false }));
      setBusy(false);
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    void send(input);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void send(input);
    }
  };

  const newConversation = () => {
    abortRef.current?.abort();
    setMessages([]);
    clearHistory();
  };

  const removeKey = async () => {
    try {
      const next = await saveAiKey(null);
      queryClient.setQueryData(["ai", "status"], next);
      void queryClient.invalidateQueries({ queryKey: ["system", "info"] });
    } catch {
      void status.refetch();
    }
  };

  const header = (
    <header className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold tracking-tight">Assistente</h1>
        <p className="text-sm text-app-muted">
          Pergunte sobre seus gastos, saldos e planos — as respostas usam os seus dados.
        </p>
      </div>
      {status.data?.configured && messages.length > 0 ? (
        <Button
          variant="ghost"
          onClick={newConversation}
          aria-label="Nova conversa"
          className="px-3"
        >
          <RotateCcw size={16} aria-hidden />
          <span className="hidden sm:inline">Nova conversa</span>
        </Button>
      ) : null}
    </header>
  );

  if (status.isLoading) {
    return (
      <div className="space-y-4">
        {header}
        <Card className="h-40 animate-pulse motion-reduce:animate-none" aria-busy="true" />
      </div>
    );
  }

  // Sem chave o recurso fica desativado: nada de chat nem anexo, só o caminho para Configurações.
  if (!status.data?.configured) {
    return (
      <div className="mx-auto max-w-xl space-y-4">
        {header}
        {status.error ? (
          <p role="alert" className="text-sm text-expense">
            Não foi possível verificar o Assistente: {status.error.message}
          </p>
        ) : null}
        <Card className="flex items-start gap-3 p-5" data-testid="ai-disabled">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-app-soft text-app-muted">
            <PowerOff size={18} aria-hidden />
          </span>
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-app-text">O Assistente está desativado</h2>
            <p className="mt-1 text-sm text-app-muted">
              Ele usa a API do Claude com a sua chave. Para ativar, coloque a chave em Configurações
              → Assistente (IA). Todo o resto do FinFlow funciona sem ele.
            </p>
            <Link
              to={`/configuracoes#${ASSISTANT_SETTINGS_ANCHOR}`}
              className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-income underline-offset-4 hover:underline"
            >
              Abrir Configurações
            </Link>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      {header}

      <section aria-label="Conversa" aria-live="polite" className="flex flex-col gap-5">
        {messages.length === 0 ? (
          <div className="flex flex-col items-center gap-4 py-8 text-center">
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-emerald-500/12 text-income">
              <Sparkles size={22} aria-hidden />
            </span>
            <div>
              <p className="text-base font-medium text-app-text">Pergunte sobre o seu dinheiro</p>
              <p className="mt-1 text-sm text-app-muted">
                Ou anexe a foto de um comprovante para lançar mais rápido.
              </p>
            </div>
            <ul className="flex w-full flex-col gap-2 sm:flex-row sm:flex-wrap sm:justify-center">
              {ASSISTANT_SUGGESTIONS.map((s) => (
                <li key={s}>
                  <button
                    type="button"
                    onClick={() => void send(s)}
                    className="min-h-11 w-full cursor-pointer rounded-full border border-app-border bg-app-surface px-4 text-sm text-app-text transition-colors hover:border-emerald-500/50 hover:bg-app-soft sm:w-auto"
                  >
                    {s}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          messages.map((m) => (
            <ChatMessage
              key={m.id}
              message={m}
              onProposalStatus={(pid, next) => onProposalStatus(m.id, pid, next)}
            />
          ))
        )}
        <div ref={endRef} />
      </section>

      <form
        onSubmit={submit}
        className="sticky bottom-[calc(6.5rem+env(safe-area-inset-bottom))] z-20 md:bottom-4"
      >
        <div className="flex items-end gap-1.5 rounded-2xl border border-app-border bg-app-surface p-1.5 shadow-lg shadow-black/5">
          <input
            ref={fileRef}
            type="file"
            accept={ATTACH_ACCEPT}
            className="sr-only"
            tabIndex={-1}
            onChange={onFile}
            data-testid="ai-attach-input"
          />
          <Button
            variant="ghost"
            size="icon"
            aria-label="Anexar comprovante ou planilha"
            onClick={() => fileRef.current?.click()}
            disabled={busy !== false}
          >
            <Paperclip size={18} aria-hidden />
          </Button>
          <label htmlFor="ai-input" className="sr-only">
            Pergunta para o assistente
          </label>
          <textarea
            id="ai-input"
            ref={textareaRef}
            rows={1}
            value={input}
            maxLength={4000}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Posso comprar…? Onde estou gastando mais?"
            className="max-h-40 min-h-11 flex-1 resize-none bg-transparent px-1.5 py-2.5 text-[15px] text-app-text placeholder:text-app-muted focus:outline-none [field-sizing:content]"
          />
          {busy === "chat" ? (
            <Button
              variant="outline"
              size="icon"
              aria-label="Parar resposta"
              onClick={() => abortRef.current?.abort()}
            >
              <Square size={16} aria-hidden />
            </Button>
          ) : (
            <Button
              type="submit"
              size="icon"
              aria-label="Enviar"
              disabled={busy !== false || !input.trim()}
            >
              <ArrowUp size={18} aria-hidden />
            </Button>
          )}
        </div>
        <p className="mt-1.5 px-2 text-center text-[11px] text-app-subtle">
          Respostas geradas por IA a partir dos seus dados — confira antes de decidir. Não é
          recomendação de investimento.
          {status.data.source === "SETTINGS" ? (
            <>
              {" "}
              <button
                type="button"
                onClick={removeKey}
                className="cursor-pointer underline underline-offset-2 hover:text-app-muted"
              >
                Remover chave
              </button>
            </>
          ) : null}
        </p>
      </form>

      <NewTransactionSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        initialValues={sheetInitial}
      />
    </div>
  );
}

export default AssistantPage;
