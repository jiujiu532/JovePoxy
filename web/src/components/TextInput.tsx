import type { InputHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

export type TextInputProps = {
  readonly label: string;
  readonly hint?: string;
  readonly error?: string;
  readonly trailing?: ReactNode;
} & Omit<InputHTMLAttributes<HTMLInputElement>, "className"> & {
    readonly className?: string;
    readonly inputClassName?: string;
  };

export function TextInput({
  label,
  hint,
  error,
  trailing,
  id,
  className,
  inputClassName,
  disabled,
  ...rest
}: TextInputProps) {
  const inputId = id ?? rest.name ?? label;
  const describedBy = error
    ? `${inputId}-error`
    : hint
      ? `${inputId}-hint`
      : undefined;

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <label htmlFor={inputId} className="text-caption font-semibold text-ink">
        {label}
      </label>
      <div className="relative">
        <input
          id={inputId}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn(
            "h-11 w-full rounded-none border-2 bg-paper-1 px-3 text-sm text-ink",
            "placeholder:text-ink-faint transition-[border-color,box-shadow] duration-150",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
            error
              ? "border-status-error"
              : "border-border hover:border-border-strong",
            disabled && "cursor-not-allowed bg-paper-0 opacity-60",
            trailing && "pr-11",
            inputClassName,
          )}
          {...rest}
        />
        {trailing ? (
          <div className="absolute inset-y-0 right-1 flex items-center">
            {trailing}
          </div>
        ) : null}
      </div>
      {error ? (
        <p id={`${inputId}-error`} className="text-caption text-status-error" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p id={`${inputId}-hint`} className="text-caption text-ink-muted">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
