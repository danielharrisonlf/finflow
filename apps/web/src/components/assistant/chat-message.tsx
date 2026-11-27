import { AlertCircle, Check, Loader2, Sparkles } from "lucide-react";
import type { StoredProposal, UiMessage } from "../../lib/ai";
import { cn } from "../../lib/cn";
import { ProposalCard } from "./proposal-card";
import { SafeMarkdown } from "./safe-markdown";

export function ChatMessage({
  message,
  onProposalStatus,
}: {
  message: UiMessage;
  onProposalStatus: (proposalId: string, status: StoredProposal["status"]) => void;
}) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end" data-testid="ai-message-user">
        <div className="max-w-[85%] rounded-2xl rounded-br-md bg-emerald-500/15 px-4 py-2.5 text-[15px] whitespace-pre-wrap text-app-text">
          {message.content}
        </div>
      </div>
    );
  }

  const tools = message.tools ?? [];
  const activeTool = message.streaming && !message.content ? tools[tools.length - 1] : undefined;
  const thinking = message.streaming && !message.content && tools.length === 0;

  return (
    <div className="flex gap-2.5" data-testid="ai-message-assistant">
      <span
        className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-app-soft text-income"
        aria-hidden
      >
        <Sparkles size={16} />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2.5">
        {tools.length > 0 ? (
          <ul className="flex flex-wrap gap-1.5" aria-label="Consultas feitas">
            {tools.map((tool, i) => {
              const running = tool === activeTool;
              return (
                <li
                  // biome-ignore lint/suspicious/noArrayIndexKey: a mesma ferramenta pode aparecer várias vezes
                  key={`${tool.name}-${i}`}
                  data-testid="ai-tool-chip"
                  className={cn(
                    "inline-flex min-h-7 items-center gap-1.5 rounded-full border px-2.5 text-xs",
                    running
                      ? "border-emerald-500/40 text-app-text"
                      : "border-app-border/80 text-app-muted",
                  )}
                >
                  {running ? (
                    <Loader2
                      size={12}
                      className="animate-spin motion-reduce:animate-none"
                      aria-hidden
                    />
                  ) : (
                    <Check size={12} aria-hidden />
                  )}
                  {running ? `${tool.label}…` : tool.label}
                </li>
              );
            })}
          </ul>
        ) : null}

        {thinking ? (
          <p className="flex items-center gap-2 text-sm text-app-muted" role="status">
            <Loader2 size={14} className="animate-spin motion-reduce:animate-none" aria-hidden />
            Pensando…
          </p>
        ) : null}

        {message.content ? (
          <div className="text-[15px] text-app-text" data-testid="ai-answer">
            <SafeMarkdown text={message.content} />
          </div>
        ) : null}

        {message.proposals?.map((p) => (
          <ProposalCard
            key={p.id}
            proposal={p}
            onStatusChange={(status) => onProposalStatus(p.id, status)}
          />
        ))}

        {message.error ? (
          <p
            role="alert"
            className="flex items-start gap-2 rounded-(--radius-control) border border-rose-500/30 bg-rose-500/8 px-3 py-2 text-sm text-app-text"
          >
            <AlertCircle size={16} className="mt-0.5 shrink-0 text-expense" aria-hidden />
            {message.error}
          </p>
        ) : null}
      </div>
    </div>
  );
}
