import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function Field({
  label,
  error,
  children,
  className,
}: {
  label: string;
  error?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: o controle chega via children
    <label className={cn("flex flex-col gap-1.5", className)}>
      <span className="text-sm text-zinc-400">{label}</span>
      {children}
      {error ? <span className="text-xs text-rose-400">{error}</span> : null}
    </label>
  );
}

const controlCls =
  "h-11 w-full rounded-(--radius-control) border border-zinc-700/80 bg-zinc-900 px-3 text-[15px] text-zinc-100 placeholder:text-zinc-500 focus:border-emerald-500 focus:outline-none";

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn(controlCls, className)} {...props} />;
}

export function Select({ className, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cn(controlCls, "appearance-none", className)} {...props} />;
}
