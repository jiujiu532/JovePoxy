import { ArrowClockwise, Stack } from "@phosphor-icons/react";
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
  slicePage,
} from "@/components";
import { api, ApiError, type ModelDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n } from "@/lib/i18n";

/** free≈public, paid≈zen — single axis. */
type KindFilter = "all" | "free" | "paid";
type SortKey = "id_asc" | "id_desc" | "free_first" | "paid_first";

function modelFamily(id: string): string {
  const parts = id.split(/[-_/]/).filter(Boolean);
  return (parts[0] ?? id).toLowerCase();
}

export function ModelsPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
  const [models, setModels] = useState<ModelDTO[]>([]);
  const [stale, setStale] = useState(false);
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState<KindFilter>("all");
  const [familyFilter, setFamilyFilter] = useState("all");
  const [sortKey, setSortKey] = useState<SortKey>("id_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

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
  }, [query, kind, familyFilter, sortKey]);

  const families = useMemo(() => {
    const set = new Set(models.map((m) => modelFamily(m.id)));
    return [...set].sort();
  }, [models]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = models.filter((model) => {
      if (kind === "free" && !model.free) return false;
      if (kind === "paid" && model.free) return false;
      if (familyFilter !== "all" && modelFamily(model.id) !== familyFilter) return false;
      if (!q) return true;
      return (
        model.id.toLowerCase().includes(q) ||
        modelFamily(model.id).includes(q) ||
        (model.free ? "free public" : "paid zen").includes(q)
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
  }, [models, query, kind, familyFilter, sortKey]);

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
            : t("models.summaryMeta", { total: models.length, free: freeCount, paid: paidCount })
        }
        actions={
          <Button variant="secondary" size="sm" loading={refreshing} onClick={() => void load(true)}>
            <ArrowClockwise size={16} className="mr-1.5" />
            {t("common.refresh")}
          </Button>
        }
        className="!pb-3"
      />

      {loading ? <Skeleton className="h-48 w-full" /> : null}
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}

      {!loading && !error ? (
        <SectionPanel
          title={t("models.catalogTitle")}
          description={t("models.catalogDescription", {
            filtered: filtered.length,
            total: models.length,
            filteredFree,
            filteredPaid: filtered.length - filteredFree,
          })}
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
              action={
                <Button size="sm" loading={refreshing} onClick={() => void load(true)}>
                  {t("models.emptyAction")}
                </Button>
              }
            />
          ) : filtered.length === 0 ? (
            <EmptyState
              compact
              title={t("models.emptyFilteredTitle")}
            />
          ) : (
            <div className="min-w-0 overflow-hidden">
              <div className="divide-y divide-border md:hidden">
                {paged.map((model) => (
                  <div key={model.id} className="flex items-start justify-between gap-3 px-3 py-3">
                    <div className="min-w-0">
                      <p className="truncate font-mono text-[13px] font-medium text-ink">
                        {model.id}
                      </p>
                      <p className="mt-0.5 text-[12px] text-ink-muted">
                        {modelFamily(model.id)} ·{" "}
                        {model.free ? t("models.routeFree") : t("models.routePaid")}
                      </p>
                    </div>
                    {model.free ? (
                      <Badge kind="warning">free</Badge>
                    ) : (
                      <Badge kind="neutral">paid</Badge>
                    )}
                  </div>
                ))}
              </div>
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full min-w-[36rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="whitespace-nowrap px-3 py-2 font-medium">{t("models.table.id")}</th>
                      <th className="whitespace-nowrap px-3 py-2 font-medium">{t("models.table.family")}</th>
                      <th className="whitespace-nowrap px-3 py-2 font-medium">{t("models.table.kind")}</th>
                      <th className="whitespace-nowrap px-3 py-2 font-medium">{t("models.table.route")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paged.map((model) => (
                      <tr
                        key={model.id}
                        className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                      >
                        <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[13px] text-ink">
                          {model.id}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5 text-ink-muted">
                          {modelFamily(model.id)}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5">
                          {model.free ? (
                            <Badge kind="warning">free</Badge>
                          ) : (
                            <Badge kind="neutral">paid</Badge>
                          )}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5 text-[12px] text-ink-muted">
                          {model.free ? t("models.routeFree") : t("models.routePaid")}
                        </td>
                      </tr>
                    ))}
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
      ) : null}
    </div>
  );
}
