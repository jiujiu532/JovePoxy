import { cn } from "@/lib/cn";

export type ProgressRingProps = {
  readonly percent: number;
  readonly label: string;
  readonly valueText: string;
  readonly hint?: string;
  readonly size?: number;
  readonly strokeWidth?: number;
  readonly className?: string;
};

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, value));
}

function ringTone(percent: number): string {
  if (percent >= 90) return "var(--status-error)";
  if (percent >= 70) return "var(--status-warning)";
  if (percent <= 0.05) return "var(--status-success)";
  return "var(--accent)";
}

export function ProgressRing({
  percent,
  label,
  valueText,
  hint,
  size = 88,
  strokeWidth = 7,
  className,
}: ProgressRingProps) {
  const p = clampPercent(percent);
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - p / 100);
  const tone = ringTone(p);

  return (
    <div className={cn("flex flex-col items-center gap-1.5", className)}>
      <div className="relative" style={{ width: size, height: size }}>
        <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden>
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke="var(--border)"
            strokeWidth={strokeWidth}
          />
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke={tone}
            strokeWidth={strokeWidth}
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            transform={`rotate(-90 ${size / 2} ${size / 2})`}
            className="transition-[stroke-dashoffset] duration-300 ease-out"
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center px-1 text-center">
          <span className="text-[15px] font-semibold tabular-nums leading-none text-ink">
            {valueText}
          </span>
        </div>
      </div>
      <div className="text-center">
        <p className="text-caption font-medium text-ink-muted">{label}</p>
        {hint ? <p className="mt-0.5 text-[11px] tabular-nums text-ink-faint">{hint}</p> : null}
      </div>
    </div>
  );
}

export type ProgressBarProps = {
  readonly percent: number;
  readonly label: string;
  readonly valueText: string;
  readonly hint?: string;
  readonly dense?: boolean;
  readonly className?: string;
};

export function ProgressBar({
  percent,
  label,
  valueText,
  hint,
  dense = false,
  className,
}: ProgressBarProps) {
  const p = clampPercent(percent);
  const tone = ringTone(p);

  return (
    <div className={cn("min-w-0", className)}>
      <div className="flex items-baseline justify-between gap-2">
        <span className={cn("font-medium text-ink-muted", dense ? "text-[11px]" : "text-caption")}>
          {label}
        </span>
        <span
          className={cn(
            "shrink-0 font-medium tabular-nums text-ink",
            dense ? "text-[11px]" : "text-[12px]",
          )}
        >
          {valueText}
        </span>
      </div>
      <div
        className={cn(
          "mt-1 overflow-hidden rounded-none border border-border bg-paper-0",
          dense ? "h-1" : "h-1.5",
        )}
      >
        <div
          className="h-full rounded-none transition-[width] duration-300 ease-out"
          style={{ width: `${p}%`, backgroundColor: tone }}
        />
      </div>
      {hint ? (
        <p className={cn("mt-1 tabular-nums text-ink-faint", dense ? "text-[10px]" : "text-[11px]")}>
          {hint}
        </p>
      ) : null}
    </div>
  );
}
