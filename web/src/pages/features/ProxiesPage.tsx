import { Globe, Path, PencilSimple, Plus } from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  ComposerPanel,
  DeleteButton,
  Dialog,
  EmptyState,
  FilterSelect,
  HelpTip,
  ListToolbar,
  MetaChip,
  MobileEntityCard,
  PageHeader,
  Pagination,
  ResponsiveList,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  TextInput,
  fieldInputClass,
  slicePage,
  useToast,
} from "@/components";
import { api, ApiError } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { cn } from "@/lib/cn";
import { useI18n, type Translate } from "@/lib/i18n";
import {
  compareBySort,
  matchWeight,
  useRowSelection,
  type SortKey,
  type StatusFilter,
  type WeightFilter,
} from "@/lib/selection";
import { tableRowClass } from "@/lib/table-row";

type ProxyRow = {
  id: string;
  label: string;
  scheme: string;
  host: string;
  weight: number;
  enabled: boolean;
  cooldown_until?: string;
};

function friendlyError(err: unknown, fallback: string, t: Translate): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return t("proxies.sessionExpired");
    return err.message || fallback;
  }
  if (err instanceof TypeError) return t("proxies.connectFailed");
  if (err instanceof Error) {
    if (/failed to fetch|networkerror|load failed/i.test(err.message)) {
      return t("proxies.connectFailed");
    }
    return err.message || fallback;
  }
  return fallback;
}

/** Parse multi-line proxy batch: `url` or `label|url` or `label|url|weight` */
function parseProxyLines(text: string): Array<{ label: string; url: string; weight: number }> {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"))
    .map((line, index) => {
      const parts = line.split("|").map((p) => p.trim());
      if (parts.length === 1) {
        return { label: `proxy-${index + 1}`, url: parts[0] ?? "", weight: 1 };
      }
      if (parts.length === 2) {
        return { label: parts[0] || `proxy-${index + 1}`, url: parts[1] ?? "", weight: 1 };
      }
      const w = Number(parts[2]);
      return {
        label: parts[0] || `proxy-${index + 1}`,
        url: parts[1] ?? "",
        weight: Number.isFinite(w) && w > 0 ? w : 1,
      };
    })
    .filter((row) => row.url.length > 0);
}

export function ProxiesPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const { t } = useI18n();
  const [rows, setRows] = useState<ProxyRow[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [scheme, setScheme] = useState("all");
  const [weightFilter, setWeightFilter] = useState<WeightFilter>("all");
  const [sort, setSort] = useState<SortKey>("label_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [batch, setBatch] = useState("");
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<ProxyRow | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editWeight, setEditWeight] = useState("1");
  const [editUrl, setEditUrl] = useState("");
  const [editSaving, setEditSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const res = await api.proxies();
      const next: ProxyRow[] = res.proxies.map((p) => {
        const row: ProxyRow = {
          id: p.id,
          label: p.label,
          scheme: p.scheme,
          host: p.host,
          weight: p.weight,
          enabled: p.enabled,
        };
        if (p.cooldown_until) row.cooldown_until = p.cooldown_until;
        return row;
      });
      setRows(next);
      setListError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setListError(friendlyError(err, t("common.loadFailed"), t));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    const items = parseProxyLines(batch);
    if (items.length === 0) {
      push(t("proxies.pasteAtLeastOne"), "error");
      return;
    }
    setSaving(true);
    let ok = 0;
    let fail = 0;
    try {
      for (const item of items) {
        try {
          await api.createProxy(item.label, item.url, item.weight);
          ok += 1;
        } catch {
          fail += 1;
        }
      }
      if (ok > 0) {
        setBatch("");
        setShowAdd(false);
        push(
          fail > 0 ? t("proxies.addResultMixed", { ok, fail }) : t("proxies.addResultAll", { n: ok }),
          fail > 0 ? "info" : "success",
        );
        await load();
      } else {
        push(t("proxies.addAllFailed"), "error");
      }
    } finally {
      setSaving(false);
    }
  }

  function openEdit(row: ProxyRow) {
    setEditing(row);
    setEditLabel(row.label);
    setEditWeight(String(row.weight));
    setEditUrl("");
  }

  async function onSaveEdit(event: FormEvent) {
    event.preventDefault();
    if (!editing) return;
    if (!editLabel.trim()) {
      push(t("proxies.labelRequired"), "error");
      return;
    }
    setEditSaving(true);
    try {
      const payload: { label: string; weight: number; url?: string } = {
        label: editLabel.trim(),
        weight: Number(editWeight) || 1,
      };
      if (editUrl.trim()) payload.url = editUrl.trim();
      await api.updateProxy(editing.id, payload);
      setEditing(null);
      push(t("proxies.saved"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("proxies.saveFailed"), t), "error");
    } finally {
      setEditSaving(false);
    }
  }

  const enabled = rows.filter((r) => r.enabled).length;
  const cooling = rows.filter((r) => r.cooldown_until).length;
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = rows.filter((r) => {
      if (status === "enabled" && !r.enabled) return false;
      if (status === "disabled" && r.enabled) return false;
      if (status === "cooling" && !r.cooldown_until) return false;
      if (scheme !== "all" && r.scheme !== scheme) return false;
      if (!matchWeight(r.weight, weightFilter)) return false;
      if (!q) return true;
      return (
        r.label.toLowerCase().includes(q) ||
        r.host.toLowerCase().includes(q) ||
        r.scheme.toLowerCase().includes(q)
      );
    });
    list.sort((a, b) =>
      compareBySort(a, b, sort, {
        label: (x) => x.label,
        weight: (x) => x.weight,
        host: (x) => x.host,
        statusRank: (x) =>
          x.cooldown_until ? 2 : x.enabled ? 0 : 1,
      }),
    );
    return list;
  }, [rows, query, status, scheme, weightFilter, sort]);

  useEffect(() => {
    setPage(1);
  }, [query, status, scheme, weightFilter, sort]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );
  const visibleIds = useMemo(() => paged.map((r) => r.id), [paged]);
  const selection = useRowSelection(visibleIds);

  async function bulkSetEnabled(next: boolean) {
    if (selection.selected.size === 0) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.setProxyEnabled(id, next);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(
        next ? t("proxies.enabledBulk", { n: ok }) : t("proxies.disabledBulk", { n: ok }),
        ok > 0 ? "success" : "error",
      );
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  async function bulkDelete() {
    if (selection.selected.size === 0) return;
    if (!window.confirm(t("proxies.bulkDeleteConfirm", { n: selection.selected.size }))) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.deleteProxy(id);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(t("proxies.deletedBulk", { n: ok }), ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  const parsedCount = parseProxyLines(batch).length;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("proxies.title")}
        meta={
          <>
            <MetaChip>{t("proxies.metaNodes", { n: rows.length })}</MetaChip>
            <MetaChip>{t("proxies.metaEnabled", { n: enabled })}</MetaChip>
            <MetaChip>{t("proxies.metaCooling", { n: cooling })}</MetaChip>
            <HelpTip content={t("proxies.behaviorTip")} label={t("proxies.behaviorTipLabel")} />
          </>
        }
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.refresh")}
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? t("proxies.collapse") : t("proxies.addBatch")}
            </Button>
          </div>
        }
      />

      {showAdd ? (
        <ComposerPanel
          title={t("proxies.composerTitle")}
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <div className="flex flex-wrap items-center gap-1.5">
                <MetaChip>{t("proxies.parsedCount", { n: parsedCount })}</MetaChip>
                <span className="text-[11px] text-ink-faint">{t("proxies.commentHint")}</span>
              </div>
              <Button type="submit" form="proxy-batch-create" size="sm" loading={saving}>
                {t("proxies.writeToPool")}
              </Button>
            </>
          }
        >
          <form id="proxy-batch-create" onSubmit={(e) => void onCreate(e)}>
            <textarea
              className={cn(
                fieldInputClass,
                "min-h-[9.5rem] resize-y py-3 font-mono text-[12px] leading-relaxed",
              )}
              value={batch}
              onChange={(e) => setBatch(e.target.value)}
              placeholder={t("proxies.placeholder")}
              spellCheck={false}
            />
            <div className="mt-3 flex flex-wrap gap-1.5">
              {[
                "socks5h://…",
                "label|url|weight",
                "http / https",
              ].map((tip) => (
                <span
                  key={tip}
                  className="rounded-none border border-border bg-paper-0 px-2.5 py-1 text-[11px] text-ink-faint"
                >
                  {tip}
                </span>
              ))}
            </div>
          </form>
        </ComposerPanel>
      ) : null}

      <SectionPanel
        title={t("proxies.listTitle")}
        description={t("proxies.listStats", {
          filtered: filtered.length,
          total: rows.length,
          enabled,
          cooling,
        })}
        bodyClassName="p-0"
      >
        <ListToolbar
          search={query}
          onSearchChange={setQuery}
          searchPlaceholder={t("proxies.searchPlaceholder")}
          selectedCount={selection.selected.size}
          totalVisible={paged.length}
          allSelected={selection.allSelected}
          onSelectAll={selection.toggleAll}
          onInvert={selection.invert}
          onClear={selection.clear}
          filters={
            <SegmentedFilter
              aria-label={t("proxies.statusAria")}
              value={status}
              onChange={(v) => setStatus(v as StatusFilter)}
              options={[
                { value: "all", label: t("common.all") },
                { value: "enabled", label: t("common.enabled") },
                { value: "disabled", label: t("common.disabled") },
                { value: "cooling", label: t("proxies.statusCooling") },
              ]}
            />
          }
          trailing={
            <>
              <FilterSelect
                label={t("proxies.schemeLabel")}
                value={scheme}
                onChange={setScheme}
                options={[
                  { value: "all", label: t("common.all") },
                  { value: "http", label: "http" },
                  { value: "https", label: "https" },
                  { value: "socks5", label: "socks5" },
                  { value: "socks5h", label: "socks5h" },
                ]}
              />
              <FilterSelect
                label={t("proxies.weightFilterLabel")}
                value={weightFilter}
                onChange={(v) => setWeightFilter(v as WeightFilter)}
                options={[
                  { value: "all", label: t("common.all") },
                  { value: "1", label: t("proxies.weightEq1") },
                  { value: "ge2", label: t("proxies.weightGe2") },
                  { value: "ge5", label: t("proxies.weightGe5") },
                ]}
              />
              <FilterSelect
                label={t("proxies.sortLabel")}
                value={sort}
                onChange={(v) => setSort(v as SortKey)}
                options={[
                  { value: "label_asc", label: t("proxies.sortLabelAsc") },
                  { value: "host_asc", label: t("proxies.sortHost") },
                  { value: "weight_desc", label: t("proxies.sortWeightDesc") },
                  { value: "status", label: t("proxies.sortStatus") },
                ]}
              />
            </>
          }
          bulkActions={
            <>
              <Button
                variant="secondary"
                size="sm"
                loading={bulkBusy}
                onClick={() => void bulkSetEnabled(true)}
              >
                {t("common.enable")}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={bulkBusy}
                onClick={() => void bulkSetEnabled(false)}
              >
                {t("common.disable")}
              </Button>
              <DeleteButton loading={bulkBusy} onClick={() => void bulkDelete()} />
            </>
          }
        />
        {loading ? (
          <div className="space-y-2 p-3">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : listError ? (
          <EmptyState
            compact
            icon={Globe}
            title={t("proxies.loadFailedTitle")}
            description={listError}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                {t("common.retry")}
              </Button>
            }
          />
        ) : rows.length === 0 ? (
          <EmptyState
            compact
            icon={Globe}
            title={t("proxies.emptyTitle")}
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                {t("proxies.addBatch")}
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState compact title={t("proxies.noMatchTitle")} description={t("proxies.noMatchDescription")} />
        ) : (
          <div className="min-w-0">
            <div className="flex items-center gap-2 border-b border-border bg-paper-0/40 px-3 py-2 md:hidden">
              <input
                type="checkbox"
                checked={selection.allSelected}
                ref={(el) => {
                  if (el) {
                    el.indeterminate =
                      selection.someSelected && !selection.allSelected;
                  }
                }}
                onChange={selection.toggleAll}
                aria-label={t("proxies.selectAllAria")}
              />
              <span className="text-[12px] text-ink-muted">{t("proxies.selectAllPageLabel")}</span>
            </div>
            <ResponsiveList
              mobile={
                <div>
                  {paged.map((row) => (
                    <MobileEntityCard
                      key={row.id}
                      muted={!row.enabled}
                      leading={
                        <input
                          type="checkbox"
                          checked={selection.selected.has(row.id)}
                          onChange={() => selection.toggleOne(row.id)}
                          aria-label={t("proxies.selectRowAria", { label: row.label })}
                        />
                      }
                      title={row.label}
                      subtitle={
                        <span className="font-mono text-[11px]">
                          {row.scheme} · {row.host}
                        </span>
                      }
                      badge={
                        row.cooldown_until ? (
                          <Badge kind="warning">{t("proxies.statusCooling")}</Badge>
                        ) : row.enabled ? (
                          <Badge kind="healthy">{t("common.enabled")}</Badge>
                        ) : (
                          <Badge kind="neutral">{t("common.disabled")}</Badge>
                        )
                      }
                      fields={[
                        { label: t("proxies.weightFilterLabel"), value: row.weight },
                        {
                          label: t("proxies.statusCooling"),
                          value: row.cooldown_until
                            ? new Date(row.cooldown_until).toLocaleString()
                            : t("common.none"),
                        },
                      ]}
                      actions={
                        <>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label={t("proxies.editAria")}
                            onClick={() => openEdit(row)}
                          >
                            <PencilSimple size={14} />
                          </Button>
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() =>
                              void api
                                .setProxyEnabled(row.id, !row.enabled)
                                .then(load)
                                .catch((err) =>
                                  push(friendlyError(err, t("common.actionFailed"), t), "error"),
                                )
                            }
                          >
                            {row.enabled ? t("common.disable") : t("common.enable")}
                          </Button>
                          <DeleteButton
                            onClick={() => {
                              if (window.confirm(t("proxies.deleteConfirm", { label: row.label }))) {
                                void api
                                  .deleteProxy(row.id)
                                  .then(load)
                                  .catch((err) =>
                                    push(friendlyError(err, t("proxies.deleteFailed"), t), "error"),
                                  );
                              }
                            }}
                          />
                        </>
                      }
                    />
                  ))}
                </div>
              }
              desktop={
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[42rem] text-left text-sm">
                    <thead>
                      <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                        <th className="w-10 px-3 py-2">
                          <input
                            type="checkbox"
                            checked={selection.allSelected}
                            ref={(el) => {
                              if (el) {
                                el.indeterminate =
                                  selection.someSelected && !selection.allSelected;
                              }
                            }}
                            onChange={selection.toggleAll}
                            aria-label={t("proxies.selectAllAria")}
                          />
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("proxies.colLabel")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("proxies.schemeLabel")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("proxies.sortHost")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">
                          <span className="inline-flex items-center gap-1">
                            {t("proxies.weightFilterLabel")}
                            <HelpTip content={t("proxies.weightTip")} />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("proxies.statusAria")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("proxies.statusCooling")}</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          {t("proxies.colActions")}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((row) => (
                        <tr
                          key={row.id}
                          className={tableRowClass(!row.enabled)}
                          aria-disabled={!row.enabled}
                        >
                          <td className="px-3 py-2.5">
                            <input
                              type="checkbox"
                              checked={selection.selected.has(row.id)}
                              onChange={() => selection.toggleOne(row.id)}
                              aria-label={t("proxies.selectRowAria", { label: row.label })}
                            />
                          </td>
                          <td
                            className={`whitespace-nowrap px-3 py-2.5 font-medium ${row.enabled ? "text-ink" : "text-ink-faint"}`}
                          >
                            {row.label}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {row.scheme}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {row.host}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5">
                            <span className="inline-flex items-center gap-1 tabular-nums">
                              <Path
                                size={14}
                                className={
                                  row.enabled ? "text-accent" : "text-ink-faint"
                                }
                                weight="duotone"
                              />
                              {row.weight}
                            </span>
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5">
                            {row.cooldown_until ? (
                              <Badge kind="warning">{t("proxies.statusCooling")}</Badge>
                            ) : row.enabled ? (
                              <Badge kind="healthy">{t("common.enabled")}</Badge>
                            ) : (
                              <Badge kind="neutral">{t("common.disabled")}</Badge>
                            )}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 text-[12px] text-ink-muted">
                            {row.cooldown_until
                              ? new Date(row.cooldown_until).toLocaleString()
                              : t("common.none")}
                          </td>
                          <td className="px-3 py-2.5">
                            <div className="flex justify-end gap-1.5">
                              <Button
                                variant="ghost"
                                size="sm"
                                aria-label={t("proxies.editAria")}
                                onClick={() => openEdit(row)}
                              >
                                <PencilSimple size={14} />
                              </Button>
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() =>
                                  void api
                                    .setProxyEnabled(row.id, !row.enabled)
                                    .then(load)
                                    .catch((err) =>
                                      push(friendlyError(err, t("common.actionFailed"), t), "error"),
                                    )
                                }
                              >
                                {row.enabled ? t("common.disable") : t("common.enable")}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(t("proxies.deleteConfirm", { label: row.label }))) {
                                    void api
                                      .deleteProxy(row.id)
                                      .then(load)
                                      .catch((err) =>
                                        push(
                                          friendlyError(err, t("proxies.deleteFailed"), t),
                                          "error",
                                        ),
                                      );
                                  }
                                }}
                              />
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              }
            />
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

      <Dialog
        open={editing !== null}
        title={t("proxies.editTitle")}
        onClose={() => setEditing(null)}
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onSaveEdit(e)}>
          <TextInput
            label={t("proxies.colLabel")}
            value={editLabel}
            onChange={(e) => setEditLabel(e.target.value)}
          />
          <div>
            <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
              {t("proxies.weightFilterLabel")}
              <HelpTip content={t("proxies.weightTipEdit")} />
            </label>
            <input
              className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
              value={editWeight}
              onChange={(e) => setEditWeight(e.target.value)}
              inputMode="numeric"
            />
          </div>
          <TextInput
            label={t("proxies.newUrlLabel")}
            value={editUrl}
            onChange={(e) => setEditUrl(e.target.value)}
            placeholder={t("proxies.newUrlPlaceholder")}
          />
          {editing ? (
            <p className="font-mono text-[12px] text-ink-faint">
              {t("proxies.currentUrl", { scheme: editing.scheme, host: editing.host })}
            </p>
          ) : null}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" size="sm" onClick={() => setEditing(null)}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" size="sm" loading={editSaving}>
              {t("common.save")}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
