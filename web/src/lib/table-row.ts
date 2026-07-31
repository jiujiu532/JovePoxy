import { cn } from "@/lib/cn";

/** Shared list-row tone for enabled vs disabled pool/key/proxy items. */
export function tableRowClass(disabled: boolean, className?: string): string {
  return cn(
    "border-b border-border last:border-b-0 transition-[background-color,opacity,color] duration-150",
    disabled
      ? "bg-paper-2/40 text-ink-faint opacity-55 hover:bg-paper-2/40"
      : "hover:bg-paper-2/40",
    className,
  );
}
