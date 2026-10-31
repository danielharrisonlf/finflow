import type { ButtonHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

type Variant = "primary" | "ghost" | "outline" | "danger";
type Size = "sm" | "md" | "icon";

const variants: Record<Variant, string> = {
  primary:
    "bg-emerald-500 text-emerald-950 font-semibold hover:bg-emerald-400 active:bg-emerald-500 disabled:bg-zinc-700 disabled:text-zinc-400",
  ghost: "text-zinc-300 hover:bg-zinc-800/80 hover:text-zinc-100",
  outline: "border border-zinc-700 text-zinc-200 hover:border-zinc-500 hover:bg-zinc-900",
  danger: "text-rose-400 hover:bg-rose-500/10",
};

const sizes: Record<Size, string> = {
  sm: "h-9 px-3 text-sm",
  md: "h-11 px-4 text-sm",
  icon: "h-11 w-11",
};

export function Button({
  variant = "primary",
  size = "md",
  className,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: Size }) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 rounded-(--radius-control) transition-colors disabled:cursor-not-allowed",
        variants[variant],
        sizes[size],
        className,
      )}
      {...props}
    />
  );
}
