import { ChevronLeft, ChevronRight } from "lucide-react";
import { useState } from "react";
import { formatMonthYear } from "../../lib/format";
import type { Period } from "../../lib/queries";

export function currentPeriod(): Period {
  const now = new Date();
  return { month: now.getMonth() + 1, year: now.getFullYear() };
}

/** Mês/ano selecionado com deslocamento relativo (−1 = mês anterior). */
export function usePeriodState(): [Period, (delta: number) => void] {
  const [period, setPeriod] = useState<Period>(currentPeriod);
  const shift = (delta: number): void => {
    setPeriod((p) => {
      const zero = p.year * 12 + (p.month - 1) + delta;
      return { year: Math.floor(zero / 12), month: (zero % 12) + 1 };
    });
  };
  return [period, shift];
}

const arrowCls =
  "flex h-11 w-11 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-surface hover:text-app-text";

export function MonthSwitcher({
  period,
  onShift,
}: {
  period: Period;
  onShift: (delta: number) => void;
}) {
  return (
    <div className="flex items-center gap-1">
      <button
        type="button"
        onClick={() => onShift(-1)}
        aria-label="Mês anterior"
        className={arrowCls}
      >
        <ChevronLeft size={18} />
      </button>
      <span
        className="tnum min-w-32 text-center text-sm font-medium text-app-text"
        aria-live="polite"
      >
        {formatMonthYear(period.month, period.year)}
      </span>
      <button
        type="button"
        onClick={() => onShift(1)}
        aria-label="Próximo mês"
        className={arrowCls}
      >
        <ChevronRight size={18} />
      </button>
    </div>
  );
}

/** Seletor de ano com setas (página Ano, exportação anual). */
export function YearSwitcher({ year, onChange }: { year: number; onChange: (y: number) => void }) {
  return (
    <div className="flex items-center gap-1">
      <button
        type="button"
        onClick={() => onChange(year - 1)}
        aria-label="Ano anterior"
        className={arrowCls}
      >
        <ChevronLeft size={18} />
      </button>
      <span
        className="tnum min-w-16 text-center text-sm font-medium text-app-text"
        aria-live="polite"
      >
        {year}
      </span>
      <button
        type="button"
        onClick={() => onChange(year + 1)}
        aria-label="Próximo ano"
        className={arrowCls}
      >
        <ChevronRight size={18} />
      </button>
    </div>
  );
}
