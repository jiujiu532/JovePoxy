import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

export type ButtonProps = {
  readonly variant?: ButtonVariant;
  readonly size?: ButtonSize;
  readonly loading?: boolean;
  readonly children: ReactNode;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children">;

function variantClass(variant: ButtonVariant): string {
  switch (variant) {
    case "primary":
      return "bg-accent text-accent-fg border-2 border-border shadow-[var(--shadow-hard)] hover:bg-accent-hover";
    case "secondary":
      return "bg-paper-1 text-ink border-2 border-border shadow-[var(--shadow-hard)] hover:bg-accent-yellow hover:text-black";
    case "ghost":
      return "bg-transparent text-ink border-2 border-transparent hover:border-border hover:bg-paper-1";
    case "danger":
      return "bg-status-error text-accent-fg border-2 border-border shadow-[var(--shadow-hard)] hover:opacity-90";
    default:
      return assertNever(variant);
  }
}

function sizeClass(size: ButtonSize): string {
  switch (size) {
    case "sm":
      return "min-h-9 h-9 px-3 text-[13px]";
    case "md":
      return "min-h-11 h-11 px-5 text-sm";
    case "lg":
      return "min-h-12 h-12 px-6 text-sm";
    default:
      return assertNever(size);
  }
}

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  disabled,
  className,
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  const isDisabled = Boolean(disabled || loading);
  return (
    <button
      type={type}
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-none font-semibold whitespace-nowrap",
        "transition-[transform,box-shadow,background-color,border-color,color,opacity] duration-200",
        "[transition-timing-function:var(--ease-toy-spring)]",
        "hover:scale-[1.02] active:translate-x-[4px] active:translate-y-[4px] active:scale-95 active:shadow-none",
        "disabled:pointer-events-none disabled:opacity-50 disabled:hover:scale-100",
        variantClass(variant),
        sizeClass(size),
        className,
      )}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className="opacity-80">处理中</span> : children}
    </button>
  );
}
