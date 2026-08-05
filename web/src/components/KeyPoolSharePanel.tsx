import { ChartPieSlice, Heartbeat, WarningCircle } from "@phosphor-icons/react";
import { Badge, HelpTip, SectionPanel } from "@/components";
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
 * Based on health/selection scores (or server traffic_pct), not historical hits
 * and not cross-provider routing.
 */
export function KeyPoolSharePanel({
  keys,
  nowMs = Date.now(),
  provider,
  className,
}: KeyPoolSharePanelProps) {
  const { t } = useI18n();
  const summary = buildKeyPoolShare(keys, nowMs);

  return (
    <SectionPanel
      {...(className ? { className } : {})}
      title={t("keypool.shareDynamicTitle")}
      description={t("keypool.shareTip")}
      icon={ChartPieSlice}
      iconTone="teal"
      actions={
        <span className="inline-flex items-center gap-1 text-[12px] text-ink-muted">
          <HelpTip content={t("keypool.pollTip")} />
          <span className="hidden sm:inline">{t("keypool.pollTipLabel")}</span>
        </span>
      }
    >
      <KeyPoolShareBody summary={summary} provider={provider} />
    </SectionPanel>
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
  const {
    slices,
    eligibleCount,
    probingCount,
    coolingCount,
    attentionCount,
  } = summary;

  return (
    <div className="flex flex-col gap-3.5">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <SummaryChip
          label={t("keypool.railEnabledHint")}
          value={eligibleCount}
          hint={t("keypool.shareTip")}
          tone="teal"
        />
        <SummaryChip
          label={t("keypool.statusProbing")}
          value={probingCount}
          hint={t("keypool.railProbingHint")}
          tone="yellow"
        />
        <SummaryChip
          label={t("keypool.statusCooling")}
          value={coolingCount}
          hint={t("keypool.railCoolingHint")}
          tone="mint"
        />
        <SummaryChip
          label={t("keypool.attentionLabel")}
          value={attentionCount}
          hint={t("keypool.railAttentionHint")}
          tone="accent"
        />
      </div>

      {eligibleCount === 0 ? (
        <div className="flex flex-col items-start gap-2 border-2 border-border bg-paper-2 p-3">
          <div className="flex items-center gap-2 text-[13px] font-semibold text-ink">
            <WarningCircle size={16} weight="duotone" className="text-accent" />
            {t("keypool.shareLabel")}: {t("keypool.shareZero")}
          </div>
          <p className="text-[12px] leading-relaxed text-ink-muted">
            {t("keypool.shareEmptyHint")}
          </p>
          <div
            className="h-6 w-full border-2 border-border bg-paper-1"
            role="img"
            aria-label={`${t("keypool.colShare")}: ${t("keypool.shareZero")}`}
          />
        </div>
      ) : (
        <>
          <DynamicShareBar slices={slices} />
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
            {slices.map((s) => (
              <li
                key={s.id}
                className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 border-2 border-border bg-paper-2 px-2.5 py-2"
              >
                <span
                  className="inline-block h-3 w-3 shrink-0 border-2 border-border"
                  style={{ backgroundColor: s.color }}
                  aria-hidden
                />
                <span className="min-w-0 truncate text-[13px] font-semibold text-ink">
                  {s.label}
                </span>
                <span className="font-mono text-[11px] text-ink-muted">{s.prefix}</span>
                <span className="inline-flex items-center gap-1 font-mono text-[12px] tabular-nums text-ink">
                  <Heartbeat size={12} weight="duotone" className="text-accent" />
                  {formatHealthScore(s.healthScore)}
                  <span className="text-ink-faint">/</span>
                  {formatHealthScore(s.selectionScore)}
                </span>
                <Badge kind="healthy">{t("keypool.statusActive")}</Badge>
                <span className="ml-auto font-mono text-[13px] font-bold tabular-nums text-ink">
                  {formatTrafficPct(s.sharePct)}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="flex flex-col gap-1 border-t-2 border-border pt-2.5 text-[11px] leading-relaxed text-ink-faint">
        <p>{t("keypool.shareDynamicNote")}</p>
        <p>{t("keypool.shareStickyNote")}</p>
        <p className="font-mono">
          {provider === "ollama" ? t("keypool.barKeyLabelOl") : t("keypool.barKeyLabelOc")}
        </p>
      </div>
    </div>
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
      <div className="flex h-7 w-full overflow-hidden border-2 border-border bg-paper-2">
        {total <= 0 ? (
          <div className="flex w-full items-center justify-center text-[11px] text-ink-faint">
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
                  minWidth: 6,
                }}
                title={`${s.label} · ${s.prefix} · ${formatTrafficPct(s.sharePct)}`}
              />
            ))
        )}
      </div>
      <figcaption className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-ink-muted">
        {slices.map((s) => (
          <span key={s.id} className="inline-flex items-center gap-1.5">
            <span
              className="inline-block h-2.5 w-2.5 border-2 border-border"
              style={{ backgroundColor: s.color }}
              aria-hidden
            />
            <span className="max-w-[8rem] truncate">{s.label}</span>
            <span className="font-medium tabular-nums text-ink">
              {formatTrafficPct(s.sharePct)}
            </span>
          </span>
        ))}
      </figcaption>
    </figure>
  );
}

function SummaryChip({
  label,
  value,
  hint,
  tone,
}: {
  readonly label: string;
  readonly value: number;
  readonly hint: string;
  readonly tone: "yellow" | "teal" | "mint" | "accent";
}) {
  const toneClass = {
    yellow: "bg-accent-yellow text-black",
    teal: "bg-accent-teal text-black",
    mint: "bg-accent-mint text-black",
    accent: "bg-accent text-black",
  }[tone];

  return (
    <div
      className={cn(
        "min-w-0 border-2 border-border p-2 shadow-[2px_2px_0_var(--border)]",
        toneClass,
      )}
      title={hint}
    >
      <p className="truncate text-[10px] font-bold uppercase tracking-wide text-black/70">
        {label}
      </p>
      <p className="mt-0.5 font-mono text-[1.25rem] font-extrabold tabular-nums leading-none text-black">
        {value}
      </p>
    </div>
  );
}
