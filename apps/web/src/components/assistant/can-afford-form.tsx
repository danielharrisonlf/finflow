import type { CanAffordResult } from "@finflow/contracts";
import { ShoppingBag } from "lucide-react";
import { type FormEvent, useState } from "react";
import { cn } from "../../lib/cn";
import { formatBRL, formatDate, parseBRLInput } from "../../lib/format";
import { useCanAfford, useCards } from "../../lib/queries";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Field, Input, Select } from "../ui/field";

const VERDICT: Record<CanAffordResult["verdict"], { label: string; cls: string }> = {
  YES: { label: "Cabe no seu orçamento", cls: "border-emerald-500/40 text-income" },
  CAREFUL: { label: "Cabe, mas com cuidado", cls: "border-amber-400/50 text-warn" },
  NO: { label: "Não cabe agora", cls: "border-rose-500/40 text-expense" },
};

/** Simulador "Posso comprar?" (§8) — funciona mesmo sem o Assistente configurado. */
export function CanAffordForm() {
  const cards = useCards();
  const mutation = useCanAfford();
  const [amount, setAmount] = useState("");
  const [installments, setInstallments] = useState("1");
  const [date, setDate] = useState("");
  const [cardId, setCardId] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const cents = parseBRLInput(amount);
    if (!cents || cents <= 0) {
      setError("valor inválido — use 3.000,00");
      return;
    }
    setError(null);
    mutation.mutate({
      amountCents: cents,
      installments: Number(installments),
      date: date || undefined,
      cardId: cardId || undefined,
      description: description.trim() || undefined,
    });
  };

  const result = mutation.data;
  const verdict = result ? VERDICT[result.verdict] : null;

  return (
    <Card className="p-5" data-testid="can-afford">
      <div className="flex items-center gap-2.5">
        <ShoppingBag size={18} className="text-app-muted" aria-hidden />
        <h2 className="text-base font-semibold text-app-text">Posso comprar?</h2>
      </div>
      <p className="mt-1 text-sm text-app-muted">
        Simule uma compra sobre a sua projeção de saldo, à vista ou parcelada.
      </p>
      <form onSubmit={submit} className="mt-4 grid grid-cols-2 gap-3">
        <Field label="Valor total" error={error ?? undefined} className="col-span-2 sm:col-span-1">
          <Input
            inputMode="decimal"
            placeholder="3.000,00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        <Field label="O que é (opcional)" className="col-span-2 sm:col-span-1">
          <Input
            placeholder="Celular, viagem…"
            maxLength={120}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </Field>
        <Field label="Parcelas">
          <Select value={installments} onChange={(e) => setInstallments(e.target.value)}>
            {Array.from({ length: 48 }, (_, i) => i + 1).map((n) => (
              <option key={n} value={n}>
                {n === 1 ? "À vista" : `${n}x`}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Data (opcional)">
          <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label="Cartão (opcional)" className="col-span-2">
          <Select value={cardId} onChange={(e) => setCardId(e.target.value)}>
            <option value="">Sai do saldo da conta</option>
            {cards.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>
        <Button type="submit" className="col-span-2" disabled={mutation.isPending}>
          {mutation.isPending ? "Simulando…" : "Simular"}
        </Button>
      </form>
      {mutation.error ? (
        <p role="alert" className="mt-3 text-sm text-expense">
          {mutation.error.message}
        </p>
      ) : null}
      {result && verdict ? (
        <div
          data-testid="can-afford-result"
          aria-live="polite"
          className={cn("mt-4 rounded-(--radius-control) border p-3.5", verdict.cls)}
        >
          <p className="font-semibold">{verdict.label}</p>
          <p className="mt-1 text-sm text-app-text">{result.message}</p>
          <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-sm text-app-muted">
            <dt>Menor saldo depois</dt>
            <dd className="tnum text-right text-app-text">
              {formatBRL(result.lowestBalanceAfterCents)} em {formatDate(result.lowestDate)}
            </dd>
            {result.fitsOn ? (
              <>
                <dt>Caberia a partir de</dt>
                <dd className="tnum text-right text-app-text">{formatDate(result.fitsOn)}</dd>
              </>
            ) : null}
          </dl>
        </div>
      ) : null}
    </Card>
  );
}
