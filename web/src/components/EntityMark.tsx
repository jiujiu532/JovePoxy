import { cn } from "@/lib/cn";
import { familyInitials, familyTone } from "@/lib/family-tone";

export type EntityMarkProps = {
  readonly name: string;
  readonly className?: string;
  readonly size?: "sm" | "md";
};

/** Hard-edged monogram chip for list row identity. */
export function EntityMark({ name, className, size = "md" }: EntityMarkProps) {
  const tone = familyTone(name);
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center border-2 border-border font-bold tracking-tight",
        "shadow-[2px_2px_0_var(--border)]",
        size === "sm" ? "h-7 w-7 text-[10px]" : "h-8 w-8 text-[11px]",
        tone.bg,
        tone.text,
        className,
      )}
      aria-hidden
    >
      {familyInitials(name)}
    </span>
  );
}
