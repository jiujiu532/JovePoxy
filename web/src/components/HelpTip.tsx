import { Question } from "@phosphor-icons/react";
import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/cn";

export type HelpTipProps = {
  readonly content: string;
  readonly className?: string;
  readonly label?: string;
};

type TipPos = { readonly top: number; readonly left: number };

/**
 * Compact "?" control. Tooltip is portaled to document.body with fixed
 * coordinates so overflow:hidden ancestors never clip it.
 */
export function HelpTip({ content, className, label = "说明" }: HelpTipProps) {
  const tipId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<TipPos>({ top: 0, left: 0 });

  useEffect(() => {
    if (!open || !triggerRef.current) return;

    function place() {
      const el = triggerRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const margin = 8;
      const maxW = 288;
      let left = rect.left + rect.width / 2;
      left = Math.min(
        window.innerWidth - maxW / 2 - margin,
        Math.max(maxW / 2 + margin, left),
      );
      const spaceBelow = window.innerHeight - rect.bottom;
      const top = spaceBelow < 80 ? rect.top - margin : rect.bottom + margin;
      setPos({ top, left });
    }

    place();
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [open]);

  const tip =
    open && typeof document !== "undefined"
      ? createPortal(
          <span
            id={tipId}
            role="tooltip"
            className={cn(
              "pointer-events-none fixed z-[200] w-max max-w-[18rem] -translate-x-1/2",
              "rounded-none border-2 border-border px-2.5 py-2 text-left text-[11px] leading-relaxed",
              "bg-[var(--ink)] text-[var(--paper-0)] shadow-[3px_3px_0_var(--border)]",
            )}
            style={{
              top: pos.top,
              left: pos.left,
              transform:
                pos.top < (triggerRef.current?.getBoundingClientRect().top ?? 0)
                  ? "translate(-50%, -100%)"
                  : "translateX(-50%)",
            }}
          >
            {content}
          </span>,
          document.body,
        )
      : null;

  return (
    <span className={cn("relative inline-flex align-middle", className)}>
      <button
        ref={triggerRef}
        type="button"
        className="inline-flex h-5 w-5 items-center justify-center rounded-none border-2 border-border bg-paper-0 text-ink-faint transition-colors hover:bg-paper-1 hover:text-ink-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--focus-ring)]"
        aria-label={label}
        aria-describedby={open ? tipId : undefined}
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
      >
        <Question size={12} weight="bold" />
      </button>
      {tip}
    </span>
  );
}
