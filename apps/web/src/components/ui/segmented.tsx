import { cn } from "../../lib/cn";

/** Alternador de opções exclusivas (botões com aria-pressed), no visual das abas do app. */
export function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
  className,
}: {
  value: T;
  onChange: (value: T) => void;
  options: readonly (readonly [T, string])[];
  label: string;
  className?: string;
}) {
  return (
    <fieldset
      className={cn("flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1", className)}
    >
      <legend className="sr-only">{label}</legend>
      {options.map(([key, text]) => (
        <button
          key={key}
          type="button"
          aria-pressed={value === key}
          onClick={() => onChange(key)}
          className={cn(
            "min-h-10 flex-1 cursor-pointer rounded-(--radius-control) px-3 text-sm whitespace-nowrap transition-colors",
            value === key
              ? "bg-app-soft font-medium text-app-text"
              : "text-app-muted hover:text-app-text",
          )}
        >
          {text}
        </button>
      ))}
    </fieldset>
  );
}

/** Pílula discreta para rótulos (tipo de gasto, duplicado, status). */
export function Badge({
  children,
  tone = "neutral",
  className,
  ...rest
}: React.HTMLAttributes<HTMLSpanElement> & {
  tone?: "neutral" | "income" | "expense" | "warn";
}) {
  const tones = {
    neutral: "border-app-border text-app-muted",
    income: "border-emerald-500/40 text-income",
    expense: "border-rose-500/40 text-expense",
    warn: "border-amber-400/50 text-warn",
  } as const;
  return (
    <span
      className={cn(
        "inline-flex h-6 shrink-0 items-center gap-1 rounded-full border px-2 text-[11px] font-medium whitespace-nowrap",
        tones[tone],
        className,
      )}
      {...rest}
    >
      {children}
    </span>
  );
}
