import {
  CaretDown,
  ChartPieSlice,
  Heartbeat,
  WarningCircle,
} from "@phosphor-icons/react";
import { useState } from "react";
import { Badge, HelpTip } from "@/components";
import { formatTrafficPct } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import {
  buildKeyPoolShare,
  formatHealthScore,
  type KeyPoolShareSlice,
  type KeyPoolShareSummary,
} from "@/lib/key-pool-share";
import type { KeyProvider, ZenKeyDTO } from "@/lib/api";
import { cn } from "@/lib/cn";

export type KeyPoolSharePanelProps = {
  readonly keys: readonly ZenKeyDTO[];
  readonly nowMs?: number;
  readonly provider: KeyProvider;
  readonly className?: string;
};

/**
 * In-pool estimated dynamic share for the current provider tab only.
 * Collapsed by default to keep the key-pool page dense.
 */
export function KeyPoolSharePanel({
  keys,
  nowMs = Date.now(),
  provider,
  className,
}: KeyPoolSharePanelProps) {
  const { t } = useI18n();
  const summary = buildKeyPoolShare(keys, nowMs);
  const [open, setOpen] = useState(false);
  const {
    eligibleCount,
    probingCount,
    coolingCount,
    attentionCount,
  } = summary;

  return (
    <section
      className={cn(
        "overflow-hidden rounded-none border-2 border-border bg-paper-1 shadow-[var(--shadow-hard)]",
        className,
      )}
    >
      <button
        type="button"
        className={cn(
          "flex w-full items-center gap-2 border-0 bg-transparent px-3 py-2 text-left",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus-ring",
          open && "border-b-2 border-border",
        )}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <span
          className="inline-flex h-7 w-7 shrink-0 items-center justify-center border-2 border-border bg-accent-teal text-black shadow-[2px_2px_0_var(--border)]"
          aria-hidden
        >
          <ChartPieSlice size={14} weight="duotone" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <span className="text-[13px] font-semibold tracking-tight text-ink">
              {t("keypool.shareDynamicTitle")}
            </span>
            <span
              onClick={(e) => e.stopPropagation()}
              onKeyDown={(e) => e.stopPropagation()}
            >
              <HelpTip
                content={`${t("keypool.shareDynamicNote")} ${t("keypool.shareStickyNote")} ${t("keypool.pollTip")}`}
                label={t("keypool.shareDynamicTitle")}
              />
            </span>
          </span>
          <span className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[11px] tabular-nums text-ink-muted">
            <MiniStat
              label={t("keypool.railEnabledHint")}
              value={eligibleCount}
              tone="teal"
            />
            <MiniStat
              label={t("keypool.statusProbing")}
              value={probingCount}
              tone="yellow"
            />
            <MiniStat
              label={t("keypool.statusCooling")}
              value={coolingCount}
              tone="mint"
            />
            <MiniStat
              label={t("keypool.attentionLabel")}
              value={attentionCount}
              tone="accent"
            />
          </span>
        </span>
        <span className="inline-flex shrink-0 items-center gap-1 text-[11px] font-medium text-ink-muted">
          <span className="hidden sm:inline">
            {open ? t("keypool.collapse") : t("keypool.expand")}
          </span>
          <CaretDown
            size={14}
            weight="bold"
            className={cn(
              "text-ink transition-transform duration-150",
              open && "rotate-180",
            )}
            aria-hidden
          />
        </span>
      </button>

      {open ? (
        <div className="flex flex-col gap-2.5 px-3 py-2.5">
          <KeyPoolShareBody summary={summary} provider={provider} />
        </div>
      ) : null}
    </section>
  );
}

function MiniStat({
  label,
  value,
  tone,
}: {
  readonly label: string;
  readonly value: number;
  readonly tone: "yellow" | "teal" | "mint" | "accent";
}) {
  const dot = {
    yellow: "bg-accent-yellow",
    teal: "bg-accent-teal",
    mint: "bg-accent-mint",
    accent: "bg-accent",
  }[tone];
  return (
    <span className="inline-flex items-center gap-1" title={label}>
      <span
        className={cn("inline-block h-1.5 w-1.5 border border-border", dot)}
        aria-hidden
      />
      <span className="text-ink-faint">{label}</span>
      <span className="font-semibold text-ink">{value}</span>
    </span>
  );
}

function KeyPoolShareBody({
  summary,
  provider,
}: {
  readonly summary: KeyPoolShareSummary;
  readonly provider: KeyProvider;
}) {
  const { t } = useI18n();
  const { slices, eligibleCount } = summary;

  return (
    <>
      {eligibleCount === 0 ? (
        <div className="flex flex-col gap-1.5 border-2 border-border bg-paper-2 px-2.5 py-2">
          <div className="flex items-center gap-1.5 text-[12px] font-semibold text-ink">
            <WarningCircle size={14} weight="duotone" className="text-accent" />
            {t("keypool.shareLabel")}: {t("keypool.shareZero")}
          </div>
          <p className="text-[11px] leading-snug text-ink-muted">
            {t("keypool.shareEmptyHint")}
          </p>
          <div
            className="h-4 w-full border-2 border-border bg-paper-1"
            role="img"
            aria-label={`${t("keypool.colShare")}: ${t("keypool.shareZero")}`}
          />
        </div>
      ) : (
        <>
          <DynamicShareBar slices={slices} />
          <ul className="m-0 flex list-none flex-col gap-1 p-0">
            {slices.map((s) => (
              <li
                key={s.id}
                className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 border-2 border-border bg-paper-2 px-2 py-1.5"
              >
                <span
                  className="inline-block h-2.5 w-2.5 shrink-0 border-2 border-border"
                  style={{ backgroundColor: s.color }}
                  aria-hidden
                />
                <span className="min-w-0 truncate text-[12px] font-semibold text-ink">
                  {s.label}
                </span>
                <span className="font-mono text-[10px] text-ink-muted">
                  {s.prefix}
                </span>
                <span className="inline-flex items-center gap-0.5 font-mono text-[11px] tabular-nums text-ink">
                  <Heartbeat
                    size={11}
                    weight="duotone"
                    className="text-accent"
                  />
                  {formatHealthScore(s.healthScore)}
                  <span className="text-ink-faint">/</span>
                  {formatHealthScore(s.selectionScore)}
                </span>
                <Badge kind="healthy">{t("keypool.statusActive")}</Badge>
                <span className="ml-auto font-mono text-[12px] font-bold tabular-nums text-ink">
                  {formatTrafficPct(s.sharePct)}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}

      <p className="font-mono text-[10px] text-ink-faint">
        {provider === "ollama"
          ? t("keypool.barKeyLabelOl")
          : t("keypool.barKeyLabelOc")}
      </p>
    </>
  );
}

function DynamicShareBar({
  slices,
}: {
  readonly slices: readonly KeyPoolShareSlice[];
}) {
  const { t } = useI18n();
  const total = slices.reduce((sum, s) => sum + s.sharePct, 0);
  const aria = slices
    .map((s) => `${s.label} ${formatTrafficPct(s.sharePct)}`)
    .join(", ");

  return (
    <figure
      className="m-0"
      role="img"
      aria-label={`${t("keypool.colShare")}: ${aria}`}
    >
      <div className="flex h-5 w-full overflow-hidden border-2 border-border bg-paper-2">
        {total <= 0 ? (
          <div className="flex w-full items-center justify-center text-[10px] text-ink-faint">
            {t("keypool.shareZero")}
          </div>
        ) : (
          slices
            .filter((s) => s.sharePct > 0)
            .map((s, i) => (
              <div
                key={s.id}
                className={cn("h-full", i > 0 && "border-l-2 border-border")}
                style={{
                  width: `${(s.sharePct / total) * 100}%`,
                  backgroundColor: s.color,
                  minWidth: 4,
                }}
                title={`${s.label} · ${s.prefix} · ${formatTrafficPct(s.sharePct)}`}
              />
            ))
        )}
      </div>
    </figure>
  );
}
