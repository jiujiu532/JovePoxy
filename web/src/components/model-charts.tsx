import { useCallback, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  LabelList,
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

/** 硬边即时浮层：无入场动画。 */
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
        <p className="mb-1.5 font-mono text-[11px] font-semibold text-ink">{label}</p>
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

/** 多序列调用趋势：硬竖线 + 方点。 */
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
            cursor={{ stroke: "var(--border)", strokeWidth: 2 }}
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

/** 占比环：悬停改中心数字，不弹浮层挡中心。 */
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
            cy="46%"
            innerRadius="58%"
            outerRadius="82%"
            paddingAngle={2}
            stroke="var(--border)"
            strokeWidth={2}
            isAnimationActive={false}
            onMouseEnter={onEnter}
            onMouseLeave={onLeave}
          >
            {slices.map((s, i) => (
              <Cell
                key={s.model}
                fill={s.color}
                // 悬停：当前扇区描边加粗，其余不变色不淡出（硬反馈）
                stroke="var(--border)"
                strokeWidth={activeIndex === i ? 3 : 2}
              />
            ))}
          </Pie>
          {/* 无 Tooltip：详情写在中心，避免遮挡 */}
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
                  "font-mono text-[11px]",
                  active && active.model === value
                    ? "font-semibold text-ink"
                    : "text-ink-muted",
                )}
              >
                {value}
              </span>
            )}
          />
        </PieChart>
      </ResponsiveContainer>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pb-10">
        <span
          className="max-w-[9.5rem] truncate text-center text-[11px] font-medium tracking-wide text-ink-muted"
          title={active ? active.model : undefined}
        >
          {active ? shortenModel(active.model, 18) : centerLabel}
        </span>
        <span className="mt-0.5 font-mono text-[1.45rem] font-bold tabular-nums leading-none text-ink">
          {active ? formatCompact(active.value) : centerValue}
        </span>
        {activePct ? (
          <span className="mt-1 font-mono text-[12px] font-semibold tabular-nums text-ink-muted">
            {activePct}%
          </span>
        ) : null}
      </div>
    </div>
  );
}

/** 排行柱：数值用 LabelList 自定义绘制，与柱体拉开间距。 */
function RankValueLabel(props: {
  readonly x?: number;
  readonly y?: number;
  readonly width?: number;
  readonly height?: number;
  readonly value?: number | string;
}) {
  const x = Number(props.x ?? 0);
  const y = Number(props.y ?? 0);
  const width = Number(props.width ?? 0);
  const height = Number(props.height ?? 0);
  const value = Number(props.value ?? 0);
  // 柱体末端再偏右 10px，避免与描边叠字
  const tx = x + width + 10;
  const ty = y + height / 2 + 4;
  return (
    <text
      x={tx}
      y={ty}
      fill="var(--ink)"
      fontSize={13}
      fontWeight={700}
      fontFamily="var(--font-mono)"
      textAnchor="start"
      style={{ paintOrder: "normal" }}
    >
      {formatCompact(value)}
    </text>
  );
}

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
          // 右侧留给数值标签
          margin={{ top: 4, right: 44, left: 4, bottom: 4 }}
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
            domain={[0, Math.ceil(maxVal * 1.12)]}
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
            cursor={{ fill: "var(--accent-soft)" }}
            // 浮层偏到光标外侧，不压住柱端数字
            offset={16}
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
          >
            {chartData.map((item) => (
              <Cell
                key={item.model}
                fill={item.color}
                stroke="var(--border)"
                strokeWidth={2}
              />
            ))}
            <LabelList dataKey="value" content={<RankValueLabel />} />
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
