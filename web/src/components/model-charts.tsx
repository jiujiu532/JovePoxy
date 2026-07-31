import { useCallback, useState } from "react";
import {
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Sector,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { cn } from "@/lib/cn";

/** Playful hard-edge palette — enough stops for full-series legends; dark-safe brights. */
export const MODEL_COLORS = [
  "var(--accent)",
  "var(--accent-teal)",
  "var(--accent-yellow)",
  "var(--accent-coral)",
  "var(--accent-mint)",
  "#7dd3fc",
  "#86efac",
  "#fbbf24",
  "#c4b5fd",
  "#f9a8d4",
  "#67e8f9",
  "#fdba74",
  "#a5b4fc",
  "#fca5a5",
  "#5eead4",
  "#fde68a",
] as const;

export function modelColor(index: number): string {
  return MODEL_COLORS[index % MODEL_COLORS.length]!;
}

export type ModelSeriesPoint = {
  readonly day: string;
  readonly total: number;
  readonly [model: string]: string | number;
};

export type ModelShareSlice = {
  readonly model: string;
  readonly value: number;
  readonly color: string;
};

export type ModelRankItem = {
  readonly model: string;
  readonly value: number;
  readonly color: string;
};

function formatCompact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}

/** 参考风格浮层：细硬边、白底、无阴影拖尾。 */
function RefTooltip({
  active,
  label,
  children,
}: {
  readonly active?: boolean;
  readonly label?: string;
  readonly children: React.ReactNode;
}) {
  if (!active) return null;
  return (
    <div className="pointer-events-none border-2 border-border bg-paper-0 px-3 py-2">
      {label ? (
        <p className="mb-1.5 font-mono text-[12px] font-semibold text-ink">{label}</p>
      ) : null}
      {children}
    </div>
  );
}

/** 多序列调用趋势：智能 Top-5 + 幽灵虚线 + 交互聚焦。 */
export function ModelCallTrendChart({
  data,
  models,
  colors,
  height = 260,
  className,
  empty,
}: {
  readonly data: ReadonlyArray<ModelSeriesPoint>;
  readonly models: ReadonlyArray<string>;
  readonly colors: ReadonlyArray<string>;
  readonly height?: number;
  readonly className?: string;
  readonly totalLabel: string;
  readonly empty?: boolean;
}) {
  const [activeModel, setActiveModel] = useState<string | null>(null);

  if (empty || data.length === 0 || models.length === 0) return null;

  // Extra room for multi-line chip legend when many models
  const legendExtra = Math.min(100, Math.max(20, Math.ceil(models.length / 3) * 24));
  const chartHeight = height + legendExtra;

  return (
    <div className={cn("w-full", className)} style={{ height: chartHeight }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={[...data]}
          margin={{ top: 10, right: 14, left: 0, bottom: 8 }}
          onMouseLeave={() => setActiveModel(null)}
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.28}
            vertical={false}
          />
          <XAxis
            dataKey="day"
            interval="preserveStartEnd"
            minTickGap={28}
            tick={{
              fill: "var(--ink-muted)",
              fontSize: 11,
              fontFamily: "var(--font-mono)",
            }}
            axisLine={{ stroke: "var(--border)", strokeWidth: 1.5 }}
            tickLine={false}
            dy={6}
          />
          <YAxis
            allowDecimals={false}
            width={36}
            tick={{
              fill: "var(--ink-muted)",
              fontSize: 10,
              fontFamily: "var(--font-mono)",
            }}
            axisLine={false}
            tickLine={false}
            tickFormatter={formatCompact}
          />
          <Tooltip
            isAnimationActive={false}
            animationDuration={0}
            cursor={{
              stroke: "var(--border)",
              strokeWidth: 1,
              strokeDasharray: "3 3",
            }}
            content={({ active, label, payload }) => {
              const rows = (payload ?? [])
                .filter((p) => p.dataKey !== "total" && typeof p.value === "number")
                .map((p) => ({
                  name: String(p.name ?? p.dataKey),
                  value: Number(p.value),
                  color: String(p.color ?? "var(--ink)"),
                }))
                .filter((r) => r.value > 0)
                .sort((a, b) => b.value - a.value);
              if (!active || rows.length === 0) return null;
              return (
                <RefTooltip active label={String(label ?? "")}>
                  <ul className="flex flex-col gap-1">
                    {rows.map((r) => (
                      <li
                        key={r.name}
                        className="flex items-center gap-2 text-[12px] text-ink"
                      >
                        <span
                          className="inline-block h-2 w-2 shrink-0 rounded-full"
                          style={{ backgroundColor: r.color }}
                          aria-hidden
                        />
                        <span className="min-w-0 truncate font-mono text-[11px] text-ink-muted">
                          {r.name}
                        </span>
                        <span className="ml-auto shrink-0 font-mono font-semibold tabular-nums">
                          {formatCompact(r.value)}
                        </span>
                      </li>
                    ))}
                  </ul>
                </RefTooltip>
              );
            }}
          />
          <Legend
            verticalAlign="bottom"
            content={({ payload }) => (
              <div className="flex flex-wrap items-center justify-center gap-1.5 pt-3">
                {payload?.map((entry) => {
                  const m = String(entry.value);
                  const i = models.indexOf(m);
                  const isTop5 = i >= 0 && i < 5;
                  const isActive = activeModel === m;
                  const isDimmed = activeModel !== null && !isActive;
                  return (
                    <button
                      key={m}
                      type="button"
                      onMouseEnter={() => setActiveModel(m)}
                      onMouseLeave={() => setActiveModel(null)}
                      className={cn(
                        "inline-flex items-center gap-1.5 border px-2 py-0.5 font-mono text-[11px] transition-all cursor-pointer",
                        isTop5
                          ? "font-semibold border-border bg-paper-0 shadow-[1px_1px_0_var(--border)]"
                          : "border-border/40 bg-paper-1 text-ink-muted",
                        isActive &&
                          "border-border bg-accent-yellow text-black font-bold scale-105 shadow-[2px_2px_0_var(--border)]",
                        isDimmed && "opacity-35 border-transparent shadow-none",
                      )}
                    >
                      <span
                        className="h-2 w-2 rounded-full shrink-0"
                        style={{
                          backgroundColor: String(entry.color),
                          opacity: isTop5 || isActive ? 1 : 0.6,
                        }}
                      />
                      <span className="truncate max-w-[130px]">{m}</span>
                      {isTop5 ? (
                        <span className="text-[9px] font-extrabold uppercase text-ink-faint">
                          #{i + 1}
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            )}
          />
          {models.map((m, i) => {
            const color = colors[i] ?? modelColor(i);
            const isTop5 = i < 5;
            const isActive = activeModel === m;

            let strokeWidth = 2;
            let strokeOpacity = 1;
            let strokeDasharray: string | undefined = undefined;

            if (activeModel !== null) {
              if (isActive) {
                strokeWidth = 3.5;
                strokeOpacity = 1;
              } else {
                strokeWidth = 1;
                strokeOpacity = 0.12;
                if (!isTop5) strokeDasharray = "3 3";
              }
            } else {
              if (isTop5) {
                strokeWidth = 2.5;
                strokeOpacity = 1;
              } else {
                strokeWidth = 1.5;
                strokeOpacity = 0.35;
                strokeDasharray = "3 3";
              }
            }

            return (
              <Line
                key={m}
                type="monotone"
                dataKey={m}
                name={m}
                stroke={color}
                strokeWidth={strokeWidth}
                strokeOpacity={strokeOpacity}
                strokeDasharray={strokeDasharray}
                dot={
                  isActive
                    ? { r: 4, strokeWidth: 0, fill: color }
                    : isTop5 && activeModel === null
                      ? { r: 2.5, strokeWidth: 0, fill: color }
                      : false
                }
                activeDot={{
                  r: 5,
                  strokeWidth: 2,
                  stroke: color,
                  fill: "var(--paper-0)",
                }}
                isAnimationActive={false}
              />
            );
          })}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

type ActiveShapeProps = {
  readonly cx?: number;
  readonly cy?: number;
  readonly innerRadius?: number;
  readonly outerRadius?: number;
  readonly startAngle?: number;
  readonly endAngle?: number;
  readonly fill?: string;
  readonly midAngle?: number;
  readonly percent?: number;
  readonly value?: number;
  readonly name?: string;
};

/** 悬停：扇区轻微外扩（参考图「放大一点」），不挡中心。 */
function DonutActiveShape(props: unknown) {
  const p = props as ActiveShapeProps;
  const cx = p.cx ?? 0;
  const cy = p.cy ?? 0;
  const innerRadius = p.innerRadius ?? 0;
  const outerRadius = p.outerRadius ?? 0;
  const startAngle = p.startAngle ?? 0;
  const endAngle = p.endAngle ?? 0;
  const fill = p.fill ?? "var(--accent)";
  return (
    <g>
      <Sector
        cx={cx}
        cy={cy}
        innerRadius={Math.max(0, innerRadius - 2)}
        outerRadius={outerRadius + 8}
        startAngle={startAngle}
        endAngle={endAngle}
        fill={fill}
        stroke="var(--border)"
        strokeWidth={2}
      />
    </g>
  );
}

/** 占比环：Smart Top-5 聚合 + 中心悬停联动 + 上浮尾部展开。 */
export function ModelShareDonut({
  slices,
  height = 280,
  className,
  centerLabel,
  centerValue,
  callsUnit,
  otherLabel = "其他",
}: {
  readonly slices: ReadonlyArray<ModelShareSlice>;
  readonly height?: number;
  readonly className?: string;
  readonly centerLabel: string;
  readonly centerValue: string;
  readonly callsUnit: string;
  readonly otherLabel?: string;
}) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const onEnter = useCallback((_: unknown, index: number) => {
    setActiveIndex(index);
  }, []);
  const onLeave = useCallback(() => setActiveIndex(null), []);

  if (slices.length === 0) return null;

  const total = slices.reduce((s, x) => s + x.value, 0);

  // 超过 5 个模型时：保留 Top 5 独立扇区，尾部模型聚合为「其他 (N个模型)」
  const topSlices = slices.slice(0, 5);
  const tailSlices = slices.slice(5);

  const displaySlices: Array<ModelShareSlice & { readonly isTailAggregate?: boolean }> =
    tailSlices.length > 0
      ? [
          ...topSlices,
          {
            model: `${otherLabel} (${tailSlices.length})`,
            value: tailSlices.reduce((s, x) => s + x.value, 0),
            color: "#cbd5e1",
            isTailAggregate: true,
          },
        ]
      : [...slices];

  const activeSlice = activeIndex != null ? displaySlices[activeIndex] : null;
  const isTailActive = activeSlice?.isTailAggregate ?? false;

  return (
    <div
      className={cn("relative w-full flex flex-col justify-between", className)}
      style={{ height }}
    >
      <div className="relative flex-1 w-full min-h-[180px]">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={displaySlices}
              dataKey="value"
              nameKey="model"
              cx="50%"
              cy="50%"
              innerRadius="58%"
              outerRadius="78%"
              paddingAngle={2}
              stroke="var(--border)"
              strokeWidth={2}
              isAnimationActive={false}
              {...(activeIndex != null ? { activeIndex } : {})}
              activeShape={DonutActiveShape}
              onMouseEnter={onEnter}
              onMouseLeave={onLeave}
            >
              {displaySlices.map((s) => (
                <Cell key={s.model} fill={s.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>

        {/* 中心信息联动（替代阻挡文字的浮动 Tooltip） */}
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center p-4 text-center">
          {activeSlice ? (
            <>
              <span className="flex items-center justify-center gap-1 max-w-[130px] truncate text-[11px] font-bold text-ink">
                <span
                  className="h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: activeSlice.color }}
                />
                <span className="truncate">{activeSlice.model}</span>
              </span>
              <span className="mt-0.5 font-mono text-[1.3rem] font-extrabold tabular-nums leading-none text-ink">
                {formatCompact(activeSlice.value)}{" "}
                <span className="text-[11px] font-normal text-ink-muted">
                  {callsUnit}
                </span>
              </span>
              <span className="mt-1 font-mono text-[11px] font-bold text-accent-coral">
                {total > 0
                  ? `${((activeSlice.value / total) * 100).toFixed(1)}%`
                  : "0%"}
              </span>
            </>
          ) : (
            <>
              <span className="text-[10px] font-bold uppercase tracking-wider text-ink-muted">
                {centerLabel}
              </span>
              <span className="mt-0.5 font-mono text-[1.4rem] font-extrabold tabular-nums leading-none text-ink">
                {centerValue}
              </span>
            </>
          )}
        </div>
      </div>

      {/* 尾部模型展开浮层 (上浮避开卡片边缘裁切) */}
      {isTailActive && tailSlices.length > 0 ? (
        <div
          className="absolute bottom-12 right-2 z-40 w-64 border-2 border-border bg-paper-0 p-2.5 shadow-[4px_4px_0_var(--border)]"
          onMouseEnter={() => {
            const tailIdx = displaySlices.findIndex((s) => s.isTailAggregate);
            if (tailIdx >= 0) setActiveIndex(tailIdx);
          }}
          onMouseLeave={() => setActiveIndex(null)}
        >
          <div className="flex items-center justify-between border-b-2 border-border pb-1.5 font-mono text-[11px] font-bold text-ink">
            <span>包含 {tailSlices.length} 个尾部模型</span>
            <span className="text-ink-muted tabular-nums">
              {formatCompact(tailSlices.reduce((s, x) => s + x.value, 0))} {callsUnit}
            </span>
          </div>
          <ul className="mt-2 max-h-36 overflow-y-auto pr-1 font-mono text-[11px] flex flex-col gap-1">
            {tailSlices.map((ts) => {
              const tsPct = total > 0 ? ((ts.value / total) * 100).toFixed(1) : "0";
              return (
                <li key={ts.model} className="flex items-center justify-between gap-2 text-ink">
                  <span className="truncate text-ink-muted max-w-[130px]" title={ts.model}>
                    {ts.model}
                  </span>
                  <span className="font-bold tabular-nums">
                    {formatCompact(ts.value)}{" "}
                    <span className="text-[10px] text-ink-faint">({tsPct}%)</span>
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}

      {/* 底部精简图例 */}
      <div className="flex flex-wrap items-center justify-center gap-1.5 pt-2 border-t-2 border-border/20">
        {displaySlices.map((s, idx) => {
          const isTop5 = idx < 5 && !s.isTailAggregate;
          const pct = total > 0 ? Math.round((s.value / total) * 100) : 0;
          const isActive = activeIndex === idx;
          return (
            <button
              key={s.model}
              type="button"
              onMouseEnter={() => setActiveIndex(idx)}
              onMouseLeave={() => setActiveIndex(null)}
              className={cn(
                "inline-flex items-center gap-1.5 border px-2 py-0.5 font-mono text-[11px] transition-all cursor-pointer",
                isTop5
                  ? "font-semibold border-border bg-paper-0 shadow-[1px_1px_0_var(--border)]"
                  : "border-border/40 bg-paper-1 text-ink-muted",
                isActive &&
                  "border-border bg-accent-yellow text-black font-bold scale-105 shadow-[2px_2px_0_var(--border)]",
              )}
            >
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: s.color }}
              />
              <span className="truncate max-w-[110px]">{s.model}</span>
              <span className="font-bold tabular-nums text-ink-faint">{pct}%</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** 模型调用排行：高密度 Neo-Brutalist 交互列表。 */
export function ModelRankBars({
  items,
  height = 280,
  className,
  unitLabel,
}: {
  readonly items: ReadonlyArray<ModelRankItem>;
  readonly height?: number;
  readonly className?: string;
  readonly unitLabel: string;
}) {
  if (items.length === 0) return null;

  const total = items.reduce((s, x) => s + x.value, 0);
  const maxVal = Math.max(1, ...items.map((i) => i.value));

  return (
    <div
      className={cn(
        "w-full flex flex-col gap-1.5 overflow-y-auto pr-1 font-mono text-[12px]",
        className,
      )}
      style={{ maxHeight: height }}
    >
      {items.map((item, index) => {
        const rankNum = index + 1;
        const isTop5 = index < 5;
        const pct = total > 0 ? ((item.value / total) * 100).toFixed(1) : "0";
        const fillPct = Math.max(4, Math.round((item.value / maxVal) * 100));

        return (
          <div
            key={item.model}
            className={cn(
              "group flex items-center gap-2 border-2 border-border p-1.5 transition-all hover:bg-paper-2 hover:shadow-[2px_2px_0_var(--border)]",
              isTop5 ? "bg-paper-0" : "bg-paper-1/60 opacity-90",
            )}
            title={`${item.model}: ${item.value} ${unitLabel} (${pct}%)`}
          >
            {/* Rank badge */}
            <span
              className={cn(
                "inline-flex h-5 w-5 shrink-0 items-center justify-center border font-mono text-[10px] font-extrabold",
                isTop5
                  ? "border-border bg-accent-yellow text-black shadow-[1px_1px_0_var(--border)]"
                  : "border-border/40 bg-paper-2 text-ink-muted",
              )}
            >
              #{rankNum}
            </span>

            {/* Model Name */}
            <span
              className="w-32 shrink-0 truncate font-mono text-[11px] font-semibold text-ink"
              title={item.model}
            >
              {item.model}
            </span>

            {/* Progress track */}
            <div className="relative flex-1 h-3.5 border-1.5 border-border bg-paper-2 overflow-hidden">
              <div
                className="h-full transition-all duration-300"
                style={{
                  width: `${fillPct}%`,
                  backgroundColor: isTop5 ? item.color : "var(--accent-mint)",
                  opacity: isTop5 ? 1 : 0.65,
                }}
              />
            </div>

            {/* Value & Percentage */}
            <div className="shrink-0 font-mono text-[11px] tabular-nums text-right min-w-[76px]">
              <span className="font-bold text-ink">{formatCompact(item.value)}</span>
              <span className="ml-1 text-[10px] text-ink-muted">({pct}%)</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
