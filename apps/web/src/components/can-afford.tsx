import type { CanAffordResult, CanAffordVerdict } from "@finflow/contracts";
import { CircleCheck, CircleX, type LucideIcon, ShoppingBag, TriangleAlert } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { useLocation } from "react-router-dom";
import { cn } from "../lib/cn";
import {
  centsToInput,
  formatLongDate,
  formatPercent,
  parseBRLInput,
  todayISO,
} from "../lib/format";
import { maskMoneyText } from "../lib/privacy";
import { useCanAfford, useCards } from "../lib/queries";
import { Button } from "./ui/button";
import { Field, Input, Select } from "./ui/field";
import { Money } from "./ui/money";

const VERDICT: Record<
  CanAffordVerdict,
  { title: string; icon: LucideIcon; box: string; text: string }
> = {
  YES: {
    title: "Pode comprar",
    icon: CircleCheck,
    box: "border-emerald-500/30 bg-emerald-500/8",
    text: "text-income",
  },
  CAREFUL: {
    title: "Dá, mas com cuidado",
    icon: TriangleAlert,
    box: "border-amber-400/40 bg-amber-400/8",
    text: "text-warn",
  },
  NO: {
    title: "Melhor esperar",
    icon: CircleX,
    box: "border-rose-500/40 bg-rose-500/8",
    text: "text-expense",
  },
};

/**
 * Simulador "Posso comprar?" (v3 §8), reutilizado em Projeção e Desejos.
 * Nada é gravado: a API recalcula a projeção com a compra e devolve o veredito.
 */
export function CanAffordForm({
  id,
  defaultAmountCents,
  description,
  className,
}: {
  /** Âncora (ex.: "posso-comprar"): com o hash na URL, rola até aqui e foca o valor. */
  id?: string;
  defaultAmountCents?: number;
  description?: string;
  className?: string;
}) {
  const cards = useCards();
  const check = useCanAfford();
  const { hash } = useLocation();
  const sectionRef = useRef<HTMLFormElement>(null);
  const fieldId = useId();
  const [amount, setAmount] = useState(defaultAmountCents ? centsToInput(defaultAmountCents) : "");
  const [date, setDate] = useState(todayISO());
  const [cardId, setCardId] = useState("");
  const [installments, setInstallments] = useState("1");

  useEffect(() => {
    if (id && hash === `#${id}`) {
      sectionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
      document.getElementById(`${fieldId}-amount`)?.focus({ preventScroll: true });
    }
  }, [hash, id, fieldId]);

  const cents = parseBRLInput(amount);
  const submit = (): void => {
    if (!cents) return;
    check.mutate({
      amountCents: cents,
      date: date || undefined,
      cardId: cardId || undefined,
      installments: cardId ? Number(installments) : undefined,
      description: description || undefined,
    });
  };

  return (
    <form
      ref={sectionRef}
      id={id}
      className={cn("flex scroll-mt-6 flex-col gap-4", className)}
      data-testid="can-afford"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      <div className="grid grid-cols-2 gap-3">
        <Field label="Valor da compra" error={amount && !cents ? "valor inválido" : undefined}>
          <Input
            id={`${fieldId}-amount`}
            inputMode="decimal"
            placeholder="1.500,00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        <Field label="Quando">
          <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Pagar com">
          <Select value={cardId} onChange={(e) => setCardId(e.target.value)}>
            <option value="">Saldo da conta (à vista)</option>
            {cards.data?.map((c) => (
              <option key={c.id} value={c.id}>
                Cartão {c.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Parcelas">
          <Select
            value={cardId ? installments : "1"}
            onChange={(e) => setInstallments(e.target.value)}
            disabled={!cardId}
          >
            {Array.from({ length: 48 }, (_, i) => i + 1).map((n) => (
              <option key={n} value={n}>
                {n === 1 ? "À vista" : `${n}x`}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <Button type="submit" variant="outline" disabled={!cents || check.isPending}>
        <ShoppingBag size={16} aria-hidden />
        {check.isPending ? "Calculando…" : "Simular compra"}
      </Button>
      {check.error ? <p className="text-sm text-expense">{check.error.message}</p> : null}
      {check.data ? <CanAffordVerdictBox result={check.data} withCard={Boolean(cardId)} /> : null}
    </form>
  );
}

function CanAffordVerdictBox({ result, withCard }: { result: CanAffordResult; withCard: boolean }) {
  const meta = VERDICT[result.verdict] ?? VERDICT.CAREFUL;
  const Icon = meta.icon;
  return (
    <div
      className={cn("flex flex-col gap-3 rounded-(--radius-control) border p-4", meta.box)}
      data-testid="can-afford-result"
      data-verdict={result.verdict}
      role="status"
    >
      <div className="flex gap-3">
        <Icon size={20} aria-hidden className={cn("mt-0.5 shrink-0", meta.text)} />
        <div className="min-w-0">
          <p className={cn("text-sm font-semibold", meta.text)}>{meta.title}</p>
          {result.message ? (
            <p className="mt-0.5 text-sm text-app-text">{maskMoneyText(result.message)}</p>
          ) : null}
        </div>
      </div>
      <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-xs text-app-muted">Menor saldo com a compra</dt>
          <dd className="tnum font-medium">
            <Money cents={result.lowestBalanceAfterCents} />{" "}
            <span className="text-xs font-normal text-app-muted">
              em {formatLongDate(result.lowestDate)}
            </span>
          </dd>
        </div>
        {result.limit ? (
          <div>
            <dt className="text-xs text-app-muted">Limite do mês depois da compra</dt>
            <dd className="tnum font-medium text-app-text">
              {formatPercent(result.limit.percentAfter)}{" "}
              <span className="text-xs font-normal text-app-muted">
                (<Money cents={result.limit.spentAfterCents} neutral /> de{" "}
                <Money cents={result.limit.limitCents} neutral />)
              </span>
            </dd>
          </div>
        ) : null}
        {result.reserveMonthsAfter !== null ? (
          <div>
            <dt className="text-xs text-app-muted">Reserva depois da compra</dt>
            <dd className="tnum font-medium text-app-text">
              cobre{" "}
              {new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 }).format(
                result.reserveMonthsAfter,
              )}{" "}
              {result.reserveMonthsAfter >= 1 && result.reserveMonthsAfter < 2 ? "mês" : "meses"}
            </dd>
          </div>
        ) : null}
        {result.fitsOn ? (
          <div>
            <dt className="text-xs text-app-muted">Cabe sem ficar negativo a partir de</dt>
            <dd className="font-medium text-app-text">{formatLongDate(result.fitsOn)}</dd>
          </div>
        ) : null}
      </dl>
      <p className="text-xs text-app-muted">
        {withCard
          ? "As parcelas entram nas faturas do cartão e saem do saldo no vencimento de cada uma."
          : "A compra sai do saldo disponível na data escolhida."}{" "}
        Simulação sobre a projeção dos próximos 12 meses; nada é lançado.
      </p>
    </div>
  );
}
