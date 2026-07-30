import {
  ChartLine,
  Cloud,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  Button,
  EmptyState,
  ErrorState,
  PageHeader,
  Pagination,
  QuotaAccountViews,
  SectionPanel,
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
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}

      {!loading && !error && activeCount === 0 ? (
        <SectionPanel
          title={t("quotas.resultsTitle")}
          icon={ChartLine}
          iconTone="yellow"
          bodyClassName="p-0"
        >
          <EmptyState
            icon={tab === "opencode" ? ChartLine : Cloud}
            title={t("quotas.emptyTitle")}
            description={
              tab === "opencode"
                ? t("quotas.emptyOpencodeDescription")
                : t("quotas.emptyOllamaDescription")
            }
            action={
              <Button
                variant="secondary"
                onClick={() =>
                  void navigate(
                    tab === "opencode" ? "/app/accounts" : "/app/accounts?tab=ollama",
                  )
                }
              >
                {t("quotas.emptyAction")}
              </Button>
            }
          />
        </SectionPanel>
      ) : null}

      {!loading && !error && activeCount > 0 ? (
        <div className="flex flex-col gap-3">
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
          <div className="overflow-hidden rounded-none border border-border bg-paper-1">
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
