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

/** 硬边即时浮层：无入场动画、无拖尾。 */
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
    >
      {label ? (
        <p className="mb-1.5 font-mono text-[11px] font-semibold text-ink">
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

/** 多序列调用趋势：十字线 + 方点高亮，系列不淡出。 */
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
  if (empty || data.length === 0 || models.length === 0) return null;

  return (
    <div className={cn("w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={[...data]}
          margin={{ top: 10, right: 14, left: 0, bottom: 0 }}
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.14}
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
            isAnimationActive={false}
            animationDuration={0}
            // 硬竖线十字：无虚线、无淡入
            cursor={{
              stroke: "var(--border)",
              strokeWidth: 2,
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
            formatter={(value) => (
              <span className="font-mono text-[11px] text-ink-muted">{value}</span>
            )}
          />
          {models.map((m, i) => {
            const color = colors[i] ?? modelColor(i);
            return (
              <Line
                key={m}
                type="linear"
                dataKey={m}
                name={m}
                stroke={color}
                strokeWidth={2.5}
                // 直角点：硬边方块，悬停变黑边放大一档
                dot={{
                  r: 3.5,
                  strokeWidth: 1.5,
                  stroke: "var(--border)",
                  fill: color,
                }}
                activeDot={{
                  r: 5.5,
                  strokeWidth: 2,
                  stroke: "var(--border)",
                  fill: color,
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

/** 占比环：仅换中心文案 + 硬边 tooltip，扇区不外扩。 */
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
  if (slices.length === 0) return null;
  const total = slices.reduce((s, x) => s + x.value, 0);

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
          >
            {slices.map((s) => (
              <Cell key={s.model} fill={s.color} />
            ))}
          </Pie>
          <Tooltip
            isAnimationActive={false}
            animationDuration={0}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const p = payload[0]!;
              const value = Number(p.value ?? 0);
              const pct = total > 0 ? ((value / total) * 100).toFixed(1) : "0";
              const name = String(p.name);
              return (
                <HardTooltip
                  active
                  rows={[
                    {
                      name,
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
            formatter={(value) => (
              <span className="font-mono text-[11px] text-ink-muted">{value}</span>
            )}
          />
        </PieChart>
      </ResponsiveContainer>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pb-9">
        <span className="text-[11px] font-medium uppercase tracking-wide text-ink-muted">
          {centerLabel}
        </span>
        <span className="mt-0.5 font-mono text-[1.35rem] font-bold tabular-nums text-ink">
          {centerValue}
        </span>
      </div>
    </div>
  );
}

/** 排行柱：硬边描边 + 右侧数值，无淡出。 */
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
              return (
                <text
                  x={x}
                  y={y}
                  dy={4}
                  textAnchor="end"
                  fill="var(--ink)"
                  fontSize={11}
                  fontFamily="var(--font-mono)"
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
            // 整行硬底高亮（非半透明扫光）
            cursor={{ fill: "var(--accent-soft)", stroke: "var(--border)", strokeWidth: 0 }}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const row = payload[0]!.payload as ModelRankItem;
              return (
                <HardTooltip
                  active
                  rows={[
                    {
                      name: row.model,
                      value: row.value,
                      color: row.color,
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
          >
            {chartData.map((item) => (
              <Cell
                key={item.model}
                fill={item.color}
                stroke="var(--border)"
                strokeWidth={2}
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
