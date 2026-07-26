import { Rows, SquaresFour, Table } from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";
import { useI18n } from "@/lib/i18n";
import { viewModeLabelKey, viewModeShortLabelKey, type ViewMode } from "@/lib/view-mode";

export type ViewModeToggleProps = {
  readonly value: ViewMode;
  readonly onChange: (mode: ViewMode) => void;
  readonly className?: string;
};

const MODES: ReadonlyArray<{ mode: ViewMode; Icon: Icon }> = [
  { mode: "grid", Icon: SquaresFour },
  { mode: "compact", Icon: Rows },
  { mode: "table", Icon: Table },
];

function modeButtonClass(active: boolean): string {
  return cn(
    "inline-flex h-8 items-center gap-1.5 rounded-none px-2.5 text-[12px] font-medium transition-[background-color,color,transform] duration-150 ease-out",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring focus-visible:ring-offset-1 focus-visible:ring-offset-paper-0",
    "active:scale-[0.98]",
    active
      ? "bg-paper-1 text-ink shadow-[2px_2px_0_var(--border)] ring-1 ring-border"
      : "text-ink-muted hover:bg-paper-1/70 hover:text-ink",
  );
}

export function ViewModeToggle({ value, onChange, className }: ViewModeToggleProps) {
  const { t } = useI18n();
  return (
    <div
      role="group"
      aria-label={t("viewmode.switch")}
      className={cn(
        "inline-flex items-center gap-0.5 rounded-none border-2 border-border bg-paper-0 p-0.5",
        className,
      )}
    >
      {MODES.map(({ mode, Icon }) => {
        const active = value === mode;
        const label = t(viewModeLabelKey(mode));
        return (
          <button
            key={mode}
            type="button"
            title={label}
            aria-label={label}
            aria-pressed={active}
            className={modeButtonClass(active)}
            onClick={() => onChange(mode)}
          >
            <Icon size={16} weight={active ? "fill" : "regular"} aria-hidden />
            <span className="hidden sm:inline">{t(viewModeShortLabelKey(mode))}</span>
          </button>
        );
      })}
    </div>
  );
}

export function viewModeGridClass(mode: ViewMode): string {
  switch (mode) {
    case "grid":
      return "grid gap-3 sm:grid-cols-2 xl:grid-cols-3";
    case "compact":
      return "grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4";
    case "table":
      return "";
    default:
      return assertNever(mode);
  }
}
