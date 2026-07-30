import {
  ChartLine,
  Plus,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  Button,
  EmptyState,
  MetricRail,
  PageHeader,
  Pagination,
  PosterEmpty,
  QuotaAccountViews,
  Skeleton,
  Tabs,
  ViewModeToggle,
  slicePage,
  type QuotaAccountView,
} from "@/components";
import { api, ApiError, type AccountQuotaDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n, type Translate } from "@/lib/i18n";
import { isProviderTab, type ProviderTab } from "@/lib/routes";
import { useViewMode } from "@/lib/view-mode";

type OllamaQuotaItem = {
  account_id: string;
  name: string;
  success: boolean;
  plan?: string;
  error?: string;
  windows?: ReadonlyArray<{
    label: string;
    used: number;
    remaining: number;
    unit: string;
    status_text?: string;
    models?: ReadonlyArray<{ model: string; requests: number }>;
  }>;
};

function formatReset(t: Translate, sec: number): string {
  if (sec <= 0) return t("quotas.resetSoon");
  if (sec < 3600) return t("quotas.resetMinutes", { n: Math.ceil(sec / 60) });
  if (sec < 86400) return t("quotas.resetHours", { n: (sec / 3600).toFixed(1) });
  return t("quotas.resetDays", { n: (sec / 86400).toFixed(1) });
}

function toOpenCodeViews(
  t: Translate,
  quotas: ReadonlyArray<AccountQuotaDTO>,
): QuotaAccountView[] {
  return quotas.map((item) => {
    const base: QuotaAccountView = {
      id: item.account_id,
      name: item.name,
      subtitle: item.workspace_id,
      success: item.success,
      windows: (item.windows ?? []).map((window) => {
        const pct =
          window.total > 0
            ? (window.used / window.total) * 100
            : Math.min(100, Math.max(0, window.used));
        return {
          label: window.label,
          percent: pct,
          primaryText: `${window.remaining.toFixed(0)}${window.unit}`,
          hint: t("quotas.usedHint", {
            used: window.used.toFixed(1),
            unit: window.unit,
            reset: formatReset(t, window.reset_in_sec),
          }),
        };
      }),
    };
    if (item.error) {
      return { ...base, error: item.error };
    }
    return base;
  });
}

function toOllamaViews(
  t: Translate,
  quotas: ReadonlyArray<OllamaQuotaItem>,
): QuotaAccountView[] {
  return quotas.map((item) => {
    const models =
      item.windows?.flatMap((w) => w.models ?? []).reduce<
        Array<{ model: string; requests: number }>
      >((acc, m) => {
        const existing = acc.find((x) => x.model === m.model);
        if (existing) {
          existing.requests += m.requests;
        } else {
          acc.push({ model: m.model, requests: m.requests });
        }
        return acc;
      }, []) ?? [];

    const base: QuotaAccountView = {
      id: item.account_id,
      name: item.name,
      success: item.success,
      windows: (item.windows ?? []).map((w) => {
        const percent = Number.isFinite(w.used) ? w.used : 0;
        return {
          label: w.label,
          percent,
          primaryText: `${percent.toFixed(0)}%`,
          hint: w.status_text
            ? w.status_text
            : t("quotas.remainingHint", { remaining: w.remaining.toFixed(1), unit: w.unit }),
        };
      }),
    };
    if (item.plan) {
      return {
        ...base,
        badge: item.plan,
        ...(item.error ? { error: item.error } : {}),
        ...(models.length > 0 ? { models } : {}),
      };
    }
    return {
      ...base,
      ...(item.error ? { error: item.error } : {}),
      ...(models.length > 0 ? { models } : {}),
    };
  });
}

function useProviderTab(
  defaultTab: ProviderTab = "opencode",
): readonly [ProviderTab, (tab: ProviderTab) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  const tab: ProviderTab = isProviderTab(raw) ? raw : defaultTab;

  function setTab(next: ProviderTab) {
    setParams(next === defaultTab ? {} : { tab: next }, { replace: true });
  }

  return [tab, setTab] as const;
}

export function QuotasPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
  const [tab, setTab] = useProviderTab("opencode");
  const [viewMode, setViewMode] = useViewMode("quota-monitor", "grid");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [ocQuotas, setOcQuotas] = useState<AccountQuotaDTO[]>([]);
  const [olQuotas, setOlQuotas] = useState<OllamaQuotaItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const [oc, ol] = await Promise.all([api.quotas(), api.ollamaQuotas()]);
      setOcQuotas(oc.quotas);
      setOlQuotas([...ol.quotas]);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    setPage(1);
  }, [tab, viewMode]);

  const ocOk = ocQuotas.filter((q) => q.success).length;
  const olOk = olQuotas.filter((q) => q.success).length;
  const activeRaw = tab === "opencode" ? ocQuotas : olQuotas;
  const activeOk = tab === "opencode" ? ocOk : olOk;
  const activeFail = Math.max(0, activeRaw.length - activeOk);
  const successRate =
    activeRaw.length === 0
      ? "—"
      : `${Math.round((activeOk / activeRaw.length) * 100)}%`;

  const activeItems = useMemo(
    () => (tab === "opencode" ? toOpenCodeViews(t, ocQuotas) : toOllamaViews(t, olQuotas)),
    [tab, ocQuotas, olQuotas, t],
  );
  const activeCount = activeItems.length;
  const pagedItems = useMemo(
    () => slicePage(activeItems, page, pageSize),
    [activeItems, page, pageSize],
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("quotas.title")}
        toolbar={
          <Tabs
            aria-label={t("quotas.tabsLabel")}
            value={tab}
            onChange={(id) => setTab(id as ProviderTab)}
            items={
              loading
                ? [
                    { id: "opencode", label: "OpenCode" },
                    { id: "ollama", label: "Ollama" },
                  ]
                : [
                    { id: "opencode", label: "OpenCode", count: ocOk },
                    { id: "ollama", label: "Ollama", count: olOk },
                  ]
            }
          />
        }
        actions={
          <>
            <ViewModeToggle value={viewMode} onChange={setViewMode} />
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("quotas.refetch")}
            </Button>
          </>
        }
      />

      {loading ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-44 w-full" />
        </div>
      ) : null}
      {!loading && error ? (
        <EmptyState
          icon={ChartLine}
          title={t("common.loadFailed")}
          description={error}
          action={
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.retry")}
            </Button>
          }
        />
      ) : null}

      {!loading && !error && activeCount === 0 ? (
        <PosterEmpty
          theme="teal"
          stamp={
            tab === "opencode"
              ? t("quotas.posterStampOc")
              : t("quotas.posterStampOl")
          }
          stampSub={t("quotas.posterStampSub")}
          title={t("quotas.emptyTitle")}
          description={
            tab === "opencode"
              ? t("quotas.emptyOpencodeDescription")
              : t("quotas.emptyOllamaDescription")
          }
          note={t("quotas.posterNote")}
          action={
            <Button
              className="!px-5 !py-3 !text-[15px] !font-black shadow-[6px_6px_0_var(--border)]"
              onClick={() =>
                void navigate(
                  tab === "opencode" ? "/app/accounts" : "/app/accounts?tab=ollama",
                )
              }
            >
              <Plus size={16} className="mr-1" weight="bold" />
              {t("quotas.emptyAction")}
            </Button>
          }
          bars={[
            {
              label: t("quotas.barScrapeLabel"),
              detail:
                tab === "opencode"
                  ? t("quotas.barScrapeDetailOc")
                  : t("quotas.barScrapeDetailOl"),
              tone: "teal",
            },
            {
              label: t("quotas.barCookieLabel"),
              detail: t("quotas.barCookieDetail"),
              tone: "yellow",
            },
            {
              label: t("quotas.barViewLabel"),
              detail: t("quotas.barViewDetail"),
              tone: "mint",
            },
          ]}
        />
      ) : null}

      {!loading && !error && activeCount > 0 ? (
        <div className="flex flex-col gap-3">
          <MetricRail
            items={[
              {
                label: t("kpi.total"),
                value: activeCount,
                hint: t("quotas.railTotalHint"),
                tone: "yellow",
              },
              {
                label: t("common.enabled"),
                value: activeOk,
                hint: t("quotas.railOkHint"),
                tone: "teal",
              },
              {
                label: t("common.loadFailed"),
                value: activeFail,
                hint: t("quotas.railFailHint"),
                tone: "accent",
              },
              {
                label: "%",
                value: successRate,
                hint: t("quotas.railRateHint"),
                tone: "mint",
              },
            ]}
          />
          <QuotaAccountViews
            mode={viewMode}
            items={pagedItems}
            {...(tab === "ollama"
              ? {
                  tableHeaders: [
                    t("quotas.table.account"),
                    t("quotas.table.plan"),
                    t("quotas.table.session"),
                    t("quotas.table.weekly"),
                    t("quotas.table.status"),
                    t("quotas.table.models"),
                  ],
                }
              : {})}
          />
          <div className="overflow-hidden rounded-none border-2 border-border bg-paper-1 shadow-[4px_4px_0_var(--border)]">
            <Pagination
              total={activeCount}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              onPageSizeChange={(size) => {
                setPageSize(size);
                setPage(1);
              }}
            />
          </div>
        </div>
      ) : null}

      {!loading && !error ? (
        <p className="text-[12px] text-ink-faint">
          {t("quotas.accountsLinkPrefix")}{" "}
          <Link
            to={tab === "opencode" ? "/app/accounts" : "/app/accounts?tab=ollama"}
            className="text-accent hover:underline"
          >
            {t("nav.accounts")}
          </Link>
        </p>
      ) : null}
    </div>
  );
}
