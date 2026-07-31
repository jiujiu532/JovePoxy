import { useCallback, useState } from "react";
import {
  Bar,
  BarChart,
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

/** Playful hard-edge palette — no AI purple. */
export const MODEL_COLORS = [
  "var(--accent)",
  "var(--accent-teal)",
  "var(--accent-yellow)",
  "var(--accent-coral)",
  "var(--accent-mint)",
  "#2d3436",
  "#00b894",
  "#e17055",
  "#0984e3",
  "#636e72",
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

function shortenModel(name: string, max = 18): string {
  if (name.length <= max) return name;
  return `${name.slice(0, max - 1)}…`;
}

type TipRow = {
  name: string;
  value: number;
  color: string;
  pct?: string | undefined;
};

function HardTooltip({
  active,
  label,
  rows,
  totalLabel,
  footerValue,
}: {
  readonly active?: boolean;
  readonly label?: string;
  readonly rows: ReadonlyArray<TipRow>;
  readonly totalLabel?: string;
  readonly footerValue?: string;
}) {
  if (!active || rows.length === 0) return null;
  const sum = rows.reduce((s, r) => s + r.value, 0);
  return (
    <div
      className={cn(
        "pointer-events-none min-w-[11.5rem] max-w-[18rem]",
        "border-2 border-border bg-paper-0 px-3 py-2",
        "shadow-[3px_3px_0_var(--border)]",
      )}
      style={{
        // 轻入场，不走 recharts 位移动画（避免拖尾）
        animation: "chart-tip-in 120ms ease-out",
      }}
    >
      <style>{`
        @keyframes chart-tip-in {
          from { opacity: 0; transform: translateY(4px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @media (prefers-reduced-motion: reduce) {
          @keyframes chart-tip-in {
            from, to { opacity: 1; transform: none; }
          }
        }
      `}</style>
      {label ? (
        <p className="mb-1.5 text-[11px] font-semibold tracking-wide text-ink-muted">
          {label}
        </p>
      ) : null}
      <ul className="flex flex-col gap-1">
        {rows.map((r) => (
          <li
            key={r.name}
            className="flex items-center justify-between gap-3 text-[12px] text-ink"
          >
            <span className="inline-flex min-w-0 items-center gap-1.5">
              <span
                className="inline-block h-2.5 w-2.5 shrink-0 border border-border"
                style={{ backgroundColor: r.color }}
                aria-hidden
              />
              <span className="truncate font-mono text-[11px]" title={r.name}>
                {r.name}
              </span>
            </span>
            <span className="shrink-0 font-mono font-semibold tabular-nums">
              {formatCompact(r.value)}
              {r.pct ? (
                <span className="ml-1.5 text-[10px] font-medium text-ink-muted">
                  {r.pct}
                </span>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
      {totalLabel != null ? (
        <p className="mt-1.5 border-t-2 border-border pt-1.5 text-[11px] text-ink-muted">
          {totalLabel}{" "}
          <span className="font-mono font-semibold tabular-nums text-ink">
            {footerValue ?? formatCompact(sum)}
          </span>
        </p>
      ) : null}
    </div>
  );
}

/** Multi-series call trend (one line per model + daily total in tooltip). */
export function ModelCallTrendChart({
  data,
  models,
  colors,
  height = 260,
  className,
  totalLabel,
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
  const [focusModel, setFocusModel] = useState<string | null>(null);

  if (empty || data.length === 0 || models.length === 0) return null;

  return (
    <div className={cn("w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={[...data]}
          margin={{ top: 10, right: 14, left: 0, bottom: 0 }}
          onMouseLeave={() => setFocusModel(null)}
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.16}
            vertical={false}
          />
          <XAxis
            dataKey="day"
            tick={{
              fill: "var(--ink-faint)",
              fontSize: 11,
              fontFamily: "var(--font-mono)",
            }}
            axisLine={{ stroke: "var(--border)", strokeWidth: 2 }}
            tickLine={false}
            dy={6}
          />
          <YAxis
            allowDecimals={false}
            width={36}
            tick={{
              fill: "var(--ink-faint)",
              fontSize: 10,
              fontFamily: "var(--font-mono)",
            }}
            axisLine={false}
            tickLine={false}
            tickFormatter={formatCompact}
          />
          <Tooltip
            // 关掉默认位移动画，改用 CSS 轻入，跟手不拖尾
            isAnimationActive={false}
            animationDuration={0}
            cursor={{
              stroke: "var(--ink)",
              strokeWidth: 1.25,
              strokeOpacity: 0.28,
            }}
            content={({ active, label, payload }) => {
              const rows: TipRow[] = (payload ?? [])
                .filter((p) => p.dataKey !== "total" && typeof p.value === "number")
                .map((p) => ({
                  name: String(p.name ?? p.dataKey),
                  value: Number(p.value),
                  color: String(p.color ?? "var(--ink)"),
                }))
                .filter((r) => r.value > 0)
                .sort((a, b) => b.value - a.value);
              const sum = rows.reduce((s, r) => s + r.value, 0);
              const withPct: TipRow[] = rows.map((r) => {
                if (sum <= 0) return r;
                return { ...r, pct: `${((r.value / sum) * 100).toFixed(0)}%` };
              });
              return (
                <HardTooltip
                  {...(active !== undefined ? { active } : {})}
                  label={String(label ?? "")}
                  rows={withPct}
                  totalLabel={totalLabel}
                />
              );
            }}
          />
          <Legend
            verticalAlign="bottom"
            height={40}
            iconType="plainline"
            wrapperStyle={{ fontSize: 11, paddingTop: 10 }}
            onMouseEnter={(e) => {
              const id = String(e?.dataKey ?? e?.value ?? "");
              if (id) setFocusModel(id);
            }}
            onMouseLeave={() => setFocusModel(null)}
            formatter={(value) => (
              <span
                className={cn(
                  "font-mono text-[11px] transition-opacity duration-150",
                  focusModel && focusModel !== value
                    ? "text-ink-faint opacity-40"
                    : "text-ink-muted",
                )}
              >
                {value}
              </span>
            )}
          />
          {models.map((m, i) => {
            const color = colors[i] ?? modelColor(i);
            const dimmed = focusModel != null && focusModel !== m;
            const focused = focusModel === m;
            return (
              <Line
                key={m}
                type="monotone"
                dataKey={m}
                name={m}
                stroke={color}
                strokeWidth={focused ? 3 : 2.25}
                strokeOpacity={dimmed ? 0.18 : 1}
                dot={{
                  r: focused ? 4 : 3,
                  strokeWidth: 1.5,
                  stroke: "var(--paper-0)",
                  fill: color,
                  fillOpacity: dimmed ? 0.2 : 1,
                }}
                activeDot={{
                  r: 6,
                  strokeWidth: 2,
                  stroke: "var(--border)",
                  fill: color,
                  // 取消 activeDot 的弹跳感
                  className: "recharts-active-dot-hard",
                }}
                isAnimationActive={false}
                onMouseEnter={() => setFocusModel(m)}
                onMouseLeave={() => setFocusModel(null)}
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
  readonly stroke?: string;
  readonly strokeWidth?: number;
  readonly midAngle?: number;
  readonly percent?: number;
  readonly value?: number;
  readonly name?: string;
};

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
        innerRadius={innerRadius - 1}
        outerRadius={outerRadius + 7}
        startAngle={startAngle}
        endAngle={endAngle}
        fill={fill}
        stroke="var(--border)"
        strokeWidth={2}
        style={{ transition: "all 140ms ease-out" }}
      />
      <Sector
        cx={cx}
        cy={cy}
        innerRadius={outerRadius + 10}
        outerRadius={outerRadius + 13}
        startAngle={startAngle}
        endAngle={endAngle}
        fill={fill}
        stroke="none"
      />
    </g>
  );
}

/** Donut share of call counts by model. */
export function ModelShareDonut({
  slices,
  height = 240,
  className,
  centerLabel,
  centerValue,
}: {
  readonly slices: ReadonlyArray<ModelShareSlice>;
  readonly height?: number;
  readonly className?: string;
  readonly centerLabel: string;
  readonly centerValue: string;
}) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const onEnter = useCallback((_: unknown, index: number) => {
    setActiveIndex(index);
  }, []);
  const onLeave = useCallback(() => setActiveIndex(null), []);

  if (slices.length === 0) return null;
  const total = slices.reduce((s, x) => s + x.value, 0);
  const active = activeIndex != null ? slices[activeIndex] : null;
  const activePct =
    active && total > 0 ? ((active.value / total) * 100).toFixed(1) : null;

  return (
    <div className={cn("relative w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={[...slices]}
            dataKey="value"
            nameKey="model"
            cx="50%"
            cy="48%"
            innerRadius="56%"
            outerRadius="80%"
            paddingAngle={2}
            stroke="var(--border)"
            strokeWidth={2}
            isAnimationActive={false}
            {...(activeIndex != null ? { activeIndex } : {})}
            activeShape={DonutActiveShape}
            onMouseEnter={onEnter}
            onMouseLeave={onLeave}
          >
            {slices.map((s, i) => (
              <Cell
                key={s.model}
                fill={s.color}
                fillOpacity={
                  activeIndex == null || activeIndex === i ? 1 : 0.28
                }
                style={{ transition: "fill-opacity 140ms ease-out" }}
              />
            ))}
          </Pie>
          <Tooltip
            isAnimationActive={false}
            animationDuration={0}
            content={({ active: tipActive, payload }) => {
              if (!tipActive || !payload?.length) return null;
              const p = payload[0]!;
              const value = Number(p.value ?? 0);
              const pct = total > 0 ? ((value / total) * 100).toFixed(1) : "0";
              return (
                <HardTooltip
                  active
                  rows={[
                    {
                      name: String(p.name),
                      value,
                      color: String(p.payload?.color ?? "var(--ink)"),
                      pct: `${pct}%`,
                    },
                  ]}
                />
              );
            }}
          />
          <Legend
            verticalAlign="bottom"
            iconType="square"
            wrapperStyle={{ fontSize: 11 }}
            onMouseEnter={(e) => {
              const name = String(e?.value ?? "");
              const idx = slices.findIndex((s) => s.model === name);
              if (idx >= 0) setActiveIndex(idx);
            }}
            onMouseLeave={onLeave}
            formatter={(value) => (
              <span
                className={cn(
                  "font-mono text-[11px] transition-opacity duration-150",
                  active && active.model !== value
                    ? "text-ink-faint opacity-40"
                    : "text-ink-muted",
                )}
              >
                {value}
              </span>
            )}
          />
        </PieChart>
      </ResponsiveContainer>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pb-9">
        <span className="text-[11px] font-medium uppercase tracking-wide text-ink-muted">
          {active ? shortenModel(active.model, 16) : centerLabel}
        </span>
        <span className="mt-0.5 font-mono text-[1.35rem] font-bold tabular-nums text-ink transition-[opacity,transform] duration-150">
          {active ? formatCompact(active.value) : centerValue}
        </span>
        {activePct ? (
          <span className="mt-0.5 font-mono text-[11px] tabular-nums text-ink-muted">
            {activePct}%
          </span>
        ) : null}
      </div>
    </div>
  );
}

/** Horizontal ranked bars for call counts. */
export function ModelRankBars({
  items,
  height = 240,
  className,
  unitLabel,
}: {
  readonly items: ReadonlyArray<ModelRankItem>;
  readonly height?: number;
  readonly className?: string;
  readonly unitLabel: string;
}) {
  const [hoverModel, setHoverModel] = useState<string | null>(null);

  if (items.length === 0) return null;
  const chartData = [...items].reverse();
  const maxVal = Math.max(1, ...items.map((i) => i.value));

  return (
    <div className={cn("w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={chartData}
          layout="vertical"
          margin={{ top: 4, right: 36, left: 4, bottom: 4 }}
          barCategoryGap="30%"
          onMouseLeave={() => setHoverModel(null)}
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.12}
            horizontal={false}
          />
          <XAxis
            type="number"
            allowDecimals={false}
            domain={[0, Math.ceil(maxVal * 1.08)]}
            tick={{
              fill: "var(--ink-faint)",
              fontSize: 10,
              fontFamily: "var(--font-mono)",
            }}
            axisLine={{ stroke: "var(--border)", strokeWidth: 1 }}
            tickLine={false}
            tickFormatter={formatCompact}
          />
          <YAxis
            type="category"
            dataKey="model"
            width={128}
            tickLine={false}
            axisLine={false}
            tick={(props) => {
              const { x, y, payload } = props as {
                x: number;
                y: number;
                payload: { value: string };
              };
              const full = String(payload.value);
              const text = shortenModel(full, 16);
              const dimmed = hoverModel != null && hoverModel !== full;
              return (
                <text
                  x={x}
                  y={y}
                  dy={4}
                  textAnchor="end"
                  fill={dimmed ? "var(--ink-faint)" : "var(--ink)"}
                  fontSize={11}
                  fontFamily="var(--font-mono)"
                  opacity={dimmed ? 0.45 : 1}
                >
                  <title>{full}</title>
                  {text}
                </text>
              );
            }}
          />
          <Tooltip
            isAnimationActive={false}
            animationDuration={0}
            cursor={{ fill: "var(--ink)", fillOpacity: 0.04 }}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const row = payload[0]!.payload as ModelRankItem;
              const pct =
                maxVal > 0 ? `${((row.value / maxVal) * 100).toFixed(0)}%` : "0%";
              return (
                <HardTooltip
                  active
                  rows={[
                    {
                      name: row.model,
                      value: row.value,
                      color: row.color,
                      pct,
                    },
                  ]}
                  totalLabel={unitLabel}
                  footerValue={formatCompact(row.value)}
                />
              );
            }}
          />
          <Bar
            dataKey="value"
            radius={0}
            isAnimationActive={false}
            maxBarSize={22}
            label={{
              position: "right",
              fill: "var(--ink-muted)",
              fontSize: 11,
              fontFamily: "var(--font-mono)",
              formatter: (v: number) => formatCompact(v),
            }}
            onMouseEnter={(state) => {
              const model = String(
                (state as { model?: string } | undefined)?.model ?? "",
              );
              if (model) setHoverModel(model);
            }}
            onMouseLeave={() => setHoverModel(null)}
          >
            {chartData.map((item) => {
              const dimmed = hoverModel != null && hoverModel !== item.model;
              const focused = hoverModel === item.model;
              return (
                <Cell
                  key={item.model}
                  fill={item.color}
                  fillOpacity={dimmed ? 0.28 : 1}
                  stroke="var(--border)"
                  strokeWidth={focused ? 2.25 : 1.5}
                  style={{ transition: "fill-opacity 140ms ease-out" }}
                />
              );
            })}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
