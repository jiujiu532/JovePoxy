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
  Sector,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { cn } from "@/lib/cn";

/** Playful hard-edge palette — no AI purple; enough stops for full-series legends. */
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
  "#d63031",
  "#6c5ce7",
  "#00cec9",
  "#fdcb6e",
  "#e84393",
  "#55a3ff",
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

/** 多序列调用趋势：虚线竖标 + 空心圆点 + 参考风 tooltip。 */
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
  if (empty || data.length === 0 || models.length === 0) return null;

  // Extra room for multi-line legend when many models (R-B full series).
  const legendExtra = Math.min(80, Math.max(0, (models.length - 4) * 14));
  const chartHeight = height + legendExtra;

  return (
    <div className={cn("w-full", className)} style={{ height: chartHeight }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={[...data]}
          margin={{ top: 10, right: 14, left: 0, bottom: 8 }}
        >
          <CartesianGrid
            stroke="var(--border)"
            strokeOpacity={0.12}
            vertical={false}
          />
          <XAxis
            dataKey="day"
            interval="preserveStartEnd"
            minTickGap={28}
            tick={{
              fill: "var(--ink-faint)",
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
            iconType="plainline"
            wrapperStyle={{
              fontSize: 11,
              paddingTop: 10,
              maxHeight: 72,
              overflowY: "auto",
              width: "100%",
            }}
            formatter={(value) => (
              <span className="font-mono text-[11px] text-ink-muted">{value}</span>
            )}
          />
          {models.map((m, i) => {
            const color = colors[i] ?? modelColor(i);
            return (
              <Line
                key={m}
                type="monotone"
                dataKey={m}
                name={m}
                stroke={color}
                strokeWidth={2}
                // 默认小实心点；悬停空心放大（参考图灵动感）
                dot={{
                  r: 3,
                  strokeWidth: 0,
                  fill: color,
                }}
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

/** 占比环：悬停放大 + 外侧 tooltip「模型: n 次 (pct%)」。 */
export function ModelShareDonut({
  slices,
  height = 240,
  className,
  centerLabel,
  centerValue,
  callsUnit,
}: {
  readonly slices: ReadonlyArray<ModelShareSlice>;
  readonly height?: number;
  readonly className?: string;
  readonly centerLabel: string;
  readonly centerValue: string;
  readonly callsUnit: string;
}) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const onEnter = useCallback((_: unknown, index: number) => {
    setActiveIndex(index);
  }, []);
  const onLeave = useCallback(() => setActiveIndex(null), []);

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
            cy="46%"
            innerRadius="56%"
            outerRadius="76%"
            paddingAngle={2}
            stroke="var(--border)"
            strokeWidth={2}
            isAnimationActive={false}
            {...(activeIndex != null ? { activeIndex } : {})}
            activeShape={DonutActiveShape}
            onMouseEnter={onEnter}
            onMouseLeave={onLeave}
          >
            {slices.map((s) => (
              <Cell key={s.model} fill={s.color} />
            ))}
          </Pie>
          <Tooltip
            isAnimationActive={false}
            animationDuration={0}
            // 外置偏移，避免压中心
            offset={18}
            allowEscapeViewBox={{ x: true, y: true }}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const p = payload[0]!;
              const value = Number(p.value ?? 0);
              const pct = total > 0 ? ((value / total) * 100).toFixed(0) : "0";
              const name = String(p.name);
              return (
                <RefTooltip active>
                  <p className="font-mono text-[12px] text-ink">
                    <span className="font-semibold">{name}</span>
                    <span className="text-ink-muted">
                      {`: ${formatCompact(value)} ${callsUnit} (${pct}%)`}
                    </span>
                  </p>
                </RefTooltip>
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
              <span className="font-mono text-[11px] text-ink-muted">{value}</span>
            )}
          />
        </PieChart>
      </ResponsiveContainer>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pb-10">
        <span className="text-[11px] font-medium uppercase tracking-wide text-ink-muted">
          {centerLabel}
        </span>
        <span className="mt-0.5 font-mono text-[1.45rem] font-bold tabular-nums leading-none text-ink">
          {centerValue}
        </span>
      </div>
    </div>
  );
}

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
            offset={16}
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const row = payload[0]!.payload as ModelRankItem;
              return (
                <RefTooltip active>
                  <p className="flex items-center gap-2 font-mono text-[12px] text-ink">
                    <span
                      className="inline-block h-2 w-2 shrink-0 rounded-full"
                      style={{ backgroundColor: row.color }}
                      aria-hidden
                    />
                    <span className="font-semibold">{row.model}</span>
                    <span className="text-ink-muted">
                      {formatCompact(row.value)} {unitLabel}
                    </span>
                  </p>
                </RefTooltip>
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
