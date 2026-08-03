import {
  ArrowClockwise,
  Coins,
  Globe,
  Stack,
  SquaresFour,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  FilterSelect,
  FilterStrip,
  PageHeader,
  Pagination,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  StatCard,
  slicePage,
} from "@/components";
import { api, ApiError, type ModelDTO, type ModelProvider } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { cn } from "@/lib/cn";
import { familyInitials, familyTone } from "@/lib/family-tone";
import { useI18n } from "@/lib/i18n";

/** free≈public, paid≈zen — single axis. */
type KindFilter = "all" | "free" | "paid";
type ProviderFilter = "all" | ModelProvider;
type SortKey = "id_asc" | "id_desc" | "free_first" | "paid_first";

function modelFamily(id: string): string {
  const parts = id.split(/[-_/]/).filter(Boolean);
  return (parts[0] ?? id).toLowerCase();
}

/** Missing provider on older backends → opencode. */
function modelProvider(model: ModelDTO): ModelProvider {
  return model.provider === "ollama" ? "ollama" : "opencode";
}

function ModelIdCell({ id, family }: { readonly id: string; readonly family: string }) {
  const tone = familyTone(family);
  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <span
        className={cn(
          "inline-flex h-7 w-7 shrink-0 items-center justify-center border-2 border-border",
          "font-mono text-[10px] font-extrabold tracking-tight shadow-[1px_1px_0_var(--border)]",
          tone.bg,
          tone.text,
        )}
        aria-hidden
      >
        {familyInitials(family)}
      </span>
      <span className="truncate font-mono text-[13px] font-bold text-ink">{id}</span>
    </div>
  );
}

function RouteCell({
  free,
  provider,
  freeLabel,
  paidLabel,
  ollamaLabel,
}: {
  readonly free: boolean;
  readonly provider: ModelProvider;
  readonly freeLabel: string;
  readonly paidLabel: string;
  readonly ollamaLabel: string;
}) {
  const isOllama = provider === "ollama";
  const label = free ? freeLabel : isOllama ? ollamaLabel : paidLabel;
  return (
    <span className="inline-flex items-center gap-1.5 font-mono text-[12px] text-ink-muted">
      {free ? (
        <Globe size={14} weight="duotone" className="shrink-0 text-accent-yellow" aria-hidden />
      ) : (
        <Coins
          size={14}
          weight="duotone"
          className={cn("shrink-0", isOllama ? "text-accent-teal" : "text-accent-coral")}
          aria-hidden
        />
      )}
      <span className="truncate">{label}</span>
    </span>
  );
}

function routeLabel(
  model: ModelDTO,
  labels: { free: string; paid: string; ollama: string },
): string {
  if (model.free) return labels.free;
  if (modelProvider(model) === "ollama") return labels.ollama;
  return labels.paid;
}

function effortLevels(model: ModelDTO): string[] {
  return Array.isArray(model.effort_levels) ? [...model.effort_levels] : [];
}

function EffortLevelsCell({
  levels,
  emptyLabel,
  title,
}: {
  readonly levels: ReadonlyArray<string>;
  readonly emptyLabel: string;
  readonly title: string;
}) {
  if (levels.length === 0) {
    return (
      <span className="font-mono text-[11px] text-ink-muted" title={title}>
        {emptyLabel}
      </span>
    );
  }
  return (
    <div className="flex max-w-[18rem] flex-wrap gap-1" title={title}>
      {levels.map((level) => (
        <span
          key={level}
          className="inline-flex items-center border border-border bg-paper px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wide text-ink shadow-[1px_1px_0_var(--border)]"
        >
          {level}
        </span>
      ))}
    </div>
  );
}

function CacheCell({
  enabled,
  yesLabel,
  noLabel,
  title,
}: {
  readonly enabled: boolean;
  readonly yesLabel: string;
  readonly noLabel: string;
  readonly title: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center border px-2 py-0.5 font-mono text-[11px] font-bold shadow-[1px_1px_0_var(--border)]",
        enabled
          ? "border-border bg-accent-teal/15 text-ink"
          : "border-border bg-paper-2 text-ink-muted",
      )}
      title={title}
    >
      {enabled ? yesLabel : noLabel}
    </span>
  );
}

export function ModelsPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
  const [models, setModels] = useState<ModelDTO[]>([]);
  const [stale, setStale] = useState(false);
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState<KindFilter>("all");
  const [providerFilter, setProviderFilter] = useState<ProviderFilter>("all");
  const [familyFilter, setFamilyFilter] = useState("all");
  const [sortKey, setSortKey] = useState<SortKey>("id_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const routeLabels = useMemo(
    () => ({
      free: t("models.routeFree"),
      paid: t("models.routePaid"),
      ollama: t("models.routeOllama"),
    }),
    [t],
  );

  async function load(refresh = false) {
    if (refresh) setRefreshing(true);
    else setLoading(true);
    try {
      const res = refresh ? await api.refreshModels() : await api.models();
      setModels(res.models);
      setStale(res.stale);
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
      setRefreshing(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query, kind, providerFilter, familyFilter, sortKey]);

  const families = useMemo(() => {
    const set = new Set(models.map((m) => modelFamily(m.id)));
    return [...set].sort();
  }, [models]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = models.filter((model) => {
      const provider = modelProvider(model);
      if (kind === "free" && !model.free) return false;
      if (kind === "paid" && model.free) return false;
      if (providerFilter !== "all" && provider !== providerFilter) return false;
      if (familyFilter !== "all" && modelFamily(model.id) !== familyFilter) return false;
      if (!q) return true;
      const routeText = routeLabel(model, routeLabels).toLowerCase();
      const effortText = effortLevels(model).join(" ").toLowerCase();
      return (
        model.id.toLowerCase().includes(q) ||
        modelFamily(model.id).includes(q) ||
        provider.includes(q) ||
        (model.free ? "free public" : "paid").includes(q) ||
        routeText.includes(q) ||
        effortText.includes(q) ||
        (provider === "ollama" && "ollama".includes(q)) ||
        (provider === "opencode" && ("opencode".includes(q) || "zen".includes(q)))
      );
    });

    rows = [...rows].sort((a, b) => {
      if (sortKey === "id_asc") return a.id.localeCompare(b.id);
      if (sortKey === "id_desc") return b.id.localeCompare(a.id);
      if (sortKey === "free_first") {
        if (a.free !== b.free) return a.free ? -1 : 1;
        return a.id.localeCompare(b.id);
      }
      if (a.free !== b.free) return a.free ? 1 : -1;
      return a.id.localeCompare(b.id);
    });
    return rows;
  }, [models, query, kind, providerFilter, familyFilter, sortKey, routeLabels]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );

  const freeCount = models.filter((model) => model.free).length;
  const paidCount = models.length - freeCount;
  const filteredFree = filtered.filter((m) => m.free).length;

  function resetFilters() {
    setQuery("");
    setKind("all");
    setProviderFilter("all");
    setFamilyFilter("all");
    setSortKey("id_asc");
    setPage(1);
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("models.title")}
        meta={
          stale
            ? t("models.staleMeta")
            : t("models.summaryMeta", {
                total: models.length,
                free: freeCount,
                paid: paidCount,
              })
        }
        actions={
          <Button
            variant="secondary"
            size="sm"
            loading={refreshing}
            onClick={() => void load(true)}
          >
            <ArrowClockwise size={16} weight="bold" className="mr-1.5" aria-hidden />
            {t("common.refresh")}
          </Button>
        }
        className="!pb-3"
      />

      {loading ? (
        <div className="flex flex-col gap-3">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-24 w-full" />
            ))}
          </div>
          <Skeleton className="h-64 w-full" />
        </div>
      ) : null}
      {!loading && error ? (
        <ErrorState title={t("common.loadFailed")} description={error} />
      ) : null}

      {!loading && !error ? (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <StatCard
              label={t("models.kpi.total")}
              value={models.length}
              hint={t("models.kpi.showing") + ` ${filtered.length}`}
              icon={Stack}
              tone="default"
            />
            <StatCard
              label={t("models.kpi.free")}
              value={freeCount}
              hint={t("models.routeFree")}
              icon={Globe}
              tone="yellow"
            />
            <StatCard
              label={t("models.kpi.paid")}
              value={paidCount}
              hint={t("models.routePaid")}
              icon={Coins}
              tone="accent"
            />
            <StatCard
              label={t("models.kpi.families")}
              value={families.length}
              hint={
                familyFilter === "all"
                  ? t("common.all")
                  : familyFilter
              }
              icon={SquaresFour}
              tone="teal"
            />
          </div>

          <SectionPanel
            title={t("models.catalogTitle")}
            description={t("models.catalogDescription", {
              filtered: filtered.length,
              total: models.length,
              filteredFree,
              filteredPaid: filtered.length - filteredFree,
            })}
            icon={Stack}
            iconTone="yellow"
            bodyClassName="p-0"
          >
            <FilterStrip
              search={query}
              onSearchChange={setQuery}
              searchPlaceholder={t("models.searchPlaceholder")}
              filters={
                <>
                  <SegmentedFilter
                    aria-label={t("models.filterKindLabel")}
                    value={kind}
                    onChange={(v) => setKind(v as KindFilter)}
                    options={[
                      { value: "all", label: t("common.all") },
                      { value: "free", label: "Free" },
                      { value: "paid", label: "Paid" },
                    ]}
                  />
                  <SegmentedFilter
                    aria-label={t("models.filterProviderLabel")}
                    value={providerFilter}
                    onChange={(v) => setProviderFilter(v as ProviderFilter)}
                    options={[
                      { value: "all", label: t("models.filterProviderAll") },
                      { value: "opencode", label: t("models.filterProviderOpenCode") },
                      { value: "ollama", label: t("models.filterProviderOllama") },
                    ]}
                  />
                  <FilterSelect
                    label={t("models.filterFamilyLabel")}
                    value={familyFilter}
                    onChange={setFamilyFilter}
                    options={[
                      { value: "all", label: t("common.all") },
                      ...families.map((family) => ({ value: family, label: family })),
                    ]}
                  />
                </>
              }
              trailing={
                <>
                  <FilterSelect
                    label={t("models.filterSortLabel")}
                    value={sortKey}
                    onChange={(v) => setSortKey(v as SortKey)}
                    options={[
                      { value: "id_asc", label: t("models.sortIdAsc") },
                      { value: "id_desc", label: t("models.sortIdDesc") },
                      { value: "free_first", label: t("models.sortFreeFirst") },
                      { value: "paid_first", label: t("models.sortPaidFirst") },
                    ]}
                  />
                  <Button variant="secondary" size="sm" onClick={resetFilters}>
                    {t("models.reset")}
                  </Button>
                </>
              }
            />

            {models.length === 0 ? (
              <EmptyState
                icon={Stack}
                title={t("models.emptyTitle")}
                description={t("models.emptyDescription")}
                action={
                  <Button size="sm" loading={refreshing} onClick={() => void load(true)}>
                    {t("models.emptyAction")}
                  </Button>
                }
              />
            ) : filtered.length === 0 ? (
              <EmptyState
                compact
                icon={Stack}
                title={t("models.emptyFilteredTitle")}
                description={t("models.emptyFilteredDescription")}
                action={
                  <Button size="sm" variant="secondary" onClick={resetFilters}>
                    {t("models.reset")}
                  </Button>
                }
              />
            ) : (
              <div className="min-w-0 overflow-hidden">
                <div className="divide-y divide-border md:hidden">
                  {paged.map((model) => {
                    const family = modelFamily(model.id);
                    return (
                      <div
                        key={model.id}
                        className="flex items-start justify-between gap-3 px-3 py-3"
                      >
                        <div className="min-w-0">
                          <ModelIdCell id={model.id} family={family} />
                          <p className="mt-1.5 pl-[2.625rem] text-[12px] text-ink-muted">
                            {family} · {routeLabel(model, routeLabels)}
                          </p>
                          <div className="mt-2 pl-[2.625rem]">
                            <EffortLevelsCell
                              levels={effortLevels(model)}
                              emptyLabel={t("models.effortEmpty")}
                              title={t("models.effortHint")}
                            />
                          </div>
                        </div>
                        {model.free ? (
                          <Badge kind="free">free</Badge>
                        ) : (
                          <Badge kind="paid">paid</Badge>
                        )}
                      </div>
                    );
                  })}
                </div>
                <div className="hidden overflow-x-auto md:block">
                  <table className="w-full min-w-[56rem] text-left text-sm">
                    <thead>
                      <tr className="border-b-2 border-border bg-paper-2 font-mono text-[11px] font-bold uppercase tracking-wider text-ink-muted">
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.id")}
                        </th>
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.family")}
                        </th>
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.kind")}
                        </th>
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.effort")}
                        </th>
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.cache")}
                        </th>
                        <th className="whitespace-nowrap px-4 py-3">
                          {t("models.table.route")}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((model) => {
                        const family = modelFamily(model.id);
                        const provider = modelProvider(model);
                        return (
                          <tr
                            key={model.id}
                            className="border-b border-border/60 last:border-b-0 transition-colors hover:bg-paper-2/80"
                          >
                            <td className="whitespace-nowrap px-4 py-3">
                              <ModelIdCell id={model.id} family={family} />
                            </td>
                            <td className="whitespace-nowrap px-4 py-3">
                              <span className="inline-flex items-center border border-border bg-paper-2 px-2 py-0.5 font-mono text-[11px] font-bold text-ink shadow-[1px_1px_0_var(--border)]">
                                {family}
                              </span>
                            </td>
                            <td className="whitespace-nowrap px-4 py-3">
                              {model.free ? (
                                <Badge kind="free">free</Badge>
                              ) : (
                                <Badge kind="paid">paid</Badge>
                              )}
                            </td>
                            <td className="px-4 py-3">
                              <EffortLevelsCell
                                levels={effortLevels(model)}
                                emptyLabel={t("models.effortEmpty")}
                                title={t("models.effortHint")}
                              />
                            </td>
                            <td className="whitespace-nowrap px-4 py-3">
                              <CacheCell
                                enabled={model.cache_usage !== false}
                                yesLabel={t("models.cacheYes")}
                                noLabel={t("models.cacheNo")}
                                title={t("models.cacheHint")}
                              />
                            </td>
                            <td className="whitespace-nowrap px-4 py-3">
                              <RouteCell
                                free={model.free}
                                provider={provider}
                                freeLabel={routeLabels.free}
                                paidLabel={routeLabels.paid}
                                ollamaLabel={routeLabels.ollama}
                              />
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
                <Pagination
                  total={filtered.length}
                  page={page}
                  pageSize={pageSize}
                  onPageChange={setPage}
                  onPageSizeChange={(size) => {
                    setPageSize(size);
                    setPage(1);
                  }}
                />
              </div>
            )}
          </SectionPanel>
        </>
      ) : null}
    </div>
  );
}
