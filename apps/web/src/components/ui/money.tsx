import { cn } from "../../lib/cn";
import { formatBRL, formatSignedBRL } from "../../lib/format";

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
  const tone = neutral
    ? "text-app-text"
    : cents >= 0
      ? "text-(--color-income)"
      : "text-(--color-expense)";
  return (
    <span className={cn("tnum", tone, className)}>
      {signed ? formatSignedBRL(cents) : formatBRL(cents)}
    </span>
  );
}
