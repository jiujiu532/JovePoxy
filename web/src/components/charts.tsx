import { useId } from "react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/cn";

/**
 * Neo-Brutalist hard-edge charts (no external chart lib).
 * 直角、黑硬边、纯色；Y 轴刻度 + 逐日 X 标签 + 峰值标注。
 */

export type TrendPoint = {
  readonly label: string;
  readonly value: number;
};

function niceMax(raw: number): number {
  if (raw <= 0) return 1;
  const exp = Math.pow(10, Math.floor(Math.log10(raw)));
  const n = raw / exp;
  const nice = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10;
  return nice * exp;
}

/** 折线 + 面积。 */
export function HardLineChart({
  points,
  height = 168,
  stroke = "var(--accent)",
  fill = "var(--accent-soft)",
  className,
  ariaLabel,
  formatValue = (v: number) => String(v),
}: {
  readonly points: ReadonlyArray<TrendPoint>;
  readonly height?: number;
  readonly stroke?: string;
  readonly fill?: string;
  readonly className?: string;
  readonly ariaLabel: string;
  readonly formatValue?: (v: number) => string;
}) {
  const uid = useId();
  const { t } = useI18n();
  if (points.length === 0) return null;

  const plotW = 100;
  const plotH = 40;
  const max = niceMax(Math.max(...points.map((p) => p.value)));
  const stepX = points.length > 1 ? plotW / (points.length - 1) : plotW;
  const coords = points.map((p, i) => ({
    x: points.length > 1 ? i * stepX : plotW / 2,
    y: plotH - (p.value / max) * (plotH - 4) - 2,
    value: p.value,
    label: p.label,
  }));
  const line = coords.map((c) => `${c.x.toFixed(2)},${c.y.toFixed(2)}`).join(" ");
  const area = `0,${plotH} ${line} ${plotW},${plotH}`;
  const peak = coords.reduce((a, b) => (b.value > a.value ? b : a), coords[0]!);
  const mid = max / 2;

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <div className="flex gap-2">
        <div
          className="flex w-9 shrink-0 flex-col justify-between py-0.5 text-right text-[10px] tabular-nums leading-none text-ink-faint"
          style={{ height }}
          aria-hidden
        >
          <span>{formatValue(max)}</span>
          <span>{formatValue(mid)}</span>
          <span>0</span>
        </div>
        <div className="min-w-0 flex-1">
          <svg
            viewBox={`0 0 ${plotW} ${plotH}`}
            preserveAspectRatio="none"
            style={{ height }}
            className="block w-full border-2 border-border bg-paper-0"
            aria-hidden
          >
            {[0, 1, 2, 3, 4].map((i) => (
              <line
                key={i}
                x1={0}
                x2={plotW}
                y1={(plotH / 4) * i}
                y2={(plotH / 4) * i}
                stroke="var(--border)"
                strokeOpacity={0.22}
                strokeWidth={0.35}
              />
            ))}
            <polygon points={area} fill={fill} />
            <polyline
              points={line}
              fill="none"
              stroke={stroke}
              strokeWidth={2}
              strokeLinejoin="miter"
              strokeLinecap="square"
              vectorEffect="non-scaling-stroke"
            />
            {coords.map((c, i) => (
              <rect
                key={`${uid}-pt-${i}`}
                x={c.x - 1.1}
                y={c.y - 1.1}
                width={2.2}
                height={2.2}
                fill={c === peak && peak.value > 0 ? "var(--ink)" : stroke}
                stroke="var(--paper-0)"
                strokeWidth={0.4}
              />
            ))}
          </svg>
          <div
            className="mt-1.5 grid text-[10px] tabular-nums text-ink-faint"
            style={{
              gridTemplateColumns: `repeat(${points.length}, minmax(0, 1fr))`,
            }}
          >
            {points.map((p) => (
              <span key={p.label} className="truncate text-center">
                {p.label}
              </span>
            ))}
          </div>
        </div>
      </div>
      {peak.value > 0 ? (
        <figcaption className="mt-2 text-[11px] text-ink-muted">
          {t("charts.peak", { value: formatValue(peak.value) })}
          <span className="text-ink-faint"> · {peak.label}</span>
        </figcaption>
      ) : null}
    </figure>
  );
}

/** 竖向硬边柱状图。 */
export function HardBarChart({
  points,
  height = 168,
  barFill = "var(--accent-teal)",
  className,
  ariaLabel,
  formatValue = (v: number) => String(v),
}: {
  readonly points: ReadonlyArray<TrendPoint>;
  readonly height?: number;
  readonly barFill?: string;
  readonly className?: string;
  readonly ariaLabel: string;
  readonly formatValue?: (v: number) => string;
}) {
  const { t } = useI18n();
  if (points.length === 0) return null;

  const max = niceMax(Math.max(...points.map((p) => p.value)));
  const peak = points.reduce((a, b) => (b.value > a.value ? b : a), points[0]!);
  const mid = max / 2;
  const plotH = Math.max(96, height - 28);

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <div className="flex gap-2">
        <div
          className="flex w-9 shrink-0 flex-col justify-between py-0.5 text-right text-[10px] tabular-nums leading-none text-ink-faint"
          style={{ height: plotH }}
          aria-hidden
        >
          <span>{formatValue(max)}</span>
          <span>{formatValue(mid)}</span>
          <span>0</span>
        </div>
        <div className="min-w-0 flex-1">
          <div
            className="flex items-end gap-1.5 border-2 border-border bg-paper-0 px-2 pt-4"
            style={{ height: plotH }}
          >
            {points.map((p) => {
              const ratio = p.value / max;
              const isPeak = p === peak && p.value > 0;
              return (
                <div
                  key={p.label}
                  className="group relative flex min-w-0 flex-1 flex-col justify-end self-stretch"
                  title={`${p.label}: ${formatValue(p.value)}`}
                >
                  {isPeak ? (
                    <span className="pointer-events-none absolute -top-0.5 left-1/2 z-10 -translate-x-1/2 whitespace-nowrap text-[10px] font-semibold tabular-nums text-ink">
                      {formatValue(p.value)}
                    </span>
                  ) : null}
                  <div
                    className={cn(
                      "w-full border-2 border-b-0 border-border transition-[background-color] duration-150",
                      "group-hover:bg-ink",
                    )}
                    style={{
                      height: `${Math.max(ratio * 100, p.value > 0 ? 8 : 2)}%`,
                      backgroundColor: p.value > 0 ? barFill : "var(--paper-0)",
                    }}
                  />
                </div>
              );
            })}
          </div>
          <div
            className="mt-1.5 grid text-[10px] tabular-nums text-ink-faint"
            style={{
              gridTemplateColumns: `repeat(${points.length}, minmax(0, 1fr))`,
            }}
          >
            {points.map((p) => (
              <span key={p.label} className="truncate text-center">
                {p.label}
              </span>
            ))}
          </div>
        </div>
      </div>
      {peak.value > 0 ? (
        <figcaption className="mt-2 text-[11px] text-ink-muted">
          {t("charts.peak", { value: formatValue(peak.value) })}
          <span className="text-ink-faint"> · {peak.label}</span>
        </figcaption>
      ) : null}
    </figure>
  );
}

export type StackSegment = {
  readonly label: string;
  readonly value: number;
  readonly color: string;
};

/** 横向分段条：状态分布（2xx / 429 / 5xx）。 */
export function StatusStackBar({
  segments,
  className,
  ariaLabel,
}: {
  readonly segments: ReadonlyArray<StackSegment>;
  readonly className?: string;
  readonly ariaLabel: string;
}) {
  const total = segments.reduce((sum, s) => sum + s.value, 0);
  const { t } = useI18n();

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <div className="flex h-6 w-full overflow-hidden border-2 border-border bg-paper-0">
        {total === 0 ? (
          <div className="flex w-full items-center justify-center text-[11px] text-ink-faint">
            {t("charts.noRequests")}
          </div>
        ) : (
          segments
            .filter((s) => s.value > 0)
            .map((s, i) => (
              <div
                key={s.label}
                className={cn("h-full", i > 0 && "border-l-2 border-border")}
                style={{
                  width: `${(s.value / total) * 100}%`,
                  backgroundColor: s.color,
                  minWidth: 6,
                }}
                title={`${s.label}: ${s.value}`}
              />
            ))
        )}
      </div>
      <figcaption className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-ink-muted">
        {segments.map((s) => (
          <span key={s.label} className="inline-flex items-center gap-1.5">
            <span
              className="inline-block h-2.5 w-2.5 border-2 border-border"
              style={{ backgroundColor: s.color }}
              aria-hidden
            />
            {s.label}
            <span className="font-medium tabular-nums text-ink">{s.value}</span>
          </span>
        ))}
      </figcaption>
    </figure>
  );
}

/** 行内占比条：表格里的模型份额。 */
export function ShareBar({
  ratio,
  color = "var(--accent-yellow)",
  className,
}: {
  readonly ratio: number;
  readonly color?: string;
  readonly className?: string;
}) {
  const clamped = Math.min(1, Math.max(0, ratio));
  return (
    <div
      className={cn("h-2.5 w-full max-w-[9rem] border border-border bg-paper-0", className)}
      aria-hidden
    >
      <div
        className="h-full"
        style={{ width: `${clamped * 100}%`, backgroundColor: color }}
      />
    </div>
  );
}
