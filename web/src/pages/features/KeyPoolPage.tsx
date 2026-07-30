import {
  Coins,
  Key,
  PencilSimple,
  Plus,
  Pulse,
  Snowflake,
  Stack,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Badge,
  Button,
  DeleteButton,
  Dialog,
  EmptyState,
  EntityMark,
  FilterSelect,
  CompactField,
  ComposerPanel,
  HelpTip,
  ListToolbar,
  MetaChip,
  MobileEntityCard,
  PageHeader,
  Pagination,
  ResponsiveList,
  SecretInput,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  StatCard,
  Tabs,
  TextInput,
  fieldInputClass,
  slicePage,
  useToast,
} from "@/components";
import { api, ApiError, type KeyProvider, type ZenKeyDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n, type Translate } from "@/lib/i18n";
import { isProviderTab, type ProviderTab } from "@/lib/routes";
import {
  compareBySort,
  matchWeight,
  useRowSelection,
  type SortKey,
  type StatusFilter,
  type WeightFilter,
} from "@/lib/selection";
import { tableRowClass } from "@/lib/table-row";

function friendlyError(err: unknown, fallback: string, t: Translate): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return t("keypool.sessionExpired");
    return err.message || fallback;
  }
  if (err instanceof TypeError) return t("keypool.connectFailed");
  if (err instanceof Error) {
    if (/failed to fetch|networkerror|load failed/i.test(err.message)) {
      return t("keypool.connectFailed");
    }
    return err.message || fallback;
  }
  return fallback;
}

function useKeyProviderTab(): readonly [ProviderTab, (tab: ProviderTab) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  const tab: ProviderTab = isProviderTab(raw) ? raw : "opencode";
  function setTab(next: ProviderTab) {
    setParams(next === "opencode" ? {} : { tab: next }, { replace: true });
  }
  return [tab, setTab] as const;
}

export function KeyPoolPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
  const { push } = useToast();
  const [provider, setProvider] = useKeyProviderTab();
  const [keys, setKeys] = useState<ZenKeyDTO[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [weightFilter, setWeightFilter] = useState<WeightFilter>("all");
  const [sort, setSort] = useState<SortKey>("label_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [label, setLabel] = useState("");
  const [secret, setSecret] = useState("");
  const [weight, setWeight] = useState("1");
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<ZenKeyDTO | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editWeight, setEditWeight] = useState("1");
  const [editSecret, setEditSecret] = useState("");
  const [editSaving, setEditSaving] = useState(false);

  const keyProvider = provider as KeyProvider;
  const providerLabel = provider === "opencode" ? "OpenCode" : "Ollama";

  async function load() {
    setLoading(true);
    try {
      const res = await api.zenKeys(keyProvider);
      setKeys(res.keys);
      setListError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setListError(friendlyError(err, t("keypool.loadListFailed"), t));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setQuery("");
    setStatus("all");
    setWeightFilter("all");
    setSort("label_asc");
    setPage(1);
    setShowAdd(false);
    setEditing(null);
    void load();
  }, [provider]);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    if (!secret.trim()) {
      push(t("keypool.secretRequired", { provider: providerLabel }), "error");
      return;
    }
    setSaving(true);
    try {
      await api.createZenKey(
        label.trim() || `${provider}-key`,
        secret.trim(),
        Number(weight) || 1,
        keyProvider,
      );
      setLabel("");
      setSecret("");
      setWeight("1");
      setShowAdd(false);
      push(t("keypool.added"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("keypool.addFailed"), t), "error");
    } finally {
      setSaving(false);
    }
  }

  function openEdit(key: ZenKeyDTO) {
    setEditing(key);
    setEditLabel(key.label);
    setEditWeight(String(key.weight));
    setEditSecret("");
  }

  async function onSaveEdit(event: FormEvent) {
    event.preventDefault();
    if (!editing) return;
    if (!editLabel.trim()) {
      push(t("keypool.labelRequired"), "error");
      return;
    }
    setEditSaving(true);
    try {
      const payload: { label: string; weight: number; secret?: string } = {
        label: editLabel.trim(),
        weight: Number(editWeight) || 1,
      };
      if (editSecret.trim()) payload.secret = editSecret.trim();
      await api.updateZenKey(editing.id, payload);
      setEditing(null);
      push(t("keypool.saved"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("keypool.saveFailed"), t), "error");
    } finally {
      setEditSaving(false);
    }
  }

  const enabled = keys.filter((k) => k.enabled).length;
  const cooling = keys.filter((k) => k.cooldown_until).length;
  const totalWeight = keys.filter((k) => k.enabled && !k.cooldown_until).reduce((s, k) => s + k.weight, 0);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = keys.filter((k) => {
      if (status === "enabled" && !k.enabled) return false;
      if (status === "disabled" && k.enabled) return false;
      if (status === "cooling" && !k.cooldown_until) return false;
      if (!matchWeight(k.weight, weightFilter)) return false;
      if (!q) return true;
      return (
        k.label.toLowerCase().includes(q) ||
        k.prefix.toLowerCase().includes(q)
      );
    });
    list.sort((a, b) =>
      compareBySort(a, b, sort, {
        label: (x) => x.label,
        weight: (x) => x.weight,
        statusRank: (x) =>
          x.cooldown_until ? 2 : x.enabled ? 0 : 1,
      }),
    );
    return list;
  }, [keys, query, status, weightFilter, sort]);

  useEffect(() => {
    setPage(1);
  }, [query, status, weightFilter, sort]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );
  const visibleIds = useMemo(() => paged.map((k) => k.id), [paged]);
  const selection = useRowSelection(visibleIds);

  async function bulkSetEnabled(next: boolean) {
    if (selection.selected.size === 0) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.setZenKeyEnabled(id, next);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(
        next ? t("keypool.bulkEnabled", { n: ok }) : t("keypool.bulkDisabled", { n: ok }),
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
    if (!window.confirm(t("keypool.bulkDeleteConfirm", { n: selection.selected.size }))) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.deleteZenKey(id);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(t("keypool.bulkDeleted", { n: ok }), ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("keypool.title")}
        toolbar={
          <Tabs
            aria-label={t("keypool.providerTabAria")}
            value={provider}
            onChange={(id) => setProvider(id as ProviderTab)}
            items={[
              { id: "opencode", label: "OpenCode" },
              { id: "ollama", label: "Ollama" },
            ]}
          />
        }
        meta={
          <>
            <MetaChip>{t("keypool.metaCount", { n: keys.length })}</MetaChip>
            <MetaChip>{t("keypool.metaEnabled", { n: enabled })}</MetaChip>
            <MetaChip>{t("keypool.metaCooling", { n: cooling })}</MetaChip>
            <MetaChip>{t("keypool.metaWeight", { n: totalWeight })}</MetaChip>
            <HelpTip content={t("keypool.pollTip")} label={t("keypool.pollTipLabel")} />
          </>
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.refresh")}
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? t("keypool.collapse") : t("common.add")}
            </Button>
          </div>
        }
      />

      {!loading && !listError ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <StatCard
            label={t("kpi.total")}
            value={keys.length}
            hint={t("kpi.filtered") + ` ${filtered.length}`}
            icon={Stack}
            tone="default"
          />
          <StatCard
            label={t("kpi.enabled")}
            value={enabled}
            icon={Pulse}
            tone="success"
          />
          <StatCard
            label={t("kpi.cooling")}
            value={cooling}
            icon={Snowflake}
            tone={cooling > 0 ? "warning" : "default"}
          />
          <StatCard
            label={t("kpi.weight")}
            value={totalWeight}
            icon={Coins}
            tone="yellow"
          />
        </div>
      ) : null}

      {showAdd ? (
        <ComposerPanel
          title={t("keypool.addDialogTitle", { provider: providerLabel })}
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <p className="text-[12px] text-ink-faint">{t("keypool.weightHint")}</p>
              <Button type="submit" form="key-pool-create" size="sm" loading={saving}>
                {t("keypool.submit")}
              </Button>
            </>
          }
        >
          <form
            id="key-pool-create"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_1.3fr_6.5rem]"
            onSubmit={(e) => void onCreate(e)}
          >
            <CompactField label={t("keypool.labelField")}>
              <input
                className={fieldInputClass}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="primary"
              />
            </CompactField>
            <CompactField label={`${providerLabel} API Key`}>
              <input
                type="password"
                className={fieldInputClass}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                autoComplete="off"
                placeholder="sk-…"
              />
            </CompactField>
            <CompactField
              label={t("keypool.weightField")}
              tip={
                <HelpTip content={t("keypool.weightTip")} />
              }
            >
              <input
                className={fieldInputClass}
                value={weight}
                onChange={(e) => setWeight(e.target.value)}
                inputMode="numeric"
              />
            </CompactField>
          </form>
        </ComposerPanel>
      ) : null}

      <SectionPanel
        title={t("keypool.listTitle")}
        description={t("keypool.listDesc", { filtered: filtered.length, total: keys.length })}
        icon={Key}
        iconTone="yellow"
        bodyClassName="p-0"
      >
        <ListToolbar
          search={query}
          onSearchChange={setQuery}
          searchPlaceholder={t("keypool.searchPlaceholder")}
          selectedCount={selection.selected.size}
          totalVisible={paged.length}
          allSelected={selection.allSelected}
          onSelectAll={selection.toggleAll}
          onInvert={selection.invert}
          onClear={selection.clear}
          filters={
            <SegmentedFilter
              aria-label={t("keypool.statusAria")}
              value={status}
              onChange={(v) => setStatus(v as StatusFilter)}
              options={[
                { value: "all", label: t("common.all") },
                { value: "enabled", label: t("common.enabled") },
                { value: "disabled", label: t("common.disabled") },
                { value: "cooling", label: t("keypool.statusCooling") },
              ]}
            />
          }
          trailing={
            <>
              <FilterSelect
                label={t("keypool.weightFilterLabel")}
                value={weightFilter}
                onChange={(v) => setWeightFilter(v as WeightFilter)}
                options={[
                  { value: "all", label: t("common.all") },
                  { value: "1", label: t("keypool.weightEq1") },
                  { value: "ge2", label: t("keypool.weightGe2") },
                  { value: "ge5", label: t("keypool.weightGe5") },
                ]}
              />
              <FilterSelect
                label={t("keypool.sortLabel")}
                value={sort}
                onChange={(v) => setSort(v as SortKey)}
                options={[
                  { value: "label_asc", label: t("keypool.sortLabelAsc") },
                  { value: "label_desc", label: t("keypool.sortLabelDesc") },
                  { value: "weight_desc", label: t("keypool.sortWeightDesc") },
                  { value: "weight_asc", label: t("keypool.sortWeightAsc") },
                  { value: "status", label: t("keypool.sortStatus") },
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
            icon={Key}
            title={t("keypool.loadFailedTitle")}
            description={listError}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                {t("common.retry")}
              </Button>
            }
          />
        ) : keys.length === 0 ? (
          <EmptyState
            compact
            icon={Key}
            title={t("keypool.emptyTitle")}
            description={
              provider === "opencode"
                ? t("keypool.emptyDescOc")
                : t("keypool.emptyDescOl")
            }
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                {t("keypool.addKey")}
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState
            compact
            icon={Key}
            title={t("keypool.noMatchTitle")}
            description={t("keypool.noMatchDesc")}
          />
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
                aria-label={t("keypool.selectAllAria")}
              />
              <span className="text-[12px] text-ink-muted">{t("keypool.selectAllPage")}</span>
            </div>
            <ResponsiveList
              mobile={
                <div>
                  {paged.map((key) => {
                    const healthy = key.enabled && !key.cooldown_until;
                    const share =
                      healthy && totalWeight > 0
                        ? `${Math.round((key.weight / totalWeight) * 100)}%`
                        : "-";
                    return (
                      <MobileEntityCard
                        key={key.id}
                        muted={!key.enabled}
                        leading={
                          <input
                            type="checkbox"
                            checked={selection.selected.has(key.id)}
                            onChange={() => selection.toggleOne(key.id)}
                            aria-label={t("keypool.selectRowAria", { label: key.label })}
                          />
                        }
                        title={
                          <span className="inline-flex min-w-0 items-center gap-2">
                            <EntityMark name={key.label} size="sm" />
                            <span className="truncate">{key.label}</span>
                          </span>
                        }
                        subtitle={
                          <span className="font-mono text-[11px]">{key.prefix}</span>
                        }
                        badge={
                          key.cooldown_until ? (
                            <Badge kind="warning">{t("keypool.statusCooling")}</Badge>
                          ) : key.enabled ? (
                            <Badge kind="healthy">{t("common.enabled")}</Badge>
                          ) : (
                            <Badge kind="neutral">{t("common.disabled")}</Badge>
                          )
                        }
                        fields={[
                          { label: t("keypool.weightField"), value: key.weight },
                          { label: t("keypool.shareLabel"), value: share },
                          {
                            label: t("keypool.coolingLabel"),
                            value: key.cooldown_until
                              ? new Date(key.cooldown_until).toLocaleString()
                              : "-",
                          },
                        ]}
                        actions={
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              aria-label={t("keypool.editAria")}
                              onClick={() => openEdit(key)}
                            >
                              <PencilSimple size={14} />
                            </Button>
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() =>
                                void api
                                  .setZenKeyEnabled(key.id, !key.enabled)
                                  .then(load)
                                  .catch((err) =>
                                    push(friendlyError(err, t("common.actionFailed"), t), "error"),
                                  )
                              }
                            >
                              {key.enabled ? t("common.disable") : t("common.enable")}
                            </Button>
                            <DeleteButton
                              onClick={() => {
                                if (window.confirm(t("keypool.deleteConfirm", { label: key.label }))) {
                                  void api
                                    .deleteZenKey(key.id)
                                    .then(load)
                                    .catch((err) =>
                                      push(
                                        friendlyError(err, t("keypool.deleteFailed"), t),
                                        "error",
                                      ),
                                    );
                                }
                              }}
                            />
                          </>
                        }
                      />
                    );
                  })}
                </div>
              }
              desktop={
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[42rem] text-left text-sm">
                    <thead>
                      <tr className="border-b-2 border-border bg-paper-0 text-caption text-ink-muted">
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
                            aria-label={t("keypool.selectAllAria")}
                          />
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("keypool.colLabel")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("keypool.colPrefix")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">
                          <span className="inline-flex items-center gap-1">
                            {t("keypool.colWeight")}
                            <HelpTip content={t("keypool.weightColTip")} />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("keypool.colShare")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("keypool.colStatus")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("keypool.colCooling")}</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          {t("keypool.colActions")}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((key) => {
                        const healthy = key.enabled && !key.cooldown_until;
                        const share =
                          healthy && totalWeight > 0
                            ? `${Math.round((key.weight / totalWeight) * 100)}%`
                            : "-";
                        return (
                          <tr
                            key={key.id}
                            className={tableRowClass(!key.enabled)}
                            aria-disabled={!key.enabled}
                          >
                            <td className="px-3 py-2.5">
                              <input
                                type="checkbox"
                                checked={selection.selected.has(key.id)}
                                onChange={() => selection.toggleOne(key.id)}
                                aria-label={t("keypool.selectRowAria", { label: key.label })}
                              />
                            </td>
                            <td
                              className={`whitespace-nowrap px-3 py-2.5 font-medium ${key.enabled ? "text-ink" : "text-ink-faint"}`}
                            >
                              <span className="inline-flex min-w-0 items-center gap-2.5">
                                <EntityMark name={key.label} size="sm" />
                                <span className="truncate">{key.label}</span>
                              </span>
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                              {key.prefix}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              <span
                                className={`inline-flex items-center gap-1 font-mono tabular-nums ${key.enabled ? "text-ink" : "text-ink-faint"}`}
                              >
                                <Coins
                                  size={14}
                                  className={
                                    key.enabled ? "text-accent" : "text-ink-faint"
                                  }
                                  weight="duotone"
                                />
                                {key.weight}
                              </span>
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 tabular-nums text-ink-muted">
                              {share}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              {key.cooldown_until ? (
                                <Badge kind="warning">{t("keypool.statusCooling")}</Badge>
                              ) : key.enabled ? (
                                <Badge kind="healthy">{t("common.enabled")}</Badge>
                              ) : (
                                <Badge kind="neutral">{t("common.disabled")}</Badge>
                              )}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 text-[12px] text-ink-muted">
                              {key.cooldown_until
                                ? new Date(key.cooldown_until).toLocaleString()
                                : "-"}
                            </td>
                            <td className="px-3 py-2.5">
                              <div className="flex justify-end gap-1.5">
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  aria-label={t("keypool.editAria")}
                                  onClick={() => openEdit(key)}
                                >
                                  <PencilSimple size={14} />
                                </Button>
                                <Button
                                  variant="secondary"
                                  size="sm"
                                  onClick={() =>
                                    void api
                                      .setZenKeyEnabled(key.id, !key.enabled)
                                      .then(load)
                                      .catch((err) =>
                                        push(
                                          friendlyError(err, t("common.actionFailed"), t),
                                          "error",
                                        ),
                                      )
                                  }
                                >
                                  {key.enabled ? t("common.disable") : t("common.enable")}
                                </Button>
                                <DeleteButton
                                  onClick={() => {
                                    if (window.confirm(t("keypool.deleteConfirm", { label: key.label }))) {
                                      void api
                                        .deleteZenKey(key.id)
                                        .then(load)
                                        .catch((err) =>
                                          push(
                                            friendlyError(err, t("keypool.deleteFailed"), t),
                                            "error",
                                          ),
                                        );
                                    }
                                  }}
                                />
                              </div>
                            </td>
                          </tr>
                        );
                      })}
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
        title={t("keypool.editDialogTitle", { provider: providerLabel })}
        onClose={() => setEditing(null)}
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onSaveEdit(e)}>
          <TextInput
            label={t("keypool.labelField")}
            value={editLabel}
            onChange={(e) => setEditLabel(e.target.value)}
          />
          <div>
            <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
              {t("keypool.weightField")}
              <HelpTip content={t("keypool.weightTipShort")} />
            </label>
            <input
              className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
              value={editWeight}
              onChange={(e) => setEditWeight(e.target.value)}
              inputMode="numeric"
            />
          </div>
          <SecretInput
            label={t("keypool.newSecretLabel")}
            value={editSecret}
            onChange={(e) => setEditSecret(e.target.value)}
            placeholder={t("keypool.newSecretPlaceholder")}
          />
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
