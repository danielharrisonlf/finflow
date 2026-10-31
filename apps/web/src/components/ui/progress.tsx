import { cn } from "../../lib/cn";

const tones = {
  OK: "bg-emerald-500",
  WARNING: "bg-amber-400",
  DANGER: "bg-rose-500",
  NEUTRAL: "bg-zinc-400",
} as const;

export function Progress({
  percent,
  tone = "NEUTRAL",
  className,
}: {
  percent: number;
  tone?: keyof typeof tones;
  className?: string;
}) {
  return (
    <div
      className={cn("h-1.5 w-full overflow-hidden rounded-full bg-zinc-800", className)}
      role="progressbar"
      aria-valuenow={Math.round(percent)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className={cn("h-full rounded-full transition-[width] duration-500", tones[tone])}
        style={{ width: `${Math.min(100, Math.max(0, percent))}%` }}
      />
    </div>
  );
}
