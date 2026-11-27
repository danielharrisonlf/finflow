import { cn } from "../../lib/cn";
import { formatAxisBRL, formatBRL, formatSignedBRL } from "../../lib/format";
import { usePrivacy } from "../../lib/privacy";

/**
 * Formatadores monetários que respeitam o modo privacidade e re-renderizam quem os usa
 * quando o modo muda (os helpers de lib/format já mascaram; o hook garante a assinatura).
 */
export function useMoney() {
  const [hidden] = usePrivacy();
  return { hidden, brl: formatBRL, signed: formatSignedBRL, axis: formatAxisBRL };
}

/** Valor monetário com dígitos tabulares; cor só quando o sinal importa. */
export function Money({
  cents,
  signed = false,
  neutral = false,
  className,
}: {
  cents: number;
  signed?: boolean;
  neutral?: boolean;
  className?: string;
}) {
  const money = useMoney();
  const tone = neutral
    ? "text-app-text"
    : cents >= 0
      ? "text-(--color-income)"
      : "text-(--color-expense)";
  return (
    <span className={cn("tnum", tone, className)} data-masked={money.hidden || undefined}>
      {signed ? money.signed(cents) : money.brl(cents)}
    </span>
  );
}
