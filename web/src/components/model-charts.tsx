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

type TipRow = { name: string; value: number; color: string };

function HardTooltip({
  active,
  label,
  rows,
  totalLabel,
}: {
  readonly active?: boolean;
  readonly label?: string;
  readonly rows: ReadonlyArray<TipRow>;
  readonly totalLabel?: string;
}) {
  if (!active || rows.length === 0) return null;
  const sum = rows.reduce((s, r) => s + r.value, 0);
  return (
    <div className="min-w-[10rem] border-2 border-border bg-paper-0 px-3 py-2 shadow-[3px_3px_0_var(--border)]">
      {label ? (
        <p className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-ink-muted">
          {label}
        </p>
      ) : null}
      <ul className="flex flex-col gap-1">
        {rows.map((r) => (
          <li
            key={r.name}
            className="flex items-center justify-between gap-4 text-[12px] text-ink"
          >
            <span className="inline-flex min-w-0 items-center gap-1.5">
              <span
                className="inline-block h-2.5 w-2.5 shrink-0 border border-border"
                style={{ backgroundColor: r.color }}
                aria-hidden
              />
              <span className="truncate font-mono text-[11px]">{r.name}</span>
            </span>
            <span className="font-mono font-semibold tabular-nums">
              {formatCompact(r.value)}
            </span>
          </li>
        ))}
      </ul>
      {totalLabel != null ? (
        <p className="mt-1.5 border-t-2 border-border pt-1.5 text-[11px] text-ink-muted">
          {totalLabel}{" "}
          <span className="font-mono font-semibold tabular-nums text-ink">
            {formatCompact(sum)}
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
  if (empty || data.length === 0 || models.length === 0) return null;

  return (
    <div className={cn("w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={[...data]} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.18}
            vertical={false}
          />
          <XAxis
            dataKey="day"
            tick={{ fill: "var(--ink-faint)", fontSize: 11, fontFamily: "var(--font-mono)" }}
            axisLine={{ stroke: "var(--border)", strokeWidth: 2 }}
            tickLine={false}
            dy={6}
          />
          <YAxis
            allowDecimals={false}
            width={36}
            tick={{ fill: "var(--ink-faint)", fontSize: 10, fontFamily: "var(--font-mono)" }}
            axisLine={false}
            tickLine={false}
            tickFormatter={formatCompact}
          />
          <Tooltip
            cursor={{ stroke: "var(--border)", strokeWidth: 1, strokeDasharray: "4 4" }}
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
              return (
                <HardTooltip
                  {...(active !== undefined ? { active } : {})}
                  label={String(label ?? "")}
                  rows={rows}
                  totalLabel={totalLabel}
                />
              );
            }}
          />
          <Legend
            verticalAlign="bottom"
            height={36}
            iconType="plainline"
            wrapperStyle={{ fontSize: 11, paddingTop: 8 }}
            formatter={(value) => (
              <span className="font-mono text-[11px] text-ink-muted">{value}</span>
            )}
          />
          {models.map((m, i) => (
            <Line
              key={m}
              type="monotone"
              dataKey={m}
              name={m}
              stroke={colors[i] ?? modelColor(i)}
              strokeWidth={2.25}
              dot={{ r: 3, strokeWidth: 1.5, stroke: "var(--paper-0)", fill: colors[i] ?? modelColor(i) }}
              activeDot={{ r: 5, strokeWidth: 2, stroke: "var(--border)" }}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
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
            innerRadius="58%"
            outerRadius="82%"
            paddingAngle={1.5}
            stroke="var(--border)"
            strokeWidth={2}
            isAnimationActive={false}
          >
            {slices.map((s) => (
              <Cell key={s.model} fill={s.color} />
            ))}
          </Pie>
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
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
                    },
                  ]}
                  totalLabel={`${pct}%`}
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
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pb-8">
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
  if (items.length === 0) return null;
  // Recharts horizontal bar needs category on Y
  const chartData = [...items].reverse();

  return (
    <div className={cn("w-full", className)} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={chartData}
          layout="vertical"
          margin={{ top: 4, right: 28, left: 4, bottom: 4 }}
          barCategoryGap="28%"
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.14}
            horizontal={false}
          />
          <XAxis
            type="number"
            allowDecimals={false}
            tick={{ fill: "var(--ink-faint)", fontSize: 10, fontFamily: "var(--font-mono)" }}
            axisLine={{ stroke: "var(--border)", strokeWidth: 1 }}
            tickLine={false}
            tickFormatter={formatCompact}
          />
          <YAxis
            type="category"
            dataKey="model"
            width={108}
            tick={{ fill: "var(--ink)", fontSize: 11, fontFamily: "var(--font-mono)" }}
            axisLine={false}
            tickLine={false}
          />
          <Tooltip
            cursor={{ fill: "var(--paper-2)", opacity: 0.5 }}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const p = payload[0]!;
              const row = p.payload as ModelRankItem;
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
                />
              );
            }}
          />
          <Bar dataKey="value" radius={0} isAnimationActive={false} maxBarSize={22}>
            {chartData.map((item) => (
              <Cell
                key={item.model}
                fill={item.color}
                stroke="var(--border)"
                strokeWidth={1.5}
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
