import type {
  CreateRecurringInput,
  CreateTransactionInput,
  CreateWishItemInput,
} from "@finflow/contracts";
import { CalendarClock, Check, Gift, PiggyBank, Receipt, X } from "lucide-react";
import { useState } from "react";
import type { AiProposalKind, StoredProposal } from "../../lib/ai";
import { cn } from "../../lib/cn";
import {
  useContributeGoal,
  useCreateRecurring,
  useCreateTransaction,
  useCreateWish,
} from "../../lib/queries";
import { Button } from "../ui/button";

const KIND_META: Record<AiProposalKind, { title: string; icon: typeof Receipt }> = {
  CREATE_TRANSACTION: { title: "Lançamento", icon: Receipt },
  CREATE_RECURRING: { title: "Compromisso mensal", icon: CalendarClock },
  CONTRIBUTE_GOAL: { title: "Movimento em meta", icon: PiggyBank },
  ADD_WISH: { title: "Desejo", icon: Gift },
};

/**
 * Cartão de proposta do Assistente: nada é gravado até o usuário confirmar, e a confirmação chama o
 * endpoint normal (mesmas validações e invalidação de cache de qualquer outra escrita).
 */
export function ProposalCard({
  proposal,
  onStatusChange,
}: {
  proposal: StoredProposal;
  onStatusChange: (status: StoredProposal["status"]) => void;
}) {
  const createTransaction = useCreateTransaction();
  const createRecurring = useCreateRecurring();
  const contributeGoal = useContributeGoal();
  const createWish = useCreateWish();
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const meta = KIND_META[proposal.kind] ?? KIND_META.CREATE_TRANSACTION;
  const Icon = meta.icon;

  const confirm = async () => {
    setError(null);
    setSaving(true);
    try {
      const payload = proposal.payload;
      switch (proposal.kind) {
        case "CREATE_TRANSACTION":
          await createTransaction.mutateAsync(payload as unknown as CreateTransactionInput);
          break;
        case "CREATE_RECURRING":
          await createRecurring.mutateAsync(payload as unknown as CreateRecurringInput);
          break;
        case "CONTRIBUTE_GOAL": {
          const { goalId, ...rest } = payload as { goalId: string; amountCents: number };
          await contributeGoal.mutateAsync({ id: goalId, ...rest });
          break;
        }
        case "ADD_WISH":
          await createWish.mutateAsync(payload as unknown as CreateWishItemInput);
          break;
      }
      onStatusChange("confirmed");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar");
    } finally {
      setSaving(false);
    }
  };

  const done = proposal.status !== "pending";

  return (
    <div
      data-testid="ai-proposal"
      className={cn(
        "rounded-(--radius-card) border bg-app-surface p-3.5",
        proposal.status === "confirmed" ? "border-emerald-500/50" : "border-app-border",
        proposal.status === "discarded" && "opacity-60",
      )}
    >
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-income">
          <Icon size={18} aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium tracking-wide text-app-subtle uppercase">
            {meta.title}
          </p>
          <p className="mt-0.5 text-[15px] text-app-text">{proposal.summary}</p>
        </div>
      </div>
      {done ? (
        <p
          className={cn(
            "mt-3 flex items-center gap-1.5 text-sm",
            proposal.status === "confirmed" ? "text-income" : "text-app-muted",
          )}
        >
          {proposal.status === "confirmed" ? (
            <>
              <Check size={16} aria-hidden /> Confirmado e salvo
            </>
          ) : (
            <>
              <X size={16} aria-hidden /> Descartado
            </>
          )}
        </p>
      ) : (
        <div className="mt-3 flex flex-col gap-2">
          {error ? (
            <p role="alert" className="text-sm text-expense">
              {error}
            </p>
          ) : null}
          <div className="flex gap-2">
            <Button className="flex-1" onClick={confirm} disabled={saving}>
              {saving ? "Salvando…" : "Confirmar"}
            </Button>
            <Button
              variant="outline"
              className="flex-1"
              onClick={() => onStatusChange("discarded")}
              disabled={saving}
            >
              Descartar
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
