import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-(--radius-card) border border-app-border/80 bg-app-surface/60 backdrop-blur-sm",
        className,
      )}
      {...props}
    />
  );
}

export function CardHeader({
  title,
  action,
  className,
}: {
  title: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center justify-between px-5 pt-4 pb-1", className)}>
      <h2 className="text-sm font-medium text-app-muted">{title}</h2>
      {action}
    </div>
  );
}
