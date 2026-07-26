import { useId } from "react";
import { cn } from "@/lib/cn";

/**
 * Neo-Brutalist hard-edge SVG charts (no external chart lib).
 * 视觉规则：直角、黑硬边、纯色填充、无渐变/无模糊阴影。
 */

export type TrendPoint = {
  readonly label: string;
  readonly value: number;
};

/** 折线 + 纯色面积。用于请求/token 趋势。 */
export function HardLineChart({
  points,
  height = 132,
  stroke = "var(--accent)",
  fill = "var(--accent-soft)",
  className,
  ariaLabel,
}: {
  readonly points: ReadonlyArray<TrendPoint>;
  readonly height?: number;
  readonly stroke?: string;
  readonly fill?: string;
  readonly className?: string;
  readonly ariaLabel: string;
}) {
  const uid = useId();
  if (points.length === 0) return null;

  const w = 100;
  const h = 40;
  const max = Math.max(1, ...points.map((p) => p.value));
  const stepX = points.length > 1 ? w / (points.length - 1) : w;
  const coords = points.map((p, i) => ({
    x: points.length > 1 ? i * stepX : w / 2,
    y: h - (p.value / max) * (h - 4) - 2,
  }));
  const line = coords.map((c) => `${c.x.toFixed(2)},${c.y.toFixed(2)}`).join(" ");
  const area = `0,${h} ${line} ${w},${h}`;

  const first = points[0];
  const last = points[points.length - 1];
  const peak = points.reduce((a, b) => (b.value > a.value ? b : a), first ?? { label: "", value: 0 });

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <svg
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        style={{ height }}
        className="block w-full border-2 border-border bg-paper-0"
        aria-hidden
      >
        {/* 网格：硬细线 */}
        {[1, 2, 3].map((i) => (
          <line
            key={i}
            x1={0}
            x2={w}
            y1={(h / 4) * i}
            y2={(h / 4) * i}
            stroke="var(--border)"
            strokeOpacity={0.14}
            strokeWidth={0.4}
          />
        ))}
        <polygon points={area} fill={fill} />
        <polyline
          points={line}
          fill="none"
          stroke={stroke}
          strokeWidth={1.6}
          strokeLinejoin="miter"
          strokeLinecap="square"
        />
        {/* 峰值点：方形标记（非圆点） */}
        {coords.map((c, i) =>
          points[i] === peak && peak.value > 0 ? (
            <rect
              key={`${uid}-pk`}
              x={c.x - 1.4}
              y={c.y - 1.4}
              width={2.8}
              height={2.8}
              fill="var(--ink)"
            />
          ) : null,
        )}
      </svg>
      <figcaption className="mt-1.5 flex items-center justify-between text-[11px] tabular-nums text-ink-faint">
        <span>{first?.label}</span>
        {peak.value > 0 ? (
          <span className="font-medium text-ink-muted">峰值 {peak.value}</span>
        ) : null}
        <span>{last?.label}</span>
      </figcaption>
    </figure>
  );
}

/** 竖向硬边柱状图。用于按日 token。 */
export function HardBarChart({
  points,
  height = 132,
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
  if (points.length === 0) return null;
  const max = Math.max(1, ...points.map((p) => p.value));

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <div
        className="flex items-end gap-1.5 border-2 border-border bg-paper-0 px-2 pt-2"
        style={{ height }}
      >
        {points.map((p) => {
          const ratio = p.value / max;
          return (
            <div
              key={p.label}
              className="group relative flex min-w-0 flex-1 flex-col justify-end self-stretch"
              title={`${p.label}: ${formatValue(p.value)}`}
            >
              <div
                className="w-full border-2 border-b-0 border-border transition-[background-color] duration-150 group-hover:bg-ink"
                style={{
                  height: `${Math.max(ratio * 100, p.value > 0 ? 6 : 2)}%`,
                  backgroundColor: p.value > 0 ? barFill : "var(--paper-0)",
                }}
              />
            </div>
          );
        })}
      </div>
      <div className="mt-1.5 flex justify-between text-[11px] tabular-nums text-ink-faint">
        <span>{points[0]?.label}</span>
        <span>{points[points.length - 1]?.label}</span>
      </div>
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

  return (
    <figure className={cn("m-0", className)} role="img" aria-label={ariaLabel}>
      <div className="flex h-7 w-full overflow-hidden border-2 border-border bg-paper-0">
        {total === 0 ? (
          <div className="flex w-full items-center justify-center text-[11px] text-ink-faint">
            暂无请求
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
