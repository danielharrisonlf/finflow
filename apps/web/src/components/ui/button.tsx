import type { ButtonHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

type Variant = "primary" | "ghost" | "outline" | "danger";
type Size = "sm" | "md" | "icon";

const variants: Record<Variant, string> = {
  primary:
    "bg-emerald-500 text-emerald-950 font-semibold hover:bg-emerald-400 active:bg-emerald-500 disabled:bg-app-raised disabled:text-app-muted",
  ghost: "text-app-text hover:bg-app-soft/80 hover:text-app-text",
  outline: "border border-app-border text-app-text hover:border-app-muted hover:bg-app-surface",
  danger: "text-expense hover:bg-rose-500/10",
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
